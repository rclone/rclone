package uploader

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/rclone/rclone/backend/imagekit/client/api"
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

type UploadResponse struct {
	Data UploadResult
	api.Response
}

// Upload uploads an asset to a imagekit account.
//
// The asset can be:
//   - the actual data (io.Reader)
//   - the Data URI (Base64 encoded), max ~60 MB (62,910,000 chars)
//   - the remote FTP, HTTP or HTTPS URL address of an existing file
//
// https://docs.imagekit.io/api-reference/upload-file-api/server-side-file-upload
func (u *API) Upload(ctx context.Context, file interface{}, param UploadParam) (*UploadResponse, error) {
	var err error

	if param.FileName == "" {
		return nil, errors.New("Upload: Filename is required")
	}

	formParams, err := api.StructToParams(param)

	if err != nil {
		return nil, err
	}

	response := &UploadResponse{}

	resp, err := u.postFile(ctx, file, formParams)
	defer api.DeferredBodyClose(resp)

	api.SetResponseMeta(resp, response)

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
