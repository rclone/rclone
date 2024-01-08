package api

import (
	"time"
)

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
	HasSubfolder         string    `json:"has_subfolder"`
	HasTrashedSubfolders string    `json:"has_trashed_subfolders"`
}

type File struct {
	Discriminator            string `json:"discriminator"`
	Slug                     string `json:"slug"`
	Url                      string `json:"url"`
	Realm                    string `json:"realm"`
	Name                     string `json:"name"`
	NameSanitized            string `json:"name_sanitized"`
	Extension                string `json:"extension"`
	Filesize                 string `json:"filesize"`
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
	IsIncomplete     string    `json:"is_incomplete"`
	IsInTrash        string    `json:"is_in_trash"`
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
