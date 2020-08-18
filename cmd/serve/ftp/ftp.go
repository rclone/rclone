// Package ftp implements an FTP server for rclone

//+build !plan9,go1.13

package ftp

import (
	"fmt"
	"io"
	"net"
	"os"
	"os/user"
	"strconv"
	"sync"

	"github.com/pkg/errors"
	"github.com/rclone/rclone/cmd"
	"github.com/rclone/rclone/cmd/serve/proxy"
	"github.com/rclone/rclone/cmd/serve/proxy/proxyflags"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/accounting"
	"github.com/rclone/rclone/fs/config/flags"
	"github.com/rclone/rclone/fs/log"
	"github.com/rclone/rclone/fs/rc"
	"github.com/rclone/rclone/vfs"
	"github.com/rclone/rclone/vfs/vfsflags"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	ftp "goftp.io/server/core"
)

// Options contains options for the http Server
type Options struct {
	//TODO add more options
	ListenAddr   string // Port to listen on
	PublicIP     string // Passive ports range
	PassivePorts string // Passive ports range
	BasicUser    string // single username for basic auth if not using Htpasswd
	BasicPass    string // password for BasicUser
}

// DefaultOpt is the default values used for Options
var DefaultOpt = Options{
	ListenAddr:   "localhost:2121",
	PublicIP:     "",
	PassivePorts: "30000-32000",
	BasicUser:    "anonymous",
	BasicPass:    "",
}

// Opt is options set by command line flags
var Opt = DefaultOpt

// AddFlags adds flags for ftp
func AddFlags(flagSet *pflag.FlagSet) {
	rc.AddOption("ftp", &Opt)
	flags.StringVarP(flagSet, &Opt.ListenAddr, "addr", "", Opt.ListenAddr, "IPaddress:Port or :Port to bind server to.")
	flags.StringVarP(flagSet, &Opt.PublicIP, "public-ip", "", Opt.PublicIP, "Public IP address to advertise for passive connections.")
	flags.StringVarP(flagSet, &Opt.PassivePorts, "passive-port", "", Opt.PassivePorts, "Passive port range to use.")
	flags.StringVarP(flagSet, &Opt.BasicUser, "user", "", Opt.BasicUser, "User name for authentication.")
	flags.StringVarP(flagSet, &Opt.BasicPass, "pass", "", Opt.BasicPass, "Password for authentication. (empty value allow every password)")
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
	Long: `
rclone serve ftp implements a basic ftp server to serve the
remote over FTP protocol. This can be viewed with a ftp client
or you can make a remote of type ftp to read and write it.

### Server options

Use --addr to specify which IP address and port the server should
listen on, eg --addr 1.2.3.4:8000 or --addr :8080 to listen to all
IPs.  By default it only listens on localhost.  You can use port
:0 to let the OS choose an available port.

If you set --addr to listen on a public or LAN accessible IP address
then using Authentication is advised - see the next section for info.

#### Authentication

By default this will serve files without needing a login.

You can set a single username and password with the --user and --pass flags.
` + vfs.Help + proxy.Help,
	Run: func(command *cobra.Command, args []string) {
		var f fs.Fs
		if proxyflags.Opt.AuthProxy == "" {
			cmd.CheckArgs(1, 1, command, args)
			f = cmd.NewFsSrc(args)
		} else {
			cmd.CheckArgs(0, 0, command, args)
		}
		cmd.Run(false, false, command, func() error {
			s, err := newServer(f, &Opt)
			if err != nil {
				return err
			}
			return s.serve()
		})
	},
}

// server contains everything to run the server
type server struct {
	f     fs.Fs
	srv   *ftp.Server
	opt   Options
	vfs   *vfs.VFS
	proxy *proxy.Proxy
}

// Make a new FTP to serve the remote
func newServer(f fs.Fs, opt *Options) (*server, error) {
	host, port, err := net.SplitHostPort(opt.ListenAddr)
	if err != nil {
		return nil, errors.New("Failed to parse host:port")
	}
	portNum, err := strconv.Atoi(port)
	if err != nil {
		return nil, errors.New("Failed to parse host:port")
	}

	s := &server{
		f:   f,
		opt: *opt,
	}
	if proxyflags.Opt.AuthProxy != "" {
		s.proxy = proxy.New(&proxyflags.Opt)
	} else {
		s.vfs = vfs.New(f, &vfsflags.Opt)
	}

	ftpopt := &ftp.ServerOpts{
		Name:           "Rclone FTP Server",
		WelcomeMessage: "Welcome to Rclone " + fs.Version + " FTP Server",
		Factory:        s, // implemented by NewDriver method
		Hostname:       host,
		Port:           portNum,
		PublicIP:       opt.PublicIP,
		PassivePorts:   opt.PassivePorts,
		Auth:           s, // implemented by CheckPasswd method
		Logger:         &Logger{},
		//TODO implement a maximum of https://godoc.org/goftp.io/server#ServerOpts
	}
	s.srv = ftp.NewServer(ftpopt)
	return s, nil
}

// serve runs the ftp server
func (s *server) serve() error {
	fs.Logf(s.f, "Serving FTP on %s", s.srv.Hostname+":"+strconv.Itoa(s.srv.Port))
	return s.srv.ListenAndServe()
}

// serve runs the ftp server
func (s *server) close() error {
	fs.Logf(s.f, "Stopping FTP on %s", s.srv.Hostname+":"+strconv.Itoa(s.srv.Port))
	return s.srv.Shutdown()
}

//Logger ftp logger output formatted message
type Logger struct{}

//Print log simple text message
func (l *Logger) Print(sessionID string, message interface{}) {
	fs.Infof(sessionID, "%s", message)
}

//Printf log formatted text message
func (l *Logger) Printf(sessionID string, format string, v ...interface{}) {
	fs.Infof(sessionID, format, v...)
}

//PrintCommand log formatted command execution
func (l *Logger) PrintCommand(sessionID string, command string, params string) {
	if command == "PASS" {
		fs.Infof(sessionID, "> PASS ****")
	} else {
		fs.Infof(sessionID, "> %s %s", command, params)
	}
}

//PrintResponse log responses
func (l *Logger) PrintResponse(sessionID string, code int, message string) {
	fs.Infof(sessionID, "< %d %s", code, message)
}

// CheckPasswd handle auth based on configuration
//
// This is not used - the one in Driver should be called instead
func (s *server) CheckPasswd(user, pass string) (ok bool, err error) {
	err = errors.New("internal error: server.CheckPasswd should never be called")
	fs.Errorf(nil, "Error: %v", err)
	return false, err
}

// NewDriver starts a new session for each client connection
func (s *server) NewDriver() (ftp.Driver, error) {
	log.Trace("", "Init driver")("")
	d := &Driver{
		s:   s,
		vfs: s.vfs, // this can be nil if proxy set
	}
	return d, nil
}

//Driver implementation of ftp server
type Driver struct {
	s    *server
	vfs  *vfs.VFS
	lock sync.Mutex
}

// CheckPasswd handle auth based on configuration
func (d *Driver) CheckPasswd(user, pass string) (ok bool, err error) {
	s := d.s
	if s.proxy != nil {
		var VFS *vfs.VFS
		VFS, _, err = s.proxy.Call(user, pass, false)
		if err != nil {
			fs.Infof(nil, "proxy login failed: %v", err)
			return false, nil
		}
		d.vfs = VFS
	} else {
		ok = s.opt.BasicUser == user && (s.opt.BasicPass == "" || s.opt.BasicPass == pass)
		if !ok {
			fs.Infof(nil, "login failed: bad credentials")
			return false, nil
		}
	}
	return true, nil
}

//Stat get information on file or folder
func (d *Driver) Stat(path string) (fi ftp.FileInfo, err error) {
	defer log.Trace(path, "")("fi=%+v, err = %v", &fi, &err)
	n, err := d.vfs.Stat(path)
	if err != nil {
		return nil, err
	}
	return &FileInfo{n, n.Mode(), d.vfs.Opt.UID, d.vfs.Opt.GID}, err
}

//ChangeDir move current folder
func (d *Driver) ChangeDir(path string) (err error) {
	d.lock.Lock()
	defer d.lock.Unlock()
	defer log.Trace(path, "")("err = %v", &err)
	n, err := d.vfs.Stat(path)
	if err != nil {
		return err
	}
	if !n.IsDir() {
		return errors.New("Not a directory")
	}
	return nil
}

//ListDir list content of a folder
func (d *Driver) ListDir(path string, callback func(ftp.FileInfo) error) (err error) {
	d.lock.Lock()
	defer d.lock.Unlock()
	defer log.Trace(path, "")("err = %v", &err)
	node, err := d.vfs.Stat(path)
	if err == vfs.ENOENT {
		return errors.New("Directory not found")
	} else if err != nil {
		return err
	}
	if !node.IsDir() {
		return errors.New("Not a directory")
	}

	dir := node.(*vfs.Dir)
	dirEntries, err := dir.ReadDirAll()
	if err != nil {
		return err
	}

	// Account the transfer
	tr := accounting.GlobalStats().NewTransferRemoteSize(path, node.Size())
	defer func() {
		tr.Done(err)
	}()

	for _, file := range dirEntries {
		err = callback(&FileInfo{file, file.Mode(), d.vfs.Opt.UID, d.vfs.Opt.GID})
		if err != nil {
			return err
		}
	}
	return nil
}

//DeleteDir delete a folder and his content
func (d *Driver) DeleteDir(path string) (err error) {
	d.lock.Lock()
	defer d.lock.Unlock()
	defer log.Trace(path, "")("err = %v", &err)
	node, err := d.vfs.Stat(path)
	if err != nil {
		return err
	}
	if !node.IsDir() {
		return errors.New("Not a directory")
	}
	err = node.Remove()
	if err != nil {
		return err
	}
	return nil
}

//DeleteFile delete a file
func (d *Driver) DeleteFile(path string) (err error) {
	d.lock.Lock()
	defer d.lock.Unlock()
	defer log.Trace(path, "")("err = %v", &err)
	node, err := d.vfs.Stat(path)
	if err != nil {
		return err
	}
	if !node.IsFile() {
		return errors.New("Not a file")
	}
	err = node.Remove()
	if err != nil {
		return err
	}
	return nil
}

//Rename rename a file or folder
func (d *Driver) Rename(oldName, newName string) (err error) {
	d.lock.Lock()
	defer d.lock.Unlock()
	defer log.Trace(oldName, "newName=%q", newName)("err = %v", &err)
	return d.vfs.Rename(oldName, newName)
}

//MakeDir create a folder
func (d *Driver) MakeDir(path string) (err error) {
	d.lock.Lock()
	defer d.lock.Unlock()
	defer log.Trace(path, "")("err = %v", &err)
	dir, leaf, err := d.vfs.StatParent(path)
	if err != nil {
		return err
	}
	_, err = dir.Mkdir(leaf)
	return err
}

//GetFile download a file
func (d *Driver) GetFile(path string, offset int64) (size int64, fr io.ReadCloser, err error) {
	d.lock.Lock()
	defer d.lock.Unlock()
	defer log.Trace(path, "offset=%v", offset)("err = %v", &err)
	node, err := d.vfs.Stat(path)
	if err == vfs.ENOENT {
		fs.Infof(path, "File not found")
		return 0, nil, errors.New("File not found")
	} else if err != nil {
		return 0, nil, err
	}
	if !node.IsFile() {
		return 0, nil, errors.New("Not a file")
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
	tr := accounting.GlobalStats().NewTransferRemoteSize(path, node.Size())
	defer tr.Done(nil)

	return node.Size(), handle, nil
}

//PutFile upload a file
func (d *Driver) PutFile(path string, data io.Reader, appendData bool) (n int64, err error) {
	d.lock.Lock()
	defer d.lock.Unlock()
	defer log.Trace(path, "append=%v", appendData)("err = %v", &err)
	var isExist bool
	node, err := d.vfs.Stat(path)
	if err == nil {
		isExist = true
		if node.IsDir() {
			return 0, errors.New("A dir has the same name")
		}
	} else {
		if os.IsNotExist(err) {
			isExist = false
		} else {
			return 0, err
		}
	}

	if appendData && !isExist {
		appendData = false
	}

	if !appendData {
		if isExist {
			err = node.Remove()
			if err != nil {
				return 0, err
			}
		}
		f, err := d.vfs.OpenFile(path, os.O_RDWR|os.O_CREATE, 0660)
		if err != nil {
			return 0, err
		}
		defer closeIO(path, f)
		bytes, err := io.Copy(f, data)
		if err != nil {
			return 0, err
		}
		return bytes, nil
	}

	of, err := d.vfs.OpenFile(path, os.O_APPEND|os.O_RDWR, 0660)
	if err != nil {
		return 0, err
	}
	defer closeIO(path, of)

	_, err = of.Seek(0, os.SEEK_END)
	if err != nil {
		return 0, err
	}

	bytes, err := io.Copy(of, data)
	if err != nil {
		return 0, err
	}

	return bytes, nil
}

//FileInfo struct to hold file info for ftp server
type FileInfo struct {
	os.FileInfo

	mode  os.FileMode
	owner uint32
	group uint32
}

//Mode return mode of file.
func (f *FileInfo) Mode() os.FileMode {
	return f.mode
}

//Owner return owner of file. Try to find the username if possible
func (f *FileInfo) Owner() string {
	str := fmt.Sprint(f.owner)
	u, err := user.LookupId(str)
	if err != nil {
		return str //User not found
	}
	return u.Username
}

//Group return group of file. Try to find the group name if possible
func (f *FileInfo) Group() string {
	str := fmt.Sprint(f.group)
	g, err := user.LookupGroupId(str)
	if err != nil {
		return str //Group not found default to numerical value
	}
	return g.Name
}

func closeIO(path string, c io.Closer) {
	err := c.Close()
	if err != nil {
		log.Trace(path, "")("err = %v", &err)
	}
}
