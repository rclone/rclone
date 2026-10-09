package rest

import (
	"net/http"
	"testing"

	"github.com/rclone/rclone/fs"
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

func TestCheckContentRange(t *testing.T) {
	testCases := []struct {
		name          string
		status        int
		contentRange  string
		contentLength int64
		unknownSize   bool
		options       []fs.OpenOption
		wantErr       bool
		wantErrorIs   error
	}{
		{
			name:          "exact range",
			status:        http.StatusPartialContent,
			contentRange:  "bytes 2-4/10",
			contentLength: 3,
			options:       []fs.OpenOption{&fs.RangeOption{Start: 2, End: 4}},
		},
		{
			name:          "unknown content length",
			status:        http.StatusPartialContent,
			contentRange:  "bytes 2-4/10",
			contentLength: -1,
			options:       []fs.OpenOption{&fs.RangeOption{Start: 2, End: 4}},
		},
		{
			name:          "unknown complete length",
			status:        http.StatusPartialContent,
			contentRange:  "bytes 2-4/*",
			contentLength: 3,
			options:       []fs.OpenOption{&fs.RangeOption{Start: 2, End: 4}},
		},
		{
			name:          "open ended range",
			status:        http.StatusPartialContent,
			contentRange:  "bytes 2-9/10",
			contentLength: 8,
			options:       []fs.OpenOption{&fs.RangeOption{Start: 2, End: -1}},
		},
		{
			name:          "suffix range",
			status:        http.StatusPartialContent,
			contentRange:  "bytes 6-9/10",
			contentLength: 4,
			options:       []fs.OpenOption{&fs.RangeOption{Start: -1, End: 4}},
		},
		{
			name:          "suffix range with unknown complete length",
			status:        http.StatusPartialContent,
			contentRange:  "bytes 6-9/*",
			contentLength: 4,
			options:       []fs.OpenOption{&fs.RangeOption{Start: -1, End: 4}},
		},
		{
			name:          "suffix range larger than representation",
			status:        http.StatusPartialContent,
			contentRange:  "bytes 0-9/10",
			contentLength: 10,
			options:       []fs.OpenOption{&fs.RangeOption{Start: -1, End: 20}},
		},
		{
			name:          "seek option",
			status:        http.StatusPartialContent,
			contentRange:  "bytes 2-9/10",
			contentLength: 8,
			options:       []fs.OpenOption{&fs.SeekOption{Offset: 2}},
		},
		{
			name:          "whole representation range",
			status:        http.StatusOK,
			contentLength: 10,
			options:       []fs.OpenOption{&fs.RangeOption{Start: 0, End: 9}},
		},
		{
			name:          "open ended whole representation range with unknown size",
			status:        http.StatusOK,
			contentLength: -1,
			unknownSize:   true,
			options:       []fs.OpenOption{&fs.RangeOption{Start: 0, End: -1}},
		},
		{
			name:          "partial range ignored",
			status:        http.StatusOK,
			contentLength: 10,
			options:       []fs.OpenOption{&fs.RangeOption{Start: 2, End: 4}},
			wantErr:       true,
			wantErrorIs:   fs.ErrorRangeIgnored,
		},
		{
			name:          "whole range response has wrong length",
			status:        http.StatusOK,
			contentLength: 9,
			options:       []fs.OpenOption{&fs.RangeOption{Start: 0, End: 9}},
			wantErr:       true,
		},
		{
			name:          "no range requested",
			status:        http.StatusPartialContent,
			contentRange:  "bytes 2-4/10",
			contentLength: 3,
		},
		{
			name:          "missing content range",
			status:        http.StatusPartialContent,
			contentLength: 3,
			options:       []fs.OpenOption{&fs.RangeOption{Start: 2, End: 4}},
			wantErr:       true,
		},
		{
			name:          "wrong unit",
			status:        http.StatusPartialContent,
			contentRange:  "items 2-4/10",
			contentLength: 3,
			options:       []fs.OpenOption{&fs.RangeOption{Start: 2, End: 4}},
			wantErr:       true,
		},
		{
			name:          "wrong start",
			status:        http.StatusPartialContent,
			contentRange:  "bytes 0-2/10",
			contentLength: 3,
			options:       []fs.OpenOption{&fs.RangeOption{Start: 2, End: 4}},
			wantErr:       true,
		},
		{
			name:          "wrong end",
			status:        http.StatusPartialContent,
			contentRange:  "bytes 2-5/10",
			contentLength: 4,
			options:       []fs.OpenOption{&fs.RangeOption{Start: 2, End: 4}},
			wantErr:       true,
		},
		{
			name:          "wrong content length",
			status:        http.StatusPartialContent,
			contentRange:  "bytes 2-4/10",
			contentLength: 4,
			options:       []fs.OpenOption{&fs.RangeOption{Start: 2, End: 4}},
			wantErr:       true,
		},
		{
			name:          "invalid complete length",
			status:        http.StatusPartialContent,
			contentRange:  "bytes 2-10/10",
			contentLength: 9,
			options:       []fs.OpenOption{&fs.RangeOption{Start: 2, End: 10}},
			wantErr:       true,
		},
		{
			name:          "wrong complete length",
			status:        http.StatusPartialContent,
			contentRange:  "bytes 2-4/11",
			contentLength: 3,
			options:       []fs.OpenOption{&fs.RangeOption{Start: 2, End: 4}},
			wantErr:       true,
		},
		{
			name:          "unexpected successful status",
			status:        http.StatusNoContent,
			contentLength: 0,
			options:       []fs.OpenOption{&fs.RangeOption{Start: 2, End: 4}},
			wantErr:       true,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			expectedSize := int64(10)
			if testCase.unknownSize {
				expectedSize = -1
			}
			resp := &http.Response{
				StatusCode:    testCase.status,
				Header:        make(http.Header),
				ContentLength: testCase.contentLength,
			}
			if testCase.contentRange != "" {
				resp.Header.Set("Content-Range", testCase.contentRange)
			}

			err := CheckContentRange(resp, testCase.options, expectedSize)
			if testCase.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
			if testCase.wantErrorIs != nil {
				assert.ErrorIs(t, err, testCase.wantErrorIs)
			}
		})
	}
}
