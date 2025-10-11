// Package api provides types used by the Huawei Drive API.
package api

import (
	"time"
)

// About represents response to About.get endpoint
type About struct {
	StorageQuota struct {
		UsedSpace    string `json:"usedSpace"`
		UserCapacity string `json:"userCapacity"`
	} `json:"storageQuota"`
	MaxThumbnailSize  int64  `json:"maxThumbnailSize"`
	NeedUpdate        bool   `json:"needUpdate"`
	MaxFileUploadSize string `json:"maxFileUploadSize"`
	Domain            string `json:"domain"`
	Category          string `json:"category"`
	User              User   `json:"user"`
	FolderLevel       int    `json:"folderLevel"`
}

// StartCursor represents response to Changes.getStartCursor
type StartCursor struct {
	StartCursor string `json:"startCursor"`
}

// ChangeItem represents a change in the Changes API
type ChangeItem struct {
	ChangeType string `json:"changeType"`
	Deleted    bool   `json:"deleted"`
	File       *File  `json:"file,omitempty"`
}

// ChangesList represents response to Changes.list
type ChangesList struct {
	Category    string       `json:"category"`
	Changes     []ChangeItem `json:"changes"`
	NextCursor  string       `json:"nextCursor,omitempty"`
	HasNextPage bool         `json:"hasNextPage"`
}

// Subscription represents a webhook subscription for Changes
type Subscription struct {
	ID         string    `json:"id,omitempty"`
	Type       string    `json:"type"`
	URL        string    `json:"url"`
	Expiration time.Time `json:"expiration"`
}

// User represents a user
type User struct {
	PermissionID string `json:"permissionId"`
	DisplayName  string `json:"displayName"`
	Me           bool   `json:"me"`
	Category     string `json:"category"`
}

// FileList represents response to Files.list endpoint
type FileList struct {
	Files         []File `json:"files"`
	NextPageToken string `json:"nextPageToken,omitempty"`
	Category      string `json:"category"`
}

// File represents a file or folder in Huawei Drive
type File struct {
	ID                        string                 `json:"id"`
	FileName                  string                 `json:"fileName"`
	OriginalFilename          string                 `json:"originalFilename,omitempty"`
	Description               string                 `json:"description,omitempty"`
	MimeType                  string                 `json:"mimeType"`
	Category                  string                 `json:"category"`
	Size                      int64                  `json:"size,omitempty"`
	SHA256                    string                 `json:"sha256,omitempty"`
	CreatedTime               time.Time              `json:"createdTime"`
	EditedTime                time.Time              `json:"editedTime"`
	EditedByMeTime            time.Time              `json:"editedByMeTime,omitempty"`
	ParentFolder              []string               `json:"parentFolder,omitempty"`
	Owners                    []User                 `json:"owners"`
	LastEditor                User                   `json:"lastEditor"`
	Permissions               []Permission           `json:"permissions"`
	PermissionIDs             []string               `json:"permissionIds"`
	Capabilities              FileCapabilities       `json:"capabilities"`
	OwnedByMe                 bool                   `json:"ownedByMe"`
	EditedByMe                bool                   `json:"editedByMe"`
	ViewedByMe                bool                   `json:"viewedByMe"`
	HasShared                 bool                   `json:"hasShared"`
	Recycled                  bool                   `json:"recycled"`
	DirectlyRecycled          bool                   `json:"directlyRecycled"`
	Favorite                  bool                   `json:"favorite"`
	ExistThumbnail            bool                   `json:"existThumbnail"`
	ThumbnailVersion          int64                  `json:"thumbnailVersion"`
	IconDownloadLink          string                 `json:"iconDownloadLink,omitempty"`
	ContentDownloadLink       string                 `json:"contentDownloadLink,omitempty"`
	ContentVersion            string                 `json:"contentVersion,omitempty"`
	LastHistoryVersionID      string                 `json:"lastHistoryVersionId,omitempty"`
	OccupiedSpace             int64                  `json:"occupiedSpace,omitempty"`
	Version                   int64                  `json:"version"`
	WritersHasSharePermission bool                   `json:"writersHasSharePermission"`
	WriterHasCopyPermission   bool                   `json:"writerHasCopyPermission"`
	Containers                []string               `json:"containers"`
	Properties                map[string]interface{} `json:"properties,omitempty"`
	AppSettings               map[string]interface{} `json:"appSettings,omitempty"`
	ContentExtras             *ContentExtras         `json:"contentExtras,omitempty"`
}

// IsDir returns true if the file is a directory
func (f *File) IsDir() bool {
	return f.MimeType == FolderMimeType
}

// Permission represents file permission
type Permission struct {
	ID                 string `json:"id"`
	Type               string `json:"type"`
	Role               string `json:"role"`
	DisplayName        string `json:"displayName,omitempty"`
	Deleted            bool   `json:"deleted"`
	AllowFileDiscovery bool   `json:"allowFileDiscovery,omitempty"`
	Category           string `json:"category"`
}

// FileCapabilities represents capabilities for a file
type FileCapabilities struct {
	DownloadPermission            bool `json:"downloadPermission"`
	RenameFilePermission          bool `json:"renameFilePermission"`
	CommentPermission             bool `json:"commentPermission"`
	DeletePermission              bool `json:"deletePermission"`
	ReadHistoryVersionPermission  bool `json:"readHistoryVersionPermission"`
	RecyclePermission             bool `json:"recyclePermission"`
	AddChildNodePermission        bool `json:"addChildNodePermission"`
	UnrecyclePermission           bool `json:"unrecyclePermission"`
	WriterHasChangeCopyPermission bool `json:"writerHasChangeCopyPermission"`
	CopyPermission                bool `json:"copyPermission"`
	EditContentPermission         bool `json:"editContentPermission"`
	RemoveChildNodePermission     bool `json:"removeChildNodePermission"`
	EditPermission                bool `json:"editPermission"`
	ListChildNodePermission       bool `json:"listChildNodePermission"`
	ShareFilePermission           bool `json:"shareFilePermission"`
}

// ContentExtras represents additional content information
type ContentExtras struct {
	Thumbnail *Thumbnail `json:"thumbnail,omitempty"`
}

// Thumbnail represents thumbnail information
type Thumbnail struct {
	Content  string `json:"content"` // base64 encoded
	MimeType string `json:"mimeType"`
}

// CreateFolderRequest represents request to create a folder
type CreateFolderRequest struct {
	FileName     string                 `json:"fileName"`
	Description  string                 `json:"description,omitempty"`
	MimeType     string                 `json:"mimeType"`
	ParentFolder []string               `json:"parentFolder,omitempty"`
	Favorite     bool                   `json:"favorite,omitempty"`
	Properties   map[string]interface{} `json:"properties,omitempty"`
	AppSettings  map[string]interface{} `json:"appSettings,omitempty"`
}

// UpdateFileRequest represents request to update a file
type UpdateFileRequest struct {
	FileName                  string                 `json:"fileName,omitempty"`
	Description               string                 `json:"description,omitempty"`
	MimeType                  string                 `json:"mimeType,omitempty"`
	Favorite                  bool                   `json:"favorite,omitempty"`
	Recycled                  bool                   `json:"recycled,omitempty"`
	OriginalFilename          string                 `json:"originalFilename,omitempty"`
	WriterHasCopyPermission   bool                   `json:"writerHasCopyPermission,omitempty"`
	WritersHasSharePermission bool                   `json:"writersHasSharePermission,omitempty"`
	Properties                map[string]interface{} `json:"properties,omitempty"`
	AppSettings               map[string]interface{} `json:"appSettings,omitempty"`
	AddParentFolder           []string               `json:"addParentFolder,omitempty"`
	RemoveParentFolder        []string               `json:"removeParentFolder,omitempty"`
	CreatedTime               *time.Time             `json:"createdTime,omitempty"`
	EditedTime                *time.Time             `json:"editedTime,omitempty"`
}

// CopyFileRequest represents request to copy a file
type CopyFileRequest struct {
	FileName     string                 `json:"fileName,omitempty"`
	MimeType     string                 `json:"mimeType,omitempty"`
	ParentFolder []string               `json:"parentFolder,omitempty"`
	Properties   map[string]interface{} `json:"properties,omitempty"`
	AppSettings  map[string]interface{} `json:"appSettings,omitempty"`
	EditedTime   *time.Time             `json:"editedTime,omitempty"`
}

// ResumeUploadInitResponse represents response to resume upload init
type ResumeUploadInitResponse struct {
	SliceSize int64 `json:"sliceSize"`
}

// ErrorResponse represents an error response from the API
type ErrorResponse struct {
	Error struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

// Constants
const (
	// MIME types
	FolderMimeType = "application/vnd.huawei-apps.folder"

	// Special folder names
	ApplicationDataFolder = "applicationData"

	// Upload types
	UploadTypeMultipart = "multipart"
	UploadTypeContent   = "content"
	UploadTypeResume    = "resume"

	// Form types
	FormContent = "content"
	FormJSON    = "json"

	// Categories
	CategoryDriveFile     = "drive#file"
	CategoryDriveFileList = "drive#fileList"
	CategoryDriveAbout    = "drive#about"
	CategoryDriveUser     = "drive#user"
	CategoryDriveChanges  = "drive#changes"

	// Change types
	ChangeTypeFile   = "file"
	ChangeTypeFolder = "folder"
)
