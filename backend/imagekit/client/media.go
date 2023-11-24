package client

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/rclone/rclone/lib/rest"
	"gopkg.in/validator.v2"
)

// FilesOrFolderParam struct is a parameter type to ListFiles() function to search / list media library files.
type FilesOrFolderParam struct {
	Path        string `json:"path,omitempty"`
	Limit       int    `json:"limit,omitempty"`
	Skip        int    `json:"skip,omitempty"`
	SearchQuery string `json:"searchQuery,omitempty"`
}

// AITag represents an AI tag for a media library file.
type AITag struct {
	Name       string  `json:"name"`
	Confidence float32 `json:"confidence"`
	Source     string  `json:"source"`
}

// File represents media library File details.
type File struct {
	FileID            string            `json:"fileId"`
	Name              string            `json:"name"`
	FilePath          string            `json:"filePath"`
	Type              string            `json:"type"`
	VersionInfo       map[string]string `json:"versionInfo"`
	IsPrivateFile     *bool             `json:"isPrivateFile"`
	CustomCoordinates *string           `json:"customCoordinates"`
	URL               string            `json:"url"`
	Thumbnail         string            `json:"thumbnail"`
	FileType          string            `json:"fileType"`
	Mime              string            `json:"mime"`
	Height            int               `json:"height"`
	Width             int               `json:"Width"`
	Size              uint64            `json:"size"`
	HasAlpha          bool              `json:"hasAlpha"`
	CustomMetadata    map[string]any    `json:"customMetadata,omitempty"`
	EmbeddedMetadata  map[string]any    `json:"embeddedMetadata"`
	CreatedAt         time.Time         `json:"createdAt"`
	UpdatedAt         time.Time         `json:"updatedAt"`
	Tags              []string          `json:"tags"`
	AITags            []AITag           `json:"AITags"`
}

// Folder represents media library Folder details.
type Folder struct {
	*File
	FolderPath string `json:"folderPath"`
}

// CreateFolderParam represents parameter to create folder api
type CreateFolderParam struct {
	FolderName       string `validate:"nonzero" json:"folderName"`
	ParentFolderPath string `validate:"nonzero" json:"parentFolderPath"`
}

// DeleteFolderParam represents parameter to delete folder api
type DeleteFolderParam struct {
	FolderPath string `validate:"nonzero" json:"folderPath"`
}

// MoveFolderParam represents parameter to move folder api
type MoveFolderParam struct {
	SourceFolderPath string `validate:"nonzero" json:"sourceFolderPath"`
	DestinationPath  string `validate:"nonzero" json:"destinationPath"`
}

// JobIDResponse respresents response struct with JobID for folder operations
type JobIDResponse struct {
	JobID string `json:"jobId"`
}

// JobStatus represents response Data to job status api
type JobStatus struct {
	JobID  string `json:"jobId"`
	Type   string `json:"type"`
	Status string `json:"status"`
}

// File represents media library File details.
func (ik *ImageKit) File(ctx context.Context, fileID string) (*http.Response, *File, error) {
	data := &File{}
	response, err := ik.HTTPClient.CallJSON(ctx, &rest.Opts{
		Method:       "GET",
		Path:         fmt.Sprintf("/files/%s/details", fileID),
		RootURL:      ik.Prefix,
		IgnoreStatus: true,
	}, nil, data)

	return response, data, err
}

// Files retrieves media library files. Filter options can be supplied as FilesOrFolderParam.
func (ik *ImageKit) Files(ctx context.Context, params FilesOrFolderParam, includeVersion bool) (*http.Response, *[]File, error) {
	var SearchQuery = `type = "file"`

	if includeVersion {
		SearchQuery = `type IN ["file", "file-version"]`
	}
	if params.SearchQuery != "" {
		SearchQuery = params.SearchQuery
	}

	parameters := url.Values{}

	parameters.Set("skip", fmt.Sprintf("%d", params.Skip))
	parameters.Set("limit", fmt.Sprintf("%d", params.Limit))
	parameters.Set("path", params.Path)
	parameters.Set("searchQuery", SearchQuery)

	data := &[]File{}

	response, err := ik.HTTPClient.CallJSON(ctx, &rest.Opts{
		Method:     "GET",
		Path:       "/files",
		RootURL:    ik.Prefix,
		Parameters: parameters,
	}, nil, data)

	return response, data, err
}

// DeleteFile removes file by FileID from media library
func (ik *ImageKit) DeleteFile(ctx context.Context, fileID string) (*http.Response, error) {
	var err error

	if fileID == "" {
		return nil, errors.New("fileID can not be empty")
	}

	response, err := ik.HTTPClient.CallJSON(ctx, &rest.Opts{
		Method:     "DELETE",
		Path:       fmt.Sprintf("/files/%s", fileID),
		RootURL:    ik.Prefix,
		NoResponse: true,
	}, nil, nil)

	return response, err
}

// Folders retrieves media library files. Filter options can be supplied as FilesOrFolderParam.
func (ik *ImageKit) Folders(ctx context.Context, params FilesOrFolderParam) (*http.Response, *[]Folder, error) {
	var SearchQuery = `type = "folder"`

	if params.SearchQuery != "" {
		SearchQuery = params.SearchQuery
	}

	parameters := url.Values{}

	parameters.Set("skip", fmt.Sprintf("%d", params.Skip))
	parameters.Set("limit", fmt.Sprintf("%d", params.Limit))
	parameters.Set("path", params.Path)
	parameters.Set("searchQuery", SearchQuery)

	data := &[]Folder{}

	resp, err := ik.HTTPClient.CallJSON(ctx, &rest.Opts{
		Method:     "GET",
		Path:       "/files",
		RootURL:    ik.Prefix,
		Parameters: parameters,
	}, nil, data)

	if err != nil {
		return resp, data, err
	}

	return resp, data, err
}

// CreateFolder creates a new folder in media library
func (ik *ImageKit) CreateFolder(ctx context.Context, param CreateFolderParam) (*http.Response, error) {
	var err error

	if err = validator.Validate(&param); err != nil {
		return nil, err
	}

	response, err := ik.HTTPClient.CallJSON(ctx, &rest.Opts{
		Method:     "POST",
		Path:       "/folder",
		RootURL:    ik.Prefix,
		NoResponse: true,
	}, param, nil)

	return response, err
}

// DeleteFolder removes the folder from media library
func (ik *ImageKit) DeleteFolder(ctx context.Context, param DeleteFolderParam) (*http.Response, error) {
	var err error

	if err = validator.Validate(&param); err != nil {
		return nil, err
	}

	response, err := ik.HTTPClient.CallJSON(ctx, &rest.Opts{
		Method:     "DELETE",
		Path:       "/folder",
		RootURL:    ik.Prefix,
		NoResponse: true,
	}, param, nil)

	return response, err
}

// MoveFolder moves given folder to new path in media library
func (ik *ImageKit) MoveFolder(ctx context.Context, param MoveFolderParam) (*http.Response, *JobIDResponse, error) {
	var err error
	var response = &JobIDResponse{}

	if err = validator.Validate(&param); err != nil {
		return nil, nil, err
	}

	resp, err := ik.HTTPClient.CallJSON(ctx, &rest.Opts{
		Method:  "PUT",
		Path:    "bulkJobs/moveFolder",
		RootURL: ik.Prefix,
	}, param, response)

	return resp, response, err
}

// BulkJobStatus retrieves the status of a bulk job by job ID.
func (ik *ImageKit) BulkJobStatus(ctx context.Context, jobID string) (*http.Response, *JobStatus, error) {
	var err error
	var response = &JobStatus{}

	if jobID == "" {
		return nil, nil, errors.New("jobId can not be blank")
	}

	resp, err := ik.HTTPClient.CallJSON(ctx, &rest.Opts{
		Method:  "GET",
		Path:    "bulkJobs/" + jobID,
		RootURL: ik.Prefix,
	}, nil, response)

	return resp, response, err
}
