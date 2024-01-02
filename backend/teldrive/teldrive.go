// Package teldrive provides an interface to the teldrive storage system.
package teldrive

import (
	"bufio"
	"context"
	"crypto/md5"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/gofrs/uuid"
	"github.com/rclone/rclone/backend/teldrive/api"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config"
	"github.com/rclone/rclone/fs/config/configmap"
	"github.com/rclone/rclone/fs/config/configstruct"
	"github.com/rclone/rclone/fs/fserrors"
	"github.com/rclone/rclone/fs/fshttp"
	"github.com/rclone/rclone/fs/hash"
	"github.com/rclone/rclone/lib/encoder"
	"github.com/rclone/rclone/lib/pacer"
	"github.com/rclone/rclone/lib/rest"
)

const (
	minSleep         = 400 * time.Millisecond
	maxSleep         = 5 * time.Second
	decayConstant    = 2
	attackConstant   = 0
	timeFormat       = time.RFC3339
	maxChunkSize     = 2000 * fs.Mebi
	defaultChunkSize = 1000 * fs.Mebi
	minChunkSize     = 500 * fs.Mebi
)

func init() {
	fs.Register(&fs.RegInfo{
		Name:        "teldrive",
		Description: "Tel Drive",
		NewFs:       NewFs,
		Options: []fs.Option{{
			Help:      "Access Token Cookie",
			Name:      "access_token",
			Sensitive: true,
		}, {
			Help:      "Api Host",
			Name:      "api_host",
			Sensitive: true,
		}, {
			Help:    "Chunk Size",
			Name:    "chunk_size",
			Default: defaultChunkSize,
		}, {
			Name:     "randomise_part",
			Default:  true,
			Help:     "Randomise part",
			Advanced: true,
		}, {
			Name:      "channel_id",
			Help:      "Channel ID",
			Sensitive: true,
		}, {
			Name:     "upload_concurrency",
			Default:  4,
			Help:     "Upload Concurrency",
			Advanced: true,
		},
			{
				Name:    "encrypt_files",
				Default: false,
				Help:    "Enable Native  Teldrive Encryption",
			}, {

				Name:     config.ConfigEncoding,
				Help:     config.ConfigEncodingHelp,
				Advanced: true,
				Default:  encoder.Standard,
			}},
	})
}

// Options defines the configuration for this backend
type Options struct {
	ApiHost           string               `config:"api_host"`
	AccessToken       string               `config:"access_token"`
	ChunkSize         fs.SizeSuffix        `config:"chunk_size"`
	RandomisePart     bool                 `config:"randomise_part"`
	UploadConcurrency int                  `config:"upload_concurrency"`
	ChannelID         int64                `config:"channel_id"`
	EncryptFiles      bool                 `config:"encrypt_files"`
	Enc               encoder.MultiEncoder `config:"encoding"`
}

// Fs is the interface a cloud storage system must provide
type Fs struct {
	root     string
	name     string
	opt      Options
	features *fs.Features
	srv      *rest.Client
	pacer    *fs.Pacer
	authHash string
}

// Object represents an teldrive object
type Object struct {
	fs       *Fs
	remote   string
	id       string
	size     int64
	parentId string
	name     string
	modTime  string
}

// Name of the remote (as passed into NewFs)
func (f *Fs) Name() string {
	return f.name
}

// Root of the remote (as passed into NewFs)
func (f *Fs) Root() string {
	return f.root
}

// String returns a description of the FS
func (f *Fs) String() string {
	return fmt.Sprintf("teldrive root '%s'", f.root)
}

// Precision of the ModTimes in this Fs
func (f *Fs) Precision() time.Duration {
	return fs.ModTimeNotSupported
}

// Hashes returns the supported hash types of the filesystem
func (f *Fs) Hashes() hash.Set {
	return hash.Set(hash.None)
}

// Features returns the optional features of this Fs
func (f *Fs) Features() *fs.Features {
	return f.features
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

// dirPath returns an escaped file path (f.root, file)
func (f *Fs) dirPath(file string) string {
	//return path.Join(f.diskRoot, file)
	if file == "" || file == "." {
		return "/" + f.root
	}
	return "/" + path.Join(f.root, file)
}

// returns the full path based on root and the last element
func (f *Fs) splitPathFull(pth string) (string, string) {
	fullPath := strings.Trim(path.Join(f.root, pth), "/")

	i := len(fullPath) - 1
	for i >= 0 && fullPath[i] != '/' {
		i--
	}

	if i < 0 {
		return "/" + fullPath[:i+1], fullPath[i+1:]
	}

	// do not include the / at the split
	return "/" + fullPath[:i], fullPath[i+1:]
}

func checkUploadChunkSize(cs fs.SizeSuffix) error {
	if cs < minChunkSize {
		return fmt.Errorf("ChunkSize: %s is less than %s", cs, minChunkSize)
	}
	if cs > maxChunkSize {
		return fmt.Errorf("ChunkSize: %s is greater than %s", cs, maxChunkSize)
	}
	return nil
}

// NewFs makes a new Fs object from the path
//
// The path is of the form remote:path
//
// Remotes are looked up in the config file.  If the remote isn't
// found then NotFoundInConfigFile will be returned.
//
// On Windows avoid single character remote names as they can be mixed
// up with drive letters.
func NewFs(ctx context.Context, name string, root string, config configmap.Mapper) (fs.Fs, error) {
	opt := new(Options)
	err := configstruct.Set(config, opt)
	if err != nil {
		return nil, err
	}

	err = checkUploadChunkSize(opt.ChunkSize)
	if err != nil {
		return nil, err
	}

	f := &Fs{
		name:  name,
		root:  root,
		opt:   *opt,
		pacer: fs.NewPacer(ctx, pacer.NewDefault(pacer.MinSleep(minSleep), pacer.MaxSleep(maxSleep), pacer.DecayConstant(decayConstant), pacer.AttackConstant(attackConstant))),
	}
	if root == "/" || root == "." {
		f.root = ""
	} else {
		f.root = strings.Trim(root, "/")
	}

	f.features = (&fs.Features{
		DuplicateFiles:          false,
		CanHaveEmptyDirectories: true,
		ReadMimeType:            false,
		ChunkWriterDoesntSeek:   true,
	}).Fill(ctx, f)

	client := fshttp.NewClient(ctx)
	authCookie := http.Cookie{Name: "user-session", Value: opt.AccessToken}
	f.srv = rest.NewClient(client).SetRoot(strings.Trim(opt.ApiHost, "/")).SetCookie(&authCookie)

	opts := rest.Opts{
		Method: "GET",
		Path:   "/api/auth/session",
	}

	var (
		session     api.Session
		sessionResp *http.Response
	)

	err = f.pacer.Call(func() (bool, error) {
		sessionResp, err = f.srv.CallJSON(ctx, &opts, nil, &session)
		return shouldRetry(ctx, sessionResp, err)
	})

	if err != nil {
		return nil, err
	}

	if session.Hash == "" {
		return nil, fmt.Errorf("invalid session token")
	}

	for _, cookie := range sessionResp.Cookies() {
		if cookie.Name == "user-session" && cookie.Value != "" {
			config.Set("access_token", cookie.Value)
		}
	}

	f.authHash = session.Hash

	dir, base := f.splitPathFull("")

	res, err := f.findObject(ctx, dir, base)

	if err != nil && !errors.Is(err, fs.ErrorDirNotFound) {
		return nil, err
	}

	if res != nil && len(res.Files) == 1 && res.Files[0].Type == "file" {
		f.root = strings.Trim(dir, "/")
		return f, fs.ErrorIsFile
	}

	return f, nil
}

func (f *Fs) readMetaDataForPath(ctx context.Context, path string, options *api.MetadataRequestOptions) (*api.ReadMetadataResponse, error) {

	opts := rest.Opts{
		Method: "GET",
		Path:   "/api/files",
		Parameters: url.Values{
			"path":          []string{path},
			"perPage":       []string{strconv.FormatUint(options.PerPage, 10)},
			"sort":          []string{"name"},
			"order":         []string{"asc"},
			"op":            []string{"list"},
			"nextPageToken": []string{options.NextPageToken},
		},
	}
	var err error
	var info api.ReadMetadataResponse
	var resp *http.Response

	err = f.pacer.Call(func() (bool, error) {
		resp, err = f.srv.CallJSON(ctx, &opts, nil, &info)
		return shouldRetry(ctx, resp, err)
	})

	if resp.StatusCode == 404 {
		return nil, fs.ErrorDirNotFound
	}

	if err != nil {
		return nil, err
	}

	return &info, nil
}

func (f *Fs) getPathInfo(ctx context.Context, path string) (*api.ReadMetadataResponse, error) {

	opts := rest.Opts{
		Method: "GET",
		Path:   "/api/files",
		Parameters: url.Values{
			"path": []string{path},
			"op":   []string{"find"},
		},
	}
	var err error
	var info api.ReadMetadataResponse
	var resp *http.Response
	err = f.pacer.Call(func() (bool, error) {
		resp, err = f.srv.CallJSON(ctx, &opts, nil, &info)
		return shouldRetry(ctx, resp, err)
	})
	if err != nil {
		return nil, err
	}

	return &info, nil
}

func (f *Fs) findObject(ctx context.Context, path string, name string) (*api.ReadMetadataResponse, error) {

	opts := rest.Opts{
		Method: "GET",
		Path:   "/api/files",
		Parameters: url.Values{
			"path": []string{path},
			"op":   []string{"find"},
			"name": []string{name},
		},
	}
	var err error
	var info api.ReadMetadataResponse
	var resp *http.Response
	err = f.pacer.Call(func() (bool, error) {
		resp, err = f.srv.CallJSON(ctx, &opts, nil, &info)
		return shouldRetry(ctx, resp, err)
	})
	if err != nil && resp.StatusCode == 404 {
		return nil, fs.ErrorDirNotFound
	}

	if err != nil {
		return nil, err
	}

	return &info, nil
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
	root := f.dirPath(dir)

	var limit uint64 = 500        // max number of objects per request - 100 seems to be the maximum the api accepts
	var nextPageToken string = "" // for the next page of requests

	for {
		opts := &api.MetadataRequestOptions{
			PerPage:       limit,
			NextPageToken: nextPageToken,
		}

		info, err := f.readMetaDataForPath(ctx, root, opts)
		if err != nil {
			return nil, err
		}

		for _, item := range info.Files {
			remote := path.Join(dir, f.opt.Enc.ToStandardName(item.Name))
			if item.Type == "folder" {
				modTime, _ := time.Parse(timeFormat, item.ModTime)
				d := fs.NewDir(remote, modTime).SetID(item.Id).SetParentID(item.ParentId)
				entries = append(entries, d)
			}
			if item.Type == "file" {
				o, err := f.newObjectWithInfo(ctx, remote, &item)
				if err != nil {
					continue
				}
				entries = append(entries, o)
			}

		}

		nextPageToken = info.NextPageToken
		//check if we reached end of list
		if nextPageToken == "" {
			break
		}
	}
	return entries, nil
}

// Return an Object from a path
//
// If it can't be found it returns the error fs.ErrorObjectNotFound.
func (f *Fs) newObjectWithInfo(ctx context.Context, remote string, info *api.FileInfo) (fs.Object, error) {
	o := &Object{
		fs:       f,
		remote:   remote,
		id:       info.Id,
		size:     info.Size,
		parentId: info.ParentId,
		name:     info.Name,
		modTime:  info.ModTime,
	}
	return o, nil
}

// NewObject finds the Object at remote.  If it can't be found it
// returns the error fs.ErrorObjectNotFound.
func (f *Fs) NewObject(ctx context.Context, remote string) (fs.Object, error) {

	base, leaf := f.splitPathFull(remote)

	res, err := f.findObject(ctx, base, leaf)

	if err != nil {
		// need to change error type
		// if the parent dir doesn't exist the object doesn't exist either
		if err == fs.ErrorDirNotFound {
			return nil, fs.ErrorObjectNotFound
		}
		return nil, err
	}

	if len(res.Files) == 0 {
		return nil, fs.ErrorObjectNotFound
	}

	return f.newObjectWithInfo(ctx, remote, &res.Files[0])
}

func (f *Fs) move(ctx context.Context, dstPath string, fileID string) (err error) {

	opts := rest.Opts{
		Method: "POST",
		Path:   "/api/files/move",
	}

	mv := api.MoveFileRequest{
		Files:       []string{fileID},
		Destination: dstPath,
	}

	var resp *http.Response
	var info api.UpdateResponse
	err = f.pacer.Call(func() (bool, error) {
		resp, err = f.srv.CallJSON(ctx, &opts, &mv, &info)
		return shouldRetry(ctx, resp, err)
	})
	if err != nil {
		return fmt.Errorf("couldn't move file: %w", err)
	}
	return nil
}

// updateFileInformation set's various file attributes most importantly it's name
func (f *Fs) updateFileInformation(ctx context.Context, update *api.UpdateFileInformation, fileId string) (err error) {
	opts := rest.Opts{
		Method: "PATCH",
		Path:   "/api/files/" + fileId,
	}

	var resp *http.Response
	var info api.UpdateResponse
	err = f.pacer.Call(func() (bool, error) {
		resp, err = f.srv.CallJSON(ctx, &opts, update, &info)
		return shouldRetry(ctx, resp, err)
	})
	if err != nil {
		return fmt.Errorf("couldn't update file info: %w", err)
	}
	return err
}

func MD5(text string) string {
	algorithm := md5.New()
	algorithm.Write([]byte(text))
	return hex.EncodeToString(algorithm.Sum(nil))
}

func (f *Fs) putUnchecked(ctx context.Context, in0 io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) error {

	base, leaf := f.splitPathFull(src.Remote())

	modTime := src.ModTime(ctx).UTC().Format(timeFormat)

	uploadID := MD5(fmt.Sprintf("%s:%d:%s", path.Join(base, leaf), src.Size(), modTime))

	chunkSize := int64(f.opt.ChunkSize)

	var existingParts map[int]api.PartFile

	var uploadFile api.UploadFile

	if chunkSize < src.Size() {
		opts := rest.Opts{
			Method: "GET",
			Path:   "/api/uploads/" + uploadID,
		}

		err := f.pacer.Call(func() (bool, error) {
			resp, err := f.srv.CallJSON(ctx, &opts, nil, &uploadFile)
			return shouldRetry(ctx, resp, err)
		})

		if err == nil {
			existingParts = make(map[int]api.PartFile, len(uploadFile.Parts))
			for _, part := range uploadFile.Parts {
				existingParts[part.PartNo] = part
			}
		}
	}

	totalParts := src.Size() / chunkSize

	if src.Size()%chunkSize != 0 {
		totalParts++
	}

	var partsToCommit []api.PartFile

	var uploadedSize int64

	in := bufio.NewReader(in0)

	channelID := f.opt.ChannelID

	encryptFile := f.opt.EncryptFiles

	if len(uploadFile.Parts) > 0 {
		channelID = uploadFile.Parts[0].ChannelID

		encryptFile = uploadFile.Parts[0].Encrypted
	}

	partName := leaf

	for partNo := 1; partNo <= int(totalParts); partNo++ {

		if existing, ok := existingParts[partNo]; ok {
			io.CopyN(io.Discard, in, existing.Size)
			partsToCommit = append(partsToCommit, existing)
			uploadedSize += existing.Size
			continue
		}

		if partNo == int(totalParts) {
			chunkSize = src.Size() - uploadedSize
		}
		partReader := io.LimitReader(in, chunkSize)

		if f.opt.RandomisePart {
			u1, _ := uuid.NewV4()
			partName = hex.EncodeToString(u1.Bytes())
		} else if totalParts > 1 {
			partName = fmt.Sprintf("%s.part.%03d", leaf, partNo)
		}

		opts := rest.Opts{
			Method:        "POST",
			Path:          "/api/uploads/" + uploadID,
			Body:          partReader,
			ContentLength: &chunkSize,
			Parameters: url.Values{
				"partName":  []string{partName},
				"fileName":  []string{leaf},
				"partNo":    []string{strconv.Itoa(partNo)},
				"channelId": []string{strconv.FormatInt(channelID, 10)},
				"encrypted": []string{strconv.FormatBool(encryptFile)},
			},
		}

		var info api.PartFile
		err := f.pacer.Call(func() (bool, error) {
			resp, err := f.srv.CallJSON(ctx, &opts, nil, &info)
			return shouldRetry(ctx, resp, err)
		})
		if err != nil {
			return err
		}
		uploadedSize += chunkSize

		partsToCommit = append(partsToCommit, info)
	}

	if base != "/" {
		err := f.CreateDir(ctx, base, "")
		if err != nil {
			return err
		}
	}

	opts := rest.Opts{
		Method: "POST",
		Path:   "/api/files",
	}

	fileParts := []api.FilePart{}

	for _, part := range partsToCommit {
		fileParts = append(fileParts, api.FilePart{ID: part.PartId, Salt: part.Salt})
	}

	payload := api.CreateFileRequest{
		Name:      leaf,
		Type:      "file",
		Path:      base,
		MimeType:  fs.MimeType(ctx, src),
		Size:      src.Size(),
		Parts:     fileParts,
		ChannelID: channelID,
		Encrypted: encryptFile,
	}
	err := f.pacer.Call(func() (bool, error) {
		resp, err := f.srv.CallJSON(ctx, &opts, &payload, nil)
		return shouldRetry(ctx, resp, err)
	})
	if err != nil {
		return err
	}
	opts = rest.Opts{
		Method: "DELETE",
		Path:   "/api/uploads/" + uploadID,
	}
	err = f.pacer.Call(func() (bool, error) {
		resp, err := f.srv.Call(ctx, &opts)
		return shouldRetry(ctx, resp, err)
	})
	if err != nil {
		return err
	}

	return nil
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
func (f *Fs) PutUnchecked(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error) {
	err := f.putUnchecked(ctx, in, src, options...)
	if err != nil {
		return nil, err
	}
	return f.NewObject(ctx, src.Remote())
}

// Update the already existing object
//
// Copy the reader into the object updating modTime and size.
//
// The new object may have been created if an error is returned
func (o *Object) Update(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) error {
	if src.Size() < 0 {
		return errors.New("refusing to update with unknown size")
	}

	// upload with new size but old name
	//to change
	err := o.fs.putUnchecked(ctx, in, src, options...)
	if err != nil {
		return err
	}

	// delete duplicate object after successful upload
	err = o.Remove(ctx)
	if err != nil {
		return fmt.Errorf("failed to remove old version: %w", err)
	}

	// Fetch new object after deleting the duplicate
	info, err := o.fs.NewObject(ctx, o.Remote())
	if err != nil {
		return err
	}

	// Replace guts of old object with new one
	*o = *info.(*Object)

	return nil
}

// OpenChunkWriter returns the chunk size and a ChunkWriter
//
// Pass in the remote and the src object
// You can also use options to hint at the desired chunk size
func (f *Fs) OpenChunkWriter(
	ctx context.Context,
	remote string,
	src fs.ObjectInfo,
	options ...fs.OpenOption) (info fs.ChunkWriterInfo, writer fs.ChunkWriter, err error) {

	o := &Object{
		fs:     f,
		remote: remote,
	}
	ui, err := o.prepareUpload(ctx, src, options)

	if err != nil {
		return info, nil, fmt.Errorf("failed to prepare upload: %w", err)
	}

	size := src.Size()

	chunkSize := int64(f.opt.ChunkSize)

	totalParts := size / chunkSize

	if src.Size()%chunkSize != 0 {
		totalParts++
	}

	existingParts := make(map[int]api.PartFile, len(ui.existingChunks))

	for _, part := range ui.existingChunks {
		existingParts[part.PartNo] = part
	}

	chunkWriter := &objectChunkWriter{
		chunkSize:     chunkSize,
		size:          size,
		f:             f,
		uploadID:      ui.uploadID,
		existingParts: existingParts,
		src:           src,
		o:             o,
		totalParts:    totalParts,
		channelID:     ui.channelID,
		encryptFile:   ui.encryptFile,
	}
	info = fs.ChunkWriterInfo{
		ChunkSize:         int64(chunkSize),
		Concurrency:       o.fs.opt.UploadConcurrency,
		LeavePartsOnError: true,
	}
	fs.Debugf(o, "open chunk writer: started upload: %v", ui.uploadID)
	return info, chunkWriter, err
}

// CreateDir dir creates a directory with the given parent path
// base starts from root and may or may not include //
func (f *Fs) CreateDir(ctx context.Context, base string, leaf string) (err error) {

	var resp *http.Response
	var apiErr api.Error
	opts := rest.Opts{
		Method: "POST",
		Path:   "/api/files/directories",
	}

	dir := base
	if leaf != "" {
		dir = path.Join(dir, leaf)
	}

	if len(dir) == 0 || dir[0] != '/' {
		dir = "/" + dir
	}

	mkdir := api.CreateDirRequest{
		Path: dir,
	}
	err = f.pacer.Call(func() (bool, error) {
		resp, err = f.srv.CallJSON(ctx, &opts, &mkdir, &apiErr)
		return shouldRetry(ctx, resp, err)
	})
	if err != nil {
		return err
	}
	return nil
}

// Mkdir makes the directory (container, bucket)
//
// Shouldn't return an error if it already exists
func (f *Fs) Mkdir(ctx context.Context, dir string) (err error) {
	if dir == "" || dir == "." {
		return f.CreateDir(ctx, f.root, "")
	}
	return f.CreateDir(ctx, f.root, dir)
}

// may or may not delete folders with contents?
func (f *Fs) purge(ctx context.Context, folderID string) (err error) {
	var resp *http.Response
	opts := rest.Opts{
		Method: "POST",
		Path:   "/api/files/delete",
	}
	rm := api.RemoveFileRequest{
		Files: []string{folderID},
	}
	err = f.pacer.Call(func() (bool, error) {
		resp, err = f.srv.CallJSON(ctx, &opts, &rm, nil)
		return shouldRetry(ctx, resp, err)
	})
	if err != nil {
		return err
	}
	return nil
}

// Rmdir removes the directory (container, bucket) if empty
//
// Return an error if it doesn't exist or isn't empty
func (f *Fs) Rmdir(ctx context.Context, dir string) error {
	info, err := f.getPathInfo(ctx, f.dirPath(dir))
	if err != nil {
		return err
	}
	return f.purge(ctx, info.Files[0].Id)
}

// Move src to this remote using server-side move operations.
//
// This is stored with the remote path given.
//
// It returns the destination Object and a possible error.
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

	srcBase, srcLeaf := srcObj.fs.splitPathFull(src.Remote())
	dstBase, dstLeaf := f.splitPathFull(remote)

	needRename := srcLeaf != dstLeaf
	needMove := srcBase != dstBase

	// do the move if required
	if needMove {
		err := f.move(ctx, dstBase, srcObj.id)
		if err != nil {
			return nil, err
		}
	}

	// rename to final name if we need to
	if needRename {
		err := f.updateFileInformation(ctx, &api.UpdateFileInformation{
			Name: dstLeaf,
		}, srcObj.id)
		if err != nil {
			return nil, fmt.Errorf("move: failed final rename: %w", err)
		}
	}

	// copy the old object and apply the changes
	newObj := *srcObj
	newObj.remote = remote
	newObj.fs = f
	return &newObj, nil
}

// DirMove moves src, srcRemote to this remote at dstRemote
// using server-side move operations.
//
// Will only be called if src.Fs().Name() == f.Name()
//
// If it isn't possible then return fs.ErrorCantDirMove

// If destination exists then return fs.ErrorDirExists
func (f *Fs) DirMove(ctx context.Context, src fs.Fs, srcRemote, dstRemote string) error {
	srcFs, ok := src.(*Fs)
	if !ok {
		fs.Debugf(srcFs, "Can't move directory - not same remote type")
		return fs.ErrorCantDirMove
	}

	dstPath := f.dirPath(dstRemote)

	srcPath := srcFs.dirPath(srcRemote)

	opts := rest.Opts{
		Method: "POST",
		Path:   "/api/files/directories/move",
	}
	move := api.DirMove{
		Source:      srcPath,
		Destination: dstPath,
	}
	var resp *http.Response
	var err error
	err = f.pacer.Call(func() (bool, error) {
		resp, err = f.srv.CallJSON(ctx, &opts, &move, nil)
		return shouldRetry(ctx, resp, err)
	})
	if err != nil {
		return fmt.Errorf("dirmove: failed to move: %w", err)
	}
	return nil
}

func (o *Object) Remove(ctx context.Context) error {
	opts := rest.Opts{
		Method: "POST",
		Path:   "/api/files/delete",
	}
	delete := api.RemoveFileRequest{
		Files: []string{o.id},
	}
	var info api.UpdateResponse
	err := o.fs.pacer.Call(func() (bool, error) {
		resp, err := o.fs.srv.CallJSON(ctx, &opts, &delete, &info)
		return shouldRetry(ctx, resp, err)
	})
	if err != nil {
		return err
	}
	return nil
}

// Open an object for read
func (o *Object) Open(ctx context.Context, options ...fs.OpenOption) (in io.ReadCloser, err error) {
	var resp *http.Response

	http := o.fs.srv

	fs.FixRangeOption(options, o.size)
	opts := rest.Opts{
		Method:  "GET",
		Path:    fmt.Sprintf("/api/files/%s/stream/%s", o.id, url.QueryEscape(o.name)),
		Options: options,
		Parameters: url.Values{
			"hash": []string{o.fs.authHash},
		},
	}

	err = o.fs.pacer.Call(func() (bool, error) {
		resp, err = http.Call(ctx, &opts)
		return shouldRetry(ctx, resp, err)
	})

	if err != nil {
		return nil, err
	}
	return resp.Body, err
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
func (f *Fs) Copy(ctx context.Context, src fs.Object, remote string) (fs.Object, error) {
	srcObj, ok := src.(*Object)
	if !ok {
		fs.Debugf(src, "Can't copy - not same remote type")
		return nil, fs.ErrorCantCopy
	}

	dstBase, dstLeaf := f.splitPathFull(remote)

	srcBase, srcLeaf := srcObj.fs.splitPathFull(src.Remote())

	if dstBase == srcBase && dstLeaf == srcLeaf {
		fs.Debugf(src, "Can't copy - change file name")
		return nil, fs.ErrorCantCopy
	}

	opts := rest.Opts{
		Method: "POST",
		Path:   "/api/files/copy",
	}
	copy := api.CopyFile{
		ID:          srcObj.id,
		Name:        dstLeaf,
		Destination: dstBase,
	}
	var info api.FileInfo
	err := f.pacer.Call(func() (bool, error) {
		resp, err := f.srv.CallJSON(ctx, &opts, &copy, &info)
		return shouldRetry(ctx, resp, err)
	})
	if err != nil {
		return nil, err
	}
	return f.newObjectWithInfo(ctx, remote, &info)
}

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

// ModTime returns the modification time of the object
//
// It attempts to read the objects mtime and if that isn't present the
// LastModified returned in the http headers
func (o *Object) ModTime(ctx context.Context) time.Time {
	modTime, err := time.Parse(timeFormat, o.modTime)
	if err != nil {
		fs.Debugf(o, "Failed to read mtime from object: %v", err)
		return time.Now()
	}
	return modTime
}

// Size returns the size of an object in bytes
func (o *Object) Size() int64 {
	return o.size
}

// Hash returns the Md5sum of an object returning a lowercase hex string
func (o *Object) Hash(ctx context.Context, t hash.Type) (string, error) {
	return "", hash.ErrUnsupported
}

// ID returns the ID of the Object if known, or "" if not
func (o *Object) ID() string {
	return o.id
}

// Storable returns whether this object is storable
func (o *Object) Storable() bool {
	return true
}

// SetModTime sets the modification time of the local fs object
func (o *Object) SetModTime(ctx context.Context, modTime time.Time) error {
	return fs.ErrorCantSetModTime
}

// Check the interfaces are satisfied
var (
	_ fs.Fs              = (*Fs)(nil)
	_ fs.Copier          = (*Fs)(nil)
	_ fs.Mover           = (*Fs)(nil)
	_ fs.DirMover        = (*Fs)(nil)
	_ fs.Object          = (*Object)(nil)
	_ fs.OpenChunkWriter = (*Fs)(nil)
)
