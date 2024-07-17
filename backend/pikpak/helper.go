package pikpak

import (
	"bytes"
	"context"
	"crypto/sha1"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/rclone/rclone/backend/pikpak/api"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/lib/rest"
)

// Globals
const (
	cachePrefix = "rclone-pikpak-gcid-"
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
		Method: "POST",
		Path:   "/drive/v1/files:" + action,
	}
	info := struct {
		TaskID string `json:"task_id"`
	}{}
	var resp *http.Response
	err = f.pacer.Call(func() (bool, error) {
		resp, err = f.rst.CallJSON(ctx, &opts, &req, &info)
		return f.shouldRetry(ctx, resp, err)
	})
	if err != nil {
		return fmt.Errorf("batch action %q failed: %w", action, err)
	}
	return f.waitTask(ctx, info.TaskID)
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
		if err == nil && !info.Links.ApplicationOctetStream.Valid() {
			return true, errors.New("no link")
		}
		return f.shouldRetry(ctx, resp, err)
	})
	if err == nil {
		info.Name = f.opt.Enc.ToStandardName(info.Name)
	}
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

// getTask gets api.Task from API for the ID passed
func (f *Fs) getTask(ctx context.Context, ID string, checkPhase bool) (info *api.Task, err error) {
	opts := rest.Opts{
		Method: "GET",
		Path:   "/drive/v1/tasks/" + ID,
	}
	var resp *http.Response
	err = f.pacer.Call(func() (bool, error) {
		resp, err = f.rst.CallJSON(ctx, &opts, nil, &info)
		if checkPhase {
			if err == nil && info.Phase != api.PhaseTypeComplete {
				// could be pending right after the task is created
				return true, fmt.Errorf("%s (%s) is still in %s", info.Name, info.Type, info.Phase)
			}
		}
		return f.shouldRetry(ctx, resp, err)
	})
	return
}

// waitTask waits for async tasks to be completed
func (f *Fs) waitTask(ctx context.Context, ID string) (err error) {
	time.Sleep(taskWaitTime)
	if info, err := f.getTask(ctx, ID, true); err != nil {
		if info == nil {
			return fmt.Errorf("can't verify the task is completed: %q", ID)
		}
		return fmt.Errorf("can't verify the task is completed: %#v", info)
	}
	return
}

// deleteTask remove a task having the specified ID
func (f *Fs) deleteTask(ctx context.Context, ID string, deleteFiles bool) (err error) {
	params := url.Values{}
	params.Set("delete_files", strconv.FormatBool(deleteFiles))
	params.Set("task_ids", ID)
	opts := rest.Opts{
		Method:     "DELETE",
		Path:       "/drive/v1/tasks",
		Parameters: params,
		NoResponse: true,
	}
	var resp *http.Response
	err = f.pacer.Call(func() (bool, error) {
		resp, err = f.rst.CallJSON(ctx, &opts, nil, nil)
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

// getGcid retrieves Gcid cached in API server
func (f *Fs) getGcid(ctx context.Context, src fs.ObjectInfo) (gcid string, err error) {
	cid, err := calcCid(ctx, src)
	if err != nil {
		return
	}

	params := url.Values{}
	params.Set("cid", cid)
	params.Set("file_size", strconv.FormatInt(src.Size(), 10))
	opts := rest.Opts{
		Method:       "GET",
		Path:         "/drive/v1/resource/cid",
		Parameters:   params,
		ExtraHeaders: map[string]string{"x-device-id": f.deviceID},
	}

	info := struct {
		Gcid string `json:"gcid,omitempty"`
	}{}
	var resp *http.Response
	err = f.pacer.Call(func() (bool, error) {
		resp, err = f.rst.CallJSON(ctx, &opts, nil, &info)
		return f.shouldRetry(ctx, resp, err)
	})
	if err != nil {
		return "", err
	}
	return info.Gcid, nil
}

// Read the gcid of in returning a reader which will read the same contents
//
// The cleanup function should be called when out is finished with
// regardless of whether this function returned an error or not.
func readGcid(in io.Reader, size, threshold int64) (gcid string, out io.Reader, cleanup func(), err error) {
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

		// use the teeReader to write to the local file AND calculate the gcid while doing so
		teeReader := io.TeeReader(in, tempFile)

		// copy the ENTIRE file to disk and calculate the gcid in the process
		if gcid, err = calcGcid(teeReader, size); err != nil {
			return
		}
		// jump to the start of the local file so we can pass it along
		if _, err = tempFile.Seek(0, 0); err != nil {
			return
		}

		// replace the already read source with a reader of our cached file
		out = tempFile
	} else {
		buf := &bytes.Buffer{}
		teeReader := io.TeeReader(in, buf)

		if gcid, err = calcGcid(teeReader, size); err != nil {
			return
		}
		out = buf
	}
	return
}

// calcGcid calculates Gcid from reader
//
// Gcid is a custom hash to index a file contents
func calcGcid(r io.Reader, size int64) (string, error) {
	calcBlockSize := func(j int64) int64 {
		var psize int64 = 0x40000
		for float64(j)/float64(psize) > 0x200 && psize < 0x200000 {
			psize = psize << 1
		}
		return psize
	}

	totalHash := sha1.New()
	blockHash := sha1.New()
	readSize := calcBlockSize(size)
	for {
		blockHash.Reset()
		if n, err := io.CopyN(blockHash, r, readSize); err != nil && n == 0 {
			if err != io.EOF {
				return "", err
			}
			break
		}
		totalHash.Write(blockHash.Sum(nil))
	}
	return hex.EncodeToString(totalHash.Sum(nil)), nil
}

// calcCid calculates Cid from source
//
// Cid is a simplified version of Gcid
func calcCid(ctx context.Context, src fs.ObjectInfo) (cid string, err error) {
	srcObj := fs.UnWrapObjectInfo(src)
	if srcObj == nil {
		return "", fmt.Errorf("failed to unwrap object from src: %s", src)
	}

	size := src.Size()
	hash := sha1.New()
	var rc io.ReadCloser

	readHash := func(start, length int64) (err error) {
		end := start + length - 1
		if rc, err = srcObj.Open(ctx, &fs.RangeOption{Start: start, End: end}); err != nil {
			return fmt.Errorf("failed to open src with range (%d, %d): %w", start, end, err)
		}
		defer fs.CheckClose(rc, &err)
		_, err = io.Copy(hash, rc)
		return err
	}

	if size <= 0xF000 { // 61440 = 60KB
		err = readHash(0, size)
	} else { // 20KB from three different parts
		for _, start := range []int64{0, size / 3, size - 0x5000} {
			err = readHash(start, 0x5000)
			if err != nil {
				break
			}
		}
	}
	if err != nil {
		return "", fmt.Errorf("failed to hash: %w", err)
	}
	cid = strings.ToUpper(hex.EncodeToString(hash.Sum(nil)))
	return
}

// randomly generates device id used for request header 'x-device-id'
//
// original javascript implementation
//
//	return "xxxxxxxxxxxx4xxxyxxxxxxxxxxxxxxx".replace(/[xy]/g, (e) => {
//	    const t = (16 * Math.random()) | 0;
//	    return ("x" == e ? t : (3 & t) | 8).toString(16);
//	});
func genDeviceID() string {
	base := []byte("xxxxxxxxxxxx4xxxyxxxxxxxxxxxxxxx")
	for i, char := range base {
		switch char {
		case 'x':
			base[i] = fmt.Sprintf("%x", rand.Intn(16))[0]
		case 'y':
			base[i] = fmt.Sprintf("%x", rand.Intn(16)&3|8)[0]
		}
	}
	return string(base)
}
