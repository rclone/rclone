// Upload large files for b2
//
// Docs - https://www.backblaze.com/b2/docs/large_files.html

package b2

import (
	"bytes"
	"crypto/sha1"
	"fmt"
	"io"
	"sync"

	"github.com/ncw/rclone/b2/api"
	"github.com/ncw/rclone/fs"
	"github.com/ncw/rclone/rest"
	"github.com/pkg/errors"
)

// largeUpload is used to control the upload of large files which need chunking
type largeUpload struct {
	f        *Fs                             // parent Fs
	o        *Object                         // object being uploaded
	in       io.Reader                       // read the data from here
	id       string                          // ID of the file being uploaded
	size     int64                           // total size
	parts    int64                           // calculated number of parts
	sha1s    []string                        // slice of SHA1s for each part
	uploadMu sync.Mutex                      // lock for upload variable
	uploads  []*api.GetUploadPartURLResponse // result of get upload URL calls
}

// newLargeUpload starts an upload of object o from in with metadata in src
func (f *Fs) newLargeUpload(o *Object, in io.Reader, src fs.ObjectInfo) (up *largeUpload, err error) {
	remote := o.remote
	size := src.Size()
	parts := size / int64(chunkSize)
	if size%int64(chunkSize) != 0 {
		parts++
	}
	if parts > maxParts {
		return nil, errors.Errorf("%q too big (%d bytes) makes too many parts %d > %d - increase --b2-chunk-size", remote, size, parts, maxParts)
	}
	modTime := src.ModTime()
	opts := rest.Opts{
		Method: "POST",
		Path:   "/b2_start_large_file",
	}
	bucketID, err := f.getBucketID()
	if err != nil {
		return nil, err
	}
	var request = api.StartLargeFileRequest{
		BucketID:    bucketID,
		Name:        o.fs.root + remote,
		ContentType: fs.MimeType(src),
		Info: map[string]string{
			timeKey: timeString(modTime),
		},
	}
	// Set the SHA1 if known
	if calculatedSha1, err := src.Hash(fs.HashSHA1); err == nil && calculatedSha1 != "" {
		request.Info[sha1Key] = calculatedSha1
	}
	var response api.StartLargeFileResponse
	err = f.pacer.Call(func() (bool, error) {
		resp, err := f.srv.CallJSON(&opts, &request, &response)
		return f.shouldRetry(resp, err)
	})
	if err != nil {
		return nil, err
	}
	up = &largeUpload{
		f:     f,
		o:     o,
		in:    in,
		id:    response.ID,
		size:  size,
		parts: parts,
		sha1s: make([]string, parts),
	}
	return up, nil
}

// getUploadURL returns the upload info with the UploadURL and the AuthorizationToken
//
// This should be returned with returnUploadURL when finished
func (up *largeUpload) getUploadURL() (upload *api.GetUploadPartURLResponse, err error) {
	up.uploadMu.Lock()
	defer up.uploadMu.Unlock()
	if len(up.uploads) == 0 {
		opts := rest.Opts{
			Method: "POST",
			Path:   "/b2_get_upload_part_url",
		}
		var request = api.GetUploadPartURLRequest{
			ID: up.id,
		}
		err := up.f.pacer.Call(func() (bool, error) {
			resp, err := up.f.srv.CallJSON(&opts, &request, &upload)
			return up.f.shouldRetry(resp, err)
		})
		if err != nil {
			return nil, errors.Wrap(err, "failed to get upload URL")
		}
	} else {
		upload, up.uploads = up.uploads[0], up.uploads[1:]
	}
	return upload, nil
}

// returnUploadURL returns the UploadURL to the cache
func (up *largeUpload) returnUploadURL(upload *api.GetUploadPartURLResponse) {
	if upload == nil {
		return
	}
	up.uploadMu.Lock()
	up.uploads = append(up.uploads, upload)
	up.uploadMu.Unlock()
}

// clearUploadURL clears the current UploadURL and the AuthorizationToken
func (up *largeUpload) clearUploadURL() {
	up.uploadMu.Lock()
	up.uploads = nil
	up.uploadMu.Unlock()
}

// Transfer a chunk
func (up *largeUpload) transferChunk(part int64, body []byte) error {
	calculatedSHA1 := fmt.Sprintf("%x", sha1.Sum(body))
	up.sha1s[part-1] = calculatedSHA1
	size := int64(len(body))
	err := up.f.pacer.Call(func() (bool, error) {
		fs.Debug(up.o, "Sending chunk %d length %d", part, len(body))

		// Get upload URL
		upload, err := up.getUploadURL()
		if err != nil {
			return false, err
		}

		// Authorization
		//
		// An upload authorization token, from b2_get_upload_part_url.
		//
		// X-Bz-Part-Number
		//
		// A number from 1 to 10000. The parts uploaded for one file
		// must have contiguous numbers, starting with 1.
		//
		// Content-Length
		//
		// The number of bytes in the file being uploaded. Note that
		// this header is required; you cannot leave it out and just
		// use chunked encoding.  The minimum size of every part but
		// the last one is 100MB.
		//
		// X-Bz-Content-Sha1
		//
		// The SHA1 checksum of the this part of the file. B2 will
		// check this when the part is uploaded, to make sure that the
		// data arrived correctly.  The same SHA1 checksum must be
		// passed to b2_finish_large_file.
		opts := rest.Opts{
			Method:   "POST",
			Absolute: true,
			Path:     upload.UploadURL,
			Body:     fs.AccountPart(up.o, bytes.NewBuffer(body)),
			ExtraHeaders: map[string]string{
				"Authorization":    upload.AuthorizationToken,
				"X-Bz-Part-Number": fmt.Sprintf("%d", part),
				sha1Header:         calculatedSHA1,
			},
			ContentLength: &size,
		}

		var response api.UploadPartResponse

		resp, err := up.f.srv.CallJSON(&opts, nil, &response)
		retry, err := up.f.shouldRetry(resp, err)
		// On retryable error clear PartUploadURL
		if retry {
			fs.Debug(up.o, "Clearing part upload URL because of error: %v", err)
			upload = nil
		}
		up.returnUploadURL(upload)
		return retry, err
	})
	if err != nil {
		fs.Debug(up.o, "Error sending chunk %d: %v", part, err)
	} else {
		fs.Debug(up.o, "Done sending chunk %d", part)
	}
	return err
}

// finish closes off the large upload
func (up *largeUpload) finish() error {
	opts := rest.Opts{
		Method: "POST",
		Path:   "/b2_finish_large_file",
	}
	var request = api.FinishLargeFileRequest{
		ID:    up.id,
		SHA1s: up.sha1s,
	}
	var response api.FileInfo
	err := up.f.pacer.Call(func() (bool, error) {
		resp, err := up.f.srv.CallJSON(&opts, &request, &response)
		return up.f.shouldRetry(resp, err)
	})
	if err != nil {
		return err
	}
	return up.o.decodeMetaDataFileInfo(&response)
}

// cancel aborts the large upload
func (up *largeUpload) cancel() error {
	opts := rest.Opts{
		Method: "POST",
		Path:   "/b2_cancel_large_file",
	}
	var request = api.CancelLargeFileRequest{
		ID: up.id,
	}
	var response api.CancelLargeFileResponse
	err := up.f.pacer.Call(func() (bool, error) {
		resp, err := up.f.srv.CallJSON(&opts, &request, &response)
		return up.f.shouldRetry(resp, err)
	})
	return err
}

// Upload uploads the chunks from the input
func (up *largeUpload) Upload() error {
	fs.Debug(up.o, "Starting upload of large file in %d chunks (id %q)", up.parts, up.id)
	remaining := up.size
	errs := make(chan error, 1)
	var wg sync.WaitGroup
	var err error
	uploadCounter := up.f.newMultipartUploadCounter()
	defer uploadCounter.finished()
	fs.AccountByPart(up.o) // Cancel whole file accounting before reading
outer:
	for part := int64(1); part <= up.parts; part++ {
		// Check any errors
		select {
		case err = <-errs:
			break outer
		default:
		}

		reqSize := remaining
		if reqSize >= int64(chunkSize) {
			reqSize = int64(chunkSize)
		}

		// Read the chunk
		buf := make([]byte, reqSize)
		_, err = io.ReadFull(up.in, buf)
		if err != nil {
			break outer
		}

		// Transfer the chunk
		// Get upload Token
		token := uploadCounter.getMultipartUploadToken()
		wg.Add(1)
		go func(part int64, buf []byte, token bool) {
			defer uploadCounter.returnMultipartUploadToken(token)
			defer wg.Done()
			err := up.transferChunk(part, buf)
			if err != nil {
				select {
				case errs <- err:
				default:
				}
			}
		}(part, buf, token)

		remaining -= reqSize
	}
	wg.Wait()
	if err == nil {
		select {
		case err = <-errs:
		default:
		}
	}
	if err != nil {
		fs.Debug(up.o, "Cancelling large file upload due to error: %v", err)
		cancelErr := up.cancel()
		if cancelErr != nil {
			fs.ErrorLog(up.o, "Failed to cancel large file upload: %v", cancelErr)
		}
		return err
	}
	// Check any errors
	fs.Debug(up.o, "Finishing large file upload")
	return up.finish()
}
