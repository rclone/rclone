//go:build !plan9 && !solaris && !js
// +build !plan9,!solaris,!js

package oracleobjectstorage

import (
	"context"
	"crypto/rsa"
	"errors"
	"net/http"
	"os"
	"path"
	"strings"

	"github.com/oracle/oci-go-sdk/v65/common"
	"github.com/oracle/oci-go-sdk/v65/common/auth"
	"github.com/oracle/oci-go-sdk/v65/objectstorage"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/fserrors"
	"github.com/rclone/rclone/fs/fshttp"
)

func expandPath(filepath string) (expandedPath string) {
	if filepath == "" {
		return filepath
	}
	cleanedPath := path.Clean(filepath)
	expandedPath = cleanedPath
	if strings.HasPrefix(cleanedPath, "~") {
		rest := cleanedPath[2:]
		home, err := os.UserHomeDir()
		if err != nil {
			return expandedPath
		}
		expandedPath = path.Join(home, rest)
	}
	return
}

func getConfigurationProvider(opt *Options) (common.ConfigurationProvider, error) {
	switch opt.Provider {
	case instancePrincipal:
		return auth.InstancePrincipalConfigurationProvider()
	case userPrincipal:
		expandConfigFilePath := expandPath(opt.ConfigFile)
		if expandConfigFilePath != "" && !fileExists(expandConfigFilePath) {
			fs.Errorf(userPrincipal, "oci config file doesn't exist at %v", expandConfigFilePath)
		}
		return common.CustomProfileConfigProvider(expandConfigFilePath, opt.ConfigProfile), nil
	case resourcePrincipal:
		return auth.ResourcePrincipalConfigurationProvider()
	case noAuth:
		fs.Infof("client", "using no auth provider")
		return getNoAuthConfiguration()
	default:
	}
	return common.DefaultConfigProvider(), nil
}

func newObjectStorageClient(ctx context.Context, opt *Options) (*objectstorage.ObjectStorageClient, error) {
	p, err := getConfigurationProvider(opt)
	if err != nil {
		return nil, err
	}
	client, err := objectstorage.NewObjectStorageClientWithConfigurationProvider(p)
	if err != nil {
		fs.Errorf(opt.Provider, "failed to create object storage client, %v", err)
		return nil, err
	}
	if opt.Region != "" {
		client.SetRegion(opt.Region)
	}
	if opt.Endpoint != "" {
		client.Host = opt.Endpoint
	}
	modifyClient(ctx, opt, &client.BaseClient)
	return &client, err
}

func fileExists(filePath string) bool {
	if _, err := os.Stat(filePath); errors.Is(err, os.ErrNotExist) {
		return false
	}
	return true
}

func modifyClient(ctx context.Context, opt *Options, client *common.BaseClient) {
	client.HTTPClient = getHTTPClient(ctx)
	if opt.Provider == noAuth {
		client.Signer = getNoAuthSigner()
	}
}

// getClient makes http client according to the global options
// this has rclone specific options support like dump headers, body etc.
func getHTTPClient(ctx context.Context) *http.Client {
	return fshttp.NewClient(ctx)
}

var retryErrorCodes = []int{
	408, // Request Timeout
	429, // Rate exceeded.
	500, // Get occasional 500 Internal Server Error
	503, // Service Unavailable
	504, // Gateway Time-out
}

func shouldRetry(ctx context.Context, resp *http.Response, err error) (bool, error) {
	if fserrors.ContextError(ctx, &err) {
		return false, err
	}
	// If this is an ocierr object, try and extract more useful information to determine if we should retry
	if ociError, ok := err.(common.ServiceError); ok {
		// Simple case, check the original embedded error in case it's generically retryable
		if fserrors.ShouldRetry(err) {
			return true, err
		}
		// If it is a timeout then we want to retry that
		if ociError.GetCode() == "RequestTimeout" {
			return true, err
		}
	}
	// Ok, not an oci error, check for generic failure conditions
	return fserrors.ShouldRetry(err) || fserrors.ShouldRetryHTTP(resp, retryErrorCodes), err
}

func getNoAuthConfiguration() (common.ConfigurationProvider, error) {
	return &noAuthConfigurator{}, nil
}

func getNoAuthSigner() common.HTTPRequestSigner {
	return &noAuthSigner{}
}

type noAuthConfigurator struct {
}

type noAuthSigner struct {
}

func (n *noAuthSigner) Sign(*http.Request) error {
	return nil
}

func (n *noAuthConfigurator) PrivateRSAKey() (*rsa.PrivateKey, error) {
	return nil, nil
}

func (n *noAuthConfigurator) KeyID() (string, error) {
	return "", nil
}

func (n *noAuthConfigurator) TenancyOCID() (string, error) {
	return "", nil
}

func (n *noAuthConfigurator) UserOCID() (string, error) {
	return "", nil
}

func (n *noAuthConfigurator) KeyFingerprint() (string, error) {
	return "", nil
}

func (n *noAuthConfigurator) Region() (string, error) {
	return "", nil
}

func (n *noAuthConfigurator) AuthType() (common.AuthConfig, error) {
	return common.AuthConfig{
		AuthType:         common.UnknownAuthenticationType,
		IsFromConfigFile: false,
		OboToken:         nil,
	}, nil
}

// Check the interfaces are satisfied
var (
	_ common.ConfigurationProvider = &noAuthConfigurator{}
	_ common.HTTPRequestSigner     = &noAuthSigner{}
)
