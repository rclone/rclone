//go:build !plan9

// Package ftp implements an FTP server for rclone
package ftp

import (
	"context"
	"errors"
	"fmt"
	"io"
	iofs "io/fs"
	"net"
	"os"
	"os/user"
	"regexp"
	"strconv"
	"sync"
	"time"

	"github.com/rclone/rclone/cmd"
	"github.com/rclone/rclone/cmd/serve/proxy"
	"github.com/rclone/rclone/cmd/serve/proxy/proxyflags"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/accounting"
	"github.com/rclone/rclone/fs/config/flags"
	"github.com/rclone/rclone/fs/config/obscure"
	"github.com/rclone/rclone/fs/log"
	"github.com/rclone/rclone/vfs"
	"github.com/rclone/rclone/vfs/vfscommon"
	"github.com/rclone/rclone/vfs/vfsflags"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	ftp "goftp.io/server/v2"
)

// OptionsInfo descripts the Options in use
var OptionsInfo = fs.Options{{
	Name:    "addr",
	Default: "localhost:2121",
	Help:    "IPaddress:Port or :Port to bind server to",
}, {
	Name:    "public_ip",
	Default: "",
	Help:    "Public IP address to advertise for passive connections",
}, {
	Name:    "passive_port",
	Default: "30000-32000",
	Help:    "Passive port range to use",
}, {
	Name:    "user",
	Default: "anonymous",
	Help:    "User name for authentication",
}, {
	Name:    "pass",
	Default: "",
	Help:    "Password for authentication (empty value allow every password)",
}, {
	Name:    "cert",
	Default: "",
	Help:    "TLS PEM key (concatenation of certificate and CA certificate)",
}, {
	Name:    "key",
	Default: "",
	Help:    "TLS PEM Private key",
}}

// Options contains options for the http Server
type Options struct {
	//TODO add more options
	ListenAddr   string `config:"addr"`         // Port to listen on
	PublicIP     string `config:"public_ip"`    // Passive ports range
	PassivePorts string `config:"passive_port"` // Passive ports range
	BasicUser    string `config:"user"`         // single username for basic auth if not using Htpasswd
	BasicPass    string `config:"pass"`         // password for BasicUser
	TLSCert      string `config:"cert"`         // TLS PEM key (concatenation of certificate and CA certificate)
	TLSKey       string `config:"key"`          // TLS PEM Private key
}

// Opt is options set by command line flags
var Opt Options

// AddFlags adds flags for ftp
func AddFlags(flagSet *pflag.FlagSet) {
	flags.AddFlagsFromOptions(flagSet, "", OptionsInfo)
}

func init() {
	vfsflags.AddFlags(Command.Flags())
	proxyflags.AddFlags(Command.Flags())
	AddFlags(Command.Flags())
}

// Command definition for cobra
var Command = &cobra.Command{
	Use:   "ftp remote:path",
	Short: `Serve remote:path over FTP.`,
	Long: `Run a basic FTP server to serve a remote over FTP protocol.
This can be viewed with a FTP client or you can make a remote of
type FTP to read and write it.

### Server options

Use --addr to specify which IP address and port the server should
listen on, e.g. --addr 1.2.3.4:8000 or --addr :8080 to listen to all
IPs.  By default it only listens on localhost.  You can use port
:0 to let the OS choose an available port.

If you set --addr to listen on a public or LAN accessible IP address
then using Authentication is advised - see the next section for info.

#### Authentication

By default this will serve files without needing a login.

You can set a single username and password with the --user and --pass flags.

` + vfs.Help() + proxy.Help,
	Annotations: map[string]string{
		"versionIntroduced": "v1.44",
		"groups":            "Filter",
	},
	Run: func(command *cobra.Command, args []string) {
		var f fs.Fs
		if proxyflags.Opt.AuthProxy == "" {
			cmd.CheckArgs(1, 1, command, args)
			f = cmd.NewFsSrc(args)
		} else {
			cmd.CheckArgs(0, 0, command, args)
		}
		cmd.Run(false, false, command, func() error {
			s, err := newServer(context.Background(), f, &Opt)
			if err != nil {
				return err
			}
			return s.serve()
		})
	},
}

// driver contains everything to run the driver for the FTP server
type driver struct {
	f          fs.Fs
	srv        *ftp.Server
	ctx        context.Context // for global config
	opt        Options
	globalVFS  *vfs.VFS     // the VFS if not using auth proxy
	proxy      *proxy.Proxy // may be nil if not in use
	useTLS     bool
	userPassMu sync.Mutex        // to protect userPass
	userPass   map[string]string // cache of username => password when using vfs proxy
}

func init() {
	fs.RegisterGlobalOptions(fs.OptionsInfo{Name: "ftp", Opt: &Opt, Options: OptionsInfo})
}

var passivePortsRe = regexp.MustCompile(`^\s*\d+\s*-\s*\d+\s*$`)

// Make a new FTP to serve the remote
func newServer(ctx context.Context, f fs.Fs, opt *Options) (*driver, error) {
	host, port, err := net.SplitHostPort(opt.ListenAddr)
	if err != nil {
		return nil, fmt.Errorf("failed to parse host:port from %q", opt.ListenAddr)
	}
	portNum, err := strconv.Atoi(port)
	if err != nil {
		return nil, fmt.Errorf("failed to parse port number from %q", port)
	}

	d := &driver{
		f:   f,
		ctx: ctx,
		opt: *opt,
	}
	if proxyflags.Opt.AuthProxy != "" {
		d.proxy = proxy.New(ctx, &proxyflags.Opt)
		d.userPass = make(map[string]string, 16)
	} else {
		d.globalVFS = vfs.New(f, &vfscommon.Opt)
	}
	d.useTLS = d.opt.TLSKey != ""

	// Check PassivePorts format since the server library doesn't!
	if !passivePortsRe.MatchString(opt.PassivePorts) {
		return nil, fmt.Errorf("invalid format for passive ports %q", opt.PassivePorts)
	}

	ftpopt := &ftp.Options{
		Name:           "Rclone FTP Server",
		WelcomeMessage: "Welcome to Rclone " + fs.Version + " FTP Server",
		Driver:         d,
		Hostname:       host,
		Port:           portNum,
		PublicIP:       opt.PublicIP,
		PassivePorts:   opt.PassivePorts,
		Auth:           d,
		Perm:           ftp.NewSimplePerm("ftp", "ftp"), // fake user and group
		Logger:         &Logger{},
		TLS:            d.useTLS,
		CertFile:       d.opt.TLSCert,
		KeyFile:        d.opt.TLSKey,
		//TODO implement a maximum of https://godoc.org/goftp.io/server#ServerOpts
	}
	d.srv, err = ftp.NewServer(ftpopt)
	if err != nil {
		return nil, fmt.Errorf("failed to create new FTP server: %w", err)
	}
	return d, nil
}

// serve runs the ftp server
func (d *driver) serve() error {
	fs.Logf(d.f, "Serving FTP on %s", d.srv.Hostname+":"+strconv.Itoa(d.srv.Port))
	return d.srv.ListenAndServe()
}

// close stops the ftp server
//
//lint:ignore U1000 unused when not building linux
func (d *driver) close() error {
	fs.Logf(d.f, "Stopping FTP on %s", d.srv.Hostname+":"+strconv.Itoa(d.srv.Port))
	return d.srv.Shutdown()
}

// Logger ftp logger output formatted message
type Logger struct{}

// Print log simple text message
func (l *Logger) Print(sessionID string, message interface{}) {
	fs.Infof(sessionID, "%s", message)
}

// Printf log formatted text message
func (l *Logger) Printf(sessionID string, format string, v ...interface{}) {
	fs.Infof(sessionID, format, v...)
}

// PrintCommand log formatted command execution
func (l *Logger) PrintCommand(sessionID string, command string, params string) {
	if command == "PASS" {
		fs.Infof(sessionID, "> PASS ****")
	} else {
		fs.Infof(sessionID, "> %s %s", command, params)
	}
}

// PrintResponse log responses
func (l *Logger) PrintResponse(sessionID string, code int, message string) {
	fs.Infof(sessionID, "< %d %s", code, message)
}

// CheckPasswd handle auth based on configuration
func (d *driver) CheckPasswd(sctx *ftp.Context, user, pass string) (ok bool, err error) {
	if d.proxy != nil {
		_, _, err = d.proxy.Call(user, pass, false)
		if err != nil {
			fs.Infof(nil, "proxy login failed: %v", err)
			return false, nil
		}
		// Cache obscured password for later lookup.
		//
		// We don't cache the VFS directly in the driver as we want them
		// to be expired and the auth proxy does that for us.
		oPass, err := obscure.Obscure(pass)
		if err != nil {
			return false, err
		}
		d.userPassMu.Lock()
		d.userPass[user] = oPass
		d.userPassMu.Unlock()
	} else {
		ok = d.opt.BasicUser == user && (d.opt.BasicPass == "" || d.opt.BasicPass == pass)
		if !ok {
			fs.Infof(nil, "login failed: bad credentials")
			return false, nil
		}
	}
	return true, nil
}

// Get the VFS for this connection
func (d *driver) getVFS(sctx *ftp.Context) (VFS *vfs.VFS, err error) {
	if d.proxy == nil {
		// If no proxy always use the same VFS
		return d.globalVFS, nil
	}
	user := sctx.Sess.LoginUser()
	d.userPassMu.Lock()
	oPass, ok := d.userPass[user]
	d.userPassMu.Unlock()
	if !ok {
		return nil, fmt.Errorf("proxy user not logged in")
	}
	pass, err := obscure.Reveal(oPass)
	if err != nil {
		return nil, err
	}
	VFS, _, err = d.proxy.Call(user, pass, false)
	if err != nil {
		return nil, fmt.Errorf("proxy login failed: %w", err)
	}
	return VFS, nil
}

// Stat get information on file or folder
func (d *driver) Stat(sctx *ftp.Context, path string) (fi iofs.FileInfo, err error) {
	defer log.Trace(path, "")("fi=%+v, err = %v", &fi, &err)
	VFS, err := d.getVFS(sctx)
	if err != nil {
		return nil, err
	}
	n, err := VFS.Stat(path)
	if err != nil {
		return nil, err
	}
	return &FileInfo{n, n.Mode(), VFS.Opt.UID, VFS.Opt.GID}, err
}

// ChangeDir move current folder
func (d *driver) ChangeDir(sctx *ftp.Context, path string) (err error) {
	defer log.Trace(path, "")("err = %v", &err)
	VFS, err := d.getVFS(sctx)
	if err != nil {
		return err
	}
	n, err := VFS.Stat(path)
	if err != nil {
		return err
	}
	if !n.IsDir() {
		return errors.New("not a directory")
	}
	return nil
}

// ListDir list content of a folder
func (d *driver) ListDir(sctx *ftp.Context, path string, callback func(iofs.FileInfo) error) (err error) {
	defer log.Trace(path, "")("err = %v", &err)
	VFS, err := d.getVFS(sctx)
	if err != nil {
		return err
	}
	node, err := VFS.Stat(path)
	if err == vfs.ENOENT {
		return errors.New("directory not found")
	} else if err != nil {
		return err
	}
	if !node.IsDir() {
		return errors.New("not a directory")
	}

	dir := node.(*vfs.Dir)
	dirEntries, err := dir.ReadDirAll()
	if err != nil {
		return err
	}

	// Account the transfer
	tr := accounting.GlobalStats().NewTransferRemoteSize(path, node.Size(), d.f, nil)
	defer func() {
		tr.Done(d.ctx, err)
	}()

	for _, file := range dirEntries {
		err = callback(&FileInfo{file, file.Mode(), VFS.Opt.UID, VFS.Opt.GID})
		if err != nil {
			return err
		}
	}
	return nil
}

// DeleteDir delete a folder and his content
func (d *driver) DeleteDir(sctx *ftp.Context, path string) (err error) {
	defer log.Trace(path, "")("err = %v", &err)
	VFS, err := d.getVFS(sctx)
	if err != nil {
		return err
	}
	node, err := VFS.Stat(path)
	if err != nil {
		return err
	}
	if !node.IsDir() {
		return errors.New("not a directory")
	}
	err = node.Remove()
	if err != nil {
		return err
	}
	return nil
}

// DeleteFile delete a file
func (d *driver) DeleteFile(sctx *ftp.Context, path string) (err error) {
	defer log.Trace(path, "")("err = %v", &err)
	VFS, err := d.getVFS(sctx)
	if err != nil {
		return err
	}
	node, err := VFS.Stat(path)
	if err != nil {
		return err
	}
	if !node.IsFile() {
		return errors.New("not a file")
	}
	err = node.Remove()
	if err != nil {
		return err
	}
	return nil
}

// Rename rename a file or folder
func (d *driver) Rename(sctx *ftp.Context, oldName, newName string) (err error) {
	defer log.Trace(oldName, "newName=%q", newName)("err = %v", &err)
	VFS, err := d.getVFS(sctx)
	if err != nil {
		return err
	}
	return VFS.Rename(oldName, newName)
}

// MakeDir create a folder
func (d *driver) MakeDir(sctx *ftp.Context, path string) (err error) {
	defer log.Trace(path, "")("err = %v", &err)
	VFS, err := d.getVFS(sctx)
	if err != nil {
		return err
	}
	dir, leaf, err := VFS.StatParent(path)
	if err != nil {
		return err
	}
	_, err = dir.Mkdir(leaf)
	return err
}

// GetFile download a file
func (d *driver) GetFile(sctx *ftp.Context, path string, offset int64) (size int64, fr io.ReadCloser, err error) {
	defer log.Trace(path, "offset=%v", offset)("err = %v", &err)
	VFS, err := d.getVFS(sctx)
	if err != nil {
		return 0, nil, err
	}
	node, err := VFS.Stat(path)
	if err == vfs.ENOENT {
		fs.Infof(path, "File not found")
		return 0, nil, errors.New("file not found")
	} else if err != nil {
		return 0, nil, err
	}
	if !node.IsFile() {
		return 0, nil, errors.New("not a file")
	}

	handle, err := node.Open(os.O_RDONLY)
	if err != nil {
		return 0, nil, err
	}
	_, err = handle.Seek(offset, io.SeekStart)
	if err != nil {
		return 0, nil, err
	}

	// Account the transfer
	tr := accounting.GlobalStats().NewTransferRemoteSize(path, node.Size(), d.f, nil)
	defer tr.Done(d.ctx, nil)

	return node.Size(), handle, nil
}

// PutFile upload a file
func (d *driver) PutFile(sctx *ftp.Context, path string, data io.Reader, offset int64) (n int64, err error) {
	defer log.Trace(path, "offset=%d", offset)("err = %v", &err)

	var isExist bool
	VFS, err := d.getVFS(sctx)
	if err != nil {
		return 0, err
	}
	fi, err := VFS.Stat(path)
	if err == nil {
		isExist = true
		if fi.IsDir() {
			return 0, errors.New("can't create file - directory exists")
		}
	} else {
		if os.IsNotExist(err) {
			isExist = false
		} else {
			return 0, err
		}
	}

	if offset > -1 && !isExist {
		offset = -1
	}

	var f vfs.Handle

	if offset == -1 {
		if isExist {
			err = VFS.Remove(path)
			if err != nil {
				return 0, err
			}
		}
		f, err = VFS.Create(path)
		if err != nil {
			return 0, err
		}
		defer fs.CheckClose(f, &err)
		n, err = io.Copy(f, data)
		if err != nil {
			return 0, err
		}
		return n, nil
	}

	f, err = VFS.OpenFile(path, os.O_APPEND|os.O_RDWR, 0660)
	if err != nil {
		return 0, err
	}
	defer fs.CheckClose(f, &err)

	info, err := f.Stat()
	if err != nil {
		return 0, err
	}
	if offset > info.Size() {
		return 0, fmt.Errorf("offset %d is beyond file size %d", offset, info.Size())
	}

	_, err = f.Seek(offset, io.SeekStart)
	if err != nil {
		return 0, err
	}

	bytes, err := io.Copy(f, data)
	if err != nil {
		return 0, err
	}

	return bytes, nil
}

// FileInfo struct to hold file info for ftp server
type FileInfo struct {
	os.FileInfo

	mode  os.FileMode
	owner uint32
	group uint32
}

// Mode return mode of file.
func (f *FileInfo) Mode() os.FileMode {
	return f.mode
}

// Owner return owner of file. Try to find the username if possible
func (f *FileInfo) Owner() string {
	str := fmt.Sprint(f.owner)
	u, err := user.LookupId(str)
	if err != nil {
		return str //User not found
	}
	return u.Username
}

// Group return group of file. Try to find the group name if possible
func (f *FileInfo) Group() string {
	str := fmt.Sprint(f.group)
	g, err := user.LookupGroupId(str)
	if err != nil {
		return str //Group not found default to numerical value
	}
	return g.Name
}

// ModTime returns the time in UTC
func (f *FileInfo) ModTime() time.Time {
	return f.FileInfo.ModTime().UTC()
}
