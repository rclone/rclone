package filelu

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"

	"github.com/rclone/rclone/fs"
	rclonemultipart "github.com/rclone/rclone/lib/multipart"
	"github.com/rclone/rclone/lib/rest"
)

// multipartUpload uploads a file in fixed-size chunks using the multipart API.
func (f *Fs) multipartUpload(ctx context.Context, in io.Reader, remote string) error {
	dir := path.Dir(remote)
	if dir == "." {
		dir = ""
	}

	if dir != "" {
		if _, err := f.createFolder(ctx, dir); err != nil {
			return fmt.Errorf("failed to create multipart folder: %w", err)
		}
	}

	folder := strings.Trim(dir, "/")
	if folder != "" {
		folder = "/" + folder
	}

	file := path.Base(remote)

	initResp, err := f.multipartInit(ctx, folder, file)
	if err != nil {
		return fmt.Errorf("multipart init failed: %w", err)
	}

	uploadID := initResp.Result.UploadID
	sessID := initResp.Result.SessID
	server := initResp.Result.Server
	objectPath := initResp.Result.ObjectPath

	chunkSize := int64(f.opt.ChunkSize)
	if chunkSize <= 0 {
		return fmt.Errorf("multipart upload: chunk_size must be positive: %v", f.opt.ChunkSize)
	}
	for partNo := 1; ; partNo++ {
		// Buffer the part in memory from the global pool so it can be
		// re-sent on retry
		rw := rclonemultipart.NewRW()
		n, err := io.CopyN(rw, in, chunkSize)
		if err != nil && err != io.EOF {
			_ = rw.Close()
			return fmt.Errorf("read failed: %w", err)
		}
		if n > 0 {
			uploadErr := f.uploadPart(ctx, server, uploadID, sessID, objectPath, partNo, rw, n)
			if uploadErr != nil {
				_ = rw.Close()
				return fmt.Errorf("upload part %d failed: %w", partNo, uploadErr)
			}
		}
		if closeErr := rw.Close(); closeErr != nil {
			return closeErr
		}
		if err == io.EOF {
			break
		}
	}

	err = f.completeMultipart(ctx, server, uploadID, sessID, objectPath)
	if err != nil {
		return fmt.Errorf("complete multipart failed: %w", err)
	}

	return nil
}

// uploadPart sends a single multipart chunk of size bytes to the upload server.
func (f *Fs) uploadPart(ctx context.Context, server, uploadID, sessID, objectPath string, partNo int, r io.ReadSeeker, size int64) error {
	opts := rest.Opts{
		Method:  "PUT",
		RootURL: server,
		Parameters: url.Values{
			"partNumber": {strconv.Itoa(partNo)},
			"uploadId":   {uploadID},
		},
		Body:          r,
		ContentLength: &size,
		ExtraHeaders: map[string]string{
			"X-RC-Upload-Id": uploadID,
			"X-RC-Part-No":   strconv.Itoa(partNo),
			"X-Sess-ID":      sessID,
			"X-Object-Path":  objectPath,
		},
		NoResponse: true,
	}
	return f.pacer.Call(func() (bool, error) {
		// rewind the pooled buffer so each attempt sends the whole chunk
		_, err := r.Seek(0, io.SeekStart)
		if err != nil {
			return false, err
		}
		resp, err := f.srv.Call(ctx, &opts)
		if err != nil {
			if resp != nil {
				return shouldRetryHTTP(resp.StatusCode), fmt.Errorf("uploadPart failed: %w", err)
			}
			return shouldRetry(err), err
		}
		return false, nil
	})
}

// uploadFile uploads a file to FileLu
func (f *Fs) uploadFile(ctx context.Context, fileContent io.Reader, fileFullPath string) error {
	directory := path.Dir(fileFullPath)
	fileName := path.Base(fileFullPath)
	if directory == "." {
		directory = ""
	}
	destinationFolderPath := path.Join(f.root, directory)
	if destinationFolderPath != "" {
		destinationFolderPath = "/" + strings.Trim(destinationFolderPath, "/")
	}

	existingEntries, err := f.List(ctx, path.Dir(fileFullPath))
	if err != nil {
		if errors.Is(err, fs.ErrorDirNotFound) {
			err = f.Mkdir(ctx, path.Dir(fileFullPath))
			if err != nil {
				return fmt.Errorf("failed to create directory: %w", err)
			}
		} else {
			return fmt.Errorf("failed to list existing files: %w", err)
		}
	}

	for _, entry := range existingEntries {
		if entry.Remote() == fileFullPath {
			_, ok := entry.(fs.Object)
			if !ok {
				continue
			}

			// If the file exists but is different, remove it
			filePath := "/" + strings.Trim(destinationFolderPath+"/"+fileName, "/")
			err = f.deleteFile(ctx, filePath)
			if err != nil {
				return fmt.Errorf("failed to delete existing file: %w", err)
			}
		}
	}

	uploadURL, sessID, err := f.getUploadServer(ctx)
	if err != nil {
		return fmt.Errorf("failed to retrieve upload server: %w", err)
	}

	// Since the fileCode isn't used, just handle the error
	if _, err := f.uploadFileWithDestination(ctx, uploadURL, sessID, fileName, fileContent, destinationFolderPath); err != nil {
		return fmt.Errorf("failed to upload file: %w", err)
	}

	return nil
}

func (f *Fs) getUploadServer(ctx context.Context) (string, string, error) {

	opts := rest.Opts{
		Method: "GET",
		Path:   "/upload/server",
		Parameters: url.Values{
			"key": {f.opt.Key},
		},
	}

	var result struct {
		Status int    `json:"status"`
		SessID string `json:"sess_id"`
		Result string `json:"result"`
		Msg    string `json:"msg"`
	}

	err := f.pacer.Call(func() (bool, error) {
		_, err := f.srv.CallJSON(ctx, &opts, nil, &result)
		if err != nil {
			return shouldRetry(err), fmt.Errorf("failed to get upload server: %w", err)
		}
		return false, nil
	})

	if err != nil {
		return "", "", err
	}

	if result.Status != 200 {
		return "", "", fmt.Errorf("API error: %s", result.Msg)
	}

	return result.Result, result.SessID, nil
}

// uploadFileWithDestination uploads a file directly to a specified folder using file content reader.
func (f *Fs) uploadFileWithDestination(ctx context.Context, uploadURL, sessID, fileName string, fileContent io.Reader, dirPath string) (string, error) {
	destinationPath := f.fromStandardPath(dirPath)
	encodedFileName := f.fromStandardPath(fileName)
	pr, pw := io.Pipe()
	writer := multipart.NewWriter(pw)
	isDeletionRequired := false
	go func() {
		defer func() {
			if err := pw.Close(); err != nil {
				fs.Logf(nil, "Failed to close: %v", err)
			}
		}()
		_ = writer.WriteField("sess_id", sessID)
		_ = writer.WriteField("utype", "prem")
		_ = writer.WriteField("fld_path", destinationPath)

		part, err := writer.CreateFormFile("file_0", encodedFileName)
		if err != nil {
			pw.CloseWithError(fmt.Errorf("failed to create form file: %w", err))
			return
		}

		if _, err := io.Copy(part, fileContent); err != nil {
			isDeletionRequired = true
			pw.CloseWithError(fmt.Errorf("failed to copy file content: %w", err))
			return
		}

		if err := writer.Close(); err != nil {
			pw.CloseWithError(fmt.Errorf("failed to close writer: %w", err))
		}
	}()

	var fileCode string
	err := f.pacer.Call(func() (bool, error) {
		req, err := http.NewRequestWithContext(ctx, "POST", uploadURL, pr)
		if err != nil {
			return false, fmt.Errorf("failed to create upload request: %w", err)
		}
		req.Header.Set("Content-Type", writer.FormDataContentType())

		resp, err := f.client.Do(req)
		if err != nil {
			return shouldRetry(err), fmt.Errorf("failed to send upload request: %w", err)
		}
		defer respBodyClose(resp.Body)

		var result []struct {
			FileCode   string `json:"file_code"`
			FileStatus string `json:"file_status"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			return false, fmt.Errorf("failed to parse upload response: %w", err)
		}

		if len(result) == 0 || result[0].FileStatus != "OK" {
			return false, fmt.Errorf("upload failed with status: %s", result[0].FileStatus)
		}

		fileCode = result[0].FileCode
		return shouldRetryHTTP(resp.StatusCode), nil
	})

	if err != nil && isDeletionRequired {
		// Attempt to delete the file if upload fails
		_ = f.deleteFile(ctx, destinationPath+"/"+fileName)
	}

	return fileCode, err
}

// respBodyClose to check body response.
func respBodyClose(responseBody io.Closer) {
	if cerr := responseBody.Close(); cerr != nil {
		fmt.Printf("Error closing response body: %v\n", cerr)
	}
}
