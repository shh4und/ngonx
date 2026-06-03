package headers

import (
	"bytes"
	"errors"
	"regexp"
	"strings"
)

var isHeaderLineValid = regexp.MustCompile("^[A-Za-z0-9!#$%&'*+\\-.^_`|~]+$").MatchString
var CRLFByte []byte = []byte("\r\n")
var ColonByte []byte = []byte(":")
var SpaceByte []byte = []byte(" ")
var ErrFieldNotFound error = errors.New("field not found")
var ErrFieldNameMalformed error = errors.New("field name malformed")
var ErrHeaderLineNotValid error = errors.New("header line not valid")

type Headers map[string]string

func NewHeaders() Headers {
	return make(Headers)
}

func (h Headers) Parse(data []byte) (int, bool, error) {
	// Ranging over SplitSeq is more efficient than
	// bytes.Split(data, CRLFByte)
	numBytesRead := 0
	for {

		fieldLineIdx := bytes.Index(data[numBytesRead:], CRLFByte)

		if fieldLineIdx == 0 {
			numBytesRead += len(CRLFByte)
			break
		}

		if fieldLineIdx == -1 {
			return numBytesRead, false, nil
		}

		fieldLine := data[numBytesRead : numBytesRead+fieldLineIdx]

		fieldLineParts := bytes.SplitN(fieldLine, ColonByte, 2)
		if len(fieldLineParts) != 2 {
			return numBytesRead, false, ErrFieldNameMalformed
		}

		fieldName := fieldLineParts[0]
		fieldValue := bytes.TrimSpace(fieldLineParts[1]) // removes uneeded leading/trailing Spaces
		numBytesRead += fieldLineIdx + len(CRLFByte)

		spaceFound := bytes.Contains(fieldName, SpaceByte)
		if spaceFound { // if spaceFound is true, then it found some Space(s) within the field name and colon
			return numBytesRead, false, ErrFieldNameMalformed
		}

		// Validate field name contains only valid characters
		if !isHeaderLineValid(string(fieldName)) {
			return numBytesRead, false, ErrHeaderLineNotValid
		}

		// Store with lowercase key for case-insensitive lookup
		fieldNameLower := strings.ToLower(string(fieldName))

		if _, exists := h[fieldNameLower]; !exists {

			h[fieldNameLower] = string(fieldValue)
			continue
		}
		// multiple values for a single header key is valid based on [ RFC 9110 5.2 ]
		h[fieldNameLower] = strings.Join([]string{h[fieldNameLower], string(fieldValue)}, ", ")

	}

	return numBytesRead, true, nil

}
