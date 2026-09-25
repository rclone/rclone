// Package dosya provides an interface to the dosya.dev cloud storage system.
package dosya

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/rclone/rclone/backend/dosya/api"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config"
	"github.com/rclone/rclone/fs/config/configmap"
	"github.com/rclone/rclone/fs/config/configstruct"
	"github.com/rclone/rclone/fs/fshttp"
	"github.com/rclone/rclone/fs/hash"
	"github.com/rclone/rclone/lib/dircache"
	"github.com/rclone/rclone/lib/encoder"
	"github.com/rclone/rclone/lib/pacer"
	"github.com/rclone/rclone/lib/rest"
)

const (
	rootID         = "root"
	defaultBaseURL = "https://api.dosya.dev"
	minSleep       = 100 * time.Millisecond
	maxSleep       = 2 * time.Second
	decayConstant  = 2
	attackConstant = 0
)

func init() {
	fs.Register(&fs.RegInfo{
		Name:        "dosya",
		Description: "dosya.dev",
		NewFs:       NewFs,
		Options: []fs.Option{{
			Help:      "Your API Key, get it from https://dosya.dev/settings/api-keys.",
			Name:      "api_key",
			Sensitive: true,
			Required:  true,
		}, {
			Help:     "Your workspace ID.",
			Name:     "workspace_id",
			Required: true,
		}, {
			Help:     "API base URL.",
			Name:     "api_url",
			Default:  defaultBaseURL,
			Advanced: true,
		}, {
			Name: "upload_concurrency",
			Help: `Concurrency for multipart uploads.

This is the number of parts of the same file that are uploaded
concurrently. The first part is always uploaded on its own, because
the server creates the upload when it receives it.

Each part in flight is held in memory, so memory use grows with
this value times the part size the server chooses.

If you are uploading large files over high-speed links and these
uploads do not fully utilize your bandwidth, then increasing this
may help to speed up the transfers.`,
			Default:  4,
			Advanced: true,
		}, {
			Name:     config.ConfigEncoding,
			Help:     config.ConfigEncodingHelp,
			Advanced: true,
			Default: (encoder.Display |
				encoder.EncodeBackSlash |
				encoder.EncodeSingleQuote |
				encoder.EncodeDoubleQuote |
				encoder.EncodeLtGt |
				encoder.EncodeCtl |
				encoder.EncodeLeftSpace |
				encoder.EncodeRightSpace |
				encoder.EncodeInvalidUtf8),
		}},
	})
}

// Options defines the configuration for this backend
type Options struct {
	APIKey            string               `config:"api_key"`
	WorkspaceID       string               `config:"workspace_id"`
	APIURL            string               `config:"api_url"`
	UploadConcurrency int                  `config:"upload_concurrency"`
	Enc               encoder.MultiEncoder `config:"encoding"`
}

// Fs represents a remote dosya.dev filesystem
type Fs struct {
	name     string
	root     string
	opt      Options
	features *fs.Features
	rest     *rest.Client
	pacer    *fs.Pacer
	dirCache *dircache.DirCache
}

// Object represents a dosya.dev file
type Object struct {
	fs     *Fs
	remote string
	file   api.FileItem
}

// FindLeaf finds a directory of name leaf in the folder with ID pathID
func (f *Fs) FindLeaf(ctx context.Context, pathID, leaf string) (pathIDOut string, found bool, err error) {
	listing, err := f.listFilesAndFolders(ctx, pathID)
	if err != nil {
		return "", false, err
	}
	for _, folder := range listing.Folders {
		if f.opt.Enc.ToStandardName(folder.Name) == leaf {
			return folder.ID, true, nil
		}
	}
	return "", false, nil
}

// CreateDir makes a directory with pathID as parent and name leaf
func (f *Fs) CreateDir(ctx context.Context, pathID, leaf string) (newID string, err error) {
	name := f.opt.Enc.FromStandardName(leaf)
	resp, err := f.createFolder(ctx, name, pathID)
	if err != nil {
		return "", err
	}
	return resp.Folder.ID, nil
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
	return fmt.Sprintf("dosya.dev root '%s'", f.root)
}

// Precision of the ModTimes in this Fs
func (f *Fs) Precision() time.Duration {
	// Modification times are stored and returned to the second (the API
	// keeps them as Unix seconds, sent on upload via the source-mtime header).
	return time.Second
}

// Hashes returns the supported hash types of the filesystem
func (f *Fs) Hashes() hash.Set {
	// The server computes and stores the SHA-256 of a single-shot upload's
	// bytes; multipart uploads have none (see Object.Hash).
	return hash.NewHashSet(hash.SHA256)
}

// Features returns the optional features of this Fs
func (f *Fs) Features() *fs.Features {
	return f.features
}

// NewFs makes a new Fs object from the path
func NewFs(ctx context.Context, name string, root string, config configmap.Mapper) (fs.Fs, error) {
	opt := new(Options)
	err := configstruct.Set(config, opt)
	if err != nil {
		return nil, err
	}

	root = strings.Trim(root, "/")

	f := &Fs{
		name:  name,
		root:  root,
		opt:   *opt,
		pacer: fs.NewPacer(ctx, pacer.NewDefault(pacer.MinSleep(minSleep), pacer.MaxSleep(maxSleep), pacer.DecayConstant(decayConstant), pacer.AttackConstant(attackConstant))),
	}

	f.features = (&fs.Features{
		CaseInsensitive:         false,
		DuplicateFiles:          true,
		CanHaveEmptyDirectories: true,
		ReadMimeType:            true,
	}).Fill(ctx, f)

	client := fshttp.NewClient(ctx)
	f.rest = rest.NewClient(client).SetRoot(strings.TrimSuffix(opt.APIURL, "/")).SetErrorHandler(errorHandler)
	f.rest.SetHeader("Authorization", "Bearer "+f.opt.APIKey)

	f.dirCache = dircache.New(root, rootID, f)

	// Find the current root
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
		f.features.Fill(ctx, &tempF)
		f.dirCache = tempF.dirCache
		f.root = tempF.root
		return f, fs.ErrorIsFile
	}
	return f, nil
}

// List the objects and directories in dir into entries
func (f *Fs) List(ctx context.Context, dir string) (entries fs.DirEntries, err error) {
	directoryID, err := f.dirCache.FindDir(ctx, dir, false)
	if err != nil {
		return nil, err
	}

	listing, err := f.listFilesAndFolders(ctx, directoryID)
	if err != nil {
		return nil, err
	}

	for _, folder := range listing.Folders {
		folderName := f.opt.Enc.ToStandardName(folder.Name)
		fullPath := getRemote(dir, folderName)
		modTime := time.Unix(int64(folder.CreatedAt), 0)
		d := fs.NewDir(fullPath, modTime).SetID(folder.ID)
		entries = append(entries, d)
		f.dirCache.Put(fullPath, folder.ID)
	}

	for _, file := range listing.Files {
		fileName := f.opt.Enc.ToStandardName(file.Name)
		remote := getRemote(dir, fileName)
		o := &Object{
			fs:     f,
			remote: remote,
			file:   file,
		}
		entries = append(entries, o)
	}

	return entries, nil
}

// NewObject finds the Object at remote
func (f *Fs) NewObject(ctx context.Context, remote string) (fs.Object, error) {
	leaf, directoryID, err := f.dirCache.FindPath(ctx, remote, false)
	if err != nil {
		if err == fs.ErrorDirNotFound {
			return nil, fs.ErrorObjectNotFound
		}
		return nil, err
	}

	listing, err := f.listFilesAndFolders(ctx, directoryID)
	if err != nil {
		return nil, err
	}

	for _, file := range listing.Files {
		if f.opt.Enc.ToStandardName(file.Name) == leaf {
			path, ok := f.dirCache.GetInv(directoryID)
			if !ok {
				return nil, fmt.Errorf("cannot find dir in dircache")
			}
			return &Object{
				fs:     f,
				remote: getRemote(path, f.opt.Enc.ToStandardName(file.Name)),
				file:   file,
			}, nil
		}
	}

	return nil, fs.ErrorObjectNotFound
}

// Put uploads a new file
func (f *Fs) Put(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error) {
	existingObj, err := f.NewObject(ctx, src.Remote())
	switch err {
	case nil:
		return existingObj, existingObj.Update(ctx, in, src, options...)
	case fs.ErrorObjectNotFound:
		return f.PutUnchecked(ctx, in, src, options...)
	default:
		return nil, err
	}
}

// PutUnchecked uploads without checking for duplicates
func (f *Fs) PutUnchecked(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error) {
	remote := src.Remote()
	size := src.Size()

	if size == 0 {
		return nil, fs.ErrorCantUploadEmptyFiles
	}

	leaf, directoryID, err := f.dirCache.FindPath(ctx, remote, true)
	if err != nil {
		return nil, err
	}
	_ = leaf

	modTime := src.ModTime(ctx)
	resp, err := f.uploadFile(ctx, in, remote, size, modTime, directoryID, nil)
	if err != nil {
		return nil, err
	}

	return &Object{
		fs:     f,
		remote: remote,
		file:   fileFromUpload(resp, modTime),
	}, nil
}

// fileFromUpload builds the FileItem for the object an upload just created.
// The single-shot upload response omits updated_at, so the modification time
// is taken from what was sent (modTime) rather than defaulting to the create
// time - it is the time the server stored via the source-mtime header. The
// SHA-256 comes straight from the response (empty for a multipart upload).
func fileFromUpload(resp *api.UploadCompleteResponse, modTime time.Time) api.FileItem {
	updatedAt := resp.File.CreatedAt
	if !modTime.IsZero() {
		updatedAt = float64(modTime.Unix())
	}
	return api.FileItem{
		ID:          resp.File.ID,
		Name:        resp.File.Name,
		SizeBytes:   resp.File.SizeBytes,
		MimeType:    resp.File.MimeType,
		Extension:   resp.File.Extension,
		Region:      resp.File.Region,
		CreatedAt:   resp.File.CreatedAt,
		UpdatedAt:   updatedAt,
		ContentHash: resp.File.ContentHash,
	}
}

// Mkdir makes the directory (container, bucket)
func (f *Fs) Mkdir(ctx context.Context, dir string) error {
	_, err := f.dirCache.FindDir(ctx, dir, true)
	return err
}

// Rmdir removes the directory (container, bucket) if empty
func (f *Fs) Rmdir(ctx context.Context, dir string) error {
	directoryID, err := f.dirCache.FindDir(ctx, dir, false)
	if err != nil {
		return err
	}

	// Check if directory is empty before removing
	listing, err := f.listFilesAndFolders(ctx, directoryID)
	if err != nil {
		return err
	}
	if len(listing.Files) > 0 || len(listing.Folders) > 0 {
		return fs.ErrorDirectoryNotEmpty
	}

	err = f.removeFolder(ctx, directoryID)
	if err != nil {
		return err
	}

	f.dirCache.FlushDir(dir)
	return nil
}

// Move src to this remote using server-side move operations
func (f *Fs) Move(ctx context.Context, src fs.Object, remote string) (fs.Object, error) {
	srcObj, ok := src.(*Object)
	if !ok {
		fs.Debugf(src, "Can't move - not same remote type")
		return nil, fs.ErrorCantMove
	}

	srcLeaf := f.opt.Enc.ToStandardName(srcObj.file.Name)
	_, srcDirID, err := srcObj.fs.dirCache.FindPath(ctx, srcObj.remote, false)
	if err != nil {
		return nil, err
	}

	// Find destination directory
	dstLeaf, dstDirID, err := f.dirCache.FindPath(ctx, remote, true)
	if err != nil {
		return nil, err
	}

	needsMove := srcDirID != dstDirID
	needsRename := srcLeaf != dstLeaf

	// Moving and renaming in the same request (rather than a moveFile call
	// followed by a separate renameFile call) matters when both are needed:
	// a two-step move-then-rename can transiently place the file's CURRENT
	// name into a destination folder that already holds an unrelated file
	// with that name, even though the FINAL name is free - the server only
	// allows one active name per folder, so that intermediate state 409s.
	if needsMove || needsRename {
		newName := ""
		if needsRename {
			newName = f.opt.Enc.FromStandardName(dstLeaf)
		}
		err = f.moveFile(ctx, srcObj.file.ID, dstDirID, newName)
		if err != nil {
			return nil, err
		}
	}

	// Return new object. file.Name must reflect the destination leaf, not
	// the source's - a caller that reuses this object as the src of a
	// SECOND Move (e.g. operations.RemoveExisting's restore-on-error path)
	// recomputes srcLeaf from file.Name; a stale name there makes that
	// second move see srcLeaf == dstLeaf and skip the rename entirely,
	// silently leaving the object under the wrong name.
	newFile := srcObj.file
	newFile.Name = f.opt.Enc.FromStandardName(dstLeaf)
	return &Object{
		fs:     f,
		remote: remote,
		file:   newFile,
	}, nil
}

// DirMove moves src, srcRemote to this remote at dstRemote
func (f *Fs) DirMove(ctx context.Context, src fs.Fs, srcRemote, dstRemote string) error {
	srcFs, ok := src.(*Fs)
	if !ok {
		fs.Debugf(srcFs, "Can't move directory - not same remote type")
		return fs.ErrorCantDirMove
	}

	srcID, srcDirectoryID, _, dstDirectoryID, dstLeaf, err := f.dirCache.DirMove(ctx, srcFs.dirCache, srcFs.root, srcRemote, f.root, dstRemote)
	if err != nil {
		return err
	}

	srcLeaf := srcRemote
	if idx := strings.LastIndex(srcRemote, "/"); idx >= 0 {
		srcLeaf = srcRemote[idx+1:]
	}
	needsRename := srcLeaf != dstLeaf
	newName := ""
	if needsRename {
		newName = f.opt.Enc.FromStandardName(dstLeaf)
	}

	// Move folder to new parent only if parent changed. The new name goes
	// in the same request, as for files in Move: a move followed by a
	// separate rename can collide with an unrelated folder sharing the
	// CURRENT name in the destination even though the final name is free.
	if srcDirectoryID != dstDirectoryID {
		renamed, err := f.moveFolder(ctx, srcID, dstDirectoryID, newName)
		if err != nil {
			return fmt.Errorf("couldn't move directory: %w", err)
		}
		if renamed {
			needsRename = false
		}
	}

	if needsRename {
		err = f.renameFolder(ctx, srcID, newName)
		if err != nil {
			return fmt.Errorf("couldn't rename directory: %w", err)
		}
	}

	srcFs.dirCache.FlushDir(srcRemote)
	return nil
}

// Copy src to this remote using server-side copy operations
func (f *Fs) Copy(ctx context.Context, src fs.Object, remote string) (fs.Object, error) {
	srcObj, ok := src.(*Object)
	if !ok {
		fs.Debugf(src, "Can't copy - not same remote type")
		return nil, fs.ErrorCantCopy
	}

	_, dstDirID, err := f.dirCache.FindPath(ctx, remote, true)
	if err != nil {
		return nil, err
	}

	// Remove any existing object at the destination first: the copy API
	// creates a new row named after the source file when copying across
	// folders, which collides with one already there. rclone's own Move
	// framework code (fs/operations/operations.go) applies this identical
	// delete-stale-destination-first pattern before a server-side Move;
	// server-side Copy has no such framework-level step, so it's on us.
	if existingObj, err := f.NewObject(ctx, remote); err == nil {
		if err := existingObj.Remove(ctx); err != nil {
			return nil, fmt.Errorf("couldn't remove existing destination file: %w", err)
		}
	}

	dstLeaf, _, err := f.dirCache.FindPath(ctx, remote, false)
	if err != nil {
		return nil, err
	}
	dstName := f.opt.Enc.FromStandardName(dstLeaf)

	resp, err := f.copyFile(ctx, srcObj.file.ID, dstDirID, dstName)
	if err != nil {
		return nil, fmt.Errorf("couldn't copy file: %w", err)
	}

	// Servers that predate the name field ignore it and name the copy after
	// the source, or "Copy of <original>" in the same folder. Rename to the
	// requested destination name if it differs.
	if resp.Name != dstName {
		err = f.renameFile(ctx, resp.FileID, dstName)
		if err != nil {
			return nil, fmt.Errorf("couldn't rename copied file: %w", err)
		}
		resp.Name = dstName
	}

	// A server-side copy is byte-identical to the source and the API
	// preserves the source's modification time and content hash onto the
	// copy, so the returned object must report the SOURCE's modtime and hash,
	// not the copy's wall-clock time. Reading "now" here made the object
	// rclone holds in memory disagree with the one read back from the remote
	// (its Fingerprint check compares the two exactly).
	return &Object{
		fs:     f,
		remote: remote,
		file: api.FileItem{
			ID:          resp.FileID,
			Name:        resp.Name,
			SizeBytes:   srcObj.file.SizeBytes,
			MimeType:    srcObj.file.MimeType,
			Extension:   srcObj.file.Extension,
			Region:      srcObj.file.Region,
			CreatedAt:   srcObj.file.CreatedAt,
			UpdatedAt:   srcObj.file.UpdatedAt,
			ContentHash: srcObj.file.ContentHash,
		},
	}, nil
}

// Purge deletes all the files and the container
func (f *Fs) Purge(ctx context.Context, dir string) error {
	directoryID, err := f.dirCache.FindDir(ctx, dir, false)
	if err != nil {
		return err
	}

	// DELETE /api/folders/:id recursively deletes all contents
	err = f.removeFolder(ctx, directoryID)
	if err != nil {
		return err
	}

	f.dirCache.FlushDir(dir)
	return nil
}

// ListR lists the objects and directories of the Fs starting
// from dir recursively into out.
//
// It uses the existing List method recursively to ensure paths
// are always relative to the Fs root.
func (f *Fs) ListR(ctx context.Context, dir string, callback fs.ListRCallback) error {
	var listR func(dir string) error
	listR = func(dir string) error {
		entries, err := f.List(ctx, dir)
		if err != nil {
			return err
		}
		// Collect the subdirectories BEFORE handing entries to the
		// callback. The callback is allowed to filter the slice in place
		// (walk.ListR does exactly that for ListObjects/ListDirs, via
		// ListType.Filter), which overwrites the directory entries in the
		// backing array - iterating entries afterwards would then never
		// recurse, silently truncating the listing to the top level.
		var subdirs []string
		for _, entry := range entries {
			if d, ok := entry.(fs.Directory); ok {
				subdirs = append(subdirs, d.Remote())
			}
		}
		err = callback(entries)
		if err != nil {
			return err
		}
		// Recurse into subdirectories
		for _, subdir := range subdirs {
			err = listR(subdir)
			if err != nil {
				return err
			}
		}
		return nil
	}
	return listR(dir)
}

// PublicLink generates a public link to the remote path
func (f *Fs) PublicLink(ctx context.Context, remote string, expire fs.Duration, unlink bool) (string, error) {
	o, err := f.NewObject(ctx, remote)
	if err != nil {
		return "", err
	}
	obj := o.(*Object)

	resp, err := f.createShareLink(ctx, obj.file.ID, expire)
	if err != nil {
		return "", err
	}

	return resp.Link.URL, nil
}

// About gets quota information from the Fs
func (f *Fs) About(ctx context.Context) (*fs.Usage, error) {
	info, err := f.getWorkspaceInfo(ctx)
	if err != nil {
		return nil, err
	}
	usage := &fs.Usage{
		Used:  fs.NewUsageValue(info.Storage.Used),
		Total: fs.NewUsageValue(info.Storage.Total),
		Free:  fs.NewUsageValue(info.Storage.Free),
	}
	return usage, nil
}

// ------------------------------------------------------------
// Object methods
// ------------------------------------------------------------

// String returns a description of the Object
func (o *Object) String() string {
	return o.remote
}

// Remote returns the remote path
func (o *Object) Remote() string {
	return o.remote
}

// ModTime returns the modification date of the file
func (o *Object) ModTime(ctx context.Context) time.Time {
	if o.file.UpdatedAt > 0 {
		return time.Unix(int64(o.file.UpdatedAt), 0)
	}
	return time.Unix(int64(o.file.CreatedAt), 0)
}

// Size returns the size of the file
func (o *Object) Size() int64 {
	return o.file.SizeBytes
}

// Fs returns read only access to the Fs that this object is part of
func (o *Object) Fs() fs.Info {
	return o.fs
}

// Hash returns the selected checksum of the file
func (o *Object) Hash(ctx context.Context, t hash.Type) (string, error) {
	if t != hash.SHA256 {
		return "", hash.ErrUnsupported
	}
	// May be "" - a multipart-uploaded object carries no server-side hash,
	// which rclone treats as "hash unavailable for this object" (as it does
	// for S3 multipart objects).
	return o.file.ContentHash, nil
}

// Storable says whether this object can be stored
func (o *Object) Storable() bool {
	return true
}

// SetModTime sets the metadata on the object to set the modification date
func (o *Object) SetModTime(ctx context.Context, t time.Time) error {
	return fs.ErrorCantSetModTime
}

// Open opens the file for read
func (o *Object) Open(ctx context.Context, options ...fs.OpenOption) (io.ReadCloser, error) {
	fs.FixRangeOption(options, o.file.SizeBytes)
	return o.fs.downloadFile(ctx, o.file.ID, options)
}

// Update replaces the file content
func (o *Object) Update(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) error {
	size := src.Size()
	if size < 0 {
		return errors.New("can't upload files with unknown size")
	}

	// Find the directory ID for this file
	_, directoryID, err := o.fs.dirCache.FindPath(ctx, o.remote, false)
	if err != nil {
		return err
	}

	// Upload as new version of existing file
	fileID := o.file.ID
	modTime := src.ModTime(ctx)
	resp, err := o.fs.uploadFile(ctx, in, o.remote, size, modTime, directoryID, &fileID)
	if err != nil {
		return err
	}

	// Update object metadata
	o.file = fileFromUpload(resp, modTime)

	return nil
}

// Remove removes this object
func (o *Object) Remove(ctx context.Context) error {
	return o.fs.deleteFile(ctx, o.file.ID)
}

// MimeType of an Object if known, "" otherwise
func (o *Object) MimeType(ctx context.Context) string {
	return o.file.MimeType
}

// ID returns the ID of the Object if known, or "" if not
func (o *Object) ID() string {
	return o.file.ID
}

func getRemote(dir, fileName string) string {
	if dir == "" {
		return fileName
	}
	return dir + "/" + fileName
}

// Check the interfaces are satisfied
var (
	_ fs.Fs              = (*Fs)(nil)
	_ fs.Mover           = (*Fs)(nil)
	_ fs.DirMover        = (*Fs)(nil)
	_ fs.Copier          = (*Fs)(nil)
	_ fs.Purger          = (*Fs)(nil)
	_ fs.ListRer         = (*Fs)(nil)
	_ fs.PublicLinker    = (*Fs)(nil)
	_ fs.Abouter         = (*Fs)(nil)
	_ fs.PutUncheckeder  = (*Fs)(nil)
	_ dircache.DirCacher = (*Fs)(nil)
	_ fs.Object          = (*Object)(nil)
	_ fs.MimeTyper       = (*Object)(nil)
	_ fs.IDer            = (*Object)(nil)
)
