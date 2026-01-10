// Package internxt provides an interface to Internxt's Drive API
package internxt

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/StarHack/go-internxt-drive/internxtclient"
	"github.com/rclone/rclone/fs"
	rclone_config "github.com/rclone/rclone/fs/config"
	"github.com/rclone/rclone/fs/config/configmap"
	"github.com/rclone/rclone/fs/config/configstruct"
	"github.com/rclone/rclone/fs/config/obscure"
	"github.com/rclone/rclone/fs/hash"
	"github.com/rclone/rclone/lib/dircache"
	"github.com/rclone/rclone/lib/encoder"
)

// Register with Fs
func init() {
	fs.Register(&fs.RegInfo{
		Name:        "internxt",
		Description: "internxt",
		NewFs:       NewFs,
		Options: []fs.Option{
			{
				Name:      "email",
				Default:   "",
				Help:      "The email of the user to operate as.",
				Sensitive: true,
			},
			{
				Name:       "password",
				Default:    "",
				Help:       "The password for the user.",
				IsPassword: true,
			},
			{
				Name:    "simulateEmptyFiles",
				Default: false,
				Help:    "Simulates empty files by uploading a small placeholder file instead. Alters the filename when uploading to keep track of empty files, but this is not visible through rclone.",
			},
			{
				Name:    "use_2fa",
				Help:    "Do you use 2FA to login?",
				Default: false,
			},
			{
				Name:     rclone_config.ConfigEncoding,
				Help:     rclone_config.ConfigEncodingHelp,
				Advanced: true,

				Default: encoder.EncodeInvalidUtf8 |
					encoder.EncodeSlash |
					encoder.EncodeBackSlash |
					encoder.EncodeRightPeriod |
					encoder.EncodeDot,
			},
		}},
	)
}

const (
	EMPTY_FILE_EXT = ".__RCLONE_EMPTY__"
)

var (
	EMPTY_FILE_BYTES = []byte{0x13, 0x09, 0x20, 0x23}
)

// Options holds configuration options for this interface
type Options struct {
	Endpoint           string               `flag:"endpoint" help:"API endpoint"`
	Email              string               `flag:"email"    help:"Internxt account email"`
	Password           string               `flag:"password" help:"Internxt account password"`
	Encoding           encoder.MultiEncoder `config:"encoding"`
	SimulateEmptyFiles bool                 `config:"simulateEmptyFiles"`
	Use2FA             bool                 `config:"use_2fa" help:"Do you use 2FA to login?"`
}

// Fs represents an Internxt remote
type Fs struct {
	name     string
	root     string
	opt      Options
	dirCache *dircache.DirCache
	client   *internxtclient.Client
	features *fs.Features
}

// Object holds the data for a remote file object
type Object struct {
	f       *Fs
	remote  string
	id      string
	uuid    string
	size    int64
	modTime time.Time
}

// Name of the remote (as passed into NewFs)
func (f *Fs) Name() string { return f.name }

// Root of the remote (as passed into NewFs)
func (f *Fs) Root() string { return f.root }

// String converts this Fs to a string
func (f *Fs) String() string { return f.name + ":" + f.root }

// Features returns the optional features of this Fs
func (f *Fs) Features() *fs.Features {
	return f.features
}

// Hashes returns type of hashes supported by Internxt
func (f *Fs) Hashes() hash.Set {
	return hash.NewHashSet()
}

// Precision return the precision of this Fs
func (f *Fs) Precision() time.Duration {
	//return time.Minute
	return fs.ModTimeNotSupported
}

// NewFs constructs an Fs from the path
func NewFs(ctx context.Context, name, root string, m configmap.Mapper) (fs.Fs, error) {
	opt := new(Options)
	if err := configstruct.Set(m, opt); err != nil {
		return nil, err
	}
	clearPassword, err := obscure.Reveal(opt.Password)
	if err != nil {
		return nil, err
	}

	// Create the client with credentials
	client, err := internxtclient.NewWithCredentials(opt.Email, clearPassword)
	if err != nil {
		return nil, err
	}

	if opt.Use2FA {
		// TODO: Implement 2FA support with the new client API
		// The new client branch may handle 2FA differently
		return nil, fmt.Errorf("2FA is not yet implemented with the new client API")
	}

	f := &Fs{
		name:   name,
		root:   root,
		opt:    *opt,
		client: client,
	}

	f.features = (&fs.Features{
		ReadMimeType:             false,
		WriteMimeType:            false,
		BucketBased:              false,
		BucketBasedRootOK:        false,
		WriteDirSetModTime:       false,
		WriteMetadata:            false,
		WriteDirMetadata:         false,
		ReadMetadata:             false,
		CanHaveEmptyDirectories:  true,
		IsLocal:                  false,
		DirModTimeUpdatesOnWrite: false,
	}).Fill(ctx, f)

	// Handle leading and trailing slashes
	root = strings.Trim(root, "/")
	rootFolderID := ""
	if client.UserData != nil && client.UserData.AccessData != nil && client.UserData.AccessData.User != nil {
		rootFolderID = client.UserData.AccessData.User.RootFolderUUID
	}
	f.dirCache = dircache.New(root, rootFolderID, f)

	err = f.dirCache.FindRoot(ctx, false)
	if err != nil {

		// Assume it is a file
		newRoot, remote := dircache.SplitPath(root)

		tempF := *f
		tempF.dirCache = dircache.New(newRoot, rootFolderID, &tempF)
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

// Mkdir creates a new directory
func (f *Fs) Mkdir(ctx context.Context, dir string) error {
	id, err := f.dirCache.FindDir(ctx, dir, true)
	if err != nil {
		return err
	}

	f.dirCache.Put(dir, id)            // Is this done automatically by FindDir with create == true?
	time.Sleep(500 * time.Millisecond) //REMOVE THIS, use pacer to check for consistency?

	return nil
}

// Rmdir removes a directory
// Returns an error if it isn't empty
func (f *Fs) Rmdir(ctx context.Context, dir string) error {
	return f.purgeCheck(ctx, dir, true)
}

// Purge deletes the directory and all its contents
func (f *Fs) Purge(ctx context.Context, dir string) error {
	return f.purgeCheck(ctx, dir, false)
}

func (f *Fs) purgeCheck(ctx context.Context, dir string, check bool) (err error) {
	root := path.Join(f.root, dir)
	if root == "" {
		return errors.New("can't purge root directory")
	}

	// check that the directory exists
	id, err := f.dirCache.FindDir(ctx, dir, false)
	if err != nil {
		return fs.ErrorDirNotFound
	}

	if check {
		// Check folders and files separately in case we only need to call the API once.
		childFolders, err := f.client.Folders.ListAllFolders(id)
		if err != nil {
			return err
		}

		if len(childFolders) > 0 {
			return fs.ErrorDirectoryNotEmpty
		}

		childFiles, err := f.client.Folders.ListAllFiles(id)
		if err != nil {
			return err
		}

		if len(childFiles) > 0 {
			return fs.ErrorDirectoryNotEmpty
		}
	}

	err = f.client.Folders.DeleteFolder(id)
	if err != nil {
		if strings.Contains(err.Error(), "404") {
			return fs.ErrorDirNotFound
		}
		return err
	}

	f.dirCache.FlushDir(dir)
	time.Sleep(500 * time.Millisecond) // REMOVE THIS, use pacer to check for consistency?
	return nil

}

// FindLeaf looks for a sub‑folder named `leaf` under the Internxt folder `pathID`.
// If found, it returns its UUID and true. If not found, returns "", false.
func (f *Fs) FindLeaf(ctx context.Context, pathID, leaf string) (string, bool, error) {
	//fmt.Printf("FindLeaf pathID: %s, leaf: %s\n", pathID, leaf)
	entries, err := f.client.Folders.ListAllFolders(pathID)
	if err != nil {
		return "", false, err
	}
	for _, e := range entries {
		if f.opt.Encoding.ToStandardName(e.PlainName) == leaf {
			return e.UUID, true, nil
		}
	}
	return "", false, nil
}

// CreateDir creates a new directory
func (f *Fs) CreateDir(ctx context.Context, pathID, leaf string) (string, error) {
	resp, err := f.client.Folders.CreateFolder(internxtclient.CreateFolderRequest{
		PlainName:        f.opt.Encoding.FromStandardName(leaf),
		ParentFolderUUID: pathID,
		ModificationTime: time.Now().UTC().Format(time.RFC3339),
	})
	if err != nil {
		return "", fmt.Errorf("can't create folder, %w", err)
	}

	time.Sleep(500 * time.Millisecond) // REMOVE THIS, use pacer to check for consistency?
	return resp.UUID, nil
}

// List lists a directory
func (f *Fs) List(ctx context.Context, dir string) (fs.DirEntries, error) {
	dirID, err := f.dirCache.FindDir(ctx, dir, false)
	if err != nil {
		return nil, err
	}
	var out fs.DirEntries

	foldersList, err := f.client.Folders.ListAllFolders(dirID)
	if err != nil {
		return nil, err
	}
	for _, e := range foldersList {
		remote := filepath.Join(dir, f.opt.Encoding.ToStandardName(e.PlainName))
		out = append(out, fs.NewDir(remote, e.ModificationTime))
	}
	filesList, err := f.client.Folders.ListAllFiles(dirID)
	if err != nil {
		return nil, err
	}
	for _, e := range filesList {
		remote := e.PlainName
		if len(e.Type) > 0 {
			remote += "." + e.Type
		}
		remote = filepath.Join(dir, f.opt.Encoding.ToStandardName(remote))
		// If we found a file with the special empty file suffix, pretend that it's empty
		if f.opt.SimulateEmptyFiles && strings.HasSuffix(remote, EMPTY_FILE_EXT) {
			remote = strings.TrimSuffix(remote, EMPTY_FILE_EXT)
			e.Size = "0"
		}
		out = append(out, newObjectWithFile(f, remote, &e))
	}
	return out, nil
}

// Put uploads a file
func (f *Fs) Put(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error) {
	o := &Object{
		f:       f,
		remote:  src.Remote(),
		size:    src.Size(),
		modTime: src.ModTime(ctx),
	}

	err := o.Update(ctx, in, src, options...)
	if err != nil {
		return nil, err
	}

	return o, nil
}

// Remove removes an object
func (f *Fs) Remove(ctx context.Context, remote string) error {
	obj, err := f.NewObject(ctx, remote)
	if err == nil {
		if err := obj.Remove(ctx); err != nil {
			return err
		}
		parent := path.Dir(remote)
		f.dirCache.FlushDir(parent)
		return nil
	}

	dirID, err := f.dirCache.FindDir(ctx, remote, false)
	if err != nil {
		return err
	}
	if err := f.client.Folders.DeleteFolder(dirID); err != nil {
		return err
	}
	f.dirCache.FlushDir(remote)
	return nil
}

// Move src to this remote using server-side move operations.
func (f *Fs) Move(ctx context.Context, src fs.Object, remote string) (fs.Object, error) {
	srcObj, ok := src.(*Object)
	if !ok {
		fs.Debugf(src, "Can't move - not same remote type")
		return nil, fs.ErrorCantMove
	}

	srcLeaf, srcDirectoryID, err := f.dirCache.FindPath(ctx, srcObj.remote, false)
	if err != nil {
		return nil, err
	}

	dstLeaf, dstDirectoryID, err := f.dirCache.FindPath(ctx, remote, true)
	if err != nil {
		return nil, err
	}

	doMove := srcDirectoryID != dstDirectoryID
	doRename := srcLeaf != dstLeaf

	var dstObj fs.Object

	// If we're doing both, we should rename to a temp name in case there's a file
	// with the same name at the destination folder (we can't rename AND move with one call)
	if doMove && doRename {
		newFile, err := f.client.Files.UpdateFileMeta(srcObj.uuid, &internxtclient.File{Type: "__RCLONE_MOVE__"})
		if err != nil {
			return nil, err
		}
		time.Sleep(500 * time.Millisecond) //Find a way around this
		dstObj = newObjectWithFile(f, remote, newFile)
	}

	if doMove {
		newFile, err := f.client.Files.MoveFile(srcObj.uuid, dstDirectoryID)
		if err != nil {
			return nil, err
		}
		time.Sleep(500 * time.Millisecond) //Find a way around this
		dstObj = newObjectWithFile(f, remote, newFile)
	}

	if doRename {
		base := filepath.Base(remote)
		name := strings.TrimSuffix(base, filepath.Ext(base))
		ext := strings.TrimPrefix(filepath.Ext(base), ".")

		updated := &internxtclient.File{
			PlainName: f.opt.Encoding.FromStandardName(name),
			Type:      f.opt.Encoding.FromStandardName(ext),
		}

		newFile, err := f.client.Files.UpdateFileMeta(srcObj.uuid, updated)
		if err != nil {
			return nil, err
		}
		time.Sleep(500 * time.Millisecond) //Find a way around this
		dstObj = newObjectWithFile(f, remote, newFile)
	}

	return dstObj, nil
}

// Move dir to destination using server-side move operations.
func (f *Fs) DirMove(ctx context.Context, src fs.Fs, srcRemote, dstRemote string) error {
	srcFs, ok := src.(*Fs)
	if !ok {
		fs.Debugf(srcFs, "Can't move directory - not same remote type")
		return fs.ErrorCantDirMove
	}

	srcID, _, srcLeaf, dstDirectoryID, dstLeaf, err := f.dirCache.DirMove(ctx, srcFs.dirCache, srcFs.root, srcRemote, f.root, dstRemote)
	if err != nil {
		return err
	}

	doMove := srcID != dstDirectoryID
	doRename := srcLeaf != dstLeaf

	// If we're moving AND renaming we need to set a temp name first, else we risk collisions
	if doMove && doRename {
		err = f.client.Folders.RenameFolder(srcID, f.opt.Encoding.FromStandardName(dstLeaf+".__RCLONE_MOVE__"))
		if err != nil {
			return err
		}
		time.Sleep(500 * time.Millisecond) //Find a way around this
	}

	if doMove {
		err = f.client.Folders.MoveFolder(srcID, dstDirectoryID)
		if err != nil && !strings.Contains(err.Error(), "409") {
			return err
		}
		time.Sleep(500 * time.Millisecond) //Find a way around this

	}

	if doRename {
		err = f.client.Folders.RenameFolder(srcID, f.opt.Encoding.FromStandardName(dstLeaf))
		if err != nil && !strings.Contains(err.Error(), "409") {
			return err
		}
		time.Sleep(500 * time.Millisecond) //Find a way around this
	}

	srcFs.dirCache.FlushDir(srcRemote)
	return nil
}

// Copy copies a directory (not implemented)
func (f *Fs) Copy(ctx context.Context, src, dst fs.Object) error {
	// return f.client.Copy(ctx, f.root+src.Remote(), f.root+dst.Remote())
	return fs.ErrorCantCopy
}

// NewObject creates a new object
func (f *Fs) NewObject(ctx context.Context, remote string) (fs.Object, error) {
	parentDir := filepath.Dir(remote)

	//Is this needed?
	if parentDir == "." {
		parentDir = ""
	}

	dirID, err := f.dirCache.FindDir(ctx, parentDir, false)
	if err != nil {
		return nil, fs.ErrorObjectNotFound
	}

	files, err := f.client.Folders.ListAllFiles(dirID)
	if err != nil {
		return nil, err
	}
	for _, e := range files {
		name := e.PlainName
		if len(e.Type) > 0 {
			name += "." + e.Type
		}
		if f.opt.Encoding.ToStandardName(name) == filepath.Base(remote) {
			return newObjectWithFile(f, remote, &e), nil
		}
		// If we are simulating empty files, check for a file with the special suffix and if found return it as if empty.
		if f.opt.SimulateEmptyFiles {
			if f.opt.Encoding.ToStandardName(name) == filepath.Base(remote+EMPTY_FILE_EXT) {
				e.Size = "0"
				return newObjectWithFile(f, remote, &e), nil
			}
		}
	}
	return nil, fs.ErrorObjectNotFound
}

// newObjectWithFile returns a new object by file info
func newObjectWithFile(f *Fs, remote string, file *internxtclient.File) fs.Object {
	size, _ := file.Size.Int64()
	return &Object{
		f:       f,
		remote:  remote,
		id:      file.FileID,
		uuid:    file.UUID,
		size:    size,
		modTime: file.ModificationTime,
	}
}

// Fs returns the parent Fs
func (o *Object) Fs() fs.Info {
	return o.f
}

// String returns the remote path
func (o *Object) String() string {
	return o.remote
}

// Remote returns the remote path
func (o *Object) Remote() string {
	return o.remote
}

// Size is the file length
func (o *Object) Size() int64 {
	return o.size
}

// ModTime is the last modified time (read-only)
func (o *Object) ModTime(ctx context.Context) time.Time {
	return o.modTime
}

// Hash returns the hash value (not implemented)
func (o *Object) Hash(ctx context.Context, t hash.Type) (string, error) {
	return "", hash.ErrUnsupported
}

// Storable returns if this object is storable
func (o *Object) Storable() bool {
	return true
}

// SetModTime sets the modified time
func (o *Object) SetModTime(ctx context.Context, t time.Time) error {
	return fs.ErrorCantSetModTime
}

// About gets quota information
func (f *Fs) About(ctx context.Context) (*fs.Usage, error) {
	internxtLimit, err := f.client.Users.GetLimit()
	if err != nil {
		return nil, err
	}

	internxtUsage, err := f.client.Users.GetUsage()
	if err != nil {
		return nil, err
	}

	usage := &fs.Usage{
		Used: fs.NewUsageValue(internxtUsage.Drive),
	}

	usage.Total = fs.NewUsageValue(internxtLimit.MaxSpaceBytes)
	usage.Free = fs.NewUsageValue(*usage.Total - *usage.Used)

	return usage, nil
}

// Open opens a file for streaming
func (o *Object) Open(ctx context.Context, options ...fs.OpenOption) (io.ReadCloser, error) {
	fs.FixRangeOption(options, o.size)
	rangeValue := ""
	for _, option := range options {
		switch option.(type) {
		case *fs.RangeOption, *fs.SeekOption:
			_, rangeValue = option.Header()
		}
	}

	// Return nothing if we're faking an empty file
	if o.f.opt.SimulateEmptyFiles && o.size == 0 {
		return io.NopCloser(bytes.NewReader(nil)), nil
	}
	return o.f.client.Buckets.DownloadFileStream(o.id, rangeValue)
}

// Update updates an existing file
func (o *Object) Update(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) error {
	isEmptyFile := false
	if src.Size() == 0 {
		if !o.f.opt.SimulateEmptyFiles {
			return fs.ErrorCantUploadEmptyFiles
		} else {
			// If we're faking an empty file, write some nonsense into it and give it a special suffix
			isEmptyFile = true
			in = bytes.NewReader(EMPTY_FILE_BYTES)
			src = &Object{
				f:       o.f,
				remote:  src.Remote() + EMPTY_FILE_EXT,
				modTime: src.ModTime(ctx),
				size:    int64(len(EMPTY_FILE_BYTES)),
			}
			o.remote = o.remote + EMPTY_FILE_EXT
		}
	} else {
		if o.f.opt.SimulateEmptyFiles {
			// Remove the suffix if we're updating an empty file with actual data
			o.remote = strings.TrimSuffix(o.remote, EMPTY_FILE_EXT)
		}
	}

	// Check if object exists on the server
	existsInBackend := true
	if o.uuid == "" {
		objectInBackend, err := o.f.NewObject(ctx, src.Remote())
		if err != nil {
			existsInBackend = false
		} else {
			// If the object already exists, use the object from the server
			if objectInBackend, ok := objectInBackend.(*Object); ok {
				o = objectInBackend
			}
		}
	}

	if o.uuid != "" || existsInBackend {
		if err := o.f.client.Files.DeleteFile(o.uuid); err != nil {
			return fs.ErrorNotAFile
		}
	}

	// Create folder if it doesn't exist
	_, dirID, err := o.f.dirCache.FindPath(ctx, o.remote, true)
	if err != nil {
		return err
	}

	meta, err := o.f.client.Buckets.UploadFileStream(dirID, o.f.opt.Encoding.FromStandardName(filepath.Base(o.remote)), in, src.Size(), src.ModTime(ctx))
	if err != nil {
		return err
	}

	// Update the object with the new info
	o.uuid = meta.UUID
	o.size = src.Size()
	// If this is a simulated empty file set fake size to 0
	if isEmptyFile {
		o.size = 0
	}
	return nil
}

// Remove deletes a file
func (o *Object) Remove(ctx context.Context) error {
	err := o.f.client.Files.DeleteFile(o.uuid)
	time.Sleep(500 * time.Millisecond) // REMOVE THIS, use pacer to check for consistency?
	return err
}
