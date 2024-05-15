package object_test

import (
	"bytes"
	"context"
	"io"
	"testing"
	"time"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/hash"
	"github.com/rclone/rclone/fs/object"
	"github.com/stretchr/testify/assert"
)

func TestStaticObject(t *testing.T) {
	now := time.Now()
	remote := "path/to/object"
	size := int64(1024)

	o := object.NewStaticObjectInfo(remote, now, size, true, nil, object.MemoryFs)

	assert.Equal(t, object.MemoryFs, o.Fs())
	assert.Equal(t, remote, o.Remote())
	assert.Equal(t, remote, o.String())
	assert.Equal(t, now, o.ModTime(context.Background()))
	assert.Equal(t, size, o.Size())
	assert.Equal(t, true, o.Storable())

	Hash, err := o.Hash(context.Background(), hash.MD5)
	assert.NoError(t, err)
	assert.Equal(t, "", Hash)

	o = object.NewStaticObjectInfo(remote, now, size, true, nil, nil)
	_, err = o.Hash(context.Background(), hash.MD5)
	assert.Equal(t, hash.ErrUnsupported, err)
	assert.Equal(t, object.MemoryFs, o.Fs())

	hs := map[hash.Type]string{
		hash.MD5: "potato",
	}
	o = object.NewStaticObjectInfo(remote, now, size, true, hs, nil)
	Hash, err = o.Hash(context.Background(), hash.MD5)
	assert.NoError(t, err)
	assert.Equal(t, "potato", Hash)
	_, err = o.Hash(context.Background(), hash.SHA1)
	assert.Equal(t, hash.ErrUnsupported, err)
}

func TestMemoryFs(t *testing.T) {
	f := object.MemoryFs
	assert.Equal(t, "memory", f.Name())
	assert.Equal(t, "", f.Root())
	assert.Equal(t, "memory", f.String())
	assert.Equal(t, time.Nanosecond, f.Precision())
	assert.Equal(t, hash.Supported(), f.Hashes())
	assert.Equal(t, &fs.Features{}, f.Features())

	entries, err := f.List(context.Background(), "")
	assert.NoError(t, err)
	assert.Nil(t, entries)

	o, err := f.NewObject(context.Background(), "obj")
	assert.Equal(t, fs.ErrorObjectNotFound, err)
	assert.Nil(t, o)

	buf := bytes.NewBufferString("potato")
	now := time.Now()
	src := object.NewStaticObjectInfo("remote", now, int64(buf.Len()), true, nil, nil)
	o, err = f.Put(context.Background(), buf, src)
	assert.NoError(t, err)
	hash, err := o.Hash(context.Background(), hash.SHA1)
	assert.NoError(t, err)
	assert.Equal(t, "3e2e95f5ad970eadfa7e17eaf73da97024aa5359", hash)

	err = f.Mkdir(context.Background(), "dir")
	assert.Error(t, err)

	err = f.Rmdir(context.Background(), "dir")
	assert.Equal(t, fs.ErrorDirNotFound, err)
}

func TestMemoryObject(t *testing.T) {
	remote := "path/to/object"
	now := time.Now()
	content := []byte("potatoXXXXXXXXXXXXX")
	content = content[:6] // make some extra cap

	o := object.NewMemoryObject(remote, now, content)

	assert.Equal(t, content, o.Content())
	assert.Equal(t, object.MemoryFs, o.Fs())
	assert.Equal(t, remote, o.Remote())
	assert.Equal(t, remote, o.String())
	assert.Equal(t, now, o.ModTime(context.Background()))
	assert.Equal(t, int64(len(content)), o.Size())
	assert.Equal(t, true, o.Storable())

	Hash, err := o.Hash(context.Background(), hash.MD5)
	assert.NoError(t, err)
	assert.Equal(t, "8ee2027983915ec78acc45027d874316", Hash)

	Hash, err = o.Hash(context.Background(), hash.SHA1)
	assert.NoError(t, err)
	assert.Equal(t, "3e2e95f5ad970eadfa7e17eaf73da97024aa5359", Hash)

	newNow := now.Add(time.Minute)
	err = o.SetModTime(context.Background(), newNow)
	assert.NoError(t, err)
	assert.Equal(t, newNow, o.ModTime(context.Background()))

	checkOpen := func(rc io.ReadCloser, expected string) {
		actual, err := io.ReadAll(rc)
		assert.NoError(t, err)
		err = rc.Close()
		assert.NoError(t, err)
		assert.Equal(t, expected, string(actual))
	}

	checkContent := func(o fs.Object, expected string) {
		rc, err := o.Open(context.Background())
		assert.NoError(t, err)
		checkOpen(rc, expected)
	}

	checkContent(o, string(content))

	rc, err := o.Open(context.Background(), &fs.RangeOption{Start: 1, End: 3})
	assert.NoError(t, err)
	checkOpen(rc, "ot")

	rc, err = o.Open(context.Background(), &fs.SeekOption{Offset: 3})
	assert.NoError(t, err)
	checkOpen(rc, "ato")

	// check it fits within the buffer
	newNow = now.Add(2 * time.Minute)
	newContent := bytes.NewBufferString("Rutabaga")
	assert.True(t, newContent.Len() < cap(content)) // fits within cap(content)
	src := object.NewStaticObjectInfo(remote, newNow, int64(newContent.Len()), true, nil, nil)
	err = o.Update(context.Background(), newContent, src)
	assert.NoError(t, err)
	checkContent(o, "Rutabaga")
	assert.Equal(t, newNow, o.ModTime(context.Background()))
	assert.Equal(t, "Rutaba", string(content)) // check we re-used the buffer

	// not within the buffer
	newStr := "0123456789"
	newStr = newStr + newStr + newStr + newStr + newStr + newStr + newStr + newStr + newStr + newStr
	newContent = bytes.NewBufferString(newStr)
	assert.True(t, newContent.Len() > cap(content)) // does not fit within cap(content)
	src = object.NewStaticObjectInfo(remote, newNow, int64(newContent.Len()), true, nil, nil)
	err = o.Update(context.Background(), newContent, src)
	assert.NoError(t, err)
	checkContent(o, newStr)
	assert.Equal(t, "Rutaba", string(content)) // check we didn't reuse the buffer

	// now try streaming
	newStr = "ABCDEFGHIJKLMNOPQRSTUVWXYZ"
	newContent = bytes.NewBufferString(newStr)
	src = object.NewStaticObjectInfo(remote, newNow, -1, true, nil, nil)
	err = o.Update(context.Background(), newContent, src)
	assert.NoError(t, err)
	checkContent(o, newStr)

	// and zero length
	newStr = ""
	newContent = bytes.NewBufferString(newStr)
	src = object.NewStaticObjectInfo(remote, newNow, 0, true, nil, nil)
	err = o.Update(context.Background(), newContent, src)
	assert.NoError(t, err)
	checkContent(o, newStr)

	err = o.Remove(context.Background())
	assert.Error(t, err)
}
