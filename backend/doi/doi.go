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
			Name:     "doi",
			Help:     "The DOI or the doi.org URL.",
			Required: true,
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

The DOI resolver can be set for testing or for cases when the the canonical DOI resolver API cannot be used.

Defaults to "https://doi.org/api".`,
			Required: false,
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
	resolvedURL, err := resolveDoiURL(ctx, srv, pacer, opt)
	if err != nil {
		return "", nil, err
	}

	switch opt.Provider {
	case string(Dataverse):
		return resolveDataverseEndpoint(resolvedURL)
	case string(Invenio):
		return resolveInvenioEndpoint(ctx, srv, pacer, resolvedURL)
	case string(Zenodo):
		return resolveZenodoEndpoint(ctx, srv, pacer, resolvedURL, opt.Doi)
	}

	hostname := strings.ToLower(resolvedURL.Hostname())
	if hostname == "dataverse.harvard.edu" || activateDataverse(resolvedURL) {
		return resolveDataverseEndpoint(resolvedURL)
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

	switch f.provider {
	case Dataverse:
		f.doiProvider = newDataverseProvider(f)
	case Invenio, Zenodo:
		f.doiProvider = newInvenioProvider(f)
	default:
		return false, fmt.Errorf("provider type '%s' not supported", f.provider)
	}

	// Determine if the root is a file
	entries, err := f.doiProvider.ListEntries(ctx)
	if err != nil {
		return false, err
	}
	for _, entry := range entries {
		if entry.remote == f.root {
			isFile = true
			break
		}
	}
	return isFile, nil
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
	opt.Doi = parseDoi(opt.Doi)

	client := fshttp.NewClient(ctx)
	ci := fs.GetConfig(ctx)
	f := &Fs{
		name:  name,
		root:  root,
		opt:   *opt,
		ci:    ci,
		srv:   rest.NewClient(client),
		pacer: fs.NewPacer(ctx, pacer.NewDefault(pacer.MinSleep(minSleep), pacer.MaxSleep(maxSleep), pacer.DecayConstant(decayConstant))),
		cache: cache.New(),
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

// String returns the URL for the filesystem
func (f *Fs) String() string {
	return fmt.Sprintf("DOI %s", f.opt.Doi)
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
			return entry, nil
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
func (f *Fs) List(ctx context.Context, dir string) (entries fs.DirEntries, err error) {
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

// Open a remote http file object for reading. Seek is supported
func (o *Object) Open(ctx context.Context, options ...fs.OpenOption) (in io.ReadCloser, err error) {
	fs.FixRangeOption(options, o.size)
	opts := rest.Opts{
		Method:  "GET",
		RootURL: o.contentURL,
		Options: options,
	}
	var res *http.Response
	err = o.fs.pacer.Call(func() (bool, error) {
		res, err = o.fs.srv.Call(ctx, &opts)
		return shouldRetry(ctx, res, err)
	})
	if err != nil {
		return nil, fmt.Errorf("Open failed: %w", err)
	}

	// Handle non-compliant redirects
	if res.Header.Get("Location") != "" {
		newURL, err := res.Location()
		if err == nil {
			opts.RootURL = newURL.String()
			err = o.fs.pacer.Call(func() (bool, error) {
				res, err = o.fs.srv.Call(ctx, &opts)
				return shouldRetry(ctx, res, err)
			})
			if err != nil {
				return nil, fmt.Errorf("Open failed: %w", err)
			}
		}
	}

	return res.Body, nil
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
		_, err = f.httpConnection(ctx, &newOpt)
		if err != nil {
			return nil, fmt.Errorf("updating session: %w", err)
		}
		f.opt = newOpt
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
	_ fs.Object      = (*Object)(nil)
	_ fs.MimeTyper   = (*Object)(nil)
)
