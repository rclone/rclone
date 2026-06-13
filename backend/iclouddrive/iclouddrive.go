//go:build !plan9 && !solaris

// Package iclouddrive implements the iCloud Drive backend
package iclouddrive

import (
	"bytes"
	"context"
	"path"

	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config/configmap"
	"github.com/rclone/rclone/fs/fserrors"

	"github.com/rclone/rclone/backend/iclouddrive/api"
	"github.com/rclone/rclone/fs/hash"
	"github.com/rclone/rclone/lib/dircache"
	"github.com/rclone/rclone/lib/encoder"
	"github.com/rclone/rclone/lib/pacer"

	"golang.org/x/text/unicode/norm"
)

/*
- dirCache operates on relative path to root
- path sanitization
	- rule of thumb: sanitize before use, but store things as-is
	- the paths cached in dirCache are after sanitizing
	- the remote/dir passed in aren't, and are stored as-is
*/

const (
	configAppleID    = "apple_id"
	configPassword   = "password"
	configClientID   = "client_id"
	configCookies    = "cookies"
	configTrustToken = "trust_token"

	minSleep      = 10 * time.Millisecond
	maxSleep      = 2 * time.Second
	decayConstant = 2
)

// Options defines the configuration for this backend
type Options struct {
	AppleID    string               `config:"apple_id"`
	Password   string               `config:"password"`
	TrustToken string               `config:"trust_token"`
	Cookies    string               `config:"cookies"`
	ClientID   string               `config:"client_id"`
	Enc        encoder.MultiEncoder `config:"encoding"`
}

// Fs represents a remote icloud drive
type Fs struct {
	name     string // name of this remote
	root     string // the path we are working on.
	rootID   string
	opt      Options            // parsed config options
	m        configmap.Mapper   // config map for persisting auth state
	features *fs.Features       // optional features
	dirCache *dircache.DirCache // Map of directory path to directory id
	icloud   *api.Client
	service  *api.DriveService
	pacer    *fs.Pacer // pacer for API calls
}

// Object describes an icloud drive object
type Object struct {
	fs          *Fs       // what this object is part of
	remote      string    // The remote path (relative to the fs.root)
	size        int64     // size of the object (on server, after encryption)
	modTime     time.Time // modification time of the object
	createdTime time.Time // creation time of the object
	driveID     string    // item ID of the object
	docID       string    // document ID of the object
	itemID      string    // item ID of the object
	etag        string
	downloadURL string
}

// find item by path. Will not return any children for the item
func (f *Fs) findItem(ctx context.Context, dir string) (item *api.DriveItem, found bool, err error) {
	var resp *http.Response
	if err = f.pacer.Call(func() (bool, error) {
		item, resp, err = f.service.GetItemByPath(ctx, path.Join(f.root, dir))
		return shouldRetry(ctx, resp, err)
	}); err != nil {
		if item == nil && resp.StatusCode == 404 {
			return nil, false, nil
		}
		return nil, false, err
	}

	return item, true, nil
}

func (f *Fs) findLeafItem(ctx context.Context, pathID string, leaf string) (item *api.DriveItem, found bool, err error) {
	items, err := f.listAll(ctx, pathID)
	if err != nil {
		return nil, false, err
	}
	for _, item := range items {
		// iCloud returns file names in NFD Unicode normalization, so normalized to NFC for consistent comparison
		if strings.EqualFold(norm.NFC.String(item.FullName()), leaf) {
			return item, true, nil
		}
	}

	return nil, false, nil

}

// FindLeaf finds a directory of name leaf in the folder with ID pathID
func (f *Fs) FindLeaf(ctx context.Context, pathID string, leaf string) (pathIDOut string, found bool, err error) {
	item, found, err := f.findLeafItem(ctx, pathID, leaf)

	if err != nil {
		return "", found, err
	}

	if !found {
		return "", false, err
	}

	if !item.IsFolder() {
		return "", false, fs.ErrorIsFile
	}

	return f.folderID(item), true, nil
}

// Features implements fs.Fs.
func (f *Fs) Features() *fs.Features {
	return f.features
}

// Hashes are not exposed anywhere
func (f *Fs) Hashes() hash.Set {
	return hash.Set(hash.None)
}

func (f *Fs) purgeCheck(ctx context.Context, dir string, check bool) error {
	root := path.Join(f.root, dir)
	if root == "" {
		return errors.New("can't purge root directory")
	}

	jDirectoryID, err := f.dirCache.FindDir(ctx, dir, false)
	if err != nil {
		return err
	}
	directoryID, etag := f.parseNormalizedID(jDirectoryID)

	if api.IsSharedFolderChildID(directoryID) {
		if !check {
			return fs.ErrorCantPurge
		}
		items, err := f.listAll(ctx, jDirectoryID)
		if err != nil {
			return err
		}
		if len(items) > 0 {
			return fs.ErrorDirectoryNotEmpty
		}
		cd := f.icloud.CloudDocsService(f.pacer, shouldRetry)
		if cd == nil {
			return errors.New("iclouddrive: cannot remove shared directory: account has no ckdatabasews service")
		}
		dirUUID, err := f.cloudDocsDirectoryRecordID(ctx, cd, dir, directoryID)
		if err != nil {
			return err
		}
		if dirUUID == "" {
			return fmt.Errorf("iclouddrive: cannot remove shared directory %q: missing CloudDocs directory ID", dir)
		}
		if err := cd.DeleteDirectory(ctx, dirUUID); err != nil {
			return err
		}
		f.dirCache.FlushDir(dir)
		return nil
	}

	if check {
		item, found, err := f.findItem(ctx, dir)
		if err != nil {
			return err
		}

		if found && item.DirectChildrenCount > 0 {
			return fs.ErrorDirectoryNotEmpty
		}
	}

	var _ *api.DriveItem
	var resp *http.Response
	if err = f.pacer.Call(func() (bool, error) {
		_, resp, err = f.service.MoveItemToTrashByID(ctx, directoryID, etag, true)
		return retryResultUnknown(ctx, resp, err)
	}); err != nil {
		return err
	}

	// flush everything from the left of the dir
	f.dirCache.FlushDir(dir)

	return nil
}

// Purge all files in the directory specified
//
// Implement this if you have a way of deleting all the files
// quicker than just running Remove() on the result of List()
//
// Return an error if it doesn't exist
func (f *Fs) Purge(ctx context.Context, dir string) error {
	if dir == "" {
		return fs.ErrorCantPurge
	}
	return f.purgeCheck(ctx, dir, false)
}

func (f *Fs) listAll(ctx context.Context, dirID string) (items []*api.DriveItem, err error) {
	var resp *http.Response

	id, _ := f.parseNormalizedID(dirID)
	itemID := f.parseSharedItemID(dirID)

	// Items inside a folder shared by another Apple ID cannot be listed through the
	// drivews retrieveItemDetailsInFolders endpoint (it only operates within the
	// caller's own zone and returns HTTP 400). Use the docws enumerate-by-item_id
	// endpoint instead. See api.IsSharedFolderChildID.
	if itemID != "" && api.IsSharedFolderChildID(id) {
		var raws []*api.DriveItemRaw
		if err = f.pacer.Call(func() (bool, error) {
			raws, resp, err = f.service.GetItemsInFolder(ctx, itemID, 5000)
			return shouldRetry(ctx, resp, err)
		}); err != nil {
			return nil, err
		}
		items = make([]*api.DriveItem, 0, len(raws))
		for _, raw := range raws {
			items = append(items, raw.IntoDriveItem())
		}
	} else {
		var item *api.DriveItem
		if err = f.pacer.Call(func() (bool, error) {
			item, resp, err = f.service.GetItemByDriveID(ctx, id, true)
			return shouldRetry(ctx, resp, err)
		}); err != nil {
			return nil, err
		}
		items = item.Items
	}

	for i, item := range items {
		item.Name = f.opt.Enc.ToStandardName(item.Name)
		item.Extension = f.opt.Enc.ToStandardName(item.Extension)
		items[i] = item
	}

	return items, nil
}

// List implements fs.Fs.
func (f *Fs) List(ctx context.Context, dir string) (entries fs.DirEntries, err error) {
	dirRemoteID, err := f.dirCache.FindDir(ctx, dir, false)
	if err != nil {
		return nil, err
	}

	entries = make(fs.DirEntries, 0)
	items, err := f.listAll(ctx, dirRemoteID)

	if err != nil {
		return nil, err
	}

	for _, item := range items {
		name := item.FullName()
		remote := path.Join(dir, name)
		if item.IsFolder() {
			jid := f.putFolderCache(item, remote)
			d := fs.NewDir(remote, item.DateModified).SetID(jid).SetSize(item.AssetQuota)
			entries = append(entries, d)
		} else {
			o, err := f.NewObjectFromDriveItem(ctx, remote, item)
			if err != nil {
				return nil, err
			}
			entries = append(entries, o)
		}
	}

	return entries, nil
}

// Mkdir implements fs.Fs.
func (f *Fs) Mkdir(ctx context.Context, dir string) error {
	_, _, err := f.FindDir(ctx, dir, true)
	return err
}

// Name implements fs.Fs.
func (f *Fs) Name() string {
	return f.name
}

// Precision implements fs.Fs.
func (f *Fs) Precision() time.Duration {
	return time.Second
}

// Copy src to this remote using server-side copy operations.
//
// This is stored with the remote path given.
//
// It returns the destination Object and a possible error.
//
// Will only be called if src.Fs().Name() == f.Name()
//
// If it isn't possible then return fs.ErrorCantCopy
//
//nolint:all
func (f *Fs) Copy(ctx context.Context, src fs.Object, remote string) (fs.Object, error) {
	// ICloud cooy endpoint is broken. Once they fixed it this can be re-enabled.
	return nil, fs.ErrorCantCopy

	// note: so many calls its only just faster then a reupload for big files.
	srcObj, ok := src.(*Object)
	if !ok {
		fs.Debugf(src, "Can't copy - not same remote type")
		return nil, fs.ErrorCantCopy
	}

	file, pathID, _, err := f.FindPath(ctx, remote, true)
	if err != nil {
		return nil, err
	}

	var resp *http.Response
	var info *api.DriveItemRaw

	// make a copy
	if err = f.pacer.Call(func() (bool, error) {
		info, resp, err = f.service.CopyDocByItemID(ctx, srcObj.itemID)
		return retryResultUnknown(ctx, resp, err)
	}); err != nil {
		return nil, err
	}

	// renaming in CopyDocByID endpoint does not work :/ so do it the hard way

	// get new document
	var doc *api.Document
	if err = f.pacer.Call(func() (bool, error) {
		doc, resp, err = f.service.GetDocByItemID(ctx, info.ItemID)
		return shouldRetry(ctx, resp, err)
	}); err != nil {
		return nil, err
	}

	// get parentdrive id
	var dirDoc *api.Document
	if err = f.pacer.Call(func() (bool, error) {
		dirDoc, resp, err = f.service.GetDocByItemID(ctx, pathID)
		return shouldRetry(ctx, resp, err)
	}); err != nil {
		return nil, err
	}

	// build request
	// can't use normal rename as file needs to be "activated" first

	r := api.NewUpdateFileInfo()
	r.DocumentID = doc.DocumentID
	r.Path.Path = file
	r.Path.StartingDocumentID = dirDoc.DocumentID
	r.Data.Signature = doc.Data.Signature
	r.Data.ReferenceSignature = doc.Data.ReferenceSignature
	r.Data.WrappingKey = doc.Data.WrappingKey
	r.Data.Size = doc.Data.Size
	r.Mtime = srcObj.modTime.UnixMilli()
	r.Btime = srcObj.modTime.UnixMilli()

	var item *api.DriveItem
	if err = f.pacer.Call(func() (bool, error) {
		item, resp, err = f.service.UpdateFile(ctx, &r)
		return retryResultUnknown(ctx, resp, err)
	}); err != nil {
		return nil, err
	}

	o, err := f.NewObjectFromDriveItem(ctx, remote, item)
	if err != nil {
		return nil, err
	}
	obj := o.(*Object)

	// cheat unit tests
	obj.modTime = srcObj.modTime
	obj.createdTime = srcObj.createdTime

	return obj, nil
}

// Put in to the remote path with the modTime given of the given size
//
// When called from outside an Fs by rclone, src.Size() will always be >= 0.
// But for unknown-sized objects (indicated by src.Size() == -1), Put should either
// return an error or upload it properly (rather than e.g. calling panic).
//
// May create the object even if it returns an error - if so
// will return the object and the error, otherwise will return
// nil and the error
func (f *Fs) Put(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error) {
	size := src.Size()
	if size < 0 {
		return nil, errors.New("file size unknown")
	}
	existingObj, err := f.NewObject(ctx, src.Remote())
	switch err {
	case nil:
		// object is found
		return existingObj, existingObj.Update(ctx, in, src, options...)
	case fs.ErrorObjectNotFound:
		// object not found, so we need to create it
		remote := src.Remote()
		size := src.Size()
		modTime := src.ModTime(ctx)

		obj, err := f.createObject(ctx, remote, modTime, size)
		if err != nil {
			return nil, err
		}
		return obj, obj.Update(ctx, in, src, options...)
	default:
		// real error caught
		return nil, err
	}
}

// DirCacheFlush resets the directory cache - used in testing as an
// optional interface
func (f *Fs) DirCacheFlush() {
	f.dirCache.ResetRoot()
}

// parseNormalizedID parses a normalized ID (may be in the form `driveID#itemID` or just `itemID`)
// and returns itemID, driveID, rootURL.
// Such a normalized ID can come from (*Item).GetID()
//
// Parameters:
// - rid: the normalized ID to be parsed
//
// Returns:
// - id: the itemID extracted from the normalized ID
// - etag: the driveID extracted from the normalized ID, or an empty string if not present
func (f *Fs) parseNormalizedID(rid string) (id string, etag string) {
	split := strings.Split(rid, "#")
	if len(split) == 1 {
		return split[0], ""
	}
	return split[0], split[1]
}

// parseSharedItemID returns the item_id embedded in a normalized ID, if present.
//
// For items that live inside a folder shared by another Apple ID we cache the id as
// `drivewsid#etag#itemID`, because such items can only be addressed through the docws
// endpoints by item_id (the drivewsid is unusable, and may even be empty). Normal
// (own-zone) items are cached as `drivewsid#etag` and this returns "".
func (f *Fs) parseSharedItemID(rid string) string {
	split := strings.Split(rid, "#")
	if len(split) >= 3 {
		return split[2]
	}
	return ""
}

// folderID builds the normalized directory cache id for a DriveItem. For items inside
// a folder shared by another Apple ID it additionally embeds the item_id (see
// parseSharedItemID); for normal items it is identical to IDJoin so behaviour is
// unchanged.
func (f *Fs) folderID(item *api.DriveItem) string {
	base := f.IDJoin(item.Drivewsid, item.Etag)
	if item.Itemid != "" && api.IsSharedFolderChildID(item.Drivewsid) {
		base += "#" + item.Itemid
	}
	return base
}

// FindPath finds the leaf and directoryID from a normalized path
func (f *Fs) FindPath(ctx context.Context, remote string, create bool) (leaf, directoryID, etag string, err error) {
	leaf, jDirectoryID, err := f.dirCache.FindPath(ctx, remote, create)
	if err != nil {
		return "", "", "", err
	}
	directoryID, etag = f.parseNormalizedID(jDirectoryID)
	return leaf, directoryID, etag, nil
}

// FindDir finds the directory passed in returning the directory ID
// starting from pathID
func (f *Fs) FindDir(ctx context.Context, path string, create bool) (pathID string, etag string, err error) {
	jDirectoryID, err := f.dirCache.FindDir(ctx, path, create)
	if err != nil {
		return "", "", err
	}
	directoryID, etag := f.parseNormalizedID(jDirectoryID)
	return directoryID, etag, nil
}

// IDJoin joins the given ID and ETag into a single string with a "#" delimiter.
func (f *Fs) IDJoin(id string, etag string) string {
	if strings.Contains(id, "#") {
		// already contains an etag, replace
		id, _ = f.parseNormalizedID(id)
	}

	return strings.Join([]string{id, etag}, "#")
}

func (f *Fs) putFolderCache(item *api.DriveItem, remote string) string {
	jid := f.folderID(item)
	f.dirCache.Put(remote, jid)
	return jid
}

func (f *Fs) cloudDocsDirectoryRecordID(ctx context.Context, cd *api.CloudDocs, dirRemote, dirID string) (string, error) {
	if dirRemote == "." {
		dirRemote = ""
	}
	return f.cloudDocsDirectoryRecordIDAbs(ctx, cd, path.Join(f.root, dirRemote), dirID)
}

func (f *Fs) cloudDocsDirectoryRecordIDAbs(ctx context.Context, cd *api.CloudDocs, absDir, dirID string) (string, error) {
	if api.IsSharedFolderChildID(dirID) {
		if dirID == "" {
			parent, leaf := dircache.SplitPath(absDir)
			dc := dircache.New("", f.rootID, f)
			parentJID, err := dc.FindDir(ctx, parent, false)
			if err != nil {
				return "", err
			}
			parentID, _ := f.parseNormalizedID(parentJID)
			parentRecordID, err := f.cloudDocsDirectoryRecordIDAbs(ctx, cd, parent, parentID)
			if err != nil {
				return "", err
			}
			return cd.FindDirectoryUUID(ctx, parentRecordID, leaf)
		}
		return api.GetDocIDFromDriveID(dirID), nil
	}
	if isSharedFolderRootID(dirID) {
		parent, leaf := dircache.SplitPath(absDir)
		dc := dircache.New("", f.rootID, f)
		parentID, err := dc.FindDir(ctx, parent, false)
		if err != nil {
			return "", err
		}
		item, found, err := f.findLeafItem(ctx, parentID, leaf)
		if err != nil {
			return "", err
		}
		if !found || item == nil || item.ShareID == nil || item.ShareID.RecordName == "" {
			return "", fmt.Errorf("iclouddrive: shared root %q has no CloudDocs shareID", dirID)
		}
		return item.ShareID.RecordName, nil
	}
	return "", fmt.Errorf("iclouddrive: directory %q is not in a shared folder", dirID)
}

// Rmdir implements fs.Fs.
func (f *Fs) Rmdir(ctx context.Context, dir string) error {
	return f.purgeCheck(ctx, dir, true)
}

// Root implements fs.Fs.
func (f *Fs) Root() string {
	return f.opt.Enc.ToStandardPath(f.root)
}

// String implements fs.Fs.
func (f *Fs) String() string {
	return f.root
}

// CreateDir makes a directory with pathID as parent and name leaf
//
// This should be implemented by the backend and will be called by the
// dircache package when appropriate.
func (f *Fs) CreateDir(ctx context.Context, pathID, leaf string) (string, error) {
	var item *api.DriveItem
	var err error
	var found bool
	var resp *http.Response
	if err = f.pacer.Call(func() (bool, error) {
		id, _ := f.parseNormalizedID(pathID)
		item, resp, err = f.service.CreateNewFolderByDriveID(ctx, id, f.opt.Enc.FromStandardName(leaf))

		// check if it went oke
		if requestError, ok := err.(*api.RequestError); ok {
			if requestError.Status == "unknown" {
				fs.Debugf(requestError, " checking if dir is created with separate call.")
				time.Sleep(1 * time.Second) // sleep to give icloud time to clear up its mind
				item, found, err = f.findLeafItem(ctx, pathID, leaf)
				if err != nil {
					return false, err
				}

				if !found {
					// lets assume it failed and retry
					return true, err
				}

				// success, clear err
				err = nil
			}
		}

		return ignoreResultUnknown(ctx, resp, err)
	}); err != nil {
		return "", err
	}

	return f.folderID(item), err
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

	srcID, jsrcDirectoryID, srcLeaf, jdstDirectoryID, dstLeaf, err := f.dirCache.DirMove(ctx, srcFs.dirCache, srcFs.root, srcRemote, f.root, dstRemote)
	if err != nil {
		return err
	}

	srcDirectoryID, srcEtag := f.parseNormalizedID(jsrcDirectoryID)
	dstDirectoryID, _ := f.parseNormalizedID(jdstDirectoryID)

	_, err = f.move(ctx, srcID, srcDirectoryID, srcLeaf, srcEtag, dstDirectoryID, dstLeaf)
	if err != nil {
		return err
	}

	srcFs.dirCache.FlushDir(srcRemote)

	return nil
}

func (f *Fs) move(ctx context.Context, ID, srcDirectoryID, srcLeaf, srcEtag, dstDirectoryID, dstLeaf string) (*api.DriveItem, error) {
	var resp *http.Response
	var item *api.DriveItem
	var err error

	// move
	if srcDirectoryID != dstDirectoryID {
		if err = f.pacer.Call(func() (bool, error) {
			id, _ := f.parseNormalizedID(ID)
			item, resp, err = f.service.MoveItemByDriveID(ctx, id, srcEtag, dstDirectoryID, true)
			return ignoreResultUnknown(ctx, resp, err)
		}); err != nil {
			return nil, err
		}
		ID = item.Drivewsid
		srcEtag = item.Etag
	}

	// rename
	if srcLeaf != dstLeaf {
		if err = f.pacer.Call(func() (bool, error) {
			id, _ := f.parseNormalizedID(ID)
			item, resp, err = f.service.RenameItemByDriveID(ctx, id, srcEtag, dstLeaf, true)
			return ignoreResultUnknown(ctx, resp, err)
		}); err != nil {
			return item, err
		}
	}

	return item, err
}

// Move moves the src object to the specified remote.
func (f *Fs) Move(ctx context.Context, src fs.Object, remote string) (fs.Object, error) {
	srcObj, ok := src.(*Object)
	if !ok {
		fs.Debugf(src, "Can't move - not same remote type")
		return nil, fs.ErrorCantMove
	}

	srcLeaf, srcDirectoryID, _, err := srcObj.fs.FindPath(ctx, srcObj.remote, true)
	if err != nil {
		return nil, err
	}

	dstLeaf, dstDirectoryID, _, err := f.FindPath(ctx, remote, true)
	if err != nil {
		return nil, err
	}

	if isSharedWriteDirID(srcDirectoryID) || isSharedWriteDirID(dstDirectoryID) || api.IsSharedFolderChildID(srcObj.driveID) {
		return nil, fs.ErrorCantMove
	}

	item, err := f.move(ctx, srcObj.driveID, srcDirectoryID, srcLeaf, srcObj.etag, dstDirectoryID, dstLeaf)
	if err != nil {
		return src, err
	}

	return f.NewObjectFromDriveItem(ctx, remote, item)
}

// Creates from the parameters passed in a half finished Object which
// must have setMetaData called on it
//
// Returns the object, leaf, directoryID and error.
//
// Used to create new objects
func (f *Fs) createObject(ctx context.Context, remote string, modTime time.Time, size int64) (o *Object, err error) {
	// Create the directory for the object if it doesn't exist
	_, _, _, err = f.FindPath(ctx, remote, true)
	if err != nil {
		return
	}
	// Temporary Object under construction
	o = &Object{
		fs:      f,
		remote:  remote,
		modTime: modTime,
		size:    size,
	}
	return o, nil
}

// ReadCookies parses the raw cookie string and returns an array of http.Cookie objects.
func ReadCookies(raw string) []*http.Cookie {
	header := http.Header{}
	header.Add("Cookie", raw)
	request := http.Request{Header: header}
	return request.Cookies()
}

var retryErrorCodes = []int{
	400, // icloud is a mess, sometimes returns 400 on a perfectly fine request. So just retry
	408, // Request Timeout
	409, // Conflict, retry could fix it.
	429, // Rate exceeded.
	500, // Get occasional 500 Internal Server Error
	502, // Server overload
	503, // Service Unavailable
	504, // Gateway Time-out
}

func shouldRetry(ctx context.Context, resp *http.Response, err error) (bool, error) {
	if fserrors.ContextError(ctx, &err) {
		return false, err
	}

	return fserrors.ShouldRetry(err) || fserrors.ShouldRetryHTTP(resp, retryErrorCodes), err
}

func ignoreResultUnknown(ctx context.Context, resp *http.Response, err error) (bool, error) {
	if requestError, ok := err.(*api.RequestError); ok {
		if requestError.Status == "unknown" {
			fs.Debugf(requestError, " ignoring.")
			return false, nil
		}
	}
	return shouldRetry(ctx, resp, err)
}

func retryResultUnknown(ctx context.Context, resp *http.Response, err error) (bool, error) {
	if requestError, ok := err.(*api.RequestError); ok {
		if requestError.Status == "unknown" {
			fs.Debugf(requestError, " retrying.")
			return true, err
		}
	}
	return shouldRetry(ctx, resp, err)
}

// NewFs constructs an Fs from the path, container:path
func NewFs(ctx context.Context, name, root string, m configmap.Mapper) (fs.Fs, error) {
	icloud, opt, err := newICloudClient(ctx, name, m, api.WsDrive)
	if err != nil {
		return nil, err
	}

	root = strings.Trim(root, "/")

	f := &Fs{
		name:   name,
		root:   root,
		icloud: icloud,
		rootID: "FOLDER::com.apple.CloudDocs::root",
		opt:    *opt,
		m:      m,
		pacer:  fs.NewPacer(ctx, pacer.NewDefault(pacer.MinSleep(minSleep), pacer.MaxSleep(maxSleep), pacer.DecayConstant(decayConstant))),
	}
	f.features = (&fs.Features{
		CanHaveEmptyDirectories: true,
		PartialUploads:          false,
	}).Fill(ctx, f)

	rootID := f.rootID
	f.service, err = icloud.DriveService()
	if err != nil {
		return nil, err
	}

	f.dirCache = dircache.New(
		root,
		rootID,
		f,
	)

	err = f.dirCache.FindRoot(ctx, false)
	if err != nil {
		// Assume it is a file
		newRoot, remote := dircache.SplitPath(root)
		tempF := *f
		tempF.dirCache = dircache.New(newRoot, rootID, &tempF)
		tempF.root = newRoot
		// Make new Fs which is the parent
		err = tempF.dirCache.FindRoot(ctx, false)
		if err != nil {
			// No root so return old f
			return f, nil
		}

		_, err := tempF.NewObject(ctx, remote)
		if err != nil {
			if err == fs.ErrorObjectNotFound {
				// File doesn't exist so return old f
				return f, nil
			}

			return nil, err
		}

		f.dirCache = tempF.dirCache
		f.root = tempF.root
		// return an error with an fs which points to the parent
		return f, fs.ErrorIsFile
	}

	return f, nil
}

// NewObject creates a new fs.Object from a given remote string.
//
// ctx: The context.Context for the function.
// remote: The remote string representing the object's location.
// Returns an fs.Object and an error.
func (f *Fs) NewObject(ctx context.Context, remote string) (fs.Object, error) {
	return f.NewObjectFromDriveItem(ctx, remote, nil)
}

// NewObjectFromDriveItem creates a new fs.Object from a given remote string and DriveItem.
//
// ctx: The context.Context for the function.
// remote: The remote string representing the object's location.
// item: The optional DriveItem to use for initializing the Object. If nil, the function will read the metadata from the remote location.
// Returns an fs.Object and an error.
func (f *Fs) NewObjectFromDriveItem(ctx context.Context, remote string, item *api.DriveItem) (fs.Object, error) {
	o := &Object{
		fs:     f,
		remote: remote,
	}
	if item != nil {
		err := o.setMetaData(item)
		if err != nil {
			return nil, err
		}
	} else {
		item, err := f.readMetaData(ctx, remote)

		if err != nil {
			return nil, err
		}

		err = o.setMetaData(item)
		if err != nil {
			return nil, err
		}
	}

	return o, nil
}

func (f *Fs) readMetaData(ctx context.Context, path string) (item *api.DriveItem, err error) {
	// Use the full normalized directory ID (which, for items inside a shared folder,
	// carries the item_id needed to list the folder via the docws endpoint). Going
	// through f.FindPath here would strip it down to drivewsid#etag - see folderID
	// and parseSharedItemID.
	leaf, directoryID, err := f.dirCache.FindPath(ctx, path, false)

	if err != nil {
		if err == fs.ErrorDirNotFound {
			return nil, fs.ErrorObjectNotFound
		}
		return nil, err
	}

	item, found, err := f.findLeafItem(ctx, directoryID, leaf)

	if err != nil {
		return nil, err
	}

	if !found {
		return nil, fs.ErrorObjectNotFound
	}

	return item, nil
}

func (o *Object) setMetaData(item *api.DriveItem) (err error) {
	if item.IsFolder() {
		return fs.ErrorIsDir
	}
	o.size = item.Size
	o.modTime = item.DateModified
	o.createdTime = item.DateCreated
	o.driveID = item.Drivewsid
	o.docID = item.Docwsid
	o.itemID = item.Itemid
	o.etag = item.Etag
	o.downloadURL = item.DownloadURL()
	return nil
}

// ID returns the ID of the Object if known, or "" if not
func (o *Object) ID() string {
	return o.driveID
}

// Fs implements fs.Object.
func (o *Object) Fs() fs.Info {
	return o.fs
}

// Hash implements fs.Object.
func (o *Object) Hash(ctx context.Context, t hash.Type) (string, error) {
	return "", hash.ErrUnsupported
}

// ModTime implements fs.Object.
func (o *Object) ModTime(context.Context) time.Time {
	return o.modTime
}

// Open implements fs.Object.
func (o *Object) Open(ctx context.Context, options ...fs.OpenOption) (io.ReadCloser, error) {
	fs.FixRangeOption(options, o.size)

	// Drive does not support empty files, so we cheat
	if o.size == 0 {
		return io.NopCloser(bytes.NewBufferString("")), nil
	}

	// Files inside a folder shared by another Apple ID have no usable drivewsid
	// (they are addressed by item_id via the docws endpoints), so the by_id download
	// lookup is not available — the enumerate/item response gives a direct download
	// URL instead. That URL is short-lived, but an Object can live a long time (e.g.
	// in a VFS mount cache), so the cached URL may have gone stale: refresh it from
	// the item lookup and retry on a download failure.
	shared := api.IsSharedFolderChildID(o.driveID)

	url, err := o.resolveDownloadURL(ctx, shared, false)
	if err != nil {
		return nil, err
	}

	resp, err := o.downloadURLBody(ctx, url, options)
	if err != nil && shared {
		fs.Debugf(o, "iclouddrive: shared download failed (%v), refreshing download URL and retrying", err)
		var fresh string
		if fresh, err = o.resolveDownloadURL(ctx, shared, true); err == nil {
			resp, err = o.downloadURLBody(ctx, fresh, options)
		}
	}
	if err != nil {
		return nil, err
	}

	return resp.Body, nil
}

// resolveDownloadURL returns the download URL for the object. For own-zone items it
// resolves via the by-id lookup. For files inside a folder shared by another Apple
// ID it returns the cached URL, re-resolving it via the item lookup when the cache
// is empty or a refresh is forced (the shared URL is short-lived).
func (o *Object) resolveDownloadURL(ctx context.Context, shared, refresh bool) (string, error) {
	if !shared {
		var url string
		var resp *http.Response
		var err error
		if err = o.fs.pacer.Call(func() (bool, error) {
			url, resp, err = o.fs.service.GetDownloadURLByDriveID(ctx, o.driveID)
			return shouldRetry(ctx, resp, err)
		}); err != nil {
			return "", err
		}
		return url, nil
	}
	if o.downloadURL != "" && !refresh {
		return o.downloadURL, nil
	}
	var item *api.DriveItemRaw
	var resp *http.Response
	var err error
	if err = o.fs.pacer.Call(func() (bool, error) {
		item, resp, err = o.fs.service.GetItemRawByItemID(ctx, o.itemID)
		return shouldRetry(ctx, resp, err)
	}); err != nil {
		return "", err
	}
	if item == nil || item.ItemInfo == nil || item.ItemInfo.Urls.URLDownload == "" {
		return "", fmt.Errorf("iclouddrive: could not resolve download URL for %q", o.remote)
	}
	o.downloadURL = item.ItemInfo.Urls.URLDownload
	return o.downloadURL, nil
}

// downloadURLBody fetches the given URL through the pacer and returns the response.
func (o *Object) downloadURLBody(ctx context.Context, url string, options []fs.OpenOption) (*http.Response, error) {
	var resp *http.Response
	var err error
	if err = o.fs.pacer.Call(func() (bool, error) {
		resp, err = o.fs.service.DownloadFile(ctx, url, options)
		return shouldRetry(ctx, resp, err)
	}); err != nil {
		return nil, err
	}
	return resp, nil
}

// Remote implements fs.Object.
func (o *Object) Remote() string {
	return o.remote
}

// Remove implements fs.Object.
func (o *Object) Remove(ctx context.Context) error {
	if o.itemID == "" {
		return nil
	}

	leaf, dirID, err := o.fs.dirCache.FindPath(ctx, o.remote, false)
	if err == nil {
		id, _ := o.fs.parseNormalizedID(dirID)
		if api.IsSharedFolderChildID(id) || api.IsSharedFolderChildID(o.driveID) || o.driveID == "" {
			cd := o.fs.icloud.CloudDocsService(o.fs.pacer, shouldRetry)
			if cd == nil {
				return errors.New("iclouddrive: cannot remove shared file: account has no ckdatabasews service")
			}
			dirRecordID, err := o.fs.cloudDocsDirectoryRecordID(ctx, cd, path.Dir(path.Clean(o.remote)), id)
			if err != nil {
				return err
			}
			uuid, err := cd.FindFileUUID(ctx, dirRecordID, leaf)
			if err != nil {
				return err
			}
			if uuid == "" {
				return fs.ErrorObjectNotFound
			}
			if err := cd.DeleteFile(ctx, uuid); err != nil {
				return err
			}
			o.fs.dirCache.FlushDir(path.Dir(o.remote))
			return nil
		}
	} else if err != fs.ErrorDirNotFound {
		return err
	}

	var resp *http.Response
	if err = o.fs.pacer.Call(func() (bool, error) {
		_, resp, err = o.fs.service.MoveItemToTrashByID(ctx, o.driveID, o.etag, true)
		return retryResultUnknown(ctx, resp, err)
	}); err != nil {
		return err
	}

	return nil
}

// SetModTime implements fs.Object.
func (o *Object) SetModTime(ctx context.Context, t time.Time) error {
	return fs.ErrorCantSetModTime
}

// Size implements fs.Object.
func (o *Object) Size() int64 {
	return o.size
}

// Storable implements fs.Object.
func (o *Object) Storable() bool {
	return true
}

// String implements fs.Object.
func (o *Object) String() string {
	if o == nil {
		return "<nil>"
	}
	return o.remote
}

// Update implements fs.Object.
func (o *Object) Update(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) error {
	size := src.Size()
	if size < 0 {
		return errors.New("file size unknown")
	}

	remote := o.Remote()
	modTime := src.ModTime(ctx)

	leaf, dirID, _, err := o.fs.FindPath(ctx, path.Clean(remote), true)
	if err != nil {
		return err
	}

	// Items whose parent is a folder shared by another Apple ID cannot be written
	// through the drivews/docws upload endpoints (they only operate in the caller's
	// own zone and return HTTP 400). Route those through CloudKit instead.
	if isSharedWriteDirID(dirID) {
		return o.updateViaCloudDocs(ctx, in, src, leaf, dirID, modTime, size)
	}

	// Move current file to trash
	if o.driveID != "" {
		err = o.Remove(ctx)
		if err != nil {
			return err
		}
	}

	name := o.fs.opt.Enc.FromStandardName(leaf)
	var resp *http.Response

	// Create document
	var uploadInfo *api.UploadResponse
	if err = o.fs.pacer.Call(func() (bool, error) {
		uploadInfo, resp, err = o.fs.service.CreateUpload(ctx, size, name)
		return ignoreResultUnknown(ctx, resp, err)
	}); err != nil {
		return err
	}

	// Upload content
	var upload *api.SingleFileResponse
	if err = o.fs.pacer.Call(func() (bool, error) {
		upload, resp, err = o.fs.service.Upload(ctx, in, size, name, uploadInfo.URL)
		return ignoreResultUnknown(ctx, resp, err)
	}); err != nil {
		return err
	}

	//var doc *api.Document
	//if err = o.fs.pacer.Call(func() (bool, error) {
	//	doc, resp, err = o.fs.service.GetDocByItemID(ctx, dirID)
	//	return ignoreResultUnknown(ctx, resp, err)
	//}); err != nil {
	//	return err
	//}

	r := api.NewUpdateFileInfo()
	r.DocumentID = uploadInfo.DocumentID
	r.Path.Path = name
	r.Path.StartingDocumentID = api.GetDocIDFromDriveID(dirID)
	//r.Path.StartingDocumentID = doc.DocumentID
	r.Data.Receipt = upload.SingleFile.Receipt
	r.Data.Signature = upload.SingleFile.Signature
	r.Data.ReferenceSignature = upload.SingleFile.ReferenceSignature
	r.Data.WrappingKey = upload.SingleFile.WrappingKey
	r.Data.Size = upload.SingleFile.Size
	r.Mtime = modTime.Unix() * 1000
	r.Btime = modTime.Unix() * 1000

	// Update metadata
	var item *api.DriveItem
	if err = o.fs.pacer.Call(func() (bool, error) {
		item, resp, err = o.fs.service.UpdateFile(ctx, &r)
		return ignoreResultUnknown(ctx, resp, err)
	}); err != nil {
		return err
	}

	err = o.setMetaData(item)
	if err != nil {
		return err
	}

	o.modTime = modTime
	o.size = src.Size()

	return nil
}

func isSharedFolderRootID(id string) bool {
	return strings.HasPrefix(id, "SHARED_FOLDER::")
}

func isSharedWriteDirID(id string) bool {
	return isSharedFolderRootID(id) || api.IsSharedFolderChildID(id)
}

func sharedUploadTempName(leaf, token string) string {
	ext := ""
	if !strings.HasSuffix(leaf, ".") {
		if i := strings.LastIndexByte(leaf, '.'); i > 0 {
			ext = leaf[i:]
		}
	}
	return ".rclone-upload-" + token + ext
}

// findSharedFolderRoot returns the drivewsid of the nearest SHARED_FOLDER ancestor
// of remote. The share may be nested below the drive root, or the Fs root itself
// may be inside the share, so don't assume the first relative path component is
// the shared folder.
func (f *Fs) findSharedFolderRoot(ctx context.Context, remote string) (string, error) {
	absPath := path.Join(f.root, remote)
	dir := path.Dir(path.Clean(absPath))
	if dir == "." {
		dir = ""
	}
	dc := dircache.New("", f.rootID, f)
	for {
		jid, err := dc.FindDir(ctx, dir, false)
		if err != nil {
			return "", err
		}
		id, _ := f.parseNormalizedID(jid)
		if isSharedFolderRootID(id) {
			return id, nil
		}
		if dir == "" {
			break
		}
		dir = path.Dir(dir)
		if dir == "." {
			dir = ""
		}
	}

	return "", fmt.Errorf("iclouddrive: could not find shared-folder root for %q", remote)
}

// updateViaCloudDocs writes a document into a folder shared by another Apple ID.
// drivews/docws cannot do this directly, so it: (1) uploads the document under a
// temporary collision-free name into the caller's own zone root, (2) moves it into
// the share root (which puts it in the owner's zone with PCS chaining done
// server-side), then (3) re-parents and renames the record into the target
// sub-folder via the CloudKit shared database. See api.CloudDocs.
func (o *Object) updateViaCloudDocs(ctx context.Context, in io.Reader, src fs.ObjectInfo, leaf, dirID string, modTime time.Time, size int64) error {
	f := o.fs
	cd := f.icloud.CloudDocsService(f.pacer, shouldRetry)
	if cd == nil {
		return errors.New("iclouddrive: cannot write into a shared sub-folder: account has no ckdatabasews service")
	}

	remote := o.Remote()
	dirRemote := path.Dir(path.Clean(remote))
	targetDirUUID, err := f.cloudDocsDirectoryRecordID(ctx, cd, dirRemote, dirID)
	if err != nil {
		return err
	}
	if targetDirUUID == "" {
		return fmt.Errorf("iclouddrive: cannot write into shared directory %q: missing CloudDocs directory ID", dirRemote)
	}

	// The file must first land in the owner's zone via an addressable SHARED_FOLDER
	// ancestor. Find that ancestor from the directory cache instead of assuming
	// shared folders are direct children of the drive root.
	shareRootID, err := f.findSharedFolderRoot(ctx, remote)
	if err != nil {
		return err
	}

	name := f.opt.Enc.FromStandardName(leaf)
	tempName := f.opt.Enc.FromStandardName(sharedUploadTempName(leaf, uuid.NewString()))
	var resp *http.Response

	// (1) Upload into the caller's own zone root under a temporary name. The final
	// name is applied in CloudDocs after the record has moved into the owner's zone,
	// avoiding "name 2" suffixes if the share root already contains the same leaf.
	var uploadInfo *api.UploadResponse
	if err = f.pacer.Call(func() (bool, error) {
		uploadInfo, resp, err = f.service.CreateUpload(ctx, size, tempName)
		return ignoreResultUnknown(ctx, resp, err)
	}); err != nil {
		return err
	}
	var upload *api.SingleFileResponse
	if err = f.pacer.Call(func() (bool, error) {
		upload, resp, err = f.service.Upload(ctx, in, size, tempName, uploadInfo.URL)
		return ignoreResultUnknown(ctx, resp, err)
	}); err != nil {
		return err
	}
	r := api.NewUpdateFileInfo()
	r.DocumentID = uploadInfo.DocumentID
	r.Path.Path = tempName
	r.Path.StartingDocumentID = api.GetDocIDFromDriveID(f.rootID) // own zone root
	r.Data.Receipt = upload.SingleFile.Receipt
	r.Data.Signature = upload.SingleFile.Signature
	r.Data.ReferenceSignature = upload.SingleFile.ReferenceSignature
	r.Data.WrappingKey = upload.SingleFile.WrappingKey
	r.Data.Size = upload.SingleFile.Size
	r.Mtime = modTime.Unix() * 1000
	r.Btime = modTime.Unix() * 1000
	var ownItem *api.DriveItem
	if err = f.pacer.Call(func() (bool, error) {
		ownItem, resp, err = f.service.UpdateFile(ctx, &r)
		return ignoreResultUnknown(ctx, resp, err)
	}); err != nil {
		return err
	}

	// (2) Move it into the share root (lands in the owner's zone, PCS applied).
	var moved *api.DriveItem
	if err = f.pacer.Call(func() (bool, error) {
		moved, resp, err = f.service.MoveItemByDriveID(ctx, ownItem.Drivewsid, ownItem.Etag, shareRootID, true)
		return ignoreResultUnknown(ctx, resp, err)
	}); err != nil {
		return err
	}
	uuid := api.GetDocIDFromDriveID(moved.Drivewsid)

	// Overwrite: if a file with the same name already exists in the target folder,
	// remove its record so we don't leave a duplicate (best-effort). The lookup is
	// not query-indexable, so a miss is possible; log it so a resulting duplicate is
	// diagnosable.
	if o.itemID != "" {
		oldUUID, qerr := cd.FindFileUUID(ctx, targetDirUUID, leaf)
		switch {
		case qerr != nil:
			fs.Debugf(o, "iclouddrive: could not look up existing %q in shared folder to overwrite: %v", leaf, qerr)
		case oldUUID == "":
			fs.Debugf(o, "iclouddrive: existing copy of %q not found in shared folder before overwrite; a duplicate may result", leaf)
		case !strings.EqualFold(oldUUID, uuid):
			if derr := cd.DeleteFile(ctx, oldUUID); derr != nil {
				fs.Debugf(o, "iclouddrive: could not delete previous copy of %q (%s): %v", leaf, oldUUID, derr)
			}
		}
	}

	// (3) Re-parent the record into the target shared sub-folder and set the final
	// name. The CloudDocs request layer already paces and retries each call, so no
	// extra pacer here.
	if err = cd.ReparentStructure(ctx, uuid, targetDirUUID, name); err != nil {
		return err
	}

	o.size = size
	o.modTime = modTime
	o.docID = uuid
	// Best-effort: refresh metadata (item_id etc.) from the now-placed file.
	f.dirCache.FlushDir(dirRemote)
	if item, merr := f.readMetaData(ctx, remote); merr == nil {
		_ = o.setMetaData(item)
		o.modTime = modTime
		o.size = size
	}
	return nil
}

// Disconnect clears authentication state and removes disk caches
func (f *Fs) Disconnect(ctx context.Context) error {
	return disconnectClient(f.m, f.icloud)
}

// Check interfaces are satisfied
var (
	_ fs.Fs              = &Fs{}
	_ fs.Disconnecter    = (*Fs)(nil)
	_ fs.Mover           = (*Fs)(nil)
	_ fs.Purger          = (*Fs)(nil)
	_ fs.DirMover        = (*Fs)(nil)
	_ fs.DirCacheFlusher = (*Fs)(nil)
	_ fs.Copier          = (*Fs)(nil)
	_ fs.Object          = &Object{}
	_ fs.IDer            = (*Object)(nil)
)
