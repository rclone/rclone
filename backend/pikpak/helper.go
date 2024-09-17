package pikpak

import (
	"bytes"
	"context"
	"crypto/md5"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/rclone/rclone/backend/pikpak/api"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config/configmap"
	"github.com/rclone/rclone/fs/fserrors"
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
	if src.Size() == 0 {
		// If src is zero-length, the API will return
		// Error "cid and file_size is required" (400)
		// In this case, we can simply return cid == gcid
		return cid, nil
	}

	params := url.Values{}
	params.Set("cid", cid)
	params.Set("file_size", strconv.FormatInt(src.Size(), 10))
	opts := rest.Opts{
		Method:     "GET",
		Path:       "/drive/v1/resource/cid",
		Parameters: params,
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
			psize <<= 1
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

// ------------------------------------------------------------ authorization

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

var md5Salt = []string{
	"C9qPpZLN8ucRTaTiUMWYS9cQvWOE",
	"+r6CQVxjzJV6LCV",
	"F",
	"pFJRC",
	"9WXYIDGrwTCz2OiVlgZa90qpECPD6olt",
	"/750aCr4lm/Sly/c",
	"RB+DT/gZCrbV",
	"",
	"CyLsf7hdkIRxRm215hl",
	"7xHvLi2tOYP0Y92b",
	"ZGTXXxu8E/MIWaEDB+Sm/",
	"1UI3",
	"E7fP5Pfijd+7K+t6Tg/NhuLq0eEUVChpJSkrKxpO",
	"ihtqpG6FMt65+Xk+tWUH2",
	"NhXXU9rg4XXdzo7u5o",
}

func md5Sum(text string) string {
	hash := md5.Sum([]byte(text))
	return hex.EncodeToString(hash[:])
}

func calcCaptchaSign(deviceID string) (timestamp, sign string) {
	timestamp = fmt.Sprint(time.Now().UnixMilli())
	str := fmt.Sprint(clientID, clientVersion, packageName, deviceID, timestamp)
	for _, salt := range md5Salt {
		str = md5Sum(str + salt)
	}
	sign = "1." + str
	return
}

func newCaptchaTokenRequest(action, oldToken string, opt *Options) (req *api.CaptchaTokenRequest) {
	req = &api.CaptchaTokenRequest{
		Action:       action,
		CaptchaToken: oldToken, // can be empty initially
		ClientID:     clientID,
		DeviceID:     opt.DeviceID,
		Meta:         new(api.CaptchaTokenMeta),
	}
	switch action {
	case "POST:/v1/auth/signin":
		req.Meta.UserName = opt.Username
	default:
		timestamp, captchaSign := calcCaptchaSign(opt.DeviceID)
		req.Meta.CaptchaSign = captchaSign
		req.Meta.Timestamp = timestamp
		req.Meta.ClientVersion = clientVersion
		req.Meta.PackageName = packageName
		req.Meta.UserID = opt.UserID
	}
	return
}

// CaptchaTokenSource stores updated captcha tokens in the config file
type CaptchaTokenSource struct {
	mu    sync.Mutex
	m     configmap.Mapper
	opt   *Options
	token *api.CaptchaToken
	ctx   context.Context
	rst   *pikpakClient
}

// initialize CaptchaTokenSource from rclone.conf if possible
func newCaptchaTokenSource(ctx context.Context, opt *Options, m configmap.Mapper) *CaptchaTokenSource {
	token := new(api.CaptchaToken)
	tokenString, ok := m.Get("captcha_token")
	if !ok || tokenString == "" {
		fs.Debugf(nil, "failed to read captcha token out of config file")
	} else {
		if err := json.Unmarshal([]byte(tokenString), token); err != nil {
			fs.Debugf(nil, "failed to parse captcha token out of config file: %v", err)
		}
	}
	return &CaptchaTokenSource{
		m:     m,
		opt:   opt,
		token: token,
		ctx:   ctx,
		rst:   newPikpakClient(getClient(ctx, opt), opt),
	}
}

// requestToken retrieves captcha token from API
func (cts *CaptchaTokenSource) requestToken(ctx context.Context, req *api.CaptchaTokenRequest) (err error) {
	opts := rest.Opts{
		Method:  "POST",
		RootURL: "https://user.mypikpak.com/v1/shield/captcha/init",
	}
	var info *api.CaptchaToken
	_, err = cts.rst.CallJSON(ctx, &opts, &req, &info)
	if err == nil && info.ExpiresIn != 0 {
		// populate to Expiry
		info.Expiry = time.Now().Add(time.Duration(info.ExpiresIn) * time.Second)
		cts.token = info // update with a new one
	}
	return
}

func (cts *CaptchaTokenSource) refreshToken(opts *rest.Opts) (string, error) {
	oldToken := ""
	if cts.token != nil {
		oldToken = cts.token.CaptchaToken
	}
	action := "GET:/drive/v1/about"
	if opts.RootURL == "" && opts.Path != "" {
		action = fmt.Sprintf("%s:%s", opts.Method, opts.Path)
	} else if u, err := url.Parse(opts.RootURL); err == nil {
		action = fmt.Sprintf("%s:%s", opts.Method, u.Path)
	}
	req := newCaptchaTokenRequest(action, oldToken, cts.opt)
	if err := cts.requestToken(cts.ctx, req); err != nil {
		return "", fmt.Errorf("failed to retrieve captcha token from api: %w", err)
	}

	// put it into rclone.conf
	tokenBytes, err := json.Marshal(cts.token)
	if err != nil {
		return "", fmt.Errorf("failed to marshal captcha token: %w", err)
	}
	cts.m.Set("captcha_token", string(tokenBytes))
	return cts.token.CaptchaToken, nil
}

// Invalidate resets existing captcha token for a forced refresh
func (cts *CaptchaTokenSource) Invalidate() {
	cts.mu.Lock()
	cts.token.CaptchaToken = ""
	cts.mu.Unlock()
}

// Token returns a valid captcha token
func (cts *CaptchaTokenSource) Token(opts *rest.Opts) (string, error) {
	cts.mu.Lock()
	defer cts.mu.Unlock()
	if cts.token.Valid() {
		return cts.token.CaptchaToken, nil
	}
	return cts.refreshToken(opts)
}

// pikpakClient wraps rest.Client with a handle of captcha token
type pikpakClient struct {
	opt     *Options
	client  *rest.Client
	captcha *CaptchaTokenSource
}

// newPikpakClient takes an (oauth) http.Client and makes a new api instance for pikpak with
// * error handler
// * root url
// * default headers
func newPikpakClient(c *http.Client, opt *Options) *pikpakClient {
	client := rest.NewClient(c).SetErrorHandler(errorHandler).SetRoot(rootURL)
	for key, val := range map[string]string{
		"Referer":          "https://mypikpak.com/",
		"x-client-id":      clientID,
		"x-client-version": clientVersion,
		"x-device-id":      opt.DeviceID,
		// "x-device-model":   "firefox%2F129.0",
		// "x-device-name":    "PC-Firefox",
		// "x-device-sign": fmt.Sprintf("wdi10.%sxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx", opt.DeviceID),
		// "x-net-work-type":    "NONE",
		// "x-os-version":       "Win32",
		// "x-platform-version": "1",
		// "x-protocol-version": "301",
		// "x-provider-name":    "NONE",
		// "x-sdk-version":      "8.0.3",
	} {
		client.SetHeader(key, val)
	}
	return &pikpakClient{
		client: client,
		opt:    opt,
	}
}

// This should be called right after pikpakClient initialized
func (c *pikpakClient) SetCaptchaTokener(ctx context.Context, m configmap.Mapper) *pikpakClient {
	c.captcha = newCaptchaTokenSource(ctx, c.opt, m)
	return c
}

func (c *pikpakClient) CallJSON(ctx context.Context, opts *rest.Opts, request interface{}, response interface{}) (resp *http.Response, err error) {
	if c.captcha != nil {
		token, err := c.captcha.Token(opts)
		if err != nil || token == "" {
			return nil, fserrors.FatalError(fmt.Errorf("couldn't get captcha token: %v", err))
		}
		if opts.ExtraHeaders == nil {
			opts.ExtraHeaders = make(map[string]string)
		}
		opts.ExtraHeaders["x-captcha-token"] = token
	}
	return c.client.CallJSON(ctx, opts, request, response)
}

func (c *pikpakClient) Call(ctx context.Context, opts *rest.Opts) (resp *http.Response, err error) {
	return c.client.Call(ctx, opts)
}
