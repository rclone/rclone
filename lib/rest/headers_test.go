package rest

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseSizeFromHeaders(t *testing.T) {
	testCases := []struct {
		ContentLength, ContentRange string
		Size                        int64
	}{{
		"", "", -1,
	}, {
		"42", "", 42,
	}, {
		"42", "invalid", -1,
	}, {
		"", "bytes 22-33/42", 42,
	}, {
		"12", "bytes 22-33/42", 42,
	}, {
		"12", "otherUnit 22-33/42", -1,
	}, {
		"12", "bytes 22-33/*", -1,
	}, {
		"0", "bytes */42", 42,
	}}
	for _, testCase := range testCases {
		headers := make(http.Header, 2)
		if len(testCase.ContentLength) > 0 {
			headers.Set("Content-Length", testCase.ContentLength)
		}
		if len(testCase.ContentRange) > 0 {
			headers.Set("Content-Range", testCase.ContentRange)
		}
		assert.Equalf(t, testCase.Size, ParseSizeFromHeaders(headers), "%+v", testCase)
	}
}
