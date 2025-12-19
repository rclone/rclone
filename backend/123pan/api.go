package pan123

import (
	"context"
	"fmt"
	"math/rand"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/rclone/rclone/backend/123pan/api"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/fserrors"
	"github.com/rclone/rclone/lib/pacer"
	"github.com/rclone/rclone/lib/rest"
)

const (
	maxAPIRetries     = 10          // Maximum retries for API-level 429 errors
	baseRetryDelay    = time.Second // Base delay for exponential backoff
	maxRetryDelay     = 30 * time.Second
	tokenRefreshEarly = 5 * time.Minute // Refresh token before expiry
)

// retryErrorCodes is a slice of error codes that we will retry
var retryErrorCodes = []int{
	429, // Too Many Requests
	500, // Internal Server Error
	502, // Bad Gateway
	503, // Service Unavailable
	504, // Gateway Timeout
}

// calculateRetryDelay calculates retry delay with exponential backoff and jitter
// The jitter helps prevent thundering herd problem when multiple requests retry simultaneously
func calculateRetryDelay(attempt int, baseDelay time.Duration, maxDelay time.Duration) time.Duration {
	// Exponential backoff: baseDelay * 2^attempt
	delay := baseDelay * time.Duration(1<<uint(attempt))
	if delay > maxDelay {
		delay = maxDelay
	}
	// Add 0-25% random jitter to avoid thundering herd
	jitter := time.Duration(rand.Float64() * 0.25 * float64(delay))
	return delay + jitter
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

	// If token is still valid (refresh early), return
	if f.opt.AccessToken != "" && time.Now().Add(tokenRefreshEarly).Before(f.tokenExpiry) {
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
	f.m.Set("vip_level", formatID(int64(vipLevel)))
	f.opt.VipLevel = vipLevel

	// Set QPS limits based on VIP status
	f.setQPSLimits(vipLevel)
	if f.isVip {
		fs.Debugf(f, "VIP user detected (level %d), using higher QPS limits", f.vipLevel)
	} else {
		fs.Debugf(f, "Free user detected, using standard QPS limits")
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
	f.apiPacers["file_trash"] = createPacer(f.qpsLimits.FileTrash)
	f.apiPacers["file_delete"] = createPacer(f.qpsLimits.FileDelete)
	f.apiPacers["mkdir"] = createPacer(f.qpsLimits.Mkdir)
	f.apiPacers["download_info"] = createPacer(f.qpsLimits.DownloadInfo)
	f.apiPacers["upload_create"] = createPacer(f.qpsLimits.UploadCreate)
	f.apiPacers["upload_complete"] = createPacer(f.qpsLimits.UploadComplete)
	f.apiPacers["file_rename"] = createPacer(f.qpsLimits.FileRename)
	f.apiPacers["share_create"] = createPacer(f.qpsLimits.ShareCreate)
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
func (f *Fs) callAPI(ctx context.Context, opts *rest.Opts, request interface{}, response api.Response, pacerName string) error {
	if err := f.getAccessToken(ctx); err != nil {
		return err
	}

	p := f.getPacer(pacerName)
	retryToken := true
	apiRetryCount := 0

	for {
		err := p.Call(func() (bool, error) {
			resp, err := f.srv.CallJSON(ctx, opts, request, response)
			if resp != nil && resp.StatusCode == 429 && !f.vipRefreshed {
				f.vipRefreshed = true
				fs.Debugf(f, "HTTP rate limit hit (429), refreshing VIP level...")
				_ = f.initUserLevel(ctx, true)
			}
			return shouldRetry(ctx, resp, err)
		})

		if err != nil {
			return err
		}

		// Check for API-level errors
		if !response.IsError() {
			return nil
		}

		code, message := response.GetCode(), response.GetMessage()

		// Handle API-level rate limit (code 429)
		if code == 429 {
			apiRetryCount++
			if apiRetryCount > maxAPIRetries {
				return fmt.Errorf("API rate limit exceeded after %d retries: %s (code %d)", maxAPIRetries, message, code)
			}
			if err := f.handleRateLimit(ctx, apiRetryCount); err != nil {
				return err
			}
			continue
		}

		// Handle token errors (code 401 or token-related message)
		if f.isTokenError(code, message) && retryToken {
			retryToken = false
			if err := f.refreshToken(ctx); err != nil {
				return err
			}
			continue
		}

		return fmt.Errorf("API error: %s (code %d)", message, code)
	}
}

// isTokenError checks if the error is token-related
func (f *Fs) isTokenError(code int, message string) bool {
	return code == 401 || strings.Contains(strings.ToLower(message), "token")
}

// handleRateLimit handles API rate limit with exponential backoff
func (f *Fs) handleRateLimit(ctx context.Context, retryCount int) error {
	retryDelay := calculateRetryDelay(retryCount-1, baseRetryDelay, maxRetryDelay)
	fs.Debugf(f, "API rate limit hit (429), retry %d/%d after %v...", retryCount, maxAPIRetries, retryDelay)

	if retryCount == 1 && !f.vipRefreshed {
		f.vipRefreshed = true
		_ = f.initUserLevel(ctx, true)
	}

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(retryDelay):
		return nil
	}
}

// refreshToken forces a token refresh
func (f *Fs) refreshToken(ctx context.Context) error {
	f.tokenExpiry = time.Time{} // Force refresh
	if err := f.getAccessToken(ctx); err != nil {
		return err
	}
	if f.tokenJustRefresh {
		f.tokenJustRefresh = false
		_ = f.initUserLevel(ctx, true)
	}
	return nil
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
	params.Set("parentFileId", formatID(parentID))
	params.Set("limit", formatID(int64(limit)))
	if lastFileID > 0 {
		params.Set("lastFileId", formatID(lastFileID))
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
	params.Set("fileId", formatID(fileID))

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
	return f.callAPI(ctx, &opts, req, &resp, "file_rename")
}

// trash moves a file/directory to trash
func (f *Fs) trash(ctx context.Context, fileID int64) error {
	return f.trashBatch(ctx, []int64{fileID})
}

// trashBatch moves multiple files/directories to trash (max trashBatchSize per call)
func (f *Fs) trashBatch(ctx context.Context, fileIDs []int64) error {
	return f.batchDeleteFiles(ctx, fileIDs, "/api/v1/file/trash", "file_trash")
}

// deletePermanently permanently deletes files from trash (max trashBatchSize per call)
// Note: Files must be in trash before they can be permanently deleted
func (f *Fs) deletePermanently(ctx context.Context, fileIDs []int64) error {
	return f.batchDeleteFiles(ctx, fileIDs, "/api/v1/file/delete", "file_delete")
}

// batchDeleteFiles is a generic batch delete method for trash and permanent delete
func (f *Fs) batchDeleteFiles(ctx context.Context, fileIDs []int64, path, pacerName string) error {
	if len(fileIDs) == 0 {
		return nil
	}
	if len(fileIDs) > trashBatchSize {
		return fmt.Errorf("batch delete: too many files (%d), max %d per call", len(fileIDs), trashBatchSize)
	}

	req := &api.TrashRequest{FileIDs: fileIDs}
	opts := rest.Opts{Method: "POST", Path: path}
	var resp api.BaseResponse
	return f.callAPI(ctx, &opts, req, &resp, pacerName)
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
	err := f.callAPI(ctx, &opts, req, &resp, "upload_complete")
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
		FileIDList:  formatID(fileID),
	}

	opts := rest.Opts{
		Method: "POST",
		Path:   "/api/v1/share/create",
	}

	var resp api.ShareCreateResponse
	err := f.callAPI(ctx, &opts, req, &resp, "share_create")
	if err != nil {
		return nil, err
	}

	return &resp, nil
}
