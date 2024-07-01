// Package http provides a filesystem interface using golang.org/net/http
//
// It treats HTML pages served from the endpoint as directory
// listings, and includes any links found as files.
package http

import (
	"context"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"path"
	"strings"
	"sync"
	"time"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config/configmap"
	"github.com/rclone/rclone/fs/config/configstruct"
	"github.com/rclone/rclone/fs/fshttp"
	"github.com/rclone/rclone/fs/hash"
	"github.com/rclone/rclone/lib/rest"
	"golang.org/x/net/html"
)

var (
	errorReadOnly = errors.New("http remotes are read only")
	timeUnset     = time.Unix(0, 0)
)

func init() {
	fsi := &fs.RegInfo{
		Name:        "http",
		Description: "HTTP",
		NewFs:       NewFs,
		CommandHelp: commandHelp,
		Options: []fs.Option{{
			Name:     "url",
			Help:     "URL of HTTP host to connect to.\n\nE.g. \"https://example.com\", or \"https://user:pass@example.com\" to use a username and password.",
			Required: true,
		}, {
			Name: "headers",
			Help: `Set HTTP headers for all transactions.

Use this to set additional HTTP headers for all transactions.

The input format is comma separated list of key,value pairs.  Standard
[CSV encoding](https://godoc.org/encoding/csv) may be used.

For example, to set a Cookie use 'Cookie,name=value', or '"Cookie","name=value"'.

You can set multiple headers, e.g. '"Cookie","name=value","Authorization","xxx"'.`,
			Default:  fs.CommaSepList{},
			Advanced: true,
		}, {
			Name: "no_slash",
			Help: `Set this if the site doesn't end directories with /.

Use this if your target website does not use / on the end of
directories.

A / on the end of a path is how rclone normally tells the difference
between files and directories.  If this flag is set, then rclone will
treat all files with Content-Type: text/html as directories and read
URLs from them rather than downloading them.

Note that this may cause rclone to confuse genuine HTML files with
directories.`,
			Default:  false,
			Advanced: true,
		}, {
			Name: "no_head",
			Help: `Don't use HEAD requests.

HEAD requests are mainly used to find file sizes in dir listing.
If your site is being very slow to load then you can try this option.
Normally rclone does a HEAD request for each potential file in a
directory listing to:

- find its size
- check it really exists
- check to see if it is a directory

If you set this option, rclone will not do the HEAD request. This will mean
that directory listings are much quicker, but rclone won't have the times or
sizes of any files, and some files that don't exist may be in the listing.`,
			Default:  false,
			Advanced: true,
		}, {
			Name:    "no_escape",
			Help:    "Do not escape URL metacharacters in path names.",
			Default: false,
		}},
	}
	fs.Register(fsi)
}

// Options defines the configuration for this backend
type Options struct {
	Endpoint string          `config:"url"`
	NoSlash  bool            `config:"no_slash"`
	NoHead   bool            `config:"no_head"`
	Headers  fs.CommaSepList `config:"headers"`
	NoEscape bool            `config:"no_escape"`
}

// Fs stores the interface to the remote HTTP files
type Fs struct {
	name        string
	root        string
	features    *fs.Features   // optional features
	opt         Options        // options for this backend
	ci          *fs.ConfigInfo // global config
	endpoint    *url.URL
	endpointURL string // endpoint as a string
	httpClient  *http.Client
}

// Object is a remote object that has been stat'd (so it exists, but is not necessarily open for reading)
type Object struct {
	fs          *Fs
	remote      string
	size        int64
	modTime     time.Time
	contentType string
}

// statusError returns an error if the res contained an error
func statusError(res *http.Response, err error) error {
	if err != nil {
		return err
	}
	if res.StatusCode < 200 || res.StatusCode > 299 {
		_ = res.Body.Close()
		return fmt.Errorf("HTTP Error: %s", res.Status)
	}
	return nil
}

// getFsEndpoint decides if url is to be considered a file or directory,
// and returns a proper endpoint url to use for the fs.
func getFsEndpoint(ctx context.Context, client *http.Client, url string, opt *Options) (string, bool) {
	// If url ends with '/' it is already a proper url always assumed to be a directory.
	if url[len(url)-1] == '/' {
		return url, false
	}

	// If url does not end with '/' we send a HEAD request to decide
	// if it is directory or file, and if directory appends the missing
	// '/', or if file returns the directory url to parent instead.
	createFileResult := func() (string, bool) {
		fs.Debugf(nil, "If path is a directory you must add a trailing '/'")
		parent, _ := path.Split(url)
		return parent, true
	}
	createDirResult := func() (string, bool) {
		fs.Debugf(nil, "To avoid the initial HEAD request add a trailing '/' to the path")
		return url + "/", false
	}

	// If HEAD requests are not allowed we just have to assume it is a file.
	if opt.NoHead {
		fs.Debugf(nil, "Assuming path is a file as --http-no-head is set")
		return createFileResult()
	}

	// Use a client which doesn't follow redirects so the server
	// doesn't redirect http://host/dir to http://host/dir/
	noRedir := *client
	noRedir.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	}
	req, err := http.NewRequestWithContext(ctx, "HEAD", url, nil)
	if err != nil {
		fs.Debugf(nil, "Assuming path is a file as HEAD request could not be created: %v", err)
		return createFileResult()
	}
	addHeaders(req, opt)
	res, err := noRedir.Do(req)

	if err != nil {
		fs.Debugf(nil, "Assuming path is a file as HEAD request could not be sent: %v", err)
		return createFileResult()
	}
	if res.StatusCode == http.StatusNotFound {
		fs.Debugf(nil, "Assuming path is a directory as HEAD response is it does not exist as a file (%s)", res.Status)
		return createDirResult()
	}
	if res.StatusCode == http.StatusMovedPermanently ||
		res.StatusCode == http.StatusFound ||
		res.StatusCode == http.StatusSeeOther ||
		res.StatusCode == http.StatusTemporaryRedirect ||
		res.StatusCode == http.StatusPermanentRedirect {
		redir := res.Header.Get("Location")
		if redir != "" {
			if redir[len(redir)-1] == '/' {
				fs.Debugf(nil, "Assuming path is a directory as HEAD response is redirect (%s) to a path that ends with '/': %s", res.Status, redir)
				return createDirResult()
			}
			fs.Debugf(nil, "Assuming path is a file as HEAD response is redirect (%s) to a path that does not end with '/': %s", res.Status, redir)
			return createFileResult()
		}
		fs.Debugf(nil, "Assuming path is a file as HEAD response is redirect (%s) but no location header", res.Status)
		return createFileResult()
	}
	if res.StatusCode < 200 || res.StatusCode > 299 {
		// Example is 403 (http.StatusForbidden) for servers not allowing HEAD requests.
		fs.Debugf(nil, "Assuming path is a file as HEAD response is an error (%s)", res.Status)
		return createFileResult()
	}

	fs.Debugf(nil, "Assuming path is a file as HEAD response is success (%s)", res.Status)
	return createFileResult()
}

// Make the http connection with opt
func (f *Fs) httpConnection(ctx context.Context, opt *Options) (isFile bool, err error) {
	if len(opt.Headers)%2 != 0 {
		return false, errors.New("odd number of headers supplied")
	}

	if !strings.HasSuffix(opt.Endpoint, "/") {
		opt.Endpoint += "/"
	}

	// Parse the endpoint and stick the root onto it
	base, err := url.Parse(opt.Endpoint)
	if err != nil {
		return false, err
	}
	u, err := rest.URLJoin(base, rest.URLPathEscape(f.root))
	if err != nil {
		return false, err
	}

	client := fshttp.NewClient(ctx)

	endpoint, isFile := getFsEndpoint(ctx, client, u.String(), opt)
	fs.Debugf(nil, "Root: %s", endpoint)
	u, err = url.Parse(endpoint)
	if err != nil {
		return false, err
	}

	// Update f with the new parameters
	f.httpClient = client
	f.endpoint = u
	f.endpointURL = u.String()
	return isFile, nil
}

// NewFs creates a new Fs object from the name and root. It connects to
// the host specified in the config file.
func NewFs(ctx context.Context, name, root string, m configmap.Mapper) (fs.Fs, error) {
	// Parse config into Options struct
	opt := new(Options)
	err := configstruct.Set(m, opt)
	if err != nil {
		return nil, err
	}

	ci := fs.GetConfig(ctx)
	f := &Fs{
		name: name,
		root: root,
		opt:  *opt,
		ci:   ci,
	}
	f.features = (&fs.Features{
		CanHaveEmptyDirectories: true,
	}).Fill(ctx, f)

	// Make the http connection
	isFile, err := f.httpConnection(ctx, opt)
	if err != nil {
		return nil, err
	}

	if isFile {
		// return an error with an fs which points to the parent
		return f, fs.ErrorIsFile
	}

	if !strings.HasSuffix(f.endpointURL, "/") {
		return nil, errors.New("internal error: url doesn't end with /")
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
	return f.endpointURL
}

// Features returns the optional features of this Fs
func (f *Fs) Features() *fs.Features {
	return f.features
}

// Precision is the remote http file system's modtime precision, which we have no way of knowing. We estimate at 1s
func (f *Fs) Precision() time.Duration {
	return time.Second
}

// NewObject creates a new remote http file object
func (f *Fs) NewObject(ctx context.Context, remote string) (fs.Object, error) {
	o := &Object{
		fs:     f,
		remote: remote,
	}
	err := o.head(ctx)
	if err != nil {
		return nil, err
	}
	return o, nil
}

// Join's the remote onto the base URL
func (f *Fs) url(remote string) string {
	if f.opt.NoEscape {
		// Directly concatenate without escaping, no_escape behavior
		return f.endpointURL + remote
	}
	// Default behavior
	return f.endpointURL + rest.URLPathEscape(remote)
}

// Errors returned by parseName
var (
	errURLJoinFailed     = errors.New("URLJoin failed")
	errFoundQuestionMark = errors.New("found ? in URL")
	errHostMismatch      = errors.New("host mismatch")
	errSchemeMismatch    = errors.New("scheme mismatch")
	errNotUnderRoot      = errors.New("not under root")
	errNameIsEmpty       = errors.New("name is empty")
	errNameContainsSlash = errors.New("name contains /")
)

// parseName turns a name as found in the page into a remote path or returns an error
func parseName(base *url.URL, name string) (string, error) {
	// make URL absolute
	u, err := rest.URLJoin(base, name)
	if err != nil {
		return "", errURLJoinFailed
	}
	// check it doesn't have URL parameters
	uStr := u.String()
	if strings.Contains(uStr, "?") {
		return "", errFoundQuestionMark
	}
	// check that this is going back to the same host and scheme
	if base.Host != u.Host {
		return "", errHostMismatch
	}
	if base.Scheme != u.Scheme {
		return "", errSchemeMismatch
	}
	// check has path prefix
	if !strings.HasPrefix(u.Path, base.Path) {
		return "", errNotUnderRoot
	}
	// calculate the name relative to the base
	name = u.Path[len(base.Path):]
	// mustn't be empty
	if name == "" {
		return "", errNameIsEmpty
	}
	// mustn't contain a / - we are looking for a single level directory
	slash := strings.Index(name, "/")
	if slash >= 0 && slash != len(name)-1 {
		return "", errNameContainsSlash
	}
	return name, nil
}

// Parse turns HTML for a directory into names
// base should be the base URL to resolve any relative names from
func parse(base *url.URL, in io.Reader) (names []string, err error) {
	doc, err := html.Parse(in)
	if err != nil {
		return nil, err
	}
	var (
		walk func(*html.Node)
		seen = make(map[string]struct{})
	)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "a" {
			for _, a := range n.Attr {
				if a.Key == "href" {
					name, err := parseName(base, a.Val)
					if err == nil {
						if _, found := seen[name]; !found {
							names = append(names, name)
							seen[name] = struct{}{}
						}
					}
					break
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)
	return names, nil
}

// Adds the configured headers to the request if any
func addHeaders(req *http.Request, opt *Options) {
	for i := 0; i < len(opt.Headers); i += 2 {
		key := opt.Headers[i]
		value := opt.Headers[i+1]
		req.Header.Add(key, value)
	}
}

// Adds the configured headers to the request if any
func (f *Fs) addHeaders(req *http.Request) {
	addHeaders(req, &f.opt)
}

// Read the directory passed in
func (f *Fs) readDir(ctx context.Context, dir string) (names []string, err error) {
	URL := f.url(dir)
	u, err := url.Parse(URL)
	if err != nil {
		return nil, fmt.Errorf("failed to readDir: %w", err)
	}
	if !strings.HasSuffix(URL, "/") {
		return nil, fmt.Errorf("internal error: readDir URL %q didn't end in /", URL)
	}
	// Do the request
	req, err := http.NewRequestWithContext(ctx, "GET", URL, nil)
	if err != nil {
		return nil, fmt.Errorf("readDir failed: %w", err)
	}
	f.addHeaders(req)
	res, err := f.httpClient.Do(req)
	if err == nil {
		defer fs.CheckClose(res.Body, &err)
		if res.StatusCode == http.StatusNotFound {
			return nil, fs.ErrorDirNotFound
		}
	}
	err = statusError(res, err)
	if err != nil {
		return nil, fmt.Errorf("failed to readDir: %w", err)
	}

	contentType := strings.SplitN(res.Header.Get("Content-Type"), ";", 2)[0]
	switch contentType {
	case "text/html":
		names, err = parse(u, res.Body)
		if err != nil {
			return nil, fmt.Errorf("readDir: %w", err)
		}
	default:
		return nil, fmt.Errorf("can't parse content type %q", contentType)
	}
	return names, nil
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
	if !strings.HasSuffix(dir, "/") && dir != "" {
		dir += "/"
	}
	names, err := f.readDir(ctx, dir)
	if err != nil {
		return nil, fmt.Errorf("error listing %q: %w", dir, err)
	}
	var (
		entriesMu sync.Mutex // to protect entries
		wg        sync.WaitGroup
		checkers  = f.ci.Checkers
		in        = make(chan string, checkers)
	)
	add := func(entry fs.DirEntry) {
		entriesMu.Lock()
		entries = append(entries, entry)
		entriesMu.Unlock()
	}
	for i := 0; i < checkers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for remote := range in {
				file := &Object{
					fs:     f,
					remote: remote,
				}
				switch err := file.head(ctx); err {
				case nil:
					add(file)
				case fs.ErrorNotAFile:
					// ...found a directory not a file
					add(fs.NewDir(remote, time.Time{}))
				default:
					fs.Debugf(remote, "skipping because of error: %v", err)
				}
			}
		}()
	}
	for _, name := range names {
		isDir := name[len(name)-1] == '/'
		name = strings.TrimRight(name, "/")
		remote := path.Join(dir, name)
		if isDir {
			add(fs.NewDir(remote, time.Time{}))
		} else {
			in <- remote
		}
	}
	close(in)
	wg.Wait()
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
func (o *Object) Hash(ctx context.Context, r hash.Type) (string, error) {
	return "", hash.ErrUnsupported
}

// Size returns the size in bytes of the remote http file
func (o *Object) Size() int64 {
	return o.size
}

// ModTime returns the modification time of the remote http file
func (o *Object) ModTime(ctx context.Context) time.Time {
	return o.modTime
}

// url returns the native url of the object
func (o *Object) url() string {
	return o.fs.url(o.remote)
}

// head sends a HEAD request to update info fields in the Object
func (o *Object) head(ctx context.Context) error {
	if o.fs.opt.NoHead {
		o.size = -1
		o.modTime = timeUnset
		o.contentType = fs.MimeType(ctx, o)
		return nil
	}
	url := o.url()
	req, err := http.NewRequestWithContext(ctx, "HEAD", url, nil)
	if err != nil {
		return fmt.Errorf("stat failed: %w", err)
	}
	o.fs.addHeaders(req)
	res, err := o.fs.httpClient.Do(req)
	if err == nil && res.StatusCode == http.StatusNotFound {
		return fs.ErrorObjectNotFound
	}
	err = statusError(res, err)
	if err != nil {
		return fmt.Errorf("failed to stat: %w", err)
	}
	return o.decodeMetadata(ctx, res)
}

// decodeMetadata updates info fields in the Object according to HTTP response headers
func (o *Object) decodeMetadata(ctx context.Context, res *http.Response) error {
	t, err := http.ParseTime(res.Header.Get("Last-Modified"))
	if err != nil {
		t = timeUnset
	}
	o.modTime = t
	o.contentType = res.Header.Get("Content-Type")
	o.size = rest.ParseSizeFromHeaders(res.Header)

	// If NoSlash is set then check ContentType to see if it is a directory
	if o.fs.opt.NoSlash {
		mediaType, _, err := mime.ParseMediaType(o.contentType)
		if err != nil {
			return fmt.Errorf("failed to parse Content-Type: %q: %w", o.contentType, err)
		}
		if mediaType == "text/html" {
			return fs.ErrorNotAFile
		}
	}
	return nil
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
	url := o.url()
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("Open failed: %w", err)
	}

	// Add optional headers
	for k, v := range fs.OpenOptionHeaders(options) {
		req.Header.Add(k, v)
	}
	o.fs.addHeaders(req)

	// Do the request
	res, err := o.fs.httpClient.Do(req)
	err = statusError(res, err)
	if err != nil {
		return nil, fmt.Errorf("Open failed: %w", err)
	}
	if err = o.decodeMetadata(ctx, res); err != nil {
		return nil, fmt.Errorf("decodeMetadata failed: %w", err)
	}
	return res.Body, nil
}

// Hashes returns hash.HashNone to indicate remote hashing is unavailable
func (f *Fs) Hashes() hash.Set {
	return hash.Set(hash.None)
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

// Update in to the object with the modTime given of the given size
func (o *Object) Update(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) error {
	return errorReadOnly
}

// MimeType of an Object if known, "" otherwise
func (o *Object) MimeType(ctx context.Context) string {
	return o.contentType
}

var commandHelp = []fs.CommandHelp{{
	Name:  "set",
	Short: "Set command for updating the config parameters.",
	Long: `This set command can be used to update the config parameters
for a running http backend.

Usage Examples:

    rclone backend set remote: [-o opt_name=opt_value] [-o opt_name2=opt_value2]
    rclone rc backend/command command=set fs=remote: [-o opt_name=opt_value] [-o opt_name2=opt_value2]
    rclone rc backend/command command=set fs=remote: -o url=https://example.com

The option keys are named as they are in the config file.

This rebuilds the connection to the http backend when it is called with
the new parameters. Only new parameters need be passed as the values
will default to those currently in use.

It doesn't return anything.
`,
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
func (f *Fs) Command(ctx context.Context, name string, arg []string, opt map[string]string) (out interface{}, err error) {
	switch name {
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

// Check the interfaces are satisfied
var (
	_ fs.Fs          = &Fs{}
	_ fs.PutStreamer = &Fs{}
	_ fs.Object      = &Object{}
	_ fs.MimeTyper   = &Object{}
	_ fs.Commander   = &Fs{}
)
