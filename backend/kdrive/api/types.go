// Package api has type definitions for kDrive
//
// Converted from the API docs with help from https://mholt.github.io/json-to-go/
package api

import (
	"fmt"
	"strconv"
	"time"
)

const (
	// Sun, 16 Mar 2014 17:26:04 +0000
	timeFormat = `"` + time.RFC1123Z + `"`
)

// Time represents date and time information for the
// kdrive API, by using RFC1123Z
type Time time.Time

// MarshalJSON turns a Time into JSON (in UTC)
func (t *Time) MarshalJSON() (out []byte, err error) {
	timeString := (*time.Time)(t).Format(timeFormat)
	return []byte(timeString), nil
}

// UnmarshalJSON turns JSON into a Time
func (t *Time) UnmarshalJSON(data []byte) error {
	if string(data) == "null" {
		return nil
	}

	timestamp, err := strconv.ParseInt(string(data), 10, 64)
	if err != nil {
		return err
	}
	newT := time.Unix(timestamp, 0)
	*t = Time(newT)
	return nil
}

// ResultStatus return error details from kdrive when things go wrong
//
// If result is 0 then everything is OK
type ResultStatus struct {
	Status      string `json:"result"`
	ErrorDetail struct {
		Result      string `json:"code"`
		ErrorString string `json:"description"`
		Errors      []struct {
			Result      string `json:"code"`
			ErrorString string `json:"description"`
		} `json:"errors"`
	} `json:"error"`
}

// Error returns a string for the error and satisfies the error interface
func (e *ResultStatus) Error() string {
	var details string
	for i := range e.ErrorDetail.Errors {
		details += "|" + e.ErrorDetail.Errors[i].Result
	}

	return fmt.Sprintf("kDrive error: %s (%s: %s %s)", e.Status, e.ErrorDetail.Result, e.ErrorDetail.ErrorString, details)
}

// IsError returns true if there is an error
func (e *ResultStatus) IsError() bool {
	return e.Status != "success"
}

// Update returns err directly if it was != nil, otherwise it returns
// an Error or nil if no error was detected
func (e *ResultStatus) Update(err error) error {
	if err != nil {
		return err
	}
	if e.IsError() {
		return e
	}
	return nil
}

// Check ResultStatus satisfies the error interface
var _ error = (*ResultStatus)(nil)

// Item describes a folder or a file as returned by Get Folder Items and others
type Item struct {
	ID             int    `json:"id"`
	Name           string `json:"name"`
	Type           string `json:"type"`
	FullPath       string `json:"path"`
	Status         string `json:"status"`
	Hash           string `json:"hash"`
	Size           int64  `json:"size"`
	Visibility     string `json:"visibility"`
	DriveID        int    `json:"drive_id"`
	Depth          int    `json:"depth"`
	CreatedBy      int    `json:"created_by"`
	CreatedAt      Time   `json:"created_at"`
	AddedAt        int    `json:"added_at"`
	LastModifiedAt Time   `json:"last_modified_at"`
	LastModifiedBy int    `json:"last_modified_by"`
	RevisedAt      int    `json:"revised_at"`
	UpdatedAt      int    `json:"updated_at"`
	MimeType       string `json:"mime_type"`
	ParentID       int    `json:"parent_id"`
	Color          string `json:"color"`
}

// SearchResult is returned when a list of items is requested
type SearchResult struct {
	ResultStatus
	Data       []Item `json:"data"`
	Cursor     string `json:"cursor"`
	HasMore    bool   `json:"has_more"`
	ResponseAt int    `json:"response_at"`
}

// ModTime returns the modification time of the item
func (i *Item) ModTime() (t time.Time) {
	t = time.Time(i.LastModifiedAt)
	if t.IsZero() {
		t = time.Time(i.CreatedAt)
	}
	return t
}

// CancelResource is a kdrive resource that can be cancelled after some action
type CancelResource struct {
	CancelID   string `json:"cancel_id"`
	ValidUntil int    `json:"valid_until"`
}

// CancellableResponse is returned from kdrive when an action can be cancelled afterwards
type CancellableResponse struct {
	ResultStatus
	Data CancelResource `json:"data"`
}

// CreateDirResult is returned from kdrive after a call to MkDir
type CreateDirResult struct {
	ResultStatus
	Data Item `json:"data"`
}

// FileCopyResponse is returned from kdrive after a call to Copy
type FileCopyResponse struct {
	ResultStatus
	Data Item `json:"data"`
}

// UploadFileResponse is returned from kdrive after a call to Upload
type UploadFileResponse struct {
	ResultStatus
	Data Item `json:"data"`
}

// ChecksumFileResult is returned from kdrive after a call to Hash
type ChecksumFileResult struct {
	ResultStatus
	Data struct {
		Hash string `json:"hash"`
	} `json:"data"`
}

// PubLinkResult is currently unused, as PublicLink is disabled
type PubLinkResult struct {
	ResultStatus
	Data struct {
		URL          string `json:"url"`
		FileID       int    `json:"file_id"`
		Right        string `json:"right"`
		ValidUntil   int    `json:"valid_until"`
		CreatedBy    int    `json:"created_by"`
		CreatedAt    int    `json:"created_at"`
		UpdatedAt    int    `json:"updated_at"`
		Capabilities struct {
			CanEdit          bool `json:"can_edit"`
			CanSeeStats      bool `json:"can_see_stats"`
			CanSeeInfo       bool `json:"can_see_info"`
			CanDownload      bool `json:"can_download"`
			CanComment       bool `json:"can_comment"`
			CanRequestAccess bool `json:"can_request_access"`
		} `json:"capabilities"`
		AccessBlocked bool `json:"access_blocked"`
	} `json:"data"`
}

// QuotaInfo is return from kdrive after a call get drive info
type QuotaInfo struct {
	ResultStatus
	Data struct {
		Size     int64 `json:"size"`
		UsedSize int64 `json:"used_size"`
	} `json:"data"`
}

type SessionStartResponse struct {
	Result string `json:"result"`
	Data   struct {
		Token     string `json:"token"`
		UploadURL string `json:"upload_url"` // URL pour uploader les chunks
	} `json:"data"`
}

type ChunkUploadResponse struct {
	Result string `json:"result"`
	Data   struct {
		ReceivedBytes int64  `json:"received_bytes"`
		Hash          string `json:"hash,omitempty"` // Only present when requested with with:'hash'
	} `json:"data"`
}

type SessionFinishResponse struct {
	Result string `json:"result"`
	Data   struct {
		Token   string `json:"token"`
		File    Item   `json:"file"`
		Result  bool   `json:"result"`
		Message string `json:"message"`
	} `json:"data"`
}

type SessionCancelResponse struct {
	Result string `json:"result"`
	Data   Item   `json:"data"`
}
