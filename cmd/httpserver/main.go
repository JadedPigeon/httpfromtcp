package main

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"

	"httpfromtcp/internal/headers"
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
			case "/video":
				handleVideo(w, req)
				return
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
	respHeaders := response.GetDefaultHeaders(0)
	delete(respHeaders, "content-length")
	respHeaders["transfer-encoding"] = "chunked"
	respHeaders["content-type"] = "application/json"
	respHeaders["trailer"] = "X-Content-SHA256, X-Content-Length"

	w.WriteHeaders(respHeaders)

	buf := make([]byte, 1024)
	var fullBody []byte

	for {
		n, err := resp.Body.Read(buf)
		if n > 0 {
			chunk := buf[:n]
			fullBody = append(fullBody, chunk...)

			_, writeErr := w.WriteChunkedBody(chunk)
			if writeErr != nil {
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

	sum := sha256.Sum256(fullBody)
	hashHex := hex.EncodeToString(sum[:])
	contentLen := len(fullBody)

	trailers := headers.NewHeaders()
	trailers["X-Content-SHA256"] = hashHex
	trailers["X-Content-Length"] = strconv.Itoa(contentLen)

	log.Println("writing trailers:", hashHex, contentLen)

	w.WriteChunkedBodyDone()
	w.WriteTrailers(trailers)
}

func handleVideo(w *response.Writer, req *request.Request) {
	// default video
	video, err := os.ReadFile("assets/vim.mp4")

	if err != nil {
		log.Println("Error reading video file")
		w.WriteStatusLine(response.StatusInternalServerError)
		body := []byte("Error reading video file")
		w.WriteHeaders(response.GetDefaultHeaders(len(body)))
		w.WriteBody(body)
		return
	}

	respHeaders := response.GetDefaultHeaders(len(video))
	respHeaders["content-type"] = "video/mp4"

	w.WriteStatusLine(response.StatusOk)
	w.WriteHeaders(respHeaders)
	w.WriteBody(video)
}
