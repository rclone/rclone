// Package gofile provides an interface to the Gofile
// object storage system.
package gofile

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/rclone/rclone/backend/gofile/api"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config"
	"github.com/rclone/rclone/fs/config/configmap"
	"github.com/rclone/rclone/fs/config/configstruct"
	"github.com/rclone/rclone/fs/fserrors"
	"github.com/rclone/rclone/fs/fshttp"
	"github.com/rclone/rclone/fs/hash"
	"github.com/rclone/rclone/fs/walk"
	"github.com/rclone/rclone/lib/dircache"
	"github.com/rclone/rclone/lib/encoder"
	"github.com/rclone/rclone/lib/pacer"
	"github.com/rclone/rclone/lib/rest"
)

const (
	minSleep       = 10 * time.Millisecond
	maxSleep       = 20 * time.Second
	decayConstant  = 1 // bigger for slower decay, exponential
	rootURL        = "https://api.gofile.io"
	serversExpiry  = 60 * time.Second // check for new upload servers this often
	serversActive  = 2                // choose this many closest upload servers to use
	rateLimitSleep = 5 * time.Second  // penalise a goroutine by this long for making a rate limit error
	maxDepth       = 4                // in ListR recursive list this deep (maximum is 16)
)

/*
   // TestGoFile{sb0-v}
stringNeedsEscaping = []rune{
	'!', '*', '.', '/', ':', '<', '>', '?', '\"', '\\', '\a', '\b', '\f', '\n', '\r', '\t', '\v', '\x00', '\x01', '\x02', '\x03', '\x04', '\x05', '\x06', '\x0e', '\x0f', '\x10', '\x11', '\x12', '\x13', '\x14', '\x15', '\x16', '\x17', '\x18', '\x19', '\x1a', '\x1b', '\x1c', '\x1d', '\x1e', '\x1f', '\x7f', '\xbf', '\xfe', '|'
}
maxFileLength = 255 // for 1 byte unicode characters
maxFileLength = 255 // for 2 byte unicode characters
maxFileLength = 255 // for 3 byte unicode characters
maxFileLength = 255 // for 4 byte unicode characters
canWriteUnnormalized = true
canReadUnnormalized   = true
canReadRenormalized   = false
canStream = true
base32768isOK = false // make sure maxFileLength for 2 byte unicode chars is the same as for 1 byte characters
*/

// Register with Fs
func init() {
	fs.Register(&fs.RegInfo{
		Name:        "gofile",
		Description: "Gofile",
		NewFs:       NewFs,
		Options: []fs.Option{{
			Name: "access_token",
			Help: `API Access token

You can get this from the web control panel.`,
			Sensitive: true,
		}, {
			Name: "root_folder_id",
			Help: `ID of the root folder

Leave this blank normally, rclone will fill it in automatically.

If you want rclone to be restricted to a particular folder you can
fill it in - see the docs for more info.
`,
			Default:   "",
			Advanced:  true,
			Sensitive: true,
		}, {
			Name: "account_id",
			Help: `Account ID

Leave this blank normally, rclone will fill it in automatically.
`,
			Default:   "",
			Advanced:  true,
			Sensitive: true,
		}, {
			Name:     "list_chunk",
			Help:     `Number of items to list in each call`,
			Default:  1000,
			Advanced: true,
		}, {
			Name:     config.ConfigEncoding,
			Help:     config.ConfigEncodingHelp,
			Advanced: true,
			Default: (encoder.Display | // Slash Control Delete Dot
				encoder.EncodeDoubleQuote |
				encoder.EncodeAsterisk |
				encoder.EncodeColon |
				encoder.EncodeLtGt |
				encoder.EncodeQuestion |
				encoder.EncodeBackSlash |
				encoder.EncodePipe |
				encoder.EncodeExclamation |
				encoder.EncodeLeftPeriod |
				encoder.EncodeRightPeriod |
				encoder.EncodeInvalidUtf8),
		}},
	})
}

// Options defines the configuration for this backend
type Options struct {
	AccessToken  string               `config:"access_token"`
	RootFolderID string               `config:"root_folder_id"`
	AccountID    string               `config:"account_id"`
	ListChunk    int                  `config:"list_chunk"`
	Enc          encoder.MultiEncoder `config:"encoding"`
}

// Fs represents a remote gofile
type Fs struct {
	name           string             // name of this remote
	root           string             // the path we are working on
	opt            Options            // parsed options
	features       *fs.Features       // optional features
	srv            *rest.Client       // the connection to the server
	dirCache       *dircache.DirCache // Map of directory path to directory id
	pacer          *fs.Pacer          // pacer for API calls
	serversMu      *sync.Mutex        // protect the servers info below
	servers        []api.Server       // upload servers we can use
	serversChecked time.Time          // time the servers were refreshed
}

// Object describes a gofile object
//
// The full set of metadata will always be present
type Object struct {
	fs       *Fs       // what this object is part of
	remote   string    // The remote path
	size     int64     // size of the object
	modTime  time.Time // modification time of the object
	id       string    // ID of the object
	dirID    string    // ID of the object's directory
	mimeType string    // mime type of the object
	md5      string    // MD5 of the object content
	url      string    // where to download this object
}

// Directory describes a gofile directory
type Directory struct {
	Object
	items int64 // number of items in the directory
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
	return fmt.Sprintf("gofile root '%s'", f.root)
}

// Features returns the optional features of this Fs
func (f *Fs) Features() *fs.Features {
	return f.features
}

// parsePath parses a gofile 'url'
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

// Return true if the api error has the status given
func isAPIErr(err error, status string) bool {
	var apiErr api.Error
	if errors.As(err, &apiErr) {
		return apiErr.Status == status
	}
	return false
}

// shouldRetry returns a boolean as to whether this resp and err
// deserve to be retried.  It returns the err as a convenience
func shouldRetry(ctx context.Context, resp *http.Response, err error) (bool, error) {
	if fserrors.ContextError(ctx, &err) {
		return false, err
	}
	if isAPIErr(err, "error-rateLimit") {
		// Give an immediate penalty to rate limits
		fs.Debugf(nil, "Rate limited, sleep for %v", rateLimitSleep)
		time.Sleep(rateLimitSleep)
		//return true, pacer.RetryAfterError(err, 2*time.Second)
		return true, err
	}
	return fserrors.ShouldRetry(err) || fserrors.ShouldRetryHTTP(resp, retryErrorCodes), err
}

// readMetaDataForPath reads the metadata from the path
func (f *Fs) readMetaDataForPath(ctx context.Context, path string) (info *api.Item, err error) {
	// defer log.Trace(f, "path=%q", path)("info=%+v, err=%v", &info, &err)
	leaf, directoryID, err := f.dirCache.FindPath(ctx, path, false)
	if err != nil {
		if err == fs.ErrorDirNotFound {
			return nil, fs.ErrorObjectNotFound
		}
		return nil, err
	}

	found, err := f.listAll(ctx, directoryID, false, true, leaf, func(item *api.Item) bool {
		if item.Name == leaf {
			info = item
			return true
		}
		return false
	})
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, fs.ErrorObjectNotFound
	}
	return info, nil
}

// readMetaDataForID reads the metadata for the ID given
func (f *Fs) readMetaDataForID(ctx context.Context, id string) (info *api.Item, err error) {
	opts := rest.Opts{
		Method: "GET",
		Path:   "/contents/" + id,
		Parameters: url.Values{
			"page":     {"1"},
			"pageSize": {"1"}, // not interested in children so just ask for 1
		},
	}
	var result api.Contents
	err = f.pacer.Call(func() (bool, error) {
		resp, err := f.srv.CallJSON(ctx, &opts, nil, &result)
		// Retry not found errors - when looking for an ID it should really exist
		if isAPIErr(err, "error-notFound") {
			return true, err
		}
		return shouldRetry(ctx, resp, err)
	})
	if err = result.Err(err); err != nil {
		return nil, fmt.Errorf("failed to get item info: %w", err)
	}
	return &result.Data.Item, nil
}

// errorHandler parses a non 2xx error response into an error
func errorHandler(resp *http.Response) error {
	body, err := rest.ReadBody(resp)
	if err != nil {
		fs.Debugf(nil, "Couldn't read error out of body: %v", err)
		body = nil
	}
	// Decode error response if there was one - they can be blank
	var errResponse api.Error
	if len(body) > 0 {
		err = json.Unmarshal(body, &errResponse)
		if err != nil {
			fs.Debugf(nil, "Couldn't decode error response: %v", err)
		}
	}
	if errResponse.Status == "" {
		errResponse.Status = fmt.Sprintf("%s (%d): %s", resp.Status, resp.StatusCode, string(body))
	}
	return errResponse
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
		name:      name,
		root:      root,
		opt:       *opt,
		srv:       rest.NewClient(client).SetRoot(rootURL),
		pacer:     fs.NewPacer(ctx, pacer.NewDefault(pacer.MinSleep(minSleep), pacer.MaxSleep(maxSleep), pacer.DecayConstant(decayConstant))),
		serversMu: new(sync.Mutex),
	}
	f.features = (&fs.Features{
		CaseInsensitive:          false,
		CanHaveEmptyDirectories:  true,
		DuplicateFiles:           true,
		ReadMimeType:             true,
		WriteMimeType:            false,
		WriteDirSetModTime:       true,
		DirModTimeUpdatesOnWrite: true,
	}).Fill(ctx, f)
	f.srv.SetErrorHandler(errorHandler)
	f.srv.SetHeader("Authorization", "Bearer "+f.opt.AccessToken)

	// Read account ID if not present
	err = f.readAccountID(ctx, m)
	if err != nil {
		return nil, err
	}

	// Read Root Folder ID if not present
	err = f.readRootFolderID(ctx, m)
	if err != nil {
		return nil, err
	}

	// Get rootFolderID
	rootID := f.opt.RootFolderID
	f.dirCache = dircache.New(root, rootID, f)

	// Find the current root
	err = f.dirCache.FindRoot(ctx, false)
	if err != nil {
		// Assume it is a file
		newRoot, remote := dircache.SplitPath(root)
		tempF := *f
		tempF.dirCache = dircache.New(newRoot, rootID, &tempF)
		tempF.root = newRoot
		// Make new Fs which is the parent
		err = tempF.dirCache.FindRoot(ctx, false)
		if err != nil {
			// No root so return old f
			return f, nil
		}
		_, err := tempF.newObjectWithInfo(ctx, remote, nil)
		if err != nil {
			if err == fs.ErrorObjectNotFound {
				// File doesn't exist so return old f
				return f, nil
			}
			return nil, err
		}
		f.features.Fill(ctx, &tempF)
		// XXX: update the old f here instead of returning tempF, since
		// `features` were already filled with functions having *f as a receiver.
		// See https://github.com/rclone/rclone/issues/2182
		f.dirCache = tempF.dirCache
		f.root = tempF.root
		// return an error with an fs which points to the parent
		return f, fs.ErrorIsFile
	}
	return f, nil
}

// Read the AccountID into f.opt if not set and cache in the config file as account_id
func (f *Fs) readAccountID(ctx context.Context, m configmap.Mapper) (err error) {
	if f.opt.AccountID != "" {
		return nil
	}
	opts := rest.Opts{
		Method: "GET",
		Path:   "/accounts/getid",
	}
	var result api.AccountsGetID
	var resp *http.Response
	err = f.pacer.Call(func() (bool, error) {
		resp, err = f.srv.CallJSON(ctx, &opts, nil, &result)
		return shouldRetry(ctx, resp, err)
	})
	if err = result.Err(err); err != nil {
		return fmt.Errorf("failed to read account ID: %w", err)
	}
	f.opt.AccountID = result.Data.ID
	m.Set("account_id", f.opt.AccountID)
	return nil
}

// Read the Accounts info
func (f *Fs) getAccounts(ctx context.Context) (result *api.AccountsGet, err error) {
	opts := rest.Opts{
		Method: "GET",
		Path:   "/accounts/" + f.opt.AccountID,
	}
	result = new(api.AccountsGet)
	var resp *http.Response
	err = f.pacer.Call(func() (bool, error) {
		resp, err = f.srv.CallJSON(ctx, &opts, nil, &result)
		return shouldRetry(ctx, resp, err)
	})
	if err = result.Err(err); err != nil {
		return nil, fmt.Errorf("failed to read accountd info: %w", err)
	}
	return result, nil
}

// Read the RootFolderID into f.opt if not set and cache in the config file as root_folder_id
func (f *Fs) readRootFolderID(ctx context.Context, m configmap.Mapper) (err error) {
	if f.opt.RootFolderID != "" {
		return nil
	}
	result, err := f.getAccounts(ctx)
	if err != nil {
		return err
	}
	f.opt.RootFolderID = result.Data.RootFolder
	m.Set("root_folder_id", f.opt.RootFolderID)
	return nil
}

// Find the top n servers measured by response time
func (f *Fs) bestServers(ctx context.Context, servers []api.Server, n int) (newServers []api.Server) {
	ctx, cancel := context.WithDeadline(ctx, time.Now().Add(10*time.Second))
	defer cancel()

	if n > len(servers) {
		n = len(servers)
	}
	results := make(chan int, len(servers))

	// Test how long the servers take to respond
	for i := range servers {
		i := i // for closure
		go func() {
			opts := rest.Opts{
				Method:  "GET",
				RootURL: servers[i].Root(),
			}
			var result api.UploadServerStatus
			start := time.Now()
			_, err := f.srv.CallJSON(ctx, &opts, nil, &result)
			ping := time.Since(start)
			err = result.Err(err)
			if err != nil {
				results <- -1 // send a -ve number on error
				return
			}
			fs.Debugf(nil, "Upload server %v responded in %v", &servers[i], ping)
			results <- i
		}()
	}

	// Wait for n servers to respond
	newServers = make([]api.Server, 0, n)
	for range servers {
		i := <-results
		if i >= 0 {
			newServers = append(newServers, servers[i])
		}
		if len(newServers) >= n {
			break
		}
	}
	return newServers
}

// Clear all the upload servers - call on an error
func (f *Fs) clearServers() {
	f.serversMu.Lock()
	defer f.serversMu.Unlock()

	fs.Debugf(f, "Clearing upload servers")
	f.servers = nil
}

// Gets an upload server
func (f *Fs) getServer(ctx context.Context) (server *api.Server, err error) {
	f.serversMu.Lock()
	defer f.serversMu.Unlock()

	if len(f.servers) == 0 || time.Since(f.serversChecked) >= serversExpiry {
		opts := rest.Opts{
			Method: "GET",
			Path:   "/servers",
		}
		var result api.ServersResponse
		var resp *http.Response
		err = f.pacer.Call(func() (bool, error) {
			resp, err = f.srv.CallJSON(ctx, &opts, nil, &result)
			return shouldRetry(ctx, resp, err)
		})
		if err = result.Err(err); err != nil {
			if len(f.servers) == 0 {
				return nil, fmt.Errorf("failed to read upload servers: %w", err)
			}
			fs.Errorf(f, "failed to read new upload servers: %v", err)
		} else {
			// Find the top servers measured by response time
			f.servers = f.bestServers(ctx, result.Data.Servers, serversActive)
			f.serversChecked = time.Now()
		}
	}

	if len(f.servers) == 0 {
		return nil, errors.New("no upload servers found")
	}

	// Pick a server at random since we've already found the top ones
	i := rand.Intn(len(f.servers))
	return &f.servers[i], nil
}

// rootSlash returns root with a slash on if it is empty, otherwise empty string
func (f *Fs) rootSlash() string {
	if f.root == "" {
		return f.root
	}
	return f.root + "/"
}

// Return an Object from a path
//
// If it can't be found it returns the error fs.ErrorObjectNotFound.
func (f *Fs) newObjectWithInfo(ctx context.Context, remote string, info *api.Item) (fs.Object, error) {
	o := &Object{
		fs:     f,
		remote: remote,
	}
	var err error
	if info != nil {
		// Set info
		err = o.setMetaData(info)
	} else {
		err = o.readMetaData(ctx) // reads info and meta, returning an error
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

// FindLeaf finds a directory of name leaf in the folder with ID pathID
func (f *Fs) FindLeaf(ctx context.Context, pathID, leaf string) (pathIDOut string, found bool, err error) {
	// Find the leaf in pathID
	found, err = f.listAll(ctx, pathID, true, false, leaf, func(item *api.Item) bool {
		if item.Name == leaf {
			pathIDOut = item.ID
			return true
		}
		return false
	})
	return pathIDOut, found, err
}

// createDir makes a directory with pathID as parent and name leaf and modTime
func (f *Fs) createDir(ctx context.Context, pathID, leaf string, modTime time.Time) (item *api.Item, err error) {
	var resp *http.Response
	var result api.CreateFolderResponse
	opts := rest.Opts{
		Method: "POST",
		Path:   "/contents/createFolder",
	}
	mkdir := api.CreateFolderRequest{
		FolderName:     f.opt.Enc.FromStandardName(leaf),
		ParentFolderID: pathID,
		ModTime:        api.ToNativeTime(modTime),
	}
	err = f.pacer.Call(func() (bool, error) {
		resp, err = f.srv.CallJSON(ctx, &opts, &mkdir, &result)
		return shouldRetry(ctx, resp, err)
	})
	if err = result.Err(err); err != nil {
		return nil, fmt.Errorf("failed to create folder: %w", err)
	}
	return &result.Data, nil
}

// CreateDir makes a directory with pathID as parent and name leaf
func (f *Fs) CreateDir(ctx context.Context, pathID, leaf string) (newID string, err error) {
	// fs.Debugf(f, "CreateDir(%q, %q)\n", pathID, leaf)
	item, err := f.createDir(ctx, pathID, leaf, time.Now())
	if err != nil {
		return "", err
	}
	return item.ID, nil
}

// list the objects into the function supplied
//
// If directories is set it only sends directories
// User function to process a File item from listAll
//
// Should return true to finish processing
type listAllFn func(*api.Item) bool

// Lists the directory required calling the user function on each item found
//
// If name is set then the server will limit the returned items to those
// with that name.
//
// If the user fn ever returns true then it early exits with found = true
func (f *Fs) listAll(ctx context.Context, dirID string, directoriesOnly bool, filesOnly bool, name string, fn listAllFn) (found bool, err error) {
	opts := rest.Opts{
		Method:     "GET",
		Path:       "/contents/" + dirID,
		Parameters: url.Values{},
	}
	if name != "" {
		opts.Parameters.Add("contentname", f.opt.Enc.FromStandardName(name))
	}
	page := 1
OUTER:
	for {
		opts.Parameters.Set("page", strconv.Itoa(page))
		opts.Parameters.Set("pageSize", strconv.Itoa(f.opt.ListChunk))
		var result api.Contents
		var resp *http.Response
		err = f.pacer.Call(func() (bool, error) {
			resp, err = f.srv.CallJSON(ctx, &opts, nil, &result)
			return shouldRetry(ctx, resp, err)
		})
		if err = result.Err(err); err != nil {
			if isAPIErr(err, "error-notFound") {
				return found, fs.ErrorDirNotFound
			}
			return found, fmt.Errorf("couldn't list files: %w", err)
		}
		for id, item := range result.Data.Children {
			_ = id
			if item.Type == api.ItemTypeFolder {
				if filesOnly {
					continue
				}
			} else if item.Type == api.ItemTypeFile {
				if directoriesOnly {
					continue
				}
			} else {
				fs.Debugf(f, "Ignoring %q - unknown type %q", item.Name, item.Type)
				continue
			}
			item.Name = f.opt.Enc.ToStandardName(item.Name)
			if fn(item) {
				found = true
				break OUTER
			}
		}
		if !result.Metadata.HasNextPage {
			break
		}
		page += 1
	}
	return found, err
}

// Convert a list item into a DirEntry
func (f *Fs) itemToDirEntry(ctx context.Context, remote string, info *api.Item) (entry fs.DirEntry, err error) {
	if info.Type == api.ItemTypeFolder {
		// cache the directory ID for later lookups
		f.dirCache.Put(remote, info.ID)
		d := &Directory{
			Object: Object{
				fs:     f,
				remote: remote,
			},
			items: int64(info.ChildrenCount),
		}
		d.setMetaDataAny(info)
		entry = d
	} else if info.Type == api.ItemTypeFile {
		entry, err = f.newObjectWithInfo(ctx, remote, info)
		if err != nil {
			return nil, err
		}
	}
	return entry, nil
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
	directoryID, err := f.dirCache.FindDir(ctx, dir, false)
	if err != nil {
		return nil, err
	}
	var iErr error
	_, err = f.listAll(ctx, directoryID, false, false, "", func(info *api.Item) bool {
		remote := path.Join(dir, info.Name)
		entry, err := f.itemToDirEntry(ctx, remote, info)
		if err != nil {
			iErr = err
			return true
		}
		entries = append(entries, entry)
		return false
	})
	if err != nil {
		return nil, err
	}
	if iErr != nil {
		return nil, iErr
	}
	return entries, nil
}

// implementation of ListR
func (f *Fs) listR(ctx context.Context, dir string, list *walk.ListRHelper) (err error) {
	directoryID, err := f.dirCache.FindDir(ctx, dir, false)
	if err != nil {
		return err
	}
	opts := rest.Opts{
		Method:     "GET",
		Path:       "/contents/" + directoryID,
		Parameters: url.Values{"maxdepth": {strconv.Itoa(maxDepth)}},
	}
	page := 1
	for {
		opts.Parameters.Set("page", strconv.Itoa(page))
		opts.Parameters.Set("pageSize", strconv.Itoa(f.opt.ListChunk))
		var result api.Contents
		var resp *http.Response
		err = f.pacer.Call(func() (bool, error) {
			resp, err = f.srv.CallJSON(ctx, &opts, nil, &result)
			return shouldRetry(ctx, resp, err)
		})
		if err = result.Err(err); err != nil {
			if isAPIErr(err, "error-notFound") {
				return fs.ErrorDirNotFound
			}
			return fmt.Errorf("couldn't recursively list files: %w", err)
		}
		// Result.Data.Item now contains a recursive listing so we will have to decode recursively
		var decode func(string, *api.Item) error
		decode = func(dir string, dirItem *api.Item) error {
			// If we have ChildrenCount but no Children this means the recursion stopped here
			if dirItem.ChildrenCount > 0 && len(dirItem.Children) == 0 {
				return f.listR(ctx, dir, list)
			}
			for _, item := range dirItem.Children {
				if item.Type != api.ItemTypeFolder && item.Type != api.ItemTypeFile {
					fs.Debugf(f, "Ignoring %q - unknown type %q", item.Name, item.Type)
					continue
				}
				item.Name = f.opt.Enc.ToStandardName(item.Name)
				remote := path.Join(dir, item.Name)
				entry, err := f.itemToDirEntry(ctx, remote, item)
				if err != nil {
					return err
				}
				err = list.Add(entry)
				if err != nil {
					return err
				}
				if item.Type == api.ItemTypeFolder {
					err := decode(remote, item)
					if err != nil {
						return err
					}
				}
			}
			return nil
		}
		err = decode(dir, &result.Data.Item)
		if err != nil {
			return err
		}
		if !result.Metadata.HasNextPage {
			break
		}
		page += 1
	}
	return err
}

// ListR lists the objects and directories of the Fs starting
// from dir recursively into out.
//
// dir should be "" to start from the root, and should not
// have trailing slashes.
//
// This should return ErrDirNotFound if the directory isn't
// found.
//
// It should call callback for each tranche of entries read.
// These need not be returned in any particular order.  If
// callback returns an error then the listing will stop
// immediately.
//
// Don't implement this unless you have a more efficient way
// of listing recursively than doing a directory traversal.
func (f *Fs) ListR(ctx context.Context, dir string, callback fs.ListRCallback) (err error) {
	list := walk.NewListRHelper(callback)
	err = f.listR(ctx, dir, list)
	if err != nil {
		return err
	}
	return list.Flush()
}

// Creates from the parameters passed in a half finished Object which
// must have setMetaData called on it
//
// Returns the object, leaf, directoryID and error.
//
// Used to create new objects
func (f *Fs) createObject(ctx context.Context, remote string, modTime time.Time, size int64) (o *Object, leaf string, directoryID string, err error) {
	// Create the directory for the object if it doesn't exist
	leaf, directoryID, err = f.dirCache.FindPath(ctx, remote, true)
	if err != nil {
		return
	}
	// Temporary Object under construction
	o = &Object{
		fs:     f,
		remote: remote,
	}
	return o, leaf, directoryID, nil
}

// Put the object
//
// Copy the reader in to the new object which is returned.
//
// The new object may have been created if an error is returned
func (f *Fs) Put(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error) {
	existingObj, err := f.NewObject(ctx, src.Remote())
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

// PutStream uploads to the remote path with the modTime given of indeterminate size
func (f *Fs) PutStream(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error) {
	return f.Put(ctx, in, src, options...)
}

// PutUnchecked the object into the container
//
// This will produce a duplicate if the object already exists.
//
// Copy the reader in to the new object which is returned.
//
// The new object may have been created if an error is returned
func (f *Fs) PutUnchecked(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error) {
	remote := src.Remote()
	size := src.Size()
	modTime := src.ModTime(ctx)

	o, _, _, err := f.createObject(ctx, remote, modTime, size)
	if err != nil {
		return nil, err
	}
	return o, o.Update(ctx, in, src, options...)
}

// Mkdir creates the container if it doesn't exist
func (f *Fs) Mkdir(ctx context.Context, dir string) error {
	_, err := f.dirCache.FindDir(ctx, dir, true)
	return err
}

// DirSetModTime sets the directory modtime for dir
func (f *Fs) DirSetModTime(ctx context.Context, dir string, modTime time.Time) error {
	dirID, err := f.dirCache.FindDir(ctx, dir, false)
	if err != nil {
		return err
	}
	_, err = f.setModTime(ctx, dirID, modTime)
	return err
}

// deleteObject removes an object by ID
func (f *Fs) deleteObject(ctx context.Context, id string) error {
	opts := rest.Opts{
		Method: "DELETE",
		Path:   "/contents/",
	}
	request := api.DeleteRequest{
		ContentsID: id,
	}
	var result api.DeleteResponse
	err := f.pacer.Call(func() (bool, error) {
		resp, err := f.srv.CallJSON(ctx, &opts, &request, &result)
		return shouldRetry(ctx, resp, err)
	})
	if err = result.Err(err); err != nil {
		return fmt.Errorf("failed to delete item: %w", err)
	}
	// Check the individual result codes also
	for _, err := range result.Data {
		if err.IsError() {
			return fmt.Errorf("failed to delete item: %w", err)
		}
	}
	return nil
}

// purgeCheck removes the root directory, if check is set then it
// refuses to do so if it has anything in
func (f *Fs) purgeCheck(ctx context.Context, dir string, check bool) error {
	root := path.Join(f.root, dir)
	if root == "" {
		return errors.New("can't purge root directory")
	}
	dc := f.dirCache
	rootID, err := dc.FindDir(ctx, dir, false)
	if err != nil {
		return err
	}

	// Check to see if there is contents in the directory
	if check {
		found, err := f.listAll(ctx, rootID, false, false, "", func(item *api.Item) bool {
			return true
		})
		if err != nil {
			return err
		}
		if found {
			return fs.ErrorDirectoryNotEmpty
		}
	}

	// Delete the directory
	err = f.deleteObject(ctx, rootID)
	if err != nil {
		return err
	}

	f.dirCache.FlushDir(dir)
	return nil
}

// Rmdir deletes the root folder
//
// Returns an error if it isn't empty
func (f *Fs) Rmdir(ctx context.Context, dir string) error {
	return f.purgeCheck(ctx, dir, true)
}

// Precision return the precision of this Fs
func (f *Fs) Precision() time.Duration {
	return time.Second
}

// Purge deletes all the files and the container
//
// Optional interface: Only implement this if you have a way of
// deleting all the files quicker than just running Remove() on the
// result of List()
func (f *Fs) Purge(ctx context.Context, dir string) error {
	return f.purgeCheck(ctx, dir, false)
}

// About gets quota information
func (f *Fs) About(ctx context.Context) (usage *fs.Usage, err error) {
	result, err := f.getAccounts(ctx)
	if err != nil {
		return nil, err
	}
	used := result.Data.StatsCurrent.Storage
	files := result.Data.StatsCurrent.FileCount
	total := result.Data.SubscriptionLimitStorage
	usage = &fs.Usage{
		Used:    fs.NewUsageValue(used),         // bytes in use
		Total:   fs.NewUsageValue(total),        // bytes total
		Free:    fs.NewUsageValue(total - used), // bytes free
		Objects: fs.NewUsageValue(files),        // total objects
	}
	return usage, nil
}

// patch an attribute on an object to value
func (f *Fs) patch(ctx context.Context, id, attribute string, value any) (item *api.Item, err error) {
	var resp *http.Response
	var request = api.UpdateItemRequest{
		Attribute: attribute,
		Value:     value,
	}
	var result api.UpdateItemResponse
	opts := rest.Opts{
		Method: "PUT",
		Path:   "/contents/" + id + "/update",
	}
	err = f.pacer.Call(func() (bool, error) {
		resp, err = f.srv.CallJSON(ctx, &opts, &request, &result)
		return shouldRetry(ctx, resp, err)
	})
	if err = result.Err(err); err != nil {
		return nil, fmt.Errorf("failed to patch item %q to %v: %w", attribute, value, err)
	}
	return &result.Data, nil
}

// rename a file or a folder
func (f *Fs) rename(ctx context.Context, id, newLeaf string) (item *api.Item, err error) {
	return f.patch(ctx, id, "name", f.opt.Enc.FromStandardName(newLeaf))
}

// setModTime sets the modification time of a file or folder
func (f *Fs) setModTime(ctx context.Context, id string, modTime time.Time) (item *api.Item, err error) {
	return f.patch(ctx, id, "modTime", api.ToNativeTime(modTime))
}

// move a file or a folder to a new directory
func (f *Fs) move(ctx context.Context, id, newDirID string) (item *api.Item, err error) {
	var resp *http.Response
	var request = api.MoveRequest{
		FolderID:   newDirID,
		ContentsID: id,
	}
	var result api.MoveResponse
	opts := rest.Opts{
		Method: "PUT",
		Path:   "/contents/move",
	}
	err = f.pacer.Call(func() (bool, error) {
		resp, err = f.srv.CallJSON(ctx, &opts, &request, &result)
		return shouldRetry(ctx, resp, err)
	})
	if err = result.Err(err); err != nil {
		return nil, fmt.Errorf("failed to move item: %w", err)
	}
	itemResult, ok := result.Data[id]
	if !ok || itemResult.Item.ID == "" {
		return nil, errors.New("failed to read result of move")
	}
	return &itemResult.Item, nil
}

// move and rename a file or folder to directoryID with leaf
func (f *Fs) moveTo(ctx context.Context, id, srcLeaf, dstLeaf, srcDirectoryID, dstDirectoryID string) (info *api.Item, err error) {
	// Can have duplicates so don't have to be careful here

	// Rename if required
	if srcLeaf != dstLeaf {
		info, err = f.rename(ctx, id, dstLeaf)
		if err != nil {
			return nil, err
		}
	}
	// Move if required
	if srcDirectoryID != dstDirectoryID {
		info, err = f.move(ctx, id, dstDirectoryID)
		if err != nil {
			return nil, err
		}
	}
	if info == nil {
		return f.readMetaDataForID(ctx, id)
	}
	return info, nil
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

	// Find existing object
	srcLeaf, srcDirectoryID, err := srcObj.fs.dirCache.FindPath(ctx, srcObj.remote, false)
	if err != nil {
		return nil, err
	}

	// Create temporary object
	dstObj, dstLeaf, dstDirectoryID, err := f.createObject(ctx, remote, srcObj.modTime, srcObj.size)
	if err != nil {
		return nil, err
	}

	// Do the move
	info, err := f.moveTo(ctx, srcObj.id, srcLeaf, dstLeaf, srcDirectoryID, dstDirectoryID)
	if err != nil {
		return nil, err
	}

	err = dstObj.setMetaData(info)
	if err != nil {
		return nil, err
	}
	return dstObj, nil
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

	srcID, srcDirectoryID, srcLeaf, dstDirectoryID, dstLeaf, err := f.dirCache.DirMove(ctx, srcFs.dirCache, srcFs.root, srcRemote, f.root, dstRemote)
	if err != nil {
		return err
	}

	// Do the move
	_, err = f.moveTo(ctx, srcID, srcLeaf, dstLeaf, srcDirectoryID, dstDirectoryID)
	if err != nil {
		return err
	}
	srcFs.dirCache.FlushDir(srcRemote)
	return nil
}

// copy a file or a folder to a new directory
func (f *Fs) copy(ctx context.Context, id, newDirID string) (item *api.Item, err error) {
	var resp *http.Response
	var request = api.CopyRequest{
		FolderID:   newDirID,
		ContentsID: id,
	}
	var result api.CopyResponse
	opts := rest.Opts{
		Method: "POST",
		Path:   "/contents/copy",
	}
	err = f.pacer.Call(func() (bool, error) {
		resp, err = f.srv.CallJSON(ctx, &opts, &request, &result)
		return shouldRetry(ctx, resp, err)
	})
	if err = result.Err(err); err != nil {
		return nil, fmt.Errorf("failed to copy item: %w", err)
	}
	itemResult, ok := result.Data[id]
	if !ok || itemResult.Item.ID == "" {
		return nil, errors.New("failed to read result of copy")
	}
	return &itemResult.Item, nil
}

// copy and rename a file or folder to directoryID with leaf
func (f *Fs) copyTo(ctx context.Context, srcID, srcLeaf, dstLeaf, dstDirectoryID string) (info *api.Item, err error) {
	// Can have duplicates so don't have to be careful here

	// Copy to dstDirectoryID first
	info, err = f.copy(ctx, srcID, dstDirectoryID)
	if err != nil {
		return nil, err
	}

	// Rename if required
	if srcLeaf != dstLeaf {
		info, err = f.rename(ctx, info.ID, dstLeaf)
		if err != nil {
			return nil, err
		}
	}
	return info, nil
}

// Copy src to this remote using server-side copy operations.
//
// This is stored with the remote path given.
//
// It returns the destination Object and a possible error.
//
// Will only be called if src.Fs().Name() == f.Name()
//
// If it isn't possible then return fs.ErrorCantCopy
func (f *Fs) Copy(ctx context.Context, src fs.Object, remote string) (fs.Object, error) {
	srcObj, ok := src.(*Object)
	if !ok {
		fs.Debugf(src, "Can't copy - not same remote type")
		return nil, fs.ErrorCantCopy
	}
	srcLeaf := path.Base(srcObj.remote)

	srcPath := srcObj.fs.rootSlash() + srcObj.remote
	dstPath := f.rootSlash() + remote
	if srcPath == dstPath {
		return nil, fmt.Errorf("can't copy %q -> %q as are same name", srcPath, dstPath)
	}

	// Create temporary object
	dstObj, dstLeaf, dstDirectoryID, err := f.createObject(ctx, remote, srcObj.modTime, srcObj.size)
	if err != nil {
		return nil, err
	}

	// Copy the object
	info, err := f.copyTo(ctx, srcObj.id, srcLeaf, dstLeaf, dstDirectoryID)
	if err != nil {
		return nil, err
	}
	err = dstObj.setMetaData(info)
	if err != nil {
		return nil, err
	}

	// Reset the modification time as copy does not preserve it
	err = dstObj.SetModTime(ctx, srcObj.modTime)
	if err != nil {
		return nil, err
	}

	return dstObj, nil
}

// unLink a file or directory
func (f *Fs) unLink(ctx context.Context, remote string, id string, info *api.Item) (err error) {
	if info == nil {
		info, err = f.readMetaDataForID(ctx, id)
		if err != nil {
			return err
		}
	}
	for linkID, link := range info.DirectLinks {
		fs.Debugf(remote, "Removing direct link %s", link.DirectLink)
		opts := rest.Opts{
			Method: "DELETE",
			Path:   "/contents/" + id + "/directlinks/" + linkID,
		}
		var result api.Error
		err := f.pacer.Call(func() (bool, error) {
			resp, err := f.srv.CallJSON(ctx, &opts, nil, &result)
			return shouldRetry(ctx, resp, err)
		})
		if err = result.Err(err); err != nil {
			return fmt.Errorf("failed to unlink: %s", link.DirectLink)
		}
	}
	return nil
}

// PublicLink adds a "readable by anyone with link" permission on the given file or folder.
func (f *Fs) PublicLink(ctx context.Context, remote string, expire fs.Duration, unlink bool) (string, error) {
	id, err := f.dirCache.FindDir(ctx, remote, false)
	var info *api.Item
	if err == nil {
		fs.Debugf(f, "attempting to share directory '%s'", remote)
	} else {
		fs.Debugf(f, "attempting to share single file '%s'", remote)
		info, err = f.readMetaDataForPath(ctx, remote)
		if err != nil {
			return "", err
		}
		id = info.ID
	}
	if unlink {
		return "", f.unLink(ctx, remote, id, info)
	}
	var resp *http.Response
	var request api.DirectLinksRequest
	var result api.DirectLinksResult
	opts := rest.Opts{
		Method: "POST",
		Path:   "/contents/" + id + "/directlinks",
	}
	if expire != fs.DurationOff {
		when := time.Now().Add(time.Duration(expire))
		fs.Debugf(f, "Link expires at %v (duration %v)", when, expire)
		request.ExpireTime = api.ToNativeTime(when)
	}
	err = f.pacer.Call(func() (bool, error) {
		resp, err = f.srv.CallJSON(ctx, &opts, &request, &result)
		return shouldRetry(ctx, resp, err)
	})
	if err = result.Err(err); err != nil {
		return "", fmt.Errorf("failed to create direct link: %w", err)
	}
	return result.Data.DirectLink, err
}

// DirCacheFlush resets the directory cache - used in testing as an
// optional interface
func (f *Fs) DirCacheFlush() {
	f.dirCache.ResetRoot()
}

// Hashes returns the supported hash sets.
func (f *Fs) Hashes() hash.Set {
	return hash.Set(hash.MD5)
}

// MergeDirs merges the contents of all the directories passed
// in into the first one and rmdirs the other directories.
func (f *Fs) MergeDirs(ctx context.Context, dirs []fs.Directory) error {
	if len(dirs) < 2 {
		return nil
	}
	dstDir := dirs[0]
	for _, srcDir := range dirs[1:] {
		// list the objects
		infos := []*api.Item{}
		_, err := f.listAll(ctx, srcDir.ID(), false, false, "", func(info *api.Item) bool {
			infos = append(infos, info)
			return false
		})
		if err != nil {
			return fmt.Errorf("MergeDirs list failed on %v: %w", srcDir, err)
		}
		// move them into place
		for _, info := range infos {
			fs.Infof(srcDir, "merging %q", info.Name)
			// Move the file into the destination
			_, err = f.move(ctx, info.ID, dstDir.ID())
			if err != nil {
				return fmt.Errorf("MergeDirs move failed on %q in %v: %w", info.Name, srcDir, err)
			}
		}
		// rmdir the now empty source directory
		fs.Infof(srcDir, "removing empty directory")
		err = f.deleteObject(ctx, srcDir.ID())
		if err != nil {
			return fmt.Errorf("MergeDirs move failed to rmdir %q: %w", srcDir, err)
		}
	}
	return nil
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
	if t != hash.MD5 {
		return "", hash.ErrUnsupported
	}
	return o.md5, nil
}

// Size returns the size of an object in bytes
func (o *Object) Size() int64 {
	return o.size
}

// setMetaDataAny sets the metadata from info but doesn't check the type
func (o *Object) setMetaDataAny(info *api.Item) {
	o.size = info.Size
	o.modTime = api.FromNativeTime(info.ModTime)
	o.id = info.ID
	o.dirID = info.ParentFolder
	o.mimeType = info.MimeType
	o.md5 = info.MD5
	o.url = info.Link
}

// setMetaData sets the metadata from info
func (o *Object) setMetaData(info *api.Item) (err error) {
	if info.Type == api.ItemTypeFolder {
		return fs.ErrorIsDir
	}
	if info.Type != api.ItemTypeFile {
		return fmt.Errorf("%q is %q: %w", o.remote, info.Type, fs.ErrorNotAFile)
	}
	if info.ID == "" {
		return fmt.Errorf("ID not found in response")
	}
	o.setMetaDataAny(info)
	return nil
}

// readMetaData gets the metadata unconditionally as we expect Object
// to always have the full set of metadata
func (o *Object) readMetaData(ctx context.Context) (err error) {
	var info *api.Item
	if o.id != "" {
		info, err = o.fs.readMetaDataForID(ctx, o.id)
	} else {
		info, err = o.fs.readMetaDataForPath(ctx, o.remote)
	}
	if err != nil {
		if isAPIErr(err, "error-notFound") {
			return fs.ErrorObjectNotFound
		}
		return err
	}
	return o.setMetaData(info)
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
	info, err := o.fs.setModTime(ctx, o.id, modTime)
	if err != nil {
		return err
	}
	return o.setMetaData(info)
}

// Storable returns a boolean showing whether this object storable
func (o *Object) Storable() bool {
	return true
}

// Open an object for read
func (o *Object) Open(ctx context.Context, options ...fs.OpenOption) (in io.ReadCloser, err error) {
	if o.id == "" {
		return nil, errors.New("can't download - no id")
	}
	if o.url == "" {
		// On upload an Object is returned with no url, so fetch it here if needed
		err = o.readMetaData(ctx)
		if err != nil {
			return nil, fmt.Errorf("read metadata: %w", err)
		}
	}
	fs.FixRangeOption(options, o.size)
	var resp *http.Response
	opts := rest.Opts{
		Method:  "GET",
		RootURL: o.url,
		Options: options,
		// Workaround for bug in content servers - no longer needed
		// ExtraHeaders: map[string]string{
		// 	"Cookie": "accountToken=" + o.fs.opt.AccessToken,
		// },
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
// The new object may have been created if an error is returned.
func (o *Object) Update(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (err error) {
	remote := o.Remote()
	modTime := src.ModTime(ctx)

	// Create the directory for the object if it doesn't exist
	leaf, directoryID, err := o.fs.dirCache.FindPath(ctx, remote, true)
	if err != nil {
		return err
	}

	// Find an upload server
	server, err := o.fs.getServer(ctx)
	if err != nil {
		return err
	}
	fs.Debugf(o, "Using upload server %v", server)

	// If the file exists, delete it after a successful upload
	if o.id != "" {
		id := o.id
		o.id = ""
		defer func() {
			if err != nil {
				return
			}
			fs.Debugf(o, "Removing old object on successful upload")
			deleteErr := o.fs.deleteObject(ctx, id)
			if deleteErr != nil {
				err = fmt.Errorf("failed to delete existing object: %w", deleteErr)
			}
		}()
	}

	// Do the upload
	var resp *http.Response
	var result api.UploadResponse
	opts := rest.Opts{
		Method: "POST",
		Body:   in,
		MultipartParams: url.Values{
			"folderId": {directoryID},
			"modTime":  {strconv.FormatInt(modTime.Unix(), 10)},
		},
		MultipartContentName: "file",
		MultipartFileName:    o.fs.opt.Enc.FromStandardName(leaf),
		RootURL:              server.URL(),
		Options:              options,
	}
	err = o.fs.pacer.CallNoRetry(func() (bool, error) {
		resp, err = o.fs.srv.CallJSON(ctx, &opts, nil, &result)
		return shouldRetry(ctx, resp, err)
	})
	if err = result.Err(err); err != nil {
		if isAPIErr(err, "error-freespace") {
			fs.Errorf(o, "Upload server out of space - need to retry upload")
		}
		o.fs.clearServers()
		return fmt.Errorf("failed to upload file: %w", err)
	}
	return o.setMetaData(&result.Data)
}

// Remove an object
func (o *Object) Remove(ctx context.Context) error {
	return o.fs.deleteObject(ctx, o.id)
}

// ID returns the ID of the Object if known, or "" if not
func (o *Object) ID() string {
	return o.id
}

// MimeType returns the content type of the Object if known, or "" if not
func (o *Object) MimeType(ctx context.Context) string {
	return o.mimeType
}

// ParentID returns the ID of the Object parent if known, or "" if not
func (o *Object) ParentID() string {
	return o.dirID
}

// setMetaData sets the metadata from info for a directory
func (d *Directory) setMetaData(info *api.Item) (err error) {
	if info.Type == api.ItemTypeFile {
		return fs.ErrorIsFile
	}
	if info.Type != api.ItemTypeFolder {
		return fmt.Errorf("%q is not a directory (type %q)", d.remote, info.Type)
	}
	if info.ID == "" {
		return fmt.Errorf("ID not found in response")
	}
	d.setMetaDataAny(info)
	return nil
}

// SetModTime sets the modification time of the directory
func (d *Directory) SetModTime(ctx context.Context, modTime time.Time) error {
	info, err := d.fs.setModTime(ctx, d.id, modTime)
	if err != nil {
		return err
	}
	return d.setMetaData(info)
}

// Items returns the count of items in this directory or this
// directory and subdirectories if known, -1 for unknown
func (d *Directory) Items() int64 {
	return d.items
}

// Hash does nothing on a directory
//
// This method is implemented with the incorrect type signature to
// stop the Directory type asserting to fs.Object or fs.ObjectInfo
func (d *Directory) Hash() {
	// Does nothing
}

// Check the interfaces are satisfied
var (
	_ fs.Fs              = (*Fs)(nil)
	_ fs.Purger          = (*Fs)(nil)
	_ fs.PutStreamer     = (*Fs)(nil)
	_ fs.Copier          = (*Fs)(nil)
	_ fs.Abouter         = (*Fs)(nil)
	_ fs.Mover           = (*Fs)(nil)
	_ fs.DirMover        = (*Fs)(nil)
	_ fs.DirCacheFlusher = (*Fs)(nil)
	_ fs.PublicLinker    = (*Fs)(nil)
	_ fs.MergeDirser     = (*Fs)(nil)
	_ fs.DirSetModTimer  = (*Fs)(nil)
	_ fs.ListRer         = (*Fs)(nil)
	_ fs.Object          = (*Object)(nil)
	_ fs.IDer            = (*Object)(nil)
	_ fs.MimeTyper       = (*Object)(nil)
	_ fs.Directory       = (*Directory)(nil)
	_ fs.SetModTimer     = (*Directory)(nil)
	_ fs.ParentIDer      = (*Directory)(nil)
)
