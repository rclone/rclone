// Package api has type definitions for box
//
// Converted from the API docs with help from https://mholt.github.io/json-to-go/
package api

import (
	"encoding/json"
	"fmt"
	"time"
)

const (
	// 2017-05-03T07:26:10-07:00
	timeFormat = `"` + time.RFC3339 + `"`
)

// Time represents represents date and time information for the
// box API, by using RFC3339
type Time time.Time

// MarshalJSON turns a Time into JSON (in UTC)
func (t *Time) MarshalJSON() (out []byte, err error) {
	timeString := (*time.Time)(t).Format(timeFormat)
	return []byte(timeString), nil
}

// UnmarshalJSON turns JSON into a Time
func (t *Time) UnmarshalJSON(data []byte) error {
	newT, err := time.Parse(timeFormat, string(data))
	if err != nil {
		return err
	}
	*t = Time(newT)
	return nil
}

// Error is returned from box when things go wrong
type Error struct {
	Type        string          `json:"type"`
	Status      int             `json:"status"`
	Code        string          `json:"code"`
	ContextInfo json.RawMessage `json:"context_info"`
	HelpURL     string          `json:"help_url"`
	Message     string          `json:"message"`
	RequestID   string          `json:"request_id"`
}

// Error returns a string for the error and satisfies the error interface
func (e *Error) Error() string {
	out := fmt.Sprintf("Error %q (%d)", e.Code, e.Status)
	if e.Message != "" {
		out += ": " + e.Message
	}
	if e.ContextInfo != nil {
		out += fmt.Sprintf(" (%+v)", e.ContextInfo)
	}
	return out
}

// Check Error satisfies the error interface
var _ error = (*Error)(nil)

// ItemFields are the fields needed for FileInfo
var ItemFields = "type,id,sequence_id,etag,sha1,name,size,created_at,modified_at,content_created_at,content_modified_at,item_status,shared_link"

// Types of things in Item
const (
	ItemTypeFolder    = "folder"
	ItemTypeFile      = "file"
	ItemStatusActive  = "active"
	ItemStatusTrashed = "trashed"
	ItemStatusDeleted = "deleted"
)

// Item describes a folder or a file as returned by Get Folder Items and others
type Item struct {
	Type              string  `json:"type"`
	ID                string  `json:"id"`
	SequenceID        string  `json:"sequence_id"`
	Etag              string  `json:"etag"`
	SHA1              string  `json:"sha1"`
	Name              string  `json:"name"`
	Size              float64 `json:"size"` // box returns this in xEyy format for very large numbers - see #2261
	CreatedAt         Time    `json:"created_at"`
	ModifiedAt        Time    `json:"modified_at"`
	ContentCreatedAt  Time    `json:"content_created_at"`
	ContentModifiedAt Time    `json:"content_modified_at"`
	ItemStatus        string  `json:"item_status"` // active, trashed if the file has been moved to the trash, and deleted if the file has been permanently deleted
	SharedLink        struct {
		URL    string `json:"url,omitempty"`
		Access string `json:"access,omitempty"`
	} `json:"shared_link"`
}

// ModTime returns the modification time of the item
func (i *Item) ModTime() (t time.Time) {
	t = time.Time(i.ContentModifiedAt)
	if t.IsZero() {
		t = time.Time(i.ModifiedAt)
	}
	return t
}

// FolderItems is returned from the GetFolderItems call
type FolderItems struct {
	TotalCount int    `json:"total_count"`
	Entries    []Item `json:"entries"`
	Offset     int    `json:"offset"`
	Limit      int    `json:"limit"`
	Order      []struct {
		By        string `json:"by"`
		Direction string `json:"direction"`
	} `json:"order"`
}

// Parent defined the ID of the parent directory
type Parent struct {
	ID string `json:"id"`
}

// CreateFolder is the request for Create Folder
type CreateFolder struct {
	Name   string `json:"name"`
	Parent Parent `json:"parent"`
}

// UploadFile is the request for Upload File
type UploadFile struct {
	Name              string `json:"name"`
	Parent            Parent `json:"parent"`
	ContentCreatedAt  Time   `json:"content_created_at"`
	ContentModifiedAt Time   `json:"content_modified_at"`
}

// PreUploadCheck is the request for upload preflight check
type PreUploadCheck struct {
	Name   string `json:"name"`
	Parent Parent `json:"parent"`
	Size   *int64 `json:"size,omitempty"`
}

// PreUploadCheckResponse is the response from upload preflight check
// if successful
type PreUploadCheckResponse struct {
	UploadToken string `json:"upload_token"`
	UploadURL   string `json:"upload_url"`
}

// PreUploadCheckConflict is returned in the ContextInfo error field
// from PreUploadCheck when the error code is "item_name_in_use"
type PreUploadCheckConflict struct {
	Conflicts struct {
		Type        string `json:"type"`
		ID          string `json:"id"`
		FileVersion struct {
			Type string `json:"type"`
			ID   string `json:"id"`
			Sha1 string `json:"sha1"`
		} `json:"file_version"`
		SequenceID string `json:"sequence_id"`
		Etag       string `json:"etag"`
		Sha1       string `json:"sha1"`
		Name       string `json:"name"`
	} `json:"conflicts"`
}

// UpdateFileModTime is used in Update File Info
type UpdateFileModTime struct {
	ContentModifiedAt Time `json:"content_modified_at"`
}

// UpdateFileMove is the request for Upload File to change name and parent
type UpdateFileMove struct {
	Name   string `json:"name"`
	Parent Parent `json:"parent"`
}

// CopyFile is the request for Copy File
type CopyFile struct {
	Name   string `json:"name"`
	Parent Parent `json:"parent"`
}

// CreateSharedLink is the request for Public Link
type CreateSharedLink struct {
	SharedLink struct {
		URL    string `json:"url,omitempty"`
		Access string `json:"access,omitempty"`
	} `json:"shared_link"`
}

// UploadSessionRequest is uses in Create Upload Session
type UploadSessionRequest struct {
	FolderID string `json:"folder_id,omitempty"` // don't pass for update
	FileSize int64  `json:"file_size"`
	FileName string `json:"file_name,omitempty"` // optional for update
}

// UploadSessionResponse is returned from Create Upload Session
type UploadSessionResponse struct {
	TotalParts       int   `json:"total_parts"`
	PartSize         int64 `json:"part_size"`
	SessionEndpoints struct {
		ListParts  string `json:"list_parts"`
		Commit     string `json:"commit"`
		UploadPart string `json:"upload_part"`
		Status     string `json:"status"`
		Abort      string `json:"abort"`
	} `json:"session_endpoints"`
	SessionExpiresAt  Time   `json:"session_expires_at"`
	ID                string `json:"id"`
	Type              string `json:"type"`
	NumPartsProcessed int    `json:"num_parts_processed"`
}

// Part defines the return from upload part call which are passed to commit upload also
type Part struct {
	PartID string `json:"part_id"`
	Offset int64  `json:"offset"`
	Size   int64  `json:"size"`
	Sha1   string `json:"sha1"`
}

// UploadPartResponse is returned from the upload part call
type UploadPartResponse struct {
	Part Part `json:"part"`
}

// CommitUpload is used in the Commit Upload call
type CommitUpload struct {
	Parts      []Part `json:"parts"`
	Attributes struct {
		ContentCreatedAt  Time `json:"content_created_at"`
		ContentModifiedAt Time `json:"content_modified_at"`
	} `json:"attributes"`
}

// ConfigJSON defines the shape of a box config.json
type ConfigJSON struct {
	BoxAppSettings AppSettings `json:"boxAppSettings"`
	EnterpriseID   string      `json:"enterpriseID"`
}

// AppSettings defines the shape of the boxAppSettings within box config.json
type AppSettings struct {
	ClientID     string  `json:"clientID"`
	ClientSecret string  `json:"clientSecret"`
	AppAuth      AppAuth `json:"appAuth"`
}

// AppAuth defines the shape of the appAuth within boxAppSettings in config.json
type AppAuth struct {
	PublicKeyID string `json:"publicKeyID"`
	PrivateKey  string `json:"privateKey"`
	Passphrase  string `json:"passphrase"`
}

// User is returned from /users/me
type User struct {
	Type          string    `json:"type"`
	ID            string    `json:"id"`
	Name          string    `json:"name"`
	Login         string    `json:"login"`
	CreatedAt     time.Time `json:"created_at"`
	ModifiedAt    time.Time `json:"modified_at"`
	Language      string    `json:"language"`
	Timezone      string    `json:"timezone"`
	SpaceAmount   int64     `json:"space_amount"`
	SpaceUsed     int64     `json:"space_used"`
	MaxUploadSize int64     `json:"max_upload_size"`
	Status        string    `json:"status"`
	JobTitle      string    `json:"job_title"`
	Phone         string    `json:"phone"`
	Address       string    `json:"address"`
	AvatarURL     string    `json:"avatar_url"`
}
