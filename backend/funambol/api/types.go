// Package api has type definitions for funambol
//
// The o2 Cloud service (cloud.o2online.es) is built on the Funambol /
// OneMediaHub "SAPI" (Server API).  All calls live under /sapi, return a
// JSON envelope and authenticate with a session cookie plus a rolling
// "validationkey" query parameter.
package api

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// TimeFormat is the layout SAPI expects for upload metadata timestamps,
// e.g. "2026-06-19 14:33:00" (UTC, second precision).
const TimeFormat = "2006-01-02 15:04:05"

// ID is a SAPI identifier.  Folders are returned as JSON numbers while media
// items are returned as JSON strings, so we accept both and normalise to a
// string.
type ID string

// UnmarshalJSON accepts either a quoted string or a bare number.
func (i *ID) UnmarshalJSON(data []byte) error {
	s := strings.Trim(string(data), `"`)
	if s == "null" {
		s = ""
	}
	*i = ID(s)
	return nil
}

// String returns the id as a string
func (i ID) String() string { return string(i) }

// Error is the SAPI error object
type Error struct {
	Code       string `json:"code"`
	Message    string `json:"message"`
	Cause      string `json:"cause"`
	Data       string `json:"data"`
	Parameters []any  `json:"parameters"`
}

// Error satisfies the error interface
func (e *Error) Error() string {
	if e.Cause != "" {
		return fmt.Sprintf("%s (%s): %s", e.Message, e.Code, e.Cause)
	}
	return fmt.Sprintf("%s (%s)", e.Message, e.Code)
}

// Status is the common envelope tail shared by every SAPI response.
type Status struct {
	Err          *Error `json:"error"`
	ResponseTime int64  `json:"responsetime"`
	Success      string `json:"success"`
}

// AsErr returns the embedded error or nil.  SAPI signals most failures with
// HTTP 200 and an error object, so this must be checked on every response.
func (s *Status) AsErr() error {
	if s.Err != nil && s.Err.Code != "" {
		return s.Err
	}
	return nil
}

// Folder is a directory
type Folder struct {
	ID   ID     `json:"id"`
	Name string `json:"name"`
	Date int64  `json:"date"` // epoch millis
}

// Item is a media item (a file in rclone terms)
type Item struct {
	ID               ID          `json:"id"`
	Name             string      `json:"name"`
	Size             int64       `json:"size"`
	MediaType        string      `json:"mediatype"` // picture|video|audio|file
	Status           string      `json:"status"`
	FolderID         ID          `json:"folderid"`
	Date             int64       `json:"date"`             // upload date, epoch millis
	CreationDate     int64       `json:"creationdate"`     // epoch millis
	ModificationDate int64       `json:"modificationdate"` // epoch millis
	Etag             string      `json:"etag"`             // base64 of the MD5 digest
	URL              string      `json:"url"`              // absolute download URL
	Favorite         bool        `json:"favorite"`
	Thumbnails       []Thumbnail `json:"thumbnails"`
}

// Thumbnail is a single thumbnail rendition
type Thumbnail struct {
	URL    string `json:"url"`
	Width  int    `json:"w"`
	Height int    `json:"h"`
	Size   int    `json:"size"`
	Etag   string `json:"etag"`
}

// ModTime returns the best modification time available for the item.
func (i *Item) ModTime() time.Time {
	ms := i.ModificationDate
	if ms == 0 {
		ms = i.Date
	}
	if ms == 0 {
		ms = i.CreationDate
	}
	return time.UnixMilli(ms)
}

// ---- response envelopes ----

// FoldersResponse is returned by folder list / root
type FoldersResponse struct {
	Status
	Data struct {
		Folders []Folder `json:"folders"`
	} `json:"data"`
}

// MediaResponse is returned by /sapi/media?action=get
type MediaResponse struct {
	Status
	Data struct {
		Media []Item `json:"media"`
		More  bool   `json:"more"`
	} `json:"data"`
}

// SaveFolderResponse is returned by /sapi/media/folder?action=save
type SaveFolderResponse struct {
	Status
	ID   ID `json:"id"`
	Data struct {
		Folder Folder `json:"folder"`
	} `json:"data"`
}

// NewID returns the id of the created/updated folder
func (r *SaveFolderResponse) NewID() ID {
	if r.Data.Folder.ID != "" {
		return r.Data.Folder.ID
	}
	return r.ID
}

// UploadMetadataResponse is returned by /sapi/upload/{type}?action=save-metadata.
// Note: this envelope is NOT wrapped in "data".
type UploadMetadataResponse struct {
	Status
	ID ID `json:"id"` // GUID to send in the bytes request
}

// UploadResponse is returned by /sapi/upload/{type}?action=save
type UploadResponse struct {
	Status
	ID     ID     `json:"id"`
	State  string `json:"status"`
	Etag   string `json:"etag"`
	Update int64  `json:"lastupdate"`
}

// StorageSpace is returned by /sapi/media?action=get-storage-space
type StorageSpace struct {
	Status
	Data struct {
		Quota   int64 `json:"quota"`
		Used    int64 `json:"used"`
		Free    int64 `json:"free"`
		NoLimit bool  `json:"nolimit"`
	} `json:"data"`
}

// ---- request bodies (all wrapped in {"data": …}) ----

// SaveFolderRequest creates / renames / moves a folder.  Parent is a *ID so it
// can be omitted on rename.
type SaveFolderRequest struct {
	Data struct {
		ID     ID     `json:"id,omitempty"`
		Name   string `json:"name,omitempty"`
		Parent *ID    `json:"parentid,omitempty"`
	} `json:"data"`
}

// IDsRequest carries a list of media ids (used by media delete)
type IDsRequest struct {
	Data struct {
		IDs []string `json:"ids"`
	} `json:"data"`
}

// FoldersRequest carries a list of folder ids (folder delete)
type FoldersRequest struct {
	Data struct {
		Folders []string `json:"folders"`
	} `json:"data"`
}

// FieldsRequest selects which item fields the media listing returns
type FieldsRequest struct {
	Data struct {
		Fields []string `json:"fields"`
	} `json:"data"`
}

// UploadMetadataRequest is the first phase of an upload
type UploadMetadataRequest struct {
	Data struct {
		Name             string `json:"name"`
		ContentType      string `json:"contenttype"`
		Size             int64  `json:"size"`
		FolderID         string `json:"folderid"`
		CreationDate     string `json:"creationdate,omitempty"`
		ModificationDate string `json:"modificationdate,omitempty"`
	} `json:"data"`
}

// FormatTime renders t in the layout SAPI expects (UTC).
func FormatTime(t time.Time) string {
	return t.UTC().Format(TimeFormat)
}

// ParseID converts a numeric string id, returning 0 on failure (used only for
// debug / sorting, never for correctness).
func ParseID(id ID) int64 {
	n, _ := strconv.ParseInt(string(id), 10, 64)
	return n
}
