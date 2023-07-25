package client

import (
	"github.com/rclone/rclone/backend/imagekit/client/api/media"
	"github.com/rclone/rclone/backend/imagekit/client/api/uploader"
	"github.com/rclone/rclone/backend/imagekit/client/config"
)

// ImageKit main struct
type ImageKit struct {
	Config   config.Configuration
	Media    *media.API
	Uploader *uploader.API
}

// NewParams is a struct to define parameters to imagekit
type NewParams struct {
	PrivateKey  string
	PublicKey   string
	UrlEndpoint string
}

// New returns ImageKit object from environment variables
func New(params NewParams) (*ImageKit, error) {
	cfg, err := config.New(config.NewParams{
		PrivateKey:  params.PrivateKey,
		PublicKey:   params.PublicKey,
		UrlEndpoint: params.UrlEndpoint,
	})

	if err != nil {
		return nil, err
	}

	return &ImageKit{
		Config: *cfg,
		Media: &media.API{
			Config:     *cfg,
			HttpClient: cfg.HttpClient,
		},
		Uploader: &uploader.API{
			Config:     *cfg,
			HttpClient: cfg.HttpClient,
		},
	}, nil
}
