package putio

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/putdotio/go-putio/putio"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config/configmap"
	"github.com/rclone/rclone/fs/config/configstruct"
	"github.com/rclone/rclone/fs/fshttp"
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
	opt         Options            // options for this Fs
	client      *putio.Client      // client for making API calls to Put.io
	pacer       *fs.Pacer          // To pace the API calls
	dirCache    *dircache.DirCache // Map of directory path to directory id
	httpClient  *http.Client       // base http client
	oAuthClient *http.Client       // http client with oauth Authorization
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

// parsePath parses a putio 'url'
func parsePath(path string) (root string) {
	root = strings.Trim(path, "/")
	return
}

// NewFs constructs an Fs from the path, container:path
func NewFs(ctx context.Context, name, root string, m configmap.Mapper) (f fs.Fs, err error) {
	// defer log.Trace(name, "root=%v", root)("f=%+v, err=%v", &f, &err)
	// Parse config into Options struct
	opt := new(Options)
	err = configstruct.Set(m, opt)
	if err != nil {
		return nil, err
	}
	root = parsePath(root)
	httpClient := fshttp.NewClient(ctx)
	oAuthClient, _, err := oauthutil.NewClientWithBaseClient(ctx, name, m, putioConfig, httpClient)
	if err != nil {
		return nil, fmt.Errorf("failed to configure putio: %w", err)
	}
	p := &Fs{
		name:        name,
		root:        root,
		opt:         *opt,
		pacer:       fs.NewPacer(ctx, pacer.NewDefault(pacer.MinSleep(minSleep), pacer.MaxSleep(maxSleep), pacer.DecayConstant(decayConstant))),
		client:      putio.NewClient(oAuthClient),
		httpClient:  httpClient,
		oAuthClient: oAuthClient,
	}
	p.features = (&fs.Features{
		DuplicateFiles:          true,
		ReadMimeType:            true,
		CanHaveEmptyDirectories: true,
	}).Fill(ctx, p)
	p.dirCache = dircache.New(root, "0", p)
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
		entry, err = f.client.Files.CreateFolder(ctx, f.opt.Enc.FromStandardName(leaf), parentID)
		return shouldRetry(ctx, err)
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
		return shouldRetry(ctx, err)
	})
	if err != nil {
		if perr, ok := err.(*putio.ErrorResponse); ok && perr.Response.StatusCode == 404 {
			err = nil
		}
		return
	}
	for _, child := range children {
		if f.opt.Enc.ToStandardName(child.Name) == leaf {
			found = true
			pathIDOut = itoa(child.ID)
			if !child.IsDir() {
				err = fs.ErrorIsFile
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
	directoryID, err := f.dirCache.FindDir(ctx, dir, false)
	if err != nil {
		return nil, err
	}
	parentID := atoi(directoryID)
	var children []putio.File
	err = f.pacer.Call(func() (bool, error) {
		// fs.Debugf(f, "listing files inside List: %d", parentID)
		children, _, err = f.client.Files.List(ctx, parentID)
		return shouldRetry(ctx, err)
	})
	if err != nil {
		return
	}
	for _, child := range children {
		remote := path.Join(dir, f.opt.Enc.ToStandardName(child.Name))
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
	existingObj, err := f.NewObject(ctx, src.Remote())
	switch err {
	case nil:
		return existingObj, existingObj.Update(ctx, in, src, options...)
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
	leaf, directoryID, err := f.dirCache.FindPath(ctx, remote, true)
	if err != nil {
		return nil, err
	}
	loc, err := f.createUpload(ctx, leaf, size, directoryID, src.ModTime(ctx), options)
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
		return shouldRetry(ctx, err)
	})
	if err != nil {
		return nil, err
	}
	return f.newObjectWithInfo(ctx, remote, entry)
}

func (f *Fs) createUpload(ctx context.Context, name string, size int64, parentID string, modTime time.Time, options []fs.OpenOption) (location string, err error) {
	// defer log.Trace(f, "name=%v, size=%v, parentID=%v, modTime=%v", name, size, parentID, modTime.String())("location=%v, err=%v", location, &err)
	err = f.pacer.Call(func() (bool, error) {
		req, err := http.NewRequestWithContext(ctx, "POST", "https://upload.put.io/files/", nil)
		if err != nil {
			return false, err
		}
		req.Header.Set("tus-resumable", "1.0.0")
		req.Header.Set("upload-length", strconv.FormatInt(size, 10))
		b64name := base64.StdEncoding.EncodeToString([]byte(f.opt.Enc.FromStandardName(name)))
		b64true := base64.StdEncoding.EncodeToString([]byte("true"))
		b64parentID := base64.StdEncoding.EncodeToString([]byte(parentID))
		b64modifiedAt := base64.StdEncoding.EncodeToString([]byte(modTime.Format(time.RFC3339)))
		req.Header.Set("upload-metadata", fmt.Sprintf("name %s,no-torrent %s,parent_id %s,updated-at %s", b64name, b64true, b64parentID, b64modifiedAt))
		fs.OpenOptionAddHTTPHeaders(req.Header, options)
		resp, err := f.oAuthClient.Do(req)
		retry, err := shouldRetry(ctx, err)
		if retry {
			return true, err
		}
		if err != nil {
			return false, err
		}
		if err := checkStatusCode(resp, 201); err != nil {
			return shouldRetry(ctx, err)
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
	// defer log.Trace(f, "location=%v, size=%v", location, size)("fileID=%v, err=%v", &fileID, &err)
	if size == 0 {
		err = f.pacer.Call(func() (bool, error) {
			fs.Debugf(f, "Sending zero length chunk")
			_, fileID, err = f.transferChunk(ctx, location, 0, bytes.NewReader([]byte{}), 0)
			return shouldRetry(ctx, err)
		})
		return
	}
	var clientOffset int64
	var offsetMismatch bool
	buf := make([]byte, defaultChunkSize)
	for clientOffset < size {
		chunkSize := size - clientOffset
		if chunkSize >= int64(defaultChunkSize) {
			chunkSize = int64(defaultChunkSize)
		}
		chunk := readers.NewRepeatableLimitReaderBuffer(in, buf, chunkSize)
		chunkStart := clientOffset
		reqSize := chunkSize
		transferOffset := clientOffset
		fs.Debugf(f, "chunkStart: %d, reqSize: %d", chunkStart, reqSize)

		// Transfer the chunk
		err = f.pacer.Call(func() (bool, error) {
			if offsetMismatch {
				// Get file offset and seek to the position
				offset, err := f.getServerOffset(ctx, location)
				if err != nil {
					return shouldRetry(ctx, err)
				}
				sentBytes := offset - chunkStart
				fs.Debugf(f, "sentBytes: %d", sentBytes)
				_, err = chunk.Seek(sentBytes, io.SeekStart)
				if err != nil {
					return shouldRetry(ctx, err)
				}
				transferOffset = offset
				reqSize = chunkSize - sentBytes
				offsetMismatch = false
			}
			fs.Debugf(f, "Sending chunk. transferOffset: %d length: %d", transferOffset, reqSize)
			var serverOffset int64
			serverOffset, fileID, err = f.transferChunk(ctx, location, transferOffset, chunk, reqSize)
			if cerr, ok := err.(*statusCodeError); ok && cerr.response.StatusCode == 409 {
				offsetMismatch = true
				return true, err
			}
			if serverOffset != (transferOffset + reqSize) {
				offsetMismatch = true
				return true, errors.New("connection broken")
			}
			return shouldRetry(ctx, err)
		})
		if err != nil {
			return
		}

		clientOffset += chunkSize
	}
	return
}

func (f *Fs) getServerOffset(ctx context.Context, location string) (offset int64, err error) {
	// defer log.Trace(f, "location=%v", location)("offset=%v, err=%v", &offset, &err)
	req, err := f.makeUploadHeadRequest(ctx, location)
	if err != nil {
		return 0, err
	}
	resp, err := f.oAuthClient.Do(req)
	if err != nil {
		return 0, err
	}
	err = checkStatusCode(resp, 200)
	if err != nil {
		return 0, err
	}
	return strconv.ParseInt(resp.Header.Get("upload-offset"), 10, 64)
}

func (f *Fs) transferChunk(ctx context.Context, location string, start int64, chunk io.ReadSeeker, chunkSize int64) (serverOffset, fileID int64, err error) {
	// defer log.Trace(f, "location=%v, start=%v, chunkSize=%v", location, start, chunkSize)("fileID=%v, err=%v", &fileID, &err)
	req, err := f.makeUploadPatchRequest(ctx, location, chunk, start, chunkSize)
	if err != nil {
		return
	}
	resp, err := f.oAuthClient.Do(req)
	if err != nil {
		return
	}
	defer func() {
		_ = resp.Body.Close()
	}()
	err = checkStatusCode(resp, 204)
	if err != nil {
		return
	}
	serverOffset, err = strconv.ParseInt(resp.Header.Get("upload-offset"), 10, 64)
	if err != nil {
		return
	}
	sfid := resp.Header.Get("putio-file-id")
	if sfid != "" {
		fileID, err = strconv.ParseInt(sfid, 10, 64)
		if err != nil {
			return
		}
	}
	return
}

func (f *Fs) makeUploadHeadRequest(ctx context.Context, location string) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, "HEAD", location, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("tus-resumable", "1.0.0")
	return req, nil
}

func (f *Fs) makeUploadPatchRequest(ctx context.Context, location string, in io.Reader, offset, length int64) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, "PATCH", location, in)
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
	_, err = f.dirCache.FindDir(ctx, dir, true)
	return err
}

// purgeCheck removes the root directory, if check is set then it
// refuses to do so if it has anything in
func (f *Fs) purgeCheck(ctx context.Context, dir string, check bool) (err error) {
	// defer log.Trace(f, "dir=%v", dir)("err=%v", &err)

	root := strings.Trim(path.Join(f.root, dir), "/")

	// can't remove root
	if root == "" {
		return errors.New("can't remove root directory")
	}

	// check directory exists
	directoryID, err := f.dirCache.FindDir(ctx, dir, false)
	if err != nil {
		return fmt.Errorf("Rmdir: %w", err)
	}
	dirID := atoi(directoryID)

	if check {
		// check directory empty
		var children []putio.File
		err = f.pacer.Call(func() (bool, error) {
			// fs.Debugf(f, "listing files: %d", dirID)
			children, _, err = f.client.Files.List(ctx, dirID)
			return shouldRetry(ctx, err)
		})
		if err != nil {
			return fmt.Errorf("Rmdir: %w", err)
		}
		if len(children) != 0 {
			return errors.New("directory not empty")
		}
	}

	// remove it
	err = f.pacer.Call(func() (bool, error) {
		// fs.Debugf(f, "deleting file: %d", dirID)
		err = f.client.Files.Delete(ctx, dirID)
		return shouldRetry(ctx, err)
	})
	f.dirCache.FlushDir(dir)
	return err
}

// Rmdir deletes the container
//
// Returns an error if it isn't empty
func (f *Fs) Rmdir(ctx context.Context, dir string) (err error) {
	return f.purgeCheck(ctx, dir, true)
}

// Precision returns the precision
func (f *Fs) Precision() time.Duration {
	return time.Second
}

// Purge deletes all the files in the directory
//
// Optional interface: Only implement this if you have a way of
// deleting all the files quicker than just running Remove() on the
// result of List()
func (f *Fs) Purge(ctx context.Context, dir string) (err error) {
	// defer log.Trace(f, "")("err=%v", &err)
	return f.purgeCheck(ctx, dir, false)
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
func (f *Fs) Copy(ctx context.Context, src fs.Object, remote string) (o fs.Object, err error) {
	// defer log.Trace(f, "src=%+v, remote=%v", src, remote)("o=%+v, err=%v", &o, &err)
	srcObj, ok := src.(*Object)
	if !ok {
		return nil, fs.ErrorCantCopy
	}
	leaf, directoryID, err := f.dirCache.FindPath(ctx, remote, true)
	if err != nil {
		return nil, err
	}
	err = f.pacer.Call(func() (bool, error) {
		params := url.Values{}
		params.Set("file_id", strconv.FormatInt(srcObj.file.ID, 10))
		params.Set("parent_id", directoryID)
		params.Set("name", f.opt.Enc.FromStandardName(leaf))
		req, err := f.client.NewRequest(ctx, "POST", "/v2/files/copy", strings.NewReader(params.Encode()))
		if err != nil {
			return false, err
		}
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		// fs.Debugf(f, "copying file (%d) to parent_id: %s", srcObj.file.ID, directoryID)
		_, err = f.client.Do(req, nil)
		return shouldRetry(ctx, err)
	})
	if err != nil {
		return nil, err
	}
	return f.NewObject(ctx, remote)
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
func (f *Fs) Move(ctx context.Context, src fs.Object, remote string) (o fs.Object, err error) {
	// defer log.Trace(f, "src=%+v, remote=%v", src, remote)("o=%+v, err=%v", &o, &err)
	srcObj, ok := src.(*Object)
	if !ok {
		return nil, fs.ErrorCantMove
	}
	leaf, directoryID, err := f.dirCache.FindPath(ctx, remote, true)
	if err != nil {
		return nil, err
	}
	err = f.pacer.Call(func() (bool, error) {
		params := url.Values{}
		params.Set("file_id", strconv.FormatInt(srcObj.file.ID, 10))
		params.Set("parent_id", directoryID)
		params.Set("name", f.opt.Enc.FromStandardName(leaf))
		req, err := f.client.NewRequest(ctx, "POST", "/v2/files/move", strings.NewReader(params.Encode()))
		if err != nil {
			return false, err
		}
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		// fs.Debugf(f, "moving file (%d) to parent_id: %s", srcObj.file.ID, directoryID)
		_, err = f.client.Do(req, nil)
		return shouldRetry(ctx, err)
	})
	if err != nil {
		return nil, err
	}
	return f.NewObject(ctx, remote)
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
	// defer log.Trace(f, "src=%+v, srcRemote=%v, dstRemote", src, srcRemote, dstRemote)("err=%v", &err)
	srcFs, ok := src.(*Fs)
	if !ok {
		return fs.ErrorCantDirMove
	}

	srcID, _, _, dstDirectoryID, dstLeaf, err := f.dirCache.DirMove(ctx, srcFs.dirCache, srcFs.root, srcRemote, f.root, dstRemote)
	if err != nil {
		return err
	}

	err = f.pacer.Call(func() (bool, error) {
		params := url.Values{}
		params.Set("file_id", srcID)
		params.Set("parent_id", dstDirectoryID)
		params.Set("name", f.opt.Enc.FromStandardName(dstLeaf))
		req, err := f.client.NewRequest(ctx, "POST", "/v2/files/move", strings.NewReader(params.Encode()))
		if err != nil {
			return false, err
		}
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		// fs.Debugf(f, "moving file (%s) to parent_id: %s", srcID, dstDirectoryID)
		_, err = f.client.Do(req, nil)
		return shouldRetry(ctx, err)
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
		return shouldRetry(ctx, err)
	})
	if err != nil {
		return nil, err
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
		return shouldRetry(ctx, err)
	})
}
