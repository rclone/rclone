// Package internxt provides an interface to Internxt's Drive API
package internxt

import (
	"context"
	"fmt"
	"io"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/StarHack/go-internxt-drive/auth"
	"github.com/StarHack/go-internxt-drive/buckets"
	config "github.com/StarHack/go-internxt-drive/config"
	rclone_config "github.com/rclone/rclone/fs/config"

	"github.com/StarHack/go-internxt-drive/files"
	"github.com/StarHack/go-internxt-drive/folders"
	"github.com/rclone/rclone/fs"
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

// Options holds configuration options for this interface
type Options struct {
	Endpoint string               `flag:"endpoint" help:"API endpoint"`
	Email    string               `flag:"email"    help:"Internxt account email"`
	Password string               `flag:"password" help:"Internxt account password"`
	Encoding encoder.MultiEncoder `config:"encoding"`
}

// Fs represents an Internxt remote
type Fs struct {
	name           string
	root           string
	opt            Options
	dirCache       *dircache.DirCache
	cfg            *config.Config
	loginResponse  *auth.LoginResponse
	accessResponse *auth.AccessResponse
	features       *fs.Features
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
	cfg := config.NewDefault(opt.Email, clearPassword)
	loginResponse, err := auth.Login(cfg)
	if err != nil {
		return nil, err
	}
	accessResponse, err := auth.AccessLogin(cfg, loginResponse)
	if err != nil {
		return nil, err
	}

	f := &Fs{
		name:           name,
		root:           root,
		opt:            *opt,
		cfg:            cfg,
		loginResponse:  loginResponse,
		accessResponse: accessResponse,
	}

	f.features = (&fs.Features{
		ReadMimeType:      false,
		WriteMimeType:     false,
		BucketBased:       false,
		BucketBasedRootOK: false,
		//ChunkWriterDoesntSeek:    false,
		WriteDirSetModTime:       false,
		WriteMetadata:            false,
		WriteDirMetadata:         false,
		ReadMetadata:             false,
		CanHaveEmptyDirectories:  true,
		IsLocal:                  false,
		DirModTimeUpdatesOnWrite: false,
	}).Fill(ctx, f)

	f.dirCache = dircache.New(root, cfg.RootFolderID, f)

	err = f.dirCache.FindRoot(ctx, false)
	if err != nil {

		// Assume it is a file
		newRoot, remote := dircache.SplitPath(root)

		tempF := *f
		tempF.dirCache = dircache.New(newRoot, f.cfg.RootFolderID, &tempF)
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
func (f *Fs) Rmdir(ctx context.Context, dir string) error {
	id, err := f.dirCache.FindDir(ctx, dir, false)
	if err != nil {
		return fs.ErrorDirNotFound
	}

	// Replace these calls with GetFolderContent? (fmt.Sprintf("/storage/v2/folder/%d%s", folderID, query))
	childFolders, err := folders.ListFolders(f.cfg, id, folders.ListOptions{})
	if err != nil {
		return err
	}
	childFiles, err := folders.ListFiles(f.cfg, id, folders.ListOptions{})
	if err != nil {
		return err
	}

	if len(childFiles) > 0 || len(childFolders) > 0 {
		return fs.ErrorDirectoryNotEmpty
	}

	err = folders.DeleteFolder(f.cfg, id)
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
	entries, err := folders.ListFolders(f.cfg, pathID, folders.ListOptions{})
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
	resp, err := folders.CreateFolder(f.cfg, folders.CreateFolderRequest{
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

	foldersList, err := folders.ListFolders(f.cfg, dirID, folders.ListOptions{})
	if err != nil {
		return nil, err
	}
	for _, e := range foldersList {
		remote := filepath.Join(dir, f.opt.Encoding.ToStandardName(e.PlainName))
		out = append(out, fs.NewDir(remote, e.ModificationTime))
	}
	filesList, err := folders.ListFiles(f.cfg, dirID, folders.ListOptions{})
	if err != nil {
		return nil, err
	}
	for _, e := range filesList {
		remote := e.PlainName
		if len(e.Type) > 0 {
			remote += "." + e.Type
		}
		remote = filepath.Join(dir, f.opt.Encoding.ToStandardName(remote))
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
	if err := folders.DeleteFolder(f.cfg, dirID); err != nil {
		return err
	}
	f.dirCache.FlushDir(remote)
	return nil
}

// Move moves a directory (not implemented)
func (f *Fs) Move(ctx context.Context, src, dst fs.Object) error {
	// return f.client.Rename(ctx, f.root+src.Remote(), f.root+dst.Remote())
	return nil
}

// Copy copies a directory (not implemented)
func (f *Fs) Copy(ctx context.Context, src, dst fs.Object) error {
	// return f.client.Copy(ctx, f.root+src.Remote(), f.root+dst.Remote())
	return nil
}

// DirCacheFlush flushes the dir cache (not implemented)
func (f *Fs) DirCacheFlush(ctx context.Context) {}

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

	files, err := folders.ListFiles(f.cfg, dirID, folders.ListOptions{})
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
	}
	return nil, fs.ErrorObjectNotFound
}

// newObjectWithFile returns a new object by file info
func newObjectWithFile(f *Fs, remote string, file *folders.File) fs.Object {
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

	return buckets.DownloadFileStream(o.f.cfg, o.id, rangeValue)
}

// Update updates an existing file
func (o *Object) Update(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) error {
	if src.Size() == 0 {
		return fs.ErrorCantUploadEmptyFiles
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
		if err := files.DeleteFile(o.f.cfg, o.uuid); err != nil {
			return fs.ErrorNotAFile
		}
	}

	// Create folder if it doesn't exist
	_, dirID, err := o.f.dirCache.FindPath(ctx, o.Remote(), true)
	if err != nil {
		return err
	}

	meta, err := buckets.UploadFileStream(o.f.cfg, dirID, o.f.opt.Encoding.FromStandardName(filepath.Base(o.remote)), in, src.Size(), src.ModTime(ctx))
	if err != nil {
		return err
	}

	// Update the object with the new info
	o.uuid = meta.UUID
	o.size = src.Size()
	return nil
}

// Remove deletes a file
func (o *Object) Remove(ctx context.Context) error {
	err := files.DeleteFile(o.f.cfg, o.uuid)
	time.Sleep(500 * time.Millisecond) // REMOVE THIS, use pacer to check for consistency?
	return err
}
