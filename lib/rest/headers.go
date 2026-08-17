package rest

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/rclone/rclone/fs"
)

// ContentRange is a parsed Content-Range response header.
type ContentRange struct {
	Start int64 // first byte position of the range
	End   int64 // last byte position of the range (inclusive)
	Size  int64 // complete length of the representation, or -1 if unknown
}

// ParseContentRange parses a Content-Range header of the form
// "bytes start-end/size" as returned with a ranged (206) response.
// The Size is -1 if the complete length is unknown ("*").
func ParseContentRange(value string) (ContentRange, error) {
	const prefix = "bytes "
	if !strings.HasPrefix(value, prefix) {
		return ContentRange{}, fmt.Errorf("doesn't start with %q", prefix)
	}

	rangeAndSize := strings.Split(value[len(prefix):], "/")
	if len(rangeAndSize) != 2 {
		return ContentRange{}, errors.New("must contain one '/'")
	}
	bounds := strings.Split(rangeAndSize[0], "-")
	if len(bounds) != 2 {
		return ContentRange{}, errors.New("must contain one '-'")
	}

	start, err := strconv.ParseInt(bounds[0], 10, 64)
	if err != nil || start < 0 {
		return ContentRange{}, errors.New("invalid start")
	}
	end, err := strconv.ParseInt(bounds[1], 10, 64)
	if err != nil || end < start {
		return ContentRange{}, errors.New("invalid end")
	}

	size := int64(-1)
	if rangeAndSize[1] != "*" {
		size, err = strconv.ParseInt(rangeAndSize[1], 10, 64)
		if err != nil || size < 0 || end >= size {
			return ContentRange{}, errors.New("invalid complete length")
		}
	}

	return ContentRange{Start: start, End: end, Size: size}, nil
}

// CheckContentRange checks that a response satisfies a Range open option.
// The size is the expected size of the complete representation, or -1 if it is
// unknown. Calls without a Range option are ignored.
func CheckContentRange(resp *http.Response, options []fs.OpenOption, size int64) error {
	var requestRange string
	for _, option := range options {
		key, value := option.Header()
		if strings.EqualFold(key, "Range") {
			requestRange = value
		}
	}
	if requestRange == "" {
		return nil
	}

	requested, err := fs.ParseRangeOption(requestRange)
	if err != nil {
		return fmt.Errorf("invalid requested range %q: %w", requestRange, err)
	}
	if requested.Start < 0 && requested.End < 0 {
		return fmt.Errorf("invalid requested range %q", requestRange)
	}
	if requested.Start >= 0 && requested.End >= 0 && requested.End < requested.Start {
		return fmt.Errorf("invalid requested range %q", requestRange)
	}
	if resp == nil {
		return errors.New("nil response to range request")
	}

	if resp.StatusCode == http.StatusOK {
		responseSize := size
		if resp.ContentLength >= 0 {
			if size >= 0 && resp.ContentLength != size {
				return fmt.Errorf("Content-Length %d does not match expected size %d", resp.ContentLength, size)
			}
			responseSize = resp.ContentLength
		}
		if responseSize >= 0 {
			offset, limit := requested.Decode(responseSize)
			if limit < 0 {
				limit = responseSize - offset
			}
			if offset == 0 && limit >= responseSize {
				return nil
			}
		} else if requested.Start == 0 && requested.End < 0 {
			return nil
		}
		return fmt.Errorf("%w %q", fs.ErrorRangeIgnored, requestRange)
	}
	if resp.StatusCode != http.StatusPartialContent {
		return fmt.Errorf("response status %d does not satisfy requested range %q", resp.StatusCode, requestRange)
	}

	responseRange := resp.Header.Get("Content-Range")
	got, err := ParseContentRange(responseRange)
	if err != nil {
		return fmt.Errorf("invalid Content-Range %q: %w", responseRange, err)
	}

	if size >= 0 && got.Size >= 0 && got.Size != size {
		return fmt.Errorf("Content-Range %q does not match expected size %d", responseRange, size)
	}

	rangeSize := size
	if rangeSize < 0 {
		rangeSize = got.Size
	}
	expectedStart := requested.Start
	expectedEnd := requested.End
	if requested.Start >= 0 {
		if rangeSize >= 0 && (expectedEnd < 0 || expectedEnd >= rangeSize) {
			expectedEnd = rangeSize - 1
		}
	} else if rangeSize >= 0 {
		expectedStart = rangeSize - requested.End
		if expectedStart < 0 {
			expectedStart = 0
		}
		expectedEnd = rangeSize - 1
	}

	var matches bool
	if requested.Start < 0 && rangeSize < 0 {
		matches = got.End-got.Start+1 <= requested.End
	} else {
		matches = got.Start == expectedStart
		if expectedEnd >= 0 {
			matches = matches && got.End == expectedEnd
		}
	}
	if !matches {
		return fmt.Errorf("Content-Range %q does not match requested range %q", responseRange, requestRange)
	}

	contentLength := got.End - got.Start + 1
	if contentLength <= 0 {
		return fmt.Errorf("invalid Content-Range %q: length overflows", responseRange)
	}
	if resp.ContentLength >= 0 && resp.ContentLength != contentLength {
		return fmt.Errorf("Content-Length %d does not match Content-Range %q", resp.ContentLength, responseRange)
	}
	return nil
}

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
