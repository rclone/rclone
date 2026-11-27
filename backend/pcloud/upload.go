package pcloud

import (
	"context"
	"fmt"
	"io"
	"net/url"
	"strconv"
	"time"

	"github.com/rclone/rclone/backend/pcloud/api"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/lib/atexit"
	"github.com/rclone/rclone/lib/multipart"
	"github.com/rclone/rclone/lib/rest"
)

// uploadCreate starts a resumable upload session, returning its uploadid.
//
// upload_create, upload_write, upload_info, upload_save and upload_delete are
// not in pcloud's published API, but are the ones its own clients use.
func uploadCreate(ctx context.Context, f *Fs, size int64) (*api.UploadCreateResponse, error) {
	opts := rest.Opts{
		Method:           "PUT",
		Path:             "/upload_create",
		Parameters:       url.Values{},
		TransferEncoding: []string{"identity"}, // pcloud doesn't like chunked encoding
	}
	opts.Parameters.Set("filesize", strconv.FormatInt(size, 10))

	result := &api.UploadCreateResponse{}
	err := f.pacer.Call(func() (bool, error) {
		resp, err := f.srv.CallJSON(ctx, &opts, nil, result)
		err = result.Error.Update(err)
		return shouldRetry(ctx, resp, err)
	})
	if err != nil {
		return nil, fmt.Errorf("create upload session: %w", err)
	}
	return result, nil
}

// uploadInfo returns how many bytes of the session the server has committed.
func uploadInfo(ctx context.Context, f *Fs, uploadID int64) (*api.UploadInfoResponse, error) {
	opts := rest.Opts{
		Method:           "PUT",
		Path:             "/upload_info",
		Parameters:       url.Values{},
		TransferEncoding: []string{"identity"}, // pcloud doesn't like chunked encoding
	}
	opts.Parameters.Set("uploadid", strconv.FormatInt(uploadID, 10))

	result := &api.UploadInfoResponse{}
	err := f.pacer.Call(func() (bool, error) {
		resp, err := f.srv.CallJSON(ctx, &opts, nil, result)
		err = result.Error.Update(err)
		return shouldRetry(ctx, resp, err)
	})
	if err != nil {
		return nil, fmt.Errorf("upload info for session %d: %w", uploadID, err)
	}
	return result, nil
}

// uploadSave turns a completed session into the file leaf in directoryID.
// leaf must already be encoded with opt.Enc.
func uploadSave(ctx context.Context, f *Fs, uploadID int64, directoryID, leaf string, modTime time.Time) (*api.UploadSaveResponse, error) {
	opts := rest.Opts{
		Method:           "PUT",
		Path:             "/upload_save",
		Parameters:       url.Values{},
		TransferEncoding: []string{"identity"}, // pcloud doesn't like chunked encoding
	}
	opts.Parameters.Set("uploadid", strconv.FormatInt(uploadID, 10))
	opts.Parameters.Set("folderid", dirIDtoNumber(directoryID))
	opts.Parameters.Set("name", leaf)
	opts.Parameters.Set("mtime", fmt.Sprintf("%d", uint64(modTime.Unix())))

	result := &api.UploadSaveResponse{}
	err := f.pacer.Call(func() (bool, error) {
		resp, err := f.srv.CallJSON(ctx, &opts, nil, result)
		err = result.Error.Update(err)
		return shouldRetry(ctx, resp, err)
	})
	if err != nil {
		return nil, fmt.Errorf("save upload session %d: %w", uploadID, err)
	}
	return result, nil
}

// uploadDelete discards an unfinished session.
func uploadDelete(ctx context.Context, f *Fs, uploadID int64) error {
	opts := rest.Opts{
		Method:           "PUT",
		Path:             "/upload_delete",
		Parameters:       url.Values{},
		TransferEncoding: []string{"identity"}, // pcloud doesn't like chunked encoding
	}
	opts.Parameters.Set("uploadid", strconv.FormatInt(uploadID, 10))

	result := &api.UploadDeleteResponse{}
	return f.pacer.Call(func() (bool, error) {
		resp, err := f.srv.CallJSON(ctx, &opts, nil, result)
		err = result.Error.Update(err)
		return shouldRetry(ctx, resp, err)
	})
}

// uploadChunk writes chunkSize bytes from chunk at offset in the upload
// session uploadID.
//
// chunk must be seekable as it is re-sent from wherever upload_info says the
// server got to if a retryable write fails part way.
func (o *Object) uploadChunk(ctx context.Context, uploadID, offset int64, chunk io.ReadSeeker, chunkSize int64, options ...fs.OpenOption) error {
	skip := int64(0)
	result := &api.UploadWriteResponse{}
	err := o.fs.pacer.Call(func() (bool, error) {
		toSend := chunkSize - skip
		opts := rest.Opts{
			Method:           "PUT",
			Path:             "/upload_write",
			Body:             chunk,
			ContentLength:    &toSend,
			Parameters:       url.Values{},
			TransferEncoding: []string{"identity"}, // pcloud doesn't like chunked encoding
			Options:          options,
		}
		opts.Parameters.Set("uploadid", strconv.FormatInt(uploadID, 10))
		opts.Parameters.Set("uploadoffset", strconv.FormatInt(offset+skip, 10))
		if _, seekErr := chunk.Seek(skip, io.SeekStart); seekErr != nil {
			return false, seekErr
		}
		resp, err := o.fs.srv.CallJSON(ctx, &opts, nil, result)
		err = result.Error.Update(err)
		if err == nil {
			return false, nil
		}
		retry, err := shouldRetry(ctx, resp, err)
		if !retry {
			return false, err
		}
		info, infoErr := uploadInfo(ctx, o.fs, uploadID)
		if infoErr != nil {
			fs.Debugf(o, "Failed to read upload position: %v", infoErr)
			return true, err
		}
		newSkip := info.Size - offset
		fs.Debugf(o, "Write failed, server has %d bytes, chunk is %d..%d, bytes to skip = %d", info.Size, offset, offset+chunkSize, newSkip)
		switch {
		case newSkip < 0:
			return false, fmt.Errorf("sent block already (skip %d < 0), can't rewind: %w", newSkip, err)
		case newSkip > chunkSize:
			return false, fmt.Errorf("position is in the future (skip %d > chunk size %d), can't skip forward: %w", newSkip, chunkSize, err)
		case newSkip == chunkSize:
			fs.Debugf(o, "Skipping chunk as already sent (skip %d == chunk size %d)", newSkip, chunkSize)
			return false, nil
		}
		skip = newSkip
		return true, fmt.Errorf("retry this chunk skipping %d bytes: %w", skip, err)
	})
	if err != nil {
		return fmt.Errorf("write %d bytes at offset %d: %w", chunkSize, offset, err)
	}
	return nil
}

// uploadSession uploads in to the session uploadID, saves it as leaf in
// directoryID and sets o's metadata from the result.
//
// src.Size() must be >= 0 and leaf must already be encoded with opt.Enc. The
// session is deleted if the upload fails before it is saved.
func (o *Object) uploadSession(ctx context.Context, in io.Reader, src fs.ObjectInfo, leaf, directoryID string, uploadID int64, options ...fs.OpenOption) (err error) {
	size := src.Size()
	modTime := src.ModTime(ctx)
	fs.Debugf(o, "Starting resumable upload session %d", uploadID)

	saved := false
	defer atexit.OnError(&err, func() {
		if saved {
			return
		}
		fs.Debugf(o, "Cancelling upload session %d: %v", uploadID, err)
		if delErr := uploadDelete(ctx, o.fs, uploadID); delErr != nil {
			fs.Logf(o, "Failed to cancel upload session %d: %v (upload failed due to: %v)", uploadID, delErr, err)
		}
	})()

	chunkSize := int64(o.fs.opt.ChunkSize)
	position := int64(0)
	for position < size {
		n := min(size-position, chunkSize)
		// Buffer the chunk in memory from the global pool so it can be
		// re-sent, or partly re-sent, on retry
		rw := multipart.NewRW()
		_, err = io.CopyN(rw, in, n)
		if err != nil {
			_ = rw.Close()
			if err == io.EOF {
				err = fmt.Errorf("expected %d bytes in input, but got %d: %w", size, position, io.ErrUnexpectedEOF)
			}
			return err
		}
		fs.Debugf(o, "Uploading chunk at %d/%d size %d", position, size, n)
		err = o.uploadChunk(ctx, uploadID, position, rw, n, options...)
		closeErr := rw.Close()
		if err != nil {
			return err
		}
		if closeErr != nil {
			return closeErr
		}
		position += n
	}

	result, err := uploadSave(ctx, o.fs, uploadID, directoryID, leaf, modTime)
	if err != nil {
		return err
	}
	saved = true

	// pcloud can report a stale size straight after upload_save, so re-read the
	// metadata until it settles.
	info := &result.Metadata
	for retry := 0; info.Size != size && retry < 5; retry++ {
		fs.Debugf(o, "Size is %d, want %d: re-reading metadata, try %d/5", info.Size, size, retry+1)
		time.Sleep(time.Second)
		info, err = o.fs.readMetaDataForPath(ctx, o.remote)
		if err != nil {
			return fmt.Errorf("read metadata after upload: %w", err)
		}
	}
	if info.Size != size {
		return fmt.Errorf("incorrect size after upload: got %d, want %d", info.Size, size)
	}
	// upload_save does not return checksums, so drop any held for the previous
	// contents and let Hash fetch them again.
	o.md5, o.sha1, o.sha256 = "", "", ""
	return o.setMetaData(info)
}
