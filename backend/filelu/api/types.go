// Package api defines types for interacting with the FileLu API.
package api

// FolderListResponse represents the response from the folder/list API.
type FolderListResponse struct {
	Status int    `json:"status"` // HTTP status code of the response.
	Msg    string `json:"msg"`    // Message describing the response.
	Result struct {
		Files   []FolderListFile   `json:"files"`   // List of files in the folder.
		Folders []FolderListFolder `json:"folders"` // List of folders in the folder.
	} `json:"result"` // Nested result structure containing files and folders.
}

// FolderListFile represents a file in the FolderListResponse.
type FolderListFile struct {
	Name      string `json:"name"`      // File name.
	Size      int64  `json:"size"`      // File size in bytes.
	Uploaded  string `json:"uploaded"`  // Upload date as a string.
	Thumbnail string `json:"thumbnail"` // URL to the file's thumbnail.
	Link      string `json:"link"`      // URL to access the file.
	FldID     string `json:"fld_id"`    // Folder ID containing the file.
	FileCode  string `json:"file_code"` // Unique code for the file.
	Hash      string `json:"hash"`      // Hash of the file for verification.
}

// FolderListFolder represents a folder in the FolderListResponse.
type FolderListFolder struct {
	Name      string `json:"name"`       // Folder name.
	Code      string `json:"code"`       // Unique code for the folder.
	FldID     string `json:"fld_id"`     // Folder ID.
	FldPublic int    `json:"fld_public"` // Indicates if the folder is public.
	Filedrop  int    `json:"filedrop"`   // Indicates if the folder supports file drop.
}

// AccountInfoResponse represents the response for account information.
type AccountInfoResponse struct {
	Status int    `json:"status"` // HTTP status code of the response.
	Msg    string `json:"msg"`    // Message describing the response.
	Result struct {
		PremiumExpire string `json:"premium_expire"` // Expiration date of premium access.
		Email         string `json:"email"`          // User's email address.
		UType         string `json:"utype"`          // User type (e.g., premium or free).
		Storage       string `json:"storage"`        // Total storage available to the user.
		StorageUsed   string `json:"storage_used"`   // Amount of storage used.
	} `json:"result"` // Nested result structure containing account details.
}

// FolderDeleteResponse represents the response for deleting a folder.
type FolderDeleteResponse struct {
	Status     int    `json:"status"`      // HTTP status code of the response.
	Msg        string `json:"msg"`         // Message describing the response.
	Result     string `json:"result"`      // Result of the deletion operation.
	ServerTime string `json:"server_time"` // Server timestamp of the operation.
}

// DeleteResponse represents the response for deleting a file or folder.
type DeleteResponse struct {
	Status int    `json:"status"` // HTTP status code of the response.
	Msg    string `json:"msg"`    // Message describing the response.
}
