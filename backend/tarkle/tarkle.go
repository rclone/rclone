// Package tarkle provides an interface to the Tarkle file sharing
// service through its public files API.
package tarkle

/*
Notes

- Files are identified by digit strings, folders by uuids. The library
  root has no id: it is represented by the empty string.
- ModTime is the entry's modified_at (the uploader's own mtime if set,
  else updated_at), set on upload and by SetModTime. The only hash is
  SHA-256 (sha256_source says if the server verified it or it is the
  uploader's unverified claim).
- Uploads go straight to presigned storage URLs, never through the API
  host. The server picks the part size, so ChunkSize is not an option.
*/

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strings"
	"sync"
	"time"

	"github.com/rclone/rclone/backend/tarkle/api"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config"
	"github.com/rclone/rclone/fs/config/configmap"
	"github.com/rclone/rclone/fs/config/configstruct"
	"github.com/rclone/rclone/fs/config/obscure"
	"github.com/rclone/rclone/fs/fserrors"
	"github.com/rclone/rclone/fs/fshttp"
	"github.com/rclone/rclone/fs/hash"
	"github.com/rclone/rclone/lib/dircache"
	"github.com/rclone/rclone/lib/encoder"
	"github.com/rclone/rclone/lib/multipart"
	"github.com/rclone/rclone/lib/pacer"
	"github.com/rclone/rclone/lib/rest"
)

const (
	minSleep      = 10 * time.Millisecond
	maxSleep      = 20 * time.Second
	decayConstant = 1 // bigger for slower decay, exponential
	defaultURL    = "https://app.tarkle.com"
	apiPath       = "/api/v1"
	// a download link lives 900 s; ask for a new one a little earlier
	downloadLinkLife = 10 * time.Minute
)

var retryErrorCodes = []int{
	408, // Request Timeout
	429, // Too Many Requests
	500, // Internal Server Error
	502, // Bad Gateway
	503, // Service Unavailable
	504, // Gateway Timeout
}

// Register with Fs
func init() {
	fs.Register(&fs.RegInfo{
		Name:        "tarkle",
		Description: "Tarkle",
		NewFs:       NewFs,
		Options: []fs.Option{{
			Name: "token",
			Help: `API token.

Create one in the Tarkle workspace settings with the scopes files:read
and files:write. A token acts as the person who created it.`,
			Required:   true,
			IsPassword: true,
			Sensitive:  true,
		}, {
			Name: "workspace",
			Help: `Workspace key (8 characters, as listed by GET /workspaces).

An account-wide token works for every workspace its creator can access
so this is required: it is sent as X-Workspace-Key on every call (a workspace id is not accepted).
If it is blank, rclone lists the workspace keys and names to choose from.`,
			Sensitive: true,
		}, {
			Name:     "url",
			Help:     "URL of the Tarkle app (the API is served from <url>/api/v1).",
			Default:  defaultURL,
			Advanced: true,
		}, {
			Name:     "list_chunk",
			Help:     "Number of items to list in each call (1-1000).",
			Default:  1000,
			Advanced: true,
		}, {
			Name: "upload_concurrency",
			Help: `Number of parts of one file uploaded at the same time.

Parts are buffered in memory: up to the part size chosen by the server
(see the docs) times this many per transferred file.`,
			Default:  4,
			Advanced: true,
		}, {
			Name: "delete_transfers",
			Help: `Also delete the transfers that contain a file or folder being deleted.

Without this, deleting an item that is in an active transfer fails with
a message listing the transfers. Feedback boards and paid transfers
with purchases can never be deleted through the API.`,
			Default:  false,
			Advanced: true,
		}, {
			Name:     "disable_listr",
			Help:     "Don't use the recursive listing (GET /files?recursive=true).",
			Default:  false,
			Advanced: true,
		}, {
			Name:     config.ConfigEncoding,
			Help:     config.ConfigEncodingHelp,
			Advanced: true,
			Default: (encoder.Display |
				encoder.EncodeLeftSpace |
				encoder.EncodeRightSpace |
				encoder.EncodeBackSlash |
				encoder.EncodeInvalidUtf8),
		}},
		MetadataInfo: &fs.MetadataInfo{
			System: map[string]fs.MetadataHelp{
				"mtime": {
					Help:    "Time of last modification (the uploader's own time if it was set)",
					Type:    "RFC 3339",
					Example: "2006-01-02T15:04:05.999999999Z07:00",
				},
				"btime": {
					Help:    "Time of file birth (creation on Tarkle)",
					Type:    "RFC 3339",
					Example: "2006-01-02T15:04:05.999999999Z07:00",
				},
			},
			Help: "Tarkle reports modification and creation times. Modification time is set through the upload and `rclone touch`; creation time is read only.",
		},
	})
}

// Options defines the configuration for this backend
type Options struct {
	Token             string               `config:"token"`
	Workspace         string               `config:"workspace"`
	URL               string               `config:"url"`
	ListChunk         int                  `config:"list_chunk"`
	UploadConcurrency int                  `config:"upload_concurrency"`
	DeleteTransfers   bool                 `config:"delete_transfers"`
	DisableListR      bool                 `config:"disable_listr"`
	Enc               encoder.MultiEncoder `config:"encoding"`
}

// Fs represents a remote tarkle
type Fs struct {
	name       string             // name of this remote
	root       string             // the path we are working on
	opt        Options            // parsed options
	features   *fs.Features       // optional features
	srv        *rest.Client       // the API, sends the token
	store      *rest.Client       // presigned storage URLs: never sends the token
	dirCache   *dircache.DirCache // Map of directory path to directory id
	pacer      *fs.Pacer          // pacer for API calls
	storePacer *fs.Pacer          // pacer for storage calls
}

// Object describes a tarkle object
type Object struct {
	fs      *Fs       // what this object is part of
	remote  string    // The remote path
	size    int64     // size of the object
	modTime time.Time // modified_at of the object
	created time.Time // created_at of the object, zero if unknown
	id      string    // ID of the object
	dirID   string    // ID of the object's directory

	sha256       string // "" if unknown
	sha256Source string // "server" (verified) or "client" (the uploader's claim)

	linkMu     sync.Mutex
	link       string    // cached presigned download URL
	linkExpiry time.Time // when to stop using it
}

// ------------------------------------------------------------

// Name of the remote (as passed into NewFs)
func (f *Fs) Name() string { return f.name }

// Root of the remote (as passed into NewFs)
func (f *Fs) Root() string { return f.root }

// String converts this Fs to a string
func (f *Fs) String() string { return fmt.Sprintf("Tarkle root '%s'", f.root) }

// Features returns the optional features of this Fs
func (f *Fs) Features() *fs.Features { return f.features }

// Precision of the modification times (seconds are kept, milliseconds are sent)
func (f *Fs) Precision() time.Duration { return time.Second }

// Hashes returns the supported hash sets: SHA-256
func (f *Fs) Hashes() hash.Set { return hash.Set(hash.SHA256) }

// formatTime formats a time the way the API takes modified_at
func formatTime(t time.Time) string { return t.UTC().Format("2006-01-02T15:04:05.000Z") }

// parsePath parses a tarkle 'url'
func parsePath(p string) string { return strings.Trim(p, "/") }

// shouldRetry returns a boolean as to whether this resp and err
// deserve to be retried.  It returns the err as a convenience
func shouldRetry(ctx context.Context, resp *http.Response, err error) (bool, error) {
	if fserrors.ContextError(ctx, &err) {
		return false, err
	}
	if resp != nil && resp.StatusCode == http.StatusTooManyRequests && err != nil {
		if secs, perr := time.ParseDuration(resp.Header.Get("Retry-After") + "s"); perr == nil && secs > 0 {
			return true, pacer.RetryAfterError(err, secs)
		}
	}
	return fserrors.ShouldRetry(err) || fserrors.ShouldRetryHTTP(resp, retryErrorCodes), err
}

// errorHandler parses a non 2xx error response into an *api.Error
func errorHandler(resp *http.Response) error {
	body, err := rest.ReadBody(resp)
	if err != nil {
		fs.Debugf(nil, "Couldn't read error out of body: %v", err)
		body = nil
	}
	e := &api.Error{Status: resp.StatusCode, Body: string(body)}
	if len(body) > 0 {
		if err := json.Unmarshal(body, e); err != nil {
			fs.Debugf(nil, "Couldn't decode error response: %v", err)
		}
	}
	return e
}

// apiError returns the *api.Error inside err, or nil
func apiError(err error) *api.Error {
	var e *api.Error
	if errors.As(err, &e) {
		return e
	}
	return nil
}

// NewFs constructs an Fs from the path, container:path
func NewFs(ctx context.Context, name, root string, m configmap.Mapper) (fs.Fs, error) {
	opt := new(Options)
	err := configstruct.Set(m, opt)
	if err != nil {
		return nil, err
	}
	if opt.ListChunk < 1 || opt.ListChunk > 1000 {
		return nil, errors.New("tarkle: list_chunk must be from 1 to 1000")
	}
	if opt.UploadConcurrency < 1 {
		return nil, errors.New("tarkle: upload_concurrency must be at least 1")
	}
	token, err := obscure.Reveal(opt.Token)
	if err != nil {
		return nil, fmt.Errorf("tarkle: couldn't decrypt token: %w", err)
	}
	if token == "" {
		return nil, errors.New("tarkle: no API token set")
	}
	root = parsePath(root)

	client := fshttp.NewClient(ctx)
	f := &Fs{
		name:       name,
		root:       root,
		opt:        *opt,
		srv:        rest.NewClient(client).SetRoot(strings.TrimRight(opt.URL, "/") + apiPath),
		store:      rest.NewClient(client),
		pacer:      fs.NewPacer(ctx, pacer.NewDefault(pacer.MinSleep(minSleep), pacer.MaxSleep(maxSleep), pacer.DecayConstant(decayConstant))),
		storePacer: fs.NewPacer(ctx, pacer.NewDefault(pacer.MinSleep(minSleep), pacer.MaxSleep(maxSleep), pacer.DecayConstant(decayConstant))),
	}
	f.srv.SetErrorHandler(errorHandler)
	f.srv.SetHeader("Authorization", "Bearer "+token)
	f.srv.SetHeader("Accept", "application/json")
	if opt.Workspace != "" {
		f.srv.SetHeader("X-Workspace-Key", opt.Workspace) // the API host only: f.store never gets it
	} else if err := f.checkWorkspaceNeeded(ctx); err != nil {
		return nil, err
	}
	f.features = (&fs.Features{
		CanHaveEmptyDirectories: true,
		CaseInsensitive:         true, // a file and a folder share one namespace, ignoring case
		ReadMetadata:            true, // mtime and btime
	}).Fill(ctx, f)
	if opt.DisableListR {
		f.features.ListR = nil
	}

	f.dirCache = dircache.New(root, "", f)
	err = f.dirCache.FindRoot(ctx, false)
	if err != nil {
		// Assume it is a file
		newRoot, remote := dircache.SplitPath(root)
		tempF := *f
		tempF.dirCache = dircache.New(newRoot, "", &tempF)
		tempF.root = newRoot
		err = tempF.dirCache.FindRoot(ctx, false)
		if err != nil {
			// No root so return old f
			return f, nil
		}
		_, err := tempF.newObjectWithInfo(ctx, remote, nil)
		if err != nil {
			if err == fs.ErrorObjectNotFound {
				return f, nil
			}
			return nil, err
		}
		f.features.Fill(ctx, &tempF)
		f.dirCache = tempF.dirCache
		f.root = tempF.root
		return f, fs.ErrorIsFile
	}
	return f, nil
}

// ------------------------------------------------------------

// call does an API call with the pacer and decodes the JSON answer into out
func (f *Fs) call(ctx context.Context, opts *rest.Opts, in, out any) error {
	err := f.pacer.Call(func() (bool, error) {
		resp, err := f.srv.CallJSON(ctx, opts, in, out)
		return shouldRetry(ctx, resp, err)
	})
	return workspaceError(err)
}

// workspaceError turns the API's workspace refusals into clear, permanent errors
func workspaceError(err error) error {
	e := apiError(err)
	if e == nil {
		return err
	}
	switch e.Code() {
	case "workspace_key_required":
		return fserrors.FatalError(fmt.Errorf("the workspace option must be the workspace key (8 characters), not the workspace id: %w", err))
	case "workspace_required":
		return fserrors.FatalError(fmt.Errorf("this token works for every workspace: set the workspace option (--tarkle-workspace, config key workspace) to a workspace key: %w", err))
	case "invalid_workspace":
		return fserrors.FatalError(fmt.Errorf("the workspace option is not a valid workspace key (8 letters and digits): %w", err))
	case "no_access":
		return fserrors.FatalError(fmt.Errorf("the token's owner cannot access the workspace named in the workspace option, or it does not exist: %w", err))
	case "legacy_token":
		return fserrors.FatalError(fmt.Errorf("this token is a retired workspace token and no longer works: create a new token in Settings > API: %w", err))
	}
	return err
}

// checkWorkspaceNeeded is called when no workspace is set: every token needs
// one, so say which are available (none is picked for you).
func (f *Fs) checkWorkspaceNeeded(ctx context.Context) error {
	var res api.Workspaces
	opts := rest.Opts{Method: "GET", Path: "/workspaces"}
	if err := f.call(ctx, &opts, nil, &res); err != nil {
		if fserrors.IsFatalError(err) {
			return err // a retired token and the like: say so
		}
		return fserrors.FatalError(fmt.Errorf("tarkle: the workspace option is required (set it to the workspace key) and the list of workspaces could not be read: %w", err))
	}
	return fserrors.FatalError(errors.New(workspaceListMessage(&res)))
}

func workspaceListMessage(res *api.Workspaces) string {
	var b strings.Builder
	b.WriteString("tarkle: the workspace option is required: a token names the workspace on every call. Set it to one of these workspace keys:")
	for _, w := range res.Workspaces {
		fmt.Fprintf(&b, "\n  %s  %s", w.Key, w.Name)
	}
	if len(res.Workspaces) == 0 {
		b.WriteString("\n  (none: the token's owner has no workspace)")
	}
	return b.String()
}

// listAll lists the entries of dirID. If recursive is set it lists all
// the descendants of type kind ("file" or "folder") instead. fn returns
// true to stop. Entries are decoded to standard names.
func (f *Fs) listAll(ctx context.Context, dirID string, kind string, recursive bool, fn func(*api.Item) bool) (found bool, err error) {
	opts := rest.Opts{
		Method:     "GET",
		Path:       "/files",
		Parameters: url.Values{},
	}
	if dirID != "" {
		opts.Parameters.Set("folder", dirID)
	}
	if recursive {
		opts.Parameters.Set("recursive", "true")
	}
	if kind != "" {
		opts.Parameters.Set("type", kind)
	}
	opts.Parameters.Set("limit", fmt.Sprint(f.opt.ListChunk))
	for {
		var result api.Listing
		err = f.call(ctx, &opts, nil, &result)
		if err != nil {
			if e := apiError(err); e != nil && e.Status == http.StatusNotFound {
				return found, fs.ErrorDirNotFound
			}
			return found, fmt.Errorf("couldn't list files: %w", err)
		}
		for i := range result.Entries {
			item := &result.Entries[i]
			if kind != "" && item.Type != kind {
				continue
			}
			item.Name = f.opt.Enc.ToStandardName(item.Name)
			if fn(item) {
				return true, nil
			}
		}
		if result.NextCursor == nil || *result.NextCursor == "" {
			return found, nil
		}
		opts.Parameters.Set("cursor", *result.NextCursor)
	}
}

// FindLeaf finds a directory of name leaf in the folder with ID pathID
func (f *Fs) FindLeaf(ctx context.Context, pathID, leaf string) (pathIDOut string, found bool, err error) {
	found, err = f.listAll(ctx, pathID, api.ItemTypeFolder, false, func(item *api.Item) bool {
		if strings.EqualFold(item.Name, leaf) { // names are case insensitive
			pathIDOut = item.ID
			return true
		}
		return false
	})
	if errors.Is(err, fs.ErrorDirNotFound) {
		return "", false, nil
	}
	return pathIDOut, found, err
}

// CreateDir makes a directory with pathID as parent and name leaf
func (f *Fs) CreateDir(ctx context.Context, pathID, leaf string) (newID string, err error) {
	req := api.CreateFolder{Name: f.opt.Enc.FromStandardName(leaf)}
	if pathID != "" {
		req.ParentID = &pathID
	}
	var item api.Item
	opts := rest.Opts{Method: "POST", Path: "/folders"}
	err = f.call(ctx, &opts, &req, &item)
	if err != nil {
		return "", fmt.Errorf("failed to create directory: %w", err)
	}
	return item.ID, nil
}

// itemToDirEntry converts a list item into a DirEntry
func (f *Fs) itemToDirEntry(ctx context.Context, remote string, dirID string, info *api.Item) (fs.DirEntry, error) {
	if info.Type == api.ItemTypeFolder {
		f.dirCache.Put(remote, info.ID)
		d := fs.NewDir(remote, modTimeOf(info)).SetID(info.ID)
		if info.ParentID != nil {
			d.SetParentID(*info.ParentID)
		}
		return d, nil
	}
	return f.newObjectWithInfo(ctx, remote, info)
}

func modTimeOf(info *api.Item) time.Time {
	if info.ModifiedAt != nil {
		return *info.ModifiedAt
	}
	if info.UpdatedAt != nil {
		return *info.UpdatedAt
	}
	if info.CreatedAt != nil {
		return *info.CreatedAt
	}
	return time.Time{}
}

// List the objects and directories in dir into entries.
func (f *Fs) List(ctx context.Context, dir string) (entries fs.DirEntries, err error) {
	directoryID, err := f.dirCache.FindDir(ctx, dir, false)
	if err != nil {
		return nil, err
	}
	var iErr error
	_, err = f.listAll(ctx, directoryID, "", false, func(info *api.Item) bool {
		entry, err := f.itemToDirEntry(ctx, path.Join(dir, info.Name), directoryID, info)
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
	return entries, iErr
}

// ListR lists the objects and directories of the Fs starting from dir
// recursively into out, using two flat passes: folders, then files.
func (f *Fs) ListR(ctx context.Context, dir string, callback fs.ListRCallback) (err error) {
	directoryID, err := f.dirCache.FindDir(ctx, dir, false)
	if err != nil {
		return err
	}
	// absolute server path of dir, to turn entry paths into remotes
	var segs []string
	for _, s := range strings.Split(path.Join(f.root, dir), "/") {
		if s != "" {
			segs = append(segs, f.opt.Enc.FromStandardName(s))
		}
	}
	prefix := "/" + strings.Join(segs, "/")
	list := walkList{callback: callback, max: fs.GetConfig(ctx).Checkers * 100}
	for _, kind := range []string{api.ItemTypeFolder, api.ItemTypeFile} {
		var iErr error
		_, err = f.listAll(ctx, directoryID, kind, true, func(info *api.Item) bool {
			rel, ok := relativePath(prefix, info.Path)
			if !ok {
				iErr = fmt.Errorf("recursive listing returned path %q outside %q", info.Path, prefix)
				return true
			}
			parts := strings.Split(rel, "/")
			for i := range parts {
				parts[i] = f.opt.Enc.ToStandardName(parts[i])
			}
			remote := path.Join(dir, strings.Join(parts, "/"))
			entry, err := f.itemToDirEntry(ctx, remote, "", info)
			if err != nil {
				iErr = err
				return true
			}
			if err := list.add(entry); err != nil {
				iErr = err
				return true
			}
			return false
		})
		if err == nil {
			err = iErr
		}
		if err != nil {
			return err
		}
	}
	return list.flush()
}

// relativePath returns p relative to prefix, both absolute and '/' separated
func relativePath(prefix, p string) (string, bool) {
	if prefix == "/" {
		prefix = ""
	}
	if !strings.HasPrefix(p, prefix+"/") {
		return "", false
	}
	return p[len(prefix)+1:], true
}

// walkList batches entries for a ListRCallback
type walkList struct {
	callback fs.ListRCallback
	max      int
	entries  fs.DirEntries
}

func (w *walkList) add(e fs.DirEntry) error {
	w.entries = append(w.entries, e)
	if w.max > 0 && len(w.entries) >= w.max {
		return w.flush()
	}
	return nil
}

func (w *walkList) flush() error {
	if len(w.entries) == 0 {
		return nil
	}
	err := w.callback(w.entries)
	w.entries = nil
	return err
}

// newObjectWithInfo creates an fs.Object from info or reads it from the server
func (f *Fs) newObjectWithInfo(ctx context.Context, remote string, info *api.Item) (fs.Object, error) {
	o := &Object{fs: f, remote: remote}
	var err error
	if info != nil {
		err = o.setMetaData(info)
	} else {
		err = o.readMetaData(ctx)
	}
	if err != nil {
		return nil, err
	}
	return o, nil
}

// NewObject finds the Object at remote.
func (f *Fs) NewObject(ctx context.Context, remote string) (fs.Object, error) {
	return f.newObjectWithInfo(ctx, remote, nil)
}

// readMetaDataForPath finds the file at path
func (f *Fs) readMetaDataForPath(ctx context.Context, p string) (info *api.Item, dirID string, err error) {
	leaf, directoryID, err := f.dirCache.FindPath(ctx, p, false)
	if err != nil {
		if err == fs.ErrorDirNotFound {
			return nil, "", fs.ErrorObjectNotFound
		}
		return nil, "", err
	}
	// One call by path instead of listing the whole folder: the API resolves the name itself (GET /files/by-path).
	// Names go to the server in their encoded form, as everywhere else (the encoder maps characters a name cannot hold).
	segs := strings.Split(path.Join(f.root, p), "/")
	for i, s := range segs {
		segs[i] = f.opt.Enc.FromStandardName(s)
	}
	opts := rest.Opts{
		Method:     "GET",
		Path:       "/files/by-path",
		Parameters: url.Values{"path": {"/" + strings.Join(segs, "/")}},
	}
	var item api.Item
	err = f.call(ctx, &opts, nil, &item)
	if err != nil {
		if e := apiError(err); e != nil {
			switch e.Status {
			case http.StatusNotFound:
				return nil, "", fs.ErrorObjectNotFound
			case http.StatusConflict:
				if e.Code() == "ambiguous_path" {
					return nil, "", fmt.Errorf("more than one item is named %q here; rename one: %w", leaf, err)
				}
			}
		}
		return nil, "", err
	}
	if item.Type != api.ItemTypeFile {
		return nil, "", fs.ErrorIsDir
	}
	item.Name = f.opt.Enc.ToStandardName(item.Name)
	info = &item
	return info, directoryID, nil
}

// createObject makes a half finished Object for remote, creating its directory
func (f *Fs) createObject(ctx context.Context, remote string, modTime time.Time, size int64) (o *Object, leaf string, directoryID string, err error) {
	leaf, directoryID, err = f.dirCache.FindPath(ctx, remote, true)
	if err != nil {
		return nil, "", "", err
	}
	return &Object{fs: f, remote: remote, size: size, modTime: modTime, dirID: directoryID}, leaf, directoryID, nil
}

// Put the object, overwriting any file with the same name.
func (f *Fs) Put(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error) {
	existing, err := f.NewObject(ctx, src.Remote())
	switch err {
	case nil:
		return existing, existing.Update(ctx, in, src, options...)
	case fs.ErrorObjectNotFound:
		o, _, _, err := f.createObject(ctx, src.Remote(), src.ModTime(ctx), src.Size())
		if err != nil {
			return nil, err
		}
		return o, o.Update(ctx, in, src, options...)
	default:
		return nil, err
	}
}

// Mkdir creates the directory if it doesn't exist
func (f *Fs) Mkdir(ctx context.Context, dir string) error {
	_, err := f.dirCache.FindDir(ctx, dir, true)
	return err
}

// deleteObject moves an item to the bin
func (f *Fs) deleteObject(ctx context.Context, id string) error {
	opts := rest.Opts{
		Method:     "DELETE",
		Path:       "/files/" + id,
		Parameters: url.Values{},
		NoResponse: true,
	}
	if f.opt.DeleteTransfers {
		opts.Parameters.Set("confirm_delete_transfers", "true")
	}
	err := f.call(ctx, &opts, nil, nil)
	if e := apiError(err); e != nil {
		switch e.Code() {
		case "not_found":
			return fs.ErrorObjectNotFound
		case "transfer_active":
			return fmt.Errorf("item is in an active transfer (use --tarkle-delete-transfers to delete the transfers too): %w", err)
		}
		if e.Status == http.StatusNotFound {
			return fs.ErrorObjectNotFound
		}
	}
	return err
}

// purgeCheck removes the directory, if check is set then it
// refuses to do so if it has anything in
func (f *Fs) purgeCheck(ctx context.Context, dir string, check bool) error {
	if path.Join(f.root, dir) == "" {
		return errors.New("can't purge root directory")
	}
	rootID, err := f.dirCache.FindDir(ctx, dir, false)
	if err != nil {
		return err
	}
	if check {
		found, err := f.listAll(ctx, rootID, "", false, func(*api.Item) bool { return true })
		if err != nil {
			return err
		}
		if found {
			return fs.ErrorDirectoryNotEmpty
		}
	}
	if err = f.deleteObject(ctx, rootID); err != nil {
		return err
	}
	f.dirCache.FlushDir(dir)
	return nil
}

// Rmdir deletes the directory, which must be empty
func (f *Fs) Rmdir(ctx context.Context, dir string) error {
	return f.purgeCheck(ctx, dir, true)
}

// Purge deletes the directory and everything in it (to the bin)
func (f *Fs) Purge(ctx context.Context, dir string) error {
	return f.purgeCheck(ctx, dir, false)
}

// patch renames and/or moves an item
func (f *Fs) patch(ctx context.Context, id string, p api.Patch) (*api.Item, error) {
	var item api.Item
	opts := rest.Opts{Method: "PATCH", Path: "/files/" + id}
	if err := f.call(ctx, &opts, p, &item); err != nil {
		return nil, fmt.Errorf("failed to rename or move item: %w", err)
	}
	return &item, nil
}

// moveTo renames and/or moves id from srcDirectoryID to dstDirectoryID
func (f *Fs) moveTo(ctx context.Context, id, srcLeaf, dstLeaf, srcDirectoryID, dstDirectoryID string) (*api.Item, error) {
	p := api.Patch{}
	if srcLeaf != dstLeaf {
		n := f.opt.Enc.FromStandardName(dstLeaf)
		p.Name = &n
	}
	if srcDirectoryID != dstDirectoryID {
		p.Move = true
		if dstDirectoryID != "" {
			p.ParentID = &dstDirectoryID
		}
	}
	if p.Name == nil && !p.Move {
		return f.statItem(ctx, id)
	}
	return f.patch(ctx, id, p)
}

// statItem reads one entry
func (f *Fs) statItem(ctx context.Context, id string) (*api.Item, error) {
	var item api.Item
	opts := rest.Opts{Method: "GET", Path: "/files/" + id}
	if err := f.call(ctx, &opts, nil, &item); err != nil {
		if e := apiError(err); e != nil && e.Status == http.StatusNotFound {
			return nil, fs.ErrorObjectNotFound
		}
		return nil, err
	}
	item.Name = f.opt.Enc.ToStandardName(item.Name)
	return &item, nil
}

// Move src to this remote using server-side move operations.
func (f *Fs) Move(ctx context.Context, src fs.Object, remote string) (fs.Object, error) {
	srcObj, ok := src.(*Object)
	if !ok {
		fs.Debugf(src, "Can't move - not same remote type")
		return nil, fs.ErrorCantMove
	}
	srcLeaf, srcDirectoryID, err := srcObj.fs.dirCache.FindPath(ctx, srcObj.remote, false)
	if err != nil {
		return nil, err
	}
	dstObj, dstLeaf, dstDirectoryID, err := f.createObject(ctx, remote, srcObj.modTime, srcObj.size)
	if err != nil {
		return nil, err
	}
	info, err := f.moveTo(ctx, srcObj.id, srcLeaf, dstLeaf, srcDirectoryID, dstDirectoryID)
	if err != nil {
		return nil, err
	}
	if err = dstObj.setMetaData(info); err != nil {
		return nil, err
	}
	return dstObj, nil
}

// DirMove moves src, srcRemote to this remote at dstRemote using server-side move operations.
func (f *Fs) DirMove(ctx context.Context, src fs.Fs, srcRemote, dstRemote string) error {
	srcFs, ok := src.(*Fs)
	if !ok {
		fs.Debugf(src, "Can't move directory - not same remote type")
		return fs.ErrorCantDirMove
	}
	srcID, srcDirectoryID, srcLeaf, dstDirectoryID, dstLeaf, err := f.dirCache.DirMove(ctx, srcFs.dirCache, srcFs.root, srcRemote, f.root, dstRemote)
	if err != nil {
		return err
	}
	if _, err = f.moveTo(ctx, srcID, srcLeaf, dstLeaf, srcDirectoryID, dstDirectoryID); err != nil {
		return err
	}
	srcFs.dirCache.FlushDir(srcRemote)
	return nil
}

// Copy src to this remote using a server-side copy (files only).
func (f *Fs) Copy(ctx context.Context, src fs.Object, remote string) (fs.Object, error) {
	srcObj, ok := src.(*Object)
	if !ok {
		fs.Debugf(src, "Can't copy - not same remote type")
		return nil, fs.ErrorCantCopy
	}
	if srcObj.fs.rootSlash()+srcObj.remote == f.rootSlash()+remote {
		return nil, fmt.Errorf("can't copy %q -> %q as are same name", srcObj.remote, remote)
	}
	// copy over an existing file replaces it
	existing, err := f.NewObject(ctx, remote)
	if err != nil {
		existing = nil
	}
	dstObj, dstLeaf, dstDirectoryID, err := f.createObject(ctx, remote, srcObj.modTime, srcObj.size)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		if err = existing.Remove(ctx); err != nil {
			return nil, fmt.Errorf("failed to remove existing file before copy: %w", err)
		}
	}
	req := api.Copy{Name: f.opt.Enc.FromStandardName(dstLeaf)}
	if dstDirectoryID != "" {
		req.ParentID = &dstDirectoryID
	}
	var item api.Item
	opts := rest.Opts{Method: "POST", Path: "/files/" + srcObj.id + "/copy"}
	if err = f.call(ctx, &opts, &req, &item); err != nil {
		if e := apiError(err); e != nil && e.Code() == "storage_full" {
			return nil, fserrors.FatalError(err)
		}
		return nil, fmt.Errorf("failed to copy: %w", err)
	}
	if err = dstObj.setMetaData(&item); err != nil {
		return nil, err
	}
	return dstObj, nil
}

func (f *Fs) rootSlash() string {
	if f.root == "" {
		return f.root
	}
	return f.root + "/"
}

// DirCacheFlush resets the directory cache
func (f *Fs) DirCacheFlush() { f.dirCache.ResetRoot() }

// About gets quota information
func (f *Fs) About(ctx context.Context) (*fs.Usage, error) {
	var result api.Usage
	opts := rest.Opts{Method: "GET", Path: "/usage"}
	if err := f.call(ctx, &opts, nil, &result); err != nil {
		return nil, fmt.Errorf("failed to read usage: %w", err)
	}
	used := result.Storage.UsedBytes
	usage := &fs.Usage{Used: fs.NewUsageValue(used)}
	if result.Storage.LimitBytes != nil {
		usage.Total = fs.NewUsageValue(*result.Storage.LimitBytes)
		usage.Free = fs.NewUsageValue(max(0, *result.Storage.LimitBytes-used))
	}
	return usage, nil
}

// ------------------------------------------------------------

// Fs returns the parent Fs
func (o *Object) Fs() fs.Info { return o.fs }

// String returns a description of the Object
func (o *Object) String() string {
	if o == nil {
		return "<nil>"
	}
	return o.remote
}

// Remote returns the remote path
func (o *Object) Remote() string { return o.remote }

// Hash returns the SHA-256 of the object if known. When Tarkle could not
// verify it (files over 100 MiB) it is the uploader's own claim.
func (o *Object) Hash(ctx context.Context, t hash.Type) (string, error) {
	if t != hash.SHA256 {
		return "", hash.ErrUnsupported
	}
	if err := o.readMetaData(ctx); err != nil {
		return "", err
	}
	return o.sha256, nil
}

// Size returns the size of an object in bytes
func (o *Object) Size() int64 { return o.size }

// ID returns the ID of the Object
func (o *Object) ID() string { return o.id }

// ParentID returns the ID of the Object's directory
func (o *Object) ParentID() string { return o.dirID }

// Storable returns whether this object is storable
func (o *Object) Storable() bool { return true }

// ModTime returns the time the file was last updated on the server
func (o *Object) ModTime(ctx context.Context) time.Time { return o.modTime }

// SetModTime sets the modification time without uploading again
func (o *Object) SetModTime(ctx context.Context, modTime time.Time) error {
	if err := o.readMetaData(ctx); err != nil {
		return err
	}
	when := formatTime(modTime)
	info, err := o.fs.patch(ctx, o.id, api.Patch{ModifiedAt: &when})
	if err != nil {
		return err
	}
	return o.setMetaData(info)
}

// Metadata returns the modification and creation times of the object
func (o *Object) Metadata(ctx context.Context) (fs.Metadata, error) {
	if err := o.readMetaData(ctx); err != nil {
		return nil, err
	}
	m := fs.Metadata{"mtime": o.modTime.Format(time.RFC3339Nano)}
	if !o.created.IsZero() {
		m["btime"] = o.created.Format(time.RFC3339Nano)
	}
	return m, nil
}

func (o *Object) setMetaData(info *api.Item) error {
	if info.Type != api.ItemTypeFile {
		return fs.ErrorIsDir
	}
	o.id = info.ID
	o.size = info.Size
	o.modTime = modTimeOf(info)
	o.created = time.Time{}
	if info.CreatedAt != nil {
		o.created = *info.CreatedAt
	}
	o.sha256, o.sha256Source = "", ""
	if info.SHA256 != nil {
		o.sha256 = strings.ToLower(*info.SHA256)
		if info.SHA256Source != nil {
			o.sha256Source = *info.SHA256Source
		}
	}
	if info.ParentID != nil {
		o.dirID = *info.ParentID
	} else {
		o.dirID = ""
	}
	o.linkMu.Lock()
	o.link = "" // content may have changed
	o.linkMu.Unlock()
	return nil
}

func (o *Object) readMetaData(ctx context.Context) error {
	if o.id != "" {
		return nil
	}
	info, _, err := o.fs.readMetaDataForPath(ctx, o.remote)
	if err != nil {
		return err
	}
	return o.setMetaData(info)
}

// downloadLink returns a presigned GET URL, cached for a few minutes
func (o *Object) downloadLink(ctx context.Context, refresh bool) (string, error) {
	o.linkMu.Lock()
	defer o.linkMu.Unlock()
	if !refresh && o.link != "" && time.Now().Before(o.linkExpiry) {
		return o.link, nil
	}
	var dl api.Download
	opts := rest.Opts{Method: "GET", Path: "/files/" + o.id + "/download"}
	if err := o.fs.call(ctx, &opts, nil, &dl); err != nil {
		if e := apiError(err); e != nil && e.Status == http.StatusNotFound {
			return "", fs.ErrorObjectNotFound
		}
		return "", fmt.Errorf("couldn't get download link: %w", err)
	}
	o.link = dl.DownloadURL
	o.linkExpiry = time.Now().Add(downloadLinkLife)
	return o.link, nil
}

// Open an object for read. Range options are passed to the storage host.
func (o *Object) Open(ctx context.Context, options ...fs.OpenOption) (io.ReadCloser, error) {
	if o.id == "" {
		return nil, errors.New("can't download - no id")
	}
	fs.FixRangeOption(options, o.size)
	refresh := false
	var resp *http.Response
	err := o.fs.storePacer.Call(func() (bool, error) {
		link, err := o.downloadLink(ctx, refresh)
		if err != nil {
			return false, err
		}
		opts := rest.Opts{Method: "GET", RootURL: link, Options: options}
		resp, err = o.fs.store.Call(ctx, &opts)
		if resp != nil && (resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusUnauthorized) {
			refresh = true // link expired: ask for a new one
			return true, err
		}
		return shouldRetry(ctx, resp, err)
	})
	if err != nil {
		return nil, err
	}
	return resp.Body, nil
}

// Remove an object
func (o *Object) Remove(ctx context.Context) error {
	return o.fs.deleteObject(ctx, o.id)
}

// Update the object with the contents of the io.Reader, modTime and size.
// The size must be known: Tarkle declares it when the upload starts, so
// streams are spooled by rclone (no PutStream).
func (o *Object) Update(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) error {
	size := src.Size()
	if size < 0 {
		return errors.New("tarkle needs the size of a file before it is uploaded")
	}
	// (the chunk writer handles size 0 too: it needs no parts)
	// the upload goes to this object's remote, which may differ from src's
	cw, err := multipart.UploadMultipart(ctx, fs.NewOverrideRemote(src, o.remote), in, multipart.UploadMultipartOptions{
		Open:        o.fs,
		OpenOptions: options,
	})
	if err != nil {
		return err
	}
	w := cw.(*chunkWriter)
	return o.setMetaData(&w.entry)
}

// Check the interfaces are satisfied
var (
	_ fs.Fs              = (*Fs)(nil)
	_ fs.Purger          = (*Fs)(nil)
	_ fs.Copier          = (*Fs)(nil)
	_ fs.Mover           = (*Fs)(nil)
	_ fs.DirMover        = (*Fs)(nil)
	_ fs.DirCacheFlusher = (*Fs)(nil)
	_ fs.Abouter         = (*Fs)(nil)
	_ fs.ListRer         = (*Fs)(nil)
	_ fs.OpenChunkWriter = (*Fs)(nil)
	_ fs.Object          = (*Object)(nil)
	_ fs.IDer            = (*Object)(nil)
	_ fs.ParentIDer      = (*Object)(nil)
	_ fs.Metadataer      = (*Object)(nil)
)
