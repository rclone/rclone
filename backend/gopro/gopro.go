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
	"runtime"
	"slices"
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
	"golang.org/x/sync/errgroup"
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

	// mediaCacheTTL bounds how long a full library or trash listing is
	// reused - see allMedia. A recursive listing then costs one full fetch
	// rather than one per by-year/by-month/by-day directory, while a
	// long-running mount still picks up changes made elsewhere.
	mediaCacheTTL = 5 * time.Minute

	// listPageSize is the largest per_page /media/search and
	// /media/deleted accept - they silently cap anything larger.
	listPageSize = 200

	// listConcurrency is how many /media/search pages allMedia fetches
	// at once
	listConcurrency = 4

	defaultUploadChunkSize   = fs.SizeSuffix(6 * 1024 * 1024) // matches the reference client
	defaultUploadConcurrency = 4

	// minUploadChunkSize is S3's multipart minimum, which GoPro's upload
	// enforces for every part but the last.
	minUploadChunkSize = fs.SizeSuffix(5 * 1024 * 1024)

	// rawLabel is the sidecar label of a photo's RAW file
	rawLabel = "raw_photo"

	// rawMimeType is the MIME type of RAW files - GPR is DNG-based
	rawMimeType = "image/x-adobe-dng"

	// delete_parts values - see that option's Help text.
	deletePartsFirst  = "first"
	deletePartsRefuse = "refuse"
	deletePartsAny    = "any"

	// photo_format values - see that option's Help text.
	photoFormatBoth = "both"
	photoFormatJPEG = "jpeg"
	photoFormatRaw  = "raw"

	// verify_size modes - see that option's Help text.
	verifySizeReprocessed = "reprocessed"
	verifySizeAlways      = "always"
	verifySizeOff         = "off"
)

// deletePermanentDelay is how long deleteMedium waits between the plain
// delete and the finalising permanent=true one. GoPro offers no reliable
// "ready to finalise" signal (GET /media/{id} returning 404 isn't one),
// so this is a fixed wait: 3s always worked in testing, 1s sometimes
// didn't. It's a var so tests can shrink it.
var deletePermanentDelay = 3 * time.Second

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

// checkDeleteParts checks that mode is a legal delete_parts value
func checkDeleteParts(mode string) error {
	switch mode {
	case "", deletePartsFirst, deletePartsRefuse, deletePartsAny:
		return nil
	default:
		return fmt.Errorf("unknown delete_parts %q (must be %q, %q or %q)", mode, deletePartsFirst, deletePartsRefuse, deletePartsAny)
	}
}

// checkPhotoFormat checks that format is a legal photo_format value
func checkPhotoFormat(format string) error {
	switch format {
	case "", photoFormatBoth, photoFormatJPEG, photoFormatRaw:
		return nil
	default:
		return fmt.Errorf("unknown photo_format %q (must be %q, %q or %q)", format, photoFormatBoth, photoFormatJPEG, photoFormatRaw)
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
	collectionsAcceptHeader = "application/vnd.gopro.jk.collections+json; version=2.0.0"

	// mediaFields is the set of /media/search fields this backend reads.
	mediaFields = "id,filename,file_extension,type,captured_at,created_at,file_size,width,height,camera_model,item_count,moments_count,ready_to_view,token,content_title,resolution,reprocessed_at,available_labels,composition"

	// includedTypes is the type filter for camera media - see mediaTypes.
	includedTypes = "Photo,Video,TimeLapse,TimeLapseVideo,Burst,BurstVideo,Chaptered,Continuous,Livestream,Looped,LoopedVideo,ExternalVideo,Session,Audio"

	// editTypes are the composed media types (Highlights and user-made
	// Edits) added by --gopro-include-edits. They carry a null file_size
	// and a file_extension that doesn't match what's actually downloaded
	// (see setMetaData and selectRendition).
	editTypes = "MultiClipEdit,Edit"
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
		CommandHelp: commandHelp,
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

"source" (the default) downloads the original camera file, or the
rendered video for Highlights and Edits. Any other value is matched
against the label or quality of the renditions GoPro offers (for
example "1080p" or "high_res_proxy_mp4"), falling back to the first
file offered if nothing matches. GoPro only reports the size of the
original, so any other rendition's size is unknown unless
[--gopro-read-size](#gopro-read-size) is set.`,
		}, {
			Name:     "include_edits",
			Advanced: true,
			Default:  true,
			Help: `Include Highlights and user-made Edits in listings.

These "MultiClipEdit"/"Edit" media are rendered from other clips.
GoPro reports no size for them, so their size is unknown unless
[--gopro-read-size](#gopro-read-size) is set (e.g. for rclone mount).
The rendered video is downloaded and they are listed as ".mp4" -
unnamed auto-generated Highlights as "{id}.mp4".

Turn this off to list only camera originals.`,
		}, {
			Name:     "include_processing",
			Advanced: true,
			Default:  false,
			Help: `Include media GoPro hasn't finished processing yet.

By default only media in the "ready" state is listed. This adds the
"uploading", "registered", "transcoding" and "stabilizing" states.
Media in these states is often downloadable already, but items with no
file size yet are still skipped.`,
		}, {
			Name:     "include_failed",
			Advanced: true,
			Default:  false,
			Help: `Include media stuck in a "failure" or "unknown" state.

Such media may have no usable content. This is mainly useful to find
and remove stuck items.`,
		}, {
			Name:     "show_all",
			Advanced: true,
			Default:  false,
			Help: `List everything in the library, bypassing all filters.

This ignores [--gopro-include-edits](#gopro-include-edits),
[--gopro-include-processing](#gopro-include-processing) and
[--gopro-include-failed](#gopro-include-failed), and also lists
"export" media (internal renders GoPro's own app never shows). It can
surface media this backend doesn't know how to handle, so use it for
troubleshooting rather than normal browsing.

It has no effect with [--gopro-trashed-only](#gopro-trashed-only),
which always lists everything.`,
		}, {
			Name:     "show_empty_dirs",
			Advanced: true,
			Default:  false,
			Help: `Show every media/by-year, by-month and by-day directory.

By default only years, months and days with media in them are listed.
A path under an unlisted day can still be used as a move destination
either way - this only changes what is listed.`,
		}, {
			Name:     "start_year",
			Advanced: true,
			Default:  0,
			Help: `Year to start media/by-year, by-month and by-day listings from.

0 (the default) uses the year of the earliest media in the library.
Set it together with [--gopro-show-empty-dirs](#gopro-show-empty-dirs)
to list earlier years.`,
		}, {
			Name:     "link_allow_download",
			Advanced: true,
			Default:  false,
			Help: `Allow downloading the original file from a public share link.

This is the "Allow Download" toggle in GoPro's web app. GoPro ties it
to sharing any GPS data embedded in the file, so enabling it shares
that location data with recipients too.`,
		}, {
			Name:     "link_title",
			Advanced: true,
			Help: `Title for public share links.

Defaults to the file's name without its {id} suffix, or no title for
media without a name. As "rclone link" can't pass a title, this applies
to every link created.`,
		}, {
			Name:     "use_trash",
			Advanced: true,
			Default:  true,
			Help: `Send deleted files to GoPro's trash instead of deleting permanently.

Trashed media shows as "Recently Deleted" in GoPro's app, can be
restored for up to 60 days (see "rclone backend restore" and
[--gopro-trashed-only](#gopro-trashed-only)) and still counts against
the storage quota. Media from GoPro cameras doesn't count against any
quota, so there's nothing to gain by skipping the trash for it.`,
		}, {
			Name:     "trashed_only",
			Advanced: true,
			Default:  false,
			Help: `Only show media in GoPro's trash.

With this set, every listing under media/ shows "Recently Deleted"
instead of the active library, including items the other filters would
hide. Deleting a file here removes it permanently, regardless of
[--gopro-use-trash](#gopro-use-trash). Use "rclone backend restore" to
move it back to the library.

To view the trash next to the normal library, override this per
command, e.g. "gopro,trashed_only=true:media/all".`,
		}, {
			Name:     "always_add_id",
			Advanced: true,
			Default:  true,
			Help: `Always add the media ID to file names, as "name {id}.ext".

GoPro cameras reuse file names, so names that collide within a listing
always get the ID. Without this option whether a file collides can
change from one run to the next, renaming it - sync then deletes and
re-transfers it. Only turn this off for a library without duplicate
names.`,
		}, {
			Name:     "verify_size",
			Advanced: true,
			Default:  verifySizeReprocessed,
			Help: `Verify file sizes with a HEAD request before relying on them.

The size GoPro reports can be wrong, which fails rclone's integrity
check and makes sync re-transfer the file on every run. The only files
seen affected had been reprocessed by GoPro after upload, so by default
only those are checked.`,
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
			Name:     "photo_format",
			Advanced: true,
			Default:  photoFormatBoth,
			Help: `Which files to list for photos shot with both JPEG and RAW.

A photo shot in RAW mode has a RAW (.gpr) file next to its JPEG, listed
under the same name. A photo with only one of the two is always listed,
whatever this is set to.`,
			Examples: []fs.OptionExample{{
				Value: photoFormatBoth,
				Help:  "List both the JPEG and the RAW file",
			}, {
				Value: photoFormatJPEG,
				Help:  "List only the JPEG",
			}, {
				Value: photoFormatRaw,
				Help:  "List only the RAW file",
			}},
		}, {
			Name:     "delete_parts",
			Advanced: true,
			Default:  deletePartsRefuse,
			Help: `How to delete a single chapter, frame or RAW file.

GoPro only deletes whole items: a chaptered video, a burst, continuous
or time lapse photo series, or a photo together with its RAW file.
Deleting just one of the files rclone lists for such an item would
delete all of them.

By default none of these files can be deleted on its own, so deleting
or moving one fails and the item is kept. Delete whole items with
"rclone backend delete" instead.

The other modes delete the whole item, so use them with care: rclone
deletes each file on its own, for example straight after moving it to
another remote, so the item's other files are deleted with it even if
they were never transferred.`,
			Examples: []fs.OptionExample{{
				Value: deletePartsRefuse,
				Help:  "No file of such an item can be deleted on its own - use \"rclone backend delete\"",
			}, {
				Value: deletePartsFirst,
				Help:  "Deleting the item's first file (chapter or frame 1, or the JPEG of a RAW photo) deletes the whole item",
			}, {
				Value: deletePartsAny,
				Help:  "Deleting any file of the item deletes the whole item",
			}},
		}, {
			Name:     "read_size",
			Advanced: true,
			Default:  false,
			Help: `Read the exact size of chaptered videos, burst photos and edits.

GoPro only reports the total size of a chaptered video or burst photo
set, and none for Highlights and Edits, so their size is unknown by
default. Set this if you need exact sizes, e.g. for rclone mount. This
costs one extra request per file.`,
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
	IncludeProcessing bool                 `config:"include_processing"`
	IncludeFailed     bool                 `config:"include_failed"`
	ShowAll           bool                 `config:"show_all"`
	ShowEmptyDirs     bool                 `config:"show_empty_dirs"`
	StartYear         int                  `config:"start_year"`
	LinkAllowDownload bool                 `config:"link_allow_download"`
	LinkTitle         string               `config:"link_title"`
	UseTrash          bool                 `config:"use_trash"`
	TrashedOnly       bool                 `config:"trashed_only"`
	AlwaysAddID       bool                 `config:"always_add_id"`
	VerifySize        string               `config:"verify_size"`
	PhotoFormat       string               `config:"photo_format"`
	DeleteParts       string               `config:"delete_parts"`
	ReadSize          bool                 `config:"read_size"`
	UploadChunkSize   fs.SizeSuffix        `config:"upload_chunk_size"`
	UploadConcurrency int                  `config:"upload_concurrency"`
	Enc               encoder.MultiEncoder `config:"encoding"`
}

// listCache holds a full library or trash listing
type listCache struct {
	mu    sync.Mutex
	items []api.Medium // nil means not (yet) cached
	at    time.Time
}

// invalidate makes the next read of c fetch fresh data
func (c *listCache) invalidate() {
	c.mu.Lock()
	c.items = nil
	c.mu.Unlock()
}

// sharedCache is a listCache shared by refs Fs
type sharedCache struct {
	cache *listCache
	refs  int
}

var (
	listCachesMu sync.Mutex
	listCaches   = map[string]*sharedCache{}
)

// acquireListCache returns the listCache for key, so that every Fs of one
// remote - rclone makes one per root - shares a single listing. Each call
// must be paired with a releaseListCache.
func acquireListCache(key string) *listCache {
	listCachesMu.Lock()
	defer listCachesMu.Unlock()
	c := listCaches[key]
	if c == nil {
		c = &sharedCache{cache: &listCache{}}
		listCaches[key] = c
	}
	c.refs++
	return c.cache
}

// releaseListCache drops a reference taken by acquireListCache, freeing
// the listing once no Fs uses it
func releaseListCache(key string) {
	listCachesMu.Lock()
	defer listCachesMu.Unlock()
	if c := listCaches[key]; c != nil {
		c.refs--
		if c.refs <= 0 {
			delete(listCaches, key)
		}
	}
}

// useSharedListCaches points f at the shared listCaches for mediaKey and
// trashKey, releasing them once f is garbage collected - long-running
// processes like rclone rc create many short-lived Fs.
func (f *Fs) useSharedListCaches(mediaKey, trashKey string) {
	f.media = acquireListCache(mediaKey)
	f.trash = acquireListCache(trashKey)
	runtime.AddCleanup(f, func(keys [2]string) {
		releaseListCache(keys[0])
		releaseListCache(keys[1])
	}, [2]string{mediaKey, trashKey})
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

	media *listCache // cached result of allMedia, shared - see sharedListCache
	trash *listCache // cached result of allTrash, shared - see sharedListCache

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
	sizeMu      sync.Mutex
	reprocessed bool // true if the parent medium's reprocessed_at is set - see verify_size's "reprocessed" mode
	raw         bool // true if this is the RAW (.gpr) file of a photo rather than the photo itself
	hasRaw      bool // true if the parent medium has RAW files next to its photos
	rawUpload   bool // true if this is a RAW file uploaded on its own - see selectURL
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

// startYear returns the year to start by-year style listings from:
// --gopro-start-year if set, otherwise the earliest captured_at year in
// listedMedia, or the current year if that is empty or can't be listed.
//
// Every item is scanned since the API's ordering isn't documented.
func (f *Fs) startYear(ctx context.Context) int {
	if f.opt.StartYear != 0 {
		return f.opt.StartYear
	}
	items, err := f.listedMedia(ctx)
	if err != nil || len(items) == 0 {
		return f.dirTime().Year()
	}
	year := items[0].CapturedAt.Year()
	for i := range items {
		if y := items[i].CapturedAt.Year(); y < year {
			year = y
		}
	}
	return year
}

// showEmptyDirs reports --gopro-show-empty-dirs
func (f *Fs) showEmptyDirs() bool {
	return f.opt.ShowEmptyDirs
}

// capturedDates returns the captured_at of every item in listedMedia, for
// deciding which date directories are non-empty
func (f *Fs) capturedDates(ctx context.Context) ([]time.Time, error) {
	items, err := f.listedMedia(ctx)
	if err != nil {
		return nil, err
	}
	dates := make([]time.Time, len(items))
	for i := range items {
		dates[i] = items[i].CapturedAt
	}
	return dates, nil
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
	if err := checkPhotoFormat(opt.PhotoFormat); err != nil {
		return nil, fmt.Errorf("gopro: %w", err)
	}
	if err := checkDeleteParts(opt.DeleteParts); err != nil {
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
	f.useSharedListCaches("media:"+name+"?"+f.searchParams().Encode(), "trash:"+name)
	// upload/ always exists, even with nothing uploaded to it yet. Seed its
	// listing when this Fs is at the top level; an Fs rooted at upload already
	// uses the empty key for its own root and must not gain upload/upload.
	if root == "" {
		f.uploaded["upload"] = nil
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
		// GoPro can revoke a stored token (refresh fails with
		// "token_blacklisted"), so re-authenticate with user/pass rather
		// than fail until "rclone config reconnect" is run. Token() only
		// makes a request when a refresh is due.
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
		PublicLink:   f.PublicLink,
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
// Media from GoPro cameras is "exempt" from any limit, so Total/Free/Used
// describe only the capped "non_exempt" pool when there is one.
func (f *Fs) About(ctx context.Context) (*fs.Usage, error) {
	info, err := f.getUserInfo(ctx)
	if err != nil {
		return nil, fmt.Errorf("couldn't read user quota: %w", err)
	}
	usage := &fs.Usage{
		Used: fs.NewUsageValue(info.TotalStorage),
	}
	if info.NonExemptStorageLimit > 0 {
		usage.Used = fs.NewUsageValue(info.NonExempt.TotalStorage)
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

// updateMedium changes the fields set on upd via PUT /media/{id} and
// invalidates the cached library
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
	f.invalidateMediaCache()
	return nil
}

// createCollection creates a public share link (a "collection" in GoPro's
// API) via POST /collections and returns its id - see PublicLink.
func (f *Fs) createCollection(ctx context.Context, title string, shareGPS bool) (string, error) {
	opts := rest.Opts{
		Method:       "POST",
		Path:         "/collections",
		ExtraHeaders: map[string]string{"Accept": collectionsAcceptHeader},
	}
	body := api.CollectionCreate{Title: title, Cloneable: shareGPS}
	var result api.Collection
	var resp *http.Response
	var err error
	err = f.pacer.Call(func() (bool, error) {
		resp, err = f.srv.CallJSON(ctx, &opts, &body, &result)
		return shouldRetry(ctx, resp, err)
	})
	if err != nil {
		return "", fmt.Errorf("couldn't create share link: %w", err)
	}
	return result.ID, nil
}

// addToCollection adds a medium to a share via PUT /collections/{id} - see
// PublicLink.
func (f *Fs) addToCollection(ctx context.Context, collectionID, mediumID string) error {
	opts := rest.Opts{
		Method:       "PUT",
		Path:         "/collections/" + collectionID,
		ExtraHeaders: map[string]string{"Accept": collectionsAcceptHeader},
		NoResponse:   true,
	}
	body := api.CollectionMediaUpdate{MediaIDs: []string{mediumID}}
	var resp *http.Response
	var err error
	err = f.pacer.Call(func() (bool, error) {
		resp, err = f.srv.CallJSON(ctx, &opts, &body, nil)
		return shouldRetry(ctx, resp, err)
	})
	if err != nil {
		return fmt.Errorf("couldn't add medium to share link: %w", err)
	}
	return nil
}

// mediaTypes returns the type filter for /media/search
func (f *Fs) mediaTypes() string {
	if f.opt.IncludeEdits {
		return includedTypes + "," + editTypes
	}
	return includedTypes
}

// isEditType reports whether t is one of editTypes (a Highlight or Edit)
func isEditType(t string) bool {
	return t == "MultiClipEdit" || t == "Edit"
}

// isFailedState reports whether readyToView is one of the states
// --gopro-include-failed adds
func isFailedState(readyToView string) bool {
	return readyToView == "failure" || readyToView == "unknown"
}

// processingStates returns the processing_states filter for /media/search
func (f *Fs) processingStates() string {
	states := []string{"ready"}
	if f.opt.IncludeProcessing {
		states = append(states, "uploading", "registered", "transcoding", "stabilizing")
	}
	if f.opt.IncludeFailed {
		states = append(states, "failure", "unknown")
	}
	return strings.Join(states, ",")
}

// skipsNullSize reports whether listDir skips item for its null file_size.
// Beyond edits (which always have one) there is no usable size or content
// to list. The trash and --gopro-show-all/--gopro-include-failed list it
// anyway, as there it's shown to be restored, inspected or removed.
func (f *Fs) skipsNullSize(item *api.Medium) bool {
	return item.FileSize == nil && !isEditType(item.Type) && !f.opt.ShowAll && !f.opt.TrashedOnly &&
		!(f.opt.IncludeFailed && isFailedState(item.ReadyToView))
}

// inListing reports whether the active library's listings include item,
// applying the filters /media/search applies (see searchParams) as well
// as listDir's own
func (f *Fs) inListing(item *api.Medium) bool {
	if !f.opt.ShowAll {
		if !slices.Contains(strings.Split(f.mediaTypes(), ","), item.Type) ||
			!slices.Contains(strings.Split(f.processingStates(), ","), item.ReadyToView) ||
			item.Composition == "export" {
			return false
		}
	}
	return !f.skipsNullSize(item)
}

// listedMedia returns what the media/ listings show: the cached trash
// under --gopro-trashed-only, otherwise the cached library.
func (f *Fs) listedMedia(ctx context.Context) ([]api.Medium, error) {
	if f.opt.TrashedOnly {
		return f.allTrash(ctx)
	}
	return f.allMedia(ctx)
}

// searchParams returns the /media/search parameters selecting what is
// listed, without the page
func (f *Fs) searchParams() url.Values {
	params := url.Values{
		"fields":   {mediaFields},
		"order_by": {"captured_at"},
		"per_page": {strconv.Itoa(listPageSize)},
	}
	if !f.opt.ShowAll {
		params.Set("type", f.mediaTypes())
		params.Set("processing_states", f.processingStates())
		// "export" media are renders made for sharing (POST
		// /media/{id}/export), which GoPro's own app never lists.
		params.Set("xcomposition", "export")
	}
	return params
}

// searchPage fetches one page of /media/search, returning its media and
// the total number of pages
func (f *Fs) searchPage(ctx context.Context, page int) ([]api.Medium, int, error) {
	params := f.searchParams()
	params.Set("page", strconv.Itoa(page))
	opts := rest.Opts{
		Method:     "GET",
		Path:       "/media/search",
		Parameters: params,
	}
	var result api.SearchResponse
	var resp *http.Response
	err := f.pacer.Call(func() (bool, error) {
		var err error
		resp, err = f.srv.CallJSON(ctx, &opts, nil, &result)
		return shouldRetry(ctx, resp, err)
	})
	if err != nil {
		return nil, 0, fmt.Errorf("couldn't list media: %w", err)
	}
	return result.Embedded.Media, result.Pages.TotalPages, nil
}

// allMedia returns every medium in the library that passes the type and
// processing filters, fetching /media/search in full and caching the
// result for mediaCacheTTL. Every media/ view is narrowed from this by
// listDir, so a recursive listing costs a single fetch.
func (f *Fs) allMedia(ctx context.Context) ([]api.Medium, error) {
	c := f.media
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.items != nil && time.Since(c.at) < mediaCacheTTL {
		return c.items, nil
	}
	first, totalPages, err := f.searchPage(ctx, 1)
	if err != nil {
		return nil, err
	}
	pages := make([][]api.Medium, max(totalPages, 1))
	pages[0] = first
	g, gCtx := errgroup.WithContext(ctx)
	g.SetLimit(listConcurrency)
	for page := 2; page <= totalPages; page++ {
		g.Go(func() (err error) {
			pages[page-1], _, err = f.searchPage(gCtx, page)
			return err
		})
	}
	if err := g.Wait(); err != nil {
		return nil, err
	}
	// A medium can turn up on two pages, e.g. when the library changes
	// while they're fetched.
	items := []api.Medium{}
	seen := map[string]bool{}
	for _, pageItems := range pages {
		for _, item := range pageItems {
			if !seen[item.ID] {
				seen[item.ID] = true
				items = append(items, item)
			}
		}
	}
	c.items, c.at = items, time.Now()
	return items, nil
}

// allTrash returns every trashed medium, fetching GET /media/deleted in
// full and caching the result for mediaCacheTTL.
//
// Nothing is filtered, matching GoPro's own "Recently Deleted" view -
// /media/deleted ignores /media/search's filter parameters anyway.
func (f *Fs) allTrash(ctx context.Context) ([]api.Medium, error) {
	c := f.trash
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.items != nil && time.Since(c.at) < mediaCacheTTL {
		return c.items, nil
	}
	items := []api.Medium{}
	for page := 1; ; page++ {
		opts := rest.Opts{
			Method: "GET",
			Path:   "/media/deleted",
			Parameters: url.Values{
				"page":     {strconv.Itoa(page)},
				"per_page": {strconv.Itoa(listPageSize)},
			},
		}
		var result api.DeletedMediaResponse
		var resp *http.Response
		err := f.pacer.Call(func() (bool, error) {
			var err error
			resp, err = f.srv.CallJSON(ctx, &opts, nil, &result)
			return shouldRetry(ctx, resp, err)
		})
		if err != nil {
			return nil, fmt.Errorf("couldn't list trash: %w", err)
		}
		items = append(items, result.DeletedMedia...)
		if len(result.DeletedMedia) == 0 || page >= result.Pages.TotalPages {
			break
		}
	}
	c.items, c.at = items, time.Now()
	return items, nil
}

// invalidateMediaCache makes the next allMedia fetch fresh data - call it
// after anything that changes the library
func (f *Fs) invalidateMediaCache() {
	f.media.invalidate()
}

// invalidateTrashCache makes the next allTrash fetch fresh data
func (f *Fs) invalidateTrashCache() {
	f.trash.invalidate()
}

// commandHelp documents this backend's "rclone backend" commands - see
// Command.
var commandHelp = []fs.CommandHelp{{
	Name:  "restore",
	Short: "Restore media from GoPro's trash",
	Long: `This restores media from GoPro's trash to the active library,
whether or not --gopro-trashed-only is set.

With no arguments, it restores everything in the trash:

    rclone backend restore gopro:

Otherwise each argument names one medium to restore, either by its id
or by its "name {id}.ext" file name as listed with --gopro-trashed-only:

    rclone backend restore gopro: 6a99f18a239bf36f4c2377cf "photo {6a99f18a239bf36f4c2377cf}.jpg"

With --dry-run, it only logs what would be restored.

GoPro restores asynchronously and reports no failures, so a restored
item can take a while to reappear in the library, and occasionally
doesn't at all. Check with --gopro-trashed-only if one is missing.`,
}, {
	Name:  "delete",
	Short: "Delete whole media, with all their files",
	Long: `This deletes whole media, including every chapter, frame and RAW
file of each, following --gopro-use-trash. See --gopro-delete-parts for
why deleting one of those files on its own may not be possible.

Each argument names one medium, either by its id or by the name of any
of its files as listed by this backend:

    rclone backend delete gopro: 6a29a4bcfe314c5af39cfcbe "GX012010-2 {6a29a4bcfe314c5af39cfcbe}.MP4"

With --dry-run, it only logs what would be deleted.`,
}, {
	Name:  "link",
	Short: "Create public share links for whole media",
	Long: `This creates a public share link for each medium named, following
--gopro-link-title and --gopro-link-allow-download, and prints the links.

GoPro shares whole media, so "rclone link" refuses a single chapter,
frame or RAW file - this shares every file of the medium instead. Each
argument names one medium, either by its id or by the name of any of its
files as listed by this backend:

    rclone backend link gopro: "GX012010-2 {6a29a4bcfe314c5af39cfcbe}.MP4"

With --dry-run, it only logs what would be shared.`,
}}

// Command the backend to run a named command
//
// The command run is name
// args may be used to read arguments from
// opts may be used to read optional arguments from
//
// The result should be capable of being JSON encoded
// If it is a string or a []string it will be shown to the user
// otherwise it will be JSON encoded and shown to the user like that
func (f *Fs) Command(ctx context.Context, name string, arg []string, opt map[string]string) (any, error) {
	switch name {
	case "restore":
		return f.restore(ctx, arg)
	case "delete":
		return f.deleteCommand(ctx, arg)
	case "link":
		return f.linkCommand(ctx, arg)
	}
	return nil, fs.ErrorCommandNotFound
}

// deleteResult is returned by the "delete" backend command
type deleteResult struct {
	Deleted int
}

// deleteCommand implements the "delete" backend command
func (f *Fs) deleteCommand(ctx context.Context, arg []string) (any, error) {
	if len(arg) == 0 {
		return nil, errors.New("name at least one medium to delete")
	}
	ids := make([]string, len(arg))
	for i, a := range arg {
		ids[i] = restoreArg(a)
	}
	if fs.GetConfig(ctx).DryRun {
		fs.Logf(f, "Would delete %d medium(s): %v", len(ids), ids)
		return &deleteResult{}, nil
	}
	res := &deleteResult{}
	for _, id := range ids {
		var err error
		if f.opt.TrashedOnly {
			// A trashed medium can only be purged - see Object.Remove.
			err = f.doDeleteMedium(ctx, id, "permanent", "true")
		} else {
			err = f.deleteMedium(ctx, id, !f.opt.UseTrash)
		}
		if err != nil {
			return res, err
		}
		res.Deleted++
	}
	return res, nil
}

// restoreResult is returned by the "restore" backend command
type restoreResult struct {
	Restored int
}

// restoreArg resolves one "restore" argument - a bare id or a
// "name {id}.ext" leaf - to a medium id
func restoreArg(arg string) string {
	if m := fileIDRe.FindStringSubmatch(arg); m != nil {
		return m[1]
	}
	if id := findID(path.Base(arg)); id != "" {
		return id
	}
	return arg
}

// fileIDRe matches an Object's ID(), capturing its medium's id
var fileIDRe = regexp.MustCompile(`^([0-9a-f]{24})(?:/[0-9]+)?(?:/raw)?$`)

// restore implements the "restore" backend command
func (f *Fs) restore(ctx context.Context, arg []string) (any, error) {
	ids := make([]string, len(arg))
	for i, a := range arg {
		ids[i] = restoreArg(a)
	}
	if len(ids) == 0 {
		items, err := f.allTrash(ctx)
		if err != nil {
			return nil, fmt.Errorf("couldn't list trash: %w", err)
		}
		for i := range items {
			ids = append(ids, items[i].ID)
		}
	}
	if len(ids) == 0 {
		return &restoreResult{}, nil
	}
	if fs.GetConfig(ctx).DryRun {
		fs.Logf(f, "Would restore %d medium(s) from trash: %v", len(ids), ids)
		return &restoreResult{}, nil
	}
	if err := f.restoreMedia(ctx, ids); err != nil {
		return nil, err
	}
	return &restoreResult{Restored: len(ids)}, nil
}

// restoreMedia restores ids from the trash with one POST /media/restore.
//
// GoPro answers 202 Accepted and restores asynchronously, with no way to
// confirm completion, so the caches are invalidated on the 202 alone.
func (f *Fs) restoreMedia(ctx context.Context, ids []string) error {
	opts := rest.Opts{
		Method:     "POST",
		Path:       "/media/restore",
		NoResponse: true,
	}
	body := api.RestoreRequest{IDs: ids}
	var resp *http.Response
	var err error
	err = f.pacer.Call(func() (bool, error) {
		resp, err = f.srv.CallJSON(ctx, &opts, &body, nil)
		return shouldRetry(ctx, resp, err)
	})
	if err != nil {
		return fmt.Errorf("couldn't restore media: %w", err)
	}
	f.invalidateMediaCache()
	f.invalidateTrashCache()
	return nil
}

// addID adds the ID to name, which may be a path whose leaf is empty
func addID(name string, ID string) string {
	idStr := "{" + ID + "}"
	if name == "" || strings.HasSuffix(name, "/") {
		return name + idStr
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

// idSuffixRe matches the " {id}" (or bare "{id}") suffix addID appends,
// anchored to the end so an id-shaped substring elsewhere in a filename
// never matches
var idSuffixRe = regexp.MustCompile(` ?\{([0-9a-f]{24})\}$`)

// findID returns the id from a trailing {id} suffix of name (before its
// extension), or "" if there isn't one.
//
// Media can be renamed to anything, so a match doesn't prove this backend
// added the suffix - see readMetaData, which verifies it.
func findID(name string) string {
	ext := path.Ext(name)
	base := strings.TrimSuffix(name, ext)
	match := idSuffixRe.FindStringSubmatch(base)
	if match == nil {
		return ""
	}
	return match[1]
}

// stripSuffixID removes a trailing {id} suffix from a leaf name - the
// inverse of addFileID. Only a suffix matching id is removed, so a real
// filename that happens to end in another id is left intact.
func stripSuffixID(name, id string) string {
	ext := path.Ext(name)
	base := strings.TrimSuffix(name, ext)
	match := idSuffixRe.FindStringSubmatch(base)
	if match == nil || match[1] != id {
		return name
	}
	return idSuffixRe.ReplaceAllString(base, "") + ext
}

// listDir lists a single directory, applying filter and adding the {id}
// suffix to names that collide - GoPro cameras reuse filenames constantly
func (f *Fs) listDir(ctx context.Context, prefix string, filter mediaFilter) (entries fs.DirEntries, err error) {
	items, err := f.listedMedia(ctx)
	if err != nil {
		return nil, err
	}
	for i := range items {
		item := &items[i]
		if !filter.matches(item.CapturedAt) {
			continue
		}
		if f.skipsNullSize(item) {
			fs.Debugf(f, "Skipping %s: ready but file_size is null", item.ID)
			continue
		}
		itemCount := item.ItemCount
		if itemCount < 1 {
			itemCount = 1
		}
		leaf := mediumLeaf(f, item)
		for n := 1; n <= itemCount; n++ {
			remote := leaf
			if itemCount > 1 {
				remote = itemLeaf(leaf, n)
			}
			if f.listPhoto(item) {
				o := &Object{fs: f, remote: prefix + remote}
				o.setMetaData(item, n)
				entries = append(entries, o)
			}
			if f.listRaw(item) {
				r := &Object{fs: f, remote: prefix + rawLeaf(remote), raw: true}
				r.setMetaData(item, n)
				entries = append(entries, r)
			}
		}
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
// this same listing share that remote (count). A leaf with no name before
// its extension (an unnamed medium) always gets one regardless of the
// option or count, since there's nothing else to show.
func shouldAddID(alwaysAddID bool, remote string, count int) bool {
	leaf := remote[strings.LastIndex(remote, "/")+1:]
	return alwaysAddID || count > 1 || isUnnamedLeaf(leaf)
}

// isUnnamedLeaf reports whether leaf has nothing before its extension
func isUnnamedLeaf(leaf string) bool {
	return strings.TrimSuffix(leaf, path.Ext(leaf)) == ""
}

// downloadExtension returns the extension (without the dot) of what's
// actually downloaded for item. A MultiClipEdit/Edit's own file_extension
// is that of its Edit Decision List ("json"), but what's served is the
// rendered video.
func downloadExtension(item *api.Medium) string {
	if isEditType(item.Type) {
		return "mp4"
	}
	return item.FileExtension
}

// mediumLeaf returns the leaf name listDir gives item, before any item
// number or ID suffix: its filename, with the extension from photoExt.
func mediumLeaf(f *Fs, item *api.Medium) string {
	leaf := f.opt.Enc.ToStandardName(item.Filename)
	if isEditType(item.Type) {
		// An edit's filename is the title given in GoPro's app, which
		// may contain dots, so it's kept whole.
		if strings.EqualFold(path.Ext(leaf), ".mp4") {
			return leaf
		}
		return leaf + ".mp4"
	}
	return strings.TrimSuffix(leaf, path.Ext(leaf)) + photoExt(item)
}

// photoExt returns the extension (with the dot) of what's downloaded as
// item's photo or video - usually that of its filename, but:
//
//   - GoPro's auto-generated Highlights often have no filename at all.
//   - A RAW file uploaded on its own keeps its .gpr filename, but GoPro
//     generates a JPEG as its photo, and lists the RAW as a sidecar.
func photoExt(item *api.Medium) string {
	ext := path.Ext(item.Filename)
	switch {
	case isEditType(item.Type):
		return "." + downloadExtension(item)
	case ext == "" && downloadExtension(item) != "":
		return "." + downloadExtension(item)
	case strings.EqualFold(ext, ".gpr") && hasRaw(item) && item.FileExtension != "":
		return withExtLike("."+item.FileExtension, ext)
	}
	return ext
}

// expectedIDSuffixedName reconstructs the id-suffixed leaf listDir would
// give item's first item when --gopro-always-add-id is set, for verifying
// a name found via the readMetaData fast path actually belongs to it.
func expectedIDSuffixedName(f *Fs, item *api.Medium) string {
	leaf := mediumLeaf(f, item)
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
	o := &Object{fs: f, remote: src.Remote(), bytes: -1}
	if err := o.Update(ctx, in, src, options...); err != nil {
		// Update only records a medium once it's stored, so there's
		// nothing to return.
		return nil, err
	}
	return o, nil
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
		dirPath := strings.Trim(prefix, "/")
		// dirtree.AddEntry doesn't dedup, and a duplicate entry breaks
		// dirtree.Prune in Rmdir.
		if _, entry := f.uploaded.Find(dirPath); entry == nil {
			f.uploaded.AddEntry(fs.NewDir(dirPath, f.dirTime()))
		}
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
	match, prefix, pattern := patterns.match(f.root, dir, false)
	if pattern == nil {
		return fs.ErrorDirNotFound
	}
	if pattern.isUpload {
		if match[0] == "upload" {
			// upload/ itself always exists.
			return errCantRmdir
		}
		f.uploadedMu.Lock()
		defer f.uploadedMu.Unlock()
		dirPath := strings.Trim(prefix, "/")
		entries, ok := f.uploaded[dirPath]
		if !ok {
			return fs.ErrorDirNotFound
		}
		// dirtree.Prune doesn't check for emptiness itself.
		if len(entries) > 0 {
			return fs.ErrorDirectoryNotEmpty
		}
		return f.uploaded.Prune(map[string]bool{dirPath: true})
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
//   - A burst, continuous or time lapse photo set (item_count > 1): the
//     reverse - files[] holds one entry per photo, keyed by item_number,
//     while variations[] holds a single "source" entry with no
//     item_number, which is a cover image representing the set, not any
//     individual photo.
//
// This is told apart at runtime by counting "source"-labelled variations
// rather than switching on the medium's "type": only Video, Burst,
// Continuous and TimeLapse are verified against real examples, and other
// types may follow either shape.
func selectRendition(dl *api.DownloadResponse, variation string, itemNumber int) (dlURL, head string, err error) {
	if variation == "" {
		variation = "source"
	}
	if variation != "source" {
		// An explicit variation can be chaptered exactly like "source" -
		// one entry per item_number - or offered as a single shared
		// rendition with no item_number of its own (ItemNumber 0), so
		// prefer an exact item_number match, fall back to an unnumbered
		// shared one, and finally to the item's own offered file, rather
		// than erroring out despite the option's documented file
		// fallback.
		var shared *api.File
		for i, v := range dl.Embedded.Variations {
			if v.Label != variation && v.Quality != variation {
				continue
			}
			if v.ItemNumber == itemNumber {
				return v.URL, v.Head, nil
			}
			if v.ItemNumber == 0 && shared == nil {
				shared = &dl.Embedded.Variations[i]
			}
		}
		if shared != nil {
			return shared.URL, shared.Head, nil
		}
		for _, file := range dl.Embedded.Files {
			if file.ItemNumber == itemNumber {
				return file.URL, file.Head, nil
			}
		}
		return "", "", fmt.Errorf("no %q rendition found", variation)
	}

	sourceVariations := 0
	for _, v := range dl.Embedded.Variations {
		if isSourceLabel(v.Label) {
			sourceVariations++
		}
	}
	if sourceVariations > 1 {
		// Chaptered-video shape: one "source" variation per item_number.
		for _, v := range dl.Embedded.Variations {
			if isSourceLabel(v.Label) && v.ItemNumber == itemNumber {
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
			if isSourceLabel(v.Label) {
				return v.URL, v.Head, nil
			}
		}
		if len(dl.Embedded.Files) > 0 {
			return dl.Embedded.Files[0].URL, dl.Embedded.Files[0].Head, nil
		}
	}
	return "", "", fmt.Errorf("no source rendition found for item %d", itemNumber)
}

// isSourceLabel reports whether a variation label names the original:
// "source" for camera media, "baked_source" (the rendered video) for a
// MultiClipEdit/Edit, which has no "source" variation at all.
func isSourceLabel(label string) bool {
	return label == "source" || label == "baked_source"
}

// selectURL picks o's download URL and HEAD-able URL from dl
func (o *Object) selectURL(dl *api.DownloadResponse) (dlURL, head string, err error) {
	if o.raw {
		return selectRaw(dl, o.itemNumber)
	}
	if o.rawUpload {
		// Once processed, the source is a JPEG GoPro generated and the
		// upload its RAW file; before that the source is the upload.
		if dlURL, head, err := selectRaw(dl, o.itemNumber); err == nil {
			return dlURL, head, nil
		}
	}
	return selectRendition(dl, o.fs.opt.DownloadVariation, o.itemNumber)
}

// selectRaw picks the download URL and HEAD-able URL of the RAW file for
// itemNumber - unnumbered for a single photo, numbered for a series.
func selectRaw(dl *api.DownloadResponse, itemNumber int) (dlURL, head string, err error) {
	for _, s := range dl.Embedded.SidecarFiles {
		if s.Label == rawLabel && (s.ItemNumber == itemNumber || s.ItemNumber == 0 && itemNumber <= 1) {
			return s.URL, s.Head, nil
		}
	}
	return "", "", fmt.Errorf("no RAW file found for item %d", itemNumber)
}

// hasRaw reports whether item's photos come with RAW (.gpr) files
func hasRaw(item *api.Medium) bool {
	return slices.Contains(item.AvailableLabels, rawLabel)
}

// listPhoto reports whether item's photo (or video) files are listed -
// always, unless it has RAW files and --gopro-photo-format is "raw"
func (f *Fs) listPhoto(item *api.Medium) bool {
	return f.opt.PhotoFormat != photoFormatRaw || !hasRaw(item)
}

// listRaw reports whether item's RAW files are listed next to its photos
func (f *Fs) listRaw(item *api.Medium) bool {
	return f.opt.PhotoFormat != photoFormatJPEG && hasRaw(item)
}

// rawLeaf returns the name of the RAW file next to the photo leaf,
// matching the case of its extension
func rawLeaf(leaf string) string {
	ext := path.Ext(leaf)
	return strings.TrimSuffix(leaf, ext) + withExtLike(".gpr", ext)
}

// withExtLike returns ext in upper case if like is, else in lower case
func withExtLike(ext, like string) string {
	if like == strings.ToUpper(like) {
		return strings.ToUpper(ext)
	}
	return strings.ToLower(ext)
}

// errNoMedium is returned for an Object with no medium behind it, such as
// one a failed upload left, rather than sending an empty medium id
var errNoMedium = fmt.Errorf("gopro: no medium stored: %w", fs.ErrorObjectNotFound)

// isNotFound reports whether err is a 404 from the API
func isNotFound(err error) bool {
	var apiErr *api.Error
	return errors.As(err, &apiErr) && apiErr.Status == http.StatusNotFound
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
	for k, e := range f.dlCache {
		if time.Since(e.fetched) >= dlCacheTTL {
			delete(f.dlCache, k)
		}
	}
	f.dlCache[id] = &dlCacheEntry{resp: &result, fetched: time.Now()}
	f.dlCacheMu.Unlock()
	return &result, nil
}

// sizeCheckTimeout bounds the requests Size makes. A var so tests can
// shrink it.
var sizeCheckTimeout = time.Minute

// forgetDownload drops the cached download descriptor of medium id
func (f *Fs) forgetDownload(id string) {
	f.dlCacheMu.Lock()
	delete(f.dlCache, id)
	f.dlCacheMu.Unlock()
}

// Size returns the size of an object in bytes
//
// file_size from the API can be stale, which breaks multi-thread
// downloads (chunked by this size up front) and makes sync re-transfer
// the file on every run. --gopro-verify-size decides which known sizes
// are checked with a HEAD, and --gopro-read-size whether unknown ones
// (-1, see setMetaData) are resolved that way. The result is kept for
// this Object's lifetime.
func (o *Object) Size() int64 {
	o.sizeMu.Lock()
	defer o.sizeMu.Unlock()

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
	// Size has no context to follow, so bound what it waits for.
	ctx, cancel := context.WithTimeout(context.Background(), sizeCheckTimeout)
	defer cancel()
	dl, err := o.fs.getDownload(ctx, o.id)
	if err != nil {
		fs.Debugf(o, "Size: %v", err)
		return o.bytes
	}
	_, head, err := o.selectURL(dl)
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
	defer fs.CheckClose(resp.Body, &err)
	length, err := strconv.ParseInt(resp.Header.Get("Content-Length"), 10, 64)
	if err != nil {
		fs.Debugf(o, "Size: couldn't parse Content-Length: %v", err)
		return o.bytes
	}
	o.reportSizeMismatchLocked(length)
	o.sizeChecked = true
	return o.bytes
}

// reportSizeMismatch corrects o.bytes to actual, logging a notice if
// that changes a known size
func (o *Object) reportSizeMismatch(actual int64) {
	o.sizeMu.Lock()
	defer o.sizeMu.Unlock()
	o.reportSizeMismatchLocked(actual)
}

// reportSizeMismatchLocked is reportSizeMismatch with o.sizeMu held
func (o *Object) reportSizeMismatchLocked(actual int64) {
	if o.bytes >= 0 && actual != o.bytes {
		fs.Logf(o, "file_size from the GoPro API (%d) doesn't match the size actually being served (%d) - using the actual size; downloading anyway", o.bytes, actual)
	}
	o.bytes = actual
}

// copyFrom copies Object metadata without copying its mutex.
func (o *Object) copyFrom(src *Object) {
	if o == src {
		return
	}
	o.sizeMu.Lock()
	defer o.sizeMu.Unlock()
	src.sizeMu.Lock()
	defer src.sizeMu.Unlock()
	o.fs = src.fs
	o.remote = src.remote
	o.id = src.id
	o.itemNumber = src.itemNumber
	o.itemCount = src.itemCount
	o.bytes = src.bytes
	o.sizeChecked = src.sizeChecked
	o.reprocessed = src.reprocessed
	o.modTime = src.modTime
	o.mimeType = src.mimeType
	o.raw = src.raw
	o.hasRaw = src.hasRaw
	o.rawUpload = src.rawUpload
}

// setMetaData sets the Object data from a Medium
func (o *Object) setMetaData(item *api.Medium, itemNumber int) {
	o.sizeMu.Lock()
	defer o.sizeMu.Unlock()
	o.setMetaDataLocked(item, itemNumber)
}

// setMetaDataLocked sets the Object data from a Medium.
// The caller must hold o.sizeMu.
func (o *Object) setMetaDataLocked(item *api.Medium, itemNumber int) {
	o.id = item.ID
	o.itemNumber = itemNumber
	o.itemCount = item.ItemCount
	if o.itemCount < 1 {
		o.itemCount = 1
	}
	// For a multi-item medium (chaptered video, burst photo set)
	// file_size is the total of all items, so this item's size is left
	// unknown rather than estimated - a wrong size fails rclone's
	// integrity check, an unknown one skips it.
	o.bytes = -1
	if item.FileSize != nil && o.itemCount <= 1 {
		o.bytes = *item.FileSize
	}
	o.modTime = item.CapturedAt
	if o.modTime.IsZero() {
		o.modTime = item.CreatedAt
	}
	o.mimeType = mime.TypeByExtension(strings.ToLower(photoExt(item)))
	o.reprocessed = item.ReprocessedAt != nil
	o.hasRaw = hasRaw(item)
	if o.fs != nil && o.fs.opt.DownloadVariation != "" && o.fs.opt.DownloadVariation != "source" {
		// file_size is the original's, not the rendition's.
		o.bytes = -1
	}
	// What this backend uploads keeps its extension here, while GoPro
	// lists a processed RAW upload as the JPEG it generates for it.
	o.rawUpload = item.FileExtension == "gpr"
	if o.rawUpload {
		o.mimeType = rawMimeType
	}
	if o.raw {
		// file_size only covers the photos.
		o.bytes = -1
		o.mimeType = rawMimeType
	}
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
	match, _, pattern := patterns.match(o.fs.root, o.remote, true)
	if pattern == nil {
		return fs.ErrorObjectNotFound
	}
	if !pattern.isFile {
		return fs.ErrorNotAFile
	}
	// With an {id} suffix, fetch the medium directly - but only trust it
	// if the listing of this directory would show it under exactly this
	// name, since a renamed file can end in any id, and rclone takes the
	// same medium found under another path for a different file. Only
	// item 1 of a multi-item medium can match; the others fall through to
	// the listing. upload/ only lists this run's uploads, and GET
	// /media/{id} 404s for trashed media, so neither uses this.
	if id := findID(fileName); id != "" && o.fs.opt.AlwaysAddID && !o.fs.opt.TrashedOnly && !pattern.isUpload {
		filter, err := viewFilter(match)
		if err != nil {
			return fs.ErrorObjectNotFound
		}
		item, err := o.fs.getMedium(ctx, id)
		inView := err == nil && o.fs.inListing(item) && filter.matches(item.CapturedAt)
		switch {
		case isNotFound(err):
			// Deleted or trashed since it was listed - let the listing
			// below decide, which reports fs.ErrorObjectNotFound.
		case err != nil:
			return err
		case !inView:
		case o.fs.listPhoto(item) && expectedIDSuffixedName(o.fs, item) == fileName:
			o.setMetaData(item, 1)
			return nil
		case o.fs.listRaw(item) && rawLeaf(expectedIDSuffixedName(o.fs, item)) == fileName:
			o.raw = true
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
				o.copyFrom(newO)
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

// SetModTime sets the medium's captured_at, which is also what the
// by-year/by-month/by-day directories are based on. All items of a
// multi-item medium share it.
func (o *Object) SetModTime(ctx context.Context, modTime time.Time) error {
	if o.id == "" {
		return errNoMedium
	}
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
	if o.id == "" {
		return nil, errNoMedium
	}
	resp, err := o.download(ctx, options)
	var apiErr *api.Error
	if errors.As(err, &apiErr) && apiErr.Status == http.StatusForbidden {
		// The CDN refuses signed URLs once they expire, which a cached
		// download descriptor's may have - try once with a fresh one.
		fs.Debugf(o, "Open: fetching a fresh download URL after: %v", err)
		o.fs.forgetDownload(o.id)
		resp, err = o.download(ctx, options)
	}
	if err != nil {
		return nil, err
	}
	o.fixSize(resp)
	return resp.Body, nil
}

// download starts downloading o
func (o *Object) download(ctx context.Context, options []fs.OpenOption) (*http.Response, error) {
	dl, err := o.fs.getDownload(ctx, o.id)
	if err != nil {
		return nil, err
	}
	dlURL, _, err := o.selectURL(dl)
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
	return resp, err
}

// fixSize resolves o.bytes from a download response, if Size hasn't
// already
func (o *Object) fixSize(resp *http.Response) {
	o.sizeMu.Lock()
	defer o.sizeMu.Unlock()

	if o.sizeChecked {
		return
	}
	total := int64(-1)
	if resp.StatusCode == http.StatusPartialContent {
		// Content-Length is only the range here; the total is in
		// "Content-Range: bytes a-b/total".
		if _, after, ok := strings.Cut(resp.Header.Get("Content-Range"), "/"); ok && after != "*" {
			if n, err := strconv.ParseInt(after, 10, 64); err == nil {
				total = n
			}
		}
	} else {
		total = resp.ContentLength
	}
	if total >= 0 {
		o.reportSizeMismatchLocked(total)
		o.sizeChecked = true
	}
}

// Update the object with the contents of the io.Reader, modTime and size.
// The new object may have been created if an error is returned.
//
// GoPro can't replace a medium's content, so this uploads a new medium
// and then deletes the one it replaces - see deleteReplaced.
func (o *Object) Update(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) error {
	chunkWriter, err := multipart.UploadMultipart(ctx, src, in, multipart.UploadMultipartOptions{
		Open:        o.fs,
		OpenOptions: options,
	})
	if err != nil {
		return err
	}
	w := chunkWriter.(*gpChunkWriter)
	// Close has already replaced whatever was listed at this remote.
	if o.id != "" && o.id != w.mediumID && o.id != w.replacedID {
		if err := o.fs.replaceMedium(ctx, o.id, w.mediumID); err != nil {
			o.fs.removeUploadedEntry(o.remote)
			return err
		}
	}
	o.setMetaData(w.medium, 1)
	return nil
}

// replaceCheckInterval and replaceTimeout bound how often and how long
// replaceMedium waits for GoPro to process a replacement. vars so tests
// can shrink them.
var (
	replaceCheckInterval = 2 * time.Second
	replaceTimeout       = 2 * time.Minute
)

// replaceMedium deletes oldID once its replacement newID is safely
// stored, following --gopro-use-trash.
//
// GoPro removes an upload whose image content matches media already in
// the library while processing it, not straight away, so this waits for
// processing to finish. If GoPro removes the replacement, oldID is kept
// and an error returned; if processing outlasts replaceTimeout, oldID is
// kept and only logged about.
func (f *Fs) replaceMedium(ctx context.Context, oldID, newID string) error {
	deadline := time.Now().Add(replaceTimeout)
	for {
		item, err := f.getMedium(ctx, newID)
		switch {
		case isNotFound(err):
			return fmt.Errorf("gopro: GoPro removed the upload as a duplicate of media already in the library - kept medium %q it was to replace", oldID)
		case err != nil:
			return err
		case item.ReadyToView == "ready" || isFailedState(item.ReadyToView):
			f.deleteReplaced(ctx, oldID)
			return nil
		}
		if time.Now().After(deadline) {
			fs.Logf(f, "GoPro is still processing upload %q - kept medium %q it replaces, delete it once the upload is processed", newID, oldID)
			return nil
		}
		select {
		case <-time.After(replaceCheckInterval):
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

// deleteReplaced deletes the medium id an upload has just replaced,
// following --gopro-use-trash. A failure is only logged, as the new
// content is already in place.
func (f *Fs) deleteReplaced(ctx context.Context, id string) {
	if err := f.deleteMedium(ctx, id, !f.opt.UseTrash); err != nil {
		fs.Errorf(f, "couldn't delete medium %q replaced by an upload: %v", id, err)
	}
}

// Remove an object
//
// GoPro only deletes whole media, so removing one part of a medium with
// several (see isPart) follows --gopro-delete-parts. Under
// --gopro-trashed-only the object is already in the trash, where a plain
// delete fails, so it's purged directly.
func (o *Object) Remove(ctx context.Context) error {
	if o.id == "" {
		return errNoMedium
	}
	if o.isPart() {
		if !o.deletesWhole() {
			return o.removePart(ctx)
		}
		// Another part of the medium may have deleted it already.
		if exists, err := o.mediumExists(ctx); err != nil || !exists {
			return err
		}
	}
	var err error
	if o.fs.opt.TrashedOnly {
		err = o.fs.doDeleteMedium(ctx, o.id, "permanent", "true")
	} else {
		err = o.fs.deleteMedium(ctx, o.id, !o.fs.opt.UseTrash)
	}
	if err != nil {
		return err
	}
	o.fs.removeUploadedEntry(o.remote)
	return nil
}

// isPart reports whether o is one of several files of its medium - a
// chapter, a frame of a photo series, or a photo or its RAW file
func (o *Object) isPart() bool {
	return o.itemCount > 1 || o.hasRaw || o.raw
}

// deletesWhole reports whether removing o may delete its whole medium,
// following --gopro-delete-parts. The first part is the first item's
// photo, or its RAW file when only RAW files are listed.
func (o *Object) deletesWhole() bool {
	switch o.fs.opt.DeleteParts {
	case deletePartsAny:
		return true
	case deletePartsFirst:
		firstIsRaw := o.hasRaw && o.fs.opt.PhotoFormat == photoFormatRaw
		return o.itemNumber <= 1 && o.raw == firstIsRaw
	}
	return false
}

// mediumExists reports whether o's medium is still where o is listed -
// in the library, or in the trash under --gopro-trashed-only
func (o *Object) mediumExists(ctx context.Context) (bool, error) {
	if o.fs.opt.TrashedOnly {
		o.fs.invalidateTrashCache()
		items, err := o.fs.allTrash(ctx)
		if err != nil {
			return false, err
		}
		for i := range items {
			if items[i].ID == o.id {
				return true, nil
			}
		}
		return false, nil
	}
	_, err := o.fs.getMedium(ctx, o.id)
	if isNotFound(err) {
		return false, nil
	}
	return err == nil, err
}

// removePart removes a part of a medium that can't delete it - so this
// only succeeds once the medium has been deleted, as when deleting a
// whole directory.
func (o *Object) removePart(ctx context.Context) error {
	exists, err := o.mediumExists(ctx)
	if err != nil {
		return err
	}
	if exists {
		how := "use \"rclone backend delete\" to delete them all"
		if o.fs.opt.DeleteParts == deletePartsFirst {
			how = "delete its first file to delete them all, or use \"rclone backend delete\""
		}
		return fmt.Errorf("gopro: can't delete %q on its own - GoPro only deletes it together with the other files of its item: %s", o.remote, how)
	}
	return nil
}

// removeUploadedEntry removes the object at remote from the in-memory
// upload tree, if it's there
func (f *Fs) removeUploadedEntry(remote string) {
	f.uploadedMu.Lock()
	defer f.uploadedMu.Unlock()
	f.removeUploadedEntryLocked(remote)
}

// removeUploadedEntryLocked is removeUploadedEntry with f.uploadedMu
// held, returning the removed object or nil
func (f *Fs) removeUploadedEntryLocked(remote string) *Object {
	parent, entry := f.uploaded.Find(remote)
	o, ok := entry.(*Object)
	if !ok {
		return nil
	}
	siblings := f.uploaded[parent]
	for i, e := range siblings {
		if e == entry {
			f.uploaded[parent] = append(siblings[:i], siblings[i+1:]...)
			return o
		}
	}
	return nil
}

// deleteMedium moves the active medium id to the trash, or with permanent
// deletes it for good.
//
// A permanent=true delete of an active medium doesn't purge it; only a
// second one, once it's in the trash, does - see deletePermanentDelay.
func (f *Fs) deleteMedium(ctx context.Context, id string, permanent bool) error {
	if err := f.doDeleteMedium(ctx, id); err != nil {
		return err
	}
	if !permanent {
		return nil
	}
	select {
	case <-time.After(deletePermanentDelay):
	case <-ctx.Done():
		return ctx.Err()
	}
	return f.doDeleteMedium(ctx, id, "permanent", "true")
}

// doDeleteMedium issues one DELETE /media call for id, with optional extra
// query parameters (name/value pairs) - see deleteMedium.
func (f *Fs) doDeleteMedium(ctx context.Context, id string, extra ...string) error {
	params := url.Values{"ids": {id}}
	for i := 0; i+1 < len(extra); i += 2 {
		params.Set(extra[i], extra[i+1])
	}
	opts := rest.Opts{
		Method:     "DELETE",
		Path:       "/media",
		Parameters: params,
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
	f.invalidateMediaCache()
	f.invalidateTrashCache()
	return nil
}

// Move renames src to remote in place via PUT /media/{id}.
//
// Moving to a media/by-year, by-month or by-day directory with a
// different date changes captured_at to match. upload/ isn't supported.
func (f *Fs) Move(ctx context.Context, src fs.Object, remote string) (fs.Object, error) {
	srcObj, ok := src.(*Object)
	if !ok {
		return nil, fs.ErrorCantMove
	}
	// srcObj.fs may be a different instance of the same remote (with a
	// different root), so don't compare it to f - rclone has already
	// checked the config matches.
	if srcObj.itemCount > 1 {
		// Only the whole medium can be renamed, not one of its items.
		return nil, fs.ErrorCantMove
	}
	match, _, pattern := patterns.match(f.root, remote, true)
	if pattern == nil || !pattern.isFile || pattern.isUpload {
		return nil, fs.ErrorCantMove
	}
	srcLeaf := stripSuffixID(path.Base(srcObj.remote), srcObj.id)
	dstLeaf := stripSuffixID(match[len(match)-1], srcObj.id)

	// Only send a new name when it actually changes: filename and
	// content_title are set together, so resending an unchanged name would
	// overwrite a title set in GoPro's own app, and an unnamed medium's
	// "{id}.ext" would come back as a filename of just ".ext".
	var upd api.MediumUpdate
	if dstLeaf != srcLeaf {
		if srcObj.raw {
			return nil, fmt.Errorf("gopro: can't rename %q on its own - rename its photo instead", srcObj.remote)
		}
		if isUnnamedLeaf(dstLeaf) {
			return nil, fmt.Errorf("gopro: can't move to %q: no name before the extension", remote)
		}
		filename := f.opt.Enc.FromStandardName(dstLeaf)
		upd.Filename, upd.ContentTitle = &filename, &filename
	}
	capturedAt, ok, err := destCapturedAt(pattern, match, srcObj.modTime)
	if err != nil {
		return nil, fmt.Errorf("gopro: can't move to %q: %w", remote, err)
	}
	if ok {
		upd.CapturedAt = &capturedAt
	}
	if upd.Filename != nil || upd.CapturedAt != nil {
		if err := srcObj.fs.updateMedium(ctx, srcObj.id, upd); err != nil {
			return nil, err
		}
		// updateMedium only invalidates srcObj.fs, which is a separate
		// instance with its own cache when the roots differ.
		f.invalidateMediaCache()
	}

	dstObj := &Object{}
	dstObj.copyFrom(srcObj)
	dstObj.fs = f
	dstObj.remote = remote
	if upd.CapturedAt != nil {
		dstObj.modTime = *upd.CapturedAt
	}
	return dstObj, nil
}

// destCapturedAt derives the captured_at Move should set for a
// by-year/by-month/by-day destination, keeping whatever of modTime the
// destination doesn't pin. ok is false when there's nothing to change.
func destCapturedAt(pattern *dirPattern, match []string, modTime time.Time) (t time.Time, ok bool, err error) {
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
		return time.Time{}, false, nil
	}
	t = time.Date(year, time.Month(month), day,
		modTime.Hour(), modTime.Minute(), modTime.Second(), modTime.Nanosecond(),
		modTime.Location())
	// time.Date normalizes invalid dates (Feb 31 -> Mar 3) rather than
	// rejecting them.
	if t.Year() != year || int(t.Month()) != month || t.Day() != day {
		return time.Time{}, false, fmt.Errorf("gopro: %04d-%02d-%02d is not a valid date", year, month, day)
	}
	return t, !t.Equal(modTime), nil
}

// PublicLink creates a public share (a "collection" in GoPro's API)
// holding just this medium and returns its URL.
//
// expire and unlink are ignored: shares have no expiry, and there's no
// way to look up which shares contain a medium.
func (f *Fs) PublicLink(ctx context.Context, remote string, expire fs.Duration, unlink bool) (string, error) {
	o, err := f.NewObject(ctx, remote)
	if err != nil {
		return "", err
	}
	obj, ok := o.(*Object)
	if !ok {
		return "", fs.ErrorObjectNotFound
	}
	// GoPro shares whole media, so a link to one chapter, frame or RAW
	// file would share all of them.
	if obj.itemCount > 1 || obj.raw {
		return "", fmt.Errorf("gopro: can't share %q on its own - GoPro only shares it together with the other files of its item: use \"rclone backend link\" to share them all", remote)
	}
	_, leaf := path.Split(remote)
	return f.shareMedium(ctx, obj.id, stripSuffixID(leaf, obj.id))
}

// shareMedium creates a public share of the medium id and returns its URL,
// titled with --gopro-link-title or else name
func (f *Fs) shareMedium(ctx context.Context, id, name string) (string, error) {
	title := f.opt.LinkTitle
	if title == "" && !isUnnamedLeaf(name) {
		// Without a title GoPro's share page shows none, which reads
		// better than a bare extension.
		title = name
	}
	collectionID, err := f.createCollection(ctx, title, f.opt.LinkAllowDownload)
	if err != nil {
		return "", err
	}
	if err := f.addToCollection(ctx, collectionID, id); err != nil {
		return "", err
	}
	return "https://gopro.com/v/" + collectionID, nil
}

// linkCommand implements the "link" backend command
func (f *Fs) linkCommand(ctx context.Context, arg []string) (any, error) {
	if len(arg) == 0 {
		return nil, errors.New("name at least one medium to share")
	}
	var links []string
	for _, a := range arg {
		id := restoreArg(a)
		if fs.GetConfig(ctx).DryRun {
			fs.Logf(f, "Would share medium %s", id)
			continue
		}
		item, err := f.getMedium(ctx, id)
		if err != nil {
			return links, err
		}
		link, err := f.shareMedium(ctx, id, f.opt.Enc.ToStandardName(item.Filename))
		if err != nil {
			return links, err
		}
		links = append(links, link)
	}
	return links, nil
}

// MimeType of an Object if known, "" otherwise
func (o *Object) MimeType(ctx context.Context) string {
	return o.mimeType
}

// ID of an Object if known, "" otherwise
//
// Every file of a medium has its own ID: "{medium}" for a single file
// or photo, "{medium}/{n}" for item n of a chaptered video or photo
// series, with "/raw" added for a RAW file. The medium's own id is the
// first 24 characters.
func (o *Object) ID() string {
	id := o.id
	if o.itemCount > 1 {
		id += "/" + strconv.Itoa(o.itemNumber)
	}
	if o.raw {
		id += "/raw"
	}
	return id
}

// ------------------------------------------------------------
// Upload protocol
//
// Uploading takes five steps: create a medium, create a "Source"
// derivative for it, request pre-signed URLs for each chunk, PUT the
// chunks, then mark the derivative and the medium available. Ported from
// github.com/dustin/gopro-plus (GoPro.Plus.Upload).
// ------------------------------------------------------------

// mediumTypeForFilename guesses the GoPro "type" for a filename from its
// extension
func mediumTypeForFilename(name string) string {
	switch strings.ToUpper(strings.TrimPrefix(path.Ext(name), ".")) {
	case "JPG", "JPEG", "GPR", "DNG", "PNG", "HEIC":
		return "Photo"
	default:
		return "Video"
	}
}

// gpChunkWriter implements fs.ChunkWriter for GoPro's upload protocol
type gpChunkWriter struct {
	f            *Fs
	remote       string
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
	replacedID   string                    // medium Close replaced at remote, if any
}

// rollbackOrphanedMedium deletes mediumID when an upload setup step after
// createMedium fails, before there's a gpChunkWriter to Abort. It returns
// setupErr, only logging a failure to delete.
func (f *Fs) rollbackOrphanedMedium(ctx context.Context, mediumID string, setupErr error) error {
	if delErr := f.deleteMedium(ctx, mediumID, true); delErr != nil {
		fs.Logf(f, "gopro: couldn't roll back orphaned medium %q after upload setup failed: %v", mediumID, delErr)
	}
	return setupErr
}

// OpenChunkWriter returns the chunk size and a ChunkWriter for uploading
// remote with the contents of src.
//
// Every upload is chunked, and the protocol needs every chunk's URL up
// front, so the first four steps of the upload protocol happen here.
func (f *Fs) OpenChunkWriter(ctx context.Context, remote string, src fs.ObjectInfo, options ...fs.OpenOption) (info fs.ChunkWriterInfo, writer fs.ChunkWriter, err error) {
	match, _, pattern := patterns.match(f.root, remote, true)
	if pattern == nil || !pattern.isFile || !pattern.canUpload {
		return info, nil, errCantUpload
	}
	size := src.Size()
	if size < 0 {
		return info, nil, errors.New("gopro: can't upload a file of unknown size - the upload protocol needs it up front")
	}
	// Subdirectories of upload/ only exist in this Fs's upload tree -
	// GoPro just gets the leaf.
	filename := f.opt.Enc.FromStandardName(path.Base(match[1]))
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
		return info, nil, f.rollbackOrphanedMedium(ctx, mediumID, err)
	}
	uploadID, err := f.createUpload(ctx, derivativeID)
	if err != nil {
		return info, nil, f.rollbackOrphanedMedium(ctx, mediumID, err)
	}
	parts, err := f.getUploadParts(ctx, derivativeID, uploadID, size, chunkSize, nParts)
	if err != nil {
		return info, nil, f.rollbackOrphanedMedium(ctx, mediumID, err)
	}
	sort.Slice(parts, func(i, j int) bool { return parts[i].Part < parts[j].Part })

	info = fs.ChunkWriterInfo{
		ChunkSize:   chunkSize,
		Concurrency: f.opt.UploadConcurrency,
	}
	return info, &gpChunkWriter{
		f:            f,
		remote:       remote,
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
// The URL is pre-signed, and S3 rejects a request that also carries an
// Authorization header, so this uses the unauthenticated client.
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
// The new medium isn't listed by /media/search until GoPro has processed
// it, so a synthetic one is registered under upload/ instead, replacing
// (and deleting) one already there. This happens here rather than in
// Update because multi-thread copies call OpenChunkWriter and Close
// directly, then NewObject.
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
	o := &Object{fs: w.f, remote: w.remote}
	o.setMetaData(w.medium, 1)
	w.f.uploadedMu.Lock()
	_, entry := w.f.uploaded.Find(w.remote)
	w.f.uploadedMu.Unlock()
	if replaced, ok := entry.(*Object); ok && replaced.id != w.mediumID {
		w.replacedID = replaced.id
		if err := w.f.replaceMedium(ctx, replaced.id, w.mediumID); err != nil {
			return err
		}
	}
	w.f.uploadedMu.Lock()
	w.f.removeUploadedEntryLocked(w.remote)
	w.f.uploaded.AddEntry(o)
	w.f.uploadedMu.Unlock()
	w.f.invalidateMediaCache()
	return nil
}

// Abort deletes the medium created for this upload
func (w *gpChunkWriter) Abort(ctx context.Context) error {
	return w.f.deleteMedium(ctx, w.mediumID, true)
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
