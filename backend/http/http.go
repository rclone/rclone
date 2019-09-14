// Package http provides a filesystem interface using golang.org/net/http
//
// It treats HTML pages served from the endpoint as directory
// listings, and includes any links found as files.
package http

import (
	"context"
	"io"
	"mime"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/pkg/errors"
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
		Description: "http Connection",
		NewFs:       NewFs,
		Options: []fs.Option{{
			Name:     "url",
			Help:     "URL of http host to connect to",
			Required: true,
			Examples: []fs.OptionExample{{
				Value: "https://example.com",
				Help:  "Connect to example.com",
			}, {
				Value: "https://user:pass@example.com",
				Help:  "Connect to example.com using a username and password",
			}},
		}, {
			Name: "headers",
			Help: `Set HTTP headers for all transactions

Use this to set additional HTTP headers for all transactions

The input format is comma separated list of key,value pairs.  Standard
[CSV encoding](https://godoc.org/encoding/csv) may be used.

For example to set a Cookie use 'Cookie,name=value', or '"Cookie","name=value"'.

You can set multiple headers, eg '"Cookie","name=value","Authorization","xxx"'.
`,
			Default:  fs.CommaSepList{},
			Advanced: true,
		}, {
			Name: "no_slash",
			Help: `Set this if the site doesn't end directories with /

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
			Help: `Don't use HEAD requests to find file sizes in dir listing

If your site is being very slow to load then you can try this option.
Normally rclone does a HEAD request for each potential file in a
directory listing to:

- find its size
- check it really exists
- check to see if it is a directory

If you set this option, rclone will not do the HEAD request.  This will mean

- directory listings are much quicker
- rclone won't have the times or sizes of any files
- some files that don't exist may be in the listing
`,
			Default:  false,
			Advanced: true,
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
}

// Fs stores the interface to the remote HTTP files
type Fs struct {
	name        string
	root        string
	features    *fs.Features // optional features
	opt         Options      // options for this backend
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
		return errors.Errorf("HTTP Error %d: %s", res.StatusCode, res.Status)
	}
	return nil
}

// NewFs creates a new Fs object from the name and root. It connects to
// the host specified in the config file.
func NewFs(name, root string, m configmap.Mapper) (fs.Fs, error) {
	ctx := context.TODO()
	// Parse config into Options struct
	opt := new(Options)
	err := configstruct.Set(m, opt)
	if err != nil {
		return nil, err
	}

	if len(opt.Headers)%2 != 0 {
		return nil, errors.New("odd number of headers supplied")
	}

	if !strings.HasSuffix(opt.Endpoint, "/") {
		opt.Endpoint += "/"
	}

	// Parse the endpoint and stick the root onto it
	base, err := url.Parse(opt.Endpoint)
	if err != nil {
		return nil, err
	}
	u, err := rest.URLJoin(base, rest.URLPathEscape(root))
	if err != nil {
		return nil, err
	}

	client := fshttp.NewClient(fs.Config)

	var isFile = false
	if !strings.HasSuffix(u.String(), "/") {
		// Make a client which doesn't follow redirects so the server
		// doesn't redirect http://host/dir to http://host/dir/
		noRedir := *client
		noRedir.CheckRedirect = func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		}
		// check to see if points to a file
		req, err := http.NewRequest("HEAD", u.String(), nil)
		if err == nil {
			req = req.WithContext(ctx) // go1.13 can use NewRequestWithContext
			addHeaders(req, opt)
			res, err := noRedir.Do(req)
			err = statusError(res, err)
			if err == nil {
				isFile = true
			}
		}
	}

	newRoot := u.String()
	if isFile {
		// Point to the parent if this is a file
		newRoot, _ = path.Split(u.String())
	} else {
		if !strings.HasSuffix(newRoot, "/") {
			newRoot += "/"
		}
	}

	u, err = url.Parse(newRoot)
	if err != nil {
		return nil, err
	}

	f := &Fs{
		name:        name,
		root:        root,
		opt:         *opt,
		httpClient:  client,
		endpoint:    u,
		endpointURL: u.String(),
	}
	f.features = (&fs.Features{
		CanHaveEmptyDirectories: true,
	}).Fill(f)
	if isFile {
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
	err := o.stat(ctx)
	if err != nil {
		return nil, err
	}
	return o, nil
}

// Join's the remote onto the base URL
func (f *Fs) url(remote string) string {
	return f.endpointURL + rest.URLPathEscape(remote)
}

// parse s into an int64, on failure return def
func parseInt64(s string, def int64) int64 {
	n, e := strconv.ParseInt(s, 10, 64)
	if e != nil {
		return def
	}
	return n
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
	if strings.Index(uStr, "?") >= 0 {
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
		return nil, errors.Wrap(err, "failed to readDir")
	}
	if !strings.HasSuffix(URL, "/") {
		return nil, errors.Errorf("internal error: readDir URL %q didn't end in /", URL)
	}
	// Do the request
	req, err := http.NewRequest("GET", URL, nil)
	if err != nil {
		return nil, errors.Wrap(err, "readDir failed")
	}
	req = req.WithContext(ctx) // go1.13 can use NewRequestWithContext
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
		return nil, errors.Wrap(err, "failed to readDir")
	}

	contentType := strings.SplitN(res.Header.Get("Content-Type"), ";", 2)[0]
	switch contentType {
	case "text/html":
		names, err = parse(u, res.Body)
		if err != nil {
			return nil, errors.Wrap(err, "readDir")
		}
	default:
		return nil, errors.Errorf("Can't parse content type %q", contentType)
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
		return nil, errors.Wrapf(err, "error listing %q", dir)
	}
	var (
		entriesMu sync.Mutex // to protect entries
		wg        sync.WaitGroup
		in        = make(chan string, fs.Config.Checkers)
	)
	add := func(entry fs.DirEntry) {
		entriesMu.Lock()
		entries = append(entries, entry)
		entriesMu.Unlock()
	}
	for i := 0; i < fs.Config.Checkers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for remote := range in {
				file := &Object{
					fs:     f,
					remote: remote,
				}
				switch err := file.stat(ctx); err {
				case nil:
					add(file)
				case fs.ErrorNotAFile:
					// ...found a directory not a file
					add(fs.NewDir(remote, timeUnset))
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
			add(fs.NewDir(remote, timeUnset))
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

// stat updates the info field in the Object
func (o *Object) stat(ctx context.Context) error {
	if o.fs.opt.NoHead {
		o.size = -1
		o.modTime = timeUnset
		o.contentType = fs.MimeType(ctx, o)
		return nil
	}
	url := o.url()
	req, err := http.NewRequest("HEAD", url, nil)
	if err != nil {
		return errors.Wrap(err, "stat failed")
	}
	req = req.WithContext(ctx) // go1.13 can use NewRequestWithContext
	o.fs.addHeaders(req)
	res, err := o.fs.httpClient.Do(req)
	if err == nil && res.StatusCode == http.StatusNotFound {
		return fs.ErrorObjectNotFound
	}
	err = statusError(res, err)
	if err != nil {
		return errors.Wrap(err, "failed to stat")
	}
	t, err := http.ParseTime(res.Header.Get("Last-Modified"))
	if err != nil {
		t = timeUnset
	}
	o.size = parseInt64(res.Header.Get("Content-Length"), -1)
	o.modTime = t
	o.contentType = res.Header.Get("Content-Type")
	// If NoSlash is set then check ContentType to see if it is a directory
	if o.fs.opt.NoSlash {
		mediaType, _, err := mime.ParseMediaType(o.contentType)
		if err != nil {
			return errors.Wrapf(err, "failed to parse Content-Type: %q", o.contentType)
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

// Storable returns whether the remote http file is a regular file (not a directory, symbolic link, block device, character device, named pipe, etc)
func (o *Object) Storable() bool {
	return true
}

// Open a remote http file object for reading. Seek is supported
func (o *Object) Open(ctx context.Context, options ...fs.OpenOption) (in io.ReadCloser, err error) {
	url := o.url()
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, errors.Wrap(err, "Open failed")
	}
	req = req.WithContext(ctx) // go1.13 can use NewRequestWithContext

	// Add optional headers
	for k, v := range fs.OpenOptionHeaders(options) {
		req.Header.Add(k, v)
	}
	o.fs.addHeaders(req)

	// Do the request
	res, err := o.fs.httpClient.Do(req)
	err = statusError(res, err)
	if err != nil {
		return nil, errors.Wrap(err, "Open failed")
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

// Check the interfaces are satisfied
var (
	_ fs.Fs          = &Fs{}
	_ fs.PutStreamer = &Fs{}
	_ fs.Object      = &Object{}
	_ fs.MimeTyper   = &Object{}
)
