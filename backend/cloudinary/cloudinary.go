package cloudinary

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"path"
	"strings"
	"time"

	"github.com/cloudinary/cloudinary-go/v2"
	"github.com/cloudinary/cloudinary-go/v2/api/admin"
	"github.com/cloudinary/cloudinary-go/v2/api/admin/search"
	"github.com/cloudinary/cloudinary-go/v2/api/uploader"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config/configmap"
	"github.com/rclone/rclone/fs/config/configstruct"
	"github.com/rclone/rclone/fs/hash"
	"github.com/rclone/rclone/lib/pacer"
)

// Register with Fs
func init() {
	fs.Register(&fs.RegInfo{
		Name:        "cloudinary",
		Description: "Cloudinary",
		NewFs:       NewFs,
		Options: []fs.Option{
			{
				Name:     "cloud_name",
				Help:     "Cloudinary Cloud Name",
				Required: true,
			},
			{
				Name:     "api_key",
				Help:     "Cloudinary API Key",
				Required: true,
			},
			{
				Name:       "api_secret",
				Help:       "Cloudinary API Secret",
				Required:   true,
				IsPassword: true,
			},
			{
				Name: "upload_preset",
				Help: "Upload Preset to use for upload",
			},
		},
	})
}

// Options defines the configuration for this backend
type Options struct {
	CloudName    string `config:"cloud_name"`
	APIKey       string `config:"api_key"`
	APISecret    string `config:"api_secret"`
	UploadPreset string `config:"upload_preset"`
}

// Fs represents a remote cloudinary server
type Fs struct {
	name     string
	root     string
	opt      Options
	features *fs.Features
	pacer    *pacer.Pacer
	cld      *cloudinary.Cloudinary
}

// Object describes a cloudinary object
type Object struct {
	fs      *Fs
	remote  string
	size    int64
	modTime time.Time
	url     string
	md5sum  string
}

func (o *Object) Hash(ctx context.Context, ty hash.Type) (string, error) {
	if ty != hash.MD5 {
		return "", hash.ErrUnsupported
	}
	return o.md5sum, nil
}

func (o *Object) String() string {
	if o == nil {
		return "<nil>"
	}
	return o.remote
}

// NewFs constructs an Fs from the path, bucket:path
func NewFs(ctx context.Context, name string, root string, m configmap.Mapper) (fs.Fs, error) {
	opt := new(Options)
	err := configstruct.Set(m, opt)
	if err != nil {
		return nil, err
	}

	// Initialize the Cloudinary client
	cld, err := cloudinary.NewFromParams(opt.CloudName, opt.APIKey, opt.APISecret)
	if err != nil {
		return nil, fmt.Errorf("failed to create Cloudinary client: %w", err)
	}

	f := &Fs{
		name: name,
		root: root,
		opt:  *opt,
		cld:  cld,
	}

	f.features = (&fs.Features{
		CaseInsensitive:         false,
		CanHaveEmptyDirectories: true,
		DuplicateFiles:          true,
	}).Fill(ctx, f)

	return f, nil
}

// Name of the remote (as passed into NewFs)
func (f *Fs) Name() string {
	return f.name
}

// Root of the remote (as passed into NewFs)
func (f *Fs) Root() string {
	return f.root
}

// String converts this Fs to a string
func (f *Fs) String() string {
	return fmt.Sprintf("Cloudinary root '%s'", f.root)
}

// Features returns the optional features of this Fs
func (f *Fs) Features() *fs.Features {
	return f.features
}

// List the objects and directories in dir into entries.
func (f *Fs) List(ctx context.Context, dir string) (fs.DirEntries, error) {
	remotePrefix := path.Join(f.root, dir)
	if remotePrefix != "" && !strings.HasSuffix(remotePrefix, "/") {
		remotePrefix += "/"
	}

	var entries fs.DirEntries
	dirs := make(map[string]struct{})
	nextCursor := ""

	for {
		// user the folders api to list folders.
		folderParams := admin.SubFoldersParams{
			Folder:     remotePrefix,
			MaxResults: 500,
		}
		if nextCursor != "" {
			folderParams.NextCursor = nextCursor
		}

		results, err := f.cld.Admin.SubFolders(ctx, folderParams)
		if err != nil {
			return nil, fmt.Errorf("failed to list subfolders: %w", err)
		}
		if results.Error.Message != "" {
			if strings.HasPrefix(results.Error.Message, "Can't find folder with path") {
				return nil, fs.ErrorDirNotFound
			}

			return nil, fmt.Errorf("failed to list subfolders: %s", results.Error.Message)
		}

		for _, folder := range results.Folders {
			relativePath := strings.TrimPrefix(folder.Path, remotePrefix)
			parts := strings.Split(relativePath, "/")

			// It's a directory
			dirName := parts[0]
			if _, found := dirs[dirName]; !found {
				d := fs.NewDir(path.Join(dir, dirName), time.Now())
				entries = append(entries, d)
				dirs[dirName] = struct{}{}
			}
		}
		// Break if there are no more results
		if results.NextCursor == "" {
			break
		}
		nextCursor = results.NextCursor
	}

	for {
		// Use the assets.AssetsByAssetFolder API to list assets
		assetsParams := admin.AssetsByAssetFolderParams{
			AssetFolder: remotePrefix,
			MaxResults:  500,
		}
		if nextCursor != "" {
			assetsParams.NextCursor = nextCursor
		}

		results, err := f.cld.Admin.AssetsByAssetFolder(ctx, assetsParams)
		if err != nil {
			return nil, fmt.Errorf("failed to list assets: %w", err)
		}

		for _, asset := range results.Assets {
			remote := asset.DisplayName
			if dir != "" {
				remote = path.Join(dir, asset.DisplayName)
			}
			o := &Object{
				fs:      f,
				remote:  remote,
				size:    int64(asset.Bytes),
				modTime: asset.CreatedAt,
				url:     asset.SecureURL,
			}
			entries = append(entries, o)
		}

		// Break if there are no more results
		if results.NextCursor == "" {
			break
		}
		nextCursor = results.NextCursor
	}

	return entries, nil
}

// getCLDAsset finds the asset at Cloudinary. If it can't be found it returns the error fs.ErrorObjectNotFound.
func (f *Fs) getCLDAsset(ctx context.Context, remote string) (*admin.SearchAsset, error) {
	// Use the Search API to get the specific asset by display name and asset folder
	searchParams := search.Query{
		Expression: fmt.Sprintf("asset_folder=\"%s\" AND display_name=\"%s\"",
			strings.TrimLeft(path.Join(f.root, path.Dir(remote)), "/"),
			path.Base(remote)),
		MaxResults: 1,
	}
	results, err := f.cld.Admin.Search(ctx, searchParams)
	if err != nil || len(results.Assets) == 0 {
		return nil, fs.ErrorObjectNotFound
	}

	if len(results.Assets) > 1 {
		return nil, errors.New("duplicate objects found")
	}

	return &results.Assets[0], nil
}

// NewObject finds the Object at remote. If it can't be found it returns the error fs.ErrorObjectNotFound.
func (f *Fs) NewObject(ctx context.Context, remote string) (fs.Object, error) {
	asset, err := f.getCLDAsset(ctx, remote)
	if err != nil {
		return nil, err
	}

	o := &Object{
		fs:      f,
		remote:  remote,
		size:    int64(asset.Bytes),
		modTime: asset.UploadedAt,
		url:     asset.SecureURL,
		md5sum:  asset.Etag,
	}

	return o, nil
}

// Put uploads content to Cloudinary
func (f *Fs) Put(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error) {
	params := uploader.UploadParams{
		AssetFolder:  path.Join(f.Root(), path.Dir(src.Remote())),
		DisplayName:  path.Base(src.Remote()),
		UploadPreset: f.opt.UploadPreset,
	}
	uploadResult, err := f.cld.Upload.Upload(ctx, in, params)
	if err != nil {
		return nil, fmt.Errorf("failed to upload to Cloudinary: %w", err)
	}

	if uploadResult.Error.Message != "" {
		return nil, fmt.Errorf(uploadResult.Error.Message)
	}

	o := &Object{
		fs:      f,
		remote:  src.Remote(),
		size:    int64(uploadResult.Bytes),
		modTime: uploadResult.CreatedAt,
		url:     uploadResult.SecureURL,
	}

	return o, nil
}

// Other required methods (not fully implemented):

func (f *Fs) Precision() time.Duration {
	return fs.ModTimeNotSupported
}

func (f *Fs) Hashes() hash.Set {
	return hash.Set(hash.None)
}

func (f *Fs) Mkdir(ctx context.Context, dir string) error {
	params := admin.CreateFolderParams{Folder: path.Join(f.Root(), dir)}
	res, err := f.cld.Admin.CreateFolder(ctx, params)
	if err != nil {
		return err
	}
	if res.Error.Message != "" {
		return fmt.Errorf(res.Error.Message)
	}

	return nil
}

func (f *Fs) Rmdir(ctx context.Context, dir string) error {
	params := admin.DeleteFolderParams{Folder: path.Join(f.Root(), dir)}
	res, err := f.cld.Admin.DeleteFolder(ctx, params)
	if err != nil {
		return err
	}
	if res.Error.Message != "" {
		if strings.HasPrefix(res.Error.Message, "Can't find folder with path") {
			return fs.ErrorDirNotFound
		}

		return fmt.Errorf(res.Error.Message)
	}

	return nil
}

func (f *Fs) Remove(ctx context.Context, o fs.Object) error {
	asset, err := f.getCLDAsset(ctx, o.Remote())
	if err != nil {
		return err
	}
	params := uploader.DestroyParams{
		PublicID:     asset.PublicID,
		ResourceType: asset.ResourceType,
		Type:         asset.Type,
	}
	res, dErr := f.cld.Upload.Destroy(ctx, params)
	if dErr != nil {
		return dErr
	}

	if res.Error.Message != "" {
		return fmt.Errorf(res.Error.Message)
	}

	if res.Result != "ok" {
		return fmt.Errorf(res.Result)
	}

	return nil
}

// Object methods

func (o *Object) Fs() fs.Info {
	return o.fs
}

func (o *Object) Remote() string {
	return o.remote
}

func (o *Object) ModTime(ctx context.Context) time.Time {
	return o.modTime
}

func (o *Object) Size() int64 {
	return o.size
}

func (o *Object) Storable() bool {
	return true
}

func (o *Object) SetModTime(ctx context.Context, modTime time.Time) error {
	return fs.ErrorCantSetModTime
}

func (o *Object) Open(ctx context.Context, options ...fs.OpenOption) (io.ReadCloser, error) {
	// Cloudinary assets can be accessed via URL directly
	resp, err := http.Get(o.url)
	if err != nil {
		return nil, fmt.Errorf("failed to open Cloudinary object: %w", err)
	}
	return resp.Body, nil
}

func (o *Object) Update(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) error {
	return nil
}

func (o *Object) Remove(ctx context.Context) error {
	return o.fs.Remove(ctx, o)
}
