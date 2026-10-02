// Package darkibox provides an interface to the Darkibox video hosting service.
//
// Darkibox is an XFileSharing-based video hosting platform. This backend uses
// the REST API documented at https://darkibox.com/api.html
package darkibox

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"path"
	"strings"
	"time"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config/configmap"
	"github.com/rclone/rclone/fs/config/configstruct"
	"github.com/rclone/rclone/fs/fshttp"
	"github.com/rclone/rclone/fs/hash"
	"github.com/rclone/rclone/lib/pacer"
	"github.com/rclone/rclone/lib/rest"
)

const (
	minSleep      = pacer.MinSleep(100 * time.Millisecond)
	maxSleep      = pacer.MaxSleep(2 * time.Second)
	decayConstant = pacer.DecayConstant(2)
)

// Register with Fs
func init() {
	fs.Register(&fs.RegInfo{
		Name:        "darkibox",
		Description: "Darkibox",
		NewFs:       NewFs,
		Options: []fs.Option{{
			Name:      "api_key",
			Help:      "API key for your darkibox account.\n\nFound on https://darkibox.com/api.html when logged in.",
			Sensitive: true,
			Required:  true,
		}, {
			Name:     "api_url",
			Help:     "API endpoint URL.\n\nNormally this doesn't need to be changed.",
			Default:  "https://darkibox.com/api",
			Advanced: true,
		}},
	})
}

// Options defines the configuration for this backend.
type Options struct {
	APIKey string `config:"api_key"`
	APIURL string `config:"api_url"`
}

// Fs represents a remote darkibox filesystem.
type Fs struct {
	name     string
	root     string
	opt      Options
	features *fs.Features
	srv      *rest.Client
	pacer    *fs.Pacer

	// dirCache maps path -> folderID for resolved directories
	dirCache map[string]string
}

// Object describes a darkibox file.
type Object struct {
	fs       *Fs
	remote   string // path relative to Fs root
	fileCode string
	fileName string
	fileSize int64
	modTime  time.Time
	folderID string
}

// NewFs constructs an Fs from the path, container:path
func NewFs(ctx context.Context, name, root string, m configmap.Mapper) (fs.Fs, error) {
	opt := new(Options)
	err := configstruct.Set(m, opt)
	if err != nil {
		return nil, err
	}

	if opt.APIKey == "" {
		return nil, errors.New("darkibox: api_key is required")
	}

	f := &Fs{
		name:     name,
		root:     root,
		opt:      *opt,
		srv:      rest.NewClient(fshttp.NewClient(ctx)),
		pacer:    fs.NewPacer(ctx, pacer.NewDefault(minSleep, maxSleep, decayConstant)),
		dirCache: make(map[string]string),
	}

	f.srv.SetRoot(opt.APIURL)

	f.features = (&fs.Features{
		CanHaveEmptyDirectories: true,
	}).Fill(ctx, f)

	// Verify the API key is valid
	_, err = f.getAccountInfo(ctx)
	if err != nil {
		return nil, fmt.Errorf("darkibox: failed to verify API key: %w", err)
	}

	// Check if root is a file
	if root != "" {
		// Try to resolve the root path
		_, err := f.resolveDir(ctx, root)
		if err != nil {
			// Root might be a file path like "folder/file.mp4"
			dir := path.Dir(root)
			base := path.Base(root)
			if dir == "." {
				dir = ""
			}
			dirID, dirErr := f.resolveDir(ctx, dir)
			if dirErr == nil {
				// Check if base is a file in this directory
				fileResp, fileErr := f.getFileList(ctx, dirID, 1)
				if fileErr == nil {
					for _, file := range fileResp.Result.Files {
						name := file.GetFileName()
						if name == base || file.FileCode == base {
							// Root points to a file, adjust root to parent
							f.root = dir
							return f, fs.ErrorIsFile
						}
					}
				}
			}
		}
	}

	return f, nil
}

// resolveDir resolves a path like "folder1/folder2" to a folder ID.
// The root folder has ID "0".
func (f *Fs) resolveDir(ctx context.Context, dir string) (string, error) {
	if dir == "" || dir == "." {
		return "0", nil
	}

	// Check cache
	if id, ok := f.dirCache[dir]; ok {
		return id, nil
	}

	// Walk the path from root
	parts := strings.Split(dir, "/")
	currentID := "0"

	for i, part := range parts {
		if part == "" {
			continue
		}

		resp, err := f.getFolderList(ctx, currentID)
		if err != nil {
			return "", fmt.Errorf("failed to list folder %s: %w", currentID, err)
		}

		found := false
		for _, folder := range resp.Result.Folders {
			if folder.Name == part {
				currentID = folder.FolderID.String()
				// Cache intermediate results
				partialPath := strings.Join(parts[:i+1], "/")
				f.dirCache[partialPath] = currentID
				found = true
				break
			}
		}

		if !found {
			return "", fs.ErrorDirNotFound
		}
	}

	f.dirCache[dir] = currentID
	return currentID, nil
}

// mkdirAll creates all directories in a path, returning the final folder ID.
func (f *Fs) mkdirAll(ctx context.Context, dir string) (string, error) {
	if dir == "" || dir == "." {
		return "0", nil
	}

	// Check if already exists
	if id, err := f.resolveDir(ctx, dir); err == nil {
		return id, nil
	}

	parts := strings.Split(dir, "/")
	currentID := "0"

	for i, part := range parts {
		if part == "" {
			continue
		}

		partialPath := strings.Join(parts[:i+1], "/")

		// Check cache first
		if id, ok := f.dirCache[partialPath]; ok {
			currentID = id
			continue
		}

		// Check if folder exists at current level
		resp, err := f.getFolderList(ctx, currentID)
		if err != nil {
			return "", err
		}

		found := false
		for _, folder := range resp.Result.Folders {
			if folder.Name == part {
				currentID = folder.FolderID.String()
				f.dirCache[partialPath] = currentID
				found = true
				break
			}
		}

		if !found {
			// Create the folder
			createResp, err := f.createFolder(ctx, part, currentID)
			if err != nil {
				return "", err
			}
			currentID = createResp.Result.FolderID.String()
			f.dirCache[partialPath] = currentID
		}
	}

	return currentID, nil
}

// invalidateCache removes a directory path and all its children from the cache.
func (f *Fs) invalidateCache(dir string) {
	for k := range f.dirCache {
		if k == dir || strings.HasPrefix(k, dir+"/") {
			delete(f.dirCache, k)
		}
	}
}

// fileInfoToObject converts a FileInfo to an Object with the given directory prefix.
func (f *Fs) fileInfoToObject(info FileInfo, dir string) *Object {
	name := info.GetFileName()

	remote := name
	if dir != "" {
		remote = dir + "/" + name
	}

	return &Object{
		fs:       f,
		remote:   remote,
		fileCode: info.FileCode,
		fileName: name,
		fileSize: info.GetFileSize(),
		modTime:  parseTime(info.GetCreatedTime()),
		folderID: info.FolderID.String(),
	}
}

// --- fs.Fs interface ---

// Name of the remote (as passed into NewFs)
func (f *Fs) Name() string { return f.name }

// Root of the remote (as passed into NewFs)
func (f *Fs) Root() string { return f.root }

// String converts this Fs to a string
func (f *Fs) String() string { return fmt.Sprintf("darkibox root '%s'", f.root) }

// Precision return the precision of this Fs
func (f *Fs) Precision() time.Duration { return time.Second }

// Hashes returns the supported hash sets.
func (f *Fs) Hashes() hash.Set { return hash.Set(hash.None) }

// Features returns the optional features of this Fs
func (f *Fs) Features() *fs.Features { return f.features }

// List the objects and directories in dir into entries.
func (f *Fs) List(ctx context.Context, dir string) (entries fs.DirEntries, err error) {
	fullDir := dir
	if f.root != "" {
		if dir != "" {
			fullDir = f.root + "/" + dir
		} else {
			fullDir = f.root
		}
	}

	folderID, err := f.resolveDir(ctx, fullDir)
	if err != nil {
		return nil, fs.ErrorDirNotFound
	}

	// Get subfolders
	folderResp, err := f.getFolderList(ctx, folderID)
	if err != nil {
		return nil, err
	}

	for _, folder := range folderResp.Result.Folders {
		remote := folder.Name
		if dir != "" {
			remote = dir + "/" + folder.Name
		}
		d := fs.NewDir(remote, time.Time{})
		d.SetID(folder.FolderID.String())
		entries = append(entries, d)
	}

	// Get files with pagination
	page := 1
	for {
		fileResp, err := f.getFileList(ctx, folderID, page)
		if err != nil {
			return nil, err
		}

		for _, file := range fileResp.Result.Files {
			o := f.fileInfoToObject(file, dir)
			entries = append(entries, o)
		}

		if page >= fileResp.Result.Pages || len(fileResp.Result.Files) == 0 {
			break
		}
		page++
	}

	return entries, nil
}

// NewObject finds the Object at remote. If it can't be found it returns
// the error fs.ErrorObjectNotFound.
func (f *Fs) NewObject(ctx context.Context, remote string) (fs.Object, error) {
	dir := path.Dir(remote)
	base := path.Base(remote)
	if dir == "." {
		dir = ""
	}

	fullDir := dir
	if f.root != "" {
		if dir != "" {
			fullDir = f.root + "/" + dir
		} else {
			fullDir = f.root
		}
	}

	folderID, err := f.resolveDir(ctx, fullDir)
	if err != nil {
		return nil, fs.ErrorObjectNotFound
	}

	// Search files in this folder
	page := 1
	for {
		fileResp, err := f.getFileList(ctx, folderID, page)
		if err != nil {
			return nil, fs.ErrorObjectNotFound
		}

		for _, file := range fileResp.Result.Files {
			name := file.GetFileName()
			if name == base || file.FileCode == base {
				return f.fileInfoToObject(file, dir), nil
			}
		}

		if page >= fileResp.Result.Pages || len(fileResp.Result.Files) == 0 {
			break
		}
		page++
	}

	return nil, fs.ErrorObjectNotFound
}

// Put uploads a file to the remote path.
func (f *Fs) Put(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error) {
	remote := src.Remote()
	dir := path.Dir(remote)
	base := path.Base(remote)
	if dir == "." {
		dir = ""
	}

	fullDir := dir
	if f.root != "" {
		if dir != "" {
			fullDir = f.root + "/" + dir
		} else {
			fullDir = f.root
		}
	}

	// Ensure the target directory exists
	folderID, err := f.mkdirAll(ctx, fullDir)
	if err != nil {
		return nil, fmt.Errorf("failed to create directory %s: %w", fullDir, err)
	}

	// Get upload server
	uploadURL, err := f.getUploadServer(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get upload server: %w", err)
	}

	// Upload the file
	uploadResp, err := f.uploadFile(ctx, uploadURL, folderID, base, in)
	if err != nil {
		return nil, fmt.Errorf("failed to upload file: %w", err)
	}

	if len(uploadResp.Files) == 0 {
		return nil, errors.New("upload returned no files")
	}

	uploadedFile := uploadResp.Files[0]
	if uploadedFile.Status != "OK" {
		return nil, fmt.Errorf("upload failed: %s", uploadedFile.Status)
	}

	if uploadedFile.FileCode == "" {
		return nil, errors.New("upload returned empty file code")
	}

	// Get the file info for the uploaded file
	infoResp, err := f.getFileInfo(ctx, uploadedFile.FileCode)
	if err != nil {
		// Return a basic object even if we can't get full info
		return &Object{
			fs:       f,
			remote:   remote,
			fileCode: uploadedFile.FileCode,
			fileName: base,
			fileSize: src.Size(),
			modTime:  src.ModTime(ctx),
			folderID: folderID,
		}, nil
	}

	if len(infoResp.Result) > 0 {
		return f.fileInfoToObject(infoResp.Result[0], dir), nil
	}

	return &Object{
		fs:       f,
		remote:   remote,
		fileCode: uploadedFile.FileCode,
		fileName: base,
		fileSize: src.Size(),
		modTime:  src.ModTime(ctx),
		folderID: folderID,
	}, nil
}

// Mkdir creates the directory if it doesn't exist.
func (f *Fs) Mkdir(ctx context.Context, dir string) error {
	fullDir := dir
	if f.root != "" {
		if dir != "" {
			fullDir = f.root + "/" + dir
		} else {
			fullDir = f.root
		}
	}

	_, err := f.mkdirAll(ctx, fullDir)
	return err
}

// Rmdir deletes the directory. It should be empty.
func (f *Fs) Rmdir(ctx context.Context, dir string) error {
	fullDir := dir
	if f.root != "" {
		if dir != "" {
			fullDir = f.root + "/" + dir
		} else {
			fullDir = f.root
		}
	}

	folderID, err := f.resolveDir(ctx, fullDir)
	if err != nil {
		return fs.ErrorDirNotFound
	}

	if folderID == "0" {
		return errors.New("cannot remove root directory")
	}

	err = f.deleteFolder(ctx, folderID)
	if err != nil {
		return err
	}

	f.invalidateCache(fullDir)
	return nil
}

// Move src to this remote using server-side move operations.
func (f *Fs) Move(ctx context.Context, src fs.Object, remote string) (fs.Object, error) {
	srcObj, ok := src.(*Object)
	if !ok {
		return nil, fs.ErrorCantMove
	}

	dstDir := path.Dir(remote)
	dstBase := path.Base(remote)
	if dstDir == "." {
		dstDir = ""
	}

	fullDir := dstDir
	if f.root != "" {
		if dstDir != "" {
			fullDir = f.root + "/" + dstDir
		} else {
			fullDir = f.root
		}
	}

	// Ensure target directory exists
	dstFolderID, err := f.mkdirAll(ctx, fullDir)
	if err != nil {
		return nil, err
	}

	// Move the file to the new folder if needed
	if srcObj.folderID != dstFolderID {
		err = f.moveFile(ctx, srcObj.fileCode, dstFolderID)
		if err != nil {
			return nil, err
		}
	}

	// Rename if needed
	srcBase := path.Base(srcObj.remote)
	if srcBase != dstBase {
		newTitle := dstBase
		if idx := strings.LastIndex(newTitle, "."); idx > 0 {
			newTitle = newTitle[:idx]
		}
		err = f.renameFile(ctx, srcObj.fileCode, newTitle)
		if err != nil {
			return nil, err
		}
	}

	// Return updated object
	return &Object{
		fs:       f,
		remote:   remote,
		fileCode: srcObj.fileCode,
		fileName: dstBase,
		fileSize: srcObj.fileSize,
		modTime:  srcObj.modTime,
		folderID: dstFolderID,
	}, nil
}

// DirMove moves src, srcRemote to this remote at dstRemote using server-side
// move operations. This is not supported by the darkibox API.
func (f *Fs) DirMove(ctx context.Context, src fs.Fs, srcRemote, dstRemote string) error {
	// Darkibox API doesn't support moving folders directly
	return fs.ErrorCantDirMove
}

// About gets quota information.
func (f *Fs) About(ctx context.Context) (*fs.Usage, error) {
	info, err := f.getAccountInfo(ctx)
	if err != nil {
		return nil, err
	}

	usage := &fs.Usage{
		Used: fs.NewUsageValue(info.Result.StorageUsed),
	}

	return usage, nil
}

// --- fs.Object interface ---

// Fs returns the parent Fs
func (o *Object) Fs() fs.Info { return o.fs }

// Remote returns the remote path
func (o *Object) Remote() string { return o.remote }

// String returns a description of the Object
func (o *Object) String() string {
	if o == nil {
		return "<nil>"
	}
	return o.remote
}

// ModTime returns the modification time of the object
func (o *Object) ModTime(ctx context.Context) time.Time { return o.modTime }

// Size returns the size of an object in bytes
func (o *Object) Size() int64 { return o.fileSize }

// Hash returns the hash of an object (not supported by darkibox)
func (o *Object) Hash(ctx context.Context, t hash.Type) (string, error) {
	return "", hash.ErrUnsupported
}

// Storable returns whether this object is storable
func (o *Object) Storable() bool { return true }

// SetModTime sets the modification time (not supported by darkibox API)
func (o *Object) SetModTime(ctx context.Context, modTime time.Time) error {
	return fs.ErrorCantSetModTime
}

// Open an object for read
func (o *Object) Open(ctx context.Context, options ...fs.OpenOption) (io.ReadCloser, error) {
	dlResp, err := o.fs.getDirectLink(ctx, o.fileCode)
	if err != nil {
		return nil, fmt.Errorf("failed to get direct link: %w", err)
	}

	// Pick the best quality (original first)
	var downloadURL string
	preferredQualities := []string{"o", "h", "n", "l"}
	for _, q := range preferredQualities {
		for _, v := range dlResp.Result.Versions {
			if v.Name == q && v.URL != "" {
				downloadURL = v.URL
				break
			}
		}
		if downloadURL != "" {
			break
		}
	}

	// Fallback to first available version
	if downloadURL == "" && len(dlResp.Result.Versions) > 0 {
		downloadURL = dlResp.Result.Versions[0].URL
	}

	if downloadURL == "" {
		return nil, errors.New("no download URL available")
	}

	var resp *http.Response
	err = o.fs.pacer.Call(func() (bool, error) {
		resp, err = o.fs.srv.Call(ctx, &rest.Opts{
			Method:  "GET",
			RootURL: downloadURL,
			Options: options,
		})
		return shouldRetry(ctx, resp, err)
	})
	if err != nil {
		return nil, err
	}

	return resp.Body, nil
}

// Update the object with the contents of the io.Reader
func (o *Object) Update(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) error {
	// Delete the old file first
	if o.fileCode != "" {
		_ = o.fs.deleteFile(ctx, o.fileCode)
	}

	// Upload the new file
	newObj, err := o.fs.Put(ctx, in, src, options...)
	if err != nil {
		return err
	}

	// Copy the new object's data
	newO, ok := newObj.(*Object)
	if ok {
		o.fileCode = newO.fileCode
		o.fileName = newO.fileName
		o.fileSize = newO.fileSize
		o.modTime = newO.modTime
		o.folderID = newO.folderID
	}

	return nil
}

// Remove an object
func (o *Object) Remove(ctx context.Context) error {
	return o.fs.deleteFile(ctx, o.fileCode)
}

// Verify that all the interfaces are implemented correctly
var (
	_ fs.Fs     = (*Fs)(nil)
	_ fs.Mover  = (*Fs)(nil)
	_ fs.Abouter = (*Fs)(nil)
	_ fs.Object = (*Object)(nil)
)
