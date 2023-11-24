// Package client provides a client for interacting with the ImageKit API.
package client

import (
	"context"
	"fmt"

	"github.com/rclone/rclone/fs"
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
	URLEndpoint   string
	HTTPClient    *rest.Client
}

// NewParams is a struct to define parameters to imagekit
type NewParams struct {
	PrivateKey  string
	PublicKey   string
	URLEndpoint string
}

// New returns ImageKit object from environment variables
func New(ctx context.Context, params NewParams) (*ImageKit, error) {

	privateKey := params.PrivateKey
	publicKey := params.PublicKey
	endpointURL := params.URLEndpoint

	switch {
	case privateKey == "":
		return nil, fmt.Errorf("ImageKit.io URL endpoint is required")
	case publicKey == "":
		return nil, fmt.Errorf("ImageKit.io public key is required")
	case endpointURL == "":
		return nil, fmt.Errorf("ImageKit.io private key is required")
	}

	cliCtx, cliCfg := fs.AddConfig(ctx)

	cliCfg.UserAgent = "rclone/imagekit"
	client := rest.NewClient(fshttp.NewClient(cliCtx))

	client.SetUserPass(privateKey, "")
	client.SetHeader("Accept", "application/json")

	return &ImageKit{
		Prefix:        "https://api.imagekit.io/v2",
		UploadPrefix:  "https://upload.imagekit.io/api/v2",
		Timeout:       60,
		UploadTimeout: 3600,
		PrivateKey:    params.PrivateKey,
		PublicKey:     params.PublicKey,
		URLEndpoint:   params.URLEndpoint,
		HTTPClient:    client,
	}, nil
}
