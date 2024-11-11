package webdav

/*
   Chunked upload based on the tus protocol for ownCloud Infinite Scale
   See https://tus.io/protocols/resumable-upload
*/

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strconv"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/lib/rest"
)

// set the chunk size for testing
func (f *Fs) setUploadTusSize(cs fs.SizeSuffix) (old fs.SizeSuffix, err error) {
	old, f.opt.ChunkSize = f.opt.ChunkSize, cs
	return
}

func (o *Object) updateViaTus(ctx context.Context, in io.Reader, contentType string, src fs.ObjectInfo, options ...fs.OpenOption) (err error) {

	fn := filepath.Base(src.Remote())
	metadata := map[string]string{
		"filename": fn,
		"mtime":    strconv.FormatInt(src.ModTime(ctx).Unix(), 10),
		"filetype": contentType,
	}

	// Fingerprint is used to identify the upload when resuming. That is not yet implemented
	fingerprint := ""

	// create an upload from a file.
	upload := NewUpload(in, src.Size(), metadata, fingerprint)

	// create the uploader.
	uploader, err := o.CreateUploader(ctx, upload, options...)
	if err == nil {
		// start the uploading process.
		err = uploader.Upload(ctx, options...)
	}

	return err
}

func (f *Fs) shouldRetryCreateUpload(ctx context.Context, resp *http.Response, err error) (bool, error) {

	switch resp.StatusCode {
	case 201:
		location := resp.Header.Get("Location")
		f.chunksUploadURL = location
		return false, nil
	case 412:
		return false, ErrVersionMismatch
	case 413:
		return false, ErrLargeUpload
	}

	return f.shouldRetry(ctx, resp, err)
}

// CreateUpload creates a new upload to the server.
func (o *Object) CreateUploader(ctx context.Context, u *Upload, options ...fs.OpenOption) (*Uploader, error) {
	if u == nil {
		return nil, ErrNilUpload
	}

	// if c.Config.Resume && len(u.Fingerprint) == 0 {
	//		return nil, ErrFingerprintNotSet
	//	}

	l := int64(0)
	p := o.filePath()
	// cut the filename off
	dir, _ := filepath.Split(p)
	if dir == "" {
		dir = "/"
	}

	opts := rest.Opts{
		Method:        "POST",
		Path:          dir,
		NoResponse:    true,
		RootURL:       o.fs.endpointURL,
		ContentLength: &l,
		ExtraHeaders:  o.extraHeaders(ctx, o),
		Options:       options,
	}
	opts.ExtraHeaders["Upload-Length"] = strconv.FormatInt(u.size, 10)
	opts.ExtraHeaders["Upload-Metadata"] = u.EncodedMetadata()
	opts.ExtraHeaders["Tus-Resumable"] = "1.0.0"
	// opts.ExtraHeaders["mtime"] = strconv.FormatInt(src.ModTime(ctx).Unix(), 10)

	// rclone http call
	err := o.fs.pacer.CallNoRetry(func() (bool, error) {
		res, err := o.fs.srv.Call(ctx, &opts)
		return o.fs.shouldRetryCreateUpload(ctx, res, err)
	})
	if err != nil {
		return nil, fmt.Errorf("making upload directory failed: %w", err)
	}

	uploader := NewUploader(o.fs, o.fs.chunksUploadURL, u, 0)

	return uploader, nil
}
