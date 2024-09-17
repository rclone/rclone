// Package ftp interfaces with FTP servers
package ftp

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net"
	"net/textproto"
	"path"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/jlaffaye/ftp"
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
	"github.com/rclone/rclone/lib/proxy"
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
		Description: "FTP",
		NewFs:       NewFs,
		Options: []fs.Option{{
			Name:      "host",
			Help:      "FTP host to connect to.\n\nE.g. \"ftp.example.com\".",
			Required:  true,
			Sensitive: true,
		}, {
			Name:      "user",
			Help:      "FTP username.",
			Default:   currentUser,
			Sensitive: true,
		}, {
			Name:    "port",
			Help:    "FTP port number.",
			Default: 21,
		}, {
			Name:       "pass",
			Help:       "FTP password.",
			IsPassword: true,
		}, {
			Name: "tls",
			Help: `Use Implicit FTPS (FTP over TLS).

When using implicit FTP over TLS the client connects using TLS
right from the start which breaks compatibility with
non-TLS-aware servers. This is usually served over port 990 rather
than port 21. Cannot be used in combination with explicit FTPS.`,
			Default: false,
		}, {
			Name: "explicit_tls",
			Help: `Use Explicit FTPS (FTP over TLS).

When using explicit FTP over TLS the client explicitly requests
security from the server in order to upgrade a plain text connection
to an encrypted one. Cannot be used in combination with implicit FTPS.`,
			Default: false,
		}, {
			Name: "concurrency",
			Help: strings.ReplaceAll(`Maximum number of FTP simultaneous connections, 0 for unlimited.

Note that setting this is very likely to cause deadlocks so it should
be used with care.

If you are doing a sync or copy then make sure concurrency is one more
than the sum of |--transfers| and |--checkers|.

If you use |--check-first| then it just needs to be one more than the
maximum of |--checkers| and |--transfers|.

So for |concurrency 3| you'd use |--checkers 2 --transfers 2
--check-first| or |--checkers 1 --transfers 1|.

`, "|", "`"),
			Default:  0,
			Advanced: true,
		}, {
			Name:     "no_check_certificate",
			Help:     "Do not verify the TLS certificate of the server.",
			Default:  false,
			Advanced: true,
		}, {
			Name:     "disable_epsv",
			Help:     "Disable using EPSV even if server advertises support.",
			Default:  false,
			Advanced: true,
		}, {
			Name:     "disable_mlsd",
			Help:     "Disable using MLSD even if server advertises support.",
			Default:  false,
			Advanced: true,
		}, {
			Name:     "disable_utf8",
			Help:     "Disable using UTF-8 even if server advertises support.",
			Default:  false,
			Advanced: true,
		}, {
			Name:     "writing_mdtm",
			Help:     "Use MDTM to set modification time (VsFtpd quirk)",
			Default:  false,
			Advanced: true,
		}, {
			Name:     "force_list_hidden",
			Help:     "Use LIST -a to force listing of hidden files and folders. This will disable the use of MLSD.",
			Default:  false,
			Advanced: true,
		}, {
			Name:    "idle_timeout",
			Default: fs.Duration(60 * time.Second),
			Help: `Max time before closing idle connections.

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
			Name: "tls_cache_size",
			Help: `Size of TLS session cache for all control and data connections.

TLS cache allows to resume TLS sessions and reuse PSK between connections.
Increase if default size is not enough resulting in TLS resumption errors.
Enabled by default. Use 0 to disable.`,
			Default:  32,
			Advanced: true,
		}, {
			Name:     "disable_tls13",
			Help:     "Disable TLS 1.3 (workaround for FTP servers with buggy TLS)",
			Default:  false,
			Advanced: true,
		}, {
			Name:     "shut_timeout",
			Help:     "Maximum time to wait for data connection closing status.",
			Default:  fs.Duration(60 * time.Second),
			Advanced: true,
		}, {
			Name:    "ask_password",
			Default: false,
			Help: `Allow asking for FTP password when needed.

If this is set and no password is supplied then rclone will ask for a password
`,
			Advanced: true,
		}, {
			Name:    "socks_proxy",
			Default: "",
			Help: `Socks 5 proxy host.
		
Supports the format user:pass@host:port, user@host:port, host:port.
		
Example:
		
    myUser:myPass@localhost:9005
`,
			Advanced: true,
		}, {
			Name:    "no_check_upload",
			Default: false,
			Help: `Don't check the upload is OK

Normally rclone will try to check the upload exists after it has
uploaded a file to make sure the size and modification time are as
expected.

This flag stops rclone doing these checks. This enables uploading to
folders which are write only.

You will likely need to use the --inplace flag also if uploading to
a write only folder.
`,
			Advanced: true,
		}, {
			Name:     config.ConfigEncoding,
			Help:     config.ConfigEncodingHelp,
			Advanced: true,
			// The FTP protocol can't handle trailing spaces
			// (for instance, pureftpd turns them into '_')
			Default: (encoder.Display |
				encoder.EncodeRightSpace),
			Examples: []fs.OptionExample{{
				Value: "Asterisk,Ctl,Dot,Slash",
				Help:  "ProFTPd can't handle '*' in file names",
			}, {
				Value: "BackSlash,Ctl,Del,Dot,RightSpace,Slash,SquareBracket",
				Help:  "PureFTPd can't handle '[]' or '*' in file names",
			}, {
				Value: "Ctl,LeftPeriod,Slash",
				Help:  "VsFTPd can't handle file names starting with dot",
			}},
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
	TLSCacheSize      int                  `config:"tls_cache_size"`
	DisableTLS13      bool                 `config:"disable_tls13"`
	Concurrency       int                  `config:"concurrency"`
	SkipVerifyTLSCert bool                 `config:"no_check_certificate"`
	DisableEPSV       bool                 `config:"disable_epsv"`
	DisableMLSD       bool                 `config:"disable_mlsd"`
	DisableUTF8       bool                 `config:"disable_utf8"`
	WritingMDTM       bool                 `config:"writing_mdtm"`
	ForceListHidden   bool                 `config:"force_list_hidden"`
	IdleTimeout       fs.Duration          `config:"idle_timeout"`
	CloseTimeout      fs.Duration          `config:"close_timeout"`
	ShutTimeout       fs.Duration          `config:"shut_timeout"`
	AskPassword       bool                 `config:"ask_password"`
	Enc               encoder.MultiEncoder `config:"encoding"`
	SocksProxy        string               `config:"socks_proxy"`
	NoCheckUpload     bool                 `config:"no_check_upload"`
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
	pacer    *fs.Pacer // pacer for FTP connections
	fGetTime bool      // true if the ftp library accepts GetTime
	fSetTime bool      // true if the ftp library accepts SetTime
	fLstTime bool      // true if the List call returns precise time
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
	precise bool // true if the time is precise
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

// Return a *textproto.Error if err contains one or nil otherwise
func textprotoError(err error) (errX *textproto.Error) {
	if errors.As(err, &errX) {
		return errX
	}
	return nil
}

// returns true if this FTP error should be retried
func isRetriableFtpError(err error) bool {
	if errX := textprotoError(err); errX != nil {
		switch errX.Code {
		case ftp.StatusNotAvailable, ftp.StatusTransfertAborted:
			return true
		}
	}
	return false
}

// shouldRetry returns a boolean as to whether this err deserve to be
// retried.  It returns the err as a convenience
func shouldRetry(ctx context.Context, err error) (bool, error) {
	if fserrors.ContextError(ctx, &err) {
		return false, err
	}
	if isRetriableFtpError(err) {
		return true, err
	}
	return fserrors.ShouldRetry(err), err
}

// Get a TLS config with a unique session cache.
//
// We can't share session caches between connections.
//
// See: https://github.com/rclone/rclone/issues/7234
func (f *Fs) tlsConfig() *tls.Config {
	var tlsConfig *tls.Config
	if f.opt.TLS || f.opt.ExplicitTLS {
		tlsConfig = &tls.Config{
			ServerName:         f.opt.Host,
			InsecureSkipVerify: f.opt.SkipVerifyTLSCert,
		}
		if f.opt.TLSCacheSize > 0 {
			tlsConfig.ClientSessionCache = tls.NewLRUClientSessionCache(f.opt.TLSCacheSize)
		}
		if f.opt.DisableTLS13 {
			tlsConfig.MaxVersion = tls.VersionTLS12
		}
	}
	return tlsConfig
}

// Open a new connection to the FTP server.
func (f *Fs) ftpConnection(ctx context.Context) (c *ftp.ServerConn, err error) {
	fs.Debugf(f, "Connecting to FTP server")

	// tls.Config for this connection only. Will be used for data
	// and control connections.
	tlsConfig := f.tlsConfig()

	// Make ftp library dial with fshttp dialer optionally using TLS
	initialConnection := true
	dial := func(network, address string) (conn net.Conn, err error) {
		fs.Debugf(f, "dial(%q,%q)", network, address)
		defer func() {
			fs.Debugf(f, "> dial: conn=%T, err=%v", conn, err)
		}()
		baseDialer := fshttp.NewDialer(ctx)
		if f.opt.SocksProxy != "" {
			conn, err = proxy.SOCKS5Dial(network, address, f.opt.SocksProxy, baseDialer)
		} else {
			conn, err = baseDialer.Dial(network, address)
		}
		if err != nil {
			return nil, err
		}
		// Connect using cleartext only for non TLS
		if tlsConfig == nil {
			return conn, nil
		}
		// Initial connection only needs to be cleartext for explicit TLS
		if f.opt.ExplicitTLS && initialConnection {
			initialConnection = false
			return conn, nil
		}
		// Upgrade connection to TLS
		tlsConn := tls.Client(conn, tlsConfig)
		// Do the initial handshake - tls.Client doesn't do it for us
		// If we do this then connections to proftpd/pureftpd lock up
		// See: https://github.com/rclone/rclone/issues/6426
		// See: https://github.com/jlaffaye/ftp/issues/282
		if false {
			err = tlsConn.HandshakeContext(ctx)
			if err != nil {
				_ = conn.Close()
				return nil, err
			}
		}
		return tlsConn, nil
	}
	ftpConfig := []ftp.DialOption{
		ftp.DialWithContext(ctx),
		ftp.DialWithDialFunc(dial),
	}

	if f.opt.TLS {
		// Our dialer takes care of TLS but ftp library also needs tlsConf
		// as a trigger for sending PSBZ and PROT options to server.
		ftpConfig = append(ftpConfig, ftp.DialWithTLS(tlsConfig))
	} else if f.opt.ExplicitTLS {
		ftpConfig = append(ftpConfig, ftp.DialWithExplicitTLS(tlsConfig))
	}
	if f.opt.DisableEPSV {
		ftpConfig = append(ftpConfig, ftp.DialWithDisabledEPSV(true))
	}
	if f.opt.DisableMLSD {
		ftpConfig = append(ftpConfig, ftp.DialWithDisabledMLSD(true))
	}
	if f.opt.DisableUTF8 {
		ftpConfig = append(ftpConfig, ftp.DialWithDisabledUTF8(true))
	}
	if f.opt.ShutTimeout != 0 && f.opt.ShutTimeout != fs.DurationOff {
		ftpConfig = append(ftpConfig, ftp.DialWithShutTimeout(time.Duration(f.opt.ShutTimeout)))
	}
	if f.opt.WritingMDTM {
		ftpConfig = append(ftpConfig, ftp.DialWithWritingMDTM(true))
	}
	if f.opt.ForceListHidden {
		ftpConfig = append(ftpConfig, ftp.DialWithForceListHidden(true))
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
		err = fmt.Errorf("failed to make FTP connection to %q: %w", f.dialAddr, err)
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
		if tpErr := textprotoError(err); tpErr != nil {
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
	pass := ""
	if opt.AskPassword && opt.Pass == "" {
		pass = config.GetPassword("FTP server password")
	} else {
		pass, err = obscure.Reveal(opt.Pass)
		if err != nil {
			return nil, fmt.Errorf("NewFS decrypt password: %w", err)
		}
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
		return nil, errors.New("implicit TLS and explicit TLS are mutually incompatible, please revise your config")
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
		pacer:    fs.NewPacer(ctx, pacer.NewDefault(pacer.MinSleep(minSleep), pacer.MaxSleep(maxSleep), pacer.DecayConstant(decayConstant))),
	}
	f.features = (&fs.Features{
		CanHaveEmptyDirectories: true,
		PartialUploads:          true,
	}).Fill(ctx, f)
	// set the pool drainer timer going
	if f.opt.IdleTimeout > 0 {
		f.drain = time.AfterFunc(time.Duration(opt.IdleTimeout), func() { _ = f.drainPool(ctx) })
	}
	// Make a connection and pool it to return errors early
	c, err := f.getFtpConnection(ctx)
	if err != nil {
		return nil, fmt.Errorf("NewFs: %w", err)
	}
	f.fGetTime = c.IsGetTimeSupported()
	f.fSetTime = c.IsSetTimeSupported()
	f.fLstTime = c.IsTimePreciseInList()
	if !f.fLstTime && f.fGetTime {
		f.features.SlowModTime = true
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
			if err == fs.ErrorObjectNotFound || errors.Is(err, fs.ErrorNotAFile) {
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
	if errX := textprotoError(err); errX != nil {
		switch errX.Code {
		case ftp.StatusFileUnavailable, ftp.StatusFileActionIgnored:
			err = fs.ErrorObjectNotFound
		}
	}
	return err
}

// translateErrorDir turns FTP errors into rclone errors if possible for a directory
func translateErrorDir(err error) error {
	if errX := textprotoError(err); errX != nil {
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
	if remote == "" || remote == "." || remote == "/" {
		// if root, assume exists and synthesize an entry
		return &ftp.Entry{
			Name: "",
			Type: ftp.EntryTypeFolder,
			Time: time.Now(),
		}, nil
	}

	c, err := f.getFtpConnection(ctx)
	if err != nil {
		return nil, fmt.Errorf("findItem: %w", err)
	}

	// returns TRUE if MLST is supported which is required to call GetEntry
	if c.IsTimePreciseInList() {
		entry, err := c.GetEntry(f.opt.Enc.FromStandardPath(remote))
		f.putFtpConnection(&c, err)
		if err != nil {
			err = translateErrorFile(err)
			if err == fs.ErrorObjectNotFound {
				return nil, nil
			}
			if errX := textprotoError(err); errX != nil {
				switch errX.Code {
				case ftp.StatusBadArguments:
					err = nil
				}
			}
			return nil, err
		}
		if entry != nil {
			f.entryToStandard(entry)
		}
		return entry, nil
	}

	dir := path.Dir(remote)
	base := path.Base(remote)

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
	entry, err := f.findItem(ctx, path.Join(f.root, remote))
	if err != nil {
		return nil, err
	}
	if entry != nil && entry.Type != ftp.EntryTypeFolder {
		o := &Object{
			fs:     f,
			remote: remote,
		}
		o.info = &FileInfo{
			Name:    remote,
			Size:    entry.Size,
			ModTime: entry.Time,
			precise: f.fLstTime,
		}
		return o, nil
	}
	return nil, fs.ErrorObjectNotFound
}

// dirExists checks the directory pointed to by remote exists or not
func (f *Fs) dirExists(ctx context.Context, remote string) (exists bool, err error) {
	entry, err := f.findItem(ctx, path.Join(f.root, remote))
	if err != nil {
		return false, fmt.Errorf("dirExists: %w", err)
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
		return nil, fmt.Errorf("list: %w", err)
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
		return nil, errors.New("timeout when waiting for List")
	}

	// Annoyingly FTP returns success for a directory which
	// doesn't exist, so check it really doesn't exist if no
	// entries found.
	if len(files) == 0 {
		exists, err := f.dirExists(ctx, dir)
		if err != nil {
			return nil, fmt.Errorf("list: %w", err)
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
				precise: f.fLstTime,
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

// Precision shows whether modified time is supported or not depending on the
// FTP server capabilities, namely whether FTP server:
//   - accepts the MDTM command to get file time (fGetTime)
//     or supports MLSD returning precise file time in the list (fLstTime)
//   - accepts the MFMT command to set file time (fSetTime)
//     or non-standard form of the MDTM command (fSetTime, too)
//     used by VsFtpd for the same purpose (WritingMDTM)
//
// See "mdtm_write" in https://security.appspot.com/vsftpd/vsftpd_conf.html
func (f *Fs) Precision() time.Duration {
	if (f.fGetTime || f.fLstTime) && f.fSetTime {
		return time.Second
	}
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
		return nil, fmt.Errorf("Put mkParentDir failed: %w", err)
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
	file, err := f.findItem(ctx, remote)
	if err != nil {
		return nil, err
	} else if file != nil {
		info := &FileInfo{
			Name:    remote,
			Size:    file.Size,
			ModTime: file.Time,
			precise: f.fLstTime,
			IsDir:   file.Type == ftp.EntryTypeFolder,
		}
		return info, nil
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
		return fmt.Errorf("mkdir %q failed: %w", abspath, err)
	}
	parent := path.Dir(abspath)
	err = f.mkdir(ctx, parent)
	if err != nil {
		return err
	}
	c, connErr := f.getFtpConnection(ctx)
	if connErr != nil {
		return fmt.Errorf("mkdir: %w", connErr)
	}
	err = c.MakeDir(f.dirFromStandardPath(abspath))
	f.putFtpConnection(&c, err)
	if errX := textprotoError(err); errX != nil {
		switch errX.Code {
		case ftp.StatusRequestedFileActionOK: // some ftp servers apparently return 250 instead of 257
			err = nil // see: https://forum.rclone.org/t/rclone-pop-up-an-i-o-error-when-creating-a-folder-in-a-mounted-ftp-drive/44368/
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
		return fmt.Errorf("Rmdir: %w", translateErrorFile(err))
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
		return nil, fmt.Errorf("Move mkParentDir failed: %w", err)
	}
	c, err := f.getFtpConnection(ctx)
	if err != nil {
		return nil, fmt.Errorf("Move: %w", err)
	}
	err = c.Rename(
		f.opt.Enc.FromStandardPath(path.Join(srcObj.fs.root, srcObj.remote)),
		f.opt.Enc.FromStandardPath(path.Join(f.root, remote)),
	)
	f.putFtpConnection(&c, err)
	if err != nil {
		return nil, fmt.Errorf("Move Rename failed: %w", err)
	}
	dstObj, err := f.NewObject(ctx, remote)
	if err != nil {
		return nil, fmt.Errorf("Move NewObject failed: %w", err)
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
		return fmt.Errorf("DirMove getInfo failed: %w", err)
	}

	// Make sure the parent directory exists
	err = f.mkdir(ctx, path.Dir(dstPath))
	if err != nil {
		return fmt.Errorf("DirMove mkParentDir dst failed: %w", err)
	}

	// Do the move
	c, err := f.getFtpConnection(ctx)
	if err != nil {
		return fmt.Errorf("DirMove: %w", err)
	}
	err = c.Rename(
		f.dirFromStandardPath(srcPath),
		f.dirFromStandardPath(dstPath),
	)
	f.putFtpConnection(&c, err)
	if err != nil {
		return fmt.Errorf("DirMove Rename(%q,%q) failed: %w", srcPath, dstPath, err)
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
	if !o.info.precise && o.fs.fGetTime {
		c, err := o.fs.getFtpConnection(ctx)
		if err == nil {
			path := path.Join(o.fs.root, o.remote)
			path = o.fs.opt.Enc.FromStandardPath(path)
			modTime, err := c.GetTime(path)
			if err == nil && o.info != nil {
				o.info.ModTime = modTime
				o.info.precise = true
			}
			o.fs.putFtpConnection(&c, err)
		}
	}
	return o.info.ModTime
}

// SetModTime sets the modification time of the object
func (o *Object) SetModTime(ctx context.Context, modTime time.Time) error {
	if !o.fs.fSetTime {
		fs.Debugf(o.fs, "SetModTime is not supported")
		return nil
	}
	c, err := o.fs.getFtpConnection(ctx)
	if err != nil {
		return err
	}
	path := path.Join(o.fs.root, o.remote)
	path = o.fs.opt.Enc.FromStandardPath(path)
	err = c.SetTime(path, modTime.In(time.UTC))
	if err == nil && o.info != nil {
		o.info.ModTime = modTime
		o.info.precise = true
	}
	o.fs.putFtpConnection(&c, err)
	return err
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
	closeTimeout := f.f.opt.CloseTimeout
	if closeTimeout == 0 {
		closeTimeout = fs.DurationOff
	}
	timer := time.NewTimer(time.Duration(closeTimeout))
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
	if errX := textprotoError(err); errX != nil {
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

	var (
		fd *ftp.Response
		c  *ftp.ServerConn
	)
	err = o.fs.pacer.Call(func() (bool, error) {
		c, err = o.fs.getFtpConnection(ctx)
		if err != nil {
			return false, err // getFtpConnection has retries already
		}
		fd, err = c.RetrFrom(o.fs.opt.Enc.FromStandardPath(path), uint64(offset))
		if err != nil {
			o.fs.putFtpConnection(&c, err)
		}
		return shouldRetry(ctx, err)
	})
	if err != nil {
		return nil, fmt.Errorf("open: %w", err)
	}

	rc = &ftpReadCloser{rc: readers.NewLimitedReadCloser(fd, limit), c: c, f: o.fs}
	return rc, nil
}

// Update the already existing object
//
// Copy the reader into the object updating modTime and size.
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
		return fmt.Errorf("Update: %w", err)
	}
	err = c.Stor(o.fs.opt.Enc.FromStandardPath(path), in)
	// Ignore error 250 here - send by some servers
	if errX := textprotoError(err); errX != nil {
		switch errX.Code {
		case ftp.StatusRequestedFileActionOK:
			err = nil
		}
	}
	if err != nil {
		_ = c.Quit() // toss this connection to avoid sync errors
		// recycle connection in advance to let remove() find free token
		o.fs.putFtpConnection(nil, err)
		remove()
		return fmt.Errorf("update stor: %w", err)
	}
	o.fs.putFtpConnection(&c, nil)
	if o.fs.opt.NoCheckUpload {
		o.info = &FileInfo{
			Name:    o.remote,
			Size:    uint64(src.Size()),
			ModTime: src.ModTime(ctx),
			precise: true,
			IsDir:   false,
		}
		return nil
	}
	if err = o.SetModTime(ctx, src.ModTime(ctx)); err != nil {
		return fmt.Errorf("SetModTime: %w", err)
	}
	o.info, err = o.fs.getInfo(ctx, path)
	if err != nil {
		return fmt.Errorf("update getinfo: %w", err)
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
			return fmt.Errorf("Remove: %w", err)
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
