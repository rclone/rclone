package fs_test

import (
	"bytes"
	"io"
	"io/ioutil"
	"testing"
	"time"

	"github.com/ncw/rclone/fs"
	"github.com/ncw/rclone/fstest"
	"github.com/stretchr/testify/assert"
)

func TestStaticObject(t *testing.T) {
	r := fstest.NewRun(t)
	defer r.Finalise()

	now := time.Now()
	remote := "path/to/object"
	size := int64(1024)

	o := fs.NewStaticObjectInfo(remote, now, size, true, nil, r.Flocal)

	assert.Equal(t, r.Flocal, o.Fs())
	assert.Equal(t, remote, o.Remote())
	assert.Equal(t, remote, o.String())
	assert.Equal(t, now, o.ModTime())
	assert.Equal(t, size, o.Size())
	assert.Equal(t, true, o.Storable())

	hash, err := o.Hash(fs.HashMD5)
	assert.NoError(t, err)
	assert.Equal(t, "", hash)

	o = fs.NewStaticObjectInfo(remote, now, size, true, nil, nil)
	_, err = o.Hash(fs.HashMD5)
	assert.Equal(t, fs.ErrHashUnsupported, err)

	hs := map[fs.HashType]string{
		fs.HashMD5: "potato",
	}
	o = fs.NewStaticObjectInfo(remote, now, size, true, hs, nil)
	hash, err = o.Hash(fs.HashMD5)
	assert.NoError(t, err)
	assert.Equal(t, "potato", hash)
	_, err = o.Hash(fs.HashSHA1)
	assert.Equal(t, fs.ErrHashUnsupported, err)

}

func TestMemoryFs(t *testing.T) {
	f := fs.MemoryFs
	assert.Equal(t, "memory", f.Name())
	assert.Equal(t, "", f.Root())
	assert.Equal(t, "memory", f.String())
	assert.Equal(t, time.Nanosecond, f.Precision())
	assert.Equal(t, fs.SupportedHashes, f.Hashes())
	assert.Equal(t, &fs.Features{}, f.Features())

	entries, err := f.List("")
	assert.NoError(t, err)
	assert.Nil(t, entries)

	o, err := f.NewObject("obj")
	assert.Equal(t, fs.ErrorObjectNotFound, err)
	assert.Nil(t, o)

	buf := bytes.NewBufferString("potato")
	now := time.Now()
	src := fs.NewStaticObjectInfo("remote", now, int64(buf.Len()), true, nil, nil)
	o, err = f.Put(buf, src)
	assert.NoError(t, err)
	hash, err := o.Hash(fs.HashSHA1)
	assert.NoError(t, err)
	assert.Equal(t, "3e2e95f5ad970eadfa7e17eaf73da97024aa5359", hash)

	err = f.Mkdir("dir")
	assert.Error(t, err)

	err = f.Rmdir("dir")
	assert.Error(t, fs.ErrorDirNotFound)
}

func TestMemoryObject(t *testing.T) {
	remote := "path/to/object"
	now := time.Now()
	content := []byte("potatoXXXXXXXXXXXXX")
	content = content[:6] // make some extra cap

	o := fs.NewMemoryObject(remote, now, content)

	assert.Equal(t, content, o.Content())
	assert.Equal(t, fs.MemoryFs, o.Fs())
	assert.Equal(t, remote, o.Remote())
	assert.Equal(t, remote, o.String())
	assert.Equal(t, now, o.ModTime())
	assert.Equal(t, int64(len(content)), o.Size())
	assert.Equal(t, true, o.Storable())

	hash, err := o.Hash(fs.HashMD5)
	assert.NoError(t, err)
	assert.Equal(t, "8ee2027983915ec78acc45027d874316", hash)

	hash, err = o.Hash(fs.HashSHA1)
	assert.NoError(t, err)
	assert.Equal(t, "3e2e95f5ad970eadfa7e17eaf73da97024aa5359", hash)

	newNow := now.Add(time.Minute)
	err = o.SetModTime(newNow)
	assert.NoError(t, err)
	assert.Equal(t, newNow, o.ModTime())

	checkOpen := func(rc io.ReadCloser, expected string) {
		actual, err := ioutil.ReadAll(rc)
		assert.NoError(t, err)
		err = rc.Close()
		assert.NoError(t, err)
		assert.Equal(t, expected, string(actual))
	}

	checkContent := func(o fs.Object, expected string) {
		rc, err := o.Open()
		assert.NoError(t, err)
		checkOpen(rc, expected)
	}

	checkContent(o, string(content))

	rc, err := o.Open(&fs.RangeOption{Start: 1, End: 3})
	assert.NoError(t, err)
	checkOpen(rc, "ot")

	rc, err = o.Open(&fs.SeekOption{Offset: 3})
	assert.NoError(t, err)
	checkOpen(rc, "ato")

	// check it fits within the buffer
	newNow = now.Add(2 * time.Minute)
	newContent := bytes.NewBufferString("Rutabaga")
	assert.True(t, newContent.Len() < cap(content)) // fits within cap(content)
	src := fs.NewStaticObjectInfo(remote, newNow, int64(newContent.Len()), true, nil, nil)
	err = o.Update(newContent, src)
	assert.NoError(t, err)
	checkContent(o, "Rutabaga")
	assert.Equal(t, newNow, o.ModTime())
	assert.Equal(t, "Rutaba", string(content)) // check we re-used the buffer

	// not within the buffer
	newStr := "0123456789"
	newStr = newStr + newStr + newStr + newStr + newStr + newStr + newStr + newStr + newStr + newStr
	newContent = bytes.NewBufferString(newStr)
	assert.True(t, newContent.Len() > cap(content)) // does not fit within cap(content)
	src = fs.NewStaticObjectInfo(remote, newNow, int64(newContent.Len()), true, nil, nil)
	err = o.Update(newContent, src)
	assert.NoError(t, err)
	checkContent(o, newStr)
	assert.Equal(t, "Rutaba", string(content)) // check we didn't re-use the buffer

	// now try streaming
	newStr = "ABCDEFGHIJKLMNOPQRSTUVWXYZ"
	newContent = bytes.NewBufferString(newStr)
	src = fs.NewStaticObjectInfo(remote, newNow, -1, true, nil, nil)
	err = o.Update(newContent, src)
	assert.NoError(t, err)
	checkContent(o, newStr)

	// and zero length
	newStr = ""
	newContent = bytes.NewBufferString(newStr)
	src = fs.NewStaticObjectInfo(remote, newNow, 0, true, nil, nil)
	err = o.Update(newContent, src)
	assert.NoError(t, err)
	checkContent(o, newStr)

	err = o.Remove()
	assert.Error(t, err)
}
