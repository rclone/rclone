// Package doi provides a filesystem interface for digital objects identified by DOIs.
//
// See: https://www.doi.org/the-identifier/what-is-a-doi/
package doi

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/rclone/rclone/backend/doi/api"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config/configmap"
	"github.com/rclone/rclone/fs/config/configstruct"
	"github.com/rclone/rclone/fs/fserrors"
	"github.com/rclone/rclone/fs/fshttp"
	"github.com/rclone/rclone/fs/hash"
	"github.com/rclone/rclone/lib/cache"
	"github.com/rclone/rclone/lib/pacer"
	"github.com/rclone/rclone/lib/rest"
)

const (
	// the URL of the DOI resolver
	//
	// Reference: https://www.doi.org/the-identifier/resources/factsheets/doi-resolution-documentation
	doiResolverAPIURL = "https://doi.org/api"
	minSleep          = 10 * time.Millisecond
	maxSleep          = 2 * time.Second
	decayConstant     = 2 // bigger for slower decay, exponential
)

var (
	errorReadOnly = errors.New("doi remotes are read only")
	timeUnset     = time.Unix(0, 0)
)

func init() {
	fsi := &fs.RegInfo{
		Name:        "doi",
		Description: "DOI datasets",
		NewFs:       NewFs,
		CommandHelp: commandHelp,
		Options: []fs.Option{{
			Name: "doi",
			Help: `The DOI or the doi.org URL.

For a Dataverse dataset you can instead set host + dataset_pid (see
those options) to address it directly, without resolving a DOI through
doi.org. Direct mode also reaches drafts and restricted datasets that
have no public DOI.`,
			Required: false,
		}, {
			Name: fs.ConfigProvider,
			Help: `DOI provider.

The DOI provider can be set when rclone does not automatically recognize a supported DOI provider.`,
			Examples: []fs.OptionExample{
				{
					Value: "auto",
					Help:  "Auto-detect provider",
				},
				{
					Value: string(Zenodo),
					Help:  "Zenodo",
				}, {
					Value: string(Dataverse),
					Help:  "Dataverse",
				}, {
					Value: string(Invenio),
					Help:  "Invenio",
				}},
			Required: false,
			Advanced: true,
		}, {
			Name: "doi_resolver_api_url",
			Help: `The URL of the DOI resolver API to use.

The DOI resolver can be set for testing or for cases when the canonical DOI resolver API cannot be used.

Defaults to "https://doi.org/api".`,
			Required: false,
			Advanced: true,
		}, {
			Name: "host",
			Help: `Base URL of a Dataverse installation, e.g. https://demo.dataverse.org.

Set host together with dataset_pid to address a Dataverse dataset
directly, skipping doi.org resolution. Leave empty to resolve a DOI
instead.`,
			Advanced: true,
		}, {
			Name: "dataset_pid",
			Help: `Persistent ID of the Dataverse dataset to mount.

This is passed straight to Dataverse, so any persistent ID type the
installation supports works here — a DOI (doi:10.5072/FK2/ABCD), a
Handle (hdl:1902.1/12345), a PermaLink (perma:...), and so on — not only
DOIs. Used together with host for direct addressing (see the host option).`,
			Advanced: true,
		}, {
			Name: "token",
			Help: `Dataverse API token, sent as X-Dataverse-Key on API and access requests.

Leave empty for guest access — public datasets and published files
remain readable without a token. A token is only needed for restricted
files, draft versions, or datasets limited to authenticated readers.
The token is stripped on the redirect to S3, so it never leaves the
Dataverse host.`,
			Advanced:  true,
			Sensitive: true,
		}, {
			Name: "version",
			Help: `Dataverse dataset version to mount, e.g. :latest, :draft or 1.0.

Defaults to :latest.`,
			Default:  api.LatestVersion,
			Advanced: true,
		}, {
			Name: "ingest_format",
			Help: `Which form of a Dataverse tabular-ingest file to surface.

When Dataverse ingests a tabular upload (CSV, SPSS, Stata, …) it stores
both the original bytes and a normalised archival form (typically
tab-separated). This option picks which one the mount exposes:

- original (default): fetch with ?format=original, return the original
  upload bytes and expose their MD5. On the whole-version listing the
  file also surfaces under its original name (.csv, .sav, …); on the
  lazy /tree listing it keeps its stored name but still serves the
  original bytes, size and MD5.
- archival: return the post-ingest bytes (typically .tab). The size is
  the archival (.tab) size, so rclone copy still verifies it; only the
  MD5 is hidden, because Dataverse stores the original upload's checksum,
  which does not match the archival bytes.

Files that were not ingested are unaffected.`,
			Default: string(IngestFormatOriginal),
			Examples: []fs.OptionExample{
				{Value: string(IngestFormatOriginal), Help: "Original upload bytes (and, on the whole-version listing, name)"},
				{Value: string(IngestFormatArchival), Help: "Archival (post-ingest) bytes and name"},
			},
			Advanced: true,
		}},
	}
	fs.Register(fsi)
}

// Provider defines the type of provider hosting the DOI
type Provider string

const (
	// Zenodo provider, see https://zenodo.org
	Zenodo Provider = "zenodo"
	// Dataverse provider, see https://dataverse.harvard.edu
	Dataverse Provider = "dataverse"
	// Invenio provider, see https://inveniordm.docs.cern.ch
	Invenio Provider = "invenio"
)

// Options defines the configuration for this backend
type Options struct {
	Doi               string `config:"doi"`                  // The DOI, a digital identifier of an object, usually a dataset
	Provider          string `config:"provider"`             // The DOI provider
	DoiResolverAPIURL string `config:"doi_resolver_api_url"` // The URL of the DOI resolver API to use.
	Host              string `config:"host"`                 // Dataverse installation base URL (direct mode)
	DatasetPID        string `config:"dataset_pid"`          // Dataverse dataset persistent ID (direct mode)
	Token             string `config:"token"`                // Dataverse API token (X-Dataverse-Key)
	Version           string `config:"version"`              // Dataverse dataset version (default :latest)
	IngestFormat      string `config:"ingest_format"`        // Dataverse tabular-ingest form: original | archival
}

// Fs stores the interface to the remote HTTP files
type Fs struct {
	name        string         // name of this remote
	root        string         // the path we are working on
	provider    Provider       // the DOI provider
	doiProvider doiProvider    // the interface used to interact with the DOI provider
	features    *fs.Features   // optional features
	opt         Options        // options for this backend
	ci          *fs.ConfigInfo // global config
	endpoint    *url.URL       // the main API endpoint for this remote
	endpointURL string         // endpoint as a string
	srv         *rest.Client   // the connection to the server
	pacer       *fs.Pacer      // pacer for API calls
	cache       *cache.Cache   // a cache for the remote metadata
	httpClient  *http.Client   // client for byte-stream reads: off the pacer, follows redirects, strips the token cross-host
	useTree     bool           // Dataverse: lazy /tree listing available (feature-detected)
}

// Object is a remote object that has been stat'd (so it exists, but is not necessarily open for reading)
type Object struct {
	fs          *Fs       // what this object is part of
	remote      string    // the remote path
	contentURL  string    // the URL where the contents of the file can be downloaded
	size        int64     // size of the object
	modTime     time.Time // modification time of the object
	contentType string    // content type of the object
	md5         string    // MD5 hash of the object content
	// Dataverse-only (empty for other providers), used to attribute a
	// 403: "public" | "restricted" | "embargoed" | "retentionExpired"
	// (from /tree, or derived from the whole-version restricted flag).
	accessStatus string
}

// doiProvider is the interface used to list objects in a DOI
type doiProvider interface {
	// ListEntries returns the full list of entries found at the remote, regardless of root
	ListEntries(ctx context.Context) (entries []*Object, err error)
}

// Parse the input string as a DOI
// Examples:
// 10.1000/182 -> 10.1000/182
// https://doi.org/10.1000/182 -> 10.1000/182
// doi:10.1000/182 -> 10.1000/182
func parseDoi(doi string) string {
	doiURL, err := url.Parse(doi)
	if err != nil {
		return doi
	}
	if doiURL.Scheme == "doi" {
		return strings.TrimLeft(strings.TrimPrefix(doi, "doi:"), "/")
	}
	if strings.HasSuffix(doiURL.Hostname(), "doi.org") {
		return strings.TrimLeft(doiURL.Path, "/")
	}
	return doi
}

// Resolve a DOI to a URL
// Reference: https://www.doi.org/the-identifier/resources/factsheets/doi-resolution-documentation
func resolveDoiURL(ctx context.Context, srv *rest.Client, pacer *fs.Pacer, opt *Options) (doiURL *url.URL, err error) {
	resolverURL := opt.DoiResolverAPIURL
	if resolverURL == "" {
		resolverURL = doiResolverAPIURL
	}

	var result api.DoiResolverResponse
	params := url.Values{}
	params.Add("index", "1")
	opts := rest.Opts{
		Method:     "GET",
		RootURL:    resolverURL,
		Path:       "/handles/" + opt.Doi,
		Parameters: params,
	}
	err = pacer.Call(func() (bool, error) {
		res, err := srv.CallJSON(ctx, &opts, nil, &result)
		return shouldRetry(ctx, res, err)
	})
	if err != nil {
		return nil, err
	}

	if result.ResponseCode != 1 {
		return nil, fmt.Errorf("could not resolve DOI (error code %d)", result.ResponseCode)
	}
	resolvedURLStr := ""
	for _, value := range result.Values {
		if value.Type == "URL" && value.Data.Format == "string" {
			valueStr, ok := value.Data.Value.(string)
			if !ok {
				return nil, fmt.Errorf("could not resolve DOI (incorrect response format)")
			}
			resolvedURLStr = valueStr
		}
	}
	resolvedURL, err := url.Parse(resolvedURLStr)
	if err != nil {
		return nil, err
	}
	return resolvedURL, nil
}

// Resolve the passed configuration into a provider and enpoint
func resolveEndpoint(ctx context.Context, srv *rest.Client, pacer *fs.Pacer, opt *Options) (provider Provider, endpoint *url.URL, err error) {
	// Direct mode: address a Dataverse dataset by host + dataset_pid,
	// skipping doi.org resolution.
	if opt.Host != "" && opt.DatasetPID != "" {
		return resolveDataverseDirectEndpoint(opt)
	}

	resolvedURL, err := resolveDoiURL(ctx, srv, pacer, opt)
	if err != nil {
		return "", nil, err
	}

	switch opt.Provider {
	case string(Dataverse):
		return resolveDataverseEndpoint(resolvedURL, opt.Version)
	case string(Invenio):
		return resolveInvenioEndpoint(ctx, srv, pacer, resolvedURL)
	case string(Zenodo):
		return resolveZenodoEndpoint(ctx, srv, pacer, resolvedURL, opt.Doi)
	}

	hostname := strings.ToLower(resolvedURL.Hostname())
	if hostname == "dataverse.harvard.edu" || activateDataverse(resolvedURL) {
		return resolveDataverseEndpoint(resolvedURL, opt.Version)
	}
	if hostname == "zenodo.org" || strings.HasSuffix(hostname, ".zenodo.org") {
		return resolveZenodoEndpoint(ctx, srv, pacer, resolvedURL, opt.Doi)
	}
	if activateInvenio(ctx, srv, pacer, resolvedURL) {
		return resolveInvenioEndpoint(ctx, srv, pacer, resolvedURL)
	}

	return "", nil, fmt.Errorf("provider '%s' is not supported", resolvedURL.Hostname())
}

// Make the http connection from the passed options
func (f *Fs) httpConnection(ctx context.Context, opt *Options) (isFile bool, err error) {
	provider, endpoint, err := resolveEndpoint(ctx, f.srv, f.pacer, opt)
	if err != nil {
		return false, err
	}

	// Update f with the new parameters
	f.srv.SetRoot(endpoint.ResolveReference(&url.URL{Path: "/"}).String())
	f.endpoint = endpoint
	f.endpointURL = endpoint.String()
	f.provider = provider
	f.opt.Provider = string(provider)

	f.useTree = false
	switch f.provider {
	case Dataverse:
		// Authenticate every Dataverse API/list/tree call. The byte-stream
		// reads attach the token separately (and strip it on the
		// cross-host redirect to S3).
		if opt.Token != "" {
			f.srv.SetHeader(api.AuthHeader, opt.Token)
		} else {
			// The connection can be rebuilt by the backend "set" command:
			// a cleared token must not leave the old header behind.
			f.srv.RemoveHeader(api.AuthHeader)
		}
		f.doiProvider = newDataverseProvider(f)
		// Feature-detect the lazy /tree listing in direct mode only, so
		// existing resolved-DOI remotes keep the whole-version listing
		// (and its original-name substitution for ingested files)
		// unchanged. On a 404 or any error useTree stays false and
		// listing uses the whole-version path.
		if opt.DatasetPID != "" {
			f.useTree = treeSupported(ctx, f, opt.DatasetPID, opt.Version)
		}
	case Invenio, Zenodo:
		f.doiProvider = newInvenioProvider(f)
	default:
		return false, fmt.Errorf("provider type '%s' not supported", f.provider)
	}

	return f.rootIsFile(ctx)
}

// rootIsFile reports whether f.root points at a file (rather than a
// directory or the dataset root). On the non-tree path it also serves as
// the NewFs-time connection/auth check; tree mode validates the
// connection during /tree feature detection.
func (f *Fs) rootIsFile(ctx context.Context) (bool, error) {
	if f.useTree {
		if f.root == "" {
			return false, nil
		}
		// Resolve f.root via its parent's /tree level: a hit is a file, a
		// miss (ErrorObjectNotFound) is a directory.
		_, err := f.NewObject(ctx, "")
		if errors.Is(err, fs.ErrorObjectNotFound) {
			return false, nil
		}
		if err != nil {
			return false, err
		}
		return true, nil
	}
	entries, err := f.doiProvider.ListEntries(ctx)
	if err != nil {
		return false, err
	}
	for _, entry := range entries {
		if entry.remote == f.root {
			return true, nil
		}
	}
	return false, nil
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

// shouldRetry returns a boolean as to whether this res and err
// deserve to be retried. It returns the err as a convenience.
func shouldRetry(ctx context.Context, res *http.Response, err error) (bool, error) {
	if fserrors.ContextError(ctx, &err) {
		return false, err
	}
	return fserrors.ShouldRetry(err) || fserrors.ShouldRetryHTTP(res, retryErrorCodes), err
}

// checkOptions validates and normalises the config options in place. It
// runs at NewFs time and again when the backend "set" command rebuilds
// the connection with merged options.
func checkOptions(opt *Options) error {
	// Direct mode: host + dataset_pid address a Dataverse dataset
	// directly, skipping DOI resolution. Otherwise a DOI is required.
	switch {
	case opt.Host != "" && opt.DatasetPID == "",
		opt.Host == "" && opt.DatasetPID != "":
		return errors.New("host and dataset_pid must both be set for Dataverse direct mode")
	}
	directMode := opt.Host != "" && opt.DatasetPID != ""
	if directMode && opt.Doi != "" {
		return errors.New("set either doi or host+dataset_pid, not both")
	}
	if !directMode && opt.Doi == "" {
		return errors.New("doi is required (or set host+dataset_pid for a Dataverse dataset)")
	}
	if directMode {
		hostURL, err := url.Parse(strings.TrimRight(opt.Host, "/"))
		if err != nil || hostURL.Host == "" || (hostURL.Scheme != "http" && hostURL.Scheme != "https") {
			return fmt.Errorf("host must be an http(s) URL like https://demo.dataverse.org, got %q", opt.Host)
		}
	}
	if opt.Doi != "" {
		opt.Doi = parseDoi(opt.Doi)
	}
	if opt.Version == "" {
		opt.Version = api.LatestVersion
	}
	switch IngestFormat(opt.IngestFormat) {
	case "":
		opt.IngestFormat = string(IngestFormatOriginal)
	case IngestFormatOriginal, IngestFormatArchival:
		// ok
	default:
		return fmt.Errorf("invalid ingest_format %q (want %q or %q)", opt.IngestFormat, IngestFormatOriginal, IngestFormatArchival)
	}
	return nil
}

// NewFs creates a new Fs object from the name and root. It connects to
// the host specified in the config file.
func NewFs(ctx context.Context, name, root string, m configmap.Mapper) (fs.Fs, error) {
	root = strings.Trim(root, "/")

	// Parse config into Options struct
	opt := new(Options)
	err := configstruct.Set(m, opt)
	if err != nil {
		return nil, err
	}
	if err := checkOptions(opt); err != nil {
		return nil, err
	}

	client := fshttp.NewClient(ctx)
	ci := fs.GetConfig(ctx)
	f := &Fs{
		name:       name,
		root:       root,
		opt:        *opt,
		ci:         ci,
		srv:        rest.NewClient(client),
		pacer:      fs.NewPacer(ctx, pacer.NewDefault(pacer.MinSleep(minSleep), pacer.MaxSleep(maxSleep), pacer.DecayConstant(decayConstant))),
		cache:      cache.New(),
		httpClient: readClient(client),
	}
	f.features = (&fs.Features{
		CanHaveEmptyDirectories: true,
	}).Fill(ctx, f)

	isFile, err := f.httpConnection(ctx, opt)
	if err != nil {
		return nil, err
	}

	if isFile {
		// return an error with an fs which points to the parent
		newRoot := path.Dir(f.root)
		if newRoot == "." {
			newRoot = ""
		}
		f.root = newRoot
		return f, fs.ErrorIsFile
	}

	return f, nil
}

// Name returns the configured name of the file system
func (f *Fs) Name() string {
	return f.name
}

// Root returns the root for the filesystem
func (f *Fs) Root() string {
	return f.root
}

// String returns a description of the filesystem
func (f *Fs) String() string {
	if f.opt.Doi != "" {
		return fmt.Sprintf("DOI %s", f.opt.Doi)
	}
	return fmt.Sprintf("Dataverse dataset %s on %s", f.opt.DatasetPID, f.opt.Host)
}

// Features returns the optional features of this Fs
func (f *Fs) Features() *fs.Features {
	return f.features
}

// Precision is the remote http file system's modtime precision, which we have no way of knowing. We estimate at 1s
func (f *Fs) Precision() time.Duration {
	return time.Second
}

// Hashes returns hash.HashNone to indicate remote hashing is unavailable
func (f *Fs) Hashes() hash.Set {
	return hash.Set(hash.MD5)
	// return hash.Set(hash.None)
}

// Mkdir makes the root directory of the Fs object
func (f *Fs) Mkdir(ctx context.Context, dir string) error {
	return errorReadOnly
}

// Remove a remote http file object
func (o *Object) Remove(ctx context.Context) error {
	return errorReadOnly
}

// Rmdir removes the root directory of the Fs object
func (f *Fs) Rmdir(ctx context.Context, dir string) error {
	return errorReadOnly
}

// NewObject creates a new remote http file object
func (f *Fs) NewObject(ctx context.Context, remote string) (fs.Object, error) {
	if f.useTree {
		return f.newObjectTree(ctx, remote)
	}
	entries, err := f.doiProvider.ListEntries(ctx)
	if err != nil {
		return nil, err
	}

	remoteFullPath := remote
	if f.root != "" {
		remoteFullPath = path.Join(f.root, remote)
	}

	for _, entry := range entries {
		if entry.Remote() == remoteFullPath {
			// Provider entries carry the dataset-absolute path; the
			// returned object's Remote() must be relative to the Fs root.
			newEntry := *entry
			newEntry.remote = remote
			return &newEntry, nil
		}
	}

	return nil, fs.ErrorObjectNotFound
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
//
// List is a thin collector over ListP.
func (f *Fs) List(ctx context.Context, dir string) (entries fs.DirEntries, err error) {
	err = f.ListP(ctx, dir, func(e fs.DirEntries) error {
		entries = append(entries, e...)
		return nil
	})
	return entries, err
}

// ListP lists the immediate children of dir, calling callback for each
// tranche. On the Dataverse /tree path each tranche is one endpoint page
// (lazy: a large folder streams page-by-page); otherwise the level is
// emitted in a single tranche.
func (f *Fs) ListP(ctx context.Context, dir string, callback fs.ListRCallback) error {
	if f.useTree {
		return f.listTreeP(ctx, dir, callback)
	}
	entries, err := f.listFlat(ctx, dir)
	if err != nil {
		return err
	}
	if len(entries) == 0 {
		return nil
	}
	return callback(entries)
}

// listFlat builds one directory level from the whole-version file list.
func (f *Fs) listFlat(ctx context.Context, dir string) (entries fs.DirEntries, err error) {
	fileEntries, err := f.doiProvider.ListEntries(ctx)
	if err != nil {
		return nil, fmt.Errorf("error listing %q: %w", dir, err)
	}

	fullDir := path.Join(f.root, dir)
	if fullDir != "" {
		fullDir += "/"
	}

	dirPaths := map[string]bool{}
	for _, entry := range fileEntries {
		// First, filter out files not in `fullDir`
		if !strings.HasPrefix(entry.remote, fullDir) {
			continue
		}
		// Then, find entries in subfolers
		remotePath := entry.remote
		if fullDir != "" {
			remotePath = strings.TrimLeft(strings.TrimPrefix(remotePath, fullDir), "/")
		}
		parts := strings.SplitN(remotePath, "/", 2)
		if len(parts) == 1 {
			newEntry := *entry
			newEntry.remote = path.Join(dir, remotePath)
			entries = append(entries, &newEntry)
		} else {
			dirPaths[path.Join(dir, parts[0])] = true
		}
	}

	for dirPath := range dirPaths {
		entry := fs.NewDir(dirPath, time.Time{})
		entries = append(entries, entry)
	}

	return entries, nil
}

// Put in to the remote path with the modTime given of the given size
//
// May create the object even if it returns an error - if so
// will return the object and the error, otherwise will return
// nil and the error
func (f *Fs) Put(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error) {
	return nil, errorReadOnly
}

// PutStream uploads to the remote path with the modTime given of indeterminate size
func (f *Fs) PutStream(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error) {
	return nil, errorReadOnly
}

// Fs is the filesystem this remote http file object is located within
func (o *Object) Fs() fs.Info {
	return o.fs
}

// String returns the URL to the remote HTTP file
func (o *Object) String() string {
	if o == nil {
		return "<nil>"
	}
	return o.remote
}

// Remote the name of the remote HTTP file, relative to the fs root
func (o *Object) Remote() string {
	return o.remote
}

// Hash returns "" since HTTP (in Go or OpenSSH) doesn't support remote calculation of hashes
func (o *Object) Hash(ctx context.Context, t hash.Type) (string, error) {
	if t != hash.MD5 {
		return "", hash.ErrUnsupported
	}
	return o.md5, nil
}

// Size returns the size in bytes of the remote http file
func (o *Object) Size() int64 {
	return o.size
}

// ModTime returns the modification time of the remote http file
func (o *Object) ModTime(ctx context.Context) time.Time {
	return o.modTime
}

// SetModTime sets the modification and access time to the specified time
//
// it also updates the info field
func (o *Object) SetModTime(ctx context.Context, modTime time.Time) error {
	return errorReadOnly
}

// Storable returns whether the remote http file is a regular file (not a directory, symbolic link, block device, character device, named pipe, etc.)
func (o *Object) Storable() bool {
	return true
}

// maxStreamRefreshes bounds the number of times Open's stream wrapper
// re-fetches and resumes the byte stream after a mid-read failure. Five
// gives a comfortable margin for a multi-hour transfer through several
// presigned-URL TTLs without an unbounded retry loop on a dead backend.
const maxStreamRefreshes = 5

// readClient returns an HTTP client for byte-stream reads. Reads bypass
// the pacer: its 10ms floor would cap file opens at ~100/s regardless of
// --transfers (~100s for a 10k-file copy). Reliability instead comes from
// the resuming reader below plus fshttp's transport-level retries.
//
// In Dataverse S3-direct mode the access endpoint answers with a 302 to a
// presigned URL on another host; this client follows that redirect but
// strips the API token on the cross-host hop so it never reaches S3/AWS
// (Go auto-strips only Authorization/Cookie/WWW-Authenticate, not the
// custom X-Dataverse-Key). The transport is shared with the API client
// for connection pooling.
func readClient(base *http.Client) *http.Client {
	return &http.Client{
		Transport: base.Transport,
		Jar:       base.Jar,
		Timeout:   base.Timeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 10 {
				return fmt.Errorf("stopped after %d redirects", len(via))
			}
			if len(via) > 0 && req.URL.Host != via[0].URL.Host {
				req.Header.Del(api.AuthHeader)
			}
			return nil
		},
	}
}

// Open a remote file object for reading. Seek/Range is supported.
//
// A single redirect-following GET resolves both Dataverse serving modes
// in one shot: proxy mode returns the bytes directly, S3-direct mode
// returns a 302 followed transparently to the presigned URL (no probe).
// The body is wrapped in a reader that, on a mid-transfer failure,
// re-issues the GET from the byte offset already delivered — covering
// presigned URLs that expire mid-stream. Reads run off the pacer (see
// readClient).
func (o *Object) Open(ctx context.Context, options ...fs.OpenOption) (in io.ReadCloser, err error) {
	fs.FixRangeOption(options, o.size)

	// Pull the byte range out of the typed options so it can be
	// re-issued on resume; forward all other headers unchanged.
	// FixRangeOption has already normalised SeekOption and suffix
	// ranges into absolute RangeOptions.
	headers := make(http.Header)
	startOffset, endOffset := int64(0), int64(-1)
	for _, option := range options {
		switch x := option.(type) {
		case *fs.RangeOption:
			startOffset, endOffset = x.Start, x.End
		case *fs.SeekOption:
			startOffset = x.Offset
		default:
			if k, v := option.Header(); k != "" {
				headers.Set(k, v)
			}
		}
	}

	body, err := o.fetchRange(ctx, startOffset, endOffset, headers)
	if err != nil {
		return nil, err
	}
	return &resumingReader{
		ctx:         ctx,
		obj:         o,
		forwarded:   headers,
		startOffset: startOffset,
		endOffset:   endOffset,
		body:        body,
		maxRefresh:  maxStreamRefreshes,
	}, nil
}

// doGet issues one GET to rawURL for bytes [start,end] (end == -1 means
// to EOF), forwarding headers. The token is attached only when sendToken
// is true and one is configured (an empty X-Dataverse-Key makes
// Dataverse reject the request instead of treating it as guest access).
func (o *Object) doGet(ctx context.Context, rawURL string, start, end int64, headers http.Header, sendToken bool) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	if sendToken && o.fs.provider == Dataverse && o.fs.opt.Token != "" {
		req.Header.Set(api.AuthHeader, o.fs.opt.Token)
	}
	for k, vs := range headers {
		for _, v := range vs {
			req.Header.Add(k, v)
		}
	}
	if start != 0 || end >= 0 {
		if end >= 0 {
			req.Header.Set("Range", fmt.Sprintf("bytes=%d-%d", start, end))
		} else {
			req.Header.Set("Range", fmt.Sprintf("bytes=%d-", start))
		}
	}
	return o.fs.httpClient.Do(req)
}

// fetchRange GETs bytes [start,end] of the object. There is no separate
// access probe: the redirect-following client resolves proxy vs
// S3-direct in one fetch. As a fallback for a non-compliant response (a
// 2xx carrying a Location the client did not act on) it follows that
// Location once, dropping the token if it points to another host. A
// Dataverse 401/403 is reported as an attributed access error.
func (o *Object) fetchRange(ctx context.Context, start, end int64, forwardedHeaders http.Header) (io.ReadCloser, error) {
	res, err := o.doGet(ctx, o.contentURL, start, end, forwardedHeaders, true)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", o.remote, err)
	}
	if res.Header.Get("Location") != "" {
		if loc, lerr := res.Location(); lerr == nil {
			_ = res.Body.Close()
			keepToken := false
			if cu, perr := url.Parse(o.contentURL); perr == nil {
				keepToken = loc.Host == cu.Host
			}
			res, err = o.doGet(ctx, loc.String(), start, end, forwardedHeaders, keepToken)
			if err != nil {
				return nil, fmt.Errorf("read %s: %w", o.remote, err)
			}
		}
	}
	if o.fs.provider == Dataverse && (res.StatusCode == http.StatusUnauthorized || res.StatusCode == http.StatusForbidden) {
		_ = res.Body.Close()
		return nil, o.accessDeniedError(res.StatusCode)
	}
	if res.StatusCode != http.StatusOK && res.StatusCode != http.StatusPartialContent {
		msg, _ := io.ReadAll(io.LimitReader(res.Body, 1024))
		_ = res.Body.Close()
		readErr := fmt.Errorf("read %s: %s: %s", o.remote, res.Status, strings.TrimSpace(string(msg)))
		// Byte reads bypass the pacer, so hand a transient server status to
		// rclone's transfer-level retry (which honours Retry-After).
		if fserrors.ShouldRetryHTTP(res, retryErrorCodes) {
			if secs, e := strconv.Atoi(res.Header.Get("Retry-After")); e == nil && secs > 0 {
				return nil, pacer.RetryAfterError(readErr, time.Duration(secs)*time.Second)
			}
			return nil, fserrors.RetryError(readErr)
		}
		return nil, readErr
	}
	if start > 0 && res.StatusCode == http.StatusOK {
		// The server ignored the Range request. Delivering the body
		// would splice bytes from offset 0 into the middle of a resumed
		// stream, so fail instead and let the transfer-level retry
		// restart cleanly.
		_ = res.Body.Close()
		return nil, fmt.Errorf("read %s: server ignored Range request at offset %d", o.remote, start)
	}
	return res.Body, nil
}

// accessDeniedError turns a Dataverse 401/403 into a message that tells
// the user why the file couldn't be fetched, so rclone's per-file
// skip-and-continue surfaces an actionable reason. The access marker
// comes from the /tree listing or the whole-version restricted flag.
func (o *Object) accessDeniedError(status int) error {
	switch o.accessStatus {
	case "embargoed":
		return fmt.Errorf("read %s: HTTP %d: file is under embargo — it is not yet available for download", o.remote, status)
	case "retentionExpired":
		return fmt.Errorf("read %s: HTTP %d: file's retention period has expired — it is no longer available for download", o.remote, status)
	case "", "public":
		return fmt.Errorf("read %s: HTTP %d: access denied — the file is restricted or embargoed, or the API token is missing/invalid", o.remote, status)
	default:
		// "restricted" or any other non-public marker
		return fmt.Errorf("read %s: HTTP %d: file is restricted — your API token does not grant access to it", o.remote, status)
	}
}

// resumingReader wraps the response body so a mid-stream failure
// (presigned URL expired, S3 dropped the connection, transient blip)
// transparently re-issues the read from the byte offset already
// delivered. bytesRead survives across body swaps; maxRefresh bounds the
// resumes so a dead backend can't trap rclone in a loop.
type resumingReader struct {
	ctx         context.Context
	obj         *Object
	forwarded   http.Header
	startOffset int64
	endOffset   int64

	body         io.ReadCloser
	bytesRead    int64
	refreshCount int
	maxRefresh   int
}

func (r *resumingReader) Read(p []byte) (int, error) {
	n, err := r.body.Read(p)
	r.bytesRead += int64(n)

	if err == nil || err == io.EOF {
		return n, err
	}
	if r.refreshCount >= r.maxRefresh {
		return n, err
	}
	if cerr := r.ctx.Err(); cerr != nil {
		return n, err
	}

	// Mid-stream failure: drop the old body, re-fetch from where we left
	// off, resume.
	_ = r.body.Close()
	newBody, fetchErr := r.obj.fetchRange(r.ctx, r.startOffset+r.bytesRead, r.endOffset, r.forwarded)
	if fetchErr != nil {
		return n, err
	}
	r.body = newBody
	r.refreshCount++

	if n > 0 {
		return n, nil
	}
	return r.Read(p)
}

func (r *resumingReader) Close() error {
	return r.body.Close()
}

// Update in to the object with the modTime given of the given size
func (o *Object) Update(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) error {
	return errorReadOnly
}

// MimeType of an Object if known, "" otherwise
func (o *Object) MimeType(ctx context.Context) string {
	return o.contentType
}

var commandHelp = []fs.CommandHelp{{
	Name:  "metadata",
	Short: "Show metadata about the DOI.",
	Long: `This command returns a JSON object with some information about the DOI.

Usage example:

` + "```console" + `
rclone backend metadata doi:
` + "```" + `

It returns a JSON object representing metadata about the DOI.`,
}, {
	Name:  "set",
	Short: "Set command for updating the config parameters.",
	Long: `This set command can be used to update the config parameters
for a running doi backend.

Usage examples:

` + "```console" + `
rclone backend set doi: [-o opt_name=opt_value] [-o opt_name2=opt_value2]
rclone rc backend/command command=set fs=doi: [-o opt_name=opt_value] [-o opt_name2=opt_value2]
rclone rc backend/command command=set fs=doi: -o doi=NEW_DOI
` + "```" + `

The option keys are named as they are in the config file.

This rebuilds the connection to the doi backend when it is called with
the new parameters. Only new parameters need be passed as the values
will default to those currently in use.

It doesn't return anything.`,
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
func (f *Fs) Command(ctx context.Context, name string, arg []string, opt map[string]string) (out any, err error) {
	switch name {
	case "metadata":
		return f.ShowMetadata(ctx)
	case "set":
		newOpt := f.opt
		err := configstruct.Set(configmap.Simple(opt), &newOpt)
		if err != nil {
			return nil, fmt.Errorf("reading config: %w", err)
		}
		if err := checkOptions(&newOpt); err != nil {
			return nil, err
		}
		// Adopt the new options and drop the metadata caches before
		// reconnecting: the connection check reads f.opt, and cached
		// listings from the old dataset/version/format must not survive.
		f.opt = newOpt
		f.cache.Clear()
		_, err = f.httpConnection(ctx, &newOpt)
		if err != nil {
			return nil, fmt.Errorf("updating session: %w", err)
		}
		keys := []string{}
		for k := range opt {
			keys = append(keys, k)
		}
		fs.Logf(f, "Updated config values: %s", strings.Join(keys, ", "))
		return nil, nil
	default:
		return nil, fs.ErrorCommandNotFound
	}
}

// ShowMetadata returns some metadata about the corresponding DOI
func (f *Fs) ShowMetadata(ctx context.Context) (metadata any, err error) {
	doiURL, err := url.Parse("https://doi.org/" + f.opt.Doi)
	if err != nil {
		return nil, err
	}

	info := map[string]any{}
	info["DOI"] = f.opt.Doi
	info["URL"] = doiURL.String()
	info["metadataURL"] = f.endpointURL
	info["provider"] = f.provider
	return info, nil
}

// Check the interfaces are satisfied
var (
	_ fs.Fs          = (*Fs)(nil)
	_ fs.PutStreamer = (*Fs)(nil)
	_ fs.Commander   = (*Fs)(nil)
	_ fs.ListPer     = (*Fs)(nil)
	_ fs.Object      = (*Object)(nil)
	_ fs.MimeTyper   = (*Object)(nil)
)
