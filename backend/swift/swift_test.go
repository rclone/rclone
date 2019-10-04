// Test Swift filesystem interface
package swift

import (
	"bytes"
	"context"
	"io"
	"testing"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/hash"
	"github.com/rclone/rclone/fs/object"
	"github.com/rclone/rclone/fstest"
	"github.com/rclone/rclone/fstest/fstests"
	"github.com/rclone/rclone/lib/random"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestIntegration runs integration tests against the remote
func TestIntegration(t *testing.T) {
	fstests.Run(t, &fstests.Opt{
		RemoteName: "TestSwiftAIO:",
		NilObject:  (*Object)(nil),
	})
}

func (f *Fs) SetUploadChunkSize(cs fs.SizeSuffix) (fs.SizeSuffix, error) {
	return f.setUploadChunkSize(cs)
}

var _ fstests.SetUploadChunkSizer = (*Fs)(nil)

// Check that PutStream works with NoChunk as it is the major code
// deviation
func (f *Fs) testNoChunk(t *testing.T) {
	ctx := context.Background()
	f.opt.NoChunk = true
	defer func() {
		f.opt.NoChunk = false
	}()

	file := fstest.Item{
		ModTime: fstest.Time("2001-02-03T04:05:06.499999999Z"),
		Path:    "piped data no chunk.txt",
		Size:    -1, // use unknown size during upload
	}

	const contentSize = 100

	contents := random.String(contentSize)
	buf := bytes.NewBufferString(contents)
	uploadHash := hash.NewMultiHasher()
	in := io.TeeReader(buf, uploadHash)

	file.Size = -1
	obji := object.NewStaticObjectInfo(file.Path, file.ModTime, file.Size, true, nil, nil)
	obj, err := f.Features().PutStream(ctx, in, obji)
	require.NoError(t, err)

	file.Hashes = uploadHash.Sums()
	file.Size = int64(contentSize) // use correct size when checking
	file.Check(t, obj, f.Precision())

	// Re-read the object and check again
	obj, err = f.NewObject(ctx, file.Path)
	require.NoError(t, err)
	file.Check(t, obj, f.Precision())

	// Delete the object
	assert.NoError(t, obj.Remove(ctx))
}

// Additional tests that aren't in the framework
func (f *Fs) InternalTest(t *testing.T) {
	t.Run("NoChunk", f.testNoChunk)
}

var _ fstests.InternalTester = (*Fs)(nil)
