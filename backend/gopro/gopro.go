// Package gopro provides an interface to GoPro Media Library.
//
// GoPro Media Library has no published API. This backend is built on reverse
// engineering the gopro.com web app, cross-checked against community
// clients (github.com/dustin/gopro-plus, github.com/mvisonneau/gpcd,
// github.com/aricha/GoProcure, github.com/itsankoff/gopro-plus). GoPro can
// change or remove this API at any time without notice.
package gopro

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"path"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/rclone/rclone/backend/gopro/api"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config"
	"github.com/rclone/rclone/fs/config/configmap"
	"github.com/rclone/rclone/fs/config/configstruct"
	"github.com/rclone/rclone/fs/config/obscure"
	"github.com/rclone/rclone/fs/dirtree"
	"github.com/rclone/rclone/fs/fserrors"
	"github.com/rclone/rclone/fs/fshttp"
	"github.com/rclone/rclone/fs/hash"
	"github.com/rclone/rclone/lib/encoder"
	"github.com/rclone/rclone/lib/multipart"
	"github.com/rclone/rclone/lib/oauthutil"
	"github.com/rclone/rclone/lib/pacer"
	"github.com/rclone/rclone/lib/rest"
	"golang.org/x/oauth2"
)

// Constants
const (
	rootURL       = "https://api.gopro.com"
	minSleep      = 100 * time.Millisecond
	maxSleep      = 5 * time.Second
	decayConstant = 2 // bigger for slower decay, exponential

	// dlCacheTTL bounds how long a /media/{id}/download response (which
	// carries short-lived signed CDN URLs) is reused for.
	dlCacheTTL = 4 * time.Minute

	defaultUploadChunkSize   = fs.SizeSuffix(6 * 1024 * 1024) // matches the reference client
	defaultUploadConcurrency = 4

	// minUploadChunkSize is enforced by GoPro's S3-backed upload for every
	// part but the last (AWS S3's own multipart minimum) - confirmed live:
	// a smaller chunk size gets "PartSize is less than < 5242880" back
	// from the API on the very first chunk.
	minUploadChunkSize = fs.SizeSuffix(5 * 1024 * 1024)

	// verify_size modes - see that option's Help text.
	verifySizeReprocessed = "reprocessed"
	verifySizeAlways      = "always"
	verifySizeOff         = "off"
)

// checkUploadChunkSize checks that cs is a legal upload chunk size
func checkUploadChunkSize(cs fs.SizeSuffix) error {
	if cs < minUploadChunkSize {
		return fmt.Errorf("upload chunk size %v is less than the minimum of %v", cs, minUploadChunkSize)
	}
	return nil
}

// checkVerifySizeMode checks that mode is a legal verify_size value
func checkVerifySizeMode(mode string) error {
	switch mode {
	case verifySizeReprocessed, verifySizeAlways, verifySizeOff:
		return nil
	default:
		return fmt.Errorf("unknown verify_size %q (must be %q, %q or %q)", mode, verifySizeReprocessed, verifySizeAlways, verifySizeOff)
	}
}

// shouldVerifySize decides whether Size should make a live check for an
// object with a known size, given verify_size's mode and whether this
// object's medium has been reprocessed since upload.
func shouldVerifySize(mode string, reprocessed bool) bool {
	switch mode {
	case verifySizeOff:
		return false
	case verifySizeAlways:
		return true
	default: // verifySizeReprocessed
		return reprocessed
	}
}

// setUploadChunkSize changes the chunk size used for upload, returning the
// previous value
func (f *Fs) setUploadChunkSize(cs fs.SizeSuffix) (old fs.SizeSuffix, err error) {
	err = checkUploadChunkSize(cs)
	if err == nil {
		old, f.opt.UploadChunkSize = f.opt.UploadChunkSize, cs
	}
	return
}

const (
	mediaAcceptHeader       = "application/vnd.gopro.jk.media+json; version=2.0.0"
	userUploadsAcceptHeader = "application/vnd.gopro.jk.user-uploads+json; version=2.0.0"

	// mediaFields is the set of /media/search fields this backend reads.
	mediaFields = "id,filename,file_extension,type,captured_at,created_at,file_size,width,height,camera_model,item_count,moments_count,ready_to_view,token,content_title,resolution,reprocessed_at"

	// includedTypes is the default type filter - it excludes "MultiClipEdit"
	// and "Edit" (server-generated Highlights and user-made Edits), which
	// --gopro-include-edits opts back into - see editTypes and mediaTypes.
	includedTypes = "Photo,Video,TimeLapse,TimeLapseVideo,Burst,BurstVideo,Chaptered,Continuous,Livestream,Looped,LoopedVideo,ExternalVideo,Session,Audio"

	// editTypes are the composed/derived media types --gopro-include-edits
	// opts into - not included by default because they carry a null
	// file_size and a file_extension that doesn't match what's actually
	// downloaded (see setMetaData and selectRendition).
	editTypes = "MultiClipEdit,Edit"

	// capturedRangeLayout matches the millisecond-precision UTC timestamp
	// format the web app sends for the captured_range/range parameters.
	capturedRangeLayout = "2006-01-02T15:04:05.000Z"
)

// oauthConfig describes how to authenticate against GoPro Media Library.
//
// The client ID and secret are a public constant embedded in GoPro's own
// web app, reverse engineered by the community (see package doc); they are
// obscured only to keep automated secret scanners quiet, not for security.
var oauthConfig = &oauthutil.Config{
	ClientID:     "71611e67ea968cfacf45e2b6936c81156fcf5dbe553a2bf2d342da1562d05f46",
	ClientSecret: obscure.MustReveal("9KZj9CSM0mtYdMRu0vRJyVoDF8Wp3FY3-QwBaMNvEaE_i6yk5yPHVOo5iw2zqvetyKRbp3kGkx80R9eK2J3aR9iyPEgR118UHkFwUe3DH6A"),
	TokenURL:     rootURL + "/v1/oauth2/token",
	AuthStyle:    oauth2.AuthStyleInParams,
	Scopes:       []string{"root", "root:channels", "public", "me", "upload", "media_library_beta", "live"},
	RedirectURL:  oauthutil.RedirectURL,
}

var errCantUpload = errors.New("can't upload files here")
var errCantMkdir = errors.New("can't make directories here")
var errCantRmdir = errors.New("can't remove this directory")

// gproAuthorize retrieves an OAuth token using username/password and saves
// it to rclone.conf
func gproAuthorize(ctx context.Context, opt *Options, name string, m configmap.Mapper) error {
	if opt.User == "" {
		return errors.New("no username")
	}
	pass, err := obscure.Reveal(opt.Pass)
	if err != nil {
		return fmt.Errorf("failed to decode password - did you obscure it?: %w", err)
	}
	oa2Ctx := oauthutil.Context(ctx, fshttp.NewClient(ctx))
	token, err := oauthConfig.MakeOauth2Config().PasswordCredentialsToken(oa2Ctx, opt.User, pass)
	if err != nil {
		return fmt.Errorf("failed to retrieve token using username/password: %w", err)
	}
	return oauthutil.PutToken(name, m, token, false)
}

// Register with Fs
func init() {
	fs.Register(&fs.RegInfo{
		Name:        "gopro",
		Description: "GoPro Media Library",
		NewFs:       NewFs,
		Config: func(ctx context.Context, name string, m configmap.Mapper, configIn fs.ConfigIn) (*fs.ConfigOut, error) {
			opt := new(Options)
			if err := configstruct.Set(m, opt); err != nil {
				return nil, fmt.Errorf("couldn't parse config into struct: %w", err)
			}
			if opt.AccessToken != "" {
				// Static bearer token configured - nothing to authorize.
				return nil, nil
			}
			switch configIn.State {
			case "":
				if _, err := oauthutil.GetToken(name, m); err != nil {
					return fs.ConfigGoto("authorize")
				}
				return fs.ConfigConfirm("authorize_ok", false, "consent_to_authorize", "Re-authorize for new token?")
			case "authorize_ok":
				if configIn.Result == "false" {
					return nil, nil
				}
				return fs.ConfigGoto("authorize")
			case "authorize":
				if err := gproAuthorize(ctx, opt, name, m); err != nil {
					return nil, err
				}
				return nil, nil
			}
			return nil, fmt.Errorf("unknown state %q", configIn.State)
		},
		Options: []fs.Option{{
			Name:      "user",
			Help:      "GoPro account email.\n\nLeave blank if using access_token instead.",
			Sensitive: true,
		}, {
			Name:       "pass",
			Help:       "GoPro account password.\n\nLeave blank if using access_token instead.",
			IsPassword: true,
		}, {
			Name:     "access_token",
			Advanced: true,
			Help: `Static bearer token, as an alternative to user/pass.

Copy the value of the gp_access_token cookie from a browser session
logged into gopro.com/media-library. This does not refresh, so it
will stop working (typically within a few hours) and need pasting in
again - prefer user/pass unless your account can't complete that
flow.`,
			Sensitive: true,
		}, {
			Name:     "download_variation",
			Advanced: true,
			Default:  "source",
			Help: `Which rendition to download.

"source" (the default) downloads the original camera file. Any other
value is matched against the label or quality of the renditions GoPro
offers for that media item (for example "1080p", or a proxy label such
as "high_res_proxy_mp4"); if no match is found the backend falls back
to the first file offered.`,
		}, {
			Name:     "include_edits",
			Advanced: true,
			Default:  false,
			Help: `Include Highlights and user-made Edits in listings.

Off by default: these "MultiClipEdit"/"Edit" media are composed from
other clips rather than being their own camera-original recording, so
they're often a redundant rendering of content the library already
has natively, and they behave differently enough to be worth opting
into deliberately rather than getting by surprise:

- file_size is always null for these - this backend reports their
  size as unknown (like a chaptered video or burst photo set) rather
  than skipping them, so [--gopro-read-size](#gopro-read-size) is
  needed for an exact size (e.g. for rclone mount).
- Their own file_extension is that of an internal Edit Decision List
  (typically "json"), not of what's actually downloaded - GoPro
  serves the rendered video (the "baked_source" rendition) for these,
  not the EDL, and this backend's Content-Type follows the filename's
  own extension (usually ".mp4") to match what's actually served.`,
		}, {
			Name:     "always_add_id",
			Advanced: true,
			Default:  true,
			Help: `Always name files "name {id}.ext" instead of just "name.ext".

Without this, a file only gets its GoPro media ID appended to its name
when it collides with another file of the same name in the same
listing - but GoPro cameras recycle filenames constantly, so whether a
given file collides depends on what else happens to exist at listing
time, which can change from one run to the next with no change to the
file itself. That makes a plain name unstable: a file uploaded earlier
under "GX010123.MP4" can silently become "GX010123 {id}.MP4" the
moment a second "GX010123.MP4" turns up elsewhere in the library, and
rclone has no way to know the two names refer to the same file -
"sync" would delete and re-transfer it under the new name, and "copy"
would leave a stale duplicate behind under the old one, forever.

On by default because that failure mode is silent and only shows up
intermittently, on whichever run happens to introduce a colliding
name - a normal-looking successful sync/copy today doesn't mean this
won't bite on some future run. Turn it off only if you want clean
names and are confident this library won't hit a same-name collision
(a small, static library of hand-picked clips, say), or don't mind the
occasional renamed-and-re-transferred file.`,
		}, {
			Name:     "verify_size",
			Advanced: true,
			Default:  verifySizeReprocessed,
			Help: `Verify a file's size with a live request before relying on it.

file_size from the search API can be wrong - confirmed live, a few KB
larger than the size actually served, specifically for a video whose
"source" rendition had been moved to colder S3 storage. A wrong size
here does more than look odd: it fails rclone's post-transfer
integrity check (discarding an otherwise-complete download and
restarting it), can corrupt rclone's multi-thread downloader (which
divides a file into ranged chunks using this size before the transfer
starts), and makes "sync" re-download an already-correct file on every
single run, since the sizes would never match.

GoPro's API exposes no storage class or anything else that reliably
predicts which files are affected - checked live, colder storage alone
isn't enough, most files there still report their size correctly. The
one thing a live account probe did find in common with the one
affected file, out of hundreds checked, is that GoPro had reprocessed
it after upload (a non-null reprocessed_at). That's a single data
point, not a proven rule, but it's free to check - the same listing
already fetches it - so it's the default: cheaper than checking every
file, safer than checking none.`,
			Examples: []fs.OptionExample{{
				Value: verifySizeReprocessed,
				Help:  "Verify only files GoPro has reprocessed since upload",
			}, {
				Value: verifySizeAlways,
				Help:  "Verify every file - safest, one extra request per file",
			}, {
				Value: verifySizeOff,
				Help:  "Never verify - fastest, trusts file_size from the API as-is",
			}},
		}, {
			Name:     "read_size",
			Advanced: true,
			Default:  false,
			Help: `Read the exact size of a chaptered video or burst photo set item.

file_size from the search API is only the total across every item of a
chaptered video or burst photo set, not any single one, so an
individual item's size is normally left unknown (-1) rather than
guessed at by dividing it evenly. Set this if you need an exact size
for one of these, for example for rclone mount. This does one extra
request per file, on top of what --gopro-verify-size already costs -
so listing a large library with lots of chaptered/burst items will be
slower still.`,
		}, {
			Name:     "upload_chunk_size",
			Advanced: true,
			Default:  defaultUploadChunkSize,
			Help: `Chunk size for uploads to the upload/ directory.

Must be at least 5Mi: GoPro's upload endpoint is S3-backed and rejects
anything smaller for every part but the last.`,
		}, {
			Name:     "upload_concurrency",
			Advanced: true,
			Default:  defaultUploadConcurrency,
			Help: `Concurrency for multipart uploads.

GoPro's chunk upload protocol accepts parts in any order, so chunks of
a single file are PUT concurrently once read. Note that chunks are
buffered in memory, so total memory use can be up to
upload_chunk_size * upload_concurrency.`,
		}, {
			Name:     config.ConfigEncoding,
			Help:     config.ConfigEncodingHelp,
			Advanced: true,
			Default: (encoder.Base |
				encoder.EncodeCrLf |
				encoder.EncodeInvalidUtf8),
		}},
	})
}

// Options defines the configuration for this backend
type Options struct {
	User              string               `config:"user"`
	Pass              string               `config:"pass"`
	AccessToken       string               `config:"access_token"`
	DownloadVariation string               `config:"download_variation"`
	IncludeEdits      bool                 `config:"include_edits"`
	AlwaysAddID       bool                 `config:"always_add_id"`
	VerifySize        string               `config:"verify_size"`
	ReadSize          bool                 `config:"read_size"`
	UploadChunkSize   fs.SizeSuffix        `config:"upload_chunk_size"`
	UploadConcurrency int                  `config:"upload_concurrency"`
	Enc               encoder.MultiEncoder `config:"encoding"`
}

// dlCacheEntry caches a download descriptor, which carries short-lived
// signed CDN URLs
type dlCacheEntry struct {
	resp    *api.DownloadResponse
	fetched time.Time
}

// Fs represents a GoPro Media Library
type Fs struct {
	name      string
	root      string
	opt       Options
	features  *fs.Features
	srv       *rest.Client
	unAuth    *rest.Client           // no Authorization header - required for pre-signed chunk upload URLs
	ts        *oauthutil.TokenSource // nil when using a static access_token
	pacer     *fs.Pacer
	startTime time.Time // time Fs was started - used for datestamps

	ridMu           sync.Mutex
	resourceOwnerID string // cached for the upload protocol

	dlCacheMu sync.Mutex
	dlCache   map[string]*dlCacheEntry

	uploadedMu sync.Mutex
	uploaded   dirtree.DirTree // record of items uploaded this run
}

// Object describes a GoPro Media Library item
type Object struct {
	fs          *Fs
	remote      string
	id          string
	itemNumber  int // 1-based; always 1 unless itemCount > 1
	itemCount   int // total items on the parent medium; 1 for an ordinary single-file medium
	bytes       int64
	sizeChecked bool // true once bytes has been confirmed (or corrected) against a live response
	reprocessed bool // true if the parent medium's reprocessed_at is set - see verify_size's "reprocessed" mode
	modTime     time.Time
	mimeType    string
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
	return fmt.Sprintf("GoPro Media Library path %q", f.root)
}

// Features returns the optional features of this Fs
func (f *Fs) Features() *fs.Features {
	return f.features
}

// dirTime returns the time to set a directory to
func (f *Fs) dirTime() time.Time {
	return f.startTime
}

// startYear returns the year to start "by-year" style listings from.
//
// GoPro Media Library has no library-wide creation date to anchor this on,
// so use a fixed year rather than adding a config option for something that
// only changes the size of a directory listing.
func (f *Fs) startYear() int {
	return 2010
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
// deserve to be retried. It returns the err as a convenience
func shouldRetry(ctx context.Context, resp *http.Response, err error) (bool, error) {
	if fserrors.ContextError(ctx, &err) {
		return false, err
	}
	return fserrors.ShouldRetry(err) || fserrors.ShouldRetryHTTP(resp, retryErrorCodes), err
}

// errorHandler parses a non 2xx error response into an error
//
// The OAuth token endpoint uses the standard {"error","error_description"}
// shape; other endpoints may return something else or nothing parseable,
// so the raw body and status are always preserved as a fallback.
func errorHandler(resp *http.Response) error {
	body, err := rest.ReadBody(resp)
	if err != nil {
		body = nil
	}
	e := &api.Error{Status: resp.StatusCode, Body: string(body)}
	if body != nil {
		_ = json.Unmarshal(body, e)
	}
	return e
}

// NewFs constructs an Fs from the path, root
func NewFs(ctx context.Context, name, root string, m configmap.Mapper) (fs.Fs, error) {
	opt := new(Options)
	if err := configstruct.Set(m, opt); err != nil {
		return nil, err
	}
	if opt.UploadChunkSize > 0 {
		if err := checkUploadChunkSize(opt.UploadChunkSize); err != nil {
			return nil, fmt.Errorf("gopro: %w", err)
		}
	}
	if err := checkVerifySizeMode(opt.VerifySize); err != nil {
		return nil, fmt.Errorf("gopro: %w", err)
	}

	root = strings.Trim(path.Clean(root), "/")
	if root == "." || root == "/" {
		root = ""
	}

	f := &Fs{
		name:      name,
		root:      root,
		opt:       *opt,
		pacer:     fs.NewPacer(ctx, pacer.NewDefault(pacer.MinSleep(minSleep), pacer.MaxSleep(maxSleep), pacer.DecayConstant(decayConstant))),
		startTime: time.Now(),
		dlCache:   map[string]*dlCacheEntry{},
		uploaded:  dirtree.New(),
	}

	baseClient := fshttp.NewClient(ctx)
	if opt.AccessToken != "" {
		f.srv = rest.NewClient(baseClient).SetRoot(rootURL)
		f.srv.SetHeader("Authorization", "Bearer "+opt.AccessToken)
	} else {
		oAuthClient, ts, err := oauthutil.NewClientWithBaseClient(ctx, name, m, oauthConfig, baseClient)
		if err != nil {
			return nil, fmt.Errorf("failed to configure gopro: %w", err)
		}
		// Token() is free when the cached token is still valid - it only
		// makes a request when a refresh is actually needed - so checking
		// it here costs nothing extra in the common case, but lets a stored
		// token GoPro has revoked or blacklisted (confirmed live: refresh
		// failing with "token_blacklisted") be recovered automatically by
		// re-authenticating with user/pass, the same recovery
		// "rclone config reconnect" performs manually, rather than failing
		// every command until someone runs that by hand.
		if _, tokErr := ts.Token(); tokErr != nil {
			if opt.User == "" || opt.Pass == "" {
				return nil, fmt.Errorf("failed to configure gopro: %w", tokErr)
			}
			fs.Logf(name, "stored token can't be used (%v) - re-authenticating with user/pass", tokErr)
			if authErr := gproAuthorize(ctx, opt, name, m); authErr != nil {
				return nil, fmt.Errorf("failed to configure gopro: stored token invalid (%v) and re-authentication failed: %w", tokErr, authErr)
			}
			oAuthClient, ts, err = oauthutil.NewClientWithBaseClient(ctx, name, m, oauthConfig, baseClient)
			if err != nil {
				return nil, fmt.Errorf("failed to configure gopro: %w", err)
			}
			if _, tokErr = ts.Token(); tokErr != nil {
				return nil, fmt.Errorf("failed to configure gopro: re-authenticated but token still unusable: %w", tokErr)
			}
		}
		f.ts = ts
		f.srv = rest.NewClient(oAuthClient).SetRoot(rootURL)
	}
	f.srv.SetErrorHandler(errorHandler)
	f.srv.SetHeader("Accept", mediaAcceptHeader)
	f.unAuth = rest.NewClient(baseClient)
	f.unAuth.SetErrorHandler(errorHandler)

	f.features = (&fs.Features{
		ReadMimeType: true,
		Move:         f.Move,
	}).Fill(ctx, f)

	// Check to see if the root is actually a file
	_, _, pattern := patterns.match(f.root, "", true)
	if pattern != nil && pattern.isFile {
		oldRoot := f.root
		var leaf string
		f.root, leaf = path.Split(f.root)
		f.root = strings.TrimRight(f.root, "/")
		_, err := f.NewObject(ctx, leaf)
		if err == nil {
			return f, fs.ErrorIsFile
		}
		f.root = oldRoot
	}
	return f, nil
}

// currentAccessToken returns the bearer token currently in use, whether it
// came from the OAuth token source or a static access_token
func (f *Fs) currentAccessToken(ctx context.Context) (string, error) {
	if f.ts == nil {
		return f.opt.AccessToken, nil
	}
	tok, err := f.ts.Token()
	if err != nil {
		return "", err
	}
	return tok.AccessToken, nil
}

// getResourceOwnerID returns the GoPro user id needed by the upload
// protocol.
//
// The initial OAuth token response carries it as an extra field, but that
// isn't guaranteed to survive a token refresh, so this falls back to
// GET /media/user, whose "id" field is the same value, if it's missing.
func (f *Fs) getResourceOwnerID(ctx context.Context) (string, error) {
	f.ridMu.Lock()
	defer f.ridMu.Unlock()
	if f.resourceOwnerID != "" {
		return f.resourceOwnerID, nil
	}
	if f.ts != nil {
		if tok, err := f.ts.Token(); err == nil {
			if rid, ok := tok.Extra("resource_owner_id").(string); ok && rid != "" {
				f.resourceOwnerID = rid
				return rid, nil
			}
		}
	}
	info, err := f.getUserInfo(ctx)
	if err != nil {
		return "", fmt.Errorf("couldn't determine gopro user id: %w", err)
	}
	f.resourceOwnerID = info.ID
	return f.resourceOwnerID, nil
}

// getUserInfo fetches GET /media/user, which carries account quota
// information as well as the account id
func (f *Fs) getUserInfo(ctx context.Context) (*api.UserInfo, error) {
	opts := rest.Opts{Method: "GET", Path: "/media/user"}
	var info api.UserInfo
	var resp *http.Response
	var err error
	err = f.pacer.Call(func() (bool, error) {
		resp, err = f.srv.CallJSON(ctx, &opts, nil, &info)
		return shouldRetry(ctx, resp, err)
	})
	if err != nil {
		return nil, err
	}
	return &info, nil
}

// About gets quota information
//
// GoPro-branded camera media is "exempt" and doesn't count against any
// limit ("Unlimited Storage" in the account dashboard); Total/Free only
// have a meaningful ceiling to report against the "non_exempt" pool
// ("Additional Storage: x/100GB" in the dashboard). Comparing
// NonExemptStorageLimit against the combined total would be wrong: an
// account whose non-exempt pool is nearly empty could still show as
// having no free space at all if most of its storage is exempt.
func (f *Fs) About(ctx context.Context) (*fs.Usage, error) {
	info, err := f.getUserInfo(ctx)
	if err != nil {
		return nil, fmt.Errorf("couldn't read user quota: %w", err)
	}
	usage := &fs.Usage{
		Used: fs.NewUsageValue(info.TotalStorage),
	}
	if info.NonExemptStorageLimit > 0 {
		usage.Total = fs.NewUsageValue(info.NonExemptStorageLimit)
		free := info.NonExemptStorageLimit - info.NonExempt.TotalStorage
		if free < 0 {
			free = 0
		}
		usage.Free = fs.NewUsageValue(free)
	}
	return usage, nil
}

// getMedium fetches a single medium by ID
func (f *Fs) getMedium(ctx context.Context, id string) (*api.Medium, error) {
	opts := rest.Opts{
		Method:     "GET",
		Path:       "/media/" + id,
		Parameters: url.Values{"fields": {mediaFields}},
	}
	var item api.Medium
	var resp *http.Response
	var err error
	err = f.pacer.Call(func() (bool, error) {
		resp, err = f.srv.CallJSON(ctx, &opts, nil, &item)
		return shouldRetry(ctx, resp, err)
	})
	if err != nil {
		return nil, fmt.Errorf("couldn't get medium %q: %w", id, err)
	}
	return &item, nil
}

// updateMedium updates a medium's filename/content_title/captured_at via
// PUT /media/{id} (204, no body) - only the fields set on upd are changed.
func (f *Fs) updateMedium(ctx context.Context, id string, upd api.MediumUpdate) error {
	opts := rest.Opts{
		Method:     "PUT",
		Path:       "/media/" + id,
		NoResponse: true,
	}
	var resp *http.Response
	var err error
	err = f.pacer.Call(func() (bool, error) {
		resp, err = f.srv.CallJSON(ctx, &opts, &upd, nil)
		return shouldRetry(ctx, resp, err)
	})
	if err != nil {
		return fmt.Errorf("couldn't update medium %q: %w", id, err)
	}
	return nil
}

// mediaTypes returns the type filter for /media/search - editTypes are
// only added when --gopro-include-edits is set (see includedTypes).
func (f *Fs) mediaTypes() string {
	if f.opt.IncludeEdits {
		return includedTypes + "," + editTypes
	}
	return includedTypes
}

// isEditType reports whether t is one of editTypes (a Highlight or Edit) -
// only reachable at all when --gopro-include-edits is set, since the
// server-side type filter excludes them otherwise.
func isEditType(t string) bool {
	return t == "MultiClipEdit" || t == "Edit"
}

// list pages through /media/search, calling fn for every included medium
// that matches filter.
//
// "MultiClipEdit" and "Edit" media are excluded server-side via the type
// parameter, unless --gopro-include-edits opts them back in (see
// mediaTypes). A non-empty filter is also applied server-side via
// captured_range, which cuts a by-day/by-month/by-year listing down from a
// full-library scan to just the matching window; the filter is re-checked
// client-side as well as a backstop in case the server-side bound is ever
// inexact.
func (f *Fs) list(ctx context.Context, filter mediaFilter, fn func(item *api.Medium) error) (err error) {
	const perPage = 100
	page := 1
	totalPages := 0
	lastID := ""
	params := url.Values{
		"fields":            {mediaFields},
		"type":              {f.mediaTypes()},
		"processing_states": {"ready"},
		"order_by":          {"captured_at"},
		"per_page":          {strconv.Itoa(perPage)},
	}
	if start, end, ok := filter.capturedRange(); ok {
		rangeParam := start.Format(capturedRangeLayout) + "," + end.Format(capturedRangeLayout)
		// The web app sends both of these set to the same value; replicate
		// that rather than risk depending on an alias that isn't actually
		// honoured on its own.
		params.Set("range", rangeParam)
		params.Set("captured_range", rangeParam)
	}
	for {
		params.Set("page", strconv.Itoa(page))
		opts := rest.Opts{
			Method:     "GET",
			Path:       "/media/search",
			Parameters: params,
		}
		var result api.SearchResponse
		var resp *http.Response
		err = f.pacer.Call(func() (bool, error) {
			resp, err = f.srv.CallJSON(ctx, &opts, nil, &result)
			return shouldRetry(ctx, resp, err)
		})
		if err != nil {
			return fmt.Errorf("couldn't list media: %w", err)
		}
		items := result.Embedded.Media
		if len(items) > 0 && items[0].ID == lastID {
			// skip first if ID duplicated from last page
			items = items[1:]
		}
		if len(items) > 0 {
			lastID = items[len(items)-1].ID
		}
		for i := range items {
			if err := fn(&items[i]); err != nil {
				return err
			}
		}
		if totalPages == 0 {
			totalPages = result.Pages.TotalPages
		}
		if len(result.Embedded.Media) == 0 || page >= totalPages {
			break
		}
		page++
	}
	return nil
}

// addID adds the ID to name
func addID(name string, ID string) string {
	idStr := "{" + ID + "}"
	if name == "" {
		return idStr
	}
	return name + " " + idStr
}

// addFileID adds the ID to the fileName passed in
func addFileID(fileName string, ID string) string {
	ext := path.Ext(fileName)
	base := fileName[:len(fileName)-len(ext)]
	return addID(base, ID) + ext
}

// itemLeaf names one item of a multi-item medium (a chaptered video or a
// burst photo set), e.g. "GX010294.MP4" item 2 -> "GX010294-2.MP4"
func itemLeaf(fileName string, itemNumber int) string {
	ext := path.Ext(fileName)
	base := fileName[:len(fileName)-len(ext)]
	return fmt.Sprintf("%s-%d%s", base, itemNumber, ext)
}

// idRe matches a GoPro medium id embedded in a deduped filename - 24
// lowercase hex characters in braces, the shape addID/addFileID produce.
//
// A camera-generated filename can never coincidentally match this, but a
// medium can now be renamed to an arbitrary filename (see the rename
// support below), so a match here is no longer proof the id is one this
// backend added - readMetaData's fast path verifies against the fetched
// medium's own name (expectedIDSuffixedName) before trusting it, rather
// than relying on this pattern alone.
var idRe = regexp.MustCompile(`\{([0-9a-f]{24})\}`)

// idSuffixRe matches the "name {id}" (or, for an empty name, just "{id}")
// suffix addID appends - anchored to the end, unlike idRe, so it can be
// stripped back off a leaf name (see stripSuffixID) without also matching
// an id-shaped substring elsewhere in an arbitrary, user-chosen filename.
var idSuffixRe = regexp.MustCompile(` ?\{[0-9a-f]{24}\}$`)

// findID finds an ID in a string if one is there, or ""
func findID(name string) string {
	match := idRe.FindStringSubmatch(name)
	if match == nil {
		return ""
	}
	return match[1]
}

// stripSuffixID removes a trailing " {id}" disambiguation suffix from a
// leaf name, if present - the inverse of addFileID. This backend's own
// listings always carry one under --gopro-always-add-id (or when a name
// collides even without it), but it's never part of the real filename, so
// a Move destination must have it stripped before being sent as the new
// filename - GoPro assigns ids itself and doesn't accept one from a
// rename request.
func stripSuffixID(name string) string {
	ext := path.Ext(name)
	base := strings.TrimSuffix(name, ext)
	return idSuffixRe.ReplaceAllString(base, "") + ext
}

// listDir lists a single directory, applying filter and deduping colliding
// filenames (GoPro cameras reuse filenames constantly, so collisions
// within one listing are routine, not exceptional).
func (f *Fs) listDir(ctx context.Context, prefix string, filter mediaFilter) (entries fs.DirEntries, err error) {
	err = f.list(ctx, filter, func(item *api.Medium) error {
		if !filter.matches(item.CapturedAt) {
			return nil
		}
		if item.FileSize == nil && !isEditType(item.Type) {
			// A ready medium can still have a null file_size beyond the
			// MultiClipEdit/Edit types, which always have one (handled
			// below via the same unknown-size path as a multi-item
			// medium, not skipped) - skip it defensively rather than list
			// an entry with no usable size or content.
			fs.Debugf(f, "Skipping %s: ready but file_size is null", item.ID)
			return nil
		}
		itemCount := item.ItemCount
		if itemCount < 1 {
			itemCount = 1
		}
		leaf := f.opt.Enc.ToStandardName(item.Filename)
		for n := 1; n <= itemCount; n++ {
			remote := leaf
			if itemCount > 1 {
				remote = itemLeaf(leaf, n)
			}
			o := &Object{fs: f, remote: prefix + remote}
			o.setMetaData(item, n)
			entries = append(entries, o)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	dupes := map[string]int{}
	for _, entry := range entries {
		if o, ok := entry.(*Object); ok {
			dupes[o.remote]++
		}
	}
	for _, entry := range entries {
		if o, ok := entry.(*Object); ok {
			if shouldAddID(f.opt.AlwaysAddID, o.remote, dupes[o.remote]) {
				o.remote = addFileID(o.remote, o.id)
			}
		}
	}
	return entries, nil
}

// shouldAddID decides whether a listed entry's remote should have its
// medium ID appended, given --gopro-always-add-id and how many entries in
// this same listing share that remote (count). An empty remote always
// gets one regardless of the option or count, since there's nothing else
// to show.
func shouldAddID(alwaysAddID bool, remote string, count int) bool {
	return alwaysAddID || count > 1 || remote == ""
}

// expectedIDSuffixedName reconstructs the id-suffixed leaf listDir would
// give item's first item when --gopro-always-add-id is set, for verifying
// a name found via the readMetaData fast path actually belongs to it.
func expectedIDSuffixedName(f *Fs, item *api.Medium) string {
	leaf := f.opt.Enc.ToStandardName(item.Filename)
	if item.ItemCount > 1 {
		leaf = itemLeaf(leaf, 1)
	}
	return addFileID(leaf, item.ID)
}

// listUploads lists a single directory from the items uploaded this run
func (f *Fs) listUploads(ctx context.Context, dir string) (entries fs.DirEntries, err error) {
	f.uploadedMu.Lock()
	entries, ok := f.uploaded[dir]
	f.uploadedMu.Unlock()
	if !ok && dir != "" {
		return nil, fs.ErrorDirNotFound
	}
	return entries, nil
}

// Return an Object from a path
//
// If it can't be found it returns the error fs.ErrorObjectNotFound.
func (f *Fs) newObjectWithInfo(ctx context.Context, remote string, info *api.Medium) (fs.Object, error) {
	o := &Object{
		fs:     f,
		remote: remote,
	}
	if info != nil {
		o.setMetaData(info, 1)
	} else {
		if err := o.readMetaData(ctx); err != nil {
			return nil, err
		}
	}
	return o, nil
}

// NewObject finds the Object at remote. If it can't be found
// it returns the error fs.ErrorObjectNotFound.
func (f *Fs) NewObject(ctx context.Context, remote string) (fs.Object, error) {
	return f.newObjectWithInfo(ctx, remote, nil)
}

// List the objects and directories in dir into entries. The
// entries can be returned in any order but should be for a
// complete directory.
//
// dir should be "" to list the root, and should not have
// trailing slashes.
//
// This should return ErrDirNotFound if the directory isn't
// found.
func (f *Fs) List(ctx context.Context, dir string) (entries fs.DirEntries, err error) {
	match, prefix, pattern := patterns.match(f.root, dir, false)
	if pattern == nil || pattern.isFile {
		return nil, fs.ErrorDirNotFound
	}
	if pattern.toEntries != nil {
		return pattern.toEntries(ctx, f, prefix, match)
	}
	return nil, fs.ErrorDirNotFound
}

// Put the object into the media library
//
// Copy the reader in to the new object which is returned.
//
// The new object may have been created if an error is returned
func (f *Fs) Put(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error) {
	o := &Object{fs: f, remote: src.Remote()}
	return o, o.Update(ctx, in, src, options...)
}

// Mkdir creates the upload directory if it doesn't exist; every other
// directory in the tree is synthetic and always considered to exist
// already.
func (f *Fs) Mkdir(ctx context.Context, dir string) error {
	_, prefix, pattern := patterns.match(f.root, dir, false)
	if pattern == nil {
		return fs.ErrorDirNotFound
	}
	if pattern.isUpload {
		f.uploadedMu.Lock()
		d := fs.NewDir(strings.Trim(prefix, "/"), f.dirTime())
		f.uploaded.AddEntry(d)
		f.uploadedMu.Unlock()
		return nil
	}
	if !pattern.canMkdir {
		return errCantMkdir
	}
	return nil
}

// Rmdir removes an empty upload directory; every other directory in the
// tree is synthetic and can't be removed.
func (f *Fs) Rmdir(ctx context.Context, dir string) error {
	_, _, pattern := patterns.match(f.root, dir, false)
	if pattern == nil {
		return fs.ErrorDirNotFound
	}
	if pattern.isUpload {
		f.uploadedMu.Lock()
		err := f.uploaded.Prune(map[string]bool{dir: true})
		f.uploadedMu.Unlock()
		return err
	}
	if !pattern.canMkdir {
		return errCantRmdir
	}
	return nil
}

// Precision returns the precision
func (f *Fs) Precision() time.Duration {
	return fs.ModTimeNotSupported
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

// Hash is not supported
func (o *Object) Hash(ctx context.Context, t hash.Type) (string, error) {
	return "", hash.ErrUnsupported
}

// selectRendition picks the download URL and HEAD-able URL for the given
// item of a download descriptor.
//
// "source" (the default) prefers the true camera original, but which array
// holds it depends on the medium's shape:
//
//   - An ordinary single-item photo or video: files[0] and the
//     variations[] entry labelled "source" are the same file (photo) or
//     files[0] is a proxy and the "source" variation is the original
//     (video) - either way there is exactly one "source" variation, and
//     it's the right answer regardless of itemNumber.
//   - A chaptered video (item_count > 1): files[] still holds only one
//     (proxy) entry, but variations[] holds one "source" entry per
//     chapter, keyed by item_number.
//   - A burst photo set (item_count > 1): the reverse - files[] holds one
//     entry per photo, keyed by item_number, while variations[] holds a
//     single "source" entry with no item_number, which is a cover image
//     representing the set, not any individual photo.
//
// This is told apart at runtime by counting "source"-labelled variations
// rather than switching on the medium's "type": only Video and Burst are
// verified against real examples of each, and other types (TimeLapse,
// Continuous, ...) may follow either shape.
func selectRendition(dl *api.DownloadResponse, variation string, itemNumber int) (dlURL, head string, err error) {
	if variation == "" {
		variation = "source"
	}
	if variation != "source" {
		for _, v := range dl.Embedded.Variations {
			if v.Label == variation || v.Quality == variation {
				return v.URL, v.Head, nil
			}
		}
		return "", "", fmt.Errorf("no %q rendition found", variation)
	}

	sourceVariations := 0
	for _, v := range dl.Embedded.Variations {
		if v.Label == "source" {
			sourceVariations++
		}
	}
	if sourceVariations > 1 {
		// Chaptered-video shape: one "source" variation per item_number.
		for _, v := range dl.Embedded.Variations {
			if v.Label == "source" && v.ItemNumber == itemNumber {
				return v.URL, v.Head, nil
			}
		}
	} else if len(dl.Embedded.Files) > 1 {
		// Burst shape: files[] holds one entry per item_number; the lone
		// "source" variation (if any) is a cover image, not this item.
		for _, file := range dl.Embedded.Files {
			if file.ItemNumber == itemNumber {
				return file.URL, file.Head, nil
			}
		}
	} else {
		// Ordinary single-item medium.
		for _, v := range dl.Embedded.Variations {
			if v.Label == "source" {
				return v.URL, v.Head, nil
			}
		}
		if len(dl.Embedded.Files) > 0 {
			return dl.Embedded.Files[0].URL, dl.Embedded.Files[0].Head, nil
		}
	}
	return "", "", fmt.Errorf("no source rendition found for item %d", itemNumber)
}

// getDownload fetches (and caches) the download descriptor for a medium
func (f *Fs) getDownload(ctx context.Context, id string) (*api.DownloadResponse, error) {
	f.dlCacheMu.Lock()
	if e, ok := f.dlCache[id]; ok && time.Since(e.fetched) < dlCacheTTL {
		f.dlCacheMu.Unlock()
		return e.resp, nil
	}
	f.dlCacheMu.Unlock()

	opts := rest.Opts{Method: "GET", Path: "/media/" + id + "/download"}
	var result api.DownloadResponse
	var resp *http.Response
	var err error
	err = f.pacer.Call(func() (bool, error) {
		resp, err = f.srv.CallJSON(ctx, &opts, nil, &result)
		return shouldRetry(ctx, resp, err)
	})
	if err != nil {
		return nil, fmt.Errorf("couldn't get download info: %w", err)
	}

	f.dlCacheMu.Lock()
	f.dlCache[id] = &dlCacheEntry{resp: &result, fetched: time.Now()}
	f.dlCacheMu.Unlock()
	return &result, nil
}

// Size returns the size of an object in bytes
//
// file_size from /media/search is only accurate for the "source"
// download_variation on a single-item medium, and even then it can be
// stale - confirmed live on a file whose file_size was 3245 bytes larger
// than the Content-Length its "source" rendition actually serves. A stale
// Size() isn't just cosmetic: fs/operations's multi-thread copy divides a
// download into ranged chunks using this value before the first byte is
// requested, so a too-large Size() truncates the last chunk's range and
// fails the whole transfer ("failed to write chunk: expected ... but
// wrote ..."), and a sync run that only ever sees the stale value would
// re-transfer a file that's already correct on every single run, forever,
// because the size never matches.
//
// --gopro-verify-size controls which files with a known size get this
// check via shouldVerifySize: "always" checks every one, "off" checks
// none, and the default "reprocessed" checks only ones whose medium has
// been reprocessed since upload - the one thing a live account probe
// found in common with the one affected file out of hundreds checked
// (GoPro's API otherwise exposes nothing that correlates, not even
// storage class - colder storage alone isn't sufficient, most files there
// still report correctly). The check costs one HEAD per Object, cached
// for this Object's lifetime so repeated Size() calls in the same run
// (list, sync compare, transfer) only pay for it once.
//
// For a multi-item medium (a chaptered video or a burst photo set)
// file_size is the total across every item, not this one, so o.bytes is
// -1 (unknown) by design - see setMetaData - and this only attempts to
// resolve that when --gopro-read-size is set, since dividing it exactly
// always needs a HEAD regardless of whether file_size itself is stale.
func (o *Object) Size() int64 {
	if o.bytes < 0 {
		if !o.fs.opt.ReadSize {
			return o.bytes
		}
	} else if !shouldVerifySize(o.fs.opt.VerifySize, o.reprocessed) {
		return o.bytes
	}
	if o.sizeChecked {
		return o.bytes
	}
	ctx := context.TODO()
	dl, err := o.fs.getDownload(ctx, o.id)
	if err != nil {
		fs.Debugf(o, "Size: %v", err)
		return o.bytes
	}
	_, head, err := selectRendition(dl, o.fs.opt.DownloadVariation, o.itemNumber)
	if err != nil || head == "" {
		fs.Debugf(o, "Size: %v", err)
		return o.bytes
	}
	var resp *http.Response
	opts := rest.Opts{Method: "HEAD", RootURL: head}
	err = o.fs.pacer.Call(func() (bool, error) {
		resp, err = o.fs.unAuth.Call(ctx, &opts)
		return shouldRetry(ctx, resp, err)
	})
	if err != nil {
		fs.Debugf(o, "Size: HEAD failed: %v", err)
		return o.bytes
	}
	length, err := strconv.ParseInt(resp.Header.Get("Content-Length"), 10, 64)
	if err != nil {
		fs.Debugf(o, "Size: couldn't parse Content-Length: %v", err)
		return o.bytes
	}
	o.reportSizeMismatch(length)
	o.sizeChecked = true
	return o.bytes
}

// reportSizeMismatch corrects o.bytes to actual if it differs, warning
// every time this happens since a mismatch between GoPro's reported
// file_size and the size actually served is a data integrity signal
// worth surfacing, not just a debug-level detail. The file is still
// downloaded - actual, taken from a live response, is trustworthy.
func (o *Object) reportSizeMismatch(actual int64) {
	if o.bytes >= 0 && actual != o.bytes {
		fs.Logf(o, "file_size from the GoPro API (%d) doesn't match the size actually being served (%d) - using the actual size; downloading anyway", o.bytes, actual)
	}
	o.bytes = actual
}

// setMetaData sets the Object data from a Medium
func (o *Object) setMetaData(item *api.Medium, itemNumber int) {
	o.id = item.ID
	o.itemNumber = itemNumber
	o.itemCount = item.ItemCount
	if o.itemCount < 1 {
		o.itemCount = 1
	}
	o.bytes = -1
	// file_size is the total across every item of a multi-item medium (a
	// chaptered video or burst photo set), not this item's size, and
	// individual item sizes vary too much to estimate safely: an estimate
	// that's even slightly wrong makes rclone's own transfer integrity
	// check ("corrupted on transfer: sizes differ") fail on every
	// download. -1 (unknown) is the correct, deliberate choice here -
	// fs/operations.sizeDiffers skips its check whenever either side's
	// Size() is negative, which is exactly what's wanted until an exact
	// HEAD is done. Only a single-item medium's file_size is trustworthy
	// enough to use directly.
	if item.FileSize != nil && o.itemCount <= 1 {
		o.bytes = *item.FileSize
	}
	o.modTime = item.CapturedAt
	if o.modTime.IsZero() {
		o.modTime = item.CreatedAt
	}
	// file_extension is the native format of the medium's own record, not
	// necessarily of what's actually downloaded: a MultiClipEdit's
	// file_extension is "json" (its Edit Decision List), but its filename
	// still ends in ".mp4" and selectRendition serves the rendered video,
	// not the EDL - so the filename's own extension is what actually
	// matches the bytes served here. Only fall back to file_extension
	// when the filename has none to go on.
	ext := path.Ext(item.Filename)
	if ext == "" {
		ext = "." + item.FileExtension
	}
	o.mimeType = mime.TypeByExtension(strings.ToLower(ext))
	o.reprocessed = item.ReprocessedAt != nil
}

// readMetaData gets the metadata if it hasn't already been fetched
//
// it also sets the info
func (o *Object) readMetaData(ctx context.Context) (err error) {
	if !o.modTime.IsZero() {
		return nil
	}
	dir, fileName := path.Split(o.remote)
	dir = strings.Trim(dir, "/")
	_, _, pattern := patterns.match(o.fs.root, o.remote, true)
	if pattern == nil {
		return fs.ErrorObjectNotFound
	}
	if !pattern.isFile {
		return fs.ErrorNotAFile
	}
	// If have ID fetch it directly. The {id} suffix only ever encodes a
	// filename-collision disambiguator (D4), not which item of a
	// multi-item medium was meant, so this always resolves to item 1 - a
	// compound edge case (an id-suffixed name that's also a non-first
	// chapter/burst item) would resolve to the wrong item here.
	//
	// GoPro media can now be renamed to an arbitrary filename (see the
	// rename support below), so idRe matching a {24-hex} substring no
	// longer proves it's a suffix this backend added - a renamed file
	// could coincidentally (or deliberately) contain one. Fetching by
	// that id and trusting it unconditionally would silently serve a
	// different medium's content under this name. Guard against that by
	// only trusting the fast path when it's reconstructible: this backend
	// only ever produces exactly this shape when --gopro-always-add-id is
	// set (the default), so verify the fetched medium's own real name,
	// re-suffixed the same way, matches fileName exactly before using it;
	// otherwise fall through to the listing-based lookup below, which
	// fails closed with fs.ErrorObjectNotFound if fileName doesn't
	// genuinely belong to anything.
	if id := findID(fileName); id != "" && o.fs.opt.AlwaysAddID {
		item, err := o.fs.getMedium(ctx, id)
		if err != nil {
			return err
		}
		if expectedIDSuffixedName(o.fs, item) == fileName {
			o.setMetaData(item, 1)
			return nil
		}
	}
	// Otherwise list the directory the file is in
	entries, err := o.fs.List(ctx, dir)
	if err != nil {
		if err == fs.ErrorDirNotFound {
			return fs.ErrorObjectNotFound
		}
		return err
	}
	for _, entry := range entries {
		if entry.Remote() == o.remote {
			if newO, ok := entry.(*Object); ok {
				*o = *newO
				return nil
			}
		}
	}
	return fs.ErrorObjectNotFound
}

// ModTime returns the modification time of the object
func (o *Object) ModTime(ctx context.Context) time.Time {
	if err := o.readMetaData(ctx); err != nil {
		fs.Debugf(o, "ModTime: Failed to read metadata: %v", err)
		return time.Now()
	}
	return o.modTime
}

// SetModTime changes captured_at via PUT /media/{id} - confirmed live this
// is not actually fixed at upload time, unlike most backends' notion of a
// server-set, immutable capture/creation date.
//
// This changes GoPro's own record of when the medium was captured, not
// just a local modification-time label - a deliberate choice to reuse the
// one time field GoPro exposes rather than invent a second one it has
// nowhere to store; treat it accordingly; rclone commands that call this
// (touch, and copy with --update in some modes) will rewrite that history.
// captured_at is a medium-level field shared by every item of a
// multi-item medium (chaptered video, burst photo set), so this changes
// all of them at once, matching how they already share one modTime.
func (o *Object) SetModTime(ctx context.Context, modTime time.Time) error {
	if err := o.fs.updateMedium(ctx, o.id, api.MediumUpdate{CapturedAt: &modTime}); err != nil {
		return err
	}
	o.modTime = modTime
	return nil
}

// Storable returns a boolean as to whether this object is storable
func (o *Object) Storable() bool {
	return true
}

// Open an object for read
func (o *Object) Open(ctx context.Context, options ...fs.OpenOption) (io.ReadCloser, error) {
	if err := o.readMetaData(ctx); err != nil {
		fs.Debugf(o, "Open: Failed to read metadata: %v", err)
		return nil, err
	}
	dl, err := o.fs.getDownload(ctx, o.id)
	if err != nil {
		return nil, err
	}
	dlURL, _, err := selectRendition(dl, o.fs.opt.DownloadVariation, o.itemNumber)
	if err != nil {
		return nil, err
	}
	var resp *http.Response
	opts := rest.Opts{
		Method:  "GET",
		RootURL: dlURL,
		Options: options,
	}
	err = o.fs.pacer.Call(func() (bool, error) {
		resp, err = o.fs.unAuth.Call(ctx, &opts)
		return shouldRetry(ctx, resp, err)
	})
	if err != nil {
		return nil, err
	}
	o.fixSize(resp)
	return resp.Body, nil
}

// fixSize reconciles o.bytes against the actual size of a download
// response, for the case where Open is called without Size having
// already resolved (and cached) it - see Size and reportSizeMismatch.
// The GET response this method already makes is itself an authoritative
// source, so there's no need for a separate HEAD request to fix it here.
func (o *Object) fixSize(resp *http.Response) {
	if o.sizeChecked {
		return
	}
	total := int64(-1)
	if resp.StatusCode == http.StatusPartialContent {
		// A ranged request/resume only reports the length of that range in
		// Content-Length, not the whole file - never fall back to it here,
		// or a resumed download would wrongly shrink o.bytes to just the
		// range size. The real total is in "Content-Range: bytes a-b/total".
		if _, after, ok := strings.Cut(resp.Header.Get("Content-Range"), "/"); ok && after != "*" {
			if n, err := strconv.ParseInt(after, 10, 64); err == nil {
				total = n
			}
		}
	} else {
		total = resp.ContentLength
	}
	if total >= 0 {
		o.reportSizeMismatch(total)
		o.sizeChecked = true
	}
}

// Update the object with the contents of the io.Reader, modTime and size
//
// The new object may have been created if an error is returned
func (o *Object) Update(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) error {
	chunkWriter, err := multipart.UploadMultipart(ctx, src, in, multipart.UploadMultipartOptions{
		Open:        o.fs,
		OpenOptions: options,
	})
	if err != nil {
		return err
	}
	o.setMetaData(chunkWriter.(*gpChunkWriter).medium, 1)
	o.fs.uploadedMu.Lock()
	o.fs.uploaded.AddEntry(o)
	o.fs.uploadedMu.Unlock()
	return nil
}

// Remove an object
func (o *Object) Remove(ctx context.Context) error {
	return o.fs.deleteMedium(ctx, o.id)
}

// deleteMedium deletes the medium with the given id - shared by Remove and
// gpChunkWriter.Abort, which deletes an incomplete upload's medium so a
// failed or cancelled upload doesn't leave an unusable entry behind in the
// library.
func (f *Fs) deleteMedium(ctx context.Context, id string) error {
	opts := rest.Opts{
		Method:     "DELETE",
		Path:       "/media",
		Parameters: url.Values{"ids": {id}},
	}
	var result api.DeleteResponse
	var resp *http.Response
	var err error
	err = f.pacer.Call(func() (bool, error) {
		resp, err = f.srv.CallJSON(ctx, &opts, nil, &result)
		return shouldRetry(ctx, resp, err)
	})
	if err != nil {
		return fmt.Errorf("couldn't delete %q: %w", id, err)
	}
	if len(result.Embedded.Errors) > 0 {
		e := result.Embedded.Errors[0]
		return fmt.Errorf("couldn't delete %q: %s", id, e.Description)
	}
	return nil
}

// Move renames src to remote in place via PUT /media/{id} (updateMedium) -
// confirmed live this can change both a medium's filename and its
// captured_at, neither fixed at upload time as most backends' equivalents
// would be.
//
// A destination under media/by-year, media/by-month or media/by-day whose
// date differs from src's current one is honoured by changing captured_at
// to match, the one way this "move" can actually reposition an item
// between those views - they're derived from captured_at, and there's no
// real folder for anything to move between otherwise. media/all and a
// same-bucket destination only rename it. upload/ has no medium to rename
// until an upload completes, so isn't supported as either endpoint.
func (f *Fs) Move(ctx context.Context, src fs.Object, remote string) (fs.Object, error) {
	srcObj, ok := src.(*Object)
	if !ok {
		return nil, fs.ErrorCantMove
	}
	// Not srcObj.fs != f: rclone calls this on the *destination* Fs, and
	// only after its own SameConfig check already confirmed src belongs
	// to the same gopro: remote - but "same remote" doesn't mean "same
	// *Fs instance". A moveto whose source and destination land under
	// different roots (e.g. different media/by-day buckets) gets two
	// distinct *Fs, one per resolved root, even for the same config - a
	// pointer-identity check here would wrongly refuse exactly the
	// cross-directory moves this method exists for (confirmed live: it
	// did, silently falling back to rclone's generic copy+delete, which
	// then failed outright since only upload/ accepts new files). Use
	// srcObj.fs, not f, for the mutation itself - it's the authoritative
	// Fs for the object actually being changed.
	if srcObj.itemCount > 1 {
		// The API renames the whole medium, not one chapter/frame of it -
		// renaming just one wouldn't be meaningful.
		return nil, fs.ErrorCantMove
	}
	match, _, pattern := patterns.match(f.root, remote, true)
	if pattern == nil || !pattern.isFile || pattern.isUpload {
		return nil, fs.ErrorCantMove
	}
	leaf := stripSuffixID(match[len(match)-1])
	filename := f.opt.Enc.FromStandardName(leaf)

	upd := api.MediumUpdate{Filename: &filename, ContentTitle: &filename}
	if capturedAt, ok := destCapturedAt(pattern, match, srcObj.modTime); ok {
		upd.CapturedAt = &capturedAt
	}
	if err := srcObj.fs.updateMedium(ctx, srcObj.id, upd); err != nil {
		return nil, err
	}

	dstObj := &Object{}
	*dstObj = *srcObj
	dstObj.fs = f
	dstObj.remote = remote
	if upd.CapturedAt != nil {
		dstObj.modTime = *upd.CapturedAt
	}
	return dstObj, nil
}

// destCapturedAt derives the captured_at Move should set for a destination
// matched against a media/by-year, media/by-month or media/by-day file
// pattern, preserving whatever of modTime's own year/month/day/time isn't
// pinned by the destination. ok is false for media/all (no date implied)
// or when the implied date already matches modTime (nothing to change).
func destCapturedAt(pattern *dirPattern, match []string, modTime time.Time) (t time.Time, ok bool) {
	var year, month, day int
	switch pattern.re {
	case `^media/by-year/(\d{4})/([^/]+)$`:
		year, _ = strconv.Atoi(match[1])
		month, day = int(modTime.Month()), modTime.Day()
	case `^media/by-month/\d{4}/(\d{4})-(\d{2})/([^/]+)$`:
		year, _ = strconv.Atoi(match[1])
		m, _ := strconv.Atoi(match[2])
		month, day = m, modTime.Day()
	case `^media/by-day/\d{4}/(\d{4})-(\d{2})-(\d{2})/([^/]+)$`:
		year, _ = strconv.Atoi(match[1])
		m, _ := strconv.Atoi(match[2])
		d, _ := strconv.Atoi(match[3])
		month, day = m, d
	default:
		return time.Time{}, false
	}
	t = time.Date(year, time.Month(month), day,
		modTime.Hour(), modTime.Minute(), modTime.Second(), modTime.Nanosecond(),
		modTime.Location())
	return t, !t.Equal(modTime)
}

// MimeType of an Object if known, "" otherwise
func (o *Object) MimeType(ctx context.Context) string {
	return o.mimeType
}

// ID of an Object if known, "" otherwise
func (o *Object) ID() string {
	return o.id
}

// ------------------------------------------------------------
// Upload protocol
//
// GoPro Media Library has no simple single-shot upload endpoint. Uploading
// a file takes five round trips: create a medium, create a "Source"
// derivative for it, request upload authorizations for each chunk, PUT the
// chunks to their (pre-signed) authorization URLs, then mark the
// derivative and the medium available. Ported from the most complete
// reference,
// github.com/dustin/gopro-plus (GoPro.Plus.Upload).
// ------------------------------------------------------------

// mediumTypeForFilename guesses the GoPro "type" value for a filename by
// extension, mirroring the reference client (JPG/GPR -> Photo, else Video)
func mediumTypeForFilename(name string) string {
	switch strings.ToUpper(strings.TrimPrefix(path.Ext(name), ".")) {
	case "JPG", "JPEG", "GPR", "DNG", "PNG", "HEIC":
		return "Photo"
	default:
		return "Video"
	}
}

// gpChunkWriter implements fs.ChunkWriter, driven by lib/multipart's
// generic uploader (see OpenChunkWriter). It PUTs pre-authorized chunks to
// GoPro's S3-backed upload endpoint using pooled buffers from the global
// memory pool that multipart.UploadMultipart hands to WriteChunk, rather
// than allocating its own - see "Managing memory" in CONTRIBUTING.md.
type gpChunkWriter struct {
	f            *Fs
	mediumID     string
	derivativeID string
	uploadID     string
	filename     string
	ext          string
	mediumType   string
	chunkSize    int64
	size         int64
	parts        []api.UploadAuthorization // sorted by Part; parts[i] is part i+1
	medium       *api.Medium               // set by Close
}

// OpenChunkWriter returns the chunk size and a ChunkWriter for uploading
// remote with the contents of src.
//
// GoPro has no single-shot upload endpoint - every upload is chunked, and
// the protocol needs the medium created and every chunk's upload
// authorization fetched up front, before the first byte is sent, so all of
// that (the first four of five steps of the upload protocol) happens here
// rather than in WriteChunk. Ported from the most complete reference,
// github.com/dustin/gopro-plus (GoPro.Plus.Upload).
func (f *Fs) OpenChunkWriter(ctx context.Context, remote string, src fs.ObjectInfo, options ...fs.OpenOption) (info fs.ChunkWriterInfo, writer fs.ChunkWriter, err error) {
	match, _, pattern := patterns.match(f.root, remote, true)
	if pattern == nil || !pattern.isFile || !pattern.canUpload {
		return info, nil, errCantUpload
	}
	size := src.Size()
	if size < 0 {
		return info, nil, errors.New("gopro: can't upload a file of unknown size - the upload protocol needs it up front")
	}
	filename := match[1]
	ext := strings.ToUpper(strings.TrimPrefix(path.Ext(filename), "."))
	mediumType := mediumTypeForFilename(filename)

	chunkSize := int64(f.opt.UploadChunkSize)
	if chunkSize <= 0 {
		chunkSize = int64(defaultUploadChunkSize)
	}
	nParts := int((size + chunkSize - 1) / chunkSize)
	if nParts == 0 {
		nParts = 1
	}

	mediumID, err := f.createMedium(ctx, filename, ext, mediumType)
	if err != nil {
		return info, nil, err
	}
	derivativeID, err := f.createDerivative(ctx, mediumID, ext, nParts)
	if err != nil {
		return info, nil, err
	}
	uploadID, err := f.createUpload(ctx, derivativeID)
	if err != nil {
		return info, nil, err
	}
	parts, err := f.getUploadParts(ctx, derivativeID, uploadID, size, chunkSize, nParts)
	if err != nil {
		return info, nil, err
	}
	sort.Slice(parts, func(i, j int) bool { return parts[i].Part < parts[j].Part })

	info = fs.ChunkWriterInfo{
		ChunkSize:   chunkSize,
		Concurrency: f.opt.UploadConcurrency,
	}
	return info, &gpChunkWriter{
		f:            f,
		mediumID:     mediumID,
		derivativeID: derivativeID,
		uploadID:     uploadID,
		filename:     filename,
		ext:          ext,
		mediumType:   mediumType,
		chunkSize:    chunkSize,
		size:         size,
		parts:        parts,
	}, nil
}

// WriteChunk PUTs chunk number chunkNumber (0-based) from reader to its
// pre-authorized URL.
//
// This must use the unauthenticated client: sending our own Authorization
// header alongside the URL's own pre-signed query auth gets a 400 from S3
// ("Only one auth mechanism allowed"). reader is a pooled, seekable buffer
// - it's rewound to the start before every attempt, including the first,
// so a retry re-sends the same chunk rather than a truncated one.
func (w *gpChunkWriter) WriteChunk(ctx context.Context, chunkNumber int, reader io.ReadSeeker) (int64, error) {
	if chunkNumber < 0 || chunkNumber >= len(w.parts) {
		return 0, fmt.Errorf("gopro: chunk number %d out of range (have %d parts)", chunkNumber, len(w.parts))
	}
	part := w.parts[chunkNumber]

	var size int64
	err := w.f.pacer.Call(func() (bool, error) {
		var err error
		size, err = reader.Seek(0, io.SeekEnd)
		if err != nil {
			return false, err
		}
		if _, err := reader.Seek(0, io.SeekStart); err != nil {
			return false, err
		}
		opts := rest.Opts{
			Method:        "PUT",
			RootURL:       part.URL,
			Body:          reader,
			ContentLength: &size,
			NoResponse:    true,
		}
		resp, err := w.f.unAuth.Call(ctx, &opts)
		return shouldRetry(ctx, resp, err)
	})
	if err != nil {
		return 0, fmt.Errorf("couldn't upload part %d: %w", part.Part, err)
	}
	return size, nil
}

// Close finalises the upload: marks all chunks complete, then the
// derivative and medium available.
//
// The resulting medium isn't re-fetched from /media/search once uploaded:
// it won't appear there until GoPro finishes processing it
// (processing_states=ready is what this backend lists), so re-fetching
// immediately would race the pipeline for no benefit. A synthetic Medium
// is built instead.
func (w *gpChunkWriter) Close(ctx context.Context) error {
	if err := w.f.completeUpload(ctx, w.derivativeID, w.uploadID, w.size, w.chunkSize); err != nil {
		return err
	}
	if err := w.f.markDerivativeAvailable(ctx, w.derivativeID); err != nil {
		return err
	}
	if err := w.f.markMediumAvailable(ctx, w.mediumID); err != nil {
		return err
	}
	now := time.Now()
	w.medium = &api.Medium{
		ID:            w.mediumID,
		Filename:      w.filename,
		FileExtension: strings.ToLower(w.ext),
		Type:          w.mediumType,
		CapturedAt:    now,
		CreatedAt:     now,
		FileSize:      &w.size,
	}
	return nil
}

// Abort deletes the medium created for this upload, so a failed or
// cancelled upload doesn't leave an incomplete, unusable entry behind in
// the library.
func (w *gpChunkWriter) Abort(ctx context.Context) error {
	return w.f.deleteMedium(ctx, w.mediumID)
}

// createMedium is step 1: POST /media
func (f *Fs) createMedium(ctx context.Context, filename, ext, mediumType string) (string, error) {
	tok, err := f.currentAccessToken(ctx)
	if err != nil {
		return "", err
	}
	rid, err := f.getResourceOwnerID(ctx)
	if err != nil {
		return "", err
	}
	body := map[string]any{
		"file_extension":    ext,
		"filename":          filename,
		"type":              mediumType,
		"on_public_profile": false,
		"content_title":     filename,
		"content_source":    "gda",
		"access_token":      tok,
		"gopro_user_id":     rid,
	}
	opts := rest.Opts{Method: "POST", Path: "/media"}
	var result struct {
		ID string `json:"id"`
	}
	var resp *http.Response
	err = f.pacer.Call(func() (bool, error) {
		resp, err = f.srv.CallJSON(ctx, &opts, &body, &result)
		return shouldRetry(ctx, resp, err)
	})
	if err != nil {
		return "", fmt.Errorf("couldn't create medium: %w", err)
	}
	if result.ID == "" {
		return "", errors.New("couldn't create medium: empty id returned")
	}
	return result.ID, nil
}

// createDerivative is step 2: POST /derivatives
func (f *Fs) createDerivative(ctx context.Context, mediumID, ext string, nParts int) (string, error) {
	tok, err := f.currentAccessToken(ctx)
	if err != nil {
		return "", err
	}
	rid, err := f.getResourceOwnerID(ctx)
	if err != nil {
		return "", err
	}
	body := map[string]any{
		"medium_id":         mediumID,
		"file_extension":    ext,
		"type":              "Source",
		"label":             "Source",
		"available":         false,
		"item_count":        nParts,
		"camera_positions":  "default",
		"on_public_profile": false,
		"access_token":      tok,
		"gopro_user_id":     rid,
	}
	opts := rest.Opts{Method: "POST", Path: "/derivatives"}
	var result struct {
		ID string `json:"id"`
	}
	var resp *http.Response
	err = f.pacer.Call(func() (bool, error) {
		resp, err = f.srv.CallJSON(ctx, &opts, &body, &result)
		return shouldRetry(ctx, resp, err)
	})
	if err != nil {
		return "", fmt.Errorf("couldn't create derivative: %w", err)
	}
	if result.ID == "" {
		return "", errors.New("couldn't create derivative: empty id returned")
	}
	return result.ID, nil
}

// createUpload is step 3a: POST /user-uploads
func (f *Fs) createUpload(ctx context.Context, derivativeID string) (string, error) {
	tok, err := f.currentAccessToken(ctx)
	if err != nil {
		return "", err
	}
	rid, err := f.getResourceOwnerID(ctx)
	if err != nil {
		return "", err
	}
	body := map[string]any{
		"derivative_id":   derivativeID,
		"camera_position": "default",
		"item_number":     1,
		"access_token":    tok,
		"gopro_user_id":   rid,
	}
	opts := rest.Opts{
		Method:       "POST",
		Path:         "/user-uploads",
		ExtraHeaders: map[string]string{"Accept": userUploadsAcceptHeader},
	}
	var result struct {
		ID string `json:"id"`
	}
	var resp *http.Response
	err = f.pacer.Call(func() (bool, error) {
		resp, err = f.srv.CallJSON(ctx, &opts, &body, &result)
		return shouldRetry(ctx, resp, err)
	})
	if err != nil {
		return "", fmt.Errorf("couldn't create upload: %w", err)
	}
	if result.ID == "" {
		return "", errors.New("couldn't create upload: empty id returned")
	}
	return result.ID, nil
}

// getUploadParts is step 3b: GET /user-uploads/{derivativeID}, returning
// one pre-signed authorization per chunk
func (f *Fs) getUploadParts(ctx context.Context, derivativeID, uploadID string, size, chunkSize int64, nParts int) ([]api.UploadAuthorization, error) {
	opts := rest.Opts{
		Method:       "GET",
		Path:         "/user-uploads/" + derivativeID,
		ExtraHeaders: map[string]string{"Accept": userUploadsAcceptHeader},
		Parameters: url.Values{
			"id":              {uploadID},
			"page":            {"1"},
			"per_page":        {strconv.Itoa(nParts)},
			"item_number":     {"1"},
			"camera_position": {"default"},
			"file_size":       {strconv.FormatInt(size, 10)},
			"part_size":       {strconv.FormatInt(chunkSize, 10)},
		},
	}
	var result api.UserUploadsResponse
	var resp *http.Response
	var err error
	err = f.pacer.Call(func() (bool, error) {
		resp, err = f.srv.CallJSON(ctx, &opts, nil, &result)
		return shouldRetry(ctx, resp, err)
	})
	if err != nil {
		return nil, fmt.Errorf("couldn't get upload authorizations: %w", err)
	}
	if len(result.Embedded.Authorizations) == 0 {
		return nil, errors.New("couldn't get upload authorizations: none returned")
	}
	return result.Embedded.Authorizations, nil
}

// completeUpload is the second half of step 4: PUT /user-uploads/{derivativeID}
// marking all chunks complete
func (f *Fs) completeUpload(ctx context.Context, derivativeID, uploadID string, size, chunkSize int64) error {
	body := map[string]any{
		"id":              uploadID,
		"item_number":     1,
		"camera_position": "default",
		"complete":        true,
		"derivative_id":   derivativeID,
		"file_size":       strconv.FormatInt(size, 10),
		"part_size":       strconv.FormatInt(chunkSize, 10),
	}
	opts := rest.Opts{
		Method:       "PUT",
		Path:         "/user-uploads/" + derivativeID,
		ExtraHeaders: map[string]string{"Accept": userUploadsAcceptHeader},
		NoResponse:   true,
	}
	var resp *http.Response
	var err error
	err = f.pacer.Call(func() (bool, error) {
		resp, err = f.srv.CallJSON(ctx, &opts, &body, nil)
		return shouldRetry(ctx, resp, err)
	})
	if err != nil {
		return fmt.Errorf("couldn't complete upload: %w", err)
	}
	return nil
}

// markDerivativeAvailable is step 5a: PUT /derivatives/{derivativeID}
func (f *Fs) markDerivativeAvailable(ctx context.Context, derivativeID string) error {
	tok, err := f.currentAccessToken(ctx)
	if err != nil {
		return err
	}
	rid, err := f.getResourceOwnerID(ctx)
	if err != nil {
		return err
	}
	body := map[string]any{
		"available":     true,
		"access_token":  tok,
		"gopro_user_id": rid,
	}
	opts := rest.Opts{
		Method:       "PUT",
		Path:         "/derivatives/" + derivativeID,
		ExtraHeaders: map[string]string{"Accept": userUploadsAcceptHeader},
		NoResponse:   true,
	}
	var resp *http.Response
	err = f.pacer.Call(func() (bool, error) {
		resp, err = f.srv.CallJSON(ctx, &opts, &body, nil)
		return shouldRetry(ctx, resp, err)
	})
	if err != nil {
		return fmt.Errorf("couldn't mark derivative available: %w", err)
	}
	return nil
}

// markMediumAvailable is step 5b: PUT /media/{mediumID}, the final step
// that completes the upload
func (f *Fs) markMediumAvailable(ctx context.Context, mediumID string) error {
	tok, err := f.currentAccessToken(ctx)
	if err != nil {
		return err
	}
	rid, err := f.getResourceOwnerID(ctx)
	if err != nil {
		return err
	}
	now := time.Now().UTC().Format(time.RFC3339)
	body := map[string]any{
		"upload_completed_at": now,
		"client_updated_at":   now,
		"revision_number":     0,
		"access_token":        tok,
		"gopro_user_id":       rid,
	}
	opts := rest.Opts{
		Method:     "PUT",
		Path:       "/media/" + mediumID,
		NoResponse: true,
	}
	var resp *http.Response
	err = f.pacer.Call(func() (bool, error) {
		resp, err = f.srv.CallJSON(ctx, &opts, &body, nil)
		return shouldRetry(ctx, resp, err)
	})
	if err != nil {
		return fmt.Errorf("couldn't finalise medium: %w", err)
	}
	return nil
}

// Check the interfaces are satisfied
var (
	_ fs.Fs              = &Fs{}
	_ fs.Abouter         = &Fs{}
	_ fs.OpenChunkWriter = &Fs{}
	_ fs.Object          = &Object{}
	_ fs.MimeTyper       = &Object{}
	_ fs.IDer            = &Object{}
	_ fs.ChunkWriter     = &gpChunkWriter{}
)
