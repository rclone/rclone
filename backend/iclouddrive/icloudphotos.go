//go:build !plan9 && !solaris

// iCloud Photos backend implementation
package iclouddrive

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"path"
	"strings"
	"time"

	"github.com/rclone/rclone/backend/iclouddrive/api"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config/configmap"
	"github.com/rclone/rclone/fs/config/configstruct"
	"github.com/rclone/rclone/fs/config/obscure"
	"github.com/rclone/rclone/fs/hash"
	"github.com/rclone/rclone/lib/dircache"
	"github.com/rclone/rclone/lib/encoder"
	"github.com/rclone/rclone/lib/pacer"
)

const rootID = "photos-root"

// PhotosOptions defines the configuration for the Photos backend
type PhotosOptions struct {
	AppleID    string               `config:"apple_id"`
	Password   string               `config:"password"`
	TrustToken string               `config:"trust_token"`
	Cookies    string               `config:"cookies"`
	ClientID   string               `config:"client_id"`
	Enc        encoder.MultiEncoder `config:"encoding"`
}

// PhotosFs represents a remote iCloud Photos server
type PhotosFs struct {
	name     string
	root     string
	opt      PhotosOptions
	features *fs.Features
	icloud   *api.Client
	pacer    *fs.Pacer
	dirCache *dircache.DirCache
}

// PhotosObject describes an iCloud Photos object
type PhotosObject struct {
	fs          *PhotosFs
	remote      string
	size        int64
	modTime     time.Time
	downloadURL string
}

// NewFsPhotos constructs an Fs for Photos from the path, container:path
func NewFsPhotos(ctx context.Context, name, root string, m configmap.Mapper) (fs.Fs, error) {
	// Parse configuration
	opt := new(PhotosOptions)
	err := configstruct.Set(m, opt)
	if err != nil {
		return nil, err
	}

	if opt.Password != "" {
		opt.Password, err = obscure.Reveal(opt.Password)
		if err != nil {
			return nil, fmt.Errorf("couldn't decrypt user password: %w", err)
		}
	}

	if opt.TrustToken == "" {
		return nil, fmt.Errorf("missing icloud trust token: try refreshing it with \"rclone config reconnect %s:\"", name)
	}

	cookies := ReadCookies(opt.Cookies)

	callback := func(session *api.Session) {
		m.Set(configCookies, session.GetCookieString())
	}

	icloud, err := api.New(
		opt.AppleID,
		opt.Password,
		opt.TrustToken,
		opt.ClientID,
		cookies,
		callback,
	)
	if err != nil {
		return nil, err
	}

	if err := icloud.Authenticate(ctx); err != nil {
		return nil, err
	}

	if icloud.Session.Requires2FA() {
		return nil, errors.New("trust token expired, please reauth")
	}

	root = strings.Trim(root, "/")

	f := &PhotosFs{
		name:   name,
		root:   root,
		opt:    *opt,
		icloud: icloud,
		pacer:  fs.NewPacer(ctx, pacer.NewDefault(pacer.MinSleep(minSleep), pacer.MaxSleep(maxSleep), pacer.DecayConstant(decayConstant))),
	}

	f.features = (&fs.Features{
		CanHaveEmptyDirectories: false,
		PartialUploads:          false,
	}).Fill(ctx, f)

	f.dirCache = dircache.New(root, rootID, f)

	err = f.dirCache.FindRoot(ctx, false)
	if err != nil {
		// Photos mode doesn't support file-like roots
		return f, nil
	}

	return f, nil
}

// PhotosFs implements fs.Fs

// Name implements fs.Fs
func (f *PhotosFs) Name() string {
	return f.name
}

// Root implements fs.Fs
func (f *PhotosFs) Root() string {
	return f.root
}

// String implements fs.Fs
func (f *PhotosFs) String() string {
	return fmt.Sprintf("iCloud Photos root '%s'", f.root)
}

// Precision implements fs.Fs
func (f *PhotosFs) Precision() time.Duration {
	return time.Second
}

// Hashes implements fs.Fs
func (f *PhotosFs) Hashes() hash.Set {
	return hash.Set(hash.None)
}

// Features implements fs.Fs
func (f *PhotosFs) Features() *fs.Features {
	return f.features
}

// List implements fs.Fs
func (f *PhotosFs) List(ctx context.Context, dir string) (entries fs.DirEntries, err error) {
	photosService, err := f.icloud.PhotosService()
	if err != nil {
		return nil, fmt.Errorf("failed to get photos service: %w", err)
	}

	entries = make(fs.DirEntries, 0)

	// Combine root and dir to get full path
	fullPath := path.Join(f.root, dir)
	fullPath = strings.Trim(fullPath, "/")

	// Handle root directory - list libraries
	if fullPath == "" {
		libraries, err := photosService.GetLibraries(ctx)
		if err != nil {
			return nil, err
		}

		for libraryName, library := range libraries {
			// Get album count for this library
			count, err := library.GetAlbumCount(ctx)
			if err != nil {
				fs.Debugf(nil, "Failed to get album count for library %q: %v", libraryName, err)
				count = -1 // Use -1 to indicate unknown count
			}

			d := fs.NewDir(libraryName, time.Now()).SetItems(count)
			entries = append(entries, d)
		}
		return entries, nil
	}

	// Parse directory path
	parts := strings.Split(fullPath, "/")

	if len(parts) == 1 {
		// List albums in a library
		libraryName := parts[0]
		libraries, err := photosService.GetLibraries(ctx)
		if err != nil {
			return nil, err
		}

		library, exists := libraries[libraryName]
		if !exists {
			return nil, fs.ErrorDirNotFound
		}

		albums, err := library.GetAlbums(ctx)
		if err != nil {
			return nil, err
		}

		for albumName, album := range albums {
			// Get photo count for this album
			count, err := album.GetPhotoCount(ctx)
			if err != nil {
				fs.Debugf(nil, "Failed to get photo count for album %q in library %q: %v", albumName, libraryName, err)
				count = -1 // Use -1 to indicate unknown count
			}

			d := fs.NewDir(albumName, time.Now()).SetSize(count)
			entries = append(entries, d)
		}
		return entries, nil
	}

	if len(parts) == 2 {
		// List photos in an album
		libraryName := parts[0]
		albumName := parts[1]

		libraries, err := photosService.GetLibraries(ctx)
		if err != nil {
			return nil, err
		}

		library, exists := libraries[libraryName]
		if !exists {
			return nil, fs.ErrorDirNotFound
		}

		albums, err := library.GetAlbums(ctx)
		if err != nil {
			return nil, err
		}

		album, exists := albums[albumName]
		if !exists {
			return nil, fs.ErrorDirNotFound
		}

		photos, err := album.GetPhotos(ctx, 0)
		if err != nil {
			return nil, err
		}

		for _, photo := range photos {
			if photo.Filename == "" {
				continue
			}
			// Build remote path including directory prefix when needed
			remotePath := photo.Filename
			if dir != "" {
				remotePath = dir + "/" + photo.Filename
			}
			modTime := time.Unix(photo.AssetDate/1000, 0)
			o := &PhotosObject{
				fs:          f,
				remote:      remotePath,
				size:        photo.Size,
				modTime:     modTime,
				downloadURL: photo.DownloadURL,
			}
			entries = append(entries, o)
		}
		return entries, nil
	}

	// Deeper nesting not supported in Photos
	return nil, fs.ErrorDirNotFound
}

// NewObject implements fs.Fs
func (f *PhotosFs) NewObject(ctx context.Context, remote string) (fs.Object, error) {
	// Combine root and remote to get full path
	fullPath := path.Join(f.root, remote)
	parts := strings.Split(strings.Trim(fullPath, "/"), "/")
	if len(parts) != 3 {
		return nil, fs.ErrorObjectNotFound
	}

	libraryName := parts[0]
	albumName := parts[1]
	filename := parts[2]

	photosService, err := f.icloud.PhotosService()
	if err != nil {
		return nil, fmt.Errorf("failed to get photos service: %w", err)
	}

	photo, err := photosService.GetPhoto(ctx, libraryName, albumName, filename)
	if err != nil {
		return nil, fs.ErrorObjectNotFound
	}

	modTime := time.Unix(photo.AssetDate/1000, 0)
	return &PhotosObject{
		fs:          f,
		remote:      remote,
		size:        photo.Size,
		modTime:     modTime,
		downloadURL: photo.DownloadURL,
	}, nil
}

// Put implements fs.Fs (read-only)
func (f *PhotosFs) Put(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error) {
	return nil, fs.ErrorNotImplemented
}

// Mkdir implements fs.Fs (read-only)
func (f *PhotosFs) Mkdir(ctx context.Context, dir string) error {
	return fs.ErrorNotImplemented
}

// Rmdir implements fs.Fs (read-only)
func (f *PhotosFs) Rmdir(ctx context.Context, dir string) error {
	return fs.ErrorNotImplemented
}

// FindLeaf finds a directory of name leaf in the folder with ID pathID
func (f *PhotosFs) FindLeaf(ctx context.Context, pathID, leaf string) (pathIDOut string, found bool, err error) {
	photosService, err := f.icloud.PhotosService()
	if err != nil {
		return "", false, fmt.Errorf("failed to get photos service: %w", err)
	}

	if pathID == rootID {
		// Looking for a library at root level
		libraries, err := photosService.GetLibraries(ctx)
		if err != nil {
			return "", false, err
		}

		if _, exists := libraries[leaf]; exists {
			// Return library name as the path ID
			return "lib:" + leaf, true, nil
		}
		return "", false, nil
	}

	// Check if pathID is a library ID (lib:libraryName format)
	if strings.HasPrefix(pathID, "lib:") {
		libraryName := strings.TrimPrefix(pathID, "lib:")
		libraries, err := photosService.GetLibraries(ctx)
		if err != nil {
			return "", false, err
		}

		library, exists := libraries[libraryName]
		if !exists {
			return "", false, fs.ErrorDirNotFound
		}

		// This is a library, look for album
		albums, err := library.GetAlbums(ctx)
		if err != nil {
			return "", false, err
		}

		if album, exists := albums[leaf]; exists {
			// Create unique album ID combining library and album info
			albumID := "album:" + libraryName + ":" + album.RecordName
			return albumID, true, nil
		}
		return "", false, nil
	}

	// Albums contain photos, not directories
	return "", false, fs.ErrorIsFile
}

// CreateDir is not supported for read-only Photos
func (f *PhotosFs) CreateDir(ctx context.Context, pathID, leaf string) (newID string, err error) {
	return "", fs.ErrorNotImplemented
}

// PhotosObject implements fs.Object

// Fs implements fs.Object
func (o *PhotosObject) Fs() fs.Info {
	return o.fs
}

// String implements fs.Object
func (o *PhotosObject) String() string {
	if o == nil {
		return "<nil>"
	}
	return o.remote
}

// Remote implements fs.Object
func (o *PhotosObject) Remote() string {
	return o.remote
}

// ModTime implements fs.Object
func (o *PhotosObject) ModTime(ctx context.Context) time.Time {
	return o.modTime
}

// Size implements fs.Object
func (o *PhotosObject) Size() int64 {
	return o.size
}

// Storable implements fs.Object
func (o *PhotosObject) Storable() bool {
	return false // Read-only
}

// Hash implements fs.Object
func (o *PhotosObject) Hash(ctx context.Context, ty hash.Type) (string, error) {
	return "", hash.ErrUnsupported
}

// SetModTime implements fs.Object (read-only)
func (o *PhotosObject) SetModTime(ctx context.Context, modTime time.Time) error {
	return fs.ErrorNotImplemented
}

// Open implements fs.Object
func (o *PhotosObject) Open(ctx context.Context, options ...fs.OpenOption) (io.ReadCloser, error) {
	if o.downloadURL == "" {
		return nil, errors.New("no download URL available for this object")
	}

	// Handle range requests
	var rangeOption *fs.RangeOption
	for _, option := range options {
		if ro, ok := option.(*fs.RangeOption); ok {
			rangeOption = ro
			break
		}
	}

	var result io.ReadCloser
	err := o.fs.pacer.Call(func() (bool, error) {
		var err error
		result, err = o.openURL(ctx, o.downloadURL, rangeOption)
		return false, err // Don't retry on errors for now
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (o *PhotosObject) openURL(ctx context.Context, url string, rangeOption *fs.RangeOption) (io.ReadCloser, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Add range header if specified
	if rangeOption != nil {
		key, value := rangeOption.Header()
		req.Header.Set(key, value)
	}

	// Add authentication headers
	headers := o.fs.icloud.Session.GetHeaders(nil)
	for key, value := range headers {
		req.Header.Set(key, value)
	}

	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to download: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		resp.Body.Close()
		return nil, fmt.Errorf("download failed with status %d: %s", resp.StatusCode, resp.Status)
	}

	return resp.Body, nil
}

// Update implements fs.Object (read-only)
func (o *PhotosObject) Update(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) error {
	return fs.ErrorNotImplemented
}

// Remove implements fs.Object (read-only)
func (o *PhotosObject) Remove(ctx context.Context) error {
	return fs.ErrorNotImplemented
}
