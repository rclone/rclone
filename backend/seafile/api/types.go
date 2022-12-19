// Package api provides types used by the Seafile API.
package api

// Some api objects are duplicated with only small differences,
// it's because the returned JSON objects are very inconsistent between api calls

// AuthenticationRequest contains user credentials
type AuthenticationRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// AuthenticationResult is returned by a call to the authentication api
type AuthenticationResult struct {
	Token  string   `json:"token"`
	Errors []string `json:"non_field_errors"`
}

// AccountInfo contains simple user properties
type AccountInfo struct {
	Usage int64  `json:"usage"`
	Total int64  `json:"total"`
	Email string `json:"email"`
	Name  string `json:"name"`
}

// ServerInfo contains server information
type ServerInfo struct {
	Version string `json:"version"`
}

// DefaultLibrary when none specified
type DefaultLibrary struct {
	ID     string `json:"repo_id"`
	Exists bool   `json:"exists"`
}

// CreateLibraryRequest contains the information needed to create a library
type CreateLibraryRequest struct {
	Name        string `json:"name"`
	Description string `json:"desc"`
	Password    string `json:"passwd"`
}

// Library properties. Please note not all properties are going to be useful for rclone
type Library struct {
	Encrypted bool   `json:"encrypted"`
	Owner     string `json:"owner"`
	ID        string `json:"id"`
	Size      int64  `json:"size"`
	Name      string `json:"name"`
	Modified  int64  `json:"mtime"`
}

// CreateLibrary properties. Seafile is not consistent and returns different types for different API calls
type CreateLibrary struct {
	ID   string `json:"repo_id"`
	Name string `json:"repo_name"`
}

// FileType is either "dir" or "file"
type FileType string

// File types
var (
	FileTypeDir  FileType = "dir"
	FileTypeFile FileType = "file"
)

// FileDetail contains file properties (for older api v2.0)
type FileDetail struct {
	ID       string   `json:"id"`
	Type     FileType `json:"type"`
	Name     string   `json:"name"`
	Size     int64    `json:"size"`
	Parent   string   `json:"parent_dir"`
	Modified string   `json:"last_modified"`
}

// DirEntries contains a list of DirEntry
type DirEntries struct {
	Entries []DirEntry `json:"dirent_list"`
}

// DirEntry contains a directory entry
type DirEntry struct {
	ID       string   `json:"id"`
	Type     FileType `json:"type"`
	Name     string   `json:"name"`
	Size     int64    `json:"size"`
	Path     string   `json:"parent_dir"`
	Modified int64    `json:"mtime"`
}

// Operation is move, copy or rename
type Operation string

// Operations
var (
	CopyFileOperation   Operation = "copy"
	MoveFileOperation   Operation = "move"
	RenameFileOperation Operation = "rename"
)

// FileOperationRequest is sent to the api to copy, move or rename a file
type FileOperationRequest struct {
	Operation            Operation `json:"operation"`
	DestinationLibraryID string    `json:"dst_repo"` // For copy/move operation
	DestinationPath      string    `json:"dst_dir"`  // For copy/move operation
	NewName              string    `json:"newname"`  // Only to be used by the rename operation
}

// FileInfo is returned by a server file copy/move/rename (new api v2.1)
type FileInfo struct {
	Type      string `json:"type"`
	LibraryID string `json:"repo_id"`
	Path      string `json:"parent_dir"`
	Name      string `json:"obj_name"`
	ID        string `json:"obj_id"`
	Size      int64  `json:"size"`
}

// CreateDirRequest only contain an operation field
type CreateDirRequest struct {
	Operation string `json:"operation"`
}

// DirectoryDetail contains the directory details specific to the getDirectoryDetails call
type DirectoryDetail struct {
	ID   string `json:"repo_id"`
	Name string `json:"name"`
	Path string `json:"path"`
}

// ShareLinkRequest contains the information needed to create or list shared links
type ShareLinkRequest struct {
	LibraryID string `json:"repo_id"`
	Path      string `json:"path"`
}

// SharedLink contains the information returned by a call to shared link creation
type SharedLink struct {
	Link      string `json:"link"`
	IsExpired bool   `json:"is_expired"`
}

// BatchSourceDestRequest contains JSON parameters for sending a batch copy or move operation
type BatchSourceDestRequest struct {
	SrcLibraryID string   `json:"src_repo_id"`
	SrcParentDir string   `json:"src_parent_dir"`
	SrcItems     []string `json:"src_dirents"`
	DstLibraryID string   `json:"dst_repo_id"`
	DstParentDir string   `json:"dst_parent_dir"`
}
