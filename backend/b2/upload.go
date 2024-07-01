// Upload large files for b2
//
// Docs - https://www.backblaze.com/docs/cloud-storage-large-files

package b2

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	gohash "hash"
	"io"
	"strings"
	"sync"

	"github.com/rclone/rclone/backend/b2/api"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/accounting"
	"github.com/rclone/rclone/fs/chunksize"
	"github.com/rclone/rclone/fs/hash"
	"github.com/rclone/rclone/lib/atexit"
	"github.com/rclone/rclone/lib/pool"
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
	parts     int                             // calculated number of parts, if known
	sha1smu   sync.Mutex                      // mutex to protect sha1s
	sha1s     []string                        // slice of SHA1s for each part
	uploadMu  sync.Mutex                      // lock for upload variable
	uploads   []*api.GetUploadPartURLResponse // result of get upload URL calls
	chunkSize int64                           // chunk size to use
	src       *Object                         // if copying, object we are reading from
	info      *api.FileInfo                   // final response with info about the object
}

// newLargeUpload starts an upload of object o from in with metadata in src
//
// If newInfo is set then metadata from that will be used instead of reading it from src
func (f *Fs) newLargeUpload(ctx context.Context, o *Object, in io.Reader, src fs.ObjectInfo, defaultChunkSize fs.SizeSuffix, doCopy bool, newInfo *api.File, options ...fs.OpenOption) (up *largeUpload, err error) {
	size := src.Size()
	parts := 0
	chunkSize := defaultChunkSize
	if size == -1 {
		fs.Debugf(o, "Streaming upload with --b2-chunk-size %s allows uploads of up to %s and will fail only when that limit is reached.", f.opt.ChunkSize, maxParts*f.opt.ChunkSize)
	} else {
		chunkSize = chunksize.Calculator(o, size, maxParts, defaultChunkSize)
		parts = int(size / int64(chunkSize))
		if size%int64(chunkSize) != 0 {
			parts++
		}
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
	optionsToSend := make([]fs.OpenOption, 0, len(options))
	if newInfo == nil {
		modTime, err := o.getModTime(ctx, src, options)
		if err != nil {
			return nil, err
		}

		request.ContentType = fs.MimeType(ctx, src)
		request.Info = map[string]string{
			timeKey: timeString(modTime),
		}
		// Custom upload headers - remove header prefix since they are sent in the body
		for _, option := range options {
			k, v := option.Header()
			k = strings.ToLower(k)
			if strings.HasPrefix(k, headerPrefix) {
				request.Info[k[len(headerPrefix):]] = v
			} else {
				optionsToSend = append(optionsToSend, option)
			}
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
	opts := rest.Opts{
		Method:  "POST",
		Path:    "/b2_start_large_file",
		Options: optionsToSend,
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
		sha1s:     make([]string, 0, 16),
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
	if len(up.uploads) > 0 {
		upload, up.uploads = up.uploads[0], up.uploads[1:]
		up.uploadMu.Unlock()
		return upload, nil
	}
	up.uploadMu.Unlock()

	opts := rest.Opts{
		Method: "POST",
		Path:   "/b2_get_upload_part_url",
	}
	var request = api.GetUploadPartURLRequest{
		ID: up.id,
	}
	err = up.f.pacer.Call(func() (bool, error) {
		resp, err := up.f.srv.CallJSON(ctx, &opts, &request, &upload)
		return up.f.shouldRetry(ctx, resp, err)
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get upload URL: %w", err)
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

// Add an sha1 to the being built up sha1s
func (up *largeUpload) addSha1(chunkNumber int, sha1 string) {
	up.sha1smu.Lock()
	defer up.sha1smu.Unlock()
	if len(up.sha1s) < chunkNumber+1 {
		up.sha1s = append(up.sha1s, make([]string, chunkNumber+1-len(up.sha1s))...)
	}
	up.sha1s[chunkNumber] = sha1
}

// WriteChunk will write chunk number with reader bytes, where chunk number >= 0
func (up *largeUpload) WriteChunk(ctx context.Context, chunkNumber int, reader io.ReadSeeker) (size int64, err error) {
	// Only account after the checksum reads have been done
	if do, ok := reader.(pool.DelayAccountinger); ok {
		// To figure out this number, do a transfer and if the accounted size is 0 or a
		// multiple of what it should be, increase or decrease this number.
		do.DelayAccounting(1)
	}

	err = up.f.pacer.Call(func() (bool, error) {
		// Discover the size by seeking to the end
		size, err = reader.Seek(0, io.SeekEnd)
		if err != nil {
			return false, err
		}

		// rewind the reader on retry and after reading size
		_, err = reader.Seek(0, io.SeekStart)
		if err != nil {
			return false, err
		}

		fs.Debugf(up.o, "Sending chunk %d length %d", chunkNumber, size)

		// Get upload URL
		upload, err := up.getUploadURL(ctx)
		if err != nil {
			return false, err
		}

		in := newHashAppendingReader(reader, sha1.New())
		sizeWithHash := size + int64(in.AdditionalLength())

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
				"X-Bz-Part-Number": fmt.Sprintf("%d", chunkNumber+1),
				sha1Header:         "hex_digits_at_end",
			},
			ContentLength: &sizeWithHash,
		}

		var response api.UploadPartResponse

		resp, err := up.f.srv.CallJSON(ctx, &opts, nil, &response)
		retry, err := up.f.shouldRetry(ctx, resp, err)
		if err != nil {
			fs.Debugf(up.o, "Error sending chunk %d (retry=%v): %v: %#v", chunkNumber, retry, err, err)
		}
		// On retryable error clear PartUploadURL
		if retry {
			fs.Debugf(up.o, "Clearing part upload URL because of error: %v", err)
			upload = nil
		}
		up.returnUploadURL(upload)
		up.addSha1(chunkNumber, in.HexSum())
		return retry, err
	})
	if err != nil {
		fs.Debugf(up.o, "Error sending chunk %d: %v", chunkNumber, err)
	} else {
		fs.Debugf(up.o, "Done sending chunk %d", chunkNumber)
	}
	return size, err
}

// Copy a chunk
func (up *largeUpload) copyChunk(ctx context.Context, part int, partSize int64) error {
	err := up.f.pacer.Call(func() (bool, error) {
		fs.Debugf(up.o, "Copying chunk %d length %d", part, partSize)
		opts := rest.Opts{
			Method: "POST",
			Path:   "/b2_copy_part",
		}
		offset := int64(part) * up.chunkSize // where we are in the source file
		var request = api.CopyPartRequest{
			SourceID:    up.src.id,
			LargeFileID: up.id,
			PartNumber:  int64(part + 1),
			Range:       fmt.Sprintf("bytes=%d-%d", offset, offset+partSize-1),
		}
		var response api.UploadPartResponse
		resp, err := up.f.srv.CallJSON(ctx, &opts, &request, &response)
		retry, err := up.f.shouldRetry(ctx, resp, err)
		if err != nil {
			fs.Debugf(up.o, "Error copying chunk %d (retry=%v): %v: %#v", part, retry, err, err)
		}
		up.addSha1(part, response.SHA1)
		return retry, err
	})
	if err != nil {
		fs.Debugf(up.o, "Error copying chunk %d: %v", part, err)
	} else {
		fs.Debugf(up.o, "Done copying chunk %d", part)
	}
	return err
}

// Close closes off the large upload
func (up *largeUpload) Close(ctx context.Context) error {
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
	up.info = &response
	return nil
}

// Abort aborts the large upload
func (up *largeUpload) Abort(ctx context.Context) error {
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
func (up *largeUpload) Stream(ctx context.Context, initialUploadBlock *pool.RW) (err error) {
	defer atexit.OnError(&err, func() { _ = up.Abort(ctx) })()
	fs.Debugf(up.o, "Starting streaming of large file (id %q)", up.id)
	var (
		g, gCtx      = errgroup.WithContext(ctx)
		hasMoreParts = true
	)
	up.size = initialUploadBlock.Size()
	up.parts = 0
	for part := 0; hasMoreParts; part++ {
		// Get a block of memory from the pool and token which limits concurrency.
		var rw *pool.RW
		if part == 0 {
			rw = initialUploadBlock
		} else {
			rw = up.f.getRW(false)
		}

		// Fail fast, in case an errgroup managed function returns an error
		// gCtx is cancelled. There is no point in uploading all the other parts.
		if gCtx.Err() != nil {
			up.f.putRW(rw)
			break
		}

		// Read the chunk
		var n int64
		if part == 0 {
			n = rw.Size()
		} else {
			n, err = io.CopyN(rw, up.in, up.chunkSize)
			if err == io.EOF {
				if n == 0 {
					fs.Debugf(up.o, "Not sending empty chunk after EOF - ending.")
					up.f.putRW(rw)
					break
				} else {
					fs.Debugf(up.o, "Read less than a full chunk %d, making this the last one.", n)
				}
				hasMoreParts = false
			} else if err != nil {
				// other kinds of errors indicate failure
				up.f.putRW(rw)
				return err
			}
		}

		// Keep stats up to date
		up.parts += 1
		up.size += n
		if part > maxParts {
			up.f.putRW(rw)
			return fmt.Errorf("%q too big (%d bytes so far) makes too many parts %d > %d - increase --b2-chunk-size", up.o, up.size, up.parts, maxParts)
		}

		part := part // for the closure
		g.Go(func() (err error) {
			defer up.f.putRW(rw)
			_, err = up.WriteChunk(gCtx, part, rw)
			return err
		})
	}
	err = g.Wait()
	if err != nil {
		return err
	}
	return up.Close(ctx)
}

// Copy the chunks from the source to the destination
func (up *largeUpload) Copy(ctx context.Context) (err error) {
	defer atexit.OnError(&err, func() { _ = up.Abort(ctx) })()
	fs.Debugf(up.o, "Starting %s of large file in %d chunks (id %q)", up.what, up.parts, up.id)
	var (
		g, gCtx   = errgroup.WithContext(ctx)
		remaining = up.size
	)
	g.SetLimit(up.f.opt.UploadConcurrency)
	for part := 0; part < up.parts; part++ {
		// Fail fast, in case an errgroup managed function returns an error
		// gCtx is cancelled. There is no point in copying all the other parts.
		if gCtx.Err() != nil {
			break
		}

		reqSize := remaining
		if reqSize >= up.chunkSize {
			reqSize = up.chunkSize
		}

		part := part // for the closure
		g.Go(func() (err error) {
			return up.copyChunk(gCtx, part, reqSize)
		})
		remaining -= reqSize
	}
	err = g.Wait()
	if err != nil {
		return err
	}
	return up.Close(ctx)
}
