// Package azurefiles provides an interface to Microsoft Azure Files
package azurefiles

import (
	"context"
	"errors"
	"fmt"
	"path"

	"github.com/Azure/azure-sdk-for-go/sdk/storage/azfile/directory"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azfile/file"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azfile/service"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config"
	"github.com/rclone/rclone/fs/config/configmap"
	"github.com/rclone/rclone/fs/config/configstruct"
	"github.com/rclone/rclone/lib/encoder"
)

// TODO: enable x-ms-allow-trailing-do
// TODO: length
// EncodeCtl | EncodeDel because del is defined as a CTL characater in section 2.2 of RFC 2616.
var defaultEncoder = (encoder.EncodeDoubleQuote |
	encoder.EncodeBackSlash |
	encoder.EncodeSlash |
	encoder.EncodeColon |
	encoder.EncodePipe |
	encoder.EncodeLtGt |
	encoder.EncodeAsterisk |
	encoder.EncodeQuestion |
	encoder.EncodeInvalidUtf8 |
	encoder.EncodeCtl | encoder.EncodeDel |
	encoder.EncodeDot | encoder.EncodeRightPeriod)

func init() {
	fs.Register(&fs.RegInfo{
		Name:        "azurefiles",
		Description: "Microsoft Azure Files",
		NewFs:       NewFs,
		Options: []fs.Option{{
			Name: "share_name",
			Help: `Azure Files Share Name.`,
		}, {
			Name: "connection_string",
			Help: `Azure Files Connection String.`,
		}, {
			Name: "account",
			Help: `Storage Account Name.`,
		}, {
			Name:      "key",
			Help:      `Storage Account Shared Key.`,
			Sensitive: true,
		}, {
			Name: "sas_url",
			Help: `Shared Access Signature. 
			
Works after allowing access to service, Container and Object resource types`,
			Sensitive: true,
		}, {
			Name:     config.ConfigEncoding,
			Help:     config.ConfigEncodingHelp,
			Advanced: true,
			Default:  defaultEncoder,
		}},
	})
}

// Options defines the configuration for this backend
type Options struct {
	ShareName        string
	ConnectionString string
	Account          string
	Key              string
	SASUrl           string               `config:"sas_url"`
	Enc              encoder.MultiEncoder `config:"encoding"`
}

type authenticationScheme int

const (
	accountAndKey authenticationScheme = iota
	connectionString
	sasURL
)

func authenticationSchemeFromOptions(opt *Options) (authenticationScheme, error) {
	if opt.ConnectionString != "" {
		return connectionString, nil
	} else if opt.Account != "" && opt.Key != "" {
		return accountAndKey, nil
	} else if opt.SASUrl != "" {
		return sasURL, nil
	}
	return -1, errors.New("could not determine authentication scheme from options")
}

// Factored out from NewFs so that it can be tested with opt *Options and without m configmap.Mapper
func newFsFromOptions(ctx context.Context, name, root string, opt *Options) (fs.Fs, error) {
	as, err := authenticationSchemeFromOptions(opt)
	if err != nil {
		return nil, err
	}
	var serviceClient *service.Client
	switch as {
	case connectionString:
		serviceClient, err = service.NewClientFromConnectionString(opt.ConnectionString, nil)
		if err != nil {
			return nil, err
		}
	case accountAndKey:
		skc, err := file.NewSharedKeyCredential(opt.Account, opt.Key)
		if err != nil {
			return nil, err
		}
		fileURL := fmt.Sprintf("https://%s.file.core.windows.net/%s", opt.Account, opt.ShareName)
		serviceClient, err = service.NewClientWithSharedKeyCredential(fileURL, skc, nil)
		if err != nil {
			return nil, err
		}
	case sasURL:
		if err != nil {
			return nil, fmt.Errorf("failed to parse SAS URL: %w", err)
		}
		serviceClient, err = service.NewClientWithNoCredential(opt.SASUrl, nil)
		if err != nil {
			return nil, fmt.Errorf("unable to create SAS URL client: %w", err)
		}
	}

	shareClient := serviceClient.NewShareClient(opt.ShareName)
	shareRootDirClient := shareClient.NewRootDirectoryClient()
	f := Fs{
		shareRootDirClient: shareRootDirClient,
		name:               name,
		root:               root,
		opt:                opt,
	}
	// How to check whether a file exists at this location
	_, propsErr := shareRootDirClient.NewFileClient(f.opt.Enc.FromStandardPath(root)).GetProperties(ctx, nil)
	if propsErr == nil {
		f.root = path.Dir(root)
		return &f, fs.ErrorIsFile
	}

	return &f, nil
}

// NewFs constructs an Fs from the path, container:path
//
// TODO: what happens when root is a file
func NewFs(ctx context.Context, name, root string, m configmap.Mapper) (fs.Fs, error) {
	opt := new(Options)
	err := configstruct.Set(m, opt)
	if err != nil {
		return nil, err
	}
	return newFsFromOptions(ctx, name, root, opt)
}

var listFilesAndDirectoriesOptions = &directory.ListFilesAndDirectoriesOptions{
	Include: directory.ListFilesInclude{
		Timestamps: true,
	},
}

// Fs represents a root directory inside a share. The root directory can be ""
type Fs struct {
	shareRootDirClient *directory.Client
	name               string
	root               string
	opt                *Options
}

func (c *common) String() string {
	return c.remote
}

func (c *common) Remote() string {
	return c.remote
}

// TODO: implement MimeTyper
// TODO: what heppens when update is called on Directory

type common struct {
	f      *Fs
	remote string
	properties
}
