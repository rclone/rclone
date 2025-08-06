// Package api provides types for the filejump backend
package api

import (
	"strconv"
	"time"
)

// Types of things in Item/ItemMini
const (
	// ItemTypeFolder is a folder
	ItemTypeFolder = "folder"
	// ItemTypeImage is an image
	ItemTypeImage = "image"
	// ItemTypeText is a text file
	ItemTypeText = "text"
	// ItemTypeAudio is an audio file
	ItemTypeAudio = "audio"
	// ItemTypeVideo is a video file
	ItemTypeVideo = "video"
	// ItemTypePdf is a pdf file
	ItemTypePdf = "pdf"
	// ItemStatusActive  = "active"
	// ItemStatusDeleted = "deleted"
)

// FileEntries is a list of files
type FileEntries struct {
	CurrentPage uint   `json:"current_page,omitempty"`
	Data        []Item `json:"data,omitempty"`
	From        uint   `json:"from,omitempty"`
	NextPage    *uint  `json:"next_page,omitempty"`
	PerPage     uint   `json:"per_page,omitempty"`
	PrevPage    *uint  `json:"prev_page,omitempty"`
	To          uint   `json:"to,omitempty"`
	Folder      Folder `json:"folder,omitempty"`
}

// Item is a file or folder
type Item struct {
	ID          int    `json:"id,omitempty"`
	Name        string `json:"name,omitempty"`
	Description any    `json:"description,omitempty"`
	FileName    string `json:"file_name,omitempty"`
	Mime        string `json:"mime,omitempty"`
	FileSize    int    `json:"file_size,omitempty"`
	UserID      any    `json:"user_id,omitempty"`
	ParentID    any    `json:"parent_id,omitempty"`
	CreatedAt   string `json:"created_at,omitempty"`
	UpdatedAt   string `json:"updated_at,omitempty"`
	DeletedAt   any    `json:"deleted_at,omitempty"`
	Path        string `json:"path,omitempty"`
	DiskPrefix  any    `json:"disk_prefix,omitempty"`
	Type        string `json:"type,omitempty"`
	Extension   any    `json:"extension,omitempty"`
	Public      bool   `json:"public,omitempty"`
	Thumbnail   bool   `json:"thumbnail,omitempty"`
	WorkspaceID int    `json:"workspace_id,omitempty"`
	OwnerID     int    `json:"owner_id,omitempty"`
	Hash        string `json:"hash,omitempty"`
	URL         any    `json:"url,omitempty"`
	Tags        []any  `json:"tags,omitempty"`
}

// FileUploadResponse is the response from the single file upload endpoint
type FileUploadResponse struct {
	Status    string `json:"status"`
	FileEntry Item   `json:"fileEntry"`
}

// GetID returns the ID of the item
func (i *Item) GetID() (id string) {
	if i.ID == 0 {
		// Return empty string for invalid ID instead of "0"
		return ""
	}
	return strconv.Itoa(i.ID)
}

// ModTime returns the modification time of the item
func (i *Item) ModTime() (t time.Time) {
	// Parse UpdatedAt first
	if i.UpdatedAt != "" {
		format := "2006-01-02T15:04:05.000000Z"
		if parsed, err := time.Parse(format, i.UpdatedAt); err == nil {
			return parsed.Local()
		}
	}

	// Fall back to CreatedAt if UpdatedAt parsing failed
	if i.CreatedAt != "" {
		format := "2006-01-02T15:04:05.000000Z"
		if parsed, err := time.Parse(format, i.CreatedAt); err == nil {
			return parsed.Local()
		}
	}

	// If all parsing fails, return zero time
	return time.Time{}
}

// Folder is a folder
type Folder struct {
	Type        string `json:"type,omitempty"`
	ID          int    `json:"id,omitempty"`
	Hash        string `json:"hash,omitempty"`
	Path        string `json:"path,omitempty"`
	WorkspaceID int    `json:"workspace_id,omitempty"`
	Name        string `json:"name,omitempty"`
}
