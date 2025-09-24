// Package api defines types for interacting with the FileLu API.
package api

import "encoding/json"

// CreateFolderResponse represents the response for creating a folder.
type CreateFolderResponse struct {
	Status int    `json:"status"`
	Msg    string `json:"msg"`
	Result struct {
		FldID any `json:"fld_id"`
	} `json:"result"`
}

// DeleteFolderResponse represents the response for deleting a folder.
type DeleteFolderResponse struct {
	Status int    `json:"status"`
	Msg    string `json:"msg"`
}

// FolderListResponse represents the response for listing folders.
type FolderListResponse struct {
	Status int    `json:"status"`
	Msg    string `json:"msg"`
	Result struct {
		Files []struct {
			Name     string      `json:"name"`
			FldID    json.Number `json:"fld_id"`
			Path     string      `json:"path"`
			FileCode string      `json:"file_code"`
			Size     int64       `json:"size"`
		} `json:"files"`
		Folders []struct {
			Name  string      `json:"name"`
			FldID json.Number `json:"fld_id"`
			Path  string      `json:"path"`
		} `json:"folders"`
	} `json:"result"`
}

// FileDirectLinkResponse represents the response for a direct link to a file.
type FileDirectLinkResponse struct {
	Status int    `json:"status"`
	Msg    string `json:"msg"`
	Result struct {
		URL  string `json:"url"`
		Size int64  `json:"size"`
	} `json:"result"`
}

// FileInfoResponse represents the response for file information.
type FileInfoResponse struct {
	Status int    `json:"status"`
	Msg    string `json:"msg"`
	Result []struct {
		Size     string `json:"size"`
		Name     string `json:"name"`
		FileCode string `json:"filecode"`
		Hash     string `json:"hash"`
		Status   int    `json:"status"`
	} `json:"result"`
}

// DeleteFileResponse represents the response for deleting a file.
type DeleteFileResponse struct {
	Status int    `json:"status"`
	Msg    string `json:"msg"`
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
