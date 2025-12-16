// Package shade provides an interface to the Shade storage system.
package shade

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/rclone/rclone/backend/shade/api"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config"
	"github.com/rclone/rclone/fs/config/configmap"
	"github.com/rclone/rclone/fs/config/configstruct"
	"github.com/rclone/rclone/fs/fshttp"
	"github.com/rclone/rclone/fs/hash"
	"github.com/rclone/rclone/fs/object"
	"github.com/rclone/rclone/lib/encoder"
	"github.com/rclone/rclone/lib/pacer"
	"github.com/rclone/rclone/lib/rest"
)

const (
	defaultEndpoint  = "https://fs.shade.inc"        // Default local development endpoint
	apiEndpoint      = "https://api.shade.inc"       // API endpoint for getting tokens
	minSleep         = 10 * time.Millisecond         // Minimum sleep time for the pacer
	maxSleep         = 5 * time.Minute               // Maximum sleep time for the pacer
	decayConstant    = 1                             // Bigger for slower decay, exponential
	defaultChunkSize = int64(64 * 1024 * 1024)       // Default chunk size (64MB)
	minChunkSize     = int64(5 * 1024 * 1024)        // Minimum chunk size (5MB) - S3 requirement
	maxChunkSize     = int64(5 * 1024 * 1024 * 1024) // Maximum chunk size (5GB)
	maxUploadParts   = 10000                         // maximum allowed number of parts in a multipart upload
)

// Register with Fs
func init() {
	fs.Register(&fs.RegInfo{
		Name:        "shade",
		Description: "Shade FS",
		NewFs:       NewFS,
		Options: []fs.Option{{
			Name:      "drive_id",
			Help:      "The ID of your drive, see this in the drive settings. Individual rclone configs must be made per drive.",
			Required:  true,
			Sensitive: false,
		}, {
			Name:      "api_key",
			Help:      "An API key for your account.",
			Required:  true,
			Sensitive: true,
		}, {
			Name:     "endpoint",
			Help:     "Endpoint for the service.\n\nLeave blank normally.",
			Advanced: true,
		}, {
			Name:     "chunk_size",
			Help:     "Chunk size to use for uploading.\n\nAny files larger than this will be uploaded in chunks of this size.\n\nNote that this is stored in memory per transfer, so increasing it will\nincrease memory usage.\n\nMinimum is 5MB, maximum is 5GB.",
			Default:  fs.SizeSuffix(defaultChunkSize),
			Advanced: true,
		}, {
			Name:     "upload_concurrency",
			Help:     `Concurrency for multipart uploads and copies. This is the number of chunks of the same file that are uploaded concurrently for multipart uploads and copies.`,
			Default:  4,
			Advanced: true,
		}, {
			Name:     "max_upload_parts",
			Help:     "Maximum amount of parts in a multipart upload.",
			Default:  maxUploadParts,
			Advanced: true,
		}, {
			Name:     "token",
			Help:     "JWT Token for performing Shade FS operations. Don't set this value - rclone will set it automatically",
			Default:  "",
			Advanced: true,
		}, {
			Name:     "token_expiry",
			Help:     "JWT Token Expiration time. Don't set this value - rclone will set it automatically",
			Default:  "",
			Advanced: true,
		}, {
			Name:     config.ConfigEncoding,
			Help:     config.ConfigEncodingHelp,
			Advanced: true,
			Default: encoder.Display |
				encoder.EncodeBackSlash |
				encoder.EncodeInvalidUtf8,
		}},
	})
}

// refreshJWTToken retrieves or refreshes the ShadeFS token
func (f *Fs) refreshJWTToken(ctx context.Context) (string, error) {
	f.tokenMu.Lock()
	defer f.tokenMu.Unlock()
	// Return existing token if it's still valid
	checkTime := f.tokenExp.Add(-2 * time.Minute)
	//If the token expires in less than two minutes, just get a new one
	if f.token != "" && time.Now().Before(checkTime) {
		return f.token, nil
	}

	// Token has expired or doesn't exist, get a new one
	opts := rest.Opts{
		Method:  "GET",
		RootURL: apiEndpoint,
		Path:    fmt.Sprintf("/workspaces/drives/%s/shade-fs-token", f.drive),
		ExtraHeaders: map[string]string{
			"Authorization": f.opt.APIKey,
		},
	}

	var err error
	var tokenStr string

	err = f.pacer.Call(func() (bool, error) {
		res, err := f.apiSrv.Call(ctx, &opts)
		if err != nil {
			fs.Debugf(f, "Token request failed: %v", err)
			return false, err
		}

		defer fs.CheckClose(res.Body, &err)

		if res.StatusCode != http.StatusOK {
			fs.Debugf(f, "Token request failed with code: %d", res.StatusCode)
			return res.StatusCode == http.StatusTooManyRequests, fmt.Errorf("failed to get ShadeFS token, status: %d", res.StatusCode)
		}

		// Read token directly as plain text
		tokenBytes, err := io.ReadAll(res.Body)
		if err != nil {
			return false, err
		}

		tokenStr = strings.TrimSpace(string(tokenBytes))
		return false, nil
	})

	if err != nil {
		return "", err
	}

	if tokenStr == "" {
		return "", fmt.Errorf("empty token received from server")
	}

	parts := strings.Split(tokenStr, ".")
	if len(parts) < 2 {
		return "", fmt.Errorf("invalid token received from server")
	}
	// Decode the payload (2nd part of the token)
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return "", fmt.Errorf("invalid token received from server")
	}
	var claims map[string]interface{}
	if err := json.Unmarshal(payload, &claims); err != nil {
		return "", err
	}
	var exp int64
	// Extract exp/
	if v, ok := claims["exp"].(float64); ok {
		exp = int64(v)
	}

	f.token = tokenStr
	f.tokenExp = time.Unix(exp, 0)

	f.m.Set("token", f.token)
	f.m.Set("token_expiry", f.tokenExp.Format(time.RFC3339))

	return f.token, nil
}

func (f *Fs) callAPI(ctx context.Context, method, path string, response interface{}) (*http.Response, error) {
	token, err := f.refreshJWTToken(ctx)
	if err != nil {
		return nil, err
	}
	opts := rest.Opts{
		Method:  method,
		Path:    path,
		RootURL: f.endpoint,
		ExtraHeaders: map[string]string{
			"Authorization": "Bearer " + token,
		},
	}
	var res *http.Response
	err = f.pacer.Call(func() (bool, error) {
		if response != nil {
			res, err = f.srv.CallJSON(ctx, &opts, nil, response)
		} else {
			res, err = f.srv.Call(ctx, &opts)
		}
		if err != nil {
			return res != nil && res.StatusCode == http.StatusTooManyRequests, err
		}
		return false, nil
	})
	return res, err
}

// Options defines the configuration for this backend
type Options struct {
	Drive          string        `config:"drive_id"`
	APIKey         string        `config:"api_key"`
	Endpoint       string        `config:"endpoint"`
	ChunkSize      fs.SizeSuffix `config:"chunk_size"`
	MaxUploadParts int           `config:"max_upload_parts"`
	Concurrency    int           `config:"upload_concurrency"`
	Token          string        `config:"token"`
	TokenExpiry    string        `config:"token_expiry"`
	Encoding       encoder.MultiEncoder
}

// Fs represents a shade remote
type Fs struct {
	name         string       // name of this remote
	root         string       // the path we are working on
	opt          Options      // parsed options
	features     *fs.Features // optional features
	srv          *rest.Client // REST client for ShadeFS API
	apiSrv       *rest.Client // REST client for Shade API
	endpoint     string       // endpoint for ShadeFS
	drive        string       // drive ID
	pacer        *fs.Pacer    // pacer for API calls
	token        string       // ShadeFS token
	tokenExp     time.Time    // Token expiration time
	tokenMu      sync.Mutex
	m            configmap.Mapper //Config Mapper to store tokens for future use
	recursive    bool
	createdDirs  map[string]bool // Cache of directories we've created
	createdDirMu sync.RWMutex    // Mutex for createdDirs map
}

// Object describes a ShadeFS object
type Object struct {
	fs       *Fs    // what this object is part of
	remote   string // The remote path
	mtime    int64  // Modified time
	size     int64  // Size of the object
	original string //Presigned download link
}

// Directory describes a ShadeFS directory
type Directory struct {
	fs     *Fs    // Reference to the filesystem
	remote string // Path to the directory
	mtime  int64  // Modification time
	size   int64  // Size (typically 0 for directories)
}

// Name of the remote (as passed into NewFs)
func (f *Fs) Name() string {
	return f.name
}

// Root of the remote (as passed into NewFs)
func (f *Fs) Root() string {
	return f.root
}

// String returns a description of the FS
func (f *Fs) String() string {
	return fmt.Sprintf("Shade drive %s path %s", f.opt.Drive, f.root)
}

// Precision returns the precision of the ModTimes
func (f *Fs) Precision() time.Duration {
	return fs.ModTimeNotSupported
}

// Move src to this remote using server-side move operations.
//
// This is stored with the remote path given.
//
// It returns the destination Object and a possible error.
//
// Will only be called if src.Fs().Name() == f.Name()
//
// If it isn't possible then return fs.ErrorCantMove
func (f *Fs) Move(ctx context.Context, src fs.Object, remote string) (fs.Object, error) {
	srcObj, ok := src.(*Object)
	if !ok {
		fs.Debugf(src, "Can't move - not same remote type")
		return nil, fs.ErrorCantMove
	}

	token, err := f.refreshJWTToken(ctx)
	if err != nil {
		return nil, err
	}

	//Need to make sure destination exists
	err = f.ensureParentDirectories(ctx, remote)
	if err != nil {
		return nil, err
	}

	// Create temporary object
	o := &Object{
		fs:     f,
		remote: remote,
		mtime:  srcObj.mtime,
		size:   srcObj.size,
	}
	fromFullPath := path.Join(src.Fs().Root(), srcObj.remote)
	toFullPath := path.Join(f.root, remote)

	// Build query parameters
	params := url.Values{}
	params.Set("path", remote)
	params.Set("from", fromFullPath)
	params.Set("to", toFullPath)

	opts := rest.Opts{
		Method: "POST",
		Path:   fmt.Sprintf("/%s/fs/move?%s", f.drive, params.Encode()),
		ExtraHeaders: map[string]string{
			"Authorization": "Bearer " + token,
		},
	}

	err = o.fs.pacer.Call(func() (bool, error) {
		resp, err := f.srv.Call(ctx, &opts)

		if err != nil && resp.StatusCode == http.StatusBadRequest {
			fs.Debugf(f, "Bad token from server: %v", token)
		}

		return resp != nil && resp.StatusCode == http.StatusTooManyRequests, err
	})
	if err != nil {
		return nil, err
	}
	return o, nil
}

// DirMove moves src, srcRemote to this remote at dstRemote
// using server-side move operations.
//
// Will only be called if src.Fs().Name() == f.Name()
//
// If it isn't possible then return fs.ErrorCantDirMove
//
// If destination exists then return fs.ErrorDirExists
func (f *Fs) DirMove(ctx context.Context, src fs.Fs, srcRemote, dstRemote string) error {
	srcFs, ok := src.(*Fs)
	if !ok {
		fs.Debugf(srcFs, "Can't move directory - not same remote type")
		return fs.ErrorCantDirMove
	}
	//Need to check if destination exists
	fullPath := f.buildFullPath(dstRemote)
	var response api.ListDirResponse
	res, _ := f.callAPI(ctx, "GET", fmt.Sprintf("/%s/fs/attr?path=%s", f.drive, fullPath), &response)

	if res.StatusCode != http.StatusNotFound {
		return fs.ErrorDirExists
	}

	fullPathSrc := f.buildFullPath(srcRemote)
	fullPathSrcUnencoded, err := url.QueryUnescape(fullPathSrc)
	if err != nil {
		return err
	}

	fullPathDstUnencoded, err := url.QueryUnescape(fullPath)
	if err != nil {
		return err
	}

	err = f.ensureParentDirectories(ctx, dstRemote)
	if err != nil {
		return err
	}

	o := &Object{
		fs:     srcFs,
		remote: srcRemote,
	}

	_, err = f.Move(ctx, o, dstRemote)

	if err == nil {

		f.createdDirMu.Lock()
		f.createdDirs[fullPathSrcUnencoded] = false
		f.createdDirs[fullPathDstUnencoded] = true
		f.createdDirMu.Unlock()
	}

	return err
}

// Hashes returns the supported hash types
func (f *Fs) Hashes() hash.Set {
	return hash.Set(hash.None)
}

// Features returns the optional features of this Fs
func (f *Fs) Features() *fs.Features {
	return f.features
}

// NewFS constructs an FS from the path, container:path
func NewFS(ctx context.Context, name, root string, m configmap.Mapper) (fs.Fs, error) {
	// Parse config into Options struct
	opt := new(Options)
	err := configstruct.Set(m, opt)
	if err != nil {
		return nil, err
	}

	fs.Debugf(nil, "Creating new ShadeFS backend with drive: %s", opt.Drive)

	f := &Fs{
		name:        name,
		root:        root,
		opt:         *opt,
		drive:       opt.Drive,
		m:           m,
		srv:         rest.NewClient(fshttp.NewClient(ctx)).SetRoot(defaultEndpoint),
		apiSrv:      rest.NewClient(fshttp.NewClient(ctx)),
		pacer:       fs.NewPacer(ctx, pacer.NewDefault(pacer.MinSleep(minSleep), pacer.MaxSleep(maxSleep), pacer.DecayConstant(decayConstant))),
		recursive:   true,
		createdDirs: make(map[string]bool),
		token:       opt.Token,
	}

	f.features = &fs.Features{
		// Initially set minimal features
		// We'll expand this in a future iteration
		CanHaveEmptyDirectories: true,
		Move:                    f.Move,
		DirMove:                 f.DirMove,
		OpenChunkWriter:         f.OpenChunkWriter,
	}

	if opt.TokenExpiry != "" {
		tokenExpiry, err := time.Parse(time.RFC3339, opt.TokenExpiry)
		if err != nil {
			fs.Errorf(nil, "Failed to parse token_expiry option: %v", err)
		} else {
			f.tokenExp = tokenExpiry
		}
	}

	// Set the endpoint
	if opt.Endpoint == "" {
		f.endpoint = defaultEndpoint
	} else {
		f.endpoint = opt.Endpoint
	}

	// Validate and set chunk size
	if opt.ChunkSize == 0 {
		opt.ChunkSize = fs.SizeSuffix(defaultChunkSize)
	} else if opt.ChunkSize < fs.SizeSuffix(minChunkSize) {
		return nil, fmt.Errorf("chunk_size %d is less than minimum %d", opt.ChunkSize, minChunkSize)
	} else if opt.ChunkSize > fs.SizeSuffix(maxChunkSize) {
		return nil, fmt.Errorf("chunk_size %d is greater than maximum %d", opt.ChunkSize, maxChunkSize)
	}

	// Ensure root doesn't have trailing slash
	f.root = strings.Trim(f.root, "/")

	// Check that we can log in by getting a token
	_, err = f.refreshJWTToken(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get ShadeFS token: %w", err)
	}

	var response api.ListDirResponse
	_, _ = f.callAPI(ctx, "GET", fmt.Sprintf("/%s/fs/attr?path=%s", f.drive, url.QueryEscape(root)), &response)

	if response.Type == "file" {
		//Specified a single file path, not a directory.
		f.root = filepath.Dir(f.root)
		return f, fs.ErrorIsFile
	}
	return f, nil
}

// NewObject finds the Object at remote
func (f *Fs) NewObject(ctx context.Context, remote string) (fs.Object, error) {

	fullPath := f.buildFullPath(remote)

	var response api.ListDirResponse
	res, err := f.callAPI(ctx, "GET", fmt.Sprintf("/%s/fs/attr?path=%s", f.drive, fullPath), &response)

	if res != nil && res.StatusCode == http.StatusNotFound {
		return nil, fs.ErrorObjectNotFound
	}

	if err != nil {
		return nil, err
	}

	if res != nil && res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("attr failed with status code: %d", res.StatusCode)
	}

	if response.Type == "tree" {
		return nil, fs.ErrorIsDir
	}

	if response.Type != "file" {
		return nil, fmt.Errorf("path is not a file: %s", remote)
	}

	return &Object{
		fs:     f,
		remote: remote,
		mtime:  response.Mtime,
		size:   response.Size,
	}, nil
}

// Put uploads a file
func (f *Fs) Put(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error) {
	// Create temporary object
	o := &Object{
		fs:     f,
		remote: src.Remote(),
	}
	return o, o.Update(ctx, in, src, options...)
}

// List the objects and directories in dir into entries
func (f *Fs) List(ctx context.Context, dir string) (entries fs.DirEntries, err error) {

	fullPath := f.buildFullPath(dir)

	var response []api.ListDirResponse
	res, err := f.callAPI(ctx, "GET", fmt.Sprintf("/%s/fs/listdir?path=%s", f.drive, fullPath), &response)
	if err != nil {
		fs.Debugf(f, "Error from List call: %v", err)
		return nil, fs.ErrorDirNotFound
	}

	if res.StatusCode == http.StatusNotFound {
		fs.Debugf(f, "Directory not found")
		return nil, fs.ErrorDirNotFound
	}

	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("listdir failed with status code: %d", res.StatusCode)
	}

	for _, r := range response {
		if r.Draft {
			continue
		}

		// Make path relative to f.root
		entryPath := strings.TrimPrefix(r.Path, "/")
		if f.root != "" {
			if !strings.HasPrefix(entryPath, f.root) {
				continue
			}
			entryPath = strings.TrimPrefix(strings.TrimPrefix(entryPath, f.root), "/")
		}

		if r.Type == "file" {
			entries = append(entries, &Object{
				fs:     f,
				remote: entryPath,
				mtime:  r.Mtime,
				size:   r.Size,
			})
		} else if r.Type == "tree" {
			dirEntry := &Directory{
				fs:     f,
				remote: entryPath,
				mtime:  r.Mtime,
				size:   r.Size, // Typically 0 for directories
			}
			entries = append(entries, dirEntry)
		} else {
			fs.Debugf(f, "Unknown entry type: %s for path: %s", r.Type, entryPath)
		}
	}

	return entries, nil
}

// ensureParentDirectories creates all parent directories for a given path
func (f *Fs) ensureParentDirectories(ctx context.Context, remotePath string) error {
	// Build the full path including root
	fullPath := remotePath
	if f.root != "" {
		fullPath = path.Join(f.root, remotePath)
	}

	// Get the parent directory path
	parentDir := path.Dir(fullPath)

	// If parent is root, empty, or current dir, nothing to create
	if parentDir == "" || parentDir == "." || parentDir == "/" {
		return nil
	}

	// Ensure the full parent directory path exists
	return f.ensureDirectoryPath(ctx, parentDir)
}

// ensureDirectoryPath creates all directories in a path
func (f *Fs) ensureDirectoryPath(ctx context.Context, dirPath string) error {
	// Check cache first
	f.createdDirMu.RLock()
	if f.createdDirs[dirPath] {
		f.createdDirMu.RUnlock()
		return nil
	}
	f.createdDirMu.RUnlock()

	// Build list of all directories that need to be created
	var dirsToCreate []string
	currentPath := dirPath

	for currentPath != "" && currentPath != "." && currentPath != "/" {
		// Check if this directory is already in cache
		f.createdDirMu.RLock()
		inCache := f.createdDirs[currentPath]
		f.createdDirMu.RUnlock()

		if !inCache {
			dirsToCreate = append([]string{currentPath}, dirsToCreate...)
		}
		currentPath = path.Dir(currentPath)
	}

	// If all directories are cached, we're done
	if len(dirsToCreate) == 0 {
		return nil
	}

	// Create each directory in order
	for _, dir := range dirsToCreate {

		fullPath := url.QueryEscape(dir)
		res, err := f.callAPI(ctx, "POST", fmt.Sprintf("/%s/fs/mkdir?path=%s", f.drive, fullPath), nil)

		// If directory already exists, that's fine
		if err == nil && res != nil {
			if res.StatusCode == http.StatusConflict || res.StatusCode == http.StatusUnprocessableEntity {
				f.createdDirMu.Lock()
				f.createdDirs[dir] = true
				f.createdDirMu.Unlock()
			} else if res.StatusCode != http.StatusOK && res.StatusCode != http.StatusCreated {
				fs.Debugf(f, "Failed to create directory %s: status code %d", dir, res.StatusCode)
			} else {
				f.createdDirMu.Lock()
				f.createdDirs[dir] = true
				f.createdDirMu.Unlock()
			}

			fs.CheckClose(res.Body, &err)
		} else if err != nil {
			fs.Debugf(f, "Error creating directory %s: %v", dir, err)
			// Continue anyway
			continue
		}
	}

	// Mark the full path as created in cache
	f.createdDirMu.Lock()
	f.createdDirs[dirPath] = true
	f.createdDirMu.Unlock()

	return nil
}

// Mkdir creates the container if it doesn't exist
func (f *Fs) Mkdir(ctx context.Context, dir string) error {

	// Build the full path for the directory
	fullPath := dir
	if dir == "" {
		// If dir is empty, we're creating the root directory
		if f.root != "" && f.root != "/" && f.root != "." {
			fullPath = f.root
		} else {
			// Nothing to create
			return nil
		}
	} else if f.root != "" {
		fullPath = path.Join(f.root, dir)
	}

	// Ensure all parent directories exist first
	if err := f.ensureDirectoryPath(ctx, fullPath); err != nil {
		return fmt.Errorf("failed to create directory path: %w", err)
	}

	// Add to cache
	f.createdDirMu.Lock()
	f.createdDirs[fullPath] = true
	f.createdDirMu.Unlock()

	return nil
}

// Rmdir deletes the root folder
//
// Returns an error if it isn't empty
func (f *Fs) Rmdir(ctx context.Context, dir string) error {
	fullPath := f.buildFullPath(dir)

	if fullPath == "" {
		return errors.New("cannot delete root directory")
	}

	var response []api.ListDirResponse
	res, err := f.callAPI(ctx, "GET", fmt.Sprintf("/%s/fs/listdir?path=%s", f.drive, fullPath), &response)

	if res != nil && res.StatusCode != http.StatusOK {
		return err
	}

	if len(response) > 0 {
		return fs.ErrorDirectoryNotEmpty
	}

	// Use the delete endpoint which handles both files and directories
	res, err = f.callAPI(ctx, "POST", fmt.Sprintf("/%s/fs/delete?path=%s", f.drive, fullPath), nil)
	if err != nil {
		return err
	}
	defer fs.CheckClose(res.Body, &err)

	if res.StatusCode == http.StatusNotFound {
		return fs.ErrorDirNotFound
	}

	if res.StatusCode != http.StatusOK && res.StatusCode != http.StatusCreated {
		return fmt.Errorf("rmdir failed with status code: %d", res.StatusCode)
	}

	f.createdDirMu.Lock()
	defer f.createdDirMu.Unlock()
	unescapedPath, err := url.QueryUnescape(fullPath)
	if err != nil {
		return err
	}
	f.createdDirs[unescapedPath] = false

	return nil
}

// Attempts to construct the full path for an object query-escaped
func (f *Fs) buildFullPath(remote string) string {
	if f.root == "" {
		return url.QueryEscape(remote)
	}
	return url.QueryEscape(path.Join(f.root, remote))
}

// -------------------------------------------------
// Object implementation
// -------------------------------------------------

// Fs returns the parent Fs
func (o *Object) Fs() fs.Info {
	return o.fs
}

// String returns a description of the Object
func (o *Object) String() string {
	if o == nil {
		return "<nil>"
	}
	return o.remote
}

// Remote returns the remote path
func (o *Object) Remote() string {
	return o.remote
}

// Hash returns the requested hash of the object content
func (o *Object) Hash(ctx context.Context, t hash.Type) (string, error) {
	return "", hash.ErrUnsupported
}

// Size returns the size of the object
func (o *Object) Size() int64 {
	return o.size
}

// ModTime returns the modification date of the object
func (o *Object) ModTime(context.Context) time.Time {
	return time.Unix(0, o.mtime*int64(time.Millisecond))
}

// SetModTime sets the modification time of the object
func (o *Object) SetModTime(context.Context, time.Time) error {
	// Not implemented for now
	return fs.ErrorCantSetModTime
}

// Storable returns whether this object is storable
func (o *Object) Storable() bool {
	return true
}

// Open an object for read
func (o *Object) Open(ctx context.Context, options ...fs.OpenOption) (io.ReadCloser, error) {

	if o.size == 0 {
		// Empty file: return an empty reader
		return io.NopCloser(bytes.NewReader(nil)), nil
	}
	fs.FixRangeOption(options, o.size)

	token, err := o.fs.refreshJWTToken(ctx)
	if err != nil {
		return nil, err
	}

	fullPath := o.fs.buildFullPath(o.remote)
	// Construct the initial request URL
	downloadURL := fmt.Sprintf("%s/%s/fs/download?path=%s", o.fs.endpoint, o.fs.drive, fullPath)

	// Create HTTP request manually
	req, err := http.NewRequestWithContext(ctx, "GET", downloadURL, nil)
	if err != nil {
		fs.Debugf(o.fs, "Failed to create request: %v", err)
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)

	// Use pacer to manage retries and rate limiting
	var res *http.Response
	err = o.fs.pacer.Call(func() (bool, error) {

		if res != nil {
			err = res.Body.Close()
			if err != nil {
				return false, err
			}
		}

		client := http.Client{
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return http.ErrUseLastResponse // Don't follow redirects
			},
		}
		res, err = client.Do(req)
		if err != nil {
			return false, err
		}
		return res.StatusCode == http.StatusTooManyRequests, nil
	})

	if err != nil {
		return nil, fmt.Errorf("download request failed: %w", err)
	}
	if res == nil {
		return nil, fmt.Errorf("no response received from initial request")
	}

	// Handle response based on status code
	switch res.StatusCode {
	case http.StatusOK:
		return res.Body, nil

	case http.StatusTemporaryRedirect:
		// Read the presigned URL from the body
		bodyBytes, err := io.ReadAll(res.Body)
		fs.CheckClose(res.Body, &err) // Close body after reading
		if err != nil {
			return nil, fmt.Errorf("failed to read redirect body: %w", err)
		}

		presignedURL := strings.TrimSpace(string(bodyBytes))
		o.original = presignedURL //Save for later for hashing

		client := rest.NewClient(fshttp.NewClient(ctx)).SetRoot(presignedURL)
		var downloadRes *http.Response
		opts := rest.Opts{
			Method:  "GET",
			Path:    "",
			Options: options,
		}

		err = o.fs.pacer.Call(func() (bool, error) {
			downloadRes, err = client.Call(ctx, &opts)
			if err != nil {
				return false, err
			}
			if downloadRes == nil {
				return false, fmt.Errorf("failed to fetch presigned URL")
			}
			return downloadRes.StatusCode == http.StatusTooManyRequests, nil
		})

		if err != nil {
			return nil, fmt.Errorf("presigned URL request failed: %w", err)
		}
		if downloadRes == nil {
			return nil, fmt.Errorf("no response received from presigned URL request")
		}

		if downloadRes.StatusCode != http.StatusOK && downloadRes.StatusCode != http.StatusPartialContent {
			body, _ := io.ReadAll(downloadRes.Body)
			fs.CheckClose(downloadRes.Body, &err)
			return nil, fmt.Errorf("presigned URL request failed with status %d: %q", downloadRes.StatusCode, string(body))
		}

		return downloadRes.Body, nil

	default:
		body, _ := io.ReadAll(res.Body)
		fs.CheckClose(res.Body, &err)
		return nil, fmt.Errorf("download failed with status %d: %q", res.StatusCode, string(body))
	}
}

// Update in to the object with the modTime given of the given size
//
// When called from outside an Fs by rclone, src.Size() will always be >= 0.
// But for unknown-sized objects (indicated by src.Size() == -1), Upload should either
// return an error or update the object properly (rather than e.g. calling panic).
func (o *Object) Update(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) error {

	//Need to ensure parent directories exist before updating
	err := o.fs.ensureParentDirectories(ctx, o.remote)
	if err != nil {
		return err
	}

	//If the source remote is different from this object's remote, as in we're updating a file with some other file's data,
	//need to construct a new object info in order to correctly upload to THIS object, not the src one
	var srcInfo fs.ObjectInfo
	if o.remote != src.Remote() {
		srcInfo = object.NewStaticObjectInfo(o.remote, src.ModTime(ctx), src.Size(), true, nil, o.Fs())
	} else {
		srcInfo = src
	}

	return o.uploadMultipart(ctx, srcInfo, in, options...)
}

// Remove removes the object
func (o *Object) Remove(ctx context.Context) error {

	fullPath := o.fs.buildFullPath(o.remote)

	res, err := o.fs.callAPI(ctx, "POST", fmt.Sprintf("/%s/fs/delete?path=%s", o.fs.drive, fullPath), nil)
	if err != nil {
		return err
	}
	defer fs.CheckClose(res.Body, &err) // Ensure body is closed

	if res.StatusCode != http.StatusOK && res.StatusCode != http.StatusCreated {
		return fmt.Errorf("object removal failed with status code: %d", res.StatusCode)
	}
	return nil
}

// -------------------------------------------------
// Directory implementation
// -------------------------------------------------

// Remote returns the remote path
func (d *Directory) Remote() string {
	return d.remote
}

// ModTime returns the modification time
func (d *Directory) ModTime(context.Context) time.Time {
	return time.Unix(0, d.mtime*int64(time.Millisecond))
}

// Size returns the size (0 for directories)
func (d *Directory) Size() int64 {
	return d.size
}

// Fs returns the filesystem info
func (d *Directory) Fs() fs.Info {
	return d.fs
}

// Hash is unsupported for directories
func (d *Directory) Hash(context.Context, hash.Type) (string, error) {
	return "", hash.ErrUnsupported
}

// SetModTime is unsupported for directories
func (d *Directory) SetModTime(context.Context, time.Time) error {
	return fs.ErrorCantSetModTime
}

// Storable indicates directories aren’t storable as files
func (d *Directory) Storable() bool {
	return false
}

// Open returns an error for directories
func (d *Directory) Open() (io.ReadCloser, error) {
	return nil, fs.ErrorIsDir
}

// Items returns the number of items in the directory (-1 if unknown)
func (d *Directory) Items() int64 {
	return -1 // Unknown
}

// ID returns the directory ID (empty if not applicable)
func (d *Directory) ID() string {
	return ""
}

func (d *Directory) String() string {
	return fmt.Sprintf("Directory: %s", d.remote)
}

var (
	_ fs.Fs        = &Fs{}
	_ fs.Object    = &Object{}
	_ fs.Directory = &Directory{}

	_ fs.Mover    = &Fs{}
	_ fs.DirMover = &Fs{}
)
