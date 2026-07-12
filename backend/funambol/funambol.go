// Package funambol provides an interface to Funambol / OneMediaHub
// (cloud.o2online.es) storage system, which is built on the Funambol /
// OneMediaHub "SAPI".
package funambol

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"path"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/rclone/rclone/backend/funambol/api"
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
	"github.com/rclone/rclone/lib/pacer"
	"github.com/rclone/rclone/lib/rest"
)

const (
	defaultEndpoint = "https://cloud.o2online.es"
	minSleep        = 10 * time.Millisecond
	maxSleep        = 2 * time.Second
	decayConstant   = 2
	listChunk       = 200    // media items per page
	uploadMediaType = "file" // store everything as a generic file (no transcoding)
)

// mediaFields are the item fields we ask the listing endpoint to return.
// "contenttype"/"mime" are intentionally absent: SAPI rejects them as fields.
var mediaFields = []string{
	"name", "size", "creationdate", "modificationdate", "url", "favorite",
}

// Register with Fs
func init() {
	fs.Register(&fs.RegInfo{
		Name:        "funambol",
		Description: "Funambol / OneMediaHub (O2 Cloud and other telco tenants)",
		NewFs:       NewFs,
		Config:      Config,
		Options: []fs.Option{{
			Name:      "user",
			Help:      "Username for the Funambol / OneMediaHub account.\n\nUsually the email address or mobile number (MSISDN) you log in with.",
			Required:  true,
			Sensitive: true,
		}, {
			Name:       "pass",
			Help:       "Password for the Funambol / OneMediaHub account.",
			Required:   true,
			IsPassword: true,
		}, {
			Name: "cookies",
			Help: `Persisted session cookies (set automatically by rclone config).

This holds the logged-in session established during config (after any
two-factor step) so it can be reused.  When it expires, reconnect with
"rclone config reconnect remote:".`,
			Hide:      fs.OptionHideBoth,
			Sensitive: true,
			Advanced:  true,
		}, {
			Name: "device_id",
			Help: `Device id reported to the service (auto-generated if left blank).

This identifies the rclone "device" to the service and appears in your
account's device list.  Leave it empty: rclone generates a stable
"web-<hex>" id on first login and saves it here, registering itself just
like the official app.  Set it only to reuse a specific existing id.`,
			Advanced:  true,
			Sensitive: true,
		}, {
			Name: "endpoint",
			Help: `SAPI endpoint of the provider hosting the service.

Funambol / OneMediaHub is white-labelled by several operators.  Leave
this set to the default for O2 Spain, or point it at another tenant,
e.g. https://www.cloud.example.com`,
			Default:  defaultEndpoint,
			Advanced: true,
		}, {
			Name:     config.ConfigEncoding,
			Help:     config.ConfigEncodingHelp,
			Advanced: true,
			Default: encoder.Display |
				encoder.EncodeBackSlash |
				encoder.EncodeDoubleQuote |
				encoder.EncodeInvalidUtf8,
		}},
	})
}

// Options defines the configuration for this backend
type Options struct {
	User     string               `config:"user"`
	Pass     string               `config:"pass"`
	Endpoint string               `config:"endpoint"`
	Cookies  string               `config:"cookies"`
	DeviceID string               `config:"device_id"`
	Enc      encoder.MultiEncoder `config:"encoding"`
}

// Fs represents a remote Funambol / OneMediaHub
type Fs struct {
	name     string
	root     string
	opt      Options
	features *fs.Features
	client   *http.Client
	srv      *rest.Client
	dirCache *dircache.DirCache
	pacer    *fs.Pacer
	rootID   string           // numeric id of the account root folder
	m        configmap.Mapper // for persisting refreshed session cookies

	auth *authState // shared session state (pointer so *Fs stays copyable)
}

// authState serialises (re-)authentication.  The session itself lives in the
// http.Client cookie jar (JSESSIONID + persistent-login cookie) and the
// validation key, so only a mutex is needed to guard concurrent refreshes.
type authState struct {
	mu sync.Mutex
}

// Object describes a Funambol / OneMediaHub file
type Object struct {
	fs          *Fs
	remote      string
	hasMetaData bool
	size        int64
	modTime     time.Time
	id          string
	url         string
}

// ------------------------------------------------------------

// Name of the remote (as passed into NewFs)
func (f *Fs) Name() string { return f.name }

// Root of the remote (as passed into NewFs)
func (f *Fs) Root() string { return f.root }

// String converts this Fs to a string
func (f *Fs) String() string { return fmt.Sprintf("funambol root '%s'", f.root) }

// Features returns the optional features of this Fs
func (f *Fs) Features() *fs.Features { return f.features }

// Precision of the remote (SAPI stores second-resolution timestamps)
func (f *Fs) Precision() time.Duration { return time.Second }

// Hashes returns the supported hash sets.  The item "etag" looks like a digest
// but is assigned per upload (two uploads of identical bytes get different
// etags), so it is not a usable content hash and no hash type is advertised.
func (f *Fs) Hashes() hash.Set { return hash.Set(hash.None) }

func parsePath(p string) string { return strings.Trim(p, "/") }

var retryErrorCodes = []int{429, 500, 502, 503, 504, 509}

func shouldRetry(ctx context.Context, resp *http.Response, err error) (bool, error) {
	if fserrors.ContextError(ctx, &err) {
		return false, err
	}
	return fserrors.ShouldRetry(err) || fserrors.ShouldRetryHTTP(resp, retryErrorCodes), err
}

// ---- authentication ----
//
// The session lives in the cookie jar (JSESSIONID + validationKey [+ PLC]).
// SAPI wants the validation key as a query parameter, and its value is exactly
// the validationKey cookie, so key() reads it straight from the jar.  The
// interactive login / two-factor flow lives in session.go.

// errReconnect is returned when the persistent login has expired and re-login
// would need a fresh two-factor code.
func errReconnect(name string) error {
	return fmt.Errorf("funambol: session expired - run %q to log in again", "rclone config reconnect "+name+":")
}

// key returns the current validation key (stored as the validationKey cookie).
// It may be empty after a fresh restore from the persistent login cookie, in
// which case the first call deliberately provokes a SEC-1003 to mint one.
func (f *Fs) key() string {
	return f.validationKeyFromJar()
}

// authCodes are SAPI error codes that mean the persistent login itself is no
// longer valid, so only a fresh interactive login (two-factor) can recover.
func isReauthCode(code string) bool {
	switch code {
	case "SEC-1000", "SEC-1001", "SEC-1004", "SEC-1005":
		return true
	}
	return false
}

// isTransientCode reports SAPI error codes that signal a temporary server-side
// fault rather than a definitive failure, so the call is worth retrying with
// backoff.  These come back as HTTP 200 + an error envelope, so the HTTP-level
// shouldRetry never sees them.
func isTransientCode(code string) bool {
	switch code {
	// "Unknown exception in folder handling" — an internal server hiccup, seen
	// when several transfers create/list the same folder tree concurrently.
	case "FOL-1000":
		return true
	}
	return false
}

// ensureSession makes sure we have something to authenticate with: either a
// live validation key or the persistent login cookie that can mint one.
// Logging in from scratch needs the emailed verification code, so it can only
// happen interactively via "rclone config".
func (f *Fs) ensureLogin(ctx context.Context) error {
	f.auth.mu.Lock()
	defer f.auth.mu.Unlock()
	if f.validationKeyFromJar() != "" || f.persistentLoginPresent() {
		return nil
	}
	return errReconnect(f.name)
}

// callJSON makes an authenticated SAPI JSON call.  The validation key rotates
// and eventually expires; when it does the server replies 401 SEC-1003 with a
// fresh key in the error's data field (and renews the persistent login
// cookie).  We pick that up, store it, persist the refreshed cookies and retry
// - so a session established once with two-factor keeps working unattended.
func (f *Fs) callJSON(ctx context.Context, opts *rest.Opts, request, response any) error {
	if err := f.ensureLogin(ctx); err != nil {
		return err
	}
	for attempt := 0; attempt < 3; attempt++ {
		if opts.Parameters == nil {
			opts.Parameters = url.Values{}
		}
		if vk := f.key(); vk != "" {
			opts.Parameters.Set("validationkey", vk)
		} else {
			opts.Parameters.Del("validationkey")
		}
		var resp *http.Response
		err := f.pacer.Call(func() (bool, error) {
			var err error
			resp, err = f.srv.CallJSON(ctx, opts, request, response)
			if retry, rErr := shouldRetry(ctx, resp, err); retry || err != nil {
				return retry, rErr
			}
			// HTTP succeeded: retry transient SAPI exceptions (HTTP 200 +
			// error envelope) with the pacer's backoff. SEC-1003 and other
			// envelope errors are left for the outer loop to handle.
			if er, ok := response.(interface{ AsErr() error }); ok {
				if aErr := er.AsErr(); aErr != nil {
					var ae *api.Error
					if errors.As(aErr, &ae) && isTransientCode(ae.Code) {
						return true, aErr
					}
				}
			}
			return false, nil
		})
		if err != nil {
			if f.refreshOnError(err) && attempt < 2 {
				continue
			}
			var ae *api.Error
			if errors.As(err, &ae) && (resp != nil && resp.StatusCode == http.StatusUnauthorized || isReauthCode(ae.Code)) {
				return errReconnect(f.name)
			}
			return err
		}
		// SAPI may signal failure with HTTP 200 + an error envelope.
		if er, ok := response.(interface{ AsErr() error }); ok {
			if aErr := er.AsErr(); aErr != nil {
				if f.refreshOnError(aErr) && attempt < 2 {
					continue
				}
				return aErr
			}
		}
		return nil
	}
	return errors.New("funambol: authentication retry exhausted")
}

// refreshOnError handles a SEC-1003 ("invalid mandatory validation key") by
// adopting the fresh key the server hands back and persisting the renewed
// cookies.  It reports whether the caller should retry.
func (f *Fs) refreshOnError(err error) bool {
	var ae *api.Error
	if !errors.As(err, &ae) || ae.Code != "SEC-1003" || ae.Data == "" {
		return false
	}
	f.setValidationKey(ae.Data)
	f.persistSession() // the 401 also renewed the persistent login cookie
	return true
}

// ---- dircache.DirCacher ----

// FindLeaf finds a directory called leaf in the directory with id pathID
func (f *Fs) FindLeaf(ctx context.Context, pathID, leaf string) (pathIDOut string, found bool, err error) {
	folders, err := f.listFolders(ctx, pathID)
	if err != nil {
		return "", false, err
	}
	for _, fl := range folders {
		if strings.EqualFold(f.opt.Enc.ToStandardName(fl.Name), leaf) {
			return fl.ID.String(), true, nil
		}
	}
	return "", false, nil
}

// CreateDir makes a directory with pathID as parent and name leaf
func (f *Fs) CreateDir(ctx context.Context, pathID, leaf string) (newID string, err error) {
	var req api.SaveFolderRequest
	req.Data.Name = f.opt.Enc.FromStandardName(leaf)
	parent := api.ID(pathID)
	req.Data.Parent = &parent
	var res api.SaveFolderResponse
	opts := rest.Opts{Method: "POST", Path: "/sapi/media/folder", Parameters: url.Values{"action": {"save"}}}
	if err := f.callJSON(ctx, &opts, &req, &res); err != nil {
		return "", fmt.Errorf("CreateDir: %w", err)
	}
	id := res.NewID()
	if id == "" {
		return "", errors.New("CreateDir: server returned no folder id")
	}
	return id.String(), nil
}

// ---- listing ----

// listFolders returns every sub-folder of dirID in a single request.
//
// Unlike the media listing, the folder-list endpoint takes only "parentid":
// the official client sends no "limit" or "offset", and the server returns the
// full set in one response.  Passing "offset" makes the server reject the call
// with COM-1021 ("Invalid parameter value"), so we deliberately don't paginate
// here — matching the app's behaviour.
func (f *Fs) listFolders(ctx context.Context, dirID string) ([]api.Folder, error) {
	var res api.FoldersResponse
	opts := rest.Opts{
		Method:     "GET",
		Path:       "/sapi/media/folder",
		Parameters: url.Values{"action": {"list"}, "parentid": {dirID}},
	}
	if err := f.callJSON(ctx, &opts, nil, &res); err != nil {
		return nil, fmt.Errorf("list folders: %w", err)
	}
	return res.Data.Folders, nil
}

func (f *Fs) listMedia(ctx context.Context, dirID string, fn func(*api.Item)) error {
	offset := 0
	var req api.FieldsRequest
	req.Data.Fields = mediaFields
	for {
		var res api.MediaResponse
		opts := rest.Opts{
			Method: "POST",
			Path:   "/sapi/media",
			Parameters: url.Values{
				"action":   {"get"},
				"folderid": {dirID},
				"limit":    {strconv.Itoa(listChunk)},
				"offset":   {strconv.Itoa(offset)},
			},
		}
		if err := f.callJSON(ctx, &opts, &req, &res); err != nil {
			return fmt.Errorf("list media: %w", err)
		}
		for i := range res.Data.Media {
			fn(&res.Data.Media[i])
		}
		if !res.Data.More || len(res.Data.Media) == 0 {
			return nil
		}
		offset += len(res.Data.Media)
	}
}

// List the objects and directories in dir into entries.
func (f *Fs) List(ctx context.Context, dir string) (entries fs.DirEntries, err error) {
	dirID, err := f.dirCache.FindDir(ctx, dir, false)
	if err != nil {
		return nil, err
	}
	folders, err := f.listFolders(ctx, dirID)
	if err != nil {
		return nil, err
	}
	for _, fl := range folders {
		name := f.opt.Enc.ToStandardName(fl.Name)
		remote := path.Join(dir, name)
		f.dirCache.Put(remote, fl.ID.String())
		entries = append(entries, fs.NewDir(remote, time.UnixMilli(fl.Date)).SetID(fl.ID.String()))
	}
	var iErr error
	err = f.listMedia(ctx, dirID, func(item *api.Item) {
		if item.Name == "" {
			return
		}
		remote := path.Join(dir, f.opt.Enc.ToStandardName(item.Name))
		o, oErr := f.newObjectWithInfo(ctx, remote, item)
		if oErr != nil {
			iErr = oErr
			return
		}
		entries = append(entries, o)
	})
	if err != nil {
		return nil, err
	}
	if iErr != nil {
		return nil, iErr
	}
	return entries, nil
}

// ---- objects ----

func (f *Fs) newObjectWithInfo(ctx context.Context, remote string, info *api.Item) (fs.Object, error) {
	o := &Object{fs: f, remote: remote}
	if info != nil {
		if err := o.setMetaData(info); err != nil {
			return nil, err
		}
	} else if err := o.readMetaData(ctx); err != nil {
		return nil, err
	}
	return o, nil
}

// NewObject finds the Object at remote.
func (f *Fs) NewObject(ctx context.Context, remote string) (fs.Object, error) {
	return f.newObjectWithInfo(ctx, remote, nil)
}

func (f *Fs) readMetaDataForPath(ctx context.Context, remote string) (*api.Item, error) {
	leaf, dirID, err := f.dirCache.FindPath(ctx, remote, false)
	if err != nil {
		if err == fs.ErrorDirNotFound {
			return nil, fs.ErrorObjectNotFound
		}
		return nil, err
	}
	lcLeaf := strings.ToLower(leaf)
	var found *api.Item
	err = f.listMedia(ctx, dirID, func(item *api.Item) {
		if found == nil && strings.ToLower(f.opt.Enc.ToStandardName(item.Name)) == lcLeaf {
			it := *item
			found = &it
		}
	})
	if err != nil {
		return nil, err
	}
	if found == nil {
		return nil, fs.ErrorObjectNotFound
	}
	return found, nil
}

func (o *Object) setMetaData(info *api.Item) error {
	o.hasMetaData = true
	o.size = info.Size
	o.modTime = info.ModTime()
	o.id = info.ID.String()
	o.url = info.URL
	return nil
}

func (o *Object) readMetaData(ctx context.Context) error {
	if o.hasMetaData {
		return nil
	}
	info, err := o.fs.readMetaDataForPath(ctx, o.remote)
	if err != nil {
		return err
	}
	return o.setMetaData(info)
}

// ---- Fs write operations ----

// Mkdir creates the directory if it doesn't exist
func (f *Fs) Mkdir(ctx context.Context, dir string) error {
	_, err := f.dirCache.FindDir(ctx, dir, true)
	return err
}

func (f *Fs) deleteFolder(ctx context.Context, id string) error {
	var req api.FoldersRequest
	req.Data.Folders = []string{id}
	opts := rest.Opts{Method: "POST", Path: "/sapi/media/folder", Parameters: url.Values{"action": {"delete"}}, NoResponse: true}
	return f.callJSON(ctx, &opts, &req, &struct{}{})
}

func (f *Fs) purgeCheck(ctx context.Context, dir string, check bool) error {
	root := path.Join(f.root, dir)
	if root == "" {
		return errors.New("can't purge root directory")
	}
	dirID, err := f.dirCache.FindDir(ctx, dir, false)
	if err != nil {
		return err
	}
	if check {
		folders, err := f.listFolders(ctx, dirID)
		if err != nil {
			return err
		}
		empty := len(folders) == 0
		if empty {
			err = f.listMedia(ctx, dirID, func(*api.Item) { empty = false })
			if err != nil {
				return err
			}
		}
		if !empty {
			return fs.ErrorDirectoryNotEmpty
		}
	}
	if err := f.deleteFolder(ctx, dirID); err != nil {
		return fmt.Errorf("rmdir: %w", err)
	}
	f.dirCache.FlushDir(dir)
	return nil
}

// Rmdir removes the directory, refusing if not empty
func (f *Fs) Rmdir(ctx context.Context, dir string) error {
	return f.purgeCheck(ctx, dir, true)
}

// Purge deletes a directory and all its contents.
func (f *Fs) Purge(ctx context.Context, dir string) error {
	root := path.Join(f.root, dir)
	if root == "" {
		return errors.New("can't purge root directory")
	}
	dirID, err := f.dirCache.FindDir(ctx, dir, false)
	if err != nil {
		return err
	}
	if err := f.purgeDir(ctx, dirID); err != nil {
		return fmt.Errorf("purge: %w", err)
	}
	f.dirCache.FlushDir(dir)
	return nil
}

// deleteBatch is the maximum number of media ids sent to a single delete call.
// The only documented limit is the server's "can't delete more than 1000
// elements", so 999 stays one under that (and under it whether the boundary is
// inclusive or not) while keeping round-trips to a minimum.
const deleteBatch = 999

// purgeDir deletes dirID and everything beneath it.
//
// The fast path is the server's own recursive folder-delete, which removes a
// whole subtree in a single request.  That call is refused once a subtree holds
// more than ~1000 elements ("can't delete more than 1000 elements"), so on any
// failure we fall back to emptying this folder ourselves: recurse into each
// sub-folder (which retries the fast path at the smaller scale) and delete the
// folder's own media in capped batches, then remove the now-empty folder.
//
// This matters for speed: most trees are wiped in a handful of recursive
// folder-deletes instead of listing every file (200 per page) and deleting it
// in batches — the round-trip count, not the byte count, dominates the time.
func (f *Fs) purgeDir(ctx context.Context, dirID string) error {
	// Fast path: one call deletes the whole subtree when it's small enough.
	if err := f.deleteFolder(ctx, dirID); err == nil {
		return nil
	}
	// Too big (or otherwise refused): drain this level and recurse.
	folders, err := f.listFolders(ctx, dirID)
	if err != nil {
		return err
	}
	for _, fl := range folders {
		if err := f.purgeDir(ctx, fl.ID.String()); err != nil {
			return err
		}
	}
	// Fully paginate the listing before deleting so we never mutate the folder
	// while a paged read of it is still in flight.
	var ids []string
	if err := f.listMedia(ctx, dirID, func(it *api.Item) {
		ids = append(ids, it.ID.String())
	}); err != nil {
		return err
	}
	for i := 0; i < len(ids); i += deleteBatch {
		end := i + deleteBatch
		if end > len(ids) {
			end = len(ids)
		}
		if err := f.deleteMedia(ctx, ids[i:end]...); err != nil {
			return err
		}
	}
	// Contents gone — the folder itself is now small enough to delete.
	return f.deleteFolder(ctx, dirID)
}

func (f *Fs) deleteMedia(ctx context.Context, ids ...string) error {
	var req api.IDsRequest
	req.Data.IDs = ids
	opts := rest.Opts{Method: "POST", Path: "/sapi/media", Parameters: url.Values{"action": {"delete"}}}
	var res api.Status
	return f.callJSON(ctx, &opts, &req, &res)
}

// CleanUp empties the server-side trash.  Deletes are soft: removed items land
// in the trash and keep counting against the storage quota until it is emptied.
// The call returns an empty body on success, hence NoResponse.
func (f *Fs) CleanUp(ctx context.Context) error {
	opts := rest.Opts{Method: "POST", Path: "/sapi/media/trash", Parameters: url.Values{"action": {"empty"}}, NoResponse: true}
	if err := f.callJSON(ctx, &opts, nil, &struct{}{}); err != nil {
		return fmt.Errorf("cleanup: %w", err)
	}
	return nil
}

// Put the object into the container
func (f *Fs) Put(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error) {
	existing, err := f.newObjectWithInfo(ctx, src.Remote(), nil)
	switch err {
	case nil:
		return existing, existing.Update(ctx, in, src, options...)
	case fs.ErrorObjectNotFound:
		o := &Object{fs: f, remote: src.Remote()}
		return o, o.Update(ctx, in, src, options...)
	default:
		return nil, err
	}
}

// About gets quota information
func (f *Fs) About(ctx context.Context) (*fs.Usage, error) {
	var res api.StorageSpace
	opts := rest.Opts{Method: "GET", Path: "/sapi/media", Parameters: url.Values{"action": {"get-storage-space"}}}
	if err := f.callJSON(ctx, &opts, nil, &res); err != nil {
		return nil, err
	}
	usage := &fs.Usage{Used: fs.NewUsageValue(res.Data.Used)}
	if !res.Data.NoLimit && res.Data.Quota > 0 {
		usage.Total = fs.NewUsageValue(res.Data.Quota)
		usage.Free = fs.NewUsageValue(res.Data.Free)
	}
	return usage, nil
}

// DirCacheFlush resets the directory cache
func (f *Fs) DirCacheFlush() { f.dirCache.ResetRoot() }

// saveFolder renames and/or re-parents a folder via action=save.  A nil parent
// leaves the folder where it is (rename only).
func (f *Fs) saveFolder(ctx context.Context, id, name string, parent *string) error {
	var req api.SaveFolderRequest
	req.Data.ID = api.ID(id)
	req.Data.Name = f.opt.Enc.FromStandardName(name)
	if parent != nil {
		p := api.ID(*parent)
		req.Data.Parent = &p
	}
	var res api.SaveFolderResponse
	opts := rest.Opts{Method: "POST", Path: "/sapi/media/folder", Parameters: url.Values{"action": {"save"}}}
	return f.callJSON(ctx, &opts, &req, &res)
}

// DirMove moves and/or renames the directory srcRemote in srcFs to dstRemote in
// f using a server-side folder save.
func (f *Fs) DirMove(ctx context.Context, src fs.Fs, srcRemote, dstRemote string) error {
	srcFs, ok := src.(*Fs)
	if !ok {
		fs.Debugf(srcFs, "Can't move directory - not same remote type")
		return fs.ErrorCantDirMove
	}
	srcID, _, _, dstDirectoryID, dstLeaf, err := f.dirCache.DirMove(ctx, srcFs.dirCache, srcFs.root, srcRemote, f.root, dstRemote)
	if err != nil {
		return err
	}
	if err := f.saveFolder(ctx, srcID, dstLeaf, &dstDirectoryID); err != nil {
		return fmt.Errorf("DirMove: %w", err)
	}
	srcFs.dirCache.FlushDir(srcRemote)
	return nil
}

// ---- Object interface ----

// Fs returns the parent Fs
func (o *Object) Fs() fs.Info { return o.fs }

// String returns the remote path
func (o *Object) String() string {
	if o == nil {
		return "<nil>"
	}
	return o.remote
}

// Remote returns the remote path
func (o *Object) Remote() string { return o.remote }

// Hash is unsupported: the service exposes no usable content hash.
func (o *Object) Hash(ctx context.Context, t hash.Type) (string, error) {
	return "", hash.ErrUnsupported
}

// Size returns the size of the object in bytes
func (o *Object) Size() int64 {
	if err := o.readMetaData(context.TODO()); err != nil {
		fs.Logf(o, "Failed to read metadata: %v", err)
		return 0
	}
	return o.size
}

// ModTime returns the modification time of the object
func (o *Object) ModTime(ctx context.Context) time.Time {
	if err := o.readMetaData(ctx); err != nil {
		fs.Logf(o, "Failed to read metadata: %v", err)
		return time.Now()
	}
	return o.modTime
}

// SetModTime is not supported standalone (mtime is set on upload)
func (o *Object) SetModTime(ctx context.Context, t time.Time) error {
	return fs.ErrorCantSetModTime
}

// Storable returns whether this object is storable
func (o *Object) Storable() bool { return true }

// ID returns the ID of the Object
func (o *Object) ID() string { return o.id }

// Open an object for read
func (o *Object) Open(ctx context.Context, options ...fs.OpenOption) (io.ReadCloser, error) {
	if err := o.readMetaData(ctx); err != nil {
		return nil, err
	}
	if o.url == "" {
		return nil, errors.New("can't download - no URL")
	}
	if err := o.fs.ensureLogin(ctx); err != nil {
		return nil, err
	}
	fs.FixRangeOption(options, o.size)
	opts := rest.Opts{Method: "GET", RootURL: o.url, Options: options}
	// The download streams from a direct URL and so bypasses callJSON's
	// SEC-1003 handling.  Replicate it: if the validation key has rotated,
	// adopt the fresh one the server returns, persist the renewed cookies and
	// retry once.  Retrying is safe here because the body hasn't been read yet.
	for attempt := 0; attempt < 2; attempt++ {
		var resp *http.Response
		err := o.fs.pacer.Call(func() (bool, error) {
			var err error
			resp, err = o.fs.srv.Call(ctx, &opts)
			return shouldRetry(ctx, resp, err)
		})
		if err == nil {
			return resp.Body, nil
		}
		if attempt == 0 && o.fs.refreshOnError(err) {
			continue
		}
		return nil, err
	}
	return nil, errors.New("funambol: download authentication retry exhausted")
}

// Update the object with the contents of the io.Reader
func (o *Object) Update(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) error {
	size := src.Size()
	if size < 0 {
		return errors.New("funambol: can't upload unknown-sized objects")
	}
	leaf, dirID, err := o.fs.dirCache.FindPath(ctx, o.remote, true)
	if err != nil {
		return err
	}
	name := o.fs.opt.Enc.FromStandardName(leaf)

	// Overwrite: the server appends " (1)", " (2)" … when a name already
	// exists rather than replacing, so remove any current same-name item
	// before uploading to keep the requested name.
	if o.id != "" {
		if dErr := o.fs.deleteMedia(ctx, o.id); dErr != nil {
			fs.Logf(o, "couldn't remove previous version before upload: %v", dErr)
		}
		o.id = ""
	}

	// Phase 1: metadata -> obtain the upload GUID
	var metaReq api.UploadMetadataRequest
	metaReq.Data.Name = name
	metaReq.Data.ContentType = fs.MimeType(ctx, src)
	metaReq.Data.Size = size
	metaReq.Data.FolderID = dirID
	mod := src.ModTime(ctx)
	metaReq.Data.CreationDate = api.FormatTime(mod)
	metaReq.Data.ModificationDate = api.FormatTime(mod)
	var metaRes api.UploadMetadataResponse
	metaOpts := rest.Opts{
		Method:     "POST",
		Path:       "/sapi/upload/" + uploadMediaType,
		Parameters: url.Values{"action": {"save-metadata"}},
	}
	if err := o.fs.callJSON(ctx, &metaOpts, &metaReq, &metaRes); err != nil {
		return fmt.Errorf("upload metadata: %w", err)
	}
	guid := metaRes.ID.String()
	if guid == "" {
		return errors.New("upload metadata: server returned no id")
	}

	// Phase 2: stream the bytes
	if err := o.fs.ensureLogin(ctx); err != nil {
		return err
	}
	var upRes api.UploadResponse
	upOpts := rest.Opts{
		Method:        "POST",
		Path:          "/sapi/upload/" + uploadMediaType,
		Parameters:    url.Values{"action": {"save"}, "validationkey": {o.fs.key()}},
		Body:          in,
		ContentType:   "application/octet-stream",
		ContentLength: &size,
		ExtraHeaders: map[string]string{
			"x-funambol-id":        guid,
			"x-funambol-file-size": strconv.FormatInt(size, 10),
		},
		Options: options,
	}
	err = o.fs.pacer.CallNoRetry(func() (bool, error) {
		resp, err := o.fs.srv.CallJSON(ctx, &upOpts, nil, &upRes)
		return shouldRetry(ctx, resp, err)
	})
	if err == nil {
		err = upRes.AsErr()
	}
	if err != nil {
		// The byte stream bypasses callJSON's SEC-1003 handling.  If the
		// validation key rotated, adopt the fresh key + renewed cookies the
		// server returned, then ask the caller to retry: the body has already
		// been consumed and can't be replayed here, so operations.Copy re-runs
		// Update with a freshly-opened reader (its phase-1 metadata call now
		// uses the new key).
		if o.fs.refreshOnError(err) {
			return fserrors.RetryError(fmt.Errorf("upload bytes: validation key rotated, retrying: %w", err))
		}
		return fmt.Errorf("upload bytes: %w", err)
	}

	// Populate metadata from the upload response.  Re-reading via a listing
	// here would race the server's indexing (the just-uploaded item is not
	// immediately listable), which previously caused spurious retries and
	// duplicate " (N)" files.
	o.id = upRes.ID.String()
	o.size = size
	o.modTime = mod
	o.hasMetaData = true
	return nil
}

// Remove an object
func (o *Object) Remove(ctx context.Context) error {
	if err := o.readMetaData(ctx); err != nil {
		return err
	}
	return o.fs.deleteMedia(ctx, o.id)
}

// ---- construction ----

// NewFs constructs an Fs from the path
func NewFs(ctx context.Context, name, root string, m configmap.Mapper) (fs.Fs, error) {
	opt := new(Options)
	if err := configstruct.Set(m, opt); err != nil {
		return nil, err
	}
	if opt.Pass != "" {
		clear, err := obscure.Reveal(opt.Pass)
		if err != nil {
			return nil, fmt.Errorf("couldn't decrypt password: %w", err)
		}
		opt.Pass = clear
	}
	if opt.Endpoint == "" {
		opt.Endpoint = defaultEndpoint
	}
	opt.Endpoint = strings.TrimRight(opt.Endpoint, "/")
	if opt.DeviceID == "" {
		opt.DeviceID = newDeviceID()
		m.Set("device_id", opt.DeviceID)
	}

	client := fshttp.NewClient(ctx)
	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, err
	}
	client.Jar = jar
	srv := rest.NewClient(client).SetRoot(opt.Endpoint)
	srv.SetErrorHandler(sapiErrorHandler)
	srv.SetHeader("X-deviceid", opt.DeviceID)

	f := &Fs{
		name:   name,
		root:   parsePath(root),
		opt:    *opt,
		m:      m,
		client: client,
		srv:    srv,
		pacer:  fs.NewPacer(ctx, pacer.NewDefault(pacer.MinSleep(minSleep), pacer.MaxSleep(maxSleep), pacer.DecayConstant(decayConstant))),
		auth:   new(authState),
	}
	f.features = (&fs.Features{
		CaseInsensitive:         true,
		CanHaveEmptyDirectories: true,
		ReadMimeType:            false,
	}).Fill(ctx, f)

	// Restore the session established during config (survives 2FA).
	if opt.Cookies != "" {
		f.restoreCookies(opt.Cookies)
	}

	// Authenticate and discover the root folder id.
	if err := f.ensureLogin(ctx); err != nil {
		return nil, err
	}
	var rootRes api.FoldersResponse
	opts := rest.Opts{Method: "GET", Path: "/sapi/media/folder/root", Parameters: url.Values{"action": {"get"}}}
	if err := f.callJSON(ctx, &opts, nil, &rootRes); err != nil {
		return nil, fmt.Errorf("couldn't find root folder: %w", err)
	}
	if len(rootRes.Data.Folders) == 0 {
		return nil, errors.New("couldn't find root folder")
	}
	f.rootID = rootRes.Data.Folders[0].ID.String()

	f.dirCache = dircache.New(f.root, f.rootID, f)
	err = f.dirCache.FindRoot(ctx, false)
	if err != nil {
		// Maybe root is a file
		newRoot, remote := dircache.SplitPath(f.root)
		tempF := *f
		tempF.dirCache = dircache.New(newRoot, f.rootID, &tempF)
		tempF.root = newRoot
		if err := tempF.dirCache.FindRoot(ctx, false); err != nil {
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

// Check the interfaces are satisfied
var (
	_ fs.Fs              = (*Fs)(nil)
	_ fs.Purger          = (*Fs)(nil)
	_ fs.CleanUpper      = (*Fs)(nil)
	_ fs.Abouter         = (*Fs)(nil)
	_ fs.DirMover        = (*Fs)(nil)
	_ fs.DirCacheFlusher = (*Fs)(nil)
	_ fs.Object          = (*Object)(nil)
	_ fs.IDer            = (*Object)(nil)
)
