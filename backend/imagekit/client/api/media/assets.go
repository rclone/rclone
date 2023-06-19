package media

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"time"

	"github.com/rclone/rclone/backend/imagekit/client/api"
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
	FileId            string            `json:"fileId"`
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

// FileResponse represents response type of File().
type FileResponse struct {
	Data File
	api.Response
}

// FilesResponse represents response type of Files().
type FilesResponse struct {
	Data []File
	api.Response
}

// Folder represents media library Folder details.
type Folder struct {
	*File
	FolderPath string `json:"folderPath"`
}

// FoldersResponse represents response type of Files() .
type FoldersResponse struct {
	Data []Folder
	api.Response
}

// File represents media library File details.
func (m *API) File(ctx context.Context, fileId string) (*FileResponse, error) {

	response := &FileResponse{}

	resp, err := m.get(ctx, fmt.Sprintf("/files/%s/details", fileId), response)

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

// Files retrieves media library files. Filter options can be supplied as FilesOrFolderParam.
func (m *API) Files(ctx context.Context, params FilesOrFolderParam, includeVersion bool) (*FilesResponse, error) {

	var SearchQuery string = `type = "file"`

	if includeVersion {
		SearchQuery = `type IN ["file", "file-version"]`
	}

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

	response := &FilesResponse{}

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

// DeleteFile removes file by FileId from media library
func (m *API) DeleteFile(ctx context.Context, fileId string) (*api.Response, error) {
	var err error
	response := &api.Response{}

	if fileId == "" {
		return nil, errors.New("fileId can not be empty")
	}

	resp, err := m.delete(ctx, "files/"+fileId, nil, response)

	if err != nil {
		return response, err
	}

	if resp.StatusCode != 204 {
		err = response.ParseError()
	}
	return response, err
}