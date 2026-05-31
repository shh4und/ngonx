package headers

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHeadersParse(t *testing.T) {
	// Test: Valid single header with lowercase key
	headers := NewHeaders()
	data := []byte("HOST: localhost:42069\r\n\r\n")
	n, done, err := headers.Parse(data)
	require.NoError(t, err)
	require.NotNil(t, headers)
	assert.Equal(t, "localhost:42069", headers["host"])
	assert.Equal(t, 25, n)
	assert.True(t, done)

	// Test: Header with uppercase letters (should be stored as lowercase)
	headers = NewHeaders()
	data = []byte("Host: localhost:42069\r\nUser-Agent: curl/7.81.0\r\nContent-Length: 24\r\n\r\n")
	n, done, err = headers.Parse(data)
	require.NoError(t, err)
	require.NotNil(t, headers)
	assert.Equal(t, "localhost:42069", headers["host"])
	assert.Equal(t, "curl/7.81.0", headers["user-agent"])
	assert.Equal(t, "24", headers["content-length"])
	assert.Equal(t, 70, n)
	assert.True(t, done)

	// Test: Header with mixed case letters (should be stored as lowercase)
	headers = NewHeaders()
	data = []byte("CoNtEnT-LenGTH: 100\r\n\r\n")
	n, done, err = headers.Parse(data)
	require.NoError(t, err)
	require.NotNil(t, headers)
	assert.Equal(t, "100", headers["content-length"])
	assert.Equal(t, 23, n)
	assert.True(t, done)

	// Test: Invalid character in header key (©)
	headers = NewHeaders()
	data = []byte("H©st: localhost:42069\r\n\r\n")
	n, done, err = headers.Parse(data)
	require.Error(t, err)
	require.Equal(t, ErrHeaderLineNotValid, err)
	assert.False(t, done)

	// Test: Invalid spacing at start of header
	headers = NewHeaders()
	data = []byte("       Host: localhost:42069\r\n\r\n")
	n, done, err = headers.Parse(data)
	require.Error(t, err)
	assert.False(t, done)

	// Test: more than one field-line with case insensitivity
	headers = NewHeaders()
	data = []byte("Host: localhost:42069\r\nUser-Agent: curl/7.81.0\r\nContent-Length: 24\r\n\r\n")
	n, done, err = headers.Parse(data)
	require.NoError(t, err)
	require.NotNil(t, headers)
	// All keys should be lowercase
	assert.Equal(t, "localhost:42069", headers["host"])
	assert.Equal(t, "curl/7.81.0", headers["user-agent"])
	assert.Equal(t, "24", headers["content-length"])
	assert.Equal(t, 70, n)
	assert.True(t, done)

	headers = NewHeaders()
	data = []byte("Host: localhost:42069\r\nSet-Person: lane-loves-go\r\nSet-Person: prime-loves-zig\r\nSet-Person: tj-loves-ocaml\r\nUser-Agent: curl/7.81.0\r\nContent-Length: 24\r\n\r\n")
	n, done, err = headers.Parse(data)
	require.NoError(t, err)
	require.NotNil(t, headers)
	// All keys should be lowercase
	assert.Equal(t, "localhost:42069", headers["host"])
	assert.Equal(t, "curl/7.81.0", headers["user-agent"])
	assert.Equal(t, "24", headers["content-length"])
	assert.Equal(t, "lane-loves-go, prime-loves-zig, tj-loves-ocaml", headers["set-person"])
	assert.Equal(t, 154, n)
	assert.True(t, done)
}
