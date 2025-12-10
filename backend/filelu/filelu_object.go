package filelu

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/hash"
)

// Object describes a FileLu object
type Object struct {
	fs      *Fs
	remote  string
	size    int64
	modTime time.Time
}

// NewObject creates a new Object for the given remote path
func (f *Fs) NewObject(ctx context.Context, remote string) (fs.Object, error) {
	var filePath string
	filePath = path.Join(f.root, remote)
	filePath = "/" + strings.Trim(filePath, "/")

	// Get File code
	fileCode, err := f.getFileCode(ctx, filePath)
	if err != nil {
		return nil, fs.ErrorObjectNotFound
	}

	// Get File info
	fileInfos, err := f.getFileInfo(ctx, fileCode)
	if err != nil {
		return nil, fmt.Errorf("failed to get file info: %w", err)
	}

	fileInfo := fileInfos.Result[0]
	size, _ := strconv.ParseInt(fileInfo.Size, 10, 64)

	returnedRemote := remote
	return &Object{
		fs:      f,
		remote:  returnedRemote,
		size:    size,
		modTime: time.Now(),
	}, nil
}

// Open opens the object for reading
func (o *Object) Open(ctx context.Context, options ...fs.OpenOption) (io.ReadCloser, error) {
	filePath := path.Join(o.fs.root, o.remote)

	// 1) Resolve direct link + size
	directLink, size, err := o.fs.getDirectLink(ctx, filePath)
	if err != nil {
		return nil, fmt.Errorf("getDirectLink failed: %w", err)
	}

	o.size = size

	// 2) Parse Range options
	var offset int64
	var count int64 = -1

	fs.FixRangeOption(options, o.size)
	for _, opt := range options {
		switch x := opt.(type) {
		case *fs.RangeOption:
			off, cnt := x.Decode(o.size)
			offset, count = off, cnt
		case *fs.SeekOption:
			offset = x.Offset
			count = o.size - offset
		}
	}

	// 3) Build HTTP request
	req, err := http.NewRequestWithContext(ctx, "GET", directLink, nil)
	if err != nil {
		return nil, err
	}

	// 4) Apply HTTP Range if needed
	if offset > 0 || count >= 0 {
		if count >= 0 {
			req.Header.Set("Range", fmt.Sprintf("bytes=%d-%d", offset, offset+count-1))
		} else {
			req.Header.Set("Range", fmt.Sprintf("bytes=%d-", offset))
		}
	}

	// 5) Perform request
	resp, err := o.fs.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("download request failed: %w", err)
	}

	// 6) Validate response codes
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusPartialContent {
		body, _ := io.ReadAll(resp.Body)
		if cerr := resp.Body.Close(); cerr != nil {
			fs.Logf(o.fs, "failed to close response body: %v", cerr)
		}
		return nil, fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(body))
	}

	// 7) Return streaming body (NO buffering!)
	return resp.Body, nil
}

// Update updates the object with new data
func (o *Object) Update(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) error {
	if src.Size() <= 0 {
		return fs.ErrorCantUploadEmptyFiles
	}

	err := o.fs.uploadFile(ctx, in, o.remote)
	if err != nil {
		return fmt.Errorf("failed to upload file: %w", err)
	}
	o.size = src.Size()
	return nil
}

// Remove deletes the object from FileLu
func (o *Object) Remove(ctx context.Context) error {
	fullPath := "/" + strings.Trim(path.Join(o.fs.root, o.remote), "/")

	err := o.fs.deleteFile(ctx, fullPath)
	if err != nil {
		return err
	}
	fs.Infof(o.fs, "Successfully deleted file: %s", fullPath)
	return nil
}

// Hash returns the MD5 hash of an object
func (o *Object) Hash(ctx context.Context, t hash.Type) (string, error) {
	if t != hash.MD5 {
		return "", hash.ErrUnsupported
	}

	var fileCode string
	if isFileCode(o.fs.root) {
		fileCode = o.fs.root
	} else {
		matches := regexp.MustCompile(`\((.*?)\)`).FindAllStringSubmatch(o.remote, -1)
		for _, match := range matches {
			if len(match) > 1 && len(match[1]) == 12 {
				fileCode = match[1]
				break
			}
		}
	}
	if fileCode == "" {
		return "", fmt.Errorf("no valid file code found in the remote path")
	}

	apiURL := fmt.Sprintf("%s/file/info?file_code=%s&key=%s",
		o.fs.endpoint, url.QueryEscape(fileCode), url.QueryEscape(o.fs.opt.Key))

	var result struct {
		Status int    `json:"status"`
		Msg    string `json:"msg"`
		Result []struct {
			Hash string `json:"hash"`
		} `json:"result"`
	}
	err := o.fs.pacer.Call(func() (bool, error) {
		req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
		if err != nil {
			return false, err
		}
		resp, err := o.fs.client.Do(req)
		if err != nil {
			return shouldRetry(err), err
		}
		defer func() {
			if err := resp.Body.Close(); err != nil {
				fs.Logf(nil, "Failed to close response body: %v", err)
			}
		}()

		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			return false, err
		}
		return shouldRetryHTTP(resp.StatusCode), nil
	})
	if err != nil {
		return "", err
	}
	if result.Status != 200 || len(result.Result) == 0 {
		return "", fmt.Errorf("error: unable to fetch hash: %s", result.Msg)
	}

	return result.Result[0].Hash, nil
}

// String returns a string representation of the object
func (o *Object) String() string {
	return o.remote
}

// Fs returns the parent Fs
func (o *Object) Fs() fs.Info {
	return o.fs
}

// Remote returns the remote path
func (o *Object) Remote() string {
	return o.remote
}

// Size returns the size of the object
func (o *Object) Size() int64 {
	return o.size
}

// ModTime returns the modification time of the object
func (o *Object) ModTime(ctx context.Context) time.Time {
	return o.modTime
}

// SetModTime sets the modification time of the object
func (o *Object) SetModTime(ctx context.Context, modTime time.Time) error {
	return fs.ErrorCantSetModTime
}

// Storable indicates whether the object is storable
func (o *Object) Storable() bool {
	return true
}
