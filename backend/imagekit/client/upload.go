package client

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/rclone/rclone/lib/rest"
)

// UploadParam defines upload parameters
type UploadParam struct {
	FileName                string         `json:"fileName"`
	UseUniqueFileName       *bool          `json:"useUniqueFileName,omitempty"`
	Tags                    string         `json:"tags,omitempty"`
	Folder                  string         `json:"folder,omitempty"`        // default value:  /
	IsPrivateFile           *bool          `json:"isPrivateFile,omitempty"` // default: false
	CustomCoordinates       string         `json:"customCoordinates,omitempty"`
	ResponseFields          string         `json:"responseFields,omitempty"`
	WebhookUrl              string         `json:"webhookUrl,omitempty"`
	OverwriteFile           *bool          `json:"overwriteFile,omitempty"`
	OverwriteAITags         *bool          `json:"overwriteAITags,omitempty"`
	OverwriteTags           *bool          `json:"overwriteTags,omitempty"`
	OverwriteCustomMetadata *bool          `json:"overwriteCustomMetadata,omitempty"`
	CustomMetadata          map[string]any `json:"customMetadata,omitempty"`
}

type UploadResult struct {
	FileId       string            `json:"fileId"`
	Name         string            `json:"name"`
	Url          string            `json:"url"`
	ThumbnailUrl string            `json:"thumbnailUrl"`
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

	formParams, err := StructToParams(param)

	if err != nil {
		return nil, nil, err
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

	resp, err := ik.HttpClient.CallJSON(ctx, &opts, nil, response)

	if err != nil {
		return resp, response, err
	}

	if resp.StatusCode != 200 {
		err = ParseError(resp)
	}

	return resp, response, err
}
