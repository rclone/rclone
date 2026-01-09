package mediavfs

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/rclone/rclone/fs"
)

// ErrMediaNotFound is returned when a media item doesn't exist (404)
var ErrMediaNotFound = errors.New("media item not found")

const (
	defaultTimeout = 60 * time.Second
	maxRetries     = 10
)

// GPhotoAPI handles Google Photos API interactions
type GPhotoAPI struct {
	token          string
	tokenExpiry    int64 // Token expiry in milliseconds
	httpClient     *http.Client
	tokenServerURL string
	nativeAuth     *GooglePhotosAuth // Native auth client (optional)
	userAgent      string
	user           string
	timeout        time.Duration
}

// NewGPhotoAPI creates a new Google Photos API client
func NewGPhotoAPI(user string, tokenServerURL string, httpClient *http.Client) *GPhotoAPI {
	return &GPhotoAPI{
		user:           user,
		tokenServerURL: tokenServerURL,
		httpClient:     httpClient,
		userAgent:      "com.google.android.apps.photos/49029607 (Linux; U; Android 9; en_US; Pixel XL; Build/PQ2A.190205.001; Cronet/127.0.6510.5) (gzip)",
		timeout:        defaultTimeout,
	}
}

// NewGPhotoAPIWithNativeAuth creates a new Google Photos API client with native authentication
func NewGPhotoAPIWithNativeAuth(user, tokenServerURL, masterToken, privateKeyS, androidID string, httpClient *http.Client) (*GPhotoAPI, error) {
	api := &GPhotoAPI{
		user:           user,
		tokenServerURL: tokenServerURL,
		httpClient:     httpClient,
		userAgent:      "com.google.android.apps.photos/49029607 (Linux; U; Android 9; en_US; Pixel XL; Build/PQ2A.190205.001; Cronet/127.0.6510.5) (gzip)",
		timeout:        defaultTimeout,
	}

	// If master token is provided, create native auth client
	if masterToken != "" {
		email := user
		if !strings.Contains(email, "@") {
			email = user + "@gmail.com"
		}
		nativeAuth, err := NewGooglePhotosAuth(email, masterToken, androidID, privateKeyS, httpClient)
		if err != nil {
			return nil, fmt.Errorf("failed to create native auth: %w", err)
		}
		api.nativeAuth = nativeAuth
		fs.Infof(nil, "gphoto: using native token generation for %s", user)
	}

	return api, nil
}

// GetAuthToken fetches or refreshes the authentication token
func (api *GPhotoAPI) GetAuthToken(ctx context.Context, force bool) error {
	// Use native auth if available
	if api.nativeAuth != nil {
		return api.getNativeAuthToken(ctx, force)
	}

	// Fall back to token server
	return api.getTokenServerToken(ctx, force)
}

// getNativeAuthToken gets token using native authentication
func (api *GPhotoAPI) getNativeAuthToken(ctx context.Context, force bool) error {
	// Check if current token is still valid (with 60s buffer)
	if !force && api.token != "" && api.tokenExpiry > 0 {
		nowMs := time.Now().UnixMilli()
		if nowMs < (api.tokenExpiry - 60000) {
			return nil // Token still valid
		}
	}

	result, err := api.nativeAuth.GetToken(ctx)
	if err != nil {
		return fmt.Errorf("native token fetch failed: %w", err)
	}

	if result.Error != "" {
		return fmt.Errorf("native token error: %s", result.Error)
	}

	api.token = result.Token
	api.tokenExpiry = result.Expiry
	fs.Debugf(nil, "gphoto: obtained native token, expires at %d", api.tokenExpiry)
	return nil
}

// getTokenServerToken gets token from external token server
func (api *GPhotoAPI) getTokenServerToken(ctx context.Context, force bool) error {
	if api.tokenServerURL == "" {
		return fmt.Errorf("no token server URL configured and no native auth available")
	}

	url := fmt.Sprintf("%s/token/%s", api.tokenServerURL, api.user)
	if force {
		url += "?force=true"
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create token request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer @localhost@")

	resp, err := api.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to fetch token: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("token request failed with status %d", resp.StatusCode)
	}

	var tokenResp struct {
		Token string `json:"token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return fmt.Errorf("failed to decode token response: %w", err)
	}

	api.token = tokenResp.Token
	return nil
}

// request makes an authenticated HTTP request with retry logic
func (api *GPhotoAPI) request(ctx context.Context, method, url string, headers map[string]string, body io.Reader) (*http.Response, error) {
	var resp *http.Response
	var err error
	var bodyBytes []byte
	authRetries := 0    // Track auth failures to escalate to force refresh
	lastStatusCode := 0 // Track last status code for error reporting

	// If body is provided, read it into memory so we can retry
	if body != nil {
		bodyBytes, err = io.ReadAll(body)
		if err != nil {
			return nil, fmt.Errorf("failed to read request body: %w", err)
		}
	}

	for retry := 0; retry < 5; retry++ {
		// Ensure we have a valid token
		if api.token == "" || (api.nativeAuth != nil && api.tokenExpiry > 0 && time.Now().UnixMilli() >= api.tokenExpiry-60000) {
			if err := api.GetAuthToken(ctx, false); err != nil {
				return nil, err
			}
		}

		// Create new body reader for each retry
		var reqBody io.Reader
		if bodyBytes != nil {
			reqBody = bytes.NewReader(bodyBytes)
		}

		req, err := http.NewRequestWithContext(ctx, method, url, reqBody)
		if err != nil {
			return nil, fmt.Errorf("failed to create request: %w", err)
		}

		// Set default headers
		req.Header.Set("User-Agent", api.userAgent)
		req.Header.Set("Accept-Encoding", "gzip")
		req.Header.Set("Accept-Language", "en_US")

		// Set authorization if token exists and not requesting token service
		if api.token != "" {
			req.Header.Set("Authorization", "Bearer "+api.token)
		}

		// Set custom headers
		for k, v := range headers {
			req.Header.Set(k, v)
		}

		resp, err = api.httpClient.Do(req)
		if err != nil {
			return nil, err
		}

		// Handle status codes
		switch resp.StatusCode {
		case http.StatusOK, http.StatusPartialContent:
			return resp, nil

		case http.StatusUnauthorized, http.StatusForbidden:
			lastStatusCode = resp.StatusCode
			resp.Body.Close()
			authRetries++
			forceRefresh := authRetries > 1
			fs.Debugf(nil, "gphoto: auth error (%d), retry %d/5, forceRefresh=%v", lastStatusCode, retry+1, forceRefresh)
			if err := api.GetAuthToken(ctx, forceRefresh); err != nil {
				return nil, fmt.Errorf("auth refresh failed after %d: %w", lastStatusCode, err)
			}
			continue

		case http.StatusTooManyRequests: // 429
			lastStatusCode = resp.StatusCode
			// Check Retry-After header
			backoff := time.Duration(1<<uint(retry)) * time.Second
			if retryAfter := resp.Header.Get("Retry-After"); retryAfter != "" {
				if seconds, err := strconv.Atoi(retryAfter); err == nil && seconds > 0 {
					backoff = time.Duration(seconds) * time.Second
				}
			}
			// Minimum 5 second backoff for rate limits, max 60 seconds
			if backoff < 5*time.Second {
				backoff = 5 * time.Second
			}
			if backoff > 60*time.Second {
				backoff = 60 * time.Second
			}
			resp.Body.Close()
			fs.Infof(nil, "gphoto: rate limited (429), retry %d/5, waiting %s", retry+1, backoff)
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(backoff):
			}
			continue

		case http.StatusInternalServerError, http.StatusServiceUnavailable:
			lastStatusCode = resp.StatusCode
			// Log error body for debugging
			body, _ := readResponseBody(resp)
			resp.Body.Close()
			fs.Errorf(nil, "gphoto: server error (%d), retry %d/5, body: %s", resp.StatusCode, retry+1, string(body))
			backoff := time.Duration(1<<uint(retry)) * time.Second
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(backoff):
			}
			continue

		case http.StatusNotFound: // 404
			return resp, nil

		default:
			resp.Body.Close()
			return nil, fmt.Errorf("HTTP error: status %d", resp.StatusCode)
		}
	}

	return resp, fmt.Errorf("max retries exceeded (last status: %d)", lastStatusCode)
}

// readResponseBody reads the response body, decompressing if gzip-encoded
func readResponseBody(resp *http.Response) ([]byte, error) {
	var reader io.Reader = resp.Body

	// Check if response is gzip-compressed
	if resp.Header.Get("Content-Encoding") == "gzip" {
		gzReader, err := gzip.NewReader(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("failed to create gzip reader: %w", err)
		}
		defer gzReader.Close()
		reader = gzReader
	}

	return io.ReadAll(reader)
}

// GetUploadToken obtains an upload token from Google Photos
func (api *GPhotoAPI) GetUploadToken(ctx context.Context, sha1HashB64 string, fileSize int64) (string, error) {
	// Encode protobuf message matching Python implementation
	encoder := NewProtoEncoder()
	encoder.EncodeInt32(1, 2)
	encoder.EncodeInt32(2, 2)
	encoder.EncodeInt32(3, 1)
	encoder.EncodeInt32(4, 3)
	encoder.EncodeInt64(7, fileSize)

	serializedData := encoder.Bytes()

	headers := map[string]string{
		"Content-Type":            "application/x-protobuf",
		"X-Goog-Hash":             fmt.Sprintf("sha1=%s", sha1HashB64),
		"X-Upload-Content-Length": fmt.Sprintf("%d", fileSize),
	}

	resp, err := api.request(ctx, "POST",
		"https://photos.googleapis.com/data/upload/uploadmedia/interactive",
		headers, bytes.NewReader(serializedData))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	uploadToken := resp.Header.Get("X-GUploader-UploadID")
	if uploadToken == "" {
		return "", fmt.Errorf("no upload token in response")
	}

	return uploadToken, nil
}

// FindRemoteMediaByHash checks if a file with the given SHA1 hash already exists
func (api *GPhotoAPI) FindRemoteMediaByHash(ctx context.Context, sha1Hash []byte) (string, error) {
	// Encode nested protobuf message matching Python implementation
	// Field 1 -> Field 1 -> Field 1: raw SHA1 hash bytes (NOT base64)
	innermost := NewProtoEncoder()
	innermost.EncodeBytes(1, sha1Hash)

	middle := NewProtoEncoder()
	middle.EncodeMessage(1, innermost.Bytes())
	middle.EncodeMessage(2, []byte{}) // Empty message

	encoder := NewProtoEncoder()
	encoder.EncodeMessage(1, middle.Bytes())

	serializedData := encoder.Bytes()

	headers := map[string]string{
		"Content-Type": "application/x-protobuf",
	}

	resp, err := api.request(ctx, "POST",
		"https://photosdata-pa.googleapis.com/6439526531001121323/5084965799730810217",
		headers, bytes.NewReader(serializedData))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	// Parse protobuf response
	respBody, err := readResponseBody(resp)
	if err != nil {
		return "", err
	}

	result, err := DecodeToMap(respBody)
	if err != nil {
		return "", err
	}

	// Extract media key if found (field 1 -> field 2 -> field 2 -> field 1)
	// Python: decoded_message["1"].get("2", {}).get("2", {}).get("1", None)
	if mediaData, ok := result["1"].(map[string]interface{}); ok {
		if field2, ok := mediaData["2"].(map[string]interface{}); ok {
			if field2_2, ok := field2["2"].(map[string]interface{}); ok {
				if mediaKey, ok := field2_2["1"].(string); ok {
					return mediaKey, nil
				}
			}
		}
	}

	return "", nil // File not found
}

// UploadFile uploads file content to Google Photos and returns the decoded response
func (api *GPhotoAPI) UploadFile(ctx context.Context, uploadToken string, content io.Reader, fileSize int64) ([]byte, error) {
	url := fmt.Sprintf("https://photos.googleapis.com/data/upload/uploadmedia/interactive?upload_id=%s", uploadToken)

	headers := map[string]string{
		"Content-Type":   "application/octet-stream",
		"Content-Length": fmt.Sprintf("%d", fileSize),
	}

	fs.Debugf(nil, "gphoto: uploading %d bytes...", fileSize)

	resp, err := api.request(ctx, "PUT", url, headers, content)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	// Read the response body - this is needed for commit
	respBody, err := readResponseBody(resp)
	if err != nil {
		return nil, err
	}

	fs.Debugf(nil, "gphoto: upload complete")
	return respBody, nil
}

// CommitUpload commits the uploaded file to Google Photos
func (api *GPhotoAPI) CommitUpload(ctx context.Context, uploadResponse []byte, fileName string, sha1Hash []byte, fileSize int64, uploadTimestamp int64, model, quality string) (string, error) {
	qualityMap := map[string]int32{
		"saver":    1,
		"original": 3,
	}

	if uploadTimestamp == 0 {
		uploadTimestamp = time.Now().Unix()
	}

	// Build nested protobuf structure matching Python implementation
	// Field 1 -> Field 4: timestamp message
	timestampMsg := NewProtoEncoder()
	timestampMsg.EncodeInt64(1, uploadTimestamp)
	timestampMsg.EncodeInt32(2, 46000000)

	// Field 1: main content message
	field1 := NewProtoEncoder()
	// Field 1.1: upload response (raw protobuf bytes from upload)
	field1.EncodeMessage(1, uploadResponse)
	// Field 1.2: file name
	field1.EncodeString(2, fileName)
	// Field 1.3: SHA1 hash as raw bytes (NOT base64)
	field1.EncodeBytes(3, sha1Hash)
	// Field 1.4: timestamp
	field1.EncodeMessage(4, timestampMsg.Bytes())
	// Field 1.7: quality
	field1.EncodeInt32(7, qualityMap[quality])
	// Field 1.10: unknown (always 1)
	field1.EncodeInt32(10, 1)
	// Field 1.17: unknown (always 0)
	field1.EncodeInt32(17, 0)

	// Field 2: device info
	field2 := NewProtoEncoder()
	field2.EncodeString(3, model)
	field2.EncodeString(4, "Google")
	field2.EncodeInt32(5, 28) // Android API version

	// Main message
	encoder := NewProtoEncoder()
	encoder.EncodeMessage(1, field1.Bytes())
	encoder.EncodeMessage(2, field2.Bytes())
	encoder.EncodeBytes(3, []byte{1, 3})

	serializedData := encoder.Bytes()

	headers := map[string]string{
		"Content-Type":           "application/x-protobuf",
		"x-goog-ext-173412678-bin": "CgcIAhClARgC",
		"x-goog-ext-174067345-bin": "CgIIAg==",
	}

	resp, err := api.request(ctx, "POST",
		"https://photosdata-pa.googleapis.com/6439526531001121323/16538846908252377752",
		headers, bytes.NewReader(serializedData))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	// Parse protobuf response (may be gzip compressed)
	respBody, err := readResponseBody(resp)
	if err != nil {
		return "", err
	}

	result, err := DecodeToMap(respBody)
	if err != nil {
		return "", fmt.Errorf("failed to decode response: %w", err)
	}

	// Log response structure for debugging
	fs.Debugf(nil, "gphoto: CommitUpload response keys: %v", getMapKeys(result))

	// Extract media key (field 1 -> field 3 -> field 1)
	if mediaData, ok := result["1"].(map[string]interface{}); ok {
		fs.Debugf(nil, "gphoto: CommitUpload field 1 keys: %v", getMapKeys(mediaData))
		if keyData, ok := mediaData["3"].(map[string]interface{}); ok {
			fs.Debugf(nil, "gphoto: CommitUpload field 1.3 keys: %v", getMapKeys(keyData))
			if mediaKey, ok := keyData["1"].(string); ok {
				return mediaKey, nil
			}
		}
		// Try alternate path: field 1 -> field 1 (for duplicates)
		if mediaKey, ok := mediaData["1"].(string); ok {
			fs.Debugf(nil, "gphoto: Found media key at alternate location (field 1.1)")
			return mediaKey, nil
		}
	}

	// Try to find any string that looks like a media key in the response
	if mediaKey := findMediaKeyInResponse(result); mediaKey != "" {
		fs.Debugf(nil, "gphoto: Found media key via deep search: %s", mediaKey)
		return mediaKey, nil
	}

	// Log the full response for debugging
	fs.Errorf(nil, "gphoto: CommitUpload response structure: %+v", result)

	return "", fmt.Errorf("media key not found in response")
}

// findMediaKeyInResponse recursively searches for a media key in the response
func findMediaKeyInResponse(data interface{}) string {
	switch v := data.(type) {
	case map[string]interface{}:
		for _, val := range v {
			if result := findMediaKeyInResponse(val); result != "" {
				return result
			}
		}
	case string:
		// Media keys are typically long alphanumeric strings
		if len(v) > 20 && len(v) < 100 {
			return v
		}
	}
	return ""
}

// MoveToTrash moves files to trash (supports batch deletion)
func (api *GPhotoAPI) MoveToTrash(ctx context.Context, dedupKeys []string) error {
	fs.Debugf(nil, "gphoto: MoveToTrash processing %d files", len(dedupKeys))

	// Build nested protobuf structure for MoveToTrash
	// Matching exact structure from captured Google Photos app request:
	// {
	//   "2": 1,
	//   "3": ["dedup_key1", "dedup_key2", ...],  // repeated field
	//   "4": 1,
	//   "8": {"4": {"2": "", "3": {"1": ""}, "4": "", "5": {"1": ""}}},
	//   "9": {"1": 5, "2": {"1": 51079550, "2": "33"}}
	// }

	// Field 8.4.3: message with field 1 = empty string
	field8_4_3 := NewProtoEncoder()
	field8_4_3.EncodeString(1, "")

	// Field 8.4.5: message with field 1 = empty string
	field8_4_5 := NewProtoEncoder()
	field8_4_5.EncodeString(1, "")

	// Field 8.4: contains empty strings and nested messages
	field8_4 := NewProtoEncoder()
	field8_4.EncodeString(2, "")
	field8_4.EncodeMessage(3, field8_4_3.Bytes())
	field8_4.EncodeString(4, "")
	field8_4.EncodeMessage(5, field8_4_5.Bytes())

	field8 := NewProtoEncoder()
	field8.EncodeMessage(4, field8_4.Bytes())

	// Field 9.2: version info (INT for field 1, STRING for field 2)
	field9_2 := NewProtoEncoder()
	field9_2.EncodeInt32(1, 51079550) // App version as INT
	field9_2.EncodeString(2, "33")    // Android API version

	field9 := NewProtoEncoder()
	field9.EncodeInt32(1, 5)
	field9.EncodeMessage(2, field9_2.Bytes())

	// Main message
	encoder := NewProtoEncoder()
	encoder.EncodeInt32(2, 1)
	// Field 3 is repeated - encode each dedup_key with the same field number
	// Database stores base64-encoded dedup_keys, need to decode before sending
	for _, dedupKey := range dedupKeys {
		// Decode from URL-safe base64 (database format) to get raw dedup_key
		decoded, err := base64.URLEncoding.DecodeString(dedupKey)
		if err != nil {
			// Try with padding if needed
			decoded, err = base64.RawURLEncoding.DecodeString(dedupKey)
			if err != nil {
				fs.Errorf(nil, "gphoto: failed to decode dedup_key %s: %v", dedupKey, err)
				continue
			}
		}
		encoder.EncodeString(3, string(decoded))
	}
	encoder.EncodeInt32(4, 1)
	encoder.EncodeMessage(8, field8.Bytes())
	encoder.EncodeMessage(9, field9.Bytes())

	headers := map[string]string{
		"Content-Type": "application/x-protobuf",
	}

	resp, err := api.request(ctx, "POST",
		"https://photosdata-pa.googleapis.com/6439526531001121323/17490284929287180316",
		headers, bytes.NewReader(encoder.Bytes()))
	if err != nil {
		return fmt.Errorf("failed to move files to trash: %w", err)
	}
	resp.Body.Close()

	fs.Debugf(nil, "gphoto: MoveToTrash completed, %d files moved to trash", len(dedupKeys))
	return nil
}

// GetLibraryState gets the current library state from Google Photos
func (api *GPhotoAPI) GetLibraryState(ctx context.Context, stateToken, pageToken string) ([]byte, error) {
	// Build protobuf message using official Google protobuf library
	protoBody := buildGetLibraryStateMessage(stateToken, pageToken)

	// Encode using Google's official protobuf wire format
	serializedData, err := EncodeDynamicMessage(protoBody)
	if err != nil {
		return nil, fmt.Errorf("failed to encode library state message: %w", err)
	}

	headers := map[string]string{
		"Content-Type":             "application/x-protobuf",
		"x-goog-ext-173412678-bin": "CgcIAhClARgC",
		"x-goog-ext-174067345-bin": "CgIIAg==",
	}

	resp, err := api.request(ctx, "POST",
		"https://photosdata-pa.googleapis.com/6439526531001121323/18047484249733410717",
		headers, bytes.NewReader(serializedData))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	// Decompress if gzip compressed
	var reader io.Reader = resp.Body
	if resp.Header.Get("Content-Encoding") == "gzip" {
		gzipReader, err := gzip.NewReader(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("failed to create gzip reader: %w", err)
		}
		defer gzipReader.Close()
		reader = gzipReader
	}

	return io.ReadAll(reader)
}

// GetLibraryPage gets a page of library results (for incremental sync)
func (api *GPhotoAPI) GetLibraryPage(ctx context.Context, pageToken, stateToken string) ([]byte, error) {
	return api.GetLibraryState(ctx, stateToken, pageToken)
}

// GetLibraryPageInit gets a page of library results during initial sync
// This uses a different message template that returns batches of items
func (api *GPhotoAPI) GetLibraryPageInit(ctx context.Context, pageToken string) ([]byte, error) {
	// Build protobuf message using init template
	protoBody := buildGetLibraryPageInitMessage(pageToken)

	// Encode using Google's official protobuf wire format
	serializedData, err := EncodeDynamicMessage(protoBody)
	if err != nil {
		return nil, fmt.Errorf("failed to encode library page init message: %w", err)
	}

	headers := map[string]string{
		"Content-Type":             "application/x-protobuf",
		"x-goog-ext-173412678-bin": "CgcIAhClARgC",
		"x-goog-ext-174067345-bin": "CgIIAg==",
	}

	resp, err := api.request(ctx, "POST",
		"https://photosdata-pa.googleapis.com/6439526531001121323/18047484249733410717",
		headers, bytes.NewReader(serializedData))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	// Decompress if gzip compressed
	var reader io.Reader = resp.Body
	if resp.Header.Get("Content-Encoding") == "gzip" {
		gzipReader, err := gzip.NewReader(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("failed to create gzip reader: %w", err)
		}
		defer gzipReader.Close()
		reader = gzipReader
	}

	return io.ReadAll(reader)
}

// GetDownloadURL gets the download URL for a media item
// Based on Python implementation: api.get_download_url()
func (api *GPhotoAPI) GetDownloadURL(ctx context.Context, mediaKey string) (string, error) {
	// Build protobuf message matching Python implementation
	// Field 1 -> Field 1 -> Field 1: media_key
	field1_1 := NewProtoEncoder()
	field1_1.EncodeString(1, mediaKey)

	field1 := NewProtoEncoder()
	field1.EncodeMessage(1, field1_1.Bytes())

	// Field 2 -> Field 1 -> Field 7 -> Field 2: empty message
	field2_1_7_2 := NewProtoEncoder()
	// Empty message

	field2_1_7 := NewProtoEncoder()
	field2_1_7.EncodeMessage(2, field2_1_7_2.Bytes())

	field2_1 := NewProtoEncoder()
	field2_1.EncodeMessage(7, field2_1_7.Bytes())

	// Field 2 -> Field 5 -> Field 2, 3, 5
	field2_5_2 := NewProtoEncoder()
	// Empty message

	field2_5_3 := NewProtoEncoder()
	// Empty message

	field2_5_5_1 := NewProtoEncoder()
	// Empty message

	field2_5_5 := NewProtoEncoder()
	field2_5_5.EncodeMessage(1, field2_5_5_1.Bytes())
	field2_5_5.EncodeInt32(3, 0)

	field2_5 := NewProtoEncoder()
	field2_5.EncodeMessage(2, field2_5_2.Bytes())
	field2_5.EncodeMessage(3, field2_5_3.Bytes())
	field2_5.EncodeMessage(5, field2_5_5.Bytes())

	field2 := NewProtoEncoder()
	field2.EncodeMessage(1, field2_1.Bytes())
	field2.EncodeMessage(5, field2_5.Bytes())

	// Main message
	encoder := NewProtoEncoder()
	encoder.EncodeMessage(1, field1.Bytes())
	encoder.EncodeMessage(2, field2.Bytes())

	serializedData := encoder.Bytes()

	headers := map[string]string{
		"Content-Type":             "application/x-protobuf",
		"x-goog-ext-173412678-bin": "CgcIAhClARgC",
		"x-goog-ext-174067345-bin": "CgIIAg==",
	}

	resp, err := api.request(ctx, "POST",
		"https://photosdata-pa.googleapis.com/$rpc/social.frontend.photos.preparedownloaddata.v1.PhotosPrepareDownloadDataService/PhotosPrepareDownload",
		headers, bytes.NewReader(serializedData))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	// Handle 404 - resource not found (item deleted from Google Photos)
	if resp.StatusCode == http.StatusNotFound {
		return "", fmt.Errorf("%w: %s", ErrMediaNotFound, mediaKey)
	}

	// Parse protobuf response
	respBody, err := readResponseBody(resp)
	if err != nil {
		return "", err
	}

	// Check if response is gzip compressed (magic bytes: 1f 8b)
	if len(respBody) >= 2 && respBody[0] == 0x1f && respBody[1] == 0x8b {
		gzipReader, err := gzip.NewReader(bytes.NewReader(respBody))
		if err != nil {
			return "", fmt.Errorf("failed to create gzip reader: %w", err)
		}
		defer gzipReader.Close()
		respBody, err = io.ReadAll(gzipReader)
		if err != nil {
			return "", fmt.Errorf("failed to decompress gzip response: %w", err)
		}
	}

	if resp.StatusCode != 200 {
		return "", fmt.Errorf("GetDownloadURL failed with status %d", resp.StatusCode)
	}

	result, err := DecodeToMap(respBody)
	if err != nil {
		return "", err
	}

	// Extract download URL from response (field paths: 1->5->2->6 for original, 1->5->2->5 for edited)
	if field1Data, ok := result["1"].(map[string]interface{}); ok {
		if field5Data, ok := field1Data["5"].(map[string]interface{}); ok {
			if field2Data, ok := field5Data["2"].(map[string]interface{}); ok {
				if url, ok := field2Data["6"].(string); ok && url != "" {
					return url, nil
				}
				if url, ok := field2Data["5"].(string); ok && url != "" {
					return url, nil
				}
			}
			if field3Data, ok := field5Data["3"].(map[string]interface{}); ok {
				if url, ok := field3Data["6"].(string); ok && url != "" {
					return url, nil
				}
				if url, ok := field3Data["5"].(string); ok && url != "" {
					return url, nil
				}
			}
		}
	}

	return "", fmt.Errorf("download URL not found in response for media_key %s", mediaKey)
}
