package chunkedreader

import (
	"context"
	"fmt"
	"io"
	"math/rand"
	"testing"

	"github.com/rclone/rclone/fstest/mockobject"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestChunkedReader(t *testing.T) {
	content := makeContent(t, 1024)

	for _, mode := range mockobject.SeekModes {
		t.Run(mode.String(), testRead(content, mode))
	}
}

func testRead(content []byte, mode mockobject.SeekMode) func(*testing.T) {
	return func(t *testing.T) {
		chunkSizes := []int64{-1, 0, 1, 15, 16, 17, 1023, 1024, 1025, 2000}
		offsets := []int64{0, 1, 2, 3, 4, 5, 7, 8, 9, 15, 16, 17, 31, 32, 33,
			63, 64, 65, 511, 512, 513, 1023, 1024, 1025}
		limits := []int64{-1, 0, 1, 31, 32, 33, 1023, 1024, 1025}
		cl := int64(len(content))
		bl := 32
		buf := make([]byte, bl)

		o := mockobject.New("test.bin").WithContent(content, mode)
		for ics, cs := range chunkSizes {
			for icsMax, csMax := range chunkSizes {
				// skip tests where chunkSize is much bigger than maxChunkSize
				if ics > icsMax+1 {
					continue
				}

				t.Run(fmt.Sprintf("Chunksize_%d_%d", cs, csMax), func(t *testing.T) {
					cr := New(context.Background(), o, cs, csMax)

					for _, offset := range offsets {
						for _, limit := range limits {
							what := fmt.Sprintf("offset %d, limit %d", offset, limit)

							p, err := cr.RangeSeek(context.Background(), offset, io.SeekStart, limit)
							if offset >= cl {
								require.Error(t, err, what)
								return
							}
							require.NoError(t, err, what)
							require.Equal(t, offset, p, what)

							n, err := cr.Read(buf)
							end := offset + int64(bl)
							if end > cl {
								end = cl
							}
							l := int(end - offset)
							if l < bl {
								require.Equal(t, io.EOF, err, what)
							} else {
								require.NoError(t, err, what)
							}
							require.Equal(t, l, n, what)
							require.Equal(t, content[offset:end], buf[:n], what)
						}
					}
				})
			}
		}
	}
}

func TestErrorAfterClose(t *testing.T) {
	content := makeContent(t, 1024)
	o := mockobject.New("test.bin").WithContent(content, mockobject.SeekModeNone)

	// Close
	cr := New(context.Background(), o, 0, 0)
	require.NoError(t, cr.Close())
	require.Error(t, cr.Close())

	// Read
	cr = New(context.Background(), o, 0, 0)
	require.NoError(t, cr.Close())
	var buf [1]byte
	_, err := cr.Read(buf[:])
	require.Error(t, err)

	// Seek
	cr = New(context.Background(), o, 0, 0)
	require.NoError(t, cr.Close())
	_, err = cr.Seek(1, io.SeekCurrent)
	require.Error(t, err)

	// RangeSeek
	cr = New(context.Background(), o, 0, 0)
	require.NoError(t, cr.Close())
	_, err = cr.RangeSeek(context.Background(), 1, io.SeekCurrent, 0)
	require.Error(t, err)
}

func makeContent(t *testing.T, size int) []byte {
	content := make([]byte, size)
	r := rand.New(rand.NewSource(42))
	_, err := io.ReadFull(r, content)
	assert.NoError(t, err)
	return content
}
