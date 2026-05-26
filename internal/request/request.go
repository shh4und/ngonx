package request

import (
	"bytes"
	"errors"
	"io"
	"regexp"
)

var IsUpperLetter = regexp.MustCompile(`^[A-Z]+$`).MatchString
var CRLFStr string = "\r\n"
var CRLFByte []byte = []byte("\r\n")
var ErrMalformedMsg error = errors.New("malformed message")
var ErrIncompleteRequestLine error = errors.New("incomplete request line")
var ErrUnsuportedHTTPVersion error = errors.New("unsupported http version")
var ErrRequestInErrorState error = errors.New("parse entered in error state")

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

const BufferSize int = 1024

type ParserState int

const (
	StateInitialized ParserState = iota
	StateDone
	StateError
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
	return &Request{ParserState: StateInitialized}
}

func (r *Request) parse(data []byte) (int, error) {
	read := 0
outer:
	for {
		switch r.ParserState {
		case StateError:
			return 0, ErrRequestInErrorState

		case StateInitialized:
			n, rl, err := parseRequestLine(data[read:])
			if err != nil {
				r.ParserState = StateError
				return 0, err
			}

			if n == 0 {
				break outer
			}

			r.RequestLine = *rl
			read += n
			r.ParserState = StateDone

		case StateDone:
			break outer
		}
	}
	return read, nil
}

func (r *Request) done() bool {
	return r.ParserState == StateDone || r.ParserState == StateError
}

func parseRequestLine(text []byte) (int, *RequestLine, error) {
	lineIdx := bytes.Index(text, CRLFByte)
	if lineIdx == -1 {
		return 0, nil, nil
	}
	reqLine := text[:lineIdx]
	// bytes consumidos
	n := lineIdx + len(CRLFStr)

	reqParts := bytes.Split(reqLine, []byte(" "))
	httpParts := bytes.Split(reqParts[len(reqParts)-1], []byte("/"))

	if !IsUpperLetter(string(reqParts[0])) || len(reqParts) != 3 || len(httpParts) != 2 {
		return n, nil, ErrIncompleteRequestLine
	}

	if string(httpParts[1]) != "1.1" {
		return n, nil, ErrUnsuportedHTTPVersion
	}

	return n, &RequestLine{
		Method:      string(reqParts[0]),
		RequestURI:  string(reqParts[1]),
		HttpVersion: string(httpParts[1]),
	}, nil
}

func RequestFromReader(reader io.Reader) (*Request, error) {
	request := NewRequest()
	buf := make([]byte, BufferSize)
	bufLen := 0

	// TODO:
	for !request.done() {
		n, err := reader.Read(buf[bufLen:])
		if err != nil {
			return nil, err
		}
		// if n == 0 {
		// 	return nil, errors.New("TODO:")
		// }

		bufLen += n

		parsedN, err := request.parse(buf[:bufLen])
		if err != nil {
			return nil, err
		}
		copy(buf, buf[parsedN:bufLen])
		bufLen -= parsedN
	}

	return request, nil
}
