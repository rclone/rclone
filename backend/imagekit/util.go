package imagekit

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/rclone/rclone/backend/imagekit/client/api"
	"github.com/rclone/rclone/backend/imagekit/client/api/media"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/fserrors"
	"github.com/rclone/rclone/lib/pacer"
)

func (f *Fs) getFiles(ctx context.Context, path string, includeVersions bool) (files []media.File, err error) {

	files = make([]media.File, 0)

	var hasMore = true

	for hasMore {
		err = f.pacer.Call(func() (bool, error) {
			var res *media.FilesResponse
			res, err = f.ik.Media.Files(ctx, media.FilesOrFolderParam{
				Skip:  len(files),
				Limit: 100,
				Path:  path,
			}, includeVersions)

			if len(res.Data) == 0 && len(res.Data) < 100 {
				hasMore = false
			} else {
				files = append(files, res.Data...)
			}

			return f.shouldRetry(ctx, &res.Response, err)
		})
	}

	if err != nil {
		return make([]media.File, 0), err
	}

	return files, nil
}

func (f *Fs) getFolders(ctx context.Context, path string) (folders []media.Folder, err error) {

	folders = make([]media.Folder, 0)

	var hasMore = true

	for hasMore {
		err = f.pacer.Call(func() (bool, error) {
			var res *media.FoldersResponse
			res, err = f.ik.Media.Folders(ctx, media.FilesOrFolderParam{
				Skip:  len(folders),
				Limit: 10,
				Path:  path,
			})

			if len(res.Data) == 0 && len(res.Data) < 100 {
				hasMore = false
			} else {
				folders = append(folders, res.Data...)
			}

			return f.shouldRetry(ctx, &res.Response, err)
		})
	}

	if err != nil {
		return make([]media.Folder, 0), err
	}

	return folders, nil
}

func (f *Fs) getFileByName(ctx context.Context, path string, name string) (file *media.File) {

	err := f.pacer.Call(func() (bool, error) {
		res, err := f.ik.Media.Files(ctx, media.FilesOrFolderParam{
			Limit:       1,
			Path:        path,
			SearchQuery: fmt.Sprintf(`type = "file" AND name = %s`, strconv.Quote(name)),
		}, false)

		if len(res.Data) == 0 {
			file = nil
		} else {
			file = &res.Data[0]
		}

		return f.shouldRetry(ctx, &res.Response, err)
	})

	if err != nil {
		return nil
	}

	return file
}

func (f *Fs) getFolderByName(ctx context.Context, path string, name string) (folder *media.Folder) {

	err := f.pacer.Call(func() (bool, error) {
		res, err := f.ik.Media.Folders(ctx, media.FilesOrFolderParam{
			Limit:       1,
			Path:        path,
			SearchQuery: fmt.Sprintf(`type = "folder" AND name = %s`, strconv.Quote(name)),
		})

		if len(res.Data) == 0 {
			folder = nil
		} else {
			folder = &res.Data[0]
		}

		return f.shouldRetry(ctx, &res.Response, err)
	})

	if err != nil {
		return nil
	}

	return folder
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

func ShouldRetryHTTP(resp *api.Response, retryErrorCodes []int) bool {
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

func (f *Fs) shouldRetry(ctx context.Context, resp *api.Response, err error) (bool, error) {
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

	return fserrors.ShouldRetry(err) || ShouldRetryHTTP(resp, retryErrorCodes), err
}

func (f *Fs) EncodePath(str string) string {
	return f.opt.Enc.FromStandardPath(str)
}

func (f *Fs) DecodePath(str string) string {
	return f.opt.Enc.ToStandardPath(str)
}

func (f *Fs) EncodeFileName(str string) string {
	return f.opt.Enc.FromStandardName(str)
}

func (f *Fs) DecodeFileName(str string) string {
	return f.opt.Enc.ToStandardName(str)
}
