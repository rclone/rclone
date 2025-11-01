package seafile

import (
	"context"
	"io"
	"time"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/chunksize"
	"github.com/rclone/rclone/fs/hash"
)

// Object describes a seafile object (also commonly called a file)
type Object struct {
	fs            *Fs       // what this object is part of
	id            string    // internal ID of object
	remote        string    // The remote path (full path containing library name if target at root)
	pathInLibrary string    // Path of the object without the library name
	size          int64     // size of the object
	modTime       time.Time // modification time of the object
	libraryID     string    // Needed to download the file
}

// ==================== Interface fs.DirEntry ====================

// Return a string version
func (o *Object) String() string {
	if o == nil {
		return "<nil>"
	}
	return o.remote
}

// Remote returns the remote string
func (o *Object) Remote() string {
	return o.remote
}

// ModTime returns last modified time
func (o *Object) ModTime(context.Context) time.Time {
	return o.modTime
}

// Size returns the size of an object in bytes
func (o *Object) Size() int64 {
	return o.size
}

// ==================== Interface fs.ObjectInfo ====================

// Fs returns the parent Fs
func (o *Object) Fs() fs.Info {
	return o.fs
}

// Hash returns the selected checksum of the file
// If no checksum is available it returns ""
func (o *Object) Hash(ctx context.Context, ty hash.Type) (string, error) {
	return "", hash.ErrUnsupported
}

// Storable says whether this object can be stored
func (o *Object) Storable() bool {
	return true
}

// ==================== Interface fs.Object ====================

// SetModTime sets the metadata on the object to set the modification date
func (o *Object) SetModTime(ctx context.Context, t time.Time) error {
	return fs.ErrorCantSetModTime
}

// Open opens the file for read.  Call Close() on the returned io.ReadCloser
func (o *Object) Open(ctx context.Context, options ...fs.OpenOption) (io.ReadCloser, error) {
	downloadLink, err := o.fs.getDownloadLink(ctx, o.libraryID, o.pathInLibrary)
	if err != nil {
		return nil, err
	}
	reader, err := o.fs.download(ctx, downloadLink, o.Size(), options...)
	if err != nil {
		return nil, err
	}
	return reader, nil
}

// Update in to the object with the modTime given of the given size
//
// When called from outside an Fs by rclone, src.Size() will always be >= 0.
// But for unknown-sized objects (indicated by src.Size() == -1), Upload should either
// return an error or update the object properly (rather than e.g. calling panic).
func (o *Object) Update(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) error {
	size := src.Size()
	if size <= int64(o.fs.opt.UploadCutoff) || o.fs.noChunkUpload {
		// upload whole file in 1 request
		return o.upload(ctx, in, src)
	}
	// upload in parts
	chunkSize := chunksize.Calculator(o, size, maxParts, o.fs.opt.ChunkSize)
	return o.uploadLargeFile(ctx, in, src, chunkSize)
}

// upload whole file in 1 request
func (o *Object) upload(ctx context.Context, in io.Reader, src fs.ObjectInfo) error {
	uploadLink, err := o.fs.getUploadLink(ctx, o.libraryID)
	if err != nil {
		return err
	}

	uploaded, err := o.fs.upload(ctx, in, uploadLink, o.pathInLibrary, src.Size())
	if err != nil {
		return err
	}
	// Set the properties from the upload back to the object
	o.size = uploaded.Size
	o.id = uploaded.ID

	return nil
}

func (o *Object) uploadLargeFile(ctx context.Context, in io.Reader, src fs.ObjectInfo, chunkSize fs.SizeSuffix) error {
	uploadLink, err := o.fs.getUploadLink(ctx, o.libraryID)
	if err != nil {
		return err
	}
	size := src.Size()
	contentRange := newChunkedContentRange(int64(chunkSize), size)
	for {
		fs.Debugf(nil, "uploading chunk %s", contentRange.getContentRangeHeader())
		err = o.fs.uploadChunk(ctx, in, uploadLink, o.pathInLibrary, contentRange)
		if err != nil {
			return err
		}
		contentRange.next()
		// the last part is a slightly different API call
		if contentRange.isLastChunk() {
			break
		}
	}
	fs.Debugf(nil, "uploading last chunk %s", contentRange.getContentRangeHeader())
	uploaded, err := o.fs.uploadLastChunk(ctx, in, uploadLink, o.pathInLibrary, contentRange)
	if err != nil {
		return err
	}
	// Set the properties from the upload back to the object
	o.size = uploaded.Size
	o.id = uploaded.ID

	return nil
}

// Remove this object
func (o *Object) Remove(ctx context.Context) error {
	return o.fs.deleteFile(ctx, o.libraryID, o.pathInLibrary)
}

// ==================== Optional Interface fs.IDer ====================

// ID returns the ID of the Object if known, or "" if not
func (o *Object) ID() string {
	return o.id
}
