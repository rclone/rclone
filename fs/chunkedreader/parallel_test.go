package chunkedreader

import (
	"context"
	"io"
	"math/rand"
	"testing"

	"github.com/rclone/rclone/fstest/mockobject"
	"github.com/rclone/rclone/lib/multipart"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParallel(t *testing.T) {
	content := makeContent(t, 1024)

	for _, mode := range mockobject.SeekModes {
		t.Run(mode.String(), testRead(content, mode, 3))
	}
}

func TestParallelErrorAfterClose(t *testing.T) {
	testErrorAfterClose(t, 3)
}

func TestParallelLarge(t *testing.T) {
	ctx := context.Background()
	const streams = 3
	const chunkSize = multipart.BufferSize
	const size = (2*streams+1)*chunkSize + 255
	content := makeContent(t, size)
	o := mockobject.New("test.bin").WithContent(content, mockobject.SeekModeNone)

	cr := New(ctx, o, chunkSize, 0, streams)

	for _, test := range []struct {
		name     string
		offset   int64
		seekMode int
	}{
		{name: "Straight", offset: 0, seekMode: -1},
		{name: "Rewind", offset: 0, seekMode: io.SeekStart},
		{name: "NearStart", offset: 1, seekMode: io.SeekStart},
		{name: "NearEnd", offset: size - 2*chunkSize - 127, seekMode: io.SeekEnd},
	} {
		t.Run(test.name, func(t *testing.T) {
			if test.seekMode >= 0 {
				var n int64
				var err error
				if test.seekMode == io.SeekEnd {
					n, err = cr.Seek(test.offset-size, test.seekMode)
				} else {
					n, err = cr.Seek(test.offset, test.seekMode)
				}
				require.NoError(t, err)
				assert.Equal(t, test.offset, n)
			}
			got, err := io.ReadAll(cr)
			require.NoError(t, err)
			require.Equal(t, len(content[test.offset:]), len(got))
			assert.Equal(t, content[test.offset:], got)
		})
	}

	require.NoError(t, cr.Close())

	t.Run("Seeky", func(t *testing.T) {
		cr := New(ctx, o, chunkSize, 0, streams)
		offset := 0
		buf := make([]byte, 1024)

		for {
			// Read and check a random read
			readSize := rand.Intn(1024)
			readBuf := buf[:readSize]
			n, err := cr.Read(readBuf)

			require.Equal(t, content[offset:offset+n], readBuf[:n])
			offset += n

			if err == io.EOF {
				assert.Equal(t, size, offset)
				break
			}
			require.NoError(t, err)

			// Now do a smaller random seek backwards
			seekSize := rand.Intn(512)
			if offset-seekSize < 0 {
				seekSize = offset
			}
			nn, err := cr.Seek(-int64(seekSize), io.SeekCurrent)
			offset -= seekSize
			require.NoError(t, err)
			assert.Equal(t, nn, int64(offset))
		}

		require.NoError(t, cr.Close())
	})

}
