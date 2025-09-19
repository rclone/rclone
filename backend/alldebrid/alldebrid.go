// Package alldebrid provides an interface to the alldebrid.com
// object storage system.
package alldebrid

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/rclone/rclone/backend/alldebrid/api"
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
)

const (
	minSleep      = 10 * time.Millisecond
	maxSleep      = 2 * time.Second
	decayConstant = 2 // bigger for slower decay, exponential
	rootURL       = "https://api.alldebrid.com"
)

// Globals

// Register with Fs
func init() {
	fs.Register(&fs.RegInfo{
		Name:        "alldebrid",
		Description: "alldebrid.com",
		NewFs:       NewFs,
		Config: func(ctx context.Context, name string, m configmap.Mapper, config fs.ConfigIn) (*fs.ConfigOut, error) {
			return &fs.ConfigOut{}, nil
		},
		Options: []fs.Option{{
			Name:      "api_key",
			Help:      `API Key.\n\nGet yours from https://alldebrid.com/apikeys`,
			Required:  true,
			Sensitive: true,
		}, {
			Name:     config.ConfigEncoding,
			Help:     config.ConfigEncodingHelp,
			Advanced: true,
			// Encode invalid UTF-8 bytes as json doesn't handle them properly.
			Default: (encoder.Display |
				encoder.EncodeBackSlash |
				encoder.EncodeDoubleQuote |
				encoder.EncodeInvalidUtf8),
		}},
	})
}

// Options defines the configuration for this backend
type Options struct {
	APIKey string               `config:"api_key"`
	Enc    encoder.MultiEncoder `config:"encoding"`
}

// Fs represents a remote cloud storage system
type Fs struct {
	name             string                         // name of this remote
	root             string                         // the path we are working on
	opt              Options                        // parsed options
	features         *fs.Features                   // optional features
	srv              *rest.Client                   // the connection to the server
	pacer            *fs.Pacer                      // pacer for API calls
	magnetFilesCache map[int]*MagnetFilesCacheEntry // cache for magnet files
}

// MagnetFilesCacheEntry holds cached magnet files with expiration
type MagnetFilesCacheEntry struct {
	files   []api.MagnetFile
	expires time.Time
}

// Object describes a file
type Object struct {
	fs          *Fs       // what this object is part of
	remote      string    // The remote path
	hasMetaData bool      // metadata is present and correct
	size        int64     // size of the object
	modTime     time.Time // modification time of the object
	id          string    // ID of the object
	mimeType    string    // Mime type of object
	url         string    // URL to download file
	dLink       string
}

// ------------------------------------------------------------

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
	return fmt.Sprintf("alldebrid root '%s'", f.root)
}

// Features returns the optional features of this Fs
func (f *Fs) Features() *fs.Features {
	return f.features
}

// parsePath parses an alldebrid 'url'
func parsePath(path string) (root string) {
	root = strings.Trim(path, "/")
	return
}

// retryErrorCodes is a slice of error codes that we will retry
var retryErrorCodes = []int{
	429, // Too Many Requests.
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
	return fserrors.ShouldRetry(err) || fserrors.ShouldRetryHTTP(resp, retryErrorCodes), err
}

// errorHandler parses a non 2xx error response into an error
func errorHandler(resp *http.Response) error {
	body, err := rest.ReadBody(resp)
	if err != nil {
		body = nil
	}
	var e api.Response
	if body != nil {
		_ = json.Unmarshal(body, &e)
		if e.Error != nil {
			return fmt.Errorf("alldebrid error %s: %s", e.Error.Code, e.Error.Message)
		}
	}
	return fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
}

// NewFs constructs an Fs from the path, container:path
func NewFs(ctx context.Context, name, root string, m configmap.Mapper) (fs.Fs, error) {
	// Parse config into Options struct
	opt := new(Options)
	err := configstruct.Set(m, opt)
	if err != nil {
		return nil, err
	}

	root = parsePath(root)

	client := fshttp.NewClient(ctx)

	f := &Fs{
		name:             name,
		root:             root,
		opt:              *opt,
		srv:              rest.NewClient(client).SetRoot(rootURL),
		pacer:            fs.NewPacer(ctx, pacer.NewDefault(pacer.MinSleep(minSleep), pacer.MaxSleep(maxSleep), pacer.DecayConstant(decayConstant))),
		magnetFilesCache: make(map[int]*MagnetFilesCacheEntry),
	}
	f.features = (&fs.Features{
		CaseInsensitive:         true,
		CanHaveEmptyDirectories: true,
		ReadMimeType:            false,
	}).Fill(ctx, f)
	f.srv.SetErrorHandler(errorHandler)

	// Set authorization header
	if opt.APIKey != "" {
		f.srv.SetHeader("Authorization", "Bearer "+opt.APIKey)
	}

	// Validate API key
	err = f.validateAPIKey(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to configure alldebrid: %w", err)
	}

	// For alldebrid, check if root points to a file
	if root != "" {
		if _, err := f.newObjectWithInfo(ctx, root, nil); err == nil {
			// Root points to a file, return parent directory
			f.root = path.Dir(root)
			return f, fs.ErrorIsFile
		}
		// Root is a directory path
		f.root = root
	}

	return f, nil
}

// validateAPIKey validates the API key by calling the user endpoint
func (f *Fs) validateAPIKey(ctx context.Context) error {
	var userInfo api.UserResponse
	opts := rest.Opts{
		Method: "GET",
		Path:   "/v4/user",
	}

	err := f.pacer.Call(func() (bool, error) {
		resp, err := f.srv.CallJSON(ctx, &opts, nil, &userInfo)
		return shouldRetry(ctx, resp, err)
	})
	if err != nil {
		return err
	}
	if err := userInfo.AsErr(); err != nil {
		return err
	}
	return nil
}

// Return an Object from a path
//
// If it can't be found it returns the error fs.ErrorObjectNotFound.
func (f *Fs) newObjectWithInfo(ctx context.Context, remote string, info any) (fs.Object, error) {
	o := &Object{
		fs:     f,
		remote: remote,
	}
	var err error
	if info != nil {
		// Set info based on type
		switch v := info.(type) {
		case *api.Link:
			err = o.setLinkMetaData(v)
		case *api.MagnetFile:
			err = o.setMagnetFileMetaData(v)
		default:
			return nil, fmt.Errorf("unsupported info type: %T", info)
		}
	} else {
		// Load all magnets or history and match if it's a file
		parts := strings.Split(o.remote, "/")
		if len(parts) < 2 {
			return nil, fs.ErrorObjectNotFound
		}
		category := parts[0]
		switch category {
		case "links":
			links, err := f.fetchLinks(ctx)
			if err != nil {
				return nil, err
			}
			filename := path.Base(o.remote)
			for _, link := range links {
				if link.Filename == filename {
					err = o.setLinkMetaData(&link)
					return o, err
				}
			}
			return nil, fs.ErrorObjectNotFound
		case "history":
			history, err := f.fetchHistory(ctx)
			if err != nil {
				return nil, err
			}
			filename := path.Base(o.remote)
			for _, link := range history {
				if link.Filename == filename {
					err = o.setLinkMetaData(&link)
					return o, err
				}
			}
			return nil, fs.ErrorObjectNotFound
		case "magnets":
			magnetName := parts[1]
			magnets, err := f.fetchMagnets(ctx)
			if err != nil {
				return nil, err
			}
			for _, magnet := range magnets {
				if magnet.Filename == magnetName {
					if len(parts) == 2 {
						// Single file magnet
						o.size = magnet.Size
						o.modTime = time.Unix(magnet.UploadDate, 0)
						o.url = "" // Defer link fetching to Open
						o.id = fmt.Sprintf("%d", magnet.ID)
						o.hasMetaData = true
						return o, nil
					} else {
						// Multi-level path
						files, err := f.fetchMagnetFiles(ctx, magnet.ID)
						if err != nil {
							continue
						}
						filePath := strings.Join(parts[2:], "/")
						targetPath := path.Join(magnet.Filename, filePath)
						found := f.findFileInMagnet(files, targetPath)
						if found != nil {
							err = o.setMagnetFileMetaData(found)
							return o, err
						}
					}
				}
			}
			return nil, fs.ErrorObjectNotFound
		default:
			return nil, fs.ErrorObjectNotFound
		}
	}
	if err != nil {
		return nil, err
	}
	return o, nil
}

// NewObject finds the Object at remote.  If it can't be found
// it returns the error fs.ErrorObjectNotFound.
func (f *Fs) NewObject(ctx context.Context, remote string) (fs.Object, error) {
	return f.newObjectWithInfo(ctx, remote, nil)
}

// CreateDir makes a directory with pathID as parent and name leaf
func (f *Fs) CreateDir(ctx context.Context, pathID, leaf string) (newID string, err error) {
	// Alldebrid doesn't support creating directories
	return "", fs.ErrorNotImplemented
}

// List the objects and directories in dir into entries.  The
// entries can be returned in any order but should be for a
// complete directory.
//
// dir should be "" to list the root, and should not have
// trailing slashes.
//
// This should return ErrDirNotFound if the directory isn't
// found.
func (f *Fs) List(ctx context.Context, dir string) (entries fs.DirEntries, err error) {
	// Parse the directory path and handle accordingly
	parts := strings.Split(filepath.Join(f.root, strings.Trim(dir, "/")), "/")

	switch len(parts) {
	case 1:
		if parts[0] == "" {
			// Root directory - show virtual directories
			now := time.Now()
			return fs.DirEntries{
				fs.NewDir("links", now),
				fs.NewDir("history", now),
				fs.NewDir("magnets", now),
			}, nil
		}

		// Top level directories
		switch parts[0] {
		case "links":
			return f.listLinksDirectory(ctx, dir)
		case "history":
			return f.listHistoryDirectory(ctx, dir)
		case "magnets":
			return f.listMagnetsDirectory(ctx, dir)
		default:
			return nil, fs.ErrorDirNotFound
		}
	default:
		// Handle deeper paths within virtual directories
		if len(parts) >= 2 {
			switch parts[0] {
			case "magnets":
				// parts[1] is the magnet filename, find the corresponding magnet ID
				magnets, err := f.fetchMagnets(ctx)
				if err != nil {
					return nil, err
				}
				for _, magnet := range magnets {
					if magnet.Filename == parts[1] {
						subDir := ""
						if len(parts) > 2 {
							subDir = strings.Join(parts[2:], "/")
						}
						return f.listMagnetSubDirectory(ctx, &magnet, dir, subDir)
					}
				}
				return nil, fs.ErrorDirNotFound
			case "links", "history":
				// These are flat directories, no subdirectories supported
				return nil, fs.ErrorDirNotFound
			default:
				return nil, fs.ErrorDirNotFound
			}
		}
		return nil, fs.ErrorDirNotFound
	}

}

// listLinksDirectory lists the contents of the links directory
func (f *Fs) listLinksDirectory(ctx context.Context, dir string) (entries fs.DirEntries, err error) {
	var linksInfo api.LinksResponse
	opts := rest.Opts{
		Method: "GET",
		Path:   "/v4/user/links",
	}

	err = f.pacer.Call(func() (bool, error) {
		resp, err := f.srv.CallJSON(ctx, &opts, nil, &linksInfo)
		return shouldRetry(ctx, resp, err)
	})
	if err != nil {
		return nil, err
	}
	if err := linksInfo.AsErr(); err != nil {
		return nil, err
	}

	for i := range linksInfo.Data.Links {
		link := &linksInfo.Data.Links[i]
		remote := path.Join(dir, link.Filename)
		o, err := f.newObjectWithInfo(ctx, remote, link)
		if err != nil {
			return nil, err
		}
		entries = append(entries, o)
	}
	return entries, nil
}

// listHistoryDirectory lists the contents of the history directory
func (f *Fs) listHistoryDirectory(ctx context.Context, dir string) (entries fs.DirEntries, err error) {
	var historyInfo api.HistoryResponse
	opts := rest.Opts{
		Method: "GET",
		Path:   "/v4/user/history",
	}

	err = f.pacer.Call(func() (bool, error) {
		resp, err := f.srv.CallJSON(ctx, &opts, nil, &historyInfo)
		return shouldRetry(ctx, resp, err)
	})
	if err != nil {
		return nil, err
	}
	if err := historyInfo.AsErr(); err != nil {
		return nil, err
	}

	for i := range historyInfo.Data.Links {
		link := &historyInfo.Data.Links[i]
		remote := path.Join(dir, link.Filename)
		o, err := f.newObjectWithInfo(ctx, remote, link)
		if err != nil {
			return nil, err
		}
		entries = append(entries, o)
	}
	return entries, nil
}

// listMagnetsDirectory lists the contents of the magnets directory
func (f *Fs) listMagnetsDirectory(ctx context.Context, dir string) (entries fs.DirEntries, err error) {
	magnets, err := f.fetchMagnets(ctx)
	if err != nil {
		return nil, err
	}

	for _, magnet := range magnets {
		modTime := time.Unix(magnet.UploadDate, 0)
		remote := path.Join(dir, magnet.Filename)

		if magnet.NBLinks == 1 {
			// Single file magnet - list as file
			obj := &Object{
				fs:      f,
				remote:  remote,
				size:    magnet.Size,
				modTime: modTime,
				url:     "", // Defer link fetching to Open
				id:      fmt.Sprintf("%d", magnet.ID),
			}
			obj.hasMetaData = true
			entries = append(entries, obj)
		} else {
			// Multi-file magnet - list as directory
			d := fs.NewDir(remote, modTime)
			d.SetID(fmt.Sprintf("%d", magnet.ID))
			entries = append(entries, d)
		}
	}
	return entries, nil
}

// listMagnetSubDirectory lists files within a subdirectory of a magnet
func (f *Fs) listMagnetSubDirectory(ctx context.Context, magnet *api.Magnet, dir, subDir string) (entries fs.DirEntries, err error) {
	// Use the provided magnet object (already validated)

	// Now fetch files for this specific magnet
	var filesInfo api.MagnetFilesResponse
	opts := rest.Opts{
		Method: "POST",
		Path:   "/v4/magnet/files",
	}

	opts.Parameters = url.Values{}
	opts.Parameters.Set("id[]", fmt.Sprintf("%d", magnet.ID))

	err = f.pacer.Call(func() (bool, error) {
		resp, err := f.srv.CallJSON(ctx, &opts, nil, &filesInfo)
		return shouldRetry(ctx, resp, err)
	})
	if err != nil {
		return nil, err
	}
	if err := filesInfo.AsErr(); err != nil {
		return nil, err
	}

	// Process the file tree
	for _, magnetFiles := range filesInfo.Data.Magnets {
		if magnetFiles.ID == fmt.Sprintf("%d", magnet.ID) {
			f.processMagnetFileTree(dir, "", path.Join(magnet.Filename, subDir), magnet.Filename, magnetFiles.Files, &entries)
			break
		}
	}
	return entries, nil
}

// processMagnetFileTree recursively processes the magnet file tree
//
// This function handles the conversion of Alldebrid's magnet file structure
// to rclone's filesystem entries. Files with download links are treated as
// regular files, while entries with subdirectories are processed recursively.
// Files without links but with size > 0 are assumed to be files, others as directories.
// listPath specifies the subdirectory to list (empty string for root).
func (f *Fs) processMagnetFileTree(dir, currentPath, listPath, magnetFilename string, files []api.MagnetFile, entries *fs.DirEntries) {
	for _, file := range files {
		fullPath := path.Join(currentPath, file.Name)

		// Determine if we should include this entry
		shouldInclude := false
		var displayName string

		if listPath == "" {
			// Root listing: include all files with adjusted path to avoid duplication
			if file.Link != "" || (file.Size > 0 && len(file.Entries) == 0) {
				shouldInclude = true
				displayName, _ = strings.CutPrefix(fullPath, magnetFilename+"/")

			}
		} else {
			// Subdirectory listing: include immediate children of the target directory
			if currentPath == listPath {
				shouldInclude = true
				displayName = file.Name
			}
		}

		if shouldInclude {
			remote := path.Join(dir, displayName)

			if file.Link != "" {
				// This is a file
				obj := &Object{
					fs:      f,
					remote:  remote,
					size:    file.Size,
					modTime: time.Now(), // We don't have mod time for magnet files
					url:     file.Link,
				}
				obj.hasMetaData = true
				*entries = append(*entries, obj)
			} else if len(file.Entries) > 0 {
				// This is a directory
				if listPath == "" {
					// For root listing, adjust displayName
					dirDisplayName, _ := strings.CutPrefix(fullPath, magnetFilename+"/")
					dirRemote := path.Join(dir, dirDisplayName)
					d := fs.NewDir(dirRemote, time.Now())
					*entries = append(*entries, d)
				} else {
					d := fs.NewDir(remote, time.Now())
					*entries = append(*entries, d)
				}
			} else {
				// This might be a file without a link (not yet available)
				if file.Size > 0 {
					// Assume it's a file
					obj := &Object{
						fs:      f,
						remote:  remote,
						size:    file.Size,
						modTime: time.Now(),
					}
					obj.hasMetaData = true
					*entries = append(*entries, obj)
				} else {
					// Assume it's a directory
					if listPath == "" {
						// For root listing, adjust displayName
						dirDisplayName, _ := strings.CutPrefix(fullPath, magnetFilename+"/")
						dirRemote := path.Join(dir, dirDisplayName)
						d := fs.NewDir(dirRemote, time.Now())
						*entries = append(*entries, d)
					} else {
						d := fs.NewDir(remote, time.Now())
						*entries = append(*entries, d)
					}
				}
			}
		}

		// Always recurse into subdirectories to find the target path
		if len(file.Entries) > 0 {
			f.processMagnetFileTree(dir, fullPath, listPath, magnetFilename, file.Entries, entries)
		}
	}
}

// fetchLinks fetches the saved links
func (f *Fs) fetchLinks(ctx context.Context) ([]api.Link, error) {
	var linksInfo api.LinksResponse
	opts := rest.Opts{
		Method: "GET",
		Path:   "/v4/user/links",
	}

	err := f.pacer.Call(func() (bool, error) {
		resp, err := f.srv.CallJSON(ctx, &opts, nil, &linksInfo)
		return shouldRetry(ctx, resp, err)
	})
	if err != nil {
		return nil, err
	}
	if err := linksInfo.AsErr(); err != nil {
		return nil, err
	}

	return linksInfo.Data.Links, nil
}

// fetchHistory fetches the download history
func (f *Fs) fetchHistory(ctx context.Context) ([]api.Link, error) {
	var historyInfo api.HistoryResponse
	opts := rest.Opts{
		Method: "GET",
		Path:   "/v4/user/history",
	}

	err := f.pacer.Call(func() (bool, error) {
		resp, err := f.srv.CallJSON(ctx, &opts, nil, &historyInfo)
		return shouldRetry(ctx, resp, err)
	})
	if err != nil {
		return nil, err
	}
	if err := historyInfo.AsErr(); err != nil {
		return nil, err
	}

	return historyInfo.Data.Links, nil
}

// fetchMagnetFiles fetches files for a specific magnet with caching
func (f *Fs) fetchMagnetFiles(ctx context.Context, magnetID int) ([]api.MagnetFile, error) {
	// Check cache first
	if entry, ok := f.magnetFilesCache[magnetID]; ok && time.Now().Before(entry.expires) {
		return entry.files, nil
	}

	var filesInfo api.MagnetFilesResponse
	opts := rest.Opts{
		Method: "POST",
		Path:   "/v4/magnet/files",
	}

	opts.Parameters = url.Values{}
	opts.Parameters.Set("id[]", fmt.Sprintf("%d", magnetID))

	err := f.pacer.Call(func() (bool, error) {
		resp, err := f.srv.CallJSON(ctx, &opts, nil, &filesInfo)
		return shouldRetry(ctx, resp, err)
	})
	if err != nil {
		return nil, err
	}
	if err := filesInfo.AsErr(); err != nil {
		return nil, err
	}

	// Return files from the first (and only) magnet
	for _, magnetFiles := range filesInfo.Data.Magnets {
		if magnetFiles.ID == fmt.Sprintf("%d", magnetID) {
			// Cache the result
			f.magnetFilesCache[magnetID] = &MagnetFilesCacheEntry{
				files:   magnetFiles.Files,
				expires: time.Now().Add(3 * time.Hour),
			}
			return magnetFiles.Files, nil
		}
	}
	return nil, fmt.Errorf("magnet files not found for ID %d", magnetID)
}

// findFileInMagnet finds a file in the magnet file tree by path
func (f *Fs) findFileInMagnet(files []api.MagnetFile, targetPath string) *api.MagnetFile {
	return f.findFileRecursive(files, "", targetPath)
}

func (f *Fs) findFileRecursive(files []api.MagnetFile, currentPath, targetPath string) *api.MagnetFile {
	for _, file := range files {
		fullPath := path.Join(currentPath, file.Name)
		if fullPath == targetPath {
			return &file
		}
		// Recurse into subdirectories
		if len(file.Entries) > 0 {
			if found := f.findFileRecursive(file.Entries, fullPath, targetPath); found != nil {
				return found
			}
		}
	}
	return nil
}

// fetchMagnets fetches the current magnet list
func (f *Fs) fetchMagnets(ctx context.Context) ([]api.Magnet, error) {
	var magnetsInfo api.MagnetStatusResponse
	opts := rest.Opts{
		Method: "POST",
		Path:   "/v4.1/magnet/status",
	}

	err := f.pacer.Call(func() (bool, error) {
		resp, err := f.srv.CallJSON(ctx, &opts, nil, &magnetsInfo)
		return shouldRetry(ctx, resp, err)
	})
	if err != nil {
		return nil, err
	}
	if err := magnetsInfo.AsErr(); err != nil {
		return nil, err
	}

	// Filter for active magnets only (status codes 0-3: Processing/In Queue, Downloading, Compressing/Moving, Uploading)
	var activeMagnets []api.Magnet
	for _, magnet := range magnetsInfo.Data.Magnets {
		if magnet.StatusCode == 4 {
			activeMagnets = append(activeMagnets, magnet)
		}
	}

	return activeMagnets, nil
}

// Put the object
//
// Copy the reader in to the new object which is returned.
//
// The new object may have been created if an error is returned
func (f *Fs) Put(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error) {
	existingObj, err := f.newObjectWithInfo(ctx, src.Remote(), nil)
	switch err {
	case nil:
		return existingObj, existingObj.Update(ctx, in, src, options...)
	case fs.ErrorObjectNotFound:
		// Not found so create it
		return f.PutUnchecked(ctx, in, src, options...)
	default:
		return nil, err
	}
}

// PutUnchecked the object into the container
//
// This will produce an error if the object already exists.
//
// Copy the reader in to the new object which is returned.
//
// The new object may have been created if an error is returned
func (f *Fs) PutUnchecked(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error) {
	remote := src.Remote()

	// For alldebrid, we can add links or magnets
	if strings.HasPrefix(remote, "links/") {
		return f.addLink(ctx, remote, src)
	} else if strings.HasPrefix(remote, "magnets/") {
		return f.addMagnet(ctx, in, src)
	}

	return nil, fs.ErrorNotImplemented
}

// addLink adds a link to the saved links
func (f *Fs) addLink(ctx context.Context, remote string, src fs.ObjectInfo) (fs.Object, error) {
	linkURL := strings.TrimPrefix(src.Remote(), "links/")

	var saveInfo api.LinkSaveResponse
	opts := rest.Opts{
		Method: "POST",
		Path:   "/v4/user/links/save",
	}

	opts.Parameters = url.Values{}
	opts.Parameters.Set("links[]", linkURL)

	err := f.pacer.Call(func() (bool, error) {
		resp, err := f.srv.CallJSON(ctx, &opts, nil, &saveInfo)
		return shouldRetry(ctx, resp, err)
	})
	if err != nil {
		return nil, err
	}
	if err := saveInfo.AsErr(); err != nil {
		return nil, err
	}

	// Create a virtual object representing the saved link
	o := &Object{
		fs:     f,
		remote: remote,
		url:    linkURL,
	}
	return o, nil
}

// addMagnet adds a magnet or torrent file
func (f *Fs) addMagnet(ctx context.Context, in io.Reader, src fs.ObjectInfo) (fs.Object, error) {
	remote := src.Remote()

	// Extract the magnet identifier from the path
	parts := strings.Split(strings.TrimPrefix(remote, "magnets/"), "/")
	if len(parts) == 0 {
		return nil, errors.New("invalid magnet path")
	}

	magnetInput := parts[0]

	// Check if it's a magnet URI (starts with magnet:)
	if strings.HasPrefix(magnetInput, "magnet:") {
		return f.uploadMagnetURI(ctx, magnetInput)
	}

	// Check if it's a torrent file (has .torrent extension or contains torrent data)
	if strings.HasSuffix(magnetInput, ".torrent") || f.isTorrentData(in) {
		return f.uploadTorrentFile(ctx, in, src)
	}

	// Default to treating as magnet URI
	return f.uploadMagnetURI(ctx, magnetInput)
}

// uploadMagnetURI uploads a magnet URI to alldebrid
func (f *Fs) uploadMagnetURI(ctx context.Context, magnetURI string) (fs.Object, error) {
	var uploadInfo api.MagnetUploadResponse
	opts := rest.Opts{
		Method: "POST",
		Path:   "/v4/magnet/upload",
	}

	opts.Parameters = url.Values{}
	opts.Parameters.Set("magnets[]", magnetURI)

	err := f.pacer.Call(func() (bool, error) {
		resp, err := f.srv.CallJSON(ctx, &opts, nil, &uploadInfo)
		return shouldRetry(ctx, resp, err)
	})
	if err != nil {
		return nil, err
	}
	if err := uploadInfo.AsErr(); err != nil {
		return nil, err
	}

	// Check if upload was successful
	if len(uploadInfo.Data.Magnets) == 0 {
		return nil, errors.New("no magnet upload result received")
	}

	magnet := uploadInfo.Data.Magnets[0]
	if magnet.Error != nil {
		return nil, magnet.Error
	}

	// Create a virtual object representing the uploaded magnet
	o := &Object{
		fs:      f,
		remote:  fmt.Sprintf("magnets/%d", magnet.ID),
		size:    magnet.Size,
		modTime: time.Now(),
		id:      fmt.Sprintf("%d", magnet.ID),
	}

	return o, nil
}

// uploadTorrentFile uploads a torrent file to alldebrid
func (f *Fs) uploadTorrentFile(ctx context.Context, in io.Reader, src fs.ObjectInfo) (fs.Object, error) {
	// Read the torrent file data
	torrentData, err := io.ReadAll(in)
	if err != nil {
		return nil, fmt.Errorf("failed to read torrent file: %w", err)
	}

	var uploadInfo api.MagnetUploadFileResponse
	opts := rest.Opts{
		Method: "POST",
		Path:   "/v4/magnet/upload/file",
	}

	// Create multipart form data
	opts.MultipartParams = url.Values{}
	opts.MultipartParams.Set("file", string(torrentData))
	opts.MultipartParams.Set("name", src.Remote())

	err = f.pacer.Call(func() (bool, error) {
		resp, err := f.srv.CallJSON(ctx, &opts, nil, &uploadInfo)
		return shouldRetry(ctx, resp, err)
	})
	if err != nil {
		return nil, err
	}
	if err := uploadInfo.AsErr(); err != nil {
		return nil, err
	}

	// Check if upload was successful
	if len(uploadInfo.Data.Files) == 0 {
		return nil, errors.New("no torrent upload result received")
	}

	file := uploadInfo.Data.Files[0]
	if file.Error != nil {
		return nil, file.Error
	}

	// Create a virtual object representing the uploaded torrent
	o := &Object{
		fs:      f,
		remote:  fmt.Sprintf("magnets/%d", file.ID),
		size:    file.Size,
		modTime: time.Now(),
		id:      fmt.Sprintf("%d", file.ID),
	}

	return o, nil
}

// isTorrentData checks if the input data looks like a torrent file
func (f *Fs) isTorrentData(in io.Reader) bool {
	// Read first few bytes to check for torrent file signature
	buf := make([]byte, 11)
	n, err := in.Read(buf)
	if err != nil || n < 11 {
		return false
	}

	// Torrent files start with "d8:announce" or similar bencoded data
	// For simplicity, we'll check if it starts with 'd' (dictionary in bencode)
	return buf[0] == 'd'
}

// Mkdir creates the container if it doesn't exist
func (f *Fs) Mkdir(ctx context.Context, dir string) error {
	// Alldebrid doesn't support creating directories
	return fs.ErrorNotImplemented
}

// Rmdir deletes the root folder
//
// Returns an error if it isn't empty
func (f *Fs) Rmdir(ctx context.Context, dir string) error {
	// Alldebrid doesn't support deleting directories
	return fs.ErrorNotImplemented
}

// Precision return the precision of this Fs
func (f *Fs) Precision() time.Duration {
	return fs.ModTimeNotSupported
}

// Purge deletes all the files in the directory
//
// Optional interface: Only implement this if you have a way of
// deleting all the files quicker than just running Remove() on the
// result of List()
func (f *Fs) Purge(ctx context.Context, dir string) error {
	// Alldebrid doesn't support purging directories
	return fs.ErrorCantPurge
}

// Hashes returns the supported hash sets.
func (f *Fs) Hashes() hash.Set {
	return hash.Set(hash.None)
}

// ------------------------------------------------------------

// Fs returns the parent Fs
func (o *Object) Fs() fs.Info {
	return o.fs
}

// Return a string version
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

// Hash returns the SHA-1 of an object returning a lowercase hex string
func (o *Object) Hash(ctx context.Context, t hash.Type) (string, error) {
	return "", hash.ErrUnsupported
}

// Size returns the size of an object in bytes
func (o *Object) Size() int64 {
	if o.hasMetaData {
		return o.size
	}
	return 0
}

// setLinkMetaData sets the metadata from a link
func (o *Object) setLinkMetaData(info *api.Link) (err error) {
	o.hasMetaData = true
	o.size = info.Size
	o.modTime = time.Unix(info.Date, 0)
	o.url = info.Link
	return nil
}

// setMagnetFileMetaData sets the metadata from a magnet file
func (o *Object) setMagnetFileMetaData(info *api.MagnetFile) (err error) {
	o.hasMetaData = true
	o.size = info.Size
	o.modTime = time.Now() // We don't have mod time for magnet files
	o.url = info.Link
	return nil
}

// unlockLink unlocks a link to get the download URL
func (o *Object) unlockLink(ctx context.Context) error {
	var unlockInfo api.LinkUnlockResponse
	opts := rest.Opts{
		Method: "POST",
		Path:   "/v4/link/unlock",
	}

	opts.Parameters = url.Values{}
	opts.Parameters.Set("link", o.url)

	err := o.fs.pacer.Call(func() (bool, error) {
		resp, err := o.fs.srv.CallJSON(ctx, &opts, nil, &unlockInfo)
		return shouldRetry(ctx, resp, err)
	})
	if err != nil {
		return err
	}
	if err := unlockInfo.AsErr(); err != nil {
		return err
	}

	o.hasMetaData = true
	o.size = unlockInfo.Data.Filesize
	o.dLink = unlockInfo.Data.Link
	return nil
}

// ModTime returns the modification time of the object
//
// It attempts to read the objects mtime and if that isn't present the
// LastModified returned in the http headers
func (o *Object) ModTime(ctx context.Context) time.Time {
	return o.modTime
}

// SetModTime sets the modification time of the local fs object
func (o *Object) SetModTime(ctx context.Context, modTime time.Time) error {
	return fs.ErrorCantSetModTime
}

// Storable returns a boolean showing whether this object storable
func (o *Object) Storable() bool {
	return true
}

// Open an object for read
func (o *Object) Open(ctx context.Context, options ...fs.OpenOption) (in io.ReadCloser, err error) {
	if o.url == "" {
		// For single-file magnets, fetch the link from /v4/magnet/files
		if o.id != "" {
			magnetID, err := strconv.Atoi(o.id)
			if err == nil {
				files, err := o.fs.fetchMagnetFiles(ctx, magnetID)
				if err == nil && len(files) == 1 {
					o.url = files[0].Link
				}
			}
		}
		if o.url == "" {
			return nil, errors.New("can't download - no URL")
		}
	}
	if o.dLink == "" {
		err = o.unlockLink(ctx)
	}
	if err != nil {
		return nil, err
	}
	fs.FixRangeOption(options, o.size)
	var resp *http.Response
	opts := rest.Opts{
		Path:    "",
		RootURL: o.dLink,
		Method:  "GET",
		Options: options,
	}
	err = o.fs.pacer.Call(func() (bool, error) {
		resp, err = o.fs.srv.Call(ctx, &opts)
		return shouldRetry(ctx, resp, err)
	})
	if err != nil {
		return nil, err
	}
	return resp.Body, err
}

// Update the object with the contents of the io.Reader, modTime and size
//
// If existing is set then it updates the object rather than creating a new one.
//
// The new object may have been created if an error is returned
func (o *Object) Update(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (err error) {
	// Alldebrid doesn't support updating existing objects
	return fs.ErrorNotImplemented
}

// Remove an object
func (o *Object) Remove(ctx context.Context) error {
	// For links, we can remove from saved links
	if strings.HasPrefix(o.remote, "links/") {
		return o.removeLink(ctx)
	}
	// For magnets, we can delete magnets
	if strings.HasPrefix(o.remote, "magnets/") {
		return o.removeMagnet(ctx)
	}
	return fs.ErrorNotImplemented
}

// removeLink removes a link from saved links
func (o *Object) removeLink(ctx context.Context) error {
	var deleteInfo api.LinkDeleteResponse
	opts := rest.Opts{
		Method: "POST",
		Path:   "/v4/user/links/delete",
	}

	opts.Parameters = url.Values{}
	opts.Parameters.Set("links[]", o.url)

	err := o.fs.pacer.Call(func() (bool, error) {
		resp, err := o.fs.srv.CallJSON(ctx, &opts, nil, &deleteInfo)
		return shouldRetry(ctx, resp, err)
	})
	if err != nil {
		return err
	}
	if err := deleteInfo.AsErr(); err != nil {
		return err
	}
	return nil
}

// removeMagnet removes a magnet
func (o *Object) removeMagnet(ctx context.Context) error {
	// Extract magnet filename from remote path
	parts := strings.Split(o.remote, "/")
	if len(parts) < 2 {
		return fs.ErrorObjectNotFound
	}

	magnetFilename := parts[1]

	// Find the magnet by filename to get ID
	magnets, err := o.fs.fetchMagnets(ctx)
	if err != nil {
		return err
	}
	var magnetID int
	found := false
	for _, magnet := range magnets {
		if magnet.Filename == magnetFilename {
			magnetID = magnet.ID
			found = true
			break
		}
	}
	if !found {
		return fs.ErrorObjectNotFound
	}

	var deleteInfo api.MagnetDeleteResponse
	opts := rest.Opts{
		Method: "POST",
		Path:   "/v4/magnet/delete",
	}

	opts.Parameters = url.Values{}
	opts.Parameters.Set("id", fmt.Sprintf("%d", magnetID))

	err = o.fs.pacer.Call(func() (bool, error) {
		resp, err := o.fs.srv.CallJSON(ctx, &opts, nil, &deleteInfo)
		return shouldRetry(ctx, resp, err)
	})
	if err != nil {
		return err
	}
	if err := deleteInfo.AsErr(); err != nil {
		return err
	}
	// Clear cache for this magnet
	delete(o.fs.magnetFilesCache, magnetID)
	return nil
}

// MimeType of an Object if known, "" otherwise
func (o *Object) MimeType(ctx context.Context) string {
	return o.mimeType
}

// ID returns the ID of the Object if known, or "" if not
func (o *Object) ID() string {
	return o.id
}

// Check the interfaces are satisfied
var (
	_ fs.Fs        = (*Fs)(nil)
	_ fs.Purger    = (*Fs)(nil)
	_ fs.Object    = (*Object)(nil)
	_ fs.MimeTyper = (*Object)(nil)
	_ fs.IDer      = (*Object)(nil)
)
