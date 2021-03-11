package fichier

import (
	"context"
	"io"
	"net/http"
	"time"

	"github.com/pkg/errors"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/hash"
	"github.com/rclone/rclone/lib/rest"
)

// Object is a filesystem like object provided by an Fs
type Object struct {
	fs     *Fs
	remote string
	file   File
}

// String returns a description of the Object
func (o *Object) String() string {
	return o.file.Filename
}

// Remote returns the remote path
func (o *Object) Remote() string {
	return o.remote
}

// ModTime returns the modification date of the file
// It should return a best guess if one isn't available
func (o *Object) ModTime(ctx context.Context) time.Time {
	modTime, err := time.Parse("2006-01-02 15:04:05", o.file.Date)

	if err != nil {
		return time.Now()
	}

	return modTime
}

// Size returns the size of the file
func (o *Object) Size() int64 {
	return o.file.Size
}

// Fs returns read only access to the Fs that this object is part of
func (o *Object) Fs() fs.Info {
	return o.fs
}

// Hash returns the selected checksum of the file
// If no checksum is available it returns ""
func (o *Object) Hash(ctx context.Context, t hash.Type) (string, error) {
	if t != hash.Whirlpool {
		return "", hash.ErrUnsupported
	}

	return o.file.Checksum, nil
}

// Storable says whether this object can be stored
func (o *Object) Storable() bool {
	return true
}

// SetModTime sets the metadata on the object to set the modification date
func (o *Object) SetModTime(context.Context, time.Time) error {
	return fs.ErrorCantSetModTime
	//return errors.New("setting modtime is not supported for 1fichier remotes")
}

func (o *Object) setMetaData(file File) {
	o.file = file
}

// Open opens the file for read.  Call Close() on the returned io.ReadCloser
func (o *Object) Open(ctx context.Context, options ...fs.OpenOption) (io.ReadCloser, error) {
	fs.FixRangeOption(options, o.file.Size)
	downloadToken, err := o.fs.getDownloadToken(ctx, o.file.URL)

	if err != nil {
		return nil, err
	}

	var resp *http.Response
	opts := rest.Opts{
		Method:  "GET",
		RootURL: downloadToken.URL,
		Options: options,
	}

	err = o.fs.pacer.Call(func() (bool, error) {
		resp, err = o.fs.rest.Call(ctx, &opts)
		return shouldRetry(ctx, resp, err)
	})

	if err != nil {
		return nil, err
	}
	return resp.Body, err
}

// Update in to the object with the modTime given of the given size
//
// When called from outside an Fs by rclone, src.Size() will always be >= 0.
// But for unknown-sized objects (indicated by src.Size() == -1), Upload should either
// return an error or update the object properly (rather than e.g. calling panic).
func (o *Object) Update(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) error {
	if src.Size() < 0 {
		return errors.New("refusing to update with unknown size")
	}

	// upload with new size but old name
	info, err := o.fs.putUnchecked(ctx, in, o.Remote(), src.Size(), options...)
	if err != nil {
		return err
	}

	// Delete duplicate after successful upload
	err = o.Remove(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to remove old version")
	}

	// Replace guts of old object with new one
	*o = *info.(*Object)

	return nil
}

// Remove removes this object
func (o *Object) Remove(ctx context.Context) error {
	// fs.Debugf(f, "Removing file `%s` with url `%s`", o.file.Filename, o.file.URL)

	_, err := o.fs.deleteFile(ctx, o.file.URL)

	if err != nil {
		return err
	}

	return nil
}

// MimeType of an Object if known, "" otherwise
func (o *Object) MimeType(ctx context.Context) string {
	return o.file.ContentType
}

// ID returns the ID of the Object if known, or "" if not
func (o *Object) ID() string {
	return o.file.URL
}

// Check the interfaces are satisfied
var (
	_ fs.Object    = (*Object)(nil)
	_ fs.MimeTyper = (*Object)(nil)
	_ fs.IDer      = (*Object)(nil)
)
