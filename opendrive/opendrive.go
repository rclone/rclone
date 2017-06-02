package opendrive

import (
	"bytes"
	"io"
	"mime/multipart"
	"net/http"
	"strconv"
	"strings"
	"time"

	"fmt"

	"github.com/ncw/rclone/dircache"
	"github.com/ncw/rclone/fs"
	"github.com/ncw/rclone/pacer"
	"github.com/ncw/rclone/rest"
	"github.com/pkg/errors"
)

const (
	defaultEndpoint = "https://dev.opendrive.com/api/v1"
	minSleep        = 10 * time.Millisecond
	maxSleep        = 5 * time.Minute
	decayConstant   = 1 // bigger for slower decay, exponential
	maxParts        = 10000
	maxVersions     = 100 // maximum number of versions we search in --b2-versions mode
)

// Register with Fs
func init() {
	fs.Register(&fs.RegInfo{
		Name:        "opendrive",
		Description: "OpenDRIVE",
		NewFs:       NewFs,
		Options: []fs.Option{{
			Name: "username",
			Help: "Username",
		}, {
			Name:       "password",
			Help:       "Password.",
			IsPassword: true,
		}},
	})
}

// Fs represents a remote b2 server
type Fs struct {
	name     string             // name of this remote
	features *fs.Features       // optional features
	username string             // account name
	password string             // auth key0
	srv      *rest.Client       // the connection to the b2 server
	pacer    *pacer.Pacer       // To pace and retry the API calls
	session  UserSessionInfo    // contains the session data
	dirCache *dircache.DirCache // Map of directory path to directory id
}

// Object describes a b2 object
type Object struct {
	fs      *Fs       // what this object is part of
	remote  string    // The remote path
	id      string    // b2 id of the file
	modTime time.Time // The modified time of the object if known
	md5     string    // MD5 hash if known
	size    int64     // Size of the object
}

// parsePath parses an acd 'url'
func parsePath(path string) (root string) {
	root = strings.Trim(path, "/")
	return
}

// ------------------------------------------------------------

// Name of the remote (as passed into NewFs)
func (f *Fs) Name() string {
	return f.name
}

// Root of the remote (as passed into NewFs)
func (f *Fs) Root() string {
	return "/"
}

// String converts this Fs to a string
func (f *Fs) String() string {
	return "OpenDRIVE"
}

// Features returns the optional features of this Fs
func (f *Fs) Features() *fs.Features {
	return f.features
}

// Hashes returns the supported hash sets.
func (f *Fs) Hashes() fs.HashSet {
	return fs.HashSet(fs.HashMD5)
}

// List walks the path returning iles and directories into out
func (f *Fs) List(out fs.ListOpts, dir string) {
	f.dirCache.List(f, out, dir)
}

// NewFs contstructs an Fs from the path, bucket:path
func NewFs(name, root string) (fs.Fs, error) {
	root = parsePath(root)
	fs.Debugf(nil, "NewFS(\"%s\", \"%s\"", name, root)
	username := fs.ConfigFileGet(name, "username")
	if username == "" {
		return nil, errors.New("username not found")
	}
	password, err := fs.Reveal(fs.ConfigFileGet(name, "password"))
	if err != nil {
		return nil, errors.New("password coudl not revealed")
	}
	if password == "" {
		return nil, errors.New("password not found")
	}

	fs.Debugf(nil, "OpenDRIVE-user: %s", username)
	fs.Debugf(nil, "OpenDRIVE-pass: %s", password)

	f := &Fs{
		name:     name,
		username: username,
		password: password,
		srv:      rest.NewClient(fs.Config.Client()).SetErrorHandler(errorHandler),
		pacer:    pacer.New().SetMinSleep(minSleep).SetMaxSleep(maxSleep).SetDecayConstant(decayConstant),
	}

	f.dirCache = dircache.New(root, "0", f)

	// set the rootURL for the REST client
	f.srv.SetRoot(defaultEndpoint)

	// get sessionID
	var resp *http.Response
	err = f.pacer.Call(func() (bool, error) {
		account := Account{Username: username, Password: password}

		opts := rest.Opts{
			Method: "POST",
			Path:   "/session/login.json",
		}
		resp, err = f.srv.CallJSON(&opts, &account, &f.session)
		return f.shouldRetry(resp, err)
	})
	if err != nil {
		return nil, errors.Wrap(err, "failed to create session")
	}

	fs.Debugf(nil, "Starting OpenDRIVE session with ID: %s", f.session.SessionID)

	f.features = (&fs.Features{ReadMimeType: true, WriteMimeType: true}).Fill(f)

	// Find the current root
	err = f.dirCache.FindRoot(false)
	if err != nil {
		// Assume it is a file
		newRoot, remote := dircache.SplitPath(root)
		newF := *f
		newF.dirCache = dircache.New(newRoot, "0", &newF)

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

// errorHandler parses a non 2xx error response into an error
func errorHandler(resp *http.Response) error {
	// Decode error response
	// errResponse := new(api.Error)
	// err := rest.DecodeJSON(resp, &errResponse)
	// if err != nil {
	// 	fs.Debugf(nil, "Couldn't decode error response: %v", err)
	// }
	// if errResponse.Code == "" {
	// 	errResponse.Code = "unknown"
	// }
	// if errResponse.Status == 0 {
	// 	errResponse.Status = resp.StatusCode
	// }
	// if errResponse.Message == "" {
	// 	errResponse.Message = "Unknown " + resp.Status
	// }
	// return errResponse
	return nil
}

// Mkdir creates the bucket if it doesn't exist
func (f *Fs) Mkdir(dir string) error {
	fs.Debugf(nil, "Mkdir(\"%s\")", dir)
	err := f.dirCache.FindRoot(true)
	if err != nil {
		return err
	}
	if dir != "" {
		_, err = f.dirCache.FindDir(dir, true)
	}
	return err
}

// Rmdir deletes the bucket if the fs is at the root
//
// Returns an error if it isn't empty
func (f *Fs) Rmdir(dir string) error {
	fs.Debugf(nil, "Rmdir(\"%s\")", dir)
	// if f.root != "" || dir != "" {
	// 	return nil
	// }
	// opts := rest.Opts{
	// 	Method: "POST",
	// 	Path:   "/b2_delete_bucket",
	// }
	// bucketID, err := f.getBucketID()
	// if err != nil {
	// 	return err
	// }
	// var request = api.DeleteBucketRequest{
	// 	ID:        bucketID,
	// 	AccountID: f.info.AccountID,
	// }
	// var response api.Bucket
	// err = f.pacer.Call(func() (bool, error) {
	// 	resp, err := f.srv.CallJSON(&opts, &request, &response)
	// 	return f.shouldRetry(resp, err)
	// })
	// if err != nil {
	// 	return errors.Wrap(err, "failed to delete bucket")
	// }
	// f.clearBucketID()
	// f.clearUploadURL()
	return nil
}

// Precision of the remote
func (f *Fs) Precision() time.Duration {
	return time.Millisecond
}

// Return an Object from a path
//
// If it can't be found it returns the error fs.ErrorObjectNotFound.
func (f *Fs) newObjectWithInfo(remote string, file *File) (fs.Object, error) {
	fs.Debugf(nil, "newObjectWithInfo(%s, %v)", remote, file)

	var o *Object
	if nil != file {
		o = &Object{
			fs:      f,
			remote:  remote,
			id:      file.FileID,
			modTime: time.Unix(file.DateModified, 0),
			size:    file.Size,
		}
	} else {
		o = &Object{
			fs:     f,
			remote: remote,
		}

		err := o.readMetaData()
		if err != nil {
			return nil, err
		}
	}
	fs.Debugf(nil, "%v", o)
	return o, nil
}

// NewObject finds the Object at remote.  If it can't be found
// it returns the error fs.ErrorObjectNotFound.
func (f *Fs) NewObject(remote string) (fs.Object, error) {
	fs.Debugf(nil, "NewObject(\"%s\"", remote)
	return f.newObjectWithInfo(remote, nil)
}

// Creates from the parameters passed in a half finished Object which
// must have setMetaData called on it
//
// Returns the object, leaf, directoryID and error
//
// Used to create new objects
func (f *Fs) createObject(remote string, modTime time.Time, size int64) (o *Object, leaf string, directoryID string, err error) {
	// Create the directory for the object if it doesn't exist
	leaf, directoryID, err = f.dirCache.FindRootAndPath(remote, true)
	if err != nil {
		return nil, leaf, directoryID, err
	}
	// Temporary Object under construction
	o = &Object{
		fs:     f,
		remote: remote,
	}
	return o, leaf, directoryID, nil
}

// Put the object into the bucket
//
// Copy the reader in to the new object which is returned
//
// The new object may have been created if an error is returned
func (f *Fs) Put(in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error) {
	remote := src.Remote()
	size := src.Size()
	modTime := src.ModTime()

	fs.Debugf(nil, "Put(%s)", remote)

	o, _, _, err := f.createObject(remote, modTime, size)
	if err != nil {
		return nil, err
	}
	return o, o.Update(in, src, options...)
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
	// if resp != nil {
	// 	if resp.StatusCode == 401 {
	// 		f.tokenRenewer.Invalidate()
	// 		fs.Debugf(f, "401 error received - invalidating token")
	// 		return true, err
	// 	}
	// 	// Work around receiving this error sporadically on authentication
	// 	//
	// 	// HTTP code 403: "403 Forbidden", reponse body: {"message":"Authorization header requires 'Credential' parameter. Authorization header requires 'Signature' parameter. Authorization header requires 'SignedHeaders' parameter. Authorization header requires existence of either a 'X-Amz-Date' or a 'Date' header. Authorization=Bearer"}
	// 	if resp.StatusCode == 403 && strings.Contains(err.Error(), "Authorization header requires") {
	// 		fs.Debugf(f, "403 \"Authorization header requires...\" error received - retry")
	// 		return true, err
	// 	}
	// }
	return fs.ShouldRetry(err) || fs.ShouldRetryHTTP(resp, retryErrorCodes), err
}

// DirCacher methos

// CreateDir makes a directory with pathID as parent and name leaf
func (f *Fs) CreateDir(pathID, leaf string) (newID string, err error) {
	fs.Debugf(nil, "CreateDir(\"%s\", \"%s\")", pathID, leaf)
	// //fmt.Printf("CreateDir(%q, %q)\n", pathID, leaf)
	// folder := acd.FolderFromId(pathID, f.c.Nodes)
	// var resp *http.Response
	// var info *acd.Folder
	// err = f.pacer.Call(func() (bool, error) {
	// 	info, resp, err = folder.CreateFolder(leaf)
	// 	return f.shouldRetry(resp, err)
	// })
	// if err != nil {
	// 	//fmt.Printf("...Error %v\n", err)
	// 	return "", err
	// }
	// //fmt.Printf("...Id %q\n", *info.Id)
	// return *info.Id, nil
	return "", fmt.Errorf("CreateDir not implemented")
}

// FindLeaf finds a directory of name leaf in the folder with ID pathID
func (f *Fs) FindLeaf(pathID, leaf string) (pathIDOut string, found bool, err error) {
	fs.Debugf(nil, "FindLeaf(\"%s\", \"%s\")", pathID, leaf)

	if pathID == "0" && leaf == "" {
		fs.Debugf(nil, "Found OpenDRIVE root")
		// that's the root directory
		return pathID, true, nil
	}

	// get the folderIDs
	var resp *http.Response
	folderList := FolderList{}
	err = f.pacer.Call(func() (bool, error) {
		opts := rest.Opts{
			Method: "GET",
			Path:   "/folder/list.json/" + f.session.SessionID + "/" + pathID,
		}
		resp, err = f.srv.CallJSON(&opts, nil, &folderList)
		return f.shouldRetry(resp, err)
	})
	if err != nil {
		return "", false, errors.Wrap(err, "failed to get folder list")
	}

	for _, folder := range folderList.Folders {
		fs.Debugf(nil, "Folder: %s (%s)", folder.Name, folder.FolderID)

		if leaf == folder.Name {
			// found
			return folder.FolderID, true, nil
		}
	}

	return "", false, nil
}

// ListDir reads the directory specified by the job into out, returning any more jobs
func (f *Fs) ListDir(out fs.ListOpts, job dircache.ListDirJob) (jobs []dircache.ListDirJob, err error) {
	fs.Debugf(nil, "ListDir(%v, %v)", out, job)
	// get the folderIDs
	var resp *http.Response
	folderList := FolderList{}
	err = f.pacer.Call(func() (bool, error) {
		opts := rest.Opts{
			Method: "GET",
			Path:   "/folder/list.json/" + f.session.SessionID + "/" + job.DirID,
		}
		resp, err = f.srv.CallJSON(&opts, nil, &folderList)
		return f.shouldRetry(resp, err)
	})
	if err != nil {
		return nil, errors.Wrap(err, "failed to get folder list")
	}

	for _, folder := range folderList.Folders {
		fs.Debugf(nil, "Folder: %s (%s)", folder.Name, folder.FolderID)
		remote := job.Path + folder.Name
		if out.IncludeDirectory(remote) {
			dir := &fs.Dir{
				Name:  remote,
				Bytes: -1,
				Count: -1,
			}
			dir.When = time.Unix(int64(folder.DateModified), 0)
			if out.AddDir(dir) {
				continue
			}
			if job.Depth > 0 {
				jobs = append(jobs, dircache.ListDirJob{DirID: folder.FolderID, Path: remote + "/", Depth: job.Depth - 1})
			}
		}
	}

	for _, file := range folderList.Files {
		fs.Debugf(nil, "File: %s (%s)", file.Name, file.FileID)
		remote := job.Path + file.Name
		o, err := f.newObjectWithInfo(remote, &file)
		if err != nil {
			out.SetError(err)
			continue
		}
		out.Add(o)
	}

	return jobs, nil
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
	return o.md5, nil
}

// Size returns the size of an object in bytes
func (o *Object) Size() int64 {
	return o.size // Object is likely PENDING
}

// ModTime returns the modification time of the object
//
//
// It attempts to read the objects mtime and if that isn't present the
// LastModified returned in the http headers
func (o *Object) ModTime() time.Time {
	return o.modTime
}

// SetModTime sets the modification time of the local fs object
func (o *Object) SetModTime(modTime time.Time) error {
	// FIXME not implemented
	return fs.ErrorCantSetModTime
}

// Open an object for read
func (o *Object) Open(options ...fs.OpenOption) (in io.ReadCloser, err error) {
	fs.Debugf(nil, "Open(\"%v\")", o.remote)

	// get the folderIDs
	var resp *http.Response
	err = o.fs.pacer.Call(func() (bool, error) {
		opts := rest.Opts{
			Method: "GET",
			Path:   "/download/file.json/" + o.id + "?session_id=" + o.fs.session.SessionID,
		}
		resp, err = o.fs.srv.Call(&opts)
		return o.fs.shouldRetry(resp, err)
	})
	if err != nil {
		return nil, errors.Wrap(err, "failed to open file)")
	}

	return resp.Body, nil
}

// Remove an object
func (o *Object) Remove() error {
	fs.Debugf(nil, "Remove(\"%s\")", o.id)
	return fmt.Errorf("Remove not implemented")
}

// Storable returns a boolean showing whether this object storable
func (o *Object) Storable() bool {
	return true
}

// Update the object with the contents of the io.Reader, modTime and size
//
// The new object may have been created if an error is returned
func (o *Object) Update(in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) error {
	size := src.Size()
	modTime := src.ModTime()
	fs.Debugf(nil, "%d %d", size, modTime)
	fs.Debugf(nil, "Update(\"%s\", \"%s\")", o.id, o.remote)

	var err error
	if "" == o.id {
		// We need to create a ID for this file
		var resp *http.Response
		response := createFileResponse{}
		err = o.fs.pacer.Call(func() (bool, error) {
			createFileData := createFile{SessionID: o.fs.session.SessionID, FolderID: "0", Name: o.remote}
			opts := rest.Opts{
				Method: "POST",
				Path:   "/upload/create_file.json",
			}
			resp, err = o.fs.srv.CallJSON(&opts, &createFileData, &response)
			return o.fs.shouldRetry(resp, err)
		})
		if err != nil {
			return errors.Wrap(err, "failed to create file")
		}

		o.id = response.FileID
	}
	fmt.Println(o.id)

	// Open file for upload
	var resp *http.Response
	openResponse := openUploadResponse{}
	err = o.fs.pacer.Call(func() (bool, error) {
		openUploadData := openUpload{SessionID: o.fs.session.SessionID, FileID: o.id, Size: size}
		fs.Debugf(nil, "PreOpen: %s", openUploadData)
		opts := rest.Opts{
			Method: "POST",
			Path:   "/upload/open_file_upload.json",
		}
		resp, err = o.fs.srv.CallJSON(&opts, &openUploadData, &openResponse)
		return o.fs.shouldRetry(resp, err)
	})
	if err != nil {
		return errors.Wrap(err, "failed to create file")
	}
	fs.Debugf(nil, "PostOpen: %s", openResponse)

	// 1 MB chunks size
	chunkSize := int64(1024 * 1024 * 10)
	chunkOffset := int64(0)
	remainingBytes := size
	chunkCounter := 0

	for remainingBytes > 0 {
		currentChunkSize := chunkSize
		if currentChunkSize > remainingBytes {
			currentChunkSize = remainingBytes
		}
		remainingBytes -= currentChunkSize
		fs.Debugf(nil, "Chunk %d: size=%d, remain=%d", chunkCounter, currentChunkSize, remainingBytes)

		err = o.fs.pacer.Call(func() (bool, error) {
			var formBody bytes.Buffer
			w := multipart.NewWriter(&formBody)
			fw, err := w.CreateFormFile("file_data", o.remote)
			if err != nil {
				return false, err
			}
			if _, err = io.CopyN(fw, in, currentChunkSize); err != nil {
				return false, err
			}
			// Add session_id
			if fw, err = w.CreateFormField("session_id"); err != nil {
				return false, err
			}
			if _, err = fw.Write([]byte(o.fs.session.SessionID)); err != nil {
				return false, err
			}
			// Add session_id
			if fw, err = w.CreateFormField("session_id"); err != nil {
				return false, err
			}
			if _, err = fw.Write([]byte(o.fs.session.SessionID)); err != nil {
				return false, err
			}
			// Add file_id
			if fw, err = w.CreateFormField("file_id"); err != nil {
				return false, err
			}
			if _, err = fw.Write([]byte(o.id)); err != nil {
				return false, err
			}
			// Add temp_location
			if fw, err = w.CreateFormField("temp_location"); err != nil {
				return false, err
			}
			if _, err = fw.Write([]byte(openResponse.TempLocation)); err != nil {
				return false, err
			}
			// Add chunk_offset
			if fw, err = w.CreateFormField("chunk_offset"); err != nil {
				return false, err
			}
			if _, err = fw.Write([]byte(strconv.FormatInt(chunkOffset, 10))); err != nil {
				return false, err
			}
			// Add chunk_size
			if fw, err = w.CreateFormField("chunk_size"); err != nil {
				return false, err
			}
			if _, err = fw.Write([]byte(strconv.FormatInt(currentChunkSize, 10))); err != nil {
				return false, err
			}
			// Don't forget to close the multipart writer.
			// If you don't close it, your request will be missing the terminating boundary.
			w.Close()

			opts := rest.Opts{
				Method:       "POST",
				Path:         "/upload/upload_file_chunk.json",
				Body:         &formBody,
				ExtraHeaders: map[string]string{"Content-Type": w.FormDataContentType()},
			}
			resp, err = o.fs.srv.Call(&opts)
			return o.fs.shouldRetry(resp, err)
		})
		if err != nil {
			return errors.Wrap(err, "failed to create file")
		}

		fmt.Println(resp.Body)
		resp.Body.Close()

		chunkCounter++
		chunkOffset += currentChunkSize
	}

	// CLose file for upload
	closeResponse := closeUploadResponse{}
	err = o.fs.pacer.Call(func() (bool, error) {
		closeUploadData := closeUpload{SessionID: o.fs.session.SessionID, FileID: o.id, Size: size, TempLocation: openResponse.TempLocation}
		fs.Debugf(nil, "PreClose: %s", closeUploadData)
		opts := rest.Opts{
			Method: "POST",
			Path:   "/upload/close_file_upload.json",
		}
		resp, err = o.fs.srv.CallJSON(&opts, &closeUploadData, &closeResponse)
		return o.fs.shouldRetry(resp, err)
	})
	if err != nil {
		return errors.Wrap(err, "failed to create file")
	}
	fs.Debugf(nil, "PostClose: %s", closeResponse)

	// file := acd.File{Node: o.info}
	// var info *acd.File
	// var resp *http.Response
	// var err error
	// err = o.fs.pacer.CallNoRetry(func() (bool, error) {
	// 	start := time.Now()
	// 	o.fs.tokenRenewer.Start()
	// 	info, resp, err = file.Overwrite(in)
	// 	o.fs.tokenRenewer.Stop()
	// 	var ok bool
	// 	ok, info, err = o.fs.checkUpload(resp, in, src, info, err, time.Since(start))
	// 	if ok {
	// 		return false, nil
	// 	}
	// 	return o.fs.shouldRetry(resp, err)
	// })
	// if err != nil {
	// 	return err
	// }
	// o.info = info.Node
	// return nil

	return nil
}

func (o *Object) readMetaData() (err error) {
	leaf, directoryID, err := o.fs.dirCache.FindRootAndPath(o.remote, false)
	if err != nil {
		if err == fs.ErrorDirNotFound {
			return fs.ErrorObjectNotFound
		}
		return err
	}
	var resp *http.Response
	folderList := FolderList{}
	err = o.fs.pacer.Call(func() (bool, error) {
		opts := rest.Opts{
			Method: "GET",
			Path:   "/folder/itembyname.json/" + o.fs.session.SessionID + "/" + directoryID + "?name=" + leaf,
		}
		resp, err = o.fs.srv.CallJSON(&opts, nil, &folderList)
		return o.fs.shouldRetry(resp, err)
	})
	if err != nil {
		return errors.Wrap(err, "failed to get folder list")
	}

	if len(folderList.Files) == 0 {
		return fs.ErrorObjectNotFound
	}

	leafFile := folderList.Files[0]
	o.id = leafFile.FileID
	o.modTime = time.Unix(leafFile.DateModified, 0)
	o.md5 = ""
	o.size = leafFile.Size

	return nil
}
