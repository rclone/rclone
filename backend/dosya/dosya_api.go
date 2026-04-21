package dosya

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/rclone/rclone/backend/dosya/api"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/fserrors"
	"github.com/rclone/rclone/lib/rest"
)

// retryErrorCodes is a slice of error codes that we will retry
var retryErrorCodes = []int{
	429, // Too Many Requests
	500, // Internal Server Error
	502, // Bad Gateway
	503, // Service Unavailable
	504, // Gateway Timeout
}

// shouldRetry returns a boolean as to whether this resp and err
// deserve to be retried
func shouldRetry(ctx context.Context, resp *http.Response, err error) (bool, error) {
	if fserrors.ContextError(ctx, &err) {
		return false, err
	}
	return fserrors.ShouldRetry(err) || fserrors.ShouldRetryHTTP(resp, retryErrorCodes), err
}

// listFilesAndFolders lists files and folders in a directory
func (f *Fs) listFilesAndFolders(ctx context.Context, folderID string) (*api.ListResponse, error) {
	opts := rest.Opts{
		Method: "GET",
		Path:   "/api/files",
		Parameters: map[string][]string{
			"workspace_id": {f.opt.WorkspaceID},
			"per_page":     {"500"},
		},
	}
	if folderID != "" && folderID != rootID {
		opts.Parameters["folder_id"] = []string{folderID}
	}

	var result api.ListResponse
	err := f.pacer.Call(func() (bool, error) {
		resp, err := f.rest.CallJSON(ctx, &opts, nil, &result)
		return shouldRetry(ctx, resp, err)
	})
	if err != nil {
		return nil, fmt.Errorf("couldn't list files: %w", err)
	}
	if !result.OK {
		return nil, fmt.Errorf("API error: %s", result.Error)
	}
	return &result, nil
}

// getFolderTree gets the full folder tree for path resolution
func (f *Fs) getFolderTree(ctx context.Context) (*api.FolderTreeResponse, error) {
	opts := rest.Opts{
		Method: "GET",
		Path:   "/api/folders/tree",
		Parameters: map[string][]string{
			"workspace_id": {f.opt.WorkspaceID},
		},
	}

	var result api.FolderTreeResponse
	err := f.pacer.Call(func() (bool, error) {
		resp, err := f.rest.CallJSON(ctx, &opts, nil, &result)
		return shouldRetry(ctx, resp, err)
	})
	if err != nil {
		return nil, fmt.Errorf("couldn't get folder tree: %w", err)
	}
	if !result.OK {
		return nil, fmt.Errorf("API error: %s", result.Error)
	}
	return &result, nil
}

// createFolder creates a folder
func (f *Fs) createFolder(ctx context.Context, name string, parentID string) (*api.CreateFolderResponse, error) {
	request := api.CreateFolderRequest{
		WorkspaceID: f.opt.WorkspaceID,
		Name:        name,
	}
	if parentID != "" && parentID != rootID {
		request.ParentID = &parentID
	}

	opts := rest.Opts{
		Method: "POST",
		Path:   "/api/folders",
	}

	var result api.CreateFolderResponse
	err := f.pacer.Call(func() (bool, error) {
		resp, err := f.rest.CallJSON(ctx, &opts, &request, &result)
		return shouldRetry(ctx, resp, err)
	})
	if err != nil {
		return nil, fmt.Errorf("couldn't create folder: %w", err)
	}
	if !result.OK {
		return nil, fmt.Errorf("API error: %s", result.Error)
	}
	return &result, nil
}

// removeFolder deletes a folder
func (f *Fs) removeFolder(ctx context.Context, folderID string) error {
	opts := rest.Opts{
		Method: "DELETE",
		Path:   "/api/folders/" + folderID,
	}

	var result api.DeleteFolderResponse
	err := f.pacer.Call(func() (bool, error) {
		resp, err := f.rest.CallJSON(ctx, &opts, nil, &result)
		return shouldRetry(ctx, resp, err)
	})
	if err != nil {
		return fmt.Errorf("couldn't remove folder: %w", err)
	}
	if !result.OK {
		return fmt.Errorf("API error: %s", result.Error)
	}
	return nil
}

// renameFolder renames a folder
func (f *Fs) renameFolder(ctx context.Context, folderID string, newName string) error {
	request := api.RenameFolderRequest{Name: newName}
	opts := rest.Opts{
		Method: "PUT",
		Path:   "/api/folders/" + folderID + "/rename",
	}

	var result api.Response
	err := f.pacer.Call(func() (bool, error) {
		resp, err := f.rest.CallJSON(ctx, &opts, &request, &result)
		return shouldRetry(ctx, resp, err)
	})
	if err != nil {
		return fmt.Errorf("couldn't rename folder: %w", err)
	}
	if !result.OK {
		return fmt.Errorf("API error: %s", result.Error)
	}
	return nil
}

// moveFolder moves a folder to a new parent
func (f *Fs) moveFolder(ctx context.Context, folderID string, newParentID string) error {
	request := api.MoveFolderRequest{}
	if newParentID != "" && newParentID != rootID {
		request.ParentID = &newParentID
	}
	opts := rest.Opts{
		Method: "PUT",
		Path:   "/api/folders/" + folderID + "/move",
	}

	var result api.Response
	err := f.pacer.Call(func() (bool, error) {
		resp, err := f.rest.CallJSON(ctx, &opts, &request, &result)
		return shouldRetry(ctx, resp, err)
	})
	if err != nil {
		return fmt.Errorf("couldn't move folder: %w", err)
	}
	if !result.OK {
		return fmt.Errorf("API error: %s", result.Error)
	}
	return nil
}

// deleteFile deletes a file (soft delete first, then permanent)
func (f *Fs) deleteFile(ctx context.Context, fileID string) error {
	opts := rest.Opts{
		Method: "DELETE",
		Path:   "/api/files/" + fileID,
	}

	var result api.DeleteFileResponse
	err := f.pacer.Call(func() (bool, error) {
		resp, err := f.rest.CallJSON(ctx, &opts, nil, &result)
		return shouldRetry(ctx, resp, err)
	})
	if err != nil {
		return fmt.Errorf("couldn't delete file: %w", err)
	}
	if !result.OK {
		return fmt.Errorf("API error: %s", result.Error)
	}

	// If not permanently deleted, call again for permanent deletion
	if !result.Permanent {
		err = f.pacer.Call(func() (bool, error) {
			resp, err := f.rest.CallJSON(ctx, &opts, nil, &result)
			return shouldRetry(ctx, resp, err)
		})
		if err != nil {
			return fmt.Errorf("couldn't permanently delete file: %w", err)
		}
	}
	return nil
}

// renameFile renames a file
func (f *Fs) renameFile(ctx context.Context, fileID string, newName string) error {
	request := api.RenameFileRequest{Name: newName}
	opts := rest.Opts{
		Method: "PUT",
		Path:   "/api/files/" + fileID + "/rename",
	}

	var result api.RenameFileResponse
	err := f.pacer.Call(func() (bool, error) {
		resp, err := f.rest.CallJSON(ctx, &opts, &request, &result)
		return shouldRetry(ctx, resp, err)
	})
	if err != nil {
		return fmt.Errorf("couldn't rename file: %w", err)
	}
	if !result.OK {
		return fmt.Errorf("API error: %s", result.Error)
	}
	return nil
}

// moveFile moves a file to a new folder
func (f *Fs) moveFile(ctx context.Context, fileID string, folderID string) error {
	request := api.MoveFileRequest{}
	if folderID != "" && folderID != rootID {
		request.FolderID = &folderID
	}
	opts := rest.Opts{
		Method: "PUT",
		Path:   "/api/files/" + fileID + "/move",
	}

	var result api.Response
	err := f.pacer.Call(func() (bool, error) {
		resp, err := f.rest.CallJSON(ctx, &opts, &request, &result)
		return shouldRetry(ctx, resp, err)
	})
	if err != nil {
		return fmt.Errorf("couldn't move file: %w", err)
	}
	if !result.OK {
		return fmt.Errorf("API error: %s", result.Error)
	}
	return nil
}

// copyFile copies a file to a folder
func (f *Fs) copyFile(ctx context.Context, fileID string, folderID string) (*api.CopyFileResponse, error) {
	request := api.CopyFileRequest{}
	if folderID != "" && folderID != rootID {
		request.FolderID = &folderID
	}
	opts := rest.Opts{
		Method: "POST",
		Path:   "/api/files/" + fileID + "/copy",
	}

	var result api.CopyFileResponse
	err := f.pacer.Call(func() (bool, error) {
		resp, err := f.rest.CallJSON(ctx, &opts, &request, &result)
		return shouldRetry(ctx, resp, err)
	})
	if err != nil {
		return nil, fmt.Errorf("couldn't copy file: %w", err)
	}
	if !result.OK {
		return nil, fmt.Errorf("API error: %s", result.Error)
	}
	return &result, nil
}

// downloadFile downloads a file by its ID
//
// The download endpoint returns a 302 redirect to a presigned R2 URL.
// We use a raw HTTP client to get the redirect Location, then fetch
// the file from the presigned URL with Range support.
func (f *Fs) downloadFile(ctx context.Context, fileID string, options []fs.OpenOption) (io.ReadCloser, error) {
	// Build the download URL
	baseURL := strings.TrimSuffix(f.opt.APIURL, "/")
	downloadEndpoint := baseURL + "/api/files/" + fileID + "/download"

	// Use raw http to get the 302 redirect without following it
	req, err := http.NewRequestWithContext(ctx, "GET", downloadEndpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("couldn't create download request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+f.opt.APIKey)

	noRedirectClient := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	var redirectResp *http.Response
	err = f.pacer.Call(func() (bool, error) {
		var err error
		redirectResp, err = noRedirectClient.Do(req)
		if err != nil {
			return shouldRetry(ctx, redirectResp, err)
		}
		if redirectResp.StatusCode == http.StatusFound || redirectResp.StatusCode == http.StatusTemporaryRedirect {
			return false, nil
		}
		// Unexpected status code
		if redirectResp.StatusCode >= 500 {
			return true, fmt.Errorf("server error %d", redirectResp.StatusCode)
		}
		return false, fmt.Errorf("expected redirect, got %d", redirectResp.StatusCode)
	})
	if err != nil {
		return nil, fmt.Errorf("couldn't get download URL: %w", err)
	}
	if redirectResp.Body != nil {
		redirectResp.Body.Close()
	}

	downloadURL := redirectResp.Header.Get("Location")
	if downloadURL == "" {
		return nil, fmt.Errorf("no download URL in redirect response")
	}

	// Fetch the actual file from the presigned R2 URL
	// Use a plain HTTP client — no auth headers, as the presigned URL
	// already contains credentials and R2 rejects extra Authorization headers.
	dlReq, err := http.NewRequestWithContext(ctx, "GET", downloadURL, nil)
	if err != nil {
		return nil, fmt.Errorf("couldn't create download request: %w", err)
	}
	// Apply range options for partial downloads
	fs.OpenOptionAddHTTPHeaders(dlReq.Header, options)

	var resp *http.Response
	err = f.pacer.Call(func() (bool, error) {
		var err error
		resp, err = http.DefaultClient.Do(dlReq)
		if err != nil {
			return shouldRetry(ctx, resp, err)
		}
		if resp.StatusCode >= 500 {
			return true, fmt.Errorf("server error %d", resp.StatusCode)
		}
		if resp.StatusCode >= 400 {
			return false, fmt.Errorf("download error %d", resp.StatusCode)
		}
		return false, nil
	})
	if err != nil {
		return nil, fmt.Errorf("couldn't download file: %w", err)
	}
	return resp.Body, nil
}

// uploadSmallFile uploads a file smaller than 10MB in a single request
func (f *Fs) uploadSmallFile(ctx context.Context, in io.Reader, sessionID string, size int64) (*api.UploadCompleteResponse, error) {
	opts := rest.Opts{
		Method:        "PUT",
		Path:          "/api/upload/" + sessionID,
		Body:          in,
		ContentLength: &size,
		ContentType:   "application/octet-stream",
	}

	var result api.UploadCompleteResponse
	err := f.pacer.Call(func() (bool, error) {
		resp, err := f.rest.CallJSON(ctx, &opts, nil, &result)
		return shouldRetry(ctx, resp, err)
	})
	if err != nil {
		return nil, fmt.Errorf("couldn't upload file: %w", err)
	}
	if !result.OK {
		return nil, fmt.Errorf("API error: %s", result.Error)
	}
	return &result, nil
}

// uploadPart uploads a single part of a multipart upload
func (f *Fs) uploadPart(ctx context.Context, in io.Reader, sessionID string, partNumber int, size int64) (*api.UploadPartResponse, error) {
	opts := rest.Opts{
		Method:        "PUT",
		Path:          "/api/upload/" + sessionID + "/part/" + strconv.Itoa(partNumber),
		Body:          in,
		ContentLength: &size,
		ContentType:   "application/octet-stream",
	}

	var result api.UploadPartResponse
	err := f.pacer.Call(func() (bool, error) {
		resp, err := f.rest.CallJSON(ctx, &opts, nil, &result)
		return shouldRetry(ctx, resp, err)
	})
	if err != nil {
		return nil, fmt.Errorf("couldn't upload part %d: %w", partNumber, err)
	}
	if !result.OK {
		return nil, fmt.Errorf("API error: %s", result.Error)
	}
	return &result, nil
}

// completeUpload finalizes a multipart upload
func (f *Fs) completeUpload(ctx context.Context, sessionID string) (*api.UploadCompleteResponse, error) {
	opts := rest.Opts{
		Method: "POST",
		Path:   "/api/upload/" + sessionID + "/complete",
	}

	var result api.UploadCompleteResponse
	err := f.pacer.Call(func() (bool, error) {
		resp, err := f.rest.CallJSON(ctx, &opts, nil, &result)
		return shouldRetry(ctx, resp, err)
	})
	if err != nil {
		return nil, fmt.Errorf("couldn't complete upload: %w", err)
	}
	if !result.OK {
		return nil, fmt.Errorf("API error: %s", result.Error)
	}
	return &result, nil
}

// initUpload initializes a new upload session
func (f *Fs) initUpload(ctx context.Context, fileName string, fileSize int64, mimeType string, folderID string, fileID *string) (*api.UploadInitResponse, error) {
	request := api.UploadInitRequest{
		WorkspaceID: f.opt.WorkspaceID,
		FileName:    fileName,
		FileSize:    fileSize,
		MimeType:    mimeType,
		FileID:      fileID,
	}
	if folderID != "" && folderID != rootID {
		request.FolderID = &folderID
	}

	opts := rest.Opts{
		Method: "POST",
		Path:   "/api/upload/init",
	}

	var result api.UploadInitResponse
	err := f.pacer.Call(func() (bool, error) {
		resp, err := f.rest.CallJSON(ctx, &opts, &request, &result)
		return shouldRetry(ctx, resp, err)
	})
	if err != nil {
		return nil, fmt.Errorf("couldn't init upload: %w", err)
	}
	if !result.OK {
		return nil, fmt.Errorf("API error: %s", result.Error)
	}
	return &result, nil
}

// uploadFile handles the full upload flow (small or multipart)
func (f *Fs) uploadFile(ctx context.Context, in io.Reader, remote string, size int64, folderID string, fileID *string) (*api.UploadCompleteResponse, error) {
	leaf := f.opt.Enc.FromStandardName(path.Base(remote))

	mimeType := fs.MimeTypeFromName(remote)
	if mimeType == "" {
		mimeType = "application/octet-stream"
	}

	initResp, err := f.initUpload(ctx, leaf, size, mimeType, folderID, fileID)
	if err != nil {
		return nil, err
	}

	// Small file upload (< 10MB, no resumable)
	if initResp.Resumable == nil {
		return f.uploadSmallFile(ctx, in, initResp.SessionID, size)
	}

	// Multipart upload
	partSize := initResp.Resumable.PartSize
	totalParts := initResp.Resumable.TotalParts

	for partNum := 1; partNum <= totalParts; partNum++ {
		currentPartSize := partSize
		if partNum == totalParts {
			currentPartSize = size - partSize*int64(totalParts-1)
		}
		partReader := io.LimitReader(in, currentPartSize)
		_, err := f.uploadPart(ctx, partReader, initResp.SessionID, partNum, currentPartSize)
		if err != nil {
			return nil, err
		}
	}

	return f.completeUpload(ctx, initResp.SessionID)
}

// createShareLink creates a public share link for a file
func (f *Fs) createShareLink(ctx context.Context, fileID string, expire fs.Duration) (*api.ShareLinkResponse, error) {
	request := api.ShareLinkRequest{}
	if expire > 0 {
		days := int(time.Duration(expire).Hours() / 24)
		if days < 1 {
			days = 1
		}
		request.ExpiresInDays = &days
	}

	opts := rest.Opts{
		Method: "POST",
		Path:   "/api/files/" + fileID + "/share",
	}

	var result api.ShareLinkResponse
	err := f.pacer.Call(func() (bool, error) {
		resp, err := f.rest.CallJSON(ctx, &opts, &request, &result)
		return shouldRetry(ctx, resp, err)
	})
	if err != nil {
		return nil, fmt.Errorf("couldn't create share link: %w", err)
	}
	if !result.OK {
		return nil, fmt.Errorf("API error: %s", result.Error)
	}
	return &result, nil
}

// getWorkspaceInfo fetches workspace storage info
func (f *Fs) getWorkspaceInfo(ctx context.Context) (*api.WorkspaceInfoResponse, error) {
	opts := rest.Opts{
		Method: "GET",
		Path:   "/api/workspaces/" + f.opt.WorkspaceID,
	}

	var result api.WorkspaceInfoResponse
	err := f.pacer.Call(func() (bool, error) {
		resp, err := f.rest.CallJSON(ctx, &opts, nil, &result)
		return shouldRetry(ctx, resp, err)
	})
	if err != nil {
		return nil, fmt.Errorf("couldn't get workspace info: %w", err)
	}
	if !result.OK {
		return nil, fmt.Errorf("API error: %s", result.Error)
	}
	return &result, nil
}
