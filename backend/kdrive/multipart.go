package kdrive

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/url"
	"path"
	"strconv"
	"strings"
	"sync"

	"github.com/rclone/rclone/backend/kdrive/api"
	"github.com/rclone/rclone/backend/kdrive/khash"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/chunksize"
	"github.com/rclone/rclone/lib/rest"
	"github.com/zeebo/xxh3"
	"golang.org/x/text/unicode/norm"
)

const (
	// maxUploadParts is the maximum number of chunks the kDrive API accepts per file
	maxUploadParts = 10000

	// maxChunkSize is the largest chunk size the kDrive API accepts
	maxChunkSize = 1 * 1000 * 1000 * 1000
)

// uploadSession implements fs.ChunkWriter for kdrive multipart uploads
type uploadSession struct {
	f         *Fs
	token     string
	uploadURL string
	fileInfo  *api.Item
	fileSize  int64 // total size of the file being uploaded
	chunkSize int64 // size of the chunks of this session

	mu          sync.Mutex
	chunkHashes map[int64]string // chunk number -> hash of the chunk contents
}

// OpenChunkWriter returns the chunk size and a ChunkWriter
//
// Pass in the remote and the src object
// You can also use options to hint at the desired chunk size
// @see https://developer.infomaniak.com/docs/api/post/3/drive/%7Bdrive_id%7D/upload/session/start
func (f *Fs) OpenChunkWriter(ctx context.Context, remote string, src fs.ObjectInfo, options ...fs.OpenOption) (info fs.ChunkWriterInfo, writer fs.ChunkWriter, err error) {
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

	// Calculate the chunk size, honouring any fs.ChunkOption hint
	chunkSize := f.opt.ChunkSize
	for _, opt := range options {
		if chunkOpt, ok := opt.(*fs.ChunkOption); ok && chunkOpt.ChunkSize > 0 {
			chunkSize = fs.SizeSuffix(chunkOpt.ChunkSize)
		}
	}
	chunkSize = chunksize.Calculator(src, fileSize, maxUploadParts, chunkSize)
	if chunkSize > maxChunkSize {
		chunkSize = maxChunkSize
	}

	// Calculate the total number of chunks
	totalChunks := max((fileSize+int64(chunkSize)-1)/int64(chunkSize), 1)

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
		FileName:       f.opt.Enc.FromStandardName(norm.NFC.String(leaf)),
		LastModifiedAt: fmt.Sprintf("%d", uint64(src.ModTime(ctx).Unix())),
		TotalChunks:    totalChunks,
		TotalSize:      fileSize,
	}

	// https://developer.infomaniak.com/docs/api/post/3/drive/%7Bdrive_id%7D/upload/session/start
	opts := rest.Opts{
		Method: "POST",
		Path:   fmt.Sprintf("/3/drive/%s/upload/session/start", f.opt.DriveID),
	}
	var sessionResp api.SessionStartResponse
	_, err = f.srv.CallJSON(ctx, &opts, &sessionReq, &sessionResp)
	if err != nil {
		return info, nil, fmt.Errorf("failed to start upload session: %w", err)
	}

	info = fs.ChunkWriterInfo{
		ChunkSize:   int64(chunkSize),
		Concurrency: f.opt.UploadConcurrency,
	}

	writer = &uploadSession{
		f:           f,
		token:       sessionResp.Data.Token,
		uploadURL:   sessionResp.Data.UploadURL,
		fileSize:    fileSize,
		chunkSize:   int64(chunkSize),
		chunkHashes: make(map[int64]string),
	}

	fs.Debugf(f, "open chunk writer: started upload session for %d chunks of %v", totalChunks, chunkSize)
	return info, writer, nil
}

// WriteChunk uploads a single chunk
// @see https://developer.infomaniak.com/docs/api/post/3/drive/%7Bdrive_id%7D/upload/session/%7Bsession_token%7D/chunk
func (u *uploadSession) WriteChunk(ctx context.Context, chunkNumber int, reader io.ReadSeeker) (bytesWritten int64, err error) {
	if chunkNumber < 0 {
		return -1, fmt.Errorf("invalid chunk number provided: %v", chunkNumber)
	}

	// Calculate the size of this chunk from the session parameters so
	// the chunk can be streamed to the API in a single pass
	chunkLen := u.chunkSize
	if remaining := u.fileSize - int64(chunkNumber)*u.chunkSize; remaining < chunkLen {
		chunkLen = remaining
	}
	if chunkLen <= 0 {
		return 0, nil
	}

	sourceChunkNumber := chunkNumber + 1 // kDrive API uses 1-based numbering

	// The chunk_hash parameter is optional, so the chunk is streamed
	// in a single pass, hashing the contents on the way with a
	// TeeReader. The server returns the hash of the chunk it received,
	// which is checked below.
	chunkHasher := xxh3.New()

	// https://developer.infomaniak.com/docs/api/post/3/drive/%7Bdrive_id%7D/upload/session/%7Bsession_token%7D/chunk
	chunkOpts := rest.Opts{
		Method:  "POST",
		RootURL: u.uploadURL,
		Path:    fmt.Sprintf("/3/drive/%s/upload/session/%s/chunk", u.f.opt.DriveID, u.token),
		Parameters: url.Values{
			"chunk_number": {fmt.Sprintf("%d", sourceChunkNumber)},
			"chunk_size":   {fmt.Sprintf("%d", chunkLen)},
			"with":         {"hash"},
		},
		Body:          io.TeeReader(reader, chunkHasher),
		ContentLength: &chunkLen,
	}

	var chunkResp api.ChunkUploadResponse
	_, err = u.f.srv.CallJSON(ctx, &chunkOpts, nil, &chunkResp)
	if err != nil {
		return -1, fmt.Errorf("failed to upload chunk %d: %w", sourceChunkNumber, err)
	}

	chunkHash := hex.EncodeToString(chunkHasher.Sum(nil))

	u.mu.Lock()
	u.chunkHashes[int64(chunkNumber)] = chunkHash
	u.mu.Unlock()

	// The server hashed the chunk it received, so compare with what we
	// sent to detect any corruption immediately
	if chunkResp.Data.Hash != "" {
		if serverHash, _, parseErr := khash.ParseHash(chunkResp.Data.Hash); parseErr == nil {
			if !strings.EqualFold(chunkHash, serverHash) {
				return -1, fmt.Errorf("chunk %d hash mismatch: client=%s, server=%s", sourceChunkNumber, chunkHash, serverHash)
			}
		} else {
			fs.Debugf(u, "Failed to parse server chunk hash %q: %v", chunkResp.Data.Hash, parseErr)
		}
	}

	fs.Debugf(u, "uploaded chunk %d (size: %d, hash: %s)", sourceChunkNumber, chunkLen, chunkHash)
	return chunkLen, nil
}

// Close finalizes the upload session, verifies the hash of the uploaded
// file and returns the created file info
// @see https://developer.infomaniak.com/docs/api/post/3/drive/%7Bdrive_id%7D/upload/session/%7Bsession_token%7D/finish
func (u *uploadSession) Close(ctx context.Context) error {
	// https://developer.infomaniak.com/docs/api/post/3/drive/%7Bdrive_id%7D/upload/session/%7Bsession_token%7D/finish
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

	return u.verifyHash(ctx)
}

// verifyHash checks the hash of the uploaded file against the hashes of
// the chunks we uploaded, deleting the file if it is corrupted
func (u *uploadSession) verifyHash(ctx context.Context) (err error) {
	remoteHash := u.fileInfo.Hash
	if remoteHash == "" {
		// Hash might be missing in the finish response, try to fetch it
		obj := &Object{
			fs: u.f,
			id: strconv.Itoa(u.fileInfo.ID),
		}
		var hashErr error
		if remoteHash, hashErr = obj.retrieveHash(ctx); hashErr != nil {
			// skip verification
			fs.Debugf(u, "Failed to retrieve hash for verification: %v", hashErr)
			return nil
		}
	}
	parsedRemoteHash, isNested, err := khash.ParseHash(remoteHash)
	if err != nil {
		fs.Debugf(u, "Failed to parse server hash %q: %v", remoteHash, err)
		return nil
	}

	u.mu.Lock()
	defer u.mu.Unlock()

	// Rebuild the hash the server should have stored from the chunk
	// hashes we uploaded, in order
	chunkCount := int64(len(u.chunkHashes))
	if chunkCount == 0 {
		return nil
	}
	chunkHashes := make([]string, 0, chunkCount)
	for i := int64(0); i < chunkCount; i++ {
		chunkHash, ok := u.chunkHashes[i]
		if !ok {
			fs.Debugf(u, "Chunk %d hash missing, skipping hash verification", i)
			return nil
		}
		chunkHashes = append(chunkHashes, chunkHash)
	}

	var localHash string
	if isNested {
		if localHash, err = khash.NestedChunkHash(chunkHashes); err != nil {
			fs.Debugf(u, "Failed to compute nested hash: %v", err)
			return nil
		}
	} else if chunkCount == 1 {
		localHash, _, _ = khash.ParseHash(chunkHashes[0])
	} else {
		// The server returned a simple hash for a multi chunk upload,
		// nothing to compare it against
		fs.Debugf(u, "Server returned a simple hash for a %d chunk upload, skipping hash verification", chunkCount)
		return nil
	}

	if !strings.EqualFold(localHash, parsedRemoteHash) {
		err = fmt.Errorf(
			"multipart upload hash mismatch: local=%s, remote=%s",
			localHash, parsedRemoteHash,
		)
		fs.Errorf(u, "%v", err)

		// Remove the corrupted file
		obj := &Object{
			fs: u.f,
			id: strconv.Itoa(u.fileInfo.ID),
		}
		if delErr := obj.Remove(ctx); delErr != nil {
			fs.Errorf(nil, "Failed to remove corrupted file after hash mismatch: %v", delErr)
		} else {
			fs.Debugf(nil, "Removed corrupted file after hash mismatch")
		}

		return err
	}
	fs.Debugf(u, "Multipart upload hash verified: %s", localHash)

	return nil
}

// Abort the upload session
// @see  https://developer.infomaniak.com/docs/api/delete/2/drive/%7Bdrive_id%7D/upload/session/%7Bsession_token%7D
func (u *uploadSession) Abort(ctx context.Context) error {
	// https://developer.infomaniak.com/docs/api/delete/2/drive/%7Bdrive_id%7D/upload/session/%7Bsession_token%7D
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

// Verify interface compliance
var _ fs.ChunkWriter = (*uploadSession)(nil)
