//go:build !plan9 && !solaris

package iclouddrive

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"path"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/rclone/rclone/backend/iclouddrive/api"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config/configmap"
	"github.com/rclone/rclone/fs/fshttp"
	"github.com/rclone/rclone/fs/hash"
	"github.com/rclone/rclone/fs/list"
	"github.com/rclone/rclone/lib/dircache"
	"github.com/rclone/rclone/lib/pacer"
)

const rootID = "photos-root"

// PhotosFs represents a remote iCloud Photos server
type PhotosFs struct {
	name       string
	root       string
	opt        Options
	features   *fs.Features
	icloud     *api.Client
	m          configmap.Mapper
	pacer      *fs.Pacer
	dirCache   *dircache.DirCache
	httpClient *http.Client
	startTime  time.Time
	mu         sync.Mutex
	photos     *api.PhotosService
}

// PhotosObject describes an iCloud Photos object
type PhotosObject struct {
	fs          *PhotosFs
	remote      string
	size        int64
	modTime     time.Time
	masterID    string // CloudKit recordName for fresh URL lookup (master or asset depending on resource)
	zone        string // CloudKit zone (e.g. "PrimarySync")
	resourceKey string // CloudKit resource field (resOriginalRes or resOriginalVidComplRes)
	width       int
	height      int
	addedDate   int64
	isFavorite  bool
	isHidden    bool
}

// NewFsPhotos constructs an Fs for Photos from the path, container:path
func NewFsPhotos(ctx context.Context, name, root string, m configmap.Mapper) (fs.Fs, error) {
	icloud, opt, err := newICloudClient(ctx, name, m)
	if err != nil {
		return nil, err
	}

	root = strings.Trim(root, "/")

	f := &PhotosFs{
		name:       name,
		root:       root,
		opt:        *opt,
		icloud:     icloud,
		m:          m,
		pacer:      fs.NewPacer(ctx, pacer.NewDefault(pacer.MinSleep(minSleep), pacer.MaxSleep(maxSleep), pacer.DecayConstant(decayConstant))),
		httpClient: fshttp.NewClient(ctx),
		startTime:  time.Now(),
	}

	f.features = (&fs.Features{
		CanHaveEmptyDirectories: true,
		PartialUploads:          false,
		ReadMimeType:            false,
		ReadMetadata:            true,
	}).Fill(ctx, f)

	f.dirCache = dircache.New(root, rootID, f)

	err = f.dirCache.FindRoot(ctx, false)
	if err != nil {
		// Check if root points to a file (e.g. PrimarySync/Videos/file.mp4)
		newRoot, remote := dircache.SplitPath(root)
		tempF := &PhotosFs{
			name:       f.name,
			root:       newRoot,
			opt:        f.opt,
			features:   f.features,
			icloud:     f.icloud,
			pacer:      f.pacer,
			httpClient: f.httpClient,
			startTime:  f.startTime,
			photos:     f.photos, // nil here; photosService() creates lazily on first use
		}
		tempF.dirCache = dircache.New(newRoot, rootID, tempF)
		if err2 := tempF.dirCache.FindRoot(ctx, false); err2 != nil {
			return f, nil
		}
		_, err2 := tempF.NewObject(ctx, remote)
		if err2 != nil {
			return f, nil
		}
		// Root is a file - adjust f to parent dir, signal ErrorIsFile
		f.root = newRoot
		f.dirCache = tempF.dirCache
		return f, fs.ErrorIsFile
	}

	return f, nil
}

// photosService returns the PhotosService, creating it on first call
func (f *PhotosFs) photosService(ctx context.Context) (*api.PhotosService, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.photos == nil {
		var err error
		f.photos, err = api.NewPhotosService(ctx, f.icloud, f.pacer, shouldRetry)
		if err != nil {
			return nil, err
		}
	}
	return f.photos, nil
}

// Name of the remote (as passed into NewFs)
func (f *PhotosFs) Name() string {
	return f.name
}

// Root of the remote (as passed into NewFs)
func (f *PhotosFs) Root() string {
	return f.opt.Enc.ToStandardPath(f.root)
}

// String converts this Fs to a string
func (f *PhotosFs) String() string {
	return fmt.Sprintf("iCloud Photos root '%s'", f.root)
}

// Precision of the object storage system
func (f *PhotosFs) Precision() time.Duration {
	return time.Second
}

// Hashes returns the supported hash sets
func (f *PhotosFs) Hashes() hash.Set {
	return hash.Set(hash.None)
}

// Features returns the optional features of this Fs
func (f *PhotosFs) Features() *fs.Features {
	return f.features
}

// List the objects and directories in dir into entries
func (f *PhotosFs) List(ctx context.Context, dir string) (entries fs.DirEntries, err error) {
	photosService, err := f.photosService(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get photos service: %w", err)
	}

	dirID, err := f.dirCache.FindDir(ctx, dir, false)
	if err != nil {
		return nil, err
	}

	switch {
	case dirID == rootID:
		// List libraries
		libraries, err := photosService.GetLibraries(ctx)
		if err != nil {
			return nil, err
		}

		albumCounts, err := photosService.GetLibraryAlbumCounts(ctx)
		if err != nil {
			fs.Debugf(f, "Failed to get library album counts: %v", err)
		}

		for libraryName := range libraries {
			count := int64(-1)
			if albumCounts != nil {
				if c, ok := albumCounts[libraryName]; ok {
					count = c
				}
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
			fs.Debugf(f, "Failed to get album counts for library %q: %v", libraryName, err)
		}

		for albumName := range albums {
			count := int64(-1)
			if counts != nil {
				if c, ok := counts[albumName]; ok {
					count = c
				}
			}
			d := fs.NewDir(path.Join(dir, f.opt.Enc.ToStandardName(albumName)), f.startTime).SetItems(count)
			entries = append(entries, d)
		}

	case strings.HasPrefix(dirID, "album:"):
		// List contents of an album or folder
		libraryName, albumPath, ok := parseAlbumDirID(dirID)
		if !ok {
			return nil, fs.ErrorDirNotFound
		}

		album, err := f.resolveAlbum(ctx, photosService, libraryName, albumPath)
		if err != nil {
			return nil, err
		}

		// Folders list their child albums as subdirectories
		if album.IsFolder {
			for childName, child := range album.Children {
				d := fs.NewDir(path.Join(dir, f.opt.Enc.ToStandardName(childName)), f.startTime)
				if child.IsFolder {
					d.SetItems(int64(len(child.Children)))
				}
				entries = append(entries, d)
			}
			return entries, nil
		}

		photos, err := album.GetPhotos(ctx)
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
			o := f.newPhotosObject(remotePath, photo, libraryName)
			entries = append(entries, o)
		}

	default:
		return nil, fs.ErrorDirNotFound
	}

	return entries, nil
}

// NewObject finds the Object at remote.  If it can't be found
// it returns the error fs.ErrorObjectNotFound
func (f *PhotosFs) NewObject(ctx context.Context, remote string) (fs.Object, error) {
	dir, leaf := dircache.SplitPath(remote)
	filename := f.opt.Enc.FromStandardName(leaf)

	dirID, err := f.dirCache.FindDir(ctx, dir, false)
	if err != nil {
		fs.Debugf(f, "NewObject(%s): FindDir: %v", remote, err)
		return nil, fs.ErrorObjectNotFound
	}

	if !strings.HasPrefix(dirID, "album:") {
		return nil, fs.ErrorObjectNotFound
	}

	libraryName, albumPath, ok := parseAlbumDirID(dirID)
	if !ok {
		return nil, fs.ErrorObjectNotFound
	}

	photosService, err := f.photosService(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get photos service: %w", err)
	}

	album, err := f.resolveAlbum(ctx, photosService, libraryName, albumPath)
	if err != nil || album.IsFolder {
		fs.Debugf(f, "NewObject(%s): album resolve failed or is folder", remote)
		return nil, fs.ErrorObjectNotFound
	}

	photo, err := album.GetPhotoByName(ctx, filename)
	if err != nil {
		fs.Debugf(f, "NewObject(%s): %v", remote, err)
		return nil, fs.ErrorObjectNotFound
	}

	return f.newPhotosObject(remote, photo, libraryName), nil
}

// newPhotosObject creates a PhotosObject from a Photo and zone
func (f *PhotosFs) newPhotosObject(remote string, photo *api.Photo, zone string) *PhotosObject {
	return &PhotosObject{
		fs:          f,
		remote:      remote,
		size:        photo.Size,
		modTime:     time.UnixMilli(photo.AssetDate),
		masterID:    photo.ID,
		zone:        zone,
		resourceKey: photo.ResourceKey,
		width:       photo.Width,
		height:      photo.Height,
		addedDate:   photo.AddedDate,
		isFavorite:  photo.IsFavorite,
		isHidden:    photo.IsHidden,
	}
}

// Put is not supported for this read-only backend
func (f *PhotosFs) Put(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error) {
	return nil, fs.ErrorNotImplemented
}

// Mkdir is not supported for this read-only backend
func (f *PhotosFs) Mkdir(ctx context.Context, dir string) error {
	return fs.ErrorNotImplemented
}

// Rmdir is not supported for this read-only backend
func (f *PhotosFs) Rmdir(ctx context.Context, dir string) error {
	return fs.ErrorNotImplemented
}

// FindLeaf finds a directory of name leaf in the folder with ID pathID
func (f *PhotosFs) FindLeaf(ctx context.Context, pathID, leaf string) (pathIDOut string, found bool, err error) {
	decodedLeaf := f.opt.Enc.FromStandardName(leaf)

	photosService, err := f.photosService(ctx)
	if err != nil {
		return "", false, fmt.Errorf("failed to get photos service: %w", err)
	}

	if pathID == rootID {
		libraries, err := photosService.GetLibraries(ctx)
		if err != nil {
			return "", false, err
		}

		if _, exists := libraries[decodedLeaf]; exists {
			return "lib:" + decodedLeaf, true, nil
		}
		return "", false, nil
	}

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

	if strings.HasPrefix(pathID, "album:") {
		// Check if this is a folder containing child albums
		libraryName, albumPath, ok := parseAlbumDirID(pathID)
		if !ok {
			return "", false, nil
		}

		album, err := f.resolveAlbum(ctx, photosService, libraryName, albumPath)
		if err != nil {
			return "", false, nil
		}

		if album.IsFolder {
			if _, exists := album.Children[decodedLeaf]; exists {
				childID := "album:" + libraryName + ":" + albumPath + "/" + decodedLeaf
				return childID, true, nil
			}
		}
		return "", false, nil
	}

	return "", false, nil
}

// CreateDir makes a directory with pathID as parent and name leaf
func (f *PhotosFs) CreateDir(ctx context.Context, pathID, leaf string) (newID string, err error) {
	return "", fs.ErrorNotImplemented
}

// Fs returns the parent Fs
func (o *PhotosObject) Fs() fs.Info {
	return o.fs
}

// Return a string version
func (o *PhotosObject) String() string {
	if o == nil {
		return "<nil>"
	}
	return o.remote
}

// Remote returns the remote path
func (o *PhotosObject) Remote() string {
	return o.remote
}

// ModTime returns the modification time of the object
func (o *PhotosObject) ModTime(ctx context.Context) time.Time {
	return o.modTime
}

// Size returns the size of an object in bytes
func (o *PhotosObject) Size() int64 {
	return o.size
}

// Storable returns a boolean as to whether this object is storable
func (o *PhotosObject) Storable() bool {
	return true
}

// Hash returns the hash of an object returning a lowercase hex string
func (o *PhotosObject) Hash(ctx context.Context, ty hash.Type) (string, error) {
	return "", hash.ErrUnsupported
}

// SetModTime sets the modification time of the object
func (o *PhotosObject) SetModTime(ctx context.Context, modTime time.Time) error {
	return fs.ErrorCantSetModTime
}

// Metadata returns metadata for the photo object
func (o *PhotosObject) Metadata(ctx context.Context) (fs.Metadata, error) {
	metadata := make(fs.Metadata, 5)
	if o.width > 0 {
		metadata["width"] = strconv.Itoa(o.width)
	}
	if o.height > 0 {
		metadata["height"] = strconv.Itoa(o.height)
	}
	if o.addedDate > 0 {
		metadata["added-time"] = time.UnixMilli(o.addedDate).UTC().Format(time.RFC3339)
	}
	metadata["favorite"] = strconv.FormatBool(o.isFavorite)
	metadata["hidden"] = strconv.FormatBool(o.isHidden)
	return metadata, nil
}

// Open an object for read, fetching a fresh download URL via records/lookup
func (o *PhotosObject) Open(ctx context.Context, options ...fs.OpenOption) (io.ReadCloser, error) {
	photosService, err := o.fs.photosService(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get photos service: %w", err)
	}

	downloadURL, err := photosService.LookupDownloadURL(ctx, o.masterID, o.zone, o.resourceKey)
	if err != nil {
		return nil, fmt.Errorf("failed to get download URL for %q: %w", o.remote, err)
	}

	fs.FixRangeOption(options, o.size)

	var resp *http.Response
	err = o.fs.pacer.Call(func() (bool, error) {
		if resp != nil {
			_ = resp.Body.Close()
			resp = nil
		}
		req, err := http.NewRequestWithContext(ctx, "GET", downloadURL, nil)
		if err != nil {
			return false, fmt.Errorf("failed to create request: %w", err)
		}
		fs.OpenOptionAddHTTPHeaders(req.Header, options)

		resp, err = o.fs.httpClient.Do(req)
		return shouldRetry(ctx, resp, err)
	})
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusPartialContent {
		_ = resp.Body.Close()
		return nil, fmt.Errorf("download %q failed: %s", o.remote, resp.Status)
	}

	return resp.Body, nil
}

// Update is not supported for this read-only backend
func (o *PhotosObject) Update(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) error {
	return fs.ErrorNotImplemented
}

// Remove is not supported for this read-only backend
func (o *PhotosObject) Remove(ctx context.Context) error {
	return fs.ErrorNotImplemented
}

// resolveAlbum finds an album by path, traversing folder hierarchy
// albumPath can be "AlbumName" or "FolderName/AlbumName" for nested albums
func (f *PhotosFs) resolveAlbum(ctx context.Context, ps *api.PhotosService, libraryName, albumPath string) (*api.Album, error) {
	libraries, err := ps.GetLibraries(ctx)
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

	return resolveAlbumPath(albums, albumPath)
}

// resolveAlbumPath finds an album inside an album tree by slash-separated path
func resolveAlbumPath(albums map[string]*api.Album, albumPath string) (*api.Album, error) {
	parts := strings.Split(albumPath, "/")
	current := albums
	for i, part := range parts {
		album, exists := current[part]
		if !exists {
			return nil, fs.ErrorDirNotFound
		}
		if i == len(parts)-1 {
			return album, nil
		}
		if !album.IsFolder || album.Children == nil {
			return nil, fs.ErrorDirNotFound
		}
		current = album.Children
	}
	return nil, fs.ErrorDirNotFound
}

// parseAlbumDirID extracts library and album names from "album:lib:album" dirID
func parseAlbumDirID(dirID string) (libraryName, albumName string, ok bool) {
	parts := strings.SplitN(strings.TrimPrefix(dirID, "album:"), ":", 2)
	if len(parts) != 2 {
		return "", "", false
	}
	return parts[0], parts[1], true
}

// DirCacheFlush resets the directory cache - used in testing as an
// optional interface
func (f *PhotosFs) DirCacheFlush() {
	f.dirCache.ResetRoot()
	// Also flush API-layer caches so albums/photos are re-fetched
	// DirCacheFlusher interface has no ctx param; use a bounded timeout
	// to avoid hanging indefinitely if photosService triggers first init
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if ps, err := f.photosService(ctx); err == nil {
		ps.FlushCaches()
	}
}

// listRAlbumJob represents an album to be listed by a ListR worker
type listRAlbumJob struct {
	album   *api.Album
	zone    string
	dirPath string // path relative to Fs root (e.g. "Videos" when f.root="PrimarySync")
}

// ListR lists the objects and directories of the Fs starting from dir
// recursively into out, calling callback for each batch of entries
// Albums are listed in parallel using a goroutine pool
func (f *PhotosFs) ListR(ctx context.Context, dir string, callback fs.ListRCallback) error {
	ps, err := f.photosService(ctx)
	if err != nil {
		return err
	}

	helper := list.NewHelper(callback)
	var mu sync.Mutex // protects helper (not thread-safe)

	addEntry := func(entry fs.DirEntry) error {
		mu.Lock()
		defer mu.Unlock()
		return helper.Add(entry)
	}

	// Resolve dir through dirCache - this respects f.root
	dirID, err := f.dirCache.FindDir(ctx, dir, false)
	if err != nil {
		return err
	}

	// Collect album jobs, scoped to the resolved directory
	var jobs []listRAlbumJob

	// collectAlbumJobs emits directory entries for albums/folders under basePath
	// and collects leaf albums as jobs for parallel photo listing
	// Recurses into folders to handle arbitrary nesting depth
	var collectAlbumJobs func(albums map[string]*api.Album, zone, basePath string) error
	collectAlbumJobs = func(albums map[string]*api.Album, zone, basePath string) error {
		for albumName, album := range albums {
			albumPath := path.Join(basePath, f.opt.Enc.ToStandardName(albumName))
			if err := addEntry(fs.NewDir(albumPath, f.startTime)); err != nil {
				return err
			}
			if album.IsFolder {
				if err := collectAlbumJobs(album.Children, zone, albumPath); err != nil {
					return err
				}
			} else {
				jobs = append(jobs, listRAlbumJob{album: album, zone: zone, dirPath: albumPath})
			}
		}
		return nil
	}

	switch {
	case dirID == rootID:
		// Root level - emit libraries, then all albums
		libraries, err := ps.GetLibraries(ctx)
		if err != nil {
			return err
		}
		for libName, lib := range libraries {
			libPath := path.Join(dir, f.opt.Enc.ToStandardName(libName))
			if err := addEntry(fs.NewDir(libPath, f.startTime)); err != nil {
				return err
			}
			albums, err := lib.GetAlbums(ctx)
			if err != nil {
				return err
			}
			if err := collectAlbumJobs(albums, libName, libPath); err != nil {
				return err
			}
		}

	case strings.HasPrefix(dirID, "lib:"):
		// Library level - emit and collect all albums
		libraryName := strings.TrimPrefix(dirID, "lib:")
		libraries, err := ps.GetLibraries(ctx)
		if err != nil {
			return err
		}
		library, exists := libraries[libraryName]
		if !exists {
			return fs.ErrorDirNotFound
		}
		albums, err := library.GetAlbums(ctx)
		if err != nil {
			return err
		}
		if err := collectAlbumJobs(albums, libraryName, dir); err != nil {
			return err
		}

	case strings.HasPrefix(dirID, "album:"):
		// Album or folder level
		libraryName, albumPath, ok := parseAlbumDirID(dirID)
		if !ok {
			return fs.ErrorDirNotFound
		}
		album, err := f.resolveAlbum(ctx, ps, libraryName, albumPath)
		if err != nil {
			return err
		}
		if album.IsFolder {
			if err := collectAlbumJobs(album.Children, libraryName, dir); err != nil {
				return err
			}
		} else {
			// Single album - just list its photos
			jobs = append(jobs, listRAlbumJob{album: album, zone: libraryName, dirPath: dir})
		}

	default:
		return fs.ErrorDirNotFound
	}

	if len(jobs) == 0 {
		return helper.Flush()
	}

	// Fan out album photo listing across goroutines
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	jobCh := make(chan listRAlbumJob, len(jobs))
	for _, job := range jobs {
		jobCh <- job
	}
	close(jobCh)

	workers := fs.GetConfig(ctx).Checkers
	if len(jobs) < workers {
		workers = len(jobs)
	}
	errs := make(chan error, workers)

	for range workers {
		go func() {
			for job := range jobCh {
				photos, err := job.album.GetPhotos(ctx)
				if err != nil {
					errs <- err
					cancel()
					return
				}
				for _, photo := range photos {
					if photo.Filename == "" {
						continue
					}
					remotePath := path.Join(job.dirPath, f.opt.Enc.ToStandardName(photo.Filename))
					o := f.newPhotosObject(remotePath, photo, job.zone)
					if err := addEntry(o); err != nil {
						errs <- err
						cancel()
						return
					}
				}
			}
			errs <- nil
		}()
	}

	for range workers {
		if e := <-errs; e != nil && err == nil {
			err = e
		}
	}
	if err != nil {
		return err
	}

	return helper.Flush()
}

func notifyAlbumTree(albums map[string]*api.Album, base string, notifyFunc func(string, fs.EntryType)) {
	for name, album := range albums {
		albumPath := name
		if base != "" {
			albumPath = path.Join(base, name)
		}
		notifyFunc(albumPath, fs.EntryDirectory)
		if album.IsFolder {
			notifyAlbumTree(album.Children, albumPath, notifyFunc)
		}
	}
}

func (f *PhotosFs) notifyZoneChange(ctx context.Context, ps *api.PhotosService, zone string, notifyFunc func(string, fs.EntryType)) {
	if f.root == "" {
		notifyFunc(zone, fs.EntryDirectory)
		return
	}

	libraryName, albumPath, _ := strings.Cut(f.root, "/")
	if libraryName != zone {
		return
	}

	// Always invalidate the mounted root when its backing zone changes
	notifyFunc("", fs.EntryDirectory)

	libraries, err := ps.GetLibraries(ctx)
	if err != nil {
		return
	}
	library, ok := libraries[zone]
	if !ok {
		return
	}
	albums, err := library.GetAlbums(ctx)
	if err != nil {
		return
	}
	if albumPath == "" {
		notifyAlbumTree(albums, "", notifyFunc)
		return
	}

	album, err := resolveAlbumPath(albums, albumPath)
	if err != nil {
		return
	}
	if album.IsFolder {
		notifyAlbumTree(album.Children, "", notifyFunc)
	}
}

// ChangeNotify polls for changes and notifies the VFS when directories are modified
func (f *PhotosFs) ChangeNotify(ctx context.Context, notifyFunc func(string, fs.EntryType), pollIntervalChan <-chan time.Duration) {
	go func() {
		var ticker *time.Ticker
		var tickerC <-chan time.Time
		for {
			select {
			case pollInterval, ok := <-pollIntervalChan:
				if !ok {
					if ticker != nil {
						ticker.Stop()
					}
					return
				}
				if ticker != nil {
					ticker.Stop()
					tickerC = nil
				}
				if pollInterval > 0 {
					ticker = time.NewTicker(pollInterval)
					tickerC = ticker.C
				}
			case <-tickerC:
				ps, err := f.photosService(ctx)
				if err != nil {
					fs.Debugf(f, "ChangeNotify: failed to get photos service: %v", err)
					continue
				}
				changedZones := ps.PollForChanges(ctx)
				for _, zone := range changedZones {
					f.notifyZoneChange(ctx, ps, zone, notifyFunc)
				}
			case <-ctx.Done():
				if ticker != nil {
					ticker.Stop()
				}
				return
			}
		}
	}()
}

// Disconnect clears authentication state and removes disk caches
func (f *PhotosFs) Disconnect(ctx context.Context) error {
	return disconnectClient(f.m, f.icloud)
}

// Check interfaces are satisfied
var (
	_ fs.Fs              = (*PhotosFs)(nil)
	_ fs.Disconnecter    = (*PhotosFs)(nil)
	_ fs.ListRer         = (*PhotosFs)(nil)
	_ fs.ChangeNotifier  = (*PhotosFs)(nil)
	_ fs.DirCacheFlusher = (*PhotosFs)(nil)
	_ fs.Object          = (*PhotosObject)(nil)
	_ fs.Metadataer      = (*PhotosObject)(nil)
)
