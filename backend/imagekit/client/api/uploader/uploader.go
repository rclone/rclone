package uploader

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"time"

	"github.com/rclone/rclone/backend/imagekit/client/api"
	"github.com/rclone/rclone/backend/imagekit/client/config"
)

// API is the upload feature main struct
type API struct {
	Config config.Configuration
	HttpClient *http.Client
}

// postFile uploads file with url.Values parameters
func (u *API) postFile(ctx context.Context, file interface{}, formParams url.Values) (*http.Response, error) {
	uploadEndpoint := api.BuildPath("files", "upload")

	switch fileValue := file.(type) {
	case string:
		// Can be URL, Base64 encoded string, etc.
		formParams.Add("file", fileValue)
		return u.postForm(ctx, uploadEndpoint, formParams)
	case io.Reader:
		return u.postIOReader(ctx, uploadEndpoint, fileValue, formParams, map[string]string{})

	default:
		return nil, errors.New("unsupported file type")
	}
}

// postIOReader uploads file using io.Reader
func (u *API) postIOReader(ctx context.Context, urlPath string, reader io.Reader, formParams url.Values, headers map[string]string) (*http.Response, error) {
	bodyBuf := new(bytes.Buffer)
	formWriter := multipart.NewWriter(bodyBuf)

	headers["Content-Type"] = formWriter.FormDataContentType()

	for key, val := range formParams {
		_ = formWriter.WriteField(key, val[0])
	}

	if flag.Lookup("test.v") != nil {
		fileName := formParams.Get("fileName")
		formParams.Set("fileName", fileName + "_test.txt")
	}

	partWriter, err := formWriter.CreateFormFile("file", formParams.Get("fileName"))
	if err != nil {
		return nil, err
	}

	if _, err = io.Copy(partWriter, reader); err != nil {
		return nil, err
	}

	if err = formWriter.Close(); err != nil {
		return nil, err
	}

	if u.Config.UploadTimeout != 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, time.Duration(u.Config.UploadTimeout)*time.Second)
		defer cancel()
	}

	return u.postBody(ctx, urlPath, bodyBuf, headers)
}

func (u *API) postBody(ctx context.Context, urlPath string, bodyBuf *bytes.Buffer, headers map[string]string) (*http.Response, error) {

	req, err := http.NewRequest(http.MethodPost,
		u.Config.UploadPrefix+urlPath,
		bodyBuf,
	)

	if err != nil {
		return nil, err
	}

	req.SetBasicAuth(u.Config.PrivateKey, "")

	for key, val := range headers {
		req.Header.Add(key, val)
	}

	req = req.WithContext(ctx)

	return u.HttpClient.Do(req)
}

func (u *API) postForm(ctx context.Context, urlPath string, formParams url.Values) (*http.Response, error) {

	bodyBuf := new(bytes.Buffer)
	writer := multipart.NewWriter(bodyBuf)

	for k, _ := range formParams {
		writer.WriteField(k, formParams.Get(k))
	}
	err := writer.Close()
	if err != nil {
		return nil, err
	}

	h := map[string]string{"Content-Type": writer.FormDataContentType()}
	ctx, cancel := context.WithTimeout(ctx, time.Duration(u.Config.Timeout)*time.Second)
	defer cancel()

	return u.postBody(ctx, urlPath, bodyBuf, h)
}