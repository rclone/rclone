package webhdfs

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"time"

	"github.com/rclone/rclone/backend/webhdfs/api"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/hash"
	"github.com/rclone/rclone/lib/rest"
)

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

// SetModTime sets the modification time of the object
func (o *Object) SetModTime(ctx context.Context, modTime time.Time) error {
	realpath := o.fs.realpath(o.remote)
	if err := o.fs.setTimes(ctx, realpath, modTime); err != nil {
		return err
	}
	// SETTIMES stores milliseconds so truncate here too to keep the
	// in-memory modtime identical to the one a fresh listing returns.
	o.modTime = modTime.Truncate(time.Millisecond)
	return nil
}

// Storable returns whether this object is storable
func (o *Object) Storable() bool {
	return true
}

// String returns a string version
func (o *Object) String() string {
	if o == nil {
		return "<nil>"
	}
	return o.Remote()
}

// Hash is not supported
func (o *Object) Hash(ctx context.Context, t hash.Type) (string, error) {
	return "", hash.ErrUnsupported
}

// Open an object for read
func (o *Object) Open(ctx context.Context, options ...fs.OpenOption) (in io.ReadCloser, err error) {
	realpath := o.fs.realpath(o.remote)

	var offset, limit int64 = 0, -1
	for _, option := range options {
		switch x := option.(type) {
		case *fs.SeekOption:
			offset = x.Offset
		case *fs.RangeOption:
			offset, limit = x.Decode(o.Size())
		default:
			if option.Mandatory() {
				fs.Logf(o, "Unsupported mandatory option: %v", option)
			}
		}
	}

	params := url.Values{}
	if offset > 0 {
		params.Set("offset", fmt.Sprintf("%d", offset))
	}
	if limit >= 0 {
		params.Set("length", fmt.Sprintf("%d", limit))
	}

	opts := &rest.Opts{
		Method:     http.MethodGet,
		Path:       encodePath(realpath),
		Parameters: o.fs.params("OPEN", params),
	}
	resp, err := o.fs.srv.Call(ctx, opts)
	if err != nil {
		if apiErr, ok := err.(*api.Error); ok && apiErr.IsFileNotFound() {
			return nil, fs.ErrorObjectNotFound
		}
		return nil, err
	}
	return resp.Body, nil
}

// Update the object with the contents of in
func (o *Object) Update(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) error {
	realpath := o.fs.realpath(o.remote)

	if err := o.fs.mkdirs(ctx, path.Dir(realpath)); err != nil {
		return err
	}

	size := src.Size()
	if err := o.fs.create(ctx, realpath, in, size); err != nil {
		return err
	}

	info, err := o.fs.getFileStatus(ctx, realpath)
	if err != nil {
		return err
	}
	o.size = info.Length
	o.modTime = info.ModTime()

	return o.SetModTime(ctx, src.ModTime(ctx))
}

// Remove an object
func (o *Object) Remove(ctx context.Context) error {
	realpath := o.fs.realpath(o.remote)
	return o.fs.remove(ctx, realpath, false)
}

// Check the interfaces are satisfied
var (
	_ fs.Object = (*Object)(nil)
)
