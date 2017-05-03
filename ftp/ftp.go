// Package ftp interfaces with FTP servers
package ftp

import (
	"io"
	"net/url"
	"path"
	"strings"
	"sync"
	"time"

	"github.com/jlaffaye/ftp"
	"github.com/ncw/rclone/fs"
	"github.com/pkg/errors"
)

// This mutex is only used by ftpConnection. We create a new ftp
// connection for each transfer, but we need to serialize it otherwise
// Dial() and Login() might be mixed...
var globalMux = sync.Mutex{}

// Register with Fs
func init() {
	fs.Register(&fs.RegInfo{
		Name:        "ftp",
		Description: "FTP interface",
		NewFs:       NewFs,
		Options: []fs.Option{
			{
				Name: "username",
				Help: "Username",
			}, {
				Name:       "password",
				Help:       "Password",
				IsPassword: true,
			}, {
				Name: "url",
				Help: "FTP URL",
			},
		},
	})
}

// Fs represents a remote FTP server
type Fs struct {
	name     string          // name of this remote
	root     string          // the path we are working on if any
	features *fs.Features    // optional features
	c        *ftp.ServerConn // the connection to the FTP server
	url      *url.URL
	mu       sync.Mutex
}

// Object describes an FTP file
type Object struct {
	fs     *Fs
	remote string
	info   *FileInfo
}

// FileInfo is the metadata known about an FTP file
type FileInfo struct {
	Name    string
	Size    uint64
	ModTime time.Time
	IsDir   bool
}

// ------------------------------------------------------------

// Name of this fs
func (f *Fs) Name() string {
	return f.name
}

// Root of the remote (as passed into NewFs)
func (f *Fs) Root() string {
	return f.root
}

// String returns a description of the FS
func (f *Fs) String() string {
	return f.url.String()
}

// Features returns the optional features of this Fs
func (f *Fs) Features() *fs.Features {
	return f.features
}

// Open a new connection to the FTP server.
func ftpConnection(name, root string) (*ftp.ServerConn, *url.URL, error) {
	URL := fs.ConfigFileGet(name, "url")
	user := fs.ConfigFileGet(name, "username")
	pass := fs.ConfigFileGet(name, "password")
	pass, err := fs.Reveal(pass)
	if err != nil {
		return nil, nil, errors.Wrap(err, "failed to decrypt password")
	}
	u, err := url.Parse(URL)
	if err != nil {
		return nil, nil, errors.Wrap(err, "open ftp connection url parse")
	}
	u.Path = path.Join(u.Path, root)
	fs.Debugf(nil, "New ftp Connection with name %s and url %s (path %s)", name, u.String(), u.Path)
	globalMux.Lock()
	defer globalMux.Unlock()
	dialAddr := u.Hostname()
	if u.Port() != "" {
		dialAddr += ":" + u.Port()
	} else {
		dialAddr += ":21"
	}
	c, err := ftp.DialTimeout(dialAddr, 30*time.Second)
	if err != nil {
		fs.Errorf(nil, "Error while Dialing %s: %s", dialAddr, err)
		return nil, u, err
	}
	err = c.Login(user, pass)
	if err != nil {
		fs.Errorf(nil, "Error while Logging in into %s: %s", dialAddr, err)
		return nil, u, err
	}
	return c, u, nil
}

// NewFs contstructs an Fs from the path, container:path
func NewFs(name, root string) (fs.Fs, error) {
	fs.Debugf(nil, "ENTER function 'NewFs' with name %s and root %s", name, root)
	defer fs.Debugf(nil, "EXIT function 'NewFs'")
	c, u, err := ftpConnection(name, root)
	if err != nil {
		return nil, err
	}
	f := &Fs{
		name: name,
		root: u.Path,
		c:    c,
		url:  u,
		mu:   sync.Mutex{},
	}
	f.features = (&fs.Features{}).Fill(f)
	return f, err
}

// NewObject finds the Object at remote.  If it can't be found
// it returns the error fs.ErrorObjectNotFound.
func (f *Fs) NewObject(remote string) (fs.Object, error) {
	fs.Debugf(f, "ENTER function 'NewObject' called with remote %s", remote)
	defer fs.Debugf(f, "EXIT function 'NewObject'")
	dir := path.Dir(remote)
	base := path.Base(remote)

	f.mu.Lock()
	files, err := f.c.List(dir)
	f.mu.Unlock()
	if err != nil {
		return nil, err
	}
	for i := range files {
		if files[i].Name == base {
			o := &Object{
				fs:     f,
				remote: remote,
			}
			info := &FileInfo{
				Name:    remote,
				Size:    files[i].Size,
				ModTime: files[i].Time,
			}
			o.info = info

			return o, nil
		}
	}
	return nil, fs.ErrorObjectNotFound
}

func (f *Fs) list(out fs.ListOpts, dir string, curlevel int) {
	fs.Debugf(f, "ENTER function 'list'")
	defer fs.Debugf(f, "EXIT function 'list'")
	f.mu.Lock()
	files, err := f.c.List(path.Join(f.root, dir))
	f.mu.Unlock()
	if err != nil {
		out.SetError(err)
		return
	}
	for i := range files {
		object := files[i]
		newremote := path.Join(dir, object.Name)
		switch object.Type {
		case ftp.EntryTypeFolder:
			if out.IncludeDirectory(newremote) {
				d := &fs.Dir{
					Name:  newremote,
					When:  object.Time,
					Bytes: 0,
					Count: -1,
				}
				if curlevel < out.Level() {
					f.list(out, path.Join(dir, object.Name), curlevel+1)
				}
				if out.AddDir(d) {
					return
				}
			}
		default:
			o := &Object{
				fs:     f,
				remote: newremote,
			}
			info := &FileInfo{
				Name:    newremote,
				Size:    object.Size,
				ModTime: object.Time,
			}
			o.info = info
			if out.Add(o) {
				return
			}
		}
	}
}

// List the objects and directories of the Fs starting from dir
//
// dir should be "" to start from the root, and should not
// have trailing slashes.
//
// This should return ErrDirNotFound (using out.SetError())
// if the directory isn't found.
//
// Fses must support recursion levels of fs.MaxLevel and 1.
// They may return ErrorLevelNotSupported otherwise.
func (f *Fs) List(out fs.ListOpts, dir string) {
	fs.Debugf(f, "ENTER function 'List' on directory '%s/%s'", f.root, dir)
	defer fs.Debugf(f, "EXIT function 'List' for directory '%s/%s'", f.root, dir)
	f.list(out, dir, 1)
	out.Finished()
}

// Hashes are not supported
func (f *Fs) Hashes() fs.HashSet {
	return 0
}

// Precision shows Modified Time not supported
func (f *Fs) Precision() time.Duration {
	return fs.ModTimeNotSupported
}

// Put in to the remote path with the modTime given of the given size
//
// May create the object even if it returns an error - if so
// will return the object and the error, otherwise will return
// nil and the error
func (f *Fs) Put(in io.Reader, src fs.ObjectInfo) (fs.Object, error) {
	fs.Debugf(f, "Trying to put file %s", src.Remote())
	o := &Object{
		fs:     f,
		remote: src.Remote(),
	}
	err := o.Update(in, src)
	return o, err
}

// getInfo reads the FileInfo for a path
func (f *Fs) getInfo(remote string) (*FileInfo, error) {
	fs.Debugf(f, "ENTER function 'getInfo' on file %s", remote)
	defer fs.Debugf(f, "EXIT function 'getInfo'")
	dir := path.Dir(remote)
	base := path.Base(remote)

	f.mu.Lock()
	files, err := f.c.List(dir)
	f.mu.Unlock()
	if err != nil {
		return nil, err
	}

	for i := range files {
		if files[i].Name == base {
			info := &FileInfo{
				Name:    remote,
				Size:    files[i].Size,
				ModTime: files[i].Time,
				IsDir:   files[i].Type == ftp.EntryTypeFolder,
			}
			return info, nil
		}
	}
	return nil, fs.ErrorObjectNotFound
}

func (f *Fs) mkdir(abspath string) error {
	_, err := f.getInfo(abspath)
	if err != nil {
		fs.Debugf(f, "Trying to create directory %s", abspath)
		f.mu.Lock()
		err := f.c.MakeDir(abspath)
		f.mu.Unlock()
		if err != nil {
			return err
		}
	}
	return err
}

// Mkdir creates the directory if it doesn't exist
func (f *Fs) Mkdir(dir string) error {
	// This actually works as mkdir -p
	fs.Debugf(f, "ENTER function 'Mkdir' on '%s/%s'", f.root, dir)
	defer fs.Debugf(f, "EXIT function 'Mkdir' on '%s/%s'", f.root, dir)
	abspath := path.Join(f.root, dir)
	tokens := strings.Split(abspath, "/")
	curdir := ""
	for i := range tokens {
		curdir += "/" + tokens[i]
		err := f.mkdir(curdir)
		if err != nil {
			return err
		}
	}
	return nil
}

// Rmdir removes the directory (container, bucket) if empty
//
// Return an error if it doesn't exist or isn't empty
func (f *Fs) Rmdir(dir string) error {
	// This is actually a recursive remove directory
	f.mu.Lock()
	files, err := f.c.List(path.Join(f.root, dir))
	f.mu.Unlock()
	if err != nil {
		return errors.Wrap(err, "rmdir")
	}
	for i := range files {
		if files[i].Type == ftp.EntryTypeFolder {
			err = f.Rmdir(path.Join(dir, files[i].Name))
			if err != nil {
				return errors.Wrap(err, "rmdir")
			}
		}
	}
	f.mu.Lock()
	err = f.c.RemoveDir(path.Join(f.root, dir))
	f.mu.Unlock()
	return err
}

// ------------------------------------------------------------

// Fs returns the parent Fs
func (o *Object) Fs() fs.Info {
	return o.fs
}

// String version of o
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

// Hash returns the hash of an object returning a lowercase hex string
func (o *Object) Hash(t fs.HashType) (string, error) {
	return "", fs.ErrHashUnsupported
}

// Size returns the size of an object in bytes
func (o *Object) Size() int64 {
	return int64(o.info.Size)
}

// ModTime returns the modification time of the object
func (o *Object) ModTime() time.Time {
	return o.info.ModTime
}

// SetModTime sets the modification time of the object
func (o *Object) SetModTime(modTime time.Time) error {
	return nil
}

// Storable returns a boolean as to whether this object is storable
func (o *Object) Storable() bool {
	return true
}

// ftpReadCloser implements io.ReadCloser for FTP objects.
type ftpReadCloser struct {
	io.ReadCloser
	c *ftp.ServerConn
}

// Close the FTP reader
func (f *ftpReadCloser) Close() error {
	err := f.ReadCloser.Close()
	err2 := f.c.Quit()
	if err == nil {
		err = err2
	}
	return err
}

// Open an object for read
func (o *Object) Open(options ...fs.OpenOption) (io.ReadCloser, error) {
	path := path.Join(o.fs.root, o.remote)
	fs.Debugf(o.fs, "ENTER function 'Open' on file '%s' in root '%s'", o.remote, o.fs.root)
	defer fs.Debugf(o.fs, "EXIT function 'Open' %s", path)
	c, _, err := ftpConnection(o.fs.name, o.fs.root)
	if err != nil {
		return nil, errors.Wrap(err, "open")
	}
	fd, err := c.Retr(path)
	if err != nil {
		return nil, errors.Wrap(err, "open")
	}
	return &ftpReadCloser{ReadCloser: fd, c: c}, nil
}

// makeAllDir creates the parent directories for the object
func (o *Object) makeAllDir() error {
	tokens := strings.Split(path.Dir(o.remote), "/")
	dir := ""
	for i := range tokens {
		dir += tokens[i] + "/"
		err := o.fs.Mkdir(dir)
		if err != nil {
			return err
		}
	}
	return nil
}

// Update the already existing object
//
// Copy the reader into the object updating modTime and size
//
// The new object may have been created if an error is returned
func (o *Object) Update(in io.Reader, src fs.ObjectInfo) error {
	// Create all upper directory first...
	err := o.makeAllDir()
	if err != nil {
		return errors.Wrap(err, "update")
	}
	path := path.Join(o.fs.root, o.remote)
	c, _, err := ftpConnection(o.fs.name, o.fs.root)
	if err != nil {
		return errors.Wrap(err, "update")
	}
	err = c.Stor(path, in)
	if err != nil {
		return errors.Wrap(err, "update")
	}
	o.info, err = o.fs.getInfo(path)
	if err != nil {
		return errors.Wrap(err, "update")
	}
	return nil
}

// Remove an object
func (o *Object) Remove() error {
	path := path.Join(o.fs.root, o.remote)
	fs.Debugf(o, "ENTER function 'Remove' for obejct at %s", path)
	defer fs.Debugf(o, "EXIT function 'Remove' for obejct at %s", path)
	// Check if it's a directory or a file
	info, err := o.fs.getInfo(path)
	if err != nil {
		return err
	}
	if info.IsDir {
		err = o.fs.Rmdir(o.remote)
	} else {
		o.fs.mu.Lock()
		err = o.fs.c.Delete(path)
		o.fs.mu.Unlock()
	}
	return err
}

// Check the interfaces are satisfied
var (
	_ fs.Fs     = &Fs{}
	_ fs.Object = &Object{}
)
