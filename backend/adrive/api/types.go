// Package api has type definitions for Aliyun Drive
//
// Converted from the API docs with help from https://mholt.github.io/json-to-go/
package api

import (
	"fmt"
	"time"
)

const (
	// 2017-05-03T07:26:10-07:00
	timeFormat = `"` + time.RFC3339 + `"`
)

// Time represents date and time information for the
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

// Types of things in Item/ItemMini
const (
	ItemTypeFolder = "folder"
	ItemTypeFile   = "file"
)

// UserInfo represents the user information returned by the API
type UserInfo struct {
	UserName            string `json:"user_name"`
	NickName            string `json:"nick_name"`
	Nickname            string `json:"nickname"`
	FileDriveID         string `json:"default_drive_id"`
	UsedSize            uint64 `json:"used_size"`
	TotalSize           uint64 `json:"total_size"`
	Email               string `json:"email"`
	Phone               string `json:"phone"`
	Role                string `json:"role"`
	Status              string `json:"status"`
	ThirdPartyVip       bool   `json:"third_party_vip"`
	ThirdPartyVipExpire string `json:"third_party_vip_expire"`
}

// SpaceInfo represents the space information returned by the API
type SpaceInfo struct {
	PersonalSpaceInfo struct {
		UsedSize  int64 `json:"used_size"`
		TotalSize int64 `json:"total_size"`
	} `json:"personal_space_info"`
}

// DriveInfo represents the drive information returned by the API
type DriveInfo struct {
	DefaultDriveID string `json:"default_drive_id"`
}

// FileEntity represents a file or folder entity
type FileEntity struct {
	DriveID         string `json:"drive_id"`
	FileID          string `json:"file_id"`
	ParentFileID    string `json:"parent_file_id"`
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
	DriveID        string `json:"drive_id"`
	ParentFileID   string `json:"parent_file_id"`
	Limit          int    `json:"limit,omitempty"`
	Marker         string `json:"marker,omitempty"`
	OrderBy        string `json:"order_by,omitempty"`
	OrderDirection string `json:"order_direction,omitempty"`
}

// FileBatchActionParam contains parameters for file actions like delete
type FileBatchActionParam struct {
	DriveID string `json:"drive_id"`
	FileID  string `json:"file_id"`
}

// FileCopyParam contains parameters for copying a file
type FileCopyParam struct {
	DriveID        string `json:"drive_id"`
	FileID         string `json:"file_id"`
	ToParentFileID string `json:"to_parent_file_id"`
}

// FileMoveParam contains parameters for moving a file
type FileMoveParam struct {
	DriveID        string `json:"drive_id"`
	FileID         string `json:"file_id"`
	ToParentFileID string `json:"to_parent_file_id"`
}

// FileActionResponse is the response from file actions
type FileActionResponse struct {
	FileID string `json:"file_id"`
}

// GetFileDownloadURLParam contains parameters for getting download URL
type GetFileDownloadURLParam struct {
	DriveID string `json:"drive_id"`
	FileID  string `json:"file_id"`
}

// DownloadURLResponse is the response when getting a download URL
type DownloadURLResponse struct {
	URL string `json:"url"`
}

// PartInfo contains information about a part of a file
type PartInfo struct {
	PartNumber int    `json:"part_number"`
	UploadURL  string `json:"upload_url"`
	PartSize   int64  `json:"part_size"`
}

// FileUploadCreateParam contains parameters for creating a file
type FileUploadCreateParam struct {
	DriveID         string     `json:"drive_id"`
	ParentFileID    string     `json:"parent_file_id"`
	Name            string     `json:"name"`
	Type            string     `json:"type"`
	CheckNameMode   string     `json:"check_name_mode"`
	Size            int64      `json:"size"`
	PartInfoList    []PartInfo `json:"part_info_list"`
	ContentHash     string     `json:"content_hash"`
	ContentHashName string     `json:"content_hash_name"`
	ProofCode       string     `json:"proof_code"`
	ProofVersion    string     `json:"proof_version"`
	LocalCreatedAt  string     `json:"local_created_at"`
	LocalModifiedAt string     `json:"local_modified_at"`
}

// FileUploadCreateResponse contains the result of creating a file
type FileUploadCreateResponse struct {
	DriveID      string     `json:"drive_id"`
	ParentFileID string     `json:"parent_file_id"`
	FileID       string     `json:"file_id"`
	FileName     string     `json:"file_name"`
	Status       string     `json:"status"`
	UploadID     string     `json:"upload_id"`
	Available    bool       `json:"available"`
	Exist        bool       `json:"exist"`
	RapidUpload  bool       `json:"rapid_upload"`
	PartInfoList []PartInfo `json:"part_info_list"`
}

// FileUploadGetUploadURLParam Get upload url param
type FileUploadGetUploadURLParam struct {
	DriveID      string     `json:"drive_id"`
	FileID       string     `json:"file_id"`
	UploadID     string     `json:"upload_id"`
	PartInfoList []PartInfo `json:"part_info_list"`
}

// FileUploadGetUploadURLResponse Get upload url response
type FileUploadGetUploadURLResponse struct {
	DriveID      string     `json:"drive_id"`
	FileID       string     `json:"file_id"`
	UploadID     string     `json:"upload_id"`
	CreatedAt    string     `json:"created_at"`
	PartInfoList []PartInfo `json:"part_info_list"`
}

// FileUploadCompleteParam Upload complete param
type FileUploadCompleteParam struct {
	DriveID  string `json:"drive_id"`
	FileID   string `json:"file_id"`
	UploadID string `json:"upload_id"`
}

// FileUploadCompleteResponse Upload complete response
type FileUploadCompleteResponse struct {
	DriveID         string `json:"drive_id"`
	ParentFileID    string `json:"parent_file_id"`
	FileID          string `json:"file_id"`
	Name            string `json:"name"`
	Type            string `json:"type"`
	Size            int64  `json:"size"`
	Category        string `json:"category"`
	FileExtension   string `json:"file_extension"`
	ContentHash     string `json:"content_hash"`
	ContentHashName string `json:"content_hash_name"`
	LocalCreatedAt  string `json:"local_created_at"`
	LocalModifiedAt string `json:"local_modified_at"`
}
