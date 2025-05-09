package namecrane

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/google/uuid"
	"github.com/rclone/rclone/fs"
	"io"
	"math"
	"mime/multipart"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"
	"time"
)

var (
	ErrUnknownType      = errors.New("unknown content type")
	ErrUnexpectedStatus = errors.New("unexpected status")
	ErrNoFolder         = errors.New("no folder found")
	ErrNoFile           = errors.New("no file found")
)

const (
	defaultFileType    = "application/octet-stream"
	contextFileStorage = "file-storage"
	maxChunkSize       = 15 * 1024 * 1024 // 15 MB

	apiUpload       = "api/upload"
	apiFiles        = "api/v1/filestorage/files"
	apiDeleteFiles  = "api/v1/filestorage/delete-files"
	apiMoveFiles    = "api/v1/filestorage/move-files"
	apiEditFile     = "api/v1/filestorage/{fileId}/edit"
	apiGetFileLink  = "api/v1/filestorage/{fileId}/getlink"
	apiFolder       = "api/v1/filestorage/folder"
	apiFolders      = "api/v1/filestorage/folders"
	apiPutFolder    = "api/v1/filestorage/folder-put"
	apiDeleteFolder = "api/v1/filestorage/delete-folder"
	apiPatchFolder  = "api/v1/filestorage/folder-patch"
	apiFileDownload = "api/v1/filestorage/%s/download"
)

// Namecrane is the Namecrane API Client implementation
type Namecrane struct {
	apiURL      string
	authManager *AuthManager
	client      *http.Client
}

// NewClient creates a new Namecrane Client with the specified URL and auth manager
func NewClient(apiURL string, authManager *AuthManager) *Namecrane {
	return &Namecrane{
		apiURL:      apiURL,
		authManager: authManager,
		client:      http.DefaultClient,
	}
}

// defaultResponse represents a default API response, containing Success and optionally Message
type defaultResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

// Response wraps an *http.Response and provides extra functionality
type Response struct {
	*http.Response
}

// Data is a quick and dirty "read this data" for debugging
func (r *Response) Data() []byte {
	b, _ := io.ReadAll(r.Body)

	return b
}

// Decode only supports JSON.
// The API is weird and returns text/plain for JSON sometimes, but it's almost always JSON.
func (r *Response) Decode(data any) error {
	// Close by default on decode
	defer r.Close()

	return json.NewDecoder(r.Body).Decode(data)
}

// Close is a redirect to r.Body.Close for shorthand
func (r *Response) Close() error {
	return r.Body.Close()
}

// File represents a file object on the remote server, identified by `ID`
type File struct {
	ID         string    `json:"id"`
	Name       string    `json:"fileName"`
	Type       string    `json:"type"`
	Size       int64     `json:"size"`
	DateAdded  time.Time `json:"dateAdded"`
	FolderPath string    `json:"folderPath"`
}

// Folder represents a folder object on the remote server
type Folder struct {
	Name       string   `json:"name"`
	Path       string   `json:"path"`
	Size       int64    `json:"size"`
	Version    string   `json:"version"`
	Count      int      `json:"count"`
	Subfolders []Folder `json:"subfolders"`
	Files      []File   `json:"files"`
}

// Flatten takes all folders and subfolders, returning them as a single slice
func (f Folder) Flatten() []Folder {
	folders := []Folder{f}

	for _, folder := range f.Subfolders {
		folders = append(folders, folder.Flatten()...)
	}

	return folders
}

func (n *Namecrane) String() string {
	return "Namecrane API (Endpoint: " + n.apiURL + ")"
}

// uploadChunk uploads a chunk, then waits for it to be accepted.
// When the last chunk is uploaded, the backend will combine the file, then return a 200 with a body.
func (n *Namecrane) uploadChunk(ctx context.Context, reader io.Reader, fileName string, fileSize, chunkSize int64, fields map[string]string) (*Response, error) {
	// Send POST request to upload
	var requestBody bytes.Buffer
	writer := multipart.NewWriter(&requestBody)

	for key, value := range fields {
		if err := writer.WriteField(key, value); err != nil {
			return nil, fmt.Errorf("failed to write field %s: %w", key, err)
		}
	}

	// Add the file content for this chunk
	part, err := writer.CreateFormFile("file", fileName)

	if err != nil {
		return nil, fmt.Errorf("failed to create form file: %w", err)
	}

	if _, err = io.CopyN(part, reader, chunkSize); err != nil && err != io.EOF {
		return nil, fmt.Errorf("failed to copy chunk data: %w", err)
	}

	if err := writer.Close(); err != nil {
		return nil, err
	}

	// --- Send the chunk ---

	resp, err := n.doRequest(ctx, http.MethodPost, apiUpload,
		requestBody.Bytes(),
		WithContentType(writer.FormDataContentType()))

	if err != nil {
		return nil, fmt.Errorf("failed to upload file: %w", err)
	}

	return resp, err
}

// Upload will push a file to the Namecrane API
func (n *Namecrane) Upload(ctx context.Context, in io.Reader, filePath string, fileSize int64) (*File, error) {
	fileName := path.Base(filePath)

	// encode brackets, fixing bug within uploader
	//	fileName = url.PathEscape(fileName)

	basePath := path.Dir(filePath)

	if basePath == "" || basePath[0] != '/' {
		basePath = "/" + basePath
	}

	// Prepare context data
	contextBytes, err := json.Marshal(folderRequest{
		Folder: basePath,
	})

	if err != nil {
		return nil, err
	}

	contextData := string(contextBytes)

	// Calculate total chunks
	totalChunks := int(math.Ceil(float64(fileSize) / maxChunkSize))

	remaining := fileSize

	id, err := uuid.NewV7()

	if err != nil {
		return nil, err
	}

	fields := map[string]string{
		"resumableChunkSize":    strconv.FormatInt(maxChunkSize, 10),
		"resumableTotalSize":    strconv.FormatInt(fileSize, 10),
		"resumableIdentifier":   id.String(),
		"resumableType":         defaultFileType,
		"resumableFilename":     fileName,
		"resumableRelativePath": fileName,
		"resumableTotalChunks":  strconv.Itoa(totalChunks),
		"context":               contextFileStorage,
		"contextData":           contextData,
	}

	var res *Response

	for chunk := 1; chunk <= totalChunks; chunk++ {
		chunkSize := int64(maxChunkSize)

		if remaining < maxChunkSize {
			chunkSize = remaining
		}

		// strconv.FormatInt is pretty much fmt.Sprintf but without needing to parse the format, replace things, etc.
		// base 10 is the default, see strconv.Itoa
		fields["resumableChunkNumber"] = strconv.Itoa(chunk)
		fields["resumableCurrentChunkSize"] = strconv.FormatInt(chunkSize, 10)

		// --- Prepare the chunk payload ---
		res, err = n.uploadChunk(ctx, in, fileName, fileSize, chunkSize, fields)

		if err != nil {
			return nil, fmt.Errorf("chunk upload failed, error: %w", err)
		}

		if res.StatusCode != http.StatusOK {
			var status defaultResponse

			if err := res.Decode(&status); err != nil {
				return nil, fmt.Errorf("chunk %d upload failed, status: %d, response: %s", chunk, res.StatusCode, string(res.Data()))
			}

			return nil, fmt.Errorf("chunk %d upload failed, status: %d, message: %s", chunk, res.StatusCode, status.Message)
		}

		fs.Debugf(n, "Successfully uploaded chunk %d of %d of size %d/%d for file '%s'\n", chunk, totalChunks, chunkSize, remaining, fileName)

		if chunk == totalChunks {
			var file File

			if err := res.Decode(&file); err != nil {
				return nil, err
			}

			return &file, nil
		} else {
			_ = res.Close()
		}

		// Update progress
		remaining -= chunkSize
	}

	fs.Errorf(n, "Received no response from last upload chunk")

	return nil, errors.New("no response from endpoint")
}

type ListResponse struct {
	Files []File `json:"files"`
}

type FolderResponse struct {
	defaultResponse
	Folder Folder `json:"folder"`
}

// GetFolders returns all folders at the root level
func (n *Namecrane) GetFolders(ctx context.Context) ([]Folder, error) {
	res, err := n.doRequest(ctx, http.MethodGet, apiFolders, nil)

	if err != nil {
		return nil, err
	}

	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%w: %d", ErrUnexpectedStatus, res.StatusCode)
	}

	var response FolderResponse

	if err := res.Decode(&response); err != nil {
		return nil, err
	}

	// Root folder is response.Folder
	return response.Folder.Flatten(), nil
}

// GetFolder returns a single folder
func (n *Namecrane) GetFolder(ctx context.Context, folder string) (*Folder, error) {
	res, err := n.doRequest(ctx, http.MethodPost, apiFolder, folderRequest{
		Folder: folder,
	})

	if err != nil {
		return nil, err
	}

	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%w: %d", ErrUnexpectedStatus, res.StatusCode)
	}

	var folderResponse FolderResponse

	if err := res.Decode(&folderResponse); err != nil {
		return nil, err
	}

	if !folderResponse.Success {
		if folderResponse.Message == "Folder not found" {
			return nil, ErrNoFolder
		}

		return nil, fmt.Errorf("received error from API: %s", folderResponse.Message)
	}

	return &folderResponse.Folder, nil
}

// filesRequest is a struct containing the appropriate fields for making a `GetFiles` request
type filesRequest struct {
	FileIDs []string `json:"fileIds"`
}

// GetFiles returns file data of the specified files
func (n *Namecrane) GetFiles(ctx context.Context, ids ...string) ([]File, error) {
	res, err := n.doRequest(ctx, http.MethodPost, apiFiles, filesRequest{
		FileIDs: ids,
	})

	if err != nil {
		return nil, err
	}

	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%w: %d", ErrUnexpectedStatus, res.StatusCode)
	}

	var response ListResponse

	if err := res.Decode(&response); err != nil {
		return nil, err
	}

	return response.Files, nil
}

// DeleteFiles deletes the remote files specified by ids
func (n *Namecrane) DeleteFiles(ctx context.Context, ids ...string) error {
	res, err := n.doRequest(ctx, http.MethodPost, apiDeleteFiles, filesRequest{
		FileIDs: ids,
	})

	if err != nil {
		return err
	}

	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("%w: %d", ErrUnexpectedStatus, res.StatusCode)
	}

	var response defaultResponse

	if err := res.Decode(&response); err != nil {
		return err
	}

	return nil
}

// DownloadFile opens the specified file as an io.ReadCloser, with optional `opts` (range header, etc)
func (n *Namecrane) DownloadFile(ctx context.Context, id string, opts ...RequestOpt) (io.ReadCloser, error) {
	res, err := n.doRequest(ctx, http.MethodGet, fmt.Sprintf(apiFileDownload, id), nil, opts...)

	if err != nil {
		return nil, err
	}

	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%w: %d", ErrUnexpectedStatus, res.StatusCode)
	}

	return res.Body, nil
}

// GetFileID gets a file id from a specified directory and file name
func (n *Namecrane) GetFileID(ctx context.Context, dir, fileName string) (string, error) {
	var folder *Folder

	if dir == "" || dir == "/" {
		folders, err := n.GetFolders(ctx)

		if err != nil {
			return "", err
		}

		folder = &folders[0]
	} else {
		var err error

		folder, err = n.GetFolder(ctx, dir)

		if err != nil {
			return "", err
		}
	}

	for _, file := range folder.Files {
		if file.Name == fileName {
			return file.ID, nil
		}
	}

	return "", ErrNoFile
}

// Find uses similar methods to GetFileID, but instead checks for both files AND folders
func (n *Namecrane) Find(ctx context.Context, file string) (*Folder, *File, error) {
	base, name := n.parsePath(file)

	var folder *Folder

	if base == "" || base == "/" {
		folders, err := n.GetFolders(ctx)

		if err != nil {
			return nil, nil, err
		}

		folder = &folders[0]

		if name == "" {
			return folder, nil, nil
		}
	} else {
		var err error

		folder, err = n.GetFolder(ctx, base)

		if err != nil {
			return nil, nil, err
		}
	}

	for _, file := range folder.Files {
		if file.Name == name {
			return nil, &file, nil
		}
	}

	for _, folder := range folder.Subfolders {
		if folder.Name == name {
			return &folder, nil, nil
		}
	}

	return nil, nil, ErrNoFile
}

// folderRequest is used for creating and deleting folders
type folderRequest struct {
	ParentFolder string `json:"parentFolder,omitempty"`
	Folder       string `json:"folder"`
}

// CreateFolder creates a new remote folder
func (n *Namecrane) CreateFolder(ctx context.Context, folder string) (*Folder, error) {
	parent, subfolder := n.parsePath(folder)

	res, err := n.doRequest(ctx, http.MethodPost, apiPutFolder, folderRequest{
		ParentFolder: parent,
		Folder:       subfolder,
	})

	if err != nil {
		return nil, err
	}

	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%w: %d", ErrUnexpectedStatus, res.StatusCode)
	}

	var response FolderResponse

	if err := res.Decode(&response); err != nil {
		return nil, err
	}

	if !response.Success {
		return nil, fmt.Errorf("failed to create directory, status: %d, response: %s", res.StatusCode, response.Message)
	}

	return &response.Folder, nil
}

// DeleteFolder deletes a specified folder by name
func (n *Namecrane) DeleteFolder(ctx context.Context, folder string) error {
	parent, subfolder := n.parsePath(folder)

	res, err := n.doRequest(ctx, http.MethodPost, apiDeleteFolder, folderRequest{
		ParentFolder: parent,
		Folder:       subfolder,
	})

	if err != nil {
		return err
	}

	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("%w: %d", ErrUnexpectedStatus, res.StatusCode)
	}

	var status defaultResponse

	if err := res.Decode(&status); err != nil {
		return err
	}

	if !status.Success {
		return fmt.Errorf("failed to remove directory, status: %d, response: %s", res.StatusCode, status.Message)
	}

	return nil
}

type moveFilesRequest struct {
	NewFolder string   `json:"newFolder"`
	FileIDs   []string `json:"fileIDs"`
}

// MoveFiles moves files to the specified folder
func (n *Namecrane) MoveFiles(ctx context.Context, folder string, fileIDs ...string) error {
	res, err := n.doRequest(ctx, http.MethodPost, apiMoveFiles, moveFilesRequest{
		NewFolder: folder,
		FileIDs:   fileIDs,
	})

	if err != nil {
		return err
	}

	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("%w: %d", ErrUnexpectedStatus, res.StatusCode)
	}

	var response FolderResponse

	if err := res.Decode(&response); err != nil {
		return err
	}

	if !response.Success {
		return fmt.Errorf("failed to create directory, status: %d, response: %s", res.StatusCode, response.Message)
	}

	return nil
}

type editFileRequest struct {
	NewFilename string `json:"newFilename"`
}

// RenameFile will rename the specified file to the new name
func (n *Namecrane) RenameFile(ctx context.Context, fileID string, name string) error {
	res, err := n.doRequest(ctx, http.MethodPost, apiEditFile, editFileRequest{
		NewFilename: name,
	}, WithURLParameter("fileId", fileID))

	if err != nil {
		return err
	}

	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("%w: %d", ErrUnexpectedStatus, res.StatusCode)
	}

	var response defaultResponse

	if err := res.Decode(&response); err != nil {
		return err
	}

	if !response.Success {
		return fmt.Errorf("failed to create directory, status: %d, response: %s", res.StatusCode, response.Message)
	}

	return nil
}

type EditFileParams struct {
	Password           string    `json:"password"`
	Published          bool      `json:"published"`
	PublishedUntil     time.Time `json:"publishedUntil"`
	ShortLink          string    `json:"shortLink"`
	PublicDownloadLink string    `json:"publicDownloadLink"`
}

// EditFile updates a file on the backend
func (n *Namecrane) EditFile(ctx context.Context, fileID string, params EditFileParams) error {
	res, err := n.doRequest(ctx, http.MethodPost, apiEditFile, params, WithURLParameter("fileId", fileID))

	if err != nil {
		return err
	}

	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("%w: %d", ErrUnexpectedStatus, res.StatusCode)
	}

	var response defaultResponse

	if err := res.Decode(&response); err != nil {
		return err
	}

	if !response.Success {
		return fmt.Errorf("failed to create directory, status: %d, response: %s", res.StatusCode, response.Message)
	}

	return nil
}

type linkResponse struct {
	defaultResponse
	PublicLink string `json:"publicLink"`
	ShortLink  string `json:"shortLink"`
	IsPublic   bool   `json:"isPublic"`
}

// GetLink creates a short link and public link to a file
// This is combined with EditFile to make it public
func (n *Namecrane) GetLink(ctx context.Context, fileID string) (string, string, error) {
	res, err := n.doRequest(ctx, http.MethodGet, apiGetFileLink, nil, WithURLParameter("fileId", fileID))

	if err != nil {
		return "", "", err
	}

	if res.StatusCode != http.StatusOK {
		return "", "", fmt.Errorf("%w: %d", ErrUnexpectedStatus, res.StatusCode)
	}

	var response linkResponse

	if err := res.Decode(&response); err != nil {
		return "", "", err
	}

	if !response.Success {
		return "", "", fmt.Errorf("failed to create directory, status: %d, response: %s", res.StatusCode, response.Message)
	}

	return response.ShortLink, response.PublicLink, nil
}

type patchFolderRequest struct {
	folderRequest
	ParentFolder    string `json:"parentFolder"`
	Folder          string `json:"folder"`
	NewFolderName   string `json:"newFolderName,omitempty"`
	NewParentFolder string `json:"newParentFolder,omitempty"`
}

func (n *Namecrane) MoveFolder(ctx context.Context, folder, newParentFolder string) error {
	_, subfolder := n.parsePath(folder)

	res, err := n.doRequest(ctx, http.MethodPost, apiPatchFolder, patchFolderRequest{
		//ParentFolder:    parent,
		Folder:          folder,
		NewParentFolder: newParentFolder,
		NewFolderName:   subfolder,
	})

	if err != nil {
		return err
	}

	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("%w: %d (%s)", ErrUnexpectedStatus, res.StatusCode, string(res.Data()))
	}

	var response defaultResponse

	if err := res.Decode(&response); err != nil {
		return err
	}

	if !response.Success {
		return fmt.Errorf("failed to move directory, status: %d, response: %s", res.StatusCode, response.Message)
	}

	return nil
}

// apiUrl joins the base API URL with the path specified
func (n *Namecrane) apiUrl(subPath string) (string, error) {
	u, err := url.Parse(n.apiURL)

	if err != nil {
		return "", err
	}

	u.Path = path.Join(u.Path, subPath)

	return u.String(), nil
}

// RequestOpt is a quick helper for changing request options
type RequestOpt func(r *http.Request)

// WithContentType overrides specified content types
func WithContentType(contentType string) RequestOpt {
	return func(r *http.Request) {
		r.Header.Set("Content-Type", contentType)
	}
}

// WithHeader sets header values on the request
func WithHeader(key, value string) RequestOpt {
	return func(r *http.Request) {
		r.Header.Set(key, value)
	}
}

// WithURLParameter replaces a URL parameter encased in {} with the value
func WithURLParameter(key string, value any) RequestOpt {
	return func(r *http.Request) {
		var valStr string
		switch v := value.(type) {
		case string:
			valStr = v
		case int:
			valStr = strconv.Itoa(v)
		default:
			valStr = fmt.Sprintf("%v", v)
		}

		r.URL.Path = strings.Replace(r.URL.Path, "{"+key+"}", valStr, -1)
	}
}

func doHttpRequest(ctx context.Context, client *http.Client, method, u string, body any, opts ...RequestOpt) (*Response, error) {
	var bodyReader io.Reader
	var jsonBody bool

	if body != nil {
		switch method {
		case http.MethodPost:
			switch v := body.(type) {
			case io.Reader:
				bodyReader = v
			case []byte:
				bodyReader = bytes.NewReader(v)
			case string:
				bodyReader = strings.NewReader(v)
			default:
				b, err := json.Marshal(body)

				if err != nil {
					return nil, err
				}

				fs.Debugf(nil, "body: %s", string(b))

				bodyReader = bytes.NewReader(b)

				jsonBody = true
			}
		case http.MethodGet:
			switch v := body.(type) {
			case *url.Values:
				u += "?" + v.Encode()
			}
		}
	}

	// Create the HTTP request
	req, err := http.NewRequestWithContext(ctx, method, u, bodyReader)

	if err != nil {
		return nil, fmt.Errorf("failed to create rmdir request: %w", err)
	}

	if jsonBody {
		req.Header.Set("Content-Type", "application/json")
	}

	// Apply extra options like overriding content types
	for _, opt := range opts {
		opt(req)
	}

	// Execute the HTTP request
	resp, err := client.Do(req)

	if err != nil {
		return nil, fmt.Errorf("failed to execute rmdir request: %w", err)
	}

	return &Response{
		Response: resp,
	}, err
}

func (n *Namecrane) doRequest(ctx context.Context, method, path string, body any, opts ...RequestOpt) (*Response, error) {
	ctx = context.WithValue(ctx, "httpClient", n.client)

	token, err := n.authManager.GetToken(ctx)

	if err != nil {
		return nil, fmt.Errorf("failed to retrieve token: %w", err)
	}

	opts = append(opts, WithHeader("Authorization", "Bearer "+token))

	apiUrl, err := n.apiUrl(path)

	if err != nil {
		return nil, err
	}

	return doHttpRequest(ctx, n.client, method, apiUrl, body, opts...)
}

// parsePath parses the last segment off the specified path, representing either a file or directory
func (n *Namecrane) parsePath(path string) (basePath, lastSegment string) {
	trimmedPath := strings.Trim(path, "/")

	segments := strings.Split(trimmedPath, "/")

	if len(segments) > 1 {
		basePath = "/" + strings.Join(segments[:len(segments)-1], "/")

		lastSegment = segments[len(segments)-1]
	} else {
		basePath = "/"
		lastSegment = segments[0]
	}

	return
}
