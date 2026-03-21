package filelu

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/rclone/rclone/backend/filelu/api"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/fserrors"
	"github.com/rclone/rclone/lib/rest"
)

// multipartInit starts a new multipart upload and returns server details.
func (f *Fs) multipartInit(ctx context.Context, folderPath, filename string) (*api.MultipartInitResponse, error) {
	opts := rest.Opts{
		Method: "GET",
		Path:   "/multipart/init",
		Parameters: url.Values{
			"key":         {f.opt.Key},
			"filename":    {filename},
			"folder_path": {folderPath},
		},
	}

	var result api.MultipartInitResponse

	err := f.pacer.Call(func() (bool, error) {
		_, err := f.srv.CallJSON(ctx, &opts, nil, &result)
		return fserrors.ShouldRetry(err), err
	})
	if err != nil {
		return nil, err
	}

	if result.Status != 200 {
		return nil, fmt.Errorf("multipart init error: %s", result.Msg)
	}

	return &result, nil
}

// completeMultipart finalizes the multipart upload on the file server.
func (f *Fs) completeMultipart(ctx context.Context, server string, uploadID string, sessID string, objectPath string) error {
	req, err := http.NewRequestWithContext(ctx, "POST", server, nil)
	if err != nil {
		return err
	}

	req.Header.Set("X-RC-Upload-Id", uploadID)
	req.Header.Set("X-Sess-ID", sessID)
	req.Header.Set("X-Object-Path", objectPath)

	resp, err := f.client.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != 202 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("completeMultipart failed %d: %s", resp.StatusCode, string(body))
	}
	return nil
}

// createFolder creates a folder at the specified path.
func (f *Fs) createFolder(ctx context.Context, dirPath string) (*api.CreateFolderResponse, error) {
	encodedDir := f.fromStandardPath(dirPath)

	opts := rest.Opts{
		Method: "GET",
		Path:   "/folder/create",
		Parameters: url.Values{
			"folder_path": {encodedDir},
			"key":         {f.opt.Key},
		},
	}

	var result api.CreateFolderResponse

	err := f.pacer.Call(func() (bool, error) {
		_, err := f.srv.CallJSON(ctx, &opts, nil, &result)
		return fserrors.ShouldRetry(err), err
	})
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}

	if result.Status != 200 {
		return nil, fmt.Errorf("error: %s", result.Msg)
	}

	fs.Infof(f, "Successfully created folder %q with ID %v", dirPath, result.Result.FldID)
	return &result, nil
}

// getFolderList List both files and folders in a directory.
func (f *Fs) getFolderList(ctx context.Context, path string) (*api.FolderListResponse, error) {
	encodedDir := f.fromStandardPath(path)

	opts := rest.Opts{
		Method: "GET",
		Path:   "/folder/list",
		Parameters: url.Values{
			"folder_path": {encodedDir},
			"key":         {f.opt.Key},
		},
	}

	var response api.FolderListResponse

	err := f.pacer.Call(func() (bool, error) {
		_, err := f.srv.CallJSON(ctx, &opts, nil, &response)
		if err != nil {
			return shouldRetry(err), fmt.Errorf("failed to list directory: %w", err)
		}
		return false, nil
	})
	if err != nil {
		return nil, err
	}

	if response.Status != 200 {
		if strings.Contains(response.Msg, "Folder not found") {
			return nil, fs.ErrorDirNotFound
		}
		return nil, fmt.Errorf("API error: %s", response.Msg)
	}

	for index := range response.Result.Folders {
		response.Result.Folders[index].Path = f.toStandardPath(response.Result.Folders[index].Path)
	}

	for index := range response.Result.Files {
		response.Result.Files[index].Name = f.toStandardPath(response.Result.Files[index].Name)
	}

	return &response, nil
}

// deleteFolder deletes a folder at the specified path.
func (f *Fs) deleteFolder(ctx context.Context, fullPath string) error {
	fullPath = f.fromStandardPath(fullPath)

	opts := rest.Opts{
		Method: "GET",
		Path:   "/folder/delete",
		Parameters: url.Values{
			"folder_path": {fullPath},
			"key":         {f.opt.Key},
		},
	}

	delResp := api.DeleteFolderResponse{}

	err := f.pacer.Call(func() (bool, error) {
		_, err := f.srv.CallJSON(ctx, &opts, nil, &delResp)
		if err != nil {
			return fserrors.ShouldRetry(err), err
		}
		if delResp.Status != 200 {
			return false, fmt.Errorf("delete error: %s", delResp.Msg)
		}
		return false, nil
	})
	if err != nil {
		return err
	}

	fs.Infof(f, "Rmdir: successfully deleted %q", fullPath)
	return nil
}

// getDirectLink of file from FileLu to download.
func (f *Fs) getDirectLink(ctx context.Context, filePath string) (string, int64, error) {
	filePath = f.fromStandardPath(filePath)

	opts := rest.Opts{
		Method: "GET",
		Path:   "/file/direct_link",
		Parameters: url.Values{
			"file_path": {filePath},
			"key":       {f.opt.Key},
		},
	}

	result := api.FileDirectLinkResponse{}

	err := f.pacer.Call(func() (bool, error) {
		_, err := f.srv.CallJSON(ctx, &opts, nil, &result)
		if err != nil {
			return shouldRetry(err), fmt.Errorf("failed to fetch direct link: %w", err)
		}
		if result.Status != 200 {
			return false, fmt.Errorf("API error: %s", result.Msg)
		}
		return false, nil
	})
	if err != nil {
		return "", 0, err
	}

	return result.Result.URL, result.Result.Size, nil
}

// deleteFile deletes a file based on filePath
func (f *Fs) deleteFile(ctx context.Context, filePath string) error {
	filePath = f.fromStandardPath(filePath)

	opts := rest.Opts{
		Method: "GET",
		Path:   "/file/remove",
		Parameters: url.Values{
			"file_path": {filePath},
			"key":       {f.opt.Key},
		},
	}

	result := api.DeleteFileResponse{}

	err := f.pacer.Call(func() (bool, error) {
		_, err := f.srv.CallJSON(ctx, &opts, nil, &result)
		if err != nil {
			return shouldRetry(err), fmt.Errorf("failed to delete file: %w", err)
		}

		if result.Status != 200 {
			return false, fmt.Errorf("API error: %s", result.Msg)
		}

		return false, nil
	})

	return err
}

// getAccountInfo retrieves account information
func (f *Fs) getAccountInfo(ctx context.Context) (*api.AccountInfoResponse, error) {
	opts := rest.Opts{
		Method: "GET",
		Path:   "/account/info",
		Parameters: url.Values{
			"key": {f.opt.Key},
		},
	}

	var result api.AccountInfoResponse
	err := f.pacer.Call(func() (bool, error) {
		_, callErr := f.srv.CallJSON(ctx, &opts, nil, &result)
		return fserrors.ShouldRetry(callErr), callErr
	})

	if err != nil {
		return nil, err
	}

	if result.Status != 200 {
		return nil, fmt.Errorf("error: %s", result.Msg)
	}

	return &result, nil
}

// getFileInfo retrieves file information based on file code
func (f *Fs) getFileInfo(ctx context.Context, fileCode string) (*api.FileInfoResponse, error) {

	opts := rest.Opts{
		Method: "GET",
		Path:   "/file/info2",
		Parameters: url.Values{
			"file_code": {fileCode},
			"key":       {f.opt.Key},
		},
	}

	result := api.FileInfoResponse{}

	err := f.pacer.Call(func() (bool, error) {
		_, err := f.srv.CallJSON(ctx, &opts, nil, &result)
		if err != nil {
			return shouldRetry(err), fmt.Errorf("failed to fetch file info: %w", err)
		}
		return false, nil
	})
	if err != nil {
		return nil, err
	}

	if result.Status != 200 || len(result.Result) == 0 {
		return nil, fs.ErrorObjectNotFound
	}

	return &result, nil
}
