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
	"github.com/rclone/rclone/lib/atexit"
	"github.com/rclone/rclone/lib/rest"
	"golang.org/x/sync/errgroup"
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
	f         *Fs                             // parent Fs
	o         *Object                         // object being uploaded
	doCopy    bool                            // doing copy rather than upload
	what      string                          // text name of operation for logs
	in        io.Reader                       // read the data from here
	wrap      accounting.WrapFn               // account parts being transferred
	id        string                          // ID of the file being uploaded
	size      int64                           // total size
	parts     int64                           // calculated number of parts, if known
	sha1s     []string                        // slice of SHA1s for each part
	uploadMu  sync.Mutex                      // lock for upload variable
	uploads   []*api.GetUploadPartURLResponse // result of get upload URL calls
	chunkSize int64                           // chunk size to use
	src       *Object                         // if copying, object we are reading from
}

// newLargeUpload starts an upload of object o from in with metadata in src
//
// If newInfo is set then metadata from that will be used instead of reading it from src
func (f *Fs) newLargeUpload(ctx context.Context, o *Object, in io.Reader, src fs.ObjectInfo, chunkSize fs.SizeSuffix, doCopy bool, newInfo *api.File) (up *largeUpload, err error) {
	remote := o.remote
	size := src.Size()
	parts := int64(0)
	sha1SliceSize := int64(maxParts)
	if size == -1 {
		fs.Debugf(o, "Streaming upload with --b2-chunk-size %s allows uploads of up to %s and will fail only when that limit is reached.", f.opt.ChunkSize, maxParts*f.opt.ChunkSize)
	} else {
		parts = size / int64(chunkSize)
		if size%int64(chunkSize) != 0 {
			parts++
		}
		if parts > maxParts {
			return nil, errors.Errorf("%q too big (%d bytes) makes too many parts %d > %d - increase --b2-chunk-size", remote, size, parts, maxParts)
		}
		sha1SliceSize = parts
	}

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
		BucketID: bucketID,
		Name:     f.opt.Enc.FromStandardPath(bucketPath),
	}
	if newInfo == nil {
		modTime := src.ModTime(ctx)
		request.ContentType = fs.MimeType(ctx, src)
		request.Info = map[string]string{
			timeKey: timeString(modTime),
		}
		// Set the SHA1 if known
		if !o.fs.opt.DisableCheckSum || doCopy {
			if calculatedSha1, err := src.Hash(ctx, hash.SHA1); err == nil && calculatedSha1 != "" {
				request.Info[sha1Key] = calculatedSha1
			}
		}
	} else {
		request.ContentType = newInfo.ContentType
		request.Info = newInfo.Info
	}
	var response api.StartLargeFileResponse
	err = f.pacer.Call(func() (bool, error) {
		resp, err := f.srv.CallJSON(ctx, &opts, &request, &response)
		return f.shouldRetry(ctx, resp, err)
	})
	if err != nil {
		return nil, err
	}
	up = &largeUpload{
		f:         f,
		o:         o,
		doCopy:    doCopy,
		what:      "upload",
		id:        response.ID,
		size:      size,
		parts:     parts,
		sha1s:     make([]string, sha1SliceSize),
		chunkSize: int64(chunkSize),
	}
	// unwrap the accounting from the input, we use wrap to put it
	// back on after the buffering
	if doCopy {
		up.what = "copy"
		up.src = src.(*Object)
	} else {
		up.in, up.wrap = accounting.UnWrap(in)
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
		// use chunked encoding. The minimum size of every part but
		// the last one is 100 MB (100,000,000 bytes)
		//
		// X-Bz-Content-Sha1
		//
		// The SHA1 checksum of the this part of the file. B2 will
		// check this when the part is uploaded, to make sure that the
		// data arrived correctly. The same SHA1 checksum must be
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

// Copy a chunk
func (up *largeUpload) copyChunk(ctx context.Context, part int64, partSize int64) error {
	err := up.f.pacer.Call(func() (bool, error) {
		fs.Debugf(up.o, "Copying chunk %d length %d", part, partSize)
		opts := rest.Opts{
			Method: "POST",
			Path:   "/b2_copy_part",
		}
		offset := (part - 1) * up.chunkSize // where we are in the source file
		var request = api.CopyPartRequest{
			SourceID:    up.src.id,
			LargeFileID: up.id,
			PartNumber:  part,
			Range:       fmt.Sprintf("bytes=%d-%d", offset, offset+partSize-1),
		}
		var response api.UploadPartResponse
		resp, err := up.f.srv.CallJSON(ctx, &opts, &request, &response)
		retry, err := up.f.shouldRetry(ctx, resp, err)
		if err != nil {
			fs.Debugf(up.o, "Error copying chunk %d (retry=%v): %v: %#v", part, retry, err, err)
		}
		up.sha1s[part-1] = response.SHA1
		return retry, err
	})
	if err != nil {
		fs.Debugf(up.o, "Error copying chunk %d: %v", part, err)
	} else {
		fs.Debugf(up.o, "Done copying chunk %d", part)
	}
	return err
}

// finish closes off the large upload
func (up *largeUpload) finish(ctx context.Context) error {
	fs.Debugf(up.o, "Finishing large file %s with %d parts", up.what, up.parts)
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
	fs.Debugf(up.o, "Cancelling large file %s", up.what)
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
	if err != nil {
		fs.Errorf(up.o, "Failed to cancel large file %s: %v", up.what, err)
	}
	return err
}

// Stream uploads the chunks from the input, starting with a required initial
// chunk. Assumes the file size is unknown and will upload until the input
// reaches EOF.
//
// Note that initialUploadBlock must be returned to f.putBuf()
func (up *largeUpload) Stream(ctx context.Context, initialUploadBlock []byte) (err error) {
	defer atexit.OnError(&err, func() { _ = up.cancel(ctx) })()
	fs.Debugf(up.o, "Starting streaming of large file (id %q)", up.id)
	var (
		g, gCtx      = errgroup.WithContext(ctx)
		hasMoreParts = true
	)
	up.size = int64(len(initialUploadBlock))
	g.Go(func() error {
		for part := int64(1); hasMoreParts; part++ {
			// Get a block of memory from the pool and token which limits concurrency.
			var buf []byte
			if part == 1 {
				buf = initialUploadBlock
			} else {
				buf = up.f.getBuf(false)
			}

			// Fail fast, in case an errgroup managed function returns an error
			// gCtx is cancelled. There is no point in uploading all the other parts.
			if gCtx.Err() != nil {
				up.f.putBuf(buf, false)
				return nil
			}

			// Read the chunk
			var n int
			if part == 1 {
				n = len(buf)
			} else {
				n, err = io.ReadFull(up.in, buf)
				if err == io.ErrUnexpectedEOF {
					fs.Debugf(up.o, "Read less than a full chunk, making this the last one.")
					buf = buf[:n]
					hasMoreParts = false
				} else if err == io.EOF {
					fs.Debugf(up.o, "Could not read any more bytes, previous chunk was the last.")
					up.f.putBuf(buf, false)
					return nil
				} else if err != nil {
					// other kinds of errors indicate failure
					up.f.putBuf(buf, false)
					return err
				}
			}

			// Keep stats up to date
			up.parts = part
			up.size += int64(n)
			if part > maxParts {
				up.f.putBuf(buf, false)
				return errors.Errorf("%q too big (%d bytes so far) makes too many parts %d > %d - increase --b2-chunk-size", up.o, up.size, up.parts, maxParts)
			}

			part := part // for the closure
			g.Go(func() (err error) {
				defer up.f.putBuf(buf, false)
				return up.transferChunk(gCtx, part, buf)
			})
		}
		return nil
	})
	err = g.Wait()
	if err != nil {
		return err
	}
	up.sha1s = up.sha1s[:up.parts]
	return up.finish(ctx)
}

// Upload uploads the chunks from the input
func (up *largeUpload) Upload(ctx context.Context) (err error) {
	defer atexit.OnError(&err, func() { _ = up.cancel(ctx) })()
	fs.Debugf(up.o, "Starting %s of large file in %d chunks (id %q)", up.what, up.parts, up.id)
	var (
		g, gCtx   = errgroup.WithContext(ctx)
		remaining = up.size
	)
	g.Go(func() error {
		for part := int64(1); part <= up.parts; part++ {
			// Get a block of memory from the pool and token which limits concurrency.
			buf := up.f.getBuf(up.doCopy)

			// Fail fast, in case an errgroup managed function returns an error
			// gCtx is cancelled. There is no point in uploading all the other parts.
			if gCtx.Err() != nil {
				up.f.putBuf(buf, up.doCopy)
				return nil
			}

			reqSize := remaining
			if reqSize >= up.chunkSize {
				reqSize = up.chunkSize
			}

			if !up.doCopy {
				// Read the chunk
				buf = buf[:reqSize]
				_, err = io.ReadFull(up.in, buf)
				if err != nil {
					up.f.putBuf(buf, up.doCopy)
					return err
				}
			}

			part := part // for the closure
			g.Go(func() (err error) {
				defer up.f.putBuf(buf, up.doCopy)
				if !up.doCopy {
					err = up.transferChunk(gCtx, part, buf)
				} else {
					err = up.copyChunk(gCtx, part, reqSize)
				}
				return err
			})
			remaining -= reqSize
		}
		return nil
	})
	err = g.Wait()
	if err != nil {
		return err
	}
	return up.finish(ctx)
}
