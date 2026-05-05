package request

import (
	"errors"
	"io"
	"log"
	"regexp"
	"strings"
)

var IsUpperLetter = regexp.MustCompile(`^[A-Z]+$`).MatchString

type RequestLine struct {
	Method      string
	RequestURI  string
	HttpVersion string
}

type Request struct {
	RequestLine
}

// var MethodsSet = map[string]bool{
// 	"GET":     true,
// 	"HEAD":    true,
// 	"POST":    true,
// 	"PUT":     true,
// 	"DELETE":  true,
// 	"CONNECT": true,
// 	"OPTIONS": true,
// 	"TRACE":   true,
// }

var ERROR_MALFORMED_MSG error = errors.New("malformed message")
var ERROR_INCOMPLETE_REQ_LINE error = errors.New("incomplete request line")

var ERROR_UNSUPPORTED_HTTP error = errors.New("unsupported http version")

// var ERROR_MALFORMED_MSG error = errors.New("malformed message")

func parseRequestLine(text []byte, requestLine *RequestLine) (int, error) {
	var n int
	lineIdx := strings.Index(string(text), "\r\n")
	if lineIdx == -1 {
		return 0, ERROR_MALFORMED_MSG
	}
	reqLine := string(text[:lineIdx])
	n = len(reqLine)
	reqParts := strings.Split(reqLine, " ")

	httpParts := strings.Split(reqParts[len(reqParts)-1], "/")

	if !IsUpperLetter(reqParts[0]) || len(reqParts) != 3 || len(httpParts) != 2 {
		return n, ERROR_INCOMPLETE_REQ_LINE
	}

	if httpParts[1] != "1.1" {
		return n, ERROR_UNSUPPORTED_HTTP
	}
	requestLine.Method = reqParts[0]
	requestLine.RequestURI = reqParts[1]
	requestLine.HttpVersion = httpParts[1]
	return n, nil
}

func RequestFromReader(reader io.Reader) (*Request, error) {

	text, err := io.ReadAll(reader)
	if err != nil {
		log.Fatalf("error at reading all reader, err: %v", err)
	}
	reqLineParts := RequestLine{}
	n, err := parseRequestLine(text, &reqLineParts)
	if err != nil {
		return nil, err
	}

	return &Request{RequestLine: reqLineParts}, nil
}

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
	endIndex := cr.pos + cr.numBytesPerRead
	if endIndex > len(cr.data) {
		endIndex = len(cr.data)
	}
	n = copy(p, cr.data[cr.pos:endIndex])
	cr.pos += n

	return n, nil
}
