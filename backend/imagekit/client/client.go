package client

import (
	"context"
	"fmt"

	"github.com/rclone/rclone/fs/fshttp"
	"github.com/rclone/rclone/lib/rest"
)

// ImageKit main struct
type ImageKit struct {
	Prefix        string
	UploadPrefix  string
	Timeout       int64
	UploadTimeout int64
	PrivateKey    string
	PublicKey     string
	UrlEndpoint   string
	HttpClient    *rest.Client
	ApiHeaders    map[string]string
}

// NewParams is a struct to define parameters to imagekit
type NewParams struct {
	PrivateKey  string
	PublicKey   string
	UrlEndpoint string
}

// New returns ImageKit object from environment variables
func New(ctx context.Context, params NewParams) (*ImageKit, error) {

	privateKey := params.PrivateKey
	publicKey := params.PublicKey
	endpointUrl := params.UrlEndpoint

	switch {
	case privateKey == "":
		return nil, fmt.Errorf("ImageKit.io URL endpoint is required")
	case publicKey == "":
		return nil, fmt.Errorf("ImageKit.io public key is required")
	case endpointUrl == "":
		return nil, fmt.Errorf("ImageKit.io private key is required")
	}

	client := rest.NewClient(fshttp.NewClient(ctx))

	client.SetUserPass(privateKey, "")
	client.SetHeader("Accept", "application/json")
	client.SetHeader("User-Agent", "rclone/imagekit")
	client.SetErrorHandler(ParseError)

	return &ImageKit{
		Prefix:        "https://api.imagekit.io/v2",
		UploadPrefix:  "https://upload.imagekit.io/api/v2",
		Timeout:       60,
		UploadTimeout: 3600,
		PrivateKey:    params.PrivateKey,
		PublicKey:     params.PublicKey,
		UrlEndpoint:   params.UrlEndpoint,
		HttpClient:    client,
		ApiHeaders: map[string]string{
			"Accept":     "application/json",
			"User-Agent": "rclone/imagekit",
		},
	}, nil
}
