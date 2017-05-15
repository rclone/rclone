// Package sftp provides a filesystem interface using github.com/pkg/sftp

// +build !plan9

package sftp

import (
	"io"
	"net"
	"os"
	"path"
	"path/filepath"
	"sync"
	"time"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"

	"github.com/ncw/rclone/fs"
	"github.com/pkg/errors"
	"github.com/pkg/sftp"
)

func init() {
	fsi := &fs.RegInfo{
		Name:        "sftp",
		Description: "SSH/SFTP Connection",
		NewFs:       NewFs,
		Options: []fs.Option{{
			Name:     "host",
			Help:     "SSH host to connect to",
			Optional: false,
			Examples: []fs.OptionExample{{
				Value: "example.com",
				Help:  "Connect to example.com",
			}},
		}, {
			Name:     "user",
			Help:     "SSH username, leave blank for current username, " + os.Getenv("USER"),
			Optional: true,
		}, {
			Name:     "port",
			Help:     "SSH port",
			Optional: true,
		}, {
			Name:       "pass",
			Help:       "SSH password, leave blank to use ssh-agent",
			Optional:   true,
			IsPassword: true,
		}},
	}
	fs.Register(fsi)
}

// Fs stores the interface to the remote SFTP files
type Fs struct {
	name       string
	root       string
	features   *fs.Features // optional features
	url        string
	sshClient  *ssh.Client
	sftpClient *sftp.Client
	mkdirLock  *stringLock
}

// Object is a remote SFTP file that has been stat'd (so it exists, but is not necessarily open for reading)
type Object struct {
	fs     *Fs
	remote string
	info   os.FileInfo
}

// ObjectReader holds the sftp.File interface to a remote SFTP file opened for reading
type ObjectReader struct {
	object   *Object
	sftpFile *sftp.File
}

// NewFs creates a new Fs object from the name and root. It connects to
// the host specified in the config file.
func NewFs(name, root string) (fs.Fs, error) {
	user := fs.ConfigFileGet(name, "user")
	host := fs.ConfigFileGet(name, "host")
	port := fs.ConfigFileGet(name, "port")
	pass := fs.ConfigFileGet(name, "pass")
	if user == "" {
		user = os.Getenv("USER")
	}
	if port == "" {
		port = "22"
	}
	config := &ssh.ClientConfig{
		User:            user,
		Auth:            []ssh.AuthMethod{},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         fs.Config.ConnectTimeout,
	}
	if pass == "" {
		authSock := os.Getenv("SSH_AUTH_SOCK")
		if authSock == "" {
			return nil, errors.New("SSH_AUTH_SOCK is unset so can't connect to ssh-agent")
		}
		sshAgent, err := net.Dial("unix", authSock)
		if err != nil {
			return nil, errors.Wrap(err, "couldn't connect to ssh-agent")
		}
		sshAgentClient := agent.NewClient(sshAgent)
		signers, err := sshAgentClient.Signers()
		if err != nil {
			return nil, errors.Wrap(err, "couldn't read ssh agent signers")
		}
		/*
			for i, signer := range signers {
				if 2*i < len(signers) {
					signers[i] = signers[len(signers)-i-1]
					signers[len(signers)-i-1] = signer
				}
			}
		*/
		config.Auth = append(config.Auth, ssh.PublicKeys(signers...))
	} else {
		clearpass, err := fs.Reveal(pass)
		if err != nil {
			return nil, err
		}
		config.Auth = append(config.Auth, ssh.Password(clearpass))
	}
	sshClient, err := ssh.Dial("tcp", host+":"+port, config)
	if err != nil {
		return nil, errors.Wrap(err, "couldn't connect ssh")
	}
	sftpClient, err := sftp.NewClient(sshClient)
	if err != nil {
		_ = sshClient.Close()
		return nil, errors.Wrap(err, "couldn't initialise SFTP")
	}
	f := &Fs{
		name:       name,
		root:       root,
		sshClient:  sshClient,
		sftpClient: sftpClient,
		url:        "sftp://" + user + "@" + host + ":" + port + "/" + root,
		mkdirLock:  newStringLock(),
	}
	f.features = (&fs.Features{}).Fill(f)
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
	go func() {
		// FIXME re-open the connection here...
		err := f.sshClient.Conn.Wait()
		fs.Errorf(f, "SSH connection closed: %v", err)
	}()
	return f, nil
}

// Name returns the configured name of the file system
func (f *Fs) Name() string {
	return f.name
}

// Root returns the root for the filesystem
func (f *Fs) Root() string {
	return f.root
}

// String returns the URL for the filesystem
func (f *Fs) String() string {
	return f.url
}

// Features returns the optional features of this Fs
func (f *Fs) Features() *fs.Features {
	return f.features
}

// Precision is the remote sftp file system's modtime precision, which we have no way of knowing. We estimate at 1s
func (f *Fs) Precision() time.Duration {
	return time.Second
}

// NewObject creates a new remote sftp file object
func (f *Fs) NewObject(remote string) (fs.Object, error) {
	o := &Object{
		fs:     f,
		remote: remote,
	}
	err := o.stat()
	if err != nil {
		return nil, err
	}
	return o, nil
}

// dirExists returns true,nil if the directory exists, false, nil if
// it doesn't or false, err
func (f *Fs) dirExists(dir string) (bool, error) {
	if dir == "" {
		dir = "."
	}
	info, err := f.sftpClient.Stat(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, errors.Wrap(err, "dirExists stat failed")
	}
	if !info.IsDir() {
		return false, fs.ErrorIsFile
	}
	return true, nil
}

func (f *Fs) list(out fs.ListOpts, dir string, level int, wg *sync.WaitGroup, tokens chan struct{}) {
	defer wg.Done()
	// take a token
	<-tokens
	// return it when done
	defer func() {
		tokens <- struct{}{}
	}()
	sftpDir := path.Join(f.root, dir)
	if sftpDir == "" {
		sftpDir = "."
	}
	infos, err := f.sftpClient.ReadDir(sftpDir)
	if err != nil {
		err = errors.Wrapf(err, "error listing %q", dir)
		fs.Errorf(f, "Listing failed: %v", err)
		out.SetError(err)
		return
	}
	for _, info := range infos {
		remote := path.Join(dir, info.Name())
		if info.IsDir() {
			if out.IncludeDirectory(remote) {
				dir := &fs.Dir{
					Name:  remote,
					When:  info.ModTime(),
					Bytes: -1,
					Count: -1,
				}
				out.AddDir(dir)
				if level < out.Level() {
					wg.Add(1)
					go f.list(out, remote, level+1, wg, tokens)
				}
			}
		} else {
			file := &Object{
				fs:     f,
				remote: remote,
				info:   info,
			}
			out.Add(file)
		}
	}
}

// List the files and directories starting at <dir>
func (f *Fs) List(out fs.ListOpts, dir string) {
	root := path.Join(f.root, dir)
	ok, err := f.dirExists(root)
	if err != nil {
		out.SetError(errors.Wrap(err, "List failed"))
		return
	}
	if !ok {
		out.SetError(fs.ErrorDirNotFound)
		return
	}
	// tokens to control the concurrency
	tokens := make(chan struct{}, fs.Config.Checkers)
	for i := 0; i < fs.Config.Checkers; i++ {
		tokens <- struct{}{}
	}
	wg := new(sync.WaitGroup)
	wg.Add(1)
	f.list(out, dir, 1, wg, tokens)
	wg.Wait()
	out.Finished()
}

// Put data from <in> into a new remote sftp file object described by <src.Remote()> and <src.ModTime()>
func (f *Fs) Put(in io.Reader, src fs.ObjectInfo) (fs.Object, error) {
	err := f.mkParentDir(src.Remote())
	if err != nil {
		return nil, errors.Wrap(err, "Put mkParentDir failed")
	}
	// Temporary object under construction
	o := &Object{
		fs:     f,
		remote: src.Remote(),
	}
	err = o.Update(in, src)
	if err != nil {
		return nil, err
	}
	return o, nil
}

// mkParentDir makes the parent of remote if necessary and any
// directories above that
func (f *Fs) mkParentDir(remote string) error {
	parent := path.Dir(remote)
	return f.mkdir(path.Join(f.root, parent))
}

// mkdir makes the directory and parents using native paths
func (f *Fs) mkdir(path string) error {
	f.mkdirLock.Lock(path)
	defer f.mkdirLock.Unlock(path)
	if path == "." || path == "/" {
		return nil
	}
	ok, err := f.dirExists(path)
	if err != nil {
		return errors.Wrap(err, "mkdir dirExists failed")
	}
	if ok {
		return nil
	}
	parent := filepath.Dir(path)
	err = f.mkdir(parent)
	if err != nil {
		return err
	}
	err = f.sftpClient.Mkdir(path)
	if err != nil {
		return errors.Wrapf(err, "mkdir %q failed", path)
	}
	return nil
}

// Mkdir makes the root directory of the Fs object
func (f *Fs) Mkdir(dir string) error {
	root := path.Join(f.root, dir)
	return f.mkdir(root)
}

// Rmdir removes the root directory of the Fs object
func (f *Fs) Rmdir(dir string) error {
	root := path.Join(f.root, dir)
	return f.sftpClient.Remove(root)
}

// Move renames a remote sftp file object
func (f *Fs) Move(src fs.Object, remote string) (fs.Object, error) {
	srcObj, ok := src.(*Object)
	if !ok {
		fs.Debugf(src, "Can't move - not same remote type")
		return nil, fs.ErrorCantMove
	}
	err := f.mkParentDir(remote)
	if err != nil {
		return nil, errors.Wrap(err, "Move mkParentDir failed")
	}
	err = f.sftpClient.Rename(
		srcObj.path(),
		path.Join(f.root, remote),
	)
	if err != nil {
		return nil, errors.Wrap(err, "Move Rename failed")
	}
	dstObj, err := f.NewObject(remote)
	if err != nil {
		return nil, errors.Wrap(err, "Move NewObject failed")
	}
	return dstObj, nil
}

// DirMove moves src, srcRemote to this remote at dstRemote
// using server side move operations.
//
// Will only be called if src.Fs().Name() == f.Name()
//
// If it isn't possible then return fs.ErrorCantDirMove
//
// If destination exists then return fs.ErrorDirExists
func (f *Fs) DirMove(src fs.Fs, srcRemote, dstRemote string) error {
	srcFs, ok := src.(*Fs)
	if !ok {
		fs.Debugf(srcFs, "Can't move directory - not same remote type")
		return fs.ErrorCantDirMove
	}
	srcPath := path.Join(srcFs.root, srcRemote)
	dstPath := path.Join(f.root, dstRemote)

	// Check if destination exists
	ok, err := f.dirExists(dstPath)
	if err != nil {
		return errors.Wrap(err, "DirMove dirExists dst failed")
	}
	if ok {
		return fs.ErrorDirExists
	}

	// Make sure the parent directory exists
	err = f.mkdir(path.Dir(dstPath))
	if err != nil {
		return errors.Wrap(err, "DirMove mkParentDir dst failed")
	}

	// Do the move
	err = f.sftpClient.Rename(
		srcPath,
		dstPath,
	)
	if err != nil {
		return errors.Wrapf(err, "DirMove Rename(%q,%q) failed", srcPath, dstPath)
	}
	return nil
}

// Hashes returns fs.HashNone to indicate remote hashing is unavailable
func (f *Fs) Hashes() fs.HashSet {
	return fs.HashSet(fs.HashNone)
}

// Fs is the filesystem this remote sftp file object is located within
func (o *Object) Fs() fs.Info {
	return o.fs
}

// String returns the URL to the remote SFTP file
func (o *Object) String() string {
	if o == nil {
		return "<nil>"
	}
	return o.remote
}

// Remote the name of the remote SFTP file, relative to the fs root
func (o *Object) Remote() string {
	return o.remote
}

// Hash returns "" since SFTP (in Go or OpenSSH) doesn't support remote calculation of hashes
func (o *Object) Hash(r fs.HashType) (string, error) {
	return "", fs.ErrHashUnsupported
}

// Size returns the size in bytes of the remote sftp file
func (o *Object) Size() int64 {
	return o.info.Size()
}

// ModTime returns the modification time of the remote sftp file
func (o *Object) ModTime() time.Time {
	return o.info.ModTime()
}

// path returns the native path of the object
func (o *Object) path() string {
	return path.Join(o.fs.root, o.remote)
}

// stat updates the info field in the Object
func (o *Object) stat() error {
	info, err := o.fs.sftpClient.Stat(o.path())
	if err != nil {
		if os.IsNotExist(err) {
			return fs.ErrorObjectNotFound
		}
		return errors.Wrap(err, "stat failed")
	}
	if info.IsDir() {
		return errors.Wrapf(fs.ErrorNotAFile, "%q", o.remote)
	}
	o.info = info
	return nil
}

// SetModTime sets the modification and access time to the specified time
//
// it also updates the info field
func (o *Object) SetModTime(modTime time.Time) error {
	err := o.fs.sftpClient.Chtimes(o.path(), modTime, modTime)
	if err != nil {
		return errors.Wrap(err, "SetModTime failed")
	}
	err = o.stat()
	if err != nil {
		return errors.Wrap(err, "SetModTime failed")
	}
	return nil
}

// Storable returns whether the remote sftp file is a regular file (not a directory, symbolic link, block device, character device, named pipe, etc)
func (o *Object) Storable() bool {
	return o.info.Mode().IsRegular()
}

// Read from a remote sftp file object reader
func (file *ObjectReader) Read(p []byte) (n int, err error) {
	n, err = file.sftpFile.Read(p)
	return n, err
}

// Close a reader of a remote sftp file
func (file *ObjectReader) Close() (err error) {
	err = file.sftpFile.Close()
	return err
}

// Open a remote sftp file object for reading. Seek is supported
func (o *Object) Open(options ...fs.OpenOption) (in io.ReadCloser, err error) {
	var offset int64
	offset = 0
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
	sftpFile, err := o.fs.sftpClient.Open(o.path())
	if err != nil {
		return nil, errors.Wrap(err, "Open failed")
	}
	if offset > 0 {
		off, err := sftpFile.Seek(offset, 0)
		if err != nil || off != offset {
			return nil, errors.Wrap(err, "Open Seek failed")
		}
	}
	in = &ObjectReader{
		object:   o,
		sftpFile: sftpFile,
	}
	return in, nil
}

// Update a remote sftp file using the data <in> and ModTime from <src>
func (o *Object) Update(in io.Reader, src fs.ObjectInfo) error {
	file, err := o.fs.sftpClient.Create(o.path())
	if err != nil {
		return errors.Wrap(err, "Update Create failed")
	}
	// remove the file if upload failed
	remove := func() {
		removeErr := o.fs.sftpClient.Remove(o.path())
		if removeErr != nil {
			fs.Debugf(src, "Failed to remove: %v", removeErr)
		} else {
			fs.Debugf(src, "Removed after failed upload: %v", err)
		}
	}
	_, err = file.ReadFrom(in)
	if err != nil {
		remove()
		return errors.Wrap(err, "Update ReadFrom failed")
	}
	err = file.Close()
	if err != nil {
		remove()
		return errors.Wrap(err, "Update Close failed")
	}
	err = o.SetModTime(src.ModTime())
	if err != nil {
		return errors.Wrap(err, "Update SetModTime failed")
	}
	return nil
}

// Remove a remote sftp file object
func (o *Object) Remove() error {
	return o.fs.sftpClient.Remove(o.path())
}

// Check the interfaces are satisfied
var (
	_ fs.Fs       = &Fs{}
	_ fs.Mover    = &Fs{}
	_ fs.DirMover = &Fs{}
	_ fs.Object   = &Object{}
)
