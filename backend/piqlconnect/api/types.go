// Package api has type definitions for piqlconnect
package api

// Item describes a folder or a file as returned by /api/files
// Folder items always end with "/" and have size 0
type Item struct {
	Id        string `json:"id"`
	Path      string `json:"path"`
	Size      int64  `json:"size"`
	UpdatedAt string `json:"updatedAt"`
}

// Package represents a single remote file hierarchy
type Package struct {
	Id        string `json:"id"`
	Name      string `json:"name"`
	Status    string `json:"status"`
	UpdatedAt string `json:"updatedAt"`
}

// Download file is the request body for downloading files (/api/files/download)
type DownloadFile struct {
	OrganisationId string    `json:"organisationId"`
	PackageId      string    `json:"packageId"`
	BlobNames      [1]string `json:"blobNames"`
}

type RemoveFile struct {
	OrganisationId string    `json:"organisationId"`
	PackageId      string    `json:"packageId"`
	FileIds        [1]string `json:"fileIds"`
}
