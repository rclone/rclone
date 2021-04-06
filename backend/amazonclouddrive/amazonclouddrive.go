// Package amazonclouddrive provides an interface to the Amazon Cloud
// Drive object storage system.
package amazonclouddrive

/*
FIXME make searching for directory in id and file in id more efficient
- use the name: search parameter - remember the escaping rules
- use Folder GetNode and GetFile

FIXME make the default for no files and no dirs be (FILE & FOLDER) so
we ignore assets completely!
*/

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"path"
	"strings"
	"time"

	acd "github.com/ncw/go-acd"
	"github.com/pkg/errors"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config"
	"github.com/rclone/rclone/fs/config/configmap"
	"github.com/rclone/rclone/fs/config/configstruct"
	"github.com/rclone/rclone/fs/fserrors"
	"github.com/rclone/rclone/fs/fshttp"
	"github.com/rclone/rclone/fs/hash"
	"github.com/rclone/rclone/lib/dircache"
	"github.com/rclone/rclone/lib/encoder"
	"github.com/rclone/rclone/lib/oauthutil"
	"github.com/rclone/rclone/lib/pacer"
	"golang.org/x/oauth2"
)

const (
	folderKind               = "FOLDER"
	fileKind                 = "FILE"
	statusAvailable          = "AVAILABLE"
	timeFormat               = time.RFC3339 // 2014-03-07T22:31:12.173Z
	minSleep                 = 20 * time.Millisecond
	warnFileSize             = 50000 << 20            // Display warning for files larger than this size
	defaultTempLinkThreshold = fs.SizeSuffix(9 << 30) // Download files bigger than this via the tempLink
)

// Globals
var (
	// Description of how to auth for this app
	acdConfig = &oauth2.Config{
		Scopes: []string{"clouddrive:read_all", "clouddrive:write"},
		Endpoint: oauth2.Endpoint{
			AuthURL:  "https://www.amazon.com/ap/oa",
			TokenURL: "https://api.amazon.com/auth/o2/token",
		},
		ClientID:     "",
		ClientSecret: "",
		RedirectURL:  oauthutil.RedirectURL,
	}
)

// Register with Fs
func init() {
	fs.Register(&fs.RegInfo{
		Name:        "amazon cloud drive",
		Prefix:      "acd",
		Description: "Amazon Drive",
		NewFs:       NewFs,
		Config: func(ctx context.Context, name string, m configmap.Mapper) error {
			err := oauthutil.Config(ctx, "amazon cloud drive", name, m, acdConfig, nil)
			if err != nil {
				return errors.Wrap(err, "failed to configure token")
			}
			return nil
		},
		Options: append(oauthutil.SharedOptions, []fs.Option{{
			Name:     "checkpoint",
			Help:     "Checkpoint for internal polling (debug).",
			Hide:     fs.OptionHideBoth,
			Advanced: true,
		}, {
			Name: "upload_wait_per_gb",
			Help: `Additional time per GiB to wait after a failed complete upload to see if it appears.

Sometimes Amazon Drive gives an error when a file has been fully
uploaded but the file appears anyway after a little while.  This
happens sometimes for files over 1 GiB in size and nearly every time for
files bigger than 10 GiB. This parameter controls the time rclone waits
for the file to appear.

The default value for this parameter is 3 minutes per GiB, so by
default it will wait 3 minutes for every GiB uploaded to see if the
file appears.

You can disable this feature by setting it to 0. This may cause
conflict errors as rclone retries the failed upload but the file will
most likely appear correctly eventually.

These values were determined empirically by observing lots of uploads
of big files for a range of file sizes.

Upload with the "-v" flag to see more info about what rclone is doing
in this situation.`,
			Default:  fs.Duration(180 * time.Second),
			Advanced: true,
		}, {
			Name: "templink_threshold",
			Help: `Files >= this size will be downloaded via their tempLink.

Files this size or more will be downloaded via their "tempLink". This
is to work around a problem with Amazon Drive which blocks downloads
of files bigger than about 10 GiB. The default for this is 9 GiB which
shouldn't need to be changed.

To download files above this threshold, rclone requests a "tempLink"
which downloads the file through a temporary URL directly from the
underlying S3 storage.`,
			Default:  defaultTempLinkThreshold,
			Advanced: true,
		}, {
			Name:     config.ConfigEncoding,
			Help:     config.ConfigEncodingHelp,
			Advanced: true,
			// Encode invalid UTF-8 bytes as json doesn't handle them properly.
			Default: (encoder.Base |
				encoder.EncodeInvalidUtf8),
		}}...),
	})
}

// Options defines the configuration for this backend
type Options struct {
	Checkpoint        string               `config:"checkpoint"`
	UploadWaitPerGB   fs.Duration          `config:"upload_wait_per_gb"`
	TempLinkThreshold fs.SizeSuffix        `config:"templink_threshold"`
	Enc               encoder.MultiEncoder `config:"encoding"`
}

// Fs represents a remote acd server
type Fs struct {
	name         string             // name of this remote
	features     *fs.Features       // optional features
	opt          Options            // options for this Fs
	ci           *fs.ConfigInfo     // global config
	c            *acd.Client        // the connection to the acd server
	noAuthClient *http.Client       // unauthenticated http client
	root         string             // the path we are working on
	dirCache     *dircache.DirCache // Map of directory path to directory id
	pacer        *fs.Pacer          // pacer for API calls
	trueRootID   string             // ID of true root directory
	tokenRenewer *oauthutil.Renew   // renew the token on expiry
}

// Object describes an acd object
//
// Will definitely have info but maybe not meta
type Object struct {
	fs     *Fs       // what this object is part of
	remote string    // The remote path
	info   *acd.Node // Info from the acd object if known
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
	return fmt.Sprintf("amazon drive root '%s'", f.root)
}

// Features returns the optional features of this Fs
func (f *Fs) Features() *fs.Features {
	return f.features
}

// parsePath parses an acd 'url'
func parsePath(path string) (root string) {
	root = strings.Trim(path, "/")
	return
}

// retryErrorCodes is a slice of error codes that we will retry
var retryErrorCodes = []int{
	400, // Bad request (seen in "Next token is expired")
	401, // Unauthorized (seen in "Token has expired")
	408, // Request Timeout
	429, // Rate exceeded.
	500, // Get occasional 500 Internal Server Error
	502, // Bad Gateway when doing big listings
	503, // Service Unavailable
	504, // Gateway Time-out
}

// shouldRetry returns a boolean as to whether this resp and err
// deserve to be retried.  It returns the err as a convenience
func (f *Fs) shouldRetry(ctx context.Context, resp *http.Response, err error) (bool, error) {
	if fserrors.ContextError(ctx, &err) {
		return false, err
	}
	if resp != nil {
		if resp.StatusCode == 401 {
			f.tokenRenewer.Invalidate()
			fs.Debugf(f, "401 error received - invalidating token")
			return true, err
		}
		// Work around receiving this error sporadically on authentication
		//
		// HTTP code 403: "403 Forbidden", response body: {"message":"Authorization header requires 'Credential' parameter. Authorization header requires 'Signature' parameter. Authorization header requires 'SignedHeaders' parameter. Authorization header requires existence of either a 'X-Amz-Date' or a 'Date' header. Authorization=Bearer"}
		if resp.StatusCode == 403 && strings.Contains(err.Error(), "Authorization header requires") {
			fs.Debugf(f, "403 \"Authorization header requires...\" error received - retry")
			return true, err
		}
	}
	return fserrors.ShouldRetry(err) || fserrors.ShouldRetryHTTP(resp, retryErrorCodes), err
}

// If query parameters contain X-Amz-Algorithm remove Authorization header
//
// This happens when ACD redirects to S3 for the download.  The oauth
// transport puts an Authorization header in which we need to remove
// otherwise we get this message from AWS
//
// Only one auth mechanism allowed; only the X-Amz-Algorithm query
// parameter, Signature query string parameter or the Authorization
// header should be specified
func filterRequest(req *http.Request) {
	if req.URL.Query().Get("X-Amz-Algorithm") != "" {
		fs.Debugf(nil, "Removing Authorization: header after redirect to S3")
		req.Header.Del("Authorization")
	}
}

// NewFs constructs an Fs from the path, container:path
func NewFs(ctx context.Context, name, root string, m configmap.Mapper) (fs.Fs, error) {
	// Parse config into Options struct
	opt := new(Options)
	err := configstruct.Set(m, opt)
	if err != nil {
		return nil, err
	}
	root = parsePath(root)
	baseClient := fshttp.NewClient(ctx)
	if do, ok := baseClient.Transport.(interface {
		SetRequestFilter(f func(req *http.Request))
	}); ok {
		do.SetRequestFilter(filterRequest)
	} else {
		fs.Debugf(name+":", "Couldn't add request filter - large file downloads will fail")
	}
	oAuthClient, ts, err := oauthutil.NewClientWithBaseClient(ctx, name, m, acdConfig, baseClient)
	if err != nil {
		return nil, errors.Wrap(err, "failed to configure Amazon Drive")
	}

	c := acd.NewClient(oAuthClient)
	ci := fs.GetConfig(ctx)
	f := &Fs{
		name:         name,
		root:         root,
		opt:          *opt,
		ci:           ci,
		c:            c,
		pacer:        fs.NewPacer(ctx, pacer.NewAmazonCloudDrive(pacer.MinSleep(minSleep))),
		noAuthClient: fshttp.NewClient(ctx),
	}
	f.features = (&fs.Features{
		CaseInsensitive:         true,
		ReadMimeType:            true,
		CanHaveEmptyDirectories: true,
	}).Fill(ctx, f)

	// Renew the token in the background
	f.tokenRenewer = oauthutil.NewRenew(f.String(), ts, func() error {
		_, err := f.getRootInfo(ctx)
		return err
	})

	// Update endpoints
	var resp *http.Response
	err = f.pacer.Call(func() (bool, error) {
		_, resp, err = f.c.Account.GetEndpoints()
		return f.shouldRetry(ctx, resp, err)
	})
	if err != nil {
		return nil, errors.Wrap(err, "failed to get endpoints")
	}

	// Get rootID
	rootInfo, err := f.getRootInfo(ctx)
	if err != nil || rootInfo.Id == nil {
		return nil, errors.Wrap(err, "failed to get root")
	}
	f.trueRootID = *rootInfo.Id

	f.dirCache = dircache.New(root, f.trueRootID, f)

	// Find the current root
	err = f.dirCache.FindRoot(ctx, false)
	if err != nil {
		// Assume it is a file
		newRoot, remote := dircache.SplitPath(root)
		tempF := *f
		tempF.dirCache = dircache.New(newRoot, f.trueRootID, &tempF)
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

// getRootInfo gets the root folder info
func (f *Fs) getRootInfo(ctx context.Context) (rootInfo *acd.Folder, err error) {
	var resp *http.Response
	err = f.pacer.Call(func() (bool, error) {
		rootInfo, resp, err = f.c.Nodes.GetRoot()
		return f.shouldRetry(ctx, resp, err)
	})
	return rootInfo, err
}

// Return an Object from a path
//
// If it can't be found it returns the error fs.ErrorObjectNotFound.
func (f *Fs) newObjectWithInfo(ctx context.Context, remote string, info *acd.Node) (fs.Object, error) {
	o := &Object{
		fs:     f,
		remote: remote,
	}
	if info != nil {
		// Set info but not meta
		o.info = info
	} else {
		err := o.readMetaData(ctx) // reads info and meta, returning an error
		if err != nil {
			return nil, err
		}
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
	//fs.Debugf(f, "FindLeaf(%q, %q)", pathID, leaf)
	folder := acd.FolderFromId(pathID, f.c.Nodes)
	var resp *http.Response
	var subFolder *acd.Folder
	err = f.pacer.Call(func() (bool, error) {
		subFolder, resp, err = folder.GetFolder(f.opt.Enc.FromStandardName(leaf))
		return f.shouldRetry(ctx, resp, err)
	})
	if err != nil {
		if err == acd.ErrorNodeNotFound {
			//fs.Debugf(f, "...Not found")
			return "", false, nil
		}
		//fs.Debugf(f, "...Error %v", err)
		return "", false, err
	}
	if subFolder.Status != nil && *subFolder.Status != statusAvailable {
		fs.Debugf(f, "Ignoring folder %q in state %q", leaf, *subFolder.Status)
		time.Sleep(1 * time.Second) // FIXME wait for problem to go away!
		return "", false, nil
	}
	//fs.Debugf(f, "...Found(%q, %v)", *subFolder.Id, leaf)
	return *subFolder.Id, true, nil
}

// CreateDir makes a directory with pathID as parent and name leaf
func (f *Fs) CreateDir(ctx context.Context, pathID, leaf string) (newID string, err error) {
	//fmt.Printf("CreateDir(%q, %q)\n", pathID, leaf)
	folder := acd.FolderFromId(pathID, f.c.Nodes)
	var resp *http.Response
	var info *acd.Folder
	err = f.pacer.Call(func() (bool, error) {
		info, resp, err = folder.CreateFolder(f.opt.Enc.FromStandardName(leaf))
		return f.shouldRetry(ctx, resp, err)
	})
	if err != nil {
		//fmt.Printf("...Error %v\n", err)
		return "", err
	}
	//fmt.Printf("...Id %q\n", *info.Id)
	return *info.Id, nil
}

// list the objects into the function supplied
//
// If directories is set it only sends directories
// User function to process a File item from listAll
//
// Should return true to finish processing
type listAllFn func(*acd.Node) bool

// Lists the directory required calling the user function on each item found
//
// If the user fn ever returns true then it early exits with found = true
func (f *Fs) listAll(ctx context.Context, dirID string, title string, directoriesOnly bool, filesOnly bool, fn listAllFn) (found bool, err error) {
	query := "parents:" + dirID
	if directoriesOnly {
		query += " AND kind:" + folderKind
	} else if filesOnly {
		query += " AND kind:" + fileKind
	} else {
		// FIXME none of these work
		//query += " AND kind:(" + fileKind + " OR " + folderKind + ")"
		//query += " AND (kind:" + fileKind + " OR kind:" + folderKind + ")"
	}
	opts := acd.NodeListOptions{
		Filters: query,
	}
	var nodes []*acd.Node
	var out []*acd.Node
	//var resp *http.Response
	for {
		var resp *http.Response
		err = f.pacer.CallNoRetry(func() (bool, error) {
			nodes, resp, err = f.c.Nodes.GetNodes(&opts)
			return f.shouldRetry(ctx, resp, err)
		})
		if err != nil {
			return false, err
		}
		if nodes == nil {
			break
		}
		for _, node := range nodes {
			if node.Name != nil && node.Id != nil && node.Kind != nil && node.Status != nil {
				// Ignore nodes if not AVAILABLE
				if *node.Status != statusAvailable {
					continue
				}
				// Ignore bogus nodes Amazon Drive sometimes reports
				hasValidParent := false
				for _, parent := range node.Parents {
					if parent == dirID {
						hasValidParent = true
						break
					}
				}
				if !hasValidParent {
					continue
				}
				*node.Name = f.opt.Enc.ToStandardName(*node.Name)
				// Store the nodes up in case we have to retry the listing
				out = append(out, node)
			}
		}
	}
	// Send the nodes now
	for _, node := range out {
		if fn(node) {
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
	maxTries := f.ci.LowLevelRetries
	var iErr error
	for tries := 1; tries <= maxTries; tries++ {
		entries = nil
		_, err = f.listAll(ctx, directoryID, "", false, false, func(node *acd.Node) bool {
			remote := path.Join(dir, *node.Name)
			switch *node.Kind {
			case folderKind:
				// cache the directory ID for later lookups
				f.dirCache.Put(remote, *node.Id)
				when, _ := time.Parse(timeFormat, *node.ModifiedDate) // FIXME
				d := fs.NewDir(remote, when).SetID(*node.Id)
				entries = append(entries, d)
			case fileKind:
				o, err := f.newObjectWithInfo(ctx, remote, node)
				if err != nil {
					iErr = err
					return true
				}
				entries = append(entries, o)
			default:
				// ignore ASSET, etc.
			}
			return false
		})
		if iErr != nil {
			return nil, iErr
		}
		if fserrors.IsRetryError(err) {
			fs.Debugf(f, "Directory listing error for %q: %v - low level retry %d/%d", dir, err, tries, maxTries)
			continue
		}
		if err != nil {
			return nil, err
		}
		break
	}
	return entries, nil
}

// checkUpload checks to see if an error occurred after the file was
// completely uploaded.
//
// If it was then it waits for a while to see if the file really
// exists and is the right size and returns an updated info.
//
// If the file wasn't found or was the wrong size then it returns the
// original error.
//
// This is a workaround for Amazon sometimes returning
//
//  * 408 REQUEST_TIMEOUT
//  * 504 GATEWAY_TIMEOUT
//  * 500 Internal server error
//
// At the end of large uploads.  The speculation is that the timeout
// is waiting for the sha1 hashing to complete and the file may well
// be properly uploaded.
func (f *Fs) checkUpload(ctx context.Context, resp *http.Response, in io.Reader, src fs.ObjectInfo, inInfo *acd.File, inErr error, uploadTime time.Duration) (fixedError bool, info *acd.File, err error) {
	// Return if no error - all is well
	if inErr == nil {
		return false, inInfo, inErr
	}
	// If not one of the errors we can fix return
	// if resp == nil || resp.StatusCode != 408 && resp.StatusCode != 500 && resp.StatusCode != 504 {
	// 	return false, inInfo, inErr
	// }

	// The HTTP status
	httpStatus := "HTTP status UNKNOWN"
	if resp != nil {
		httpStatus = resp.Status
	}

	// check to see if we read to the end
	buf := make([]byte, 1)
	n, err := in.Read(buf)
	if !(n == 0 && err == io.EOF) {
		fs.Debugf(src, "Upload error detected but didn't finish upload: %v (%q)", inErr, httpStatus)
		return false, inInfo, inErr
	}

	// Don't wait for uploads - assume they will appear later
	if f.opt.UploadWaitPerGB <= 0 {
		fs.Debugf(src, "Upload error detected but waiting disabled: %v (%q)", inErr, httpStatus)
		return false, inInfo, inErr
	}

	// Time we should wait for the upload
	uploadWaitPerByte := float64(f.opt.UploadWaitPerGB) / 1024 / 1024 / 1024
	timeToWait := time.Duration(uploadWaitPerByte * float64(src.Size()))

	const sleepTime = 5 * time.Second                        // sleep between tries
	retries := int((timeToWait + sleepTime - 1) / sleepTime) // number of retries, rounded up

	fs.Debugf(src, "Error detected after finished upload - waiting to see if object was uploaded correctly: %v (%q)", inErr, httpStatus)
	remote := src.Remote()
	for i := 1; i <= retries; i++ {
		o, err := f.NewObject(ctx, remote)
		if err == fs.ErrorObjectNotFound {
			fs.Debugf(src, "Object not found - waiting (%d/%d)", i, retries)
		} else if err != nil {
			fs.Debugf(src, "Object returned error - waiting (%d/%d): %v", i, retries, err)
		} else {
			if src.Size() == o.Size() {
				fs.Debugf(src, "Object found with correct size %d after waiting (%d/%d) - %v - returning with no error", src.Size(), i, retries, sleepTime*time.Duration(i-1))
				info = &acd.File{
					Node: o.(*Object).info,
				}
				return true, info, nil
			}
			fs.Debugf(src, "Object found but wrong size %d vs %d - waiting (%d/%d)", src.Size(), o.Size(), i, retries)
		}
		time.Sleep(sleepTime)
	}
	fs.Debugf(src, "Giving up waiting for object - returning original error: %v (%q)", inErr, httpStatus)
	return false, inInfo, inErr
}

// Put the object into the container
//
// Copy the reader in to the new object which is returned
//
// The new object may have been created if an error is returned
func (f *Fs) Put(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error) {
	remote := src.Remote()
	size := src.Size()
	// Temporary Object under construction
	o := &Object{
		fs:     f,
		remote: remote,
	}
	// Check if object already exists
	err := o.readMetaData(ctx)
	switch err {
	case nil:
		return o, o.Update(ctx, in, src, options...)
	case fs.ErrorObjectNotFound:
		// Not found so create it
	default:
		return nil, err
	}
	// If not create it
	leaf, directoryID, err := f.dirCache.FindPath(ctx, remote, true)
	if err != nil {
		return nil, err
	}
	if size > warnFileSize {
		fs.Logf(f, "Warning: file %q may fail because it is too big. Use --max-size=%dM to skip large files.", remote, warnFileSize>>20)
	}
	folder := acd.FolderFromId(directoryID, o.fs.c.Nodes)
	var info *acd.File
	var resp *http.Response
	err = f.pacer.CallNoRetry(func() (bool, error) {
		start := time.Now()
		f.tokenRenewer.Start()
		info, resp, err = folder.Put(in, f.opt.Enc.FromStandardName(leaf))
		f.tokenRenewer.Stop()
		var ok bool
		ok, info, err = f.checkUpload(ctx, resp, in, src, info, err, time.Since(start))
		if ok {
			return false, nil
		}
		return f.shouldRetry(ctx, resp, err)
	})
	if err != nil {
		return nil, err
	}
	o.info = info.Node
	return o, nil
}

// Mkdir creates the container if it doesn't exist
func (f *Fs) Mkdir(ctx context.Context, dir string) error {
	_, err := f.dirCache.FindDir(ctx, dir, true)
	return err
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
	//  go test -v -run '^Test(Setup|Init|FsMkdir|FsPutFile1|FsPutFile2|FsUpdateFile1|FsMove)$'
	srcObj, ok := src.(*Object)
	if !ok {
		fs.Debugf(src, "Can't move - not same remote type")
		return nil, fs.ErrorCantMove
	}

	// create the destination directory if necessary
	srcLeaf, srcDirectoryID, err := srcObj.fs.dirCache.FindPath(ctx, srcObj.remote, false)
	if err != nil {
		return nil, err
	}
	dstLeaf, dstDirectoryID, err := f.dirCache.FindPath(ctx, remote, true)
	if err != nil {
		return nil, err
	}
	err = f.moveNode(ctx, srcObj.remote, dstLeaf, dstDirectoryID, srcObj.info, srcLeaf, srcDirectoryID, false)
	if err != nil {
		return nil, err
	}
	// Wait for directory caching so we can no longer see the old
	// object and see the new object
	time.Sleep(200 * time.Millisecond) // enough time 90% of the time
	var (
		dstObj         fs.Object
		srcErr, dstErr error
	)
	for i := 1; i <= f.ci.LowLevelRetries; i++ {
		_, srcErr = srcObj.fs.NewObject(ctx, srcObj.remote) // try reading the object
		if srcErr != nil && srcErr != fs.ErrorObjectNotFound {
			// exit if error on source
			return nil, srcErr
		}
		dstObj, dstErr = f.NewObject(ctx, remote)
		if dstErr != nil && dstErr != fs.ErrorObjectNotFound {
			// exit if error on dst
			return nil, dstErr
		}
		if srcErr == fs.ErrorObjectNotFound && dstErr == nil {
			// finished if src not found and dst found
			break
		}
		fs.Debugf(src, "Wait for directory listing to update after move %d/%d", i, f.ci.LowLevelRetries)
		time.Sleep(1 * time.Second)
	}
	return dstObj, dstErr
}

// DirCacheFlush resets the directory cache - used in testing as an
// optional interface
func (f *Fs) DirCacheFlush() {
	f.dirCache.ResetRoot()
}

// DirMove moves src, srcRemote to this remote at dstRemote
// using server-side move operations.
//
// Will only be called if src.Fs().Name() == f.Name()
//
// If it isn't possible then return fs.ErrorCantDirMove
//
// If destination exists then return fs.ErrorDirExists
func (f *Fs) DirMove(ctx context.Context, src fs.Fs, srcRemote, dstRemote string) (err error) {
	srcFs, ok := src.(*Fs)
	if !ok {
		fs.Debugf(src, "DirMove error: not same remote type")
		return fs.ErrorCantDirMove
	}
	srcPath := path.Join(srcFs.root, srcRemote)
	dstPath := path.Join(f.root, dstRemote)

	// Refuse to move to or from the root
	if srcPath == "" || dstPath == "" {
		fs.Debugf(src, "DirMove error: Can't move root")
		return errors.New("can't move root directory")
	}

	// Find ID of dst parent, creating subdirs if necessary
	dstLeaf, dstDirectoryID, err := f.dirCache.FindPath(ctx, dstRemote, true)
	if err != nil {
		return err
	}

	// Check destination does not exist
	_, err = f.dirCache.FindDir(ctx, dstRemote, false)
	if err == fs.ErrorDirNotFound {
		// OK
	} else if err != nil {
		return err
	} else {
		return fs.ErrorDirExists
	}

	// Find ID of src parent
	_, srcDirectoryID, err := srcFs.dirCache.FindPath(ctx, srcRemote, false)
	if err != nil {
		return err
	}
	srcLeaf, _ := dircache.SplitPath(srcPath)

	// Find ID of src
	srcID, err := srcFs.dirCache.FindDir(ctx, srcRemote, false)
	if err != nil {
		return err
	}

	// FIXME make a proper node.UpdateMetadata command
	srcInfo := acd.NodeFromId(srcID, f.c.Nodes)
	var jsonStr string
	err = srcFs.pacer.Call(func() (bool, error) {
		jsonStr, err = srcInfo.GetMetadata()
		return srcFs.shouldRetry(ctx, nil, err)
	})
	if err != nil {
		fs.Debugf(src, "DirMove error: error reading src metadata: %v", err)
		return err
	}
	err = json.Unmarshal([]byte(jsonStr), &srcInfo)
	if err != nil {
		fs.Debugf(src, "DirMove error: error reading unpacking src metadata: %v", err)
		return err
	}

	err = f.moveNode(ctx, srcPath, dstLeaf, dstDirectoryID, srcInfo, srcLeaf, srcDirectoryID, true)
	if err != nil {
		return err
	}

	srcFs.dirCache.FlushDir(srcRemote)
	return nil
}

// purgeCheck remotes the root directory, if check is set then it
// refuses to do so if it has anything in
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

	if check {
		// check directory is empty
		empty := true
		_, err = f.listAll(ctx, rootID, "", false, false, func(node *acd.Node) bool {
			switch *node.Kind {
			case folderKind:
				empty = false
				return true
			case fileKind:
				empty = false
				return true
			default:
				fs.Debugf("Found ASSET %s", *node.Id)
			}
			return false
		})
		if err != nil {
			return err
		}
		if !empty {
			return errors.New("directory not empty")
		}
	}

	node := acd.NodeFromId(rootID, f.c.Nodes)
	var resp *http.Response
	err = f.pacer.Call(func() (bool, error) {
		resp, err = node.Trash()
		return f.shouldRetry(ctx, resp, err)
	})
	if err != nil {
		return err
	}

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
	return fs.ModTimeNotSupported
}

// Hashes returns the supported hash sets.
func (f *Fs) Hashes() hash.Set {
	return hash.Set(hash.MD5)
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
//func (f *Fs) Copy(ctx context.Context, src fs.Object, remote string) (fs.Object, error) {
// srcObj, ok := src.(*Object)
// if !ok {
// 	fs.Debugf(src, "Can't copy - not same remote type")
// 	return nil, fs.ErrorCantCopy
// }
// srcFs := srcObj.fs
// _, err := f.c.ObjectCopy(srcFs.container, srcFs.root+srcObj.remote, f.container, f.root+remote, nil)
// if err != nil {
// 	return nil, err
// }
// return f.NewObject(ctx, remote), nil
//}

// Purge deletes all the files and the container
//
// Optional interface: Only implement this if you have a way of
// deleting all the files quicker than just running Remove() on the
// result of List()
func (f *Fs) Purge(ctx context.Context, dir string) error {
	return f.purgeCheck(ctx, dir, false)
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

// Hash returns the Md5sum of an object returning a lowercase hex string
func (o *Object) Hash(ctx context.Context, t hash.Type) (string, error) {
	if t != hash.MD5 {
		return "", hash.ErrUnsupported
	}
	if o.info.ContentProperties != nil && o.info.ContentProperties.Md5 != nil {
		return *o.info.ContentProperties.Md5, nil
	}
	return "", nil
}

// Size returns the size of an object in bytes
func (o *Object) Size() int64 {
	if o.info.ContentProperties != nil && o.info.ContentProperties.Size != nil {
		return int64(*o.info.ContentProperties.Size)
	}
	return 0 // Object is likely PENDING
}

// readMetaData gets the metadata if it hasn't already been fetched
//
// it also sets the info
//
// If it can't be found it returns the error fs.ErrorObjectNotFound.
func (o *Object) readMetaData(ctx context.Context) (err error) {
	if o.info != nil {
		return nil
	}
	leaf, directoryID, err := o.fs.dirCache.FindPath(ctx, o.remote, false)
	if err != nil {
		if err == fs.ErrorDirNotFound {
			return fs.ErrorObjectNotFound
		}
		return err
	}
	folder := acd.FolderFromId(directoryID, o.fs.c.Nodes)
	var resp *http.Response
	var info *acd.File
	err = o.fs.pacer.Call(func() (bool, error) {
		info, resp, err = folder.GetFile(o.fs.opt.Enc.FromStandardName(leaf))
		return o.fs.shouldRetry(ctx, resp, err)
	})
	if err != nil {
		if err == acd.ErrorNodeNotFound {
			return fs.ErrorObjectNotFound
		}
		return err
	}
	o.info = info.Node
	return nil
}

// ModTime returns the modification time of the object
//
//
// It attempts to read the objects mtime and if that isn't present the
// LastModified returned in the http headers
func (o *Object) ModTime(ctx context.Context) time.Time {
	err := o.readMetaData(ctx)
	if err != nil {
		fs.Debugf(o, "Failed to read metadata: %v", err)
		return time.Now()
	}
	modTime, err := time.Parse(timeFormat, *o.info.ModifiedDate)
	if err != nil {
		fs.Debugf(o, "Failed to read mtime from object: %v", err)
		return time.Now()
	}
	return modTime
}

// SetModTime sets the modification time of the local fs object
func (o *Object) SetModTime(ctx context.Context, modTime time.Time) error {
	// FIXME not implemented
	return fs.ErrorCantSetModTime
}

// Storable returns a boolean showing whether this object storable
func (o *Object) Storable() bool {
	return true
}

// Open an object for read
func (o *Object) Open(ctx context.Context, options ...fs.OpenOption) (in io.ReadCloser, err error) {
	bigObject := o.Size() >= int64(o.fs.opt.TempLinkThreshold)
	if bigObject {
		fs.Debugf(o, "Downloading large object via tempLink")
	}
	file := acd.File{Node: o.info}
	var resp *http.Response
	headers := fs.OpenOptionHeaders(options)
	err = o.fs.pacer.Call(func() (bool, error) {
		if !bigObject {
			in, resp, err = file.OpenHeaders(headers)
		} else {
			in, resp, err = file.OpenTempURLHeaders(o.fs.noAuthClient, headers)
		}
		return o.fs.shouldRetry(ctx, resp, err)
	})
	return in, err
}

// Update the object with the contents of the io.Reader, modTime and size
//
// The new object may have been created if an error is returned
func (o *Object) Update(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) error {
	file := acd.File{Node: o.info}
	var info *acd.File
	var resp *http.Response
	var err error
	err = o.fs.pacer.CallNoRetry(func() (bool, error) {
		start := time.Now()
		o.fs.tokenRenewer.Start()
		info, resp, err = file.Overwrite(in)
		o.fs.tokenRenewer.Stop()
		var ok bool
		ok, info, err = o.fs.checkUpload(ctx, resp, in, src, info, err, time.Since(start))
		if ok {
			return false, nil
		}
		return o.fs.shouldRetry(ctx, resp, err)
	})
	if err != nil {
		return err
	}
	o.info = info.Node
	return nil
}

// Remove a node
func (f *Fs) removeNode(ctx context.Context, info *acd.Node) error {
	var resp *http.Response
	var err error
	err = f.pacer.Call(func() (bool, error) {
		resp, err = info.Trash()
		return f.shouldRetry(ctx, resp, err)
	})
	return err
}

// Remove an object
func (o *Object) Remove(ctx context.Context) error {
	return o.fs.removeNode(ctx, o.info)
}

// Restore a node
func (f *Fs) restoreNode(ctx context.Context, info *acd.Node) (newInfo *acd.Node, err error) {
	var resp *http.Response
	err = f.pacer.Call(func() (bool, error) {
		newInfo, resp, err = info.Restore()
		return f.shouldRetry(ctx, resp, err)
	})
	return newInfo, err
}

// Changes name of given node
func (f *Fs) renameNode(ctx context.Context, info *acd.Node, newName string) (newInfo *acd.Node, err error) {
	var resp *http.Response
	err = f.pacer.Call(func() (bool, error) {
		newInfo, resp, err = info.Rename(f.opt.Enc.FromStandardName(newName))
		return f.shouldRetry(ctx, resp, err)
	})
	return newInfo, err
}

// Replaces one parent with another, effectively moving the file. Leaves other
// parents untouched. ReplaceParent cannot be used when the file is trashed.
func (f *Fs) replaceParent(ctx context.Context, info *acd.Node, oldParentID string, newParentID string) error {
	return f.pacer.Call(func() (bool, error) {
		resp, err := info.ReplaceParent(oldParentID, newParentID)
		return f.shouldRetry(ctx, resp, err)
	})
}

// Adds one additional parent to object.
func (f *Fs) addParent(ctx context.Context, info *acd.Node, newParentID string) error {
	return f.pacer.Call(func() (bool, error) {
		resp, err := info.AddParent(newParentID)
		return f.shouldRetry(ctx, resp, err)
	})
}

// Remove given parent from object, leaving the other possible
// parents untouched. Object can end up having no parents.
func (f *Fs) removeParent(ctx context.Context, info *acd.Node, parentID string) error {
	return f.pacer.Call(func() (bool, error) {
		resp, err := info.RemoveParent(parentID)
		return f.shouldRetry(ctx, resp, err)
	})
}

// moveNode moves the node given from the srcLeaf,srcDirectoryID to
// the dstLeaf,dstDirectoryID
func (f *Fs) moveNode(ctx context.Context, name, dstLeaf, dstDirectoryID string, srcInfo *acd.Node, srcLeaf, srcDirectoryID string, useDirErrorMsgs bool) (err error) {
	// fs.Debugf(name, "moveNode dst(%q,%s) <- src(%q,%s)", dstLeaf, dstDirectoryID, srcLeaf, srcDirectoryID)
	cantMove := fs.ErrorCantMove
	if useDirErrorMsgs {
		cantMove = fs.ErrorCantDirMove
	}

	if len(srcInfo.Parents) > 1 && srcLeaf != dstLeaf {
		fs.Debugf(name, "Move error: object is attached to multiple parents and should be renamed. This would change the name of the node in all parents.")
		return cantMove
	}

	if srcLeaf != dstLeaf {
		// fs.Debugf(name, "renaming")
		_, err = f.renameNode(ctx, srcInfo, dstLeaf)
		if err != nil {
			fs.Debugf(name, "Move: quick path rename failed: %v", err)
			goto OnConflict
		}
	}
	if srcDirectoryID != dstDirectoryID {
		// fs.Debugf(name, "trying parent replace: %s -> %s", oldParentID, newParentID)
		err = f.replaceParent(ctx, srcInfo, srcDirectoryID, dstDirectoryID)
		if err != nil {
			fs.Debugf(name, "Move: quick path parent replace failed: %v", err)
			return err
		}
	}

	return nil

OnConflict:
	fs.Debugf(name, "Could not directly rename file, presumably because there was a file with the same name already. Instead, the file will now be trashed where such operations do not cause errors. It will be restored to the correct parent after. If any of the subsequent calls fails, the rename/move will be in an invalid state.")

	// fs.Debugf(name, "Trashing file")
	err = f.removeNode(ctx, srcInfo)
	if err != nil {
		fs.Debugf(name, "Move: remove node failed: %v", err)
		return err
	}
	// fs.Debugf(name, "Renaming file")
	_, err = f.renameNode(ctx, srcInfo, dstLeaf)
	if err != nil {
		fs.Debugf(name, "Move: rename node failed: %v", err)
		return err
	}
	// note: replacing parent is forbidden by API, modifying them individually is
	// okay though
	// fs.Debugf(name, "Adding target parent")
	err = f.addParent(ctx, srcInfo, dstDirectoryID)
	if err != nil {
		fs.Debugf(name, "Move: addParent failed: %v", err)
		return err
	}
	// fs.Debugf(name, "removing original parent")
	err = f.removeParent(ctx, srcInfo, srcDirectoryID)
	if err != nil {
		fs.Debugf(name, "Move: removeParent failed: %v", err)
		return err
	}
	// fs.Debugf(name, "Restoring")
	_, err = f.restoreNode(ctx, srcInfo)
	if err != nil {
		fs.Debugf(name, "Move: restoreNode node failed: %v", err)
		return err
	}
	return nil
}

// MimeType of an Object if known, "" otherwise
func (o *Object) MimeType(ctx context.Context) string {
	if o.info.ContentProperties != nil && o.info.ContentProperties.ContentType != nil {
		return *o.info.ContentProperties.ContentType
	}
	return ""
}

// ChangeNotify calls the passed function with a path that has had changes.
// If the implementation uses polling, it should adhere to the given interval.
//
// Automatically restarts itself in case of unexpected behaviour of the remote.
//
// Close the returned channel to stop being notified.
func (f *Fs) ChangeNotify(ctx context.Context, notifyFunc func(string, fs.EntryType), pollIntervalChan <-chan time.Duration) {
	checkpoint := f.opt.Checkpoint

	go func() {
		var ticker *time.Ticker
		var tickerC <-chan time.Time
		for {
			select {
			case pollInterval, ok := <-pollIntervalChan:
				if !ok {
					if ticker != nil {
						ticker.Stop()
					}
					return
				}
				if pollInterval == 0 {
					if ticker != nil {
						ticker.Stop()
						ticker, tickerC = nil, nil
					}
				} else {
					ticker = time.NewTicker(pollInterval)
					tickerC = ticker.C
				}
			case <-tickerC:
				checkpoint = f.changeNotifyRunner(notifyFunc, checkpoint)
				if err := config.SetValueAndSave(f.name, "checkpoint", checkpoint); err != nil {
					fs.Debugf(f, "Unable to save checkpoint: %v", err)
				}
			}
		}
	}()
}

func (f *Fs) changeNotifyRunner(notifyFunc func(string, fs.EntryType), checkpoint string) string {
	var err error
	var resp *http.Response
	var reachedEnd bool
	var csCount int
	var nodeCount int

	fs.Debugf(f, "Checking for changes on remote (Checkpoint %q)", checkpoint)
	err = f.pacer.CallNoRetry(func() (bool, error) {
		resp, err = f.c.Changes.GetChangesFunc(&acd.ChangesOptions{
			Checkpoint:    checkpoint,
			IncludePurged: true,
		}, func(changeSet *acd.ChangeSet, err error) error {
			if err != nil {
				return err
			}

			type entryType struct {
				path      string
				entryType fs.EntryType
			}
			var pathsToClear []entryType
			csCount++
			nodeCount += len(changeSet.Nodes)
			if changeSet.End {
				reachedEnd = true
			}
			if changeSet.Checkpoint != "" {
				checkpoint = changeSet.Checkpoint
			}
			for _, node := range changeSet.Nodes {
				if path, ok := f.dirCache.GetInv(*node.Id); ok {
					if node.IsFile() {
						pathsToClear = append(pathsToClear, entryType{path: path, entryType: fs.EntryObject})
					} else {
						pathsToClear = append(pathsToClear, entryType{path: path, entryType: fs.EntryDirectory})
					}
					continue
				}

				if node.IsFile() {
					// translate the parent dir of this object
					if len(node.Parents) > 0 {
						if path, ok := f.dirCache.GetInv(node.Parents[0]); ok {
							// and append the drive file name to compute the full file name
							name := f.opt.Enc.ToStandardName(*node.Name)
							if len(path) > 0 {
								path = path + "/" + name
							} else {
								path = name
							}
							// this will now clear the actual file too
							pathsToClear = append(pathsToClear, entryType{path: path, entryType: fs.EntryObject})
						}
					} else { // a true root object that is changed
						pathsToClear = append(pathsToClear, entryType{path: *node.Name, entryType: fs.EntryObject})
					}
				}
			}

			visitedPaths := make(map[string]bool)
			for _, entry := range pathsToClear {
				if _, ok := visitedPaths[entry.path]; ok {
					continue
				}
				visitedPaths[entry.path] = true
				notifyFunc(entry.path, entry.entryType)
			}

			return nil
		})
		return false, err
	})
	fs.Debugf(f, "Got %d ChangeSets with %d Nodes", csCount, nodeCount)

	if err != nil && err != io.ErrUnexpectedEOF {
		fs.Debugf(f, "Failed to get Changes: %v", err)
		return checkpoint
	}

	if reachedEnd {
		reachedEnd = false
		fs.Debugf(f, "All changes were processed. Waiting for more.")
	} else if checkpoint == "" {
		fs.Debugf(f, "Did not get any checkpoint, something went wrong! %+v", resp)
	}
	return checkpoint
}

// ID returns the ID of the Object if known, or "" if not
func (o *Object) ID() string {
	if o.info.Id == nil {
		return ""
	}
	return *o.info.Id
}

// Check the interfaces are satisfied
var (
	_ fs.Fs     = (*Fs)(nil)
	_ fs.Purger = (*Fs)(nil)
	//	_ fs.Copier   = (*Fs)(nil)
	_ fs.Mover           = (*Fs)(nil)
	_ fs.DirMover        = (*Fs)(nil)
	_ fs.DirCacheFlusher = (*Fs)(nil)
	_ fs.ChangeNotifier  = (*Fs)(nil)
	_ fs.Object          = (*Object)(nil)
	_ fs.MimeTyper       = &Object{}
	_ fs.IDer            = &Object{}
)
