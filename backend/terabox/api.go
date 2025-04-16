package terabox

import (
	"bytes"
	"context"
	"crypto/md5"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	libPath "path"

	"github.com/rclone/rclone/backend/terabox/api"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/lib/rest"
)

var retryErrorCodes = []int{
	429, // Too Many Requests.
	500, // Internal Server Error
	502, // Bad Gateway
	503, // Service Unavailable
	504, // Gateway Timeout
	509, // Bandwidth Limit Exceeded
}

func (f *Fs) apiExec(ctx context.Context, opts *rest.Opts, res any) error {
	if opts == nil {
		return fmt.Errorf("empty request")
	}
	opts.IgnoreStatus = true

	retry := 0

retry:
	if !f.notFirstRun {
		f.client.SetRoot(f.baseURL)
		f.client.SetHeader("Accept", "application/json, text/plain, */*")
		if f.accessToken == "" {
			f.client.SetHeader("Referer", baseURL)
			f.client.SetHeader("X-Requested-With", "XMLHttpRequest")
			f.client.SetHeader("Cookie", f.opt.Cookie)
		}
		f.notFirstRun = true
	}

	if opts.Parameters != nil {
		if f.accessToken == "" {
			opts.Parameters.Add("app_id", "250528")
			opts.Parameters.Add("channel", "dubox")
			opts.Parameters.Add("clienttype", "0")
		} else {
			opts.Parameters.Add("access_tokens", f.accessToken)
		}
	}

	if retry == 0 && opts.Method == http.MethodPost && opts.MultipartParams != nil {
		var overhead int64
		var err error
		opts.Body, opts.ContentType, overhead, err = rest.MultipartUpload(ctx, opts.Body, opts.MultipartParams, opts.MultipartContentName, opts.MultipartFileName)
		if err != nil {
			return err
		}
		if opts.ContentLength != nil {
			*opts.ContentLength += overhead
		}
	}

	resp, err := f.client.Call(ctx, opts)
	if err != nil {
		return err
	}

	debug(f.opt, 3, "Request: %+v", resp.Request)
	debug(f.opt, 2, "Response: %+v", resp)

	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	debug(f.opt, 2, "Response body: %s", string(body))

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		err = fmt.Errorf("http error %d: %v", resp.StatusCode, resp.Status)
		fs.Debug(nil, err.Error())
		if IsInSlice(resp.StatusCode, retryErrorCodes) {
			retry++
			if retry > 2 {
				return err
			}

			time.Sleep(time.Duration(retry) * time.Second)
			goto retry
		}
		return err
	}

	if err := json.Unmarshal(body, res); err != nil {
		return err
	}

	if _, skip := res.(*api.ResponseUploadedChunk); !skip {
		if err, ok := res.(api.ErrorInterface); ok {
			if err.Err() != nil {
				return err
			}
		} else {
			return fmt.Errorf("response have no api error interface")
		}
	}

	return nil
}

func (f *Fs) apiJsToken(ctx context.Context) error {
	res, err := f.client.Call(ctx, &rest.Opts{Method: http.MethodGet})
	if err != nil {
		return err
	}

	defer res.Body.Close()
	body, err := io.ReadAll(res.Body)
	if err != nil {
		return err
	}

	jsToken := getStrBetween(string(body), "`function%20fn%28a%29%7Bwindow.jsToken%20%3D%20a%7D%3Bfn%28%22", "%22%29`")
	if jsToken == "" {
		debug(f.opt, 3, "jsToken not found, body: %s", string(body))
		return fmt.Errorf("jsToken not found")
	}

	f.jsToken = jsToken
	return nil
}

func (f *Fs) apiCheckLogin(ctx context.Context) error {
	var res api.ResponseDefault
	err := f.apiExec(ctx, NewRequest(http.MethodGet, "/api/check/login"), &res)
	if err != nil {
		return err
	}

	return nil
}

func (f *Fs) apiList(ctx context.Context, dir string) ([]*api.Item, error) {
	if len(dir) == 0 || dir[0] != '/' {
		dir = "/" + dir
	}

	page := 1
	limit := 100
	opt := NewRequest(http.MethodGet, "/api/list")
	opt.Parameters.Add("dir", dir)
	// opt.Parameters.Add("web", "1") // If 1 is passed, the thumbnail field thumbs will be returned.
	// opt.Parameters.Add("order", ...) // Sorting field: time (modification time), name (file name), size (size; note that directories do not have a size)
	// if true {
	// 	opt.Parameters.Add("desc", "1") // 1: descending order; 0: ascending order
	// }

	list := make([]*api.Item, 0)
	for {
		opt.Parameters.Set("page", strconv.Itoa(page))
		opt.Parameters.Set("num", strconv.Itoa(limit))

		var res api.ResponseList
		err := f.apiExec(ctx, opt, &res)
		if err != nil {
			return nil, err
		}

		list = append(list, res.List...)

		if len(res.List) == 0 || len(res.List) < limit {
			break
		}

		page++
	}

	return list, nil
}

// files info, can return info about a few files, but we're use it for only one file
func (f *Fs) apiItemInfo(ctx context.Context, path string, downloadLink bool) (*api.Item, error) {
	opt := NewRequest(http.MethodGet, "/api/filemetas")
	opt.Parameters.Add("target", fmt.Sprintf(`["%s"]`, path))
	if downloadLink {
		opt.Parameters.Add("dlink", "1")
	} else {
		opt.Parameters.Add("dlink", "0")
	}

	var res api.ResponseItemInfo
	err := f.apiExec(ctx, opt, &res)
	// firstly we will check error for a file, and only then one for full operation {"errno":12,"info":[{"errno":-9}],"request_id":8798843383335989660}
	if len(res.List) > 0 {
		if res.List[0].Err() != nil {
			return nil, res.List[0].Err()
		}

		return &res.List[0].Item, nil
	}

	if err != nil {
		return nil, err
	}

	return nil, fs.ErrorObjectNotFound
}

func (f *Fs) apiMkDir(ctx context.Context, path string) error {
	opt := NewRequest(http.MethodPost, "/api/create")
	opt.MultipartParams = url.Values{}
	opt.MultipartParams.Add("path", path)
	opt.MultipartParams.Add("isdir", "1")
	opt.MultipartParams.Add("rtype", "0") // The file naming policy. The default value is 1. 0: Do not rename. If a file with the same name exists in the cloud, this call will fail and return a conflict; 1: Rename if there is any path conflict; 2: Rename only if there is a path conflict and the block_list is different; 3: Overwrite

	var res api.ResponseDefault
	err := f.apiExec(ctx, opt, &res)
	return err
}

// operation - copy (file copy), move (file movement), rename (file renaming), and delete (file deletion)
// opera=copy: filelist: [{"path":"/hello/test.mp4","dest":"","newname":"test.mp4"}]
// opera=move: filelist: [{"path":"/test.mp4","dest":"/test_dir","newname":"test.mp4"}]
// opera=rename: filelist：[{"path":"/hello/test.mp4","newname":"test_one.mp4"}]
// opera=delete: filelist: ["/test.mp4"]
func (f *Fs) apiOperation(ctx context.Context, operation string, items []api.OperationalItem) error {
	opt := NewRequest(http.MethodPost, "/api/filemanager")
	opt.Parameters.Add("opera", operation)
	opt.Parameters.Add("async", "0") // The default value is 0; 0: synchronous; 1: adaptive; 2: asynchronous. The difference lies in whether to care about the success of the request, and the returned structure differs. Different structures are returned based on the request parameters; see the return examples for details.)

	var list any
	if operation == "delete" {
		list = make([]string, len(items))
		for idx, item := range items {
			list.([]string)[idx] = item.Path
		}
	} else {
		list = items
	}

	mItems, err := json.Marshal(list)
	if err != nil {
		return err
	}

	body := fmt.Sprintf("filelist=%s", strings.ReplaceAll(url.QueryEscape(string(mItems)), "+", "%20"))
	opt.Body = bytes.NewBufferString(body)

	var res api.ResponseOperational
	if err := f.apiExec(ctx, opt, &res); err != nil {
		return err
	}

	for _, oi := range res.Info {
		if oi.Err() != nil {
			return oi.Err()
		}
	}

	if operation == "delete" && f.opt.DeletePermanently {
		if err := f.apiCleanRecycleBin(ctx); err != nil {
			return err
		}
	}

	return nil
}

// Download file
func (f *Fs) apiDownloadLink(ctx context.Context, fileID uint64) (*api.ResponseDownload, error) {
	var err error
	f.signsMX.Do(func() {
		err = f.apiSignPrepare(ctx)
	})
	if err != nil {
		f.signsMX = sync.Once{}
		return nil, err
	}

	opt := NewRequest(http.MethodGet, "/api/download")
	opt.Parameters.Add("type", "dlink")
	opt.Parameters.Add("vip", "2")
	opt.Parameters.Add("sign", sign(f.signs[0], f.signs[1]))
	opt.Parameters.Add("timestamp", fmt.Sprintf("%d", time.Now().Unix()))
	opt.Parameters.Add("need_speed", "1")
	opt.Parameters.Add("fidlist", fmt.Sprintf("[%d]", fileID))

	var res api.ResponseDownload
	if err := f.apiExec(ctx, opt, &res); err != nil {
		return nil, err
	}

	return &res, nil
}

func (f *Fs) apiSignPrepare(ctx context.Context) error {
	opt := NewRequest(http.MethodGet, "/api/home/info")

	var res api.ResponseHomeInfo
	if err := f.apiExec(ctx, opt, &res); err != nil {
		return err
	}

	f.signs = []string{res.Data.Sign3, res.Data.Sign1}
	return nil
}

// Delete files from Recycle Bin
func (f *Fs) apiCleanRecycleBin(ctx context.Context) error {
	opt := NewRequest(http.MethodPost, "/api/recycle/clear")
	opt.Parameters.Add("async", "0") // The default value is 0; 0: synchronous; 1: adaptive; 2: asynchronous. The difference lies in whether to care about the success of the request, and the returned structure differs. Different structures are returned based on the request parameters; see the return examples for details.)

	var res api.ResponseDefault
	if err := f.apiExec(ctx, opt, &res); err != nil {
		return err
	}

	return nil
}

// Quota limits for storage
func (f *Fs) apiQuotaInfo(ctx context.Context) (*api.ResponseQuota, error) {
	opt := NewRequest(http.MethodGet, "/api/quota")
	opt.Parameters.Add("checkexpire", "1")
	opt.Parameters.Add("checkfree", "1")

	var res api.ResponseQuota
	if err := f.apiExec(ctx, opt, &res); err != nil {
		return nil, err
	}

	return &res, nil
}

// Upload file
func (f *Fs) apiFileUpload(ctx context.Context, path string, size int64, modTime time.Time, in io.Reader, options []fs.OpenOption, overwriteMode uint8) error {
	if size > fileLimitSize {
		return api.Num2Err(58)
	}

	// get host for upload
	var err error
	f.uploadHostMX.Do(func() {
		err = f.apiFileLocateUpload(ctx)
	})
	if err != nil {
		f.uploadHostMX = sync.Once{}
		return err
	}

	// get JS token
	if f.jsToken == "" {
		if err := f.apiJsToken(ctx); err != nil {
			return err
		}
	}

	// precreate file
	resPreCreate, err := f.apiFilePrecreate(ctx, path, size, modTime)
	if err != nil {
		return err
	}

	if resPreCreate.Type == 2 {
		return api.Num2Err(-8)
	}

	// upload chunks
	chunksUploaded := map[int]string{}
	chunksUploadedCounter := 0
	chunkData := make([]byte, chunkSize)
	for {
		// check context
		if ctx.Err() != nil {
			return ctx.Err()
		}

		// read chunk
		r, err := in.Read(chunkData)
		if r == 0 && err != nil {
			if errors.Is(err, io.EOF) {
				break
			}

			return err
		}

		// calculate md5
		chunksUploaded[chunksUploadedCounter] = fmt.Sprintf("%x", md5.Sum(chunkData))
		resUpload, err := f.apiFileUploadChunk(ctx, path, resPreCreate.UploadID, chunksUploadedCounter, int64(r), chunkData[:r], options)
		if err != nil {
			return err
		}

		// upload chunk
		if chunksUploaded[chunksUploadedCounter] != resUpload.MD5 {
			debug(f.opt, 1, "uploaded chunk have another md5 then our: %s, uploaded: %s", chunksUploaded[chunksUploadedCounter], resUpload.MD5)
			chunksUploaded[chunksUploadedCounter] = resUpload.MD5
		}

		chunksUploadedCounter++
	}

	chunksUploadedList := make([]string, len(chunksUploaded))
	for k, v := range chunksUploaded {
		chunksUploadedList[k] = v
	}

	// create file
	err = f.apiFileCreate(ctx, path, resPreCreate.UploadID, size, modTime, chunksUploadedList, overwriteMode)
	if err != nil {
		return err
	}

	return nil
}

func (f *Fs) apiFileLocateUpload(ctx context.Context) error {
	opt := NewRequest(http.MethodGet, "https://d.terabox.com/rest/2.0/pcs/file?method=locateupload")
	opt.Parameters = nil

	var res api.ResponseFileLocateUpload
	if err := f.apiExec(ctx, opt, &res); err != nil {
		return err
	}

	f.uploadHost = res.Host

	return nil
}

func (f *Fs) apiFilePrecreate(ctx context.Context, path string, size int64, modTime time.Time) (*api.ResponsePrecreate, error) {
	opt := NewRequest(http.MethodPost, "/api/precreate")
	opt.Parameters.Add("jsToken", f.jsToken)

	opt.MultipartParams = url.Values{}
	opt.MultipartParams.Add("path", path)
	opt.MultipartParams.Add("autoinit", "1")
	opt.MultipartParams.Add("local_mtime", fmt.Sprintf("%d", modTime.Unix()))
	opt.MultipartParams.Add("file_limit_switch_v34", "true")
	opt.MultipartParams.Add("size", fmt.Sprintf("%d", size))

	dirPath, _ := libPath.Split(path)
	opt.MultipartParams.Add("target_path", dirPath)

	if size > chunkSize {
		opt.MultipartParams.Add("block_list", `["5910a591dd8fc18c32a8f3df4fdc1761", "a5fc157d78e6ad1c7e114b056c92821e"]`)
	} else {
		opt.MultipartParams.Add("block_list", `["5910a591dd8fc18c32a8f3df4fdc1761"]`)
	}

	var jsTokenRequested bool
	var res api.ResponsePrecreate
	for {
		err := f.apiExec(ctx, opt, &res)
		if err != nil {
			if api.ErrIsNum(err, 4000023) && !jsTokenRequested {
				jsTokenRequested = true
				if err := f.apiJsToken(ctx); err != nil {
					return nil, err
				}

				opt.Parameters.Set("jsToken", f.jsToken)
				continue
			}

			return nil, err
		}

		break
	}
	return &res, nil
}

func (f *Fs) apiFileUploadChunk(ctx context.Context, path, uploadID string, chunkNumber int, size int64, data []byte, options []fs.OpenOption) (*api.ResponseUploadedChunk, error) {
	opt := NewRequest(http.MethodPost, fmt.Sprintf("https://%s/rest/2.0/pcs/superfile2", f.uploadHost))
	opt.Parameters.Add("method", "upload")
	opt.Parameters.Add("path", path)
	opt.Parameters.Add("uploadid", uploadID)
	opt.Parameters.Add("partseq", fmt.Sprintf("%d", chunkNumber))
	opt.Parameters.Add("uploadsign", "0")
	opt.Options = options

	formReader, contentType, overhead, err := rest.MultipartUpload(ctx, bytes.NewReader(data), opt.MultipartParams, "file", "blob")
	if err != nil {
		return nil, fmt.Errorf("failed to make multipart upload for file: %w", err)
	}
	contentLength := overhead + size
	opt.ContentLength = &contentLength
	opt.ContentType = contentType
	opt.Body = formReader

	var res api.ResponseUploadedChunk
	if err := f.apiExec(ctx, opt, &res); err != nil {
		return nil, err
	}

	return &res, nil
}

func (f *Fs) apiFileCreate(ctx context.Context, path, uploadID string, size int64, modTime time.Time, blockList []string, overwriteMode uint8) error {
	opt := NewRequest(http.MethodPost, "/api/create")
	opt.Parameters.Add("isdir", "0")

	// The file naming policy. The default value is 1. 0: Do not rename. If a file with the same name exists in the cloud, this call will fail and return a conflict; 1: Rename if there is any path conflict; 2: Rename only if there is a path conflict and the block_list is different; 3: Overwrite
	opt.Parameters.Add("rtype", fmt.Sprintf("%d", overwriteMode))
	if overwriteMode > 3 {
		opt.Parameters.Set("rtype", "1")
	}

	opt.MultipartParams = url.Values{}
	opt.MultipartParams.Add("path", path)
	// opt.MultipartParams.Add("isdir", "0") // for dir create this param shold be in body, for upload in URL
	// opt.MultipartParams.Add("rtype", "0") // for dir create this param shold be in body, for upload in URL
	opt.MultipartParams.Add("local_mtime", fmt.Sprintf("%d", modTime.Unix()))
	opt.MultipartParams.Add("uploadid", uploadID)
	opt.MultipartParams.Add("size", fmt.Sprintf("%d", size))

	dirPath, _ := libPath.Split(path)
	opt.MultipartParams.Add("target_path", dirPath)

	blockListStr, err := json.Marshal(blockList)
	if err != nil {
		return err
	}
	opt.MultipartParams.Add("block_list", string(blockListStr))

	debug(f.opt, 3, "%+v", opt.MultipartParams)
	var res api.ResponseDefault
	return f.apiExec(ctx, opt, &res)
}
