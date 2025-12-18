package mediavfs

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/rclone/rclone/fs"
)

const (
	defaultTimeout = 60 * time.Second
	maxRetries     = 10
)

// GPhotoAPI handles Google Photos API interactions
type GPhotoAPI struct {
	token          string
	httpClient     *http.Client
	tokenServerURL string
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

// GetAuthToken fetches or refreshes the authentication token
func (api *GPhotoAPI) GetAuthToken(ctx context.Context, force bool) error {
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
	fs.Infof(nil, "gphoto: obtained auth token for user %s", api.user)
	return nil
}

// request makes an authenticated HTTP request with retry logic
func (api *GPhotoAPI) request(ctx context.Context, method, url string, headers map[string]string, body io.Reader) (*http.Response, error) {
	var resp *http.Response
	var err error

	for retry := 0; retry < 5; retry++ {
		// Ensure we have a token
		if api.token == "" && api.tokenServerURL != "" {
			if err := api.GetAuthToken(ctx, false); err != nil {
				return nil, err
			}
		}

		req, err := http.NewRequestWithContext(ctx, method, url, body)
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
			resp.Body.Close()
			fs.Infof(nil, "gphoto: token expired (status %d), refreshing...", resp.StatusCode)
			if err := api.GetAuthToken(ctx, true); err != nil {
				return nil, err
			}
			continue

		case http.StatusInternalServerError, http.StatusServiceUnavailable:
			resp.Body.Close()
			retry++
			time.Sleep(1 * time.Second)
			continue

		default:
			resp.Body.Close()
			return nil, fmt.Errorf("HTTP error: status %d", resp.StatusCode)
		}
	}

	return resp, err
}

// GetUploadToken obtains an upload token from Google Photos
func (api *GPhotoAPI) GetUploadToken(ctx context.Context, sha1HashB64 string, fileSize int64) (string, error) {
	// Encode protobuf message (simplified - using JSON for now)
	// In production, you'd use actual protobuf encoding
	protoBody := map[string]interface{}{
		"1": 2,
		"2": 2,
		"3": 1,
		"4": 3,
		"7": fileSize,
	}

	// For simplicity, marshal as JSON (in production, use protobuf)
	// This is placeholder - actual implementation would use blackboxprotobuf equivalent
	serializedData, _ := json.Marshal(protoBody)

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
	// Encode protobuf message
	protoBody := map[string]interface{}{
		"1": map[string]interface{}{
			"1": map[string]interface{}{
				"1": base64.StdEncoding.EncodeToString(sha1Hash),
			},
			"2": map[string]interface{}{},
		},
	}

	serializedData, _ := json.Marshal(protoBody)

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

	// Parse response (simplified)
	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}

	// Extract media key if found
	if mediaData, ok := result["1"].(map[string]interface{}); ok {
		if mediaInfo, ok := mediaData["2"].(map[string]interface{}); ok {
			if mediaKey, ok := mediaInfo["1"].(string); ok {
				return mediaKey, nil
			}
		}
	}

	return "", nil // File not found
}

// UploadFile uploads file content to Google Photos
func (api *GPhotoAPI) UploadFile(ctx context.Context, uploadToken string, content io.Reader) error {
	url := fmt.Sprintf("https://photos.googleapis.com/data/upload/uploadmedia/interactive?upload_id=%s", uploadToken)

	headers := map[string]string{}

	resp, err := api.request(ctx, "PUT", url, headers, content)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	return nil
}

// CommitUpload commits the uploaded file to Google Photos
func (api *GPhotoAPI) CommitUpload(ctx context.Context, fileName string, sha1Hash []byte, fileSize int64, uploadTimestamp int64, model, quality string) (string, error) {
	qualityMap := map[string]int{
		"saver":    1,
		"original": 3,
	}

	if uploadTimestamp == 0 {
		uploadTimestamp = time.Now().Unix()
	}

	// Simplified protobuf structure
	protoBody := map[string]interface{}{
		"1": map[string]interface{}{
			"2": fileName,
			"3": base64.StdEncoding.EncodeToString(sha1Hash),
			"4": map[string]interface{}{
				"1": uploadTimestamp,
				"2": 46000000,
			},
			"7":  qualityMap[quality],
			"10": 1,
			"17": 0,
		},
		"2": map[string]interface{}{
			"3": model,
			"4": "Google",
			"5": 28, // Android API version
		},
		"3": []byte{1, 3},
	}

	serializedData, _ := json.Marshal(protoBody)

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

	// Parse response to get media key
	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}

	// Extract media key
	if mediaData, ok := result["1"].(map[string]interface{}); ok {
		if keyData, ok := mediaData["3"].(map[string]interface{}); ok {
			if mediaKey, ok := keyData["1"].(string); ok {
				return mediaKey, nil
			}
		}
	}

	return "", fmt.Errorf("media key not found in response")
}

// MoveToTrash moves files to trash
func (api *GPhotoAPI) MoveToTrash(ctx context.Context, dedupKeys []string) error {
	// Process in batches of 500
	batchSize := 500
	for i := 0; i < len(dedupKeys); i += batchSize {
		end := i + batchSize
		if end > len(dedupKeys) {
			end = len(dedupKeys)
		}

		batch := dedupKeys[i:end]

		protoBody := map[string]interface{}{
			"2": 1,
			"3": batch,
			"4": 1,
			"8": map[string]interface{}{
				"4": map[string]interface{}{
					"2": map[string]interface{}{},
					"3": map[string]interface{}{"1": map[string]interface{}{}},
					"4": map[string]interface{}{},
					"5": map[string]interface{}{"1": map[string]interface{}{}},
				},
			},
			"9": map[string]interface{}{
				"1": 5,
				"2": map[string]interface{}{
					"1": "49029607",
					"2": "28",
				},
			},
		}

		serializedData, _ := json.Marshal(protoBody)

		headers := map[string]string{
			"Content-Type": "application/x-protobuf",
		}

		resp, err := api.request(ctx, "POST",
			"https://photosdata-pa.googleapis.com/6439526531001121323/17490284929287180316",
			headers, bytes.NewReader(serializedData))
		if err != nil {
			return err
		}
		resp.Body.Close()
	}

	return nil
}

// GetLibraryState gets the current library state from Google Photos
func (api *GPhotoAPI) GetLibraryState(ctx context.Context, stateToken, pageToken string) ([]byte, error) {
	// Construct protobuf request (simplified - using placeholder)
	protoBody := map[string]interface{}{
		"1": map[string]interface{}{
			"6": stateToken,
			"4": pageToken,
			"7": 2,
		},
	}

	serializedData, _ := json.Marshal(protoBody)

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

	// Read response body
	return io.ReadAll(resp.Body)
}

// GetLibraryPage gets a page of library results
func (api *GPhotoAPI) GetLibraryPage(ctx context.Context, pageToken, stateToken string) ([]byte, error) {
	return api.GetLibraryState(ctx, stateToken, pageToken)
}
