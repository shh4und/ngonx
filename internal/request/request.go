package request

import (
	"errors"
	"io"
	"regexp"
	"strings"
)

var IsUpperLetter = regexp.MustCompile(`^[A-Z]+$`).MatchString
var CRLFStr string = "\r\n"
var CRLFByte []byte = []byte("\r\n")
var ErrMalformedMsg error = errors.New("malformed message")
var ErrIncompleteRequestLine error = errors.New("incomplete request line")
var ErrUnsuportedHTTPVersion error = errors.New("unsupported http version")

// var ErrMalformedMsg error = errors.New("malformed message")

//	var MethodsSet = map[string]bool{
//		"GET":     true,
//		"HEAD":    true,
//		"POST":    true,
//		"PUT":     true,
//		"DELETE":  true,
//		"CONNECT": true,
//		"OPTIONS": true,
//		"TRACE":   true,
//	}

const BufferSize int = 8

type ParserState int

const (
	Initialized ParserState = iota
	Done
)

type RequestLine struct {
	Method      string
	RequestURI  string
	HttpVersion string
}

type Request struct {
	RequestLine
	ParserState
}

func NewRequest() *Request {
	return &Request{ParserState: Initialized}
}

func (r *Request) parse(data []byte) (int, error)
func (r *Request) done() bool {
	return r.ParserState == Done
}

func parseRequestLine(text []byte, requestLine *RequestLine) (int, error) {
	lineIdx := strings.Index(string(text), CRLFStr)
	if lineIdx == -1 {
		return 0, nil
	}
	reqLine := string(text[:lineIdx])
	// bytes consumidos
	n := lineIdx + len(CRLFStr)

	reqParts := strings.Split(reqLine, " ")
	httpParts := strings.Split(reqParts[len(reqParts)-1], "/")

	if !IsUpperLetter(reqParts[0]) || len(reqParts) != 3 || len(httpParts) != 2 {
		return n, ErrIncompleteRequestLine
	}

	if httpParts[1] != "1.1" {
		return n, ErrUnsuportedHTTPVersion
	}
	requestLine.Method = reqParts[0]
	requestLine.RequestURI = reqParts[1]
	requestLine.HttpVersion = httpParts[1]
	return n, nil
}

func RequestFromReader(reader io.Reader) (*Request, error) {
	request := NewRequest()
	buf := make([]byte, BufferSize, BufferSize)
	readToIndex := 0

	// TODO:
	for !request.done() {
		n, err := reader.Read(buf[readToIndex:])
		if err != nil {
			return nil, err
		}

		readToIndex += n
		if n == 0 {
			return nil, errors.New("TODO:")
		}
	}

	return request, nil
}
