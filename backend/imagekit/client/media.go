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

// FileType represents all, image or non-image etc type in request filter.
type FileType string

const (
	All      FileType = "all"
	Image    FileType = "image"
	NonImage FileType = "non-image"
)

// FilesOrFolderParam struct is a parameter type to ListFiles() function to search / list media library files.
type FilesOrFolderParam struct {
	Path        string `json:"path,omitempty"`
	Limit       int    `json:"limit,omitempty"`
	Skip        int    `json:"skip,omitempty"`
	SearchQuery string `json:"searchQuery,omitempty"`
}

type AITag struct {
	Name       string `json:"name"`
	Confidence string `json:"confidence"`
	Source     string `json:"source"`
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
	Url               string            `json:"url"`
	Thumbnail         string            `json:"thumbnail"`
	FileType          FileType          `json:"fileType"`
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

// // FileResponse represents response type of File().
// type FileResponse struct {
// 	Data File
// 	Response
// }

// // FilesResponse represents response type of Files().
// type FilesResponse struct {
// 	Data []File
// 	Response
// }

// Folder represents media library Folder details.
type Folder struct {
	*File
	FolderPath string `json:"folderPath"`
}

// // FoldersResponse represents response type of Files() .
// type FoldersResponse struct {
// 	Data []Folder
// 	Response
// }

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

// JobIdResponse respresents response struct with JobId for folder operations
type JobIdResponse struct {
	JobId string `json:"jobId"`
}

// // FolderResponse respresents struct for response to move folder api.
// type FolderResponse struct {
// 	Data JobIdResponse
// 	Response
// }

// JobStatus represents response Data to job status api
type JobStatus struct {
	JobId  string `json:"jobId"`
	Type   string `json:"type"`
	Status string `json:"status"`
}

// // JobStatusResponse represents response to job status api
// type JobStatusResponse struct {
// 	Data JobStatus
// 	Response
// }

// File represents media library File details.
func (ik *ImageKit) File(ctx context.Context, fileId string) (*http.Response, *File, error) {
	data := &File{}
	response, err := ik.HTTPClient.CallJSON(ctx, &rest.Opts{
		Method:       "GET",
		Path:         fmt.Sprintf("/files/%s/details", fileId),
		RootURL:      ik.Prefix,
		IgnoreStatus: true,
	}, nil, data)

	if err != nil {
		return response, data, err
	}

	if response.StatusCode != 200 {
		err = fmt.Errorf("http error %d: %v", response.StatusCode, response.Status)
	}

	return response, data, err
}

// Files retrieves media library files. Filter options can be supplied as FilesOrFolderParam.
func (ik *ImageKit) Files(ctx context.Context, params FilesOrFolderParam, includeVersion bool) (*http.Response, *[]File, error) {
	var SearchQuery string = `type = "file"`

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

	if err != nil {
		return response, data, err
	}

	if response.StatusCode != 200 {
		err = ParseError(response)
	}

	return response, data, err
}

// DeleteFile removes file by FileID from media library
func (ik *ImageKit) DeleteFile(ctx context.Context, fileId string) (*http.Response, error) {
	var err error

	if fileId == "" {
		return nil, errors.New("fileId can not be empty")
	}

	response, err := ik.HTTPClient.CallJSON(ctx, &rest.Opts{
		Method:  "DELETE",
		Path:    fmt.Sprintf("/files/%s", fileId),
		RootURL: ik.Prefix,
	}, nil, nil)

	if err != nil {
		return response, err
	}

	if response.StatusCode != 204 {
		err = ParseError(response)
	}

	return response, err
}

// Folders retrieves media library files. Filter options can be supplied as FilesOrFolderParam.
func (ik *ImageKit) Folders(ctx context.Context, params FilesOrFolderParam) (*http.Response, *[]Folder, error) {
	var SearchQuery string = `type = "folder"`

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

	if resp.StatusCode != 200 {
		err = ParseError(resp)
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
		Method:  "POST",
		Path:    "/folder",
		RootURL: ik.Prefix,
	}, param, nil)

	if err != nil {
		return response, err
	}

	if response.StatusCode != 200 {
		err = ParseError(response)
	}

	return response, err
}

// DeleteFolder removes the folder from media library
func (ik *ImageKit) DeleteFolder(ctx context.Context, param DeleteFolderParam) (*http.Response, error) {
	var err error

	if err = validator.Validate(&param); err != nil {
		return nil, err
	}

	response, err := ik.HTTPClient.CallJSON(ctx, &rest.Opts{
		Method:  "DELETE",
		Path:    "/folder",
		RootURL: ik.Prefix,
	}, param, nil)

	if err != nil {
		return response, err
	}

	if response.StatusCode != 204 {
		err = ParseError(response)
	}
	return response, err
}

// MoveFolder moves given folder to new aath in media library
func (ik *ImageKit) MoveFolder(ctx context.Context, param MoveFolderParam) (*http.Response, *JobIdResponse, error) {
	var err error
	var response = &JobIdResponse{}

	if err = validator.Validate(&param); err != nil {
		return nil, nil, err
	}

	resp, err := ik.HTTPClient.CallJSON(ctx, &rest.Opts{
		Method:  "PUT",
		Path:    "bulkJobs/moveFolder",
		RootURL: ik.Prefix,
	}, param, response)

	if err != nil {
		return resp, response, err
	}

	if resp.StatusCode != 200 {
		err = ParseError(resp)
	}

	return resp, response, err
}

func (ik *ImageKit) BulkJobStatus(ctx context.Context, jobId string) (*http.Response, *JobStatus, error) {
	var err error
	var response = &JobStatus{}

	if jobId == "" {
		return nil, nil, errors.New("jobId can not be blank")
	}

	resp, err := ik.HTTPClient.CallJSON(ctx, &rest.Opts{
		Method:  "GET",
		Path:    "bulkJobs/" + jobId,
		RootURL: ik.Prefix,
	}, nil, response)

	if err != nil {
		return resp, response, err
	}

	if resp.StatusCode != 200 {
		err = ParseError(resp)
	}

	return resp, response, err
}
