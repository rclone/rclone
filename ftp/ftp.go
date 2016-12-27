// Package fs is a generic file system interface for rclone object storage systems
package ftp

import (
	"fmt"
	"github.com/jlaffaye/ftp"
	"github.com/ncw/rclone/fs"
	"io"
	"path/filepath"
	"regexp"
	"strconv"
	"time"
	"sync"
	"strings"
)

// Register with Fs
func init() {
	fs.Register(&fs.RegInfo{
		Name: "Ftp",
		Description: "FTP interface",
		NewFs: NewFs,
		Options: []fs.Option{
			{
				Name: "username",
				Help: "Username",
			}, {
				Name: "password",
				Help: "Password",
			}, {
				Name: "url",
				Help: "FTP url",
			},
		},
	})
}

type Url struct {
	Scheme string
	Host string
	Port int
	Path string
}

type Fs struct {
	name              string            // name of this remote
	c                 *ftp.ServerConn   // the connection to the FTP server
	root              string            // the path we are working on if any
	url               Url
	mu                sync.Mutex
}

type Object struct {
	fs      *Fs
	remote  string
	info    *FileInfo
}

type FileInfo struct {
	Name    string
	Size    uint64
	ModTime time.Time
	IsDir   bool
}



// Implements ReadCloser for FTP objects.
type FtpReadCloser struct {
	remote 	   string
	c      	   *ftp.ServerConn
	fd         io.ReadCloser
}

/////////////////
// Url methods //
/////////////////
func (u *Url) ToDial() string {
	return fmt.Sprintf("%s:%d", u.Host, u.Port)
}

func (u *Url) String() string {
	return fmt.Sprintf("ftp://%s:%d/%s", u.Host, u.Port, u.Path)
}

func parseUrl(url string) Url {
	// This is *similar* to the RFC 3986 regexp but it matches the
	// port independently from the host
	r, _ := regexp.Compile("^(([^:/?#]+):)?(//([^/?#:]*))?(:([0-9]+))?([^?#]*)(\\?([^#]*))?(#(.*))?")
	
	data := r.FindAllStringSubmatch(url, -1)

	if data[0][5] == "" { data[0][5] = "21" }
	port, _ := strconv.Atoi(data[0][5])
	return Url{data[0][2], data[0][4], port, data[0][7]}
}

////////////////
// Fs Methods //
////////////////

func (f *Fs) Put(in io.Reader, src fs.ObjectInfo) (fs.Object, error) {
	fs.Debug(f, "Trying to put file %s", src.Remote())
	o := &Object{
		fs: f,
		remote: src.Remote(),
	}
	err := o.Update(in, src)
	return o, err
}

func (f *Fs) Rmdir(dir string) error {
	// This is actually a recursive remove directory
	f.mu.Lock()
	files, _ := f.c.List(filepath.Join(f.root, dir))
	f.mu.Unlock()
	for i:= range files {
		if files[i].Type == ftp.EntryTypeFolder {
			f.Rmdir(filepath.Join(dir, files[i].Name))
		}
	}	
	f.mu.Lock()
	err := f.c.RemoveDir(filepath.Join(f.root, dir))
	f.mu.Unlock()
	return err
}

func (f *Fs) Name() string {
        return f.name
}

// Root of the remote (as passed into NewFs)
func (f *Fs) Root() string {
	return f.root
}

func (f *Fs) String() string {
	return fmt.Sprintf("FTP Connection to %s", f.url.String())
}

// Hash are not supported
func (f *Fs) Hashes() fs.HashSet {
	return 0
}

// Modified Time not supported
func (f *Fs) Precision() time.Duration {
	return fs.ModTimeNotSupported
}

func (f *Fs) mkdir(abspath string) error {
	_, err := f.GetInfo(abspath)
	if err != nil {
		fs.Debug(f, "Trying to create directory %s", abspath)
		f.mu.Lock()
		err := f.c.MakeDir(abspath)
		f.mu.Unlock()
		if err != nil {
			return err
		}
	}
	return err	
}

func (f *Fs) Mkdir(dir string) error {
	// This actually works as mkdir -p
	fs.Debug(f, "ENTER function 'Mkdir' on '%s/%s'", f.root, dir)
	defer fs.Debug(f, "EXIT function 'Mkdir' on '%s/%s'", f.root, dir)
	abspath := filepath.Join(f.root, dir)
	tokens := strings.Split(abspath, "/")
	curdir := ""
	for i:= range tokens {
		curdir += "/" + tokens[i]
		f.mkdir(curdir)
	}
	return nil
}

func (f *Fs) GetInfo(remote string) (*FileInfo, error) {
	fs.Debug(f, "ENTER function 'GetInfo' on file %s", remote)
	defer fs.Debug(f, "EXIT function 'GetInfo'")
	dir := filepath.Dir(remote)
	base := filepath.Base(remote)

	f.mu.Lock()
	files, _ := f.c.List(dir)
	f.mu.Unlock()
	for i:= range files {
		if files[i].Name == base {
			info := &FileInfo{
				Name: remote,
				Size: files[i].Size,
				ModTime: files[i].Time,
				IsDir: files[i].Type == ftp.EntryTypeFolder,
			}
			return info, nil
		}
	}
	return nil, fs.ErrorObjectNotFound
}

func (f *Fs) NewObject(remote string) (fs.Object, error) {
	fs.Debug(f, "ENTER function 'NewObject' called with remote %s", remote)
	defer fs.Debug(f, "EXIT function 'NewObject'")
	dir := filepath.Dir(remote)
	base := filepath.Base(remote)

	f.mu.Lock()
	files, _ := f.c.List(dir)
	f.mu.Unlock()
	for i:= range files {
		if files[i].Name == base {
			o := &Object{
				fs:     f,
				remote: remote,
			}
			info := &FileInfo{
				Name: remote,
				Size: files[i].Size,
				ModTime: files[i].Time,
			}
			o.info = info
			
			return o, nil
		}
	}
	return nil, fs.ErrorObjectNotFound
}

func (f *Fs) list(out  fs.ListOpts, dir string, curlevel int) {
	fs.Debug(f, "ENTER function 'list'")
	defer fs.Debug(f, "EXIT function 'list'")
	f.mu.Lock()
	files, _ := f.c.List(filepath.Join(f.root, dir))
	f.mu.Unlock()
	for i:= range files {
		object := files[i]
		newremote := filepath.Join(dir, object.Name)
		switch object.Type {
		case ftp.EntryTypeFolder:
			if out.IncludeDirectory(newremote){
				d := &fs.Dir{
					Name:  newremote,
					When:  object.Time,
					Bytes: 0,
					Count: -1,
				}
				if curlevel < out.Level(){
					f.list(out, filepath.Join(dir, object.Name), curlevel +1 )
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
				Name: newremote,
				Size: object.Size,
				ModTime: object.Time,
			}
			o.info = info
			if out.Add(o) {
				return
			}
		}
	}
}

func (f *Fs) List(out fs.ListOpts, dir string) {
	fs.Debug(f, "ENTER function 'List' on directory '%s/%s'", f.root, dir)
	defer fs.Debug(f, "EXIT function 'List' for directory '%s/%s'", f.root, dir)
	f.list(out, dir, 1)
	out.Finished()
}

////////////////////
// Object methods //
////////////////////

func (o *Object) Hash(t fs.HashType) (string, error) {
	return "", fs.ErrHashUnsupported
}

func (o *Object) Open(options ...fs.OpenOption) (io.ReadCloser, error) {
	path := filepath.Join(o.fs.root, o.remote)
	fs.Debug(o.fs, "ENTER function 'Open' on file '%s' in root '%s'", o.remote, o.fs.root)
	defer fs.Debug(o.fs, "EXIT function 'Open' %s", path)
	c, _, err := ftpConnection(o.fs.name, o.fs.root)
	if err != nil {
		return nil, err
	}
	fd, err := c.Retr(path)
	if err != nil {
		return nil, err
	}
	return FtpReadCloser{path, c, fd}, nil
}

func (o *Object) Remote() string {
	return o.remote
}

func (o *Object) Remove() error {
	path := filepath.Join(o.fs.root, o.remote)
	fs.Debug(o, "ENTER function 'Remove' for obejct at %s", path)
	defer fs.Debug(o, "EXIT function 'Remove' for obejct at %s", path)
	// Check if it's a directory or a file
	info, _ := o.fs.GetInfo(path)
	var err error
	if info.IsDir {
		err = o.fs.Rmdir(o.remote)
	} else {
		o.fs.mu.Lock()
		err = o.fs.c.Delete(path)
		o.fs.mu.Unlock()
	}
	return err
}

func (o *Object) SetModTime(modTime time.Time) error {
	return nil
}

func (o *Object) Fs() fs.Info {
	return o.fs
}

func (o *Object) ModTime() time.Time {
	return o.info.ModTime
}

func (o *Object) Size() int64 {
	return int64(o.info.Size)
}

func (o *Object) Storable() bool {
	return true
}

func (o *Object) String() string {
	return fmt.Sprintf("FTP file at %s/%s", o.fs.url.String(), o.remote)
}

func (o *Object) MakeAllDir() {
	tokens := strings.Split(filepath.Dir(o.remote), "/")
	dir := ""
	for i:= range tokens {
		dir += tokens[i]+"/"
		o.fs.Mkdir(dir)
	}
}
func (o *Object) Update(in io.Reader, src fs.ObjectInfo) error {
	// Create all upper directory first...
	o.MakeAllDir()
	path := filepath.Join(o.fs.root, o.remote)
	c, _, _ := ftpConnection(o.fs.name, o.fs.root)
	err := c.Stor(path, in)
	o.info, _ = o.fs.GetInfo(path)
	return err
}

///////////////////////////
// FtpReadCloser methods //
///////////////////////////

func (f FtpReadCloser) Read(p []byte) (int, error) {
	return f.fd.Read(p)
}

func (f FtpReadCloser) Close() error {
	err := f.fd.Close()
	defer f.c.Quit()
	if err != nil {
		return nil
	}
	return nil
}

// This mutex is only used by ftpConnection. We create a new ftp
// connection for each transfer, but we need to serialize it otherwise
// Dial() and Login() might be mixed...
var globalMux = sync.Mutex{}

func ftpConnection(name, root string) (*ftp.ServerConn, Url, error) {
	// Open a new connection to the FTP server.
	url := fs.ConfigFileGet(name, "url")
	user := fs.ConfigFileGet(name, "username")
	pass := fs.ConfigFileGet(name, "password")
	u := parseUrl(url)
	u.Path = filepath.Join(u.Path, root)
	fs.Debug(nil, "New ftp Connection with name %s and url %s (path %s)", name, u.String(), u.Path)
	globalMux.Lock()
	defer globalMux.Unlock()
	c, err := ftp.DialTimeout(u.ToDial(), 30*time.Second)
	if err != nil {
		fs.ErrorLog(nil, "Error while Dialing %s: %s", u.ToDial(), err)
		return nil, u, err
	}
	err = c.Login(user, pass)
	if err != nil {
		fs.ErrorLog(nil, "Error while Logging in into %s: %s", u.ToDial(), err)
		return nil, u, err
	}
	return c, u, nil
}



// Register the FS
func NewFs(name, root string) (fs.Fs, error) {
	fs.Debug(nil, "ENTER function 'NewFs' with name %s and root %s", name, root)
	defer fs.Debug(nil, "EXIT function 'NewFs'")
	c, u, err := ftpConnection(name, root)
	if err != nil {
		return nil, err
	}
	fs := &Fs{
		name: name,
		root: u.Path,
		c: c,
		url: u,
		mu: sync.Mutex{},
	}
	return fs, err
}

var (
	_ fs.Fs = &Fs{}
)
