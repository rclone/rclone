// Package api has type definitions for dosya.dev
package api

import "fmt"

// Response is the base response wrapper
type Response struct {
	OK    bool   `json:"ok"`
	Error string `json:"error,omitempty"`
}

// Error is an error response from the API.
//
// StatusCode and Status are filled in from the HTTP response. The API
// sets ErrorCode on the errors a client is expected to act on rather
// than just report, e.g. "concurrent_upload_limit".
type Error struct {
	StatusCode           int    `json:"-"`
	Status               string `json:"-"`
	OK                   bool   `json:"ok"`
	Message              string `json:"error"`
	ErrorCode            string `json:"error_code,omitempty"`
	MaxConcurrentUploads int    `json:"max_concurrent_uploads,omitempty"`
}

// Error satisfies the error interface
func (e *Error) Error() string {
	out := fmt.Sprintf("HTTP error %d (%s)", e.StatusCode, e.Status)
	if e.Message != "" {
		out += ": " + e.Message
	}
	if e.ErrorCode != "" {
		out += " (" + e.ErrorCode + ")"
	}
	return out
}

// FileItem represents a file from the dosya API
type FileItem struct {
	ID        string  `json:"id"`
	Name      string  `json:"name"`
	SizeBytes int64   `json:"size_bytes"`
	MimeType  string  `json:"mime_type"`
	Extension string  `json:"extension"`
	Region    string  `json:"region"`
	CreatedAt float64 `json:"created_at"`
	UpdatedAt float64 `json:"updated_at"`
	FolderID  *string `json:"folder_id"`
	IsSynced  int     `json:"is_synced"`
	// ContentHash is the lowercase-hex SHA-256 of the file's bytes, or "" when
	// the server has none (e.g. a file uploaded as an R2 multipart, whose parts
	// are never re-read to hash the whole object - the same limitation S3 has).
	ContentHash string `json:"content_hash"`
}

// FolderItem represents a folder from the dosya API
type FolderItem struct {
	ID        string  `json:"id"`
	Name      string  `json:"name"`
	ParentID  *string `json:"parent_id"`
	CreatedAt float64 `json:"created_at"`
	FileCount int     `json:"file_count"`
	IsSynced  int     `json:"is_synced"`
}

// ListResponse is the response from GET /api/files
type ListResponse struct {
	Response
	Files      []FileItem   `json:"files"`
	Folders    []FolderItem `json:"folders"`
	Pagination Pagination   `json:"pagination"`
}

// Pagination describes which page of files a ListResponse holds
type Pagination struct {
	Page       int `json:"page"`
	PerPage    int `json:"per_page"`
	TotalFiles int `json:"total_files"`
	TotalPages int `json:"total_pages"`
}

// CreateFolderRequest is the request body for POST /api/folders
type CreateFolderRequest struct {
	WorkspaceID string  `json:"workspace_id"`
	ParentID    *string `json:"parent_id"`
	Name        string  `json:"name"`
}

// CreateFolderResponse is the response from POST /api/folders
type CreateFolderResponse struct {
	Response
	Folder struct {
		ID        string  `json:"id"`
		Name      string  `json:"name"`
		ParentID  *string `json:"parent_id"`
		Workspace string  `json:"workspace_id"`
	} `json:"folder"`
}

// DeleteFolderResponse is the response from DELETE /api/folders/:id
type DeleteFolderResponse struct {
	Response
	Permanent      bool `json:"permanent"`
	Complete       bool `json:"complete"`
	Remaining      int  `json:"remaining"`
	FilesAffected  int  `json:"files_affected"`
	FoldersRemoved int  `json:"folders_removed"`
}

// RenameFolderRequest is the request body for PUT /api/folders/:id/rename
type RenameFolderRequest struct {
	Name string `json:"name"`
}

// MoveFolderRequest is the request body for PUT /api/folders/:id/move
type MoveFolderRequest struct {
	ParentID *string `json:"parent_id"`
	Name     string  `json:"name,omitempty"`
}

// MoveFolderResponse is the response from PUT /api/folders/:id/move
type MoveFolderResponse struct {
	Response
	Name string `json:"name"`
}

// UploadInitRequest is the request body for POST /api/upload/init
type UploadInitRequest struct {
	WorkspaceID string  `json:"workspace_id"`
	FileName    string  `json:"file_name"`
	FileSize    int64   `json:"file_size"`
	MimeType    string  `json:"mime_type"`
	Region      string  `json:"region,omitempty"`
	FolderID    *string `json:"folder_id"`
	FileID      *string `json:"file_id,omitempty"`
}

// UploadInitResponse is the response from POST /api/upload/init
type UploadInitResponse struct {
	Response
	SessionID   string `json:"session_id"`
	UploadURL   string `json:"upload_url"`
	WorkspaceID string `json:"workspace_id"`
	FileName    string `json:"file_name"`
	FileSize    int64  `json:"file_size"`
	MimeType    string `json:"mime_type"`
	Extension   string `json:"extension"`
	Region      string `json:"region"`
	Resumable   *struct {
		PartSize      int64  `json:"part_size"`
		TotalParts    int    `json:"total_parts"`
		PartUploadURL string `json:"part_upload_url"`
		CompleteURL   string `json:"complete_url"`
		StatusURL     string `json:"status_url"`
	} `json:"resumable"`
}

// UploadCompleteResponse is the response from PUT /api/upload/:sessionId or POST /api/upload/:sessionId/complete
type UploadCompleteResponse struct {
	Response
	File struct {
		ID        string  `json:"id"`
		Name      string  `json:"name"`
		SizeBytes int64   `json:"size_bytes"`
		MimeType  string  `json:"mime_type"`
		Extension string  `json:"extension"`
		Region    string  `json:"region"`
		Version   int     `json:"version"`
		CreatedAt float64 `json:"created_at"`
		// SHA-256 of the uploaded bytes for a single-shot (small-file) upload;
		// "" for a multipart upload, which the server does not hash.
		ContentHash string `json:"content_hash"`
	} `json:"file"`
}

// UploadPartResponse is the response from PUT /api/upload/:sessionId/part/:partNumber
type UploadPartResponse struct {
	Response
	PartNumber     int    `json:"part_number"`
	ETag           string `json:"etag"`
	BytesUploaded  int64  `json:"bytes_uploaded"`
	PartsCompleted int    `json:"parts_completed"`
	TotalParts     int    `json:"total_parts"`
}

// DeleteFileResponse is the response from DELETE /api/files/:id
type DeleteFileResponse struct {
	Response
	Permanent bool `json:"permanent"`
}

// RenameFileRequest is the request body for PUT /api/files/:id/rename
type RenameFileRequest struct {
	Name string `json:"name"`
}

// RenameFileResponse is the response from PUT /api/files/:id/rename
type RenameFileResponse struct {
	Response
	Name string `json:"name"`
}

// MoveFileRequest is the request body for PUT /api/files/:id/move
type MoveFileRequest struct {
	FolderID *string `json:"folder_id"`
	Name     *string `json:"name,omitempty"`
}

// CopyFileRequest is the request body for POST /api/files/:id/copy
type CopyFileRequest struct {
	FolderID *string `json:"folder_id"`
	Name     string  `json:"name,omitempty"`
}

// CopyFileResponse is the response from POST /api/files/:id/copy
type CopyFileResponse struct {
	Response
	FileID string `json:"file_id"`
	Name   string `json:"name"`
}

// ShareLinkRequest is the request body for POST /api/files/:id/share
type ShareLinkRequest struct {
	ExpiresInDays *int `json:"expires_in_days,omitempty"`
}

// ShareLinkResponse is the response from POST /api/files/:id/share
type ShareLinkResponse struct {
	Response
	Link struct {
		ID        string  `json:"id"`
		Token     string  `json:"token"`
		URL       string  `json:"url"`
		ExpiresAt *int64  `json:"expires_at"`
		CreatedAt float64 `json:"created_at"`
	} `json:"link"`
}

// WorkspaceInfoResponse is the response from GET /api/workspaces/:id
type WorkspaceInfoResponse struct {
	Response
	Storage struct {
		Used  int64 `json:"used"`
		Total int64 `json:"total"`
		Free  int64 `json:"free"`
	} `json:"storage"`
}
