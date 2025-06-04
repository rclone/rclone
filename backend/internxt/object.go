// Package internxt provides an interface to Internxt's Drive API
package internxt

import (
	"context"
	"errors"
	"io"
	"path"
	"strings"
	"time"

	"github.com/StarHack/go-internxt-drive/buckets"
	"github.com/StarHack/go-internxt-drive/files"
	"github.com/StarHack/go-internxt-drive/folders"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/hash"
)

// Object holds the data for a remote file object
type Object struct {
	f       *Fs
	remote  string
	id      string
	uuid    string
	size    int64
	modTime time.Time
}

// newObjectWithFile returns a new object by file info
func newObjectWithFile(f *Fs, remote string, file *folders.File) fs.Object {
	size, _ := file.Size.Int64()
	return &Object{
		f:       f,
		remote:  remote,
		id:      file.FileID,
		uuid:    file.UUID,
		size:    size,
		modTime: file.ModificationTime,
	}
}

// newObjectWithMetaFile returns a new object by meta file info
func newObjectWithMetaFile(f *Fs, remote string, file *buckets.CreateMetaResp) fs.Object {
	size, _ := file.Size.Int64()
	return &Object{
		f:       f,
		remote:  remote,
		uuid:    file.UUID,
		size:    size,
		modTime: time.Now(),
	}
}

// Fs returns the parent Fs
func (o *Object) Fs() fs.Info {
	return o.f
}

// String returns the remote path
func (o *Object) String() string {
	return o.remote
}

// Remote returns the remote path
func (o *Object) Remote() string {
	return o.remote
}

// Size is the file length
func (o *Object) Size() int64 {
	return o.size
}

// ModTime is the last modified time (read-only)
func (o *Object) ModTime(ctx context.Context) time.Time {
	return o.modTime
}

// Hash returns the hash value (not implemented)
func (o *Object) Hash(ctx context.Context, t hash.Type) (string, error) {
	return "", errors.New("not implemented")
}

// Storable returns if this object is storable
func (o *Object) Storable() bool {
	return true
}

// SetModTime sets the modified time
func (o *Object) SetModTime(ctx context.Context, t time.Time) error {
	return errors.New("not implemented")
}

// Open opens a file for streaming
func (o *Object) Open(ctx context.Context, options ...fs.OpenOption) (io.ReadCloser, error) {
	return buckets.DownloadFileStream(o.f.cfg, o.id)
}

// Update updates an existing file
func (o *Object) Update(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) error {
	parentDir, _ := path.Split(o.remote)
	parentDir = strings.Trim(parentDir, "/")
	folderUUID, err := o.f.dirCache.FindDir(ctx, parentDir, false)
	if err != nil {
		return err
	}

	if err := files.DeleteFile(o.f.cfg, o.uuid); err != nil {
		return err
	}

	meta, err := buckets.UploadFileStream(o.f.cfg, folderUUID, path.Base(o.remote), in, src.Size())
	if err != nil {
		return err
	}
	o.uuid = meta.UUID
	o.size = src.Size()
	o.modTime = time.Now()
	return nil
}

// Remove deletes a file
func (o *Object) Remove(ctx context.Context) error {
	return files.DeleteFile(o.f.cfg, o.uuid)
}
