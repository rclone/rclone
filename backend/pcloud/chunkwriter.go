package pcloud

import (
	"bytes"
	"context"
	"crypto/sha1"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/url"
	"strconv"
	"sync"

	"github.com/rclone/rclone/backend/pcloud/api"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/lib/rest"
	"golang.org/x/sync/errgroup"
)

type chunkWriter struct {
	client    *rest.Client
	fs        *Fs
	chunkSize int64
	src       fs.ObjectInfo
	remote    string
	fd        int64
	fileID    int64

	mu        sync.RWMutex
	byteCount int64
}

func (c *chunkWriter) GetBytesCount() int64 {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.byteCount
}

func (c *chunkWriter) AddBytesCount(count int64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.byteCount = c.byteCount + count
}

func newChunkWriter(
	ctx context.Context,
	remote string,
	src fs.ObjectInfo,
	srcFs *Fs,
) (info fs.ChunkWriterInfo, writer *chunkWriter, err error) {
	client, err := srcFs.newSingleConnClient(ctx)
	if err != nil {
		return info, writer, fmt.Errorf("create client: %w", err)
	}
	// init an empty file
	leaf, directoryID, err := srcFs.dirCache.FindPath(ctx, remote, true)
	if err != nil {
		return info, writer, fmt.Errorf("resolve src: %w", err)
	}
	openResult, err := fileOpenNew(ctx, client, srcFs, directoryID, leaf)
	if err != nil {
		return info, writer, fmt.Errorf("open file: %w", err)
	}

	info = fs.ChunkWriterInfo{
		ChunkSize:         int64(srcFs.opt.ChunkSize),
		Concurrency:       srcFs.opt.UploadConcurrency,
		LeavePartsOnError: true,
	}
	writer = &chunkWriter{
		client:    client,
		fs:        srcFs,
		chunkSize: info.ChunkSize,
		src:       src,
		remote:    remote,
		fd:        openResult.FileDescriptor,
		fileID:    openResult.Fileid,
	}

	return info, writer, nil
}

// Abort implements fs.ChunkWriter.
func (c *chunkWriter) Abort(ctx context.Context) error {
	o, err := c.fs.NewObject(ctx, c.remote)
	if err != nil {
		return fmt.Errorf("open aborted: %w", err)
	}
	if err := o.Remove(ctx); err != nil {
		return fmt.Errorf("delete aborted: %w", err)
	}
	return nil
}

// Close implements fs.ChunkWriter.
func (c *chunkWriter) Close(ctx context.Context) error {
	// In case the file existed previosly and was larger than the newly written file, we need to truncate it to the right size.
	byteCount := c.GetBytesCount()
	fs.Debugf(c.src, "truncate target to %d bytes", byteCount)
	if _, err := c.fileTruncate(ctx, byteCount); err != nil {
		return fmt.Errorf("truncate file: %w", err)
	}

	// close fd
	if _, err := c.fileClose(ctx); err != nil {
		return fmt.Errorf("close fd: %w", err)
	}

	// There seems to be no option in the fileops API to set the modtime.
	// Therefore, we apply the modtime via the regular file API.
	o, err := c.fs.NewObject(ctx, c.remote)
	if err != nil {
		return fmt.Errorf("open newly copied object: %w", err)
	}
	if err := o.SetModTime(ctx, c.src.ModTime(ctx)); err != nil {
		return fmt.Errorf("set modtime: %w", err)
	}

	return nil
}

// WriteChunk implements fs.ChunkWriter.
func (c *chunkWriter) WriteChunk(ctx context.Context, chunkNumber int, reader io.ReadSeeker) (int64, error) {
	return c.writeChunk(ctx, chunkNumber, reader)
}

// writeChunk writes c.chunkSize bytes from reader (offset 0) to the target file with offset c.chunkSize * chunkNumber.
// This method is used by the exported `chunkWriter.WriteChunk` as well as `Object.Update`.
func (c *chunkWriter) writeChunk(ctx context.Context, chunkNumber int, reader io.Reader) (int64, error) {
	chunk := int64(chunkNumber)

	// read chunk into buffer
	buffer := make([]byte, c.chunkSize)
	offset := chunk * c.chunkSize
	eg, egCtx := errgroup.WithContext(ctx)

	// read input buffer and calculate hash
	inSHA1 := ""
	eg.Go(func() error {
		byteCount, err := io.ReadFull(reader, buffer)
		buffer = buffer[:byteCount]
		if err != nil && !errors.Is(err, io.ErrUnexpectedEOF) && !errors.Is(err, io.EOF) {
			return err
		}
		inSHA1Bytes := sha1.Sum(buffer)
		inSHA1 = hex.EncodeToString(inSHA1Bytes[:])
		return nil
	})

	// get target hash
	outSHA1 := ""
	eg.Go(func() error {
		targetChecksum, err := c.fileChecksum(egCtx, offset, c.chunkSize)
		if err != nil {
			return err
		}
		outSHA1 = targetChecksum.SHA1
		return nil
	})

	if err := eg.Wait(); err != nil {
		if errors.Is(err, io.EOF) {
			return 0, nil
		}
		return 0, fmt.Errorf("check hashes of chunk %d: %w", chunk, err)
	}
	if outSHA1 == "" || inSHA1 == "" {
		return 0, fmt.Errorf("block %d: expect both hashes to be filled: src: %q, target: %q", chunk, inSHA1, outSHA1)
	}

	byteCount := int64(len(buffer))
	c.AddBytesCount(byteCount)
	// check hash of chunk, skip if fits
	if inSHA1 == outSHA1 {
		fs.Debugf(c.src, "chunk %d matches", chunk)
		return byteCount, nil
	}

	// upload buffered chunk with offset if necessary
	if _, err := c.filePWrite(ctx, offset, buffer); err != nil {
		return 0, err
	}
	fs.Debugf(c.src, "chunk %d written", chunk)
	return byteCount, nil
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
func (c *chunkWriter) fileChecksum(
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
func (c *chunkWriter) filePWrite(
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

// Call pcloud file_truncate, see [API Doc.]
// [API Doc]: https://docs.pcloud.com/methods/fileops/file_truncate.html
func (c *chunkWriter) fileTruncate(ctx context.Context, length int64) (*api.FileTruncateResponse, error) {
	opts := rest.Opts{
		Method:           "PUT",
		Path:             "/file_truncate",
		Parameters:       url.Values{},
		TransferEncoding: []string{"identity"}, // pcloud doesn't like chunked encoding
		ExtraHeaders: map[string]string{
			"Connection": "keep-alive",
		},
	}
	opts.Parameters.Set("fd", strconv.FormatInt(c.fd, 10))
	opts.Parameters.Set("length", strconv.FormatInt(length, 10))

	result := &api.FileTruncateResponse{}
	err := c.fs.pacer.CallNoRetry(func() (bool, error) {
		resp, err := c.client.CallJSON(ctx, &opts, nil, result)
		err = result.Error.Update(err)
		return shouldRetry(ctx, resp, err)
	})
	if err != nil {
		return nil, fmt.Errorf("truncate fd %d to %d: %w", c.fd, length, err)
	}
	return result, nil
}

// Call pcloud file_close, see [API Doc.]
// [API Doc]: https://docs.pcloud.com/methods/fileops/file_close.html
func (c *chunkWriter) fileClose(ctx context.Context) (*api.FileCloseResponse, error) {
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
