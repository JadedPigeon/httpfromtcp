package request

import (
	"bytes"
	"fmt"
	"io"
	"strconv"
	"strings"

	"httpfromtcp/internal/headers"
)

type Request struct {
	RequestLine RequestLine
	// State tracks the parser state for this request.
	state   ParserState
	Headers headers.Headers
	Body    []byte
}

type RequestLine struct {
	HttpVersion   string
	RequestTarget string
	Method        string
}

// ParserState is an internal enum representing the parser's progress for a
// Request. It's intentionally an int so it is small and zero-value friendly.
type ParserState int

const (
	// ParserInitialized indicates parsing has started but not completed.
	ParserInitialized ParserState = iota
	// requestStateParsingHeaders indicates the parser is currently parsing headers.
	requestStateParsingHeaders
	// requestStateParsingBody indicates the parser is currently parsing the body.
	requestStateParsingBody
	// ParserDone indicates the request has been fully parsed.
	ParserDone
)

func RequestFromReader(reader io.Reader) (*Request, error) {
	// Initialize a request and a fixed buffer for streaming reads.
	req := &Request{state: ParserInitialized}

	buf := make([]byte, 8)
	readTo := 0 // number of valid bytes in buf
	for {
		// If parser already completed (could happen if previous chunk finished), break.
		if req.state == ParserDone {
			break
		}

		// Attempt to parse with current buffer first.
		consumed, err := req.parse(buf[:readTo])
		if err != nil {
			return nil, err
		}
		if consumed > 0 {
			// Shift consumed bytes out by copying remaining bytes to the front
			// and update readTo.
			if consumed < readTo {
				copy(buf[0:], buf[consumed:readTo])
				readTo = readTo - consumed
			} else {
				readTo = 0
			}
			// continue to try parsing again (in case multiple lines present)
			continue
		}

		// Need more data: ensure buffer has space and read directly into it.
		if readTo == len(buf) {
			// grow buffer
			newCap := len(buf) * 2
			if newCap == 0 {
				newCap = 8
			}
			newBuf := make([]byte, newCap)
			copy(newBuf, buf[:readTo])
			buf = newBuf
		}

		n, err := reader.Read(buf[readTo:])
		if n > 0 {
			readTo += n
		}
		if err != nil {
			if err == io.EOF {
				// final attempt: parse whatever is in buf
				consumed, perr := req.parse(buf[:readTo])
				if perr != nil {
					return nil, perr
				}
				if consumed > 0 {
					if consumed < readTo {
						copy(buf[0:], buf[consumed:readTo])
						readTo = readTo - consumed
					} else {
						readTo = 0
					}
				}
				// If parser is done, return, otherwise EOF and incomplete
				if req.state == ParserDone {
					// Silence potential staticcheck unused-value warnings for bookkeeping vars
					_ = readTo
					return req, nil
				}
				return nil, fmt.Errorf("incomplete request after EOF: need more data")
			}
			return nil, err
		}
	}

	return req, nil
}

// parse consumes bytes from data to incrementally parse the request. It
// returns the number of bytes consumed and an error. If it needs more data
// to complete parsing it returns consumed=0 and err=nil.
func (r *Request) parse(data []byte) (int, error) {
	if r == nil {
		return 0, fmt.Errorf("nil Request")
	}

	switch r.state {
	case ParserInitialized:
		// Use the byte-based parser which validates and returns a RequestLine.
		rl, consumed, err := parseRequestLine(data)
		if err != nil {
			return consumed, err
		}
		if consumed == 0 {
			// Need more data
			return 0, nil
		}

		// Populate the Request, initialize headers, and move to header parsing state
		r.RequestLine = *rl
		r.Headers = headers.NewHeaders()
		r.state = requestStateParsingHeaders

		return consumed, nil

	case requestStateParsingHeaders:
		if r.Headers == nil {
			r.Headers = headers.NewHeaders()
		}
		n, done, err := r.Headers.Parse(data)
		if err != nil {
			return 0, err
		}
		if n == 0 && !done {
			// need more data
			return 0, nil
		}
		if done {
			r.state = requestStateParsingBody
			return n, nil
		}
		// consumed a header line, remain in headers state
		return n, nil

	case requestStateParsingBody:

		// Check for Content-Length header
		headerVal := r.Headers.Get("Content-Length")
		if headerVal == "" {
			// No body expected: mark done and consume any provided bytes so the
			// caller can make forward progress rather than looping on 0-consume.
			r.state = ParserDone
			if len(data) == 0 {
				return 0, nil
			}
			return len(data), nil
		}

		// Convert Content-Length to integer using strconv for clearer errors.
		contentLength, err := strconv.Atoi(headerVal)
		if err != nil {
			return 0, fmt.Errorf("invalid Content-Length: %q", headerVal)
		}

		// Special case: if content length is 0, transition to done and consume
		// any provided data so the caller can discard extra bytes and make
		// forward progress.
		if contentLength == 0 {
			r.state = ParserDone
			if len(data) == 0 {
				return 0, nil
			}
			return len(data), nil
		}

		// Initialize body if needed
		if r.Body == nil {
			r.Body = make([]byte, 0, contentLength)
		}

		// Append available data to body
		bytesToCopy := len(data)
		if bytesToCopy > 0 {
			r.Body = append(r.Body, data...)
		}

		// Check if body exceeds content length
		if len(r.Body) > contentLength {
			return 0, fmt.Errorf("body length %d exceeds Content-Length %d", len(r.Body), contentLength)
		}

		// If we've reached the expected length, transition to done
		if len(r.Body) == contentLength {
			r.state = ParserDone
		}

		return bytesToCopy, nil

	case ParserDone:
		// Already done with this request, consume no bytes
		return 0, nil

	default:
		return 0, fmt.Errorf("unknown parser state: %d", r.state)
	}
}

// parseRequestLine examines the provided bytes for a CRLF-terminated
// request-line. If a full line is found it parses and validates the
// request-line and returns a populated RequestLine and the number of bytes
// consumed (including the CRLF). If no CRLF is found it returns (nil,0,nil)
// to indicate more data is required.
func parseRequestLine(input []byte) (*RequestLine, int, error) {
	if len(input) == 0 {
		return nil, 0, nil
	}

	idx := bytes.Index(input, []byte("\r\n"))
	if idx == -1 {
		return nil, 0, nil
	}

	line := string(input[:idx])
	consumed := idx + 2

	rl, err := requestLineFromString(line)
	if err != nil {
		return nil, consumed, err
	}
	return rl, consumed, nil
}

// requestLineFromString validates a request-line string and returns a
// populated RequestLine. Validation moved here to keep parse simple.
func requestLineFromString(line string) (*RequestLine, error) {
	if line == "" {
		return nil, fmt.Errorf("empty request")
	}
	parts := strings.SplitN(line, " ", 3)
	if len(parts) != 3 {
		return nil, fmt.Errorf("malformed request line: %q", line)
	}

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

	httpVersionToken := parts[2]
	httpVersion := httpVersionToken
	if strings.HasPrefix(httpVersionToken, "HTTP/") {
		httpVersion = strings.TrimPrefix(httpVersionToken, "HTTP/")
	}
	if httpVersion != "1.1" {
		return nil, fmt.Errorf("unsupported HTTP version in request line: %q", httpVersionToken)
	}

	return &RequestLine{
		Method:        method,
		RequestTarget: requestTarget,
		HttpVersion:   httpVersion,
	}, nil
}
