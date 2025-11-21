package server

import (
	"fmt"
	"net"
	"sync"
	"sync/atomic"

	"httpfromtcp/internal/response"
)

// Contains the state of the server
type Server struct {
	listener net.Listener
	closed   atomic.Bool
	wg       sync.WaitGroup
}

// Creates a net.Listener and returns a new Server instance. Starts listening for requests inside a goroutine.
func Serve(port int) (*Server, error) {
	ln, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		return nil, err
	}

	s := &Server{listener: ln}
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

// Update this function to return our "default" response
// HTTP/1.1 200 OK
// Content-Length: 0
// Connection: close
// Content-Type: text/plain
func (s *Server) handle(conn net.Conn) {
	defer s.wg.Done()
	defer conn.Close()

	// Simple fixed response body
	body := ""

	// Write status line
	_ = response.WriteStatusLine(conn, response.StatusOk)

	// Default headers and write them
	hdrs := response.GetDefaultHeaders(len(body))
	_ = response.WriteHeaders(conn, hdrs)

	// Write body
	_, _ = conn.Write([]byte(body))
}
