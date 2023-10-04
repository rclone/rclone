package azurefiles

import (
	"context"
	"fmt"
	"log"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/storage/azfile/directory"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azfile/service"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config"
	"github.com/rclone/rclone/fs/config/configmap"
	"github.com/rclone/rclone/fs/config/configstruct"
	"github.com/rclone/rclone/lib/encoder"
)

const (
	modTimeKey    string = "mtime"
	KB            int64  = 1024
	MB            int64  = 1024 * KB
	GB            int64  = 1024 * MB
	TB            int64  = 1024 * GB
	maxFileSize   int64  = 4 * TB
	pathSeparator string = "/"
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
			Name: "connection_string",
			Help: `Azure Files Connection String.`,
		}, {
			Name: "share_name",
			Help: `Azure Files Share Name.`,
		}, {
			Name:     config.ConfigEncoding,
			Help:     config.ConfigEncodingHelp,
			Advanced: true,
			Default:  defaultEncoder,
		}},
	})
}

type Options struct {
	ConnectionString string
	ShareName        string
	Enc              encoder.MultiEncoder `config:"encoding"`
}

// TODO: what happens when root is a file
func NewFs(ctx context.Context, name, root string, m configmap.Mapper) (fs.Fs, error) {
	opt := new(Options)
	err := configstruct.Set(m, opt)
	if err != nil {
		return nil, err
	}
	serviceClient, err := service.NewClientFromConnectionString(opt.ConnectionString, nil)
	if err != nil {
		log.Fatal("could not create service client: %w", err)
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
	log.Print(propsErr)

	return &f, nil
}

var listFilesAndDirectoriesOptions = &directory.ListFilesAndDirectoriesOptions{
	Include: directory.ListFilesInclude{
		Timestamps: true,
	},
}

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

func ensureInterfacesAreSatisfied() {
	var _ fs.Fs = (*Fs)(nil)
	var _ fs.Object = (*Object)(nil)
	var _ fs.Directory = (*Directory)(nil)
}

// TODO: implement MimeTyper
// TODO: what heppens when update is called on Directory
// TODO: what happens when remove is called on Directory

func modTimeFromMetadata(md map[string]*string) (time.Time, error) {
	tStr, ok := getCaseInvariantMetaDataValue(md, modTimeKey)
	if !ok {
		return time.Now(), fmt.Errorf("could not find key=%s in metadata", modTimeKey)
	}
	i, err := strconv.ParseInt(*tStr, 10, 64)
	if err != nil {
		return time.Now(), err
	}
	tm := time.Unix(i, 0)
	return tm, nil
}

type common struct {
	f        *Fs
	remote   string
	metaData map[string]*string
	properties
}

// returns metadata values corresponding to case independent keys
func getCaseInvariantMetaDataValue(md map[string]*string, key string) (*string, bool) {
	for k, v := range md {
		if strings.EqualFold(k, key) {
			return v, true
		}
	}
	return nil, false
}

func setCaseInvariantMetaDataValue(md map[string]*string, key string, value string) {
	keysToBeDeleted := []string{}
	for k := range md {
		if strings.EqualFold(k, key) {
			keysToBeDeleted = append(keysToBeDeleted, k)
		}
	}

	for _, k := range keysToBeDeleted {
		delete(md, k)
	}

	md[key] = &value
}
