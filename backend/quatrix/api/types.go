// Package api provides types used by the Quatrix API.
package api

import (
	"strconv"
	"time"
)

// OverwriteMode is a conflict resolve mode during copy or move. Files with conflicting names will be overwritten
const OverwriteMode = "overwrite"

// ProfileInfo is a profile info about quota
type ProfileInfo struct {
	UserUsed  int64 `json:"user_used"`
	UserLimit int64 `json:"user_limit"`
	AccUsed   int64 `json:"acc_used"`
	AccLimit  int64 `json:"acc_limit"`
}

// IDList is a general object that contains list of ids
type IDList struct {
	IDs []string `json:"ids"`
}

// DeleteParams is the request to delete object
type DeleteParams struct {
	IDs               []string `json:"ids"`
	DeletePermanently bool     `json:"delete_permanently"`
}

// FileInfoParams is the request to get object's (file or directory) info
type FileInfoParams struct {
	ParentID string `json:"parent_id,omitempty"`
	Path     string `json:"path"`
}

// FileInfo is the response to get object's (file or directory) info
type FileInfo struct {
	FileID   string `json:"file_id"`
	ParentID string `json:"parent_id"`
	Src      string `json:"src"`
	Type     string `json:"type"`
}

// IsFile returns true if object is a file
// false otherwise
func (fi *FileInfo) IsFile() bool {
	if fi == nil {
		return false
	}

	return fi.Type == "F"
}

// IsDir returns true if object is a directory
// false otherwise
func (fi *FileInfo) IsDir() bool {
	if fi == nil {
		return false
	}

	return fi.Type == "D" || fi.Type == "S" || fi.Type == "T"
}

// CreateDirParams is the request to create a directory
type CreateDirParams struct {
	Target  string `json:"target,omitempty"`
	Name    string `json:"name"`
	Resolve bool   `json:"resolve"`
}

// File represent metadata about object in Quatrix (file or directory)
type File struct {
	ID         string   `json:"id"`
	Created    JSONTime `json:"created"`
	Modified   JSONTime `json:"modified"`
	Name       string   `json:"name"`
	ParentID   string   `json:"parent_id"`
	Size       int64    `json:"size"`
	ModifiedMS JSONTime `json:"modified_ms"`
	Type       string   `json:"type"`
	Operations int      `json:"operations"`
	SubType    string   `json:"sub_type"`
	Content    []File   `json:"content"`
}

// IsFile returns true if object is a file
// false otherwise
func (f *File) IsFile() bool {
	if f == nil {
		return false
	}

	return f.Type == "F"
}

// IsDir returns true if object is a directory
// false otherwise
func (f *File) IsDir() bool {
	if f == nil {
		return false
	}

	return f.Type == "D" || f.Type == "S" || f.Type == "T"
}

// IsProjectFolder returns true if object is a project folder
// false otherwise
func (f *File) IsProjectFolder() bool {
	if f == nil {
		return false
	}

	return f.Type == "S"
}

// SetMTimeParams is the request to set modification time for object
type SetMTimeParams struct {
	ID    string   `json:"id,omitempty"`
	MTime JSONTime `json:"mtime"`
}

// JSONTime provides methods to marshal/unmarshal time.Time as Unix time
type JSONTime time.Time

// MarshalJSON returns time representation in Unix time
func (u JSONTime) MarshalJSON() ([]byte, error) {
	return []byte(strconv.FormatFloat(float64(time.Time(u).UTC().UnixNano())/1e9, 'f', 6, 64)), nil
}

// UnmarshalJSON sets time from Unix time representation
func (u *JSONTime) UnmarshalJSON(data []byte) error {
	f, err := strconv.ParseFloat(string(data), 64)
	if err != nil {
		return err
	}

	t := JSONTime(time.Unix(0, int64(f*1e9)))
	*u = t

	return nil
}

// String returns Unix time representation of time as string
func (u JSONTime) String() string {
	return strconv.FormatInt(time.Time(u).UTC().Unix(), 10)
}

// DownloadLinkResponse is the response to download-link request
type DownloadLinkResponse struct {
	ID string `json:"id"`
}

// UploadLinkParams is the request to get upload-link
type UploadLinkParams struct {
	Name     string `json:"name"`
	ParentID string `json:"parent_id"`
	Resolve  bool   `json:"resolve"`
}

// UploadLinkResponse is the response to upload-link request
type UploadLinkResponse struct {
	Name      string `json:"name"`
	FileID    string `json:"file_id"`
	ParentID  string `json:"parent_id"`
	UploadKey string `json:"upload_key"`
}

// UploadFinalizeResponse is the response to finalize file method
type UploadFinalizeResponse struct {
	FileID   string `json:"id"`
	ParentID string `json:"parent_id"`
	Modified int64  `json:"modified"`
	FileSize int64  `json:"size"`
}

// FileModifyParams is the request to get modify file link
type FileModifyParams struct {
	ID       string `json:"id"`
	Truncate int64  `json:"truncate"`
}

// FileCopyMoveOneParams is the request to do server-side copy and move
// can be used for file or directory
type FileCopyMoveOneParams struct {
	ID          string   `json:"file_id"`
	Target      string   `json:"target_id"`
	Name        string   `json:"name"`
	MTime       JSONTime `json:"mtime"`
	Resolve     bool     `json:"resolve"`
	ResolveMode string   `json:"resolve_mode"`
}
