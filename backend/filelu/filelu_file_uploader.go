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
	"strings"

	"github.com/rclone/rclone/fs"
)

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

// getUploadServer gets the upload server URL with proper key authentication
func (f *Fs) getUploadServer(ctx context.Context) (string, string, error) {
	apiURL := fmt.Sprintf("%s/upload/server?key=%s", f.endpoint, url.QueryEscape(f.opt.Key))

	var result struct {
		Status int    `json:"status"`
		SessID string `json:"sess_id"`
		Result string `json:"result"`
		Msg    string `json:"msg"`
	}

	err := f.pacer.Call(func() (bool, error) {
		req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
		if err != nil {
			return false, fmt.Errorf("failed to create request: %w", err)
		}

		resp, err := f.client.Do(req)
		if err != nil {
			return shouldRetry(err), fmt.Errorf("failed to get upload server: %w", err)
		}
		defer func() {
			if err := resp.Body.Close(); err != nil {
				fs.Logf(nil, "Failed to close response body: %v", err)
			}
		}()

		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			return false, fmt.Errorf("error decoding response: %w", err)
		}

		if result.Status != 200 {
			return false, fmt.Errorf("API error: %s", result.Msg)
		}

		return shouldRetryHTTP(resp.StatusCode), nil
	})

	if err != nil {
		return "", "", err
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
