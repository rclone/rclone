package namecrane

import (
	"context"
	"errors"
	"fmt"
	"github.com/rclone/rclone/fs/config/configstruct"
	"github.com/rclone/rclone/fs/config/obscure"
	"io"
	"net/http"
	"path"
	"strings"
	"time"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config/configmap"
	"github.com/rclone/rclone/fs/hash"
)

/**
 * Namecrane Storage Backend
 * Copyright (c) 2025 Namecrane LLC
 * PSA: No cranes harmed in the development of this module.
 */

var (
	ErrEmptyDirectory = errors.New("directory name cannot be empty")
)

type Fs struct {
	name        string
	root        string
	features    *fs.Features
	client      *Namecrane
	apiURL      string
	authManager *AuthManager
}

type Object struct {
	fs     *Fs
	file   *File
	folder *Folder
	remote string
}

type Directory struct {
	*Object
}

type Options struct {
	ApiURL   string `config:"api_url"`
	Username string `config:"username"`
	Password string `config:"password"`
}

func init() {
	fs.Register(&fs.RegInfo{
		Name:        "namecrane",
		Description: "Namecrane File Storage API Backend",
		NewFs:       NewFs,
		Options: []fs.Option{{
			Name:      "api_url",
			Help:      `Namecrane API URL, like https://us1.workspace.org`,
			Default:   "https://us1.workspace.org",
			Sensitive: true,
		}, {
			Name:      "username",
			Help:      `Namecrane username`,
			Required:  true,
			Sensitive: true,
		}, {
			Name:       "password",
			Help:       `Namecrane password`,
			Required:   true,
			IsPassword: true,
		}},
	})
}

func NewFs(ctx context.Context, name, root string, m configmap.Mapper) (fs.Fs, error) {
	opt := new(Options)

	if err := configstruct.Set(m, opt); err != nil {
		return nil, err
	}

	pass, err := obscure.Reveal(opt.Password)

	if err != nil {
		return nil, fmt.Errorf("NewFS decrypt password: %w", err)
	}

	opt.Password = pass

	if root == "" || root == "." {
		root = "/"
	}

	authManager := NewAuthManager(http.DefaultClient, opt.ApiURL, opt.Username, opt.Password)

	if _, err := authManager.GetToken(ctx); err != nil {
		return nil, err
	}

	client := NewClient(opt.ApiURL, authManager)

	f := &Fs{
		name:        name,
		root:        root,
		client:      client,
		apiURL:      opt.ApiURL,
		authManager: authManager,
	}

	// Validate that the root is a directory, not a file
	_, file, err := client.Find(ctx, root)

	// Ignore ErrNoFile as rclone will create directories for us
	if err != nil && !errors.Is(err, ErrNoFile) {
		return nil, err
	}

	if file != nil {
		// Path is a file, not a folder. Set the root to the folder and return a special error.
		f.root = file.FolderPath

		return f, fs.ErrorIsFile
	}

	return f, nil
}

func (f *Fs) Name() string {
	return f.name
}

func (f *Fs) Root() string {
	return f.root
}

func (f *Fs) String() string {
	if f.root == "" {
		return fmt.Sprintf("NameCrane backend at %s", f.apiURL)
	}

	return fmt.Sprintf("NameCrane backend at %s, root '%s'", f.apiURL, f.root)
}

func (f *Fs) Features() *fs.Features {
	if f.features == nil {
		f.features = (&fs.Features{
			CanHaveEmptyDirectories: true,
		}).Fill(context.Background(), f)
	}
	return f.features
}

func (f *Fs) Rmdir(ctx context.Context, dir string) error {
	normalizedDir := path.Join(f.root, path.Clean(dir))

	if normalizedDir == "" {
		return ErrEmptyDirectory
	}

	// Normalize the directory path
	err := f.client.DeleteFolder(ctx, normalizedDir)

	if err != nil {
		return err
	}

	fs.Debugf(f, "Successfully removed directory '%s'", normalizedDir)
	return nil
}

func (f *Fs) Mkdir(ctx context.Context, dir string) error {
	normalizedDir := path.Join(f.root, path.Clean(dir))

	if normalizedDir == "" {
		return ErrEmptyDirectory
	}

	res, err := f.client.CreateFolder(ctx, normalizedDir)

	if err != nil {
		return err
	}

	fs.Debugf(f, "Successfully created directory '%s'", res.Path)
	return nil
}

func (f *Fs) Stat(ctx context.Context, remote string) (fs.DirEntry, error) {
	// Fetch the folder path and file name from the remote path
	dir, fileName := path.Split(remote)

	// Prepend root path
	dir = path.Join(f.root, dir)

	if dir == "" || dir[0] != '/' {
		dir = "/" + dir
	}

	fs.Debugf(f, "Stat file at %s: %s -> %s", remote, dir, fileName)

	id, err := f.client.GetFileID(ctx, dir, fileName)

	if err != nil {
		return nil, err
	}

	files, err := f.client.GetFiles(ctx, id)

	if err != nil {
		return nil, err
	}

	file := files[0]

	return &Object{
		fs:   f,
		file: &file,
	}, nil
}

func (f *Fs) Hashes() hash.Set {
	// Return the hash types supported by the backend.
	// If no hashing is supported, return hash.None.
	return hash.NewHashSet()
}

func (f *Fs) List(ctx context.Context, dir string) (fs.DirEntries, error) {
	remote := path.Join(f.root, dir)

	if remote == "" || remote[0] != '/' {
		remote = "/" + remote
	}

	fs.Debugf(f, "List contents of %s: %s", dir, remote)

	// If the path is a subdirectory, use GetFolder instead of GetFolders
	if remote != "/" {
		fs.Debugf(f, "Listing files in non-root directory %s", remote)

		folder, err := f.client.GetFolder(ctx, remote)

		if err != nil {
			if errors.Is(err, ErrNoFolder) {
				return nil, fs.ErrorDirNotFound
			}

			fs.Errorf(f, "Unable to find directory %s", remote)
			return nil, err
		}

		return f.folderToEntries(*folder), nil
	}

	root, err := f.client.GetFolders(ctx)

	if err != nil {
		return nil, err
	}

	// root[0] is always the root folder
	return f.folderToEntries(root[0]), nil
}

func (f *Fs) folderToEntries(folder Folder) fs.DirEntries {
	var entries fs.DirEntries

	for _, file := range folder.Files {
		entries = append(entries, &Object{
			fs:   f,
			file: &file,
		})
	}

	for _, subfolder := range folder.Subfolders {
		entries = append(entries, &Directory{
			Object: &Object{
				fs:     f,
				folder: &subfolder,
			},
		})
	}

	return entries
}

func (f *Fs) Sortable() bool {
	return false
}

func (f *Fs) NewObject(ctx context.Context, remote string) (fs.Object, error) {
	fs.Debugf(f, "New object %s", remote)

	folder, file, err := f.client.Find(ctx, remote)

	if err != nil {
		fs.Debugf(f, "Unable to find existing file, not necessarily a bad thing: %s", err.Error())
	}

	return &Object{
		fs:     f,
		remote: remote,
		folder: folder,
		file:   file,
	}, nil
}

func (f *Fs) Put(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error) {
	remote := src.Remote()

	remote = path.Join(f.root, remote)

	if remote[0] != '/' {
		remote = "/" + remote
	}

	fs.Debugf(f, "Put contents of %s to %s", src.Remote(), remote)

	file, err := f.client.Upload(ctx, in, remote, src.Size())

	if err != nil {
		return nil, err
	}

	// Return the uploaded object
	return &Object{
		fs:   f,
		file: file,
	}, nil
}

func (o *Object) ModTime(ctx context.Context) time.Time {
	if o.file != nil {
		return o.file.DateAdded
	}

	return time.Time{}
}

func (o *Object) Fs() fs.Info {
	return o.fs
}

func (o *Object) SetModTime(ctx context.Context, modTime time.Time) error {
	return fs.ErrorCantSetModTime
}

func (o *Object) String() string {
	if o.file != nil {
		return strings.TrimRight(o.file.FolderPath, "/") + "/" + o.file.Name
	} else if o.folder != nil {
		return o.folder.Path
	}

	return o.remote
}

func (o *Object) Hash(ctx context.Context, ht hash.Type) (string, error) {
	return "", hash.ErrUnsupported
}

func (f *Fs) Precision() time.Duration {
	// Return the time precision supported by the backend.
	// Use fs.ModTimeNotSupported if modification times are not supported.
	return fs.ModTimeNotSupported
}

// Remote joins the path with the fs root
func (o *Object) Remote() string {
	// Ensure paths are normalized and relative
	remotePath := path.Clean(o.String())
	rootPath := path.Clean(o.fs.root)

	// Strip the root path from the remote if necessary
	remotePath = strings.TrimPrefix(remotePath, rootPath)

	// Return the relative path
	return strings.TrimLeft(remotePath, "/")
}

func (o *Object) Storable() bool {
	return true
}

// Size returns the size of the object in bytes.
func (o *Object) Size() int64 {
	if o.file != nil {
		return o.file.Size
	}

	return 0
}

// Stat will ensure that either folder or file is populated, then return the object to use as ObjectInfo
func (o *Object) Stat(ctx context.Context) (fs.ObjectInfo, error) {
	if o.file != nil || o.folder != nil {
		return o, nil
	}

	fs.Debugf(o.fs, "Stat object %s", o.remote)

	folder, file, err := o.fs.client.Find(ctx, o.remote)

	if err != nil {
		return nil, err
	}

	// Since one of these will be nil, we're fine setting both without an if check
	o.folder = folder
	o.file = file

	return o, nil
}

// Open will open the file for reading
func (o *Object) Open(ctx context.Context, options ...fs.OpenOption) (io.ReadCloser, error) {
	if o.file == nil {
		// Populate file from path
		_, file, err := o.fs.client.Find(ctx, o.remote)

		if err != nil {
			return nil, err
		} else if file == nil {
			return nil, fs.ErrorIsDir
		}

		o.file = file
	}

	// Support ranges (maybe, not sure if the API supports this?)
	opts := make([]RequestOpt, 0)

	for _, opt := range options {
		key, value := opt.Header()

		if key != "" && value != "" {
			opts = append(opts, WithHeader(key, value))
		}
	}

	return o.fs.client.DownloadFile(ctx, o.file.ID, opts...)
}

// Update pushes a file up to the backend
func (o *Object) Update(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) error {
	obj, err := o.fs.Put(ctx, in, src, options...)

	if err != nil {
		return err
	}

	o.file = obj.(*Object).file

	return nil
}

// Remove deletes the file represented by the object from the remote.
func (o *Object) Remove(ctx context.Context) error {
	return o.fs.client.DeleteFiles(ctx, o.file.ID)
}

// Items returns the count of items in this directory or this
// directory and subdirectories if known, -1 for unknown
func (d *Directory) Items() int64 {
	return int64(len(d.folder.Files))
}

// ID returns the internal ID of this directory if known, or
// "" otherwise
func (d *Directory) ID() string {
	return ""
}

// Hash does nothing on a directory
//
// This method is implemented with the incorrect type signature to
// stop the Directory type asserting to fs.Object or fs.ObjectInfo
func (d *Directory) Hash() {
	// Does nothing
}

// Check the interfaces are satisfied
var (
	_ fs.Fs          = &Fs{}
	_ fs.Object      = &Object{}
	_ fs.Directory   = &Directory{}
	_ fs.SetModTimer = &Directory{}
)
