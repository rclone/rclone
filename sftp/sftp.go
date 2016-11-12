// Package sftp provides a filesystem interface using github.com/pkg/sftp
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

type stringer interface {
	String() string
}

func debug(o stringer, msg string) {
	fs.Debug(o, msg)
}

// Fs stores the interface to the remote SFTP files
type Fs struct {
	name       string
	root       string
	features   *fs.Features // optional features
	url        string
	sshClient  *ssh.Client
	sftpClient *sftp.Client
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
	if root == "" {
		root = "."
	}
	if user == "" {
		user = os.Getenv("USER")
	}
	if port == "" {
		port = "22"
	}
	config := &ssh.ClientConfig{
		User: user,
		Auth: []ssh.AuthMethod{},
	}
	if pass == "" {
		if sshAgent, err := net.Dial("unix", os.Getenv("SSH_AUTH_SOCK")); err == nil {
			sshAgentClient := agent.NewClient(sshAgent)
			signers, _ := sshAgentClient.Signers()
			for i, signer := range signers {
				if 2*i < len(signers) {
					signers[i] = signers[len(signers)-i-1]
					signers[len(signers)-i-1] = signer
				}
			}
			config.Auth = append(config.Auth, ssh.PublicKeys(signers...))
		}
	} else {
		clearpass, err := fs.Reveal(pass)
		if err != nil {
			return nil, err
		}
		config.Auth = append(config.Auth, ssh.Password(clearpass))
	}
	if sshClient, err := ssh.Dial("tcp", host+":"+port, config); err != nil {
		return nil, err
	} else if sftpClient, err := sftp.NewClient(sshClient); err != nil {
		_ = sshClient.Close()
		return nil, err
	} else {
		f := &Fs{
			name:       name,
			root:       root,
			sshClient:  sshClient,
			sftpClient: sftpClient,
			url:        "sftp://" + user + "@" + host + ":" + port + "/" + root,
		}
		f.features = (&fs.Features{}).Fill(f)
		return f, nil
	}
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
	debug(f, "New '"+remote+"'")
	info, err := f.sftpClient.Stat(f.sftpClient.Join(f.root, remote))
	if err != nil {
		return nil, err
	}
	object := &Object{
		fs:     f,
		remote: remote,
		info:   info,
	}
	return object, nil
}

func (f *Fs) list(out fs.ListOpts, dirs string, name string, info os.FileInfo, level int, done *sync.WaitGroup) {
	debug(f, "list '"+f.sftpClient.Join(dirs, name)+"'")
	defer done.Done()
	if info.IsDir() {
		if out.IncludeDirectory(info.Name()) {
			dir := &fs.Dir{
				Name:  info.Name(),
				When:  info.ModTime(),
				Bytes: -1,
				Count: -1,
			}
			if level >= out.Level() {
				return
			}
			if infos, err := f.sftpClient.ReadDir(f.sftpClient.Join(f.root, dirs, name)); err == nil {
				dir.Count = int64(len(infos))
				out.AddDir(dir)
				done.Add(len(infos))
				for _, newInfo := range infos {
					go f.list(out, f.sftpClient.Join(dirs, name), newInfo.Name(), newInfo, level+1, done)
				}
			}
		}
	} else {
		file := &Object{
			fs:     f,
			remote: f.sftpClient.Join(dirs, info.Name()),
			info:   info,
		}
		out.Add(file)
	}
}

// List the files and directories starting at <dir>
func (f *Fs) List(out fs.ListOpts, dir string) {
	debug(f, "List '"+dir+"'")
	if dir == "" {
		dir = "."
	}
	var done sync.WaitGroup
	if info, _ := f.sftpClient.Stat(f.sftpClient.Join(f.root, dir)); info != nil {
		done.Add(1)
		f.list(out, "", ".", info, 0, &done)
	}
	debug(f, "List--waiting")
	done.Wait()
	out.Finished()
}

// Put data from <in> into a new remote sftp file object described by <src.Remote()> and <src.ModTime()>
func (f *Fs) Put(in io.Reader, src fs.ObjectInfo) (fs.Object, error) {
	debug(f, "Put '"+src.Remote()+"'")
	_ = f.mkdir(f.sftpClient.Join(f.root, filepath.Dir(src.Remote())))
	file, err := f.sftpClient.Create(f.sftpClient.Join(f.root, src.Remote()))
	if err != nil {
		return nil, err
	}
	_, err = file.ReadFrom(in)
	if err != nil {
		return nil, err
	}
	o, err := f.NewObject(src.Remote())
	if err != nil {
		return nil, err
	}
	err = o.SetModTime(src.ModTime())
	if err != nil {
		return nil, err
	}
	return o, nil
}

func (f *Fs) mkdir(path string) error {
	debug(f, "mkdir '"+path+"'")
	parent := filepath.Dir(path)
	if parent != "." && parent != "/" {
		_ = f.mkdir(parent)
	}
	return f.sftpClient.Mkdir(path)
}

// Mkdir makes the root directory of the Fs object
func (f *Fs) Mkdir(dir string) error {
	root := path.Join(f.root, dir)
	debug(f, "Mkdir '"+root+"'")
	o, _ := f.NewObject("")
	if o == nil {
		return f.mkdir(root)
	}
	return nil
}

// Rmdir removes the root directory of the Fs object
func (f *Fs) Rmdir(dir string) error {
	root := path.Join(f.root, dir)
	debug(f, "Rmdir '"+root+"'")
	return f.sftpClient.Remove(f.root)
}

// Move renames a remote sftp file object
func (f *Fs) Move(src fs.Object, remote string) (fs.Object, error) {
	debug(f, "Move '"+src.Remote()+"' to '"+remote+"'")
	err := f.sftpClient.Rename(
		f.sftpClient.Join(f.root, src.Remote()),
		f.sftpClient.Join(f.root, remote))
	if err != nil {
		return nil, err
	}
	dstObj, err := f.NewObject(remote)
	return dstObj, err
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
	return o.fs.url + "/" + o.remote
}

// Remote the name of the remote SFTP file, relative to the fs root
func (o *Object) Remote() string {
	return o.remote
}

// Hash returns "" since SFTP (in Go or OpenSSH) doesn't support remote calculation of hashes
func (o *Object) Hash(r fs.HashType) (string, error) {
	debug(o.fs, "Hash '"+o.remote+"'")
	return "", fs.ErrHashUnsupported
}

// Size returns the size in bytes of the remote sftp file
func (o *Object) Size() int64 {
	debug(o.fs, "Size '"+o.remote+"'")
	return o.info.Size()
}

// ModTime returns the modification time of the remote sftp file
func (o *Object) ModTime() time.Time {
	debug(o.fs, "ModTime '"+o.remote+"'")
	return o.info.ModTime()
}

// SetModTime sets the modification and access time to the specified time
func (o *Object) SetModTime(modTime time.Time) error {
	debug(o.fs, "SetModTime '"+o.remote+"'")
	err := o.fs.sftpClient.Chtimes(o.fs.sftpClient.Join(o.fs.root, o.remote), modTime, modTime)
	if err != nil {
		return err
	}
	o.info, err = o.fs.sftpClient.Stat(o.fs.sftpClient.Join(o.fs.root, o.remote))
	return err
}

// Storable returns whether the remote sftp file is a regular file (not a directory, symbolic link, block device, character device, named pipe, etc)
func (o *Object) Storable() bool {
	debug(o.fs, "Storable '"+o.remote+"'?")
	return o.info.Mode().IsRegular()
}

// Read from a remote sftp file object reader
func (file *ObjectReader) Read(p []byte) (n int, err error) {
	debug(file.object.fs, "Read '"+file.object.remote+"'")
	n, err = file.sftpFile.Read(p)
	return n, err
}

// Close a reader of a remote sftp file
func (file *ObjectReader) Close() (err error) {
	debug(file.object.fs, "Close '"+file.object.remote+"'")
	err = file.sftpFile.Close()
	return err
}

// Open a remote sftp file object for reading. Seek is supported
func (o *Object) Open(options ...fs.OpenOption) (in io.ReadCloser, err error) {
	debug(o.fs, "Open '"+o.remote+"'")
	var offset int64
	offset = 0
	for _, option := range options {
		switch x := option.(type) {
		case *fs.SeekOption:
			offset = x.Offset
		default:
			if option.Mandatory() {
				fs.Log(o, "Unsupported mandatory option: %v", option)
			}
		}
	}
	sftpFile, err := o.fs.sftpClient.Open(o.fs.sftpClient.Join(o.fs.root, o.remote))
	if err != nil {
		return nil, err
	}
	if offset > 0 {
		off, err := sftpFile.Seek(offset, 0)
		if err != nil || off != offset {
			return nil, err
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
	debug(o.fs, "Update '"+o.remote+"'")
	file, err := o.fs.sftpClient.Create(o.fs.sftpClient.Join(o.fs.root, o.remote))
	if err == nil {
		_, err = file.ReadFrom(in)
		if err != nil {
			return err
		}
		err = o.SetModTime(src.ModTime())
		return err
	}
	return err
}

// Remove a remote sftp file object
func (o *Object) Remove() error {
	debug(o.fs, "Remove '"+o.remote+"'")
	return o.fs.sftpClient.Remove(o.fs.sftpClient.Join(o.fs.root, o.remote))
}

// Check the interfaces are satisfied
var (
	_ fs.Fs     = &Fs{}
	_ fs.Mover  = &Fs{}
	_ fs.Object = &Object{}
)
