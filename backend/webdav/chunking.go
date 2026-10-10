package webdav

/*
	chunked update for Nextcloud
	see https://docs.nextcloud.com/server/28/developer_manual/client_apis/WebDAV/chunking.html
	chunk v2 appears in Nextcloud from version 28. It allows S3 direct upload if it is used as primary or external backend.
	For Nextcloud < 28, the server will accept chunks as if it is v1 mode (because only additional headers are added for v2).
  For Nextcloud >= 28, the server will accept chunk v2 mode if the prerequisites at server are met. Otherwise it fallbacks to v1.
*/

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"path"
	"strconv"
	"time"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/lib/readers"
	"github.com/rclone/rclone/lib/rest"
)

const (
	// See https://docs.nextcloud.com/server/28/developer_manual/client_apis/WebDAV/chunking.html#introduction

	// Nextcloud Chunking v2 requires chunks to be between 5 MiB and 5 GiB, except for the final chunk which may be smaller.
	nextcloudV2ChunkSizeMin = fs.SizeSuffix(5 * 1024 * 1024)
	nextcloudV2ChunkSizeMax = fs.SizeSuffix(5 * 1024 * 1024 * 1024)

	// Nextcloud Chunking v2 supports at most 10000 chunks.
	nextcloudV2ChunkNbMax = int64(10000)
)

func (f *Fs) shouldRetryChunkMerge(ctx context.Context, resp *http.Response, err error, sleepTime *time.Duration, wasLocked *bool) (bool, error) {
	// Not found. Can be returned by NextCloud when merging chunks of an upload.
	if resp != nil && resp.StatusCode == 404 {
		if *wasLocked {
			// Assume a 404 error after we've received a 423 error is actually a success
			return false, nil
		}
		return true, err
	}

	// 423 LOCKED
	if resp != nil && resp.StatusCode == 423 {
		*wasLocked = true
		fs.Logf(f, "Sleeping for %v to wait for chunks to be merged after 423 error", *sleepTime)
		time.Sleep(*sleepTime)
		*sleepTime *= 2
		return true, fmt.Errorf("merging the uploaded chunks failed with 423 LOCKED. This usually happens when the chunks merging is still in progress on NextCloud, but it may also indicate a failed transfer: %w", err)
	}

	return f.shouldRetry(ctx, resp, err)
}

// set the chunk size for testing
func (f *Fs) setUploadChunkSize(cs fs.SizeSuffix) (old fs.SizeSuffix, err error) {
	old, f.opt.ChunkSize = f.opt.ChunkSize, cs
	return
}

func (o *Object) getChunksUploadDir() (string, error) {
	hasher := md5.New()
	_, err := hasher.Write([]byte(o.filePath()))
	if err != nil {
		return "", fmt.Errorf("chunked upload couldn't hash URL: %w", err)
	}
	uploadDir := "rclone-chunked-upload-" + hex.EncodeToString(hasher.Sum(nil))
	return uploadDir, nil
}

func (f *Fs) getChunksUploadURL() (string, error) {
	submatch := nextCloudURLRegex.FindStringSubmatch(f.endpointURL)
	if submatch == nil {
		return "", errors.New("the remote url looks incorrect. Note that nextcloud chunked uploads require you to use the /dav/files/USER endpoint instead of /webdav. Please check 'rclone config show remotename' to verify that the url field ends in /dav/files/USERNAME")
	}

	baseURL, user := submatch[1], submatch[2]
	chunksUploadURL := fmt.Sprintf("%s/dav/uploads/%s/", baseURL, user)

	return chunksUploadURL, nil
}

func (o *Object) shouldUseChunkedUpload(src fs.ObjectInfo) bool {
	return o.fs.canChunk && o.fs.opt.ChunkSize > 0 && src.Size() > int64(o.fs.opt.ChunkSize)
}

func (o *Object) updateChunked(ctx context.Context, in0 io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (err error) {
	var uploadDir string

	// Nextcloud Chunking v2 requires the final destination URL to be supplied during MKCOL, PUT  and MOVE of chunk upload.
	destinationURL, err := rest.URLJoin(o.fs.endpoint, o.filePath())
	if err != nil {
		return fmt.Errorf("finalize chunked upload couldn't join URL: %w", err)
	}

	chunkSize := o.fs.opt.ChunkSize
	if chunkSize < nextcloudV2ChunkSizeMin {
		return fmt.Errorf("chunked upload chunk size %s is too small for Nextcloud Chunking v2; minimum is %s", chunkSize, nextcloudV2ChunkSizeMin)
	}
	if chunkSize > nextcloudV2ChunkSizeMax {
		return fmt.Errorf("chunked upload chunk size %s is too large for Nextcloud Chunking v2; maximum is %s", chunkSize, nextcloudV2ChunkSizeMax)
	}

	size := src.Size()
	// This is equivalent to chunkCount = ceil(size / chunkSize) for intergers.
	// See https://stackoverflow.com/questions/2745074/fast-ceiling-of-an-integer-division-in-c-c
	chunkCount := 1 + (size-1)/int64(chunkSize)
	if chunkCount > nextcloudV2ChunkNbMax {
		return fmt.Errorf("chunked upload requires %d chunks, but Nextcloud Chunking v2 supports at most %d chunks; increase the chunk size", chunkCount, nextcloudV2ChunkNbMax)
	}

	// see https://docs.nextcloud.com/server/28/developer_manual/client_apis/WebDAV/chunking.html#starting-a-chunked-upload
	uploadDir, err = o.createChunksUploadDirectory(ctx, destinationURL.String())
	if err != nil {
		return err
	}

	partObj := &Object{
		fs: o.fs,
	}

	// see https://docs.nextcloud.com/server/28/developer_manual/client_apis/WebDAV/chunking.html#uploading-chunks
	err = o.uploadChunks(ctx, in0, size, partObj, uploadDir, destinationURL.String(), options)
	if err != nil {
		return err
	}

	// see https://docs.nextcloud.com/server/28/developer_manual/client_apis/WebDAV/chunking.html#assembling-the-chunks
	err = o.mergeChunks(ctx, uploadDir, destinationURL.String(), options, src)
	if err != nil {
		return err
	}

	return nil
}

func (o *Object) uploadChunks(ctx context.Context, in0 io.Reader, size int64, partObj *Object, uploadDir string, destinationURL string, options []fs.OpenOption) error {
	chunkSize := int64(partObj.fs.opt.ChunkSize)

	// TODO: upload chunks in parallel for faster transfer speeds
	chunkNumber := int64(1)
	for offset := int64(0); offset < size; offset += chunkSize {
		if err := ctx.Err(); err != nil {
			return err
		}

		// Last chunk may be smaller.
		contentLength := min(size-offset, chunkSize)

		// Nextcloud Chunking v2 requires chunk names to be numeric, in
		// ascending order, between 1 and 10000.
		partObj.remote = fmt.Sprintf("%s/%05d", uploadDir, chunkNumber)
		chunkNumber++

		// Enable low-level HTTP 2 retries.
		// 2022-04-28 15:59:06 ERROR : stuff/video.avi: Failed to copy: uploading chunk failed: Put "https://censored.com/remote.php/dav/uploads/Admin/rclone-chunked-upload-censored/000006113198080-000006123683840": http2: Transport: cannot retry err [http2: Transport received Server's graceful shutdown GOAWAY] after Request.Body was written; define Request.GetBody to avoid this error

		buf := make([]byte, chunkSize)
		in := readers.NewRepeatableLimitReaderBuffer(in0, buf, chunkSize)

		getBody := func() (io.ReadCloser, error) {
			// RepeatableReader{} plays well with accounting so rewinding doesn't make the progress buggy
			if _, err := in.Seek(0, io.SeekStart); err != nil {
				return nil, err
			}

			return io.NopCloser(in), nil
		}

		headers := map[string]string{
			"Destination":     destinationURL,
			"OC-Total-Length": strconv.FormatInt(size, 10),
		}

		err := partObj.updateSimple(ctx, in, getBody, partObj.remote, contentLength, "application/x-www-form-urlencoded", headers, o.fs.chunksUploadURL, options...)
		if err != nil {
			return fmt.Errorf("uploading chunk failed: %w", err)
		}
	}
	return nil
}

func (o *Object) createChunksUploadDirectory(ctx context.Context, destinationURL string) (string, error) {
	uploadDir, err := o.getChunksUploadDir()
	if err != nil {
		return uploadDir, err
	}

	err = o.purgeUploadedChunks(ctx, uploadDir)
	if err != nil {
		return "", fmt.Errorf("chunked upload couldn't purge upload directory: %w", err)
	}

	opts := rest.Opts{
		Method:     "MKCOL",
		Path:       uploadDir + "/",
		NoResponse: true,
		RootURL:    o.fs.chunksUploadURL,
		ExtraHeaders: map[string]string{
			"Destination": destinationURL,
		},
	}
	err = o.fs.pacer.CallNoRetry(func() (bool, error) {
		resp, err := o.fs.srv.Call(ctx, &opts)
		return o.fs.shouldRetry(ctx, resp, err)
	})
	if err != nil {
		return "", fmt.Errorf("making upload directory failed: %w", err)
	}
	return uploadDir, err
}

func (o *Object) mergeChunks(ctx context.Context, uploadDir string, destinationURL string, options []fs.OpenOption, src fs.ObjectInfo) error {
	// see https://docs.nextcloud.com/server/latest/developer_manual/client_apis/WebDAV/chunking.html#assembling-the-chunks
	opts := rest.Opts{
		Method:     "MOVE",
		Path:       path.Join(uploadDir, ".file"),
		NoResponse: true,
		Options:    options,
		RootURL:    o.fs.chunksUploadURL,
	}
	opts.ExtraHeaders = o.extraHeaders(ctx, src)
	opts.ExtraHeaders["Destination"] = destinationURL
	opts.ExtraHeaders["OC-Total-Length"] = strconv.FormatInt(src.Size(), 10)
	opts.ExtraHeaders["X-OC-Mtime"] = strconv.FormatInt(src.ModTime(ctx).Unix(), 10)
	sleepTime := 5 * time.Second
	wasLocked := false
	err := o.fs.pacer.Call(func() (bool, error) {
		resp, err := o.fs.srv.Call(ctx, &opts)
		return o.fs.shouldRetryChunkMerge(ctx, resp, err, &sleepTime, &wasLocked)
	})
	if err != nil {
		return fmt.Errorf("finalize chunked upload failed, destinationURL: \"%s\": %w", destinationURL, err)
	}
	return err
}

func (o *Object) purgeUploadedChunks(ctx context.Context, uploadDir string) error {
	// clean the upload directory if it exists (this means that a previous try didn't clean up properly).
	opts := rest.Opts{
		Method:     "DELETE",
		Path:       uploadDir + "/",
		NoResponse: true,
		RootURL:    o.fs.chunksUploadURL,
	}

	err := o.fs.pacer.Call(func() (bool, error) {
		resp, err := o.fs.srv.CallXML(ctx, &opts, nil, nil)

		// directory doesn't exist, no need to purge
		if resp != nil && resp.StatusCode == http.StatusNotFound {
			return false, nil
		}

		return o.fs.shouldRetry(ctx, resp, err)
	})

	return err
}
