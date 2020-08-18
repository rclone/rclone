package yandex

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/pkg/errors"
	"github.com/rclone/rclone/backend/yandex/api"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config"
	"github.com/rclone/rclone/fs/config/configmap"
	"github.com/rclone/rclone/fs/config/configstruct"
	"github.com/rclone/rclone/fs/config/obscure"
	"github.com/rclone/rclone/fs/fserrors"
	"github.com/rclone/rclone/fs/hash"
	"github.com/rclone/rclone/lib/encoder"
	"github.com/rclone/rclone/lib/oauthutil"
	"github.com/rclone/rclone/lib/pacer"
	"github.com/rclone/rclone/lib/readers"
	"github.com/rclone/rclone/lib/rest"
	"golang.org/x/oauth2"
)

//oAuth
const (
	rcloneClientID              = "ac39b43b9eba4cae8ffb788c06d816a8"
	rcloneEncryptedClientSecret = "EfyyNZ3YUEwXM5yAhi72G9YwKn2mkFrYwJNS7cY0TJAhFlX9K-uJFbGlpO-RYjrJ"
	rootURL                     = "https://cloud-api.yandex.com/v1/disk"
	minSleep                    = 10 * time.Millisecond
	maxSleep                    = 2 * time.Second // may needs to be increased, testing needed
	decayConstant               = 2               // bigger for slower decay, exponential
)

// Globals
var (
	// Description of how to auth for this app
	oauthConfig = &oauth2.Config{
		Endpoint: oauth2.Endpoint{
			AuthURL:  "https://oauth.yandex.com/authorize", //same as https://oauth.yandex.ru/authorize
			TokenURL: "https://oauth.yandex.com/token",     //same as https://oauth.yandex.ru/token
		},
		ClientID:     rcloneClientID,
		ClientSecret: obscure.MustReveal(rcloneEncryptedClientSecret),
		RedirectURL:  oauthutil.RedirectURL,
	}
)

// Register with Fs
func init() {
	fs.Register(&fs.RegInfo{
		Name:        "yandex",
		Description: "Yandex Disk",
		NewFs:       NewFs,
		Config: func(name string, m configmap.Mapper) {
			err := oauthutil.Config("yandex", name, m, oauthConfig, nil)
			if err != nil {
				log.Fatalf("Failed to configure token: %v", err)
				return
			}
		},
		Options: append(oauthutil.SharedOptions, []fs.Option{{
			Name:     config.ConfigEncoding,
			Help:     config.ConfigEncodingHelp,
			Advanced: true,
			// Of the control characters \t \n \r are allowed
			// it doesn't seem worth making an exception for this
			Default: (encoder.Display |
				encoder.EncodeInvalidUtf8),
		}}...),
	})
}

// Options defines the configuration for this backend
type Options struct {
	Token string               `config:"token"`
	Enc   encoder.MultiEncoder `config:"encoding"`
}

// Fs represents a remote yandex
type Fs struct {
	name     string
	root     string       // root path
	opt      Options      // parsed options
	features *fs.Features // optional features
	srv      *rest.Client // the connection to the yandex server
	pacer    *fs.Pacer    // pacer for API calls
	diskRoot string       // root path with "disk:/" container name
}

// Object describes a swift object
type Object struct {
	fs          *Fs       // what this object is part of
	remote      string    // The remote path
	hasMetaData bool      // whether info below has been set
	md5sum      string    // The MD5Sum of the object
	size        int64     // Bytes in the object
	modTime     time.Time // Modified time of the object
	mimeType    string    // Content type according to the server

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
	return fmt.Sprintf("Yandex %s", f.root)
}

// Precision return the precision of this Fs
func (f *Fs) Precision() time.Duration {
	return time.Nanosecond
}

// Hashes returns the supported hash sets.
func (f *Fs) Hashes() hash.Set {
	return hash.Set(hash.MD5)
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

// errorHandler parses a non 2xx error response into an error
func errorHandler(resp *http.Response) error {
	// Decode error response
	errResponse := new(api.ErrorResponse)
	err := rest.DecodeJSON(resp, &errResponse)
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

// Sets root in f
func (f *Fs) setRoot(root string) {
	//Set root path
	f.root = strings.Trim(root, "/")
	//Set disk root path.
	//Adding "disk:" to root path as all paths on disk start with it
	var diskRoot string
	if f.root == "" {
		diskRoot = "disk:/"
	} else {
		diskRoot = "disk:/" + f.root + "/"
	}
	f.diskRoot = diskRoot
}

// filePath returns an escaped file path (f.root, file)
func (f *Fs) filePath(file string) string {
	return path.Join(f.diskRoot, file)
}

// dirPath returns an escaped file path (f.root, file) ending with '/'
func (f *Fs) dirPath(file string) string {
	return path.Join(f.diskRoot, file) + "/"
}

func (f *Fs) readMetaDataForPath(ctx context.Context, path string, options *api.ResourceInfoRequestOptions) (*api.ResourceInfoResponse, error) {
	opts := rest.Opts{
		Method:     "GET",
		Path:       "/resources",
		Parameters: url.Values{},
	}

	opts.Parameters.Set("path", f.opt.Enc.FromStandardPath(path))

	if options.SortMode != nil {
		opts.Parameters.Set("sort", options.SortMode.String())
	}
	if options.Limit != 0 {
		opts.Parameters.Set("limit", strconv.FormatUint(options.Limit, 10))
	}
	if options.Offset != 0 {
		opts.Parameters.Set("offset", strconv.FormatUint(options.Offset, 10))
	}
	if options.Fields != nil {
		opts.Parameters.Set("fields", strings.Join(options.Fields, ","))
	}

	var err error
	var info api.ResourceInfoResponse
	var resp *http.Response
	err = f.pacer.Call(func() (bool, error) {
		resp, err = f.srv.CallJSON(ctx, &opts, nil, &info)
		return shouldRetry(resp, err)
	})

	if err != nil {
		return nil, err
	}

	info.Name = f.opt.Enc.ToStandardName(info.Name)
	return &info, nil
}

// NewFs constructs an Fs from the path, container:path
func NewFs(name, root string, m configmap.Mapper) (fs.Fs, error) {
	ctx := context.TODO()
	// Parse config into Options struct
	opt := new(Options)
	err := configstruct.Set(m, opt)
	if err != nil {
		return nil, err
	}

	token, err := oauthutil.GetToken(name, m)
	if err != nil {
		log.Fatalf("Couldn't read OAuth token (this should never happen).")
	}
	if token.RefreshToken == "" {
		log.Fatalf("Unable to get RefreshToken. If you are upgrading from older versions of rclone, please run `rclone config` and re-configure this backend.")
	}
	if token.TokenType != "OAuth" {
		token.TokenType = "OAuth"
		err = oauthutil.PutToken(name, m, token, false)
		if err != nil {
			log.Fatalf("Couldn't save OAuth token (this should never happen).")
		}
		log.Printf("Automatically upgraded OAuth config.")
	}
	oAuthClient, _, err := oauthutil.NewClient(name, m, oauthConfig)
	if err != nil {
		log.Fatalf("Failed to configure Yandex: %v", err)
	}

	f := &Fs{
		name:  name,
		opt:   *opt,
		srv:   rest.NewClient(oAuthClient).SetRoot(rootURL),
		pacer: fs.NewPacer(pacer.NewDefault(pacer.MinSleep(minSleep), pacer.MaxSleep(maxSleep), pacer.DecayConstant(decayConstant))),
	}
	f.setRoot(root)
	f.features = (&fs.Features{
		ReadMimeType:            true,
		WriteMimeType:           true,
		CanHaveEmptyDirectories: true,
	}).Fill(f)
	f.srv.SetErrorHandler(errorHandler)

	// Check to see if the object exists and is a file
	//request object meta info
	// Check to see if the object exists and is a file
	//request object meta info
	if info, err := f.readMetaDataForPath(ctx, f.diskRoot, &api.ResourceInfoRequestOptions{}); err != nil {

	} else {
		if info.ResourceType == "file" {
			rootDir := path.Dir(root)
			if rootDir == "." {
				rootDir = ""
			}
			f.setRoot(rootDir)
			// return an error with an fs which points to the parent
			return f, fs.ErrorIsFile
		}
	}
	return f, nil
}

// Convert a list item into a DirEntry
func (f *Fs) itemToDirEntry(ctx context.Context, remote string, object *api.ResourceInfoResponse) (fs.DirEntry, error) {
	switch object.ResourceType {
	case "dir":
		t, err := time.Parse(time.RFC3339Nano, object.Modified)
		if err != nil {
			return nil, errors.Wrap(err, "error parsing time in directory item")
		}
		d := fs.NewDir(remote, t).SetSize(object.Size)
		return d, nil
	case "file":
		o, err := f.newObjectWithInfo(ctx, remote, object)
		if err != nil {
			return nil, err
		}
		return o, nil
	default:
		fs.Debugf(f, "Unknown resource type %q", object.ResourceType)
	}
	return nil, nil
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
	root := f.dirPath(dir)

	var limit uint64 = 1000 // max number of objects per request
	var itemsCount uint64   // number of items per page in response
	var offset uint64       // for the next page of requests

	for {
		opts := &api.ResourceInfoRequestOptions{
			Limit:  limit,
			Offset: offset,
		}
		info, err := f.readMetaDataForPath(ctx, root, opts)

		if err != nil {
			if apiErr, ok := err.(*api.ErrorResponse); ok {
				// does not exist
				if apiErr.ErrorName == "DiskNotFoundError" {
					return nil, fs.ErrorDirNotFound
				}
			}
			return nil, err
		}
		itemsCount = uint64(len(info.Embedded.Items))

		if info.ResourceType == "dir" {
			//list all subdirs
			for _, element := range info.Embedded.Items {
				element.Name = f.opt.Enc.ToStandardName(element.Name)
				remote := path.Join(dir, element.Name)
				entry, err := f.itemToDirEntry(ctx, remote, &element)
				if err != nil {
					return nil, err
				}
				if entry != nil {
					entries = append(entries, entry)
				}
			}
		} else if info.ResourceType == "file" {
			return nil, fs.ErrorIsFile
		}

		//offset for the next page of items
		offset += itemsCount
		//check if we reached end of list
		if itemsCount < limit {
			break
		}
	}

	return entries, nil
}

// Return an Object from a path
//
// If it can't be found it returns the error fs.ErrorObjectNotFound.
func (f *Fs) newObjectWithInfo(ctx context.Context, remote string, info *api.ResourceInfoResponse) (fs.Object, error) {
	o := &Object{
		fs:     f,
		remote: remote,
	}
	var err error
	if info != nil {
		err = o.setMetaData(info)
	} else {
		err = o.readMetaData(ctx)
		if apiErr, ok := err.(*api.ErrorResponse); ok {
			// does not exist
			if apiErr.ErrorName == "DiskNotFoundError" {
				return nil, fs.ErrorObjectNotFound
			}
		}
	}
	if err != nil {
		return nil, err
	}
	return o, nil
}

// NewObject finds the Object at remote.  If it can't be found it
// returns the error fs.ErrorObjectNotFound.
func (f *Fs) NewObject(ctx context.Context, remote string) (fs.Object, error) {
	return f.newObjectWithInfo(ctx, remote, nil)
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
func (f *Fs) Put(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error) {
	o := f.createObject(src.Remote(), src.ModTime(ctx), src.Size())
	return o, o.Update(ctx, in, src, options...)
}

// PutStream uploads to the remote path with the modTime given of indeterminate size
func (f *Fs) PutStream(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error) {
	return f.Put(ctx, in, src, options...)
}

// CreateDir makes a directory
func (f *Fs) CreateDir(ctx context.Context, path string) (err error) {
	//fmt.Printf("CreateDir: %s\n", path)

	var resp *http.Response
	opts := rest.Opts{
		Method:     "PUT",
		Path:       "/resources",
		Parameters: url.Values{},
		NoResponse: true,
	}

	// If creating a directory with a : use (undocumented) disk: prefix
	if strings.IndexRune(path, ':') >= 0 {
		path = "disk:" + path
	}
	opts.Parameters.Set("path", f.opt.Enc.FromStandardPath(path))

	err = f.pacer.Call(func() (bool, error) {
		resp, err = f.srv.Call(ctx, &opts)
		return shouldRetry(resp, err)
	})
	if err != nil {
		// fmt.Printf("CreateDir %q Error: %s\n", path, err.Error())
		return err
	}
	// fmt.Printf("...Id %q\n", *info.Id)
	return nil
}

// This really needs improvement and especially proper error checking
// but Yandex does not publish a List of possible errors and when they're
// expected to occur.
func (f *Fs) mkDirs(ctx context.Context, path string) (err error) {
	//trim filename from path
	//dirString := strings.TrimSuffix(path, filepath.Base(path))
	//trim "disk:" from path
	dirString := strings.TrimPrefix(path, "disk:")
	if dirString == "" {
		return nil
	}

	if err = f.CreateDir(ctx, dirString); err != nil {
		if apiErr, ok := err.(*api.ErrorResponse); ok {
			// already exists
			if apiErr.ErrorName != "DiskPathPointsToExistentDirectoryError" {
				// 2 if it fails then create all directories in the path from root.
				dirs := strings.Split(dirString, "/") //path separator
				var mkdirpath = "/"                   //path separator /
				for _, element := range dirs {
					if element != "" {
						mkdirpath += element + "/" //path separator /
						if err = f.CreateDir(ctx, mkdirpath); err != nil {
							// ignore errors while creating dirs
						}
					}
				}
			}
			return nil
		}
	}
	return err
}

func (f *Fs) mkParentDirs(ctx context.Context, resPath string) error {
	// defer log.Trace(dirPath, "")("")
	// chop off trailing / if it exists
	if strings.HasSuffix(resPath, "/") {
		resPath = resPath[:len(resPath)-1]
	}
	parent := path.Dir(resPath)
	if parent == "." {
		parent = ""
	}
	return f.mkDirs(ctx, parent)
}

// Mkdir creates the container if it doesn't exist
func (f *Fs) Mkdir(ctx context.Context, dir string) error {
	path := f.filePath(dir)
	return f.mkDirs(ctx, path)
}

// waitForJob waits for the job with status in url to complete
func (f *Fs) waitForJob(ctx context.Context, location string) (err error) {
	opts := rest.Opts{
		RootURL: location,
		Method:  "GET",
	}
	deadline := time.Now().Add(fs.Config.Timeout)
	for time.Now().Before(deadline) {
		var resp *http.Response
		var body []byte
		err = f.pacer.Call(func() (bool, error) {
			resp, err = f.srv.Call(ctx, &opts)
			if err != nil {
				return fserrors.ShouldRetry(err), err
			}
			body, err = rest.ReadBody(resp)
			return fserrors.ShouldRetry(err), err
		})
		if err != nil {
			return err
		}
		// Try to decode the body first as an api.AsyncOperationStatus
		var status api.AsyncStatus
		err = json.Unmarshal(body, &status)
		if err != nil {
			return errors.Wrapf(err, "async status result not JSON: %q", body)
		}

		switch status.Status {
		case "failure":
			return errors.Errorf("async operation returned %q", status.Status)
		case "success":
			return nil
		}

		time.Sleep(1 * time.Second)
	}
	return errors.Errorf("async operation didn't complete after %v", fs.Config.Timeout)
}

func (f *Fs) delete(ctx context.Context, path string, hardDelete bool) (err error) {
	opts := rest.Opts{
		Method:     "DELETE",
		Path:       "/resources",
		Parameters: url.Values{},
	}

	opts.Parameters.Set("path", f.opt.Enc.FromStandardPath(path))
	opts.Parameters.Set("permanently", strconv.FormatBool(hardDelete))

	var resp *http.Response
	var body []byte
	err = f.pacer.Call(func() (bool, error) {
		resp, err = f.srv.Call(ctx, &opts)
		if err != nil {
			return fserrors.ShouldRetry(err), err
		}
		body, err = rest.ReadBody(resp)
		return fserrors.ShouldRetry(err), err
	})
	if err != nil {
		return err
	}

	// if 202 Accepted it's an async operation we have to wait for it complete before retuning
	if resp.StatusCode == 202 {
		var info api.AsyncInfo
		err = json.Unmarshal(body, &info)
		if err != nil {
			return errors.Wrapf(err, "async info result not JSON: %q", body)
		}
		return f.waitForJob(ctx, info.HRef)
	}
	return nil
}

// purgeCheck remotes the root directory, if check is set then it
// refuses to do so if it has anything in
func (f *Fs) purgeCheck(ctx context.Context, dir string, check bool) error {
	root := f.filePath(dir)
	if check {
		//to comply with rclone logic we check if the directory is empty before delete.
		//send request to get list of objects in this directory.
		info, err := f.readMetaDataForPath(ctx, root, &api.ResourceInfoRequestOptions{})
		if err != nil {
			return errors.Wrap(err, "rmdir failed")
		}
		if len(info.Embedded.Items) != 0 {
			return fs.ErrorDirectoryNotEmpty
		}
	}
	//delete directory
	return f.delete(ctx, root, false)
}

// Rmdir deletes the container
//
// Returns an error if it isn't empty
func (f *Fs) Rmdir(ctx context.Context, dir string) error {
	return f.purgeCheck(ctx, dir, true)
}

// Purge deletes all the files in the directory
//
// Optional interface: Only implement this if you have a way of
// deleting all the files quicker than just running Remove() on the
// result of List()
func (f *Fs) Purge(ctx context.Context, dir string) error {
	return f.purgeCheck(ctx, dir, false)
}

// copyOrMoves copies or moves directories or files depending on the method parameter
func (f *Fs) copyOrMove(ctx context.Context, method, src, dst string, overwrite bool) (err error) {
	opts := rest.Opts{
		Method:     "POST",
		Path:       "/resources/" + method,
		Parameters: url.Values{},
	}

	opts.Parameters.Set("from", f.opt.Enc.FromStandardPath(src))
	opts.Parameters.Set("path", f.opt.Enc.FromStandardPath(dst))
	opts.Parameters.Set("overwrite", strconv.FormatBool(overwrite))

	var resp *http.Response
	var body []byte
	err = f.pacer.Call(func() (bool, error) {
		resp, err = f.srv.Call(ctx, &opts)
		if err != nil {
			return fserrors.ShouldRetry(err), err
		}
		body, err = rest.ReadBody(resp)
		return fserrors.ShouldRetry(err), err
	})
	if err != nil {
		return err
	}

	// if 202 Accepted it's an async operation we have to wait for it complete before retuning
	if resp.StatusCode == 202 {
		var info api.AsyncInfo
		err = json.Unmarshal(body, &info)
		if err != nil {
			return errors.Wrapf(err, "async info result not JSON: %q", body)
		}
		return f.waitForJob(ctx, info.HRef)
	}
	return nil
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
func (f *Fs) Copy(ctx context.Context, src fs.Object, remote string) (fs.Object, error) {
	srcObj, ok := src.(*Object)
	if !ok {
		fs.Debugf(src, "Can't copy - not same remote type")
		return nil, fs.ErrorCantCopy
	}

	dstPath := f.filePath(remote)
	err := f.mkParentDirs(ctx, dstPath)
	if err != nil {
		return nil, err
	}
	err = f.copyOrMove(ctx, "copy", srcObj.filePath(), dstPath, false)

	if err != nil {
		return nil, errors.Wrap(err, "couldn't copy file")
	}

	return f.NewObject(ctx, remote)
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
func (f *Fs) Move(ctx context.Context, src fs.Object, remote string) (fs.Object, error) {
	srcObj, ok := src.(*Object)
	if !ok {
		fs.Debugf(src, "Can't move - not same remote type")
		return nil, fs.ErrorCantMove
	}

	dstPath := f.filePath(remote)
	err := f.mkParentDirs(ctx, dstPath)
	if err != nil {
		return nil, err
	}
	err = f.copyOrMove(ctx, "move", srcObj.filePath(), dstPath, false)

	if err != nil {
		return nil, errors.Wrap(err, "couldn't move file")
	}

	return f.NewObject(ctx, remote)
}

// DirMove moves src, srcRemote to this remote at dstRemote
// using server side move operations.
//
// Will only be called if src.Fs().Name() == f.Name()
//
// If it isn't possible then return fs.ErrorCantDirMove
//
// If destination exists then return fs.ErrorDirExists
func (f *Fs) DirMove(ctx context.Context, src fs.Fs, srcRemote, dstRemote string) error {
	srcFs, ok := src.(*Fs)
	if !ok {
		fs.Debugf(srcFs, "Can't move directory - not same remote type")
		return fs.ErrorCantDirMove
	}
	srcPath := path.Join(srcFs.diskRoot, srcRemote)
	dstPath := f.dirPath(dstRemote)

	//fmt.Printf("Move src: %s (FullPath: %s), dst: %s (FullPath: %s)\n", srcRemote, srcPath, dstRemote, dstPath)

	// Refuse to move to or from the root
	if srcPath == "disk:/" || dstPath == "disk:/" {
		fs.Debugf(src, "DirMove error: Can't move root")
		return errors.New("can't move root directory")
	}

	err := f.mkParentDirs(ctx, dstPath)
	if err != nil {
		return err
	}

	_, err = f.readMetaDataForPath(ctx, dstPath, &api.ResourceInfoRequestOptions{})
	if apiErr, ok := err.(*api.ErrorResponse); ok {
		// does not exist
		if apiErr.ErrorName == "DiskNotFoundError" {
			// OK
		}
	} else if err != nil {
		return err
	} else {
		return fs.ErrorDirExists
	}

	err = f.copyOrMove(ctx, "move", srcPath, dstPath, false)

	if err != nil {
		return errors.Wrap(err, "couldn't move directory")
	}
	return nil
}

// PublicLink generates a public link to the remote path (usually readable by anyone)
func (f *Fs) PublicLink(ctx context.Context, remote string, expire fs.Duration, unlink bool) (link string, err error) {
	var path string
	if unlink {
		path = "/resources/unpublish"
	} else {
		path = "/resources/publish"
	}
	opts := rest.Opts{
		Method:     "PUT",
		Path:       f.opt.Enc.FromStandardPath(path),
		Parameters: url.Values{},
		NoResponse: true,
	}

	opts.Parameters.Set("path", f.opt.Enc.FromStandardPath(f.filePath(remote)))

	var resp *http.Response
	err = f.pacer.Call(func() (bool, error) {
		resp, err = f.srv.Call(ctx, &opts)
		return shouldRetry(resp, err)
	})

	if apiErr, ok := err.(*api.ErrorResponse); ok {
		// does not exist
		if apiErr.ErrorName == "DiskNotFoundError" {
			return "", fs.ErrorObjectNotFound
		}
	}
	if err != nil {
		if unlink {
			return "", errors.Wrap(err, "couldn't remove public link")
		}
		return "", errors.Wrap(err, "couldn't create public link")
	}

	info, err := f.readMetaDataForPath(ctx, f.filePath(remote), &api.ResourceInfoRequestOptions{})
	if err != nil {
		return "", err
	}

	if info.PublicURL == "" {
		return "", errors.New("couldn't create public link - no link path received")
	}
	return info.PublicURL, nil
}

// CleanUp permanently deletes all trashed files/folders
func (f *Fs) CleanUp(ctx context.Context) (err error) {
	var resp *http.Response
	opts := rest.Opts{
		Method:     "DELETE",
		Path:       "/trash/resources",
		NoResponse: true,
	}

	err = f.pacer.Call(func() (bool, error) {
		resp, err = f.srv.Call(ctx, &opts)
		return shouldRetry(resp, err)
	})
	return err
}

// About gets quota information
func (f *Fs) About(ctx context.Context) (*fs.Usage, error) {
	opts := rest.Opts{
		Method: "GET",
		Path:   "/",
	}

	var resp *http.Response
	var info api.DiskInfo
	var err error
	err = f.pacer.Call(func() (bool, error) {
		resp, err = f.srv.CallJSON(ctx, &opts, nil, &info)
		return shouldRetry(resp, err)
	})

	if err != nil {
		return nil, err
	}

	usage := &fs.Usage{
		Total: fs.NewUsageValue(info.TotalSpace),
		Used:  fs.NewUsageValue(info.UsedSpace),
		Free:  fs.NewUsageValue(info.TotalSpace - info.UsedSpace),
	}
	return usage, nil
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

// Returns the full remote path for the object
func (o *Object) filePath() string {
	return o.fs.filePath(o.remote)
}

// setMetaData sets the fs data from a storage.Object
func (o *Object) setMetaData(info *api.ResourceInfoResponse) (err error) {
	o.hasMetaData = true
	o.size = info.Size
	o.md5sum = info.Md5
	o.mimeType = info.MimeType

	var modTimeString string
	modTimeObj, ok := info.CustomProperties["rclone_modified"]
	if ok {
		// read modTime from rclone_modified custom_property of object
		modTimeString, ok = modTimeObj.(string)
	}
	if !ok {
		// read modTime from Modified property of object as a fallback
		modTimeString = info.Modified
	}
	t, err := time.Parse(time.RFC3339Nano, modTimeString)
	if err != nil {
		return errors.Wrapf(err, "failed to parse modtime from %q", modTimeString)
	}
	o.modTime = t
	return nil
}

// readMetaData reads ands sets the new metadata for a storage.Object
func (o *Object) readMetaData(ctx context.Context) (err error) {
	if o.hasMetaData {
		return nil
	}
	info, err := o.fs.readMetaDataForPath(ctx, o.filePath(), &api.ResourceInfoRequestOptions{})
	if err != nil {
		return err
	}
	if info.ResourceType != "file" {
		return fs.ErrorNotAFile
	}
	return o.setMetaData(info)
}

// ModTime returns the modification time of the object
//
// It attempts to read the objects mtime and if that isn't present the
// LastModified returned in the http headers
func (o *Object) ModTime(ctx context.Context) time.Time {
	err := o.readMetaData(ctx)
	if err != nil {
		fs.Logf(o, "Failed to read metadata: %v", err)
		return time.Now()
	}
	return o.modTime
}

// Size returns the size of an object in bytes
func (o *Object) Size() int64 {
	ctx := context.TODO()
	err := o.readMetaData(ctx)
	if err != nil {
		fs.Logf(o, "Failed to read metadata: %v", err)
		return 0
	}
	return o.size
}

// Hash returns the Md5sum of an object returning a lowercase hex string
func (o *Object) Hash(ctx context.Context, t hash.Type) (string, error) {
	if t != hash.MD5 {
		return "", hash.ErrUnsupported
	}
	return o.md5sum, nil
}

// Storable returns whether this object is storable
func (o *Object) Storable() bool {
	return true
}

func (o *Object) setCustomProperty(ctx context.Context, property string, value string) (err error) {
	var resp *http.Response
	opts := rest.Opts{
		Method:     "PATCH",
		Path:       "/resources",
		Parameters: url.Values{},
		NoResponse: true,
	}

	opts.Parameters.Set("path", o.fs.opt.Enc.FromStandardPath(o.filePath()))
	rcm := map[string]interface{}{
		property: value,
	}
	cpr := api.CustomPropertyResponse{CustomProperties: rcm}

	err = o.fs.pacer.Call(func() (bool, error) {
		resp, err = o.fs.srv.CallJSON(ctx, &opts, &cpr, nil)
		return shouldRetry(resp, err)
	})
	return err
}

// SetModTime sets the modification time of the local fs object
//
// Commits the datastore
func (o *Object) SetModTime(ctx context.Context, modTime time.Time) error {
	// set custom_property 'rclone_modified' of object to modTime
	err := o.setCustomProperty(ctx, "rclone_modified", modTime.Format(time.RFC3339Nano))
	if err != nil {
		return err
	}
	o.modTime = modTime
	return nil
}

// Open an object for read
func (o *Object) Open(ctx context.Context, options ...fs.OpenOption) (in io.ReadCloser, err error) {
	// prepare download
	var resp *http.Response
	var dl api.AsyncInfo
	opts := rest.Opts{
		Method:     "GET",
		Path:       "/resources/download",
		Parameters: url.Values{},
	}

	opts.Parameters.Set("path", o.fs.opt.Enc.FromStandardPath(o.filePath()))

	err = o.fs.pacer.Call(func() (bool, error) {
		resp, err = o.fs.srv.CallJSON(ctx, &opts, nil, &dl)
		return shouldRetry(resp, err)
	})

	if err != nil {
		return nil, err
	}

	// perform the download
	opts = rest.Opts{
		RootURL: dl.HRef,
		Method:  "GET",
		Options: options,
	}
	err = o.fs.pacer.Call(func() (bool, error) {
		resp, err = o.fs.srv.Call(ctx, &opts)
		return shouldRetry(resp, err)
	})
	if err != nil {
		return nil, err
	}
	return resp.Body, err
}

func (o *Object) upload(ctx context.Context, in io.Reader, overwrite bool, mimeType string, options ...fs.OpenOption) (err error) {
	// prepare upload
	var resp *http.Response
	var ur api.AsyncInfo
	opts := rest.Opts{
		Method:     "GET",
		Path:       "/resources/upload",
		Parameters: url.Values{},
		Options:    options,
	}

	opts.Parameters.Set("path", o.fs.opt.Enc.FromStandardPath(o.filePath()))
	opts.Parameters.Set("overwrite", strconv.FormatBool(overwrite))

	err = o.fs.pacer.Call(func() (bool, error) {
		resp, err = o.fs.srv.CallJSON(ctx, &opts, nil, &ur)
		return shouldRetry(resp, err)
	})

	if err != nil {
		return err
	}

	// perform the actual upload
	opts = rest.Opts{
		RootURL:     ur.HRef,
		Method:      "PUT",
		ContentType: mimeType,
		Body:        in,
		NoResponse:  true,
	}

	err = o.fs.pacer.Call(func() (bool, error) {
		resp, err = o.fs.srv.Call(ctx, &opts)
		return shouldRetry(resp, err)
	})

	return err
}

// Update the already existing object
//
// Copy the reader into the object updating modTime and size
//
// The new object may have been created if an error is returned
func (o *Object) Update(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) error {
	in1 := readers.NewCountingReader(in)
	modTime := src.ModTime(ctx)
	remote := o.filePath()

	//create full path to file before upload.
	err := o.fs.mkParentDirs(ctx, remote)
	if err != nil {
		return err
	}

	//upload file
	err = o.upload(ctx, in1, true, fs.MimeType(ctx, src), options...)
	if err != nil {
		return err
	}

	//if file uploaded successfully then return metadata
	o.modTime = modTime
	o.md5sum = ""                   // according to unit tests after put the md5 is empty.
	o.size = int64(in1.BytesRead()) // better solution o.readMetaData() ?
	//and set modTime of uploaded file
	err = o.SetModTime(ctx, modTime)

	return err
}

// Remove an object
func (o *Object) Remove(ctx context.Context) error {
	return o.fs.delete(ctx, o.filePath(), false)
}

// MimeType of an Object if known, "" otherwise
func (o *Object) MimeType(ctx context.Context) string {
	return o.mimeType
}

// Check the interfaces are satisfied
var (
	_ fs.Fs           = (*Fs)(nil)
	_ fs.Purger       = (*Fs)(nil)
	_ fs.Copier       = (*Fs)(nil)
	_ fs.Mover        = (*Fs)(nil)
	_ fs.DirMover     = (*Fs)(nil)
	_ fs.PublicLinker = (*Fs)(nil)
	_ fs.CleanUpper   = (*Fs)(nil)
	_ fs.Abouter      = (*Fs)(nil)
	_ fs.Object       = (*Object)(nil)
	_ fs.MimeTyper    = (*Object)(nil)
)
