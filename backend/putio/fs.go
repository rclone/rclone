package putio

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/pkg/errors"
	"github.com/putdotio/go-putio/putio"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config/configmap"
	"github.com/rclone/rclone/fs/fserrors"
	"github.com/rclone/rclone/fs/hash"
	"github.com/rclone/rclone/lib/dircache"
	"github.com/rclone/rclone/lib/oauthutil"
	"github.com/rclone/rclone/lib/pacer"
	"github.com/rclone/rclone/lib/readers"
)

// Fs represents a remote Putio server
type Fs struct {
	name        string             // name of this remote
	root        string             // the path we are working on
	features    *fs.Features       // optional features
	client      *putio.Client      // client for making API calls to Put.io
	pacer       *fs.Pacer          // To pace the API calls
	dirCache    *dircache.DirCache // Map of directory path to directory id
	oAuthClient *http.Client
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
	return fmt.Sprintf("Putio root '%s'", f.root)
}

// Features returns the optional features of this Fs
func (f *Fs) Features() *fs.Features {
	return f.features
}

// shouldRetry returns a boolean as to whether this err deserves to be
// retried.  It returns the err as a convenience
func shouldRetry(err error) (bool, error) {
	if err == nil {
		return false, nil
	}
	if fserrors.ShouldRetry(err) {
		return true, err
	}
	if perr, ok := err.(*putio.ErrorResponse); ok {
		if perr.Response.StatusCode == 429 || perr.Response.StatusCode >= 500 {
			return true, err
		}
	}
	return false, err
}

// NewFs constructs an Fs from the path, container:path
func NewFs(name, root string, m configmap.Mapper) (f fs.Fs, err error) {
	// defer log.Trace(name, "root=%v", root)("f=%+v, err=%v", &f, &err)
	oAuthClient, _, err := oauthutil.NewClient(name, m, putioConfig)
	if err != nil {
		return nil, errors.Wrap(err, "failed to configure putio")
	}
	p := &Fs{
		name:        name,
		root:        root,
		pacer:       fs.NewPacer(pacer.NewDefault(pacer.MinSleep(minSleep), pacer.MaxSleep(maxSleep), pacer.DecayConstant(decayConstant))),
		client:      putio.NewClient(oAuthClient),
		oAuthClient: oAuthClient,
	}
	p.features = (&fs.Features{
		DuplicateFiles:          true,
		ReadMimeType:            true,
		CanHaveEmptyDirectories: true,
	}).Fill(p)
	p.dirCache = dircache.New(root, "0", p)
	ctx := context.Background()
	// Find the current root
	err = p.dirCache.FindRoot(ctx, false)
	if err != nil {
		// Assume it is a file
		newRoot, remote := dircache.SplitPath(root)
		tempF := *p
		tempF.dirCache = dircache.New(newRoot, "0", &tempF)
		tempF.root = newRoot
		// Make new Fs which is the parent
		err = tempF.dirCache.FindRoot(ctx, false)
		if err != nil {
			// No root so return old f
			return p, nil
		}
		_, err := tempF.NewObject(ctx, remote)
		if err != nil {
			// unable to list folder so return old f
			return p, nil
		}
		// XXX: update the old f here instead of returning tempF, since
		// `features` were already filled with functions having *f as a receiver.
		// See https://github.com/rclone/rclone/issues/2182
		p.dirCache = tempF.dirCache
		p.root = tempF.root
		return p, fs.ErrorIsFile
	}
	// fs.Debugf(p, "Root id: %s", p.dirCache.RootID())
	return p, nil
}

func itoa(i int64) string {
	return strconv.FormatInt(i, 10)
}

func atoi(a string) int64 {
	i, err := strconv.ParseInt(a, 10, 64)
	if err != nil {
		panic(err)
	}
	return i
}

// CreateDir makes a directory with pathID as parent and name leaf
func (f *Fs) CreateDir(ctx context.Context, pathID, leaf string) (newID string, err error) {
	// defer log.Trace(f, "pathID=%v, leaf=%v", pathID, leaf)("newID=%v, err=%v", newID, &err)
	parentID := atoi(pathID)
	var entry putio.File
	err = f.pacer.Call(func() (bool, error) {
		// fs.Debugf(f, "creating folder. part: %s, parentID: %d", leaf, parentID)
		entry, err = f.client.Files.CreateFolder(ctx, leaf, parentID)
		return shouldRetry(err)
	})
	return itoa(entry.ID), err
}

// FindLeaf finds a directory of name leaf in the folder with ID pathID
func (f *Fs) FindLeaf(ctx context.Context, pathID, leaf string) (pathIDOut string, found bool, err error) {
	// defer log.Trace(f, "pathID=%v, leaf=%v", pathID, leaf)("pathIDOut=%v, found=%v, err=%v", pathIDOut, found, &err)
	if pathID == "0" && leaf == "" {
		// that's the root directory
		return pathID, true, nil
	}
	fileID := atoi(pathID)
	var children []putio.File
	err = f.pacer.Call(func() (bool, error) {
		// fs.Debugf(f, "listing file: %d", fileID)
		children, _, err = f.client.Files.List(ctx, fileID)
		return shouldRetry(err)
	})
	if err != nil {
		if perr, ok := err.(*putio.ErrorResponse); ok && perr.Response.StatusCode == 404 {
			err = nil
		}
		return
	}
	for _, child := range children {
		if child.Name == leaf {
			found = true
			pathIDOut = itoa(child.ID)
			if !child.IsDir() {
				err = fs.ErrorNotAFile
			}
			return
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
	// defer log.Trace(f, "dir=%v", dir)("err=%v", &err)
	err = f.dirCache.FindRoot(ctx, false)
	if err != nil {
		return nil, err
	}
	directoryID, err := f.dirCache.FindDir(ctx, dir, false)
	if err != nil {
		return nil, err
	}
	parentID := atoi(directoryID)
	var children []putio.File
	err = f.pacer.Call(func() (bool, error) {
		// fs.Debugf(f, "listing files inside List: %d", parentID)
		children, _, err = f.client.Files.List(ctx, parentID)
		return shouldRetry(err)
	})
	if err != nil {
		return
	}
	for _, child := range children {
		remote := path.Join(dir, child.Name)
		// fs.Debugf(f, "child: %s", remote)
		if child.IsDir() {
			f.dirCache.Put(remote, itoa(child.ID))
			d := fs.NewDir(remote, child.UpdatedAt.Time)
			entries = append(entries, d)
		} else {
			o, err := f.newObjectWithInfo(ctx, remote, child)
			if err != nil {
				return nil, err
			}
			entries = append(entries, o)
		}
	}
	return
}

// Put the object
//
// Copy the reader in to the new object which is returned
//
// The new object may have been created if an error is returned
func (f *Fs) Put(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (o fs.Object, err error) {
	// defer log.Trace(f, "src=%+v", src)("o=%+v, err=%v", &o, &err)
	exisitingObj, err := f.NewObject(ctx, src.Remote())
	switch err {
	case nil:
		return exisitingObj, exisitingObj.Update(ctx, in, src, options...)
	case fs.ErrorObjectNotFound:
		// Not found so create it
		return f.PutUnchecked(ctx, in, src, options...)
	default:
		return nil, err
	}
}

// PutUnchecked uploads the object
//
// This will create a duplicate if we upload a new file without
// checking to see if there is one already - use Put() for that.
func (f *Fs) PutUnchecked(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (o fs.Object, err error) {
	// defer log.Trace(f, "src=%+v", src)("o=%+v, err=%v", &o, &err)
	size := src.Size()
	remote := src.Remote()
	leaf, directoryID, err := f.dirCache.FindRootAndPath(ctx, remote, true)
	if err != nil {
		return nil, err
	}
	loc, err := f.createUpload(ctx, leaf, size, directoryID, src.ModTime(ctx))
	if err != nil {
		return nil, err
	}
	fileID, err := f.sendUpload(ctx, loc, size, in)
	if err != nil {
		return nil, err
	}
	var entry putio.File
	err = f.pacer.Call(func() (bool, error) {
		// fs.Debugf(f, "getting file: %d", fileID)
		entry, err = f.client.Files.Get(ctx, fileID)
		return shouldRetry(err)
	})
	if err != nil {
		return nil, err
	}
	return f.newObjectWithInfo(ctx, remote, entry)
}

func (f *Fs) createUpload(ctx context.Context, name string, size int64, parentID string, modTime time.Time) (location string, err error) {
	// defer log.Trace(f, "name=%v, size=%v, parentID=%v, modTime=%v", name, size, parentID, modTime.String())("location=%v, err=%v", location, &err)
	err = f.pacer.Call(func() (bool, error) {
		req, err := http.NewRequest("POST", "https://upload.put.io/files/", nil)
		if err != nil {
			return false, err
		}
		req.Header.Set("tus-resumable", "1.0.0")
		req.Header.Set("upload-length", strconv.FormatInt(size, 10))
		b64name := base64.StdEncoding.EncodeToString([]byte(name))
		b64true := base64.StdEncoding.EncodeToString([]byte("true"))
		b64parentID := base64.StdEncoding.EncodeToString([]byte(parentID))
		b64modifiedAt := base64.StdEncoding.EncodeToString([]byte(modTime.Format(time.RFC3339)))
		req.Header.Set("upload-metadata", fmt.Sprintf("name %s,no-torrent %s,parent_id %s,updated-at %s", b64name, b64true, b64parentID, b64modifiedAt))
		resp, err := f.oAuthClient.Do(req)
		retry, err := shouldRetry(err)
		if retry {
			return true, err
		}
		if err != nil {
			return false, err
		}
		if resp.StatusCode != 201 {
			return false, fmt.Errorf("unexpected status code from upload create: %d", resp.StatusCode)
		}
		location = resp.Header.Get("location")
		if location == "" {
			return false, errors.New("empty location header from upload create")
		}
		return false, nil
	})
	return
}

func (f *Fs) sendUpload(ctx context.Context, location string, size int64, in io.Reader) (fileID int64, err error) {
	// defer log.Trace(f, "location=%v, size=%v", location, size)("fileID=%v, err=%v", fileID, &err)
	if size == 0 {
		err = f.pacer.Call(func() (bool, error) {
			fs.Debugf(f, "Sending zero length chunk")
			fileID, err = f.transferChunk(ctx, location, 0, bytes.NewReader([]byte{}), 0)
			return shouldRetry(err)
		})
		return
	}
	var start int64
	buf := make([]byte, defaultChunkSize)
	for start < size {
		reqSize := size - start
		if reqSize >= int64(defaultChunkSize) {
			reqSize = int64(defaultChunkSize)
		}
		chunk := readers.NewRepeatableLimitReaderBuffer(in, buf, reqSize)

		// Transfer the chunk
		err = f.pacer.Call(func() (bool, error) {
			fs.Debugf(f, "Sending chunk. start: %d length: %d", start, reqSize)
			// TODO get file offset and seek to the position
			fileID, err = f.transferChunk(ctx, location, start, chunk, reqSize)
			return shouldRetry(err)
		})
		if err != nil {
			return
		}

		start += reqSize
	}
	return
}

func (f *Fs) transferChunk(ctx context.Context, location string, start int64, chunk io.ReadSeeker, chunkSize int64) (fileID int64, err error) {
	// defer log.Trace(f, "location=%v, start=%v, chunkSize=%v", location, start, chunkSize)("fileID=%v, err=%v", fileID, &err)
	_, _ = chunk.Seek(0, io.SeekStart)
	req, err := f.makeUploadPatchRequest(location, chunk, start, chunkSize)
	if err != nil {
		return 0, err
	}
	req = req.WithContext(ctx)
	res, err := f.oAuthClient.Do(req)
	if err != nil {
		return 0, err
	}
	defer func() {
		_ = res.Body.Close()
	}()
	if res.StatusCode != 204 {
		return 0, fmt.Errorf("unexpected status code while transferring chunk: %d", res.StatusCode)
	}
	sfid := res.Header.Get("putio-file-id")
	if sfid != "" {
		fileID, err = strconv.ParseInt(sfid, 10, 64)
		if err != nil {
			return 0, err
		}
	}
	return fileID, nil
}

func (f *Fs) makeUploadPatchRequest(location string, in io.Reader, offset, length int64) (*http.Request, error) {
	req, err := http.NewRequest("PATCH", location, in)
	if err != nil {
		return nil, err
	}
	req.Header.Set("tus-resumable", "1.0.0")
	req.Header.Set("upload-offset", strconv.FormatInt(offset, 10))
	req.Header.Set("content-length", strconv.FormatInt(length, 10))
	req.Header.Set("content-type", "application/offset+octet-stream")
	return req, nil
}

// Mkdir creates the container if it doesn't exist
func (f *Fs) Mkdir(ctx context.Context, dir string) (err error) {
	// defer log.Trace(f, "dir=%v", dir)("err=%v", &err)
	err = f.dirCache.FindRoot(ctx, true)
	if err != nil {
		return err
	}
	if dir != "" {
		_, err = f.dirCache.FindDir(ctx, dir, true)
	}
	return err
}

// Rmdir deletes the container
//
// Returns an error if it isn't empty
func (f *Fs) Rmdir(ctx context.Context, dir string) (err error) {
	// defer log.Trace(f, "dir=%v", dir)("err=%v", &err)

	root := strings.Trim(path.Join(f.root, dir), "/")

	// can't remove root
	if root == "" {
		return errors.New("can't remove root directory")
	}

	// check directory exists
	directoryID, err := f.dirCache.FindDir(ctx, dir, false)
	if err != nil {
		return errors.Wrap(err, "Rmdir")
	}
	dirID := atoi(directoryID)

	// check directory empty
	var children []putio.File
	err = f.pacer.Call(func() (bool, error) {
		// fs.Debugf(f, "listing files: %d", dirID)
		children, _, err = f.client.Files.List(ctx, dirID)
		return shouldRetry(err)
	})
	if err != nil {
		return errors.Wrap(err, "Rmdir")
	}
	if len(children) != 0 {
		return errors.New("directory not empty")
	}

	// remove it
	err = f.pacer.Call(func() (bool, error) {
		// fs.Debugf(f, "deleting file: %d", dirID)
		err = f.client.Files.Delete(ctx, dirID)
		return shouldRetry(err)
	})
	f.dirCache.FlushDir(dir)
	return err
}

// Precision returns the precision
func (f *Fs) Precision() time.Duration {
	return time.Second
}

// Purge deletes all the files and the container
//
// Optional interface: Only implement this if you have a way of
// deleting all the files quicker than just running Remove() on the
// result of List()
func (f *Fs) Purge(ctx context.Context) (err error) {
	// defer log.Trace(f, "")("err=%v", &err)

	if f.root == "" {
		return errors.New("can't purge root directory")
	}
	err = f.dirCache.FindRoot(ctx, false)
	if err != nil {
		return err
	}

	rootID := atoi(f.dirCache.RootID())
	// Let putio delete the filesystem tree
	err = f.pacer.Call(func() (bool, error) {
		// fs.Debugf(f, "deleting file: %d", rootID)
		err = f.client.Files.Delete(ctx, rootID)
		return shouldRetry(err)
	})
	f.dirCache.ResetRoot()
	return err
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
func (f *Fs) Copy(ctx context.Context, src fs.Object, remote string) (o fs.Object, err error) {
	// defer log.Trace(f, "src=%+v, remote=%v", src, remote)("o=%+v, err=%v", &o, &err)
	srcObj, ok := src.(*Object)
	if !ok {
		return nil, fs.ErrorCantCopy
	}
	leaf, directoryID, err := f.dirCache.FindRootAndPath(ctx, remote, true)
	if err != nil {
		return nil, err
	}
	err = f.pacer.Call(func() (bool, error) {
		params := url.Values{}
		params.Set("file_id", strconv.FormatInt(srcObj.file.ID, 10))
		params.Set("parent_id", directoryID)
		params.Set("name", leaf)
		req, err := f.client.NewRequest(ctx, "POST", "/v2/files/copy", strings.NewReader(params.Encode()))
		if err != nil {
			return false, err
		}
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		// fs.Debugf(f, "copying file (%d) to parent_id: %s", srcObj.file.ID, directoryID)
		_, err = f.client.Do(req, nil)
		return shouldRetry(err)
	})
	if err != nil {
		return nil, err
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
func (f *Fs) Move(ctx context.Context, src fs.Object, remote string) (o fs.Object, err error) {
	// defer log.Trace(f, "src=%+v, remote=%v", src, remote)("o=%+v, err=%v", &o, &err)
	srcObj, ok := src.(*Object)
	if !ok {
		return nil, fs.ErrorCantMove
	}
	leaf, directoryID, err := f.dirCache.FindRootAndPath(ctx, remote, true)
	if err != nil {
		return nil, err
	}
	err = f.pacer.Call(func() (bool, error) {
		params := url.Values{}
		params.Set("file_id", strconv.FormatInt(srcObj.file.ID, 10))
		params.Set("parent_id", directoryID)
		params.Set("name", leaf)
		req, err := f.client.NewRequest(ctx, "POST", "/v2/files/move", strings.NewReader(params.Encode()))
		if err != nil {
			return false, err
		}
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		// fs.Debugf(f, "moving file (%d) to parent_id: %s", srcObj.file.ID, directoryID)
		_, err = f.client.Do(req, nil)
		return shouldRetry(err)
	})
	if err != nil {
		return nil, err
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
func (f *Fs) DirMove(ctx context.Context, src fs.Fs, srcRemote, dstRemote string) (err error) {
	// defer log.Trace(f, "src=%+v, srcRemote=%v, dstRemote", src, srcRemote, dstRemote)("err=%v", &err)
	srcFs, ok := src.(*Fs)
	if !ok {
		return fs.ErrorCantDirMove
	}
	srcPath := path.Join(srcFs.root, srcRemote)
	dstPath := path.Join(f.root, dstRemote)

	// Refuse to move to or from the root
	if srcPath == "" || dstPath == "" {
		return errors.New("can't move root directory")
	}

	// find the root src directory
	err = srcFs.dirCache.FindRoot(ctx, false)
	if err != nil {
		return err
	}

	// find the root dst directory
	if dstRemote != "" {
		err = f.dirCache.FindRoot(ctx, true)
		if err != nil {
			return err
		}
	} else {
		if f.dirCache.FoundRoot() {
			return fs.ErrorDirExists
		}
	}

	// Find ID of dst parent, creating subdirs if necessary
	var leaf, dstDirectoryID string
	findPath := dstRemote
	if dstRemote == "" {
		findPath = f.root
	}
	leaf, dstDirectoryID, err = f.dirCache.FindPath(ctx, findPath, true)
	if err != nil {
		return err
	}

	// Check destination does not exist
	if dstRemote != "" {
		_, err = f.dirCache.FindDir(ctx, dstRemote, false)
		if err == fs.ErrorDirNotFound {
			// OK
		} else if err != nil {
			return err
		} else {
			return fs.ErrorDirExists
		}
	}

	// Find ID of src
	srcID, err := srcFs.dirCache.FindDir(ctx, srcRemote, false)
	if err != nil {
		return err
	}

	err = f.pacer.Call(func() (bool, error) {
		params := url.Values{}
		params.Set("file_id", srcID)
		params.Set("parent_id", dstDirectoryID)
		params.Set("name", leaf)
		req, err := f.client.NewRequest(ctx, "POST", "/v2/files/move", strings.NewReader(params.Encode()))
		if err != nil {
			return false, err
		}
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		// fs.Debugf(f, "moving file (%s) to parent_id: %s", srcID, dstDirectoryID)
		_, err = f.client.Do(req, nil)
		return shouldRetry(err)
	})
	srcFs.dirCache.FlushDir(srcRemote)
	return err
}

// About gets quota information
func (f *Fs) About(ctx context.Context) (usage *fs.Usage, err error) {
	// defer log.Trace(f, "")("usage=%+v, err=%v", usage, &err)
	var ai putio.AccountInfo
	err = f.pacer.Call(func() (bool, error) {
		// fs.Debugf(f, "getting account info")
		ai, err = f.client.Account.Info(ctx)
		return shouldRetry(err)
	})
	if err != nil {
		return nil, errors.Wrap(err, "about failed")
	}
	return &fs.Usage{
		Total: fs.NewUsageValue(ai.Disk.Size),  // quota of bytes that can be used
		Used:  fs.NewUsageValue(ai.Disk.Used),  // bytes in use
		Free:  fs.NewUsageValue(ai.Disk.Avail), // bytes which can be uploaded before reaching the quota
	}, nil
}

// Hashes returns the supported hash sets.
func (f *Fs) Hashes() hash.Set {
	return hash.Set(hash.CRC32)
}

// DirCacheFlush resets the directory cache - used in testing as an
// optional interface
func (f *Fs) DirCacheFlush() {
	// defer log.Trace(f, "")("")
	f.dirCache.ResetRoot()
}

// CleanUp the trash in the Fs
func (f *Fs) CleanUp(ctx context.Context) (err error) {
	// defer log.Trace(f, "")("err=%v", &err)
	return f.pacer.Call(func() (bool, error) {
		req, err := f.client.NewRequest(ctx, "POST", "/v2/trash/empty", nil)
		if err != nil {
			return false, err
		}
		// fs.Debugf(f, "emptying trash")
		_, err = f.client.Do(req, nil)
		return shouldRetry(err)
	})
}
