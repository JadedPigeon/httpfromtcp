package request

import (
	"fmt"
	"io"
	"strings"
)

type Request struct {
	RequestLine RequestLine
}

type RequestLine struct {
	HttpVersion   string
	RequestTarget string
	Method        string
}

func RequestFromReader(reader io.Reader) (*Request, error) {
	read, err := io.ReadAll(reader)
	if err != nil {
		return nil, err
	}

	lines := string(read)
	requestLineStr, err := parseRequestLine(lines)
	if err != nil {
		return nil, err
	}

	parts := strings.SplitN(requestLineStr, " ", 3)
	if len(parts) != 3 {
		return nil, fmt.Errorf("malformed request line: %q", requestLineStr)
	}

	// Verify that the "method" part only contains capital alphabetic characters.
	method := parts[0]
	for _, ch := range method {
		if ch < 'A' || ch > 'Z' {
			return nil, fmt.Errorf("invalid method in request line: %q", method)
		}
	}

	requestTarget := parts[1]
	if !strings.HasPrefix(requestTarget, "/") {
		return nil, fmt.Errorf("invalid request target in request line: %q", requestTarget)
	}

	// Verify that the http version part is 1.1.
	httpVersionToken := parts[2]
	httpVersion := httpVersionToken
	if strings.HasPrefix(httpVersionToken, "HTTP/") {
		httpVersion = strings.TrimPrefix(httpVersionToken, "HTTP/")
	}
	if httpVersion != "1.1" {
		return nil, fmt.Errorf("unsupported HTTP version in request line: %q", httpVersionToken)
	}

	req := &Request{
		RequestLine: RequestLine{
			Method:        method,
			RequestTarget: requestTarget,
			HttpVersion:   httpVersion,
		},
	}

	return req, nil
}

// Just returning first line for now
func parseRequestLine(line string) (string, error) {
	parts := strings.Split(line, "\r\n")
	if len(parts) == 0 || parts[0] == "" {
		return "", fmt.Errorf("empty request")
	}
	return parts[0], nil
}
