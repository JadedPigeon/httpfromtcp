package request

import (
	"bytes"
	"fmt"
	"io"
	"strings"
)

type Request struct {
	RequestLine RequestLine
	// State tracks the parser state for this request.
	state ParserState
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
	// ParserDone indicates the request has been fully parsed.
	ParserDone
)

func RequestFromReader(reader io.Reader) (*Request, error) {
	// Initialize a request and a fixed buffer for streaming reads.
	req := &Request{state: ParserInitialized}

	buf := make([]byte, 8)
	readTo := 0 // number of valid bytes in buf

	var totalRead int
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
			totalRead += n
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
					_ = totalRead

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

	// If already done, this is an error: caller shouldn't feed more data.
	if r.state == ParserDone {
		return 0, fmt.Errorf("trying to read data in a done state")
	}

	// Convert incoming bytes to string for existing helpers.
	// Use the byte-based parser which validates and returns a RequestLine.
	rl, consumed, err := parseRequestLine(data)
	if err != nil {
		return consumed, err
	}
	if consumed == 0 {
		// Need more data
		return 0, nil
	}

	// Populate the Request and update state
	r.RequestLine = *rl
	r.state = ParserDone

	return consumed, nil
}

// Just returning first line for now
// parseRequestLine returns the request line (without CRLF), the number of
// bytes consumed from the input (including the terminating "\r\n"), and an
// error. If the input does not yet contain a CRLF, it returns consumed=0 and
// nil error to indicate the caller needs to provide more data.
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
