// Package funambol provides an interface to Funambol / OneMediaHub
// (cloud.o2online.es) storage system, which is built on the Funambol /
// OneMediaHub "SAPI".
package funambol

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"path"
	"reflect"
	"slices"
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
	defaultEndpoint   = "https://cloud.o2online.es"
	minSleep          = 10 * time.Millisecond
	maxSleep          = 2 * time.Second
	decayConstant     = 2
	listChunk         = 200    // media items per page
	uploadMediaType   = "file" // store everything as a generic file (no transcoding)
	asyncUploadCutoff = 200 * 1024 * 1024
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
		CommandHelp: commandHelp,
		Options: []fs.Option{{
			Name:      "user",
			Help:      "Username for the Funambol / OneMediaHub account.\n\nUsually the email address or mobile number (MSISDN) you log in with.\nFor SMS login (no password) this must be the mobile number.",
			Required:  true,
			Sensitive: true,
		}, {
			Name:       "pass",
			Help:       "Password for the Funambol / OneMediaHub account.\n\nLeave empty for accounts without one (O2 Spain accounts that sign in\nvia Mi O2): rclone then logs in with an SMS code sent to the mobile\nnumber in user.  Set client_id too to get a session that renews itself.",
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
"web-<hex>" id ("fol-<base64>" with client_id) on first login and saves it
here, registering itself just like the official app.  Set it only to reuse a specific existing id.`,
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
			Name: "client_id",
			Help: `OAuth client ID of an O2 Spain desktop app.

Only for O2 Spain accounts without a password.  When set, the SMS login
gets an OAuth refresh token, so rclone creates new sessions unattended
and no new SMS code is needed when a session expires.  rclone does not
include a client ID: you must supply one.`,
			Advanced:  true,
			Sensitive: true,
		}, {
			Name:       "client_secret",
			Help:       "OAuth client secret that goes with client_id, if it has one.",
			IsPassword: true,
			Advanced:   true,
		}, {
			Name:     "oauth_platform",
			Help:     "Platform of the app that client_id belongs to, e.g. \"windows\" or \"macos\".",
			Default:  defaultOAuthPlatform,
			Advanced: true,
		}, {
			Name:      config.ConfigToken,
			Help:      "OAuth token (set automatically by rclone config).",
			Hide:      fs.OptionHideBoth,
			Sensitive: true,
			Advanced:  true,
		}, {
			Name:      configOAuthCreds,
			Help:      "OAuth token envelope last returned by the server (set automatically).",
			Hide:      fs.OptionHideBoth,
			Sensitive: true,
			Advanced:  true,
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
	User          string               `config:"user"`
	Pass          string               `config:"pass"`
	Endpoint      string               `config:"endpoint"`
	Cookies       string               `config:"cookies"`
	DeviceID      string               `config:"device_id"`
	ClientID      string               `config:"client_id"`
	ClientSecret  string               `config:"client_secret"`
	OAuthPlatform string               `config:"oauth_platform"`
	Enc           encoder.MultiEncoder `config:"encoding"`
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

	auth    *authState   // shared session state (pointer so *Fs stays copyable)
	uploads *uploadCache // items uploaded too recently to be listed
}

// recentUploadTTL is how long a finished upload stays in the uploadCache.
const recentUploadTTL = 5 * time.Minute

// uploadCache remembers recently uploaded items by path.  The server lists a
// new item only after a delay, so a lookup straight after an upload (restic
// deletes its lock files within the same second) would otherwise miss it.
type uploadCache struct {
	mu    sync.Mutex
	items map[string]uploadEntry // keyed by lower-cased absolute path; empty IDs mark removals
}

type uploadEntry struct {
	item api.Item
	at   time.Time
}

func newUploadCache() *uploadCache {
	return &uploadCache{items: make(map[string]uploadEntry)}
}

// put records item as uploaded to p and drops expired entries.
func (c *uploadCache) put(p string, item api.Item) {
	c.mu.Lock()
	defer c.mu.Unlock()
	now := time.Now()
	for k, e := range c.items {
		if now.Sub(e.at) > recentUploadTTL {
			delete(c.items, k)
		}
	}
	c.items[strings.ToLower(p)] = uploadEntry{item: item, at: now}
}

// get returns the item uploaded to p, if it is recent enough.
func (c *uploadCache) get(p string) (api.Item, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	e, ok := c.items[strings.ToLower(p)]
	if !ok || e.item.ID == "" || time.Since(e.at) > recentUploadTTL {
		return api.Item{}, false
	}
	return e.item, true
}

// remove hides stale listings of a removed or moved item until indexing catches up.
func (c *uploadCache) remove(p string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.items[strings.ToLower(p)] = uploadEntry{at: time.Now()}
}

// snapshot returns recent writes and removals, pruning expired entries.
func (c *uploadCache) snapshot() map[string]uploadEntry {
	c.mu.Lock()
	defer c.mu.Unlock()
	items := make(map[string]uploadEntry, len(c.items))
	for p, e := range c.items {
		if time.Since(e.at) > recentUploadTTL {
			delete(c.items, p)
		} else {
			items[p] = e
		}
	}
	return items
}

// clear forgets every item.  Directory moves and purges call it, since they
// change or remove items under paths the cache does not track.
func (c *uploadCache) clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	clear(c.items)
}

// authState serialises (re-)authentication.  The session itself lives in the
// http.Client cookie jar (JSESSIONID + persistent-login cookie) and the
// validation key, so only a mutex is needed to guard concurrent refreshes.
// tokenMu guards the saved OAuth token, which every SAPI reply may renew.
type authState struct {
	mu      sync.Mutex
	tokenMu sync.Mutex
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
	// "Unknown exception in media handling" — seen in bursts together with
	// HTTP 408/503; the official app retries save-metadata on it.
	case "FOL-1000", "MED-1000":
		return true
	}
	return false
}

// isFatalCode reports SAPI error codes for hard account limits — FOL-1035
// (maximum folder count per user) and FOL-1015 (maximum folder depth).
// Retrying cannot succeed, so these must abort the run instead of letting
// rclone's retry layers re-run the whole transfer.
func isFatalCode(code string) bool {
	switch code {
	case "FOL-1015", "FOL-1035":
		return true
	}
	return false
}

// wrapFatal marks hard-limit SAPI errors as fatal for rclone's retry logic.
func wrapFatal(err error) error {
	var ae *api.Error
	if errors.As(err, &ae) && isFatalCode(ae.Code) {
		return fserrors.FatalError(err)
	}
	return err
}

// ensureLogin makes sure we have something to authenticate with: either a
// live validation key or the persistent login cookie that can mint one, or
// else a new session from the OAuth token.  Logging in from scratch needs the
// emailed verification code, so it can only happen interactively via
// "rclone config".
func (f *Fs) ensureLogin(ctx context.Context) error {
	f.auth.mu.Lock()
	defer f.auth.mu.Unlock()
	if f.validationKeyFromJar() != "" || f.persistentLoginPresent() {
		return nil
	}
	if f.opt.usesOAuth() {
		return f.oauthLogin(ctx)
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
		vk := f.key()
		if vk != "" {
			opts.Parameters.Set("validationkey", vk)
		} else {
			opts.Parameters.Del("validationkey")
		}
		var resp *http.Response
		err := f.pacer.Call(func() (bool, error) {
			var err error
			// Decoding leaves fields the reply doesn't carry untouched, so a
			// retry must not see the error envelope of the previous attempt.
			if v := reflect.ValueOf(response); v.Kind() == reflect.Pointer && !v.IsNil() {
				v.Elem().SetZero()
			}
			requestOpts := opts
			if opts.GetBody != nil {
				body, bodyErr := opts.GetBody()
				if bodyErr != nil {
					return false, bodyErr
				}
				defer fs.CheckClose(body, &err)
				requestOpts = opts.Copy()
				requestOpts.Body = body
			}
			resp, err = f.srv.CallJSON(ctx, requestOpts, request, response)
			err = redactKey(err)
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
			if f.refreshOnError(err, vk) && attempt < 2 {
				continue
			}
			if attempt < 2 {
				if retry, rErr := f.reloginOnError(ctx, resp, err, vk); rErr != nil {
					return rErr
				} else if retry {
					continue
				}
			}
			var ae *api.Error
			if errors.As(err, &ae) && (resp != nil && resp.StatusCode == http.StatusUnauthorized || isReauthCode(ae.Code)) {
				// A concurrent call rotated the session while this one was
				// in flight, so the rejection may only concern the old key.
				if f.key() != vk && attempt < 2 {
					continue
				}
				fs.Debugf(f, "session expired: %s (persistent login cookie present: %v)", ae.Code, f.persistentLoginPresent())
				return errReconnect(f.name)
			}
			return wrapFatal(err)
		}
		// SAPI may signal failure with HTTP 200 + an error envelope.
		if er, ok := response.(interface{ AsErr() error }); ok {
			if aErr := er.AsErr(); aErr != nil {
				if f.refreshOnError(aErr, vk) && attempt < 2 {
					continue
				}
				if attempt < 2 {
					if retry, rErr := f.reloginOnError(ctx, nil, aErr, vk); rErr != nil {
						return rErr
					} else if retry {
						continue
					}
				}
				return wrapFatal(aErr)
			}
		}
		return nil
	}
	return errors.New("funambol: authentication retry exhausted")
}

// refreshOnError handles a SEC-1003 ("invalid mandatory validation key") for
// a request sent with the key sent, by adopting the fresh key the server
// hands back and persisting the renewed cookies.  It reports whether the
// caller should retry.
//
// If another call has replaced the key since sent was read, that key is
// newer than the one in this reply (a long upload can return long after
// other calls rotated the key), so it is kept and the caller just retries.
func (f *Fs) refreshOnError(err error, sent string) bool {
	var ae *api.Error
	if !errors.As(err, &ae) || ae.Code != "SEC-1003" {
		return false
	}
	f.auth.mu.Lock()
	defer f.auth.mu.Unlock()
	if f.key() != sent {
		return true
	}
	if ae.Data == "" {
		return false
	}
	f.setValidationKey(ae.Data)
	f.persistSession() // the 401 also renewed the persistent login cookie
	return true
}

// redactKey hides the validation key in the URL that net/http errors quote,
// so the key does not end up in the logs.
func redactKey(err error) error {
	var ue *url.Error
	if errors.As(err, &ue) {
		if u, pErr := url.Parse(ue.URL); pErr == nil && u.Query().Has("validationkey") {
			q := u.Query()
			q.Set("validationkey", "REDACTED")
			u.RawQuery = q.Encode()
			ue.URL = u.String()
		}
	}
	return err
}

// downloadURL returns raw with the current validation key added, as the
// official app does on every SAPI call.  URLs on other hosts are returned
// unchanged: they may be signed, and an extra parameter would break that.
func (f *Fs) downloadURL(raw string) string {
	u, err := url.Parse(raw)
	key := f.key()
	if err != nil || key == "" || !strings.EqualFold(u.Host, f.jarURL().Host) {
		return raw
	}
	q := u.Query()
	q.Set("validationkey", key)
	u.RawQuery = q.Encode()
	return u.String()
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

// listFileIDs returns the id of every live file in the account, from the
// profile-changes feed queried from time zero.  This is the only enumeration
// that includes items detached from any folder: a server-side folder delete
// orphans the contained files, which then vanish from every folder listing
// (and from the folder-less media listing, which returns nothing) but remain
// in the changes feed and keep counting against the quota.
func (f *Fs) listFileIDs(ctx context.Context) ([]string, error) {
	var res api.ChangesResponse
	opts := rest.Opts{
		Method: "GET",
		Path:   "/sapi/profile/changes",
		Parameters: url.Values{
			"action": {"get"},
			"from":   {"0"},
			"origin": {"omh,dropbox"},
		},
	}
	if err := f.callJSON(ctx, &opts, nil, &res); err != nil {
		return nil, fmt.Errorf("list file ids: %w", err)
	}
	ids := make([]string, 0, len(res.Data.File.Updated))
	for _, id := range res.Data.File.Updated {
		if id.String() != "" {
			ids = append(ids, id.String())
		}
	}
	return ids, nil
}

// hydrateBatch is how many ids are fetched per by-id media call (the official
// web client uses 400).
const hydrateBatch = 400

// getMediaByIDs fetches the given media items by id.  Only fields from the
// server's accepted set may be requested (an unknown name fails the call with
// COM-1021); "size" and "name" are known-good.
func (f *Fs) getMediaByIDs(ctx context.Context, ids []string, fields []string) ([]api.Item, error) {
	var req api.IDFieldsRequest
	req.Data.Fields = fields
	req.Data.IDs = numericIDs(ids)
	var res api.MediaResponse
	opts := rest.Opts{
		Method:     "POST",
		Path:       "/sapi/media",
		Parameters: url.Values{"action": {"get"}, "origin": {"omh,dropbox"}},
	}
	if err := f.callJSON(ctx, &opts, &req, &res); err != nil {
		return nil, fmt.Errorf("get media by ids: %w", err)
	}
	return res.Data.Media, nil
}

// numericIDs converts string ids to the JSON-number form SAPI expects,
// dropping (and logging) anything non-numeric rather than corrupting the
// request body.
func numericIDs(ids []string) []json.Number {
	out := make([]json.Number, 0, len(ids))
	for _, id := range ids {
		if _, err := strconv.ParseInt(id, 10, 64); err != nil {
			fs.Errorf(nil, "funambol: skipping non-numeric media id %q", id)
			continue
		}
		out = append(out, json.Number(id))
	}
	return out
}

// walkFolderIDs records dirID and every folder id beneath it into seen.
func (f *Fs) walkFolderIDs(ctx context.Context, dirID string, seen map[string]struct{}) error {
	seen[dirID] = struct{}{}
	folders, err := f.listFolders(ctx, dirID)
	if err != nil {
		return err
	}
	for _, fl := range folders {
		if err := f.walkFolderIDs(ctx, fl.ID.String(), seen); err != nil {
			return err
		}
	}
	return nil
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
	recent := f.uploads.snapshot()
	err = f.listMedia(ctx, dirID, func(item *api.Item) {
		if item.Name == "" {
			return
		}
		remote := path.Join(dir, f.opt.Enc.ToStandardName(item.Name))
		key := strings.ToLower(f.absPath(remote))
		if cached, ok := recent[key]; ok {
			if cached.item.ID == "" || cached.item.ID != item.ID {
				return
			}
			delete(recent, key)
		}
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
	for key, e := range recent {
		if e.item.ID == "" || path.Dir(key) != strings.ToLower(path.Clean(f.absPath(dir))) {
			continue
		}
		remote := path.Join(dir, f.opt.Enc.ToStandardName(e.item.Name))
		o, err := f.newObjectWithInfo(ctx, remote, &e.item)
		if err != nil {
			return nil, err
		}
		entries = append(entries, o)
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
	if cached, ok := f.uploads.snapshot()[strings.ToLower(f.absPath(remote))]; ok && cached.item.ID == "" {
		return nil, fs.ErrorObjectNotFound
	}
	leaf, dirID, err := f.dirCache.FindPath(ctx, remote, false)
	if err != nil {
		if err == fs.ErrorDirNotFound {
			return nil, fs.ErrorObjectNotFound
		}
		return nil, err
	}
	lcLeaf := strings.ToLower(leaf)
	cached, isCached := f.uploads.get(f.absPath(remote))
	var found, exact *api.Item
	err = f.listMedia(ctx, dirID, func(item *api.Item) {
		if strings.ToLower(f.opt.Enc.ToStandardName(item.Name)) != lcLeaf {
			return
		}
		it := *item
		if found == nil {
			found = &it
		}
		if isCached && exact == nil && it.ID == cached.ID {
			exact = &it
		}
	})
	if err != nil {
		return nil, err
	}
	// Prefer what this Fs uploaded last: the listing may not show it yet.
	switch {
	case exact != nil:
		return exact, nil
	case isCached:
		return &cached, nil
	case found != nil:
		return found, nil
	}
	return nil, fs.ErrorObjectNotFound
}

// absPath returns the path of remote from the account root, the key for
// the uploadCache.
func (f *Fs) absPath(remote string) string {
	return path.Join(f.root, remote)
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
	f.uploads.clear()
	if err := f.purgeDir(ctx, dirID); err != nil {
		return fmt.Errorf("purge: %w", err)
	}
	f.dirCache.FlushDir(dir)
	return nil
}

// deleteBatch is the maximum number of media ids sent to a single delete call.
// media?action=delete rejects bigger batches with MED-1025 ("Number of items
// in the request is over the limit"); the boundary was measured against the
// live service: 500 is accepted, 501 is refused.
const deleteBatch = 500

// purgeDir deletes dirID and everything beneath it.
//
// The server's own recursive folder-delete must NOT be used on a folder that
// still has files in it: it removes the folder tree but merely detaches the
// files, which disappear from every folder listing yet live on in the
// account-wide media listing (the web app's "Documents" view) and keep
// counting against the storage quota forever.  So each folder's media is
// deleted explicitly in capped batches, sub-folders are recursed into, and
// only then is the empty folder itself removed.
func (f *Fs) purgeDir(ctx context.Context, dirID string) error {
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
	// Contents gone — deleting the now-empty folder can't orphan anything.
	return f.deleteFolder(ctx, dirID)
}

func (f *Fs) deleteMedia(ctx context.Context, ids ...string) error {
	var req api.IDsRequest
	req.Data.IDs = numericIDs(ids)
	if len(req.Data.IDs) == 0 {
		return nil
	}
	opts := rest.Opts{Method: "POST", Path: "/sapi/media", Parameters: url.Values{"action": {"delete"}}}
	var res api.Status
	return f.callJSON(ctx, &opts, &req, &res)
}

// commandHelp describes the backend-specific commands.
var commandHelp = []fs.CommandHelp{{
	Name:  "cleanup-orphans",
	Short: "Delete files left behind by server-side folder deletes",
	Long: `The service's folder-delete detaches the files a folder contains
instead of removing them: they disappear from every folder listing but
live on (visible in the web app's "Documents" view) and keep counting
against the storage quota.

This command enumerates every file in the account via the
profile-changes feed — the only server-side enumeration that includes
detached items — compares that against what is reachable through the
folder tree, and deletes the difference by id.  Deleted items land in
the trash, so follow up with "rclone cleanup remote:" to actually free
the quota.

Usage:

    rclone backend cleanup-orphans remote:

It returns a JSON object with the number of orphans deleted and their
total size in bytes.`,
}}

// cleanupOrphans deletes every media item that is not reachable through the
// folder tree.  An item is an orphan when its id appears in the account-wide
// changes feed but in no folder's listing — which is what a server-side
// folder delete leaves behind.  Reachability is judged purely by comparing
// the two enumerations, never by an item's own folderid (the server omits
// that field for detached items, so trusting it would be ambiguous).
func (f *Fs) cleanupOrphans(ctx context.Context) (any, error) {
	all, err := f.listFileIDs(ctx)
	if err != nil {
		return nil, fmt.Errorf("cleanup-orphans: %w", err)
	}
	live := make(map[string]struct{})
	if err := f.walkFolderIDs(ctx, f.rootID, live); err != nil {
		return nil, fmt.Errorf("cleanup-orphans: walking folders: %w", err)
	}
	reachable := make(map[string]struct{})
	for folderID := range live {
		err := f.listMedia(ctx, folderID, func(it *api.Item) {
			reachable[it.ID.String()] = struct{}{}
		})
		if err != nil {
			return nil, fmt.Errorf("cleanup-orphans: listing folder contents: %w", err)
		}
	}
	var orphans []string
	for _, id := range all {
		if _, ok := reachable[id]; !ok {
			orphans = append(orphans, id)
		}
	}
	fs.Infof(f, "cleanup-orphans: %d file(s) in account, %d reachable via folders, %d orphaned", len(all), len(reachable), len(orphans))
	// Size accounting is best-effort: the deletion below works from ids alone.
	var bytes int64
	for i := 0; i < len(orphans); i += hydrateBatch {
		end := min(i+hydrateBatch, len(orphans))
		items, err := f.getMediaByIDs(ctx, orphans[i:end], []string{"size", "name"})
		if err != nil {
			fs.Logf(f, "cleanup-orphans: couldn't fetch sizes (continuing): %v", err)
			bytes = -1
			break
		}
		for i := range items {
			bytes += items[i].Size
		}
	}
	for i := 0; i < len(orphans); i += deleteBatch {
		end := min(i+deleteBatch, len(orphans))
		if err := f.deleteMedia(ctx, orphans[i:end]...); err != nil {
			return nil, fmt.Errorf("cleanup-orphans: deleting: %w", err)
		}
		fs.Infof(f, "cleanup-orphans: deleted %d/%d orphaned item(s)", end, len(orphans))
	}
	return map[string]any{"deleted": len(orphans), "bytes": bytes}, nil
}

// Command executes a backend-specific command.
func (f *Fs) Command(ctx context.Context, name string, arg []string, opt map[string]string) (any, error) {
	switch name {
	case "cleanup-orphans":
		return f.cleanupOrphans(ctx)
	default:
		return nil, fs.ErrorCommandNotFound
	}
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

// Put the object into the container.  No existence check here: Update sweeps
// every same-name copy from a fresh listing before uploading anyway.
func (f *Fs) Put(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error) {
	o := &Object{fs: f, remote: src.Remote()}
	return o, o.Update(ctx, in, src, options...)
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

// waitForFile waits for validation and, after a move/copy, the expected metadata.
func (f *Fs) waitForFile(ctx context.Context, id, name, folderID string, size int64) (*api.Item, error) {
	for {
		items, err := f.getMediaByIDs(ctx, []string{id}, []string{"name", "size", "url", "folderid", "modificationdate"})
		if err != nil {
			return nil, err
		}
		for _, item := range items {
			if item.ID.String() != id {
				continue
			}
			if item.FolderID == "" {
				item.FolderID = item.Folder
			}
			if item.MediaType != "" && item.MediaType != uploadMediaType {
				return &item, nil
			}
			if (item.Status == "U" || item.Status == "N") && item.URL != "" &&
				(name == "" || item.Name == name) && (folderID == "" || item.FolderID.String() == folderID) &&
				(size < 0 || item.Size == size) {
				return &item, nil
			}
		}
		if err := waitFileOperation(ctx); err != nil {
			return nil, fmt.Errorf("waiting for file %s validation or metadata: %w", id, err)
		}
	}
}

func waitFileOperation(ctx context.Context) error {
	timer := time.NewTimer(time.Second)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

// callFileOperation also retries validation races, within the operation's deadline.
func (f *Fs) callFileOperation(ctx context.Context, opts *rest.Opts, req, res any) error {
	for {
		err := f.callJSON(ctx, opts, req, res)
		var ae *api.Error
		if !errors.As(err, &ae) || ae.Code != "MED-1017" {
			return err
		}
		if err := waitFileOperation(ctx); err != nil {
			return err
		}
	}
}

// fileOperationSource rejects other accounts and untested media types before any writes.
func (f *Fs) fileOperationSource(ctx context.Context, src fs.Object, cant error) (*Object, *api.Item, error) {
	o, ok := src.(*Object)
	if !ok || o.fs.name != f.name || o.fs.opt.Endpoint != f.opt.Endpoint || o.fs.rootID == "" || o.fs.rootID != f.rootID {
		return nil, nil, cant
	}
	if err := o.readMetaData(ctx); err != nil {
		return nil, nil, err
	}
	if id, err := strconv.ParseInt(o.id, 10, 64); err != nil || id <= 0 {
		return nil, nil, errors.New("invalid source media ID")
	}
	item, err := o.fs.waitForFile(ctx, o.id, "", "", -1)
	if err != nil {
		return nil, nil, err
	}
	if item.MediaType != "" && item.MediaType != uploadMediaType {
		return nil, nil, cant
	}
	return o, item, nil
}

// removeFileDestination prevents SAPI from silently adding a duplicate-name suffix.
func (f *Fs) removeFileDestination(ctx context.Context, remote, dirID, leaf, sourceID string) error {
	ids := []string{}
	if cached, ok := f.uploads.get(f.absPath(remote)); ok && cached.ID.String() != sourceID {
		ids = append(ids, cached.ID.String())
	}
	if err := f.listMedia(ctx, dirID, func(item *api.Item) {
		id := item.ID.String()
		if id != "" && id != sourceID && strings.EqualFold(f.opt.Enc.ToStandardName(item.Name), leaf) && !slices.Contains(ids, id) {
			ids = append(ids, id)
		}
	}); err != nil {
		return err
	}
	if len(ids) != 0 {
		if err := f.deleteMedia(ctx, ids...); err != nil {
			return err
		}
		f.uploads.remove(f.absPath(remote))
	}
	return nil
}

// Copy copies generic files on the same configured remote without transferring their contents.
// Unsupported sources return fs.ErrorCantCopy.
func (f *Fs) Copy(ctx context.Context, src fs.Object, remote string) (fs.Object, error) {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	o, item, err := f.fileOperationSource(ctx, src, fs.ErrorCantCopy)
	if err != nil {
		return nil, err
	}
	leaf, dirID, err := f.dirCache.FindPath(ctx, remote, true)
	if err != nil {
		return nil, err
	}
	name := f.opt.Enc.FromStandardName(leaf)
	if item.FolderID.String() == dirID && strings.EqualFold(item.Name, name) {
		return nil, errors.New("can't copy a file onto itself")
	}
	if err := f.removeFileDestination(ctx, remote, dirID, leaf, o.id); err != nil {
		return nil, err
	}
	var req api.CopyFileRequest
	req.Data.ID, req.Data.ParentID, req.Data.Name = json.Number(o.id), json.Number(dirID), name
	var res api.CopyFileResponse
	opts := rest.Opts{Method: "POST", Path: "/sapi/media/file", Parameters: url.Values{"action": {"copy"}}}
	if err := f.callFileOperation(ctx, &opts, &req, &res); err != nil {
		return nil, fmt.Errorf("copy file: %w", err)
	}
	id := res.Data.ID.String()
	if n, err := strconv.ParseInt(id, 10, 64); err != nil || n <= 0 || id == o.id || res.Data.Name != name {
		return nil, fmt.Errorf("copy file: invalid result (ID %q, name %q)", id, res.Data.Name)
	}
	copied, err := f.waitForFile(ctx, id, name, dirID, item.Size)
	if err != nil {
		return nil, err
	}
	f.uploads.put(f.absPath(remote), *copied)
	return f.newObjectWithInfo(ctx, remote, copied)
}

// Move moves generic files on the same configured remote using metadata only.
// Extension changes return fs.ErrorCantMove so rclone can use server-side copy/delete.
func (f *Fs) Move(ctx context.Context, src fs.Object, remote string) (fs.Object, error) {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	o, item, err := f.fileOperationSource(ctx, src, fs.ErrorCantMove)
	if err != nil {
		return nil, err
	}
	name := f.opt.Enc.FromStandardName(path.Base(remote))
	// SAPI silently retains the old extension when updating metadata.
	if path.Ext(item.Name) != path.Ext(name) {
		return nil, fs.ErrorCantMove
	}
	leaf, dirID, err := f.dirCache.FindPath(ctx, remote, true)
	if err != nil {
		return nil, err
	}
	if item.FolderID.String() == dirID && item.Name == name {
		return f.newObjectWithInfo(ctx, remote, item)
	}
	if err := f.removeFileDestination(ctx, remote, dirID, leaf, o.id); err != nil {
		return nil, err
	}
	var req api.MoveFileRequest
	req.Data.ID, req.Data.Name, req.Data.FolderID = api.ID(o.id), name, json.Number(dirID)
	req.Data.ModificationDate = item.ModTime().UTC().Format("20060102T150405Z")
	encoded, err := json.Marshal(&req)
	if err != nil {
		return nil, err
	}
	body := url.Values{"data": {string(encoded)}}.Encode()
	opts := rest.Opts{
		Method: "POST", Path: "/sapi/upload/file", Parameters: url.Values{"action": {"save-metadata"}},
		ContentType: "application/x-www-form-urlencoded;charset=UTF-8",
		GetBody:     func() (io.ReadCloser, error) { return io.NopCloser(strings.NewReader(body)), nil },
	}
	var res api.UploadMetadataResponse
	if err := f.callFileOperation(ctx, &opts, nil, &res); err != nil {
		return nil, fmt.Errorf("move file: %w", err)
	}
	if res.ID.String() != o.id {
		return nil, errors.New("move file: server returned a different media ID")
	}
	moved, err := f.waitForFile(ctx, o.id, name, dirID, item.Size)
	if err != nil {
		return nil, err
	}
	o.fs.uploads.remove(o.fs.absPath(o.remote))
	f.uploads.remove(o.fs.absPath(o.remote))
	o.fs.uploads.put(f.absPath(remote), *moved)
	f.uploads.put(f.absPath(remote), *moved)
	return f.newObjectWithInfo(ctx, remote, moved)
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
	srcFs.uploads.clear()
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
	if err := o.fs.ensureLogin(ctx); err != nil {
		return nil, err
	}
	fs.FixRangeOption(options, o.size)
	// The download streams from a direct URL and so bypasses callJSON's
	// session handling.  Replicate it and retry once, which is safe because
	// the body hasn't been read yet: on SEC-1003 adopt the fresh key, and on
	// any other refusal (the service may answer 403 with its HTML app page)
	// read the metadata again, which renews the key and the URL.
	for attempt := 0; attempt < 2; attempt++ {
		if o.url == "" {
			if err := o.fetchURL(ctx); err != nil {
				return nil, err
			}
		}
		vk := o.fs.key()
		opts := rest.Opts{Method: "GET", RootURL: o.fs.downloadURL(o.url), Options: options}
		var resp *http.Response
		err := o.fs.pacer.Call(func() (bool, error) {
			var err error
			resp, err = o.fs.srv.Call(ctx, &opts)
			return shouldRetry(ctx, resp, redactKey(err))
		})
		if err == nil {
			return resp.Body, nil
		}
		if attempt > 0 {
			return nil, err
		}
		if o.fs.refreshOnError(err, vk) {
			continue
		}
		if retry, rErr := o.fs.reloginOnError(ctx, resp, err, vk); rErr != nil {
			return nil, rErr
		} else if retry {
			continue
		}
		if resp != nil && (resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden) {
			fs.Debugf(o, "download refused, reading metadata again: %v", err)
			o.hasMetaData = false
			o.url = ""
			if err := o.readMetaData(ctx); err != nil {
				return nil, err
			}
			continue
		}
		return nil, err
	}
	return nil, errors.New("funambol: download authentication retry exhausted")
}

// fetchURL looks up the download URL by id.  An item known only from its
// upload reply has no URL, and the folder listing may not show it yet.
func (o *Object) fetchURL(ctx context.Context) error {
	if o.id == "" {
		return errors.New("can't download - no URL")
	}
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	for {
		items, err := o.fs.getMediaByIDs(ctx, []string{o.id}, []string{"url"})
		if err != nil {
			return err
		}
		for _, item := range items {
			if item.ID.String() == o.id && item.URL != "" {
				o.url = item.URL
				return nil
			}
		}
		if err := waitFileOperation(ctx); err != nil {
			return fmt.Errorf("waiting for download URL: %w", err)
		}
	}
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
	// before uploading to keep the requested name.  Sweep ALL copies from a
	// fresh listing, not just the one this object knows about: a retried
	// upload whose first attempt timed out after the server had committed it
	// leaves an extra same-name copy behind (the fresh copy is not listable
	// straight away, so the retry cannot see it and delete it).
	ids := make([]string, 0, 1)
	if o.id != "" {
		ids = append(ids, o.id)
	}
	absPath := o.fs.absPath(o.remote)
	if cached, ok := o.fs.uploads.get(absPath); ok && cached.ID.String() != o.id {
		ids = append(ids, cached.ID.String())
	}
	o.fs.uploads.remove(absPath)
	lcLeaf := strings.ToLower(leaf)
	lErr := o.fs.listMedia(ctx, dirID, func(item *api.Item) {
		id := item.ID.String()
		if id == "" || slices.Contains(ids, id) {
			return
		}
		if strings.ToLower(o.fs.opt.Enc.ToStandardName(item.Name)) == lcLeaf {
			ids = append(ids, id)
		}
	})
	if lErr != nil {
		fs.Logf(o, "couldn't check for previous versions before upload: %v", lErr)
	}
	if len(ids) > 0 {
		if dErr := o.fs.deleteMedia(ctx, ids...); dErr != nil {
			fs.Logf(o, "couldn't remove previous version(s) before upload: %v", dErr)
		}
	}
	o.id = ""

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
	vk := o.fs.key()
	upOpts := rest.Opts{
		Method:        "POST",
		Path:          "/sapi/upload/" + uploadMediaType,
		Parameters:    url.Values{"action": {"save"}, "validationkey": {vk}},
		Body:          in,
		ContentType:   "application/octet-stream",
		ContentLength: &size,
		ExtraHeaders: map[string]string{
			"x-funambol-id":        guid,
			"x-funambol-file-size": strconv.FormatInt(size, 10),
		},
		Options: options,
	}
	// Match the web client's large-file uploads so server-side processing
	// does not keep the request open until the gateway times out.
	if size > asyncUploadCutoff {
		upOpts.Parameters.Set("acceptasynchronous", "true")
	}
	var resp *http.Response
	err = o.fs.pacer.CallNoRetry(func() (bool, error) {
		var err error
		resp, err = o.fs.srv.CallJSON(ctx, &upOpts, nil, &upRes)
		return shouldRetry(ctx, resp, redactKey(err))
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
		if o.fs.refreshOnError(err, vk) {
			return fserrors.RetryError(fmt.Errorf("upload bytes: validation key rotated, retrying: %w", err))
		}
		if retry, rErr := o.fs.reloginOnError(ctx, resp, err, vk); rErr != nil {
			return rErr
		} else if retry {
			return fserrors.RetryError(fmt.Errorf("upload bytes: session renewed, retrying: %w", err))
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
	o.url = ""
	o.hasMetaData = true
	if o.id != "" {
		o.fs.uploads.put(absPath, api.Item{ID: upRes.ID, Name: name, Size: size, ModificationDate: mod.UnixMilli()})
	}
	return nil
}

// Remove an object
func (o *Object) Remove(ctx context.Context) error {
	if err := o.readMetaData(ctx); err != nil {
		return err
	}
	if err := o.fs.deleteMedia(ctx, o.id); err != nil {
		return err
	}
	o.fs.uploads.remove(o.fs.absPath(o.remote))
	return nil
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
		opt.DeviceID = newDeviceID(opt.usesOAuth())
		m.Set("device_id", opt.DeviceID)
	}

	// The service rejects any User-Agent starting with "rclone" (HTTP 401
	// from the CDN, before SAPI), so every call must look like a browser.
	ctx, ci := fs.AddConfig(ctx)
	ci.UserAgent = browserUA

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
		name:    name,
		root:    parsePath(root),
		opt:     *opt,
		m:       m,
		client:  client,
		srv:     srv,
		pacer:   fs.NewPacer(ctx, pacer.NewDefault(pacer.MinSleep(minSleep), pacer.MaxSleep(maxSleep), pacer.DecayConstant(decayConstant))),
		auth:    new(authState),
		uploads: newUploadCache(),
	}
	f.features = (&fs.Features{
		CaseInsensitive:         true,
		CanHaveEmptyDirectories: true,
		ReadMimeType:            false,
		// The server stores multiple items with the same name in the same
		// folder (e.g. when an upload is retried after a timeout), so expose
		// that to rclone and make "rclone dedupe" usable for repair.
		DuplicateFiles: true,
	}).Fill(ctx, f)
	f.useOAuthTransport()

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
	_ fs.Commander       = (*Fs)(nil)
	_ fs.CleanUpper      = (*Fs)(nil)
	_ fs.Abouter         = (*Fs)(nil)
	_ fs.DirMover        = (*Fs)(nil)
	_ fs.Copier          = (*Fs)(nil)
	_ fs.Mover           = (*Fs)(nil)
	_ fs.DirCacheFlusher = (*Fs)(nil)
	_ fs.Object          = (*Object)(nil)
	_ fs.IDer            = (*Object)(nil)
)
