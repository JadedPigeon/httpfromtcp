package main

import (
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"httpfromtcp/internal/request"
	"httpfromtcp/internal/response"
	"httpfromtcp/internal/server"
)

const port = 42069

func main() {
	handler := func(w *response.Writer, req *request.Request) {
		target := req.RequestLine.RequestTarget

		var status response.StatusCode
		var html string

		if strings.HasPrefix(target, "/httpbin/") {
			handleHTTPBin(w, req)
			return
		} else {
			switch target {
			case "/yourproblem":
				status = response.StatusBadRequest
				html = `<html>
	<head>
		<title>400 Bad Request</title>
	</head>
	<body>
		<h1>Bad Request</h1>
		<p>Your request honestly kinda sucked.</p>
	</body>
	</html>`
			case "/myproblem":
				status = response.StatusInternalServerError
				html = `<html>
	<head>
		<title>500 Internal Server Error</title>
	</head>
	<body>
		<h1>Internal Server Error</h1>
		<p>Okay, you know what? This one is on me.</p>
	</body>
	</html>`
			default:
				status = response.StatusOk
				html = `<html>
	<head>
		<title>200 OK</title>
	</head>
	<body>
		<h1>Success!</h1>
		<p>Your request was an absolute banger.</p>
	</body>
	</html>`
			}
		}

		body := []byte(html)
		headers := response.GetDefaultHeaders(len(body))
		headers["content-type"] = "text/html"

		w.WriteStatusLine(status)
		w.WriteHeaders(headers)
		w.WriteBody(body)
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

// helper function
func handleHTTPBin(w *response.Writer, req *request.Request) {

	path := req.RequestLine.RequestTarget
	trimmed := strings.TrimPrefix(path, "/httpbin")
	url := "https://httpbin.org" + trimmed

	// Debugging
	log.Println("proxying to:", url)

	resp, err := http.Get(url)
	if err != nil {
		// write a 502 or 500 back to the client
		w.WriteStatusLine(response.StatusInternalServerError)
		body := []byte("Error contacting httpbin.org")
		w.WriteHeaders(response.GetDefaultHeaders(len(body)))
		w.WriteBody(body)
		return
	}
	defer resp.Body.Close()

	w.WriteStatusLine(response.StatusOk)

	// start from default, but we won't use Content-Length
	headers := response.GetDefaultHeaders(0)
	delete(headers, "content-length")
	headers["transfer-encoding"] = "chunked"
	headers["content-type"] = "application/json"

	w.WriteHeaders(headers)

	buf := make([]byte, 1024)

	for {
		n, err := resp.Body.Read(buf)
		if n > 0 {
			// send that chunk
			_, writeErr := w.WriteChunkedBody(buf[:n])
			if writeErr != nil {
				// maybe log and break/return
				return
			}
		}

		if err != nil {
			if err == io.EOF {
				break
			}
			// some other read error
			// maybe send 500 or just break
			break
		}
	}

	// send final 0-length chunk
	w.WriteChunkedBodyDone()
}
