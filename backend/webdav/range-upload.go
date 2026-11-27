package webdav

/*
	chunked upload with partial PUT requests (a PUT with a Content-Range
	header), for servers without a chunked upload API of their own, such
	as Apache with mod_dav
*/

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/fserrors"
	"github.com/rclone/rclone/lib/random"
	"github.com/rclone/rclone/lib/readers"
	"github.com/rclone/rclone/lib/rest"
)

func (o *Object) shouldUseRangeUpload(src fs.ObjectInfo) bool {
	chunkSize := int64(o.fs.opt.RangeChunkSize)
	return !o.fs.canTus && !o.fs.canChunk && chunkSize > 0 && src.Size() > chunkSize
}

// updateRange uploads in to a temporary file next to o, one chunk per
// partial PUT request, then moves it over o.
//
// If the upload fails the temporary file is removed and o is left as it was.
func (o *Object) updateRange(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (err error) {
	part := &Object{
		fs:     o.fs,
		remote: fmt.Sprintf("%s.%s%s", o.remote, random.String(8), fs.GetConfig(ctx).PartialSuffix),
	}
	defer func() {
		if err != nil {
			// Remove the temporary file even when the upload was cancelled
			if removeErr := part.Remove(context.WithoutCancel(ctx)); removeErr != nil {
				fs.Debugf(part, "Failed to remove temporary file of failed upload: %v", removeErr)
			}
		}
	}()

	size := src.Size()
	chunkSize := int64(o.fs.opt.RangeChunkSize)
	contentType := fs.MimeType(ctx, src)
	buf := make([]byte, chunkSize)
	for offset := int64(0); offset < size; offset += chunkSize {
		if err = ctx.Err(); err != nil {
			return err
		}
		length := min(size-offset, chunkSize)
		chunk := readers.NewRepeatableLimitReaderBuffer(in, buf, length)
		if err = part.putRange(ctx, chunk, offset, length, size, contentType, options); err != nil {
			return fmt.Errorf("uploading chunk at offset %d failed: %w", offset, err)
		}
		if err = part.checkRangeSize(ctx, offset+length); err != nil {
			return err
		}
	}
	return part.moveRange(ctx, o, src)
}

// putRange writes chunk at offset in o. The first chunk creates o, or
// truncates it.
//
// The chunk is retried in place, so a transient failure doesn't make the
// caller send the chunks already written again.
func (o *Object) putRange(ctx context.Context, chunk *readers.RepeatableReader, offset, length, size int64, contentType string, options []fs.OpenOption) error {
	getBody := func() (io.ReadCloser, error) {
		// RepeatableReader rewinds without upsetting the accounting
		if _, err := chunk.Seek(0, io.SeekStart); err != nil {
			return nil, err
		}
		return io.NopCloser(chunk), nil
	}
	opts := rest.Opts{
		Method:        "PUT",
		Path:          o.filePath(),
		GetBody:       getBody,
		NoResponse:    true,
		ContentLength: &length,
		ContentType:   contentType,
		Options:       options,
	}
	if offset > 0 {
		opts.ExtraHeaders = map[string]string{
			"Content-Range": fmt.Sprintf("bytes %d-%d/%d", offset, offset+length-1, size),
		}
	}
	err := o.fs.pacer.Call(func() (bool, error) {
		body, err := getBody()
		if err != nil {
			return false, err
		}
		opts.Body = body
		resp, err := o.fs.srv.Call(ctx, &opts)
		return o.fs.shouldRetry(ctx, resp, err)
	})
	if err != nil && (fserrors.IsRetryError(err) || fserrors.ShouldRetry(err)) {
		// The chunk's retries are used up. Don't let the caller's low level
		// retries start the whole upload again.
		err = fserrors.NoLowLevelRetryError(errors.New(err.Error()))
	}
	return err
}

// checkRangeSize checks that o is want bytes long. It isn't when the
// server ignored the Content-Range header and stored the chunk as the
// whole file.
func (o *Object) checkRangeSize(ctx context.Context, want int64) error {
	o.hasMetaData = false
	if err := o.readMetaData(ctx); err != nil {
		return fmt.Errorf("couldn't read the size of the temporary file: %w", err)
	}
	if o.size != want {
		return fserrors.NoRetryError(fmt.Errorf("the temporary file is %d bytes instead of %d: the server doesn't support partial PUT requests, so set range_upload_chunk_size to 0", o.size, want))
	}
	return nil
}

// moveRange moves the temporary file o over dst.
func (o *Object) moveRange(ctx context.Context, dst *Object, src fs.ObjectInfo) error {
	destinationURL, err := rest.URLJoin(o.fs.endpoint, dst.filePath())
	if err != nil {
		return fmt.Errorf("couldn't join URL: %w", err)
	}
	opts := rest.Opts{
		Method:     "MOVE",
		Path:       o.filePath(),
		NoResponse: true,
		ExtraHeaders: map[string]string{
			"Destination": destinationURL.String(),
			"Overwrite":   "T",
		},
	}
	if o.fs.useOCMtime {
		opts.ExtraHeaders["X-OC-Mtime"] = fmt.Sprintf("%d", src.ModTime(ctx).Unix())
	}
	err = o.fs.pacer.Call(func() (bool, error) {
		resp, err := o.fs.srv.Call(ctx, &opts)
		return o.fs.shouldRetry(ctx, resp, err)
	})
	if err != nil {
		return fmt.Errorf("moving the temporary file into place failed: %w", err)
	}
	return nil
}
