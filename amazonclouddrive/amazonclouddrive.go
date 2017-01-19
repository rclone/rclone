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
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"path"
	"regexp"
	"strings"
	"sync/atomic"
	"time"

	"github.com/ncw/go-acd"
	"github.com/ncw/rclone/dircache"
	"github.com/ncw/rclone/fs"
	"github.com/ncw/rclone/oauthutil"
	"github.com/ncw/rclone/pacer"
	"github.com/ncw/rclone/rest"
	"github.com/pkg/errors"
	"golang.org/x/oauth2"
)

const (
	rcloneClientID              = "amzn1.application-oa2-client.6bf18d2d1f5b485c94c8988bb03ad0e7"
	rcloneEncryptedClientSecret = "ZP12wYlGw198FtmqfOxyNAGXU3fwVcQdmt--ba1d00wJnUs0LOzvVyXVDbqhbcUqnr5Vd1QejwWmiv1Ep7UJG1kUQeuBP5n9goXWd5MrAf0"
	folderKind                  = "FOLDER"
	fileKind                    = "FILE"
	assetKind                   = "ASSET"
	statusAvailable             = "AVAILABLE"
	timeFormat                  = time.RFC3339 // 2014-03-07T22:31:12.173Z
	minSleep                    = 20 * time.Millisecond
	warnFileSize                = 50000 << 20 // Display warning for files larger than this size
)

// Globals
var (
	// Flags
	tempLinkThreshold = fs.SizeSuffix(9 << 30) // Download files bigger than this via the tempLink
	uploadWaitPerGB   = fs.DurationP("acd-upload-wait-per-gb", "", 180*time.Second, "Additional time per GB to wait after a failed complete upload to see if it appears.")
	// Description of how to auth for this app
	acdConfig = &oauth2.Config{
		Scopes: []string{"clouddrive:read_all", "clouddrive:write"},
		Endpoint: oauth2.Endpoint{
			AuthURL:  "https://www.amazon.com/ap/oa",
			TokenURL: "https://api.amazon.com/auth/o2/token",
		},
		ClientID:     rcloneClientID,
		ClientSecret: fs.MustReveal(rcloneEncryptedClientSecret),
		RedirectURL:  oauthutil.RedirectURL,
	}
)

// Register with Fs
func init() {
	fs.Register(&fs.RegInfo{
		Name:        "amazon cloud drive",
		Description: "Amazon Drive",
		NewFs:       NewFs,
		Config: func(name string) {
			err := oauthutil.Config("amazon cloud drive", name, acdConfig)
			if err != nil {
				log.Fatalf("Failed to configure token: %v", err)
			}
		},
		Options: []fs.Option{{
			Name: fs.ConfigClientID,
			Help: "Amazon Application Client Id - leave blank normally.",
		}, {
			Name: fs.ConfigClientSecret,
			Help: "Amazon Application Client Secret - leave blank normally.",
		}},
	})
	fs.VarP(&tempLinkThreshold, "acd-templink-threshold", "", "Files >= this size will be downloaded via their tempLink.")
}

// Fs represents a remote acd server
type Fs struct {
	name         string                 // name of this remote
	features     *fs.Features           // optional features
	c            *acd.Client            // the connection to the acd server
	noAuthClient *http.Client           // unauthenticated http client
	root         string                 // the path we are working on
	dirCache     *dircache.DirCache     // Map of directory path to directory id
	pacer        *pacer.Pacer           // pacer for API calls
	ts           *oauthutil.TokenSource // token source for oauth
	uploads      int32                  // number of uploads in progress - atomic access required
	trueRootID   string                 // ID of true root directory
}

// Object describes a acd object
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

// Pattern to match a acd path
var matcher = regexp.MustCompile(`^([^/]*)(.*)$`)

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
func (f *Fs) shouldRetry(resp *http.Response, err error) (bool, error) {
	if resp != nil {
		if resp.StatusCode == 401 {
			f.ts.Invalidate()
			fs.Log(f, "401 error received - invalidating token")
			return true, err
		}
		// Work around receiving this error sporadically on authentication
		//
		// HTTP code 403: "403 Forbidden", reponse body: {"message":"Authorization header requires 'Credential' parameter. Authorization header requires 'Signature' parameter. Authorization header requires 'SignedHeaders' parameter. Authorization header requires existence of either a 'X-Amz-Date' or a 'Date' header. Authorization=Bearer"}
		if resp.StatusCode == 403 && strings.Contains(err.Error(), "Authorization header requires") {
			fs.Log(f, "403 \"Authorization header requires...\" error received - retry")
			return true, err
		}
	}
	return fs.ShouldRetry(err) || fs.ShouldRetryHTTP(resp, retryErrorCodes), err
}

// NewFs constructs an Fs from the path, container:path
func NewFs(name, root string) (fs.Fs, error) {
	root = parsePath(root)
	oAuthClient, ts, err := oauthutil.NewClient(name, acdConfig)
	if err != nil {
		log.Fatalf("Failed to configure Amazon Drive: %v", err)
	}

	c := acd.NewClient(oAuthClient)
	f := &Fs{
		name:         name,
		root:         root,
		c:            c,
		pacer:        pacer.New().SetMinSleep(minSleep).SetPacer(pacer.AmazonCloudDrivePacer),
		noAuthClient: fs.Config.Client(),
		ts:           ts,
	}
	f.features = (&fs.Features{CaseInsensitive: true, ReadMimeType: true}).Fill(f)

	// Update endpoints
	var resp *http.Response
	err = f.pacer.Call(func() (bool, error) {
		_, resp, err = f.c.Account.GetEndpoints()
		return f.shouldRetry(resp, err)
	})
	if err != nil {
		return nil, errors.Wrap(err, "failed to get endpoints")
	}

	// Get rootID
	rootInfo, err := f.getRootInfo()
	if err != nil || rootInfo.Id == nil {
		return nil, errors.Wrap(err, "failed to get root")
	}
	f.trueRootID = *rootInfo.Id

	// Renew the token in the background
	go f.renewToken()

	f.dirCache = dircache.New(root, f.trueRootID, f)

	// Find the current root
	err = f.dirCache.FindRoot(false)
	if err != nil {
		// Assume it is a file
		newRoot, remote := dircache.SplitPath(root)
		newF := *f
		newF.dirCache = dircache.New(newRoot, f.trueRootID, &newF)
		newF.root = newRoot
		// Make new Fs which is the parent
		err = newF.dirCache.FindRoot(false)
		if err != nil {
			// No root so return old f
			return f, nil
		}
		_, err := newF.newObjectWithInfo(remote, nil)
		if err != nil {
			if err == fs.ErrorObjectNotFound {
				// File doesn't exist so return old f
				return f, nil
			}
			return nil, err
		}
		// return an error with an fs which points to the parent
		return &newF, fs.ErrorIsFile
	}
	return f, nil
}

// getRootInfo gets the root folder info
func (f *Fs) getRootInfo() (rootInfo *acd.Folder, err error) {
	var resp *http.Response
	err = f.pacer.Call(func() (bool, error) {
		rootInfo, resp, err = f.c.Nodes.GetRoot()
		return f.shouldRetry(resp, err)
	})
	return rootInfo, err
}

// Renew the token - runs in the background
//
// Renews the token whenever it expires.  Useful when there are lots
// of uploads in progress and the token doesn't get renewed.  Amazon
// seem to cancel your uploads if you don't renew your token for 2hrs.
func (f *Fs) renewToken() {
	expiry := f.ts.OnExpiry()
	for {
		<-expiry
		uploads := atomic.LoadInt32(&f.uploads)
		if uploads != 0 {
			fs.Debug(f, "Token expired - %d uploads in progress - refreshing", uploads)
			// Do a transaction
			_, err := f.getRootInfo()
			if err == nil {
				fs.Debug(f, "Token refresh successful")
			} else {
				fs.ErrorLog(f, "Token refresh failed: %v", err)
			}
		} else {
			fs.Debug(f, "Token expired but no uploads in progress - doing nothing")
		}
	}
}

func (f *Fs) startUpload() {
	atomic.AddInt32(&f.uploads, 1)
}

func (f *Fs) stopUpload() {
	atomic.AddInt32(&f.uploads, -1)
}

// Return an Object from a path
//
// If it can't be found it returns the error fs.ErrorObjectNotFound.
func (f *Fs) newObjectWithInfo(remote string, info *acd.Node) (fs.Object, error) {
	o := &Object{
		fs:     f,
		remote: remote,
	}
	if info != nil {
		// Set info but not meta
		o.info = info
	} else {
		err := o.readMetaData() // reads info and meta, returning an error
		if err != nil {
			return nil, err
		}
	}
	return o, nil
}

// NewObject finds the Object at remote.  If it can't be found
// it returns the error fs.ErrorObjectNotFound.
func (f *Fs) NewObject(remote string) (fs.Object, error) {
	return f.newObjectWithInfo(remote, nil)
}

// FindLeaf finds a directory of name leaf in the folder with ID pathID
func (f *Fs) FindLeaf(pathID, leaf string) (pathIDOut string, found bool, err error) {
	//fs.Debug(f, "FindLeaf(%q, %q)", pathID, leaf)
	folder := acd.FolderFromId(pathID, f.c.Nodes)
	var resp *http.Response
	var subFolder *acd.Folder
	err = f.pacer.Call(func() (bool, error) {
		subFolder, resp, err = folder.GetFolder(leaf)
		return f.shouldRetry(resp, err)
	})
	if err != nil {
		if err == acd.ErrorNodeNotFound {
			//fs.Debug(f, "...Not found")
			return "", false, nil
		}
		//fs.Debug(f, "...Error %v", err)
		return "", false, err
	}
	if subFolder.Status != nil && *subFolder.Status != statusAvailable {
		fs.Debug(f, "Ignoring folder %q in state %q", leaf, *subFolder.Status)
		time.Sleep(1 * time.Second) // FIXME wait for problem to go away!
		return "", false, nil
	}
	//fs.Debug(f, "...Found(%q, %v)", *subFolder.Id, leaf)
	return *subFolder.Id, true, nil
}

// CreateDir makes a directory with pathID as parent and name leaf
func (f *Fs) CreateDir(pathID, leaf string) (newID string, err error) {
	//fmt.Printf("CreateDir(%q, %q)\n", pathID, leaf)
	folder := acd.FolderFromId(pathID, f.c.Nodes)
	var resp *http.Response
	var info *acd.Folder
	err = f.pacer.Call(func() (bool, error) {
		info, resp, err = folder.CreateFolder(leaf)
		return f.shouldRetry(resp, err)
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
func (f *Fs) listAll(dirID string, title string, directoriesOnly bool, filesOnly bool, fn listAllFn) (found bool, err error) {
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
			return f.shouldRetry(resp, err)
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

// ListDir reads the directory specified by the job into out, returning any more jobs
func (f *Fs) ListDir(out fs.ListOpts, job dircache.ListDirJob) (jobs []dircache.ListDirJob, err error) {
	fs.Debug(f, "Reading %q", job.Path)
	maxTries := fs.Config.LowLevelRetries
	for tries := 1; tries <= maxTries; tries++ {
		_, err = f.listAll(job.DirID, "", false, false, func(node *acd.Node) bool {
			remote := job.Path + *node.Name
			switch *node.Kind {
			case folderKind:
				if out.IncludeDirectory(remote) {
					dir := &fs.Dir{
						Name:  remote,
						Bytes: -1,
						Count: -1,
					}
					dir.When, _ = time.Parse(timeFormat, *node.ModifiedDate) // FIXME
					if out.AddDir(dir) {
						return true
					}
					if job.Depth > 0 {
						jobs = append(jobs, dircache.ListDirJob{DirID: *node.Id, Path: remote + "/", Depth: job.Depth - 1})
					}
				}
			case fileKind:
				o, err := f.newObjectWithInfo(remote, node)
				if err != nil {
					out.SetError(err)
					return true
				}
				if out.Add(o) {
					return true
				}
			default:
				// ignore ASSET etc
			}
			return false
		})
		if fs.IsRetryError(err) {
			fs.Debug(f, "Directory listing error for %q: %v - low level retry %d/%d", job.Path, err, tries, maxTries)
			continue
		}
		if err != nil {
			return nil, err
		}
		break
	}
	fs.Debug(f, "Finished reading %q", job.Path)
	return jobs, err
}

// List walks the path returning iles and directories into out
func (f *Fs) List(out fs.ListOpts, dir string) {
	f.dirCache.List(f, out, dir)
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
func (f *Fs) checkUpload(resp *http.Response, in io.Reader, src fs.ObjectInfo, inInfo *acd.File, inErr error, uploadTime time.Duration) (fixedError bool, info *acd.File, err error) {
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
		fs.Debug(src, "Upload error detected but didn't finish upload: %v (%q)", inErr, httpStatus)
		return false, inInfo, inErr
	}

	// Don't wait for uploads - assume they will appear later
	if *uploadWaitPerGB <= 0 {
		fs.Debug(src, "Upload error detected but waiting disabled: %v (%q)", inErr, httpStatus)
		return false, inInfo, inErr
	}

	// Time we should wait for the upload
	uploadWaitPerByte := float64(*uploadWaitPerGB) / 1024 / 1024 / 1024
	timeToWait := time.Duration(uploadWaitPerByte * float64(src.Size()))

	const sleepTime = 5 * time.Second                        // sleep between tries
	retries := int((timeToWait + sleepTime - 1) / sleepTime) // number of retries, rounded up

	fs.Debug(src, "Error detected after finished upload - waiting to see if object was uploaded correctly: %v (%q)", inErr, httpStatus)
	remote := src.Remote()
	for i := 1; i <= retries; i++ {
		o, err := f.NewObject(remote)
		if err == fs.ErrorObjectNotFound {
			fs.Debug(src, "Object not found - waiting (%d/%d)", i, retries)
		} else if err != nil {
			fs.Debug(src, "Object returned error - waiting (%d/%d): %v", i, retries, err)
		} else {
			if src.Size() == o.Size() {
				fs.Debug(src, "Object found with correct size %d after waiting (%d/%d) - %v - returning with no error", src.Size(), i, retries, sleepTime*time.Duration(i-1))
				info = &acd.File{
					Node: o.(*Object).info,
				}
				return true, info, nil
			}
			fs.Debug(src, "Object found but wrong size %d vs %d - waiting (%d/%d)", src.Size(), o.Size(), i, retries)
		}
		time.Sleep(sleepTime)
	}
	fs.Debug(src, "Giving up waiting for object - returning original error: %v (%q)", inErr, httpStatus)
	return false, inInfo, inErr
}

// Put the object into the container
//
// Copy the reader in to the new object which is returned
//
// The new object may have been created if an error is returned
func (f *Fs) Put(in io.Reader, src fs.ObjectInfo) (fs.Object, error) {
	remote := src.Remote()
	size := src.Size()
	// Temporary Object under construction
	o := &Object{
		fs:     f,
		remote: remote,
	}
	// Check if object already exists
	err := o.readMetaData()
	switch err {
	case nil:
		return o, o.Update(in, src)
	case fs.ErrorObjectNotFound:
		// Not found so create it
	default:
		return nil, err
	}
	// If not create it
	leaf, directoryID, err := f.dirCache.FindPath(remote, true)
	if err != nil {
		return nil, err
	}
	if size > warnFileSize {
		fs.Debug(f, "Warning: file %q may fail because it is too big. Use --max-size=%dM to skip large files.", remote, warnFileSize>>20)
	}
	folder := acd.FolderFromId(directoryID, o.fs.c.Nodes)
	var info *acd.File
	var resp *http.Response
	err = f.pacer.CallNoRetry(func() (bool, error) {
		start := time.Now()
		f.startUpload()
		info, resp, err = folder.Put(in, leaf)
		f.stopUpload()
		var ok bool
		ok, info, err = f.checkUpload(resp, in, src, info, err, time.Since(start))
		if ok {
			return false, nil
		}
		return f.shouldRetry(resp, err)
	})
	if err != nil {
		return nil, err
	}
	o.info = info.Node
	return o, nil
}

// Mkdir creates the container if it doesn't exist
func (f *Fs) Mkdir(dir string) error {
	err := f.dirCache.FindRoot(true)
	if err != nil {
		return err
	}
	if dir != "" {
		_, err = f.dirCache.FindDir(dir, true)
	}
	return err
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
	//  go test -v -run '^Test(Setup|Init|FsMkdir|FsPutFile1|FsPutFile2|FsUpdateFile1|FsMove)$'
	srcObj, ok := src.(*Object)
	if !ok {
		fs.Debug(src, "Can't move - not same remote type")
		return nil, fs.ErrorCantMove
	}

	// create the destination directory if necessary
	err := f.dirCache.FindRoot(true)
	if err != nil {
		return nil, err
	}
	srcLeaf, srcDirectoryID, err := srcObj.fs.dirCache.FindPath(srcObj.remote, false)
	if err != nil {
		return nil, err
	}
	dstLeaf, dstDirectoryID, err := f.dirCache.FindPath(remote, true)
	if err != nil {
		return nil, err
	}
	dstInfo, err := f.moveNode(srcObj.remote, dstLeaf, dstDirectoryID, srcObj.info, srcLeaf, srcDirectoryID, false)
	if err != nil {
		return nil, err
	}
	dstObj := &Object{
		fs:     f,
		remote: remote,
		info:   dstInfo,
	}
	// Wait for directory caching so we can no longer see the old object
	time.Sleep(200 * time.Millisecond) // enough time 90% of the time
	for i := 1; i <= fs.Config.LowLevelRetries; i++ {
		_, err := srcObj.fs.NewObject(srcObj.remote) // try reading the object
		if err == fs.ErrorObjectNotFound {
			break
		} else if err != nil {
			return nil, err
		}
		fs.Debug(src, "Wait for directory listing to update after move %d/%d", i, fs.Config.LowLevelRetries)
		time.Sleep(1 * time.Second)
	}
	return dstObj, nil
}

// DirCacheFlush resets the directory cache - used in testing as an
// optional interface
func (f *Fs) DirCacheFlush() {
	f.dirCache.ResetRoot()
}

// DirMove moves src directory to this remote using server side move
// operations.
//
// Will only be called if src.Fs().Name() == f.Name()
//
// If it isn't possible then return fs.ErrorCantDirMove
//
// If destination exists then return fs.ErrorDirExists
func (f *Fs) DirMove(src fs.Fs) (err error) {
	// go test -v -run '^Test(Setup|Init|FsMkdir|FsPutFile1|FsPutFile2|FsUpdateFile1|FsDirMove)$
	srcFs, ok := src.(*Fs)
	if !ok {
		fs.Debug(src, "DirMove error: not same remote type")
		return fs.ErrorCantDirMove
	}

	// Check if destination exists
	if f.dirCache.FoundRoot() {
		fs.Debug(src, "DirMove error: destination exists")
		return fs.ErrorDirExists
	}

	// Refuse to move to or from the root
	if f.root == "" || srcFs.root == "" {
		fs.Debug(src, "DirMove error: Can't move root")
		return errors.New("can't move root directory")
	}

	// Find ID of parent
	dstLeaf, dstDirectoryID, err := f.dirCache.FindPath(f.root, true)
	if err != nil {
		return err
	}

	// Find the ID of the source and make a node from it
	err = srcFs.dirCache.FindRoot(false)
	if err != nil {
		fs.Debug(src, "DirMove error: error finding src root: %v", err)
		return err
	}
	srcDirectoryID, err := srcFs.dirCache.RootParentID()
	if err != nil {
		fs.Debug(src, "DirMove error: error finding src RootParentID: %v", err)
		return err
	}
	srcLeaf, _ := dircache.SplitPath(srcFs.root)

	// FIXME make a proper node.UpdateMetadata command
	srcInfo := acd.NodeFromId(srcFs.dirCache.RootID(), f.c.Nodes)
	var jsonStr string
	err = srcFs.pacer.Call(func() (bool, error) {
		jsonStr, err = srcInfo.GetMetadata()
		return srcFs.shouldRetry(nil, err)
	})
	if err != nil {
		fs.Debug(src, "DirMove error: error reading src metadata: %v", err)
		return err
	}
	err = json.Unmarshal([]byte(jsonStr), &srcInfo)
	if err != nil {
		fs.Debug(src, "DirMove error: error reading unpacking src metadata: %v", err)
		return err
	}

	_, err = f.moveNode(srcFs.root, dstLeaf, dstDirectoryID, srcInfo, srcLeaf, srcDirectoryID, true)

	srcFs.dirCache.ResetRoot()
	return err
}

// purgeCheck remotes the root directory, if check is set then it
// refuses to do so if it has anything in
func (f *Fs) purgeCheck(dir string, check bool) error {
	root := path.Join(f.root, dir)
	if root == "" {
		return errors.New("can't purge root directory")
	}
	dc := f.dirCache
	err := dc.FindRoot(false)
	if err != nil {
		return err
	}
	rootID, err := dc.FindDir(dir, false)
	if err != nil {
		return err
	}

	if check {
		// check directory is empty
		empty := true
		_, err = f.listAll(rootID, "", false, false, func(node *acd.Node) bool {
			switch *node.Kind {
			case folderKind:
				empty = false
				return true
			case fileKind:
				empty = false
				return true
			default:
				fs.Debug("Found ASSET %s", *node.Id)
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
		return f.shouldRetry(resp, err)
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
func (f *Fs) Rmdir(dir string) error {
	return f.purgeCheck(dir, true)
}

// Precision return the precision of this Fs
func (f *Fs) Precision() time.Duration {
	return fs.ModTimeNotSupported
}

// Hashes returns the supported hash sets.
func (f *Fs) Hashes() fs.HashSet {
	return fs.HashSet(fs.HashMD5)
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
//func (f *Fs) Copy(src fs.Object, remote string) (fs.Object, error) {
// srcObj, ok := src.(*Object)
// if !ok {
// 	fs.Debug(src, "Can't copy - not same remote type")
// 	return nil, fs.ErrorCantCopy
// }
// srcFs := srcObj.fs
// _, err := f.c.ObjectCopy(srcFs.container, srcFs.root+srcObj.remote, f.container, f.root+remote, nil)
// if err != nil {
// 	return nil, err
// }
// return f.NewObject(remote), nil
//}

// Purge deletes all the files and the container
//
// Optional interface: Only implement this if you have a way of
// deleting all the files quicker than just running Remove() on the
// result of List()
func (f *Fs) Purge() error {
	return f.purgeCheck("", false)
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
func (o *Object) Hash(t fs.HashType) (string, error) {
	if t != fs.HashMD5 {
		return "", fs.ErrHashUnsupported
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
func (o *Object) readMetaData() (err error) {
	if o.info != nil {
		return nil
	}
	leaf, directoryID, err := o.fs.dirCache.FindPath(o.remote, false)
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
		info, resp, err = folder.GetFile(leaf)
		return o.fs.shouldRetry(resp, err)
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
func (o *Object) ModTime() time.Time {
	err := o.readMetaData()
	if err != nil {
		fs.Log(o, "Failed to read metadata: %v", err)
		return time.Now()
	}
	modTime, err := time.Parse(timeFormat, *o.info.ModifiedDate)
	if err != nil {
		fs.Log(o, "Failed to read mtime from object: %v", err)
		return time.Now()
	}
	return modTime
}

// SetModTime sets the modification time of the local fs object
func (o *Object) SetModTime(modTime time.Time) error {
	// FIXME not implemented
	return fs.ErrorCantSetModTime
}

// Storable returns a boolean showing whether this object storable
func (o *Object) Storable() bool {
	return true
}

// Open an object for read
func (o *Object) Open(options ...fs.OpenOption) (in io.ReadCloser, err error) {
	bigObject := o.Size() >= int64(tempLinkThreshold)
	if bigObject {
		fs.Debug(o, "Dowloading large object via tempLink")
	}
	file := acd.File{Node: o.info}
	var resp *http.Response
	headers := fs.OpenOptionHeaders(options)
	err = o.fs.pacer.Call(func() (bool, error) {
		if !bigObject {
			in, resp, err = file.OpenHeaders(headers)
		} else {
			in, resp, err = file.OpenTempURLHeaders(rest.ClientWithHeaderReset(o.fs.noAuthClient, headers), headers)
		}
		return o.fs.shouldRetry(resp, err)
	})
	return in, err
}

// Update the object with the contents of the io.Reader, modTime and size
//
// The new object may have been created if an error is returned
func (o *Object) Update(in io.Reader, src fs.ObjectInfo) error {
	file := acd.File{Node: o.info}
	var info *acd.File
	var resp *http.Response
	var err error
	err = o.fs.pacer.CallNoRetry(func() (bool, error) {
		start := time.Now()
		o.fs.startUpload()
		info, resp, err = file.Overwrite(in)
		o.fs.stopUpload()
		var ok bool
		ok, info, err = o.fs.checkUpload(resp, in, src, info, err, time.Since(start))
		if ok {
			return false, nil
		}
		return o.fs.shouldRetry(resp, err)
	})
	if err != nil {
		return err
	}
	o.info = info.Node
	return nil
}

// Remove a node
func (f *Fs) removeNode(info *acd.Node) error {
	var resp *http.Response
	var err error
	err = f.pacer.Call(func() (bool, error) {
		resp, err = info.Trash()
		return f.shouldRetry(resp, err)
	})
	return err
}

// Remove an object
func (o *Object) Remove() error {
	return o.fs.removeNode(o.info)
}

// Restore a node
func (f *Fs) restoreNode(info *acd.Node) (newInfo *acd.Node, err error) {
	var resp *http.Response
	err = f.pacer.Call(func() (bool, error) {
		newInfo, resp, err = info.Restore()
		return f.shouldRetry(resp, err)
	})
	return newInfo, err
}

// Changes name of given node
func (f *Fs) renameNode(info *acd.Node, newName string) (newInfo *acd.Node, err error) {
	var resp *http.Response
	err = f.pacer.Call(func() (bool, error) {
		newInfo, resp, err = info.Rename(newName)
		return f.shouldRetry(resp, err)
	})
	return newInfo, err
}

// Replaces one parent with another, effectively moving the file. Leaves other
// parents untouched. ReplaceParent cannot be used when the file is trashed.
func (f *Fs) replaceParent(info *acd.Node, oldParentID string, newParentID string) error {
	return f.pacer.Call(func() (bool, error) {
		resp, err := info.ReplaceParent(oldParentID, newParentID)
		return f.shouldRetry(resp, err)
	})
}

// Adds one additional parent to object.
func (f *Fs) addParent(info *acd.Node, newParentID string) error {
	return f.pacer.Call(func() (bool, error) {
		resp, err := info.AddParent(newParentID)
		return f.shouldRetry(resp, err)
	})
}

// Remove given parent from object, leaving the other possible
// parents untouched. Object can end up having no parents.
func (f *Fs) removeParent(info *acd.Node, parentID string) error {
	return f.pacer.Call(func() (bool, error) {
		resp, err := info.RemoveParent(parentID)
		return f.shouldRetry(resp, err)
	})
}

// moveNode moves the node given from the srcLeaf,srcDirectoryID to
// the dstLeaf,dstDirectoryID
func (f *Fs) moveNode(name, dstLeaf, dstDirectoryID string, srcInfo *acd.Node, srcLeaf, srcDirectoryID string, useDirErrorMsgs bool) (dstInfo *acd.Node, err error) {
	// fs.Debug(name, "moveNode dst(%q,%s) <- src(%q,%s)", dstLeaf, dstDirectoryID, srcLeaf, srcDirectoryID)
	cantMove := fs.ErrorCantMove
	if useDirErrorMsgs {
		cantMove = fs.ErrorCantDirMove
	}

	if len(srcInfo.Parents) > 1 && srcLeaf != dstLeaf {
		fs.Debug(name, "Move error: object is attached to multiple parents and should be renamed. This would change the name of the node in all parents.")
		return nil, cantMove
	}

	if srcLeaf != dstLeaf {
		// fs.Debug(name, "renaming")
		dstInfo, err = f.renameNode(srcInfo, dstLeaf)
		if err != nil {
			fs.Debug(name, "Move: quick path rename failed: %v", err)
			goto OnConflict
		}
	}
	if srcDirectoryID != dstDirectoryID {
		// fs.Debug(name, "trying parent replace: %s -> %s", oldParentID, newParentID)
		err = f.replaceParent(srcInfo, srcDirectoryID, dstDirectoryID)
		if err != nil {
			fs.Debug(name, "Move: quick path parent replace failed: %v", err)
			return nil, err
		}
	}

	return dstInfo, nil

OnConflict:
	fs.Debug(name, "Could not directly rename file, presumably because there was a file with the same name already. Instead, the file will now be trashed where such operations do not cause errors. It will be restored to the correct parent after. If any of the subsequent calls fails, the rename/move will be in an invalid state.")

	// fs.Debug(name, "Trashing file")
	err = f.removeNode(srcInfo)
	if err != nil {
		fs.Debug(name, "Move: remove node failed: %v", err)
		return nil, err
	}
	// fs.Debug(name, "Renaming file")
	_, err = f.renameNode(srcInfo, dstLeaf)
	if err != nil {
		fs.Debug(name, "Move: rename node failed: %v", err)
		return nil, err
	}
	// note: replacing parent is forbidden by API, modifying them individually is
	// okay though
	// fs.Debug(name, "Adding target parent")
	err = f.addParent(srcInfo, dstDirectoryID)
	if err != nil {
		fs.Debug(name, "Move: addParent failed: %v", err)
		return nil, err
	}
	// fs.Debug(name, "removing original parent")
	err = f.removeParent(srcInfo, srcDirectoryID)
	if err != nil {
		fs.Debug(name, "Move: removeParent failed: %v", err)
		return nil, err
	}
	// fs.Debug(name, "Restoring")
	dstInfo, err = f.restoreNode(srcInfo)
	if err != nil {
		fs.Debug(name, "Move: restoreNode node failed: %v", err)
		return nil, err
	}
	return dstInfo, nil
}

// MimeType of an Object if known, "" otherwise
func (o *Object) MimeType() string {
	if o.info.ContentProperties != nil && o.info.ContentProperties.ContentType != nil {
		return *o.info.ContentProperties.ContentType
	}
	return ""
}

// Check the interfaces are satisfied
var (
	_ fs.Fs     = (*Fs)(nil)
	_ fs.Purger = (*Fs)(nil)
	//	_ fs.Copier   = (*Fs)(nil)
	_ fs.Mover           = (*Fs)(nil)
	_ fs.DirMover        = (*Fs)(nil)
	_ fs.DirCacheFlusher = (*Fs)(nil)
	_ fs.Object          = (*Object)(nil)
	_ fs.MimeTyper       = &Object{}
)
