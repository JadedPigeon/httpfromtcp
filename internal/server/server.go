package server

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"httpfromtcp/internal/request"
	"httpfromtcp/internal/response"
)

// Contains the state of the server
type Server struct {
	listener net.Listener
	closed   atomic.Bool
	wg       sync.WaitGroup
	handler  Handler
}

// Creates a net.Listener and returns a new Server instance. Starts listening for requests inside a goroutine.
func Serve(port int, handler Handler) (*Server, error) {
	ln, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		return nil, err
	}

	s := &Server{listener: ln, handler: handler}
	s.closed.Store(false)
	go s.listen()
	return s, nil
}

// Closes the listener and the server
func (s *Server) Close() error {
	// Mark as closed so listen loop can exit cleanly on Accept errors
	s.closed.Store(true)
	// Closing the listener will cause Accept to return an error and the
	// listen goroutine to exit.
	if s.listener != nil {
		_ = s.listener.Close()
	}
	// Wait for any active handlers to finish
	s.wg.Wait()
	return nil
}

// Uses a loop to .Accept new connections as they come in, and handles each one in a new goroutine.
// We use an atomic.Bool to track whether the server is closed so Accept errors can be ignored after close.
func (s *Server) listen() {
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			// If server is closed, exit the listen loop. Otherwise, continue accepting.
			if s.closed.Load() {
				return
			}
			// transient error; continue
			continue
		}
		// Handle connection in its own goroutine and track with waitgroup
		s.wg.Add(1)
		go s.handle(conn)
	}
}

func (s *Server) handle(conn net.Conn) {
	defer s.wg.Done()
	defer conn.Close()
	log.Println("handle: new connection")

	// Add a read deadline so a client that stops sending can't hang the server
	_ = conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	// Parse the request from the connection
	req, err := request.RequestFromReader(conn)
	log.Printf("handle: RequestFromReader returned, err=%v\n", err)
	if err != nil {
		// If it was a timeout, return a clearer message and log
		if ne, ok := err.(net.Error); ok && ne.Timeout() {
			log.Printf("handle: read timeout from %s: %v", conn.RemoteAddr(), err)
			he := HandlerError{StatusCode: response.StatusBadRequest, Message: "request read timeout\n"}
			he.Write(conn)
			return
		}
		he := HandlerError{StatusCode: response.StatusBadRequest, Message: err.Error()}
		he.Write(conn)
		log.Println("handle: wrote bad request error")
		return
	}
	// Clear the read deadline now that we've successfully read the request
	_ = conn.SetReadDeadline(time.Time{})
	log.Printf("handle: parsed request line: method=%s target=%s version=%s\n",
		req.RequestLine.Method,
		req.RequestLine.RequestTarget,
		req.RequestLine.HttpVersion,
	)

	// Prepare a buffer for the handler to write its response body
	buf := &bytes.Buffer{}
	log.Println("handle: calling handler")

	// Call the handler
	if s.handler == nil {
		// No handler provided: internal server error
		he := HandlerError{StatusCode: response.StatusInternalServerError, Message: "no handler"}
		he.Write(conn)
		return
	}

	herr := s.handler(buf, req)
	log.Printf("handle: handler returned, herr=%v\n", herr)

	if herr != nil {
		// Handler reported an error; write it to the connection
		herr.Write(conn)
		log.Println("handle: wrote handler error")
		return
	}

	// Handler succeeded: build default headers, write status line, headers, and body
	hdrs := response.GetDefaultHeaders(buf.Len())
	_ = response.WriteStatusLine(conn, response.StatusOk)
	_ = response.WriteHeaders(conn, hdrs)
	if buf.Len() > 0 {
		_, _ = io.Copy(conn, buf)
	}
}

type HandlerError struct {
	StatusCode response.StatusCode
	Message    string
}

type Handler func(w io.Writer, req *request.Request) *HandlerError

func (he HandlerError) Write(w io.Writer) {
	// 1) write status line
	_ = response.WriteStatusLine(w, he.StatusCode)

	// 2) build headers using len(he.Message)
	hdrs := response.GetDefaultHeaders(len(he.Message))

	// 3) write headers
	_ = response.WriteHeaders(w, hdrs)

	// 4) write body (he.Message)
	if len(he.Message) > 0 {
		_, _ = io.WriteString(w, he.Message)
	}
}
