package server

import (
	"bufio"
	"fmt"
	"httpfromtcp/internal/request"
	"httpfromtcp/internal/response"
	"io"
	"net"
	"strconv"
	"strings"
	"testing"
	"time"
)

func doRequest(t *testing.T, addr string, path string) (int, string, error) {
	conn, err := net.DialTimeout("tcp", addr, 2*time.Second)
	if err != nil {
		return 0, "", err
	}
	defer conn.Close()

	// send request (use Write and CloseWrite to ensure server receives bytes promptly)
	req := fmt.Sprintf("GET %s HTTP/1.1\r\nHost: localhost\r\n\r\n", path)
	_, err = conn.Write([]byte(req))
	if err != nil {
		return 0, "", err
	}
	if tcpConn, ok := conn.(*net.TCPConn); ok {
		_ = tcpConn.CloseWrite()
	}

	// read status line
	r := bufio.NewReader(conn)
	statusLine, err := r.ReadString('\n')
	if err != nil {
		return 0, "", err
	}
	statusLine = strings.TrimRight(statusLine, "\r\n")
	// parse status code from "HTTP/1.1 XXX Reason"
	parts := strings.SplitN(statusLine, " ", 3)
	if len(parts) < 2 {
		return 0, "", fmt.Errorf("malformed status line: %q", statusLine)
	}
	code, err := strconv.Atoi(parts[1])
	if err != nil {
		return 0, "", err
	}

	// read headers
	var contentLen int
	for {
		line, err := r.ReadString('\n')
		if err != nil {
			return code, "", err
		}
		if line == "\r\n" {
			break
		}
		line = strings.TrimRight(line, "\r\n")
		if strings.HasPrefix(strings.ToLower(line), "content-length:") {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				n, _ := strconv.Atoi(strings.TrimSpace(parts[1]))
				contentLen = n
			}
		}
	}

	var body string
	if contentLen > 0 {
		buf := make([]byte, contentLen)
		_, err := io.ReadFull(r, buf)
		if err != nil {
			return code, "", err
		}
		body = string(buf)
	}
	return code, body, nil
}

func TestServerIntegration(t *testing.T) {
	handler := func(w *response.Writer, req *request.Request) {
		target := req.RequestLine.RequestTarget
		switch target {
		case "/yourproblem":
			body := []byte("Your problem is not my problem\n")
			_ = w.WriteStatusLine(response.StatusBadRequest)
			_ = w.WriteHeaders(response.GetDefaultHeaders(len(body)))
			_, _ = w.WriteBody(body)
			return
		case "/myproblem":
			body := []byte("Woopsie, my bad\n")
			_ = w.WriteStatusLine(response.StatusInternalServerError)
			_ = w.WriteHeaders(response.GetDefaultHeaders(len(body)))
			_, _ = w.WriteBody(body)
			return
		default:
			body := []byte("All good, frfr\n")
			_ = w.WriteStatusLine(response.StatusOk)
			_ = w.WriteHeaders(response.GetDefaultHeaders(len(body)))
			_, _ = w.WriteBody(body)
			return
		}
	}

	s, err := Serve(0, handler)
	if err != nil {
		t.Fatalf("Serve failed: %v", err)
	}
	defer s.Close()

	// determine address (use loopback IPv4 to match client dial)
	tcpAddr := s.listener.Addr().(*net.TCPAddr)
	addr := fmt.Sprintf("127.0.0.1:%d", tcpAddr.Port)

	// Give the server a moment to start
	time.Sleep(20 * time.Millisecond)

	tests := []struct {
		path     string
		wantCode int
		wantBody string
	}{
		{"/yourproblem", 400, "Your problem is not my problem\n"},
		{"/myproblem", 500, "Woopsie, my bad\n"},
		{"/", 200, "All good, frfr\n"},
	}

	for _, tt := range tests {
		code, body, err := doRequest(t, addr, tt.path)
		if err != nil {
			t.Fatalf("request %s failed: %v", tt.path, err)
		}
		if code != tt.wantCode {
			t.Fatalf("%s: got code %d want %d", tt.path, code, tt.wantCode)
		}
		if body != tt.wantBody {
			t.Fatalf("%s: got body %q want %q", tt.path, body, tt.wantBody)
		}
	}
}
