// Package internxt provides an interface to Internxt's Drive API
package internxt

import (
	"context"
	"io"
	"path"
	"strings"
	"time"

	"github.com/StarHack/go-internxt-drive/auth"
	"github.com/StarHack/go-internxt-drive/buckets"
	config "github.com/StarHack/go-internxt-drive/config"
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
		}},
	)
}

// Options holds configuration options for this interface
type Options struct {
	Endpoint string `flag:"endpoint" help:"API endpoint"`
	Email    string `flag:"email"    help:"Internxt account email"`
	Password string `flag:"password" help:"Internxt account password"`
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
	rootIsFile     bool
	rootFile       *folders.File
	features       *fs.Features
	encoding       encoder.MultiEncoder
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

// Precision returns the precision of mtime that the server responds
func (f *Fs) Precision() time.Duration { return time.Minute }

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
		ReadMetadata:            false,
		CanHaveEmptyDirectories: false,
	})

	f.encoding = encoder.EncodeBackSlash | encoder.EncodeHash | encoder.EncodePercent

	f.dirCache = dircache.New("", cfg.RootFolderID, f)

	if root != "" {
		parent, leaf := path.Split(root)
		parent = strings.Trim(parent, "/")
		dirID, err := f.dirCache.FindDir(ctx, strings.ReplaceAll(parent, "\\", "%5C"), false)
		if err != nil {
			return nil, err
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
			if name == leaf {
				f.rootIsFile = true
				f.rootFile = &e
				break
			}
		}

		if !f.rootIsFile {
			folderID, err := f.dirCache.FindDir(ctx, strings.ReplaceAll(root, "\\", "%5C"), true)
			if err != nil {
				return nil, err
			}
			f.dirCache = dircache.New("", folderID, f)
		}
	}

	return f, nil
}

// Mkdir creates a new directory
func (f *Fs) Mkdir(ctx context.Context, dir string) error {
	dir = strings.ReplaceAll(dir, "\\", "%5C")

	id, err := f.dirCache.FindDir(ctx, dir, true)
	if err != nil {
		if strings.Contains(err.Error(), `"statusCode":400`) {
			return nil
		}
		return err
	}

	f.dirCache.Put(dir, id)
	return nil
}

// Rmdir removes a directory
func (f *Fs) Rmdir(ctx context.Context, dir string) error {
	id, err := f.dirCache.FindDir(ctx, dir, false)
	if err == fs.ErrorDirNotFound {
		if id, err = f.dirCache.FindDir(ctx, dir, true); err != nil {
			return nil
		}
	}
	if err != nil {
		return err
	}

	if id == f.cfg.RootFolderID {
		return nil
	}

	err = folders.DeleteFolder(f.cfg, id)
	if err != nil {
		if strings.Contains(err.Error(), "statusCode\":404") ||
			strings.Contains(err.Error(), "directory not found") {
			err = fs.ErrorDirNotFound
		}
		return err
	}

	f.dirCache.FlushDir(dir)
	f.dirCache.FlushDir(path.Dir(dir))
	return nil
}

// FindLeaf looks for a sub‑folder named `leaf` under the Internxt folder `pathID`.
// If found, it returns its UUID and true. If not found, returns "", false.
func (f *Fs) FindLeaf(ctx context.Context, pathID, leaf string) (string, bool, error) {
	entries, err := folders.ListFolders(f.cfg, pathID, folders.ListOptions{})
	if err != nil {
		return "", false, err
	}
	for _, e := range entries {
		if e.PlainName == leaf {
			return e.UUID, true, nil
		}
	}
	return "", false, nil
}

// CreateDir creates a new directory
func (f *Fs) CreateDir(ctx context.Context, pathID, leaf string) (string, error) {
	resp, err := folders.CreateFolder(f.cfg, folders.CreateFolderRequest{
		PlainName:        strings.ReplaceAll(leaf, "\\", "%5C"),
		ParentFolderUUID: pathID,
		ModificationTime: time.Now().UTC().Format(time.RFC3339),
	})
	if err != nil {
		return "", err
	}
	return resp.UUID, nil
}

// List lists a directory
func (f *Fs) List(ctx context.Context, dir string) (fs.DirEntries, error) {
	if f.rootIsFile && dir == "" {
		return fs.DirEntries{newObjectWithFile(f, f.root, f.rootFile)}, nil
	}
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
		remote := path.Join(dir, e.PlainName)
		remote = strings.ReplaceAll(remote, "%5C", "\\")
		f.dirCache.Put(remote, e.UUID)
		out = append(out, fs.NewDir(remote, e.ModificationTime))
	}
	filesList, err := folders.ListFiles(f.cfg, dirID, folders.ListOptions{})
	if err != nil {
		return nil, err
	}
	for _, e := range filesList {
		remote := path.Join(dir, e.PlainName)
		remote = strings.ReplaceAll(remote, "%5C", "\\")
		if len(e.Type) > 0 {
			remote += "." + e.Type
		}
		f.dirCache.Put(remote, e.UUID)
		out = append(out, newObjectWithFile(f, remote, &e))
	}
	return out, nil
}

// Put uploads a file
func (f *Fs) Put(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error) {
	remote := src.Remote()

	parentDir, fileName := path.Split(remote)
	parentDir = strings.Trim(parentDir, "/")

	parentDir = strings.ReplaceAll(parentDir, "\\", "%5C")
	folderUUID, err := f.dirCache.FindDir(ctx, parentDir, true)
	if err != nil {
		return nil, err
	}

	meta, err := buckets.UploadFileStream(f.cfg, folderUUID, fileName, in, src.Size(), src.ModTime(ctx))
	if err != nil {
		return nil, err
	}

	f.dirCache.Put(remote, meta.UUID)

	modTime := src.ModTime(ctx)

	return &Object{
		f:       f,
		remote:  remote,
		uuid:    meta.UUID,
		size:    src.Size(),
		modTime: modTime,
	}, nil
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
	remote = strings.ReplaceAll(remote, "\\", "%5C")
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
	if f.rootIsFile {
		leaf := path.Base(f.root)
		if remote == "" || remote == leaf {
			return newObjectWithFile(f, f.root, f.rootFile), nil
		}
	}
	parentDir, fileName := path.Split(remote)
	parentDir = strings.Trim(parentDir, "/")
	parentDir = strings.ReplaceAll(parentDir, "\\", "%5C")
	dirID, err := f.dirCache.FindDir(ctx, parentDir, true)
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
		if name == fileName {
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
	return buckets.DownloadFileStream(o.f.cfg, o.id)
}

// Update updates an existing file
func (o *Object) Update(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) error {
	parentDir, _ := path.Split(o.remote)
	parentDir = strings.Trim(parentDir, "/")
	parentDir = strings.ReplaceAll(parentDir, "\\", "%5C")
	folderUUID, err := o.f.dirCache.FindDir(ctx, parentDir, false)
	if err != nil {
		return err
	}

	if err := files.DeleteFile(o.f.cfg, o.uuid); err != nil {
		return err
	}

	meta, err := buckets.UploadFileStream(o.f.cfg, folderUUID, path.Base(o.remote), in, src.Size(), src.ModTime(ctx))
	if err != nil {
		return err
	}

	o.uuid = meta.UUID
	o.size = src.Size()
	o.modTime = src.ModTime(ctx)
	return nil
}

// Remove deletes a file
func (o *Object) Remove(ctx context.Context) error {
	return files.DeleteFile(o.f.cfg, o.uuid)
}
