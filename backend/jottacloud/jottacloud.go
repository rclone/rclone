package jottacloud

import (
	"bytes"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/ncw/rclone/backend/jottacloud/api"
	"github.com/ncw/rclone/fs"
	"github.com/ncw/rclone/fs/accounting"
	"github.com/ncw/rclone/fs/config"
	"github.com/ncw/rclone/fs/config/configmap"
	"github.com/ncw/rclone/fs/config/configstruct"
	"github.com/ncw/rclone/fs/config/obscure"
	"github.com/ncw/rclone/fs/fserrors"
	"github.com/ncw/rclone/fs/fshttp"
	"github.com/ncw/rclone/fs/hash"
	"github.com/ncw/rclone/fs/walk"
	"github.com/ncw/rclone/lib/oauthutil"
	"github.com/ncw/rclone/lib/pacer"
	"github.com/ncw/rclone/lib/rest"
	"github.com/pkg/errors"
	"golang.org/x/oauth2"
)

// Globals
const (
	minSleep                    = 10 * time.Millisecond
	maxSleep                    = 2 * time.Second
	decayConstant               = 2 // bigger for slower decay, exponential
	defaultDevice               = "Jotta"
	defaultMountpoint           = "Sync"
	rootURL                     = "https://www.jottacloud.com/jfs/"
	apiURL                      = "https://api.jottacloud.com/files/v1/"
	baseURL                     = "https://www.jottacloud.com/"
	tokenURL                    = "https://api.jottacloud.com/auth/v1/token"
	cachePrefix                 = "rclone-jcmd5-"
	rcloneClientID              = "nibfk8biu12ju7hpqomr8b1e40"
	rcloneEncryptedClientSecret = "Vp8eAv7eVElMnQwN-kgU9cbhgApNDaMqWdlDi5qFydlQoji4JBxrGMF2"
	configUsername              = "user"
)

var (
	// Description of how to auth for this app for a personal account
	oauthConfig = &oauth2.Config{
		Endpoint: oauth2.Endpoint{
			AuthURL:  tokenURL,
			TokenURL: tokenURL,
		},
		ClientID:     rcloneClientID,
		ClientSecret: obscure.MustReveal(rcloneEncryptedClientSecret),
		RedirectURL:  oauthutil.RedirectLocalhostURL,
	}
)

// Register with Fs
func init() {
	fs.Register(&fs.RegInfo{
		Name:        "jottacloud",
		Description: "JottaCloud",
		NewFs:       NewFs,
		Config: func(name string, m configmap.Mapper) {
			tokenString, ok := m.Get("token")
			if ok && tokenString != "" {
				fmt.Printf("Already have a token - refresh?\n")
				if !config.Confirm() {
					return
				}
			}

			username, ok := m.Get(configUsername)
			if !ok {
				fs.Errorf(nil, "No username defined")
			}
			// Password
			password := config.GetPassword("Your Jottacloud password is only required during config and will not be stored.")
			if password == "" {
				fs.Errorf(nil, "Password must not be empty!")
			}

			srv := rest.NewClient(fshttp.NewClient(fs.Config))
			values := url.Values{}
			values.Set("grant_type", "PASSWORD")
			values.Set("password", password)
			values.Set("username", username)
			values.Set("client_id", oauthConfig.ClientID)
			values.Set("client_secret", oauthConfig.ClientSecret)
			opts := rest.Opts{
				Method:      "POST",
				RootURL:     oauthConfig.Endpoint.AuthURL,
				ContentType: "application/x-www-form-urlencoded",
				Parameters:  values,
			}

			var jsonToken api.TokenJSON
			resp, err := srv.CallJSON(&opts, nil, &jsonToken)
			if err != nil {
				fs.Errorf(nil, "Failed to get resource token: %v", err)
				return
			}
			if resp.StatusCode != 200 {
				fs.Errorf(nil, "Failed to get resource token: Got HTTP error code %d", resp.StatusCode)
				return
			}

			var token oauth2.Token
			token.AccessToken = jsonToken.AccessToken
			token.RefreshToken = jsonToken.RefreshToken
			token.TokenType = jsonToken.TokenType
			token.Expiry = time.Now().Add(time.Duration(jsonToken.ExpiresIn) * time.Second)

			// finally save them in the config
			err = oauthutil.PutToken(name, m, &token, true)
			if err != nil {
				fs.Errorf(nil, "Error while setting token: %s", err)
			}
		},
		Options: []fs.Option{{
			Name: configUsername,
			Help: "User Name:",
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
		}, {
			Name:     "md5_memory_limit",
			Help:     "Files bigger than this will be cached on disk to calculate the MD5 if required.",
			Default:  fs.SizeSuffix(10 * 1024 * 1024),
			Advanced: true,
		}, {
			Name:     "hard_delete",
			Help:     "Delete files permanently rather than putting them into the trash.",
			Default:  false,
			Advanced: true,
		}, {
			Name:     "unlink",
			Help:     "Remove existing public link to file/folder with link command rather than creating.\nDefault is false, meaning link command will create or retrieve public link.",
			Default:  false,
			Advanced: true,
		}, {
			Name:     "upload_resume_limit",
			Help:     "Files bigger than this can be resumed if the upload failes.",
			Default:  fs.SizeSuffix(10 * 1024 * 1024),
			Advanced: true,
		}},
	})
}

// Options defines the configuration for this backend
type Options struct {
	User               string        `config:"user"`
	Mountpoint         string        `config:"mountpoint"`
	MD5MemoryThreshold fs.SizeSuffix `config:"md5_memory_limit"`
	HardDelete         bool          `config:"hard_delete"`
	Unlink             bool          `config:"unlink"`
	UploadThreshold    fs.SizeSuffix `config:"upload_resume_limit"`
}

// Fs represents a remote jottacloud
type Fs struct {
	name         string
	root         string
	user         string
	opt          Options
	features     *fs.Features
	endpointURL  string
	srv          *rest.Client
	apiSrv       *rest.Client
	pacer        *pacer.Pacer
	tokenRenewer *oauthutil.Renew // renew the token on expiry
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

// getAccountInfo retrieves account information
func (f *Fs) getAccountInfo() (info *api.AccountInfo, err error) {
	opts := rest.Opts{
		Method: "GET",
		Path:   urlPathEscape(f.user),
	}

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

// setEndpointUrl reads the account id and generates the API endpoint URL
func (f *Fs) setEndpointURL(mountpoint string) (err error) {
	info, err := f.getAccountInfo()
	if err != nil {
		return errors.Wrap(err, "failed to get endpoint url")
	}
	f.endpointURL = urlPathEscape(path.Join(info.Username, defaultDevice, mountpoint))
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

// Jottacloud want's '+' to be URL encoded even though the RFC states it's not reserved
func urlPathEscape(in string) string {
	return strings.Replace(rest.URLPathEscape(in), "+", "%2B", -1)
}

// filePathRaw returns an unescaped file path (f.root, file)
func (f *Fs) filePathRaw(file string) string {
	return path.Join(f.endpointURL, replaceReservedChars(path.Join(f.root, file)))
}

// filePath returns a escaped file path (f.root, file)
func (f *Fs) filePath(file string) string {
	return urlPathEscape(f.filePathRaw(file))
}

// filePath returns a escaped file path (f.root, remote)
func (o *Object) filePath() string {
	return o.fs.filePath(o.remote)
}

// Jottacloud requires the grant_type 'refresh_token' string
// to be uppercase and throws a 400 Bad Request if we use the
// lower case used by the oauth2 module
//
// This filter catches all refresh requests, reads the body,
// changes the case and then sends it on
func grantTypeFilter(req *http.Request) {
	if tokenURL == req.URL.String() {
		// read the entire body
		refreshBody, err := ioutil.ReadAll(req.Body)
		if err != nil {
			return
		}
		_ = req.Body.Close()

		// make the refesh token upper case
		refreshBody = []byte(strings.Replace(string(refreshBody), "grant_type=refresh_token", "grant_type=REFRESH_TOKEN", 1))

		// set the new ReadCloser (with a dummy Close())
		req.Body = ioutil.NopCloser(bytes.NewReader(refreshBody))
	}
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

	// the oauth client for the api servers needs
	// a filter to fix the grant_type issues (see above)
	baseClient := fshttp.NewClient(fs.Config)
	if do, ok := baseClient.Transport.(interface {
		SetRequestFilter(f func(req *http.Request))
	}); ok {
		do.SetRequestFilter(grantTypeFilter)
	} else {
		fs.Debugf(name+":", "Couldn't add request filter - uploads will fail")
	}
	oAuthClient, ts, err := oauthutil.NewClientWithBaseClient(name, m, oauthConfig, baseClient)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to configure Jottacloud oauth client")
	}

	f := &Fs{
		name: name,
		root: root,
		user: opt.User,
		opt:  *opt,
		//endpointURL: rest.URLPathEscape(path.Join(user, defaultDevice, opt.Mountpoint)),
		srv:    rest.NewClient(oAuthClient).SetRoot(rootURL),
		apiSrv: rest.NewClient(oAuthClient).SetRoot(apiURL),
		pacer:  pacer.New().SetMinSleep(minSleep).SetMaxSleep(maxSleep).SetDecayConstant(decayConstant),
	}
	f.features = (&fs.Features{
		CaseInsensitive:         true,
		CanHaveEmptyDirectories: true,
		ReadMimeType:            true,
		WriteMimeType:           true,
	}).Fill(f)
	f.srv.SetErrorHandler(errorHandler)

	// Renew the token in the background
	f.tokenRenewer = oauthutil.NewRenew(f.String(), ts, func() error {
		_, err := f.readMetaDataForPath("")
		return err
	})

	err = f.setEndpointURL(opt.Mountpoint)
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
		err = o.readMetaData(false) // reads info and meta, returning an error
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
	//fmt.Printf("List: %s\n", f.filePath(dir))
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

// listFileDirFn is called from listFileDir to handle an object.
type listFileDirFn func(fs.DirEntry) error

// List the objects and directories into entries, from a
// special kind of JottaFolder representing a FileDirLis
func (f *Fs) listFileDir(remoteStartPath string, startFolder *api.JottaFolder, fn listFileDirFn) error {
	pathPrefix := "/" + f.filePathRaw("") // Non-escaped prefix of API paths to be cut off, to be left with the remote path including the remoteStartPath
	pathPrefixLength := len(pathPrefix)
	startPath := path.Join(pathPrefix, remoteStartPath) // Non-escaped API path up to and including remoteStartPath, to decide if it should be created as a new dir object
	startPathLength := len(startPath)
	for i := range startFolder.Folders {
		folder := &startFolder.Folders[i]
		if folder.Deleted {
			return nil
		}
		folderPath := restoreReservedChars(path.Join(folder.Path, folder.Name))
		folderPathLength := len(folderPath)
		var remoteDir string
		if folderPathLength > pathPrefixLength {
			remoteDir = folderPath[pathPrefixLength+1:]
			if folderPathLength > startPathLength {
				d := fs.NewDir(remoteDir, time.Time(folder.ModifiedAt))
				err := fn(d)
				if err != nil {
					return err
				}
			}
		}
		for i := range folder.Files {
			file := &folder.Files[i]
			if file.Deleted || file.State != "COMPLETED" {
				continue
			}
			remoteFile := path.Join(remoteDir, restoreReservedChars(file.Name))
			o, err := f.newObjectWithInfo(remoteFile, file)
			if err != nil {
				return err
			}
			err = fn(o)
			if err != nil {
				return err
			}
		}
	}
	return nil
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
func (f *Fs) ListR(dir string, callback fs.ListRCallback) (err error) {
	opts := rest.Opts{
		Method:     "GET",
		Path:       f.filePath(dir),
		Parameters: url.Values{},
	}
	opts.Parameters.Set("mode", "list")

	var resp *http.Response
	var result api.JottaFolder // Could be JottaFileDirList, but JottaFolder is close enough
	err = f.pacer.Call(func() (bool, error) {
		resp, err = f.srv.CallXML(&opts, nil, &result)
		return shouldRetry(resp, err)
	})
	if err != nil {
		if apiErr, ok := err.(*api.Error); ok {
			// does not exist
			if apiErr.StatusCode == http.StatusNotFound {
				return fs.ErrorDirNotFound
			}
		}
		return errors.Wrap(err, "couldn't list files")
	}
	list := walk.NewListRHelper(callback)
	err = f.listFileDir(dir, &result, func(entry fs.DirEntry) error {
		return list.Add(entry)
	})
	if err != nil {
		return err
	}
	return list.Flush()
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

	if f.opt.HardDelete {
		opts.Parameters.Set("rmDir", "true")
	} else {
		opts.Parameters.Set("dlDir", "true")
	}

	var resp *http.Response
	err = f.pacer.Call(func() (bool, error) {
		resp, err = f.srv.Call(&opts)
		return shouldRetry(resp, err)
	})
	if err != nil {
		return errors.Wrap(err, "couldn't purge directory")
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
		return nil, errors.Wrap(err, "couldn't copy file")
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
		return nil, errors.Wrap(err, "couldn't move file")
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
		return errors.Wrap(err, "couldn't move directory")
	}
	return nil
}

// PublicLink generates a public link to the remote path (usually readable by anyone)
func (f *Fs) PublicLink(remote string) (link string, err error) {
	opts := rest.Opts{
		Method:     "GET",
		Path:       f.filePath(remote),
		Parameters: url.Values{},
	}

	if f.opt.Unlink {
		opts.Parameters.Set("mode", "disableShare")
	} else {
		opts.Parameters.Set("mode", "enableShare")
	}

	var resp *http.Response
	var result api.JottaFile
	err = f.pacer.Call(func() (bool, error) {
		resp, err = f.srv.CallXML(&opts, nil, &result)
		return shouldRetry(resp, err)
	})

	if apiErr, ok := err.(*api.Error); ok {
		// does not exist
		if apiErr.StatusCode == http.StatusNotFound {
			return "", fs.ErrorObjectNotFound
		}
	}
	if err != nil {
		if f.opt.Unlink {
			return "", errors.Wrap(err, "couldn't remove public link")
		}
		return "", errors.Wrap(err, "couldn't create public link")
	}
	if f.opt.Unlink {
		if result.PublicSharePath != "" {
			return "", errors.Errorf("couldn't remove public link - %q", result.PublicSharePath)
		}
		return "", nil
	}
	if result.PublicSharePath == "" {
		return "", errors.New("couldn't create public link - no link path received")
	}
	link = path.Join(baseURL, result.PublicSharePath)
	return link, nil
}

// About gets quota information
func (f *Fs) About() (*fs.Usage, error) {
	info, err := f.getAccountInfo()
	if err != nil {
		return nil, err
	}

	usage := &fs.Usage{
		Used: fs.NewUsageValue(info.Usage),
	}
	if info.Capacity > 0 {
		usage.Total = fs.NewUsageValue(info.Capacity)
		usage.Free = fs.NewUsageValue(info.Capacity - info.Usage)
	}
	return usage, nil
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
	err := o.readMetaData(false)
	if err != nil {
		fs.Logf(o, "Failed to read metadata: %v", err)
		return 0
	}
	return o.size
}

// MimeType of an Object if known, "" otherwise
func (o *Object) MimeType() string {
	return o.mimeType
}

// setMetaData sets the metadata from info
func (o *Object) setMetaData(info *api.JottaFile) (err error) {
	o.hasMetaData = true
	o.size = int64(info.Size)
	o.md5 = info.MD5
	o.mimeType = info.MimeType
	o.modTime = time.Time(info.ModifiedAt)
	return nil
}

func (o *Object) readMetaData(force bool) (err error) {
	if o.hasMetaData && !force {
		return nil
	}
	info, err := o.fs.readMetaDataForPath(o.remote)
	if err != nil {
		return err
	}
	if info.Deleted {
		return fs.ErrorObjectNotFound
	}
	return o.setMetaData(info)
}

// ModTime returns the modification time of the object
//
// It attempts to read the objects mtime and if that isn't present the
// LastModified returned in the http headers
func (o *Object) ModTime() time.Time {
	err := o.readMetaData(false)
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

// Read the md5 of in returning a reader which will read the same contents
//
// The cleanup function should be called when out is finished with
// regardless of whether this function returned an error or not.
func readMD5(in io.Reader, size, threshold int64) (md5sum string, out io.Reader, cleanup func(), err error) {
	// we need a MD5
	md5Hasher := md5.New()
	// use the teeReader to write to the local file AND caclulate the MD5 while doing so
	teeReader := io.TeeReader(in, md5Hasher)

	// nothing to clean up by default
	cleanup = func() {}

	// don't cache small files on disk to reduce wear of the disk
	if size > threshold {
		var tempFile *os.File

		// create the cache file
		tempFile, err = ioutil.TempFile("", cachePrefix)
		if err != nil {
			return
		}

		_ = os.Remove(tempFile.Name()) // Delete the file - may not work on Windows

		// clean up the file after we are done downloading
		cleanup = func() {
			// the file should normally already be close, but just to make sure
			_ = tempFile.Close()
			_ = os.Remove(tempFile.Name()) // delete the cache file after we are done - may be deleted already
		}

		// copy the ENTIRE file to disc and calculate the MD5 in the process
		if _, err = io.Copy(tempFile, teeReader); err != nil {
			return
		}
		// jump to the start of the local file so we can pass it along
		if _, err = tempFile.Seek(0, 0); err != nil {
			return
		}

		// replace the already read source with a reader of our cached file
		out = tempFile
	} else {
		// that's a small file, just read it into memory
		var inData []byte
		inData, err = ioutil.ReadAll(teeReader)
		if err != nil {
			return
		}

		// set the reader to our read memory block
		out = bytes.NewReader(inData)
	}
	return hex.EncodeToString(md5Hasher.Sum(nil)), out, cleanup, nil
}

// Update the object with the contents of the io.Reader, modTime and size
//
// If existing is set then it updates the object rather than creating a new one
//
// The new object may have been created if an error is returned
func (o *Object) Update(in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (err error) {
	size := src.Size()
	md5String, err := src.Hash(hash.MD5)
	if err != nil || md5String == "" {
		// unwrap the accounting from the input, we use wrap to put it
		// back on after the buffering
		var wrap accounting.WrapFn
		in, wrap = accounting.UnWrap(in)
		var cleanup func()
		md5String, in, cleanup, err = readMD5(in, size, int64(o.fs.opt.MD5MemoryThreshold))
		defer cleanup()
		if err != nil {
			return errors.Wrap(err, "failed to calculate MD5")
		}
		// Wrap the accounting back onto the stream
		in = wrap(in)
	}

	// use the api to allocate the file first and get resume / deduplication info
	var resp *http.Response
	opts := rest.Opts{
		Method:       "POST",
		Path:         "allocate",
		ExtraHeaders: make(map[string]string),
	}
	fileDate := api.Time(src.ModTime()).APIString()

	// the allocate request
	var request = api.AllocateFileRequest{
		Bytes:    size,
		Created:  fileDate,
		Modified: fileDate,
		Md5:      md5String,
		Path:     path.Join(o.fs.opt.Mountpoint, replaceReservedChars(path.Join(o.fs.root, o.remote))),
	}

	// send it
	var response api.AllocateFileResponse
	err = o.fs.pacer.CallNoRetry(func() (bool, error) {
		resp, err = o.fs.apiSrv.CallJSON(&opts, &request, &response)
		return shouldRetry(resp, err)
	})
	if err != nil {
		return err
	}

	// If the file state is INCOMPLETE and CORRPUT, try to upload a then
	if response.State != "COMPLETED" {
		// how much do we still have to upload?
		remainingBytes := size - response.ResumePos
		opts = rest.Opts{
			Method:        "POST",
			RootURL:       response.UploadURL,
			ContentLength: &remainingBytes,
			ContentType:   "application/octet-stream",
			Body:          in,
			ExtraHeaders:  make(map[string]string),
		}
		if response.ResumePos != 0 {
			opts.ExtraHeaders["Range"] = "bytes=" + strconv.FormatInt(response.ResumePos, 10) + "-" + strconv.FormatInt(size-1, 10)
		}

		// copy the already uploaded bytes into the trash :)
		var result api.UploadResponse
		_, err = io.CopyN(ioutil.Discard, in, response.ResumePos)
		if err != nil {
			return err
		}

		// send the remaining bytes
		resp, err = o.fs.apiSrv.CallJSON(&opts, nil, &result)
		if err != nil {
			return err
		}

		// finally update the meta data
		o.hasMetaData = true
		o.size = int64(result.Bytes)
		o.md5 = result.Md5
		o.modTime = time.Unix(result.Modified/1000, 0)
	} else {
		// If the file state is COMPLETE we don't need to upload it because the file was allready found but we still ned to update our metadata
		return o.readMetaData(true)
	}

	return nil
}

// Remove an object
func (o *Object) Remove() error {
	opts := rest.Opts{
		Method:     "POST",
		Path:       o.filePath(),
		Parameters: url.Values{},
		NoResponse: true,
	}

	if o.fs.opt.HardDelete {
		opts.Parameters.Set("rm", "true")
	} else {
		opts.Parameters.Set("dl", "true")
	}

	return o.fs.pacer.Call(func() (bool, error) {
		resp, err := o.fs.srv.CallXML(&opts, nil, nil)
		return shouldRetry(resp, err)
	})
}

// Check the interfaces are satisfied
var (
	_ fs.Fs           = (*Fs)(nil)
	_ fs.Purger       = (*Fs)(nil)
	_ fs.Copier       = (*Fs)(nil)
	_ fs.Mover        = (*Fs)(nil)
	_ fs.DirMover     = (*Fs)(nil)
	_ fs.ListRer      = (*Fs)(nil)
	_ fs.PublicLinker = (*Fs)(nil)
	_ fs.Abouter      = (*Fs)(nil)
	_ fs.Object       = (*Object)(nil)
	_ fs.MimeTyper    = (*Object)(nil)
)
