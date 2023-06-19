// Package config defines the ImageKit configuration.
package config

import (
	"fmt"
	"net/http"

	"github.com/creasty/defaults"
)

// Configuration is the main configuration struct.
type Configuration struct {
	API   API
	Cloud Cloud
	HttpClient *http.Client
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
	cloudConf := Cloud{
		PrivateKey:  privateKey,
		PublicKey:   publicKey,
		UrlEndpoint: endpointUrl,
	}

	var api = API{}
	defaults.Set(&api)

	return &Configuration{
		Cloud: cloudConf,
		API:   api,
		HttpClient: &http.Client{},
	}
}