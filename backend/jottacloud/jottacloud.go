package jottacloud

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/ncw/rclone/backend/jottacloud/api"
	"github.com/ncw/rclone/fs"
	"github.com/ncw/rclone/fs/config"
	"github.com/ncw/rclone/fs/config/configmap"
	"github.com/ncw/rclone/fs/config/configstruct"
	"github.com/ncw/rclone/fs/config/obscure"
	"github.com/ncw/rclone/fs/fserrors"
	"github.com/ncw/rclone/fs/fshttp"
	"github.com/ncw/rclone/fs/hash"
	"github.com/ncw/rclone/lib/pacer"
	"github.com/ncw/rclone/lib/rest"
	"github.com/pkg/errors"
)

// Globals
const (
	minSleep          = 10 * time.Millisecond
	maxSleep          = 2 * time.Second
	decayConstant     = 2 // bigger for slower decay, exponential
	defaultDevice     = "Jotta"
	defaultMountpoint = "Sync"
	rootURL           = "https://www.jottacloud.com/jfs/"
	//newApiRootUrl   = "https://api.jottacloud.com"
	//newUploadUrl	  = "https://up-no-001.jottacloud.com"
)

// Register with Fs
func init() {
	fs.Register(&fs.RegInfo{
		Name:        "jottacloud",
		Description: "JottaCloud",
		NewFs:       NewFs,
		Options: []fs.Option{{
			Name: "user",
			Help: "User Name",
		}, {
			Name:       "pass",
			Help:       "Password.",
			IsPassword: true,
		}, {
			Name:     "mountpoint",
			Help:     "The mountpoint to use.",
			Required: true,
			Examples: []fs.OptionExample{{
				Value: "Sync",
				Help:  "Will be synced by the official client.",
			}, {
				Value: "Archive",
				Help:  "Archive",
			}},
		}},
	})
}

// Options defines the configuration for this backend
type Options struct {
	User       string `config:"user"`
	Pass       string `config:"pass"`
	Mountpoint string `config:"mountpoint"`
}

// Fs represents a remote jottacloud
type Fs struct {
	name        string
	root        string
	opt         Options
	features    *fs.Features
	endpointURL string
	srv         *rest.Client
	pacer       *pacer.Pacer
}

// Object describes a jottacloud object
//
// Will definitely have info but maybe not meta
type Object struct {
	fs          *Fs
	remote      string
	hasMetaData bool
	size        int64
	modTime     time.Time
	md5         string
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
	return fmt.Sprintf("jottacloud root '%s'", f.root)
}

// Features returns the optional features of this Fs
func (f *Fs) Features() *fs.Features {
	return f.features
}

// parsePath parses an box 'url'
func parsePath(path string) (root string) {
	root = strings.Trim(path, "/")
	return
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

// readMetaDataForPath reads the metadata from the path
func (f *Fs) readMetaDataForPath(path string) (info *api.JottaFile, err error) {
	opts := rest.Opts{
		Method: "GET",
		Path:   f.filePath(path),
	}
	var result api.JottaFile
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
	if result.XMLName.Local != "file" {
		return nil, fs.ErrorNotAFile
	}
	return &result, nil
}

// setEndpointUrl reads the account id and generates the API endpoint URL
func (f *Fs) setEndpointURL(user, mountpoint string) (err error) {
	opts := rest.Opts{
		Method: "GET",
		Path:   rest.URLPathEscape(user),
	}

	var result api.AccountInfo
	var resp *http.Response
	err = f.pacer.Call(func() (bool, error) {
		resp, err = f.srv.CallXML(&opts, nil, &result)
		return shouldRetry(resp, err)
	})
	if err != nil {
		return err
	}

	f.endpointURL = rest.URLPathEscape(path.Join(result.Username, defaultDevice, mountpoint))
	return nil
}

// errorHandler parses a non 2xx error response into an error
func errorHandler(resp *http.Response) error {
	// Decode error response
	errResponse := new(api.Error)
	err := rest.DecodeXML(resp, &errResponse)
	if err != nil {
		fs.Debugf(nil, "Couldn't decode error response: %v", err)
	}
	if errResponse.Message == "" {
		errResponse.Message = resp.Status
	}
	if errResponse.StatusCode == 0 {
		errResponse.StatusCode = resp.StatusCode
	}
	return errResponse
}

// filePath returns a escaped file path (f.root, file)
func (f *Fs) filePath(file string) string {
	return rest.URLPathEscape(path.Join(f.endpointURL, replaceReservedChars(path.Join(f.root, file))))
}

// filePath returns a escaped file path (f.root, remote)
func (o *Object) filePath() string {
	return o.fs.filePath(o.remote)
}

// NewFs constructs an Fs from the path, container:path
func NewFs(name, root string, m configmap.Mapper) (fs.Fs, error) {
	// Parse config into Options struct
	opt := new(Options)
	err := configstruct.Set(m, opt)
	if err != nil {
		return nil, err
	}

	rootIsDir := strings.HasSuffix(root, "/")
	root = parsePath(root)

	user := config.FileGet(name, "user")
	pass := config.FileGet(name, "pass")

	if opt.Pass != "" {
		var err error
		opt.Pass, err = obscure.Reveal(opt.Pass)
		if err != nil {
			return nil, errors.Wrap(err, "couldn't decrypt password")
		}
	}

	f := &Fs{
		name: name,
		root: root,
		opt:  *opt,
		//endpointURL: rest.URLPathEscape(path.Join(user, defaultDevice, opt.Mountpoint)),
		srv:   rest.NewClient(fshttp.NewClient(fs.Config)).SetRoot(rootURL),
		pacer: pacer.New().SetMinSleep(minSleep).SetMaxSleep(maxSleep).SetDecayConstant(decayConstant),
	}
	f.features = (&fs.Features{
		CaseInsensitive:         true,
		CanHaveEmptyDirectories: true,
	}).Fill(f)

	if user == "" || pass == "" {
		return nil, errors.New("jottacloud needs user and password")
	}

	f.srv.SetUserPass(opt.User, opt.Pass)
	f.srv.SetErrorHandler(errorHandler)

	err = f.setEndpointURL(opt.User, opt.Mountpoint)
	if err != nil {
		return nil, errors.Wrap(err, "couldn't get account info")
	}

	if root != "" && !rootIsDir {
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

// Return an Object from a path
//
// If it can't be found it returns the error fs.ErrorObjectNotFound.
func (f *Fs) newObjectWithInfo(remote string, info *api.JottaFile) (fs.Object, error) {
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

// CreateDir makes a directory
func (f *Fs) CreateDir(path string) (jf *api.JottaFolder, err error) {
	// fs.Debugf(f, "CreateDir(%q, %q)\n", pathID, leaf)
	var resp *http.Response
	opts := rest.Opts{
		Method:     "POST",
		Path:       f.filePath(path),
		Parameters: url.Values{},
	}

	opts.Parameters.Set("mkDir", "true")

	err = f.pacer.Call(func() (bool, error) {
		resp, err = f.srv.CallXML(&opts, nil, &jf)
		return shouldRetry(resp, err)
	})
	if err != nil {
		//fmt.Printf("...Error %v\n", err)
		return nil, err
	}
	// fmt.Printf("...Id %q\n", *info.Id)
	return jf, nil
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
	//fmt.Printf("List: %s\n", dir)
	opts := rest.Opts{
		Method: "GET",
		Path:   f.filePath(dir),
	}

	var resp *http.Response
	var result api.JottaFolder
	err = f.pacer.Call(func() (bool, error) {
		resp, err = f.srv.CallXML(&opts, nil, &result)
		return shouldRetry(resp, err)
	})

	if err != nil {
		if apiErr, ok := err.(*api.Error); ok {
			// does not exist
			if apiErr.StatusCode == http.StatusNotFound {
				return nil, fs.ErrorDirNotFound
			}
		}
		return nil, errors.Wrap(err, "couldn't list files")
	}

	if result.Deleted {
		return nil, fs.ErrorDirNotFound
	}

	for i := range result.Folders {
		item := &result.Folders[i]
		if item.Deleted {
			continue
		}
		remote := path.Join(dir, restoreReservedChars(item.Name))
		d := fs.NewDir(remote, time.Time(item.ModifiedAt))
		entries = append(entries, d)
	}

	for i := range result.Files {
		item := &result.Files[i]
		if item.Deleted || item.State != "COMPLETED" {
			continue
		}
		remote := path.Join(dir, restoreReservedChars(item.Name))
		o, err := f.newObjectWithInfo(remote, item)
		if err != nil {
			continue
		}
		entries = append(entries, o)
	}
	//fmt.Printf("Entries: %+v\n", entries)
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
	return f.Mkdir(parent)
}

// Mkdir creates the container if it doesn't exist
func (f *Fs) Mkdir(dir string) error {
	_, err := f.CreateDir(dir)
	return err
}

// purgeCheck removes the root directory, if check is set then it
// refuses to do so if it has anything in
func (f *Fs) purgeCheck(dir string, check bool) (err error) {
	root := path.Join(f.root, dir)
	if root == "" {
		return errors.New("can't purge root directory")
	}

	// check that the directory exists
	entries, err := f.List(dir)
	if err != nil {
		return err
	}

	if check {
		if len(entries) != 0 {
			return fs.ErrorDirectoryNotEmpty
		}
	}

	opts := rest.Opts{
		Method:     "POST",
		Path:       f.filePath(dir),
		Parameters: url.Values{},
		NoResponse: true,
	}

	opts.Parameters.Set("dlDir", "true")

	var resp *http.Response
	err = f.pacer.Call(func() (bool, error) {
		resp, err = f.srv.Call(&opts)
		return shouldRetry(resp, err)
	})
	if err != nil {
		return errors.Wrap(err, "rmdir failed")
	}

	// TODO: Parse response?
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
	return time.Second
}

// Purge deletes all the files and the container
//
// Optional interface: Only implement this if you have a way of
// deleting all the files quicker than just running Remove() on the
// result of List()
func (f *Fs) Purge() error {
	return f.purgeCheck("", false)
}

// copyOrMoves copys or moves directories or files depending on the mthod parameter
func (f *Fs) copyOrMove(method, src, dest string) (info *api.JottaFile, err error) {
	opts := rest.Opts{
		Method:     "POST",
		Path:       src,
		Parameters: url.Values{},
	}

	opts.Parameters.Set(method, "/"+path.Join(f.endpointURL, replaceReservedChars(path.Join(f.root, dest))))

	var resp *http.Response
	err = f.pacer.Call(func() (bool, error) {
		resp, err = f.srv.CallXML(&opts, nil, &info)
		return shouldRetry(resp, err)
	})
	if err != nil {
		return nil, err
	}

	return info, nil
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
	srcObj, ok := src.(*Object)
	if !ok {
		fs.Debugf(src, "Can't copy - not same remote type")
		return nil, fs.ErrorCantMove
	}

	err := f.mkParentDir(remote)
	if err != nil {
		return nil, err
	}
	info, err := f.copyOrMove("cp", srcObj.filePath(), remote)

	if err != nil {
		return nil, errors.Wrap(err, "copy failed")
	}

	return f.newObjectWithInfo(remote, info)
	//return f.newObjectWithInfo(remote, &result)
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
	srcObj, ok := src.(*Object)
	if !ok {
		fs.Debugf(src, "Can't move - not same remote type")
		return nil, fs.ErrorCantMove
	}

	err := f.mkParentDir(remote)
	if err != nil {
		return nil, err
	}
	info, err := f.copyOrMove("mv", srcObj.filePath(), remote)

	if err != nil {
		return nil, errors.Wrap(err, "move failed")
	}

	return f.newObjectWithInfo(remote, info)
	//return f.newObjectWithInfo(remote, result)
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
	srcPath := path.Join(srcFs.root, srcRemote)
	dstPath := path.Join(f.root, dstRemote)

	// Refuse to move to or from the root
	if srcPath == "" || dstPath == "" {
		fs.Debugf(src, "DirMove error: Can't move root")
		return errors.New("can't move root directory")
	}
	//fmt.Printf("Move src: %s (FullPath %s), dst: %s (FullPath: %s)\n", srcRemote, srcPath, dstRemote, dstPath)

	var err error
	_, err = f.List(dstRemote)
	if err == fs.ErrorDirNotFound {
		// OK
	} else if err != nil {
		return err
	} else {
		return fs.ErrorDirExists
	}

	_, err = f.copyOrMove("mvDir", path.Join(f.endpointURL, replaceReservedChars(srcPath))+"/", dstRemote)

	if err != nil {
		return errors.Wrap(err, "moveDir failed")
	}
	return nil
}

// Hashes returns the supported hash sets.
func (f *Fs) Hashes() hash.Set {
	return hash.Set(hash.MD5)
}

// ---------------------------------------------

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

// Hash returns the MD5 of an object returning a lowercase hex string
func (o *Object) Hash(t hash.Type) (string, error) {
	if t != hash.MD5 {
		return "", hash.ErrUnsupported
	}
	return o.md5, nil
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
func (o *Object) setMetaData(info *api.JottaFile) (err error) {
	o.hasMetaData = true
	o.size = int64(info.Size)
	o.md5 = info.MD5
	o.modTime = time.Time(info.ModifiedAt)
	return nil
}

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
	fs.FixRangeOption(options, o.size)
	var resp *http.Response
	opts := rest.Opts{
		Method:     "GET",
		Path:       o.filePath(),
		Parameters: url.Values{},
		Options:    options,
	}

	opts.Parameters.Set("mode", "bin")

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
	size := src.Size()

	var resp *http.Response
	var result api.JottaFile
	opts := rest.Opts{
		Method:        "POST",
		Path:          o.filePath(),
		Body:          in,
		ContentType:   fs.MimeType(src),
		ContentLength: &size,
		ExtraHeaders:  make(map[string]string),
		Parameters:    url.Values{},
	}

	md5, err := src.Hash(hash.MD5)
	if err != nil {
		opts.ExtraHeaders["JMd5"] = md5
		opts.Parameters.Set("cphash", md5)
	}

	opts.ExtraHeaders["JSize"] = strconv.FormatInt(size, 10)
	//opts.ExtraHeaders["JCreated"] =
	opts.ExtraHeaders["JModified"] = api.Time(src.ModTime()).String()

	// Parameters observed in other implementations
	//opts.ExtraHeaders["X-Jfs-DeviceName"] = "Jotta"
	//opts.ExtraHeaders["X-Jfs-Devicename-Base64"] = ""
	//opts.ExtraHeaders["X-Jftp-Version"] = "2.4" this appears to be the current version
	//opts.ExtraHeaders["jx_csid"] = ""
	//opts.ExtraHeaders["jx_lisence"] = ""

	opts.Parameters.Set("umode", "nomultipart")

	err = o.fs.pacer.CallNoRetry(func() (bool, error) {
		resp, err = o.fs.srv.CallXML(&opts, nil, &result)
		return shouldRetry(resp, err)
	})
	if err != nil {
		return err
	}

	// TODO: Check returned Metadata? Timeout on big uploads?
	return o.setMetaData(&result)
}

// Remove an object
func (o *Object) Remove() error {
	opts := rest.Opts{
		Method:     "POST",
		Path:       o.filePath(),
		Parameters: url.Values{},
	}

	opts.Parameters.Set("dl", "true")

	return o.fs.pacer.Call(func() (bool, error) {
		resp, err := o.fs.srv.CallXML(&opts, nil, nil)
		return shouldRetry(resp, err)
	})
}

// Check the interfaces are satisfied
var (
	_ fs.Fs       = (*Fs)(nil)
	_ fs.Purger   = (*Fs)(nil)
	_ fs.Copier   = (*Fs)(nil)
	_ fs.Mover    = (*Fs)(nil)
	_ fs.DirMover = (*Fs)(nil)
	_ fs.Object   = (*Object)(nil)
)
