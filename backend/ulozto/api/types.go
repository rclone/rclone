package api

import (
	"errors"
	"fmt"
	"time"
)

type Error struct {
	ErrorCode  int    `json:"error"`
	StatusCode int    `json:"code"`
	Message    string `json:"message"`
}

func (e *Error) Error() string {
	out := fmt.Sprintf("Error %d (%d)", e.ErrorCode, e.StatusCode)
	if e.Message != "" {
		out += ": " + e.Message
	}
	return out
}

func (e *Error) Is(target error) bool {
	var err *Error
	ok := errors.As(target, &err)
	return ok
}

type ResponseMetadata struct {
	Timestamp  time.Time `json:"RunAt"`
	Offset     int32     `json:"offset"`
	Limit      int32     `json:"limit"`
	ItemsCount int32     `json:"items_count"`
}

type Folder struct {
	Discriminator        string    `json:"discriminator"`
	Name                 string    `json:"name"`
	SanitizedName        string    `json:"name_sanitized"`
	Slug                 string    `json:"slug"`
	Status               string    `json:"status"`
	PublicUrl            string    `json:"public_url"`
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

type File struct {
	Discriminator            string `json:"discriminator"`
	Slug                     string `json:"slug"`
	Url                      string `json:"url"`
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

type CreateFolderRequest struct {
	Name             string `json:"name"`
	ParentFolderSlug string `json:"parent_folder_slug"`
}

type ListFoldersResponse struct {
	Metadata   ResponseMetadata `json:"metadata"`
	Folder     Folder           `json:"folder"`
	Subfolders []Folder         `json:"subfolders"`
}

type ListFilesResponse struct {
	Metadata ResponseMetadata `json:"metadata"`
	Items    []File           `json:"items"`
}

type DeleteFoldersRequest struct {
	Slugs []string `json:"slugs"`
}

type CreateUploadUrlRequest struct {
	UserLogin           string `json:"user_login"`
	Realm               string `json:"realm"`
	ExistingSessionSlug string `json:"private_slug"`
}

type CreateUploadUrlResponse struct {
	UploadUrl        string    `json:"upload_url"`
	PrivateSlug      string    `json:"private_slug"`
	ValidUntil       time.Time `json:"valid_until"`
	ValidityInterval int64     `json:"validity_interval"`
}

type BatchUpdateFilePropertiesRequest struct {
	Name         string            `json:"name"`
	FolderSlug   string            `json:"folder_slug"`
	Description  string            `json:"description"`
	Slugs        []string          `json:"slugs"`
	UploadTokens map[string]string `json:"upload_tokens"`
}

type SendFilePayloadResponse struct {
	Size        int    `json:"size"`
	ContentType string `json:"contentType"`
	Md5         string `json:"md5"`
	Message     string `json:"message"`
	ReturnCode  int    `json:"return_code"`
	Slug        string `json:"slug"`
}

type CommitUploadBatchRequest struct {
	Status     string `json:"status"`
	OwnerLogin string `json:"owner_login"`
}

type CommitUploadBatchResponse struct {
	PrivateSlug          string    `json:"private_slug"`
	PublicSlug           string    `json:"public_slug"`
	Status               string    `json:"status"`
	ConfirmedAt          time.Time `json:"confirmed_at"`
	Discriminator        string    `json:"discriminator"`
	Privacy              string    `json:"privacy"`
	Name                 time.Time `json:"name"`
	PublicUrl            string    `json:"public_url"`
	FilesCountOk         int       `json:"files_count_ok"`
	FilesCountTrash      int       `json:"files_count_trash"`
	FilesCountIncomplete int       `json:"files_count_incomplete"`
}

type UpdateDescriptionRequest struct {
	Description string `json:"description"`
}

type GetDownloadLinkRequest struct {
	Slug      string `json:"file_slug"`
	UserLogin string `json:"user_login"`
	DeviceId  string `json:"device_id"`
}

type GetDownloadLinkResponse struct {
	Link                        string    `json:"link"`
	DownloadUrlValidUntil       time.Time `json:"download_url_valid_until"`
	DownloadUrlValidityInterval int       `json:"download_url_validity_interval"`
	Hash                        string    `json:"hash"`
}

type AuthenticateRequest struct {
	Login    string `json:"login"`
	Password string `json:"password"`
}

type AuthenticateResponse struct {
	TokenId               string `json:"token_id"`
	TokenValidityInterval int    `json:"token_validity_interval"`
	Session               struct {
		Country          string `json:"country"`
		IsLimitedCountry bool   `json:"is_limited_country"`
		User             struct {
			Login               string `json:"login"`
			UserId              int64  `json:"user_id"`
			Credit              int64  `json:"credit"`
			AvatarUrl           string `json:"avatar_url"`
			FavoritesLink       string `json:"favorites_link"`
			RootFolderSlug      string `json:"root_folder_slug"`
			FavoritesFolderSlug string `json:"favorites_folder_slug"`
			HasCloud            bool   `json:"has_cloud"`
		} `json:"user"`
	} `json:"session"`
}
