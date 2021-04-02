package api

import "fmt"

// Error contains the error code and message returned by the API
type Error struct {
	Success    bool   `json:"success,omitempty"`
	StatusCode int    `json:"statusCode,omitempty"`
	Message    string `json:"message,omitempty"`
	Data       string `json:"data,omitempty"`
}

// Error returns a string for the error and satisfies the error interface
func (e Error) Error() string {
	out := fmt.Sprintf("api error %d", e.StatusCode)
	if e.Message != "" {
		out += ": " + e.Message
	}
	if e.Data != "" {
		out += ": " + e.Data
	}
	return out
}

// FolderEntry represents a Uptobox subfolder when listing folder contents
type FolderEntry struct {
	FolderID    uint64 `json:"fld_id"`
	Description string `json:"fld_descr"`
	Password    string `json:"fld_password"`
	FullPath    string `json:"fullPath"`
	Path        string `json:"fld_name"`
	Name        string `json:"name"`
	Hash        string `json:"hash"`
}

// FolderInfo represents the current folder when listing folder contents
type FolderInfo struct {
	FolderID      uint64 `json:"fld_id"`
	Hash          string `json:"hash"`
	FileCount     uint64 `json:"fileCount"`
	TotalFileSize int64  `json:"totalFileSize"`
}

// FileInfo represents a file when listing folder contents
type FileInfo struct {
	Name         string `json:"file_name"`
	Description  string `json:"file_descr"`
	Created      string `json:"file_created"`
	Size         int64  `json:"file_size"`
	Downloads    uint64 `json:"file_downloads"`
	Code         string `json:"file_code"`
	Password     string `json:"file_password"`
	Public       int    `json:"file_public"`
	LastDownload string `json:"file_last_download"`
	ID           uint64 `json:"id"`
}

// ReadMetadataResponse is the response when listing folder contents
type ReadMetadataResponse struct {
	StatusCode int    `json:"statusCode"`
	Message    string `json:"message"`
	Data       struct {
		CurrentFolder  FolderInfo    `json:"currentFolder"`
		Folders        []FolderEntry `json:"folders"`
		Files          []FileInfo    `json:"files"`
		PageCount      int           `json:"pageCount"`
		TotalFileCount int           `json:"totalFileCount"`
		TotalFileSize  int64         `json:"totalFileSize"`
	} `json:"data"`
}

// UploadInfo is the response when initiating an upload
type UploadInfo struct {
	StatusCode int    `json:"statusCode"`
	Message    string `json:"message"`
	Data       struct {
		UploadLink string `json:"uploadLink"`
		MaxUpload  string `json:"maxUpload"`
	} `json:"data"`
}

// UploadResponse is the respnse to a successful upload
type UploadResponse struct {
	Files []struct {
		Name      string `json:"name"`
		Size      int64  `json:"size"`
		URL       string `json:"url"`
		DeleteURL string `json:"deleteUrl"`
	} `json:"files"`
}

// UpdateResponse is a generic response to various action on files (rename/copy/move)
type UpdateResponse struct {
	Message    string `json:"message"`
	StatusCode int    `json:"statusCode"`
}

// Download is the response when requesting a download link
type Download struct {
	StatusCode int    `json:"statusCode"`
	Message    string `json:"message"`
	Data       struct {
		DownloadLink string `json:"dlLink"`
	} `json:"data"`
}

// MetadataRequestOptions represents all the options when listing folder contents
type MetadataRequestOptions struct {
	Limit       uint64
	Offset      uint64
	SearchField string
	Search      string
}

// CreateFolderRequest is used for creating a folder
type CreateFolderRequest struct {
	Token string `json:"token"`
	Path  string `json:"path"`
	Name  string `json:"name"`
}

// DeleteFolderRequest is used for deleting a folder
type DeleteFolderRequest struct {
	Token    string `json:"token"`
	FolderID uint64 `json:"fld_id"`
}

// CopyMoveFileRequest is used for moving/copying a file
type CopyMoveFileRequest struct {
	Token               string `json:"token"`
	FileCodes           string `json:"file_codes"`
	DestinationFolderID uint64 `json:"destination_fld_id"`
	Action              string `json:"action"`
}

// MoveFolderRequest is used for moving a folder
type MoveFolderRequest struct {
	Token               string `json:"token"`
	FolderID            uint64 `json:"fld_id"`
	DestinationFolderID uint64 `json:"destination_fld_id"`
	Action              string `json:"action"`
}

// RenameFolderRequest is used for renaming a folder
type RenameFolderRequest struct {
	Token    string `json:"token"`
	FolderID uint64 `json:"fld_id"`
	NewName  string `json:"new_name"`
}

// UpdateFileInformation is used for renaming a file
type UpdateFileInformation struct {
	Token       string `json:"token"`
	FileCode    string `json:"file_code"`
	NewName     string `json:"new_name,omitempty"`
	Description string `json:"description,omitempty"`
	Password    string `json:"password,omitempty"`
	Public      string `json:"public,omitempty"`
}

// RemoveFileRequest is used for deleting a file
type RemoveFileRequest struct {
	Token     string `json:"token"`
	FileCodes string `json:"file_codes"`
}

// Token represents the authentication token
type Token struct {
	Token string `json:"token"`
}
