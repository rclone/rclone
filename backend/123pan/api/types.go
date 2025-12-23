// Package api provides types for the 123pan API
package api

// Response is the interface for all API responses
type Response interface {
	IsError() bool
	GetCode() int
	GetMessage() string
}

// BaseResponse is the common response structure for all API calls
type BaseResponse struct {
	Code     int    `json:"code"`
	Message  string `json:"message"`
	XTraceID string `json:"x-traceID"`
}

// IsError returns true if the response indicates an error
func (r *BaseResponse) IsError() bool {
	return r.Code != 0
}

// GetCode returns the error code
func (r *BaseResponse) GetCode() int {
	return r.Code
}

// GetMessage returns the error message
func (r *BaseResponse) GetMessage() string {
	return r.Message
}

// AccessTokenRequest for getting access token
type AccessTokenRequest struct {
	ClientID     string `json:"clientID"`
	ClientSecret string `json:"clientSecret"`
}

// AccessTokenResponse from access token endpoint
type AccessTokenResponse struct {
	BaseResponse
	Data struct {
		AccessToken string `json:"accessToken"`
		ExpiredAt   string `json:"expiredAt"`
	} `json:"data"`
}

// VipInfo contains VIP subscription details
type VipInfo struct {
	VipLevel  int    `json:"vipLevel"` // 1=VIP, 2=SVIP, 3=长期VIP
	VipLabel  string `json:"vipLabel"` // "VIP", "SVIP", "长期VIP"
	StartTime string `json:"startTime"`
	EndTime   string `json:"endTime"`
}

// UserInfoResponse from user info endpoint
type UserInfoResponse struct {
	BaseResponse
	Data struct {
		UID            uint64    `json:"uid"`
		Nickname       string    `json:"nickname"`
		HeadImage      string    `json:"headImage"`
		Passport       string    `json:"passport"`
		Mail           string    `json:"mail"`
		SpaceUsed      int64     `json:"spaceUsed"`
		SpacePermanent int64     `json:"spacePermanent"`
		SpaceTemp      int64     `json:"spaceTemp"`
		SpaceTempExpr  int64     `json:"spaceTempExpr"` // Timestamp, can be 0
		Vip            bool      `json:"vip"`
		DirectTraffic  int64     `json:"directTraffic"`
		IsHideUID      bool      `json:"isHideUID"`
		VipInfo        []VipInfo `json:"vipInfo"`
	} `json:"data"`
}

// File represents a file or directory item from the API
type File struct {
	FileID       int64  `json:"fileId"`
	Filename     string `json:"filename"`
	Type         int    `json:"type"` // 0=file, 1=folder
	Size         int64  `json:"size"`
	Etag         string `json:"etag"` // MD5 hash
	Status       int    `json:"status"`
	ParentFileID int64  `json:"parentFileId"`
	Category     int    `json:"category"` // 0=unknown, 1=audio, 2=video, 3=image
	Trashed      int    `json:"trashed"`  // 0=no, 1=yes (in recycle bin)
	S3KeyFlag    string `json:"s3KeyFlag"`
	StorageNode  string `json:"storageNode"`
	CreateAt     string `json:"createAt"`
	UpdateAt     string `json:"updateAt"`
}

// IsDir returns true if the file is a directory
func (f *File) IsDir() bool {
	return f.Type == 1
}

// FileListResponse from file list endpoint
type FileListResponse struct {
	BaseResponse
	Data struct {
		LastFileID int64  `json:"lastFileId"` // -1 means last page
		FileList   []File `json:"fileList"`
	} `json:"data"`
}

// DownloadInfoResponse from download endpoint
type DownloadInfoResponse struct {
	BaseResponse
	Data struct {
		DownloadURL string `json:"downloadUrl"`
	} `json:"data"`
}

// MkdirRequest for creating directory
type MkdirRequest struct {
	Name     string `json:"name"`
	ParentID int64  `json:"parentID"`
}

// MkdirResponse from mkdir endpoint
type MkdirResponse struct {
	BaseResponse
	Data struct {
		DirID int64 `json:"dirID"`
	} `json:"data"`
}

// MoveRequest for moving files
type MoveRequest struct {
	FileIDs        []int64 `json:"fileIDs"`
	ToParentFileID int64   `json:"toParentFileID"`
}

// RenameRequest for renaming a file
type RenameRequest struct {
	FileID   int64  `json:"fileId"`
	FileName string `json:"fileName"`
}

// TrashRequest for deleting files to trash
type TrashRequest struct {
	FileIDs []int64 `json:"fileIDs"`
}

// DeleteRequest for permanently deleting files (must be in trash first)
type DeleteRequest struct {
	FileIDs []int64 `json:"fileIDs"`
}

// UploadCreateRequest for initiating an upload
type UploadCreateRequest struct {
	ParentFileID int64  `json:"parentFileID"`
	Filename     string `json:"filename"`
	Etag         string `json:"etag"` // File MD5
	Size         int64  `json:"size"`
	Duplicate    int    `json:"duplicate,omitempty"` // 1=keep both (rename), 2=overwrite
	ContainDir   bool   `json:"containDir,omitempty"`
}

// UploadCreateResponse from create file endpoint
type UploadCreateResponse struct {
	BaseResponse
	Data struct {
		FileID      int64    `json:"fileID"`      // Non-zero if instant upload succeeded
		PreuploadID string   `json:"preuploadID"` // ID for subsequent upload operations
		Reuse       bool     `json:"reuse"`       // True if instant upload succeeded (秒传)
		SliceSize   int64    `json:"sliceSize"`   // Required chunk size for upload
		Servers     []string `json:"servers"`     // Upload server URLs
	} `json:"data"`
}

// UploadCompleteRequest for completing an upload
type UploadCompleteRequest struct {
	PreuploadID string `json:"preuploadID"`
}

// UploadCompleteResponse from complete endpoint
type UploadCompleteResponse struct {
	BaseResponse
	Data struct {
		Completed bool  `json:"completed"`
		FileID    int64 `json:"fileID"`
	} `json:"data"`
}

// ShareCreateRequest for creating a share link
type ShareCreateRequest struct {
	ShareName   string `json:"shareName"`          // Share name/title
	ShareExpire int    `json:"shareExpire"`        // Expiry days: 1, 7, 30, or 0 (permanent)
	FileIDList  string `json:"fileIDList"`         // Comma-separated file IDs
	SharePwd    string `json:"sharePwd,omitempty"` // Optional password
}

// ShareCreateResponse from share create endpoint
type ShareCreateResponse struct {
	BaseResponse
	Data struct {
		ShareID  int64  `json:"shareID"`
		ShareKey string `json:"shareKey"` // Use with https://www.123pan.com/s/{shareKey}
	} `json:"data"`
}
