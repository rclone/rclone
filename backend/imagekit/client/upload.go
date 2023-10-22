package client

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"github.com/rclone/rclone/lib/rest"
)

// UploadParam defines upload parameters
type UploadParam struct {
	FileName      string `json:"fileName"`
	Folder        string `json:"folder,omitempty"` // default value:  /
	Tags          string `json:"tags,omitempty"`
	IsPrivateFile *bool  `json:"isPrivateFile,omitempty"` // default: false
}

// UploadResult defines the response structure for the upload API
type UploadResult struct {
	FileID       string            `json:"fileId"`
	Name         string            `json:"name"`
	URL          string            `json:"url"`
	ThumbnailURL string            `json:"thumbnailUrl"`
	Height       int               `json:"height"`
	Width        int               `json:"Width"`
	Size         uint64            `json:"size"`
	FilePath     string            `json:"filePath"`
	AITags       []map[string]any  `json:"AITags"`
	VersionInfo  map[string]string `json:"versionInfo"`
}

// Upload uploads an asset to a imagekit account.
//
// The asset can be:
//   - the actual data (io.Reader)
//   - the Data URI (Base64 encoded), max ~60 MB (62,910,000 chars)
//   - the remote FTP, HTTP or HTTPS URL address of an existing file
//
// https://docs.imagekit.io/api-reference/upload-file-api/server-side-file-upload
func (ik *ImageKit) Upload(ctx context.Context, file io.Reader, param UploadParam) (*http.Response, *UploadResult, error) {
	var err error

	if param.FileName == "" {
		return nil, nil, errors.New("Upload: Filename is required")
	}

	// Initialize URL values
	formParams := url.Values{}

	formParams.Add("useUniqueFileName", fmt.Sprint(false))

	// Add individual fields to URL values
	if param.FileName != "" {
		formParams.Add("fileName", param.FileName)
	}

	if param.Tags != "" {
		formParams.Add("tags", param.Tags)
	}

	if param.Folder != "" {
		formParams.Add("folder", param.Folder)
	}

	if param.IsPrivateFile != nil {
		formParams.Add("isPrivateFile", fmt.Sprintf("%v", *param.IsPrivateFile))
	}

	response := &UploadResult{}

	formReader, contentType, _, err := rest.MultipartUpload(ctx, file, formParams, "file", param.FileName)

	if err != nil {
		return nil, nil, fmt.Errorf("failed to make multipart upload: %w", err)
	}

	opts := rest.Opts{
		Method:      "POST",
		Path:        "/files/upload",
		RootURL:     ik.UploadPrefix,
		Body:        formReader,
		ContentType: contentType,
	}

	resp, err := ik.HTTPClient.CallJSON(ctx, &opts, nil, response)

	if err != nil {
		return resp, response, err
	}

	return resp, response, err
}
