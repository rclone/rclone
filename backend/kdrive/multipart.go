//go:build !plan9 && !js

package kdrive

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"net/url"
	"path"
	"strings"

	"github.com/rclone/rclone/backend/kdrive/api"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/lib/rest"
	"github.com/zeebo/xxh3"
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
	hash       string // Hash from the last chunk upload
}

const (
	maxChunkSize     = 1 * 1024 * 1024 * 1024 // 1 Go (max API)
	defaultChunkSize = 20 * 1024 * 1024       // 20 Mo
	maxChunks        = 10000                  // Limit API
	mebi             = 1024 * 1024
)

func calculateChunkSize(fileSize int64, preferredChunkSize int64) int64 {
	// Use preferred chunk size
	chunkSize := preferredChunkSize
	if chunkSize <= 0 {
		chunkSize = defaultChunkSize
	}

	// Round to greater MiB
	if chunkSize%mebi != 0 {
		chunkSize += mebi - (chunkSize % mebi)
	}

	// Limit chunk size to 1 Go
	if chunkSize > maxChunkSize {
		chunkSize = maxChunkSize
	}

	// For large files, use a bigger chunk size
	requiredChunks := calculateTotalChunks(fileSize, chunkSize)
	if requiredChunks > maxChunks {
		chunkSize = fileSize / maxChunks
		if fileSize%maxChunks != 0 {
			chunkSize++
		}

		// Round to greater MiB
		if chunkSize%mebi != 0 {
			chunkSize += mebi - (chunkSize % mebi)
		}

		// Limit chunk size to 1 Go
		if chunkSize > maxChunkSize {
			chunkSize = maxChunkSize
		}
	}

	return chunkSize
}

func calculateTotalChunks(fileSize int64, chunkSize int64) int64 {
	totalChunks := math.Ceil(float64(fileSize) / float64(chunkSize))

	return int64(totalChunks)
}

// OpenChunkWriter returns chunk writer info and the upload session
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

	var preferredChunkSize int64
	for _, opt := range options {
		if chunkOpt, ok := opt.(*fs.ChunkOption); ok {
			preferredChunkSize = chunkOpt.ChunkSize
		}
	}

	chunkSize := calculateChunkSize(fileSize, preferredChunkSize)
	totalChunks := calculateTotalChunks(fileSize, chunkSize)

	sessionReq := struct {
		Conflict    string `json:"conflict"`
		DirectoryID string `json:"directory_id"`
		FileName    string `json:"file_name"`
		TotalChunks int64  `json:"total_chunks"`
		TotalSize   int64  `json:"total_size"`
	}{
		Conflict:    "version",
		DirectoryID: parentID,
		FileName:    leaf,
		TotalChunks: totalChunks,
		TotalSize:   fileSize,
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
		serverHash := strings.TrimPrefix(chunkResp.Data.Hash, "xxh3:")
		clientHash := strings.TrimPrefix(chunkHash, "xxh3:")
		if serverHash != clientHash {
			fs.Debugf(u, "chunk %d hash mismatch: client=%s, server=%s", sourceChunkNumber, clientHash, serverHash)
		}
		u.hash = serverHash
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
