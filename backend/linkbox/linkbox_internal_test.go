package linkbox

import (
	"bytes"
	"crypto/md5"
	"fmt"
	"io"
	"testing"

	"github.com/rclone/rclone/lib/pool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReadPrefix(t *testing.T) {
	for _, size := range []int64{1, 1024, prefixSize - 1, prefixSize, prefixSize + 1, 3 * prefixSize} {
		t.Run(fmt.Sprintf("%d", size), func(t *testing.T) {
			inUse := pool.Global().InUse()
			content := bytes.Repeat([]byte("x"), int(size))
			for i := range content {
				content[i] = byte(i)
			}
			wantPrefix := content[:min(size, prefixSize)]
			wantMD5 := fmt.Sprintf("%x", md5.Sum(wantPrefix))

			in := bytes.NewReader(content)
			rw, gotMD5, err := readPrefix(in)
			require.NoError(t, err)
			assert.Equal(t, wantMD5, gotMD5)
			assert.Equal(t, int64(len(wantPrefix)), rw.Size())

			// The prefix followed by the rest of in must reproduce the content
			got, err := io.ReadAll(io.MultiReader(rw, in))
			require.NoError(t, err)
			assert.Equal(t, content, got)

			require.NoError(t, rw.Close())
			assert.Equal(t, inUse, pool.Global().InUse(), "pool buffers leaked")
		})
	}
}
