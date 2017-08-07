// Package sftp provides a filesystem interface using github.com/pkg/sftp

// +build !plan9

package sftp

import (
	"context"
	"io"
	"io/ioutil"
	"os"
	"path"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/ncw/rclone/fs"
	"github.com/pkg/errors"
	"github.com/pkg/sftp"
	"github.com/xanzy/ssh-agent"
	"golang.org/x/crypto/ssh"
	"golang.org/x/time/rate"
)

const (
	connectionsPerSecond = 10 // don't make more than this many ssh connections/s
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
			Help:     "SSH port, leave blank to use default (22)",
			Optional: true,
		}, {
			Name:       "pass",
			Help:       "SSH password, leave blank to use ssh-agent.",
			Optional:   true,
			IsPassword: true,
		}, {
			Name:     "key_file",
			Help:     "Path to unencrypted PEM-encoded private key file, leave blank to use ssh-agent.",
			Optional: true,
		}},
	}
	fs.Register(fsi)
}

// Fs stores the interface to the remote SFTP files
type Fs struct {
	name         string
	root         string
	features     *fs.Features // optional features
	config       *ssh.ClientConfig
	host         string
	port         string
	url          string
	mkdirLock    *stringLock
	cachedHashes *fs.HashSet
	poolMu       sync.Mutex
	pool         []*conn
	connLimit    *rate.Limiter // for limiting number of connections per second
}

// Object is a remote SFTP file that has been stat'd (so it exists, but is not necessarily open for reading)
type Object struct {
	fs      *Fs
	remote  string
	size    int64       // size of the object
	modTime time.Time   // modification time of the object
	mode    os.FileMode // mode bits from the file
	md5sum  *string     // Cached MD5 checksum
	sha1sum *string     // Cached SHA1 checksum
}

// ObjectReader holds the sftp.File interface to a remote SFTP file opened for reading
type ObjectReader struct {
	object   *Object
	sftpFile *sftp.File
}

// Dial starts a client connection to the given SSH server. It is a
// convenience function that connects to the given network address,
// initiates the SSH handshake, and then sets up a Client.
func Dial(network, addr string, config *ssh.ClientConfig) (*ssh.Client, error) {
	dialer := fs.Config.NewDialer()
	conn, err := dialer.Dial(network, addr)
	if err != nil {
		return nil, err
	}
	c, chans, reqs, err := ssh.NewClientConn(conn, addr, config)
	if err != nil {
		return nil, err
	}
	return ssh.NewClient(c, chans, reqs), nil
}

// conn encapsulates an ssh client and corresponding sftp client
type conn struct {
	sshClient  *ssh.Client
	sftpClient *sftp.Client
	err        chan error
}

// Wait for connection to close
func (c *conn) wait() {
	c.err <- c.sshClient.Conn.Wait()
}

// Closes the connection
func (c *conn) close() error {
	sftpErr := c.sftpClient.Close()
	sshErr := c.sshClient.Close()
	if sftpErr != nil {
		return sftpErr
	}
	return sshErr
}

// Returns an error if closed
func (c *conn) closed() error {
	select {
	case err := <-c.err:
		return err
	default:
	}
	return nil
}

// Open a new connection to the SFTP server.
func (f *Fs) sftpConnection() (c *conn, err error) {
	// Rate limit rate of new connections
	err = f.connLimit.Wait(context.Background())
	if err != nil {
		return nil, errors.Wrap(err, "limiter failed in connect")
	}
	c = &conn{
		err: make(chan error, 1),
	}
	c.sshClient, err = Dial("tcp", f.host+":"+f.port, f.config)
	if err != nil {
		return nil, errors.Wrap(err, "couldn't connect SSH")
	}
	c.sftpClient, err = sftp.NewClient(c.sshClient)
	if err != nil {
		_ = c.sshClient.Close()
		return nil, errors.Wrap(err, "couldn't initialise SFTP")
	}
	go c.wait()
	return c, nil
}

// Get an SFTP connection from the pool, or open a new one
func (f *Fs) getSftpConnection() (c *conn, err error) {
	f.poolMu.Lock()
	for len(f.pool) > 0 {
		c = f.pool[0]
		f.pool = f.pool[1:]
		err := c.closed()
		if err == nil {
			break
		}
		fs.Errorf(f, "Discarding closed SSH connection: %v", err)
		c = nil
	}
	f.poolMu.Unlock()
	if c != nil {
		return c, nil
	}
	return f.sftpConnection()
}

// Return an SFTP connection to the pool
//
// It nils the pointed to connection out so it can't be reused
//
// if err is not nil then it checks the connection is alive using a
// Getwd request
func (f *Fs) putSftpConnection(pc **conn, err error) {
	c := *pc
	*pc = nil
	if err != nil {
		// work out if this is an expected error
		underlyingErr := errors.Cause(err)
		isRegularError := false
		switch underlyingErr {
		case os.ErrNotExist:
			isRegularError = true
		default:
			switch underlyingErr.(type) {
			case *sftp.StatusError, *os.PathError:
				isRegularError = true
			}
		}
		// If not a regular SFTP error code then check the connection
		if !isRegularError {
			_, nopErr := c.sftpClient.Getwd()
			if nopErr != nil {
				fs.Debugf(f, "Connection failed, closing: %v", nopErr)
				_ = c.close()
				return
			}
			fs.Debugf(f, "Connection OK after error: %v", err)
		}
	}
	f.poolMu.Lock()
	f.pool = append(f.pool, c)
	f.poolMu.Unlock()
}

// NewFs creates a new Fs object from the name and root. It connects to
// the host specified in the config file.
func NewFs(name, root string) (fs.Fs, error) {
	user := fs.ConfigFileGet(name, "user")
	host := fs.ConfigFileGet(name, "host")
	port := fs.ConfigFileGet(name, "port")
	pass := fs.ConfigFileGet(name, "pass")
	keyFile := fs.ConfigFileGet(name, "key_file")
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

	// Add ssh agent-auth if no password or file specified
	if pass == "" && keyFile == "" {
		sshAgentClient, _, err := sshagent.New()
		if err != nil {
			return nil, errors.Wrap(err, "couldn't connect to ssh-agent")
		}
		signers, err := sshAgentClient.Signers()
		if err != nil {
			return nil, errors.Wrap(err, "couldn't read ssh agent signers")
		}
		config.Auth = append(config.Auth, ssh.PublicKeys(signers...))
	}

	// Load key file if specified
	if keyFile != "" {
		key, err := ioutil.ReadFile(keyFile)
		if err != nil {
			return nil, errors.Wrap(err, "failed to read private key file")
		}
		signer, err := ssh.ParsePrivateKey(key)
		if err != nil {
			return nil, errors.Wrap(err, "failed to parse private key file")
		}
		config.Auth = append(config.Auth, ssh.PublicKeys(signer))
	}

	// Auth from password if specified
	if pass != "" {
		clearpass, err := fs.Reveal(pass)
		if err != nil {
			return nil, err
		}
		config.Auth = append(config.Auth, ssh.Password(clearpass))
	}

	f := &Fs{
		name:      name,
		root:      root,
		config:    config,
		host:      host,
		port:      port,
		url:       "sftp://" + user + "@" + host + ":" + port + "/" + root,
		mkdirLock: newStringLock(),
		connLimit: rate.NewLimiter(rate.Limit(connectionsPerSecond), 1),
	}
	f.features = (&fs.Features{}).Fill(f)
	// Make a connection and pool it to return errors early
	c, err := f.getSftpConnection()
	if err != nil {
		return nil, errors.Wrap(err, "NewFs")
	}
	f.putSftpConnection(&c, nil)
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
	c, err := f.getSftpConnection()
	if err != nil {
		return false, errors.Wrap(err, "dirExists")
	}
	info, err := c.sftpClient.Stat(dir)
	f.putSftpConnection(&c, err)
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

// List the objects and directories in dir into entries.  The
// entries can be returned in any order but should be for a
// complete directory.
//
// dir should be "" to list the root, and should not have
// trailing slashes.
//
// This should return ErrDirNotFound if the directory isn't
// found.
func (f *Fs) List(dir string) (entries fs.DirEntries, err error) {
	root := path.Join(f.root, dir)
	ok, err := f.dirExists(root)
	if err != nil {
		return nil, errors.Wrap(err, "List failed")
	}
	if !ok {
		return nil, fs.ErrorDirNotFound
	}
	sftpDir := root
	if sftpDir == "" {
		sftpDir = "."
	}
	c, err := f.getSftpConnection()
	if err != nil {
		return nil, errors.Wrap(err, "List")
	}
	infos, err := c.sftpClient.ReadDir(sftpDir)
	f.putSftpConnection(&c, err)
	if err != nil {
		return nil, errors.Wrapf(err, "error listing %q", dir)
	}
	for _, info := range infos {
		remote := path.Join(dir, info.Name())
		if info.IsDir() {
			d := fs.NewDir(remote, info.ModTime())
			entries = append(entries, d)
		} else {
			o := &Object{
				fs:     f,
				remote: remote,
			}
			o.setMetadata(info)
			entries = append(entries, o)
		}
	}
	return entries, nil
}

// Put data from <in> into a new remote sftp file object described by <src.Remote()> and <src.ModTime()>
func (f *Fs) Put(in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error) {
	err := f.mkParentDir(src.Remote())
	if err != nil {
		return nil, errors.Wrap(err, "Put mkParentDir failed")
	}
	// Temporary object under construction
	o := &Object{
		fs:     f,
		remote: src.Remote(),
	}
	err = o.Update(in, src, options...)
	if err != nil {
		return nil, err
	}
	return o, nil
}

// PutStream uploads to the remote path with the modTime given of indeterminate size
func (f *Fs) PutStream(in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error) {
	return f.Put(in, src, options...)
}

// mkParentDir makes the parent of remote if necessary and any
// directories above that
func (f *Fs) mkParentDir(remote string) error {
	parent := path.Dir(remote)
	return f.mkdir(path.Join(f.root, parent))
}

// mkdir makes the directory and parents using native paths
func (f *Fs) mkdir(dirPath string) error {
	f.mkdirLock.Lock(dirPath)
	defer f.mkdirLock.Unlock(dirPath)
	if dirPath == "." || dirPath == "/" {
		return nil
	}
	ok, err := f.dirExists(dirPath)
	if err != nil {
		return errors.Wrap(err, "mkdir dirExists failed")
	}
	if ok {
		return nil
	}
	parent := path.Dir(dirPath)
	err = f.mkdir(parent)
	if err != nil {
		return err
	}
	c, err := f.getSftpConnection()
	if err != nil {
		return errors.Wrap(err, "mkdir")
	}
	err = c.sftpClient.Mkdir(dirPath)
	f.putSftpConnection(&c, err)
	if err != nil {
		return errors.Wrapf(err, "mkdir %q failed", dirPath)
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
	c, err := f.getSftpConnection()
	if err != nil {
		return errors.Wrap(err, "Rmdir")
	}
	err = c.sftpClient.Remove(root)
	f.putSftpConnection(&c, err)
	return err
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
	c, err := f.getSftpConnection()
	if err != nil {
		return nil, errors.Wrap(err, "Move")
	}
	err = c.sftpClient.Rename(
		srcObj.path(),
		path.Join(f.root, remote),
	)
	f.putSftpConnection(&c, err)
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
	c, err := f.getSftpConnection()
	if err != nil {
		return errors.Wrap(err, "DirMove")
	}
	err = c.sftpClient.Rename(
		srcPath,
		dstPath,
	)
	f.putSftpConnection(&c, err)
	if err != nil {
		return errors.Wrapf(err, "DirMove Rename(%q,%q) failed", srcPath, dstPath)
	}
	return nil
}

// Hashes returns the supported hash types of the filesystem
func (f *Fs) Hashes() fs.HashSet {
	if f.cachedHashes != nil {
		return *f.cachedHashes
	}

	c, err := f.getSftpConnection()
	if err != nil {
		fs.Errorf(f, "Couldn't get SSH connection to figure out Hashes: %v", err)
		return fs.HashSet(fs.HashNone)
	}
	defer f.putSftpConnection(&c, err)
	session, err := c.sshClient.NewSession()
	if err != nil {
		return fs.HashSet(fs.HashNone)
	}
	sha1Output, _ := session.Output("echo 'abc' | sha1sum")
	expectedSha1 := "03cfd743661f07975fa2f1220c5194cbaff48451"
	_ = session.Close()

	session, err = c.sshClient.NewSession()
	if err != nil {
		return fs.HashSet(fs.HashNone)
	}
	md5Output, _ := session.Output("echo 'abc' | md5sum")
	expectedMd5 := "0bee89b07a248e27c83fc3d5951213c1"
	_ = session.Close()

	sha1Works := parseHash(sha1Output) == expectedSha1
	md5Works := parseHash(md5Output) == expectedMd5

	set := fs.NewHashSet()
	if !sha1Works && !md5Works {
		set.Add(fs.HashNone)
	}
	if sha1Works {
		set.Add(fs.HashSHA1)
	}
	if md5Works {
		set.Add(fs.HashMD5)
	}

	_ = session.Close()
	f.cachedHashes = &set
	return set
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

// Hash returns the selected checksum of the file
// If no checksum is available it returns ""
func (o *Object) Hash(r fs.HashType) (string, error) {
	if r == fs.HashMD5 && o.md5sum != nil {
		return *o.md5sum, nil
	} else if r == fs.HashSHA1 && o.sha1sum != nil {
		return *o.sha1sum, nil
	}

	c, err := o.fs.getSftpConnection()
	if err != nil {
		return "", errors.Wrap(err, "Hash")
	}
	session, err := c.sshClient.NewSession()
	o.fs.putSftpConnection(&c, err)
	if err != nil {
		o.fs.cachedHashes = nil // Something has changed on the remote system
		return "", fs.ErrHashUnsupported
	}

	err = fs.ErrHashUnsupported
	var outputBytes []byte
	escapedPath := shellEscape(o.path())
	if r == fs.HashMD5 {
		outputBytes, err = session.Output("md5sum " + escapedPath)
	} else if r == fs.HashSHA1 {
		outputBytes, err = session.Output("sha1sum " + escapedPath)
	}

	if err != nil {
		o.fs.cachedHashes = nil // Something has changed on the remote system
		_ = session.Close()
		return "", fs.ErrHashUnsupported
	}

	_ = session.Close()
	str := parseHash(outputBytes)
	if r == fs.HashMD5 {
		o.md5sum = &str
	} else if r == fs.HashSHA1 {
		o.sha1sum = &str
	}
	return str, nil
}

var shellEscapeRegex = regexp.MustCompile(`[^A-Za-z0-9_.,:/@\n-]`)

// Escape a string s.t. it cannot cause unintended behavior
// when sending it to a shell.
func shellEscape(str string) string {
	safe := shellEscapeRegex.ReplaceAllString(str, `\$0`)
	return strings.Replace(safe, "\n", "'\n'", -1)
}

// Converts a byte array from the SSH session returned by
// an invocation of md5sum/sha1sum to a hash string
// as expected by the rest of this application
func parseHash(bytes []byte) string {
	return strings.Split(string(bytes), " ")[0] // Split at hash / filename separator
}

// Size returns the size in bytes of the remote sftp file
func (o *Object) Size() int64 {
	return o.size
}

// ModTime returns the modification time of the remote sftp file
func (o *Object) ModTime() time.Time {
	return o.modTime
}

// path returns the native path of the object
func (o *Object) path() string {
	return path.Join(o.fs.root, o.remote)
}

// setMetadata updates the info in the object from the stat result passed in
func (o *Object) setMetadata(info os.FileInfo) {
	o.modTime = info.ModTime()
	o.size = info.Size()
	o.mode = info.Mode()
}

// stat updates the info in the Object
func (o *Object) stat() error {
	c, err := o.fs.getSftpConnection()
	if err != nil {
		return errors.Wrap(err, "stat")
	}
	info, err := c.sftpClient.Stat(o.path())
	o.fs.putSftpConnection(&c, err)
	if err != nil {
		if os.IsNotExist(err) {
			return fs.ErrorObjectNotFound
		}
		return errors.Wrap(err, "stat failed")
	}
	if info.IsDir() {
		return errors.Wrapf(fs.ErrorNotAFile, "%q", o.remote)
	}
	o.setMetadata(info)
	return nil
}

// SetModTime sets the modification and access time to the specified time
//
// it also updates the info field
func (o *Object) SetModTime(modTime time.Time) error {
	c, err := o.fs.getSftpConnection()
	if err != nil {
		return errors.Wrap(err, "SetModTime")
	}
	err = c.sftpClient.Chtimes(o.path(), modTime, modTime)
	o.fs.putSftpConnection(&c, err)
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
	return o.mode.IsRegular()
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
	c, err := o.fs.getSftpConnection()
	if err != nil {
		return nil, errors.Wrap(err, "Open")
	}
	sftpFile, err := c.sftpClient.Open(o.path())
	o.fs.putSftpConnection(&c, err)
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
func (o *Object) Update(in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) error {
	// Clear the hash cache since we are about to update the object
	o.md5sum = nil
	o.sha1sum = nil
	c, err := o.fs.getSftpConnection()
	if err != nil {
		return errors.Wrap(err, "Update")
	}
	file, err := c.sftpClient.Create(o.path())
	o.fs.putSftpConnection(&c, err)
	if err != nil {
		return errors.Wrap(err, "Update Create failed")
	}
	// remove the file if upload failed
	remove := func() {
		c, removeErr := o.fs.getSftpConnection()
		if removeErr != nil {
			fs.Debugf(src, "Failed to open new SSH connection for delete: %v", removeErr)
			return
		}
		removeErr = c.sftpClient.Remove(o.path())
		o.fs.putSftpConnection(&c, removeErr)
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
	c, err := o.fs.getSftpConnection()
	if err != nil {
		return errors.Wrap(err, "Remove")
	}
	err = c.sftpClient.Remove(o.path())
	o.fs.putSftpConnection(&c, err)
	return err
}

// Check the interfaces are satisfied
var (
	_ fs.Fs          = &Fs{}
	_ fs.PutStreamer = &Fs{}
	_ fs.Mover       = &Fs{}
	_ fs.DirMover    = &Fs{}
	_ fs.Object      = &Object{}
)
