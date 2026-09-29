// Package api provides the JSON types used by the WebHDFS REST API.
//
// See: https://hadoop.apache.org/docs/r1.0.4/webhdfs.html
package api

import (
	"encoding/json"
	"fmt"
	"time"
)

// FileType enumerates the type of a FileStatus entry
const (
	FileTypeFile      = "FILE"
	FileTypeDirectory = "DIRECTORY"
)

// FileStatus is the JSON object returned for GETFILESTATUS and as an
// element of FileStatuses for LISTSTATUS
type FileStatus struct {
	AccessTime       int64  `json:"accessTime"`
	BlockSize        int64  `json:"blockSize"`
	Group            string `json:"group"`
	Length           int64  `json:"length"`
	ModificationTime int64  `json:"modificationTime"`
	Owner            string `json:"owner"`
	PathSuffix       string `json:"pathSuffix"`
	Permission       string `json:"permission"`
	Replication      int    `json:"replication"`
	Type             string `json:"type"` // enum {FILE, DIRECTORY}
}

// IsDir returns true if the FileStatus is a directory
func (fi *FileStatus) IsDir() bool {
	return fi.Type == FileTypeDirectory
}

// ModTime returns the modification time of the FileStatus
func (fi *FileStatus) ModTime() time.Time {
	return time.UnixMilli(fi.ModificationTime)
}

// AccessModTime returns the access time of the FileStatus
func (fi *FileStatus) AccessModTime() time.Time {
	return time.UnixMilli(fi.AccessTime)
}

// FileStatusResponse is the response to GETFILESTATUS
type FileStatusResponse struct {
	FileStatus FileStatus `json:"FileStatus"`
}

// FileStatusesResponse is the response to LISTSTATUS
type FileStatusesResponse struct {
	FileStatuses struct {
		FileStatus []FileStatus `json:"FileStatus"`
	} `json:"FileStatuses"`
}

// BooleanResponse is the response to MKDIRS, RENAME, DELETE and SETREPLICATION
type BooleanResponse struct {
	Boolean bool `json:"boolean"`
}

// ContentSummaryResponse is the response to GETCONTENTSUMMARY
type ContentSummaryResponse struct {
	ContentSummary struct {
		DirectoryCount int64 `json:"directoryCount"`
		FileCount      int64 `json:"fileCount"`
		Length         int64 `json:"length"`
		Quota          int64 `json:"quota"`
		SpaceConsumed  int64 `json:"spaceConsumed"`
		SpaceQuota     int64 `json:"spaceQuota"`
	} `json:"ContentSummary"`
}

// remoteException is the JSON body of an error response
type remoteException struct {
	RemoteException struct {
		Exception     string `json:"exception"`
		JavaClassName string `json:"javaClassName"`
		Message       string `json:"message"`
	} `json:"RemoteException"`
}

// Error is returned for a non 2xx response from the WebHDFS API
type Error struct {
	StatusCode    int
	Exception     string
	JavaClassName string
	Message       string
}

// NewError builds an *Error from a non 2xx HTTP status code and its
// response body, decoding a RemoteException JSON body if present
func NewError(statusCode int, body []byte) *Error {
	var re remoteException
	if err := json.Unmarshal(body, &re); err == nil && re.RemoteException.Exception != "" {
		return &Error{
			StatusCode:    statusCode,
			Exception:     re.RemoteException.Exception,
			JavaClassName: re.RemoteException.JavaClassName,
			Message:       re.RemoteException.Message,
		}
	}
	return &Error{StatusCode: statusCode, Message: string(body)}
}

// Error satisfies the error interface
func (e *Error) Error() string {
	if e.Exception == "" {
		return fmt.Sprintf("webhdfs: HTTP error %d: %s", e.StatusCode, e.Message)
	}
	return fmt.Sprintf("webhdfs: %s: %s (HTTP %d)", e.Exception, e.Message, e.StatusCode)
}

// IsFileNotFound returns true if this error represents a FileNotFoundException
func (e *Error) IsFileNotFound() bool {
	return e != nil && e.Exception == "FileNotFoundException"
}
