package pikpak

import (
	"bytes"
	"context"
	"crypto/sha1"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"

	"github.com/rclone/rclone/backend/pikpak/api"
	"github.com/rclone/rclone/lib/rest"
)

// Globals
const (
	cachePrefix = "rclone-pikpak-sha1sum-"
)

// requestDecompress requests decompress of compressed files
func (f *Fs) requestDecompress(ctx context.Context, file *api.File, password string) (info *api.DecompressResult, err error) {
	req := &api.RequestDecompress{
		Gcid:          file.Hash,
		Password:      password,
		FileID:        file.ID,
		Files:         []*api.FileInArchive{},
		DefaultParent: true,
	}
	opts := rest.Opts{
		Method: "POST",
		Path:   "/decompress/v1/decompress",
	}
	var resp *http.Response
	err = f.pacer.Call(func() (bool, error) {
		resp, err = f.rst.CallJSON(ctx, &opts, &req, &info)
		return f.shouldRetry(ctx, resp, err)
	})
	return
}

// getUserInfo gets UserInfo from API
func (f *Fs) getUserInfo(ctx context.Context) (info *api.User, err error) {
	opts := rest.Opts{
		Method:  "GET",
		RootURL: "https://user.mypikpak.com/v1/user/me",
	}
	var resp *http.Response
	err = f.pacer.Call(func() (bool, error) {
		resp, err = f.rst.CallJSON(ctx, &opts, nil, &info)
		return f.shouldRetry(ctx, resp, err)
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get userinfo: %w", err)
	}
	return
}

// getVIPInfo gets VIPInfo from API
func (f *Fs) getVIPInfo(ctx context.Context) (info *api.VIP, err error) {
	opts := rest.Opts{
		Method:  "GET",
		RootURL: "https://api-drive.mypikpak.com/drive/v1/privilege/vip",
	}
	var resp *http.Response
	err = f.pacer.Call(func() (bool, error) {
		resp, err = f.rst.CallJSON(ctx, &opts, nil, &info)
		return f.shouldRetry(ctx, resp, err)
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get vip info: %w", err)
	}
	return
}

// requestBatchAction requests batch actions to API
//
// action can be one of batch{Copy,Delete,Trash,Untrash}
func (f *Fs) requestBatchAction(ctx context.Context, action string, req *api.RequestBatch) (err error) {
	opts := rest.Opts{
		Method:     "POST",
		Path:       "/drive/v1/files:" + action,
		NoResponse: true, // Only returns `{"task_id":""}
	}
	var resp *http.Response
	err = f.pacer.Call(func() (bool, error) {
		resp, err = f.rst.CallJSON(ctx, &opts, &req, nil)
		return f.shouldRetry(ctx, resp, err)
	})
	if err != nil {
		return fmt.Errorf("batch action %q failed: %w", action, err)
	}
	return nil
}

// requestNewTask requests a new api.NewTask and returns api.Task
func (f *Fs) requestNewTask(ctx context.Context, req *api.RequestNewTask) (info *api.Task, err error) {
	opts := rest.Opts{
		Method: "POST",
		Path:   "/drive/v1/files",
	}
	var newTask api.NewTask
	var resp *http.Response
	err = f.pacer.Call(func() (bool, error) {
		resp, err = f.rst.CallJSON(ctx, &opts, &req, &newTask)
		return f.shouldRetry(ctx, resp, err)
	})
	if err != nil {
		return nil, err
	}
	return newTask.Task, nil
}

// requestNewFile requests a new api.NewFile and returns api.File
func (f *Fs) requestNewFile(ctx context.Context, req *api.RequestNewFile) (info *api.NewFile, err error) {
	opts := rest.Opts{
		Method: "POST",
		Path:   "/drive/v1/files",
	}
	var resp *http.Response
	err = f.pacer.Call(func() (bool, error) {
		resp, err = f.rst.CallJSON(ctx, &opts, &req, &info)
		return f.shouldRetry(ctx, resp, err)
	})
	return
}

// getFile gets api.File from API for the ID passed
// and returns rich information containing additional fields below
// * web_content_link
// * thumbnail_link
// * links
// * medias
func (f *Fs) getFile(ctx context.Context, ID string) (info *api.File, err error) {
	opts := rest.Opts{
		Method: "GET",
		Path:   "/drive/v1/files/" + ID,
	}
	var resp *http.Response
	err = f.pacer.Call(func() (bool, error) {
		resp, err = f.rst.CallJSON(ctx, &opts, nil, &info)
		if err == nil && info.Phase != api.PhaseTypeComplete {
			// could be pending right after file is created/uploaded.
			return true, errors.New("not PHASE_TYPE_COMPLETE")
		}
		return f.shouldRetry(ctx, resp, err)
	})
	return
}

// patchFile updates attributes of the file by ID
//
// currently known patchable fields are
// * name
func (f *Fs) patchFile(ctx context.Context, ID string, req *api.File) (info *api.File, err error) {
	opts := rest.Opts{
		Method: "PATCH",
		Path:   "/drive/v1/files/" + ID,
	}
	var resp *http.Response
	err = f.pacer.Call(func() (bool, error) {
		resp, err = f.rst.CallJSON(ctx, &opts, &req, &info)
		return f.shouldRetry(ctx, resp, err)
	})
	return
}

// getAbout gets drive#quota information from server
func (f *Fs) getAbout(ctx context.Context) (info *api.About, err error) {
	opts := rest.Opts{
		Method: "GET",
		Path:   "/drive/v1/about",
	}
	var resp *http.Response
	err = f.pacer.Call(func() (bool, error) {
		resp, err = f.rst.CallJSON(ctx, &opts, nil, &info)
		return f.shouldRetry(ctx, resp, err)
	})
	return
}

// requestShare returns information about sharable links
func (f *Fs) requestShare(ctx context.Context, req *api.RequestShare) (info *api.Share, err error) {
	opts := rest.Opts{
		Method: "POST",
		Path:   "/drive/v1/share",
	}
	var resp *http.Response
	err = f.pacer.Call(func() (bool, error) {
		resp, err = f.rst.CallJSON(ctx, &opts, &req, &info)
		return f.shouldRetry(ctx, resp, err)
	})
	return
}

// Read the sha1 of in returning a reader which will read the same contents
//
// The cleanup function should be called when out is finished with
// regardless of whether this function returned an error or not.
func readSHA1(in io.Reader, size, threshold int64) (sha1sum string, out io.Reader, cleanup func(), err error) {
	// we need an SHA1
	hash := sha1.New()
	// use the teeReader to write to the local file AND calculate the SHA1 while doing so
	teeReader := io.TeeReader(in, hash)

	// nothing to clean up by default
	cleanup = func() {}

	// don't cache small files on disk to reduce wear of the disk
	if size > threshold {
		var tempFile *os.File

		// create the cache file
		tempFile, err = os.CreateTemp("", cachePrefix)
		if err != nil {
			return
		}

		_ = os.Remove(tempFile.Name()) // Delete the file - may not work on Windows

		// clean up the file after we are done downloading
		cleanup = func() {
			// the file should normally already be close, but just to make sure
			_ = tempFile.Close()
			_ = os.Remove(tempFile.Name()) // delete the cache file after we are done - may be deleted already
		}

		// copy the ENTIRE file to disc and calculate the SHA1 in the process
		if _, err = io.Copy(tempFile, teeReader); err != nil {
			return
		}
		// jump to the start of the local file so we can pass it along
		if _, err = tempFile.Seek(0, 0); err != nil {
			return
		}

		// replace the already read source with a reader of our cached file
		out = tempFile
	} else {
		// that's a small file, just read it into memory
		var inData []byte
		inData, err = io.ReadAll(teeReader)
		if err != nil {
			return
		}

		// set the reader to our read memory block
		out = bytes.NewReader(inData)
	}
	return hex.EncodeToString(hash.Sum(nil)), out, cleanup, nil
}
