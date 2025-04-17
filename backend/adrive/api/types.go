package api

import (
	"fmt"
)

type Error struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// Error returns a string for the error and satisfies the error interface
func (e *Error) Error() string {
	out := fmt.Sprintf("Error %q (%v)", e.Message, e.Code)
	if e.Message != "" {
		out += ": " + e.Message
	}
	return out
}

// Check Error satisfies the error interface
var _ error = (*Error)(nil)

// UserInfo represents the user information returned by the API
type UserInfo struct {
	UserName            string `json:"user_name"`
	NickName            string `json:"nick_name"`
	Nickname            string `json:"nickname"`
	FileDriveId         string `json:"default_drive_id"`
	UsedSize            uint64 `json:"used_size"`
	TotalSize           uint64 `json:"total_size"`
	Email               string `json:"email"`
	Phone               string `json:"phone"`
	Role                string `json:"role"`
	Status              string `json:"status"`
	ThirdPartyVip       bool   `json:"third_party_vip"`
	ThirdPartyVipExpire string `json:"third_party_vip_expire"`
}

// FileEntity represents a file or folder entity
type FileEntity struct {
	DriveId         string `json:"drive_id"`
	FileId          string `json:"file_id"`
	ParentFileId    string `json:"parent_file_id"`
	FileName        string `json:"name"`
	FileType        string `json:"type"`
	FileSize        uint64 `json:"size"`
	UpdatedAt       string `json:"updated_at"`
	CreatedAt       string `json:"created_at"`
	ContentHash     string `json:"content_hash"`
	ContentHashName string `json:"content_hash_name"`
	Category        string `json:"category"`
}

// FileList is a list of FileEntity
type FileList []*FileEntity

// FileListResponse is the response from the API for file list
type FileListResponse struct {
	Items      FileList `json:"items"`
	NextMarker string   `json:"next_marker"`
}

// FileListParam contains parameters for listing files
type FileListParam struct {
	DriveId        string `json:"drive_id"`
	ParentFileId   string `json:"parent_file_id"`
	Limit          int    `json:"limit,omitempty"`
	Marker         string `json:"marker,omitempty"`
	OrderBy        string `json:"order_by,omitempty"`
	OrderDirection string `json:"order_direction,omitempty"`
}

// FileBatchActionParam contains parameters for file actions like delete
type FileBatchActionParam struct {
	DriveId string `json:"drive_id"`
	FileId  string `json:"file_id"`
}

// FileCopyParam contains parameters for copying a file
type FileCopyParam struct {
	DriveId        string `json:"drive_id"`
	FileId         string `json:"file_id"`
	ToParentFileId string `json:"to_parent_file_id"`
}

// FileMoveParam contains parameters for moving a file
type FileMoveParam struct {
	DriveId        string `json:"drive_id"`
	FileId         string `json:"file_id"`
	ToParentFileId string `json:"to_parent_file_id"`
}

// FileActionResponse is the response from file actions
type FileActionResponse struct {
	FileId string `json:"file_id"`
}

// GetFileDownloadUrlParam contains parameters for getting download URL
type GetFileDownloadUrlParam struct {
	DriveId string `json:"drive_id"`
	FileId  string `json:"file_id"`
}

// DownloadUrlResponse is the response when getting a download URL
type DownloadUrlResponse struct {
	Url string `json:"url"`
}
