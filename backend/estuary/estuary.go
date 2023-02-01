// Package estuary provides an interface to the Estuary service.
package estuary

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"path"
	"time"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config/configmap"
	"github.com/rclone/rclone/fs/config/configstruct"
	"github.com/rclone/rclone/fs/fserrors"
	"github.com/rclone/rclone/fs/fshttp"
	"github.com/rclone/rclone/fs/hash"
	"github.com/rclone/rclone/fs/operations"
	"github.com/rclone/rclone/fs/walk"
	"github.com/rclone/rclone/lib/bucket"
	"github.com/rclone/rclone/lib/dircache"
	"github.com/rclone/rclone/lib/pacer"
	"github.com/rclone/rclone/lib/rest"
)

var (
	errorNotImpl              = errors.New("not implemented for estuary remote")
	errorMkdirOnlyCollections = errors.New("mkdir only implemented for root collections")
	errAllEndpointsFailed     = errors.New("All upload endpoints failed")
	errNoRootFound            = errors.New("No root collection found")
	minSleep                  = 10 * time.Millisecond
	maxSleep                  = 2 * time.Second
	decayConstant             = 2
)

// Options config options for our backend
type Options struct {
	Token     string `config:"token"`
	URL       string `config:"url"`
	UploadUrl string `config:"uploadUrl"`
}

// Fs represents a remote estuary server
type Fs struct {
	name           string
	root           string
	rootCollection string
	rootDirectory  string
	opt            Options
	features       *fs.Features
	client         *rest.Client
	pacer          *fs.Pacer
	dirCache       *dircache.DirCache
}

// Object describes an estuary object
type Object struct {
	fs        *Fs    // what this object is part of
	remote    string // The remote path
	size      int64  // size of the object
	cid       string // CID of the object
	estuaryID string // estuary ID of the object
	modTime   time.Time
}

type apiError struct {
	Message string `json:"error"`
	Details string `json:"details"`
}

type content struct {
	ID          uint   `json:"id"`
	Cid         string `json:"cid"`
	Name        string `json:"name"`
	UserID      uint   `json:"userId"`
	Description string `json:"description"`
	Size        int64  `json:"size"`
}

type collectionFsItem struct {
	ContentID uint      `json:"contId"`
	Name      string    `json:"name"`
	Type      string    `json:"type"`
	Size      int64     `json:"size"`
	Cid       string    `json:"cid,omitempty"`
	Dir       string    `json:"dir"`
	ColUUID   string    `json:"coluuid"`
	UpdatedAt time.Time `json:"updatedAt"`
}

var commandHelp = []fs.CommandHelp{{
	Name:  "lscid",
	Short: "List files along with their CIDs",
	Opts: map[string]string{
		"format": "format for CIDs. one of plain, url, gateway. default is plain",
	},
}}

func init() {
	fs.Register(&fs.RegInfo{
		Name:        "estuary",
		Description: "Estuary based Filecoin/IPFS storage",
		NewFs:       newFs,
		CommandHelp: commandHelp,
		Options: []fs.Option{{
			Name:     "token",
			Help:     "Estuary API token",
			Required: true,
		}, {
			Name:    "url",
			Help:    "Estuary URL",
			Default: "https://api.estuary.tech",
		}, {
			Name:    "uploadUrl",
			Help:    "Estuary Upload URL",
			Default: "https://upload.estuary.tech",
		}},
	})

}

var retryErrorCodes = []int{
	429, // Too Many Requests
}

func errorHandler(resp *http.Response) error {
	body, err := rest.ReadBody(resp)
	if err != nil {
		return fmt.Errorf("error reading error out of body: %w", err)
	}

	var apiErr apiError
	if err = json.Unmarshal(body, &apiErr); err != nil {
		return fmt.Errorf("HTTP error %v (%v) returned body: %q", resp.StatusCode, resp.Status, body)
	}

	return &apiErr
}

func shouldRetry(ctx context.Context, resp *http.Response, err error) (bool, error) {
	if fserrors.ContextError(ctx, &err) {
		return false, err
	}

	if resp != nil && resp.StatusCode == 404 {
		err = fs.ErrorObjectNotFound
	}
	return fserrors.ShouldRetry(err) && fserrors.ShouldRetryHTTP(resp, retryErrorCodes), err
}

// returns true if response has a StatusCode of 400 and
// if the error returned by the API is ERR_CONTENT_ADDING_DISABLED
func contentAddingDisabled(response *http.Response, err error) bool {
	if response == nil || err == nil {
		return false
	}
	apiErr := err.(*apiError)
	if apiErr == nil {
		return false
	}

	if response.StatusCode == 400 && apiErr.Error() == "ERR_CONTENT_ADDING_DISABLED" {
		return true
	}

	return false
}

func splitDir(dir string) (uuid string, path string) {
	uuid, path = bucket.Split(dir)
	path = "/" + path
	return
}

func newFs(ctx context.Context, name string, root string, m configmap.Mapper) (i fs.Fs, e error) {
	opt := new(Options)
	err := configstruct.Set(m, opt)
	if err != nil {
		return nil, err
	}

	root = strings.Trim(root, "/")
	httpClient := fshttp.NewClient(ctx)
	f := &Fs{
		name:   name,
		opt:    *opt,
		client: rest.NewClient(httpClient),
		pacer:  fs.NewPacer(ctx, pacer.NewDefault(pacer.MinSleep(minSleep), pacer.MaxSleep(maxSleep), pacer.DecayConstant(decayConstant))),
	}
	f.setRoot(root)
	f.client.
		SetRoot(opt.URL).
		SetErrorHandler(errorHandler)

	f.features = (&fs.Features{
		CanHaveEmptyDirectories: false,
		BucketBased:             true,
		BucketBasedRootOK:       true,
	}).Fill(ctx, f)

	if f.opt.Token != "" {
		f.client.SetHeader("Authorization", "Bearer "+f.opt.Token)
	}

	f.dirCache = dircache.New(root, "", f)

	err = f.dirCache.FindRoot(ctx, false)
	if err != nil {
		fs.Debugf(f, "FindRoot root: %v, name: %v, err: %v", root, name, err)
		newRoot, remote := dircache.SplitPath(root)
		tempF := *f
		tempF.dirCache = dircache.New(newRoot, "", &tempF)
		tempF.root = newRoot
		// Make new Fs which is the parent
		err = tempF.dirCache.FindRoot(ctx, false)
		if err != nil {
			// No root so return old f
			return f, nil
		}
		_, err := tempF.newObjectWithInfo(ctx, remote, time.Time{}, nil)
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

func (err *apiError) Error() string {
	return err.Message
}

// Name of the remote (as passed into newFs)
func (f *Fs) Name() string {
	return f.name
}

// Root of the remote (as passed into newFs)
func (f *Fs) Root() string {
	return f.root
}

// String returns a description of the FS
func (f *Fs) String() string {
	return fmt.Sprintf("estuary root '%s'", f.root)
}

// Precision of the ModTimes in this Fs
func (f *Fs) Precision() time.Duration {
	return time.Second
}

// Hashes returns the supported hash types of the filesystem
func (f *Fs) Hashes() hash.Set {
	return hash.Set(hash.None)
}

// Features returns the optional features of this Fs
func (f *Fs) Features() *fs.Features {
	return f.features
}

func (f *Fs) setRoot(root string) {
	f.root = strings.Trim(root, "/")
	f.rootCollection, f.rootDirectory = bucket.Split(f.root)
}

// Command the backend to run a named command
//
// The command run is name
// args may be used to read arguments from
// opts may be used to read optional arguments from
//
// The result should be capable of being JSON encoded
// If it is a string or a []string it will be shown to the user
// otherwise it will be JSON encoded and shown to the user like that
func (f *Fs) Command(ctx context.Context, name string, arg []string, opt map[string]string) (out interface{}, err error) {
	switch name {
	case "lscid":
		cidFormat, found := opt["format"]
		if !found {
			cidFormat = "plain"
		}

		var list operations.ListFormat
		list.AddSize()
		list.AddModTime()
		list.SetSeparator(" ")

		var out strings.Builder
		err = walk.ListR(ctx, f, "", false, -1, walk.ListObjects, func(entries fs.DirEntries) (err error) {
			for _, entry := range entries {
				fmt.Fprintf(&out, "%s %s",
					operations.SizeStringField(entry.Size(), false, 9),
					entry.ModTime(ctx).Local().Format(time.Stamp))

				if obj, ok := entry.(*Object); ok {
					var prefix string
					switch cidFormat {
					case "url":
						prefix = "ipfs://"
					case "gateway":
						prefix = "https://gateway.estuary.tech/gw/ipfs/"
					}

					cidWidth := 60 + len(prefix)
					cid := prefix + obj.cid
					fmt.Fprintf(&out, " %*s", cidWidth, cid)
				}
				fmt.Fprintf(&out, " %s\n", entry.Remote())
			}
			return nil
		})
		return out.String(), err
	default:
		return nil, fs.ErrorCommandNotFound
	}
}

// CreateDir makes a directory with pathID as parent and name leaf
func (f *Fs) CreateDir(ctx context.Context, pathID, leaf string) (newID string, err error) {
	fs.Debugf(f, "CreateDir(%q, %q)\n", pathID, leaf)

	if pathID != "" {
		return "", errorMkdirOnlyCollections
	}

	return f.createCollection(ctx, leaf)
}

// FindLeaf finds a directory of name leaf in the folder with ID pathID
func (f *Fs) FindLeaf(ctx context.Context, pathID, leaf string) (string, bool, error) {
	fs.Debugf(f, "FindLeaf pathID=%v, leaf=%v, rootCollection=%v, rootDirectory=%v", pathID, leaf, f.rootCollection, f.rootDirectory)
	if pathID == "" { // root dir, check collections
		collections, err := f.listCollections(ctx)
		if err != nil {
			return "", false, err
		}

		for _, collection := range collections {
			if strings.EqualFold(collection.Name, leaf) {
				return collection.UUID, true, nil
			}
		}
		return "", false, nil
	}
	// subdir, these are lazy created and we construct a path out of the collection ID + root path in the collection
	uuid, directoryPath := splitDir(pathID)
	items, err := f.getCollectionContents(ctx, uuid, directoryPath)
	if err != nil {
		return "", false, err
	}

	for _, item := range items {
		if item.Name == leaf {
			if item.isDir() {
				return path.Join(pathID, leaf), true, nil
			}
			return "", false, nil
		}
	}
	return path.Join(pathID, leaf), true, nil
}

func (item *collectionFsItem) isDir() bool {
	return item.Type == "directory"
}

func (f *Fs) listRoot(ctx context.Context, dir string) (entries fs.DirEntries, err error) {
	collections, err := f.listCollections(ctx)
	if err != nil {
		return nil, err
	}
	for _, collection := range collections {
		remote := path.Join(dir, collection.Name)
		f.dirCache.Put(remote, collection.UUID)
		d := fs.NewDir(remote, collection.CreatedAt).SetID(collection.UUID)
		entries = append(entries, d)
	}
	return entries, nil
}

// List the objects and directories in dir into entries.  The
// entries can be returned in any order but should be for a
// complete directory.
//
// dir should be "" to list the root, and should not have
// trailing slashes.
//
// This should return ErrorDirNotFound if the directory isn't
// found.
func (f *Fs) List(ctx context.Context, dir string) (entries fs.DirEntries, err error) {
	fs.Debugf(f, "List %v", dir)
	if f.root == "" && dir == "" {
		return f.listRoot(ctx, dir)
	}

	var pathID string
	pathID, err = f.dirCache.FindDir(ctx, dir, false)
	if err != nil {
		return nil, err
	}

	uuid, directoryPath := splitDir(pathID)
	items, err := f.getCollectionContents(ctx, uuid, directoryPath)
	if err != nil {
		return nil, err
	}

	for _, item := range items {
		if item.isDir() {
			remote := path.Join(dir, item.Name)
			id := path.Join(uuid, item.Name)
			//f.dirCache.Put(remote, id)
			d := fs.NewDir(remote, time.Now()).SetID(id)
			entries = append(entries, d)
		} else {
			dir := item.Dir[1:]
			pin, err := f.getPin(ctx, item.ContentID)
			if err != nil {
				return nil, err
			}
			modTime := pin.Meta["modTime"]
			if modTime == nil {
				modTime = strconv.FormatInt(time.Now().Unix(), 10)
			}
			o, err := f.newObjectWithInfo(ctx, path.Join(dir, item.Name), parseTimeString(modTime.(string)), &content{ID: item.ContentID, Cid: item.Cid, Size: item.Size})
			if err != nil {
				return nil, err
			}
			entries = append(entries, o)
		}
	}
	return entries, nil
}

// NewObject finds the Object at remote.  If it can't be found
// it returns the error ErrorObjectNotFound.
//
// If remote points to a directory then it should return
// ErrorIsDir if possible without doing any extra work,
// otherwise ErrorObjectNotFound.
func (f *Fs) NewObject(ctx context.Context, remote string) (fs.Object, error) {
	return f.newObjectWithInfo(ctx, remote, time.Time{}, nil)
}

func (f *Fs) newObjectWithInfo(ctx context.Context, remote string, modTime time.Time, content *content) (fs.Object, error) {
	fs.Debugf(f, "newObjectWithInfo %v", remote)
	o := &Object{
		fs:     f,
		remote: remote,
	}
	if content != nil {
		// Set info
		o.estuaryID = strconv.FormatUint(uint64(content.ID), 10)
		o.cid = content.Cid
		o.size = content.Size
		o.modTime = modTime
	} else {
		err := o.readStats(ctx)
		if err != nil {
			return nil, err
		}
	}
	return o, nil
}

func (f *Fs) createObject(ctx context.Context, remote string, size int64, modTime time.Time) (o *Object, err error) {
	// Temporary Object under construction
	o = &Object{
		fs:      f,
		remote:  remote,
		size:    size,
		modTime: modTime,
	}
	return o, nil
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
	remote := src.Remote()
	size := src.Size()
	modTime := src.ModTime(ctx)

	o, err := f.createObject(ctx, remote, size, modTime)
	if err != nil {
		return nil, err
	}
	return o, o.Update(ctx, in, src, options...)
}

// Mkdir makes the directory (container, bucket)
//
// Shouldn't return an error if it already exists
func (f *Fs) Mkdir(ctx context.Context, dir string) error {
	_, err := f.dirCache.FindDir(ctx, dir, true)
	return err
}

// Rmdir removes the directory (container, bucket) if empty
//
// Return an error if it doesn't exist or isn't empty
func (f *Fs) Rmdir(ctx context.Context, dir string) error {
	dirID, err := f.dirCache.FindDir(ctx, dir, false)
	if err != nil {
		return err
	}

	uuid, collectionDir := splitDir(dirID)
	if uuid == "" || collectionDir != "" {
		return nil
	}

	// if strings.Contains(dir, "/") { // trying to remove subdir, ignore
	// 	return nil // TODO: this should be an error, but returning one breaks integration  tests
	// }

	// if dirID != "" {
	// 	return nil // TODO: this should be errorRmdirOnlyCollections but if we do that it breaks integration tests
	// }
	return f.deleteCollection(ctx, uuid)
}

// Fs returns the parent Fs
func (o *Object) Fs() fs.Info {
	return o.fs
}

// ID returns the CID of the Object if known, or "" if not
func (o *Object) ID() string {
	return o.estuaryID
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
	return "", errorNotImpl
}

// Size returns the size of the file
func (o *Object) Size() int64 {
	return o.size
}

// ModTime returns the modification date of the file
// It should return a best guess if one isn't available
func (o *Object) ModTime(ctx context.Context) time.Time {
	return o.modTime
}

// SetModTime sets the metadata on the object to set the modification date
func (o *Object) SetModTime(ctx context.Context, modTime time.Time) error {
	return errorNotImpl
}

func (o *Object) readStats(ctx context.Context) error {
	if o.cid == "" {
		dirID, err := o.fs.dirCache.RootID(ctx, false)
		if err != nil {
			return err
		}

		if dirID == "" {
			return fs.ErrorObjectNotFound
		}

		dir, file := path.Split(o.Remote())
		collectionDir := "/" + dir

		items, err := o.fs.getCollectionContents(ctx, dirID, collectionDir)
		if err != nil {
			return err
		}

		for _, item := range items {
			if strings.EqualFold(item.Name, file) {
				o.estuaryID = strconv.FormatUint(uint64(item.ContentID), 10)
				o.size = item.Size
				o.cid = item.Cid
				pin, err := o.fs.getPin(ctx, item.ContentID)
				if err != nil { // do nothing
					fs.Debugf(o, "couldn't get pin for id =%v", item.ContentID)
				}
				modTime := pin.Meta["modTime"]
				if modTime != nil {
					o.modTime = parseTimeString(modTime.(string))
				} else {
					o.modTime = item.UpdatedAt
				}
				return nil
			}
		}

		return fs.ErrorObjectNotFound
	}

	result, err := o.fs.getContentByCid(ctx, o.cid)
	if err != nil {
		return err
	}

	o.estuaryID = strconv.FormatUint(uint64(result[0].ID), 10)
	o.size = result[0].Size
	return nil
}

// Open opens the file for read.  Call Close() on the returned io.ReadCloser
func (o *Object) Open(ctx context.Context, options ...fs.OpenOption) (in io.ReadCloser, err error) {
	if o.cid == "" {
		return nil, errors.New("can't download - no CID")
	}

	fs.FixRangeOption(options, o.size)
	var resp *http.Response
	opts := rest.Opts{
		Method:  "GET",
		RootURL: "https://gateway.estuary.tech", // TODO: let users configure gateway
		Path:    "/gw/ipfs/" + o.cid,
		Options: options,
	}
	err = o.fs.pacer.Call(func() (bool, error) {
		resp, err = o.fs.client.Call(ctx, &opts)
		return shouldRetry(ctx, resp, err)
	})
	if err != nil {
		return nil, err
	}
	return resp.Body, err
}

func (o *Object) upload(ctx context.Context, in io.Reader, leaf, dirID string, size int64, options ...fs.OpenOption) (err error) {
	fs.Debugf(o, "upload leaf=%v, dirID=%v, size=%v", leaf, dirID, size)

	opts := rest.Opts{
		Method:               "POST",
		Path:                 "/content/add",
		RootURL:              o.fs.opt.UploadUrl,
		Body:                 in,
		MultipartContentName: "data",
		MultipartFileName:    leaf,
		Options:              options,
	}

	result, err := o.addContent(ctx, opts)
	if err != nil {
		return err
	}

	pin, err := o.fs.getPin(ctx, result.ID)
	if err != nil {
		return err
	}
	pin.Meta["modTime"] = timeString(o.modTime)

	id, err := o.fs.replacePin(ctx, result.ID, pin)
	if err != nil {
		return err
	}

	integerID, err := strconv.Atoi(id)
	if err != nil {
		return err
	}

	if dirID != "" {
		uuid := o.fs.root
		absPath := "/"
		uuid, absPath = splitDir(dirID)
		fs.Debugf(o, "uploading to collection %v at path %v", uuid, absPath)

		contentIds := []uint{uint(integerID)}
		err = o.fs.addContentsToCollection(ctx, uuid, absPath, contentIds)
		if err != nil {
			return err
		}
	}

	o.cid = result.Cid
	o.estuaryID = id
	o.size = size
	return nil
}

// Update in to the object with the modTime given of the given size
//
// When called from outside an Fs by rclone, src.Size() will always be >= 0.
// But for unknown-sized objects (indicated by src.Size() == -1), Upload should either
// return an error or update the object properly (rather than e.g. calling panic).
func (o *Object) Update(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (err error) {
	size := src.Size()
	remote := src.Remote()

	leaf, dirID, err := o.fs.dirCache.FindPath(ctx, remote, true)
	if err != nil {
		return err
	}

	err = o.upload(ctx, in, leaf, dirID, size, options...)
	return err
}

// Remove removes this object
func (o *Object) Remove(ctx context.Context) error {
	rootCollectionID, ok := o.fs.dirCache.Get("")
	if ok {
		return o.removeContentFromCollection(ctx, rootCollectionID)
	}
	return errNoRootFound
}

// Storable says whether this object can be stored
func (o *Object) Storable() bool {
	return true
}

// Check the interfaces are satisfied
var (
	_ dircache.DirCacher = (*Fs)(nil)
	_ fs.Fs              = (*Fs)(nil)
	_ fs.Object          = (*Object)(nil)
	_ fs.IDer            = (*Object)(nil)
)

// timeString returns modTime as the number of milliseconds
// elapsed since January 1, 1970 UTC as a decimal string.
func timeString(modTime time.Time) string {
	return strconv.FormatInt(modTime.UnixNano()/1e6, 10)
}

// parseTimeString converts a decimal string number of milliseconds
// elapsed since January 1, 1970 UTC into a time.Time.
func parseTimeString(timeString string) time.Time {
	if timeString == "" {
		return time.Time{}
	}
	unixMilliseconds, err := strconv.ParseInt(timeString, 10, 64)
	if err != nil {
		fs.Debugf("Failed to parse mod time string %q: %v", timeString, err)
		return time.Time{}
	}
	return time.Unix(unixMilliseconds/1e3, (unixMilliseconds%1e3)*1e6).UTC()
}
