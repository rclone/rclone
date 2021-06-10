package fichier

// FileInfoRequest is the request structure of the corresponding request
type FileInfoRequest struct {
	URL string `json:"url"`
}

// ListFolderRequest is the request structure of the corresponding request
type ListFolderRequest struct {
	FolderID int `json:"folder_id"`
}

// ListFilesRequest is the request structure of the corresponding request
type ListFilesRequest struct {
	FolderID int `json:"folder_id"`
}

// DownloadRequest is the request structure of the corresponding request
type DownloadRequest struct {
	URL    string `json:"url"`
	Single int    `json:"single"`
	Pass   string `json:"pass,omitempty"`
}

// RemoveFolderRequest is the request structure of the corresponding request
type RemoveFolderRequest struct {
	FolderID int `json:"folder_id"`
}

// RemoveFileRequest is the request structure of the corresponding request
type RemoveFileRequest struct {
	Files []RmFile `json:"files"`
}

// RmFile is the request structure of the corresponding request
type RmFile struct {
	URL string `json:"url"`
}

// GenericOKResponse is the response structure of the corresponding request
type GenericOKResponse struct {
	Status  string `json:"status"`
	Message string `json:"message"`
}

// MakeFolderRequest is the request structure of the corresponding request
type MakeFolderRequest struct {
	Name     string `json:"name"`
	FolderID int    `json:"folder_id"`
}

// MakeFolderResponse is the response structure of the corresponding request
type MakeFolderResponse struct {
	Name     string `json:"name"`
	FolderID int    `json:"folder_id"`
}

// MoveFileRequest is the request structure of the corresponding request
type MoveFileRequest struct {
	URLs     []string `json:"urls"`
	FolderID int      `json:"destination_folder_id"`
	Rename   string   `json:"rename,omitempty"`
}

// MoveFileResponse is the response structure of the corresponding request
type MoveFileResponse struct {
	Status  string   `json:"status"`
	Message string   `json:"message"`
	URLs    []string `json:"urls"`
}

// CopyFileRequest is the request structure of the corresponding request
type CopyFileRequest struct {
	URLs     []string `json:"urls"`
	FolderID int      `json:"folder_id"`
	Rename   string   `json:"rename,omitempty"`
}

// CopyFileResponse is the response structure of the corresponding request
type CopyFileResponse struct {
	Status  string     `json:"status"`
	Message string     `json:"message"`
	Copied  int        `json:"copied"`
	URLs    []FileCopy `json:"urls"`
}

// FileCopy is used in the the CopyFileResponse
type FileCopy struct {
	FromURL string `json:"from_url"`
	ToURL   string `json:"to_url"`
}

// RenameFileURL is the data structure to rename a single file
type RenameFileURL struct {
	URL      string `json:"url"`
	Filename string `json:"filename"`
}

// RenameFileRequest is the request structure of the corresponding request
type RenameFileRequest struct {
	URLs   []RenameFileURL `json:"urls"`
	Pretty int             `json:"pretty"`
}

// RenameFileResponse is the response structure of the corresponding request
type RenameFileResponse struct {
	Status  string `json:"status"`
	Message string `json:"message"`
	Renamed int    `json:"renamed"`
	URLs    []struct {
		URL         string `json:"url"`
		OldFilename string `json:"old_filename"`
		NewFilename string `json:"new_filename"`
	} `json:"urls"`
}

// GetUploadNodeResponse is the response structure of the corresponding request
type GetUploadNodeResponse struct {
	ID  string `json:"id"`
	URL string `json:"url"`
}

// GetTokenResponse is the response structure of the corresponding request
type GetTokenResponse struct {
	URL     string `json:"url"`
	Status  string `json:"Status"`
	Message string `json:"Message"`
}

// SharedFolderResponse is the response structure of the corresponding request
type SharedFolderResponse []SharedFile

// SharedFile is the structure how 1Fichier returns a shared File
type SharedFile struct {
	Filename string `json:"filename"`
	Link     string `json:"link"`
	Size     int64  `json:"size"`
}

// EndFileUploadResponse is the response structure of the corresponding request
type EndFileUploadResponse struct {
	Incoming int `json:"incoming"`
	Links    []struct {
		Download  string `json:"download"`
		Filename  string `json:"filename"`
		Remove    string `json:"remove"`
		Size      string `json:"size"`
		Whirlpool string `json:"whirlpool"`
	} `json:"links"`
}

// File is the structure how 1Fichier returns a File
type File struct {
	CDN         int    `json:"cdn"`
	Checksum    string `json:"checksum"`
	ContentType string `json:"content-type"`
	Date        string `json:"date"`
	Filename    string `json:"filename"`
	Pass        int    `json:"pass"`
	Size        int64  `json:"size"`
	URL         string `json:"url"`
}

// FilesList is the structure how 1Fichier returns a list of files
type FilesList struct {
	Items  []File `json:"items"`
	Status string `json:"Status"`
}

// Folder is the structure how 1Fichier returns a Folder
type Folder struct {
	CreateDate string `json:"create_date"`
	ID         int    `json:"id"`
	Name       string `json:"name"`
	Pass       int    `json:"pass"`
}

// FoldersList is the structure how 1Fichier returns a list of Folders
type FoldersList struct {
	FolderID   int      `json:"folder_id"`
	Name       string   `json:"name"`
	Status     string   `json:"Status"`
	SubFolders []Folder `json:"sub_folders"`
}
