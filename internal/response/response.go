package response

import (
	"errors"
	"io"
	"maps"
	"ngonx/internal/headers"
	"strconv"
	"time"
)

var ErrMissingExpectedHeader = errors.New("missing expected header")
var ErrUnknownStatusCode = errors.New("unknown status code")
var ErrWritingHeaders = errors.New("writing headers error")
var ErrFormatingHeaders = errors.New("formating headers error")
var ErrWritingBody = errors.New("writing body error")

type StatusCode int

var CommonHeaders = headers.Headers{
	"content-length":    "0",
	"content-type":      "text/html; charset=utf-8",
	"connection":        "keep-alive",
	"date":              getCurrentUTCDate(),
	"content-encoding ": "identity",
	"cache-control ":    "no-cache",
	"server":            "ngonx/http",
}

const (
	StatusOK                      StatusCode = 200
	StatusCreated                 StatusCode = 201
	StatusNoContent               StatusCode = 204
	StatusMultipleChoices         StatusCode = 300
	StatusMovedPermanently        StatusCode = 301
	StatusFound                   StatusCode = 302
	StatusSeeOther                StatusCode = 303
	StatusNotModified             StatusCode = 304
	StatusTemporaryRedirect       StatusCode = 307
	StatusPermanentRedirect       StatusCode = 308
	StatusBadRequest              StatusCode = 400
	StatusUnauthorized            StatusCode = 401
	StatusForbidden               StatusCode = 403
	StatusNotFound                StatusCode = 404
	StatusInternalServerError     StatusCode = 500
	StatusServiceUnavailable      StatusCode = 503
	StatusHTTPVersionNotSupported StatusCode = 505
)

const dateRFC7231Format string = "Mon, 02 Jan 2006 15:04:05 GMT"

var reasonPhrases = map[StatusCode]string{
	StatusOK:                      "OK",
	StatusCreated:                 "Created",
	StatusNoContent:               "No Content",
	StatusMultipleChoices:         "Multiple Choices",
	StatusMovedPermanently:        "Moved Permanently",
	StatusFound:                   "Found",
	StatusSeeOther:                "See Other",
	StatusNotModified:             "Not Modified",
	StatusTemporaryRedirect:       "Temporary Redirect",
	StatusPermanentRedirect:       "Permanent Redirect",
	StatusBadRequest:              "Bad Request",
	StatusUnauthorized:            "Unauthorized",
	StatusForbidden:               "Forbidden",
	StatusNotFound:                "Not Found",
	StatusInternalServerError:     "Internal Server Error",
	StatusServiceUnavailable:      "Service Unavailable",
	StatusHTTPVersionNotSupported: "HTTP Version Not Supported",
}

var defaultHeaders []string = []string{"content-length", "content-type", "connection", "date", "content-encoding", "cache-control", "server"}

func getCurrentUTCDate() string {
	t := time.Now().UTC()
	return t.Format(dateRFC7231Format)
}

func WriteStatusLine(writer io.Writer, status StatusCode) error {
	text, ok := reasonPhrases[status]
	if !ok {
		return ErrUnknownStatusCode
	}

	_, err := writer.Write([]byte("HTTP/1.1 " + strconv.Itoa(int(status)) + " " + text + "\r\n"))
	return err
}

func GetDefaultHeaders(h headers.Headers) headers.Headers {
	if h == nil {
		return CommonHeaders
	}
	maps.Copy(CommonHeaders, h)

	return CommonHeaders
}

func WriteHeaders(writer io.Writer, headers headers.Headers) error {
	h := GetDefaultHeaders(headers)
	for key, value := range h {

		_, err := writer.Write([]byte(key + ": " + value + "\r\n"))
		if err != nil {
			return ErrFormatingHeaders
		}
	}
	_, err := writer.Write([]byte("\r\n"))
	return err
}

func WriteResponse(writer io.Writer, status int, headers headers.Headers, body []byte) error {
	if err := WriteStatusLine(writer, StatusCode(status)); err != nil {
		return err
	}
	if err := WriteHeaders(writer, headers); err != nil {
		return ErrWritingHeaders
	}

	// response body
	_, err := writer.Write(body)
	if err != nil {
		return ErrWritingBody
	}
	return nil
}
