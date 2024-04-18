//go:build !plan9

package storj

import (
	"context"
	"errors"
	"io"
	"path"
	"time"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/hash"
	"github.com/rclone/rclone/lib/bucket"
	"golang.org/x/text/unicode/norm"

	"storj.io/uplink"
)

// Object describes a Storj object
type Object struct {
	fs *Fs

	absolute string

	size     int64
	created  time.Time
	modified time.Time
}

// Check the interfaces are satisfied.
var _ fs.Object = &Object{}

// newObjectFromUplink creates a new object from a Storj uplink object.
func newObjectFromUplink(f *Fs, relative string, object *uplink.Object) *Object {
	// Attempt to use the modified time from the metadata. Otherwise
	// fallback to the server time.
	modified := object.System.Created

	if modifiedStr, ok := object.Custom["rclone:mtime"]; ok {
		var err error

		modified, err = time.Parse(time.RFC3339Nano, modifiedStr)
		if err != nil {
			modified = object.System.Created
		}
	}

	bucketName, _ := bucket.Split(path.Join(f.root, relative))

	return &Object{
		fs: f,

		absolute: norm.NFC.String(bucketName + "/" + object.Key),

		size:     object.System.ContentLength,
		created:  object.System.Created,
		modified: modified,
	}
}

// String returns a description of the Object
func (o *Object) String() string {
	if o == nil {
		return "<nil>"
	}

	return o.Remote()
}

// Remote returns the remote path
func (o *Object) Remote() string {
	// It is possible that we have an empty root (meaning the filesystem is
	// rooted at the project level). In this case the relative path is just
	// the full absolute path to the object (including the bucket name).
	if o.fs.root == "" {
		return o.absolute
	}

	// At this point we know that the filesystem itself is at least a
	// bucket name (and possibly a prefix path).
	//
	//                               . This is necessary to remove the slash.
	//                               |
	//                               v
	return o.absolute[len(o.fs.root)+1:]
}

// ModTime returns the modification date of the file
// It should return a best guess if one isn't available
func (o *Object) ModTime(ctx context.Context) time.Time {
	return o.modified
}

// Size returns the size of the file
func (o *Object) Size() int64 {
	return o.size
}

// Fs returns read only access to the Fs that this object is part of
func (o *Object) Fs() fs.Info {
	return o.fs
}

// Hash returns the selected checksum of the file
// If no checksum is available it returns ""
func (o *Object) Hash(ctx context.Context, ty hash.Type) (_ string, err error) {
	fs.Debugf(o, "%s", ty)

	return "", hash.ErrUnsupported
}

// Storable says whether this object can be stored
func (o *Object) Storable() bool {
	return true
}

// SetModTime sets the metadata on the object to set the modification date
func (o *Object) SetModTime(ctx context.Context, t time.Time) (err error) {
	fs.Debugf(o, "touch -d %q sj://%s", t, o.absolute)

	return fs.ErrorCantSetModTime
}

// Open opens the file for read. Call Close() on the returned io.ReadCloser
func (o *Object) Open(ctx context.Context, options ...fs.OpenOption) (_ io.ReadCloser, err error) {
	fs.Debugf(o, "cat sj://%s # %+v", o.absolute, options)

	bucketName, bucketPath := bucket.Split(o.absolute)

	// Convert the semantics of HTTP range headers to an offset and length
	// that libuplink can use.
	var (
		offset int64
		length int64 = -1
	)

	for _, option := range options {
		switch opt := option.(type) {
		case *fs.RangeOption:
			s := opt.Start >= 0
			e := opt.End >= 0

			switch {
			case s && e:
				offset = opt.Start
				length = (opt.End + 1) - opt.Start
			case s && !e:
				offset = opt.Start
			case !s && e:
				offset = -opt.End
			}
		case *fs.SeekOption:
			offset = opt.Offset
		default:
			if option.Mandatory() {
				fs.Errorf(o, "Unsupported mandatory option: %v", option)

				return nil, errors.New("unsupported mandatory option")
			}
		}
	}

	fs.Debugf(o, "range %d + %d", offset, length)

	return o.fs.project.DownloadObject(ctx, bucketName, bucketPath, &uplink.DownloadOptions{
		Offset: offset,
		Length: length,
	})
}

// Update in to the object with the modTime given of the given size
//
// When called from outside an Fs by rclone, src.Size() will always be >= 0.
// But for unknown-sized objects (indicated by src.Size() == -1), Upload should either
// return an error or update the object properly (rather than e.g. calling panic).
func (o *Object) Update(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (err error) {
	fs.Debugf(o, "cp input ./%s %+v", o.Remote(), options)

	oNew, err := o.fs.put(ctx, in, src, o.Remote(), options...)

	if err == nil {
		*o = *(oNew.(*Object))
	}

	return err
}

// Remove this object.
func (o *Object) Remove(ctx context.Context) (err error) {
	fs.Debugf(o, "rm sj://%s", o.absolute)

	bucketName, bucketPath := bucket.Split(o.absolute)

	_, err = o.fs.project.DeleteObject(ctx, bucketName, bucketPath)

	return err
}
