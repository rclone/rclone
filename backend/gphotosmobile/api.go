// api.go implements the Google Photos mobile API client.
//
// # Protocol overview
//
// This backend uses the same private API that the Android Google Photos app
// uses. The API was reverse-engineered by the gpmc project
// (https://github.com/nicholasgasior/gpmc) by intercepting traffic from the
// official Google Photos Android app.
//
// All API calls use raw protobuf over HTTPS (Content-Type: application/x-protobuf).
// There is no published .proto schema — the field numbers were mapped by
// inspecting request/response pairs. The code works at the protobuf wire
// format level using protowire_utils.go, which is functionally equivalent to
// Python's blackboxprotobuf library.
//
// # Authentication
//
// Authentication uses Android device tokens, NOT OAuth2. The flow is:
//
//  1. User extracts auth_data from their Android device (via Google Photos
//     ReVanced + ADB logcat). auth_data is a URL-encoded string containing
//     fields: androidId, Email, Token, client_sig, callerSig, etc.
//  2. On each API call, we POST the auth_data to
//     https://android.googleapis.com/auth to obtain a short-lived bearer
//     token (the "Auth" field in the response, expiring per the "Expiry" field).
//  3. The bearer token is sent as "Authorization: Bearer <token>" on all
//     subsequent API calls.
//
// The bearer token is cached in memory and refreshed automatically when
// the expiry time is reached.
//
// # Endpoints
//
// The API uses several endpoints, all accepting/returning raw protobuf:
//
//   - photosdata-pa.googleapis.com/6439526531001121323/18047484249733410717
//     Library sync: GetLibraryState, GetLibraryPageInit, GetLibraryPage
//   - photosdata-pa.googleapis.com/6439526531001121323/5084965799730810217
//     Hash-based dedup check: FindRemoteMediaByHash
//   - photosdata-pa.googleapis.com/6439526531001121323/16538846908252377752
//     Commit upload: CommitUpload
//   - photosdata-pa.googleapis.com/6439526531001121323/17490284929287180316
//     Trash: MoveToTrash
//   - photosdata-pa.googleapis.com/$rpc/social.frontend.photos.preparedownloaddata.v1.PhotosPrepareDownloadDataService/PhotosPrepareDownload
//     Download URL generation: GetDownloadURL
//   - photos.googleapis.com/data/upload/uploadmedia/interactive
//     Binary upload: GetUploadToken + UploadFile
//
// The numeric path segments (e.g. 6439526531001121323/18047484249733410717)
// are opaque service/method identifiers embedded in the Google Photos APK.

package gphotosmobile

import (
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/fserrors"
	"github.com/rclone/rclone/fs/fshttp"
)

const (
	defaultModel      = "Pixel 9a"
	defaultMake       = "Google"
	defaultAPKVersion = 49029607
	defaultAndroidAPI = 35 // Android 15, shipped with Pixel 9a
)

// MobileAPI is the Google Photos mobile API client
type MobileAPI struct {
	authData   string
	language   string
	userAgent  string
	model      string
	deviceMake string
	apkVersion int64
	androidAPI int64
	client     *http.Client
	authCache  map[string]string
	authMu     sync.Mutex
}

// NewMobileAPI creates a new mobile API client.
// It uses rclone's fshttp transport which respects global flags like
// --proxy, --timeout, --dump, --tpslimit, etc.
func NewMobileAPI(ctx context.Context, authData, model, deviceMake string, apkVersion, androidAPI int64) *MobileAPI {
	language := parseLanguage(authData)
	if language == "" {
		language = "en_US"
	}
	if model == "" {
		model = defaultModel
	}
	if deviceMake == "" {
		deviceMake = defaultMake
	}
	if apkVersion == 0 {
		apkVersion = defaultAPKVersion
	}
	if androidAPI == 0 {
		androidAPI = defaultAndroidAPI
	}

	api := &MobileAPI{
		authData:   authData,
		language:   language,
		model:      model,
		deviceMake: deviceMake,
		apkVersion: apkVersion,
		androidAPI: androidAPI,
		client:     fshttp.NewClient(ctx),
		authCache:  map[string]string{"Expiry": "0", "Auth": ""},
	}

	api.userAgent = fmt.Sprintf(
		"com.google.android.apps.photos/%d (Linux; U; Android 9; %s; %s; Build/PQ2A.190205.001; Cronet/127.0.6510.5) (gzip)",
		apkVersion, language, model,
	)

	return api
}

// bearerToken returns the current auth token, refreshing if needed.
// The token is cached in memory with its server-provided expiry time.
// Thread-safe: uses authMu to serialize concurrent refresh attempts.
func (a *MobileAPI) bearerToken(ctx context.Context) (string, error) {
	a.authMu.Lock()
	defer a.authMu.Unlock()

	expiryStr := a.authCache["Expiry"]
	expiry, _ := strconv.ParseInt(expiryStr, 10, 64)

	if expiry <= time.Now().Unix() {
		resp, err := a.getAuthToken(ctx)
		if err != nil {
			return "", fmt.Errorf("auth token refresh failed: %w", err)
		}
		a.authCache = resp
	}

	token := a.authCache["Auth"]
	if token == "" {
		return "", fmt.Errorf("auth response missing bearer token")
	}
	return token, nil
}

// getAuthToken exchanges the long-lived device token for a short-lived bearer token.
// It POSTs the auth_data fields to https://android.googleapis.com/auth.
// The response is a newline-delimited key=value text body (NOT protobuf):
//
//	Auth=ya29.xxx...
//	Expiry=1706812345
//	...
//
// The "Auth" value becomes the bearer token and "Expiry" is the unix timestamp
// when it expires. Typical lifetime is ~1 hour.
func (a *MobileAPI) getAuthToken(ctx context.Context) (map[string]string, error) {
	authDataValues, err := url.ParseQuery(a.authData)
	if err != nil {
		return nil, fmt.Errorf("failed to parse auth data: %w", err)
	}

	form := url.Values{
		"androidId":                    {authDataValues.Get("androidId")},
		"app":                          {"com.google.android.apps.photos"},
		"client_sig":                   {authDataValues.Get("client_sig")},
		"callerPkg":                    {"com.google.android.apps.photos"},
		"callerSig":                    {authDataValues.Get("callerSig")},
		"device_country":               {authDataValues.Get("device_country")},
		"Email":                        {authDataValues.Get("Email")},
		"google_play_services_version": {authDataValues.Get("google_play_services_version")},
		"lang":                         {authDataValues.Get("lang")},
		"oauth2_foreground":            {authDataValues.Get("oauth2_foreground")},
		"sdk_version":                  {authDataValues.Get("sdk_version")},
		"service":                      {authDataValues.Get("service")},
		"Token":                        {authDataValues.Get("Token")},
	}

	req, err := http.NewRequestWithContext(ctx, "POST", "https://android.googleapis.com/auth",
		strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Accept-Encoding", "gzip")
	req.Header.Set("app", "com.google.android.apps.photos")
	req.Header.Set("Connection", "Keep-Alive")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("device", authDataValues.Get("androidId"))
	req.Header.Set("User-Agent", "GoogleAuth/1.4 (Pixel XL PQ2A.190205.001); gzip")

	resp, err := a.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("auth request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("auth failed with status %d: %s", resp.StatusCode, string(body))
	}

	body, err := readResponseBody(resp)
	if err != nil {
		return nil, err
	}

	result := make(map[string]string)
	for _, line := range strings.Split(string(body), "\n") {
		line = strings.TrimSpace(line)
		if parts := strings.SplitN(line, "=", 2); len(parts) == 2 {
			result[parts[0]] = parts[1]
		}
	}

	if result["Auth"] == "" {
		return nil, fmt.Errorf("auth response missing Auth token")
	}
	return result, nil
}

// commonHeaders returns headers used for most API calls
func (a *MobileAPI) commonHeaders(ctx context.Context) (map[string]string, error) {
	token, err := a.bearerToken(ctx)
	if err != nil {
		return nil, err
	}
	return map[string]string{
		"Accept-Encoding":          "gzip",
		"Accept-Language":          a.language,
		"Content-Type":             "application/x-protobuf",
		"User-Agent":               a.userAgent,
		"Authorization":            "Bearer " + token,
		"x-goog-ext-173412678-bin": "CgcIAhClARgC",
		"x-goog-ext-174067345-bin": "CgIIAg==",
	}, nil
}

// retryableStatusCodes lists HTTP status codes that should be retried.
var retryableStatusCodes = []int{
	429, // Too Many Requests
	500, // Internal Server Error
	502, // Bad Gateway
	503, // Service Unavailable
	504, // Gateway Timeout
}

// apiError represents an HTTP error response from the API.
// It carries the status code so the retry logic can distinguish
// retryable server errors (5xx, 429) from permanent client errors (4xx).
type apiError struct {
	StatusCode int
	Body       string
}

func (e *apiError) Error() string {
	return fmt.Sprintf("API error %d: %s", e.StatusCode, e.Body)
}

// shouldRetryAPI returns true if the error deserves a retry.
// It retries on:
//   - network-level errors (timeouts, connection resets, etc.) via fserrors.ShouldRetry
//   - HTTP 429 and 5xx via the status code in apiError
//
// It does NOT retry on:
//   - context cancellation/deadline exceeded
//   - HTTP 4xx (except 429) — these are permanent client errors
func shouldRetryAPI(ctx context.Context, err error) bool {
	if err == nil {
		return false
	}
	// Never retry if the context is done
	if fserrors.ContextError(ctx, &err) {
		return false
	}
	// Check for retryable HTTP status codes
	var ae *apiError
	if errors.As(err, &ae) {
		for _, code := range retryableStatusCodes {
			if ae.StatusCode == code {
				return true
			}
		}
		return false // non-retryable HTTP error (400, 401, 403, 404, etc.)
	}
	// For network errors, use rclone's standard retry logic
	return fserrors.ShouldRetry(err)
}

// doProtoRequest makes a protobuf API request with up to 3 attempts.
// It only retries on retryable errors (network errors, 429, 5xx) using
// exponential backoff with jitter. Non-retryable errors (400, 403, 404)
// are returned immediately.
func (a *MobileAPI) doProtoRequest(ctx context.Context, urlStr string, body []byte) ([]byte, error) {
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		if attempt > 0 {
			// Exponential backoff: 1s, 2s (+ up to 500ms jitter)
			baseDelay := time.Duration(1<<(attempt-1)) * time.Second
			jitter := time.Duration(time.Now().UnixNano()%500) * time.Millisecond
			delay := baseDelay + jitter
			fs.Debugf(nil, "Retrying request to %s (attempt %d/%d, backoff %v)", urlStr, attempt+1, 3, delay)
			timer := time.NewTimer(delay)
			select {
			case <-ctx.Done():
				timer.Stop()
				return nil, ctx.Err()
			case <-timer.C:
			}
		}

		result, err := a.doProtoRequestOnce(ctx, urlStr, body)
		if err == nil {
			return result, nil
		}
		lastErr = err
		if !shouldRetryAPI(ctx, err) {
			return nil, err // non-retryable, return immediately
		}
		fs.Debugf(nil, "Request to %s failed (retryable): %v", urlStr, err)
	}
	return nil, lastErr
}

func (a *MobileAPI) doProtoRequestOnce(ctx context.Context, urlStr string, body []byte) ([]byte, error) {
	headers, err := a.commonHeaders(ctx)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", urlStr, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}

	for k, v := range headers {
		req.Header.Set(k, v)
	}

	resp, err := a.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, &apiError{StatusCode: resp.StatusCode, Body: string(respBody)}
	}

	return readResponseBody(resp)
}

// readResponseBody handles gzip decoding
func readResponseBody(resp *http.Response) ([]byte, error) {
	var reader io.Reader = resp.Body

	if resp.Header.Get("Content-Encoding") == "gzip" {
		gzReader, err := gzip.NewReader(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("gzip decode failed: %w", err)
		}
		defer func() { _ = gzReader.Close() }()
		reader = gzReader
	}

	return io.ReadAll(reader)
}

// --- API Methods ---

// GetLibraryState gets the library state (incremental or initial)
func (a *MobileAPI) GetLibraryState(ctx context.Context, stateToken string) ([]byte, error) {
	body := buildGetLibStateRequest(stateToken)
	return a.doProtoRequest(ctx,
		"https://photosdata-pa.googleapis.com/6439526531001121323/18047484249733410717",
		body,
	)
}

// GetLibraryPageInit gets a library page during init
func (a *MobileAPI) GetLibraryPageInit(ctx context.Context, pageToken string) ([]byte, error) {
	body := buildGetLibPageInitRequest(pageToken)
	return a.doProtoRequest(ctx,
		"https://photosdata-pa.googleapis.com/6439526531001121323/18047484249733410717",
		body,
	)
}

// GetLibraryPage gets a library page (delta update)
func (a *MobileAPI) GetLibraryPage(ctx context.Context, pageToken, stateToken string) ([]byte, error) {
	body := buildGetLibPageRequest(pageToken, stateToken)
	return a.doProtoRequest(ctx,
		"https://photosdata-pa.googleapis.com/6439526531001121323/18047484249733410717",
		body,
	)
}

// FindRemoteMediaByHash checks if a file with the given SHA1 hash already
// exists in the user's library (server-side deduplication). If found, it
// returns the existing media_key; if not found, it returns "".
// This is called before uploading to avoid re-uploading identical files.
// The response media_key is at field path 1.2.2.1.
func (a *MobileAPI) FindRemoteMediaByHash(ctx context.Context, sha1Hash []byte) (string, error) {
	body := buildHashCheckRequest(sha1Hash)

	respBytes, err := a.doProtoRequest(ctx,
		"https://photosdata-pa.googleapis.com/6439526531001121323/5084965799730810217",
		body,
	)
	if err != nil {
		return "", err
	}

	// Parse response
	root, err := DecodeRaw(respBytes)
	if err != nil {
		return "", err
	}

	// Navigate: field1 -> field2 -> field2 -> field1
	f1, err := root.GetMessage(1)
	if err != nil {
		return "", nil // no match
	}
	f12, err := f1.GetMessage(2)
	if err != nil {
		return "", nil
	}
	f122, err := f12.GetMessage(2)
	if err != nil {
		return "", nil
	}
	return f122.GetString(1), nil
}

// GetUploadToken obtains a resumable upload session ID from Google.
// The flow is a two-phase upload protocol (like GCS resumable uploads):
//
//  1. POST a small protobuf body with file metadata + SHA1 hash header.
//     Google returns the session ID in the X-GUploader-UploadID response header.
//  2. PUT the actual file bytes to the same URL with ?upload_id=<token>.
//
// The protobuf body specifies upload parameters:
//
//	{1: 2, 2: 2, 3: 1, 4: 3, 7: fileSize}
//
// The X-Goog-Hash header carries the base64-encoded SHA1 for server-side
// integrity verification. X-Upload-Content-Length tells the server the total size.
func (a *MobileAPI) GetUploadToken(ctx context.Context, sha1B64 string, fileSize int64) (string, error) {
	// Build protobuf: {1: 2, 2: 2, 3: 1, 4: 3, 7: fileSize}
	b := NewProtoBuilder()
	b.AddVarint(1, 2)
	b.AddVarint(2, 2)
	b.AddVarint(3, 1)
	b.AddVarint(4, 3)
	b.AddVarint(7, uint64(fileSize))

	token, err := a.bearerToken(ctx)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, "POST",
		"https://photos.googleapis.com/data/upload/uploadmedia/interactive",
		bytes.NewReader(b.Bytes()))
	if err != nil {
		return "", err
	}

	req.Header.Set("Accept-Encoding", "gzip")
	req.Header.Set("Accept-Language", a.language)
	req.Header.Set("Content-Type", "application/x-protobuf")
	req.Header.Set("User-Agent", a.userAgent)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("X-Goog-Hash", "sha1="+sha1B64)
	req.Header.Set("X-Upload-Content-Length", strconv.FormatInt(fileSize, 10))

	resp, err := a.client.Do(req)
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("upload token error %d: %s", resp.StatusCode, string(body))
	}

	uploadToken := resp.Header.Get("X-GUploader-UploadID")
	if uploadToken == "" {
		return "", fmt.Errorf("missing X-GUploader-UploadID header")
	}
	return uploadToken, nil
}

// UploadFile uploads file data using the upload token, returns raw protobuf response
func (a *MobileAPI) UploadFile(ctx context.Context, body io.Reader, size int64, uploadToken string) ([]byte, error) {
	token, err := a.bearerToken(ctx)
	if err != nil {
		return nil, err
	}

	uploadURL := "https://photos.googleapis.com/data/upload/uploadmedia/interactive?upload_id=" + uploadToken

	req, err := http.NewRequestWithContext(ctx, "PUT", uploadURL, body)
	if err != nil {
		return nil, err
	}
	req.ContentLength = size

	req.Header.Set("Accept-Encoding", "gzip")
	req.Header.Set("Accept-Language", a.language)
	req.Header.Set("User-Agent", a.userAgent)
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := a.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("upload error %d: %s", resp.StatusCode, string(body))
	}

	return readResponseBody(resp)
}

// CommitUpload finalizes an upload by committing it to the user's library.
// After UploadFile succeeds, the raw protobuf response bytes are passed here
// as the first field of the commit request (they contain an opaque upload
// token that Google uses to associate the committed item with the uploaded bytes).
// The response contains the new item's media_key at field path 1.3.1.
func (a *MobileAPI) CommitUpload(ctx context.Context, uploadResponse []byte, fileName string, sha1Hash []byte) (string, error) {
	body := buildCommitUploadRequest(uploadResponse, fileName, sha1Hash, a.model, a.deviceMake, a.androidAPI)

	respBytes, err := a.doProtoRequest(ctx,
		"https://photosdata-pa.googleapis.com/6439526531001121323/16538846908252377752",
		body,
	)
	if err != nil {
		return "", err
	}

	// Parse response: field1 -> field3 -> field1 = media_key
	root, err := DecodeRaw(respBytes)
	if err != nil {
		return "", fmt.Errorf("failed to decode commit response: %w", err)
	}

	f1, err := root.GetMessage(1)
	if err != nil {
		return "", fmt.Errorf("upload rejected: no field1")
	}
	f13, err := f1.GetMessage(3)
	if err != nil {
		return "", fmt.Errorf("upload rejected: no field3")
	}
	mediaKey := f13.GetString(1)
	if mediaKey == "" {
		return "", fmt.Errorf("upload rejected: no media key")
	}
	return mediaKey, nil
}

// MoveToTrash moves items to trash by their dedup keys.
// The dedup_key is a URL-safe base64-encoded SHA1 hash that uniquely
// identifies a media item (distinct from media_key, which is an opaque
// server-assigned string). Items remain in trash for 60 days before
// permanent deletion (matching the Google Photos UI behavior).
func (a *MobileAPI) MoveToTrash(ctx context.Context, dedupKeys []string) error {
	body := buildMoveToTrashRequest(dedupKeys, a.apkVersion, a.androidAPI)
	_, err := a.doProtoRequest(ctx,
		"https://photosdata-pa.googleapis.com/6439526531001121323/17490284929287180316",
		body,
	)
	return err
}

// GetDownloadURL obtains a time-limited download URL for a media item.
// The URL is typically valid for a few hours. The response structure differs
// by media type:
//
//	Photos: URL at field path 1.5.2.6 (original) or 1.5.2.5 (edited)
//	Videos: URL at field path 1.5.3.5
//
// We prefer the original URL (field 6) over the edited one (field 5).
// If field 2 (photos) is absent, we fall back to field 3 (videos).
func (a *MobileAPI) GetDownloadURL(ctx context.Context, mediaKey string) (string, error) {
	body := buildGetDownloadURLsRequest(mediaKey)

	respBytes, err := a.doProtoRequest(ctx,
		"https://photosdata-pa.googleapis.com/$rpc/social.frontend.photos.preparedownloaddata.v1.PhotosPrepareDownloadDataService/PhotosPrepareDownload",
		body,
	)
	if err != nil {
		return "", err
	}

	// Parse response to extract URL
	// output_dict["1"]["5"]["2"]["5"] - edited URL
	// output_dict["1"]["5"]["2"]["6"] - original URL
	root, err := DecodeRaw(respBytes)
	if err != nil {
		return "", err
	}

	f1, err := root.GetMessage(1)
	if err != nil {
		return "", fmt.Errorf("no download info in response")
	}

	f15, err := f1.GetMessage(5)
	if err != nil {
		return "", fmt.Errorf("no download URLs in response")
	}

	// The URL data location depends on media type:
	// field 1.5.1 = media type indicator (1=photo, 2=video)
	// Photos: URLs at 1.5.2.6 (original) / 1.5.2.5 (edited)
	// Videos: URLs at 1.5.3.5 (download URL)
	var urlContainer ProtoMap

	// Try field 2 first (photos), then field 3 (videos)
	urlContainer, err = f15.GetMessage(2)
	if err != nil {
		urlContainer, err = f15.GetMessage(3)
		if err != nil {
			return "", fmt.Errorf("no download URL data in response")
		}
	}

	// Try original URL first (field 6), then edited/video (field 5)
	downloadURL := urlContainer.GetString(6)
	if downloadURL == "" {
		downloadURL = urlContainer.GetString(5)
	}
	if downloadURL == "" {
		return "", fmt.Errorf("no download URL found in response")
	}

	return downloadURL, nil
}

// DownloadFile downloads a file from the given URL
func (a *MobileAPI) DownloadFile(ctx context.Context, downloadURL string, options ...fs.OpenOption) (io.ReadCloser, error) {
	token, err := a.bearerToken(ctx)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, "GET", downloadURL, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("User-Agent", a.userAgent)

	// Apply range headers if present
	for _, option := range options {
		switch o := option.(type) {
		case *fs.RangeOption:
			if o.Start >= 0 && o.End >= 0 {
				req.Header.Set("Range", fmt.Sprintf("bytes=%d-%d", o.Start, o.End))
			} else if o.Start >= 0 {
				req.Header.Set("Range", fmt.Sprintf("bytes=%d-", o.Start))
			} else if o.End >= 0 {
				req.Header.Set("Range", fmt.Sprintf("bytes=-%d", o.End))
			}
		case *fs.SeekOption:
			req.Header.Set("Range", fmt.Sprintf("bytes=%d-", o.Offset))
		}
	}

	resp, err := a.client.Do(req)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		return nil, fmt.Errorf("download error %d: %s", resp.StatusCode, string(body))
	}

	return resp.Body, nil
}
