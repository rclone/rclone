// Package darkibox provides an interface to the Darkibox file hosting service.
package darkibox

import (
	"encoding/json"
	"fmt"
	"time"
)

// APIResponse is the standard response wrapper from the darkibox API.
type APIResponse struct {
	Status    int    `json:"status"`
	Msg       string `json:"msg"`
	ServerTime string `json:"server_time"`
}

// AccountInfoResponse is returned by /api/account/info
type AccountInfoResponse struct {
	APIResponse
	Result AccountInfo `json:"result"`
}

// AccountInfo contains the account details.
type AccountInfo struct {
	Login         string `json:"login"`
	Email         string `json:"email"`
	Balance       string `json:"balance"`
	Premium       int    `json:"premium"`
	PremiumExpire string `json:"premium_expire"`
	StorageUsed   int64  `json:"storage_used"`
	StorageLeft   string `json:"storage_left"`
	FilesTotal    int    `json:"files_total"`
}

// FileInfoResponse is returned by /api/file/info
type FileInfoResponse struct {
	APIResponse
	Result []FileInfo `json:"result"`
}

// FileInfo contains metadata about a single file.
// Note: field names differ between /api/file/info and /api/file/list endpoints.
type FileInfo struct {
	// Common fields
	FileCode string `json:"file_code"`
	FolderID FlexID `json:"fld_id"`
	Status   int    `json:"status"`

	// From /api/file/info
	FileName    string `json:"file_name"`
	FileTitle   string `json:"file_title"`
	FileSize    int64  `json:"file_size"`
	FileLength  int    `json:"file_length"`
	PremiumOnly int    `json:"file_premium_only"`
	PlayerImg   string `json:"player_img"`
	Streaming   int    `json:"streaming"`
	FileCreated string `json:"file_created"`
	CategoryID  int    `json:"cat_id"`

	// From /api/file/list (different field names for same data)
	Name     string `json:"name"`
	Title    string `json:"title"`
	Size     int64  `json:"size"`
	Length   int    `json:"length"`
	Uploaded string `json:"uploaded"`
	Link     string `json:"link"`
	Views    int    `json:"views"`
	CanPlay  int    `json:"canplay"`
	Public   int    `json:"public"`
}

// GetFileName returns the best available filename.
func (fi FileInfo) GetFileName() string {
	if fi.Name != "" {
		return fi.Name
	}
	if fi.FileName != "" {
		return fi.FileName
	}
	title := fi.Title
	if title == "" {
		title = fi.FileTitle
	}
	if title != "" {
		return title + ".mp4"
	}
	return fi.FileCode + ".mp4"
}

// GetFileSize returns the best available file size.
func (fi FileInfo) GetFileSize() int64 {
	if fi.Size > 0 {
		return fi.Size
	}
	return fi.FileSize
}

// GetCreatedTime returns the best available creation time.
func (fi FileInfo) GetCreatedTime() string {
	if fi.Uploaded != "" {
		return fi.Uploaded
	}
	return fi.FileCreated
}

// FileListResponse is returned by /api/file/list
type FileListResponse struct {
	APIResponse
	Result FileListResult `json:"result"`
}

// FileListResult contains paginated file listing.
type FileListResult struct {
	Results      int        `json:"results"`
	ResultsTotal int        `json:"results_total"`
	Pages        int        `json:"pages"`
	Files        []FileInfo `json:"files"`
}

// DirectLinkResponse is returned by /api/file/direct_link
type DirectLinkResponse struct {
	APIResponse
	Result DirectLinkResult `json:"result"`
}

// DirectLinkResult contains the direct download links.
type DirectLinkResult struct {
	Versions  []FileVersion `json:"versions"`
	FileLength int          `json:"file_length"`
	PlayerImg  string       `json:"player_img"`
}

// FileVersion represents a single quality version of a file.
type FileVersion struct {
	URL      string `json:"url"`
	Size     int64  `json:"size"`
	Name     string `json:"name"`
	Filename string `json:"filename"`
}

// FolderListResponse is returned by /api/folder/list
type FolderListResponse struct {
	APIResponse
	Result FolderListResult `json:"result"`
}

// FolderListResult contains folder listing.
type FolderListResult struct {
	Folders []FolderInfo `json:"folders"`
	Files   []FileInfo   `json:"files"`
}

// FlexID is a type that can unmarshal both JSON numbers and strings to a string.
// Darkibox API inconsistently returns folder IDs as numbers or strings.
type FlexID string

// UnmarshalJSON implements json.Unmarshaler for FlexID.
func (f *FlexID) UnmarshalJSON(data []byte) error {
	// Try string first
	var s string
	if err := json.Unmarshal(data, &s); err == nil {
		*f = FlexID(s)
		return nil
	}
	// Try number
	var n json.Number
	if err := json.Unmarshal(data, &n); err == nil {
		*f = FlexID(n.String())
		return nil
	}
	return fmt.Errorf("cannot unmarshal %s into FlexID", string(data))
}

// String returns the string value.
func (f FlexID) String() string {
	return string(f)
}

// FolderInfo contains metadata about a single folder.
type FolderInfo struct {
	FolderID   FlexID `json:"fld_id"`
	ParentID   FlexID `json:"parent_id"`
	Code       string `json:"code"`
	Name       string `json:"name"`
	Descr      string `json:"descr"`
	TotalFiles int    `json:"total_files"`
	TotalSize  int64  `json:"total_size"`
}

// FolderCreateResponse is returned by /api/folder/create
type FolderCreateResponse struct {
	APIResponse
	Result FolderCreateResult `json:"result"`
}

// FolderCreateResult contains the new folder ID.
type FolderCreateResult struct {
	FolderID FlexID `json:"fld_id"`
}

// UploadServerResponse is returned by /api/upload/server
type UploadServerResponse struct {
	APIResponse
	Result string `json:"result"`
}

// UploadResponse is returned by the upload endpoint.
type UploadResponse struct {
	Msg    string       `json:"msg"`
	Status int          `json:"status"`
	Files  []UploadFile `json:"files"`
}

// UploadFile contains info about an uploaded file.
type UploadFile struct {
	FileCode string `json:"filecode"`
	Filename string `json:"filename"`
	Status   string `json:"status"`
}

// GenericResponse is used for simple API responses (delete, edit, move).
// The "result" field can be "true"/"false" (string), true/false (bool), or other values.
type GenericResponse struct {
	APIResponse
	Result json.RawMessage `json:"result"`
}

// parseTime parses the darkibox time format "2006-01-02 15:04:05"
func parseTime(s string) time.Time {
	t, err := time.Parse("2006-01-02 15:04:05", s)
	if err != nil {
		return time.Time{}
	}
	return t
}
