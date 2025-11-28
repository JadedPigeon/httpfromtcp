package response

import (
	"fmt"
	"httpfromtcp/internal/headers"
	"io"
	"strconv"
)

type StatusCode int

const (
	StatusOk                  StatusCode = 200
	StatusBadRequest          StatusCode = 400
	StatusInternalServerError StatusCode = 500
	StatusRequestTimeout      StatusCode = 408
)

type Writer struct {
	dest io.Writer
}

func (w *Writer) WriteStatusLine(statusCode StatusCode) error {
	var reason string
	switch statusCode {
	case StatusOk:
		reason = "OK"
	case StatusBadRequest:
		reason = "Bad Request"
	case StatusInternalServerError:
		reason = "Internal Server Error"
	case StatusRequestTimeout:
		reason = "Request Timeout"
	default:
		reason = ""
	}

	var n int
	var err error
	if reason == "" {
		n, err = fmt.Fprintf(w.dest, "HTTP/1.1 %d \r\n", statusCode)
	} else {
		n, err = fmt.Fprintf(w.dest, "HTTP/1.1 %d %s\r\n", statusCode, reason)
	}
	if err != nil {
		return err
	}
	if n <= 0 {
		return fmt.Errorf("no bytes written for status line")
	}
	return nil
}

func GetDefaultHeaders(contentLen int) headers.Headers {
	h := headers.NewHeaders()
	h["content-length"] = strconv.Itoa(contentLen)
	h["connection"] = "close"
	h["content-type"] = "text/plain"
	return h
}

func (w *Writer) WriteHeaders(h headers.Headers) error {
	for key, value := range h {
		n, err := fmt.Fprintf(w.dest, "%s: %s\r\n", key, value)
		if err != nil {
			return err
		}
		if n <= 0 {
			return fmt.Errorf("no bytes written for header %q", key)
		}
	}
	// Write final CRLF to end headers section
	n, err := fmt.Fprintf(w.dest, "\r\n")
	if err != nil {
		return err
	}
	if n <= 0 {
		return fmt.Errorf("no bytes written for final CRLF after headers")
	}
	return nil
}

func (w *Writer) WriteBody(p []byte) (int, error) {
	return w.dest.Write(p)
}

func NewWriter(dest io.Writer) *Writer {
	return &Writer{dest: dest}
}
