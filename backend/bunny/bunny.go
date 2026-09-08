// Package bunny provides an interface to Bunny.net object storage.
package bunny

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"path"
	"strings"
	"time"

	"github.com/rclone/rclone/backend/bunny/api"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config"
	"github.com/rclone/rclone/fs/config/configmap"
	"github.com/rclone/rclone/fs/config/configstruct"
	"github.com/rclone/rclone/fs/config/obscure"
	"github.com/rclone/rclone/fs/fshttp"
	"github.com/rclone/rclone/fs/hash"
	"github.com/rclone/rclone/lib/encoder"
	"github.com/rclone/rclone/lib/pacer"
	"github.com/rclone/rclone/lib/readers"
	"github.com/rclone/rclone/lib/rest"
)

const (
	minSleep      = 10 * time.Millisecond
	maxSleep      = 1 * time.Minute
	decayConstant = 1 // bigger for slower decay, exponential
)

func init() {
	fs.Register(&fs.RegInfo{
		Name:        "bunny",
		Description: "Bunny.net Storage",
		NewFs:       NewFs,
		Options: []fs.Option{
			{
				Name:     "storage_zone",
				Help:     "Storage zone name.",
				Required: true,
			},
			{
				Name:       "access_key",
				Help:       "Storage zone API key (password).",
				Required:   true,
				IsPassword: true,
				Sensitive:  true,
			},
			{
				Name:    "endpoint",
				Help:    "Storage endpoint. Select the region your storage zone is in.",
				Default: "storage.bunnycdn.com",
				Examples: []fs.OptionExample{
					{Value: "storage.bunnycdn.com", Help: "Frankfurt, DE (default)"},
					{Value: "uk.storage.bunnycdn.com", Help: "London, UK"},
					{Value: "ny.storage.bunnycdn.com", Help: "New York, US"},
					{Value: "la.storage.bunnycdn.com", Help: "Los Angeles, US"},
					{Value: "sg.storage.bunnycdn.com", Help: "Singapore, SG"},
					{Value: "se.storage.bunnycdn.com", Help: "Stockholm, SE"},
					{Value: "br.storage.bunnycdn.com", Help: "São Paulo, BR"},
					{Value: "jh.storage.bunnycdn.com", Help: "Johannesburg, SA"},
					{Value: "syd.storage.bunnycdn.com", Help: "Sydney, AU"},
				},
			},
			{
				Name:     config.ConfigEncoding,
				Help:     config.ConfigEncodingHelp,
				Advanced: true,
				Default: (encoder.Display |
					encoder.EncodeBackSlash |
					encoder.EncodeInvalidUtf8 |
					encoder.EncodeDoubleQuote |
					encoder.EncodeLtGt |
					encoder.EncodeHashPercent |
					encoder.EncodeQuestion |
					encoder.EncodeLeftSpace |
					encoder.EncodeLeftTilde |
					encoder.EncodeLeftCrLfHtVt |
					encoder.EncodeLeftPeriod |
					encoder.EncodeRightSpace |
					encoder.EncodeRightPeriod |
					encoder.EncodeRightCrLfHtVt),
			},
		},
	})
}

// Options defines the configuration for this backend
type Options struct {
	StorageZone string               `config:"storage_zone"`
	AccessKey   string               `config:"access_key"`
	Endpoint    string               `config:"endpoint"`
	Enc         encoder.MultiEncoder `config:"encoding"`
}

// Fs represents a remote bunny storage zone
type Fs struct {
	name        string         // name of this remote
	root        string         // the path we are working on if any
	opt         Options        // parsed config options
	ci          *fs.ConfigInfo // global config
	features    *fs.Features   // optional features
	srv         *rest.Client   // REST client for all API calls
	pacer       *fs.Pacer      // pacer for API calls
	endpointURL string         // full endpoint URL
}

// Object describes a bunny storage object
type Object struct {
	fs          *Fs
	remote      string
	size        int64
	modTime     time.Time
	name        string
	sha256      string
	contentType string
}

// NewFs constructs an Fs from the path, container:path
func NewFs(ctx context.Context, name, root string, m configmap.Mapper) (fs.Fs, error) {
	opt := new(Options)
	err := configstruct.Set(m, opt)
	if err != nil {
		return nil, err
	}
	if opt.StorageZone == "" {
		return nil, errors.New("storage_zone is required")
	}
	if opt.AccessKey == "" {
		return nil, errors.New("access_key is required")
	}
	if revealed, err := obscure.Reveal(opt.AccessKey); err == nil {
		opt.AccessKey = revealed
	}
	ci := fs.GetConfig(ctx)
	srv := rest.NewClient(fshttp.NewClient(ctx))
	endpointURL := "https://" + opt.Endpoint
	srv.SetRoot(endpointURL)
	srv.SetHeader("AccessKey", opt.AccessKey)
	f := &Fs{
		name:        name,
		opt:         *opt,
		root:        root,
		ci:          ci,
		srv:         srv,
		pacer:       fs.NewPacer(ctx, pacer.NewDefault(pacer.MinSleep(minSleep), pacer.MaxSleep(maxSleep), pacer.DecayConstant(decayConstant))),
		endpointURL: endpointURL,
	}
	f.features = (&fs.Features{
		BucketBased:             true,
		BucketBasedRootOK:       true,
		CanHaveEmptyDirectories: false,
		ReadMimeType:            false,
		WriteMimeType:           false,
	}).Fill(ctx, f)

	// Check if root is actually an existing file
	if root != "" {
		remote := path.Base(root)
		dir := path.Dir(root)
		if dir == "." {
			dir = ""
		}
		f.root = dir
		list, err := f.list(ctx, "")
		if err == nil {
			for _, item := range list.Items {
				if !item.IsDirectory && f.opt.Enc.ToStandardName(item.ObjectName) == remote {
					// Root is a file, not a directory
					f.root = root[:len(root)-len(remote)]
					f.root = strings.TrimSuffix(f.root, "/")
					return f, fs.ErrorIsFile
				}
			}
		}
		// Root is a directory (or doesn't exist yet)
		f.root = root
	}

	return f, nil
}

// list retrieves a directory listing from Bunny
func (f *Fs) list(ctx context.Context, dir string) (list *api.DirList, err error) {
	reqPath := f.getFullFilePath(dir, false)
	var response []api.DirItem
	opts := rest.Opts{
		Method:       "GET",
		Path:         reqPath + "/",
		ExtraHeaders: map[string]string{"Accept": "application/json"},
	}
	err = f.pacer.Call(func() (retry bool, err error) {
		resp, err := f.srv.CallJSON(ctx, &opts, nil, &response)
		if resp != nil && resp.StatusCode == 404 {
			return false, fs.ErrorDirNotFound
		}
		return shouldRetry(ctx, resp, err)
	})
	if err != nil {
		return nil, err
	}
	list = &api.DirList{
		Dir:   dir,
		Items: response,
	}
	return list, nil
}

// List the objects and directories in dir into entries.
func (f *Fs) List(ctx context.Context, dir string) (entries fs.DirEntries, err error) {
	list, err := f.list(ctx, dir)
	if err != nil {
		return nil, err
	}

	for _, file := range list.Items {
		objName := f.opt.Enc.ToStandardName(file.ObjectName)
		remote := joinPath(dir, objName)
		if file.IsDirectory {
			d := fs.NewDir(remote, file.ModTime())
			entries = append(entries, d)
		} else {
			entries = append(entries, f.newObjectWithInfo(remote, &file))
		}
	}
	return entries, nil
}

// Features returns the optional features of this Fs
func (f *Fs) Features() *fs.Features {
	return f.features
}

// NewObject finds the Object at remote.
func (f *Fs) NewObject(ctx context.Context, remote string) (fs.Object, error) {
	if remote == "" {
		return nil, fs.ErrorObjectNotFound
	}
	dir := path.Dir(remote)
	if dir == "." {
		dir = ""
	}
	leaf := path.Base(remote)
	list, err := f.list(ctx, dir)
	if err != nil {
		return nil, fs.ErrorObjectNotFound
	}
	for _, item := range list.Items {
		itemName := f.opt.Enc.ToStandardName(item.ObjectName)
		if itemName == leaf {
			if item.IsDirectory {
				return nil, fs.ErrorIsDir
			}
			return f.newObjectWithInfo(remote, &item), nil
		}
	}
	return nil, fs.ErrorObjectNotFound
}

// Put uploads a file to the remote path
func (f *Fs) Put(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (o fs.Object, err error) {
	remote := src.Remote()
	obj := &Object{
		fs:     f,
		remote: remote,
	}
	err = obj.Update(ctx, in, src, options...)
	if err != nil {
		return nil, err
	}
	return obj, nil
}

// Mkdir creates the storage zone path - subdirectories are virtual prefixes only
func (f *Fs) Mkdir(ctx context.Context, dir string) error {
	// Storage zone must exist (precondition) - subdirectories are
	// virtual prefixes that are created implicitly on file upload
	return nil
}

// Rmdir - deletes directory, treating 404 as success
func (f *Fs) Rmdir(ctx context.Context, dir string) error {
	if dir == "" && f.root == "" {
		return fs.ErrorDirNotFound
	}
	list, err := f.list(ctx, dir)
	if err != nil {
		return err
	}
	if len(list.Items) > 0 {
		return fs.ErrorDirectoryNotEmpty
	}
	reqPath := f.getFullFilePath(dir, false) + "/"
	opts := rest.Opts{
		Method:       "DELETE",
		Path:         reqPath,
		IgnoreStatus: true,
		NoResponse:   true,
	}
	var resp *http.Response
	err = f.pacer.Call(func() (bool, error) {
		resp, err = f.srv.Call(ctx, &opts)
		if err != nil {
			return false, fmt.Errorf("failed to rmdir %s: %w", dir, err)
		}
		return shouldRetry(ctx, resp, err)
	})
	if err != nil {
		return err
	}
	if resp.StatusCode != 200 && resp.StatusCode != 404 {
		return fmt.Errorf("failed to rmdir %s: status %d", dir, resp.StatusCode)
	}
	return nil
}

// Name of the remote (as passed into NewFs)
func (f *Fs) Name() string {
	return f.name
}

// Root of the remote (as passed into NewFs)
func (f *Fs) Root() string {
	return f.root
}

// Hashes returns the supported hash sets.
func (f *Fs) Hashes() hash.Set {
	return hash.NewHashSet(hash.SHA256)
}

// Precision of the remote
func (f *Fs) Precision() time.Duration {
	return fs.ModTimeNotSupported
}

// String converts this Fs to a string
func (f *Fs) String() string {
	return fmt.Sprintf("Bunny storage zone %s path %s", f.opt.StorageZone, f.root)
}

func shouldRetry(_ context.Context, resp *http.Response, err error) (bool, error) {
	if resp != nil && resp.StatusCode == 429 {
		return true, pacer.RetryAfterError(err, 5*time.Second)
	}
	return false, err
}

func (f *Fs) newObjectWithInfo(remote string, file *api.DirItem) *Object {
	return &Object{
		fs:          f,
		remote:      remote,
		size:        file.Length,
		modTime:     file.ModTime(),
		name:        file.ObjectName,
		sha256:      strings.ToLower(file.Checksum),
		contentType: file.ContentType,
	}
}

// joinPath joins dir and name without path.Join's dot normalization
func joinPath(dir, name string) string {
	if dir == "" {
		return name
	}
	if name == "" {
		return dir
	}
	return dir + "/" + name
}

func (f *Fs) getFullFilePath(remote string, incRoot bool) string {
	basePath := "/" + f.opt.StorageZone
	subPath := joinPath(f.root, remote)
	if subPath != "" {
		encoded := f.opt.Enc.FromStandardPath(subPath)
		basePath = basePath + "/" + rest.URLPathEscape(encoded)
	}
	if incRoot {
		return f.endpointURL + basePath
	}
	return basePath
}

// Fs returns the parent Fs
func (o *Object) Fs() fs.Info {
	return o.fs
}

// Size returns the size of the object
func (o *Object) Size() int64 {
	return o.size
}

// ModTime returns the modification time
func (o *Object) ModTime(context.Context) time.Time {
	return o.modTime
}

// Remote returns the remote path
func (o *Object) Remote() string {
	return o.remote
}

// String returns a description of the Object
func (o *Object) String() string {
	if o == nil {
		return "<nil>"
	}
	return o.remote
}

// Hash returns the hash of the object
func (o *Object) Hash(_ context.Context, ty hash.Type) (string, error) {
	if ty == hash.SHA256 {
		return o.sha256, nil
	}
	return "", hash.ErrUnsupported
}

// Storable returns if this object is storable
func (o *Object) Storable() bool {
	return true
}

// SetModTime sets the modification time - not supported by Bunny
func (o *Object) SetModTime(_ context.Context, _ time.Time) error {
	return fs.ErrorCantSetModTime
}

// MimeType returns the content type of the object
func (o *Object) MimeType(_ context.Context) string {
	return o.contentType
}

// Open opens the file for reading
func (o *Object) Open(ctx context.Context, options ...fs.OpenOption) (in io.ReadCloser, err error) {
	reqPath := o.fs.getFullFilePath(o.remote, false)
	opts := rest.Opts{
		Method:       "GET",
		Path:         reqPath,
		Options:      options,
		IgnoreStatus: true,
	}
	var resp *http.Response
	err = o.fs.pacer.Call(func() (bool, error) {
		resp, err = o.fs.srv.Call(ctx, &opts)
		if err != nil {
			return false, err
		}
		if resp.StatusCode == 404 {
			_ = resp.Body.Close()
			return false, fs.ErrorObjectNotFound
		}
		if resp.StatusCode != 200 && resp.StatusCode != 206 {
			_ = resp.Body.Close()
			return false, fmt.Errorf("unexpected status %d downloading %s", resp.StatusCode, o.remote)
		}
		return shouldRetry(ctx, resp, err)
	})
	if err != nil {
		return nil, err
	}
	return resp.Body, nil
}

// Update replaces the contents of the object
func (o *Object) Update(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (err error) {
	reqPath := o.fs.getFullFilePath(o.remote, false)
	extraHeaders := map[string]string{}

	// Set Content-Type from source object
	mimeType := fs.MimeType(ctx, src)
	if mimeType != "" {
		extraHeaders["Content-Type"] = mimeType
	}

	srcHash, err := src.Hash(ctx, hash.SHA256)
	if err == nil && srcHash != "" {
		extraHeaders["Checksum"] = strings.ToUpper(srcHash)
	}

	// Enable low-level HTTP/2 retries via GetBody.
	// Bunny may send GOAWAY during upload; a repeatable reader
	// allows the transport to retry without re-reading from source.
	size := src.Size()
	var getBody func() (io.ReadCloser, error)
	if size >= 0 {
		buf := make([]byte, size)
		repeatableIn := readers.NewRepeatableLimitReaderBuffer(in, buf, size)
		in = repeatableIn
		getBody = func() (io.ReadCloser, error) {
			_, seekErr := repeatableIn.Seek(0, io.SeekStart)
			if seekErr != nil {
				return nil, seekErr
			}
			return io.NopCloser(repeatableIn), nil
		}
	}

	opts := rest.Opts{
		Method:       "PUT",
		Path:         reqPath,
		Body:         in,
		Options:      options,
		ExtraHeaders: extraHeaders,
		GetBody:      getBody,
		IgnoreStatus: true,
		NoResponse:   true,
	}
	var resp *http.Response
	err = o.fs.pacer.Call(func() (bool, error) {
		resp, err = o.fs.srv.Call(ctx, &opts)
		if err != nil {
			return false, fmt.Errorf("failed to upload %s: %w", o.remote, err)
		}
		if resp.StatusCode != 201 {
			return false, fmt.Errorf("upload failed with status %d for %s", resp.StatusCode, o.remote)
		}
		return shouldRetry(ctx, resp, err)
	})
	if err != nil {
		return err
	}

	// Update object metadata from source
	o.size = src.Size()
	o.modTime = time.Now()
	// Clear hash - it will be re-read from listing on next access
	o.sha256 = ""

	// Re-read object info to get contentType and checksum from Bunny
	dir := path.Dir(o.remote)
	if dir == "." {
		dir = ""
	}
	leaf := path.Base(o.remote)
	list, listErr := o.fs.list(ctx, dir)
	if listErr == nil {
		for _, item := range list.Items {
			if o.fs.opt.Enc.ToStandardName(item.ObjectName) == leaf {
				o.sha256 = strings.ToLower(item.Checksum)
				o.contentType = item.ContentType
				o.modTime = item.ModTime()
				o.size = item.Length
				break
			}
		}
	}
	return nil
}

// Remove deletes the object
func (o *Object) Remove(ctx context.Context) (err error) {
	reqPath := o.fs.getFullFilePath(o.remote, false)
	opts := rest.Opts{
		Method:       "DELETE",
		Path:         reqPath,
		IgnoreStatus: true,
		NoResponse:   true,
	}
	var resp *http.Response
	err = o.fs.pacer.Call(func() (bool, error) {
		resp, err = o.fs.srv.Call(ctx, &opts)
		if err != nil {
			return false, fmt.Errorf("failed to delete %s: %w", o.remote, err)
		}
		return shouldRetry(ctx, resp, err)
	})
	if err != nil {
		return err
	}
	if resp.StatusCode != 200 {
		return fmt.Errorf("failed to delete %s: status %d", o.remote, resp.StatusCode)
	}
	return nil
}
