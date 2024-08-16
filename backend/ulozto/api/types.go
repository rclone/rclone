// Package api has type definitions for uloz.to
package api

import (
	"errors"
	"fmt"
	"time"
)

// Error is a representation of the JSON structure returned by uloz.to for unsuccessful requests.
type Error struct {
	ErrorCode  int    `json:"error"`
	StatusCode int    `json:"code"`
	Message    string `json:"message"`
}

// Error implements error.Error() and returns a string representation of the error.
func (e *Error) Error() string {
	out := fmt.Sprintf("Error %d (%d)", e.ErrorCode, e.StatusCode)
	if e.Message != "" {
		out += ": " + e.Message
	}
	return out
}

// Is determines if the error is an instance of another error. It's required for the
// errors package to search in causal chain.
func (e *Error) Is(target error) bool {
	var err *Error
	ok := errors.As(target, &err)
	return ok
}

// ListResponseMetadata groups fields common for all API List calls,
// and maps to the Metadata API JSON object.
type ListResponseMetadata struct {
	Timestamp  time.Time `json:"RunAt"`
	Offset     int32     `json:"offset"`
	Limit      int32     `json:"limit"`
	ItemsCount int32     `json:"items_count"`
}

// Folder represents a single folder, and maps to the AggregatePrivateViewFolder
// JSON API object.
type Folder struct {
	Discriminator        string    `json:"discriminator"`
	Name                 string    `json:"name"`
	SanitizedName        string    `json:"name_sanitized"`
	Slug                 string    `json:"slug"`
	Status               string    `json:"status"`
	PublicURL            string    `json:"public_url"`
	IsPasswordProtected  bool      `json:"is_password_protected"`
	Type                 string    `json:"type"`
	FileManagerLink      string    `json:"file_manager_link"`
	ParentFolderSlug     string    `json:"parent_folder_slug"`
	Privacy              string    `json:"privacy"`
	Created              time.Time `json:"created"`
	LastUserModified     time.Time `json:"last_user_modified"`
	HasSubfolder         bool      `json:"has_subfolder"`
	HasTrashedSubfolders bool      `json:"has_trashed_subfolders"`
}

// File represents a single file, and maps to the AggregatePrivateViewFileV3
// JSON API object.
type File struct {
	Discriminator            string `json:"discriminator"`
	Slug                     string `json:"slug"`
	URL                      string `json:"url"`
	Realm                    string `json:"realm"`
	Name                     string `json:"name"`
	NameSanitized            string `json:"name_sanitized"`
	Extension                string `json:"extension"`
	Filesize                 int64  `json:"filesize"`
	PasswordProtectedFile    bool   `json:"password_protected_file"`
	Description              string `json:"description"`
	DescriptionSanitized     string `json:"description_sanitized"`
	IsPorn                   bool   `json:"is_porn"`
	Rating                   int    `json:"rating"`
	PasswordProtectedArchive bool   `json:"password_protected_archive"`
	MalwareStatus            string `json:"malware_status"`
	ContentStatus            string `json:"content_status"`
	ContentType              string `json:"content_type"`
	Format                   struct {
	} `json:"format"`
	DownloadTypes []interface{} `json:"download_types"`
	ThumbnailInfo []interface{} `json:"thumbnail_info"`
	PreviewInfo   struct {
	} `json:"preview_info"`
	Privacy          string    `json:"privacy"`
	IsPornByUploader bool      `json:"is_porn_by_uploader"`
	ExpireDownload   int       `json:"expire_download"`
	ExpireTime       time.Time `json:"expire_time"`
	UploadTime       time.Time `json:"upload_time"`
	LastUserModified time.Time `json:"last_user_modified"`
	FolderSlug       string    `json:"folder_slug"`
	IsIncomplete     bool      `json:"is_incomplete"`
	IsInTrash        bool      `json:"is_in_trash"`
	Processing       struct {
		Identify       bool `json:"identify"`
		Thumbnails     bool `json:"thumbnails"`
		LivePreview    bool `json:"live_preview"`
		ArchiveContent bool `json:"archive_content"`
		Preview        bool `json:"preview"`
	} `json:"processing"`
}

// CreateFolderRequest represents the JSON API object
// that's sent to the create folder API endpoint.
type CreateFolderRequest struct {
	Name             string `json:"name"`
	ParentFolderSlug string `json:"parent_folder_slug"`
}

// ListFoldersResponse represents the JSON API object
// that's received from the list folders API endpoint.
type ListFoldersResponse struct {
	Metadata   ListResponseMetadata `json:"metadata"`
	Folder     Folder               `json:"folder"`
	Subfolders []Folder             `json:"subfolders"`
}

// ListFilesResponse represents the JSON API object
// that's received from the list files API endpoint.
type ListFilesResponse struct {
	Metadata ListResponseMetadata `json:"metadata"`
	Items    []File               `json:"items"`
}

// DeleteFoldersRequest represents the JSON API object
// that's sent to the delete folders API endpoint.
type DeleteFoldersRequest struct {
	Slugs []string `json:"slugs"`
}

// CreateUploadURLRequest represents the JSON API object that's
// sent to the API endpoint generating URLs for new file uploads.
type CreateUploadURLRequest struct {
	UserLogin           string `json:"user_login"`
	Realm               string `json:"realm"`
	ExistingSessionSlug string `json:"private_slug"`
}

// CreateUploadURLResponse represents the JSON API object that's
// received from the API endpoint generating URLs for new file uploads.
type CreateUploadURLResponse struct {
	UploadURL        string    `json:"upload_url"`
	PrivateSlug      string    `json:"private_slug"`
	ValidUntil       time.Time `json:"valid_until"`
	ValidityInterval int64     `json:"validity_interval"`
}

// BatchUpdateFilePropertiesRequest represents the JSON API object that's
// sent to the API endpoint moving the uploaded files from a scratch space
// to their final destination.
type BatchUpdateFilePropertiesRequest struct {
	Name         string            `json:"name"`
	FolderSlug   string            `json:"folder_slug"`
	Description  string            `json:"description"`
	Slugs        []string          `json:"slugs"`
	UploadTokens map[string]string `json:"upload_tokens"`
}

// SendFilePayloadResponse represents the JSON API object that's received
// in response to uploading a file's body to the CDN URL.
type SendFilePayloadResponse struct {
	Size        int64  `json:"size"`
	ContentType string `json:"contentType"`
	Md5         string `json:"md5"`
	Message     string `json:"message"`
	ReturnCode  int    `json:"return_code"`
	Slug        string `json:"slug"`
}

// CommitUploadBatchRequest represents the JSON API object that's
// sent to the API endpoint marking the upload batch as final.
type CommitUploadBatchRequest struct {
	Status     string `json:"status"`
	OwnerLogin string `json:"owner_login"`
}

// CommitUploadBatchResponse represents the JSON API object that's
// received from the API endpoint marking the upload batch as final.
type CommitUploadBatchResponse struct {
	PrivateSlug          string    `json:"private_slug"`
	PublicSlug           string    `json:"public_slug"`
	Status               string    `json:"status"`
	ConfirmedAt          time.Time `json:"confirmed_at"`
	Discriminator        string    `json:"discriminator"`
	Privacy              string    `json:"privacy"`
	Name                 time.Time `json:"name"`
	PublicURL            string    `json:"public_url"`
	FilesCountOk         int       `json:"files_count_ok"`
	FilesCountTrash      int       `json:"files_count_trash"`
	FilesCountIncomplete int       `json:"files_count_incomplete"`
}

// UpdateDescriptionRequest represents the JSON API object that's
// sent to the file modification API endpoint marking the upload batch as final.
type UpdateDescriptionRequest struct {
	Description string `json:"description"`
}

// MoveFolderRequest represents the JSON API object that's
// sent to the folder moving API endpoint.
type MoveFolderRequest struct {
	FolderSlugs         []string `json:"slugs"`
	NewParentFolderSlug string   `json:"parent_folder_slug"`
}

// RenameFolderRequest represents the JSON API object that's
// sent to the folder moving API endpoint.
type RenameFolderRequest struct {
	NewName string `json:"name"`
}

// MoveFileRequest represents the JSON API object that's
// sent to the file  moving API endpoint.
type MoveFileRequest struct {
	ParentFolderSlug string `json:"folder_slug,omitempty"`
	NewFilename      string `json:"name,omitempty"`
}

// GetDownloadLinkRequest represents the JSON API object that's
// sent to the API endpoint that generates CDN download links for file payloads.
type GetDownloadLinkRequest struct {
	Slug      string `json:"file_slug"`
	UserLogin string `json:"user_login"`
	DeviceID  string `json:"device_id"`
}

// GetDownloadLinkResponse represents the JSON API object that's
// received from the API endpoint that generates CDN download links for file payloads.
type GetDownloadLinkResponse struct {
	Link                        string    `json:"link"`
	DownloadURLValidUntil       time.Time `json:"download_url_valid_until"`
	DownloadURLValidityInterval int       `json:"download_url_validity_interval"`
	Hash                        string    `json:"hash"`
}

// AuthenticateRequest represents the JSON API object that's sent to the auth API endpoint.
type AuthenticateRequest struct {
	Login    string `json:"login"`
	Password string `json:"password"`
}

// AuthenticateResponse represents the JSON API object that's received from the auth API endpoint.
type AuthenticateResponse struct {
	TokenID               string `json:"token_id"`
	TokenValidityInterval int    `json:"token_validity_interval"`
	Session               struct {
		Country          string `json:"country"`
		IsLimitedCountry bool   `json:"is_limited_country"`
		User             struct {
			Login               string `json:"login"`
			UserID              int64  `json:"user_id"`
			Credit              int64  `json:"credit"`
			AvatarURL           string `json:"avatar_url"`
			FavoritesLink       string `json:"favorites_link"`
			RootFolderSlug      string `json:"root_folder_slug"`
			FavoritesFolderSlug string `json:"favorites_folder_slug"`
			HasCloud            bool   `json:"has_cloud"`
		} `json:"user"`
	} `json:"session"`
}
