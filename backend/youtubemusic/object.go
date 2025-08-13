package youtubemusic

import (
	"context"
	"io"
	"net/http"
	"time"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/hash"
	"github.com/rclone/rclone/fs/log"
	"github.com/rclone/rclone/lib/rest"
)

// Object describes a youtubemusic object
type Object struct {
	fs       *Fs       // what this object is part of
	remote   string    // The remote path
	url      string    // download path
	id       string    // ID of this object
	bytes    int64     // Bytes in the object
	modTime  time.Time // Modified time of the object
	mimeType string
}

// Fs returns the parent Fs
func (o *Object) Fs() fs.Info {
	return o.fs
}

// Return a string version
func (o *Object) String() string {
	if o == nil {
		return "<nil>"
	}
	return o.remote
}

// Remote returns the remote path
func (o *Object) Remote() string {
	return o.remote
}

// ModTime returns the modification time of the object
//
// It attempts to read the objects mtime and if that isn't present the
// LastModified returned in the http headers
func (o *Object) ModTime(ctx context.Context) time.Time {
	defer log.Trace(o, "")("")
	err := o.readMetaData(ctx)
	if err != nil {
		fs.Debugf(o, "ModTime: Failed to read metadata: %v", err)
		return time.Now()
	}
	return o.modTime
}

// Size returns the size of an object in bytes
func (o *Object) Size() int64 {
	// TODO:
	return 0
}

// Hash returns the Md5sum of an object returning a lowercase hex string
func (o *Object) Hash(ctx context.Context, ty hash.Type) (string, error) {
	return "", hash.ErrUnsupported

}

// Storable returns a boolean as to whether this object is storable
func (o *Object) Storable() bool {
	return true
}

// SetModTime sets the modification time of the local fs object
func (o *Object) SetModTime(ctx context.Context, t time.Time) error {
	return fs.ErrorCantSetModTime
}

// Open an object for read
func (o *Object) Open(ctx context.Context, options ...fs.OpenOption) (in io.ReadCloser, err error) {
	defer log.Trace(o, "")("")
	err = o.readMetaData(ctx)
	if err != nil {
		fs.Debugf(o, "Open: Failed to read metadata: %v", err)
		return nil, err
	}
	var resp *http.Response
	opts := rest.Opts{
		Method:  "GET",
		RootURL: o.downloadURL(),
		Options: options,
	}
	err = o.fs.pacer.Call(func() (bool, error) {
		resp, err = o.fs.srv.Call(ctx, &opts)
		return shouldRetry(ctx, resp, err)
	})
	if err != nil {
		return nil, err
	}
	return resp.Body, err
}

// Update the object with the contents of the io.Reader, modTime and size
//
// The new object may have been created if an error is returned
func (o *Object) Update(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) error {
	// TODO:
	return nil
}

// Remove an object
func (o *Object) Remove(ctx context.Context) error {
	// TODO:
	return nil
}

// readMetaData gets the metadata if it hasn't already been fetched
//
// it also sets the info
func (o *Object) readMetaData(ctx context.Context) (err error) {
	// TODO:
	return nil
}

// setMetaData sets the fs data from a storage.Object
func (o *Object) setMetaData() {
	// TODO:
}

// downloadURL returns the URL for a full bytes download for the object
func (o *Object) downloadURL() (url string) {
	// TODO:
	// url := o.url + "=d"
	// if strings.HasPrefix(o.mimeType, "video/") {
	// 	url += "v"
	// }
	return url
}

// Check the interfaces are satisfied
var _ fs.Object = &Object{}
