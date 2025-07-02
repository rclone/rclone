// Package cloudinary provides an interface to the Cloudinary DAM
package cloudinary

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"slices"
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
	"github.com/rclone/rclone/fs/fserrors"
	"github.com/rclone/rclone/fs/fshttp"
	"github.com/rclone/rclone/fs/hash"
	"github.com/rclone/rclone/lib/encoder"
	"github.com/rclone/rclone/lib/pacer"
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
				Name:      "cloud_name",
				Help:      "Cloudinary Environment Name",
				Required:  true,
				Sensitive: true,
			},
			{
				Name:      "api_key",
				Help:      "Cloudinary API Key",
				Required:  true,
				Sensitive: true,
			},
			{
				Name:      "api_secret",
				Help:      "Cloudinary API Secret",
				Required:  true,
				Sensitive: true,
			},
			{
				Name: "upload_prefix",
				Help: "Specify the API endpoint for environments out of the US",
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
				Default:  fs.Duration(0),
				Advanced: true,
				Help:     "Wait N seconds for eventual consistency of the databases that support the backend operation",
			},
			{
				Name:     "adjust_media_files_extensions",
				Default:  true,
				Advanced: true,
				Help:     "Cloudinary handles media formats as a file attribute and strips it from the name, which is unlike most other file systems",
			},
			{
				Name: "media_extensions",
				Default: []string{
					"3ds", "3g2", "3gp", "ai", "arw", "avi", "avif", "bmp", "bw",
					"cr2", "cr3", "djvu", "dng", "eps3", "fbx", "flif", "flv", "gif",
					"glb", "gltf", "hdp", "heic", "heif", "ico", "indd", "jp2", "jpe",
					"jpeg", "jpg", "jxl", "jxr", "m2ts", "mov", "mp4", "mpeg", "mts",
					"mxf", "obj", "ogv", "pdf", "ply", "png", "psd", "svg", "tga",
					"tif", "tiff", "ts", "u3ma", "usdz", "wdp", "webm", "webp", "wmv"},
				Advanced: true,
				Help:     "Cloudinary supported media extensions",
			},
		},
	})
}

// Options defines the configuration for this backend
type Options struct {
	CloudName                  string               `config:"cloud_name"`
	APIKey                     string               `config:"api_key"`
	APISecret                  string               `config:"api_secret"`
	UploadPrefix               string               `config:"upload_prefix"`
	UploadPreset               string               `config:"upload_preset"`
	Enc                        encoder.MultiEncoder `config:"encoding"`
	EventuallyConsistentDelay  fs.Duration          `config:"eventually_consistent_delay"`
	MediaExtensions            []string             `config:"media_extensions"`
	AdjustMediaFilesExtensions bool                 `config:"adjust_media_files_extensions"`
}

// Fs represents a remote cloudinary server
type Fs struct {
	name     string
	root     string
	opt      Options
	features *fs.Features
	pacer    *fs.Pacer
	srv      *rest.Client           // For downloading assets via the Cloudinary CDN
	cld      *cloudinary.Cloudinary // API calls are going through the Cloudinary SDK
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
	cld.Admin.Client = *fshttp.NewClient(ctx)
	cld.Upload.Client = *fshttp.NewClient(ctx)
	if opt.UploadPrefix != "" {
		cld.Config.API.UploadPrefix = opt.UploadPrefix
	}
	client := fshttp.NewClient(ctx)
	f := &Fs{
		name:  name,
		root:  root,
		opt:   *opt,
		cld:   cld,
		pacer: fs.NewPacer(ctx, pacer.NewDefault(pacer.MinSleep(1000), pacer.MaxSleep(10000), pacer.DecayConstant(2))),
		srv:   rest.NewClient(client),
	}

	f.features = (&fs.Features{
		CanHaveEmptyDirectories: true,
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

// FromStandardPath implementation of the api.CloudinaryEncoder
func (f *Fs) FromStandardPath(s string) string {
	return strings.ReplaceAll(f.opt.Enc.FromStandardPath(s), "&", "\uFF06")
}

// FromStandardName implementation of the api.CloudinaryEncoder
func (f *Fs) FromStandardName(s string) string {
	if f.opt.AdjustMediaFilesExtensions {
		parsedURL, err := url.Parse(s)
		ext := ""
		if err != nil {
			fs.Logf(nil, "Error parsing URL: %v", err)
		} else {
			ext = path.Ext(parsedURL.Path)
			if slices.Contains(f.opt.MediaExtensions, strings.ToLower(strings.TrimPrefix(ext, "."))) {
				s = strings.TrimSuffix(parsedURL.Path, ext)
			}
		}
	}
	return strings.ReplaceAll(f.opt.Enc.FromStandardName(s), "&", "\uFF06")
}

// ToStandardPath implementation of the api.CloudinaryEncoder
func (f *Fs) ToStandardPath(s string) string {
	return strings.ReplaceAll(f.opt.Enc.ToStandardPath(s), "\uFF06", "&")
}

// ToStandardName implementation of the api.CloudinaryEncoder
func (f *Fs) ToStandardName(s string, assetURL string) string {
	ext := ""
	if f.opt.AdjustMediaFilesExtensions {
		parsedURL, err := url.Parse(assetURL)
		if err != nil {
			fs.Logf(nil, "Error parsing URL: %v", err)
		} else {
			ext = path.Ext(parsedURL.Path)
			if !slices.Contains(f.opt.MediaExtensions, strings.ToLower(strings.TrimPrefix(ext, "."))) {
				ext = ""
			}
		}
	}
	return strings.ReplaceAll(f.opt.Enc.ToStandardName(s), "\uFF06", "&") + ext
}

// FromStandardFullPath encodes a full path to Cloudinary standard
func (f *Fs) FromStandardFullPath(dir string) string {
	return path.Join(api.CloudinaryEncoder.FromStandardPath(f, f.root), api.CloudinaryEncoder.FromStandardPath(f, dir))
}

// ToAssetFolderAPI encodes folders as expected by the Cloudinary SDK
func (f *Fs) ToAssetFolderAPI(dir string) string {
	return strings.ReplaceAll(dir, "%", "%25")
}

// ToDisplayNameElastic encodes a special case of elasticsearch
func (f *Fs) ToDisplayNameElastic(dir string) string {
	return strings.ReplaceAll(dir, "!", "\\!")
}

// Name of the remote (as passed into NewFs)
func (f *Fs) Name() string {
	return f.name
}

// Root of the remote (as passed into NewFs)
func (f *Fs) Root() string {
	return f.root
}

// WaitEventuallyConsistent waits till the FS is eventually consistent
func (f *Fs) WaitEventuallyConsistent() {
	if f.opt.EventuallyConsistentDelay == fs.Duration(0) {
		return
	}
	delay := time.Duration(f.opt.EventuallyConsistentDelay)
	timeSinceLastCRUD := time.Since(f.lastCRUD)
	if timeSinceLastCRUD < delay {
		time.Sleep(delay - timeSinceLastCRUD)
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
			Folder:     f.ToAssetFolderAPI(remotePrefix),
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
			remote := path.Join(dir, api.CloudinaryEncoder.ToStandardName(f, asset.DisplayName, asset.SecureURL))
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
		MaxResults: 2,
	}
	var results *admin.SearchResult
	f.WaitEventuallyConsistent()
	err := f.pacer.Call(func() (bool, error) {
		var err1 error
		results, err1 = f.cld.Admin.Search(ctx, searchParams)
		if err1 == nil && results.TotalCount != len(results.Assets) {
			err1 = errors.New("partial response so waiting for eventual consistency")
		}
		return shouldRetry(ctx, nil, err1)
	})
	if err != nil {
		return nil, fs.ErrorObjectNotFound
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
	payload := []byte(path.Join(assetFolder, displayName))
	hash := blake3.Sum256(payload)
	return hex.EncodeToString(hash[:])
}

// Put uploads content to Cloudinary
func (f *Fs) Put(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error) {
	if src.Size() == 0 {
		return nil, fs.ErrorCantUploadEmptyFiles
	}

	params := uploader.UploadParams{
		UploadPreset: f.opt.UploadPreset,
	}

	updateObject := false
	var modTime time.Time
	for _, option := range options {
		if updateOptions, ok := option.(*api.UpdateOptions); ok {
			if updateOptions.PublicID != "" {
				updateObject = true
				params.Overwrite = SDKApi.Bool(true)
				params.Invalidate = SDKApi.Bool(true)
				params.PublicID = updateOptions.PublicID
				params.ResourceType = updateOptions.ResourceType
				params.Type = SDKApi.DeliveryType(updateOptions.DeliveryType)
				params.AssetFolder = updateOptions.AssetFolder
				params.DisplayName = updateOptions.DisplayName
				modTime = src.ModTime(ctx)
			}
		}
	}
	if !updateObject {
		params.AssetFolder = f.FromStandardFullPath(cldPathDir(src.Remote()))
		params.DisplayName = api.CloudinaryEncoder.FromStandardName(f, path.Base(src.Remote()))
		// We want to conform to the unique asset ID of rclone, which is (asset_folder,display_name,last_modified).
		// We also want to enable customers to choose their own public_id, in case duplicate names are not a crucial use case.
		// Upload_presets that apply randomness to the public ID would not work well with rclone duplicate assets support.
		params.FilenameOverride = f.getSuggestedPublicID(params.AssetFolder, params.DisplayName, src.ModTime(ctx))
	}
	uploadResult, err := f.cld.Upload.Upload(ctx, in, params)
	f.lastCRUD = time.Now()
	if err != nil {
		return nil, fmt.Errorf("failed to upload to Cloudinary: %w", err)
	}
	if !updateObject {
		modTime = uploadResult.CreatedAt
	}
	if uploadResult.Error.Message != "" {
		return nil, errors.New(uploadResult.Error.Message)
	}

	o := &Object{
		fs:           f,
		remote:       src.Remote(),
		size:         int64(uploadResult.Bytes),
		modTime:      modTime,
		url:          uploadResult.SecureURL,
		md5sum:       uploadResult.Etag,
		publicID:     uploadResult.PublicID,
		resourceType: uploadResult.ResourceType,
		deliveryType: uploadResult.Type,
	}
	return o, nil
}

// Precision of the remote
func (f *Fs) Precision() time.Duration {
	return fs.ModTimeNotSupported
}

// Hashes returns the supported hash sets
func (f *Fs) Hashes() hash.Set {
	return hash.Set(hash.MD5)
}

// Mkdir creates empty folders
func (f *Fs) Mkdir(ctx context.Context, dir string) error {
	params := admin.CreateFolderParams{Folder: f.ToAssetFolderAPI(f.FromStandardFullPath(dir))}
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

// Rmdir deletes empty folders
func (f *Fs) Rmdir(ctx context.Context, dir string) error {
	// Additional test because Cloudinary will delete folders without
	// assets, regardless of empty sub-folders
	folder := f.ToAssetFolderAPI(f.FromStandardFullPath(dir))
	folderParams := admin.SubFoldersParams{
		Folder:     folder,
		MaxResults: 1,
	}
	results, err := f.cld.Admin.SubFolders(ctx, folderParams)
	if err != nil {
		return err
	}
	if results.TotalCount > 0 {
		return fs.ErrorDirectoryNotEmpty
	}

	params := admin.DeleteFolderParams{Folder: folder}
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

// retryErrorCodes is a slice of error codes that we will retry
var retryErrorCodes = []int{
	420, // Too Many Requests (legacy)
	429, // Too Many Requests
	500, // Internal Server Error
	502, // Bad Gateway
	503, // Service Unavailable
	504, // Gateway Timeout
	509, // Bandwidth Limit Exceeded
}

// shouldRetry returns a boolean as to whether this resp and err
// deserve to be retried.  It returns the err as a convenience
func shouldRetry(ctx context.Context, resp *http.Response, err error) (bool, error) {
	if fserrors.ContextError(ctx, &err) {
		return false, err
	}
	if err != nil {
		tryAgain := "Try again on "
		if idx := strings.Index(err.Error(), tryAgain); idx != -1 {
			layout := "2006-01-02 15:04:05 UTC"
			dateStr := err.Error()[idx+len(tryAgain) : idx+len(tryAgain)+len(layout)]
			timestamp, err2 := time.Parse(layout, dateStr)
			if err2 == nil {
				return true, fserrors.NewErrorRetryAfter(time.Until(timestamp))
			}
		}

		fs.Debugf(nil, "Retrying API error %v", err)
		return true, err
	}

	return fserrors.ShouldRetry(err) || fserrors.ShouldRetryHTTP(resp, retryErrorCodes), err
}

// ------------------------------------------------------------

// Hash returns the MD5 of an object
func (o *Object) Hash(ctx context.Context, ty hash.Type) (string, error) {
	if ty != hash.MD5 {
		return "", hash.ErrUnsupported
	}
	return o.md5sum, nil
}

// Return a string version
func (o *Object) String() string {
	if o == nil {
		return "<nil>"
	}
	return o.remote
}

// Fs returns the parent Fs
func (o *Object) Fs() fs.Info {
	return o.fs
}

// Remote returns the remote path
func (o *Object) Remote() string {
	return o.remote
}

// ModTime returns the modification time of the object
func (o *Object) ModTime(ctx context.Context) time.Time {
	return o.modTime
}

// Size of object in bytes
func (o *Object) Size() int64 {
	return o.size
}

// Storable returns if this object is storable
func (o *Object) Storable() bool {
	return true
}

// SetModTime sets the modification time of the local fs object
func (o *Object) SetModTime(ctx context.Context, modTime time.Time) error {
	return fs.ErrorCantSetModTime
}

// Open an object for read
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
	err = o.fs.pacer.Call(func() (bool, error) {
		resp, err = o.fs.srv.Call(ctx, &opts)
		if err == nil {
			cl, clErr := strconv.Atoi(resp.Header.Get("content-length"))
			if clErr == nil && count == int64(cl) {
				return false, nil
			}
		}
		return shouldRetry(ctx, resp, err)
	})
	if err != nil {
		return nil, fmt.Errorf("failed download of \"%s\": %w", o.url, err)
	}
	return resp.Body, err
}

// Update the object with the contents of the io.Reader
func (o *Object) Update(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) error {
	options = append(options, &api.UpdateOptions{
		PublicID:     o.publicID,
		ResourceType: o.resourceType,
		DeliveryType: o.deliveryType,
		DisplayName:  api.CloudinaryEncoder.FromStandardName(o.fs, path.Base(o.Remote())),
		AssetFolder:  o.fs.FromStandardFullPath(cldPathDir(o.Remote())),
	})
	updatedObj, err := o.fs.Put(ctx, in, src, options...)
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

// Remove an object
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
