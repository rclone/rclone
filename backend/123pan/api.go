package pan123

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/rclone/rclone/backend/123pan/api"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/fserrors"
	"github.com/rclone/rclone/lib/pacer"
	"github.com/rclone/rclone/lib/rest"
)

// retryErrorCodes is a slice of error codes that we will retry
var retryErrorCodes = []int{
	429, // Too Many Requests
	500, // Internal Server Error
	502, // Bad Gateway
	503, // Service Unavailable
	504, // Gateway Timeout
}

// shouldRetry returns a boolean as to whether this resp and err deserve to be retried
func shouldRetry(ctx context.Context, resp *http.Response, err error) (bool, error) {
	if fserrors.ContextError(ctx, &err) {
		return false, err
	}
	return fserrors.ShouldRetry(err) || fserrors.ShouldRetryHTTP(resp, retryErrorCodes), err
}

// getAccessToken gets or refreshes the access token
func (f *Fs) getAccessToken(ctx context.Context) error {
	f.tokenMu.Lock()
	defer f.tokenMu.Unlock()

	// If token is still valid (refresh 5 minutes early), return
	if f.opt.AccessToken != "" && time.Now().Add(5*time.Minute).Before(f.tokenExpiry) {
		// Ensure Authorization header is set
		f.srv.SetHeader("Authorization", "Bearer "+f.opt.AccessToken)
		return nil
	}

	fs.Debugf(f, "Refreshing access token...")

	// Request new token
	req := &api.AccessTokenRequest{
		ClientID:     f.opt.ClientID,
		ClientSecret: f.opt.ClientSecret,
	}

	opts := rest.Opts{
		Method: "POST",
		Path:   "/api/v1/access_token",
	}

	var resp api.AccessTokenResponse
	err := f.pacer.Call(func() (bool, error) {
		httpResp, err := f.srv.CallJSON(ctx, &opts, req, &resp)
		return shouldRetry(ctx, httpResp, err)
	})
	if err != nil {
		return fmt.Errorf("failed to get access token: %w", err)
	}

	if resp.Code != 0 {
		return fmt.Errorf("access token error: %s", resp.Message)
	}

	// Parse expiry time (format: "2025-03-23T15:48:37+08:00")
	expiry, err := time.Parse(time.RFC3339, resp.Data.ExpiredAt)
	if err != nil {
		// Default to 29 days if parsing fails
		expiry = time.Now().Add(29 * 24 * time.Hour)
	}

	f.opt.AccessToken = resp.Data.AccessToken
	f.tokenExpiry = expiry

	// Save token to config
	f.m.Set("access_token", f.opt.AccessToken)
	f.m.Set("token_expiry", expiry.Format(time.RFC3339))

	// Update HTTP client Authorization header
	f.srv.SetHeader("Authorization", "Bearer "+f.opt.AccessToken)

	// Mark that token was just refreshed (VIP level should be refreshed too)
	f.tokenJustRefresh = true

	fs.Debugf(f, "Access token refreshed, expires at: %v", expiry)
	return nil
}

// setQPSLimits sets QPS limits based on VIP level
func (f *Fs) setQPSLimits(vipLevel int) {
	f.vipLevel = vipLevel
	f.isVip = vipLevel > 0
	if f.isVip {
		f.qpsLimits = vipUserQPS
	} else {
		f.qpsLimits = freeUserQPS
	}
}

// initUserLevel initializes user level and QPS limits
// Uses cached VIP level if available, only fetches from API if needed
func (f *Fs) initUserLevel(ctx context.Context, forceRefresh bool) error {
	// Use cached VIP level if available and not forcing refresh
	if !forceRefresh && f.opt.VipLevel >= 0 {
		f.setQPSLimits(f.opt.VipLevel)
		fs.Debugf(f, "Using cached VIP level %d", f.vipLevel)
		f.initAPIPacers(ctx)
		return nil
	}

	// Fetch user info from API
	info, err := f.getUserInfo(ctx)
	if err != nil {
		// On error, use cached value if available, otherwise default to free
		if f.opt.VipLevel >= 0 {
			f.setQPSLimits(f.opt.VipLevel)
			fs.Debugf(f, "API error, using cached VIP level %d", f.vipLevel)
		} else {
			f.setQPSLimits(0)
			fs.Debugf(f, "API error, defaulting to free user limits")
		}
		f.initAPIPacers(ctx)
		return err
	}

	f.uid = info.Data.UID

	// Determine highest VIP level
	vipLevel := 0
	for _, vip := range info.Data.VipInfo {
		if vip.VipLevel > vipLevel {
			vipLevel = vip.VipLevel
		}
	}

	// Cache VIP level to config
	f.m.Set("vip_level", strconv.Itoa(vipLevel))
	f.opt.VipLevel = vipLevel

	// Set QPS limits based on VIP status
	f.setQPSLimits(vipLevel)
	if f.isVip {
		fs.Infof(f, "VIP user detected (level %d), using higher QPS limits", f.vipLevel)
	} else {
		fs.Infof(f, "Free user detected, using standard QPS limits")
	}

	// Initialize per-API pacers
	f.initAPIPacers(ctx)

	return nil
}

// initAPIPacers creates individual pacers for each API based on QPS limits
func (f *Fs) initAPIPacers(ctx context.Context) {
	createPacer := func(qps int) *fs.Pacer {
		minSleep := time.Second / time.Duration(qps)
		return fs.NewPacer(ctx, pacer.NewDefault(
			pacer.MinSleep(minSleep),
			pacer.MaxSleep(maxSleep),
			pacer.DecayConstant(2),
		))
	}

	f.apiPacers["file_list"] = createPacer(f.qpsLimits.FileList)
	f.apiPacers["file_move"] = createPacer(f.qpsLimits.FileMove)
	f.apiPacers["file_delete"] = createPacer(f.qpsLimits.FileDelete)
	f.apiPacers["mkdir"] = createPacer(f.qpsLimits.Mkdir)
	f.apiPacers["download_info"] = createPacer(f.qpsLimits.DownloadInfo)
	f.apiPacers["upload_create"] = createPacer(f.qpsLimits.UploadCreate)
	f.apiPacers["default"] = createPacer(5)
}

// getPacer gets the pacer for a specific API
func (f *Fs) getPacer(apiName string) *fs.Pacer {
	if p, ok := f.apiPacers[apiName]; ok {
		return p
	}
	return f.pacer
}

// callAPI makes an API call with automatic token refresh on token errors
func (f *Fs) callAPI(ctx context.Context, opts *rest.Opts, request, response interface{}, pacerName string) error {
	// Ensure token is valid
	if err := f.getAccessToken(ctx); err != nil {
		return err
	}

	p := f.getPacer(pacerName)
	retryToken := true
	retryVipRefresh := true
	maxAPIRetries := 10 // Maximum retries for API-level 429 errors
	apiRetryCount := 0
	apiRetryDelay := time.Second // Initial delay for API 429 retry

	for {
		var resp *http.Response
		var err error

		err = p.Call(func() (bool, error) {
			resp, err = f.srv.CallJSON(ctx, opts, request, response)
			// Check for HTTP 429 (rate limit) - may need to refresh VIP level
			if resp != nil && resp.StatusCode == 429 && retryVipRefresh {
				retryVipRefresh = false
				fs.Debugf(f, "HTTP rate limit hit (429), refreshing VIP level...")
				_ = f.initUserLevel(ctx, true)
			}
			return shouldRetry(ctx, resp, err)
		})

		if err == nil {
			// Check for API-level errors
			if baseResp, ok := response.(interface{ IsError() bool }); ok && baseResp.IsError() {
				// Get the error code and message
				var code int
				var message string
				if apiResp, ok := response.(*api.BaseResponse); ok {
					code = apiResp.Code
					message = apiResp.Message
				} else if getter, ok := response.(interface{ GetCode() int }); ok {
					code = getter.GetCode()
					if msgGetter, ok := response.(interface{ GetMessage() string }); ok {
						message = msgGetter.GetMessage()
					}
				}

				// Check for API-level rate limit (code 429)
				if code == 429 {
					apiRetryCount++
					if apiRetryCount <= maxAPIRetries {
						fs.Debugf(f, "API rate limit hit (429), retry %d/%d after %v...", apiRetryCount, maxAPIRetries, apiRetryDelay)
						// Refresh VIP level on first rate limit hit
						if apiRetryCount == 1 && retryVipRefresh {
							retryVipRefresh = false
							_ = f.initUserLevel(ctx, true)
						}
						select {
						case <-ctx.Done():
							return ctx.Err()
						case <-time.After(apiRetryDelay):
						}
						// Exponential backoff with cap at 30 seconds
						apiRetryDelay *= 2
						if apiRetryDelay > 30*time.Second {
							apiRetryDelay = 30 * time.Second
						}
						continue
					}
					return fmt.Errorf("API rate limit exceeded after %d retries: %s (code %d)", maxAPIRetries, message, code)
				}

				// Check if token needs refresh (code 401 or token-related message)
				isTokenError := code == 401 || strings.Contains(strings.ToLower(message), "token")
				if isTokenError && retryToken {
					retryToken = false
					f.tokenExpiry = time.Time{} // Force refresh
					if err := f.getAccessToken(ctx); err != nil {
						return err
					}
					// Token was refreshed, also refresh VIP level
					if f.tokenJustRefresh {
						f.tokenJustRefresh = false
						_ = f.initUserLevel(ctx, true)
					}
					continue
				}
				return fmt.Errorf("API error: %s (code %d)", message, code)
			}
			return nil
		}

		return err
	}
}

// getUserInfo gets user information from the API
func (f *Fs) getUserInfo(ctx context.Context) (*api.UserInfoResponse, error) {
	opts := rest.Opts{
		Method: "GET",
		Path:   "/api/v1/user/info",
	}

	var resp api.UserInfoResponse
	err := f.callAPI(ctx, &opts, nil, &resp, "default")
	if err != nil {
		return nil, err
	}

	return &resp, nil
}

// listFiles lists files in a directory
func (f *Fs) listFiles(ctx context.Context, parentID int64, limit int, lastFileID int64) (*api.FileListResponse, error) {
	params := url.Values{}
	params.Set("parentFileId", strconv.FormatInt(parentID, 10))
	params.Set("limit", strconv.Itoa(limit))
	if lastFileID > 0 {
		params.Set("lastFileId", strconv.FormatInt(lastFileID, 10))
	}

	opts := rest.Opts{
		Method:     "GET",
		Path:       "/api/v2/file/list",
		Parameters: params,
	}

	var resp api.FileListResponse
	err := f.callAPI(ctx, &opts, nil, &resp, "file_list")
	if err != nil {
		return nil, err
	}

	return &resp, nil
}

// getDownloadInfo gets download URL for a file
func (f *Fs) getDownloadInfo(ctx context.Context, fileID int64) (*api.DownloadInfoResponse, error) {
	params := url.Values{}
	params.Set("fileId", strconv.FormatInt(fileID, 10))

	opts := rest.Opts{
		Method:     "GET",
		Path:       "/api/v1/file/download_info",
		Parameters: params,
	}

	var resp api.DownloadInfoResponse
	err := f.callAPI(ctx, &opts, nil, &resp, "download_info")
	if err != nil {
		return nil, err
	}

	return &resp, nil
}

// mkdir creates a directory
func (f *Fs) mkdir(ctx context.Context, parentID int64, name string) (*api.MkdirResponse, error) {
	req := &api.MkdirRequest{
		Name:     name,
		ParentID: parentID,
	}

	opts := rest.Opts{
		Method: "POST",
		Path:   "/upload/v1/file/mkdir",
	}

	var resp api.MkdirResponse
	err := f.callAPI(ctx, &opts, req, &resp, "mkdir")
	if err != nil {
		return nil, err
	}

	return &resp, nil
}

// move moves a file to a different directory
func (f *Fs) move(ctx context.Context, fileID, toParentID int64) error {
	req := &api.MoveRequest{
		FileIDs:        []int64{fileID},
		ToParentFileID: toParentID,
	}

	opts := rest.Opts{
		Method: "POST",
		Path:   "/api/v1/file/move",
	}

	var resp api.BaseResponse
	return f.callAPI(ctx, &opts, req, &resp, "file_move")
}

// rename renames a file
func (f *Fs) rename(ctx context.Context, fileID int64, newName string) error {
	req := &api.RenameRequest{
		FileID:   fileID,
		FileName: newName,
	}

	opts := rest.Opts{
		Method: "PUT",
		Path:   "/api/v1/file/name",
	}

	var resp api.BaseResponse
	return f.callAPI(ctx, &opts, req, &resp, "default")
}

// trash moves a file/directory to trash
func (f *Fs) trash(ctx context.Context, fileID int64) error {
	req := &api.TrashRequest{
		FileIDs: []int64{fileID},
	}

	opts := rest.Opts{
		Method: "POST",
		Path:   "/api/v1/file/trash",
	}

	var resp api.BaseResponse
	return f.callAPI(ctx, &opts, req, &resp, "file_delete")
}

// createFile creates a file entry (returns instant upload status)
func (f *Fs) createFile(ctx context.Context, parentID int64, filename, etag string, size int64) (*api.UploadCreateResponse, error) {
	req := &api.UploadCreateRequest{
		ParentFileID: parentID,
		Filename:     filename,
		Etag:         etag,
		Size:         size,
		Duplicate:    2, // Overwrite existing
	}

	opts := rest.Opts{
		Method: "POST",
		Path:   "/upload/v2/file/create",
	}

	var resp api.UploadCreateResponse
	err := f.callAPI(ctx, &opts, req, &resp, "upload_create")
	if err != nil {
		return nil, err
	}

	return &resp, nil
}

// completeUpload completes a multipart upload
func (f *Fs) completeUpload(ctx context.Context, preuploadID string) (*api.UploadCompleteResponse, error) {
	req := &api.UploadCompleteRequest{
		PreuploadID: preuploadID,
	}

	opts := rest.Opts{
		Method: "POST",
		Path:   "/upload/v2/file/upload_complete",
	}

	var resp api.UploadCompleteResponse
	err := f.callAPI(ctx, &opts, req, &resp, "default")
	if err != nil {
		return nil, err
	}

	return &resp, nil
}

// createShare creates a share link for a file
func (f *Fs) createShare(ctx context.Context, fileID int64, fileName string, expireDays int) (*api.ShareCreateResponse, error) {
	req := &api.ShareCreateRequest{
		ShareName:   fileName,
		ShareExpire: expireDays,
		FileIDList:  strconv.FormatInt(fileID, 10),
	}

	opts := rest.Opts{
		Method: "POST",
		Path:   "/api/v1/share/create",
	}

	var resp api.ShareCreateResponse
	err := f.callAPI(ctx, &opts, req, &resp, "default")
	if err != nil {
		return nil, err
	}

	return &resp, nil
}
