package adrive

import (
	"context"

	"github.com/rclone/rclone/backend/adrive/api"
	"github.com/rclone/rclone/lib/rest"
)

// GetUserInfo gets information about the authenticated user
func (f *Fs) GetUserInfo(ctx context.Context) (*api.UserInfo, error) {
	var result api.UserInfo
	opts := rest.Opts{
		Method: "GET",
		Path:   "/oauth/users/info",
	}
	var resp struct {
		api.UserInfo
	}
	err := f.pacer.Call(func() (bool, error) {
		resp2, err := f.srv.CallJSON(ctx, &opts, nil, &resp)
		return shouldRetry(ctx, resp2, err)
	})
	if err != nil {
		return nil, err
	}
	result = resp.UserInfo
	return &result, nil
}

// Mkdir creates a new directory
func (f *Fs) MkDirectory(ctx context.Context, driveID, parentID, name string) (*api.FileEntity, error) {
	opts := rest.Opts{
		Method: "POST",
		Path:   "/adrive/v1.0/openFile/create",
	}
	params := map[string]interface{}{
		"drive_id":        driveID,
		"parent_file_id":  parentID,
		"name":            name,
		"check_name_mode": "refuse",
		"type":            "folder",
	}
	var resp api.FileEntity
	err := f.pacer.Call(func() (bool, error) {
		resp2, err := f.srv.CallJSON(ctx, &opts, params, &resp)
		return shouldRetry(ctx, resp2, err)
	})
	if err != nil {
		return nil, err
	}
	return &resp, nil
}

// FileList lists files in a directory
func (f *Fs) FileList(ctx context.Context, param *api.FileListParam) (*api.FileListResponse, error) {
	opts := rest.Opts{
		Method: "POST",
		Path:   "/adrive/v1.0/openFile/list",
	}
	var resp api.FileListResponse
	err := f.pacer.Call(func() (bool, error) {
		resp2, err := f.srv.CallJSON(ctx, &opts, param, &resp)
		return shouldRetry(ctx, resp2, err)
	})
	if err != nil {
		return nil, err
	}
	return &resp, nil
}

// FileListGetAll gets all files (paginated requests)
func (f *Fs) FileListGetAll(ctx context.Context, param *api.FileListParam, maxItems int) (api.FileList, error) {
	var result api.FileList
	marker := ""

	for {
		param.Marker = marker
		resp, err := f.FileList(ctx, param)
		if err != nil {
			return nil, err
		}

		result = append(result, resp.Items...)

		if resp.NextMarker == "" {
			break
		}

		marker = resp.NextMarker

		if maxItems > 0 && len(result) >= maxItems {
			result = result[:maxItems]
			break
		}
	}

	return result, nil
}

// FileInfoById gets file information by ID
func (f *Fs) FileInfoById(ctx context.Context, driveID, fileID string) (*api.FileEntity, error) {
	opts := rest.Opts{
		Method: "POST",
		Path:   "/adrive/v1.0/openFile/get",
	}
	params := map[string]interface{}{
		"drive_id": driveID,
		"file_id":  fileID,
	}
	var resp api.FileEntity
	err := f.pacer.Call(func() (bool, error) {
		resp2, err := f.srv.CallJSON(ctx, &opts, params, &resp)
		return shouldRetry(ctx, resp2, err)
	})
	if err != nil {
		return nil, err
	}
	return &resp, nil
}

// FileDelete deletes a file
func (f *Fs) FileDelete(ctx context.Context, param *api.FileBatchActionParam) (*api.FileActionResponse, error) {
	opts := rest.Opts{
		Method: "POST",
		Path:   "/adrive/v1.0/openFile/delete",
	}
	var resp api.FileActionResponse
	err := f.pacer.Call(func() (bool, error) {
		resp2, err := f.srv.CallJSON(ctx, &opts, param, &resp)
		return shouldRetry(ctx, resp2, err)
	})
	if err != nil {
		return nil, err
	}
	return &resp, nil
}

// FileCopy copies a file
func (f *Fs) FileCopy(ctx context.Context, param *api.FileCopyParam) (*api.FileActionResponse, error) {
	opts := rest.Opts{
		Method: "POST",
		Path:   "/adrive/v1.0/openFile/copy",
	}
	var resp api.FileActionResponse
	err := f.pacer.Call(func() (bool, error) {
		resp2, err := f.srv.CallJSON(ctx, &opts, param, &resp)
		return shouldRetry(ctx, resp2, err)
	})
	if err != nil {
		return nil, err
	}
	return &resp, nil
}

// FileMove moves a file
func (f *Fs) FileMove(ctx context.Context, param *api.FileMoveParam) (*api.FileActionResponse, error) {
	opts := rest.Opts{
		Method: "POST",
		Path:   "/adrive/v1.0/openFile/move",
	}
	var resp api.FileActionResponse
	err := f.pacer.Call(func() (bool, error) {
		resp2, err := f.srv.CallJSON(ctx, &opts, param, &resp)
		return shouldRetry(ctx, resp2, err)
	})
	if err != nil {
		return nil, err
	}
	return &resp, nil
}

// GetFileDownloadUrl gets the download URL for a file
func (f *Fs) GetFileDownloadUrl(ctx context.Context, param *api.GetFileDownloadUrlParam) (*api.DownloadUrlResponse, error) {
	opts := rest.Opts{
		Method: "POST",
		Path:   "/adrive/v1.0/openFile/getDownloadUrl",
	}
	var resp api.DownloadUrlResponse
	err := f.pacer.Call(func() (bool, error) {
		resp2, err := f.srv.CallJSON(ctx, &opts, param, &resp)
		return shouldRetry(ctx, resp2, err)
	})
	if err != nil {
		return nil, err
	}
	return &resp, nil
}

// GetFileDownloadUrl gets the download URL for a file
func (f *Fs) FileUploadCreate(ctx context.Context, param *api.GetFileDownloadUrlParam) (*api.DownloadUrlResponse, error) {
	opts := rest.Opts{
		Method: "POST",
		Path:   "/adrive/v1.0/openFile/create",
	}
	var resp api.DownloadUrlResponse
	err := f.pacer.Call(func() (bool, error) {
		resp2, err := f.srv.CallJSON(ctx, &opts, param, &resp)
		return shouldRetry(ctx, resp2, err)
	})
	if err != nil {
		return nil, err
	}
	return &resp, nil
}

// GetFileDownloadUrl gets the download URL for a file
func (f *Fs) FileUploadGetUploadUrl(ctx context.Context, param *api.GetFileDownloadUrlParam) (*api.DownloadUrlResponse, error) {
	opts := rest.Opts{
		Method: "POST",
		Path:   "/adrive/v1.0/openFile/getUploadUrl",
	}
	var resp api.DownloadUrlResponse
	err := f.pacer.Call(func() (bool, error) {
		resp2, err := f.srv.CallJSON(ctx, &opts, param, &resp)
		return shouldRetry(ctx, resp2, err)
	})
	if err != nil {
		return nil, err
	}
	return &resp, nil
}

// GetFileDownloadUrl gets the download URL for a file
func (f *Fs) FileUploadComplete(ctx context.Context, param *api.GetFileDownloadUrlParam) (*api.DownloadUrlResponse, error) {
	opts := rest.Opts{
		Method: "POST",
		Path:   "/adrive/v1.0/openFile/complete",
	}
	var resp api.DownloadUrlResponse
	err := f.pacer.Call(func() (bool, error) {
		resp2, err := f.srv.CallJSON(ctx, &opts, param, &resp)
		return shouldRetry(ctx, resp2, err)
	})
	if err != nil {
		return nil, err
	}
	return &resp, nil
}
