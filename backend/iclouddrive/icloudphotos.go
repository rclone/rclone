//go:build !plan9 && !solaris

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
	"github.com/rclone/rclone/fs/fshttp"
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
	name       string
	root       string
	opt        PhotosOptions
	features   *fs.Features
	icloud     *api.Client
	pacer      *fs.Pacer
	dirCache   *dircache.DirCache
	httpClient *http.Client
	startTime  time.Time
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
		name:       name,
		root:       root,
		opt:        *opt,
		icloud:     icloud,
		pacer:      fs.NewPacer(ctx, pacer.NewDefault(pacer.MinSleep(minSleep), pacer.MaxSleep(maxSleep), pacer.DecayConstant(decayConstant))),
		httpClient: fshttp.NewClient(ctx),
		startTime:  time.Now(),
	}

	f.features = (&fs.Features{
		CanHaveEmptyDirectories: false,
		PartialUploads:          false,
		ReadMimeType:            false,
	}).Fill(ctx, f)

	f.dirCache = dircache.New(root, rootID, f)

	err = f.dirCache.FindRoot(ctx, false)
	if err != nil {
		// Check if root points to a file (e.g. PrimarySync/Videos/file.mp4)
		newRoot, remote := dircache.SplitPath(root)
		tempF := *f
		tempF.root = newRoot
		tempF.dirCache = dircache.New(newRoot, rootID, &tempF)
		err2 := tempF.dirCache.FindRoot(ctx, false)
		if err2 != nil {
			return f, nil
		}
		_, err2 = tempF.NewObject(ctx, remote)
		if err2 != nil {
			return f, nil
		}
		// Root is a file — adjust f to parent dir, signal ErrorIsFile
		f.root = newRoot
		f.dirCache = tempF.dirCache
		return f, fs.ErrorIsFile
	}

	return f, nil
}

// PhotosFs implements fs.Fs.

// Name implements fs.Fs.
func (f *PhotosFs) Name() string {
	return f.name
}

// Root implements fs.Fs.
func (f *PhotosFs) Root() string {
	return f.opt.Enc.ToStandardPath(f.root)
}

// String implements fs.Fs.
func (f *PhotosFs) String() string {
	return fmt.Sprintf("iCloud Photos root '%s'", f.root)
}

// Precision implements fs.Fs.
func (f *PhotosFs) Precision() time.Duration {
	return time.Second
}

// Hashes implements fs.Fs.
func (f *PhotosFs) Hashes() hash.Set {
	return hash.Set(hash.None)
}

// Features implements fs.Fs.
func (f *PhotosFs) Features() *fs.Features {
	return f.features
}

// List implements fs.Fs.
func (f *PhotosFs) List(ctx context.Context, dir string) (entries fs.DirEntries, err error) {
	photosService, err := f.icloud.PhotosService(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get photos service: %w", err)
	}

	// Resolve dir to a cached directory ID via dircache
	dirID, err := f.dirCache.FindDir(ctx, dir, false)
	if err != nil {
		return nil, err
	}

	entries = make(fs.DirEntries, 0)

	switch {
	case dirID == rootID:
		// List libraries
		libraries, err := photosService.GetLibraries(ctx)
		if err != nil {
			return nil, err
		}

		for libraryName, library := range libraries {
			count, err := library.GetAlbumCount(ctx)
			if err != nil {
				fs.Debugf(nil, "Failed to get album count for library %q: %v", libraryName, err)
				count = -1
			}

			d := fs.NewDir(path.Join(dir, f.opt.Enc.ToStandardName(libraryName)), f.startTime).SetItems(count)
			entries = append(entries, d)
		}

	case strings.HasPrefix(dirID, "lib:"):
		// List albums in a library
		libraryName := strings.TrimPrefix(dirID, "lib:")
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

		counts, err := library.GetAlbumCounts(ctx)
		if err != nil {
			fs.Debugf(nil, "Failed to get album counts for library %q: %v", libraryName, err)
		}

		for albumName := range albums {
			var count int64
			if counts != nil {
				count = counts[albumName]
			}
			d := fs.NewDir(path.Join(dir, f.opt.Enc.ToStandardName(albumName)), f.startTime).SetItems(count)
			entries = append(entries, d)
		}

	case strings.HasPrefix(dirID, "album:"):
		// List photos in an album — dirID is "album:libraryName:albumName"
		parts := strings.SplitN(strings.TrimPrefix(dirID, "album:"), ":", 2)
		if len(parts) != 2 {
			return nil, fs.ErrorDirNotFound
		}
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
			encodedName := f.opt.Enc.ToStandardName(photo.Filename)
			remotePath := encodedName
			if dir != "" {
				remotePath = path.Join(dir, encodedName)
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

	default:
		return nil, fs.ErrorDirNotFound
	}

	return entries, nil
}

// NewObject implements fs.Fs.
func (f *PhotosFs) NewObject(ctx context.Context, remote string) (fs.Object, error) {
	dir, leaf := dircache.SplitPath(remote)
	filename := f.opt.Enc.FromStandardName(leaf)

	dirID, err := f.dirCache.FindDir(ctx, dir, false)
	if err != nil {
		return nil, fs.ErrorObjectNotFound
	}

	if !strings.HasPrefix(dirID, "album:") {
		return nil, fs.ErrorObjectNotFound
	}

	parts := strings.SplitN(strings.TrimPrefix(dirID, "album:"), ":", 2)
	if len(parts) != 2 {
		return nil, fs.ErrorObjectNotFound
	}
	libraryName, albumName := parts[0], parts[1]

	photosService, err := f.icloud.PhotosService(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get photos service: %w", err)
	}

	photo, err := photosService.GetPhoto(ctx, libraryName, albumName, filename)
	if err != nil {
		fs.Debugf(nil, "NewObject(%s): %v", remote, err)
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

// Put implements fs.Fs (read-only).
func (f *PhotosFs) Put(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error) {
	return nil, fs.ErrorNotImplemented
}

// Mkdir implements fs.Fs (read-only).
func (f *PhotosFs) Mkdir(ctx context.Context, dir string) error {
	return fs.ErrorNotImplemented
}

// Rmdir implements fs.Fs (read-only).
func (f *PhotosFs) Rmdir(ctx context.Context, dir string) error {
	return fs.ErrorNotImplemented
}

// FindLeaf finds a directory of name leaf in the folder with ID pathID
func (f *PhotosFs) FindLeaf(ctx context.Context, pathID, leaf string) (pathIDOut string, found bool, err error) {
	// Decode encoded name back to API name
	decodedLeaf := f.opt.Enc.FromStandardName(leaf)

	photosService, err := f.icloud.PhotosService(ctx)
	if err != nil {
		return "", false, fmt.Errorf("failed to get photos service: %w", err)
	}

	if pathID == rootID {
		// Looking for a library at root level
		libraries, err := photosService.GetLibraries(ctx)
		if err != nil {
			return "", false, err
		}

		if _, exists := libraries[decodedLeaf]; exists {
			return "lib:" + decodedLeaf, true, nil
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

		if _, exists := albums[decodedLeaf]; exists {
			albumID := "album:" + libraryName + ":" + decodedLeaf
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

// PhotosObject implements fs.Object.

// Fs implements fs.Object.
func (o *PhotosObject) Fs() fs.Info {
	return o.fs
}

// String implements fs.Object.
func (o *PhotosObject) String() string {
	if o == nil {
		return "<nil>"
	}
	return o.remote
}

// Remote implements fs.Object.
func (o *PhotosObject) Remote() string {
	return o.remote
}

// ModTime implements fs.Object.
func (o *PhotosObject) ModTime(ctx context.Context) time.Time {
	return o.modTime
}

// Size implements fs.Object.
func (o *PhotosObject) Size() int64 {
	return o.size
}

// Storable implements fs.Object.
func (o *PhotosObject) Storable() bool {
	return true
}

// Hash implements fs.Object.
func (o *PhotosObject) Hash(ctx context.Context, ty hash.Type) (string, error) {
	return "", hash.ErrUnsupported
}

// SetModTime implements fs.Object (read-only).
func (o *PhotosObject) SetModTime(ctx context.Context, modTime time.Time) error {
	return fs.ErrorCantSetModTime
}

// Open implements fs.Object.
func (o *PhotosObject) Open(ctx context.Context, options ...fs.OpenOption) (io.ReadCloser, error) {
	if o.downloadURL == "" {
		return nil, errors.New("no download URL available for this object")
	}

	fs.FixRangeOption(options, o.size)

	// Extract range option
	var rangeOption *fs.RangeOption
	for _, option := range options {
		if ro, ok := option.(*fs.RangeOption); ok {
			rangeOption = ro
			break
		}
	}

	var resp *http.Response
	err := o.fs.pacer.Call(func() (bool, error) {
		req, err := http.NewRequestWithContext(ctx, "GET", o.downloadURL, nil)
		if err != nil {
			return false, fmt.Errorf("failed to create request: %w", err)
		}

		if rangeOption != nil {
			key, value := rangeOption.Header()
			req.Header.Set(key, value)
		}

		headers := o.fs.icloud.Session.GetHeaders(nil)
		for key, value := range headers {
			req.Header.Set(key, value)
		}

		resp, err = o.fs.httpClient.Do(req)
		return shouldRetry(ctx, resp, err)
	})
	if err != nil {
		return nil, err
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		_ = resp.Body.Close()
		return nil, fmt.Errorf("download failed with status %d: %s", resp.StatusCode, resp.Status)
	}

	return resp.Body, nil
}

// Update implements fs.Object (read-only).
func (o *PhotosObject) Update(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) error {
	return fs.ErrorNotImplemented
}

// Remove implements fs.Object (read-only).
func (o *PhotosObject) Remove(ctx context.Context) error {
	return fs.ErrorNotImplemented
}

// DirCacheFlush resets the directory cache - used in testing
// as an optional interface.
func (f *PhotosFs) DirCacheFlush() {
	f.dirCache.ResetRoot()
}

// Check interfaces are satisfied
var (
	_ fs.Fs              = &PhotosFs{}
	_ fs.DirCacheFlusher = (*PhotosFs)(nil)
	_ fs.Object          = &PhotosObject{}
)
