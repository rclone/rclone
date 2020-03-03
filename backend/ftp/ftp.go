// Package ftp interfaces with FTP servers
package ftp

import (
	"context"
	"crypto/tls"
	"io"
	"net"
	"net/textproto"
	"path"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/jlaffaye/ftp"
	"github.com/pkg/errors"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/accounting"
	"github.com/rclone/rclone/fs/config"
	"github.com/rclone/rclone/fs/config/configmap"
	"github.com/rclone/rclone/fs/config/configstruct"
	"github.com/rclone/rclone/fs/config/obscure"
	"github.com/rclone/rclone/fs/fserrors"
	"github.com/rclone/rclone/fs/fshttp"
	"github.com/rclone/rclone/fs/hash"
	"github.com/rclone/rclone/lib/encoder"
	"github.com/rclone/rclone/lib/env"
	"github.com/rclone/rclone/lib/pacer"
	"github.com/rclone/rclone/lib/readers"
)

var (
	currentUser = env.CurrentUser()
)

const (
	minSleep      = 10 * time.Millisecond
	maxSleep      = 2 * time.Second
	decayConstant = 2 // bigger for slower decay, exponential
)

// Register with Fs
func init() {
	fs.Register(&fs.RegInfo{
		Name:        "ftp",
		Description: "FTP Connection",
		NewFs:       NewFs,
		Options: []fs.Option{{
			Name:     "host",
			Help:     "FTP host to connect to",
			Required: true,
			Examples: []fs.OptionExample{{
				Value: "ftp.example.com",
				Help:  "Connect to ftp.example.com",
			}},
		}, {
			Name: "user",
			Help: "FTP username, leave blank for current username, " + currentUser,
		}, {
			Name: "port",
			Help: "FTP port, leave blank to use default (21)",
		}, {
			Name:       "pass",
			Help:       "FTP password",
			IsPassword: true,
			Required:   true,
		}, {
			Name: "tls",
			Help: `Use Implicit FTPS (FTP over TLS)
When using implicit FTP over TLS the client connects using TLS
right from the start which breaks compatibility with
non-TLS-aware servers. This is usually served over port 990 rather
than port 21. Cannot be used in combination with explicit FTP.`,
			Default: false,
		}, {
			Name: "explicit_tls",
			Help: `Use Explicit FTPS (FTP over TLS)
When using explicit FTP over TLS the client explicitly requests
security from the server in order to upgrade a plain text connection
to an encrypted one. Cannot be used in combination with implicit FTP.`,
			Default: false,
		}, {
			Name:     "concurrency",
			Help:     "Maximum number of FTP simultaneous connections, 0 for unlimited",
			Default:  0,
			Advanced: true,
		}, {
			Name:     "no_check_certificate",
			Help:     "Do not verify the TLS certificate of the server",
			Default:  false,
			Advanced: true,
		}, {
			Name:     "disable_epsv",
			Help:     "Disable using EPSV even if server advertises support",
			Default:  false,
			Advanced: true,
		}, {
			Name:     "disable_mlsd",
			Help:     "Disable using MLSD even if server advertises support",
			Default:  false,
			Advanced: true,
		}, {
			Name:    "idle_timeout",
			Default: fs.Duration(60 * time.Second),
			Help: `Max time before closing idle connections

If no connections have been returned to the connection pool in the time
given, rclone will empty the connection pool.

Set to 0 to keep connections indefinitely.
`,
			Advanced: true,
		}, {
			Name:     "close_timeout",
			Help:     "Maximum time to wait for a response to close.",
			Default:  fs.Duration(60 * time.Second),
			Advanced: true,
		}, {
			Name:     config.ConfigEncoding,
			Help:     config.ConfigEncodingHelp,
			Advanced: true,
			// The FTP protocol can't handle trailing spaces (for instance
			// pureftpd turns them into _)
			//
			// proftpd can't handle '*' in file names
			// pureftpd can't handle '[', ']' or '*'
			Default: (encoder.Display |
				encoder.EncodeRightSpace),
		}},
	})
}

// Options defines the configuration for this backend
type Options struct {
	Host              string               `config:"host"`
	User              string               `config:"user"`
	Pass              string               `config:"pass"`
	Port              string               `config:"port"`
	TLS               bool                 `config:"tls"`
	ExplicitTLS       bool                 `config:"explicit_tls"`
	Concurrency       int                  `config:"concurrency"`
	SkipVerifyTLSCert bool                 `config:"no_check_certificate"`
	DisableEPSV       bool                 `config:"disable_epsv"`
	DisableMLSD       bool                 `config:"disable_mlsd"`
	IdleTimeout       fs.Duration          `config:"idle_timeout"`
	CloseTimeout      fs.Duration          `config:"close_timeout"`
	Enc               encoder.MultiEncoder `config:"encoding"`
}

// Fs represents a remote FTP server
type Fs struct {
	name     string         // name of this remote
	root     string         // the path we are working on if any
	opt      Options        // parsed options
	ci       *fs.ConfigInfo // global config
	features *fs.Features   // optional features
	url      string
	user     string
	pass     string
	dialAddr string
	poolMu   sync.Mutex
	pool     []*ftp.ServerConn
	drain    *time.Timer // used to drain the pool when we stop using the connections
	tokens   *pacer.TokenDispenser
	tlsConf  *tls.Config
	pacer    *fs.Pacer // pacer for FTP connections
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
	return f.url
}

// Features returns the optional features of this Fs
func (f *Fs) Features() *fs.Features {
	return f.features
}

// Enable debugging output
type debugLog struct {
	mu   sync.Mutex
	auth bool
}

// Write writes len(p) bytes from p to the underlying data stream. It returns
// the number of bytes written from p (0 <= n <= len(p)) and any error
// encountered that caused the write to stop early. Write must return a non-nil
// error if it returns n < len(p). Write must not modify the slice data, even
// temporarily.
//
// Implementations must not retain p.
//
// This writes debug info to the log
func (dl *debugLog) Write(p []byte) (n int, err error) {
	dl.mu.Lock()
	defer dl.mu.Unlock()
	_, file, _, ok := runtime.Caller(1)
	direction := "FTP Rx"
	if ok && strings.Contains(file, "multi") {
		direction = "FTP Tx"
	}
	lines := strings.Split(string(p), "\r\n")
	if lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	for _, line := range lines {
		if !dl.auth && strings.HasPrefix(line, "PASS") {
			fs.Debugf(direction, "PASS *****")
			continue
		}
		fs.Debugf(direction, "%q", line)
	}
	return len(p), nil
}

type dialCtx struct {
	f   *Fs
	ctx context.Context
}

// dial a new connection with fshttp dialer
func (d *dialCtx) dial(network, address string) (net.Conn, error) {
	conn, err := fshttp.NewDialer(d.ctx).Dial(network, address)
	if err != nil {
		return nil, err
	}
	if d.f.tlsConf != nil {
		conn = tls.Client(conn, d.f.tlsConf)
	}
	return conn, err
}

// shouldRetry returns a boolean as to whether this err deserve to be
// retried.  It returns the err as a convenience
func shouldRetry(ctx context.Context, err error) (bool, error) {
	if fserrors.ContextError(ctx, &err) {
		return false, err
	}
	switch errX := err.(type) {
	case *textproto.Error:
		switch errX.Code {
		case ftp.StatusNotAvailable:
			return true, err
		}
	}
	return fserrors.ShouldRetry(err), err
}

// Open a new connection to the FTP server.
func (f *Fs) ftpConnection(ctx context.Context) (c *ftp.ServerConn, err error) {
	fs.Debugf(f, "Connecting to FTP server")
	dCtx := dialCtx{f, ctx}
	ftpConfig := []ftp.DialOption{ftp.DialWithDialFunc(dCtx.dial)}
	if f.opt.ExplicitTLS {
		ftpConfig = append(ftpConfig, ftp.DialWithExplicitTLS(f.tlsConf))
		// Initial connection needs to be cleartext for explicit TLS
		conn, err := fshttp.NewDialer(ctx).Dial("tcp", f.dialAddr)
		if err != nil {
			return nil, err
		}
		ftpConfig = append(ftpConfig, ftp.DialWithNetConn(conn))
	}
	if f.opt.DisableEPSV {
		ftpConfig = append(ftpConfig, ftp.DialWithDisabledEPSV(true))
	}
	if f.opt.DisableMLSD {
		ftpConfig = append(ftpConfig, ftp.DialWithDisabledMLSD(true))
	}
	if f.ci.Dump&(fs.DumpHeaders|fs.DumpBodies|fs.DumpRequests|fs.DumpResponses) != 0 {
		ftpConfig = append(ftpConfig, ftp.DialWithDebugOutput(&debugLog{auth: f.ci.Dump&fs.DumpAuth != 0}))
	}
	err = f.pacer.Call(func() (bool, error) {
		c, err = ftp.Dial(f.dialAddr, ftpConfig...)
		if err != nil {
			return shouldRetry(ctx, err)
		}
		err = c.Login(f.user, f.pass)
		if err != nil {
			_ = c.Quit()
			return shouldRetry(ctx, err)
		}
		return false, nil
	})
	if err != nil {
		err = errors.Wrapf(err, "failed to make FTP connection to %q", f.dialAddr)
	}
	return c, err
}

// Get an FTP connection from the pool, or open a new one
func (f *Fs) getFtpConnection(ctx context.Context) (c *ftp.ServerConn, err error) {
	if f.opt.Concurrency > 0 {
		f.tokens.Get()
	}
	accounting.LimitTPS(ctx)
	f.poolMu.Lock()
	if len(f.pool) > 0 {
		c = f.pool[0]
		f.pool = f.pool[1:]
	}
	f.poolMu.Unlock()
	if c != nil {
		return c, nil
	}
	c, err = f.ftpConnection(ctx)
	if err != nil && f.opt.Concurrency > 0 {
		f.tokens.Put()
	}
	return c, err
}

// Return an FTP connection to the pool
//
// It nils the pointed to connection out so it can't be reused
//
// if err is not nil then it checks the connection is alive using a
// NOOP request
func (f *Fs) putFtpConnection(pc **ftp.ServerConn, err error) {
	if f.opt.Concurrency > 0 {
		defer f.tokens.Put()
	}
	if pc == nil {
		return
	}
	c := *pc
	if c == nil {
		return
	}
	*pc = nil
	if err != nil {
		// If not a regular FTP error code then check the connection
		_, isRegularError := errors.Cause(err).(*textproto.Error)
		if !isRegularError {
			nopErr := c.NoOp()
			if nopErr != nil {
				fs.Debugf(f, "Connection failed, closing: %v", nopErr)
				_ = c.Quit()
				return
			}
		}
	}
	f.poolMu.Lock()
	f.pool = append(f.pool, c)
	if f.opt.IdleTimeout > 0 {
		f.drain.Reset(time.Duration(f.opt.IdleTimeout)) // nudge on the pool emptying timer
	}
	f.poolMu.Unlock()
}

// Drain the pool of any connections
func (f *Fs) drainPool(ctx context.Context) (err error) {
	f.poolMu.Lock()
	defer f.poolMu.Unlock()
	if f.opt.IdleTimeout > 0 {
		f.drain.Stop()
	}
	if len(f.pool) != 0 {
		fs.Debugf(f, "closing %d unused connections", len(f.pool))
	}
	for i, c := range f.pool {
		if cErr := c.Quit(); cErr != nil {
			err = cErr
		}
		f.pool[i] = nil
	}
	f.pool = nil
	return err
}

// NewFs constructs an Fs from the path, container:path
func NewFs(ctx context.Context, name, root string, m configmap.Mapper) (ff fs.Fs, err error) {
	// defer fs.Trace(nil, "name=%q, root=%q", name, root)("fs=%v, err=%v", &ff, &err)
	// Parse config into Options struct
	opt := new(Options)
	err = configstruct.Set(m, opt)
	if err != nil {
		return nil, err
	}
	pass, err := obscure.Reveal(opt.Pass)
	if err != nil {
		return nil, errors.Wrap(err, "NewFS decrypt password")
	}
	user := opt.User
	if user == "" {
		user = currentUser
	}
	port := opt.Port
	if port == "" {
		port = "21"
	}

	dialAddr := opt.Host + ":" + port
	protocol := "ftp://"
	if opt.TLS {
		protocol = "ftps://"
	}
	if opt.TLS && opt.ExplicitTLS {
		return nil, errors.New("Implicit TLS and explicit TLS are mutually incompatible. Please revise your config")
	}
	var tlsConfig *tls.Config
	if opt.TLS || opt.ExplicitTLS {
		tlsConfig = &tls.Config{
			ServerName:         opt.Host,
			InsecureSkipVerify: opt.SkipVerifyTLSCert,
		}
	}
	u := protocol + path.Join(dialAddr+"/", root)
	ci := fs.GetConfig(ctx)
	f := &Fs{
		name:     name,
		root:     root,
		opt:      *opt,
		ci:       ci,
		url:      u,
		user:     user,
		pass:     pass,
		dialAddr: dialAddr,
		tokens:   pacer.NewTokenDispenser(opt.Concurrency),
		tlsConf:  tlsConfig,
		pacer:    fs.NewPacer(ctx, pacer.NewDefault(pacer.MinSleep(minSleep), pacer.MaxSleep(maxSleep), pacer.DecayConstant(decayConstant))),
	}
	f.features = (&fs.Features{
		CanHaveEmptyDirectories: true,
	}).Fill(ctx, f)
	// set the pool drainer timer going
	if f.opt.IdleTimeout > 0 {
		f.drain = time.AfterFunc(time.Duration(opt.IdleTimeout), func() { _ = f.drainPool(ctx) })
	}
	// Make a connection and pool it to return errors early
	c, err := f.getFtpConnection(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "NewFs")
	}
	f.putFtpConnection(&c, nil)
	if root != "" {
		// Check to see if the root actually an existing file
		remote := path.Base(root)
		f.root = path.Dir(root)
		if f.root == "." {
			f.root = ""
		}
		_, err := f.NewObject(ctx, remote)
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

// Shutdown the backend, closing any background tasks and any
// cached connections.
func (f *Fs) Shutdown(ctx context.Context) error {
	return f.drainPool(ctx)
}

// translateErrorFile turns FTP errors into rclone errors if possible for a file
func translateErrorFile(err error) error {
	switch errX := err.(type) {
	case *textproto.Error:
		switch errX.Code {
		case ftp.StatusFileUnavailable, ftp.StatusFileActionIgnored:
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
		case ftp.StatusFileUnavailable, ftp.StatusFileActionIgnored:
			err = fs.ErrorDirNotFound
		}
	}
	return err
}

// entryToStandard converts an incoming ftp.Entry to Standard encoding
func (f *Fs) entryToStandard(entry *ftp.Entry) {
	// Skip . and .. as we don't want these encoded
	if entry.Name == "." || entry.Name == ".." {
		return
	}
	entry.Name = f.opt.Enc.ToStandardName(entry.Name)
	entry.Target = f.opt.Enc.ToStandardPath(entry.Target)
}

// dirFromStandardPath returns dir in encoded form.
func (f *Fs) dirFromStandardPath(dir string) string {
	// Skip . and .. as we don't want these encoded
	if dir == "." || dir == ".." {
		return dir
	}
	return f.opt.Enc.FromStandardPath(dir)
}

// findItem finds a directory entry for the name in its parent directory
func (f *Fs) findItem(ctx context.Context, remote string) (entry *ftp.Entry, err error) {
	// defer fs.Trace(remote, "")("o=%v, err=%v", &o, &err)
	fullPath := path.Join(f.root, remote)
	if fullPath == "" || fullPath == "." || fullPath == "/" {
		// if root, assume exists and synthesize an entry
		return &ftp.Entry{
			Name: "",
			Type: ftp.EntryTypeFolder,
			Time: time.Now(),
		}, nil
	}
	dir := path.Dir(fullPath)
	base := path.Base(fullPath)

	c, err := f.getFtpConnection(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "findItem")
	}
	files, err := c.List(f.dirFromStandardPath(dir))
	f.putFtpConnection(&c, err)
	if err != nil {
		return nil, translateErrorFile(err)
	}
	for _, file := range files {
		f.entryToStandard(file)
		if file.Name == base {
			return file, nil
		}
	}
	return nil, nil
}

// NewObject finds the Object at remote.  If it can't be found
// it returns the error fs.ErrorObjectNotFound.
func (f *Fs) NewObject(ctx context.Context, remote string) (o fs.Object, err error) {
	// defer fs.Trace(remote, "")("o=%v, err=%v", &o, &err)
	entry, err := f.findItem(ctx, remote)
	if err != nil {
		return nil, err
	}
	if entry != nil && entry.Type != ftp.EntryTypeFolder {
		o := &Object{
			fs:     f,
			remote: remote,
		}
		info := &FileInfo{
			Name:    remote,
			Size:    entry.Size,
			ModTime: entry.Time,
		}
		o.info = info

		return o, nil
	}
	return nil, fs.ErrorObjectNotFound
}

// dirExists checks the directory pointed to by remote exists or not
func (f *Fs) dirExists(ctx context.Context, remote string) (exists bool, err error) {
	entry, err := f.findItem(ctx, remote)
	if err != nil {
		return false, errors.Wrap(err, "dirExists")
	}
	if entry != nil && entry.Type == ftp.EntryTypeFolder {
		return true, nil
	}
	return false, nil
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
func (f *Fs) List(ctx context.Context, dir string) (entries fs.DirEntries, err error) {
	// defer log.Trace(dir, "dir=%q", dir)("entries=%v, err=%v", &entries, &err)
	c, err := f.getFtpConnection(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "list")
	}

	var listErr error
	var files []*ftp.Entry

	resultchan := make(chan []*ftp.Entry, 1)
	errchan := make(chan error, 1)
	go func() {
		result, err := c.List(f.dirFromStandardPath(path.Join(f.root, dir)))
		f.putFtpConnection(&c, err)
		if err != nil {
			errchan <- err
			return
		}
		resultchan <- result
	}()

	// Wait for List for up to Timeout seconds
	timer := time.NewTimer(f.ci.TimeoutOrInfinite())
	select {
	case listErr = <-errchan:
		timer.Stop()
		return nil, translateErrorDir(listErr)
	case files = <-resultchan:
		timer.Stop()
	case <-timer.C:
		// if timer fired assume no error but connection dead
		fs.Errorf(f, "Timeout when waiting for List")
		return nil, errors.New("Timeout when waiting for List")
	}

	// Annoyingly FTP returns success for a directory which
	// doesn't exist, so check it really doesn't exist if no
	// entries found.
	if len(files) == 0 {
		exists, err := f.dirExists(ctx, dir)
		if err != nil {
			return nil, errors.Wrap(err, "list")
		}
		if !exists {
			return nil, fs.ErrorDirNotFound
		}
	}
	for i := range files {
		object := files[i]
		f.entryToStandard(object)
		newremote := path.Join(dir, object.Name)
		switch object.Type {
		case ftp.EntryTypeFolder:
			if object.Name == "." || object.Name == ".." {
				continue
			}
			d := fs.NewDir(newremote, object.Time)
			entries = append(entries, d)
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
			entries = append(entries, o)
		}
	}
	return entries, nil
}

// Hashes are not supported
func (f *Fs) Hashes() hash.Set {
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
func (f *Fs) Put(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error) {
	// fs.Debugf(f, "Trying to put file %s", src.Remote())
	err := f.mkParentDir(ctx, src.Remote())
	if err != nil {
		return nil, errors.Wrap(err, "Put mkParentDir failed")
	}
	o := &Object{
		fs:     f,
		remote: src.Remote(),
	}
	err = o.Update(ctx, in, src, options...)
	return o, err
}

// PutStream uploads to the remote path with the modTime given of indeterminate size
func (f *Fs) PutStream(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error) {
	return f.Put(ctx, in, src, options...)
}

// getInfo reads the FileInfo for a path
func (f *Fs) getInfo(ctx context.Context, remote string) (fi *FileInfo, err error) {
	// defer fs.Trace(remote, "")("fi=%v, err=%v", &fi, &err)
	dir := path.Dir(remote)
	base := path.Base(remote)

	c, err := f.getFtpConnection(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "getInfo")
	}
	files, err := c.List(f.dirFromStandardPath(dir))
	f.putFtpConnection(&c, err)
	if err != nil {
		return nil, translateErrorFile(err)
	}

	for i := range files {
		file := files[i]
		f.entryToStandard(file)
		if file.Name == base {
			info := &FileInfo{
				Name:    remote,
				Size:    file.Size,
				ModTime: file.Time,
				IsDir:   file.Type == ftp.EntryTypeFolder,
			}
			return info, nil
		}
	}
	return nil, fs.ErrorObjectNotFound
}

// mkdir makes the directory and parents using unrooted paths
func (f *Fs) mkdir(ctx context.Context, abspath string) error {
	abspath = path.Clean(abspath)
	if abspath == "." || abspath == "/" {
		return nil
	}
	fi, err := f.getInfo(ctx, abspath)
	if err == nil {
		if fi.IsDir {
			return nil
		}
		return fs.ErrorIsFile
	} else if err != fs.ErrorObjectNotFound {
		return errors.Wrapf(err, "mkdir %q failed", abspath)
	}
	parent := path.Dir(abspath)
	err = f.mkdir(ctx, parent)
	if err != nil {
		return err
	}
	c, connErr := f.getFtpConnection(ctx)
	if connErr != nil {
		return errors.Wrap(connErr, "mkdir")
	}
	err = c.MakeDir(f.dirFromStandardPath(abspath))
	f.putFtpConnection(&c, err)
	switch errX := err.(type) {
	case *textproto.Error:
		switch errX.Code {
		case ftp.StatusFileUnavailable: // dir already exists: see issue #2181
			err = nil
		case 521: // dir already exists: error number according to RFC 959: issue #2363
			err = nil
		}
	}
	return err
}

// mkParentDir makes the parent of remote if necessary and any
// directories above that
func (f *Fs) mkParentDir(ctx context.Context, remote string) error {
	parent := path.Dir(remote)
	return f.mkdir(ctx, path.Join(f.root, parent))
}

// Mkdir creates the directory if it doesn't exist
func (f *Fs) Mkdir(ctx context.Context, dir string) (err error) {
	// defer fs.Trace(dir, "")("err=%v", &err)
	root := path.Join(f.root, dir)
	return f.mkdir(ctx, root)
}

// Rmdir removes the directory (container, bucket) if empty
//
// Return an error if it doesn't exist or isn't empty
func (f *Fs) Rmdir(ctx context.Context, dir string) error {
	c, err := f.getFtpConnection(ctx)
	if err != nil {
		return errors.Wrap(translateErrorFile(err), "Rmdir")
	}
	err = c.RemoveDir(f.dirFromStandardPath(path.Join(f.root, dir)))
	f.putFtpConnection(&c, err)
	return translateErrorDir(err)
}

// Move renames a remote file object
func (f *Fs) Move(ctx context.Context, src fs.Object, remote string) (fs.Object, error) {
	srcObj, ok := src.(*Object)
	if !ok {
		fs.Debugf(src, "Can't move - not same remote type")
		return nil, fs.ErrorCantMove
	}
	err := f.mkParentDir(ctx, remote)
	if err != nil {
		return nil, errors.Wrap(err, "Move mkParentDir failed")
	}
	c, err := f.getFtpConnection(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "Move")
	}
	err = c.Rename(
		f.opt.Enc.FromStandardPath(path.Join(srcObj.fs.root, srcObj.remote)),
		f.opt.Enc.FromStandardPath(path.Join(f.root, remote)),
	)
	f.putFtpConnection(&c, err)
	if err != nil {
		return nil, errors.Wrap(err, "Move Rename failed")
	}
	dstObj, err := f.NewObject(ctx, remote)
	if err != nil {
		return nil, errors.Wrap(err, "Move NewObject failed")
	}
	return dstObj, nil
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
	srcPath := path.Join(srcFs.root, srcRemote)
	dstPath := path.Join(f.root, dstRemote)

	// Check if destination exists
	fi, err := f.getInfo(ctx, dstPath)
	if err == nil {
		if fi.IsDir {
			return fs.ErrorDirExists
		}
		return fs.ErrorIsFile
	} else if err != fs.ErrorObjectNotFound {
		return errors.Wrapf(err, "DirMove getInfo failed")
	}

	// Make sure the parent directory exists
	err = f.mkdir(ctx, path.Dir(dstPath))
	if err != nil {
		return errors.Wrap(err, "DirMove mkParentDir dst failed")
	}

	// Do the move
	c, err := f.getFtpConnection(ctx)
	if err != nil {
		return errors.Wrap(err, "DirMove")
	}
	err = c.Rename(
		f.dirFromStandardPath(srcPath),
		f.dirFromStandardPath(dstPath),
	)
	f.putFtpConnection(&c, err)
	if err != nil {
		return errors.Wrapf(err, "DirMove Rename(%q,%q) failed", srcPath, dstPath)
	}
	return nil
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
func (o *Object) Hash(ctx context.Context, t hash.Type) (string, error) {
	return "", hash.ErrUnsupported
}

// Size returns the size of an object in bytes
func (o *Object) Size() int64 {
	return int64(o.info.Size)
}

// ModTime returns the modification time of the object
func (o *Object) ModTime(ctx context.Context) time.Time {
	return o.info.ModTime
}

// SetModTime sets the modification time of the object
func (o *Object) SetModTime(ctx context.Context, modTime time.Time) error {
	return nil
}

// Storable returns a boolean as to whether this object is storable
func (o *Object) Storable() bool {
	return true
}

// ftpReadCloser implements io.ReadCloser for FTP objects.
type ftpReadCloser struct {
	rc  io.ReadCloser
	c   *ftp.ServerConn
	f   *Fs
	err error // errors found during read
}

// Read bytes into p
func (f *ftpReadCloser) Read(p []byte) (n int, err error) {
	n, err = f.rc.Read(p)
	if err != nil && err != io.EOF {
		f.err = err // store any errors for Close to examine
	}
	return
}

// Close the FTP reader and return the connection to the pool
func (f *ftpReadCloser) Close() error {
	var err error
	errchan := make(chan error, 1)
	go func() {
		errchan <- f.rc.Close()
	}()
	// Wait for Close for up to 60 seconds by default
	timer := time.NewTimer(time.Duration(f.f.opt.CloseTimeout))
	select {
	case err = <-errchan:
		timer.Stop()
	case <-timer.C:
		// if timer fired assume no error but connection dead
		fs.Errorf(f.f, "Timeout when waiting for connection Close")
		f.f.putFtpConnection(nil, nil)
		return nil
	}
	// if errors while reading or closing, dump the connection
	if err != nil || f.err != nil {
		_ = f.c.Quit()
		f.f.putFtpConnection(nil, nil)
	} else {
		f.f.putFtpConnection(&f.c, nil)
	}
	// mask the error if it was caused by a premature close
	// NB StatusAboutToSend is to work around a bug in pureftpd
	// See: https://github.com/rclone/rclone/issues/3445#issuecomment-521654257
	switch errX := err.(type) {
	case *textproto.Error:
		switch errX.Code {
		case ftp.StatusTransfertAborted, ftp.StatusFileUnavailable, ftp.StatusAboutToSend:
			err = nil
		}
	}
	return err
}

// Open an object for read
func (o *Object) Open(ctx context.Context, options ...fs.OpenOption) (rc io.ReadCloser, err error) {
	// defer fs.Trace(o, "")("rc=%v, err=%v", &rc, &err)
	path := path.Join(o.fs.root, o.remote)
	var offset, limit int64 = 0, -1
	for _, option := range options {
		switch x := option.(type) {
		case *fs.SeekOption:
			offset = x.Offset
		case *fs.RangeOption:
			offset, limit = x.Decode(o.Size())
		default:
			if option.Mandatory() {
				fs.Logf(o, "Unsupported mandatory option: %v", option)
			}
		}
	}
	c, err := o.fs.getFtpConnection(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "open")
	}
	fd, err := c.RetrFrom(o.fs.opt.Enc.FromStandardPath(path), uint64(offset))
	if err != nil {
		o.fs.putFtpConnection(&c, err)
		return nil, errors.Wrap(err, "open")
	}
	rc = &ftpReadCloser{rc: readers.NewLimitedReadCloser(fd, limit), c: c, f: o.fs}
	return rc, nil
}

// Update the already existing object
//
// Copy the reader into the object updating modTime and size
//
// The new object may have been created if an error is returned
func (o *Object) Update(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (err error) {
	// defer fs.Trace(o, "src=%v", src)("err=%v", &err)
	path := path.Join(o.fs.root, o.remote)
	// remove the file if upload failed
	remove := func() {
		// Give the FTP server a chance to get its internal state in order after the error.
		// The error may have been local in which case we closed the connection.  The server
		// may still be dealing with it for a moment. A sleep isn't ideal but I haven't been
		// able to think of a better method to find out if the server has finished - ncw
		time.Sleep(1 * time.Second)
		removeErr := o.Remove(ctx)
		if removeErr != nil {
			fs.Debugf(o, "Failed to remove: %v", removeErr)
		} else {
			fs.Debugf(o, "Removed after failed upload: %v", err)
		}
	}
	c, err := o.fs.getFtpConnection(ctx)
	if err != nil {
		return errors.Wrap(err, "Update")
	}
	err = c.Stor(o.fs.opt.Enc.FromStandardPath(path), in)
	if err != nil {
		_ = c.Quit() // toss this connection to avoid sync errors
		remove()
		o.fs.putFtpConnection(nil, err)
		return errors.Wrap(err, "update stor")
	}
	o.fs.putFtpConnection(&c, nil)
	o.info, err = o.fs.getInfo(ctx, path)
	if err != nil {
		return errors.Wrap(err, "update getinfo")
	}
	return nil
}

// Remove an object
func (o *Object) Remove(ctx context.Context) (err error) {
	// defer fs.Trace(o, "")("err=%v", &err)
	path := path.Join(o.fs.root, o.remote)
	// Check if it's a directory or a file
	info, err := o.fs.getInfo(ctx, path)
	if err != nil {
		return err
	}
	if info.IsDir {
		err = o.fs.Rmdir(ctx, o.remote)
	} else {
		c, err := o.fs.getFtpConnection(ctx)
		if err != nil {
			return errors.Wrap(err, "Remove")
		}
		err = c.Delete(o.fs.opt.Enc.FromStandardPath(path))
		o.fs.putFtpConnection(&c, err)
	}
	return err
}

// Check the interfaces are satisfied
var (
	_ fs.Fs          = &Fs{}
	_ fs.Mover       = &Fs{}
	_ fs.DirMover    = &Fs{}
	_ fs.PutStreamer = &Fs{}
	_ fs.Shutdowner  = &Fs{}
	_ fs.Object      = &Object{}
)
