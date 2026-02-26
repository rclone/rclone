// Package api has type definitions for Movistar Cloud
package api

import (
	"encoding/json"
	"fmt"
	"time"
)

// Time represents a timestamp from the Movistar Cloud API.
type Time time.Time

// UnmarshalJSON parses a Movistar Cloud timestamp (Unix millis)
func (t *Time) UnmarshalJSON(data []byte) error {
	var millis int64
	if _, err := fmt.Sscanf(string(data), "%d", &millis); err == nil {
		*t = Time(time.Unix(millis/1000, (millis%1000)*int64(time.Millisecond)))
		return nil
	}
	return fmt.Errorf("can't parse Movistar Cloud time %q", string(data))
}

// BasicISO returns time formatted for Movistar Cloud upload API
// Format: 20060102T150405Z (no dashes, no colons, no fractional seconds)
func BasicISO(t time.Time) string {
	return t.UTC().Format("20060102T150405Z")
}

// Error describes an error response from the Movistar Cloud API
type Error struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Status  int    `json:"status"`
}

// Error returns a string for the error and satisfies the error interface
func (e *Error) Error() string {
	out := fmt.Sprintf("Error %q (%d)", e.Code, e.Status)
	if e.Message != "" {
		out += ": " + e.Message
	}
	return out
}

// Response is the common wrapper for GET responses
type Response struct {
	ResponseTime int64 `json:"responsetime"`
}

// GetResponse wraps a data payload
type GetResponse struct {
	Response
	Data json.RawMessage `json:"data"`
}

// PostResponse wraps an action result
type PostResponse struct {
	Response
	Success string `json:"success"`
}

// Folder represents a folder in Movistar Cloud
type Folder struct {
	Name    string `json:"name"`
	ID      int64  `json:"id"`
	Status  string `json:"status"`
	Magic   bool   `json:"magic"`
	Offline bool   `json:"offline"`
	Date    int64  `json:"date"`
}

// RootFolder extends Folder with creation date
type RootFolder struct {
	Folder
	CreationDate string `json:"creationdate,omitempty"`
}

// FolderWithParent extends Folder with parent ID
type FolderWithParent struct {
	Folder
	ParentID int64 `json:"parentid,omitempty"`
}

// Media represents a file/media item in Movistar Cloud
type Media struct {
	ID        string `json:"id"`
	Date      int64  `json:"date"`
	MediaType string `json:"mediatype"`
	Status    string `json:"status"`
	UserID    string `json:"userid"`
	Name      string `json:"name"`

	// Optional fields depending on request
	URL              string      `json:"url,omitempty"`
	CreationDate     int64       `json:"creationdate,omitempty"`
	ModificationDate int64       `json:"modificationdate,omitempty"`
	Size             int64       `json:"size,omitempty"`
	ETag             string      `json:"etag,omitempty"`
	FolderID         int64       `json:"folder,omitempty"`
	Shared           bool        `json:"shared,omitempty"`
	Origin           *ItemOrigin `json:"origin,omitempty"`
}

// ItemOrigin describes the origin of a media item
type ItemOrigin struct {
	Name string `json:"name"`
}

// ModTimeAsTime returns the modification date as a time.Time
func (m *Media) ModTimeAsTime() time.Time {
	if m.ModificationDate != 0 {
		return time.Unix(m.ModificationDate/1000, (m.ModificationDate%1000)*int64(time.Millisecond))
	}
	if m.Date != 0 {
		return time.Unix(m.Date/1000, (m.Date%1000)*int64(time.Millisecond))
	}
	return time.Time{}
}

// ----- Request types -----

// ListFilesRequest is the body for listing files in a folder
type ListFilesRequest struct {
	Data ListFilesRequestData `json:"data"`
}

// ListFilesRequestData contains the fields for listing files
type ListFilesRequestData struct {
	Fields []string `json:"fields"`
}

// GetFileInfoRequest is the body for getting file info by IDs
type GetFileInfoRequest struct {
	Data GetFileInfoRequestData `json:"data"`
}

// GetFileInfoRequestData contains the IDs and fields for file info request
type GetFileInfoRequestData struct {
	IDs    []int64  `json:"ids"`
	Fields []string `json:"fields"`
}

// CreateFolderRequest is the body for creating a folder
type CreateFolderRequest struct {
	Data CreateFolderRequestData `json:"data"`
}

// CreateFolderRequestData contains the folder creation parameters
type CreateFolderRequestData struct {
	Magic    bool   `json:"magic"`
	Offline  bool   `json:"offline"`
	Name     string `json:"name"`
	ParentID int64  `json:"parentid"`
}

// DeleteFoldersRequest is the body for deleting folders
type DeleteFoldersRequest struct {
	Data DeleteFoldersRequestData `json:"data"`
}

// DeleteFoldersRequestData contains the folder IDs to delete
type DeleteFoldersRequestData struct {
	IDs []int64 `json:"ids"`
}

// DeleteFilesRequest is the body for deleting files
type DeleteFilesRequest struct {
	Data DeleteFilesRequestData `json:"data"`
}

// DeleteFilesRequestData contains the file IDs to delete
type DeleteFilesRequestData struct {
	Files []int64 `json:"files"`
}

// UploadMetadata is the metadata sent with file uploads
type UploadMetadata struct {
	Data UploadMetadataData `json:"data"`
}

// UploadMetadataData contains the upload metadata fields
type UploadMetadataData struct {
	Name             string `json:"name"`
	Size             int64  `json:"size"`
	ModificationDate string `json:"modificationdate"`
	ContentType      string `json:"contenttype"`
	FolderID         int64  `json:"folderid"`
}

// ----- Response types -----

// RootResponse is the response for listing root folders
type RootResponse struct {
	Folders []RootFolder `json:"folders"`
}

// ListFoldersResponse is the response for listing subfolders
type ListFoldersResponse struct {
	Folders []FolderWithParent `json:"folders"`
}

// ListFilesResponse is the response for listing files in a folder
type ListFilesResponse struct {
	Media []Media `json:"media"`
	More  bool    `json:"more"`
}

// GetFileInfoResponse is the response for getting file info
type GetFileInfoResponse struct {
	Media []Media `json:"media"`
}

// CreateFolderResponse is the response for creating a folder
type CreateFolderResponse struct {
	Response
	Success string                   `json:"success"`
	ID      int64                    `json:"id"`
	Data    CreateFolderResponseData `json:"data"`
}

// CreateFolderResponseData contains the created folder
type CreateFolderResponseData struct {
	Folder  Folder `json:"folder"`
	Success string `json:"success"`
}

// UploadResponse is the response for file upload
type UploadResponse struct {
	Response
	Success string `json:"success"`
	ID      string `json:"id"`
	Status  string `json:"status"`
	ETag    string `json:"etag"`
	Type    string `json:"type"`
}

// SaveMetadataRequest is the body for updating file metadata
type SaveMetadataRequest struct {
	Data SaveMetadataRequestData `json:"data"`
}

// SaveMetadataRequestData contains the metadata fields to update.
// Only non-zero/non-empty fields are sent.
type SaveMetadataRequestData struct {
	ID               int64  `json:"id"`
	ModificationDate string `json:"modificationdate,omitempty"`
	Name             string `json:"name,omitempty"`
	FolderID         *int64 `json:"folderid,omitempty"`
}

// SaveMetadataResponse is the response from save-metadata
type SaveMetadataResponse struct {
	Response
	Success string `json:"success"`
	ID      string `json:"id"`
}

// ValidationStatusRequest is the body for get-validation-status
type ValidationStatusRequest struct {
	Data ValidationStatusRequestData `json:"data"`
}

// ValidationStatusRequestData contains the file IDs to check
type ValidationStatusRequestData struct {
	IDs []ValidationStatusID `json:"ids"`
}

// ValidationStatusID is a single file ID wrapper
type ValidationStatusID struct {
	ID int64 `json:"id"`
}

// ValidationStatusResponse is the response for get-validation-status
type ValidationStatusResponse struct {
	IDs []ValidationStatus `json:"ids"`
}

// ValidationStatus is the status of a single file
type ValidationStatus struct {
	ID     int64  `json:"id"`
	Status string `json:"status"`
}

// UserInfo is the response for user profile
type UserInfo struct {
	User UserDetails `json:"user"`
}

// UserDetails contains user information
type UserDetails struct {
	Generic UserGeneric `json:"generic"`
}

// UserGeneric contains generic user fields
type UserGeneric struct {
	FirstName string `json:"firstname"`
	LastName  string `json:"lastname"`
	Email     string `json:"useremail"`
	Timezone  string `json:"timezone"`
	Active    bool   `json:"active"`
	UserID    string `json:"userid"`
}

// LoginStartResponse is the response for initiating login
type LoginStartResponse struct {
	AuthorizationURL string `json:"authorizationurl"`
}
