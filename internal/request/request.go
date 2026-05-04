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

// var ERROR_MALFORMED_MSG error = errors.New("malformed message")
// var ERROR_MALFORMED_MSG error = errors.New("malformed message")

func parseRequestLine(text []byte) (*RequestLine, error) {
	lineIdx := strings.Index(string(text), "\r\n")
	if lineIdx == -1 {
		return nil, ERROR_MALFORMED_MSG
	}
	reqLine := string(text[:lineIdx])

	reqParts := strings.Split(reqLine, " ")

	httpParts := strings.Split(reqParts[len(reqParts)-1], "/")

	if !IsUpperLetter(reqParts[0]) || len(reqParts) != 3 || len(httpParts) != 2 {
		return nil, ERROR_INCOMPLETE_REQ_LINE
	}

	return &RequestLine{
		Method:      reqParts[0],
		RequestURI:  reqParts[1],
		HttpVersion: httpParts[1],
	}, nil
}

func RequestFromReader(reader io.Reader) (*Request, error) {

	text, err := io.ReadAll(reader)
	if err != nil {
		log.Fatalf("error at reading all reader, err: %v", err)
	}

	reqLineParts, err := parseRequestLine(text)
	if err != nil {
		return nil, err
	}

	return &Request{RequestLine: *reqLineParts}, nil
}
