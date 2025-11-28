package server

import (
	"fmt"
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
	w := response.NewWriter(conn)

	// Add a read deadline so a client that stops sending can't hang the server
	_ = conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	// Parse the request from the connection
	req, err := request.RequestFromReader(conn)
	log.Printf("handle: RequestFromReader returned, err=%v\n", err)
	if err != nil {
		w.WriteStatusLine(response.StatusBadRequest)
		body := []byte(err.Error())
		w.WriteHeaders(response.GetDefaultHeaders(len(body)))
		w.WriteBody(body)
		return
	}
	// Clear the read deadline now that we've successfully read the request
	_ = conn.SetReadDeadline(time.Time{})
	log.Printf("handle: parsed request line: method=%s target=%s version=%s\n",
		req.RequestLine.Method,
		req.RequestLine.RequestTarget,
		req.RequestLine.HttpVersion,
	)

	log.Println("handle: calling handler")

	// Call the handler
	if s.handler == nil {
		w.WriteStatusLine(response.StatusInternalServerError)
		body := []byte("no handler")
		w.WriteHeaders(response.GetDefaultHeaders(len(body)))
		w.WriteBody(body)
		return
	}

	s.handler(w, req)
}

type Handler func(w *response.Writer, req *request.Request)
