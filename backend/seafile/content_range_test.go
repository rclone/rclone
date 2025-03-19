package seafile

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestContentRangeHeader(t *testing.T) {
	fixtures := []struct {
		start, chunkSize, size int64
		expect                 string
	}{
		{0, 1, 10, "bytes 0-0/10"}, // from byte 0 (inclusive) to byte 0 (inclusive) == 1 byte
		{0, 10, 10, "bytes 0-9/10"},
		{0, 20, 10, "bytes 0-9/10"},
		{1, 1, 10, "bytes 1-1/10"},
		{1, 10, 10, "bytes 1-9/10"},
		{1, 10, 10, "bytes 1-9/10"},
		{9, 1, 10, "bytes 9-9/10"},
		{9, 2, 10, "bytes 9-9/10"},
		{9, 5, 10, "bytes 9-9/10"},
	}

	for _, fixture := range fixtures {
		t.Run(fmt.Sprintf("%+v", fixture), func(t *testing.T) {
			r := &chunkedContentRange{start: fixture.start, chunkSize: fixture.chunkSize, size: fixture.size}
			assert.Equal(t, fixture.expect, r.getContentRangeHeader())
		})
	}
}

func TestChunkSize(t *testing.T) {
	fixtures := []struct {
		start, chunkSize, size int64
		expected               int64
		isLastChunk            bool
	}{
		{0, 10, 10, 10, true},  // chunk size same as size
		{0, 20, 10, 10, true},  // chuck size bigger than size
		{0, 10, 20, 10, false}, // chuck size smaller than size
		{1, 10, 10, 9, true},   // chunk size same as size
		{1, 20, 10, 9, true},   // chuck size bigger than size
		{1, 10, 20, 10, false}, // chuck size smaller than size
		{15, 10, 20, 5, true},  // smaller remaining
	}

	for _, fixture := range fixtures {
		t.Run(fmt.Sprintf("%d/%d/%d", fixture.start, fixture.chunkSize, fixture.size), func(t *testing.T) {
			r := &chunkedContentRange{start: fixture.start, chunkSize: fixture.chunkSize, size: fixture.size}
			assert.Equal(t, fixture.expected, r.getChunkSize())
			assert.Equal(t, fixture.isLastChunk, r.isLastChunk())
		})
	}
}

func TestRanges(t *testing.T) {
	fixtures := []struct {
		size           int64
		chunkSize      int64
		expectedChunks int
	}{
		{10, 1, 10},
		{20, 2, 10},
		{10, 10, 1},
		{10, 3, 4},
	}
	for _, fixture := range fixtures {
		t.Run(fmt.Sprintf("%d/%d", fixture.size, fixture.chunkSize), func(t *testing.T) {
			r := newChunkedContentRange(fixture.chunkSize, fixture.size)
			// first chunk is counted before the loop
			count := 1
			size := r.getChunkSize()

			for !r.isLastChunk() {
				r.next()
				count++
				size += r.getChunkSize()
			}
			assert.Panics(t, func() { r.next() })
			assert.Equal(t, fixture.expectedChunks, count)
			assert.Equal(t, fixture.size, size)
		})
	}
}
