package headers

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHeadersParse(t *testing.T) {
	// Test: Valid single header
	headers := NewHeaders()
	data := []byte("Host: localhost:42069\r\n\r\n")
	n, done, err := headers.Parse(data)
	require.NoError(t, err)
	require.NotNil(t, headers)
	assert.Equal(t, "localhost:42069", headers["Host"])
	assert.Equal(t, 25, n)
	assert.True(t, done)

	// Test: Invalid spacing header
	headers = NewHeaders()
	data = []byte("       Host: localhost:42069\r\n\r\n")
	n, done, err = headers.Parse(data)
	assert.Equal(t, "", headers[""])
	require.Error(t, err)
	assert.Equal(t, 30, n)
	assert.False(t, done)

	// Test: more than one field-line
	headers = NewHeaders()
	data = []byte("Host: localhost:42069\r\nUser-Agent: curl/7.81.0\r\nContent-Length: 24\r\n\r\n")
	n, done, err = headers.Parse(data)
	require.NoError(t, err)
	require.NotNil(t, headers)
	assert.Equal(t, "localhost:42069", headers["Host"])
	assert.Equal(t, "curl/7.81.0", headers["User-Agent"])
	assert.Equal(t, "24", headers["Content-Length"])
	assert.Equal(t, 70, n)
	assert.True(t, done)
}
