// Package yandex provides an interface to the Yandex Disk storage.
//
// dibu28 <dibu28@gmail.com> github.com/dibu28
package yandex

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/ncw/rclone/fs"
	"github.com/ncw/rclone/oauthutil"
	yandex "github.com/ncw/rclone/yandex/api"
	"github.com/pkg/errors"
	"golang.org/x/oauth2"
)

//oAuth
const (
	rcloneClientID              = "ac39b43b9eba4cae8ffb788c06d816a8"
	rcloneEncryptedClientSecret = "EfyyNZ3YUEwXM5yAhi72G9YwKn2mkFrYwJNS7cY0TJAhFlX9K-uJFbGlpO-RYjrJ"
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
		ClientSecret: fs.MustReveal(rcloneEncryptedClientSecret),
		RedirectURL:  oauthutil.RedirectURL,
	}
)

// Register with Fs
func init() {
	fs.Register(&fs.RegInfo{
		Name:        "yandex",
		Description: "Yandex Disk",
		NewFs:       NewFs,
		Config: func(name string) {
			err := oauthutil.Config("yandex", name, oauthConfig)
			if err != nil {
				log.Fatalf("Failed to configure token: %v", err)
			}
		},
		Options: []fs.Option{{
			Name: fs.ConfigClientID,
			Help: "Yandex Client Id - leave blank normally.",
		}, {
			Name: fs.ConfigClientSecret,
			Help: "Yandex Client Secret - leave blank normally.",
		}},
	})
}

// Fs represents a remote yandex
type Fs struct {
	name       string
	root       string         //root path
	features   *fs.Features   // optional features
	yd         *yandex.Client // client for rest api
	diskRoot   string         //root path with "disk:/" container name
	mkdircache map[string]int
}

// Object describes a swift object
type Object struct {
	fs       *Fs       // what this object is part of
	remote   string    // The remote path
	md5sum   string    // The MD5Sum of the object
	bytes    uint64    // Bytes in the object
	modTime  time.Time // Modified time of the object
	mimeType string    // Content type according to the server
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

// Features returns the optional features of this Fs
func (f *Fs) Features() *fs.Features {
	return f.features
}

// read access token from ConfigFile string
func getAccessToken(name string) (*oauth2.Token, error) {
	// Read the token from the config file
	tokenConfig := fs.ConfigFileGet(name, "token")
	//Get access token from config string
	decoder := json.NewDecoder(strings.NewReader(tokenConfig))
	var result *oauth2.Token
	err := decoder.Decode(&result)
	if err != nil {
		return nil, err
	}
	return result, nil
}

// NewFs constructs an Fs from the path, container:path
func NewFs(name, root string) (fs.Fs, error) {
	//read access token from config
	token, err := getAccessToken(name)
	if err != nil {
		return nil, err
	}

	//create new client
	yandexDisk := yandex.NewClient(token.AccessToken, fs.Config.Client())

	f := &Fs{
		yd: yandexDisk,
	}
	f.features = (&fs.Features{ReadMimeType: true, WriteMimeType: true}).Fill(f)
	f.setRoot(root)

	// Check to see if the object exists and is a file
	//request object meta info
	var opt2 yandex.ResourceInfoRequestOptions
	if ResourceInfoResponse, err := yandexDisk.NewResourceInfoRequest(root, opt2).Exec(); err != nil {
		//return err
	} else {
		if ResourceInfoResponse.ResourceType == "file" {
			f.setRoot(path.Dir(root))
			// return an error with an fs which points to the parent
			return f, fs.ErrorIsFile
		}
	}

	return f, nil
}

// Sets root in f
func (f *Fs) setRoot(root string) {
	//Set root path
	f.root = strings.Trim(root, "/")
	//Set disk root path.
	//Adding "disk:" to root path as all paths on disk start with it
	var diskRoot = ""
	if f.root == "" {
		diskRoot = "disk:/"
	} else {
		diskRoot = "disk:/" + f.root + "/"
	}
	f.diskRoot = diskRoot
}

// listFn is called from list and listContainerRoot to handle an object.
type listFn func(remote string, item *yandex.ResourceInfoResponse, isDirectory bool) error

// listDir lists this directory only returning objects and directories
func (f *Fs) listDir(fn listFn) (err error) {
	//request object meta info
	var opt yandex.ResourceInfoRequestOptions
	ResourceInfoResponse, err := f.yd.NewResourceInfoRequest(f.diskRoot, opt).Exec()
	if err != nil {
		return err
	}
	if ResourceInfoResponse.ResourceType == "dir" {
		//list all subdirs
		for _, element := range ResourceInfoResponse.Embedded.Items {
			remote := element.Name
			switch element.ResourceType {
			case "dir":
				err = fn(remote, &element, true)
				if err != nil {
					return err
				}
			case "file":
				err = fn(remote, &element, false)
				if err != nil {
					return err
				}
			default:
				fs.Debug(f, "Unknown resource type %q", element.ResourceType)
			}
		}
	}
	return nil
}

// list the objects into the function supplied
//
// This does a flat listing of all the files in the drive
func (f *Fs) list(dir string, fn listFn) error {
	//request files list. list is divided into pages. We send request for each page
	//items per page is limited by limit
	//TODO may be add config parameter for the items per page limit
	var limit uint32 = 1000 // max number of object per request
	var itemsCount uint32   //number of items per page in response
	var offset uint32       //for the next page of request
	// yandex disk api request options
	var opt yandex.FlatFileListRequestOptions
	opt.Limit = &limit
	opt.Offset = &offset
	prefix := f.diskRoot
	if dir != "" {
		prefix += dir + "/"
	}
	//query each page of list until itemCount is less then limit
	for {
		//send request
		info, err := f.yd.NewFlatFileListRequest(opt).Exec()
		if err != nil {
			return err
		}
		itemsCount = uint32(len(info.Items))

		//list files
		for _, item := range info.Items {
			// filter file list and get only files we need
			if strings.HasPrefix(item.Path, prefix) {
				//trim root folder from filename
				var name = strings.TrimPrefix(item.Path, f.diskRoot)
				err = fn(name, &item, false)
				if err != nil {
					return err
				}
			}
		}

		//offset for the next page of items
		offset += itemsCount
		//check if we reached end of list
		if itemsCount < limit {
			break
		}
	}
	return nil
}

// List walks the path returning a channel of Objects
func (f *Fs) List(out fs.ListOpts, dir string) {
	defer out.Finished()

	listItem := func(remote string, object *yandex.ResourceInfoResponse, isDirectory bool) error {
		if isDirectory {
			t, err := time.Parse(time.RFC3339Nano, object.Modified)
			if err != nil {
				return err
			}
			dir := &fs.Dir{
				Name:  remote,
				When:  t,
				Bytes: int64(object.Size),
				Count: -1,
			}
			if out.AddDir(dir) {
				return fs.ErrorListAborted
			}
		} else {
			o, err := f.newObjectWithInfo(remote, object)
			if err != nil {
				return err
			}
			if out.Add(o) {
				return fs.ErrorListAborted
			}
		}
		return nil
	}

	var err error
	switch out.Level() {
	case 1:
		if dir == "" {
			err = f.listDir(listItem)
		} else {
			err = f.list(dir, listItem)
		}
	case fs.MaxLevel:
		err = f.list(dir, listItem)
	default:
		out.SetError(fs.ErrorLevelNotSupported)
	}

	if err != nil {
		// FIXME
		// if err == swift.ContainerNotFound {
		// 	err = fs.ErrorDirNotFound
		// }
		out.SetError(err)
	}
}

// NewObject finds the Object at remote.  If it can't be found it
// returns the error fs.ErrorObjectNotFound.
func (f *Fs) NewObject(remote string) (fs.Object, error) {
	return f.newObjectWithInfo(remote, nil)
}

// Return an Object from a path
//
// If it can't be found it returns the error fs.ErrorObjectNotFound.
func (f *Fs) newObjectWithInfo(remote string, info *yandex.ResourceInfoResponse) (fs.Object, error) {
	o := &Object{
		fs:     f,
		remote: remote,
	}
	if info != nil {
		o.setMetaData(info)
	} else {
		err := o.readMetaData()
		if err != nil {
			return nil, err
		}
	}
	return o, nil
}

// setMetaData sets the fs data from a storage.Object
func (o *Object) setMetaData(info *yandex.ResourceInfoResponse) {
	o.bytes = info.Size
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
		fs.Log("Failed to parse modtime from %q: %v", modTimeString, err)
	} else {
		o.modTime = t
	}
}

// readMetaData gets the info if it hasn't already been fetched
func (o *Object) readMetaData() (err error) {
	// exit if already fetched
	if !o.modTime.IsZero() {
		return nil
	}

	//request meta info
	var opt2 yandex.ResourceInfoRequestOptions
	ResourceInfoResponse, err := o.fs.yd.NewResourceInfoRequest(o.remotePath(), opt2).Exec()
	if err != nil {
		if dcErr, ok := err.(yandex.DiskClientError); ok {
			if dcErr.Code == "DiskNotFoundError" {
				return fs.ErrorObjectNotFound
			}
		}
		return err
	}
	o.setMetaData(ResourceInfoResponse)
	return nil
}

// Put the object
//
// Copy the reader in to the new object which is returned
//
// The new object may have been created if an error is returned
func (f *Fs) Put(in io.Reader, src fs.ObjectInfo) (fs.Object, error) {
	remote := src.Remote()
	size := src.Size()
	modTime := src.ModTime()

	o := &Object{
		fs:      f,
		remote:  remote,
		bytes:   uint64(size),
		modTime: modTime,
	}
	//TODO maybe read metadata after upload to check if file uploaded successfully
	return o, o.Update(in, src)
}

// Mkdir creates the container if it doesn't exist
func (f *Fs) Mkdir(dir string) error {
	root := f.diskRoot
	if dir != "" {
		root += dir + "/"
	}
	return mkDirFullPath(f.yd, root)
}

// Rmdir deletes the container
//
// Returns an error if it isn't empty
func (f *Fs) Rmdir(dir string) error {
	return f.purgeCheck(dir, true)
}

// purgeCheck remotes the root directory, if check is set then it
// refuses to do so if it has anything in
func (f *Fs) purgeCheck(dir string, check bool) error {
	root := f.diskRoot
	if dir != "" {
		root += dir + "/"
	}
	if check {
		//to comply with rclone logic we check if the directory is empty before delete.
		//send request to get list of objects in this directory.
		var opt yandex.ResourceInfoRequestOptions
		ResourceInfoResponse, err := f.yd.NewResourceInfoRequest(root, opt).Exec()
		if err != nil {
			return errors.Wrap(err, "rmdir failed")
		}
		if len(ResourceInfoResponse.Embedded.Items) != 0 {
			return errors.New("rmdir failed: directory not empty")
		}
	}
	//delete directory
	return f.yd.Delete(root, true)
}

// Precision return the precision of this Fs
func (f *Fs) Precision() time.Duration {
	return time.Nanosecond
}

// Purge deletes all the files and the container
//
// Optional interface: Only implement this if you have a way of
// deleting all the files quicker than just running Remove() on the
// result of List()
func (f *Fs) Purge() error {
	return f.purgeCheck("", false)
}

// Hashes returns the supported hash sets.
func (f *Fs) Hashes() fs.HashSet {
	return fs.HashSet(fs.HashMD5)
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
	return o.md5sum, nil
}

// Size returns the size of an object in bytes
func (o *Object) Size() int64 {
	var size = int64(o.bytes) //need to cast from uint64 in yandex disk to int64 in rclone. can cause overflow
	return size
}

// ModTime returns the modification time of the object
//
// It attempts to read the objects mtime and if that isn't present the
// LastModified returned in the http headers
func (o *Object) ModTime() time.Time {
	err := o.readMetaData()
	if err != nil {
		fs.Log(o, "Failed to read metadata: %v", err)
		return time.Now()
	}
	return o.modTime
}

// Open an object for read
func (o *Object) Open(options ...fs.OpenOption) (in io.ReadCloser, err error) {
	return o.fs.yd.Download(o.remotePath(), fs.OpenOptionHeaders(options))
}

// Remove an object
func (o *Object) Remove() error {
	return o.fs.yd.Delete(o.remotePath(), true)
}

// SetModTime sets the modification time of the local fs object
//
// Commits the datastore
func (o *Object) SetModTime(modTime time.Time) error {
	remote := o.remotePath()
	// set custom_property 'rclone_modified' of object to modTime
	err := o.fs.yd.SetCustomProperty(remote, "rclone_modified", modTime.Format(time.RFC3339Nano))
	if err != nil {
		return err
	}
	o.modTime = modTime
	return nil
}

// Storable returns whether this object is storable
func (o *Object) Storable() bool {
	return true
}

// Returns the remote path for the object
func (o *Object) remotePath() string {
	return o.fs.diskRoot + o.remote
}

// Update the already existing object
//
// Copy the reader into the object updating modTime and size
//
// The new object may have been created if an error is returned
func (o *Object) Update(in io.Reader, src fs.ObjectInfo) error {
	size := src.Size()
	modTime := src.ModTime()

	remote := o.remotePath()
	//create full path to file before upload.
	err1 := mkDirFullPath(o.fs.yd, remote)
	if err1 != nil {
		return err1
	}
	//upload file
	overwrite := true //overwrite existing file
	mimeType := fs.MimeType(src)
	err := o.fs.yd.Upload(in, remote, overwrite, mimeType)
	if err == nil {
		//if file uploaded sucessfully then return metadata
		o.bytes = uint64(size)
		o.modTime = modTime
		o.md5sum = "" // according to unit tests after put the md5 is empty.
		//and set modTime of uploaded file
		err = o.SetModTime(modTime)
	}
	return err
}

// utility funcs-------------------------------------------------------------------

// mkDirExecute execute mkdir
func mkDirExecute(client *yandex.Client, path string) (int, string, error) {
	statusCode, jsonErrorString, err := client.Mkdir(path)
	if statusCode == 409 { // dir already exist
		return statusCode, jsonErrorString, err
	}
	if statusCode == 201 { // dir was created
		return statusCode, jsonErrorString, err
	}
	if err != nil {
		// error creating directory
		return statusCode, jsonErrorString, errors.Wrap(err, "failed to create folder")
	}
	return 0, "", nil
}

//mkDirFullPath Creates Each Directory in the path if needed. Send request once for every directory in the path.
func mkDirFullPath(client *yandex.Client, path string) error {
	//trim filename from path
	dirString := strings.TrimSuffix(path, filepath.Base(path))
	//trim "disk:/" from path
	dirString = strings.TrimPrefix(dirString, "disk:/")

	//1 Try to create directory first
	if _, jsonErrorString, err := mkDirExecute(client, dirString); err != nil {
		er2, _ := client.ParseAPIError(jsonErrorString)
		if er2 != "DiskPathPointsToExistentDirectoryError" {
			//2 if it fails then create all directories in the path from root.
			dirs := strings.Split(dirString, "/") //path separator /
			var mkdirpath = "/"                   //path separator /
			for _, element := range dirs {
				if element != "" {
					mkdirpath += element + "/" //path separator /
					_, _, err2 := mkDirExecute(client, mkdirpath)
					if err2 != nil {
						//we continue even if some directories exist.
					}
				}
			}
		}
	}
	return nil
}

// MimeType of an Object if known, "" otherwise
func (o *Object) MimeType() string {
	err := o.readMetaData()
	if err != nil {
		fs.Log(o, "Failed to read metadata: %v", err)
		return ""
	}
	return o.mimeType
}

// Check the interfaces are satisfied
var (
	_ fs.Fs     = (*Fs)(nil)
	_ fs.Purger = (*Fs)(nil)
	//_ fs.Copier = (*Fs)(nil)
	_ fs.Object    = (*Object)(nil)
	_ fs.MimeTyper = &Object{}
)
