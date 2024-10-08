package pcloud

import (
	"bytes"
	"context"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"net/url"
	"strconv"
	"time"

	"github.com/rclone/rclone/backend/pcloud/api"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/lib/rest"
)

// writerAt implements fs.WriterAtCloser, adding the OpenWrtierAt feature to pcloud.
type writerAt struct {
	ctx    context.Context
	client *rest.Client
	fs     *Fs
	size   int64
	remote string
	fd     int64
	fileID int64
}

// Close implements WriterAt.Close.
func (c *writerAt) Close() error {
	// close fd
	if _, err := c.fileClose(c.ctx); err != nil {
		return fmt.Errorf("close fd: %w", err)
	}

	// Avoiding race conditions: Depending on the tcp connection, there might be
	// caching issues when checking the size immediately after write.
	// Hence we try avoiding them by checking the resulting size on a different connection.
	if c.size < 0 {
		// Without knowing the size, we cannot do size checks.
		// Falling back to a sleep of 1s for sake of hope.
		time.Sleep(1 * time.Second)
		return nil
	}
	sizeOk := false
	sizeLastSeen := int64(0)
	for retry := 0; retry < 5; retry++ {
		fs.Debugf(c.remote, "checking file size: try %d/5", retry)
		obj, err := c.fs.NewObject(c.ctx, c.remote)
		if err != nil {
			return fmt.Errorf("get uploaded obj: %w", err)
		}
		sizeLastSeen = obj.Size()
		if obj.Size() == c.size {
			sizeOk = true
			break
		}
		time.Sleep(1 * time.Second)
	}

	if !sizeOk {
		return fmt.Errorf("incorrect size after upload: got %d, want %d", sizeLastSeen, c.size)
	}

	return nil
}

// WriteAt implements fs.WriteAt.
func (c *writerAt) WriteAt(buffer []byte, offset int64) (n int, err error) {
	contentLength := len(buffer)

	inSHA1Bytes := sha1.Sum(buffer)
	inSHA1 := hex.EncodeToString(inSHA1Bytes[:])

	// get target hash
	outChecksum, err := c.fileChecksum(c.ctx, offset, int64(contentLength))
	if err != nil {
		return 0, err
	}
	outSHA1 := outChecksum.SHA1

	if outSHA1 == "" || inSHA1 == "" {
		return 0, fmt.Errorf("expect both hashes to be filled: src: %q, target: %q", inSHA1, outSHA1)
	}

	// check hash of buffer, skip if fits
	if inSHA1 == outSHA1 {
		return contentLength, nil
	}

	// upload buffer with offset if necessary
	if _, err := c.filePWrite(c.ctx, offset, buffer); err != nil {
		return 0, err
	}

	return contentLength, nil
}

// Call pcloud file_open using folderid and name with O_CREAT and O_WRITE flags, see [API Doc.]
// [API Doc]: https://docs.pcloud.com/methods/fileops/file_open.html
func fileOpenNew(ctx context.Context, c *rest.Client, srcFs *Fs, directoryID, filename string) (*api.FileOpenResponse, error) {
	opts := rest.Opts{
		Method:           "PUT",
		Path:             "/file_open",
		Parameters:       url.Values{},
		TransferEncoding: []string{"identity"}, // pcloud doesn't like chunked encoding
		ExtraHeaders: map[string]string{
			"Connection": "keep-alive",
		},
	}
	filename = srcFs.opt.Enc.FromStandardName(filename)
	opts.Parameters.Set("name", filename)
	opts.Parameters.Set("folderid", dirIDtoNumber(directoryID))
	opts.Parameters.Set("flags", "0x0042") // O_CREAT, O_WRITE

	result := &api.FileOpenResponse{}
	err := srcFs.pacer.CallNoRetry(func() (bool, error) {
		resp, err := c.CallJSON(ctx, &opts, nil, result)
		err = result.Error.Update(err)
		return shouldRetry(ctx, resp, err)
	})
	if err != nil {
		return nil, fmt.Errorf("open new file descriptor: %w", err)
	}
	return result, nil
}

// Call pcloud file_checksum, see [API Doc.]
// [API Doc]: https://docs.pcloud.com/methods/fileops/file_checksum.html
func (c *writerAt) fileChecksum(
	ctx context.Context,
	offset, count int64,
) (*api.FileChecksumResponse, error) {
	opts := rest.Opts{
		Method:           "PUT",
		Path:             "/file_checksum",
		Parameters:       url.Values{},
		TransferEncoding: []string{"identity"}, // pcloud doesn't like chunked encoding
		ExtraHeaders: map[string]string{
			"Connection": "keep-alive",
		},
	}
	opts.Parameters.Set("fd", strconv.FormatInt(c.fd, 10))
	opts.Parameters.Set("offset", strconv.FormatInt(offset, 10))
	opts.Parameters.Set("count", strconv.FormatInt(count, 10))

	result := &api.FileChecksumResponse{}
	err := c.fs.pacer.CallNoRetry(func() (bool, error) {
		resp, err := c.client.CallJSON(ctx, &opts, nil, result)
		err = result.Error.Update(err)
		return shouldRetry(ctx, resp, err)
	})
	if err != nil {
		return nil, fmt.Errorf("checksum of fd %d with offset %d and size %d: %w", c.fd, offset, count, err)
	}
	return result, nil
}

// Call pcloud file_pwrite, see [API Doc.]
// [API Doc]: https://docs.pcloud.com/methods/fileops/file_pwrite.html
func (c *writerAt) filePWrite(
	ctx context.Context,
	offset int64,
	buf []byte,
) (*api.FilePWriteResponse, error) {
	contentLength := int64(len(buf))
	opts := rest.Opts{
		Method:           "PUT",
		Path:             "/file_pwrite",
		Body:             bytes.NewReader(buf),
		ContentLength:    &contentLength,
		Parameters:       url.Values{},
		TransferEncoding: []string{"identity"}, // pcloud doesn't like chunked encoding
		Close:            false,
		ExtraHeaders: map[string]string{
			"Connection": "keep-alive",
		},
	}
	opts.Parameters.Set("fd", strconv.FormatInt(c.fd, 10))
	opts.Parameters.Set("offset", strconv.FormatInt(offset, 10))

	result := &api.FilePWriteResponse{}
	err := c.fs.pacer.CallNoRetry(func() (bool, error) {
		resp, err := c.client.CallJSON(ctx, &opts, nil, result)
		err = result.Error.Update(err)
		return shouldRetry(ctx, resp, err)
	})
	if err != nil {
		return nil, fmt.Errorf("write %d bytes to fd %d with offset %d: %w", contentLength, c.fd, offset, err)
	}
	return result, nil
}

// Call pcloud file_close, see [API Doc.]
// [API Doc]: https://docs.pcloud.com/methods/fileops/file_close.html
func (c *writerAt) fileClose(ctx context.Context) (*api.FileCloseResponse, error) {
	opts := rest.Opts{
		Method:           "PUT",
		Path:             "/file_close",
		Parameters:       url.Values{},
		TransferEncoding: []string{"identity"}, // pcloud doesn't like chunked encoding
		Close:            true,
	}
	opts.Parameters.Set("fd", strconv.FormatInt(c.fd, 10))

	result := &api.FileCloseResponse{}
	err := c.fs.pacer.CallNoRetry(func() (bool, error) {
		resp, err := c.client.CallJSON(ctx, &opts, nil, result)
		err = result.Error.Update(err)
		return shouldRetry(ctx, resp, err)
	})
	if err != nil {
		return nil, fmt.Errorf("close file descriptor: %w", err)
	}
	return result, nil
}
