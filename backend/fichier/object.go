package fichier

import (
	"context"
	"io"
	"net/http"
	"time"

	"github.com/ncw/rclone/fs"
	"github.com/ncw/rclone/fs/hash"
	"github.com/ncw/rclone/lib/rest"
	"github.com/pkg/errors"
)

// Object is a filesystem like object provided by an Fs
type Object struct {
	fs     *Fs
	remote string
	file   File
}

// String returns a description of the Object
func (f *Object) String() string {
	return f.file.Filename
}

// Remote returns the remote path
func (f *Object) Remote() string {
	return f.remote
}

// ModTime returns the modification date of the file
// It should return a best guess if one isn't available
func (f *Object) ModTime(ctx context.Context) time.Time {
	modTime, err := time.Parse("2006-01-02 15:04:05", f.file.Date)

	if err != nil {
		return time.Now()
	}

	return modTime
}

// Size returns the size of the file
func (f *Object) Size() int64 {
	return int64(f.file.Size)
}

// Fs returns read only access to the Fs that this object is part of
func (f *Object) Fs() fs.Info {
	return f.fs
}

// Hash returns the selected checksum of the file
// If no checksum is available it returns ""
func (f *Object) Hash(ctx context.Context, t hash.Type) (string, error) {
	if t != hash.Whirlpool {
		return "", hash.ErrUnsupported
	}

	return f.file.Checksum, nil
}

// Storable says whether this object can be stored
func (f *Object) Storable() bool {
	return false
}

// SetModTime sets the metadata on the object to set the modification date
func (f *Object) SetModTime(context.Context, time.Time) error {
	return fs.ErrorCantSetModTime
	//return errors.New("setting modtime is not supported for 1fichier remotes")
}

// Open opens the file for read.  Call Close() on the returned io.ReadCloser
func (f *Object) Open(ctx context.Context, options ...fs.OpenOption) (io.ReadCloser, error) {
	fs.FixRangeOption(options, int64(f.file.Size))
	downloadToken, err := f.fs.getDownloadToken(f.file.URL)

	if err != nil {
		return nil, err
	}

	var resp *http.Response
	opts := rest.Opts{
		Method:  "GET",
		RootURL: downloadToken.URL,
		Options: options,
	}

	err = f.fs.pacer.Call(func() (bool, error) {
		resp, err = f.fs.rest.Call(&opts)
		return shouldRetry(resp, err)
	})

	if err != nil {
		return nil, err
	}
	return resp.Body, err
}

// Update in to the object with the modTime given of the given size
//
// When called from outside a Fs by rclone, src.Size() will always be >= 0.
// But for unknown-sized objects (indicated by src.Size() == -1), Upload should either
// return an error or update the object properly (rather than e.g. calling panic).
func (f *Object) Update(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) error {
	//remove, then upload
	if src.Size() < 0 {
		return errors.New("refusing to update with unknown size")
	}

	err := f.Remove(ctx)
	if err != nil {
		return err
	}

	info, err := f.fs.Put(ctx, in, src)
	f.file = info.(*Object).file

	if err != nil {
		return err
	}

	return nil
}

// Remove removes this object
func (f *Object) Remove(ctx context.Context) error {
	// fs.Debugf(f, "Removing file `%s` with url `%s`", f.file.Filename, f.file.URL)

	_, err := f.fs.deleteFile(f.file.URL)

	if err != nil {
		return err
	}

	return nil
}

// Check the interfaces are satisfied
var (
	_ fs.Object = (*Object)(nil)
)
