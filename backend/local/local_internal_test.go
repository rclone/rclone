package local

import (
	"path"
	"testing"
	"time"

	"github.com/ncw/rclone/fs/hash"
	"github.com/ncw/rclone/fstest"
	"github.com/ncw/rclone/lib/file"
	"github.com/ncw/rclone/lib/readers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestMain drives the tests
func TestMain(m *testing.M) {
	fstest.TestMain(m)
}

func TestMapper(t *testing.T) {
	m := newMapper()
	assert.Equal(t, m.m, map[string]string{})
	assert.Equal(t, "potato", m.Save("potato", "potato"))
	assert.Equal(t, m.m, map[string]string{})
	assert.Equal(t, "-r'áö", m.Save("-r?'a´o¨", "-r'áö"))
	assert.Equal(t, m.m, map[string]string{
		"-r'áö": "-r?'a´o¨",
	})
	assert.Equal(t, "potato", m.Load("potato"))
	assert.Equal(t, "-r?'a´o¨", m.Load("-r'áö"))
}

// Test copy with source file that's updating
func TestUpdatingCheck(t *testing.T) {
	r := fstest.NewRun(t)
	defer r.Finalise()
	filePath := "sub dir/local test"
	r.WriteFile(filePath, "content", time.Now())

	fd, err := file.Open(path.Join(r.LocalName, filePath))
	if err != nil {
		t.Fatalf("failed opening file %q: %v", filePath, err)
	}

	fi, err := fd.Stat()
	require.NoError(t, err)
	o := &Object{size: fi.Size(), modTime: fi.ModTime(), fs: &Fs{}}
	wrappedFd := readers.NewLimitedReadCloser(fd, -1)
	hash, err := hash.NewMultiHasherTypes(hash.Supported)
	require.NoError(t, err)
	in := localOpenFile{
		o:    o,
		in:   wrappedFd,
		hash: hash,
		fd:   fd,
	}

	buf := make([]byte, 1)
	_, err = in.Read(buf)
	require.NoError(t, err)

	r.WriteFile(filePath, "content updated", time.Now())
	_, err = in.Read(buf)
	require.Errorf(t, err, "can't copy - source file is being updated")

	// turn the checking off and try again
	in.o.fs.opt.NoCheckUpdated = true

	r.WriteFile(filePath, "content updated", time.Now())
	_, err = in.Read(buf)
	require.NoError(t, err)

}
