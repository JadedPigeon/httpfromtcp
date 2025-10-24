package headers

import (
	"bytes"
	"fmt"
	"strings"
)

type Headers map[string]string

func (h Headers) Parse(data []byte) (n int, done bool, err error) {
	// Look for CRLF which terminates a header line
	if len(data) == 0 {
		return 0, false, nil
	}

	idx := bytes.Index(data, []byte("\r\n"))
	if idx == -1 {
		// need more data
		return 0, false, nil
	}

	// If CRLF is at start, headers section ended
	if idx == 0 {
		return 2, true, nil
	}

	// Extract the header line (without CRLF)
	line := string(data[:idx])

	// Find the first colon
	ci := strings.IndexByte(line, ':')
	if ci == -1 {
		return 0, false, fmt.Errorf("malformed header (no colon): %q", line)
	}

	keyRaw := line[:ci]
	valRaw := line[ci+1:]

	// Key must not have surrounding whitespace; no spaces between key and colon
	if strings.TrimSpace(keyRaw) != keyRaw {
		return 0, false, fmt.Errorf("invalid header key spacing: %q", keyRaw)
	}
	if strings.ContainsRune(keyRaw, ' ') {
		return 0, false, fmt.Errorf("invalid header key contains space: %q", keyRaw)
	}

	// Value: trim surrounding whitespace
	value := strings.TrimSpace(valRaw)

	// Add to headers map
	h[keyRaw] = value

	// consumed is header line plus CRLF
	consumed := idx + 2
	return consumed, false, nil
}

func NewHeaders() Headers {
	return Headers(map[string]string{})
}
