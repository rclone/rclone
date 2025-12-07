// Package api has type definitions for 123Pan
package api

import (
	"fmt"
	"time"
)

const (
	// TimeFormat is the time format used by 123Pan API (UTC+8)
	TimeFormat = "2006-01-02 15:04:05"
)

// Time represents date and time information for the 123Pan API
// The API returns time in UTC+8 timezone without timezone info
type Time time.Time

// MarshalJSON turns a Time into JSON
func (t *Time) MarshalJSON() (out []byte, err error) {
	loc := time.FixedZone("UTC+8", 8*60*60)
	timeString := (*time.Time)(t).In(loc).Format(`"` + TimeFormat + `"`)
	return []byte(timeString), nil
}

// UnmarshalJSON turns JSON into a Time
func (t *Time) UnmarshalJSON(data []byte) error {
	if string(data) == "null" || string(data) == `""` {
		return nil
	}
	loc := time.FixedZone("UTC+8", 8*60*60)
	newT, err := time.ParseInLocation(`"`+TimeFormat+`"`, string(data), loc)
	if err != nil {
		return err
	}
	*t = Time(newT)
	return nil
}

// Time returns the underlying time.Time
func (t Time) Time() time.Time {
	return time.Time(t)
}

// BaseResponse is the base response structure from 123Pan API
type BaseResponse struct {
	Code     int    `json:"code"`
	Message  string `json:"message"`
	XTraceID string `json:"x-traceID,omitempty"`
}

// Error returns the error message if Code is not 0
func (r *BaseResponse) Error() string {
	if r.Code == 0 {
		return ""
	}
	return fmt.Sprintf("123pan error: %s (code %d)", r.Message, r.Code)
}

// OK returns true if the response was successful
func (r *BaseResponse) OK() bool {
	return r.Code == 0
}

// File represents a file or folder in 123Pan
type File struct {
	FileName     string `json:"filename"`
	Size         int64  `json:"size"`
	CreateAt     Time   `json:"createAt"`
	UpdateAt     Time   `json:"updateAt"`
	FileID       int64  `json:"fileId"`
	Type         int    `json:"type"` // 0=file, 1=folder
	Etag         string `json:"etag"`
	S3KeyFlag    string `json:"s3KeyFlag,omitempty"`
	ParentFileID int64  `json:"parentFileId"`
	Category     int    `json:"category,omitempty"`
	Status       int    `json:"status,omitempty"`
	Trashed      int    `json:"trashed"`
}

// IsDir returns true if the file is a directory
func (f *File) IsDir() bool {
	return f.Type == 1
}

// ModTime returns the modification time
func (f *File) ModTime() time.Time {
	return f.UpdateAt.Time()
}

// AccessTokenRequest is the request for getting access token
type AccessTokenRequest struct {
	ClientID     string `json:"clientID"`
	ClientSecret string `json:"clientSecret"`
}

// AccessTokenResponse is the response from /api/v1/access_token
type AccessTokenResponse struct {
	BaseResponse
	Data struct {
		AccessToken string `json:"accessToken"`
		ExpiredAt   string `json:"expiredAt"`
	} `json:"data"`
}

// RefreshTokenResponse is the response from /api/v1/oauth2/access_token
type RefreshTokenResponse struct {
	AccessToken  string `json:"access_token"`
	ExpiresIn    int    `json:"expires_in"`
	RefreshToken string `json:"refresh_token"`
	Scope        string `json:"scope"`
	TokenType    string `json:"token_type"`
}

// UserInfoResponse is the response from /api/v1/user/info
type UserInfoResponse struct {
	BaseResponse
	Data struct {
		UID            uint64 `json:"uid"`
		SpaceUsed      uint64 `json:"spaceUsed"`
		SpacePermanent uint64 `json:"spacePermanent"`
		SpaceTemp      uint64 `json:"spaceTemp"`
	} `json:"data"`
}

// FileListResponse is the response from /api/v2/file/list
type FileListResponse struct {
	BaseResponse
	Data struct {
		LastFileID int64  `json:"lastFileId"`
		FileList   []File `json:"fileList"`
	} `json:"data"`
}

// DownloadInfoResponse is the response from /api/v1/file/download_info
type DownloadInfoResponse struct {
	BaseResponse
	Data struct {
		DownloadURL string `json:"downloadUrl"`
	} `json:"data"`
}

// UploadCreateRequest is the request for creating a file upload
type UploadCreateRequest struct {
	ParentFileID int64  `json:"parentFileId"`
	Filename     string `json:"filename"`
	Etag         string `json:"etag"`
	Size         int64  `json:"size"`
	Duplicate    int    `json:"duplicate"` // 2=overwrite
	ContainDir   bool   `json:"containDir"`
}

// UploadCreateResponse is the response from /upload/v2/file/create
type UploadCreateResponse struct {
	BaseResponse
	Data struct {
		FileID      int64    `json:"fileID"`
		PreuploadID string   `json:"preuploadID"`
		Reuse       bool     `json:"reuse"`
		SliceSize   int64    `json:"sliceSize"`
		Servers     []string `json:"servers"`
	} `json:"data"`
}

// UploadCompleteRequest is the request for completing an upload
type UploadCompleteRequest struct {
	PreuploadID string `json:"preuploadID"`
}

// UploadCompleteResponse is the response from /upload/v2/file/upload_complete
type UploadCompleteResponse struct {
	BaseResponse
	Data struct {
		Completed bool  `json:"completed"`
		FileID    int64 `json:"fileID"`
	} `json:"data"`
}

// MkdirRequest is the request for creating a directory
type MkdirRequest struct {
	ParentID string `json:"parentID"`
	Name     string `json:"name"`
}

// MkdirResponse is the response from /upload/v1/file/mkdir
type MkdirResponse struct {
	BaseResponse
	Data struct {
		DirID int64 `json:"dirID"`
	} `json:"data"`
}

// MoveRequest is the request for moving files
type MoveRequest struct {
	FileIDs        []int64 `json:"fileIDs"`
	ToParentFileID int64   `json:"toParentFileID"`
}

// RenameRequest is the request for renaming a file
type RenameRequest struct {
	FileID   int64  `json:"fileId"`
	FileName string `json:"fileName"`
}

// TrashRequest is the request for trashing files
type TrashRequest struct {
	FileIDs []int64 `json:"fileIDs"`
}

// DeleteRequest is the request for permanently deleting files from trash
type DeleteRequest struct {
	FileIDs []int64 `json:"fileIDs"`
}

// ShareCreateRequest is the request for creating a share link
type ShareCreateRequest struct {
	ShareName   string `json:"shareName"`
	ShareExpire int    `json:"shareExpire"` // 1, 7, 30, or 0 (permanent)
	FileIDList  string `json:"fileIDList"`  // comma-separated file IDs
	SharePwd    string `json:"sharePwd,omitempty"`
}

// ShareCreateResponse is the response from /api/v1/share/create
type ShareCreateResponse struct {
	BaseResponse
	Data struct {
		ShareID  int64  `json:"shareID"`
		ShareKey string `json:"shareKey"`
	} `json:"data"`
}
