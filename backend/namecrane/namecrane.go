package namecrane

import (
	"context"
	"errors"
	"fmt"
	"github.com/namecrane/hoist"
	"github.com/rclone/rclone/fs/config/configstruct"
	"github.com/rclone/rclone/fs/config/obscure"
	"io"
	"path"
	"strings"
	"time"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config/configmap"
	"github.com/rclone/rclone/fs/hash"
)

/**
 * NameCrane Mail File Storage
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
	client      hoist.Client
	apiURL      string
	authManager hoist.AuthManager
}

type Object struct {
	fs     *Fs
	file   *hoist.File
	folder *hoist.Folder
	remote string
}

type Directory struct {
	*Object
}

type Options struct {
	ApiURL   string `config:"api_url"`
	Username string `config:"username"`
	Password string `config:"password"`
	TwoFA    string `config:"2fa"`
}

func init() {
	fs.Register(&fs.RegInfo{
		Name:        "namecrane",
		Description: "NameCrane Mail File Storage",
		NewFs:       NewFs,
		Options: []fs.Option{{
			Name:    "api_url",
			Help:    `NameCrane API URL, like https://us1.workspace.org`,
			Default: "https://us1.workspace.org",
		}, {
			Name:     "username",
			Help:     `NameCrane username`,
			Required: true,
		}, {
			Name: "password",
			Help: `NameCrane password

Only required for the first auth, subsequent requests re-use the access/refresh token`,
			Required:   true,
			IsPassword: true,
		}, {
			Name: "2fa",
			Help: `Two Factor Authentication Code

Can be supplied with --namecrane-2fa=CODE when using any command for the first auth`,
			Required: false,
		}, {
			Name:       accessTokenKey,
			Help:       "Access token (internal only)",
			Required:   false,
			Advanced:   true,
			Sensitive:  true,
			IsPassword: true,
			Hide:       fs.OptionHideBoth,
		}, {
			Name:      accessTokenExpireKey,
			Help:      "Access token expiration (internal only)",
			Required:  false,
			Advanced:  true,
			Sensitive: true,
			Hide:      fs.OptionHideBoth,
		}, {
			Name:       refreshTokenKey,
			Help:       "Refresh token (internal only)",
			Required:   false,
			Advanced:   true,
			Sensitive:  true,
			IsPassword: true,
			Hide:       fs.OptionHideBoth,
		}, {
			Name:      refreshTokenExpireKey,
			Help:      "Refresh token expiration (internal only)",
			Required:  false,
			Advanced:  true,
			Sensitive: true,
			Hide:      fs.OptionHideBoth,
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

	authManager := hoist.NewAuthManager(opt.ApiURL, hoist.WithAuthStore(&ConfigMapperStore{
		m: m,
	}))

	if _, err := authManager.GetToken(ctx); errors.Is(err, hoist.ErrNoToken) {
		if opt.Username != "" && opt.Password != "" {
			err = authManager.Authenticate(ctx, opt.Username, opt.Password, opt.TwoFA)

			if err != nil {
				return nil, fmt.Errorf("unable to authenticate: %w", err)
			}
		}
	} else if err != nil {
		// Other error occurred, potentially needing a re-login
		return nil, err
	}

	client := hoist.NewClient(opt.ApiURL, authManager)

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
	if err != nil && !errors.Is(err, hoist.ErrNoFile) {
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
			if errors.Is(err, hoist.ErrNoFolder) {
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

func (f *Fs) folderToEntries(folder hoist.Folder) fs.DirEntries {
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

	remote = path.Join(f.root, remote)

	folder, file, err := f.client.Find(ctx, remote)

	if err != nil {
		if errors.Is(err, hoist.ErrNoFile) {
			return nil, fs.ErrorObjectNotFound
		}

		fs.Debugf(f, "Unable to find existing file at %s, not necessarily a bad thing: %s", remote, err.Error())
	}

	return &Object{
		fs:     f,
		remote: remote,
		folder: folder,
		file:   file,
	}, nil
}

func (f *Fs) newObject(remote string) *Object {
	return &Object{
		fs:     f,
		remote: remote,
	}
}

func (f *Fs) Put(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error) {
	remote := src.Remote()

	remote = path.Join(f.root, remote)

	if remote[0] != '/' {
		remote = "/" + remote
	}

	fs.Debugf(f, "Put contents of %s to %s", src.Remote(), remote)

	file, err := f.client.ChunkedUpload(ctx, in, remote, src.Size())

	if err != nil {
		return nil, err
	}

	// Return the uploaded object
	return &Object{
		fs:   f,
		file: file,
	}, nil
}

// DirMove moves src, srcRemote to this remote at dstRemote
// using server-side move operations.
//
// Will only be called if src.Fs().Name() == f.Name()
//
// If it isn't possible then return fs.ErrorCantDirMove
//
// If destination exists then return fs.ErrorDirExists
func (f *Fs) DirMove(ctx context.Context, src fs.Fs, srcRemote, dstRemote string) error {
	srcFs, ok := src.(*Fs)
	if !ok {
		fs.Debugf(srcFs, "Can't move directory - not same remote type")
		return fs.ErrorCantDirMove
	}

	fs.Debugf(f, "Moving directory %s to %s", srcRemote, dstRemote)

	srcRemote = path.Join(srcFs.root, srcRemote)
	dstRemote = path.Join(f.root, dstRemote)

	// Check source remote for the folder to move
	folder, _, err := f.client.Find(ctx, srcRemote)

	if err != nil || folder == nil {
		return fs.ErrorDirNotFound
	}

	// Confirm that the parent folder exists in the destination path
	parent, _, err := f.client.Find(ctx, dstRemote)

	if errors.Is(err, hoist.ErrNoFile) {
		// If the parent does not exist, create it (equivalent to MkdirAll)
		parent, err = f.client.CreateFolder(ctx, dstRemote)

		if err != nil {
			return fs.ErrorDirNotFound
		}
	} else if err != nil {
		return err
	}

	// Check dest path for existing folder (dstRemote + folder.Name)
	existing, _, _ := f.client.Find(ctx, path.Join(dstRemote, folder.Name))

	if existing != nil {
		return fs.ErrorDirExists
	}

	// Use server side move
	err = f.client.MoveFolder(ctx, folder.Path, parent.Path, folder.Name)

	if err != nil {
		// not quite clear, but probably trying to move directory across file system
		// boundaries. Copying might still work.
		fs.Debugf(src, "Can't move dir: %v: trying copy", err)
		return fs.ErrorCantDirMove
	}

	return nil
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

	remote = path.Join(f.root, remote)

	// Temporary Object under construction
	dstObj := f.newObject(remote)

	// Check if the destination is a folder
	_, err := dstObj.Stat(ctx)

	if errors.Is(err, hoist.ErrNoFile) {
		// OK
	} else if err != nil {
		return nil, err
	}

	if dstObj.folder != nil {
		return nil, errors.New("can't move file onto non-file")
	}

	newFolder, _ := f.client.ParsePath(remote)

	baseFolder, _, err := f.client.Find(ctx, newFolder)

	if err != nil && errors.Is(err, hoist.ErrNoFile) {
		baseFolder, err = f.client.CreateFolder(ctx, newFolder)

		if err != nil {
			fs.Debugf(f, "Unable to create parent directory due to error %s", err.Error())
			return nil, fs.ErrorDirNotFound
		}
	} else if err != nil {
		fs.Debugf(f, "Unable to get parent directory due to error %s", err.Error())
		return nil, err
	}

	err = f.client.MoveFiles(ctx, baseFolder.Path, srcObj.file.ID)

	if err != nil {
		return nil, err
	}

	_, err = dstObj.Stat(ctx)

	if err != nil {
		return nil, err
	}

	return dstObj, nil
}

// PublicLink generates a public link to the remote path (usually readable by anyone)
func (f *Fs) PublicLink(ctx context.Context, remote string, expire fs.Duration, unlink bool) (string, error) {
	remote = path.Join(f.root, remote)

	_, file, err := f.client.Find(ctx, remote)

	if errors.Is(err, hoist.ErrNoFile) {
		return "", fs.ErrorObjectNotFound
	}

	// Unlink just sets published to false
	if unlink {
		err = f.client.EditFile(ctx, file.ID, hoist.EditFileParams{
			Published:      false,
			PublishedUntil: time.Time{},
		})

		return "", nil
	}

	// Generate the link
	shortLink, publicLink, err := f.client.GetLink(ctx, file.ID)

	if err != nil {
		return "", err
	}

	publicLink = strings.TrimRight(f.apiURL, "/") + "/" + publicLink

	params := hoist.EditFileParams{
		ShortLink:          shortLink,
		PublicDownloadLink: publicLink,
		Published:          true,
	}

	if expire.IsSet() {
		params.PublishedUntil = time.Now().Add(time.Duration(expire))
	}

	// Set the file to public
	err = f.client.EditFile(ctx, file.ID, params)

	if err != nil {
		return "", err
	}

	return publicLink, nil
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
	opts := make([]hoist.RequestOpt, 0)

	for _, opt := range options {
		key, value := opt.Header()

		if key != "" && value != "" {
			opts = append(opts, hoist.WithHeader(key, value))
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
	if o.file == nil {
		return fs.ErrorNotAFile
	}
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
