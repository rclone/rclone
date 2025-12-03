// Package api has type definitions for drime
//
// Converted from the API docs with help from https://mholt.github.io/json-to-go/
package api

import (
	"encoding/json"
	"fmt"
	"time"
)

// Types of things in Item
const (
	ItemTypeFolder = "folder"
)

// User information
type User struct {
	Email            string      `json:"email"`
	ID               json.Number `json:"id"`
	Avatar           string      `json:"avatar"`
	ModelType        string      `json:"model_type"`
	OwnsEntry        bool        `json:"owns_entry"`
	EntryPermissions []any       `json:"entry_permissions"`
	DisplayName      string      `json:"display_name"`
}

// Permissions for a file
type Permissions struct {
	FilesUpdate   bool `json:"files.update"`
	FilesCreate   bool `json:"files.create"`
	FilesDownload bool `json:"files.download"`
	FilesDelete   bool `json:"files.delete"`
}

// Item describes a folder or a file as returned by /drive/file-entries
type Item struct {
	ID           json.Number `json:"id"`
	Name         string      `json:"name"`
	Description  any         `json:"description"`
	FileName     string      `json:"file_name"`
	Mime         string      `json:"mime"`
	Color        any         `json:"color"`
	Backup       bool        `json:"backup"`
	Tracked      int         `json:"tracked"`
	FileSize     int64       `json:"file_size"`
	UserID       json.Number `json:"user_id"`
	ParentID     json.Number `json:"parent_id"`
	CreatedAt    time.Time   `json:"created_at"`
	UpdatedAt    time.Time   `json:"updated_at"`
	DeletedAt    any         `json:"deleted_at"`
	IsDeleted    int         `json:"is_deleted"`
	Path         string      `json:"path"`
	DiskPrefix   any         `json:"disk_prefix"`
	Type         string      `json:"type"`
	Extension    any         `json:"extension"`
	FileHash     any         `json:"file_hash"`
	Public       bool        `json:"public"`
	Thumbnail    bool        `json:"thumbnail"`
	MuxStatus    any         `json:"mux_status"`
	ThumbnailURL any         `json:"thumbnail_url"`
	WorkspaceID  int         `json:"workspace_id"`
	IsEncrypted  int         `json:"is_encrypted"`
	Iv           any         `json:"iv"`
	VaultID      any         `json:"vault_id"`
	OwnerID      int         `json:"owner_id"`
	Hash         string      `json:"hash"`
	URL          string      `json:"url"`
	Users        []User      `json:"users"`
	Tags         []any       `json:"tags"`
	Permissions  Permissions `json:"permissions"`
}

// Listing response
type Listing struct {
	CurrentPage int    `json:"current_page"`
	Data        []Item `json:"data"`
	From        int    `json:"from"`
	LastPage    int    `json:"last_page"`
	NextPage    int    `json:"next_page"`
	PerPage     int    `json:"per_page"`
	PrevPage    int    `json:"prev_page"`
	To          int    `json:"to"`
	Total       int    `json:"total"`
}

// UploadResponse for a file
type UploadResponse struct {
	Status    string `json:"status"`
	FileEntry Item   `json:"fileEntry"`
}

// CreateFolderRequest for a folder
type CreateFolderRequest struct {
	Name     string      `json:"name"`
	ParentID json.Number `json:"parentId,omitempty"`
}

// CreateFolderResponse for a folder
type CreateFolderResponse struct {
	Status string `json:"status"`
	Folder Item   `json:"folder"`
}

// Error is returned from drime when things go wrong
type Error struct {
	Message string `json:"message"`
}

// Error returns a string for the error and satisfies the error interface
func (e Error) Error() string {
	out := fmt.Sprintf("Error %q", e.Message)
	return out
}

// Check Error satisfies the error interface
var _ error = (*Error)(nil)

// DeleteRequest is the input to DELETE /file-entries
type DeleteRequest struct {
	EntryIDs      []string `json:"entryIds"`
	DeleteForever bool     `json:"deleteForever"`
}

// DeleteResponse is the input to DELETE /file-entries
type DeleteResponse struct {
	Status  string            `json:"status"`
	Message string            `json:"message"`
	Errors  map[string]string `json:"errors"`
}

// UpdateItemRequest describes the updates to be done to an item for PUT /file-entries/{id}/
type UpdateItemRequest struct {
	Name        string `json:"name,omitempty"`
	Description string `json:"description,omitempty"`
}

// UpdateItemResponse is returned by PUT /file-entries/{id}/
type UpdateItemResponse struct {
	Status    string `json:"status"`
	FileEntry Item   `json:"fileEntry"`
}

// MoveRequest is the input to /file-entries/move
type MoveRequest struct {
	EntryIDs      []string `json:"entryIds"`
	DestinationID string   `json:"destinationId"`
}

// MoveResponse is returned by POST /file-entries/move
type MoveResponse struct {
	Status  string `json:"status"`
	Entries []Item `json:"entries"`
}

// CopyRequest is the input to /file-entries/duplicate
type CopyRequest struct {
	EntryIDs      []string `json:"entryIds"`
	DestinationID string   `json:"destinationId"`
}

// CopyResponse is returned by POST /file-entries/duplicate
type CopyResponse struct {
	Status  string `json:"status"`
	Entries []Item `json:"entries"`
}

// MultiPartCreateRequest is the input of POST /s3/multipart/create
type MultiPartCreateRequest struct {
	Filename     string      `json:"filename"`
	Mime         string      `json:"mime"`
	Size         int64       `json:"size"`
	Extension    string      `json:"extension"`
	ParentID     json.Number `json:"parent_id"`
	RelativePath string      `json:"relativePath"`
}

// MultiPartCreateResponse is returned by POST /s3/multipart/create
type MultiPartCreateResponse struct {
	UploadID string `json:"uploadId"`
	Key      string `json:"key"`
}

// CompletedPart Type for completed parts when making a multipart upload.
type CompletedPart struct {
	ETag       string `json:"ETag"`
	PartNumber int32  `json:"PartNumber"`
}

// MultiPartGetURLsRequest is the input of POST /s3/multipart/batch-sign-part-urls
type MultiPartGetURLsRequest struct {
	UploadID    string `json:"uploadId"`
	Key         string `json:"key"`
	PartNumbers []int  `json:"partNumbers"`
}

// MultiPartGetURLsResponse is the result of POST /s3/multipart/batch-sign-part-urls
type MultiPartGetURLsResponse struct {
	URLs []struct {
		URL        string `json:"url"`
		PartNumber int32  `json:"partNumber"`
	} `json:"urls"`
}

// MultiPartCompleteRequest is the input to POST /s3/multipart/complete
type MultiPartCompleteRequest struct {
	UploadID string          `json:"uploadId"`
	Key      string          `json:"key"`
	Parts    []CompletedPart `json:"parts"`
}

// MultiPartCompleteResponse is the result of POST /s3/multipart/complete
type MultiPartCompleteResponse struct {
	Location string `json:"location"`
}

// MultiPartEntriesRequest is the input to POST /s3/entries
type MultiPartEntriesRequest struct {
	ClientMime      string      `json:"clientMime"`
	ClientName      string      `json:"clientName"`
	Filename        string      `json:"filename"`
	Size            int64       `json:"size"`
	ClientExtension string      `json:"clientExtension"`
	ParentID        json.Number `json:"parent_id"`
	RelativePath    string      `json:"relativePath"`
}

// MultiPartEntriesResponse is the result of POST /s3/entries
type MultiPartEntriesResponse struct {
	FileEntry Item `json:"fileEntry"`
}

// MultiPartAbort is the input of POST /s3/multipart/abort
type MultiPartAbort struct {
	UploadID string `json:"uploadId"`
	Key      string `json:"key"`
}
