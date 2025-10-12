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

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config"
	"github.com/rclone/rclone/fs/config/configmap"
	"github.com/rclone/rclone/fs/config/configstruct"
	"github.com/rclone/rclone/fs/config/obscure"
	"github.com/rclone/rclone/fs/fserrors"

	"github.com/rclone/rclone/backend/iclouddrive/api"
	"github.com/rclone/rclone/fs/hash"
	"github.com/rclone/rclone/lib/dircache"
	"github.com/rclone/rclone/lib/encoder"
	"github.com/rclone/rclone/lib/pacer"
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

// Register with Fs
func init() {
	fs.Register(&fs.RegInfo{
		Name:        "iclouddrive",
		Description: "iCloud Drive",
		Config:      Config,
		NewFs:       NewFs,
		Options: []fs.Option{{
			Name:      configAppleID,
			Help:      "Apple ID.",
			Required:  true,
			Sensitive: true,
		}, {
			Name:       configPassword,
			Help:       "Password.",
			Required:   true,
			IsPassword: true,
			Sensitive:  true,
		}, {
			Name:       configTrustToken,
			Help:       "Trust token (internal use)",
			IsPassword: false,
			Required:   false,
			Sensitive:  true,
			Hide:       fs.OptionHideBoth,
		}, {
			Name:      configCookies,
			Help:      "cookies (internal use only)",
			Required:  false,
			Advanced:  false,
			Sensitive: true,
			Hide:      fs.OptionHideBoth,
		}, {
			Name:     configClientID,
			Help:     "Client id",
			Required: false,
			Advanced: true,
			Default:  "d39ba9916b7251055b22c7f910e2ea796ee65e98b2ddecea8f5dde8d9d1a815d",
		}, {
			Name:     config.ConfigEncoding,
			Help:     config.ConfigEncodingHelp,
			Advanced: true,
			Default: (encoder.Display |
				//encoder.EncodeDot |
				encoder.EncodeBackSlash |
				encoder.EncodeInvalidUtf8),
		}},
	})
}

// Options defines the configuration for this backend
type Options struct {
	AppleID    string               `config:"apple_id"`
	Password   string               `config:"password"`
	Photos     bool                 `config:"photos"`
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

// Config configures the iCloud remote.
func Config(ctx context.Context, name string, m configmap.Mapper, config fs.ConfigIn) (*fs.ConfigOut, error) {
	var err error
	appleid, _ := m.Get(configAppleID)
	if appleid == "" {
		return nil, errors.New("a apple ID is required")
	}

	password, _ := m.Get(configPassword)
	if password != "" {
		password, err = obscure.Reveal(password)
		if err != nil {
			return nil, err
		}
	}

	trustToken, _ := m.Get(configTrustToken)
	cookieRaw, _ := m.Get(configCookies)
	clientID, _ := m.Get(configClientID)
	cookies := ReadCookies(cookieRaw)

	switch config.State {
	case "":
		icloud, err := api.New(appleid, password, trustToken, clientID, cookies, nil)
		if err != nil {
			return nil, err
		}
		if err := icloud.Authenticate(ctx); err != nil {
			return nil, err
		}
		m.Set(configCookies, icloud.Session.GetCookieString())
		if icloud.Session.Requires2FA() {
			return fs.ConfigInput("2fa_do", "config_2fa", "Two-factor authentication: please enter your 2FA code")
		}
		return nil, nil
	case "2fa_do":
		code := config.Result
		if code == "" {
			return fs.ConfigError("authenticate", "2FA codes can't be blank")
		}

		icloud, err := api.New(appleid, password, trustToken, clientID, cookies, nil)
		if err != nil {
			return nil, err
		}
		if err := icloud.SignIn(ctx); err != nil {
			return nil, err
		}

		if err := icloud.Session.Validate2FACode(ctx, code); err != nil {
			return nil, err
		}

		m.Set(configTrustToken, icloud.Session.TrustToken)
		m.Set(configCookies, icloud.Session.GetCookieString())
		return nil, nil

	case "2fa_error":
		if config.Result == "true" {
			return fs.ConfigGoto("2fa")
		}
		return nil, errors.New("2fa authentication failed")
	}
	return nil, fmt.Errorf("unknown state %q", config.State)
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
		if strings.EqualFold(item.FullName(), leaf) {
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

	return f.IDJoin(item.Drivewsid, item.Etag), true, nil
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

	directoryID, etag, err := f.FindDir(ctx, dir, false)
	if err != nil {
		return err
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
	var item *api.DriveItem
	var resp *http.Response

	if err = f.pacer.Call(func() (bool, error) {
		id, _ := f.parseNormalizedID(dirID)
		item, resp, err = f.service.GetItemByDriveID(ctx, id, true)
		return shouldRetry(ctx, resp, err)
	}); err != nil {
		return nil, err
	}

	items = item.Items

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
		id := item.Drivewsid
		name := item.FullName()
		remote := path.Join(dir, name)
		if item.IsFolder() {
			jid := f.putFolderCache(id, item.Etag, remote)
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

func (f *Fs) putFolderCache(id, etag, remote string) string {
	jid := f.IDJoin(id, etag)
	f.dirCache.Put(remote, f.IDJoin(id, etag))
	return jid
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

	return f.IDJoin(item.Drivewsid, item.Etag), err
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
	// Parse config into Options struct
	opt := new(Options)
	err := configstruct.Set(m, opt)
	if err != nil {
		return nil, err
	}

	if opt.Password != "" {
		var err error
		opt.Password, err = obscure.Reveal(opt.Password)
		if err != nil {
			return nil, fmt.Errorf("couldn't decrypt user password: %w", err)
		}
	}

	if opt.TrustToken == "" {
		return nil, fmt.Errorf("missing icloud trust token: try refreshing it with \"rclone config reconnect %s:\"", name)
	}

	cookies := ReadCookies(opt.Cookies)

	callback := func(session *api.Session) {
		m.Set(configCookies, session.GetCookieString())
	}

	icloud, err := api.New(
		opt.AppleID,
		opt.Password,
		opt.TrustToken,
		opt.ClientID,
		cookies,
		callback,
	)
	if err != nil {
		return nil, err
	}

	if err := icloud.Authenticate(ctx); err != nil {
		return nil, err
	}

	if icloud.Session.Requires2FA() {
		return nil, errors.New("trust token expired, please reauth")
	}

	root = strings.Trim(root, "/")

	f := &Fs{
		name:   name,
		root:   root,
		icloud: icloud,
		rootID: "FOLDER::com.apple.CloudDocs::root",
		opt:    *opt,
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
	leaf, ID, _, err := f.FindPath(ctx, path, false)

	if err != nil {
		if err == fs.ErrorDirNotFound {
			return nil, fs.ErrorObjectNotFound
		}
		return nil, err
	}

	item, found, err := f.findLeafItem(ctx, ID, leaf)

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

	var resp *http.Response
	var err error

	if err = o.fs.pacer.Call(func() (bool, error) {
		var url string

		//var doc *api.Document
		//if o.docID == "" {
		//doc, resp, err = o.fs.service.GetDocByItemID(ctx, o.itemID)
		//}

		// Can not get the download url on a item to work, so do it the hard way.
		url, _, err = o.fs.service.GetDownloadURLByDriveID(ctx, o.driveID)

		resp, err = o.fs.service.DownloadFile(ctx, url, options)
		return shouldRetry(ctx, resp, err)
	}); err != nil {
		return nil, err
	}

	return resp.Body, err
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

	var resp *http.Response
	var err error
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

// Check interfaces are satisfied
var (
	_ fs.Fs              = &Fs{}
	_ fs.Mover           = (*Fs)(nil)
	_ fs.Purger          = (*Fs)(nil)
	_ fs.DirMover        = (*Fs)(nil)
	_ fs.DirCacheFlusher = (*Fs)(nil)
	_ fs.Copier          = (*Fs)(nil)
	_ fs.Object          = &Object{}
	_ fs.IDer            = (*Object)(nil)
)
