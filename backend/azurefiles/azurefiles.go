package azurefiles

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"path/filepath"
	"strconv"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/storage/azfile/directory"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azfile/file"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azfile/fileerror"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azfile/service"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config/configmap"
	"github.com/rclone/rclone/fs/config/configstruct"
	"github.com/rclone/rclone/fs/hash"
)

const (
	modTimeKey  string = "mtime"
	KB          int64  = 1024
	MB          int64  = 1024 * KB
	GB          int64  = 1024 * MB
	TB          int64  = 1024 * GB
	maxFileSize int64  = 4 * TB
)

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
		}},
	})
}

type Options struct {
	ConnectionString string
	ShareName        string
}

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
	rootDirClient := shareClient.NewRootDirectoryClient()
	c := Client{
		RootDirClient: rootDirClient,
		name:          name,
		root:          root,
	}
	return &c, nil
}

var listFilesAndDirectoriesOptions = &directory.ListFilesAndDirectoriesOptions{
	Include: directory.ListFilesInclude{
		Timestamps: true,
	},
}

type Client struct {
	RootDirClient *directory.Client
	name          string
	root          string
}

// type ObjectInfo struct {
// 	DirEntry
// 	c *Client
// }

// TODO: return ErrDirNotFound if dir not found
// TODO: handle case regariding "" and "/". I remember reading about them somewhere
func (dc *Client) List(ctx context.Context, dirPath string) (fs.DirEntries, error) {
	var entries fs.DirEntries
	subDirClient := dc.RootDirClient.NewSubdirectoryClient(dirPath)
	_, err := subDirClient.GetProperties(ctx, nil)
	if err != nil {
		return entries, fs.ErrorDirNotFound
	}
	pager := subDirClient.NewListFilesAndDirectoriesPager(listFilesAndDirectoriesOptions)
	for pager.More() {
		resp, err := pager.NextPage(ctx)
		if err != nil {
			return entries, err
		}
		for _, dir := range resp.Segment.Directories {
			de := Object{
				remote:        filepath.Join(dirPath, *dir.Name),
				contentLength: nil,
			}
			entries = append(entries, &de)
		}

		for _, f := range resp.Segment.Files {
			de := Object{
				remote:        filepath.Join(dirPath, *f.Name),
				contentLength: f.Properties.ContentLength,
			}
			entries = append(entries, &de)
		}
	}
	return entries, nil

}

// Inspired by azureblob store, this initiates a network request and returns an error if objec is not found
// TODO: when does the interface expect this function to return an interface
func (c *Client) NewObject(ctx context.Context, remote string) (fs.Object, error) {
	fileClient := c.RootDirClient.NewFileClient(remote)
	resp, err := fileClient.GetProperties(ctx, nil)
	if fileerror.HasCode(err, fileerror.ResourceNotFound) {
		return nil, fs.ErrorObjectNotFound
	}

	ob := Object{
		c:             c,
		remote:        remote,
		contentLength: resp.ContentLength,
		metaData:      resp.Metadata,
	}
	return &ob, nil
}

func (dc *Client) Mkdir(ctx context.Context, dirPath string) error {
	if dirPath == "" {
		return nil
	}
	dirClient := dc.RootDirClient.NewSubdirectoryClient(dirPath)
	_, err := dirClient.Create(ctx, nil)
	if err != nil {
		if fileerror.HasCode(err, fileerror.ResourceAlreadyExists) {
			return nil
		}
		return fmt.Errorf("unable to MkDir: %w", err)
	}
	return nil
}

// should return error if the directory is not empty or does not exist
func (c *Client) Rmdir(ctx context.Context, dirPath string) error {
	if dirPath == "" {
		return nil
	}
	dirClient := c.RootDirClient.NewSubdirectoryClient(dirPath)
	_, err := dirClient.Delete(ctx, nil)
	if err != nil {
		if fileerror.HasCode(err, fileerror.DirectoryNotEmpty) {
			return fmt.Errorf("cannot rmdir dir=\"%s\" is not empty : %w", dirPath, err)
		} else if fileerror.HasCode(err, fileerror.ResourceNotFound) {
			return fmt.Errorf("cannot rmdir dir=\"%s\" not found : %w", dirPath, err)
		}
		return fmt.Errorf("could not rmdir dir=\"%s\" : %w", dirPath, err)
	}
	return nil

}

// Copied mostly from local filesystem
// TODO: file system options
// TODO: when file.CLient.Creat is being used, provide HTTP headesr such as content type and content MD5
func (c *Client) Put(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error) {
	// size and modtime are important, what about md5 hashes
	fileSize := maxFileSize
	if src.Size() >= 0 {
		fileSize = src.Size()
	}

	fc := c.RootDirClient.NewFileClient(src.Remote())

	_, err := fc.Create(ctx, fileSize, nil)
	if err != nil {
		return nil, fmt.Errorf("unable to create file : %w", err)
	}
	if err := fc.UploadStream(ctx, in, nil); err != nil {
		return nil, fmt.Errorf("azurefiles: error uploading as stream : %w", err)
	}

	modTimeVal := fmt.Sprintf("%d", src.ModTime(ctx).Unix())
	metaData := make(map[string]*string)
	metaData[modTimeKey] = &modTimeVal
	metaDataOptions := file.SetMetadataOptions{
		Metadata: metaData,
	}
	o := &Object{
		remote: src.Remote(),
		c:      c,
	}
	if _, err := fc.SetMetadata(ctx, &metaDataOptions); err != nil {
		return o, fmt.Errorf("azurefiles: error setting metadata : %w", err)
	}
	o.metaData = metaData
	return o, nil
}

func (c *Client) Name() string {
	return c.name
}

func (c *Client) Root() string {
	return c.root
}

func (c *Client) String() string {
	return fmt.Sprintf("azurefiles root '%s'", c.root)
}

// One second. FileREST API times are in RFC1123 which in the example shows a precision of seconds
// Source: https://learn.microsoft.com/en-us/rest/api/storageservices/representation-of-date-time-values-in-headers
func (c *Client) Precision() time.Duration {
	return 1
}

// MD5: since it is listed as header in the response for file properties
// Source: https://learn.microsoft.com/en-us/rest/api/storageservices/get-file-properties
func (c *Client) Hashes() hash.Set {
	return hash.NewHashSet(hash.MD5)
}

// TODO: what are features. implement them
func (c *Client) Features() *fs.Features {
	return &fs.Features{}
}

// TODO: remove fileClient property
// TODO: are all three, name, parentDir and remote required?
type Object struct {
	c             *Client
	remote        string
	contentLength *int64
	metaData      map[string]*string
}

func (o *Object) fileClient() *file.Client {
	return o.c.RootDirClient.NewFileClient(o.remote)
}

// TODO: change the modTime property on the local object as well
func (o *Object) SetModTime(ctx context.Context, t time.Time) error {
	tStr := fmt.Sprintf("%d", t.Unix())
	if o.metaData == nil {
		o.metaData = make(map[string]*string)
	}

	o.metaData[modTimeKey] = &tStr
	metaDataOptions := file.SetMetadataOptions{
		Metadata: o.metaData,
	}
	_, err := o.fileClient().SetMetadata(ctx, &metaDataOptions)
	if err != nil {
		return fmt.Errorf("unable to SetModTime on remote=\"%s\" : %w", o.remote, err)
	}
	return nil
}

func (o *Object) Remove(ctx context.Context) error {
	// TODO: should the options for delete not be nil. Depends on behaviour expected by consumers
	if _, err := o.fileClient().Delete(ctx, nil); err != nil {
		return fmt.Errorf("unable to delete remote=\"%s\" : %w", o.remote, err)
	}
	return nil
}

// TODO: implement options. understand purpose of options
func (o *Object) Open(ctx context.Context, options ...fs.OpenOption) (io.ReadCloser, error) {
	resp, err := o.fileClient().DownloadStream(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("could not open remote=\"%s\" : %w", o.remote, err)
	}
	return resp.Body, nil
}

// TODO: implement options. understand purpose of options. what is the purpose of src objectInfo.
// TODO: set metadata options from src. Hint at the local backend
func (o *Object) Update(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) error {
	// TODO: File upload options should be included. Atleast two options is important:= Concurrency, Chunksize
	// TODO: content MD5 not set
	if err := o.fileClient().UploadStream(ctx, in, nil); err != nil {
		return fmt.Errorf("unable to upload. cannot upload stream remote=\"%s\" : %w", o.remote, err)
	}

	// Set the mtime. copied from all/local.go rclone backend
	if err := o.SetModTime(ctx, src.ModTime(ctx)); err != nil {
		return fmt.Errorf("unable to upload. cannot setModTime on remote=\"%s\" : %w", o.remote, err)
	}
	return nil

}

func (o *Object) String() string {
	return o.remote
}

func (o *Object) Remote() string {
	return o.remote
}

func (o *Object) ModTime(context.Context) time.Time {
	if o.metaData[modTimeKey] == nil {
		return time.Now() // TODO: what should this be if modTime does not exist
	}
	tStr := o.metaData[modTimeKey]
	i, err := strconv.ParseInt(*tStr, 10, 64)
	if err != nil {
		log.Println("could not parse timestamp to determine modTime")
		return time.Now()
	}
	tm := time.Unix(i, 0)
	return tm

}

func (o *Object) Size() int64 {
	return *o.contentLength
}

// TODO: make this readonly
func (o *Object) Fs() fs.Info {
	return o.c
}

// TODO: implement
func (o *Object) Hash(ctx context.Context, ty hash.Type) (string, error) {
	return "", errors.New("ObjectInfo.Hash not implemented")
}

// TODO: what does this mean?
func (o *Object) Storable() bool {
	return true
}

func ensureInterfacesAreSatisfied() {
	var _ fs.Fs = (*Client)(nil)
	var _ fs.Object = (*Object)(nil)
}

// func (o *Object) SetModTime(ctx context.Context, modTime time.Time) error {
// 	// Make sure o.meta is not nil
// 	// metaData :=
// 	options := file.SetMetadataOptions{
// 		Metadata: ,
// 	}
// 	o.fileClient.SetMetadata(ctx, options)
// 	// if o.meta == nil {
// 	// 	o.meta = make(map[string]string, 1)
// 	// }
// 	// // Set modTimeKey in it
// 	// o.meta[modTimeKey] = modTime.Format(timeFormatOut)

// 	// blb := o.getBlobSVC()
// 	// opt := blob.SetMetadataOptions{}
// 	// err := o.fs.pacer.Call(func() (bool, error) {
// 	// 	_, err := blb.SetMetadata(ctx, o.getMetadata(), &opt)
// 	// 	return o.fs.shouldRetry(ctx, err)
// 	// })
// 	// if err != nil {
// 	// 	return err
// 	// }
// 	// o.modTime = modTime
// 	// return nil
// }
