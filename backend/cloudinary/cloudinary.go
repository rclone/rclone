package cloudinary

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/cloudinary/cloudinary-go/v2"
	SDKApi "github.com/cloudinary/cloudinary-go/v2/api"
	"github.com/cloudinary/cloudinary-go/v2/api/admin"
	"github.com/cloudinary/cloudinary-go/v2/api/admin/search"
	"github.com/cloudinary/cloudinary-go/v2/api/uploader"
	"github.com/rclone/rclone/backend/cloudinary/api"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config"
	"github.com/rclone/rclone/fs/config/configmap"
	"github.com/rclone/rclone/fs/config/configstruct"
	"github.com/rclone/rclone/fs/fshttp"
	"github.com/rclone/rclone/fs/hash"
	"github.com/rclone/rclone/fs/object"
	"github.com/rclone/rclone/lib/encoder"
	"github.com/rclone/rclone/lib/rest"
	"github.com/zeebo/blake3"
)

// Cloudinary shouldn't have a trailing dot if there is no path
func cldPathDir(somePath string) string {
	if somePath == "" || somePath == "." {
		return somePath
	}
	dir := path.Dir(somePath)
	if dir == "." {
		return ""
	}
	return dir
}

// Register with Fs
func init() {
	fs.Register(&fs.RegInfo{
		Name:        "cloudinary",
		Description: "Cloudinary",
		NewFs:       NewFs,
		Options: []fs.Option{
			{
				Name:     "cloud_name",
				Help:     "Cloudinary Environment Name",
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
				Name: "upload_prefix",
				Help: "Specify alternative data center",
			},
			{
				Name: "upload_preset",
				Help: "Upload Preset to select asset manipulation on upload",
			},
			{
				Name:     config.ConfigEncoding,
				Help:     config.ConfigEncodingHelp,
				Advanced: true,
				Default: (encoder.Base | //  Slash,LtGt,DoubleQuote,Question,Asterisk,Pipe,Hash,Percent,BackSlash,Del,Ctl,RightSpace,InvalidUtf8,Dot
					encoder.EncodeSlash |
					encoder.EncodeLtGt |
					encoder.EncodeDoubleQuote |
					encoder.EncodeQuestion |
					encoder.EncodeAsterisk |
					encoder.EncodePipe |
					encoder.EncodeHash |
					encoder.EncodePercent |
					encoder.EncodeBackSlash |
					encoder.EncodeDel |
					encoder.EncodeCtl |
					encoder.EncodeRightSpace |
					encoder.EncodeInvalidUtf8 |
					encoder.EncodeDot),
			},
			{
				Name:     "eventually_consistent_delay",
				Advanced: true,
				Help:     "Wait N seconds for eventual consistency",
			},
		},
	})
}

// Options defines the configuration for this backend
type Options struct {
	CloudName                 string               `config:"cloud_name"`
	APIKey                    string               `config:"api_key"`
	APISecret                 string               `config:"api_secret"`
	UploadPrefix              string               `config:"upload_prefix"`
	UploadPreset              string               `config:"upload_preset"`
	Enc                       encoder.MultiEncoder `config:"encoding"`
	EventuallyConsistentDelay string               `config:"eventually_consistent_delay"`
}

// Fs represents a remote cloudinary server
type Fs struct {
	name     string
	root     string
	opt      Options
	features *fs.Features
	srv      *rest.Client // the connection to the server
	cld      *cloudinary.Cloudinary
	lastCRUD time.Time
}

// Object describes a cloudinary object
type Object struct {
	fs           *Fs
	remote       string
	size         int64
	modTime      time.Time
	url          string
	md5sum       string
	publicID     string
	resourceType string
	deliveryType string
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
	if opt.UploadPrefix != "" {
		cld.Config.API.UploadPrefix = opt.UploadPrefix
	}
	client := fshttp.NewClient(ctx)
	f := &Fs{
		name: name,
		root: root,
		opt:  *opt,
		cld:  cld,
		srv:  rest.NewClient(client),
	}

	f.features = (&fs.Features{
		CanHaveEmptyDirectories: true,
		DuplicateFiles:          true,
	}).Fill(ctx, f)

	if root != "" {
		// Check to see if the root actually an existing file
		remote := path.Base(root)
		f.root = cldPathDir(root)
		_, err := f.NewObject(ctx, remote)
		if err != nil {
			if err == fs.ErrorObjectNotFound || errors.Is(err, fs.ErrorNotAFile) {
				// File doesn't exist so return the previous root
				f.root = root
				return f, nil
			}
			return nil, err
		}
		// return an error with an fs which points to the parent
		return f, fs.ErrorIsFile
	}
	return f, nil
}

// ------------------------------------------------------------

// Implementation of the api.CloudinaryEncoder
func (f *Fs) FromStandardPath(s string) string {
	return strings.Replace(f.opt.Enc.FromStandardPath(s), "&", "\uFF06", -1)
}

func (f *Fs) FromStandardName(s string) string {
	return strings.Replace(f.opt.Enc.FromStandardName(s), "&", "\uFF06", -1)
}

func (f *Fs) ToStandardPath(s string) string {
	return strings.Replace(f.opt.Enc.ToStandardPath(s), "\uFF06", "&", -1)
}

func (f *Fs) ToStandardName(s string) string {
	return strings.Replace(f.opt.Enc.ToStandardName(s), "\uFF06", "&", -1)
}

func (f *Fs) FromStandardFullPath(dir string) string {
	return path.Join(api.CloudinaryEncoder.FromStandardPath(f, f.root), api.CloudinaryEncoder.FromStandardPath(f, dir))
}

func (f *Fs) ToAssetFolderApi(dir string) string {
	return strings.Replace(dir, "%", "%25", -1)
}

func (f *Fs) ToDisplayNameElastic(dir string) string {
	return strings.Replace(dir, "!", "\\!", -1)
}

// Name of the remote (as passed into NewFs)
func (f *Fs) Name() string {
	return f.name
}

// Root of the remote (as passed into NewFs)
func (f *Fs) Root() string {
	return f.root
}

// Wait till the FS is eventually consistent
func (f *Fs) WaitEventuallyConsistent() {
	if f.opt.EventuallyConsistentDelay == "" {
		return
	}
	timeSinceLastCRUD := time.Since(f.lastCRUD)
	delaySeconds, err := strconv.Atoi(f.opt.EventuallyConsistentDelay)
	if err != nil {
		fs.Errorf(f, "failed to convert EventuallyConsistentDelay to integer from '%s'", f.opt.EventuallyConsistentDelay)
	}
	if delaySeconds > 0 && timeSinceLastCRUD.Seconds() < float64(delaySeconds) {
		time.Sleep(time.Duration(delaySeconds)*time.Second - timeSinceLastCRUD)
	}
}

// String converts this Fs to a string
func (f *Fs) String() string {
	return fmt.Sprintf("Cloudinary root '%s'", f.root)
}

// Features returns the optional features of this Fs
func (f *Fs) Features() *fs.Features {
	return f.features
}

// List the objects and directories in dir into entries
func (f *Fs) List(ctx context.Context, dir string) (fs.DirEntries, error) {
	remotePrefix := f.FromStandardFullPath(dir)
	if remotePrefix != "" && !strings.HasSuffix(remotePrefix, "/") {
		remotePrefix += "/"
	}

	var entries fs.DirEntries
	dirs := make(map[string]struct{})
	nextCursor := ""
	f.WaitEventuallyConsistent()
	for {
		// user the folders api to list folders.
		folderParams := admin.SubFoldersParams{
			Folder:     f.ToAssetFolderApi(remotePrefix),
			MaxResults: 500,
		}
		if nextCursor != "" {
			folderParams.NextCursor = nextCursor
		}

		results, err := f.cld.Admin.SubFolders(ctx, folderParams)
		if err != nil {
			return nil, fmt.Errorf("failed to list sub-folders: %w", err)
		}
		if results.Error.Message != "" {
			if strings.HasPrefix(results.Error.Message, "Can't find folder with path") {
				return nil, fs.ErrorDirNotFound
			}

			return nil, fmt.Errorf("failed to list sub-folders: %s", results.Error.Message)
		}

		for _, folder := range results.Folders {
			relativePath := api.CloudinaryEncoder.ToStandardPath(f, strings.TrimPrefix(folder.Path, remotePrefix))
			parts := strings.Split(relativePath, "/")

			// It's a directory
			dirName := parts[len(parts)-1]
			if _, found := dirs[dirName]; !found {
				d := fs.NewDir(path.Join(dir, dirName), time.Time{})
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
			remote := api.CloudinaryEncoder.ToStandardName(f, asset.DisplayName)
			if dir != "" {
				remote = path.Join(dir, api.CloudinaryEncoder.ToStandardName(f, asset.DisplayName))
			}
			o := &Object{
				fs:           f,
				remote:       remote,
				size:         int64(asset.Bytes),
				modTime:      asset.CreatedAt,
				url:          asset.SecureURL,
				publicID:     asset.PublicID,
				resourceType: asset.AssetType,
				deliveryType: asset.Type,
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

// NewObject finds the Object at remote. If it can't be found it returns the error fs.ErrorObjectNotFound.
func (f *Fs) NewObject(ctx context.Context, remote string) (fs.Object, error) {
	searchParams := search.Query{
		Expression: fmt.Sprintf("asset_folder:\"%s\" AND display_name:\"%s\"",
			f.FromStandardFullPath(cldPathDir(remote)),
			f.ToDisplayNameElastic(api.CloudinaryEncoder.FromStandardName(f, path.Base(remote)))),
		SortBy:     []search.SortByField{{"uploaded_at": "desc"}},
		MaxResults: 1,
	}
	var results *admin.SearchResult
	var err error
	for i := 1; i <= 4; i++ {
		f.WaitEventuallyConsistent()
		results, err = f.cld.Admin.Search(ctx, searchParams)
		if err != nil {
			return nil, err
		}
		// Eventual consistency so retrying
		if results.TotalCount == len(results.Assets) {
			break
		}
	}
	if results.TotalCount == 0 || len(results.Assets) == 0 {
		return nil, fs.ErrorObjectNotFound
	}
	asset := results.Assets[0]

	o := &Object{
		fs:           f,
		remote:       remote,
		size:         int64(asset.Bytes),
		modTime:      asset.UploadedAt,
		url:          asset.SecureURL,
		md5sum:       asset.Etag,
		publicID:     asset.PublicID,
		resourceType: asset.ResourceType,
		deliveryType: asset.Type,
	}

	return o, nil
}

func (f *Fs) getSuggestedPublicID(assetFolder string, displayName string, modTime time.Time) string {
	payload := []byte(path.Join(assetFolder, displayName, strconv.FormatInt(modTime.Unix(), 16)))
	hash := blake3.Sum256(payload)
	return hex.EncodeToString(hash[:])
}

// Put uploads content to Cloudinary
func (f *Fs) Put(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error) {
	if src.Size() == 0 {
		return nil, fs.ErrorCantUploadEmptyFiles
	}

	params := uploader.UploadParams{
		AssetFolder:  f.FromStandardFullPath(cldPathDir(src.Remote())),
		DisplayName:  api.CloudinaryEncoder.FromStandardName(f, path.Base(src.Remote())),
		UploadPreset: f.opt.UploadPreset,
	}

	// We want to conform to the unique asset ID of rclone, which is (asset_folder,display_name,last_modified).
	// We also want to enable customers to choose their own public_id, in case duplicate names are not a crucial use case.
	// Upload_presets that apply randomness to the public ID would not work well with rclone duplicate assets support.
	params.FilenameOverride = f.getSuggestedPublicID(params.AssetFolder, params.DisplayName, src.ModTime(ctx))

	for _, option := range options {
		if updateOptions, ok := option.(*api.UpdateOptions); ok {
			if updateOptions.PublicID != "" {
				params.Overwrite = SDKApi.Bool(true)
				params.Invalidate = SDKApi.Bool(true)
				params.PublicID = updateOptions.PublicID
				params.ResourceType = updateOptions.ResourceType
				params.Type = SDKApi.DeliveryType(updateOptions.DeliveryType)
				params.FilenameOverride = ""
			}
		}
	}
	uploadResult, err := f.cld.Upload.Upload(ctx, in, params)
	f.lastCRUD = time.Now()
	if err != nil {
		return nil, fmt.Errorf("failed to upload to Cloudinary: %w", err)
	}
	if uploadResult.Error.Message != "" {
		return nil, errors.New(uploadResult.Error.Message)
	}

	o := &Object{
		fs:           f,
		remote:       src.Remote(),
		size:         int64(uploadResult.Bytes),
		modTime:      uploadResult.CreatedAt,
		url:          uploadResult.SecureURL,
		md5sum:       uploadResult.Etag,
		publicID:     uploadResult.PublicID,
		resourceType: uploadResult.ResourceType,
		deliveryType: uploadResult.Type,
	}
	return o, nil
}

func (f *Fs) Precision() time.Duration {
	return fs.ModTimeNotSupported
}

func (f *Fs) Hashes() hash.Set {
	return hash.Set(hash.MD5)
}

func (f *Fs) Mkdir(ctx context.Context, dir string) error {
	params := admin.CreateFolderParams{Folder: f.ToAssetFolderApi(f.FromStandardFullPath(dir))}
	res, err := f.cld.Admin.CreateFolder(ctx, params)
	f.lastCRUD = time.Now()
	if err != nil {
		return err
	}
	if res.Error.Message != "" {
		return errors.New(res.Error.Message)
	}

	return nil
}

func (f *Fs) Rmdir(ctx context.Context, dir string) error {
	params := admin.DeleteFolderParams{Folder: f.ToAssetFolderApi(f.FromStandardFullPath(dir))}
	res, err := f.cld.Admin.DeleteFolder(ctx, params)
	f.lastCRUD = time.Now()
	if err != nil {
		return err
	}
	if res.Error.Message != "" {
		if strings.HasPrefix(res.Error.Message, "Can't find folder with path") {
			return fs.ErrorDirNotFound
		}

		return errors.New(res.Error.Message)
	}

	return nil
}

// ------------------------------------------------------------

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

func (o *Object) Open(ctx context.Context, options ...fs.OpenOption) (in io.ReadCloser, err error) {
	var resp *http.Response
	opts := rest.Opts{
		Method:  "GET",
		RootURL: o.url,
		Options: options,
	}
	var offset int64
	var count int64
	var key string
	var value string
	fs.FixRangeOption(options, o.size)
	for _, option := range options {
		switch x := option.(type) {
		case *fs.RangeOption:
			offset, count = x.Decode(o.size)
			if count < 0 {
				count = o.size - offset
			}
			key, value = option.Header()
		case *fs.SeekOption:
			offset = x.Offset
			count = o.size - offset
			key, value = option.Header()
		default:
			if option.Mandatory() {
				fs.Logf(o, "Unsupported mandatory option: %v", option)
			}
		}
	}
	if key != "" && value != "" {
		opts.ExtraHeaders = make(map[string]string)
		opts.ExtraHeaders[key] = value
	}
	// Make sure that the asset is fully available
	for i := 1; i <= 4; i++ {
		resp, err = o.fs.srv.Call(ctx, &opts)
		if err != nil {
			return nil, fmt.Errorf("failed download of \"%s\": %w", o.url, err)
		}
		if count == 0 {
			break
		}
		cl, err := strconv.Atoi(resp.Header.Get("content-length"))
		if err == nil && count == int64(cl) {
			break
		}
		time.Sleep(time.Duration(i) * time.Second)
	}
	return resp.Body, err
}

func (o *Object) Update(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) error {
	srcImmutable := object.NewStaticObjectInfo(o.Remote(), src.ModTime(ctx), src.Size(), true, nil, o.Fs())
	options = append(options, &api.UpdateOptions{
		PublicID:     o.publicID,
		ResourceType: o.resourceType,
		DeliveryType: o.deliveryType,
	})
	updatedObj, err := o.fs.Put(ctx, in, srcImmutable, options...)
	if err != nil {
		return err
	}
	if uo, ok := updatedObj.(*Object); ok {
		o.size = uo.size
		o.modTime = time.Now() // Skipping uo.modTime because the API returns the create time
		o.url = uo.url
		o.md5sum = uo.md5sum
		o.publicID = uo.publicID
		o.resourceType = uo.resourceType
		o.deliveryType = uo.deliveryType
	}
	return nil
}

func (o *Object) Remove(ctx context.Context) error {
	params := uploader.DestroyParams{
		PublicID:     o.publicID,
		ResourceType: o.resourceType,
		Type:         o.deliveryType,
	}
	res, dErr := o.fs.cld.Upload.Destroy(ctx, params)
	o.fs.lastCRUD = time.Now()
	if dErr != nil {
		return dErr
	}

	if res.Error.Message != "" {
		return errors.New(res.Error.Message)
	}

	if res.Result != "ok" {
		return errors.New(res.Result)
	}

	return nil
}
