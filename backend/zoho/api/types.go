// Package api provides types used by the Zoho API.
package api

import (
	"strconv"
	"time"
)

// Time represents date and time information for Zoho
// Zoho uses milliseconds since unix epoch (Java currentTimeMillis)
type Time time.Time

// UnmarshalJSON turns JSON into a Time
func (t *Time) UnmarshalJSON(data []byte) error {
	millis, err := strconv.ParseInt(string(data), 10, 64)
	if err != nil {
		return err
	}
	*t = Time(time.Unix(0, millis*int64(time.Millisecond)))
	return nil
}

// User is a Zoho user we are only interested in the ZUID here
type User struct {
	FirstName   string `json:"First_Name"`
	Email       string `json:"Email"`
	LastName    string `json:"Last_Name"`
	DisplayName string `json:"Display_Name"`
	ZUID        int64  `json:"ZUID"`
}

// TeamWorkspace represents a Zoho Team or workspace
// It's actually a VERY large json object that differs between
// Team and Workspace but we are only interested in some fields
// that both of them have so we can use the same struct for both
type TeamWorkspace struct {
	ID         string `json:"id"`
	Attributes struct {
		Name    string `json:"name"`
		Created Time   `json:"created_time_in_millisecond"`
		IsPart  bool   `json:"is_partof"`
	} `json:"attributes"`
}

// TeamWorkspaceResponse is the response by the list teams api
type TeamWorkspaceResponse struct {
	TeamWorkspace []TeamWorkspace `json:"data"`
}

// Item is may represent a file or a folder in Zoho Workdrive
type Item struct {
	ID         string `json:"id"`
	Attributes struct {
		Name         string `json:"name"`
		Type         string `json:"type"`
		IsFolder     bool   `json:"is_folder"`
		CreatedTime  Time   `json:"created_time_in_millisecond"`
		ModifiedTime Time   `json:"modified_time_in_millisecond"`
		UploadedTime Time   `json:"uploaded_time_in_millisecond"`
		StorageInfo  struct {
			Size        int64 `json:"size_in_bytes"`
			FileCount   int64 `json:"files_count"`
			FolderCount int64 `json:"folders_count"`
		} `json:"storage_info"`
	} `json:"attributes"`
}

// ItemInfo contains a single Zoho Item
type ItemInfo struct {
	Item Item `json:"data"`
}

// ItemList contains multiple Zoho Items
type ItemList struct {
	Items []Item `json:"data"`
}

// UploadInfo is a simplified and slightly different version of
// the Item struct only used in the response to uploads
type UploadInfo struct {
	Attributes struct {
		ParentID    string `json:"parent_id"`
		FileName    string `json:"notes.txt"`
		RessourceID string `json:"resource_id"`
	} `json:"attributes"`
}

// UploadResponse is the response to a file Upload
type UploadResponse struct {
	Uploads []UploadInfo `json:"data"`
}

// WriteMetadataRequest is is used to write metadata for a
// single item
type WriteMetadataRequest struct {
	Data WriteMetadata `json:"data"`
}

// WriteMultiMetadataRequest can be used to write metadata for
// multiple items at once but we don't use it that way
type WriteMultiMetadataRequest struct {
	Meta []WriteMetadata `json:"data"`
}

// WriteMetadata is used to write item metadata
type WriteMetadata struct {
	Attributes WriteAttributes `json:"attributes,omitempty"`
	ID         string          `json:"id,omitempty"`
	Type       string          `json:"type"`
}

// WriteAttributes is used to set various attributes for on items
// this is used for Move, Copy, Delete, Rename
type WriteAttributes struct {
	Name        string `json:"name,omitempty"`
	ParentID    string `json:"parent_id,omitempty"`
	RessourceID string `json:"resource_id,omitempty"`
	Status      string `json:"status,omitempty"`
}
