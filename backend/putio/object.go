package putio

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"time"

	"github.com/putdotio/go-putio/putio"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/fserrors"
	"github.com/rclone/rclone/fs/hash"
)

// Object describes a Putio object
//
// Putio Objects always have full metadata
type Object struct {
	fs      *Fs // what this object is part of
	file    *putio.File
	remote  string // The remote path
	modtime time.Time
}

// NewObject finds the Object at remote.  If it can't be found
// it returns the error fs.ErrorObjectNotFound.
func (f *Fs) NewObject(ctx context.Context, remote string) (o fs.Object, err error) {
	// defer log.Trace(f, "remote=%v", remote)("o=%+v, err=%v", &o, &err)
	obj := &Object{
		fs:     f,
		remote: remote,
	}
	err = obj.readEntryAndSetMetadata(ctx)
	if err != nil {
		return nil, err
	}
	return obj, err
}

// Return an Object from a path
//
// If it can't be found it returns the error fs.ErrorObjectNotFound.
func (f *Fs) newObjectWithInfo(ctx context.Context, remote string, info putio.File) (o fs.Object, err error) {
	// defer log.Trace(f, "remote=%v, info=+v", remote, &info)("o=%+v, err=%v", &o, &err)
	obj := &Object{
		fs:     f,
		remote: remote,
	}
	err = obj.setMetadataFromEntry(info)
	if err != nil {
		return nil, err
	}
	return obj, err
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

// Hash returns the dropbox special hash
func (o *Object) Hash(ctx context.Context, t hash.Type) (string, error) {
	if t != hash.CRC32 {
		return "", hash.ErrUnsupported
	}
	err := o.readEntryAndSetMetadata(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to read hash from metadata: %w", err)
	}
	return o.file.CRC32, nil
}

// Size returns the size of an object in bytes
func (o *Object) Size() int64 {
	if o.file == nil {
		return 0
	}
	return o.file.Size
}

// ID returns the ID of the Object if known, or "" if not
func (o *Object) ID() string {
	if o.file == nil {
		return ""
	}
	return itoa(o.file.ID)
}

// MimeType returns the content type of the Object if
// known, or "" if not
func (o *Object) MimeType(ctx context.Context) string {
	err := o.readEntryAndSetMetadata(ctx)
	if err != nil {
		return ""
	}
	return o.file.ContentType
}

// setMetadataFromEntry sets the fs data from a putio.File
//
// This isn't a complete set of metadata and has an inaccurate date
func (o *Object) setMetadataFromEntry(info putio.File) error {
	o.file = &info
	o.modtime = info.UpdatedAt.Time
	return nil
}

// Reads the entry for a file from putio
func (o *Object) readEntry(ctx context.Context) (f *putio.File, err error) {
	// defer log.Trace(o, "")("f=%+v, err=%v", f, &err)
	leaf, directoryID, err := o.fs.dirCache.FindPath(ctx, o.remote, false)
	if err != nil {
		if err == fs.ErrorDirNotFound {
			return nil, fs.ErrorObjectNotFound
		}
		return nil, err
	}
	var resp struct {
		File putio.File `json:"file"`
	}
	err = o.fs.pacer.Call(func() (bool, error) {
		// fs.Debugf(o, "requesting child. directoryID: %s, name: %s", directoryID, leaf)
		req, err := o.fs.client.NewRequest(ctx, "GET", "/v2/files/"+directoryID+"/child?name="+url.QueryEscape(o.fs.opt.Enc.FromStandardName(leaf)), nil)
		if err != nil {
			return false, err
		}
		_, err = o.fs.client.Do(req, &resp)
		if perr, ok := err.(*putio.ErrorResponse); ok && perr.Response.StatusCode == 404 {
			return false, fs.ErrorObjectNotFound
		}
		return shouldRetry(ctx, err)
	})
	if err != nil {
		return nil, err
	}
	if resp.File.IsDir() {
		return nil, fs.ErrorIsDir
	}
	return &resp.File, err
}

// Read entry if not set and set metadata from it
func (o *Object) readEntryAndSetMetadata(ctx context.Context) error {
	if o.file != nil {
		return nil
	}
	entry, err := o.readEntry(ctx)
	if err != nil {
		return err
	}
	return o.setMetadataFromEntry(*entry)
}

// Returns the remote path for the object
func (o *Object) remotePath() string {
	return path.Join(o.fs.root, o.remote)
}

// ModTime returns the modification time of the object
//
// It attempts to read the objects mtime and if that isn't present the
// LastModified returned in the http headers
func (o *Object) ModTime(ctx context.Context) time.Time {
	if o.modtime.IsZero() {
		err := o.readEntryAndSetMetadata(ctx)
		if err != nil {
			fs.Debugf(o, "Failed to read metadata: %v", err)
			return time.Now()
		}
	}
	return o.modtime
}

// SetModTime sets the modification time of the local fs object
//
// Commits the datastore
func (o *Object) SetModTime(ctx context.Context, modTime time.Time) (err error) {
	// defer log.Trace(o, "modTime=%v", modTime.String())("err=%v", &err)
	req, err := o.fs.client.NewRequest(ctx, "POST", "/v2/files/touch?file_id="+strconv.FormatInt(o.file.ID, 10)+"&updated_at="+url.QueryEscape(modTime.Format(time.RFC3339)), nil)
	if err != nil {
		return err
	}
	// fs.Debugf(o, "setting modtime: %s", modTime.String())
	_, err = o.fs.client.Do(req, nil)
	if err != nil {
		return err
	}
	o.modtime = modTime
	if o.file != nil {
		o.file.UpdatedAt.Time = modTime
	}
	return nil
}

// Storable returns whether this object is storable
func (o *Object) Storable() bool {
	return true
}

// Open an object for read
func (o *Object) Open(ctx context.Context, options ...fs.OpenOption) (in io.ReadCloser, err error) {
	// defer log.Trace(o, "")("err=%v", &err)
	var storageURL string
	err = o.fs.pacer.Call(func() (bool, error) {
		storageURL, err = o.fs.client.Files.URL(ctx, o.file.ID, true)
		return shouldRetry(ctx, err)
	})
	if err != nil {
		return
	}

	var resp *http.Response
	headers := fs.OpenOptionHeaders(options)
	err = o.fs.pacer.Call(func() (bool, error) {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, storageURL, nil)
		if err != nil {
			return shouldRetry(ctx, err)
		}
		req.Header.Set("User-Agent", o.fs.client.UserAgent)

		// merge headers with extra headers
		for header, value := range headers {
			req.Header.Set(header, value)
		}
		// fs.Debugf(o, "opening file: id=%d", o.file.ID)
		resp, err = o.fs.httpClient.Do(req)
		if err != nil {
			return shouldRetry(ctx, err)
		}
		if err := checkStatusCode(resp, 200, 206); err != nil {
			return shouldRetry(ctx, err)
		}
		return false, nil
	})
	if perr, ok := err.(*putio.ErrorResponse); ok && perr.Response.StatusCode >= 400 && perr.Response.StatusCode <= 499 {
		_ = resp.Body.Close()
		return nil, fserrors.NoRetryError(err)
	}
	if err != nil {
		return nil, err
	}
	return resp.Body, nil
}

// Update the already existing object
//
// Copy the reader into the object updating modTime and size.
//
// The new object may have been created if an error is returned
func (o *Object) Update(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (err error) {
	// defer log.Trace(o, "src=%+v", src)("err=%v", &err)
	remote := o.remotePath()
	if ignoredFiles.MatchString(remote) {
		fs.Logf(o, "File name disallowed - not uploading")
		return nil
	}
	err = o.Remove(ctx)
	if err != nil {
		return err
	}
	newObj, err := o.fs.putUnchecked(ctx, in, src, o.remote, options...)
	if err != nil {
		return err
	}
	*o = *(newObj.(*Object))
	return err
}

// Remove an object
func (o *Object) Remove(ctx context.Context) (err error) {
	// defer log.Trace(o, "")("err=%v", &err)
	return o.fs.pacer.Call(func() (bool, error) {
		// fs.Debugf(o, "removing file: id=%d", o.file.ID)
		err = o.fs.client.Files.Delete(ctx, o.file.ID)
		return shouldRetry(ctx, err)
	})
}
