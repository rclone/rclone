// Package api provides types used by the Uptobox API.
package api

type Error struct {
	Status  bool   `json:"status,omitempty"`
	Message string `json:"message,omitempty"`
	ErrMsg  string `json:"error,omitempty"`
}

func (e Error) Error() string {
	out := "api error"
	if e.Message != "" {
		out += ": " + e.Message
	}
	return out
}

type Part struct {
	Id    int64
	Size  int64
	Name  string
	Start int64
	End   int64
}

// FileInfo represents a file when listing folder contents
type FileInfo struct {
	Id       string `json:"id"`
	Name     string `json:"name"`
	MimeType string `json:"mimeType"`
	Size     int64  `json:"size"`
	ParentId string `json:"parentId"`
	Type     string `json:"type"`
	ModTime  string `json:"updatedAt"`
}

// ReadMetadataResponse is the response when listing folder contents
type ReadMetadataResponse struct {
	Files         []FileInfo `json:"results"`
	NextPageToken string     `json:"nextPageToken,omitempty"`
}

// UploadInfo is the response when initiating an upload
type UploadInfo struct {
	ID    int64  `json:"id"`
	Name  string `json:"name"`
	Size  int64  `json:"size"`
	State string `json:"state"`
}

type UploadFile struct {
	Parts []PartFile `json:"parts,omitempty"`
}

// UploadResponse is the response to a successful upload
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
	Message string `json:"message,omitempty"`
	Status  bool   `json:"status"`
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
	PerPage       uint64
	SearchField   string
	Search        string
	NextPageToken string
}

// CreateFolderRequest is used for creating a folder
type CreateFolderRequest struct {
	Name string `json:"name"`
	Type string `json:"type"`
	Path string `json:"path"`
}

type CreateDirRequest struct {
	Path string `json:"path"`
}

type PartFile struct {
	Name       string `json:"name"`
	PartId     int    `json:"partId"`
	PartNo     int    `json:"partNo"`
	TotalParts int    `json:"totalParts"`
	Size       int64  `json:"size"`
	ChannelID  int64  `json:"channelId"`
	Encrypted  bool   `json:"encrypted"`
	Salt       string `json:"salt"`
}

type FilePart struct {
	ID   int    `json:"id"`
	Salt string `json:"salt"`
}

type CreateFileRequest struct {
	Name      string     `json:"name"`
	Type      string     `json:"type"`
	Path      string     `json:"path"`
	MimeType  string     `json:"mimeType"`
	Size      int64      `json:"size"`
	ChannelID int64      `json:"channelId"`
	Encrypted bool       `json:"encrypted"`
	Parts     []FilePart `json:"parts"`
	CreatedAt string     `json:"createdAt,omitempty"`
	UpdatedAt string     `json:"updatedAt,omitempty"`
}

// DeleteFolderRequest is used for deleting a folder
type DeleteFolderRequest struct {
	Token    string `json:"token"`
	FolderID string `json:"fld_id"`
}

// CopyMoveFileRequest is used for moving/copying a file
type CopyMoveFileRequest struct {
	Token               string `json:"token"`
	FileCodes           string `json:"file_codes"`
	DestinationFolderID string `json:"destination_fld_id"`
	Action              string `json:"action"`
}

// MoveFolderRequest is used for moving a folder
type MoveFileRequest struct {
	Files       []string `json:"files"`
	Destination string   `json:"destination"`
}
type DirMove struct {
	Source      string `json:"source"`
	Destination string `json:"destination"`
}

// UpdateFileInformation is used for renaming a file
type UpdateFileInformation struct {
	Name string `json:"name"`
	Type string `json:"type,omitempty"`
}

// RemoveFileRequest is used for deleting a file
type RemoveFileRequest struct {
	Files []string `json:"files"`
}

type CopyFile struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Destination string `json:"destination"`
}

// Token represents the authentication token
type Token struct {
	Token string `json:"token"`
}

type Session struct {
	UserName string `json:"userName"`
	Hash     string `json:"hash"`
}
