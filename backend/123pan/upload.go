package pan123

import (
	"bytes"
	"context"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/rclone/rclone/backend/123pan/api"
	"github.com/rclone/rclone/fs"
)

// upload uploads a file to 123pan
func (f *Fs) upload(ctx context.Context, in io.Reader, parentID int64, filename string, size int64, options ...fs.OpenOption) (*api.File, error) {
	// Read the entire file to calculate MD5 (required by 123pan API)
	// We need to buffer it because we need the MD5 before upload
	data, err := io.ReadAll(in)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	if int64(len(data)) != size {
		return nil, fmt.Errorf("file size mismatch: expected %d, got %d", size, len(data))
	}

	// Calculate MD5 hash
	hash := md5.Sum(data)
	etag := hex.EncodeToString(hash[:])

	// Create file (check for instant upload)
	createResp, err := f.createFile(ctx, parentID, filename, etag, size)
	if err != nil {
		return nil, err
	}

	fileID := createResp.Data.FileID

	// Check for instant upload (秒传)
	if createResp.Data.Reuse {
		fs.Debugf(f, "Instant upload succeeded for %s", filename)
	} else {
		// Need to upload the file
		if len(createResp.Data.Servers) == 0 {
			return nil, errors.New("no upload server returned")
		}

		// Upload slices
		err = f.uploadSlices(ctx, data, createResp.Data.Servers[0], createResp.Data.PreuploadID, createResp.Data.SliceSize)
		if err != nil {
			return nil, fmt.Errorf("failed to upload slices: %w", err)
		}

		// Complete upload with polling
		fileID = 0
		for i := 0; i < 60; i++ {
			completeResp, err := f.completeUpload(ctx, createResp.Data.PreuploadID)
			if err == nil && completeResp.Data.Completed && completeResp.Data.FileID != 0 {
				fileID = completeResp.Data.FileID
				break
			}
			time.Sleep(time.Second)
		}

		if fileID == 0 {
			return nil, errors.New("upload complete timeout")
		}
	}

	return &api.File{
		FileID:       fileID,
		Filename:     filename,
		Size:         size,
		Etag:         etag,
		ParentFileID: parentID,
		Type:         0,
		UpdateAt:     time.Now().Format("2006-01-02 15:04:05"),
	}, nil
}

// uploadSlices uploads file data in slices
func (f *Fs) uploadSlices(ctx context.Context, data []byte, uploadServer, preuploadID string, sliceSize int64) error {
	size := int64(len(data))
	numSlices := (size + sliceSize - 1) / sliceSize

	// Ensure upload server has correct scheme
	if !strings.HasPrefix(uploadServer, "http") {
		uploadServer = "http://" + uploadServer
	}

	for sliceNo := int64(1); sliceNo <= numSlices; sliceNo++ {
		offset := (sliceNo - 1) * sliceSize
		length := sliceSize
		if offset+length > size {
			length = size - offset
		}

		sliceData := data[offset : offset+length]

		// Calculate slice MD5
		sliceHash := md5.Sum(sliceData)
		sliceMD5 := hex.EncodeToString(sliceHash[:])

		err := f.uploadSlice(ctx, uploadServer, preuploadID, sliceNo, sliceMD5, sliceData)
		if err != nil {
			return fmt.Errorf("failed to upload slice %d: %w", sliceNo, err)
		}

		fs.Debugf(f, "Uploaded slice %d/%d", sliceNo, numSlices)
	}

	return nil
}

// uploadSlice uploads a single slice
func (f *Fs) uploadSlice(ctx context.Context, server, preuploadID string, sliceNo int64, sliceMD5 string, data []byte) (err error) {
	// Create multipart form
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)

	// Add form fields
	if err := w.WriteField("preuploadID", preuploadID); err != nil {
		return err
	}
	if err := w.WriteField("sliceNo", strconv.FormatInt(sliceNo, 10)); err != nil {
		return err
	}
	if err := w.WriteField("sliceMD5", sliceMD5); err != nil {
		return err
	}

	// Add file data
	fw, err := w.CreateFormFile("slice", fmt.Sprintf("part%d", sliceNo))
	if err != nil {
		return err
	}
	if _, err := fw.Write(data); err != nil {
		return err
	}

	if err := w.Close(); err != nil {
		return err
	}

	// Create request
	uploadURL := server + "/upload/v2/file/slice"
	req, err := http.NewRequestWithContext(ctx, "POST", uploadURL, &buf)
	if err != nil {
		return err
	}

	req.Header.Set("Authorization", "Bearer "+f.opt.AccessToken)
	req.Header.Set("Platform", "open_platform")
	req.Header.Set("Content-Type", w.FormDataContentType())

	// Send request
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer fs.CheckClose(resp.Body, &err)

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("slice upload failed with status %d: %s", resp.StatusCode, string(body))
	}

	// Parse response
	var apiResp api.BaseResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return err
	}

	if apiResp.Code != 0 {
		return fmt.Errorf("slice upload API error: %s", apiResp.Message)
	}

	return nil
}
