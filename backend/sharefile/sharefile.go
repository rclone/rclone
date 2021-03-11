// Package sharefile provides an interface to the Citrix Sharefile
// object storage system.
package sharefile

//go:generate ./update-timezone.sh

/* NOTES

## for docs

Detail standard/chunked/streaming uploads?

## Bugs in API

The times in updateItem are being parsed in EST/DST local time
updateItem only sets times accurate to 1 second

https://community.sharefilesupport.com/citrixsharefile/topics/bug-report-for-update-item-patch-items-id-setting-clientmodifieddate-ignores-timezone-and-milliseconds

When doing a rename+move directory, the server appears to do the
rename first in the local directory which can overwrite files of the
same name in the local directory.

https://community.sharefilesupport.com/citrixsharefile/topics/bug-report-for-update-item-patch-items-id-file-overwrite-under-certain-conditions

The Copy command can't change the name at the same time which means we
have to copy via a temporary directory.

https://community.sharefilesupport.com/citrixsharefile/topics/copy-item-needs-to-be-able-to-set-a-new-name

## Allowed characters

https://api.sharefile.com/rest/index/odata.aspx

$select to limit returned fields
https://www.odata.org/documentation/odata-version-3-0/odata-version-3-0-core-protocol/#theselectsystemqueryoption

Also $filter to select only things we need

https://support.citrix.com/article/CTX234774

The following characters should not be used in folder or file names.

\
/
.
,
:
;
*
?
"
<
>
A filename ending with a period without an extension
File names with leading or trailing whitespaces.


// sharefile
stringNeedsEscaping = []byte{
	0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0A, 0x0B, 0x0C, 0x0D, 0x0E, 0x0F, 0x10, 0x11, 0x12, 0x13, 0x14, 0x15, 0x16, 0x17, 0x18, 0x19, 0x1A, 0x1B, 0x1C, 0x1D, 0x1E, 0x1F, 0x20, 0x2A, 0x2E, 0x2F, 0x3A, 0x3C, 0x3E, 0x3F, 0x7C, 0xEFBCBC
}
maxFileLength = 256
canWriteUnnormalized = true
canReadUnnormalized   = true
canReadRenormalized   = false
canStream = true

Which is control chars + [' ', '*', '.', '/', ':', '<', '>', '?', '|']
- also \ and "

*/

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"

	"github.com/pkg/errors"
	"github.com/rclone/rclone/backend/sharefile/api"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config"
	"github.com/rclone/rclone/fs/config/configmap"
	"github.com/rclone/rclone/fs/config/configstruct"
	"github.com/rclone/rclone/fs/config/obscure"
	"github.com/rclone/rclone/fs/fserrors"
	"github.com/rclone/rclone/fs/hash"
	"github.com/rclone/rclone/lib/dircache"
	"github.com/rclone/rclone/lib/encoder"
	"github.com/rclone/rclone/lib/oauthutil"
	"github.com/rclone/rclone/lib/pacer"
	"github.com/rclone/rclone/lib/random"
	"github.com/rclone/rclone/lib/rest"
	"golang.org/x/oauth2"
)

const (
	rcloneClientID              = "djQUPlHTUM9EvayYBWuKC5IrVIoQde46"
	rcloneEncryptedClientSecret = "v7572bKhUindQL3yDnUAebmgP-QxiwT38JLxVPolcZBl6SSs329MtFzH73x7BeELmMVZtneUPvALSopUZ6VkhQ"
	minSleep                    = 10 * time.Millisecond
	maxSleep                    = 2 * time.Second
	decayConstant               = 2              // bigger for slower decay, exponential
	apiPath                     = "/sf/v3"       // add to endpoint to get API path
	tokenPath                   = "/oauth/token" // add to endpoint to get Token path
	minChunkSize                = 256 * fs.KibiByte
	maxChunkSize                = 2 * fs.GibiByte
	defaultChunkSize            = 64 * fs.MebiByte
	defaultUploadCutoff         = 128 * fs.MebiByte
)

// Generate a new oauth2 config which we will update when we know the TokenURL
func newOauthConfig(tokenURL string) *oauth2.Config {
	return &oauth2.Config{
		Scopes: nil,
		Endpoint: oauth2.Endpoint{
			AuthURL:  "https://secure.sharefile.com/oauth/authorize",
			TokenURL: tokenURL,
		},
		ClientID:     rcloneClientID,
		ClientSecret: obscure.MustReveal(rcloneEncryptedClientSecret),
		RedirectURL:  oauthutil.RedirectPublicSecureURL,
	}
}

// Register with Fs
func init() {
	fs.Register(&fs.RegInfo{
		Name:        "sharefile",
		Description: "Citrix Sharefile",
		NewFs:       NewFs,
		Config: func(ctx context.Context, name string, m configmap.Mapper) {
			oauthConfig := newOauthConfig("")
			checkAuth := func(oauthConfig *oauth2.Config, auth *oauthutil.AuthResult) error {
				if auth == nil || auth.Form == nil {
					return errors.New("endpoint not found in response")
				}
				subdomain := auth.Form.Get("subdomain")
				apicp := auth.Form.Get("apicp")
				if subdomain == "" || apicp == "" {
					return errors.Errorf("subdomain or apicp not found in response: %+v", auth.Form)
				}
				endpoint := "https://" + subdomain + "." + apicp
				m.Set("endpoint", endpoint)
				oauthConfig.Endpoint.TokenURL = endpoint + tokenPath
				return nil
			}
			opt := oauthutil.Options{
				CheckAuth: checkAuth,
			}
			err := oauthutil.Config(ctx, "sharefile", name, m, oauthConfig, &opt)
			if err != nil {
				log.Fatalf("Failed to configure token: %v", err)
			}
		},
		Options: []fs.Option{{
			Name:     "upload_cutoff",
			Help:     "Cutoff for switching to multipart upload.",
			Default:  defaultUploadCutoff,
			Advanced: true,
		}, {
			Name: "root_folder_id",
			Help: `ID of the root folder

Leave blank to access "Personal Folders".  You can use one of the
standard values here or any folder ID (long hex number ID).`,
			Examples: []fs.OptionExample{{
				Value: "",
				Help:  `Access the Personal Folders. (Default)`,
			}, {
				Value: "favorites",
				Help:  "Access the Favorites folder.",
			}, {
				Value: "allshared",
				Help:  "Access all the shared folders.",
			}, {
				Value: "connectors",
				Help:  "Access all the individual connectors.",
			}, {
				Value: "top",
				Help:  "Access the home, favorites, and shared folders as well as the connectors.",
			}},
		}, {
			Name:    "chunk_size",
			Default: defaultChunkSize,
			Help: `Upload chunk size. Must a power of 2 >= 256k.

Making this larger will improve performance, but note that each chunk
is buffered in memory one per transfer.

Reducing this will reduce memory usage but decrease performance.`,
			Advanced: true,
		}, {
			Name: "endpoint",
			Help: `Endpoint for API calls.

This is usually auto discovered as part of the oauth process, but can
be set manually to something like: https://XXX.sharefile.com
`,
			Advanced: true,
			Default:  "",
		}, {
			Name:     config.ConfigEncoding,
			Help:     config.ConfigEncodingHelp,
			Advanced: true,
			Default: (encoder.Base |
				encoder.EncodeWin | // :?"*<>|
				encoder.EncodeBackSlash | // \
				encoder.EncodeCtl |
				encoder.EncodeRightSpace |
				encoder.EncodeRightPeriod |
				encoder.EncodeLeftSpace |
				encoder.EncodeLeftPeriod |
				encoder.EncodeInvalidUtf8),
		}},
	})
}

// Options defines the configuration for this backend
type Options struct {
	RootFolderID string               `config:"root_folder_id"`
	UploadCutoff fs.SizeSuffix        `config:"upload_cutoff"`
	ChunkSize    fs.SizeSuffix        `config:"chunk_size"`
	Endpoint     string               `config:"endpoint"`
	Enc          encoder.MultiEncoder `config:"encoding"`
}

// Fs represents a remote cloud storage system
type Fs struct {
	name         string             // name of this remote
	root         string             // the path we are working on
	opt          Options            // parsed options
	ci           *fs.ConfigInfo     // global config
	features     *fs.Features       // optional features
	srv          *rest.Client       // the connection to the server
	dirCache     *dircache.DirCache // Map of directory path to directory id
	pacer        *fs.Pacer          // pacer for API calls
	bufferTokens chan []byte        // control concurrency of multipart uploads
	tokenRenewer *oauthutil.Renew   // renew the token on expiry
	rootID       string             // ID of the users root folder
	location     *time.Location     // timezone of server for SetModTime workaround
}

// Object describes a file
type Object struct {
	fs          *Fs       // what this object is part of
	remote      string    // The remote path
	hasMetaData bool      // metadata is present and correct
	size        int64     // size of the object
	modTime     time.Time // modification time of the object
	id          string    // ID of the object
	md5         string    // hash of the object
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
	return fmt.Sprintf("sharefile root '%s'", f.root)
}

// Features returns the optional features of this Fs
func (f *Fs) Features() *fs.Features {
	return f.features
}

// parsePath parses a sharefile 'url'
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
func shouldRetry(ctx context.Context, resp *http.Response, err error) (bool, error) {
	if fserrors.ContextError(ctx, &err) {
		return false, err
	}
	return fserrors.ShouldRetry(err) || fserrors.ShouldRetryHTTP(resp, retryErrorCodes), err
}

// Reads the metadata for the id passed in.  If id is "" then it returns the root
// if path is not "" then the item read use id as the root and the path is relative
func (f *Fs) readMetaDataForIDPath(ctx context.Context, id, path string, directoriesOnly bool, filesOnly bool) (info *api.Item, err error) {
	opts := rest.Opts{
		Method: "GET",
		Path:   "/Items",
		Parameters: url.Values{
			"$select": {api.ListRequestSelect},
		},
	}
	if id != "" {
		opts.Path += "(" + id + ")"
	}
	if path != "" {
		opts.Path += "/ByPath"
		opts.Parameters.Set("path", "/"+f.opt.Enc.FromStandardPath(path))
	}
	var item api.Item
	var resp *http.Response
	err = f.pacer.Call(func() (bool, error) {
		resp, err = f.srv.CallJSON(ctx, &opts, nil, &item)
		return shouldRetry(ctx, resp, err)
	})
	if err != nil {
		if resp != nil && resp.StatusCode == http.StatusNotFound {
			if filesOnly {
				return nil, fs.ErrorObjectNotFound
			}
			return nil, fs.ErrorDirNotFound
		}
		return nil, errors.Wrap(err, "couldn't find item")
	}
	if directoriesOnly && item.Type != api.ItemTypeFolder {
		return nil, fs.ErrorIsFile
	}
	if filesOnly && item.Type != api.ItemTypeFile {
		return nil, fs.ErrorNotAFile
	}
	return &item, nil
}

// Reads the metadata for the id passed in.  If id is "" then it returns the root
func (f *Fs) readMetaDataForID(ctx context.Context, id string, directoriesOnly bool, filesOnly bool) (info *api.Item, err error) {
	return f.readMetaDataForIDPath(ctx, id, "", directoriesOnly, filesOnly)
}

// readMetaDataForPath reads the metadata from the path
func (f *Fs) readMetaDataForPath(ctx context.Context, path string, directoriesOnly bool, filesOnly bool) (info *api.Item, err error) {
	leaf, directoryID, err := f.dirCache.FindPath(ctx, path, false)
	if err != nil {
		if err == fs.ErrorDirNotFound {
			return nil, fs.ErrorObjectNotFound
		}
		return nil, err
	}
	return f.readMetaDataForIDPath(ctx, directoryID, leaf, directoriesOnly, filesOnly)
}

// errorHandler parses a non 2xx error response into an error
func errorHandler(resp *http.Response) error {
	body, err := rest.ReadBody(resp)
	if err != nil {
		body = nil
	}
	var e = api.Error{
		Code:   fmt.Sprint(resp.StatusCode),
		Reason: resp.Status,
	}
	e.Message.Lang = "en"
	e.Message.Value = string(body)
	if body != nil {
		_ = json.Unmarshal(body, &e)
	}
	return &e
}

func checkUploadChunkSize(cs fs.SizeSuffix) error {
	if cs < minChunkSize {
		return errors.Errorf("ChunkSize: %s is less than %s", cs, minChunkSize)
	}
	if cs > maxChunkSize {
		return errors.Errorf("ChunkSize: %s is greater than %s", cs, maxChunkSize)
	}
	return nil
}

func (f *Fs) setUploadChunkSize(cs fs.SizeSuffix) (old fs.SizeSuffix, err error) {
	err = checkUploadChunkSize(cs)
	if err == nil {
		old, f.opt.ChunkSize = f.opt.ChunkSize, cs
		f.fillBufferTokens() // reset the buffer tokens
	}
	return
}

func checkUploadCutoff(cs fs.SizeSuffix) error {
	return nil
}

func (f *Fs) setUploadCutoff(cs fs.SizeSuffix) (old fs.SizeSuffix, err error) {
	err = checkUploadCutoff(cs)
	if err == nil {
		old, f.opt.UploadCutoff = f.opt.UploadCutoff, cs
	}
	return
}

// NewFs constructs an Fs from the path, container:path
func NewFs(ctx context.Context, name, root string, m configmap.Mapper) (fs.Fs, error) {
	// Parse config into Options struct
	opt := new(Options)
	err := configstruct.Set(m, opt)
	if err != nil {
		return nil, err
	}

	// Check parameters OK
	if opt.Endpoint == "" {
		return nil, errors.New("endpoint not set: rebuild the remote or set manually")
	}
	err = checkUploadChunkSize(opt.ChunkSize)
	if err != nil {
		return nil, err
	}
	err = checkUploadCutoff(opt.UploadCutoff)
	if err != nil {
		return nil, err
	}

	root = parsePath(root)

	oauthConfig := newOauthConfig(opt.Endpoint + tokenPath)
	var client *http.Client
	var ts *oauthutil.TokenSource
	client, ts, err = oauthutil.NewClient(ctx, name, m, oauthConfig)
	if err != nil {
		return nil, errors.Wrap(err, "failed to configure sharefile")
	}

	ci := fs.GetConfig(ctx)
	f := &Fs{
		name:  name,
		root:  root,
		opt:   *opt,
		ci:    ci,
		srv:   rest.NewClient(client).SetRoot(opt.Endpoint + apiPath),
		pacer: fs.NewPacer(ctx, pacer.NewDefault(pacer.MinSleep(minSleep), pacer.MaxSleep(maxSleep), pacer.DecayConstant(decayConstant))),
	}
	f.features = (&fs.Features{
		CaseInsensitive:         true,
		CanHaveEmptyDirectories: true,
		ReadMimeType:            false,
	}).Fill(ctx, f)
	f.srv.SetErrorHandler(errorHandler)
	f.fillBufferTokens()

	// Renew the token in the background
	if ts != nil {
		f.tokenRenewer = oauthutil.NewRenew(f.String(), ts, func() error {
			_, err := f.List(ctx, "")
			return err
		})
	}

	// Load the server timezone from an internal file
	// Used to correct the time in SetModTime
	const serverTimezone = "America/New_York"
	timezone, err := tzdata.Open(serverTimezone)
	if err != nil {
		return nil, errors.Wrap(err, "failed to open timezone db")
	}
	tzdata, err := ioutil.ReadAll(timezone)
	if err != nil {
		return nil, errors.Wrap(err, "failed to read timezone")
	}
	_ = timezone.Close()
	f.location, err = time.LoadLocationFromTZData(serverTimezone, tzdata)
	if err != nil {
		return nil, errors.Wrap(err, "failed to load location from timezone")
	}

	// Find ID of user's root folder
	if opt.RootFolderID == "" {
		item, err := f.readMetaDataForID(ctx, opt.RootFolderID, true, false)
		if err != nil {
			return nil, errors.Wrap(err, "couldn't find root ID")
		}
		f.rootID = item.ID
	} else {
		f.rootID = opt.RootFolderID
	}

	// Get rootID
	f.dirCache = dircache.New(root, f.rootID, f)

	// Find the current root
	err = f.dirCache.FindRoot(ctx, false)
	if err != nil {
		// Assume it is a file
		newRoot, remote := dircache.SplitPath(root)
		tempF := *f
		tempF.dirCache = dircache.New(newRoot, f.rootID, &tempF)
		tempF.root = newRoot
		// Make new Fs which is the parent
		err = tempF.dirCache.FindRoot(ctx, false)
		if err != nil {
			// No root so return old f
			return f, nil
		}
		_, err := tempF.newObjectWithInfo(ctx, remote, nil)
		if err != nil {
			if err == fs.ErrorObjectNotFound {
				// File doesn't exist so return old f
				return f, nil
			}
			return nil, err
		}
		f.features.Fill(ctx, &tempF)
		// XXX: update the old f here instead of returning tempF, since
		// `features` were already filled with functions having *f as a receiver.
		// See https://github.com/rclone/rclone/issues/2182
		f.dirCache = tempF.dirCache
		f.root = tempF.root
		// return an error with an fs which points to the parent
		return f, fs.ErrorIsFile
	}
	return f, nil
}

// Fill up (or reset) the buffer tokens
func (f *Fs) fillBufferTokens() {
	f.bufferTokens = make(chan []byte, f.ci.Transfers)
	for i := 0; i < f.ci.Transfers; i++ {
		f.bufferTokens <- nil
	}
}

// getUploadBlock gets a block from the pool of size chunkSize
func (f *Fs) getUploadBlock() []byte {
	buf := <-f.bufferTokens
	if buf == nil {
		buf = make([]byte, f.opt.ChunkSize)
	}
	// fs.Debugf(f, "Getting upload block %p", buf)
	return buf
}

// putUploadBlock returns a block to the pool of size chunkSize
func (f *Fs) putUploadBlock(buf []byte) {
	buf = buf[:cap(buf)]
	if len(buf) != int(f.opt.ChunkSize) {
		panic("bad blocksize returned to pool")
	}
	// fs.Debugf(f, "Returning upload block %p", buf)
	f.bufferTokens <- buf
}

// Return an Object from a path
//
// If it can't be found it returns the error fs.ErrorObjectNotFound.
func (f *Fs) newObjectWithInfo(ctx context.Context, remote string, info *api.Item) (fs.Object, error) {
	o := &Object{
		fs:     f,
		remote: remote,
	}
	var err error
	if info != nil {
		// Set info
		err = o.setMetaData(info)
	} else {
		err = o.readMetaData(ctx) // reads info and meta, returning an error
	}
	if err != nil {
		return nil, err
	}
	return o, nil
}

// NewObject finds the Object at remote.  If it can't be found
// it returns the error fs.ErrorObjectNotFound.
func (f *Fs) NewObject(ctx context.Context, remote string) (fs.Object, error) {
	return f.newObjectWithInfo(ctx, remote, nil)
}

// FindLeaf finds a directory of name leaf in the folder with ID pathID
func (f *Fs) FindLeaf(ctx context.Context, pathID, leaf string) (pathIDOut string, found bool, err error) {
	if pathID == "top" {
		// Find the leaf in pathID
		found, err = f.listAll(ctx, pathID, true, false, func(item *api.Item) bool {
			if item.Name == leaf {
				pathIDOut = item.ID
				return true
			}
			return false
		})
		return pathIDOut, found, err
	}
	info, err := f.readMetaDataForIDPath(ctx, pathID, leaf, true, false)
	if err == nil {
		found = true
		pathIDOut = info.ID
	} else if err == fs.ErrorDirNotFound {
		err = nil // don't return an error if not found
	}
	return pathIDOut, found, err
}

// CreateDir makes a directory with pathID as parent and name leaf
func (f *Fs) CreateDir(ctx context.Context, pathID, leaf string) (newID string, err error) {
	var resp *http.Response
	leaf = f.opt.Enc.FromStandardName(leaf)
	var req = api.Item{
		Name:      leaf,
		FileName:  leaf,
		CreatedAt: time.Now(),
	}
	var info api.Item
	opts := rest.Opts{
		Method: "POST",
		Path:   "/Items(" + pathID + ")/Folder",
		Parameters: url.Values{
			"$select":     {api.ListRequestSelect},
			"overwrite":   {"false"},
			"passthrough": {"false"},
		},
	}
	err = f.pacer.Call(func() (bool, error) {
		resp, err = f.srv.CallJSON(ctx, &opts, &req, &info)
		return shouldRetry(ctx, resp, err)
	})
	if err != nil {
		return "", errors.Wrap(err, "CreateDir")
	}
	return info.ID, nil
}

// list the objects into the function supplied
//
// If directories is set it only sends directories
// User function to process a File item from listAll
//
// Should return true to finish processing
type listAllFn func(*api.Item) bool

// Lists the directory required calling the user function on each item found
//
// If the user fn ever returns true then it early exits with found = true
func (f *Fs) listAll(ctx context.Context, dirID string, directoriesOnly bool, filesOnly bool, fn listAllFn) (found bool, err error) {
	opts := rest.Opts{
		Method: "GET",
		Path:   "/Items(" + dirID + ")/Children",
		Parameters: url.Values{
			"$select": {api.ListRequestSelect},
		},
	}

	var result api.ListResponse
	var resp *http.Response
	err = f.pacer.Call(func() (bool, error) {
		resp, err = f.srv.CallJSON(ctx, &opts, nil, &result)
		return shouldRetry(ctx, resp, err)
	})
	if err != nil {
		return found, errors.Wrap(err, "couldn't list files")
	}
	for i := range result.Value {
		item := &result.Value[i]
		if item.Type == api.ItemTypeFolder {
			if filesOnly {
				continue
			}
		} else if item.Type == api.ItemTypeFile {
			if directoriesOnly {
				continue
			}
		} else {
			fs.Debugf(f, "Ignoring %q - unknown type %q", item.Name, item.Type)
			continue
		}
		item.Name = f.opt.Enc.ToStandardName(item.Name)
		if fn(item) {
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
func (f *Fs) List(ctx context.Context, dir string) (entries fs.DirEntries, err error) {
	directoryID, err := f.dirCache.FindDir(ctx, dir, false)
	if err != nil {
		return nil, err
	}
	var iErr error
	_, err = f.listAll(ctx, directoryID, false, false, func(info *api.Item) bool {
		remote := path.Join(dir, info.Name)
		if info.Type == api.ItemTypeFolder {
			// cache the directory ID for later lookups
			f.dirCache.Put(remote, info.ID)
			d := fs.NewDir(remote, info.CreatedAt).SetID(info.ID).SetSize(info.Size).SetItems(int64(info.FileCount))
			entries = append(entries, d)
		} else if info.Type == api.ItemTypeFile {
			o, err := f.newObjectWithInfo(ctx, remote, info)
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
// Returns the object, leaf, directoryID and error
//
// Used to create new objects
func (f *Fs) createObject(ctx context.Context, remote string, modTime time.Time, size int64) (o *Object, leaf string, directoryID string, err error) {
	// Create the directory for the object if it doesn't exist
	leaf, directoryID, err = f.dirCache.FindPath(ctx, remote, true)
	if err != nil {
		return
	}
	// Temporary Object under construction
	o = &Object{
		fs:     f,
		remote: remote,
	}
	return o, leaf, directoryID, nil
}

// Put the object
//
// Copy the reader in to the new object which is returned
//
// The new object may have been created if an error is returned
func (f *Fs) Put(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error) {
	existingObj, err := f.newObjectWithInfo(ctx, src.Remote(), nil)
	switch err {
	case nil:
		return existingObj, existingObj.Update(ctx, in, src, options...)
	case fs.ErrorObjectNotFound:
		// Not found so create it
		return f.PutUnchecked(ctx, in, src)
	default:
		return nil, err
	}
}

// PutStream uploads to the remote path with the modTime given of indeterminate size
func (f *Fs) PutStream(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error) {
	return f.Put(ctx, in, src, options...)
}

// PutUnchecked the object into the container
//
// This will produce an error if the object already exists
//
// Copy the reader in to the new object which is returned
//
// The new object may have been created if an error is returned
func (f *Fs) PutUnchecked(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error) {
	remote := src.Remote()
	size := src.Size()
	modTime := src.ModTime(ctx)

	o, _, _, err := f.createObject(ctx, remote, modTime, size)
	if err != nil {
		return nil, err
	}
	return o, o.Update(ctx, in, src, options...)
}

// Mkdir creates the container if it doesn't exist
func (f *Fs) Mkdir(ctx context.Context, dir string) error {
	_, err := f.dirCache.FindDir(ctx, dir, true)
	return err
}

// purgeCheck removes the directory, if check is set then it refuses
// to do so if it has anything in
func (f *Fs) purgeCheck(ctx context.Context, dir string, check bool) error {
	root := path.Join(f.root, dir)
	if root == "" {
		return errors.New("can't purge root directory")
	}
	dc := f.dirCache
	rootID, err := dc.FindDir(ctx, dir, false)
	if err != nil {
		return err
	}

	// need to check if empty as it will delete recursively by default
	if check {
		found, err := f.listAll(ctx, rootID, false, false, func(item *api.Item) bool {
			return true
		})
		if err != nil {
			return errors.Wrap(err, "purgeCheck")
		}
		if found {
			return fs.ErrorDirectoryNotEmpty
		}
	}

	err = f.remove(ctx, rootID)
	f.dirCache.FlushDir(dir)
	if err != nil {
		return err
	}
	return nil
}

// Rmdir deletes the root folder
//
// Returns an error if it isn't empty
func (f *Fs) Rmdir(ctx context.Context, dir string) error {
	return f.purgeCheck(ctx, dir, true)
}

// Precision return the precision of this Fs
func (f *Fs) Precision() time.Duration {
	// sharefile returns times accurate to the millisecond, but
	// for some reason these seem only accurate 2ms.
	// updateItem seems to only set times accurate to 1 second though.
	return time.Second // this doesn't appear to be documented anywhere
}

// Purge deletes all the files and the container
//
// Optional interface: Only implement this if you have a way of
// deleting all the files quicker than just running Remove() on the
// result of List()
func (f *Fs) Purge(ctx context.Context, dir string) error {
	return f.purgeCheck(ctx, dir, false)
}

// updateItem patches a file or folder
//
// if leaf = "" or directoryID = "" or modTime == nil then it will be
// left alone
//
// Note that this seems to work by renaming first, then moving to a
// new directory which means that it can overwrite existing objects
// :-(
func (f *Fs) updateItem(ctx context.Context, id, leaf, directoryID string, modTime *time.Time) (info *api.Item, err error) {
	// Move the object
	opts := rest.Opts{
		Method: "PATCH",
		Path:   "/Items(" + id + ")",
		Parameters: url.Values{
			"$select":   {api.ListRequestSelect},
			"overwrite": {"false"},
		},
	}
	leaf = f.opt.Enc.FromStandardName(leaf)
	// FIXME this appears to be a bug in the API
	//
	// If you set the modified time via PATCH then the server
	// appears to parse it as a local time for America/New_York
	//
	// However if you set it when uploading the file then it is fine...
	//
	// Also it only sets the time to 1 second resolution where it
	// uses 1ms resolution elsewhere
	if modTime != nil && f.location != nil {
		newTime := modTime.In(f.location)
		isoTime := newTime.Format(time.RFC3339Nano)
		// Chop TZ -05:00 off the end and replace with Z
		isoTime = isoTime[:len(isoTime)-6] + "Z"
		// Parse it back into a time
		newModTime, err := time.Parse(time.RFC3339Nano, isoTime)
		if err != nil {
			return nil, errors.Wrap(err, "updateItem: time parse")
		}
		modTime = &newModTime
	}
	update := api.UpdateItemRequest{
		Name:       leaf,
		FileName:   leaf,
		ModifiedAt: modTime,
	}
	if directoryID != "" {
		update.Parent = &api.Parent{
			ID: directoryID,
		}
	}
	var resp *http.Response
	err = f.pacer.Call(func() (bool, error) {
		resp, err = f.srv.CallJSON(ctx, &opts, &update, &info)
		return shouldRetry(ctx, resp, err)
	})
	if err != nil {
		return nil, err
	}
	return info, nil
}

// move a file or folder
//
// This is complicated by the fact that we can't use updateItem to move
// to a different directory AND rename at the same time as it can
// overwrite files in the source directory.
func (f *Fs) move(ctx context.Context, isFile bool, id, oldLeaf, newLeaf, oldDirectoryID, newDirectoryID string) (item *api.Item, err error) {
	// To demonstrate bug
	// item, err = f.updateItem(ctx, id, newLeaf, newDirectoryID, nil)
	// if err != nil {
	// 	return nil, errors.Wrap(err, "Move rename leaf")
	// }
	// return item, nil
	doRenameLeaf := oldLeaf != newLeaf
	doMove := oldDirectoryID != newDirectoryID

	// Now rename the leaf to a temporary name if we are moving to
	// another directory to make sure we don't overwrite something
	// in the source directory by accident
	if doRenameLeaf && doMove {
		tmpLeaf := newLeaf + "." + random.String(8)
		item, err = f.updateItem(ctx, id, tmpLeaf, "", nil)
		if err != nil {
			return nil, errors.Wrap(err, "Move rename leaf")
		}
	}

	// Move the object to a new directory (with the existing name)
	// if required
	if doMove {
		item, err = f.updateItem(ctx, id, "", newDirectoryID, nil)
		if err != nil {
			return nil, errors.Wrap(err, "Move directory")
		}
	}

	// Rename the leaf to its final name if required
	if doRenameLeaf {
		item, err = f.updateItem(ctx, id, newLeaf, "", nil)
		if err != nil {
			return nil, errors.Wrap(err, "Move rename leaf")
		}
	}

	return item, nil
}

// Move src to this remote using server-side move operations.
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

	// Find ID of src parent, not creating subdirs
	srcLeaf, srcParentID, err := srcObj.fs.dirCache.FindPath(ctx, srcObj.remote, false)
	if err != nil {
		return nil, err
	}

	// Create temporary object
	dstObj, leaf, directoryID, err := f.createObject(ctx, remote, srcObj.modTime, srcObj.size)
	if err != nil {
		return nil, err
	}

	// Do the move
	info, err := f.move(ctx, true, srcObj.id, srcLeaf, leaf, srcParentID, directoryID)
	if err != nil {
		return nil, err
	}

	err = dstObj.setMetaData(info)
	if err != nil {
		return nil, err
	}
	return dstObj, nil
}

// DirMove moves src, srcRemote to this remote at dstRemote
// using server-side move operations.
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

	srcID, srcDirectoryID, srcLeaf, dstDirectoryID, dstLeaf, err := f.dirCache.DirMove(ctx, srcFs.dirCache, srcFs.root, srcRemote, f.root, dstRemote)
	if err != nil {
		return err
	}

	// Do the move
	_, err = f.move(ctx, false, srcID, srcLeaf, dstLeaf, srcDirectoryID, dstDirectoryID)
	if err != nil {
		return err
	}
	srcFs.dirCache.FlushDir(srcRemote)
	return nil
}

// Copy src to this remote using server-side copy operations.
//
// This is stored with the remote path given
//
// It returns the destination Object and a possible error
//
// Will only be called if src.Fs().Name() == f.Name()
//
// If it isn't possible then return fs.ErrorCantCopy
func (f *Fs) Copy(ctx context.Context, src fs.Object, remote string) (dst fs.Object, err error) {
	srcObj, ok := src.(*Object)
	if !ok {
		fs.Debugf(src, "Can't copy - not same remote type")
		return nil, fs.ErrorCantCopy
	}

	err = srcObj.readMetaData(ctx)
	if err != nil {
		return nil, err
	}

	// Find ID of src parent, not creating subdirs
	srcLeaf, srcParentID, err := srcObj.fs.dirCache.FindPath(ctx, srcObj.remote, false)
	if err != nil {
		return nil, err
	}
	srcLeaf = f.opt.Enc.FromStandardName(srcLeaf)
	_ = srcParentID

	// Create temporary object
	dstObj, dstLeaf, dstParentID, err := f.createObject(ctx, remote, srcObj.modTime, srcObj.size)
	if err != nil {
		return nil, err
	}
	dstLeaf = f.opt.Enc.FromStandardName(dstLeaf)

	sameName := strings.ToLower(srcLeaf) == strings.ToLower(dstLeaf)
	if sameName && srcParentID == dstParentID {
		return nil, errors.Errorf("copy: can't copy to a file in the same directory whose name only differs in case: %q vs %q", srcLeaf, dstLeaf)
	}

	// Discover whether we can just copy directly or not
	directCopy := false
	if sameName {
		// if copying to same name can copy directly
		directCopy = true
	} else {
		// if (dstParentID, srcLeaf) does not exist then can
		// Copy then Rename without fear of overwriting
		// something
		_, err := f.readMetaDataForIDPath(ctx, dstParentID, srcLeaf, false, false)
		if err == fs.ErrorObjectNotFound || err == fs.ErrorDirNotFound {
			directCopy = true
		} else if err != nil {
			return nil, errors.Wrap(err, "copy: failed to examine destination dir")
		} else {
			// otherwise need to copy via a temporary directory
		}
	}

	// Copy direct to destination unless !directCopy in which case
	// copy via a temporary directory
	copyTargetDirID := dstParentID
	if !directCopy {
		// Create a temporary directory to copy the object in to
		tmpDir := "rclone-temp-dir-" + random.String(16)
		err = f.Mkdir(ctx, tmpDir)
		if err != nil {
			return nil, errors.Wrap(err, "copy: failed to make temp dir")
		}
		defer func() {
			rmdirErr := f.Rmdir(ctx, tmpDir)
			if rmdirErr != nil && err == nil {
				err = errors.Wrap(rmdirErr, "copy: failed to remove temp dir")
			}
		}()
		tmpDirID, err := f.dirCache.FindDir(ctx, tmpDir, false)
		if err != nil {
			return nil, errors.Wrap(err, "copy: failed to find temp dir")
		}
		copyTargetDirID = tmpDirID
	}

	// Copy the object
	opts := rest.Opts{
		Method: "POST",
		Path:   "/Items(" + srcObj.id + ")/Copy",
		Parameters: url.Values{
			"$select":   {api.ListRequestSelect},
			"overwrite": {"false"},
			"targetid":  {copyTargetDirID},
		},
	}
	var resp *http.Response
	var info *api.Item
	err = f.pacer.Call(func() (bool, error) {
		resp, err = f.srv.CallJSON(ctx, &opts, nil, &info)
		return shouldRetry(ctx, resp, err)
	})
	if err != nil {
		return nil, err
	}

	// Rename into the correct name and directory if required and
	// set the modtime since the copy doesn't preserve it
	var updateParentID, updateLeaf string // only set these if necessary
	if srcLeaf != dstLeaf {
		updateLeaf = dstLeaf
	}
	if !directCopy {
		updateParentID = dstParentID
	}
	// set new modtime regardless
	info, err = f.updateItem(ctx, info.ID, updateLeaf, updateParentID, &srcObj.modTime)
	if err != nil {
		return nil, err
	}
	err = dstObj.setMetaData(info)
	if err != nil {
		return nil, err
	}
	return dstObj, nil
}

// DirCacheFlush resets the directory cache - used in testing as an
// optional interface
func (f *Fs) DirCacheFlush() {
	f.dirCache.ResetRoot()
}

// Hashes returns the supported hash sets.
func (f *Fs) Hashes() hash.Set {
	return hash.Set(hash.MD5)
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
func (o *Object) Hash(ctx context.Context, t hash.Type) (string, error) {
	if t != hash.MD5 {
		return "", hash.ErrUnsupported
	}
	err := o.readMetaData(ctx)
	if err != nil {
		return "", err
	}
	return o.md5, nil
}

// Size returns the size of an object in bytes
func (o *Object) Size() int64 {
	err := o.readMetaData(context.TODO())
	if err != nil {
		fs.Logf(o, "Failed to read metadata: %v", err)
		return 0
	}
	return o.size
}

// setMetaData sets the metadata from info
func (o *Object) setMetaData(info *api.Item) (err error) {
	if info.Type != api.ItemTypeFile {
		return errors.Wrapf(fs.ErrorNotAFile, "%q is %q", o.remote, info.Type)
	}
	o.hasMetaData = true
	o.size = info.Size
	if !info.ModifiedAt.IsZero() {
		o.modTime = info.ModifiedAt
	} else {
		o.modTime = info.CreatedAt
	}
	o.id = info.ID
	o.md5 = info.Hash
	return nil
}

// readMetaData gets the metadata if it hasn't already been fetched
//
// it also sets the info
func (o *Object) readMetaData(ctx context.Context) (err error) {
	if o.hasMetaData {
		return nil
	}
	var info *api.Item
	if o.id != "" {
		info, err = o.fs.readMetaDataForID(ctx, o.id, false, true)
	} else {
		info, err = o.fs.readMetaDataForPath(ctx, o.remote, false, true)
	}
	if err != nil {
		return err
	}
	return o.setMetaData(info)
}

// ModTime returns the modification time of the object
//
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

// SetModTime sets the modification time of the local fs object
func (o *Object) SetModTime(ctx context.Context, modTime time.Time) (err error) {
	info, err := o.fs.updateItem(ctx, o.id, "", "", &modTime)
	if err != nil {
		return err
	}
	err = o.setMetaData(info)
	if err != nil {
		return err
	}
	return nil
}

// Storable returns a boolean showing whether this object storable
func (o *Object) Storable() bool {
	return true
}

// Open an object for read
func (o *Object) Open(ctx context.Context, options ...fs.OpenOption) (in io.ReadCloser, err error) {
	opts := rest.Opts{
		Method: "GET",
		Path:   "/Items(" + o.id + ")/Download",
		Parameters: url.Values{
			"redirect": {"false"},
		},
	}
	var resp *http.Response
	var dl api.DownloadSpecification
	err = o.fs.pacer.Call(func() (bool, error) {
		resp, err = o.fs.srv.CallJSON(ctx, &opts, nil, &dl)
		return shouldRetry(ctx, resp, err)
	})
	if err != nil {
		return nil, errors.Wrap(err, "open: fetch download specification")
	}

	fs.FixRangeOption(options, o.size)
	opts = rest.Opts{
		Path:    "",
		RootURL: dl.URL,
		Method:  "GET",
		Options: options,
	}
	err = o.fs.pacer.Call(func() (bool, error) {
		resp, err = o.fs.srv.Call(ctx, &opts)
		return shouldRetry(ctx, resp, err)
	})
	if err != nil {
		return nil, errors.Wrap(err, "open")
	}
	return resp.Body, err
}

// Update the object with the contents of the io.Reader, modTime and size
//
// If existing is set then it updates the object rather than creating a new one
//
// The new object may have been created if an error is returned
func (o *Object) Update(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (err error) {
	remote := o.Remote()
	size := src.Size()
	modTime := src.ModTime(ctx)
	isLargeFile := size < 0 || size > int64(o.fs.opt.UploadCutoff)

	// Create the directory for the object if it doesn't exist
	leaf, directoryID, err := o.fs.dirCache.FindPath(ctx, remote, true)
	if err != nil {
		return err
	}
	leaf = o.fs.opt.Enc.FromStandardName(leaf)
	var req = api.UploadRequest{
		Method:       "standard",
		Raw:          true,
		Filename:     leaf,
		Overwrite:    true,
		CreatedDate:  modTime,
		ModifiedDate: modTime,
		Tool:         o.fs.ci.UserAgent,
	}

	if isLargeFile {
		if size < 0 {
			// For files of indeterminate size, use streamed
			req.Method = "streamed"
		} else {
			// otherwise use threaded which is more efficient
			req.Method = "threaded"
			req.ThreadCount = &o.fs.ci.Transfers
			req.Filesize = &size
		}
	}

	var resp *http.Response
	var info api.UploadSpecification
	opts := rest.Opts{
		Method:  "POST",
		Path:    "/Items(" + directoryID + ")/Upload2",
		Options: options,
	}
	err = o.fs.pacer.Call(func() (bool, error) {
		resp, err = o.fs.srv.CallJSON(ctx, &opts, &req, &info)
		return shouldRetry(ctx, resp, err)
	})
	if err != nil {
		return errors.Wrap(err, "upload get specification")
	}

	// If file is large then upload in parts
	if isLargeFile {
		up, err := o.fs.newLargeUpload(ctx, o, in, src, &info)
		if err != nil {
			return err
		}
		return up.Upload(ctx)
	}

	// Single part upload
	opts = rest.Opts{
		Method:        "POST",
		RootURL:       info.ChunkURI + "&fmt=json",
		Body:          in,
		ContentLength: &size,
	}
	var finish api.UploadFinishResponse
	err = o.fs.pacer.CallNoRetry(func() (bool, error) {
		resp, err = o.fs.srv.CallJSON(ctx, &opts, nil, &finish)
		return shouldRetry(ctx, resp, err)
	})
	if err != nil {
		return errors.Wrap(err, "upload file")
	}
	return o.checkUploadResponse(ctx, &finish)
}

// Check the upload response and update the metadata on the object
func (o *Object) checkUploadResponse(ctx context.Context, finish *api.UploadFinishResponse) (err error) {
	// Find returned ID
	id, err := finish.ID()
	if err != nil {
		return err
	}

	// Read metadata
	o.id = id
	o.hasMetaData = false
	return o.readMetaData(ctx)
}

// Remove an object by ID
func (f *Fs) remove(ctx context.Context, id string) (err error) {
	opts := rest.Opts{
		Method: "DELETE",
		Path:   "/Items(" + id + ")",
		Parameters: url.Values{
			"singleversion": {"false"},
			"forceSync":     {"true"},
		},
		NoResponse: true,
	}
	var resp *http.Response
	err = f.pacer.Call(func() (bool, error) {
		resp, err = f.srv.Call(ctx, &opts)
		return shouldRetry(ctx, resp, err)
	})
	if err != nil {
		return errors.Wrap(err, "remove")
	}
	return nil
}

// Remove an object
func (o *Object) Remove(ctx context.Context) error {
	err := o.readMetaData(ctx)
	if err != nil {
		return errors.Wrap(err, "Remove: Failed to read metadata")
	}
	return o.fs.remove(ctx, o.id)
}

// ID returns the ID of the Object if known, or "" if not
func (o *Object) ID() string {
	return o.id
}

// Check the interfaces are satisfied
var (
	_ fs.Fs              = (*Fs)(nil)
	_ fs.Purger          = (*Fs)(nil)
	_ fs.Mover           = (*Fs)(nil)
	_ fs.DirMover        = (*Fs)(nil)
	_ fs.Copier          = (*Fs)(nil)
	_ fs.PutStreamer     = (*Fs)(nil)
	_ fs.DirCacheFlusher = (*Fs)(nil)
	_ fs.Object          = (*Object)(nil)
	_ fs.IDer            = (*Object)(nil)
)
