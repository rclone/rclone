// Upload large files for b2
//
// Docs - https://www.backblaze.com/b2/docs/large_files.html

package b2

import (
	"bytes"
	"context"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	gohash "hash"
	"io"
	"strings"
	"sync"

	"github.com/pkg/errors"
	"github.com/rclone/rclone/backend/b2/api"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/accounting"
	"github.com/rclone/rclone/fs/hash"
	"github.com/rclone/rclone/lib/rest"
)

type hashAppendingReader struct {
	h         gohash.Hash
	in        io.Reader
	hexSum    string
	hexReader io.Reader
}

// Read returns bytes all bytes from the original reader, then the hex sum
// of what was read so far, then EOF.
func (har *hashAppendingReader) Read(b []byte) (int, error) {
	if har.hexReader == nil {
		n, err := har.in.Read(b)
		if err == io.EOF {
			har.in = nil // allow GC
			err = nil    // allow reading hexSum before EOF

			har.hexSum = hex.EncodeToString(har.h.Sum(nil))
			har.hexReader = strings.NewReader(har.hexSum)
		}
		return n, err
	}
	return har.hexReader.Read(b)
}

// AdditionalLength returns how many bytes the appended hex sum will take up.
func (har *hashAppendingReader) AdditionalLength() int {
	return hex.EncodedLen(har.h.Size())
}

// HexSum returns the hash sum as hex. It's only available after the original
// reader has EOF'd. It's an empty string before that.
func (har *hashAppendingReader) HexSum() string {
	return har.hexSum
}

// newHashAppendingReader takes a Reader and a Hash and will append the hex sum
// after the original reader reaches EOF. The increased size depends on the
// given hash, which may be queried through AdditionalLength()
func newHashAppendingReader(in io.Reader, h gohash.Hash) *hashAppendingReader {
	withHash := io.TeeReader(in, h)
	return &hashAppendingReader{h: h, in: withHash}
}

// largeUpload is used to control the upload of large files which need chunking
type largeUpload struct {
	f        *Fs                             // parent Fs
	o        *Object                         // object being uploaded
	in       io.Reader                       // read the data from here
	wrap     accounting.WrapFn               // account parts being transferred
	id       string                          // ID of the file being uploaded
	size     int64                           // total size
	parts    int64                           // calculated number of parts, if known
	sha1s    []string                        // slice of SHA1s for each part
	uploadMu sync.Mutex                      // lock for upload variable
	uploads  []*api.GetUploadPartURLResponse // result of get upload URL calls
}

// newLargeUpload starts an upload of object o from in with metadata in src
func (f *Fs) newLargeUpload(ctx context.Context, o *Object, in io.Reader, src fs.ObjectInfo) (up *largeUpload, err error) {
	remote := o.remote
	size := src.Size()
	parts := int64(0)
	sha1SliceSize := int64(maxParts)
	if size == -1 {
		fs.Debugf(o, "Streaming upload with --b2-chunk-size %s allows uploads of up to %s and will fail only when that limit is reached.", f.opt.ChunkSize, maxParts*f.opt.ChunkSize)
	} else {
		parts = size / int64(o.fs.opt.ChunkSize)
		if size%int64(o.fs.opt.ChunkSize) != 0 {
			parts++
		}
		if parts > maxParts {
			return nil, errors.Errorf("%q too big (%d bytes) makes too many parts %d > %d - increase --b2-chunk-size", remote, size, parts, maxParts)
		}
		sha1SliceSize = parts
	}

	modTime := src.ModTime(ctx)
	opts := rest.Opts{
		Method: "POST",
		Path:   "/b2_start_large_file",
	}
	bucket, bucketPath := o.split()
	bucketID, err := f.getBucketID(ctx, bucket)
	if err != nil {
		return nil, err
	}
	var request = api.StartLargeFileRequest{
		BucketID:    bucketID,
		Name:        f.opt.Enc.FromStandardPath(bucketPath),
		ContentType: fs.MimeType(ctx, src),
		Info: map[string]string{
			timeKey: timeString(modTime),
		},
	}
	// Set the SHA1 if known
	if !o.fs.opt.DisableCheckSum {
		if calculatedSha1, err := src.Hash(ctx, hash.SHA1); err == nil && calculatedSha1 != "" {
			request.Info[sha1Key] = calculatedSha1
		}
	}
	var response api.StartLargeFileResponse
	err = f.pacer.Call(func() (bool, error) {
		resp, err := f.srv.CallJSON(ctx, &opts, &request, &response)
		return f.shouldRetry(ctx, resp, err)
	})
	if err != nil {
		return nil, err
	}
	// unwrap the accounting from the input, we use wrap to put it
	// back on after the buffering
	in, wrap := accounting.UnWrap(in)
	up = &largeUpload{
		f:     f,
		o:     o,
		in:    in,
		wrap:  wrap,
		id:    response.ID,
		size:  size,
		parts: parts,
		sha1s: make([]string, sha1SliceSize),
	}
	return up, nil
}

// getUploadURL returns the upload info with the UploadURL and the AuthorizationToken
//
// This should be returned with returnUploadURL when finished
func (up *largeUpload) getUploadURL(ctx context.Context) (upload *api.GetUploadPartURLResponse, err error) {
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
			resp, err := up.f.srv.CallJSON(ctx, &opts, &request, &upload)
			return up.f.shouldRetry(ctx, resp, err)
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

// Transfer a chunk
func (up *largeUpload) transferChunk(ctx context.Context, part int64, body []byte) error {
	err := up.f.pacer.Call(func() (bool, error) {
		fs.Debugf(up.o, "Sending chunk %d length %d", part, len(body))

		// Get upload URL
		upload, err := up.getUploadURL(ctx)
		if err != nil {
			return false, err
		}

		in := newHashAppendingReader(bytes.NewReader(body), sha1.New())
		size := int64(len(body)) + int64(in.AdditionalLength())

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
			Method:  "POST",
			RootURL: upload.UploadURL,
			Body:    up.wrap(in),
			ExtraHeaders: map[string]string{
				"Authorization":    upload.AuthorizationToken,
				"X-Bz-Part-Number": fmt.Sprintf("%d", part),
				sha1Header:         "hex_digits_at_end",
			},
			ContentLength: &size,
		}

		var response api.UploadPartResponse

		resp, err := up.f.srv.CallJSON(ctx, &opts, nil, &response)
		retry, err := up.f.shouldRetry(ctx, resp, err)
		if err != nil {
			fs.Debugf(up.o, "Error sending chunk %d (retry=%v): %v: %#v", part, retry, err, err)
		}
		// On retryable error clear PartUploadURL
		if retry {
			fs.Debugf(up.o, "Clearing part upload URL because of error: %v", err)
			upload = nil
		}
		up.returnUploadURL(upload)
		up.sha1s[part-1] = in.HexSum()
		return retry, err
	})
	if err != nil {
		fs.Debugf(up.o, "Error sending chunk %d: %v", part, err)
	} else {
		fs.Debugf(up.o, "Done sending chunk %d", part)
	}
	return err
}

// finish closes off the large upload
func (up *largeUpload) finish(ctx context.Context) error {
	fs.Debugf(up.o, "Finishing large file upload with %d parts", up.parts)
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
		resp, err := up.f.srv.CallJSON(ctx, &opts, &request, &response)
		return up.f.shouldRetry(ctx, resp, err)
	})
	if err != nil {
		return err
	}
	return up.o.decodeMetaDataFileInfo(&response)
}

// cancel aborts the large upload
func (up *largeUpload) cancel(ctx context.Context) error {
	opts := rest.Opts{
		Method: "POST",
		Path:   "/b2_cancel_large_file",
	}
	var request = api.CancelLargeFileRequest{
		ID: up.id,
	}
	var response api.CancelLargeFileResponse
	err := up.f.pacer.Call(func() (bool, error) {
		resp, err := up.f.srv.CallJSON(ctx, &opts, &request, &response)
		return up.f.shouldRetry(ctx, resp, err)
	})
	return err
}

func (up *largeUpload) managedTransferChunk(ctx context.Context, wg *sync.WaitGroup, errs chan error, part int64, buf []byte) {
	wg.Add(1)
	go func(part int64, buf []byte) {
		defer wg.Done()
		defer up.f.putUploadBlock(buf)
		err := up.transferChunk(ctx, part, buf)
		if err != nil {
			select {
			case errs <- err:
			default:
			}
		}
	}(part, buf)
}

func (up *largeUpload) finishOrCancelOnError(ctx context.Context, err error, errs chan error) error {
	if err == nil {
		select {
		case err = <-errs:
		default:
		}
	}
	if err != nil {
		fs.Debugf(up.o, "Cancelling large file upload due to error: %v", err)
		cancelErr := up.cancel(ctx)
		if cancelErr != nil {
			fs.Errorf(up.o, "Failed to cancel large file upload: %v", cancelErr)
		}
		return err
	}
	return up.finish(ctx)
}

// Stream uploads the chunks from the input, starting with a required initial
// chunk. Assumes the file size is unknown and will upload until the input
// reaches EOF.
func (up *largeUpload) Stream(ctx context.Context, initialUploadBlock []byte) (err error) {
	fs.Debugf(up.o, "Starting streaming of large file (id %q)", up.id)
	errs := make(chan error, 1)
	hasMoreParts := true
	var wg sync.WaitGroup

	// Transfer initial chunk
	up.size = int64(len(initialUploadBlock))
	up.managedTransferChunk(ctx, &wg, errs, 1, initialUploadBlock)

outer:
	for part := int64(2); hasMoreParts; part++ {
		// Check any errors
		select {
		case err = <-errs:
			break outer
		default:
		}

		// Get a block of memory
		buf := up.f.getUploadBlock()

		// Read the chunk
		var n int
		n, err = io.ReadFull(up.in, buf)
		if err == io.ErrUnexpectedEOF {
			fs.Debugf(up.o, "Read less than a full chunk, making this the last one.")
			buf = buf[:n]
			hasMoreParts = false
			err = nil
		} else if err == io.EOF {
			fs.Debugf(up.o, "Could not read any more bytes, previous chunk was the last.")
			up.f.putUploadBlock(buf)
			err = nil
			break outer
		} else if err != nil {
			// other kinds of errors indicate failure
			up.f.putUploadBlock(buf)
			break outer
		}

		// Keep stats up to date
		up.parts = part
		up.size += int64(n)
		if part > maxParts {
			err = errors.Errorf("%q too big (%d bytes so far) makes too many parts %d > %d - increase --b2-chunk-size", up.o, up.size, up.parts, maxParts)
			break outer
		}

		// Transfer the chunk
		up.managedTransferChunk(ctx, &wg, errs, part, buf)
	}
	wg.Wait()
	up.sha1s = up.sha1s[:up.parts]

	return up.finishOrCancelOnError(ctx, err, errs)
}

// Upload uploads the chunks from the input
func (up *largeUpload) Upload(ctx context.Context) error {
	fs.Debugf(up.o, "Starting upload of large file in %d chunks (id %q)", up.parts, up.id)
	remaining := up.size
	errs := make(chan error, 1)
	var wg sync.WaitGroup
	var err error
outer:
	for part := int64(1); part <= up.parts; part++ {
		// Check any errors
		select {
		case err = <-errs:
			break outer
		default:
		}

		reqSize := remaining
		if reqSize >= int64(up.f.opt.ChunkSize) {
			reqSize = int64(up.f.opt.ChunkSize)
		}

		// Get a block of memory
		buf := up.f.getUploadBlock()[:reqSize]

		// Read the chunk
		_, err = io.ReadFull(up.in, buf)
		if err != nil {
			up.f.putUploadBlock(buf)
			break outer
		}

		// Transfer the chunk
		up.managedTransferChunk(ctx, &wg, errs, part, buf)
		remaining -= reqSize
	}
	wg.Wait()

	return up.finishOrCancelOnError(ctx, err, errs)
}
