package webdav

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/lib/rest"
)

// Uploader holds all information about a currently running upload
type Uploader struct {
	fs                  *Fs
	url                 string
	upload              *Upload
	offset              int64
	aborted             bool
	uploadSubs          []chan Upload
	notifyChan          chan bool
	overridePatchMethod bool
}

// NotifyUploadProgress subscribes to progress updates.
func (u *Uploader) NotifyUploadProgress(c chan Upload) {
	u.uploadSubs = append(u.uploadSubs, c)
}

func (f *Fs) shouldRetryChunk(ctx context.Context, resp *http.Response, err error, newOff *int64) (bool, error) {
	if resp == nil {
		return true, err
	}

	switch resp.StatusCode {
	case 204:
		if off, err := strconv.ParseInt(resp.Header.Get("Upload-Offset"), 10, 64); err == nil {
			*newOff = off
			return false, nil
		}
		return false, err

	case 409:
		return false, ErrOffsetMismatch
	case 412:
		return false, ErrVersionMismatch
	case 413:
		return false, ErrLargeUpload
	}

	return f.shouldRetry(ctx, resp, err)
}

func (u *Uploader) uploadChunk(ctx context.Context, body io.Reader, size int64, offset int64, options ...fs.OpenOption) (int64, error) {
	var method string

	if !u.overridePatchMethod {
		method = "PATCH"
	} else {
		method = "POST"
	}

	extraHeaders := map[string]string{} // FIXME: Use extraHeaders(ctx, src) from Object maybe?
	extraHeaders["Upload-Offset"] = strconv.FormatInt(offset, 10)
	extraHeaders["Tus-Resumable"] = "1.0.0"
	extraHeaders["filetype"] = u.upload.Metadata["filetype"]
	if u.overridePatchMethod {
		extraHeaders["X-HTTP-Method-Override"] = "PATCH"
	}

	url, err := url.Parse(u.url)
	if err != nil {
		return 0, fmt.Errorf("upload Chunk failed, could not parse url")
	}

	// FIXME: Use GetBody func as in chunking.go
	opts := rest.Opts{
		Method:        method,
		Path:          url.Path,
		NoResponse:    true,
		RootURL:       fmt.Sprintf("%s://%s", url.Scheme, url.Host),
		ContentLength: &size,
		Body:          body,
		ContentType:   "application/offset+octet-stream",
		ExtraHeaders:  extraHeaders,
		Options:       options,
	}

	var newOffset int64

	err = u.fs.pacer.CallNoRetry(func() (bool, error) {
		res, err := u.fs.srv.Call(ctx, &opts)
		return u.fs.shouldRetryChunk(ctx, res, err, &newOffset)
	})
	if err != nil {
		return 0, fmt.Errorf("uploadChunk failed: %w", err)
		// FIXME What do we do here? Remove the entire upload?
		// See https://github.com/tus/tusd/issues/176
	}

	return newOffset, nil
}

// Upload uploads the entire body to the server.
func (u *Uploader) Upload(ctx context.Context, options ...fs.OpenOption) error {
	cnt := 1

	fs.Debug(u.fs, "Uploaded starts")
	for u.offset < u.upload.size && !u.aborted {
		err := u.UploadChunk(ctx, cnt, options...)
		cnt++
		if err != nil {
			return err
		}
	}
	fs.Debug(u.fs, "-- Uploaded finished")

	return nil
}

// UploadChunk uploads a single chunk.
func (u *Uploader) UploadChunk(ctx context.Context, cnt int, options ...fs.OpenOption) error {
	chunkSize := u.fs.opt.ChunkSize
	data := make([]byte, chunkSize)

	_, err := u.upload.stream.Seek(u.offset, 0)

	if err != nil {
		fs.Errorf(u.fs, "Chunk %d: Error seek in stream failed: %v", cnt, err)
		return err
	}

	size, err := u.upload.stream.Read(data)

	if err != nil {
		fs.Errorf(u.fs, "Chunk %d: Error: Can not read from data stream: %v", cnt, err)
		return err
	}

	body := bytes.NewBuffer(data[:size])

	newOffset, err := u.uploadChunk(ctx, body, int64(size), u.offset, options...)

	if err == nil {
		fs.Debugf(u.fs, "Uploaded chunk no %d ok, range %d -> %d", cnt, u.offset, newOffset)
	} else {
		fs.Errorf(u.fs, "Uploaded chunk no %d failed: %v", cnt, err)

		return err
	}

	u.offset = newOffset

	u.upload.updateProgress(u.offset)

	u.notifyChan <- true

	return nil
}

// Waits for a signal to broadcast to all subscribers
func (u *Uploader) broadcastProgress() {
	for range u.notifyChan {
		for _, c := range u.uploadSubs {
			c <- *u.upload
		}
	}
}

// NewUploader creates a new Uploader.
func NewUploader(f *Fs, url string, upload *Upload, offset int64) *Uploader {
	notifyChan := make(chan bool)

	uploader := &Uploader{
		f,
		url,
		upload,
		offset,
		false,
		nil,
		notifyChan,
		false,
	}

	go uploader.broadcastProgress()

	return uploader
}
