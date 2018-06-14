// Package webdav provides an interface to the Webdav
// object storage system.
package webdav

// Owncloud: Getting Oc-Checksum:
// SHA1:f572d396fae9206628714fb2ce00f72e94f2258f on HEAD but not on
// nextcloud?

// docs for file webdav
// https://docs.nextcloud.com/server/12/developer_manual/client_apis/WebDAV/index.html

// indicates checksums can be set as metadata here
// https://github.com/nextcloud/server/issues/6129
// owncloud seems to have checksums as metadata though - can read them

// SetModTime might be possible
// https://stackoverflow.com/questions/3579608/webdav-can-a-client-modify-the-mtime-of-a-file
// ...support for a PROPSET to lastmodified (mind the missing get) which does the utime() call might be an option.
// For example the ownCloud WebDAV server does it that way.

import (
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"

	"github.com/ncw/rclone/backend/webdav/api"
	"github.com/ncw/rclone/backend/webdav/odrvcookie"
	"github.com/ncw/rclone/fs"
	"github.com/ncw/rclone/fs/config"
	"github.com/ncw/rclone/fs/config/obscure"
	"github.com/ncw/rclone/fs/fserrors"
	"github.com/ncw/rclone/fs/fshttp"
	"github.com/ncw/rclone/fs/hash"
	"github.com/ncw/rclone/lib/pacer"
	"github.com/ncw/rclone/lib/rest"
	"github.com/pkg/errors"
)

const (
	minSleep      = 10 * time.Millisecond
	maxSleep      = 2 * time.Second
	decayConstant = 2 // bigger for slower decay, exponential
)

// Register with Fs
func init() {
	fs.Register(&fs.RegInfo{
		Name:        "webdav",
		Description: "Webdav",
		NewFs:       NewFs,
		Options: []fs.Option{{
			Name:     "url",
			Help:     "URL of http host to connect to",
			Optional: false,
			Examples: []fs.OptionExample{{
				Value: "https://example.com",
				Help:  "Connect to example.com",
			}},
		}, {
			Name:     "vendor",
			Help:     "Name of the Webdav site/service/software you are using",
			Optional: false,
			Examples: []fs.OptionExample{{
				Value: "nextcloud",
				Help:  "Nextcloud",
			}, {
				Value: "owncloud",
				Help:  "Owncloud",
			}, {
				Value: "sharepoint",
				Help:  "Sharepoint",
			}, {
				Value: "other",
				Help:  "Other site/service or software",
			}},
		}, {
			Name:     "user",
			Help:     "User name",
			Optional: true,
		}, {
			Name:       "pass",
			Help:       "Password.",
			Optional:   true,
			IsPassword: true,
		}},
	})
}

// Fs represents a remote webdav
type Fs struct {
	name        string        // name of this remote
	root        string        // the path we are working on
	features    *fs.Features  // optional features
	endpoint    *url.URL      // URL of the host
	endpointURL string        // endpoint as a string
	srv         *rest.Client  // the connection to the one drive server
	pacer       *pacer.Pacer  // pacer for API calls
	user        string        // username
	pass        string        // password
	vendor      string        // name of the vendor
	precision   time.Duration // mod time precision
	canStream   bool          // set if can stream
	useOCMtime  bool          // set if can use X-OC-Mtime
}

// Object describes a webdav object
//
// Will definitely have info but maybe not meta
type Object struct {
	fs          *Fs       // what this object is part of
	remote      string    // The remote path
	hasMetaData bool      // whether info below has been set
	size        int64     // size of the object
	modTime     time.Time // modification time of the object
	sha1        string    // SHA-1 of the object content
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
	return fmt.Sprintf("webdav root '%s'", f.root)
}

// Features returns the optional features of this Fs
func (f *Fs) Features() *fs.Features {
	return f.features
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
// deserve to be retried.  It returns the err as a convenience
func shouldRetry(resp *http.Response, err error) (bool, error) {
	return fserrors.ShouldRetry(err) || fserrors.ShouldRetryHTTP(resp, retryErrorCodes), err
}

// itemIsDir returns true if the item is a directory
//
// When a client sees a resourcetype it doesn't recognize it should
// assume it is a regular non-collection resource.  [WebDav book by
// Lisa Dusseault ch 7.5.8 p170]
func itemIsDir(item *api.Response) bool {
	if t := item.Props.Type; t != nil {
		if t.Space == "DAV:" && t.Local == "collection" {
			return true
		}
		fs.Debugf(nil, "Unknown resource type %q/%q on %q", t.Space, t.Local, item.Props.Name)
	}
	return false
}

// readMetaDataForPath reads the metadata from the path
func (f *Fs) readMetaDataForPath(path string) (info *api.Prop, err error) {
	// FIXME how do we read back additional properties?
	opts := rest.Opts{
		Method: "PROPFIND",
		Path:   f.filePath(path),
		ExtraHeaders: map[string]string{
			"Depth": "1",
		},
	}
	var result api.Multistatus
	var resp *http.Response
	err = f.pacer.Call(func() (bool, error) {
		resp, err = f.srv.CallXML(&opts, nil, &result)
		return shouldRetry(resp, err)
	})
	if apiErr, ok := err.(*api.Error); ok {
		// does not exist
		if apiErr.StatusCode == http.StatusNotFound {
			return nil, fs.ErrorObjectNotFound
		}
	}
	if err != nil {
		return nil, errors.Wrap(err, "read metadata failed")
	}
	if len(result.Responses) < 1 {
		return nil, fs.ErrorObjectNotFound
	}
	item := result.Responses[0]
	if !item.Props.StatusOK() {
		return nil, fs.ErrorObjectNotFound
	}
	if itemIsDir(&item) {
		return nil, fs.ErrorNotAFile
	}
	return &item.Props, nil
}

// errorHandler parses a non 2xx error response into an error
func errorHandler(resp *http.Response) error {
	body, err := rest.ReadBody(resp)
	if err != nil {
		return errors.Wrap(err, "error when trying to read error from body")
	}
	// Decode error response
	errResponse := new(api.Error)
	err = xml.Unmarshal(body, &errResponse)
	if err != nil {
		// set the Message to be the body if can't parse the XML
		errResponse.Message = strings.TrimSpace(string(body))
	}
	errResponse.Status = resp.Status
	errResponse.StatusCode = resp.StatusCode
	return errResponse
}

// addShlash makes sure s is terminated with a / if non empty
func addSlash(s string) string {
	if s != "" && !strings.HasSuffix(s, "/") {
		s += "/"
	}
	return s
}

// filePath returns a file path (f.root, file)
func (f *Fs) filePath(file string) string {
	return rest.URLPathEscape(path.Join(f.root, file))
}

// dirPath returns a directory path (f.root, dir)
func (f *Fs) dirPath(dir string) string {
	return addSlash(f.filePath(dir))
}

// filePath returns a file path (f.root, remote)
func (o *Object) filePath() string {
	return o.fs.filePath(o.remote)
}

// NewFs constructs an Fs from the path, container:path
func NewFs(name, root string) (fs.Fs, error) {
	endpoint := config.FileGet(name, "url")
	if !strings.HasSuffix(endpoint, "/") {
		endpoint += "/"
	}
	root = strings.Trim(root, "/")

	user := config.FileGet(name, "user")
	pass := config.FileGet(name, "pass")
	if pass != "" {
		var err error
		pass, err = obscure.Reveal(pass)
		if err != nil {
			return nil, errors.Wrap(err, "couldn't decrypt password")
		}
	}
	vendor := config.FileGet(name, "vendor")

	// Parse the endpoint
	u, err := url.Parse(endpoint)
	if err != nil {
		return nil, err
	}

	f := &Fs{
		name:        name,
		root:        root,
		endpoint:    u,
		endpointURL: u.String(),
		srv:         rest.NewClient(fshttp.NewClient(fs.Config)).SetRoot(u.String()).SetUserPass(user, pass),
		pacer:       pacer.New().SetMinSleep(minSleep).SetMaxSleep(maxSleep).SetDecayConstant(decayConstant),
		user:        user,
		pass:        pass,
		precision:   fs.ModTimeNotSupported,
	}
	f.features = (&fs.Features{
		CanHaveEmptyDirectories: true,
	}).Fill(f)
	f.srv.SetErrorHandler(errorHandler)
	err = f.setQuirks(vendor)
	if err != nil {
		return nil, err
	}

	if root != "" {
		// Check to see if the root actually an existing file
		remote := path.Base(root)
		f.root = path.Dir(root)
		if f.root == "." {
			f.root = ""
		}
		_, err := f.NewObject(remote)
		if err != nil {
			if errors.Cause(err) == fs.ErrorObjectNotFound || errors.Cause(err) == fs.ErrorNotAFile {
				// File doesn't exist so return old f
				f.root = root
				return f, nil
			}
			return nil, err
		}
		// return an error with an fs which points to the parent
		return f, fs.ErrorIsFile
	}
	return f, nil
}

// setQuirks adjusts the Fs for the vendor passed in
func (f *Fs) setQuirks(vendor string) error {
	if vendor == "" {
		vendor = "other"
	}
	f.vendor = vendor
	switch vendor {
	case "owncloud":
		f.canStream = true
		f.precision = time.Second
		f.useOCMtime = true
	case "nextcloud":
		f.precision = time.Second
		f.useOCMtime = true
	case "sharepoint":
		// To mount sharepoint, two Cookies are required
		// They have to be set instead of BasicAuth
		f.srv.RemoveHeader("Authorization") // We don't need this Header if using cookies
		spCk := odrvcookie.New(f.user, f.pass, f.endpointURL)
		spCookies, err := spCk.Cookies()
		if err != nil {
			return err
		}
		f.srv.SetCookie(&spCookies.FedAuth, &spCookies.RtFa)
	case "other":
	default:
		fs.Debugf(f, "Unknown vendor %q", vendor)
	}

	// Remove PutStream from optional features
	if !f.canStream {
		f.features.PutStream = nil
	}
	return nil
}

// Return an Object from a path
//
// If it can't be found it returns the error fs.ErrorObjectNotFound.
func (f *Fs) newObjectWithInfo(remote string, info *api.Prop) (fs.Object, error) {
	o := &Object{
		fs:     f,
		remote: remote,
	}
	var err error
	if info != nil {
		// Set info
		err = o.setMetaData(info)
	} else {
		err = o.readMetaData() // reads info and meta, returning an error
	}
	if err != nil {
		return nil, err
	}
	return o, nil
}

// NewObject finds the Object at remote.  If it can't be found
// it returns the error fs.ErrorObjectNotFound.
func (f *Fs) NewObject(remote string) (fs.Object, error) {
	return f.newObjectWithInfo(remote, nil)
}

// list the objects into the function supplied
//
// If directories is set it only sends directories
// User function to process a File item from listAll
//
// Should return true to finish processing
type listAllFn func(string, bool, *api.Prop) bool

// Lists the directory required calling the user function on each item found
//
// If the user fn ever returns true then it early exits with found = true
func (f *Fs) listAll(dir string, directoriesOnly bool, filesOnly bool, fn listAllFn) (found bool, err error) {
	opts := rest.Opts{
		Method: "PROPFIND",
		Path:   f.dirPath(dir), // FIXME Should not start with /
		ExtraHeaders: map[string]string{
			"Depth": "1",
		},
	}
	var result api.Multistatus
	var resp *http.Response
	err = f.pacer.Call(func() (bool, error) {
		resp, err = f.srv.CallXML(&opts, nil, &result)
		return shouldRetry(resp, err)
	})
	if err != nil {
		if apiErr, ok := err.(*api.Error); ok {
			// does not exist
			if apiErr.StatusCode == http.StatusNotFound {
				return found, fs.ErrorDirNotFound
			}
		}
		return found, errors.Wrap(err, "couldn't list files")
	}
	//fmt.Printf("result = %#v", &result)
	baseURL, err := rest.URLJoin(f.endpoint, opts.Path)
	if err != nil {
		return false, errors.Wrap(err, "couldn't join URL")
	}
	for i := range result.Responses {
		item := &result.Responses[i]
		isDir := itemIsDir(item)

		// Find name
		u, err := rest.URLJoin(baseURL, item.Href)
		if err != nil {
			fs.Errorf(nil, "URL Join failed for %q and %q: %v", baseURL, item.Href, err)
			continue
		}
		// Make sure directories end with a /
		if isDir {
			u.Path = addSlash(u.Path)
		}
		if !strings.HasPrefix(u.Path, baseURL.Path) {
			fs.Debugf(nil, "Item with unknown path received: %q, %q", u.Path, baseURL.Path)
			continue
		}
		remote := path.Join(dir, u.Path[len(baseURL.Path):])
		if strings.HasSuffix(remote, "/") {
			remote = remote[:len(remote)-1]
		}

		// the listing contains info about itself which we ignore
		if remote == dir {
			continue
		}

		// Check OK
		if !item.Props.StatusOK() {
			fs.Debugf(remote, "Ignoring item with bad status %q", item.Props.Status)
			continue
		}

		if isDir {
			if filesOnly {
				continue
			}
		} else {
			if directoriesOnly {
				continue
			}
		}
		// 	item.Name = restoreReservedChars(item.Name)
		if fn(remote, isDir, &item.Props) {
			found = true
			break
		}
	}
	return
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
	var iErr error
	_, err = f.listAll(dir, false, false, func(remote string, isDir bool, info *api.Prop) bool {
		if isDir {
			d := fs.NewDir(remote, time.Time(info.Modified))
			// .SetID(info.ID)
			// FIXME more info from dir? can set size, items?
			entries = append(entries, d)
		} else {
			o, err := f.newObjectWithInfo(remote, info)
			if err != nil {
				iErr = err
				return true
			}
			entries = append(entries, o)
		}
		return false
	})
	if err != nil {
		return nil, err
	}
	if iErr != nil {
		return nil, iErr
	}
	return entries, nil
}

// Creates from the parameters passed in a half finished Object which
// must have setMetaData called on it
//
// Used to create new objects
func (f *Fs) createObject(remote string, modTime time.Time, size int64) (o *Object) {
	// Temporary Object under construction
	o = &Object{
		fs:      f,
		remote:  remote,
		size:    size,
		modTime: modTime,
	}
	return o
}

// Put the object
//
// Copy the reader in to the new object which is returned
//
// The new object may have been created if an error is returned
func (f *Fs) Put(in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error) {
	o := f.createObject(src.Remote(), src.ModTime(), src.Size())
	return o, o.Update(in, src, options...)
}

// PutStream uploads to the remote path with the modTime given of indeterminate size
func (f *Fs) PutStream(in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error) {
	return f.Put(in, src, options...)
}

// mkParentDir makes the parent of the native path dirPath if
// necessary and any directories above that
func (f *Fs) mkParentDir(dirPath string) error {
	// defer log.Trace(dirPath, "")("")
	// chop off trailing / if it exists
	if strings.HasSuffix(dirPath, "/") {
		dirPath = dirPath[:len(dirPath)-1]
	}
	parent := path.Dir(dirPath)
	if parent == "." {
		parent = ""
	}
	return f.mkdir(parent)
}

// mkdir makes the directory and parents using native paths
func (f *Fs) mkdir(dirPath string) error {
	// defer log.Trace(dirPath, "")("")
	// We assume the root is already ceated
	if dirPath == "" {
		return nil
	}
	// Collections must end with /
	if !strings.HasSuffix(dirPath, "/") {
		dirPath += "/"
	}
	opts := rest.Opts{
		Method:     "MKCOL",
		Path:       dirPath,
		NoResponse: true,
	}
	err := f.pacer.Call(func() (bool, error) {
		resp, err := f.srv.Call(&opts)
		return shouldRetry(resp, err)
	})
	if apiErr, ok := err.(*api.Error); ok {
		// already exists
		if apiErr.StatusCode == http.StatusMethodNotAllowed || apiErr.StatusCode == http.StatusNotAcceptable {
			return nil
		}
		// parent does not exists
		if apiErr.StatusCode == http.StatusConflict {
			err = f.mkParentDir(dirPath)
			if err == nil {
				err = f.mkdir(dirPath)
			}
		}
	}
	return err
}

// Mkdir creates the directory if it doesn't exist
func (f *Fs) Mkdir(dir string) error {
	dirPath := f.dirPath(dir)
	return f.mkdir(dirPath)
}

// dirNotEmpty returns true if the directory exists and is not Empty
//
// if the directory does not exist then err will be ErrorDirNotFound
func (f *Fs) dirNotEmpty(dir string) (found bool, err error) {
	return f.listAll(dir, false, false, func(remote string, isDir bool, info *api.Prop) bool {
		return true
	})
}

// purgeCheck removes the root directory, if check is set then it
// refuses to do so if it has anything in
func (f *Fs) purgeCheck(dir string, check bool) error {
	if check {
		notEmpty, err := f.dirNotEmpty(dir)
		if err != nil {
			return err
		}
		if notEmpty {
			return fs.ErrorDirectoryNotEmpty
		}
	}
	opts := rest.Opts{
		Method:     "DELETE",
		Path:       f.dirPath(dir),
		NoResponse: true,
	}
	var resp *http.Response
	var err error
	err = f.pacer.Call(func() (bool, error) {
		resp, err = f.srv.CallXML(&opts, nil, nil)
		return shouldRetry(resp, err)
	})
	if err != nil {
		return errors.Wrap(err, "rmdir failed")
	}
	// FIXME parse Multistatus response
	return nil
}

// Rmdir deletes the root folder
//
// Returns an error if it isn't empty
func (f *Fs) Rmdir(dir string) error {
	return f.purgeCheck(dir, true)
}

// Precision return the precision of this Fs
func (f *Fs) Precision() time.Duration {
	return f.precision
}

// Copy or Move src to this remote using server side copy operations.
//
// This is stored with the remote path given
//
// It returns the destination Object and a possible error
//
// Will only be called if src.Fs().Name() == f.Name()
//
// If it isn't possible then return fs.ErrorCantCopy/fs.ErrorCantMove
func (f *Fs) copyOrMove(src fs.Object, remote string, method string) (fs.Object, error) {
	srcObj, ok := src.(*Object)
	if !ok {
		fs.Debugf(src, "Can't copy - not same remote type")
		if method == "COPY" {
			return nil, fs.ErrorCantCopy
		}
		return nil, fs.ErrorCantMove
	}
	dstPath := f.filePath(remote)
	err := f.mkParentDir(dstPath)
	if err != nil {
		return nil, errors.Wrap(err, "Copy mkParentDir failed")
	}
	destinationURL, err := rest.URLJoin(f.endpoint, dstPath)
	if err != nil {
		return nil, errors.Wrap(err, "copyOrMove couldn't join URL")
	}
	var resp *http.Response
	opts := rest.Opts{
		Method:     method,
		Path:       srcObj.filePath(),
		NoResponse: true,
		ExtraHeaders: map[string]string{
			"Destination": destinationURL.String(),
			"Overwrite":   "F",
		},
	}
	if f.useOCMtime {
		opts.ExtraHeaders["X-OC-Mtime"] = fmt.Sprintf("%f", float64(src.ModTime().UnixNano())/1E9)
	}
	err = f.pacer.Call(func() (bool, error) {
		resp, err = f.srv.Call(&opts)
		return shouldRetry(resp, err)
	})
	if err != nil {
		return nil, errors.Wrap(err, "Copy call failed")
	}
	dstObj, err := f.NewObject(remote)
	if err != nil {
		return nil, errors.Wrap(err, "Copy NewObject failed")
	}
	return dstObj, nil
}

// Copy src to this remote using server side copy operations.
//
// This is stored with the remote path given
//
// It returns the destination Object and a possible error
//
// Will only be called if src.Fs().Name() == f.Name()
//
// If it isn't possible then return fs.ErrorCantCopy
func (f *Fs) Copy(src fs.Object, remote string) (fs.Object, error) {
	return f.copyOrMove(src, remote, "COPY")
}

// Purge deletes all the files and the container
//
// Optional interface: Only implement this if you have a way of
// deleting all the files quicker than just running Remove() on the
// result of List()
func (f *Fs) Purge() error {
	return f.purgeCheck("", false)
}

// Move src to this remote using server side move operations.
//
// This is stored with the remote path given
//
// It returns the destination Object and a possible error
//
// Will only be called if src.Fs().Name() == f.Name()
//
// If it isn't possible then return fs.ErrorCantMove
func (f *Fs) Move(src fs.Object, remote string) (fs.Object, error) {
	return f.copyOrMove(src, remote, "MOVE")
}

// DirMove moves src, srcRemote to this remote at dstRemote
// using server side move operations.
//
// Will only be called if src.Fs().Name() == f.Name()
//
// If it isn't possible then return fs.ErrorCantDirMove
//
// If destination exists then return fs.ErrorDirExists
func (f *Fs) DirMove(src fs.Fs, srcRemote, dstRemote string) error {
	srcFs, ok := src.(*Fs)
	if !ok {
		fs.Debugf(srcFs, "Can't move directory - not same remote type")
		return fs.ErrorCantDirMove
	}
	srcPath := srcFs.filePath(srcRemote)
	dstPath := f.filePath(dstRemote)

	// Check if destination exists
	_, err := f.dirNotEmpty(dstRemote)
	if err == nil {
		return fs.ErrorDirExists
	}
	if err != fs.ErrorDirNotFound {
		return errors.Wrap(err, "DirMove dirExists dst failed")
	}

	// Make sure the parent directory exists
	err = f.mkParentDir(dstPath)
	if err != nil {
		return errors.Wrap(err, "DirMove mkParentDir dst failed")
	}

	destinationURL, err := rest.URLJoin(f.endpoint, dstPath)
	if err != nil {
		return errors.Wrap(err, "DirMove couldn't join URL")
	}

	var resp *http.Response
	opts := rest.Opts{
		Method:     "MOVE",
		Path:       addSlash(srcPath),
		NoResponse: true,
		ExtraHeaders: map[string]string{
			"Destination": addSlash(destinationURL.String()),
			"Overwrite":   "F",
		},
	}
	err = f.pacer.Call(func() (bool, error) {
		resp, err = f.srv.Call(&opts)
		return shouldRetry(resp, err)
	})
	if err != nil {
		return errors.Wrap(err, "DirMove MOVE call failed")
	}
	return nil
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

// Hash returns the SHA-1 of an object returning a lowercase hex string
func (o *Object) Hash(t hash.Type) (string, error) {
	if t != hash.SHA1 {
		return "", hash.ErrUnsupported
	}
	return o.sha1, nil
}

// Size returns the size of an object in bytes
func (o *Object) Size() int64 {
	err := o.readMetaData()
	if err != nil {
		fs.Logf(o, "Failed to read metadata: %v", err)
		return 0
	}
	return o.size
}

// setMetaData sets the metadata from info
func (o *Object) setMetaData(info *api.Prop) (err error) {
	o.hasMetaData = true
	o.size = info.Size
	o.modTime = time.Time(info.Modified)
	return nil
}

// readMetaData gets the metadata if it hasn't already been fetched
//
// it also sets the info
func (o *Object) readMetaData() (err error) {
	if o.hasMetaData {
		return nil
	}
	info, err := o.fs.readMetaDataForPath(o.remote)
	if err != nil {
		return err
	}
	return o.setMetaData(info)
}

// ModTime returns the modification time of the object
//
// It attempts to read the objects mtime and if that isn't present the
// LastModified returned in the http headers
func (o *Object) ModTime() time.Time {
	err := o.readMetaData()
	if err != nil {
		fs.Logf(o, "Failed to read metadata: %v", err)
		return time.Now()
	}
	return o.modTime
}

// SetModTime sets the modification time of the local fs object
func (o *Object) SetModTime(modTime time.Time) error {
	return fs.ErrorCantSetModTime
}

// Storable returns a boolean showing whether this object storable
func (o *Object) Storable() bool {
	return true
}

// Open an object for read
func (o *Object) Open(options ...fs.OpenOption) (in io.ReadCloser, err error) {
	var resp *http.Response
	opts := rest.Opts{
		Method:  "GET",
		Path:    o.filePath(),
		Options: options,
	}
	err = o.fs.pacer.Call(func() (bool, error) {
		resp, err = o.fs.srv.Call(&opts)
		return shouldRetry(resp, err)
	})
	if err != nil {
		return nil, err
	}
	return resp.Body, err
}

// Update the object with the contents of the io.Reader, modTime and size
//
// If existing is set then it updates the object rather than creating a new one
//
// The new object may have been created if an error is returned
func (o *Object) Update(in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (err error) {
	err = o.fs.mkParentDir(o.filePath())
	if err != nil {
		return errors.Wrap(err, "Update mkParentDir failed")
	}

	size := src.Size()
	var resp *http.Response
	opts := rest.Opts{
		Method:        "PUT",
		Path:          o.filePath(),
		Body:          in,
		NoResponse:    true,
		ContentLength: &size, // FIXME this isn't necessary with owncloud - See https://github.com/nextcloud/nextcloud-snap/issues/365
	}
	if o.fs.useOCMtime {
		opts.ExtraHeaders = map[string]string{
			"X-OC-Mtime": fmt.Sprintf("%f", float64(src.ModTime().UnixNano())/1E9),
		}
	}
	err = o.fs.pacer.CallNoRetry(func() (bool, error) {
		resp, err = o.fs.srv.Call(&opts)
		return shouldRetry(resp, err)
	})
	if err != nil {
		return err
	}
	// read metadata from remote
	o.hasMetaData = false
	return o.readMetaData()
}

// Remove an object
func (o *Object) Remove() error {
	opts := rest.Opts{
		Method:     "DELETE",
		Path:       o.filePath(),
		NoResponse: true,
	}
	return o.fs.pacer.Call(func() (bool, error) {
		resp, err := o.fs.srv.Call(&opts)
		return shouldRetry(resp, err)
	})
}

// Check the interfaces are satisfied
var (
	_ fs.Fs          = (*Fs)(nil)
	_ fs.Purger      = (*Fs)(nil)
	_ fs.PutStreamer = (*Fs)(nil)
	_ fs.Copier      = (*Fs)(nil)
	_ fs.Mover       = (*Fs)(nil)
	_ fs.DirMover    = (*Fs)(nil)
	_ fs.Object      = (*Object)(nil)
)
