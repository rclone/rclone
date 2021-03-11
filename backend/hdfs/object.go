// +build !plan9

package hdfs

import (
	"context"
	"io"
	"path"
	"time"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/hash"
	"github.com/rclone/rclone/lib/readers"
)

// Object describes an HDFS file
type Object struct {
	fs      *Fs
	remote  string
	size    int64
	modTime time.Time
}

// Fs returns the parent Fs
func (o *Object) Fs() fs.Info {
	return o.fs
}

// Remote returns the remote path
func (o *Object) Remote() string {
	return o.remote
}

// Size returns the size of an object in bytes
func (o *Object) Size() int64 {
	return o.size
}

// ModTime returns the modification time of the object
func (o *Object) ModTime(ctx context.Context) time.Time {
	return o.modTime
}

// SetModTime sets the modification time of the local fs object
func (o *Object) SetModTime(ctx context.Context, modTime time.Time) error {
	realpath := o.fs.realpath(o.Remote())
	err := o.fs.client.Chtimes(realpath, modTime, modTime)
	if err != nil {
		return err
	}
	o.modTime = modTime
	return nil
}

// Storable returns whether this object is storable
func (o *Object) Storable() bool {
	return true
}

// Return a string version
func (o *Object) String() string {
	if o == nil {
		return "<nil>"
	}
	return o.Remote()
}

// Hash is not supported
func (o *Object) Hash(ctx context.Context, r hash.Type) (string, error) {
	return "", hash.ErrUnsupported
}

// Open an object for read
func (o *Object) Open(ctx context.Context, options ...fs.OpenOption) (in io.ReadCloser, err error) {
	realpath := o.realpath()
	fs.Debugf(o.fs, "open [%s]", realpath)
	f, err := o.fs.client.Open(realpath)
	if err != nil {
		return nil, err
	}

	var offset, limit int64 = 0, -1
	for _, option := range options {
		switch x := option.(type) {
		case *fs.SeekOption:
			offset = x.Offset
		case *fs.RangeOption:
			offset, limit = x.Decode(o.Size())
		}
	}

	_, err = f.Seek(offset, io.SeekStart)
	if err != nil {
		return nil, err
	}

	if limit != -1 {
		in = readers.NewLimitedReadCloser(f, limit)
	} else {
		in = f
	}

	return in, err
}

// Update object
func (o *Object) Update(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) error {
	realpath := o.fs.realpath(src.Remote())
	dirname := path.Dir(realpath)
	fs.Debugf(o.fs, "update [%s]", realpath)

	err := o.fs.client.MkdirAll(dirname, 0755)
	if err != nil {
		return err
	}

	info, err := o.fs.client.Stat(realpath)
	if err == nil {
		err = o.fs.client.Remove(realpath)
		if err != nil {
			return err
		}
	}

	out, err := o.fs.client.Create(realpath)
	if err != nil {
		return err
	}

	cleanup := func() {
		rerr := o.fs.client.Remove(realpath)
		if rerr != nil {
			fs.Errorf(o.fs, "failed to remove [%v]: %v", realpath, rerr)
		}
	}

	_, err = io.Copy(out, in)
	if err != nil {
		cleanup()
		return err
	}

	err = out.Close()
	if err != nil {
		cleanup()
		return err
	}

	info, err = o.fs.client.Stat(realpath)
	if err != nil {
		return err
	}

	err = o.SetModTime(ctx, src.ModTime(ctx))
	if err != nil {
		return err
	}
	o.size = info.Size()

	return nil
}

// Remove an object
func (o *Object) Remove(ctx context.Context) error {
	realpath := o.fs.realpath(o.remote)
	fs.Debugf(o.fs, "remove [%s]", realpath)
	return o.fs.client.Remove(realpath)
}

func (o *Object) realpath() string {
	return o.fs.opt.Enc.FromStandardPath(xPath(o.Fs().Root(), o.remote))
}

// Check the interfaces are satisfied
var (
	_ fs.Object = (*Object)(nil)
)
