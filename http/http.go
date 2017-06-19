// Package http provides a filesystem interface using golang.org/net/http
//
// It treads HTML pages served from the endpoint as directory
// listings, and includes any links found as files.

// +build !plan9

package http

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/ncw/rclone/fs"
	"github.com/pkg/errors"
	"golang.org/x/net/html"
)

var errorReadOnly = errors.New("http remotes are read only")

func init() {
	fsi := &fs.RegInfo{
		Name:        "http",
		Description: "http Connection",
		NewFs:       NewFs,
		Options: []fs.Option{{
			Name:     "endpoint",
			Help:     "URL of http host to connect to",
			Optional: false,
			Examples: []fs.OptionExample{{
				Value: "https://example.com",
				Help:  "Connect to example.com",
			}},
		}},
	}
	fs.Register(fsi)
}

// Fs stores the interface to the remote HTTP files
type Fs struct {
	name       string
	root       string
	features   *fs.Features // optional features
	endpoint   *url.URL
	httpClient *http.Client
}

// Object is a remote object that has been stat'd (so it exists, but is not necessarily open for reading)
type Object struct {
	fs     *Fs
	remote string
	info   os.FileInfo
}

// ObjectReader holds the File interface to a remote http file opened for reading
type ObjectReader struct {
	object   *Object
	httpFile io.ReadCloser
}

func urlJoin(u *url.URL, paths ...string) string {
	r := u
	for _, p := range paths {
		if p == "/" {
			continue
		}
		rel, _ := url.Parse(p)
		r = r.ResolveReference(rel)
	}
	return r.String()
}

// NewFs creates a new Fs object from the name and root. It connects to
// the host specified in the config file.
func NewFs(name, root string) (fs.Fs, error) {
	endpoint := fs.ConfigFileGet(name, "endpoint")

	u, err := url.Parse(endpoint)
	if err != nil {
		return nil, err
	}

	if !strings.HasSuffix(root, "/") && root != "" {
		root += "/"
	}

	client := fs.Config.Client()

	_, err = client.Head(urlJoin(u, root))
	if err != nil {
		return nil, errors.Wrap(err, "couldn't connect http")
	}
	f := &Fs{
		name:       name,
		root:       root,
		httpClient: client,
		endpoint:   u,
	}
	f.features = (&fs.Features{}).Fill(f)
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
	return urlJoin(f.endpoint, f.root)
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
func (f *Fs) NewObject(remote string) (fs.Object, error) {
	o := &Object{
		fs:     f,
		remote: remote,
	}
	err := o.stat()
	if err != nil {
		return nil, errors.Wrap(err, "Stat failed")
	}
	return o, nil
}

// dirExists returns true,nil if the directory exists, false, nil if
// it doesn't or false, err
func (f *Fs) dirExists(dir string) (bool, error) {
	res, err := f.httpClient.Head(urlJoin(f.endpoint, dir))
	if err != nil {
		return false, err
	}
	if res.StatusCode == http.StatusOK {
		return true, nil
	}
	return false, nil
}

type entry struct {
	name  string
	url   string
	size  int64
	mode  os.FileMode
	mtime int64
}

func (e *entry) Name() string {
	return e.name
}

func (e *entry) Size() int64 {
	return e.size
}

func (e *entry) Mode() os.FileMode {
	return os.FileMode(e.mode)
}

func (e *entry) ModTime() time.Time {
	return time.Unix(e.mtime, 0)
}

func (e *entry) IsDir() bool {
	return e.mode&os.ModeDir != 0
}

func (e *entry) Sys() interface{} {
	return nil
}

func parseInt64(s string) int64 {
	n, e := strconv.ParseInt(s, 10, 64)
	if e != nil {
		return 0
	}
	return n
}

func parseBool(s string) bool {
	b, e := strconv.ParseBool(s)
	if e != nil {
		return false
	}
	return b
}

func prepareTimeString(ts string) string {
	return strings.Trim(strings.Join(strings.SplitN(strings.Trim(ts, "\t "), " ", 3)[0:2], " "), "\r\n\t ")
}

func parseTime(n *html.Node) (t time.Time) {
	if ts := prepareTimeString(n.Data); ts != "" {
		t, _ = time.Parse("2-Jan-2006 15:04", ts)
	}
	return t
}

func (f *Fs) readDir(p string) ([]*entry, error) {
	entries := make([]*entry, 0)
	res, err := f.httpClient.Get(urlJoin(f.endpoint, p))
	if err != nil {
		return nil, err
	}
	if res.Body == nil || res.StatusCode != http.StatusOK {
		//return nil, errors.Errorf("directory listing failed with error: % (%d)", res.Status, res.StatusCode)
		return nil, nil
	}
	defer fs.CheckClose(res.Body, &err)

	switch strings.SplitN(res.Header.Get("Content-Type"), ";", 2)[0] {
	case "text/html":
		doc, err := html.Parse(res.Body)
		if err != nil {
			return nil, err
		}
		var walk func(*html.Node)
		walk = func(n *html.Node) {
			if n.Type == html.ElementNode && n.Data == "a" {
				for _, a := range n.Attr {
					if a.Key == "href" {
						name, err := url.QueryUnescape(a.Val)
						if err != nil {
							continue
						}
						if name == "../" || name == "./" || name == ".." {
							break
						}
						if strings.Index(name, "?") >= 0 || strings.HasPrefix(name, "http") {
							break
						}
						u, err := url.Parse(name)
						if err != nil {
							break
						}
						name = path.Clean(u.Path)
						e := &entry{
							name: strings.TrimRight(name, "/"),
							url:  name,
						}
						if a.Val[len(a.Val)-1] == '/' {
							e.mode = os.FileMode(0555) | os.ModeDir
						} else {
							e.mode = os.FileMode(0444)
						}
						entries = append(entries, e)
						break
					}
				}
			}
			for c := n.FirstChild; c != nil; c = c.NextSibling {
				walk(c)
			}
		}
		walk(doc)
	}
	return entries, nil
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
func (f *Fs) List(dir string) (entries fs.DirEntries, err error) {
	endpoint := path.Join(f.root, dir)
	if !strings.HasSuffix(dir, "/") {
		endpoint += "/"
	}
	ok, err := f.dirExists(endpoint)
	if err != nil {
		return nil, errors.Wrap(err, "List failed")
	}
	if !ok {
		return nil, fs.ErrorDirNotFound
	}
	httpDir := path.Join(f.root, dir)
	if !strings.HasSuffix(dir, "/") {
		httpDir += "/"
	}
	infos, err := f.readDir(httpDir)
	if err != nil {
		return nil, errors.Wrapf(err, "error listing %q", dir)
	}
	for _, info := range infos {
		remote := ""
		if dir != "" {
			remote = dir + "/" + info.Name()
		} else {
			remote = info.Name()
		}
		if info.IsDir() {
			dir := &fs.Dir{
				Name:  remote,
				When:  info.ModTime(),
				Bytes: 0,
				Count: 0,
			}
			entries = append(entries, dir)
		} else {
			file := &Object{
				fs:     f,
				remote: remote,
				info:   info,
			}
			if err = file.stat(); err != nil {
				continue
			}
			entries = append(entries, file)
		}
	}
	return entries, nil
}

// Put in to the remote path with the modTime given of the given size
//
// May create the object even if it returns an error - if so
// will return the object and the error, otherwise will return
// nil and the error
func (f *Fs) Put(in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error) {
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
func (o *Object) Hash(r fs.HashType) (string, error) {
	return "", fs.ErrHashUnsupported
}

// Size returns the size in bytes of the remote http file
func (o *Object) Size() int64 {
	return o.info.Size()
}

// ModTime returns the modification time of the remote http file
func (o *Object) ModTime() time.Time {
	return o.info.ModTime()
}

// path returns the native path of the object
func (o *Object) path() string {
	return path.Join(o.fs.root, o.remote)
}

// stat updates the info field in the Object
func (o *Object) stat() error {
	endpoint := urlJoin(o.fs.endpoint, o.fs.root, o.remote)
	if o.info.IsDir() {
		endpoint += "/"
	}
	res, err := o.fs.httpClient.Head(endpoint)
	if err != nil {
		return err
	}
	if res.StatusCode != http.StatusOK {
		return errors.New("failed to stat")
	}
	var mtime int64
	t, err := http.ParseTime(res.Header.Get("Last-Modified"))
	if err != nil {
		mtime = 0
	} else {
		mtime = t.Unix()
	}
	size := parseInt64(res.Header.Get("Content-Length"))
	e := &entry{
		name:  o.remote,
		size:  size,
		mtime: mtime,
		mode:  os.FileMode(0444),
	}
	if strings.HasSuffix(o.remote, "/") {
		e.mode = os.FileMode(0555) | os.ModeDir
		e.size = 0
		e.name = o.remote[:len(o.remote)-1]
	}
	o.info = e
	return nil
}

// SetModTime sets the modification and access time to the specified time
//
// it also updates the info field
func (o *Object) SetModTime(modTime time.Time) error {
	return errorReadOnly
}

// Storable returns whether the remote http file is a regular file (not a directory, symbolic link, block device, character device, named pipe, etc)
func (o *Object) Storable() bool {
	return o.info.Mode().IsRegular()
}

// Read from a remote http file object reader
func (file *ObjectReader) Read(p []byte) (n int, err error) {
	n, err = file.httpFile.Read(p)
	return n, err
}

// Close a reader of a remote http file
func (file *ObjectReader) Close() (err error) {
	return file.httpFile.Close()
}

// Open a remote http file object for reading. Seek is supported
func (o *Object) Open(options ...fs.OpenOption) (in io.ReadCloser, err error) {
	var offset int64
	endpoint := urlJoin(o.fs.endpoint, o.fs.root, o.remote)
	offset = 0
	for _, option := range options {
		switch x := option.(type) {
		case *fs.SeekOption:
			offset = x.Offset
		default:
			if option.Mandatory() {
				fs.Logf(o, "Unsupported mandatory option: %v", option)
			}
		}
	}

	req, err := http.NewRequest("GET", endpoint, nil)
	if err != nil {
		return nil, errors.Wrap(err, "Open failed")
	}
	if offset > 0 {
		req.Header.Set("Range", fmt.Sprintf("bytes=%d-", offset))
	}
	res, err := o.fs.httpClient.Do(req)
	if err != nil {
		return nil, errors.Wrap(err, "Open failed")
	}
	in = &ObjectReader{
		object:   o,
		httpFile: res.Body,
	}
	return in, nil
}

// Hashes returns fs.HashNone to indicate remote hashing is unavailable
func (f *Fs) Hashes() fs.HashSet {
	return fs.HashSet(fs.HashNone)
}

// Mkdir makes the root directory of the Fs object
func (f *Fs) Mkdir(dir string) error {
	return errorReadOnly
}

// Remove a remote http file object
func (o *Object) Remove() error {
	return errorReadOnly
}

// Rmdir removes the root directory of the Fs object
func (f *Fs) Rmdir(dir string) error {
	return errorReadOnly
}

// Update in to the object with the modTime given of the given size
func (o *Object) Update(in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) error {
	return errorReadOnly
}

// Check the interfaces are satisfied
var (
	_ fs.Fs     = &Fs{}
	_ fs.Object = &Object{}
)
