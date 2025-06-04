package internxt

import (
	"context"
	"fmt"
	"io"
	"path"
	"strings"
	"time"

	"github.com/StarHack/go-internxt-drive/auth"
	"github.com/StarHack/go-internxt-drive/buckets"
	config "github.com/StarHack/go-internxt-drive/config"
	"github.com/StarHack/go-internxt-drive/folders"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config/configmap"
	"github.com/rclone/rclone/fs/config/configstruct"
	"github.com/rclone/rclone/fs/config/obscure"
	"github.com/rclone/rclone/fs/hash"
	"github.com/rclone/rclone/lib/dircache"
)

// Options holds configuration options for this interface
type Options struct {
	Endpoint string `flag:"endpoint" help:"API endpoint"`
	Email    string `flag:"email"    help:"Internxt account email"`
	Password string `flag:"password" help:"Internxt account password"`
}

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
		}})
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

	f.dirCache = dircache.New("", cfg.RootFolderID, f)

	if root != "" {
		parent, leaf := path.Split(root)
		parent = strings.Trim(parent, "/")
		dirID, err := f.dirCache.FindDir(ctx, parent, false)
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
			folderID, err := f.dirCache.FindDir(ctx, root, true)
			if err != nil {
				return nil, err
			}
			f.dirCache = dircache.New("", folderID, f)
		}
	}

	return f, nil
}

// Name of the remote (as passed into NewFs)
func (f *Fs) Name() string { return f.name }

// Root of the remote (as passed into NewFs)
func (f *Fs) Root() string { return f.root }

// String converts this Fs to a string
func (f *Fs) String() string { return f.name + ":" + f.root }

// Precision returns the precision of mtime that the server responds
func (f *Fs) Precision() time.Duration { return time.Microsecond }

// Features returns the optional features of this Fs
func (f *Fs) Features() *fs.Features {
	return &fs.Features{ReadMetadata: false, CanHaveEmptyDirectories: true}
}

// Mkdir creates a new directory
func (f *Fs) Mkdir(ctx context.Context, dir string) error {
	_, err := f.dirCache.FindDir(ctx, dir, true)
	if err != nil && strings.Contains(err.Error(), `"statusCode":400`) {
		return nil
	}
	return err
}

// Rmdir removes a directory
func (f *Fs) Rmdir(ctx context.Context, dir string) error {
	id, err := f.dirCache.FindDir(ctx, dir, false)
	if err != nil {
		return err
	}

	fmt.Println(id)

	if id == f.cfg.RootFolderID {
		return fs.ErrorDirNotFound
	}

	if err := folders.DeleteFolder(f.cfg, id); err != nil {
		return err
	}

	f.dirCache.FlushDir(dir)
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
		PlainName:        leaf,
		ParentFolderUUID: pathID,
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
		f.dirCache.Put(remote, e.UUID)
		out = append(out, fs.NewDir(remote, e.ModificationTime))
	}
	filesList, err := folders.ListFiles(f.cfg, dirID, folders.ListOptions{})
	if err != nil {
		return nil, err
	}
	for _, e := range filesList {
		remote := path.Join(dir, e.PlainName)
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

	folderUUID, err := f.dirCache.FindDir(ctx, parentDir, true)

	if err != nil {
		return nil, err
	}

	meta, err := buckets.UploadFileStream(f.cfg, folderUUID, fileName, in, src.Size())
	if err != nil {
		return nil, err
	}

	f.dirCache.Put(remote, meta.UUID)

	return newObjectWithMetaFile(f, remote, meta), nil
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

// Hashes returns type of hashes supported by Internxt
func (f *Fs) Hashes() hash.Set {
	return hash.NewHashSet()
}

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
	dirID, err := f.dirCache.FindDir(ctx, parentDir, false)
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
		if name == fileName {
			return newObjectWithFile(f, remote, &e), nil
		}
	}
	return nil, fs.ErrorObjectNotFound
}
