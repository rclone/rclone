// Package databricks provides an rclone backend for Databricks Unity Catalog
// volumes via the Databricks Files REST API, using the official Databricks
// Go SDK for authentication and transport.
//
// Reference: https://docs.databricks.com/api/workspace/files
package databricks

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"

	"github.com/databricks/databricks-sdk-go/apierr"
	"github.com/databricks/databricks-sdk-go/client"
	sdkconfig "github.com/databricks/databricks-sdk-go/config"
	"github.com/databricks/databricks-sdk-go/httpclient"
	"github.com/databricks/databricks-sdk-go/service/files"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config"
	"github.com/rclone/rclone/fs/config/configmap"
	"github.com/rclone/rclone/fs/config/configstruct"
	"github.com/rclone/rclone/fs/fshttp"
	"github.com/rclone/rclone/fs/hash"
	"github.com/rclone/rclone/lib/encoder"
)

func init() {
	fsi := &fs.RegInfo{
		Name:        "databricks",
		Description: "Databricks Unity Catalog",
		NewFs:       NewFs,
		Options: []fs.Option{{
			Name:     "host",
			Help:     "Databricks workspace URL.\n\nE.g. \"https://abc-1234.azuredatabricks.net\".",
			Required: true,
		}, {
			Name:      "token",
			Help:      "Databricks personal access token.\n\nLeave blank to use another authentication method supported by the Databricks SDK (e.g. environment variables DATABRICKS_CLIENT_ID / DATABRICKS_CLIENT_SECRET for OAuth M2M).",
			Sensitive: true,
		}, {
			Name:      "client_id",
			Help:      "Databricks client ID for OAuth M2M.",
			Sensitive: true,
		}, {
			Name:      "client_secret",
			Help:      "Databricks client secret for OAuth M2M.",
			Sensitive: true,
		}, {
			Name:     config.ConfigEncoding,
			Help:     config.ConfigEncodingHelp,
			Advanced: true,
			Default:  encoder.EncodeInvalidUtf8 | encoder.EncodeCtl | encoder.EncodeSlash,
		}},
	}
	fs.Register(fsi)
}

// Options holds the config options for this backend.
type Options struct {
	Host         string               `config:"host"`
	Token        string               `config:"token"`
	ClientID     string               `config:"client_id"`
	ClientSecret string               `config:"client_secret"`
	Enc          encoder.MultiEncoder `config:"encoding"`
}

// Fs represents a Databricks Unity Catalog Files remote.
type Fs struct {
	name      string
	root      string // path within the Databricks Files API (no leading slash)
	opt       Options
	features  *fs.Features
	files     files.FilesInterface     // SDK Files API (mockable in tests)
	apiClient *client.DatabricksClient // raw client for Range-aware downloads
}

// Object represents a file on Databricks Unity Catalog.
type Object struct {
	fs          *Fs
	remote      string // rclone remote path relative to Fs.root
	size        int64
	modTime     time.Time
	contentType string
}

// encodePath URL-encodes each segment of a slash-separated path while
// preserving the separators. This matches the SDK's internal encoding used
// in httpclient.EncodeMultiSegmentPathParameter and is required when building
// raw API paths for direct DatabricksClient.Do calls (e.g. Range downloads).
func encodePath(p string) string {
	parts := strings.Split(p, "/")
	for i, part := range parts {
		parts[i] = url.PathEscape(part)
	}
	return strings.Join(parts, "/")
}

// fullPath converts an rclone-relative remote path to the absolute Databricks
// Files path. The result always starts with "/".
func (f *Fs) fullPath(relative string) string {
	return "/" + path.Join(f.root, f.opt.Enc.FromStandardPath(relative))
}

// newDatabricksClient builds the low-level Databricks SDK client, injecting
// rclone's HTTP transport so that proxy, TLS, and CA settings are honoured.
func newDatabricksClient(ctx context.Context, opt *Options) (*client.DatabricksClient, error) {
	sdkCfg := &sdkconfig.Config{
		Host:          opt.Host,
		Token:         opt.Token,
		ClientID:      opt.ClientID,
		ClientSecret:  opt.ClientSecret,
		HTTPTransport: fshttp.NewClient(ctx).Transport,
	}

	clientCfg, err := sdkconfig.HTTPClientConfigFromConfig(sdkCfg)
	if err != nil {
		return nil, fmt.Errorf("databricks config: %w", err)
	}

	// Extend the SDK retry policy to match rclone conventions: retry on 429,
	// 500, 502, 503, and 504.
	clientCfg.ErrorRetriable = func(_ context.Context, err error) bool {
		var apiErr *apierr.APIError
		if errors.As(err, &apiErr) {
			switch apiErr.StatusCode {
			case http.StatusTooManyRequests,
				http.StatusInternalServerError,
				http.StatusBadGateway,
				http.StatusServiceUnavailable,
				http.StatusGatewayTimeout:
				return true
			}
		}
		return false
	}

	return client.NewWithClient(sdkCfg, httpclient.NewApiClient(clientCfg))
}

// NewFs creates a new Fs object from the name and root passed by rclone.
func NewFs(ctx context.Context, name, root string, m configmap.Mapper) (fs.Fs, error) {
	opt := new(Options)
	if err := configstruct.Set(m, opt); err != nil {
		return nil, err
	}
	opt.Host = strings.TrimRight(opt.Host, "/")

	apiClient, err := newDatabricksClient(ctx, opt)
	if err != nil {
		return nil, err
	}

	f := &Fs{
		name:      name,
		root:      strings.Trim(root, "/"),
		opt:       *opt,
		files:     files.NewFiles(apiClient),
		apiClient: apiClient,
	}
	f.features = (&fs.Features{
		ReadMimeType:            true,
		CanHaveEmptyDirectories: true,
	}).Fill(ctx, f)

	// Check whether the root is an existing file. If so, repoint the remote
	// at the parent directory and signal ErrorIsFile so rclone can handle it.
	if f.root != "" {
		_, err := f.statFile(ctx, "")
		if err == nil {
			newRoot := path.Dir(f.root)
			if newRoot == "." {
				newRoot = ""
			}
			f.root = newRoot
			return f, fs.ErrorIsFile
		}
		// 404 (ObjectNotFound) is expected when root is a directory — ignore it.
		// All other errors (auth, server, network) must be surfaced so the
		// user knows the remote is not usable.
		if err != fs.ErrorObjectNotFound {
			return nil, err
		}
	}

	return f, nil
}

// statFile fetches the metadata for a file and returns an Object.
// It uses the SDK's GetMetadata call (HEAD /api/2.0/fs/files/{path}).
func (f *Fs) statFile(ctx context.Context, relative string) (*Object, error) {
	meta, err := f.files.GetMetadataByFilePath(ctx, f.fullPath(relative))
	if err != nil {
		if apierr.IsMissing(err) {
			return nil, fs.ErrorObjectNotFound
		}
		return nil, err
	}

	modTime := time.Time{}
	if meta.LastModified != "" {
		if t, err := http.ParseTime(meta.LastModified); err == nil {
			modTime = t
		}
	}

	return &Object{
		fs:          f,
		remote:      relative,
		size:        meta.ContentLength,
		modTime:     modTime,
		contentType: meta.ContentType,
	}, nil
}

// ------------------------------------------------------------
// fs.Fs interface
// ------------------------------------------------------------

// Name returns the configured name of the remote.
func (f *Fs) Name() string { return f.name }

// Root returns the root path of the remote.
func (f *Fs) Root() string { return f.root }

// String returns a human-readable description of the remote.
func (f *Fs) String() string {
	return fmt.Sprintf("Databricks Unity Catalog %s/%s", f.opt.Host, f.root)
}

// Features returns optional features supported by this backend.
func (f *Fs) Features() *fs.Features { return f.features }

// Precision returns the precision of the remote's modification times.
//
// The Files API does not support setting client-side modTime on upload;
// it always records the server-side receive time. We therefore return
// [fs.ModTimeNotSupported] so that callers know modTime is unreliable.
func (f *Fs) Precision() time.Duration { return fs.ModTimeNotSupported }

// Hashes returns the hash types supported by this remote. The Databricks
// Files API does not expose file hashes, so we return None.
func (f *Fs) Hashes() hash.Set { return hash.Set(hash.None) }

// List returns the entries for the directory dir. An empty dir is the root.
// Returns fs.ErrorDirNotFound if dir does not exist.
func (f *Fs) List(ctx context.Context, dir string) (entries fs.DirEntries, err error) {
	dirEntries, err := f.files.ListDirectoryContentsAll(ctx, files.ListDirectoryContentsRequest{
		DirectoryPath: f.fullPath(dir),
	})
	if err != nil {
		if apierr.IsMissing(err) {
			return nil, fs.ErrorDirNotFound
		}
		return nil, fmt.Errorf("list %q: %w", dir, err)
	}

	for _, entry := range dirEntries {
		name := f.opt.Enc.ToStandardName(entry.Name)
		entryRemote := path.Join(dir, name)

		modTime := time.UnixMilli(entry.LastModified)
		if entry.IsDirectory {
			entries = append(entries, fs.NewDir(entryRemote, modTime))
		} else {
			entries = append(entries, &Object{
				fs:      f,
				remote:  entryRemote,
				size:    entry.FileSize,
				modTime: modTime,
			})
		}
	}
	return entries, nil
}

// NewObject returns an fs.Object for the given remote path, or
// fs.ErrorObjectNotFound if it does not exist.
func (f *Fs) NewObject(ctx context.Context, remote string) (fs.Object, error) {
	return f.statFile(ctx, remote)
}

// Put uploads src to the remote path described by src.Remote(), replacing any
// existing object. It returns the newly created object.
func (f *Fs) Put(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error) {
	o := &Object{
		fs:     f,
		remote: src.Remote(),
	}
	return o, o.Update(ctx, in, src, options...)
}

// PutStream uploads src with unknown size. It is identical to Put for this backend.
func (f *Fs) PutStream(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error) {
	return f.Put(ctx, in, src, options...)
}

// Mkdir creates the directory dir (and any missing parent directories).
func (f *Fs) Mkdir(ctx context.Context, dir string) error {
	return f.files.CreateDirectory(ctx, files.CreateDirectoryRequest{
		DirectoryPath: f.fullPath(dir),
	})
}

// Rmdir removes an empty directory. Returns fs.ErrorDirectoryNotEmpty if the
// directory still contains files or subdirectories.
func (f *Fs) Rmdir(ctx context.Context, dir string) error {
	err := f.files.DeleteDirectory(ctx, files.DeleteDirectoryRequest{
		DirectoryPath: f.fullPath(dir),
	})
	if err != nil {
		var apiErr *apierr.APIError
		if errors.As(err, &apiErr) && apiErr.StatusCode == http.StatusConflict {
			// The Files API returns 409 when trying to delete a non-empty
			// directory. Check for the specific error code to avoid masking
			// other conflict scenarios.
			if strings.Contains(apiErr.Message, "DIRECTORY_NOT_EMPTY") ||
				strings.Contains(apiErr.Message, "non-empty") ||
				strings.Contains(apiErr.Message, "not empty") {
				return fs.ErrorDirectoryNotEmpty
			}
			// Fall back to DirectoryNotEmpty for unrecognised 409 messages,
			// as this is the most common cause from the Files API.
			return fs.ErrorDirectoryNotEmpty
		}
		if apierr.IsMissing(err) {
			return fs.ErrorDirNotFound
		}
		return err
	}
	return nil
}

// ------------------------------------------------------------
// fs.Object interface
// ------------------------------------------------------------

// Fs returns the parent Fs.
func (o *Object) Fs() fs.Info { return o.fs }

// String returns the remote path.
func (o *Object) String() string {
	if o == nil {
		return "<nil>"
	}
	return o.remote
}

// Remote returns the remote path relative to the Fs root.
func (o *Object) Remote() string { return o.remote }

// Hash returns "" because the Databricks Files API does not provide file hashes.
func (o *Object) Hash(ctx context.Context, t hash.Type) (string, error) {
	return "", hash.ErrUnsupported
}

// Size returns the object size in bytes.
func (o *Object) Size() int64 { return o.size }

// ModTime returns the modification time of the object.
func (o *Object) ModTime(ctx context.Context) time.Time { return o.modTime }

// SetModTime is not supported — the Databricks Files API does not allow setting modification times.
func (o *Object) SetModTime(ctx context.Context, modTime time.Time) error {
	return fs.ErrorCantSetModTime
}

// Storable returns true since all objects from the Files API are regular files.
func (o *Object) Storable() bool { return true }

// Open opens the remote file for reading with optional Range support.
//
// The SDK's FilesAPI.Download does not expose Range headers, so Range requests
// are sent via DatabricksClient.Do with explicit headers. This preserves
// rclone's ability to do parallel/partial downloads.
func (o *Object) Open(ctx context.Context, options ...fs.OpenOption) (io.ReadCloser, error) {
	fs.FixRangeOption(options, o.size)

	headers := map[string]string{"Accept": "application/octet-stream"}
	for _, opt := range options {
		k, v := opt.Header()
		if k != "" {
			headers[k] = v
		}
	}

	var resp files.DownloadResponse
	apiPath := "/api/2.0/fs/files" + encodePath(o.fs.fullPath(o.remote))
	err := o.fs.apiClient.Do(ctx, http.MethodGet, apiPath, headers, nil, nil, &resp)
	if err != nil {
		return nil, fmt.Errorf("open %q: %w", o.remote, err)
	}
	return resp.Contents, nil
}

// Update replaces the remote object's content with the data from in.
// It ensures the parent directory exists before uploading.
func (o *Object) Update(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) error {
	// Ensure the parent directory exists (idempotent on the server).
	parentDir := path.Dir(o.remote)
	if parentDir == "." {
		parentDir = ""
	}
	if err := o.fs.Mkdir(ctx, parentDir); err != nil {
		return fmt.Errorf("create parent dir: %w", err)
	}

	// The SDK passes Contents directly as the streaming request body.
	// Stream bodies are not seekable so the SDK will not retry on error.
	err := o.fs.files.Upload(ctx, files.UploadRequest{
		FilePath:  o.fs.fullPath(o.remote),
		Contents:  io.NopCloser(in),
		Overwrite: true,
	})
	if err != nil {
		return fmt.Errorf("upload %q: %w", o.remote, err)
	}

	// Refresh the object's cached metadata.
	updated, err := o.fs.statFile(ctx, o.remote)
	if err != nil {
		return fmt.Errorf("stat after upload %q: %w", o.remote, err)
	}

	o.size = updated.size
	o.modTime = updated.modTime
	o.contentType = updated.contentType
	return nil
}

// Remove deletes the remote file.
func (o *Object) Remove(ctx context.Context) error {
	return o.fs.files.DeleteByFilePath(ctx, o.fs.fullPath(o.remote))
}

// Check the interfaces are satisfied
var (
	_ fs.Fs          = (*Fs)(nil)
	_ fs.PutStreamer = (*Fs)(nil)
	_ fs.Object      = (*Object)(nil)
)
