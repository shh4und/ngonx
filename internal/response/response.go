package response

import (
	"errors"
	"fmt"
	"io"
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

const (
	StatusOK                  StatusCode = 200
	StatusNotFound            StatusCode = 404
	StatusInternalServerError StatusCode = 500
)

const dateRFC7231Format string = "Mon, 02 Jan 2006 15:04:05 GMT"

var reasonPhrases = map[StatusCode]string{
	StatusOK:                  "OK",
	StatusNotFound:            "Not Found",
	StatusInternalServerError: "Internal Server Error",
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

func GetDefaultHeaders(contentLen int, contentType *string, connection *string, currDate *string, contentEncoding *string, cacheControl *string, server *string) headers.Headers {
	h := headers.NewHeaders()
	h["content-length"] = strconv.Itoa(contentLen)

	if contentType != nil {
		h["content-type"] = *contentType
	} else {
		h["content-type"] = "text/html; charset=utf-8"
	}
	if connection != nil {
		h["connection"] = *connection
	} else {
		h["connection"] = "close"
	}

	if currDate != nil {
		h["date"] = *currDate
	} else {
		h["date"] = getCurrentUTCDate()
	}
	if contentEncoding != nil {
		h["content-encoding"] = *contentEncoding
	} else {
		h["content-encoding"] = "identity"
	}
	if cacheControl != nil {
		h["cache-control"] = *cacheControl
	} else {
		h["cache-control"] = "no-cache"
	}
	if server != nil {
		h["server"] = *server
	} else {
		h["server"] = "NGONX/Apache"
	}
	return h
}

func WriteHeaders(writer io.Writer, responseContentLen int, contentType *string, connection *string, currDate *string, contentEncoding *string, cacheControl *string, server *string) error {
	h := GetDefaultHeaders(responseContentLen, contentType, connection, currDate, contentEncoding, cacheControl, server)
	for _, key := range defaultHeaders {
		value, ok := h[key]
		if !ok {
			return fmt.Errorf("%w: %s", ErrMissingExpectedHeader, key)
		}
		_, err := writer.Write([]byte(key + ": " + value + "\r\n"))
		if err != nil {
			return ErrFormatingHeaders
		}
	}
	_, err := writer.Write([]byte("\r\n"))
	return err
}

func WriteResponse(writer io.Writer, status int, responseContentLen int, contentType *string, connection *string, currDate *string, contentEncoding *string, cacheControl *string, server *string, body []byte) error {
	if err := WriteStatusLine(writer, StatusCode(status)); err != nil {
		return err
	}
	if err := WriteHeaders(writer, responseContentLen, contentType, connection, currDate, contentEncoding, cacheControl, server); err != nil {
		return ErrWritingHeaders
	}

	// response body
	_, err := writer.Write([]byte("\r\n"))

	_, err = writer.Write(body)
	if err != nil {
		return ErrWritingBody
	}
	return err
}
