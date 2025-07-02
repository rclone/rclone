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

// createFolder creates a folder at the specified path.
func (f *Fs) createFolder(ctx context.Context, dirPath string) (*api.CreateFolderResponse, error) {
	encodedDir := f.fromStandardPath(dirPath)
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
	result := api.CreateFolderResponse{}
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

// getFolderList List both files and folders in a directory.
func (f *Fs) getFolderList(ctx context.Context, path string) (*api.FolderListResponse, error) {
	encodedDir := f.fromStandardPath(path)
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
		defer func() {
			if err := resp.Body.Close(); err != nil {
				fs.Logf(nil, "Failed to close response body: %v", err)
			}
		}()

		body, err = io.ReadAll(resp.Body)
		if err != nil {
			return false, fmt.Errorf("error reading response body: %w", err)
		}

		return shouldRetryHTTP(resp.StatusCode), nil
	})
	if err != nil {
		return nil, err
	}

	var response api.FolderListResponse
	if err := json.NewDecoder(bytes.NewReader(body)).Decode(&response); err != nil {
		return nil, fmt.Errorf("error decoding response: %w", err)
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
	deleteURL := fmt.Sprintf("%s/folder/delete?folder_path=%s&key=%s",
		f.endpoint,
		url.QueryEscape(fullPath),
		url.QueryEscape(f.opt.Key),
	)

	delResp := api.DeleteFolderResponse{}
	err := f.pacer.Call(func() (bool, error) {
		req, err := http.NewRequestWithContext(ctx, "GET", deleteURL, nil)
		if err != nil {
			return false, err
		}
		resp, err := f.client.Do(req)
		if err != nil {
			return fserrors.ShouldRetry(err), err
		}
		defer func() {
			if err := resp.Body.Close(); err != nil {
				fs.Logf(nil, "Failed to close response body: %v", err)
			}
		}()

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
	filePath = f.fromStandardPath(filePath)
	apiURL := fmt.Sprintf("%s/file/direct_link?file_path=%s&key=%s",
		f.endpoint,
		url.QueryEscape(filePath),
		url.QueryEscape(f.opt.Key),
	)

	result := api.FileDirectLinkResponse{}
	err := f.pacer.Call(func() (bool, error) {
		req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
		if err != nil {
			return false, fmt.Errorf("failed to create request: %w", err)
		}

		resp, err := f.client.Do(req)
		if err != nil {
			return shouldRetry(err), fmt.Errorf("failed to fetch direct link: %w", err)
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
		return "", 0, err
	}

	return result.Result.URL, result.Result.Size, nil
}

// deleteFile deletes a file based on filePath
func (f *Fs) deleteFile(ctx context.Context, filePath string) error {
	filePath = f.fromStandardPath(filePath)
	apiURL := fmt.Sprintf("%s/file/remove?file_path=%s&key=%s",
		f.endpoint,
		url.QueryEscape(filePath),
		url.QueryEscape(f.opt.Key),
	)

	result := api.DeleteFileResponse{}
	err := f.pacer.Call(func() (bool, error) {
		req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
		if err != nil {
			return false, fmt.Errorf("failed to create request: %w", err)
		}

		resp, err := f.client.Do(req)
		if err != nil {
			return shouldRetry(err), fmt.Errorf("failed to fetch direct link: %w", err)
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
		defer func() {
			if err := resp.Body.Close(); err != nil {
				fs.Logf(nil, "Failed to close response body: %v", err)
			}
		}()

		body, err = io.ReadAll(resp.Body)
		if err != nil {
			return false, fmt.Errorf("error reading response body: %w", err)
		}

		return shouldRetryHTTP(resp.StatusCode), nil
	})
	if err != nil {
		return nil, err
	}
	result := api.FileInfoResponse{}

	if err := json.NewDecoder(bytes.NewReader(body)).Decode(&result); err != nil {
		return nil, fmt.Errorf("error decoding response: %w", err)
	}

	if result.Status != 200 || len(result.Result) == 0 {
		return nil, fs.ErrorObjectNotFound
	}

	return &result, nil
}
