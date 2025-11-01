package seafile

import "fmt"

type contentRanger interface {
	getChunkSize() int64
	getContentRangeHeader() string
}

type streamedContentRange struct {
	size int64
}

func newStreamedContentRange(size int64) *streamedContentRange {
	return &streamedContentRange{
		size: size,
	}
}
func (r *streamedContentRange) getChunkSize() int64           { return r.size }
func (r *streamedContentRange) getContentRangeHeader() string { return "" }

type chunkedContentRange struct {
	start     int64
	chunkSize int64
	size      int64
}

// newChunkedContentRange does not support streaming (unknown size)
func newChunkedContentRange(chunkSize, size int64) *chunkedContentRange {
	if size <= 0 {
		panic("content range cannot operate on streaming")
	}
	if chunkSize <= 0 {
		panic("content range cannot operate without a chunk size")
	}
	return &chunkedContentRange{
		start:     0,
		chunkSize: chunkSize,
		size:      size,
	}
}

func (r *chunkedContentRange) getEnd() int64 {
	end := r.chunkSize + r.start
	if end > r.size {
		end = r.size
	}
	return end
}

func (r *chunkedContentRange) getChunkSize() int64 {
	return r.getEnd() - r.start
}

// next moves the range to the next frame
// it panics if it was the last chunk
func (r *chunkedContentRange) next() {
	r.start += r.chunkSize
	if r.start >= r.size {
		panic("no more chunk of data")
	}
}

func (r *chunkedContentRange) isLastChunk() bool {
	return r.getEnd() == r.size
}

func (r *chunkedContentRange) getContentRangeHeader() string {
	end := r.getEnd()
	return fmt.Sprintf("bytes %d-%d/%d", r.start, end-1, r.size)
}
