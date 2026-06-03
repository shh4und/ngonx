package request

import (
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type chunkReader struct {
	data            string
	numBytesPerRead int
	pos             int
}

// Read reads up to len(p) or numBytesPerRead bytes from the string per call
// its useful for simulating reading a variable number of bytes per chunk from a network connection
func (cr *chunkReader) Read(p []byte) (n int, err error) {
	if cr.pos >= len(cr.data) {
		return 0, io.EOF
	}
	endIndex := min(cr.pos+cr.numBytesPerRead, len(cr.data))
	n = copy(p, cr.data[cr.pos:endIndex])
	cr.pos += n

	return n, nil
}

func TestRequestLineParse(t *testing.T) {
	reader := &chunkReader{
		data:            "GET / HTTP/1.1\r\nHost: localhost:42069\r\nUser-Agent: curl/7.81.0\r\nAccept: */*\r\n\r\n",
		numBytesPerRead: 3,
	}
	r, err := RequestFromReader(reader)
	require.NoError(t, err)
	require.NotNil(t, r)
	assert.Equal(t, "localhost:42069", r.Headers["host"])
	assert.Equal(t, "curl/7.81.0", r.Headers["user-agent"])
	assert.Equal(t, "*/*", r.Headers["accept"])

	// Test: Malformed Header
	reader = &chunkReader{
		data:            "GET / HTTP/1.1\r\nHost localhost:42069\r\n\r\n",
		numBytesPerRead: 3,
	}
	r, err = RequestFromReader(reader)
	require.Error(t, err)
}

func TestRequestParseWithFragmentedChunks(t *testing.T) {
	// Test: Headers split across multiple small chunks (1 byte per read)
	reader := &chunkReader{
		data:            "GET / HTTP/1.1\r\nHost: example.com\r\nContent-Length: 42\r\n\r\n",
		numBytesPerRead: 1,
	}
	r, err := RequestFromReader(reader)
	require.NoError(t, err)
	require.NotNil(t, r)
	assert.Equal(t, "GET", r.Method)
	assert.Equal(t, "/", r.RequestURI)
	assert.Equal(t, "1.1", r.HttpVersion)
	assert.Equal(t, "example.com", r.Headers["host"])
	assert.Equal(t, "42", r.Headers["content-length"])
}

func TestRequestParseWithLargeChunks(t *testing.T) {
	// Test: Entire request in one large chunk
	reader := &chunkReader{
		data:            "POST /api/v1/users HTTP/1.1\r\nHost: api.example.com\r\nContent-Type: application/json\r\nContent-Length: 100\r\n\r\n",
		numBytesPerRead: 4096,
	}
	r, err := RequestFromReader(reader)
	require.NoError(t, err)
	require.NotNil(t, r)
	assert.Equal(t, "POST", r.Method)
	assert.Equal(t, "/api/v1/users", r.RequestURI)
	assert.Equal(t, "1.1", r.HttpVersion)
	assert.Equal(t, "api.example.com", r.Headers["host"])
	assert.Equal(t, "application/json", r.Headers["content-type"])
	assert.Equal(t, "100", r.Headers["content-length"])
}

func TestRequestParseMultipleHeaderValues(t *testing.T) {
	// Test: Multiple headers with same key (comma-separated according to RFC 9110)
	reader := &chunkReader{
		data:            "GET / HTTP/1.1\r\nHost: localhost\r\nAccept: text/html\r\nAccept: application/json\r\nAccept: text/plain\r\n\r\n",
		numBytesPerRead: 20,
	}
	r, err := RequestFromReader(reader)
	require.NoError(t, err)
	require.NotNil(t, r)
	assert.Equal(t, "text/html, application/json, text/plain", r.Headers["accept"])
}

func TestRequestParseMalformedRequestLine(t *testing.T) {
	// Test: Invalid HTTP version
	reader := &chunkReader{
		data:            "GET / HTTP/2.0\r\nHost: localhost\r\n\r\n",
		numBytesPerRead: 5,
	}
	_, err := RequestFromReader(reader)
	require.Error(t, err)
	assert.Equal(t, ErrUnsuportedHTTPVersion, err)
}

func TestRequestParseInvalidMethod(t *testing.T) {
	// Test: Method with lowercase letters (should be all uppercase)
	reader := &chunkReader{
		data:            "get / HTTP/1.1\r\nHost: localhost\r\n\r\n",
		numBytesPerRead: 10,
	}
	_, err := RequestFromReader(reader)
	require.Error(t, err)
	assert.Equal(t, ErrIncompleteRequestLine, err)
}

func TestRequestParseEmptyRequestLine(t *testing.T) {
	// Test: Empty request line
	reader := &chunkReader{
		data:            "\r\nHost: localhost\r\n\r\n",
		numBytesPerRead: 5,
	}
	_, err := RequestFromReader(reader)
	require.Error(t, err)
}

func TestRequestParseHeaderWithInvalidCharacters(t *testing.T) {
	// Test: Header field name with invalid characters
	reader := &chunkReader{
		data:            "GET / HTTP/1.1\r\nHost©: localhost\r\n\r\n",
		numBytesPerRead: 5,
	}
	_, err := RequestFromReader(reader)
	require.Error(t, err)
}

func TestRequestParseHeaderWithSpaceInFieldName(t *testing.T) {
	// Test: Header field name with space (invalid)
	reader := &chunkReader{
		data:            "GET / HTTP/1.1\r\nHost Name: localhost\r\n\r\n",
		numBytesPerRead: 5,
	}
	_, err := RequestFromReader(reader)
	require.Error(t, err)
}

func TestRequestParseHeaderCaseInsensitivity(t *testing.T) {
	// Test: Headers with mixed case are stored as lowercase
	reader := &chunkReader{
		data:            "GET / HTTP/1.1\r\nHoSt: localhost\r\nCoNtEnT-LeNgTh: 50\r\nUser-AGENT: curl\r\n\r\n",
		numBytesPerRead: 15,
	}
	r, err := RequestFromReader(reader)
	require.NoError(t, err)
	require.NotNil(t, r)
	assert.Equal(t, "localhost", r.Headers["host"])
	assert.Equal(t, "50", r.Headers["content-length"])
	assert.Equal(t, "curl", r.Headers["user-agent"])
}

func TestRequestParseHeaderWithLeadingTrailingSpaces(t *testing.T) {
	// Test: Header values with leading/trailing spaces are trimmed
	reader := &chunkReader{
		data:            "GET / HTTP/1.1\r\nHost:   localhost:8080   \r\nUser-Agent:   curl/7.81.0   \r\n\r\n",
		numBytesPerRead: 10,
	}
	r, err := RequestFromReader(reader)
	require.NoError(t, err)
	require.NotNil(t, r)
	assert.Equal(t, "localhost:8080", r.Headers["host"])
	assert.Equal(t, "curl/7.81.0", r.Headers["user-agent"])
}

func TestRequestParseComplexRequestLine(t *testing.T) {
	// Test: Request line with complex URI containing query parameters
	reader := &chunkReader{
		data:            "GET /search?q=golang&sort=date HTTP/1.1\r\nHost: search.example.com\r\n\r\n",
		numBytesPerRead: 8,
	}
	r, err := RequestFromReader(reader)
	require.NoError(t, err)
	require.NotNil(t, r)
	assert.Equal(t, "GET", r.Method)
	assert.Equal(t, "/search?q=golang&sort=date", r.RequestURI)
	assert.Equal(t, "1.1", r.HttpVersion)
}

func TestRequestParseMinimalRequest(t *testing.T) {
	// Test: Minimal valid request (only Host header required for HTTP/1.1)
	reader := &chunkReader{
		data:            "HEAD /api HTTP/1.1\r\nHost: minimal.com\r\n\r\n",
		numBytesPerRead: 5,
	}
	r, err := RequestFromReader(reader)
	require.NoError(t, err)
	require.NotNil(t, r)
	assert.Equal(t, "HEAD", r.Method)
	assert.Equal(t, "/api", r.RequestURI)
	assert.Equal(t, 1, len(r.Headers)) // Only Host header
	assert.Equal(t, "minimal.com", r.Headers["host"])
}

func TestRequestBodyParse(t *testing.T) {
	// Test: Standard Body
	reader := &chunkReader{
		data: "POST /submit HTTP/1.1\r\n" +
			"Host: localhost:42069\r\n" +
			"Content-Length: 13\r\n" +
			"\r\n" +
			"hello world!\n",
		numBytesPerRead: 3,
	}
	r, err := RequestFromReader(reader)
	require.NoError(t, err)
	require.NotNil(t, r)
	assert.Equal(t, "hello world!\n", string(r.Body))

	// Test: Body shorter than reported content length
	reader = &chunkReader{
		data: "POST /submit HTTP/1.1\r\n" +
			"Host: localhost:42069\r\n" +
			"Content-Length: 20\r\n" +
			"\r\n" +
			"partial content",
		numBytesPerRead: 3,
	}
	r, err = RequestFromReader(reader)
	require.Error(t, err)
}
