package media

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"

	"github.com/rclone/rclone/backend/imagekit/client/api"
	"gopkg.in/validator.v2"
)

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

// FolderResponse respresents struct for response to move folder api.
type FolderResponse struct {
	Data JobIdResponse
	api.Response
}

// JobStatus represents response Data to job status api
type JobStatus struct {
	JobId  string `json:"jobId"`
	Type   string `json:"type"`
	Status string `json:"status"`
}

// JobStatusResponse represents response to job status api
type JobStatusResponse struct {
	Data JobStatus
	api.Response
}

// Folders retrieves media library files. Filter options can be supplied as FilesOrFolderParam.
func (m *API) Folders(ctx context.Context, params FilesOrFolderParam) (*FoldersResponse, error) {

	var SearchQuery string = `type = "folder"`

	if params.SearchQuery != "" {
		SearchQuery = params.SearchQuery
	}

	values := url.Values{}

	values.Set("skip", fmt.Sprintf("%d", params.Skip))
	values.Set("limit", fmt.Sprintf("%d", params.Limit))
	values.Set("path", params.Path)
	values.Set("searchQuery", SearchQuery)

	var query = values.Encode()

	if query != "" {
		query = "?" + query
	}

	response := &FoldersResponse{}

	resp, err := m.get(ctx, "files"+query, response)

	if err != nil {
		return response, err
	}

	if resp.StatusCode != 200 {
		err = response.ParseError()
	} else {
		err = json.Unmarshal(response.Body(), &response.Data)
	}

	return response, err
}

// CreateFolder creates a new folder in media library
func (m *API) CreateFolder(ctx context.Context, param CreateFolderParam) (*api.Response, error) {
	var err error
	var response = &api.Response{}

	if err = validator.Validate(&param); err != nil {
		return nil, err
	}

	resp, err := m.post(ctx, "folder", &param, response)

	if err != nil {
		return response, err
	}

	if resp.StatusCode != 200 {
		err = response.ParseError()
	}

	return response, err
}

// DeleteFolder removes the folder from media library
func (m *API) DeleteFolder(ctx context.Context, param DeleteFolderParam) (*api.Response, error) {
	var err error
	var response = &api.Response{}

	if err = validator.Validate(&param); err != nil {
		return nil, err
	}

	resp, err := m.delete(ctx, "folder", &param, response)

	if err != nil {
		return response, err
	}

	if resp.StatusCode != 204 {
		err = response.ParseError()
	}
	return response, err
}

// MoveFolder moves given folder to new aath in media library
func (m *API) MoveFolder(ctx context.Context, param MoveFolderParam) (*FolderResponse, error) {
	var err error
	var response = &FolderResponse{}

	if err = validator.Validate(&param); err != nil {
		return nil, err
	}

	resp, err := m.post(ctx, "bulkJobs/moveFolder", &param, response)

	if err != nil {
		return response, err
	}

	if resp.StatusCode != 200 {
		err = response.ParseError()
	} else {
		err = json.Unmarshal(response.Body(), &response.Data)
	}
	return response, err
}

func (m *API) BulkJobStatus(ctx context.Context, jobId string) (*JobStatusResponse, error) {
	var err error
	var response = &JobStatusResponse{}

	if jobId == "" {
		return nil, errors.New("jobId can not be blank")
	}

	resp, err := m.get(ctx, "bulkJobs/"+jobId, response)

	if err != nil {
		return response, err
	}

	if resp.StatusCode != 200 {
		err = response.ParseError()
	} else {
		err = json.Unmarshal(response.Body(), &response.Data)
	}
	return response, err
}
