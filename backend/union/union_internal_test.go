package union

import (
	"bytes"
	"context"
	"testing"
	"time"

	"github.com/rclone/rclone/fs/object"
	"github.com/rclone/rclone/fstest"
	"github.com/rclone/rclone/fstest/fstests"
	"github.com/rclone/rclone/lib/random"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func (f *Fs) TestInternalReadOnly(t *testing.T) {
	if f.name != "TestUnionRO" {
		t.Skip("Only on RO union")
	}
	dir := "TestInternalReadOnly"
	ctx := context.Background()
	rofs := f.upstreams[len(f.upstreams)-1]
	assert.False(t, rofs.IsWritable())

	// Put a file onto the read only fs
	contents := random.String(50)
	file1 := fstest.NewItem(dir+"/file.txt", contents, time.Now())
	_, obj1 := fstests.PutTestContents(ctx, t, rofs, &file1, contents, true)

	// Check read from readonly fs via union
	o, err := f.NewObject(ctx, file1.Path)
	require.NoError(t, err)
	assert.Equal(t, int64(50), o.Size())

	// Now call Update on the union Object with new data
	contents2 := random.String(100)
	file2 := fstest.NewItem(dir+"/file.txt", contents2, time.Now())
	in := bytes.NewBufferString(contents2)
	src := object.NewStaticObjectInfo(file2.Path, file2.ModTime, file2.Size, true, nil, nil)
	err = o.Update(ctx, in, src)
	require.NoError(t, err)
	assert.Equal(t, int64(100), o.Size())

	// Check we read the new object via the union
	o, err = f.NewObject(ctx, file1.Path)
	require.NoError(t, err)
	assert.Equal(t, int64(100), o.Size())

	// Remove the object
	assert.NoError(t, o.Remove(ctx))

	// Check we read the old object in the read only layer now
	o, err = f.NewObject(ctx, file1.Path)
	require.NoError(t, err)
	assert.Equal(t, int64(50), o.Size())

	// Remove file and dir from read only fs
	assert.NoError(t, obj1.Remove(ctx))
	assert.NoError(t, rofs.Rmdir(ctx, dir))
}

func (f *Fs) InternalTest(t *testing.T) {
	t.Run("ReadOnly", f.TestInternalReadOnly)
}

var _ fstests.InternalTester = (*Fs)(nil)
