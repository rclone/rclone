package imagekit

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/rclone/rclone/backend/imagekit/client"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/fserrors"
	"github.com/rclone/rclone/lib/pacer"
)

func (f *Fs) getFiles(ctx context.Context, path string, includeVersions bool) (files []client.File, err error) {

	files = make([]client.File, 0)

	var hasMore = true

	for hasMore {
		err = f.pacer.Call(func() (bool, error) {
			var data *[]client.File
			var res *http.Response
			res, data, err = f.ik.Files(ctx, client.FilesOrFolderParam{
				Skip:  len(files),
				Limit: 100,
				Path:  path,
			}, includeVersions)

			hasMore = !(len(*data) == 0 || len(*data) < 100)

			if len(*data) > 0 {
				files = append(files, *data...)
			}

			return f.shouldRetry(ctx, res, err)
		})
	}

	if err != nil {
		return make([]client.File, 0), err
	}

	return files, nil
}

func (f *Fs) getFolders(ctx context.Context, path string) (folders []client.Folder, err error) {

	folders = make([]client.Folder, 0)

	var hasMore = true

	for hasMore {
		err = f.pacer.Call(func() (bool, error) {
			var data *[]client.Folder
			var res *http.Response
			res, data, err = f.ik.Folders(ctx, client.FilesOrFolderParam{
				Skip:  len(folders),
				Limit: 100,
				Path:  path,
			})

			hasMore = !(len(*data) == 0 || len(*data) < 100)

			if len(*data) > 0 {
				folders = append(folders, *data...)
			}

			return f.shouldRetry(ctx, res, err)
		})
	}

	if err != nil {
		return make([]client.Folder, 0), err
	}

	return folders, nil
}

func (f *Fs) getFileByName(ctx context.Context, path string, name string) (file *client.File) {

	err := f.pacer.Call(func() (bool, error) {
		res, data, err := f.ik.Files(ctx, client.FilesOrFolderParam{
			Limit:       1,
			Path:        path,
			SearchQuery: fmt.Sprintf(`type = "file" AND name = %s`, strconv.Quote(name)),
		}, false)

		if len(*data) == 0 {
			file = nil
		} else {
			file = &(*data)[0]
		}

		return f.shouldRetry(ctx, res, err)
	})

	if err != nil {
		return nil
	}

	return file
}

func (f *Fs) getFolderByName(ctx context.Context, path string, name string) (folder *client.Folder, err error) {
	err = f.pacer.Call(func() (bool, error) {
		res, data, err := f.ik.Folders(ctx, client.FilesOrFolderParam{
			Limit:       1,
			Path:        path,
			SearchQuery: fmt.Sprintf(`type = "folder" AND name = %s`, strconv.Quote(name)),
		})

		if len(*data) == 0 {
			folder = nil
		} else {
			folder = &(*data)[0]
		}

		return f.shouldRetry(ctx, res, err)
	})

	if err != nil {
		return nil, err
	}

	return folder, nil
}

// retryErrorCodes is a slice of error codes that we will retry
var retryErrorCodes = []int{
	401, // Unauthorized (e.g. "Token has expired")
	408, // Request Timeout
	429, // Rate exceeded.
	500, // Get occasional 500 Internal Server Error
	503, // Service Unavailable
	504, // Gateway Time-out
}

func shouldRetryHTTP(resp *http.Response, retryErrorCodes []int) bool {
	if resp == nil {
		return false
	}
	for _, e := range retryErrorCodes {
		if resp.StatusCode == e {
			return true
		}
	}
	return false
}

func (f *Fs) shouldRetry(ctx context.Context, resp *http.Response, err error) (bool, error) {
	if fserrors.ContextError(ctx, &err) {
		return false, err
	}

	if resp != nil && (resp.StatusCode == 429 || resp.StatusCode == 503) {
		var retryAfter = 1
		retryAfterString := resp.Header.Get("X-RateLimit-Reset")
		if retryAfterString != "" {
			var err error
			retryAfter, err = strconv.Atoi(retryAfterString)
			if err != nil {
				fs.Errorf(f, "Malformed %s header %q: %v", "X-RateLimit-Reset", retryAfterString, err)
			}
		}

		return true, pacer.RetryAfterError(err, time.Duration(retryAfter)*time.Millisecond)
	}

	return fserrors.ShouldRetry(err) || shouldRetryHTTP(resp, retryErrorCodes), err
}

// EncodePath encapsulates the logic for encoding a path
func (f *Fs) EncodePath(str string) string {
	return f.opt.Enc.FromStandardPath(str)
}

// DecodePath encapsulates the logic for decoding a path
func (f *Fs) DecodePath(str string) string {
	return f.opt.Enc.ToStandardPath(str)
}

// EncodeFileName encapsulates the logic for encoding a file name
func (f *Fs) EncodeFileName(str string) string {
	return f.opt.Enc.FromStandardName(str)
}

// DecodeFileName encapsulates the logic for decoding a file name
func (f *Fs) DecodeFileName(str string) string {
	return f.opt.Enc.ToStandardName(str)
}
