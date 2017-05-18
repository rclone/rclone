// Package ftp interfaces with FTP servers
package ftp

// FIXME Mover and DirMover are possible using c.Rename

import (
	"io"
	"net/textproto"
	"net/url"
	"path"
	"strings"
	"sync"
	"time"

	"github.com/jlaffaye/ftp"
	"github.com/ncw/rclone/fs"
	"github.com/pkg/errors"
)

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
	name     string       // name of this remote
	root     string       // the path we are working on if any
	features *fs.Features // optional features
	url      *url.URL
	user     string
	pass     string
	dialAddr string
	poolMu   sync.Mutex
	pool     []*ftp.ServerConn
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
func (f *Fs) ftpConnection() (*ftp.ServerConn, error) {
	fs.Debugf(f, "Connecting to FTP server")
	c, err := ftp.DialTimeout(f.dialAddr, fs.Config.ConnectTimeout)
	if err != nil {
		fs.Errorf(f, "Error while Dialing %s: %s", f.dialAddr, err)
		return nil, errors.Wrap(err, "ftpConnection Dial")
	}
	err = c.Login(f.user, f.pass)
	if err != nil {
		_ = c.Quit()
		fs.Errorf(f, "Error while Logging in into %s: %s", f.dialAddr, err)
		return nil, errors.Wrap(err, "ftpConnection Login")
	}
	return c, nil
}

// Get an FTP connection from the pool, or open a new one
func (f *Fs) getFtpConnection() (c *ftp.ServerConn, err error) {
	f.poolMu.Lock()
	if len(f.pool) > 0 {
		c = f.pool[0]
		f.pool = f.pool[1:]
	}
	f.poolMu.Unlock()
	if c != nil {
		return c, nil
	}
	return f.ftpConnection()
}

// Return an FTP connection to the pool
//
// It nils the pointed to connection out so it can't be reused
func (f *Fs) putFtpConnection(c **ftp.ServerConn) {
	f.poolMu.Lock()
	f.pool = append(f.pool, *c)
	f.poolMu.Unlock()
}

// NewFs contstructs an Fs from the path, container:path
func NewFs(name, root string) (ff fs.Fs, err error) {
	// defer fs.Trace(nil, "name=%q, root=%q", name, root)("fs=%v, err=%v", &ff, &err)
	URL := fs.ConfigFileGet(name, "url")
	user := fs.ConfigFileGet(name, "username")
	pass := fs.ConfigFileGet(name, "password")
	pass, err = fs.Reveal(pass)
	if err != nil {
		return nil, errors.Wrap(err, "NewFS decrypt password")
	}
	u, err := url.Parse(URL)
	if err != nil {
		return nil, errors.Wrap(err, "NewFS URL parse")
	}
	urlPath := strings.Trim(u.Path, "/")
	fullPath := root
	if urlPath != "" && !strings.HasPrefix("/", root) {
		fullPath = path.Join(u.Path, root)
	}
	root = fullPath
	dialAddr := u.Host
	if strings.IndexRune(dialAddr, ':') < 0 {
		dialAddr += ":21"
	}
	f := &Fs{
		name:     name,
		root:     root,
		url:      u,
		user:     user,
		pass:     pass,
		dialAddr: dialAddr,
	}
	f.features = (&fs.Features{}).Fill(f)
	// Make a connection and pool it to return errors early
	c, err := f.getFtpConnection()
	if err != nil {
		return nil, errors.Wrap(err, "NewFs")
	}
	f.putFtpConnection(&c)
	if root != "" {
		// Check to see if the root actually an existing file
		remote := path.Base(root)
		f.root = path.Dir(root)
		if f.root == "." {
			f.root = ""
		}
		_, err := f.NewObject(remote)
		if err != nil {
			if err == fs.ErrorObjectNotFound || errors.Cause(err) == fs.ErrorNotAFile {
				// File doesn't exist so return old f
				f.root = root
				return f, nil
			}
			return nil, err
		}
		// return an error with an fs which points to the parent
		return f, fs.ErrorIsFile
	}
	return f, err
}

// translateErrorFile turns FTP errors into rclone errors if possible for a file
func translateErrorFile(err error) error {
	switch errX := err.(type) {
	case *textproto.Error:
		switch errX.Code {
		case ftp.StatusFileUnavailable:
			err = fs.ErrorObjectNotFound
		}
	}
	return err
}

// translateErrorDir turns FTP errors into rclone errors if possible for a directory
func translateErrorDir(err error) error {
	switch errX := err.(type) {
	case *textproto.Error:
		switch errX.Code {
		case ftp.StatusFileUnavailable:
			err = fs.ErrorDirNotFound
		}
	}
	return err
}

// NewObject finds the Object at remote.  If it can't be found
// it returns the error fs.ErrorObjectNotFound.
func (f *Fs) NewObject(remote string) (o fs.Object, err error) {
	// defer fs.Trace(remote, "")("o=%v, err=%v", &o, &err)
	fullPath := path.Join(f.root, remote)
	dir := path.Dir(fullPath)
	base := path.Base(fullPath)

	c, err := f.getFtpConnection()
	if err != nil {
		return nil, errors.Wrap(err, "NewObject")
	}
	files, err := c.List(dir)
	f.putFtpConnection(&c)
	if err != nil {
		return nil, translateErrorFile(err)
	}
	for i, file := range files {
		if file.Type != ftp.EntryTypeFolder && file.Name == base {
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
	// defer fs.Trace(dir, "curlevel=%d", curlevel)("")
	c, err := f.getFtpConnection()
	if err != nil {
		out.SetError(errors.Wrap(err, "list"))
		return
	}
	files, err := c.List(path.Join(f.root, dir))
	f.putFtpConnection(&c)
	if err != nil {
		out.SetError(translateErrorDir(err))
		return
	}
	for i := range files {
		object := files[i]
		newremote := path.Join(dir, object.Name)
		switch object.Type {
		case ftp.EntryTypeFolder:
			if object.Name == "." || object.Name == ".." {
				continue
			}
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
	// defer fs.Trace(dir, "")("")
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
	// fs.Debugf(f, "Trying to put file %s", src.Remote())
	err := f.mkParentDir(src.Remote())
	if err != nil {
		return nil, errors.Wrap(err, "Put mkParentDir failed")
	}
	o := &Object{
		fs:     f,
		remote: src.Remote(),
	}
	err = o.Update(in, src)
	return o, err
}

// getInfo reads the FileInfo for a path
func (f *Fs) getInfo(remote string) (fi *FileInfo, err error) {
	// defer fs.Trace(remote, "")("fi=%v, err=%v", &fi, &err)
	dir := path.Dir(remote)
	base := path.Base(remote)

	c, err := f.getFtpConnection()
	if err != nil {
		return nil, errors.Wrap(err, "getInfo")
	}
	files, err := c.List(dir)
	f.putFtpConnection(&c)
	if err != nil {
		return nil, translateErrorFile(err)
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

// mkdir makes the directory and parents using unrooted paths
func (f *Fs) mkdir(abspath string) error {
	if abspath == "." || abspath == "/" {
		return nil
	}
	fi, err := f.getInfo(abspath)
	if err == nil {
		if fi.IsDir {
			return nil
		}
		return fs.ErrorIsFile
	} else if err != fs.ErrorObjectNotFound {
		return errors.Wrapf(err, "mkdir %q failed", abspath)
	}
	parent := path.Dir(abspath)
	err = f.mkdir(parent)
	if err != nil {
		return err
	}
	c, connErr := f.getFtpConnection()
	if connErr != nil {
		return errors.Wrap(connErr, "mkdir")
	}
	err = c.MakeDir(abspath)
	f.putFtpConnection(&c)
	return err
}

// mkParentDir makes the parent of remote if necessary and any
// directories above that
func (f *Fs) mkParentDir(remote string) error {
	parent := path.Dir(remote)
	return f.mkdir(path.Join(f.root, parent))
}

// Mkdir creates the directory if it doesn't exist
func (f *Fs) Mkdir(dir string) (err error) {
	// defer fs.Trace(dir, "")("err=%v", &err)
	root := path.Join(f.root, dir)
	return f.mkdir(root)
}

// Rmdir removes the directory (container, bucket) if empty
//
// Return an error if it doesn't exist or isn't empty
func (f *Fs) Rmdir(dir string) error {
	c, err := f.getFtpConnection()
	if err != nil {
		return errors.Wrap(translateErrorFile(err), "Rmdir")
	}
	err = c.RemoveDir(path.Join(f.root, dir))
	f.putFtpConnection(&c)
	return translateErrorDir(err)
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
	f *Fs
}

// Close the FTP reader and return the connection to the pool
func (f *ftpReadCloser) Close() error {
	err := f.ReadCloser.Close()
	f.f.putFtpConnection(&f.c)
	return err
}

// Open an object for read
func (o *Object) Open(options ...fs.OpenOption) (rc io.ReadCloser, err error) {
	// defer fs.Trace(o, "")("rc=%v, err=%v", &rc, &err)
	path := path.Join(o.fs.root, o.remote)
	var offset int64
	for _, option := range options {
		switch x := option.(type) {
		case *fs.SeekOption:
			offset = x.Offset
		default:
			if option.Mandatory() {
				fs.Logf(o, "Unsupported mandatory option: %v", option)
			}
		}
	}
	c, err := o.fs.getFtpConnection()
	if err != nil {
		return nil, errors.Wrap(err, "open")
	}
	fd, err := c.RetrFrom(path, uint64(offset))
	if err != nil {
		o.fs.putFtpConnection(&c)
		return nil, errors.Wrap(err, "open")
	}
	rc = &ftpReadCloser{ReadCloser: fd, c: c, f: o.fs}
	return rc, nil
}

// Update the already existing object
//
// Copy the reader into the object updating modTime and size
//
// The new object may have been created if an error is returned
func (o *Object) Update(in io.Reader, src fs.ObjectInfo) (err error) {
	// defer fs.Trace(o, "src=%v", src)("err=%v", &err)
	path := path.Join(o.fs.root, o.remote)
	// remove the file if upload failed
	remove := func() {
		removeErr := o.Remove()
		if removeErr != nil {
			fs.Debugf(o, "Failed to remove: %v", removeErr)
		} else {
			fs.Debugf(o, "Removed after failed upload: %v", err)
		}
	}
	c, err := o.fs.getFtpConnection()
	if err != nil {
		return errors.Wrap(err, "Update")
	}
	err = c.Stor(path, in)
	if err != nil {
		_ = c.Quit()
		remove()
		return errors.Wrap(err, "update stor")
	}
	o.fs.putFtpConnection(&c)
	o.info, err = o.fs.getInfo(path)
	if err != nil {
		return errors.Wrap(err, "update getinfo")
	}
	return nil
}

// Remove an object
func (o *Object) Remove() (err error) {
	// defer fs.Trace(o, "")("err=%v", &err)
	path := path.Join(o.fs.root, o.remote)
	// Check if it's a directory or a file
	info, err := o.fs.getInfo(path)
	if err != nil {
		return err
	}
	if info.IsDir {
		err = o.fs.Rmdir(o.remote)
	} else {
		c, err := o.fs.getFtpConnection()
		if err != nil {
			return errors.Wrap(err, "Remove")
		}
		err = c.Delete(path)
		o.fs.putFtpConnection(&c)
	}
	return err
}

// Check the interfaces are satisfied
var (
	_ fs.Fs     = &Fs{}
	_ fs.Object = &Object{}
)
