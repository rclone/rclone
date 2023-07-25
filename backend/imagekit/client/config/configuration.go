// Package config defines the ImageKit configuration.
package config

import (
	"fmt"
	"net/http"
)

// Configuration is the main configuration struct.
type Configuration struct {
	Prefix         string
	UploadPrefix   string
	MetadataPrefix string
	Timeout        int64
	UploadTimeout  int64
	PrivateKey     string
	PublicKey      string
	UrlEndpoint    string
	HttpClient     *http.Client
}

// NewParams is a struct to define parameters to imagekit
type NewParams struct {
	PrivateKey  string
	PublicKey   string
	UrlEndpoint string
}

// New returns a new Configuration instance from the environment variables
func New(params NewParams) (*Configuration, error) {
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

	return NewFromParams(privateKey, publicKey, endpointUrl), nil
}

// NewFromParams returns a new Configuration instance from the provided keys and endpointUrl.
func NewFromParams(privateKey string, publicKey string, endpointUrl string) *Configuration {
	return &Configuration{
		Prefix:         "https://imagekit.io/api/v2/",
		UploadPrefix:   "https://upload.imagekit.io/api/v2/",
		MetadataPrefix: "https://api.imagekit.io/v1/",
		Timeout:        60,
		UploadTimeout:  3600,
		PrivateKey:     privateKey,
		PublicKey:      publicKey,
		UrlEndpoint:    endpointUrl,
		HttpClient:     &http.Client{},
	}
}
