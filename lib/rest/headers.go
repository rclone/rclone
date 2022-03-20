package rest

import (
	"net/http"
	"strconv"
	"strings"
)

// ParseSizeFromHeaders parses HTTP response headers to get the full file size.
// Returns -1 if the headers did not exist or were invalid.
func ParseSizeFromHeaders(headers http.Header) (size int64) {
	size = -1

	var contentLength = headers.Get("Content-Length")
	if len(contentLength) != 0 {
		var err error
		if size, err = strconv.ParseInt(contentLength, 10, 64); err != nil {
			return -1
		}
	}

	var contentRange = headers.Get("Content-Range")
	if len(contentRange) == 0 {
		return size
	}

	if !strings.HasPrefix(contentRange, "bytes ") {
		return -1
	}
	slash := strings.IndexRune(contentRange, '/')
	if slash < 0 {
		return -1
	}
	ret, err := strconv.ParseInt(contentRange[slash+1:], 10, 64)
	if err != nil {
		return -1
	}
	return ret
}
