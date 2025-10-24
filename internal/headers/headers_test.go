package headers

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHeaders(t *testing.T) {
	// Test: Valid single header
	headers := NewHeaders()
	data := []byte("Host: localhost:42069\r\n\r\n")
	n, done, err := headers.Parse(data)
	require.NoError(t, err)
	require.NotNil(t, headers)
	assert.Equal(t, "localhost:42069", headers["Host"])
	assert.Equal(t, 23, n)
	assert.False(t, done)

	// Test: Invalid spacing header
	headers = NewHeaders()
	data = []byte("       Host : localhost:42069       \r\n\r\n")
	n, done, err = headers.Parse(data)
	require.Error(t, err)
	assert.Equal(t, 0, n)
	assert.False(t, done)

	// Test: Valid single header with extra whitespace
	headers = NewHeaders()
	data = []byte("Host:    localhost:42069    \r\n\r\n")
	n, done, err = headers.Parse(data)
	require.NoError(t, err)
	assert.Equal(t, "localhost:42069", headers["Host"])
	// consumed should be the header line length + CRLF (idx + 2)
	assert.Equal(t, len("Host:    localhost:42069    \r\n"), n)
	assert.False(t, done)

	// Test: Valid 2 headers with existing headers
	headers = NewHeaders()
	headers["Existing"] = "present"
	data = []byte("Host: localhost:42069\r\n")
	n, done, err = headers.Parse(data)
	require.NoError(t, err)
	assert.Equal(t, "localhost:42069", headers["Host"])
	assert.Equal(t, "present", headers["Existing"])
	assert.Equal(t, len(data), n)
	assert.False(t, done)

	// parse a second header and then final CRLF
	data2 := []byte("User-Agent: curl/7.81.0\r\n\r\n")
	n2, done2, err := headers.Parse(data2)
	require.NoError(t, err)
	assert.Equal(t, "curl/7.81.0", headers["User-Agent"])
	assert.False(t, done2)
	assert.Equal(t, len("User-Agent: curl/7.81.0\r\n"), n2)

	// Test: Valid done (CRLF at start indicates end of headers)
	headers = NewHeaders()
	data = []byte("\r\n")
	n, done, err = headers.Parse(data)
	require.NoError(t, err)
	assert.True(t, done)
	assert.Equal(t, 2, n)
}
