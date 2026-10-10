package darkibox

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"strings"

	"github.com/rclone/rclone/fs/fserrors"
	"github.com/rclone/rclone/lib/rest"
)

var retryErrorCodes = []int{
	429, // Too Many Requests
	500, // Internal Server Error
	502, // Bad Gateway
	503, // Service Unavailable
	504, // Gateway Timeout
}

// shouldRetry returns a boolean as to whether this resp and err deserve to be retried.
func shouldRetry(ctx context.Context, resp *http.Response, err error) (bool, error) {
	if fserrors.ContextError(ctx, &err) {
		return false, err
	}
	return fserrors.ShouldRetry(err) || fserrors.ShouldRetryHTTP(resp, retryErrorCodes), err
}

// apiCall makes a GET request to the darkibox API with the given path and parameters.
func (f *Fs) apiCall(ctx context.Context, apiPath string, params url.Values, result interface{}) error {
	if params == nil {
		params = url.Values{}
	}
	params.Set("key", f.opt.APIKey)

	return f.pacer.Call(func() (bool, error) {
		resp, err := f.srv.CallJSON(ctx, &rest.Opts{
			Method:     "GET",
			Path:       apiPath,
			Parameters: params,
		}, nil, result)
		return shouldRetry(ctx, resp, err)
	})
}

// getAccountInfo returns account information.
func (f *Fs) getAccountInfo(ctx context.Context) (*AccountInfoResponse, error) {
	var resp AccountInfoResponse
	err := f.apiCall(ctx, "/account/info", nil, &resp)
	if err != nil {
		return nil, err
	}
	if resp.Status != 200 {
		return nil, fmt.Errorf("account info failed: %s", resp.Msg)
	}
	return &resp, nil
}

// getFileInfo returns info for one or more files (comma-separated file codes).
func (f *Fs) getFileInfo(ctx context.Context, fileCodes string) (*FileInfoResponse, error) {
	var resp FileInfoResponse
	err := f.apiCall(ctx, "/file/info", url.Values{"file_code": {fileCodes}}, &resp)
	if err != nil {
		return nil, err
	}
	if resp.Status != 200 {
		return nil, fmt.Errorf("file info failed: %s", resp.Msg)
	}
	return &resp, nil
}

// getFileList returns files in a folder.
func (f *Fs) getFileList(ctx context.Context, folderID string, page int) (*FileListResponse, error) {
	params := url.Values{}
	if folderID != "" && folderID != "0" {
		params.Set("fld_id", folderID)
	}
	if page > 1 {
		params.Set("page", fmt.Sprintf("%d", page))
	}
	params.Set("per_page", "200")

	var resp FileListResponse
	err := f.apiCall(ctx, "/file/list", params, &resp)
	if err != nil {
		return nil, err
	}
	if resp.Status != 200 {
		return nil, fmt.Errorf("file list failed: %s", resp.Msg)
	}
	return &resp, nil
}

// getDirectLink returns a direct download URL for a file.
func (f *Fs) getDirectLink(ctx context.Context, fileCode string) (*DirectLinkResponse, error) {
	var resp DirectLinkResponse
	err := f.apiCall(ctx, "/file/direct_link", url.Values{"file_code": {fileCode}}, &resp)
	if err != nil {
		return nil, err
	}
	if resp.Status != 200 {
		return nil, fmt.Errorf("direct link failed: %s", resp.Msg)
	}
	return &resp, nil
}

// getFolderList returns subfolders (and optionally files) in a folder.
func (f *Fs) getFolderList(ctx context.Context, folderID string) (*FolderListResponse, error) {
	params := url.Values{"files": {"1"}}
	if folderID != "" && folderID != "0" {
		params.Set("fld_id", folderID)
	}

	var resp FolderListResponse
	err := f.apiCall(ctx, "/folder/list", params, &resp)
	if err != nil {
		return nil, err
	}
	if resp.Status != 200 {
		return nil, fmt.Errorf("folder list failed: %s", resp.Msg)
	}
	return &resp, nil
}

// createFolder creates a new folder.
func (f *Fs) createFolder(ctx context.Context, name string, parentID string) (*FolderCreateResponse, error) {
	params := url.Values{"name": {name}}
	if parentID != "" && parentID != "0" {
		params.Set("parent_id", parentID)
	}

	var resp FolderCreateResponse
	err := f.apiCall(ctx, "/folder/create", params, &resp)
	if err != nil {
		return nil, err
	}
	if resp.Status != 200 {
		return nil, fmt.Errorf("folder create failed: %s", resp.Msg)
	}
	return &resp, nil
}

// deleteFolder deletes a folder.
func (f *Fs) deleteFolder(ctx context.Context, folderID string) error {
	var resp GenericResponse
	err := f.apiCall(ctx, "/folder/delete", url.Values{"fld_id": {folderID}}, &resp)
	if err != nil {
		return err
	}
	if resp.Status != 200 {
		return fmt.Errorf("folder delete failed: %s", resp.Msg)
	}
	return nil
}

// deleteFile deletes a file.
func (f *Fs) deleteFile(ctx context.Context, fileCode string) error {
	var resp GenericResponse
	err := f.apiCall(ctx, "/file/delete", url.Values{"file_code": {fileCode}}, &resp)
	if err != nil {
		return err
	}
	if resp.Status != 200 {
		return fmt.Errorf("file delete failed: %s", resp.Msg)
	}
	return nil
}

// renameFile renames a file.
func (f *Fs) renameFile(ctx context.Context, fileCode string, newTitle string) error {
	var resp GenericResponse
	err := f.apiCall(ctx, "/file/edit", url.Values{
		"file_code":  {fileCode},
		"file_title": {newTitle},
	}, &resp)
	if err != nil {
		return err
	}
	if resp.Status != 200 {
		return fmt.Errorf("file rename failed: %s", resp.Msg)
	}
	return nil
}

// moveFile moves a file to a different folder.
func (f *Fs) moveFile(ctx context.Context, fileCode string, toFolderID string) error {
	var resp GenericResponse
	err := f.apiCall(ctx, "/file/move", url.Values{
		"file_code": {fileCode},
		"to_folder": {toFolderID},
	}, &resp)
	if err != nil {
		return err
	}
	if resp.Status != 200 {
		return fmt.Errorf("file move failed: %s", resp.Msg)
	}
	return nil
}

// getUploadServer returns the upload server URL.
func (f *Fs) getUploadServer(ctx context.Context) (string, error) {
	var resp UploadServerResponse
	err := f.apiCall(ctx, "/upload/server", nil, &resp)
	if err != nil {
		return "", err
	}
	if resp.Status != 200 {
		return "", fmt.Errorf("upload server failed: %s", resp.Msg)
	}
	return resp.Result, nil
}

// uploadFile uploads a file to the upload server.
func (f *Fs) uploadFile(ctx context.Context, uploadURL string, folderID string, filename string, in io.Reader) (*UploadResponse, error) {
	// We need to build a multipart form with the file
	bodyBuf := &bytes.Buffer{}
	writer := multipart.NewWriter(bodyBuf)

	_ = writer.WriteField("key", f.opt.APIKey)
	if folderID != "" && folderID != "0" {
		_ = writer.WriteField("fld_id", folderID)
	}

	// Strip .mp4 extension from title if present (darkibox adds it automatically)
	title := filename
	if idx := strings.LastIndex(title, "."); idx > 0 {
		title = title[:idx]
	}
	_ = writer.WriteField("file_title", title)

	part, err := writer.CreateFormFile("file", filename)
	if err != nil {
		return nil, fmt.Errorf("failed to create form file: %w", err)
	}

	if _, err = io.Copy(part, in); err != nil {
		return nil, fmt.Errorf("failed to copy file data: %w", err)
	}

	if err = writer.Close(); err != nil {
		return nil, fmt.Errorf("failed to close multipart writer: %w", err)
	}

	var result UploadResponse
	err = f.pacer.Call(func() (bool, error) {
		resp, err := f.srv.CallJSON(ctx, &rest.Opts{
			Method:      "POST",
			RootURL:     uploadURL,
			ContentType: writer.FormDataContentType(),
			Body:        bytes.NewReader(bodyBuf.Bytes()),
		}, nil, &result)
		return shouldRetry(ctx, resp, err)
	})
	if err != nil {
		return nil, err
	}
	if result.Status != 200 {
		return nil, fmt.Errorf("upload failed: %s", result.Msg)
	}
	return &result, nil
}

// renameFolder renames a folder.
func (f *Fs) renameFolder(ctx context.Context, folderID string, newName string) error {
	var resp GenericResponse
	err := f.apiCall(ctx, "/folder/edit", url.Values{
		"fld_id": {folderID},
		"name":   {newName},
	}, &resp)
	if err != nil {
		return err
	}
	if resp.Status != 200 {
		return fmt.Errorf("folder rename failed: %s", resp.Msg)
	}
	return nil
}
