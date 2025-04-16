package filelu

import (
	"bytes"
	"context"
	"encoding/json"
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

// CreateFolderResponse represents the response for creating a folder.
type CreateFolderResponse struct {
	Status int    `json:"status"`
	Msg    string `json:"msg"`
	Result struct {
		FldID interface{} `json:"fld_id"`
	} `json:"result"`
}

// DeleteFolderResponse represents the response for deleting a folder.
type DeleteFolderResponse struct {
	Status int    `json:"status"`
	Msg    string `json:"msg"`
}

// FolderListResponse represents the response for listing folders.
type FolderListResponse struct {
	Status int    `json:"status"`
	Msg    string `json:"msg"`
	Result struct {
		Files []struct {
			Name     string      `json:"name"`
			FldID    json.Number `json:"fld_id"`
			Path     string      `json:"path"`
			FileCode string      `json:"file_code"`
		} `json:"files"`
		Folders []struct {
			Name  string      `json:"name"`
			FldID json.Number `json:"fld_id"`
			Path  string      `json:"path"`
		} `json:"folders"`
	} `json:"result"`
}

// FileDirectLinkResponse represents the response for a direct link to a file.
type FileDirectLinkResponse struct {
	Status int    `json:"status"`
	Msg    string `json:"msg"`
	Result struct {
		URL  string `json:"url"`
		Size int64  `json:"size"`
	} `json:"result"`
}

type FileInfoResponse struct {
	Status int    `json:"status"`
	Msg    string `json:"msg"`
	Result []struct {
		Size     string `json:"size"`
		Name     string `json:"name"`
		FileCode string `json:"filecode"`
		Hash     string `json:"hash"`
		Status   int    `json:"status"`
	} `json:"result"`
}

// DeleteFileResponse represents the response for deleting a file.
type DeleteFileResponse struct {
	Status int    `json:"status"`
	Msg    string `json:"msg"`
}

// createFolder creates a folder at the specified path.
func (f *Fs) createFolder(ctx context.Context, dirPath string) (*CreateFolderResponse, error) {
	encodedDir := f.FromStandardPath(dirPath)
	apiURL := fmt.Sprintf("%s/folder/create?folder_path=%s&key=%s",
		f.endpoint,
		url.QueryEscape(encodedDir),
		url.QueryEscape(f.opt.Key), // assuming f.opt.Key is the correct field
	)

	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	var resp *http.Response
	result := CreateFolderResponse{}
	err = f.pacer.Call(func() (bool, error) {
		var innerErr error
		resp, innerErr = f.client.Do(req)
		return fserrors.ShouldRetry(innerErr), innerErr
	})
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			fs.Logf(nil, "Failed to close response body: %v", err)
		}
	}()

	err = json.NewDecoder(resp.Body).Decode(&result)
	if err != nil {
		return nil, fmt.Errorf("error decoding response: %w", err)
	}
	if result.Status != 200 {
		return nil, fmt.Errorf("error: %s", result.Msg)
	}

	fs.Infof(f, "Successfully created folder %q with ID %v", dirPath, result.Result.FldID)
	return &result, nil
}

// renameFolder handles folder renaming using folder paths
func (f *Fs) renameFolder(ctx context.Context, folderPath, newName string) error {
	folderPath = "/" + strings.Trim(folderPath, "/")
	folderPath = f.FromStandardPath(folderPath)
	newName = f.FromStandardPath(newName)
	opts := rest.Opts{
		Method: "GET",
		Path:   "/folder/rename",
		Parameters: url.Values{
			"folder_path": {folderPath},
			"name":        {newName},
			"key":         {f.opt.Key},
		},
	}

	var result struct {
		Status int    `json:"status"`
		Result string `json:"result"`
		Msg    string `json:"msg"`
	}

	err := f.pacer.Call(func() (bool, error) {
		_, err := f.srv.CallJSON(ctx, &opts, nil, &result)
		return fserrors.ShouldRetry(err), err
	})

	if err != nil {
		return fmt.Errorf("renameFolder failed: %w", err)
	}
	if result.Status != 200 {
		return fmt.Errorf("renameFolder API error: %s", result.Msg)
	}

	fs.Infof(f, "Successfully renamed folder at path: %s to %s", folderPath, newName)
	return nil
}

// getFolderList List both files and folders in a directory.
func (f *Fs) getFolderList(ctx context.Context, path string) (*FolderListResponse, error) {
	encodedDir := f.FromStandardPath(path)
	apiURL := fmt.Sprintf("%s/folder/list?folder_path=%s&key=%s",
		f.endpoint,
		url.QueryEscape(encodedDir),
		url.QueryEscape(f.opt.Key),
	)

	var body []byte
	err := f.pacer.Call(func() (bool, error) {
		req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
		if err != nil {
			return false, fmt.Errorf("failed to create request: %w", err)
		}

		resp, err := f.client.Do(req)
		if err != nil {
			return shouldRetry(err), fmt.Errorf("failed to list directory: %w", err)
		}
		defer resp.Body.Close()

		body, err = io.ReadAll(resp.Body)
		if err != nil {
			return false, fmt.Errorf("error reading response body: %w", err)
		}

		return shouldRetryHTTP(resp.StatusCode), nil
	})
	if err != nil {
		return nil, err
	}

	var response FolderListResponse
	if err := json.NewDecoder(bytes.NewReader(body)).Decode(&response); err != nil {
		return nil, fmt.Errorf("error decoding response: %w", err)
	}
	if response.Status != 200 {
		if strings.Contains(response.Msg, "Folder not found") {
			return nil, fs.ErrorDirNotFound
		}
		return nil, fmt.Errorf("API error: %s", response.Msg)
	}

	return &response, nil

}

// deleteFolder deletes a folder at the specified path.
func (f *Fs) deleteFolder(ctx context.Context, fullPath string) error {
	fullPath = f.FromStandardPath(fullPath)
	deleteURL := fmt.Sprintf("%s/folder/delete?folder_path=%s&key=%s",
		f.endpoint,
		url.QueryEscape(fullPath),
		url.QueryEscape(f.opt.Key),
	)

	delResp := DeleteFolderResponse{}
	err := f.pacer.Call(func() (bool, error) {
		req, err := http.NewRequestWithContext(ctx, "GET", deleteURL, nil)
		if err != nil {
			return false, err
		}
		resp, err := f.client.Do(req)
		if err != nil {
			return fserrors.ShouldRetry(err), err
		}
		defer resp.Body.Close()

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return false, err
		}

		if err := json.Unmarshal(body, &delResp); err != nil {
			return false, fmt.Errorf("error decoding delete response: %w", err)
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
	filePath = f.FromStandardPath(filePath)
	apiURL := fmt.Sprintf("%s/file/direct_link?file_path=%s&key=%s",
		f.endpoint,
		url.QueryEscape(filePath),
		url.QueryEscape(f.opt.Key),
	)

	result := FileDirectLinkResponse{}
	err := f.pacer.Call(func() (bool, error) {
		req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
		if err != nil {
			return false, fmt.Errorf("failed to create request: %w", err)
		}

		resp, err := f.client.Do(req)
		if err != nil {
			return shouldRetry(err), fmt.Errorf("failed to fetch direct link: %w", err)
		}
		defer resp.Body.Close()

		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			return false, fmt.Errorf("error decoding response: %w", err)
		}

		if result.Status != 200 {
			return false, fmt.Errorf("API error: %s", result.Msg)
		}

		return shouldRetryHTTP(resp.StatusCode), nil
	})
	if err != nil {
		return "", 0, err
	}

	return result.Result.URL, result.Result.Size, nil
}

// deleteFile deletes a file based on filePath
func (f *Fs) deleteFile(ctx context.Context, filePath string) error {
	filePath = f.FromStandardPath(filePath)
	opts := rest.Opts{
		Method: "GET",
		Path:   "/file/remove",
		Parameters: url.Values{
			"file_path": {filePath},
			"key":       {f.opt.Key},
		},
	}

	result := DeleteFileResponse{}
	err := f.pacer.Call(func() (bool, error) {
		_, err := f.srv.CallJSON(ctx, &opts, nil, &result)
		return fserrors.ShouldRetry(err), err
	})

	if err != nil {
		return fmt.Errorf("DeleteFile failed: %w", err)
	}
	if result.Status != 200 {
		return fmt.Errorf("DeleteFile API error: %s", result.Msg)
	}

	fs.Infof(f, "Successfully deleted file: %s", filePath)
	return nil
}

// renameFile renames a file
func (f *Fs) renameFile(ctx context.Context, filePath, newName string) error {
	filePath = f.FromStandardPath(filePath)
	newName = f.FromStandardPath(newName)
	opts := rest.Opts{
		Method: "GET",
		Path:   "/file/rename",
		Parameters: url.Values{
			"file_path": {filePath},
			"name":      {newName},
			"key":       {f.opt.Key},
		},
	}

	var result struct {
		Status int    `json:"status"`
		Result string `json:"result"`
		Msg    string `json:"msg"`
	}

	err := f.pacer.Call(func() (bool, error) {
		_, err := f.srv.CallJSON(ctx, &opts, nil, &result)
		return fserrors.ShouldRetry(err), err
	})

	if err != nil {
		return fmt.Errorf("renameFile failed: %w", err)
	}
	if result.Status != 200 {
		return fmt.Errorf("renameFile API error: %s", result.Msg)
	}

	fs.Infof(f, "Successfully renamed file at path: %s to %s", filePath, newName)
	return nil
}

// moveFolderToDestination moves a folder to a different location within FileLu
func (f *Fs) moveFolderToDestination(ctx context.Context, folderPath string, destFolderPath string) error {
	folderPath = "/" + strings.Trim(folderPath, "/")
	destFolderPath = "/" + strings.Trim(destFolderPath, "/")
	folderPath = f.FromStandardPath(folderPath)
	destFolderPath = f.FromStandardPath(destFolderPath)

	apiURL := fmt.Sprintf("%s/folder/move?folder_path=%s&dest_folder_path=%s&key=%s",
		f.endpoint,
		url.QueryEscape(folderPath),
		url.QueryEscape(destFolderPath),
		url.QueryEscape(f.opt.Key),
	)

	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create move folder request: %w", err)
	}

	var resp *http.Response
	err = f.pacer.Call(func() (bool, error) {
		var err error
		resp, err = f.client.Do(req)
		if err != nil {
			return shouldRetry(err), fmt.Errorf("failed to send move folder request: %w", err)
		}
		return shouldRetryHTTP(resp.StatusCode), nil
	})
	if err != nil {
		return err
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			fs.Fatalf(nil, "Failed to close response body: %v", err)
		}
	}()

	var result struct {
		Status      int    `json:"status"`
		Msg         string `json:"msg"`
		SourceFldID string `json:"source_fld_id"`
		DestFldID   string `json:"dest_fld_id"`
	}

	err = json.NewDecoder(resp.Body).Decode(&result)
	if err != nil {
		return fmt.Errorf("error decoding move folder response: %w", err)
	}

	if result.Status != 200 {
		return fmt.Errorf("error while moving folder: %s", result.Msg)
	}

	fs.Infof(f, "Successfully moved folder from %s to %s", folderPath, destFolderPath)
	return nil
}

// moveFileToDestination moves a file to a different folder using file paths
func (f *Fs) moveFileToDestination(ctx context.Context, filePath string, destinationFolderPath string) error {
	filePath = "/" + strings.Trim(filePath, "/")
	destinationFolderPath = "/" + strings.Trim(destinationFolderPath, "/")
	filePath = f.FromStandardPath(filePath)
	destinationFolderPath = f.FromStandardPath(destinationFolderPath)

	apiURL := fmt.Sprintf("%s/file/set_folder?file_path=%s&destination_folder_path=%s&key=%s",
		f.endpoint,
		url.QueryEscape(filePath),
		url.QueryEscape(destinationFolderPath),
		url.QueryEscape(f.opt.Key),
	)

	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create move request: %w", err)
	}

	var resp *http.Response
	err = f.pacer.Call(func() (bool, error) {
		var err error
		resp, err = f.client.Do(req)
		if err != nil {
			return shouldRetry(err), fmt.Errorf("failed to send move request: %w", err)
		}
		return shouldRetryHTTP(resp.StatusCode), nil
	})
	if err != nil {
		return err
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			fs.Fatalf(nil, "Failed to close response body: %v", err)
		}
	}()

	var result struct {
		Status int    `json:"status"`
		Msg    string `json:"msg"`
	}

	err = json.NewDecoder(resp.Body).Decode(&result)
	if err != nil {
		return fmt.Errorf("error decoding move response: %w", err)
	}

	if result.Status != 200 {
		return fmt.Errorf("error while moving file: %s", result.Msg)
	}

	fs.Infof(f, "Successfully moved file from %s to folder %s", filePath, destinationFolderPath)
	return nil
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

func (f *Fs) getFileInfo(ctx context.Context, fileCode string) (*FileInfoResponse, error) {
	u, _ := url.Parse(f.endpoint + "/file/info2")
	q := u.Query()
	q.Set("file_code", fileCode) // raw path — Go handles escaping properly here
	q.Set("key", f.opt.Key)
	u.RawQuery = q.Encode()

	apiURL := f.endpoint + "/file/info2?" + u.RawQuery

	var body []byte
	err := f.pacer.Call(func() (bool, error) {
		req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
		if err != nil {
			return false, fmt.Errorf("failed to create request: %w", err)
		}

		resp, err := f.client.Do(req)
		if err != nil {
			return shouldRetry(err), fmt.Errorf("failed to fetch file info: %w", err)
		}
		defer resp.Body.Close()

		body, err = io.ReadAll(resp.Body)
		if err != nil {
			return false, fmt.Errorf("error reading response body: %w", err)
		}

		return shouldRetryHTTP(resp.StatusCode), nil
	})
	if err != nil {
		return nil, err
	}
	result := FileInfoResponse{}

	if err := json.NewDecoder(bytes.NewReader(body)).Decode(&result); err != nil {
		return nil, fmt.Errorf("error decoding response: %w", err)
	}

	if result.Status != 200 || len(result.Result) == 0 {
		return nil, fs.ErrorObjectNotFound
	}

	return &result, nil
}
