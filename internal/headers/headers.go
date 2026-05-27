package headers

import (
	"bytes"
	"errors"
	"regexp"
)

var IsUpperLetter = regexp.MustCompile(`^[A-Z]+$`).MatchString
var CRLFByte []byte = []byte("\r\n")
var ColonByte []byte = []byte(":")
var SpaceByte []byte = []byte(" ")
var ErrFieldNotFound error = errors.New("field not found")
var ErrFieldNameMalformed error = errors.New("field name malformed")

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
		// removes uneeded leading/trailing Spaces
		h[string(bytes.TrimSpace(fieldName))] = string(fieldValue)

	}

	return numBytesRead, true, nil

}
