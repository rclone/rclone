package pikpak

import (
	"bytes"
	"io"
	"os"
	"testing"

	"github.com/rclone/rclone/lib/pool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestReadGcid checks readGcid hashes and replays its input from a pooled
// buffer for small known sizes, and from a temp file for large or unknown
// sizes, releasing the buffer on cleanup.
func TestReadGcid(t *testing.T) {
	content := bytes.Repeat([]byte("pikpak"), 1024)
	want, err := calcGcid(bytes.NewReader(content), int64(len(content)))
	require.NoError(t, err)

	for _, test := range []struct {
		name      string
		size      int64
		threshold int64
		wantFile  bool
	}{
		{"memory", int64(len(content)), int64(len(content)), false},
		{"file", int64(len(content)), int64(len(content)) - 1, true},
		{"unknown size", -1, int64(len(content)), true},
	} {
		t.Run(test.name, func(t *testing.T) {
			inUse := pool.Global().InUse()
			gcid, out, cleanup, err := readGcid(bytes.NewReader(content), test.size, test.threshold)
			require.NoError(t, err)
			assert.Equal(t, want, gcid)
			_, isFile := out.(*os.File)
			assert.Equal(t, test.wantFile, isFile, "expected temp file %v", test.wantFile)
			got, err := io.ReadAll(out)
			require.NoError(t, err)
			assert.Equal(t, content, got)
			cleanup()
			assert.Equal(t, inUse, pool.Global().InUse(), "buffer should be returned to the pool")
		})
	}
}
