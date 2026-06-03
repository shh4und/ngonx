package request

import (
	"bytes"
	"errors"
	"io"

	// "log/slog"
	"ngonx/internal/headers"
	"regexp"
	"strconv"
)

var isUpperLetter = regexp.MustCompile(`^[A-Z]+$`).MatchString
var isRequestLineValid = regexp.MustCompile(`/(\w+)\s+(.*?)\s+(.*)/`).MatchString

var CRLFStr string = "\r\n"
var CRLFByte []byte = []byte("\r\n")
var ErrMalformedMsg error = errors.New("malformed message")
var ErrIncompleteRequestLine error = errors.New("incomplete request line")
var ErrUnsuportedHTTPVersion error = errors.New("unsupported http version")
var ErrRequestInErrorState error = errors.New("parse entered in error state")
var ErrIncompleteBody error = errors.New("unexpected EOF: body incomplete")

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

const BufferSize int = 4096

type ParserState int

const (
	StateReqLineInitialized ParserState = iota
	StateReqLineDone
	StateReqLineError
	StateHeadersInitialized
	StateHeadersDone
	StateHeadersError
	StateBodyInitialized
	StateBodyDone
	StateBodyError
)

type RequestLine struct {
	Method      string
	RequestURI  string
	HttpVersion string
}

type Request struct {
	RequestLine
	headers.Headers
	Body          []byte
	contentLength int
	bodyWritten   int // Track how many bytes we've actually written to Body
	ParserState
}

func NewRequest() *Request {
	return &Request{Headers: headers.NewHeaders(), contentLength: 0, bodyWritten: 0, ParserState: StateReqLineInitialized}
}

func (r *Request) parse(data []byte) (int, error) {
	read := 0
	var err error
outer:
	for {
		switch r.ParserState {
		case StateReqLineError:
			return 0, ErrRequestInErrorState

		case StateReqLineInitialized:
			n, rl, err := parseRequestLine(data[read:])
			if err != nil {
				r.ParserState = StateReqLineError
				return 0, err
			}

			if n == 0 {
				break outer
			}

			r.RequestLine = *rl
			read += n
			r.ParserState = StateReqLineDone

		case StateReqLineDone:
			r.ParserState = StateHeadersInitialized

		case StateHeadersInitialized:
			n, done, err := r.Headers.Parse(data[read:])

			if err != nil {
				r.ParserState = StateHeadersError
				return 0, err
			}
			if n == 0 {
				break outer
			}
			read += n
			if done {
				r.ParserState = StateHeadersDone
			}

		case StateHeadersDone:
			contentLengthStr, exists := r.Headers["content-length"]
			if !exists {
				r.ParserState = StateBodyDone
				continue
			}
			r.contentLength, err = strconv.Atoi(contentLengthStr)
			if err != nil {
				r.ParserState = StateBodyError
				return 0, err
			}
			// slog.Info("headers done: ", "r.contentLength", r.contentLength)
			if r.contentLength > 0 {
				r.ParserState = StateBodyInitialized
				r.Body = make([]byte, r.contentLength)
				continue
			}

		case StateBodyInitialized:
			currData := data[read:]
			if len(currData) == 0 {
				break outer
			}

			// How many bytes do we still need?
			bytesNeeded := r.contentLength - r.bodyWritten
			// How many bytes do we have available?
			bytesAvailable := len(currData)
			// Take the minimum
			remaining := min(bytesNeeded, bytesAvailable)

			// Copy to the correct position in Body
			n := copy(r.Body[r.bodyWritten:], currData[:remaining])
			r.bodyWritten += n
			// slog.Info("body init: ", "read", read, "bytesNeeded", bytesNeeded, "bytesAvailable", bytesAvailable, "copied", n, "bodyWritten", r.bodyWritten)
			read += n
			if r.bodyWritten >= r.contentLength {
				r.ParserState = StateBodyDone
				continue
			}
			// Continue processing in the same loop iteration if we have more data
			continue

		case StateBodyDone:
			break outer
		}

	}
	return read, nil
}

func (r *Request) reqLineDone() bool {
	return r.ParserState == StateReqLineDone
}

func (r *Request) reqLineError() bool {
	return r.ParserState == StateReqLineError
}
func (r *Request) headersDone() bool {
	return r.ParserState == StateHeadersDone
}

func (r *Request) headersError() bool {
	return r.ParserState == StateHeadersError
}

func (r *Request) bodyDone() bool {
	return r.ParserState == StateBodyDone
}

func (r *Request) bodyError() bool {
	return r.ParserState == StateBodyError
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

	if !isUpperLetter(string(reqParts[0])) || len(reqParts) != 3 || len(httpParts) != 2 {
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

	for !request.bodyDone() && !request.bodyError() && !request.headersDone() && !request.headersError() && !request.reqLineError() {
		n, err := reader.Read(buf[bufLen:])

		if n == 0 && err == io.EOF {
			// If we're waiting for body and got EOF before it's complete, that's an error
			if request.ParserState == StateBodyInitialized && request.bodyWritten < request.contentLength {
				request.ParserState = StateBodyError
				return nil, ErrIncompleteBody
			}
			break
		}

		if err != nil {
			return nil, err
		}

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
