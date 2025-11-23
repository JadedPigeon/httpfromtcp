package main

import (
	"io"
	"log"
	"os"
	"os/signal"
	"syscall"

	"httpfromtcp/internal/request"
	"httpfromtcp/internal/response"
	"httpfromtcp/internal/server"
)

const port = 42069

func main() {
	handler := func(w io.Writer, req *request.Request) *server.HandlerError {
		target := req.RequestLine.RequestTarget
		switch target {
		case "/yourproblem":
			return &server.HandlerError{StatusCode: response.StatusBadRequest, Message: "Your problem is not my problem\n"}
		case "/myproblem":
			return &server.HandlerError{StatusCode: response.StatusInternalServerError, Message: "Woopsie, my bad\n"}
		default:
			_, _ = io.WriteString(w, "All good, frfr\n")
			return nil
		}
	}

	srv, err := server.Serve(port, handler)
	if err != nil {
		log.Fatalf("Error starting server: %v", err)
	}
	defer srv.Close()
	log.Println("Server started on port", port)

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan
	log.Println("Server gracefully stopped")
}
