package azblob

import (
	"errors"
	"fmt"
	"io"
	"strconv"
)

// httpRange defines a range of bytes within an HTTP resource, starting at offset and
// ending at offset+count. A zero-value httpRange indicates the entire resource. An httpRange
// which has an offset but na zero value count indicates from the offset to the resource's end.
type httpRange struct {
	offset int64
	count  int64
}

func (r httpRange) pointers() *string {
	if r.offset == 0 && r.count == 0 { // Do common case first for performance
		return nil	// No specified range
	}
	if r.offset < 0 {
		panic("The range offset must be >= 0")
	}
	if r.count < 0 {
		panic("The range count must be >= 0")
	}
	endOffset := "" // if count == 0
	if r.count > 0 {
		endOffset = strconv.FormatInt((r.offset+r.count)-1, 10)
	}
	dataRange := fmt.Sprintf("bytes=%v-%s", r.offset, endOffset)
	return &dataRange
}

////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////

func validateSeekableStreamAt0AndGetCount(body io.ReadSeeker) int64 {
	if body == nil { // nil body's are "logically" seekable to 0 and are 0 bytes long
		return 0
	}
	validateSeekableStreamAt0(body)
	count, err := body.Seek(0, io.SeekEnd)
	if err != nil {
		panic("failed to seek stream")
	}
	body.Seek(0, io.SeekStart)
	return count
}

func validateSeekableStreamAt0(body io.ReadSeeker) {
	if body == nil { // nil body's are "logically" seekable to 0
		return
	}
	if pos, err := body.Seek(0, io.SeekCurrent); pos != 0 || err != nil {
		if err != nil {
			panic(err)
		}
		panic(errors.New("stream must be set to position 0"))
	}
}
