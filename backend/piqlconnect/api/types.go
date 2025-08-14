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

// Remove file is the request body for deleting files (/api/files/delete)
type RemoveFile struct {
	OrganisationId string    `json:"organisationId"`
	PackageId      string    `json:"packageId"`
	FileIds        [1]string `json:"fileIds"`
}

type FilePath struct {
	Path string `json:"path"`
}

// CreateFileUrl is the request body for creating upload URLs in order to upload files (/api/sas-url)
type CreateFileUrl struct {
	OrganisationId string      `json:"organisationId"`
	PackageId      string      `json:"packageId"`
	Files          [1]FilePath `json:"files"`
	Method         string      `json:"method"`
}

type FilePathSize struct {
	Path string `json:"path"`
	Size int64  `json:"size"`
}

// Create file is the request body for creating the file entry after upload (/api/files)
type CreateFile struct {
	OrganisationId string          `json:"organisationId"`
	PackageId      string          `json:"packageId"`
	Files          [1]FilePathSize `json:"files"`
}

// Touch file is the request body for updating file ModTime (/api/files/touch)
type TouchFile struct {
	OrganisationId string    `json:"organisationId"`
	PackageId      string    `json:"packageId"`
	Files          [1]string `json:"files"`
}

// Create folder is the request body for creating folders (/api/folders)
type CreateFolder struct {
	OrganisationId string `json:"organisationId"`
	PackageId      string `json:"packageId"`
	FolderPath     string `json:"folderPath"`
}

// Remove folder is the request body for deleting folders (/api/folders/delete)
type RemoveFolder struct {
	OrganisationId string    `json:"organisationId"`
	PackageId      string    `json:"packageId"`
	FolderPaths    [1]string `json:"folderPaths"`
}
