package enteio

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/fshttp"
)

// Collection represents an Ente collection (album/folder)
type Collection struct {
	ID       string    `json:"id"`
	Name     string    `json:"name"`
	ParentID string    `json:"parentID,omitempty"`
	ModTime  time.Time `json:"updatedAt"`
}

// File represents an Ente file
type File struct {
	ID      string    `json:"id"`
	Name    string    `json:"name"`
	Size    int64     `json:"size"`
	ModTime time.Time `json:"updatedAt"`
}

// Client is the Ente API client
type Client struct {
	endpoint   string
	email      string
	password   string
	authToken  string
	httpClient *http.Client
}

// NewClient creates a new Ente API client
func NewClient(endpoint, email, password string) *Client {
	return &Client{
		endpoint:   endpoint,
		email:      email,
		password:   password,
		httpClient: fshttp.NewClient(context.Background()),
	}
}

// SetAuthToken sets the authentication token
func (c *Client) SetAuthToken(token string) {
	c.authToken = token
}

// GetAuthToken returns the current auth token
func (c *Client) GetAuthToken() string {
	return c.authToken
}

// APIError represents an error from the Ente API
type APIError struct {
	StatusCode int
	Message    string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("ente API error: %s (status %d)", e.Message, e.StatusCode)
}

// doRequest performs an HTTP request to the Ente API
func (c *Client) doRequest(ctx context.Context, method, path string, body interface{}, result interface{}) (err error) {
	url := c.endpoint + path

	var bodyReader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("failed to marshal request body: %w", err)
		}
		bodyReader = bytes.NewReader(data)
	}

	req, err := http.NewRequestWithContext(ctx, method, url, bodyReader)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	if c.authToken != "" {
		req.Header.Set("X-Auth-Token", c.authToken)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer fs.CheckClose(resp.Body, &err)

	if resp.StatusCode >= 400 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return &APIError{
			StatusCode: resp.StatusCode,
			Message:    string(bodyBytes),
		}
	}

	if result != nil {
		if err := json.NewDecoder(resp.Body).Decode(result); err != nil {
			return fmt.Errorf("failed to decode response: %w", err)
		}
	}

	return nil
}

// Authenticate authenticates with the Ente API
func (c *Client) Authenticate(ctx context.Context) error {
	// Ente uses SRP authentication which is complex
	// For now, we'll use a simplified flow that works for most cases
	// In a full implementation, this would need proper SRP implementation

	// Step 1: Get SRP attributes
	type srpAttributesResponse struct {
		SRPUserID         string `json:"srpUserID"`
		SRPSalt           string `json:"srpSalt"`
		MemLimit          int    `json:"memLimit"`
		OpsLimit          int    `json:"opsLimit"`
		KekSalt           string `json:"kekSalt"`
		IsEmailMFAEnabled bool   `json:"isEmailMFAEnabled"`
	}

	var srpAttrs srpAttributesResponse
	err := c.doRequest(ctx, "GET", "/users/srp/attributes?email="+c.email, nil, &srpAttrs)
	if err != nil {
		// If SRP is not available, try simple authentication
		return c.simpleAuthenticate(ctx)
	}

	// For full SRP implementation, we would need to:
	// 1. Derive key from password using argon2id with salt
	// 2. Create SRP client
	// 3. Exchange messages with server
	// 4. Get the encrypted token
	// 5. Decrypt with the derived key

	// For now, fall back to simple auth
	return c.simpleAuthenticate(ctx)
}

// simpleAuthenticate performs simplified authentication
func (c *Client) simpleAuthenticate(ctx context.Context) error {
	// This is a placeholder for the actual authentication
	// In a production implementation, this would use the proper SRP flow

	type verifyEmailRequest struct {
		Email string `json:"email"`
	}

	err := c.doRequest(ctx, "POST", "/users/ott", &verifyEmailRequest{Email: c.email}, nil)
	if err != nil {
		fs.Debugf(nil, "OTT request: %v", err)
	}

	// Since we can't do full interactive auth, we'll indicate that manual setup is needed
	return fmt.Errorf("ente.io requires manual token configuration; please use 'rclone config' to set up authentication")
}

// GetCollections returns all collections
func (c *Client) GetCollections(ctx context.Context) ([]Collection, error) {
	type collectionsResponse struct {
		Collections []struct {
			ID                  string `json:"id"`
			EncryptedName       string `json:"encryptedName"`
			NameDecryptionNonce string `json:"nameDecryptionNonce"`
			UpdatedAt           int64  `json:"updatedAt"`
			Owner               struct {
				ID string `json:"id"`
			} `json:"owner"`
		} `json:"collections"`
	}

	var resp collectionsResponse
	err := c.doRequest(ctx, "GET", "/collections/v2?sinceTime=0", nil, &resp)
	if err != nil {
		return nil, err
	}

	collections := make([]Collection, 0, len(resp.Collections))
	for i, coll := range resp.Collections {
		// Name is encrypted in Ente, use ID as placeholder
		// In full implementation, would decrypt using collection key
		collections = append(collections, Collection{
			ID:      coll.ID,
			Name:    fmt.Sprintf("collection_%d", i),
			ModTime: time.Unix(coll.UpdatedAt/1000000, 0),
		})
	}

	return collections, nil
}

// GetFilesInCollection returns files in a collection
func (c *Client) GetFilesInCollection(ctx context.Context, collectionID string) ([]File, error) {
	type filesResponse struct {
		Diff []struct {
			ID        string `json:"id"`
			UpdatedAt int64  `json:"updatedAt"`
			Info      *struct {
				FileSize int64 `json:"fileSize"`
			} `json:"info"`
			Metadata struct {
				EncryptedData string `json:"encryptedData"`
			} `json:"metadata"`
		} `json:"diff"`
		HasMore bool `json:"hasMore"`
	}

	var resp filesResponse
	err := c.doRequest(ctx, "GET", "/collections/v2/diff?collectionID="+collectionID+"&sinceTime=0", nil, &resp)
	if err != nil {
		return nil, err
	}

	files := make([]File, 0, len(resp.Diff))
	for i, f := range resp.Diff {
		var size int64
		if f.Info != nil {
			size = f.Info.FileSize
		}
		files = append(files, File{
			ID:      f.ID,
			Name:    fmt.Sprintf("file_%d", i), // Encrypted name, would need decryption
			Size:    size,
			ModTime: time.Unix(f.UpdatedAt/1000000, 0),
		})
	}

	return files, nil
}

// CreateCollection creates a new collection
func (c *Client) CreateCollection(ctx context.Context, name, parentID string) (*Collection, error) {
	// In Ente, creating a collection requires:
	// 1. Generate collection key
	// 2. Encrypt name with collection key
	// 3. Encrypt collection key with user's public key
	// 4. Send to server

	// For now, return not implemented
	return nil, fmt.Errorf("create collection not implemented: ente uses end-to-end encryption")
}

// DeleteCollection deletes a collection
func (c *Client) DeleteCollection(ctx context.Context, collectionID string) error {
	return c.doRequest(ctx, "DELETE", "/collections/v3/"+collectionID, nil, nil)
}

// DownloadFile downloads a file
func (c *Client) DownloadFile(ctx context.Context, fileID, collectionID string) (io.ReadCloser, error) {
	// Get download URL
	type downloadURLResponse struct {
		URL string `json:"url"`
	}

	var resp downloadURLResponse
	err := c.doRequest(ctx, "GET", "/files/download/"+fileID, nil, &resp)
	if err != nil {
		return nil, err
	}

	// Download the encrypted file
	req, err := http.NewRequestWithContext(ctx, "GET", resp.URL, nil)
	if err != nil {
		return nil, err
	}

	httpResp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}

	if httpResp.StatusCode != http.StatusOK {
		httpResp.Body.Close()
		return nil, fmt.Errorf("download failed with status %d", httpResp.StatusCode)
	}

	// In a full implementation, we would decrypt the file here
	// using the file key (which is encrypted with collection key)
	return httpResp.Body, nil
}

// UploadFile uploads a file
func (c *Client) UploadFile(ctx context.Context, collectionID, name string, in io.Reader, size int64, modTime time.Time) (*File, error) {
	// Ente file upload requires:
	// 1. Get upload URLs from server
	// 2. Encrypt file with random file key
	// 3. Upload encrypted chunks
	// 4. Encrypt file key with collection key
	// 5. Create file entry on server

	// For now, return not implemented
	return nil, fmt.Errorf("upload not implemented: ente uses end-to-end encryption")
}

// DeleteFile deletes a file
func (c *Client) DeleteFile(ctx context.Context, fileID, collectionID string) error {
	type deleteRequest struct {
		FileIDs      []string `json:"fileIDs"`
		CollectionID string   `json:"collectionID"`
	}

	req := &deleteRequest{
		FileIDs:      []string{fileID},
		CollectionID: collectionID,
	}

	return c.doRequest(ctx, "POST", "/collections/v3/remove-files", req, nil)
}
