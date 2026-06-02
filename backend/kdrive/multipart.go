//go:build !plan9 && !js

package kdrive

import (
	"bytes"
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"hash"
	"io"
	"net/url"
	"path"
	"strconv"
	"strings"

	"github.com/rclone/rclone/backend/kdrive/api"
	"github.com/rclone/rclone/backend/kdrive/chunksize"
	"github.com/rclone/rclone/backend/kdrive/khash"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/accounting"
	"github.com/rclone/rclone/lib/atexit"
	"github.com/rclone/rclone/lib/pacer"
	"github.com/rclone/rclone/lib/pool"
	"github.com/rclone/rclone/lib/rest"
	"github.com/zeebo/xxh3"
	"golang.org/x/sync/errgroup"
)

// uploadSession implements fs.ChunkWriter for kdrive multipart uploads
type uploadSession struct {
	f          *Fs
	parentID   string
	fileName   string
	token      string
	uploadURL  string
	fileInfo   *api.Item
	chunkCount int
}

// newChunkWriter returns chunk writer info and the upload session
// @see https://developer.infomaniak.com/docs/api/post/3/drive/%7Bdrive_id%7D/upload/session/start
func (f *Fs) newChunkWriter(ctx context.Context, remote string, src fs.ObjectInfo, options ...fs.OpenOption) (info fs.ChunkWriterInfo, writer fs.ChunkWriter, err error) {
	fileSize := src.Size()
	if fileSize < 0 {
		return info, nil, errors.New("kdrive can't upload files with unknown size")
	}

	dir, leaf := path.Split(remote)
	dir = strings.TrimSuffix(dir, "/")

	// Create parent directories if they don't exist
	parentID, err := f.dirCache.FindDir(ctx, dir, true)
	if err != nil {
		return info, nil, fmt.Errorf("failed to find parent directory: %w", err)
	}

	var preferredChunkSize int64
	for _, opt := range options {
		if chunkOpt, ok := opt.(*fs.ChunkOption); ok {
			preferredChunkSize = chunkOpt.ChunkSize
		}
	}

	chunkSize := chunksize.CalculateChunkSize(fileSize, preferredChunkSize)
	totalChunks := chunksize.CalculateTotalChunks(fileSize, chunkSize)
	lastModifiedAt := fmt.Sprintf("%d", uint64(src.ModTime(ctx).Unix()))

	sessionReq := struct {
		Conflict       string `json:"conflict"`
		DirectoryID    string `json:"directory_id"`
		FileName       string `json:"file_name"`
		LastModifiedAt string `json:"last_modified_at"`
		TotalChunks    int64  `json:"total_chunks"`
		TotalSize      int64  `json:"total_size"`
	}{
		Conflict:       "version",
		DirectoryID:    parentID,
		FileName:       leaf,
		LastModifiedAt: lastModifiedAt,
		TotalChunks:    totalChunks,
		TotalSize:      fileSize,
	}

	opts := rest.Opts{
		Method: "POST",
		Path:   fmt.Sprintf("/3/drive/%s/upload/session/start", f.opt.DriveID),
	}
	var sessionResp api.SessionStartResponse
	_, err = f.srv.CallJSON(ctx, &opts, &sessionReq, &sessionResp)
	if err != nil {
		fs.Debugf(nil, "REQUEST : %s %w", opts.Path, &sessionReq)
		return info, nil, fmt.Errorf("failed to start upload session: %w", err)
	}

	chunkWriter := &uploadSession{
		f:         f,
		parentID:  parentID,
		fileName:  leaf,
		token:     sessionResp.Data.Token,
		uploadURL: sessionResp.Data.UploadURL,
	}

	info = fs.ChunkWriterInfo{
		ChunkSize:   chunkSize,
		Concurrency: 4,
	}

	fs.Debugf(&Object{fs: f, remote: remote}, "open chunk writer: started upload session: %v", sessionResp.Data.Token)
	return info, chunkWriter, nil
}

// WriteChunk uploads a single chunk
// @see https://developer.infomaniak.com/docs/api/post/3/drive/%7Bdrive_id%7D/upload/session/%7Bsession_token%7D/chunk
func (u *uploadSession) WriteChunk(ctx context.Context, chunkNumber int, reader io.ReadSeeker) (bytesWritten int64, err error) {
	if chunkNumber < 0 {
		return -1, fmt.Errorf("invalid chunk number provided: %v", chunkNumber)
	}

	// Read the chunk data
	var buf bytes.Buffer
	n, err := io.Copy(&buf, reader)
	if err != nil {
		return -1, fmt.Errorf("failed to read chunk data: %w", err)
	}

	if n == 0 {
		return 0, nil
	}

	chunkData := buf.Bytes()
	sourceChunkNumber := chunkNumber + 1 // KDive API uses 1-based numbering

	// Calculate chunk hash
	chunkHasher := xxh3.New()
	_, _ = chunkHasher.Write(chunkData)
	chunkHash := fmt.Sprintf("xxh3:%x", chunkHasher.Sum(nil))

	uploadPath := fmt.Sprintf("/3/drive/%s/upload/session/%s/chunk", u.f.opt.DriveID, u.token)
	chunkOpts := rest.Opts{
		Method:  "POST",
		RootURL: u.uploadURL,
		Path:    uploadPath,
		Parameters: url.Values{
			"chunk_number": {fmt.Sprintf("%d", sourceChunkNumber)},
			"chunk_size":   {fmt.Sprintf("%d", n)},
			"with":         {"hash"},
			"chunk_hash":   {chunkHash},
		},
		Body: bytes.NewReader(chunkData),
	}

	var chunkResp api.ChunkUploadResponse
	_, err = u.f.srv.CallJSON(ctx, &chunkOpts, nil, &chunkResp)
	if err != nil {
		return -1, fmt.Errorf("failed to upload chunk %d: %w", sourceChunkNumber, err)
	}

	// Verify server returned matching hash (optional but good for debugging)
	if chunkResp.Data.Hash != "" {
		serverHash, _, _ := khash.ParseHash(chunkResp.Data.Hash)
		clientHash, _, _ := khash.ParseHash(chunkHash)
		if serverHash != clientHash {
			fs.Debugf(u, "chunk %d hash mismatch: client=%s, server=%s", sourceChunkNumber, clientHash, serverHash)
		}
	}

	u.chunkCount++
	fs.Debugf(u, "uploaded chunk %d (size: %d, hash: %s)", sourceChunkNumber, n, chunkHash)
	return n, nil
}

// Close finalizes the upload session and returns the created file info
// @see https://developer.infomaniak.com/docs/api/post/3/drive/%7Bdrive_id%7D/upload/session/%7Bsession_token%7D/finish
func (u *uploadSession) Close(ctx context.Context) error {
	opts := rest.Opts{
		Method: "POST",
		Path:   fmt.Sprintf("/3/drive/%s/upload/session/%s/finish", u.f.opt.DriveID, u.token),
	}
	var resp api.SessionFinishResponse
	_, err := u.f.srv.CallJSON(ctx, &opts, nil, &resp)
	if err != nil {
		return fmt.Errorf("failed to finish upload session: %w", err)
	}

	u.fileInfo = &resp.Data.File
	fs.Debugf(u, "multipart upload completed: file id %d", resp.Data.File.ID)
	return nil
}

// Abort the upload session
// @see  https://developer.infomaniak.com/docs/api/delete/2/drive/%7Bdrive_id%7D/upload/session/%7Bsession_token%7D
func (u *uploadSession) Abort(ctx context.Context) error {
	opts := rest.Opts{
		Method: "DELETE",
		Path:   fmt.Sprintf("/2/drive/%s/upload/session/%s", u.f.opt.DriveID, u.token),
	}
	var resp api.SessionCancelResponse
	_, err := u.f.srv.CallJSON(ctx, &opts, nil, &resp)
	if err != nil {
		fs.Debugf(u, "failed to cancel upload session: %v", err)
		return fmt.Errorf("failed to cancel upload session: %w", err)
	}

	fs.Debugf(u, "upload session cancelled")
	return nil
}

// String implements fmt.Stringer
func (u *uploadSession) String() string {
	return fmt.Sprintf("kdrive upload session %s", u.token)
}

// NewRW gets a pool.RW using the global pool
func NewRW() *pool.RW {
	return pool.NewRW(pool.Global())
}

// UploadMultipart does a generic multipart upload from src using f as newChunkWriter.
//
// in is read seqentially and chunks from it are uploaded in parallel.
//
// It returns the chunkWriter used in case the caller needs to extract any private info from it.
func (f *Fs) UploadMultipart(ctx context.Context, src fs.ObjectInfo, in io.Reader, opt []fs.OpenOption) (chunkWriterOut fs.ChunkWriter, err error) {
	info, chunkWriter, err := f.newChunkWriter(ctx, src.Remote(), src, opt...)
	if err != nil {
		return nil, fmt.Errorf("multipart upload failed to initialise: %w", err)
	}

	// make concurrency machinery
	concurrency := max(info.Concurrency, 1)
	tokens := pacer.NewTokenDispenser(concurrency)

	uploadCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	defer atexit.OnError(&err, func() {
		cancel()
		if info.LeavePartsOnError {
			return
		}
		fs.Debugf(src, "Cancelling multipart upload")
		errCancel := chunkWriter.Abort(ctx)
		if errCancel != nil {
			fs.Debugf(src, "Failed to cancel multipart upload: %v", errCancel)
		}
	})()

	var (
		g, gCtx   = errgroup.WithContext(uploadCtx)
		finished  = false
		off       int64
		size      = src.Size()
		chunkSize = info.ChunkSize
	)

	// Do the accounting manually
	in, acc := accounting.UnWrapAccounting(in)

	// Calculate hash as we read the input
	// We must use the same chunk size as the upload so the nested hash matches
	chunkHasher := khash.NewWithChunkSize(chunkSize)
	in = io.TeeReader(in, chunkHasher)

	for partNum := int64(0); !finished; partNum++ {
		// Get a block of memory from the pool and token which limits concurrency.
		tokens.Get()
		rw := NewRW().Reserve(chunkSize)
		if acc != nil {
			rw.SetAccounting(acc.AccountRead)
		}

		free := func() {
			// return the memory and token
			_ = rw.Close() // Can't return an error
			tokens.Put()
		}

		// Fail fast, in case an errgroup managed function returns an error
		// gCtx is cancelled. There is no point in uploading all the other parts.
		if gCtx.Err() != nil {
			free()
			break
		}

		// Read the chunk
		var n int64
		n, err = io.CopyN(rw, in, chunkSize)
		if err == io.EOF {
			if n == 0 && partNum != 0 { // end if no data and if not first chunk
				free()
				break
			}
			finished = true
		} else if err != nil {
			free()
			return nil, fmt.Errorf("multipart upload: failed to read source: %w", err)
		}

		partNum := partNum
		partOff := off
		off += n
		g.Go(func() (err error) {
			defer free()
			fs.Debugf(src, "multipart upload: starting chunk %d size %v offset %v/%v", partNum, fs.SizeSuffix(n), fs.SizeSuffix(partOff), fs.SizeSuffix(size))
			_, err = chunkWriter.WriteChunk(gCtx, int(partNum), rw)
			return err
		})
	}

	err = g.Wait()
	if err != nil {
		return nil, err
	}

	err = chunkWriter.Close(ctx)
	if err != nil {
		return nil, fmt.Errorf("multipart upload: failed to finalise: %w", err)
	}

	// Verify the hash
	if session, ok := chunkWriter.(*uploadSession); ok && session.fileInfo != nil {
		err = session.CheckHash(ctx, info, chunkHasher)
		if err != nil {
			return nil, err
		}
	}

	return chunkWriter, nil
}

func (u uploadSession) CheckHash(ctx context.Context, info fs.ChunkWriterInfo, chunkHasher hash.Hash) (err error) {
	remoteHash := u.fileInfo.Hash

	if remoteHash == "" {
		// Hash might be missing in finish response, try to fetch it
		obj := &Object{
			fs: u.f,
			id: strconv.Itoa(u.fileInfo.ID),
		}

		var hashErr error
		remoteHash, hashErr = obj.retrieveHash(ctx)
		if hashErr != nil {
			// skip validation
			fs.Debugf(u, "Failed to retrieve hash for verification: %v", hashErr)
			return nil
		}
	}

	// validate with khash only if it's a nested hash
	if !khash.IsNestedHash(remoteHash) {
		return nil
	}

	localHash := hex.EncodeToString(chunkHasher.Sum(nil))

	if valid, _ := khash.ValidateHash(localHash, remoteHash); !valid {
		err = fmt.Errorf(
			"multipart upload hash mismatch: using chunk size %v, local=%s, remote=%s",
			fs.SizeSuffix(info.ChunkSize), localHash, remoteHash,
		)
		fs.Errorf(u, "%v", err)

		// Attempt to remove the corrupted file
		// We construct a minimal Object just for removal
		obj := &Object{
			fs: u.f,
			id: strconv.Itoa(u.fileInfo.ID),
		}
		delErr := obj.Remove(ctx)
		if delErr != nil {
			fs.Errorf(nil, "Failed to remove corrupted file after hash mismatch: %v", delErr)
		} else {
			fs.Debugf(nil, "Removed corrupted file after hash mismatch")
		}

		return err
	}
	fs.Debugf(u, "Multipart upload hash verified: %s", localHash)

	// update remote hash with local hash to pass checkHashes
	u.fileInfo.Hash = localHash

	return nil
}
