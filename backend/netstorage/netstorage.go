// Package netstorage provides an interface to Akamai NetStorage API
package netstorage

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/xml"
	"errors"
	"fmt"
	gohash "hash"
	"io"
	"io/ioutil"
	"math/rand"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config/configmap"
	"github.com/rclone/rclone/fs/config/configstruct"
	"github.com/rclone/rclone/fs/config/obscure"
	"github.com/rclone/rclone/fs/fserrors"
	"github.com/rclone/rclone/fs/fshttp"
	"github.com/rclone/rclone/fs/hash"
	"github.com/rclone/rclone/fs/walk"
	"github.com/rclone/rclone/lib/pacer"
	"github.com/rclone/rclone/lib/rest"
)

// Constants
const (
	minSleep      = 10 * time.Millisecond
	maxSleep      = 2 * time.Second
	decayConstant = 2 // bigger for slower decay, exponential
)

func init() {
	fsi := &fs.RegInfo{
		Name:        "netstorage",
		Description: "Akamai NetStorage",
		NewFs:       NewFs,
		CommandHelp: commandHelp,
		Options: []fs.Option{{
			Name: "protocol",
			Help: `Select between HTTP or HTTPS protocol.

Most users should choose HTTPS, which is the default.
HTTP is provided primarily for debugging purposes.`,
			Examples: []fs.OptionExample{{
				Value: "http",
				Help:  "HTTP protocol",
			}, {
				Value: "https",
				Help:  "HTTPS protocol",
			}},
			Default:  "https",
			Advanced: true,
		}, {
			Name: "host",
			Help: `Domain+path of NetStorage host to connect to.

Format should be ` + "`<domain>/<internal folders>`",
			Required: true,
		}, {
			Name:     "account",
			Help:     "Set the NetStorage account name",
			Required: true,
		}, {
			Name: "secret",
			Help: `Set the NetStorage account secret/G2O key for authentication.

Please choose the 'y' option to set your own password then enter your secret.`,
			IsPassword: true,
			Required:   true,
		}},
	}
	fs.Register(fsi)
}

var commandHelp = []fs.CommandHelp{{
	Name:  "du",
	Short: "Return disk usage information for a specified directory",
	Long: `The usage information returned, includes the targeted directory as well as all
files stored in any sub-directories that may exist.`,
}, {
	Name:  "symlink",
	Short: "You can create a symbolic link in ObjectStore with the symlink action.",
	Long: `The desired path location (including applicable sub-directories) ending in
the object that will be the target of the symlink (for example, /links/mylink).
Include the file extension for the object, if applicable.
` + "`rclone backend symlink <src> <path>`",
},
}

// Options defines the configuration for this backend
type Options struct {
	Endpoint string `config:"host"`
	Account  string `config:"account"`
	Secret   string `config:"secret"`
	Protocol string `config:"protocol"`
}

// Fs stores the interface to the remote HTTP files
type Fs struct {
	name             string
	root             string
	features         *fs.Features      // optional features
	opt              Options           // options for this backend
	endpointURL      string            // endpoint as a string
	srv              *rest.Client      // the connection to the Netstorage server
	pacer            *fs.Pacer         // to pace the API calls
	filetype         string            // dir, file or symlink
	dirscreated      map[string]bool   // if implicit dir has been created already
	dirscreatedMutex sync.Mutex        // mutex to protect dirscreated
	statcache        map[string][]File // cache successfull stat requests
	statcacheMutex   sync.RWMutex      // RWMutex to protect statcache
}

// Object is a remote object that has been stat'd (so it exists, but is not necessarily open for reading)
type Object struct {
	fs       *Fs
	filetype string // dir, file or symlink
	remote   string // remote path
	size     int64  // size of the object in bytes
	modTime  int64  // modification time of the object
	md5sum   string // md5sum of the object
	fullURL  string // full path URL
	target   string // symlink target when filetype is symlink
}

//------------------------------------------------------------------------------

// Stat is an object which holds the information of the stat element of the response xml
type Stat struct {
	XMLName   xml.Name `xml:"stat"`
	Files     []File   `xml:"file"`
	Directory string   `xml:"directory,attr"`
}

// File is an object which holds the information of the file element of the response xml
type File struct {
	XMLName    xml.Name `xml:"file"`
	Type       string   `xml:"type,attr"`
	Name       string   `xml:"name,attr"`
	NameBase64 string   `xml:"name_base64,attr"`
	Size       int64    `xml:"size,attr"`
	Md5        string   `xml:"md5,attr"`
	Mtime      int64    `xml:"mtime,attr"`
	Bytes      int64    `xml:"bytes,attr"`
	Files      int64    `xml:"files,attr"`
	Target     string   `xml:"target,attr"`
}

// List is an object which holds the information of the list element of the response xml
type List struct {
	XMLName xml.Name   `xml:"list"`
	Files   []File     `xml:"file"`
	Resume  ListResume `xml:"resume"`
}

// ListResume represents the resume xml element of the list
type ListResume struct {
	XMLName xml.Name `xml:"resume"`
	Start   string   `xml:"start,attr"`
}

// Du represents the du xml element of the response
type Du struct {
	XMLName   xml.Name `xml:"du"`
	Directory string   `xml:"directory,attr"`
	Duinfo    DuInfo   `xml:"du-info"`
}

// DuInfo represents the du-info xml element of the response
type DuInfo struct {
	XMLName xml.Name `xml:"du-info"`
	Files   int64    `xml:"files,attr"`
	Bytes   int64    `xml:"bytes,attr"`
}

// GetName returns a normalized name of the Stat item
func (s Stat) GetName() xml.Name {
	return s.XMLName
}

// GetName returns a normalized name of the List item
func (l List) GetName() xml.Name {
	return l.XMLName
}

// GetName returns a normalized name of the Du item
func (d Du) GetName() xml.Name {
	return d.XMLName
}

//------------------------------------------------------------------------------

// NewFs creates a new Fs object from the name and root. It connects to
// the host specified in the config file.
//
// If root refers to an existing object, then it should return an Fs which
// points to the parent of that object and ErrorIsFile.
func NewFs(ctx context.Context, name, root string, m configmap.Mapper) (fs.Fs, error) {
	// Parse config into Options struct
	opt := new(Options)
	err := configstruct.Set(m, opt)
	if err != nil {
		return nil, err
	}

	// The base URL (endPoint is protocol + :// + domain/internal folder
	opt.Endpoint = opt.Protocol + "://" + opt.Endpoint
	fs.Debugf(nil, "NetStorage NewFS endpoint %q", opt.Endpoint)
	if !strings.HasSuffix(opt.Endpoint, "/") {
		opt.Endpoint += "/"
	}

	// Decrypt credentials, even though it is hard to eyedrop the hex string, it adds an extra piece of mind
	opt.Secret = obscure.MustReveal(opt.Secret)

	// Parse the endpoint and stick the root onto it
	base, err := url.Parse(opt.Endpoint)
	if err != nil {
		return nil, fmt.Errorf("couldn't parse URL %q: %w", opt.Endpoint, err)
	}
	u, err := rest.URLJoin(base, rest.URLPathEscape(root))
	if err != nil {
		return nil, fmt.Errorf("couldn't join URL %q and %q: %w", base.String(), root, err)
	}
	client := fshttp.NewClient(ctx)

	f := &Fs{
		name:        name,
		root:        root,
		opt:         *opt,
		endpointURL: u.String(),
		pacer:       fs.NewPacer(ctx, pacer.NewDefault(pacer.MinSleep(minSleep), pacer.MaxSleep(maxSleep), pacer.DecayConstant(decayConstant))),
		dirscreated: make(map[string]bool),
		statcache:   make(map[string][]File),
	}
	f.srv = rest.NewClient(client)
	f.srv.SetSigner(f.getAuth)

	f.features = (&fs.Features{
		CanHaveEmptyDirectories: true,
	}).Fill(ctx, f)

	err = f.initFs(ctx, "")
	switch err {
	case nil:
		// Object is the directory
		return f, nil
	case fs.ErrorObjectNotFound:
		return f, nil
	case fs.ErrorIsFile:
		// Fs points to the parent directory
		return f, err
	default:
		return nil, err
	}
}

// Command the backend to run a named commands: du and symlink
func (f *Fs) Command(ctx context.Context, name string, arg []string, opt map[string]string) (out interface{}, err error) {
	switch name {
	case "du":
		// No arg parsing needed, the path is passed in the fs
		return f.netStorageDuRequest(ctx)
	case "symlink":
		dst := ""
		if len(arg) > 0 {
			dst = arg[0]
		} else {
			return nil, errors.New("NetStorage symlink command: need argument for target")
		}
		// Strip off the leading slash added by NewFs on object not found
		URL := strings.TrimSuffix(f.url(""), "/")
		return f.netStorageSymlinkRequest(ctx, URL, dst, nil)
	default:
		return nil, fs.ErrorCommandNotFound
	}
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

// Precision return the precision of this Fs
func (f *Fs) Precision() time.Duration {
	return time.Second
}

// NewObject creates a new remote http file object
// NewObject finds the Object at remote
// If it can't be found returns fs.ErrorObjectNotFound
// If it isn't a file, then it returns fs.ErrorIsDir
func (f *Fs) NewObject(ctx context.Context, remote string) (fs.Object, error) {
	URL := f.url(remote)
	files, err := f.netStorageStatRequest(ctx, URL, false)
	if err != nil {
		return nil, err
	}
	if files == nil {
		fs.Errorf(nil, "Stat for %q has empty files", URL)
		return nil, fs.ErrorObjectNotFound
	}

	file := files[0]
	switch file.Type {
	case
		"file",
		"symlink":
		return f.newObjectWithInfo(remote, &file)
	case "dir":
		return nil, fs.ErrorIsDir
	default:
		return nil, fmt.Errorf("object of an unsupported type %s for %q: %w", file.Type, URL, err)
	}
}

// initFs initializes Fs based on the stat reply
func (f *Fs) initFs(ctx context.Context, dir string) error {
	// Path must end with the slash, so the join later will work correctly
	defer func() {
		if !strings.HasSuffix(f.endpointURL, "/") {
			f.endpointURL += "/"
		}
	}()
	URL := f.url(dir)
	files, err := f.netStorageStatRequest(ctx, URL, true)
	if err == fs.ErrorObjectNotFound || files == nil {
		return fs.ErrorObjectNotFound
	}
	if err != nil {
		return err
	}

	f.filetype = files[0].Type
	switch f.filetype {
	case "dir":
		// This directory is known to exist, adding to explicit directories
		f.setDirscreated(URL)
		return nil
	case
		"file",
		"symlink":
		// Fs should point to the parent of that object and return ErrorIsFile
		lastindex := strings.LastIndex(f.endpointURL, "/")
		if lastindex != -1 {
			f.endpointURL = f.endpointURL[0 : lastindex+1]
		} else {
			fs.Errorf(nil, "Remote URL %q unexpectedly does not include the slash", f.endpointURL)
		}
		return fs.ErrorIsFile
	default:
		err = fmt.Errorf("unsupported object type %s for %q: %w", f.filetype, URL, err)
		f.filetype = ""
		return err
	}
}

// url joins the remote onto the endpoint URL
func (f *Fs) url(remote string) string {
	if remote == "" {
		return f.endpointURL
	}

	pathescapeURL := rest.URLPathEscape(remote)
	// Strip off initial "./" from the path, which can be added by path escape function following the RFC 3986 4.2
	// (a segment must be preceded by a dot-segment (e.g., "./this:that") to make a relative-path reference).
	pathescapeURL = strings.TrimPrefix(pathescapeURL, "./")
	// Cannot use rest.URLJoin() here because NetStorage is an object storage and allows to have a "."
	// directory name, which will be eliminated by the join function.
	return f.endpointURL + pathescapeURL
}

// getFileName returns the file name if present, otherwise decoded name_base64
// if present, otherwise an empty string
func (f *Fs) getFileName(file *File) string {
	if file.Name != "" {
		return file.Name
	}
	if file.NameBase64 != "" {
		decoded, err := base64.StdEncoding.DecodeString(file.NameBase64)
		if err == nil {
			return string(decoded)
		}
		fs.Errorf(nil, "Failed to base64 decode object %s: %v", file.NameBase64, err)
	}
	return ""
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
	if f.filetype == "" {
		// This happens in two scenarios.
		// 1. NewFs is done on a non-existent object, then later rclone attempts to List/ListR this NewFs.
		// 2. List/ListR is called from the context of test_all and not the regular rclone binary.
		err := f.initFs(ctx, dir)
		if err != nil {
			if err == fs.ErrorObjectNotFound {
				return nil, fs.ErrorDirNotFound
			}
			return nil, err
		}
	}

	URL := f.url(dir)
	files, err := f.netStorageDirRequest(ctx, dir, URL)
	if err != nil {
		return nil, err
	}
	if dir != "" && !strings.HasSuffix(dir, "/") {
		dir += "/"
	}
	for _, item := range files {
		name := dir + f.getFileName(&item)
		switch item.Type {
		case "dir":
			when := time.Unix(item.Mtime, 0)
			entry := fs.NewDir(name, when).SetSize(item.Bytes).SetItems(item.Files)
			entries = append(entries, entry)
		case "file":
			if entry, _ := f.newObjectWithInfo(name, &item); entry != nil {
				entries = append(entries, entry)
			}
		case "symlink":
			var entry fs.Object
			// Add .rclonelink suffix to allow local backend code to convert to a symlink.
			// In case both .rclonelink file AND symlink file exists, the first will be used.
			if entry, _ = f.newObjectWithInfo(name+".rclonelink", &item); entry != nil {
				fs.Infof(nil, "Converting a symlink to the rclonelink %s target %s", entry.Remote(), item.Target)
				entries = append(entries, entry)
			}
		default:
			fs.Logf(nil, "Ignoring unsupported object type %s for %q path", item.Type, name)
		}
	}
	return entries, nil
}

// ListR lists the objects and directories of the Fs starting
// from dir recursively into out.
//
// dir should be "" to start from the root, and should not
// have trailing slashes.
//
// This should return ErrDirNotFound if the directory isn't
// found.
//
// It should call callback for each tranche of entries read.
// These need not be returned in any particular order.  If
// callback returns an error then the listing will stop
// immediately.
//
// Don't implement this unless you have a more efficient way
// of listing recursively that doing a directory traversal.
func (f *Fs) ListR(ctx context.Context, dir string, callback fs.ListRCallback) (err error) {
	if f.filetype == "" {
		// This happens in two scenarios.
		// 1. NewFs is done on a non-existent object, then later rclone attempts to List/ListR this NewFs.
		// 2. List/ListR is called from the context of test_all and not the regular rclone binary.
		err := f.initFs(ctx, dir)
		if err != nil {
			if err == fs.ErrorObjectNotFound {
				return fs.ErrorDirNotFound
			}
			return err
		}
	}

	if !strings.HasSuffix(dir, "/") && dir != "" {
		dir += "/"
	}
	URL := f.url(dir)
	u, err := url.Parse(URL)
	if err != nil {
		fs.Errorf(nil, "Unable to parse URL %q: %v", URL, err)
		return fs.ErrorDirNotFound
	}

	list := walk.NewListRHelper(callback)
	for resumeStart := u.Path; resumeStart != ""; {
		var files []File
		files, resumeStart, err = f.netStorageListRequest(ctx, URL, u.Path)
		if err != nil {
			if err == fs.ErrorObjectNotFound {
				return fs.ErrorDirNotFound
			}
			return err
		}
		for _, item := range files {
			name := f.getFileName(&item)
			// List output includes full paths starting from [CP Code]/
			path := strings.TrimPrefix("/"+name, u.Path)
			if path == "" {
				// Skip the starting directory itself
				continue
			}
			switch item.Type {
			case "dir":
				when := time.Unix(item.Mtime, 0)
				entry := fs.NewDir(dir+strings.TrimSuffix(path, "/"), when)
				if err := list.Add(entry); err != nil {
					return err
				}
			case "file":
				if entry, _ := f.newObjectWithInfo(dir+path, &item); entry != nil {
					if err := list.Add(entry); err != nil {
						return err
					}
				}
			case "symlink":
				// Add .rclonelink suffix to allow local backend code to convert to a symlink.
				// In case both .rclonelink file AND symlink file exists, the first will be used.
				if entry, _ := f.newObjectWithInfo(dir+path+".rclonelink", &item); entry != nil {
					fs.Infof(nil, "Converting a symlink to the rclonelink %s for target %s", entry.Remote(), item.Target)
					if err := list.Add(entry); err != nil {
						return err
					}
				}
			default:
				fs.Logf(nil, "Ignoring unsupported object type %s for %s path", item.Type, name)
			}
		}
		if resumeStart != "" {
			// Perform subsequent list action call, construct the
			// URL where the previous request finished
			u, err := url.Parse(f.endpointURL)
			if err != nil {
				fs.Errorf(nil, "Unable to parse URL %q: %v", f.endpointURL, err)
				return fs.ErrorDirNotFound
			}
			resumeURL, err := rest.URLJoin(u, rest.URLPathEscape(resumeStart))
			if err != nil {
				fs.Errorf(nil, "Unable to join URL %q for resumeStart %s: %v", f.endpointURL, resumeStart, err)
				return fs.ErrorDirNotFound
			}
			URL = resumeURL.String()
		}

	}
	return list.Flush()
}

// Put in to the remote path with the modTime given of the given size
//
// May create the object even if it returns an error - if so
// will return the object and the error, otherwise will return
// nil and the error
func (f *Fs) Put(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error) {
	err := f.implicitCheck(ctx, src.Remote(), true)
	if err != nil {
		return nil, err
	}
	// Barebones object will get filled in Update
	o := &Object{
		fs:      f,
		remote:  src.Remote(),
		fullURL: f.url(src.Remote()),
	}
	// We pass through the Update's error
	err = o.Update(ctx, in, src, options...)
	if err != nil {
		return nil, err
	}
	return o, nil
}

// implicitCheck prevents implicit dir creation by doing mkdir from base up to current dir,
// does NOT check if these dirs created conflict with existing dirs/files so can result in dupe
func (f *Fs) implicitCheck(ctx context.Context, remote string, isfile bool) error {
	// Find base (URL including the CPCODE path) and root (what follows after that)
	URL := f.url(remote)
	u, err := url.Parse(URL)
	if err != nil {
		fs.Errorf(nil, "Unable to parse URL %q while implicit checking directory: %v", URL, err)
		return err
	}
	startPos := 0
	if strings.HasPrefix(u.Path, "/") {
		startPos = 1
	}
	pos := strings.Index(u.Path[startPos:], "/")
	if pos == -1 {
		fs.Errorf(nil, "URL %q unexpectedly does not include the slash in the CPCODE path", URL)
		return nil
	}
	root := rest.URLPathEscape(u.Path[startPos+pos+1:])
	u.Path = u.Path[:startPos+pos]
	base := u.String()
	if !strings.HasSuffix(base, "/") {
		base += "/"
	}

	if isfile {
		// Get the base name of root
		lastindex := strings.LastIndex(root, "/")
		if lastindex == -1 {
			// We are at the level of CPCODE path
			return nil
		}
		root = root[0 : lastindex+1]
	}

	// We make sure root always has "/" at the end
	if !strings.HasSuffix(root, "/") {
		root += "/"
	}

	for root != "" {
		frontindex := strings.Index(root, "/")
		if frontindex == -1 {
			return nil
		}
		frontdir := root[0 : frontindex+1]
		root = root[frontindex+1:]
		base += frontdir
		if !f.testAndSetDirscreated(base) {
			fs.Infof(nil, "Implicitly create directory %s", base)
			err := f.netStorageMkdirRequest(ctx, base)
			if err != nil {
				fs.Errorf("Mkdir request in implicit check failed for base %s: %v", base, err)
				return err
			}
		}
	}
	return nil
}

// Purge all files in the directory specified.
// NetStorage quick-delete is disabled by default AND not instantaneous.
// Returns fs.ErrorCantPurge when quick-delete fails.
func (f *Fs) Purge(ctx context.Context, dir string) error {
	URL := f.url(dir)
	const actionHeader = "version=1&action=quick-delete&quick-delete=imreallyreallysure"
	if _, err := f.callBackend(ctx, URL, "POST", actionHeader, true, nil, nil); err != nil {
		fs.Logf(nil, "Purge using quick-delete failed, fallback on recursive delete: %v", err)
		return fs.ErrorCantPurge
	}
	fs.Logf(nil, "Purge using quick-delete has been queued, you may not see immediate changes")
	return nil
}

// PutStream uploads to the remote path with the modTime given of indeterminate size
func (f *Fs) PutStream(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error) {
	// Pass through error from Put
	return f.Put(ctx, in, src, options...)
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

// Hash returns the Md5sum of an object returning a lowercase hex string
func (o *Object) Hash(ctx context.Context, t hash.Type) (string, error) {
	if t != hash.MD5 {
		return "", hash.ErrUnsupported
	}
	return o.md5sum, nil
}

// Size returns the size in bytes of the remote http file
func (o *Object) Size() int64 {
	return o.size
}

// Md5Sum returns the md5 of the object
func (o *Object) Md5Sum() string {
	return o.md5sum
}

// ModTime returns the modification time of the object
//
// It attempts to read the objects mtime and if that isn't present the
// LastModified returned in the http headers
func (o *Object) ModTime(ctx context.Context) time.Time {
	return time.Unix(o.modTime, 0)
}

// SetModTime sets the modification and access time to the specified time
func (o *Object) SetModTime(ctx context.Context, modTime time.Time) error {
	URL := o.fullURL
	when := strconv.FormatInt(modTime.Unix(), 10)
	actionHeader := "version=1&action=mtime&mtime=" + when
	if _, err := o.fs.callBackend(ctx, URL, "POST", actionHeader, true, nil, nil); err != nil {
		fs.Debugf(nil, "NetStorage action mtime failed for %q: %v", URL, err)
		return err
	}
	o.fs.deleteStatCache(URL)
	o.modTime = modTime.Unix()
	return nil
}

// Storable returns whether this object is storable
func (o *Object) Storable() bool {
	return true
}

// Open an object for read
func (o *Object) Open(ctx context.Context, options ...fs.OpenOption) (in io.ReadCloser, err error) {
	return o.netStorageDownloadRequest(ctx, options)
}

// Hashes returns the supported hash sets.
func (f *Fs) Hashes() hash.Set {
	return hash.Set(hash.MD5)
}

// Mkdir makes the root directory of the Fs object
// Shouldn't return an error if it already exists
func (f *Fs) Mkdir(ctx context.Context, dir string) error {
	// ImplicitCheck will mkdir from base up to dir, if not already in dirscreated
	return f.implicitCheck(ctx, dir, false)
}

// Remove an object
func (o *Object) Remove(ctx context.Context) error {
	return o.netStorageDeleteRequest(ctx)
}

// Rmdir removes the root directory of the Fs object
// Return an error if it doesn't exist or isn't empty
func (f *Fs) Rmdir(ctx context.Context, dir string) error {
	return f.netStorageRmdirRequest(ctx, dir)
}

// Update netstorage with the object
func (o *Object) Update(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) error {
	o.size = src.Size()
	o.modTime = src.ModTime(ctx).Unix()
	// Don't do md5 check because that's done by server
	o.md5sum = ""
	err := o.netStorageUploadRequest(ctx, in, src)
	// We return an object updated with source stats,
	// we don't refetch the obj after upload
	if err != nil {
		return err
	}
	return nil
}

// newObjectWithInfo creates an fs.Object for any netstorage.File or symlink.
// If it can't be found it returns the error fs.ErrorObjectNotFound.
// It returns fs.ErrorIsDir error for directory objects, but still fills the
// fs.Object structure (for directory operations).
func (f *Fs) newObjectWithInfo(remote string, info *File) (fs.Object, error) {
	if info == nil {
		return nil, fs.ErrorObjectNotFound
	}
	URL := f.url(remote)
	size := info.Size
	if info.Type == "symlink" {
		// File size for symlinks is absent but for .rclonelink to work
		// the size should be the length of the target name
		size = int64(len(info.Target))
	}
	o := &Object{
		fs:       f,
		filetype: info.Type,
		remote:   remote,
		size:     size,
		modTime:  info.Mtime,
		md5sum:   info.Md5,
		fullURL:  URL,
		target:   info.Target,
	}
	if info.Type == "dir" {
		return o, fs.ErrorIsDir
	}
	return o, nil
}

// getAuth is the signing hook to get the NetStorage auth
func (f *Fs) getAuth(req *http.Request) error {
	// Set Authorization header
	dataHeader := generateDataHeader(f)
	path := req.URL.RequestURI()
	actionHeader := req.Header["X-Akamai-ACS-Action"][0]
	fs.Debugf(nil, "NetStorage API %s call %s for path %q", req.Method, actionHeader, path)
	req.Header.Set("X-Akamai-ACS-Auth-Data", dataHeader)
	req.Header.Set("X-Akamai-ACS-Auth-Sign", generateSignHeader(f, dataHeader, path, actionHeader))
	return nil
}

// retryErrorCodes is a slice of error codes that we will retry
var retryErrorCodes = []int{
	423, // Locked
	429, // Too Many Requests
	500, // Internal Server Error
	502, // Bad Gateway
	503, // Service Unavailable
	504, // Gateway Timeout
	509, // Bandwidth Limit Exceeded
}

// shouldRetry returns a boolean as to whether this resp and err
// deserve to be retried.  It returns the err as a convenience
func shouldRetry(ctx context.Context, resp *http.Response, err error) (bool, error) {
	if fserrors.ContextError(ctx, &err) {
		return false, err
	}
	return fserrors.ShouldRetry(err) || fserrors.ShouldRetryHTTP(resp, retryErrorCodes), err
}

// callBackend calls NetStorage API using either rest.Call or rest.CallXML function,
// depending on whether the response is required
func (f *Fs) callBackend(ctx context.Context, URL, method, actionHeader string, noResponse bool, response interface{}, options []fs.OpenOption) (io.ReadCloser, error) {
	opts := rest.Opts{
		Method:     method,
		RootURL:    URL,
		NoResponse: noResponse,
		ExtraHeaders: map[string]string{
			"*X-Akamai-ACS-Action": actionHeader,
		},
	}
	if options != nil {
		opts.Options = options
	}

	var resp *http.Response
	err := f.pacer.Call(func() (bool, error) {
		var err error
		if response != nil {
			resp, err = f.srv.CallXML(ctx, &opts, nil, response)
		} else {
			resp, err = f.srv.Call(ctx, &opts)
		}
		return shouldRetry(ctx, resp, err)
	})
	if err != nil {
		if resp != nil && resp.StatusCode == http.StatusNotFound {
			// 404 HTTP code translates into Object not found
			return nil, fs.ErrorObjectNotFound
		}
		return nil, fmt.Errorf("failed to call NetStorage API: %w", err)
	}
	if noResponse {
		return nil, nil
	}
	return resp.Body, nil
}

// netStorageStatRequest performs a NetStorage stat request
func (f *Fs) netStorageStatRequest(ctx context.Context, URL string, directory bool) ([]File, error) {
	if strings.HasSuffix(URL, ".rclonelink") {
		fs.Infof(nil, "Converting rclonelink to a symlink on the stat request %q", URL)
		URL = strings.TrimSuffix(URL, ".rclonelink")
	}
	URL = strings.TrimSuffix(URL, "/")
	files := f.getStatCache(URL)
	if files == nil {
		const actionHeader = "version=1&action=stat&implicit=yes&format=xml&encoding=utf-8&slash=both"
		statResp := &Stat{}
		if _, err := f.callBackend(ctx, URL, "GET", actionHeader, false, statResp, nil); err != nil {
			fs.Debugf(nil, "NetStorage action stat failed for %q: %v", URL, err)
			return nil, err
		}
		files = statResp.Files
		f.setStatCache(URL, files)
	}
	// Multiple objects can be returned with the "slash=both" option,
	// when file/symlink/directory has the same name
	for i := range files {
		if files[i].Type == "symlink" {
			// Add .rclonelink suffix to allow local backend code to convert to a symlink.
			files[i].Name += ".rclonelink"
			fs.Infof(nil, "Converting a symlink to the rclonelink on the stat request %s", files[i].Name)
		}
		entrywanted := (directory && files[i].Type == "dir") ||
			(!directory && files[i].Type != "dir")
		if entrywanted {
			filestamp := files[0]
			files[0] = files[i]
			files[i] = filestamp
		}
	}
	return files, nil
}

// netStorageDirRequest performs a NetStorage dir request
func (f *Fs) netStorageDirRequest(ctx context.Context, dir string, URL string) ([]File, error) {
	const actionHeader = "version=1&action=dir&format=xml&encoding=utf-8"
	statResp := &Stat{}
	if _, err := f.callBackend(ctx, URL, "GET", actionHeader, false, statResp, nil); err != nil {
		if err == fs.ErrorObjectNotFound {
			return nil, fs.ErrorDirNotFound
		}
		fs.Debugf(nil, "NetStorage action dir failed for %q: %v", URL, err)
		return nil, err
	}
	return statResp.Files, nil
}

// netStorageListRequest performs a NetStorage list request
// Second returning parameter is resumeStart string, if not empty the function should be restarted with the adjusted URL to continue the listing.
func (f *Fs) netStorageListRequest(ctx context.Context, URL, endPath string) ([]File, string, error) {
	actionHeader := "version=1&action=list&mtime_all=yes&format=xml&encoding=utf-8"
	if !pathIsOneLevelDeep(endPath) {
		// Add end= to limit the depth to endPath
		escapeEndPath := url.QueryEscape(strings.TrimSuffix(endPath, "/"))
		// The "0" character exists in place of the trailing slash to
		// accommodate ObjectStore directory logic
		end := "&end=" + strings.TrimSuffix(escapeEndPath, "/") + "0"
		actionHeader += end
	}
	listResp := &List{}
	if _, err := f.callBackend(ctx, URL, "GET", actionHeader, false, listResp, nil); err != nil {
		if err == fs.ErrorObjectNotFound {
			// List action is known to return 404 for a valid [CP Code] path with no objects inside.
			// Call stat to find out whether it is an empty directory or path does not exist.
			fs.Debugf(nil, "NetStorage action list returned 404, call stat for %q", URL)
			files, err := f.netStorageStatRequest(ctx, URL, true)
			if err == nil && len(files) > 0 && files[0].Type == "dir" {
				return []File{}, "", nil
			}
		}
		fs.Debugf(nil, "NetStorage action list failed for %q: %v", URL, err)
		return nil, "", err
	}
	return listResp.Files, listResp.Resume.Start, nil
}

// netStorageUploadRequest performs a NetStorage upload request
func (o *Object) netStorageUploadRequest(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) error {
	URL := o.fullURL
	if URL == "" {
		URL = o.fs.url(src.Remote())
	}
	if strings.HasSuffix(URL, ".rclonelink") {
		bits, err := ioutil.ReadAll(in)
		if err != nil {
			return err
		}
		targ := string(bits)
		symlinkloc := strings.TrimSuffix(URL, ".rclonelink")
		fs.Infof(nil, "Converting rclonelink to a symlink on upload %s target %s", symlinkloc, targ)
		_, err = o.fs.netStorageSymlinkRequest(ctx, symlinkloc, targ, &o.modTime)
		return err
	}

	u, err := url.Parse(URL)
	if err != nil {
		return fmt.Errorf("unable to parse URL %q while uploading: %w", URL, err)
	}
	path := u.RequestURI()

	const actionHeader = "version=1&action=upload&sha256=atend&mtime=atend"
	trailers := &http.Header{}
	hr := newHashReader(in, sha256.New())
	reader := customReader(
		func(p []byte) (n int, err error) {
			if n, err = hr.Read(p); err != nil && err == io.EOF {
				// Send the "chunked trailer" after upload of the object
				digest := hex.EncodeToString(hr.Sum(nil))
				actionHeader := "version=1&action=upload&sha256=" + digest +
					"&mtime=" + strconv.FormatInt(src.ModTime(ctx).Unix(), 10)
				trailers.Add("X-Akamai-ACS-Action", actionHeader)
				dataHeader := generateDataHeader(o.fs)
				trailers.Add("X-Akamai-ACS-Auth-Data", dataHeader)
				signHeader := generateSignHeader(o.fs, dataHeader, path, actionHeader)
				trailers.Add("X-Akamai-ACS-Auth-Sign", signHeader)
			}
			return
		},
	)

	var resp *http.Response
	opts := rest.Opts{
		Method:     "PUT",
		RootURL:    URL,
		NoResponse: true,
		Options:    options,
		Body:       reader,
		Trailer:    trailers,
		ExtraHeaders: map[string]string{
			"*X-Akamai-ACS-Action": actionHeader,
		},
	}
	err = o.fs.pacer.CallNoRetry(func() (bool, error) {
		resp, err = o.fs.srv.Call(ctx, &opts)
		return shouldRetry(ctx, resp, err)
	})
	if err != nil {
		if resp != nil && resp.StatusCode == http.StatusNotFound {
			// 404 HTTP code translates into Object not found
			return fs.ErrorObjectNotFound
		}
		fs.Debugf(nil, "NetStorage action upload failed for %q: %v", URL, err)
		// Remove failed upload
		_ = o.Remove(ctx)
		return fmt.Errorf("failed to call NetStorage API upload: %w", err)
	}

	// Invalidate stat cache
	o.fs.deleteStatCache(URL)
	if o.size == -1 {
		files, err := o.fs.netStorageStatRequest(ctx, URL, false)
		if err != nil {
			return nil
		}
		if files != nil {
			o.size = files[0].Size
		}
	}
	return nil
}

// netStorageDownloadRequest performs a NetStorage download request
func (o *Object) netStorageDownloadRequest(ctx context.Context, options []fs.OpenOption) (in io.ReadCloser, err error) {
	URL := o.fullURL
	// If requested file ends with .rclonelink and target has value
	// then serve the content of target (the symlink target)
	if strings.HasSuffix(URL, ".rclonelink") && o.target != "" {
		fs.Infof(nil, "Converting a symlink to the rclonelink file on download %q", URL)
		reader := strings.NewReader(o.target)
		readcloser := ioutil.NopCloser(reader)
		return readcloser, nil
	}

	const actionHeader = "version=1&action=download"
	fs.FixRangeOption(options, o.size)
	body, err := o.fs.callBackend(ctx, URL, "GET", actionHeader, false, nil, options)
	if err != nil {
		fs.Debugf(nil, "NetStorage action download failed for %q: %v", URL, err)
		return nil, err
	}
	return body, nil
}

// netStorageDuRequest performs a NetStorage du request
func (f *Fs) netStorageDuRequest(ctx context.Context) (interface{}, error) {
	URL := f.url("")
	const actionHeader = "version=1&action=du&format=xml&encoding=utf-8"
	duResp := &Du{}
	if _, err := f.callBackend(ctx, URL, "GET", actionHeader, false, duResp, nil); err != nil {
		if err == fs.ErrorObjectNotFound {
			return nil, errors.New("NetStorage du command: target is not a directory or does not exist")
		}
		fs.Debugf(nil, "NetStorage action du failed for %q: %v", URL, err)
		return nil, err
	}
	//passing the output format expected from return of Command to be displayed by rclone code
	out := map[string]int64{
		"Number of files": duResp.Duinfo.Files,
		"Total bytes":     duResp.Duinfo.Bytes,
	}
	return out, nil
}

// netStorageDuRequest performs a NetStorage symlink request
func (f *Fs) netStorageSymlinkRequest(ctx context.Context, URL string, dst string, modTime *int64) (interface{}, error) {
	target := url.QueryEscape(strings.TrimSuffix(dst, "/"))
	actionHeader := "version=1&action=symlink&target=" + target
	if modTime != nil {
		when := strconv.FormatInt(*modTime, 10)
		actionHeader += "&mtime=" + when
	}
	if _, err := f.callBackend(ctx, URL, "POST", actionHeader, true, nil, nil); err != nil {
		fs.Debugf(nil, "NetStorage action symlink failed for %q: %v", URL, err)
		return nil, fmt.Errorf("symlink creation failed: %w", err)
	}
	f.deleteStatCache(URL)
	out := map[string]string{
		"Symlink successfully created": dst,
	}
	return out, nil
}

// netStorageMkdirRequest performs a NetStorage mkdir request
func (f *Fs) netStorageMkdirRequest(ctx context.Context, URL string) error {
	const actionHeader = "version=1&action=mkdir"
	if _, err := f.callBackend(ctx, URL, "POST", actionHeader, true, nil, nil); err != nil {
		fs.Debugf(nil, "NetStorage action mkdir failed for %q: %v", URL, err)
		return err
	}
	f.deleteStatCache(URL)
	return nil
}

// netStorageDeleteRequest performs a NetStorage delete request
func (o *Object) netStorageDeleteRequest(ctx context.Context) error {
	URL := o.fullURL
	// We shouldn't be creating .rclonelink files on remote
	// but delete corresponding symlink if it exists
	if strings.HasSuffix(URL, ".rclonelink") {
		fs.Infof(nil, "Converting rclonelink to a symlink on delete %q", URL)
		URL = strings.TrimSuffix(URL, ".rclonelink")
	}

	const actionHeader = "version=1&action=delete"
	if _, err := o.fs.callBackend(ctx, URL, "POST", actionHeader, true, nil, nil); err != nil {
		fs.Debugf(nil, "NetStorage action delete failed for %q: %v", URL, err)
		return err
	}
	o.fs.deleteStatCache(URL)
	return nil
}

// netStorageRmdirRequest performs a NetStorage rmdir request
func (f *Fs) netStorageRmdirRequest(ctx context.Context, dir string) error {
	URL := f.url(dir)
	const actionHeader = "version=1&action=rmdir"
	if _, err := f.callBackend(ctx, URL, "POST", actionHeader, true, nil, nil); err != nil {
		if err == fs.ErrorObjectNotFound {
			return fs.ErrorDirNotFound
		}
		fs.Debugf(nil, "NetStorage action rmdir failed for %q: %v", URL, err)
		return err
	}
	f.deleteStatCache(URL)
	f.deleteDirscreated(URL)
	return nil
}

// deleteDirscreated deletes URL from dirscreated map thread-safely
func (f *Fs) deleteDirscreated(URL string) {
	URL = strings.TrimSuffix(URL, "/")
	f.dirscreatedMutex.Lock()
	delete(f.dirscreated, URL)
	f.dirscreatedMutex.Unlock()
}

// setDirscreated sets to true URL in dirscreated map thread-safely
func (f *Fs) setDirscreated(URL string) {
	URL = strings.TrimSuffix(URL, "/")
	f.dirscreatedMutex.Lock()
	f.dirscreated[URL] = true
	f.dirscreatedMutex.Unlock()
}

// testAndSetDirscreated atomic test-and-set to true URL in dirscreated map,
// returns the previous value
func (f *Fs) testAndSetDirscreated(URL string) bool {
	URL = strings.TrimSuffix(URL, "/")
	f.dirscreatedMutex.Lock()
	oldValue := f.dirscreated[URL]
	f.dirscreated[URL] = true
	f.dirscreatedMutex.Unlock()
	return oldValue
}

// deleteStatCache deletes URL from stat cache thread-safely
func (f *Fs) deleteStatCache(URL string) {
	URL = strings.TrimSuffix(URL, "/")
	f.statcacheMutex.Lock()
	delete(f.statcache, URL)
	f.statcacheMutex.Unlock()
}

// getStatCache gets value from statcache map thread-safely
func (f *Fs) getStatCache(URL string) (files []File) {
	URL = strings.TrimSuffix(URL, "/")
	f.statcacheMutex.RLock()
	files = f.statcache[URL]
	f.statcacheMutex.RUnlock()
	if files != nil {
		fs.Debugf(nil, "NetStorage stat cache hit for %q", URL)
	}
	return
}

// setStatCache sets value to statcache map thread-safely
func (f *Fs) setStatCache(URL string, files []File) {
	URL = strings.TrimSuffix(URL, "/")
	f.statcacheMutex.Lock()
	f.statcache[URL] = files
	f.statcacheMutex.Unlock()
}

type hashReader struct {
	io.Reader
	gohash.Hash
}

func newHashReader(r io.Reader, h gohash.Hash) hashReader {
	return hashReader{io.TeeReader(r, h), h}
}

type customReader func([]byte) (int, error)

func (c customReader) Read(p []byte) (n int, err error) {
	return c(p)
}

// generateRequestID generates the unique requestID
func generateRequestID() int64 {
	s1 := rand.NewSource(time.Now().UnixNano())
	r1 := rand.New(s1)
	return r1.Int63()
}

// computeHmac256 calculates the hash for the sign header
func computeHmac256(message string, secret string) string {
	key := []byte(secret)
	h := hmac.New(sha256.New, key)
	h.Write([]byte(message))
	return base64.StdEncoding.EncodeToString(h.Sum(nil))
}

// getEpochTimeInSeconds returns current epoch time in seconds
func getEpochTimeInSeconds() int64 {
	now := time.Now()
	secs := now.Unix()
	return secs
}

// generateDataHeader generates data header needed for making the request
func generateDataHeader(f *Fs) string {
	return "5, 0.0.0.0, 0.0.0.0, " + strconv.FormatInt(getEpochTimeInSeconds(), 10) + ", " + strconv.FormatInt(generateRequestID(), 10) + "," + f.opt.Account
}

// generateSignHeader generates sign header needed for making the request
func generateSignHeader(f *Fs, dataHeader string, path string, actionHeader string) string {
	var message = dataHeader + path + "\nx-akamai-acs-action:" + actionHeader + "\n"
	return computeHmac256(message, f.opt.Secret)
}

// pathIsOneLevelDeep returns true if a given path does not go deeper than one level
func pathIsOneLevelDeep(path string) bool {
	return !strings.Contains(strings.TrimSuffix(strings.TrimPrefix(path, "/"), "/"), "/")
}

// Check the interfaces are satisfied
var (
	_ fs.Fs          = &Fs{}
	_ fs.Purger      = &Fs{}
	_ fs.PutStreamer = &Fs{}
	_ fs.ListRer     = &Fs{}
	_ fs.Object      = &Object{}
)
