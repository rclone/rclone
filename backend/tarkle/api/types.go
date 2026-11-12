// Package api has type definitions for the Tarkle public files API
package api

import (
	"encoding/json"
	"fmt"
	"time"
)

// Item is a file or a folder as returned by the listing and stat endpoints.
type Item struct {
	ID             string     `json:"id"`
	Type           string     `json:"type"` // "file" or "folder"
	Name           string     `json:"name"`
	Size           int64      `json:"size"`
	Category       *string    `json:"category"`
	ParentID       *string    `json:"parent_id"`
	CreatedAt      *time.Time `json:"created_at"`
	UpdatedAt      *time.Time `json:"updated_at"`
	ModifiedAt     *time.Time `json:"modified_at"`   // the uploader's own mtime if set, else updated_at
	SHA256         *string    `json:"sha256"`        // 64 lowercase hex or null
	SHA256Source   *string    `json:"sha256_source"` // "server" (verified) or "client" (the uploader's claim)
	Etag           *string    `json:"etag"`
	Protected      bool       `json:"protected"`
	ProtectionKind *string    `json:"protection_kind"`
	Path           string     `json:"path"` // absolute path, only set in recursive listings and by stat
}

// Item types
const (
	ItemTypeFile   = "file"
	ItemTypeFolder = "folder"
)

// Listing is one page of GET /files
type Listing struct {
	Entries    []Item  `json:"entries"`
	NextCursor *string `json:"next_cursor"`
}

// Download is the answer of GET /files/{id}/download
type Download struct {
	DownloadURL string `json:"download_url"`
	ExpiresIn   int    `json:"expires_in"`
}

// Usage is the answer of GET /usage
type Usage struct {
	Storage struct {
		UsedBytes  int64  `json:"used_bytes"`
		LimitBytes *int64 `json:"limit_bytes"`
		FreeBytes  *int64 `json:"free_bytes"`
	} `json:"storage"`
}

// CreateFolder is the body of POST /folders
type CreateFolder struct {
	Name     string  `json:"name"`
	ParentID *string `json:"parent_id,omitempty"`
}

// Patch is the body of PATCH /files/{id}. ParentID is only sent when
// Move is set (null means the root).
type Patch struct {
	Name       *string
	ParentID   *string
	Move       bool
	ModifiedAt *string // sent alone to set a file's modification time
}

// MarshalJSON implements json.Marshaler
func (p Patch) MarshalJSON() ([]byte, error) {
	m := map[string]any{}
	if p.Name != nil {
		m["name"] = *p.Name
	}
	if p.Move {
		m["parent_id"] = p.ParentID // nil encodes as null
	}
	if p.ModifiedAt != nil {
		m["modified_at"] = *p.ModifiedAt
	}
	return json.Marshal(m)
}

// Copy is the body of POST /files/{id}/copy
type Copy struct {
	ParentID *string `json:"parent_id"`
	Name     string  `json:"name,omitempty"`
}

// StartUpload is the body of POST /uploads
type StartUpload struct {
	Name        string  `json:"name"`
	Size        int64   `json:"size"`
	FolderID    *string `json:"folder_id,omitempty"`
	ContentType string  `json:"content_type,omitempty"`
	OnConflict  string  `json:"on_conflict,omitempty"`
	ModifiedAt  string  `json:"modified_at,omitempty"` // recorded at once for an empty file
}

// PartURL is one presigned URL of an upload
type PartURL struct {
	PartNumber int    `json:"partNumber"`
	URL        string `json:"url"`
}

// UploadStarted is the answer of POST /uploads
type UploadStarted struct {
	Action        string    `json:"action"` // upload, skip or created
	Mode          string    `json:"mode"`   // single or multipart
	ID            string    `json:"id"`
	PartSize      int64     `json:"part_size"`
	PartCount     int       `json:"part_count"`
	Uploads       []PartURL `json:"uploads"`
	WillBeRenamed bool      `json:"will_be_renamed"`
	Kind          string    `json:"kind"` // "version" or "empty"
	FileID        string    `json:"file_id"`
	MultipartID   string    `json:"multipart_id"`
	File          *Item     `json:"file"` // set for action created
}

// PartsRequest is the body of POST /uploads/{id}/parts
type PartsRequest struct {
	FromPart    int    `json:"from_part"`
	Count       int    `json:"count"`
	MultipartID string `json:"multipart_id,omitempty"`
}

// PartsResponse is the answer of POST /uploads/{id}/parts
type PartsResponse struct {
	Uploads []PartURL `json:"uploads"`
}

// CompleteRequest is the body of POST /uploads/{id}/complete
type CompleteRequest struct {
	MultipartID string `json:"multipart_id,omitempty"` // optional, the server remembers it for a version
	SHA256      string `json:"sha256,omitempty"`
	ModifiedAt  string `json:"modified_at,omitempty"`
}

// Workspaces is the answer of GET /workspaces
type Workspaces struct {
	Workspaces []struct {
		ID           string `json:"id"`
		Key          string `json:"workspace_key"`
		Name         string `json:"name"`
		URL          string `json:"url"`
		Relationship string `json:"relationship"`
	} `json:"workspaces"`
}

// Error is a JSON error from the API
type Error struct {
	Status  int `json:"-"`
	Wrapper struct {
		Code    string          `json:"code"`
		Message string          `json:"message"`
		Details json.RawMessage `json:"details"`
	} `json:"error"`
	Body string `json:"-"`
}

// Code returns the error code
func (e *Error) Code() string { return e.Wrapper.Code }

// Error satisfies the error interface
func (e *Error) Error() string {
	if e.Wrapper.Message == "" {
		return fmt.Sprintf("tarkle API error %d: %s", e.Status, e.Body)
	}
	return fmt.Sprintf("tarkle API error %d %s: %s", e.Status, e.Wrapper.Code, e.Wrapper.Message)
}

// IncompleteDetails are the details of a 409 incomplete answer. The part
// lists are decoded leniently: the API may send counts or lists.
type IncompleteDetails struct {
	PartsReceived json.RawMessage `json:"parts_received"`
	PartsNeeded   json.RawMessage `json:"parts_needed"`
	MissingParts  []int           `json:"missing_parts"`
}
