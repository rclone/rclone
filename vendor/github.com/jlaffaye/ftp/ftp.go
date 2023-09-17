// Package ftp implements a FTP client as described in RFC 959.
//
// A textproto.Error is returned for errors at the protocol level.
package ftp

import (
	"bufio"
	"context"
	"crypto/tls"
	"errors"
	"io"
	"net"
	"net/textproto"
	"strconv"
	"strings"
	"time"

	"github.com/hashicorp/go-multierror"
)

const (
	// 30 seconds was chosen as it's the
	// same duration as http.DefaultTransport's timeout.
	DefaultDialTimeout = 30 * time.Second
)

// EntryType describes the different types of an Entry.
type EntryType int

// The differents types of an Entry
const (
	EntryTypeFile EntryType = iota
	EntryTypeFolder
	EntryTypeLink
)

// TransferType denotes the formats for transferring Entries.
type TransferType string

// The different transfer types
const (
	TransferTypeBinary = TransferType("I")
	TransferTypeASCII  = TransferType("A")
)

// Time format used by the MDTM and MFMT commands
const timeFormat = "20060102150405"

// ServerConn represents the connection to a remote FTP server.
// A single connection only supports one in-flight data connection.
// It is not safe to be called concurrently.
type ServerConn struct {
	options *dialOptions
	conn    *textproto.Conn // connection wrapper for text protocol
	netConn net.Conn        // underlying network connection
	host    string

	// Server capabilities discovered at runtime
	features      map[string]string
	skipEPSV      bool
	mlstSupported bool
	mfmtSupported bool
	mdtmSupported bool
	mdtmCanWrite  bool
	usePRET       bool
}

// DialOption represents an option to start a new connection with Dial
type DialOption struct {
	setup func(do *dialOptions)
}

// dialOptions contains all the options set by DialOption.setup
type dialOptions struct {
	context         context.Context
	dialer          net.Dialer
	tlsConfig       *tls.Config
	explicitTLS     bool
	disableEPSV     bool
	disableUTF8     bool
	disableMLSD     bool
	writingMDTM     bool
	forceListHidden bool
	location        *time.Location
	debugOutput     io.Writer
	dialFunc        func(network, address string) (net.Conn, error)
	shutTimeout     time.Duration // time to wait for data connection closing status
}

// Entry describes a file and is returned by List().
type Entry struct {
	Name   string
	Target string // target of symbolic link
	Type   EntryType
	Size   uint64
	Time   time.Time
}

// Response represents a data-connection
type Response struct {
	conn   net.Conn
	c      *ServerConn
	closed bool
}

// Dial connects to the specified address with optional options
func Dial(addr string, options ...DialOption) (*ServerConn, error) {
	do := &dialOptions{}
	for _, option := range options {
		option.setup(do)
	}

	if do.location == nil {
		do.location = time.UTC
	}

	dialFunc := do.dialFunc

	if dialFunc == nil {
		ctx := do.context

		if ctx == nil {
			ctx = context.Background()
		}
		if _, ok := ctx.Deadline(); !ok {
			var cancel context.CancelFunc
			ctx, cancel = context.WithTimeout(ctx, DefaultDialTimeout)
			defer cancel()
		}

		if do.tlsConfig != nil && !do.explicitTLS {
			dialFunc = func(network, address string) (net.Conn, error) {
				tlsDialer := &tls.Dialer{
					NetDialer: &do.dialer,
					Config:    do.tlsConfig,
				}
				return tlsDialer.DialContext(ctx, network, addr)
			}
		} else {

			dialFunc = func(network, address string) (net.Conn, error) {
				return do.dialer.DialContext(ctx, network, addr)
			}
		}
	}

	tconn, err := dialFunc("tcp", addr)
	if err != nil {
		return nil, err
	}

	// Use the resolved IP address in case addr contains a domain name
	// If we use the domain name, we might not resolve to the same IP.
	remoteAddr := tconn.RemoteAddr().(*net.TCPAddr)

	c := &ServerConn{
		options:  do,
		features: make(map[string]string),
		conn:     textproto.NewConn(do.wrapConn(tconn)),
		netConn:  tconn,
		host:     remoteAddr.IP.String(),
	}

	_, _, err = c.conn.ReadResponse(StatusReady)
	if err != nil {
		_ = c.Quit()
		return nil, err
	}

	if do.explicitTLS {
		if err := c.authTLS(); err != nil {
			_ = c.Quit()
			return nil, err
		}
		tconn = tls.Client(tconn, do.tlsConfig)
		c.conn = textproto.NewConn(do.wrapConn(tconn))
	}

	return c, nil
}

// DialWithTimeout returns a DialOption that configures the ServerConn with specified timeout
func DialWithTimeout(timeout time.Duration) DialOption {
	return DialOption{func(do *dialOptions) {
		do.dialer.Timeout = timeout
	}}
}

// DialWithShutTimeout returns a DialOption that configures the ServerConn with
// maximum time to wait for the data closing status on control connection
// and nudging the control connection deadline before reading status.
func DialWithShutTimeout(shutTimeout time.Duration) DialOption {
	return DialOption{func(do *dialOptions) {
		do.shutTimeout = shutTimeout
	}}
}

// DialWithDialer returns a DialOption that configures the ServerConn with specified net.Dialer
func DialWithDialer(dialer net.Dialer) DialOption {
	return DialOption{func(do *dialOptions) {
		do.dialer = dialer
	}}
}

// DialWithNetConn returns a DialOption that configures the ServerConn with the underlying net.Conn
//
// Deprecated: Use [DialWithDialFunc] instead
func DialWithNetConn(conn net.Conn) DialOption {
	return DialWithDialFunc(func(network, address string) (net.Conn, error) {
		return conn, nil
	})
}

// DialWithDisabledEPSV returns a DialOption that configures the ServerConn with EPSV disabled
// Note that EPSV is only used when advertised in the server features.
func DialWithDisabledEPSV(disabled bool) DialOption {
	return DialOption{func(do *dialOptions) {
		do.disableEPSV = disabled
	}}
}

// DialWithDisabledUTF8 returns a DialOption that configures the ServerConn with UTF8 option disabled
func DialWithDisabledUTF8(disabled bool) DialOption {
	return DialOption{func(do *dialOptions) {
		do.disableUTF8 = disabled
	}}
}

// DialWithDisabledMLSD returns a DialOption that configures the ServerConn with MLSD option disabled
//
// This is useful for servers which advertise MLSD (eg some versions
// of Serv-U) but don't support it properly.
func DialWithDisabledMLSD(disabled bool) DialOption {
	return DialOption{func(do *dialOptions) {
		do.disableMLSD = disabled
	}}
}

// DialWithWritingMDTM returns a DialOption making ServerConn use MDTM to set file time
//
// This option addresses a quirk in the VsFtpd server which doesn't support
// the MFMT command for setting file time like other servers but by default
// uses the MDTM command with non-standard arguments for that.
// See "mdtm_write" in https://security.appspot.com/vsftpd/vsftpd_conf.html
func DialWithWritingMDTM(enabled bool) DialOption {
	return DialOption{func(do *dialOptions) {
		do.writingMDTM = enabled
	}}
}

// DialWithForceListHidden returns a DialOption making ServerConn use LIST -a to include hidden files and folders in directory listings
//
// This is useful for servers that do not do this by default, but it forces the use of the LIST command
// even if the server supports MLST.
func DialWithForceListHidden(enabled bool) DialOption {
	return DialOption{func(do *dialOptions) {
		do.forceListHidden = enabled
	}}
}

// DialWithLocation returns a DialOption that configures the ServerConn with specified time.Location
// The location is used to parse the dates sent by the server which are in server's timezone
func DialWithLocation(location *time.Location) DialOption {
	return DialOption{func(do *dialOptions) {
		do.location = location
	}}
}

// DialWithContext returns a DialOption that configures the ServerConn with specified context
// The context will be used for the initial connection setup
func DialWithContext(ctx context.Context) DialOption {
	return DialOption{func(do *dialOptions) {
		do.context = ctx
	}}
}

// DialWithTLS returns a DialOption that configures the ServerConn with specified TLS config
//
// If called together with the DialWithDialFunc option, the DialWithDialFunc function
// will be used when dialing new connections but regardless of the function,
// the connection will be treated as a TLS connection.
func DialWithTLS(tlsConfig *tls.Config) DialOption {
	return DialOption{func(do *dialOptions) {
		do.tlsConfig = tlsConfig
	}}
}

// DialWithExplicitTLS returns a DialOption that configures the ServerConn to be upgraded to TLS
// See DialWithTLS for general TLS documentation
func DialWithExplicitTLS(tlsConfig *tls.Config) DialOption {
	return DialOption{func(do *dialOptions) {
		do.explicitTLS = true
		do.tlsConfig = tlsConfig
	}}
}

// DialWithDebugOutput returns a DialOption that configures the ServerConn to write to the Writer
// everything it reads from the server
func DialWithDebugOutput(w io.Writer) DialOption {
	return DialOption{func(do *dialOptions) {
		do.debugOutput = w
	}}
}

// DialWithDialFunc returns a DialOption that configures the ServerConn to use the
// specified function to establish both control and data connections
//
// If used together with the DialWithNetConn option, the DialWithNetConn
// takes precedence for the control connection, while data connections will
// be established using function specified with the DialWithDialFunc option
func DialWithDialFunc(f func(network, address string) (net.Conn, error)) DialOption {
	return DialOption{func(do *dialOptions) {
		do.dialFunc = f
	}}
}

func (o *dialOptions) wrapConn(netConn net.Conn) io.ReadWriteCloser {
	if o.debugOutput == nil {
		return netConn
	}

	return newDebugWrapper(netConn, o.debugOutput)
}

func (o *dialOptions) wrapStream(rd io.ReadCloser) io.ReadCloser {
	if o.debugOutput == nil {
		return rd
	}

	return newStreamDebugWrapper(rd, o.debugOutput)
}

// Connect is an alias to Dial, for backward compatibility
//
// Deprecated: Use [Dial] instead
func Connect(addr string) (*ServerConn, error) {
	return Dial(addr)
}

// DialTimeout initializes the connection to the specified ftp server address.
//
// Deprecated: Use [Dial] with [DialWithTimeout] option instead
func DialTimeout(addr string, timeout time.Duration) (*ServerConn, error) {
	return Dial(addr, DialWithTimeout(timeout))
}

// Login authenticates the client with specified user and password.
//
// "anonymous"/"anonymous" is a common user/password scheme for FTP servers
// that allows anonymous read-only accounts.
func (c *ServerConn) Login(user, password string) error {
	code, message, err := c.cmd(-1, "USER %s", user)
	if err != nil {
		return err
	}

	switch code {
	case StatusLoggedIn:
	case StatusUserOK:
		_, _, err = c.cmd(StatusLoggedIn, "PASS %s", password)
		if err != nil {
			return err
		}
	default:
		return errors.New(message)
	}

	// Probe features
	err = c.feat()
	if err != nil {
		return err
	}
	if _, mlstSupported := c.features["MLST"]; mlstSupported && !c.options.disableMLSD {
		c.mlstSupported = true
	}
	_, c.usePRET = c.features["PRET"]

	_, c.mfmtSupported = c.features["MFMT"]
	_, c.mdtmSupported = c.features["MDTM"]
	c.mdtmCanWrite = c.mdtmSupported && c.options.writingMDTM

	// Switch to binary mode
	if err = c.Type(TransferTypeBinary); err != nil {
		return err
	}

	// Switch to UTF-8
	if !c.options.disableUTF8 {
		err = c.setUTF8()
	}

	// If using implicit TLS, make data connections also use TLS
	if c.options.tlsConfig != nil {
		if _, _, err = c.cmd(StatusCommandOK, "PBSZ 0"); err != nil {
			return err
		}
		if _, _, err = c.cmd(StatusCommandOK, "PROT P"); err != nil {
			return err
		}
	}

	return err
}

// authTLS upgrades the connection to use TLS
func (c *ServerConn) authTLS() error {
	_, _, err := c.cmd(StatusAuthOK, "AUTH TLS")
	return err
}

// feat issues a FEAT FTP command to list the additional commands supported by
// the remote FTP server.
// FEAT is described in RFC 2389
func (c *ServerConn) feat() error {
	code, message, err := c.cmd(-1, "FEAT")
	if err != nil {
		return err
	}

	if code != StatusSystem {
		// The server does not support the FEAT command. This is not an
		// error: we consider that there is no additional feature.
		return nil
	}

	lines := strings.Split(message, "\n")
	for _, line := range lines {
		if !strings.HasPrefix(line, " ") {
			continue
		}

		line = strings.TrimSpace(line)
		featureElements := strings.SplitN(line, " ", 2)

		command := featureElements[0]

		var commandDesc string
		if len(featureElements) == 2 {
			commandDesc = featureElements[1]
		}

		c.features[command] = commandDesc
	}

	return nil
}

// setUTF8 issues an "OPTS UTF8 ON" command.
func (c *ServerConn) setUTF8() error {
	if _, ok := c.features["UTF8"]; !ok {
		return nil
	}

	code, message, err := c.cmd(-1, "OPTS UTF8 ON")
	if err != nil {
		return err
	}

	// Workaround for FTP servers, that does not support this option.
	if code == StatusBadArguments || code == StatusNotImplementedParameter {
		return nil
	}

	// The ftpd "filezilla-server" has FEAT support for UTF8, but always returns
	// "202 UTF8 mode is always enabled. No need to send this command." when
	// trying to use it. That's OK
	if code == StatusCommandNotImplemented {
		return nil
	}

	if code != StatusCommandOK {
		return errors.New(message)
	}

	return nil
}

// epsv issues an "EPSV" command to get a port number for a data connection.
func (c *ServerConn) epsv() (port int, err error) {
	_, line, err := c.cmd(StatusExtendedPassiveMode, "EPSV")
	if err != nil {
		return 0, err
	}

	start := strings.Index(line, "|||")
	end := strings.LastIndex(line, "|")
	if start == -1 || end == -1 {
		return 0, errors.New("invalid EPSV response format")
	}
	port, err = strconv.Atoi(line[start+3 : end])
	return port, err
}

// pasv issues a "PASV" command to get a port number for a data connection.
func (c *ServerConn) pasv() (host string, port int, err error) {
	_, line, err := c.cmd(StatusPassiveMode, "PASV")
	if err != nil {
		return "", 0, err
	}

	// PASV response format : 227 Entering Passive Mode (h1,h2,h3,h4,p1,p2).
	start := strings.Index(line, "(")
	end := strings.LastIndex(line, ")")
	if start == -1 || end == -1 {
		return "", 0, errors.New("invalid PASV response format")
	}

	// We have to split the response string
	pasvData := strings.Split(line[start+1:end], ",")

	if len(pasvData) < 6 {
		return "", 0, errors.New("invalid PASV response format")
	}

	// Let's compute the port number
	portPart1, err := strconv.Atoi(pasvData[4])
	if err != nil {
		return "", 0, err
	}

	portPart2, err := strconv.Atoi(pasvData[5])
	if err != nil {
		return "", 0, err
	}

	// Recompose port
	port = portPart1*256 + portPart2

	// Make the IP address to connect to
	host = strings.Join(pasvData[0:4], ".")
	return host, port, nil
}

// getDataConnPort returns a host, port for a new data connection
// it uses the best available method to do so
func (c *ServerConn) getDataConnPort() (string, int, error) {
	if !c.options.disableEPSV && !c.skipEPSV {
		if port, err := c.epsv(); err == nil {
			return c.host, port, nil
		}

		// if there is an error, skip EPSV for the next attempts
		c.skipEPSV = true
	}

	return c.pasv()
}

// openDataConn creates a new FTP data connection.
func (c *ServerConn) openDataConn() (net.Conn, error) {
	host, port, err := c.getDataConnPort()
	if err != nil {
		return nil, err
	}

	addr := net.JoinHostPort(host, strconv.Itoa(port))
	if c.options.dialFunc != nil {
		return c.options.dialFunc("tcp", addr)
	}

	if c.options.tlsConfig != nil {
		// We don't use tls.DialWithDialer here (which does Dial, create
		// the Client and then do the Handshake) because it seems to
		// hang with some FTP servers, namely proftpd and pureftpd.
		//
		// Instead we do Dial, create the Client and wait for the first
		// Read or Write to trigger the Handshake.
		//
		// This means that if we are uploading a zero sized file, we
		// need to make sure we do the Handshake explicitly as Write
		// won't have been called. This is done in StorFrom().
		//
		// See: https://github.com/jlaffaye/ftp/issues/282
		conn, err := c.options.dialer.Dial("tcp", addr)
		if err != nil {
			return nil, err
		}
		tlsConn := tls.Client(conn, c.options.tlsConfig)
		return tlsConn, nil
	}

	return c.options.dialer.Dial("tcp", addr)
}

// cmd is a helper function to execute a command and check for the expected FTP
// return code
func (c *ServerConn) cmd(expected int, format string, args ...interface{}) (int, string, error) {
	_, err := c.conn.Cmd(format, args...)
	if err != nil {
		return 0, "", err
	}

	return c.conn.ReadResponse(expected)
}

// cmdDataConnFrom executes a command which require a FTP data connection.
// Issues a REST FTP command to specify the number of bytes to skip for the transfer.
func (c *ServerConn) cmdDataConnFrom(offset uint64, format string, args ...interface{}) (net.Conn, error) {
	// If server requires PRET send the PRET command to warm it up
	// See: https://tools.ietf.org/html/draft-dd-pret-00
	if c.usePRET {
		_, _, err := c.cmd(-1, "PRET "+format, args...)
		if err != nil {
			return nil, err
		}
	}

	conn, err := c.openDataConn()
	if err != nil {
		return nil, err
	}

	if offset != 0 {
		_, _, err = c.cmd(StatusRequestFilePending, "REST %d", offset)
		if err != nil {
			_ = conn.Close()
			return nil, err
		}
	}

	_, err = c.conn.Cmd(format, args...)
	if err != nil {
		_ = conn.Close()
		return nil, err
	}

	code, msg, err := c.conn.ReadResponse(-1)
	if err != nil {
		_ = conn.Close()
		return nil, err
	}
	if code != StatusAlreadyOpen && code != StatusAboutToSend {
		_ = conn.Close()
		return nil, &textproto.Error{Code: code, Msg: msg}
	}

	return conn, nil
}

// Type switches the transfer mode for the connection.
func (c *ServerConn) Type(transferType TransferType) (err error) {
	_, _, err = c.cmd(StatusCommandOK, "TYPE "+string(transferType))
	return err
}

// NameList issues an NLST FTP command.
func (c *ServerConn) NameList(path string) (entries []string, err error) {
	space := " "
	if path == "" {
		space = ""
	}
	conn, err := c.cmdDataConnFrom(0, "NLST%s%s", space, path)
	if err != nil {
		return nil, err
	}

	var errs *multierror.Error

	r := &Response{conn: conn, c: c}

	scanner := bufio.NewScanner(c.options.wrapStream(r))
	for scanner.Scan() {
		entries = append(entries, scanner.Text())
	}

	if err := scanner.Err(); err != nil {
		errs = multierror.Append(errs, err)
	}
	if err := r.Close(); err != nil {
		errs = multierror.Append(errs, err)
	}

	return entries, errs.ErrorOrNil()
}

// List issues a LIST FTP command.
func (c *ServerConn) List(path string) (entries []*Entry, err error) {
	var cmd string
	var parser parseFunc

	if c.mlstSupported && !c.options.forceListHidden {
		cmd = "MLSD"
		parser = parseRFC3659ListLine
	} else {
		cmd = "LIST"
		if c.options.forceListHidden {
			cmd += " -a"
		}
		parser = parseListLine
	}

	space := " "
	if path == "" {
		space = ""
	}
	conn, err := c.cmdDataConnFrom(0, "%s%s%s", cmd, space, path)
	if err != nil {
		return nil, err
	}

	var errs *multierror.Error

	r := &Response{conn: conn, c: c}

	scanner := bufio.NewScanner(c.options.wrapStream(r))
	now := time.Now()
	for scanner.Scan() {
		entry, errParse := parser(scanner.Text(), now, c.options.location)
		if errParse == nil {
			entries = append(entries, entry)
		}
	}

	if err := scanner.Err(); err != nil {
		errs = multierror.Append(errs, err)
	}
	if err := r.Close(); err != nil {
		errs = multierror.Append(errs, err)
	}

	return entries, errs.ErrorOrNil()
}

// GetEntry issues a MLST FTP command which retrieves one single Entry using the
// control connection. The returnedEntry will describe the current directory
// when no path is given.
func (c *ServerConn) GetEntry(path string) (entry *Entry, err error) {
	if !c.mlstSupported {
		return nil, &textproto.Error{Code: StatusNotImplemented, Msg: StatusText(StatusNotImplemented)}
	}
	space := " "
	if path == "" {
		space = ""
	}
	_, msg, err := c.cmd(StatusRequestedFileActionOK, "%s%s%s", "MLST", space, path)
	if err != nil {
		return nil, err
	}

	// The expected reply will look something like:
	//
	//    250-File details
	//     Type=file;Size=1024;Modify=20220813133357; path
	//    250 End
	//
	// Multiple lines are allowed though, so it can also be in the form:
	//
	//    250-File details
	//     Type=file;Size=1024; path
	//     Modify=20220813133357; path
	//    250 End
	lines := strings.Split(msg, "\n")
	lc := len(lines)

	// lines must be a multi-line message with a length of 3 or more, and we
	// don't care about the first and last line
	if lc < 3 {
		return nil, errors.New("invalid response")
	}

	e := &Entry{}
	for _, l := range lines[1 : lc-1] {
		// According to RFC 3659, the entry lines must start with a space when passed over the
		// control connection. Some servers don't seem to add that space though. Both forms are
		// accepted here.
		if len(l) > 0 && l[0] == ' ' {
			l = l[1:]
		}
		// Some severs seem to send a blank line at the end which we ignore
		if l == "" {
			continue
		}
		if e, err = parseNextRFC3659ListLine(l, c.options.location, e); err != nil {
			return nil, err
		}
	}
	return e, nil
}

// IsTimePreciseInList returns true if client and server support the MLSD
// command so List can return time with 1-second precision for all files.
func (c *ServerConn) IsTimePreciseInList() bool {
	return c.mlstSupported
}

// ChangeDir issues a CWD FTP command, which changes the current directory to
// the specified path.
func (c *ServerConn) ChangeDir(path string) error {
	_, _, err := c.cmd(StatusRequestedFileActionOK, "CWD %s", path)
	return err
}

// ChangeDirToParent issues a CDUP FTP command, which changes the current
// directory to the parent directory.  This is similar to a call to ChangeDir
// with a path set to "..".
func (c *ServerConn) ChangeDirToParent() error {
	_, _, err := c.cmd(StatusRequestedFileActionOK, "CDUP")
	return err
}

// CurrentDir issues a PWD FTP command, which Returns the path of the current
// directory.
func (c *ServerConn) CurrentDir() (string, error) {
	_, msg, err := c.cmd(StatusPathCreated, "PWD")
	if err != nil {
		return "", err
	}

	start := strings.Index(msg, "\"")
	end := strings.LastIndex(msg, "\"")

	if start == -1 || end == -1 {
		return "", errors.New("unsuported PWD response format")
	}

	return msg[start+1 : end], nil
}

// FileSize issues a SIZE FTP command, which Returns the size of the file
func (c *ServerConn) FileSize(path string) (int64, error) {
	_, msg, err := c.cmd(StatusFile, "SIZE %s", path)
	if err != nil {
		return 0, err
	}

	return strconv.ParseInt(msg, 10, 64)
}

// GetTime issues the MDTM FTP command to obtain the file modification time.
// It returns a UTC time.
func (c *ServerConn) GetTime(path string) (time.Time, error) {
	var t time.Time
	if !c.mdtmSupported {
		return t, errors.New("GetTime is not supported")
	}
	_, msg, err := c.cmd(StatusFile, "MDTM %s", path)
	if err != nil {
		return t, err
	}
	return time.ParseInLocation(timeFormat, msg, time.UTC)
}

// IsGetTimeSupported allows library callers to check in advance that they
// can use GetTime to get file time.
func (c *ServerConn) IsGetTimeSupported() bool {
	return c.mdtmSupported
}

// SetTime issues the MFMT FTP command to set the file modification time.
// Also it can use a non-standard form of the MDTM command supported by
// the VsFtpd server instead of MFMT for the same purpose.
// See "mdtm_write" in https://security.appspot.com/vsftpd/vsftpd_conf.html
func (c *ServerConn) SetTime(path string, t time.Time) (err error) {
	utime := t.In(time.UTC).Format(timeFormat)
	switch {
	case c.mfmtSupported:
		_, _, err = c.cmd(StatusFile, "MFMT %s %s", utime, path)
	case c.mdtmCanWrite:
		_, _, err = c.cmd(StatusFile, "MDTM %s %s", utime, path)
	default:
		err = errors.New("SetTime is not supported")
	}
	return
}

// IsSetTimeSupported allows library callers to check in advance that they
// can use SetTime to set file time.
func (c *ServerConn) IsSetTimeSupported() bool {
	return c.mfmtSupported || c.mdtmCanWrite
}

// Retr issues a RETR FTP command to fetch the specified file from the remote
// FTP server.
//
// The returned ReadCloser must be closed to cleanup the FTP data connection.
func (c *ServerConn) Retr(path string) (*Response, error) {
	return c.RetrFrom(path, 0)
}

// RetrFrom issues a RETR FTP command to fetch the specified file from the remote
// FTP server, the server will not send the offset first bytes of the file.
//
// The returned ReadCloser must be closed to cleanup the FTP data connection.
func (c *ServerConn) RetrFrom(path string, offset uint64) (*Response, error) {
	conn, err := c.cmdDataConnFrom(offset, "RETR %s", path)
	if err != nil {
		return nil, err
	}

	return &Response{conn: conn, c: c}, nil
}

// Stor issues a STOR FTP command to store a file to the remote FTP server.
// Stor creates the specified file with the content of the io.Reader.
//
// Hint: io.Pipe() can be used if an io.Writer is required.
func (c *ServerConn) Stor(path string, r io.Reader) error {
	return c.StorFrom(path, r, 0)
}

// checkDataShut reads the "closing data connection" status from the
// control connection. It is called after transferring a piece of data
// on the data connection during which the control connection was idle.
// This may result in the idle timeout triggering on the control connection
// right when we try to read the response.
// The ShutTimeout dial option will rescue here. It will nudge the control
// connection deadline right before checking the data closing status.
func (c *ServerConn) checkDataShut() error {
	if c.options.shutTimeout != 0 {
		shutDeadline := time.Now().Add(c.options.shutTimeout)
		if err := c.netConn.SetDeadline(shutDeadline); err != nil {
			return err
		}
	}
	_, _, err := c.conn.ReadResponse(StatusClosingDataConnection)
	return err
}

// StorFrom issues a STOR FTP command to store a file to the remote FTP server.
// Stor creates the specified file with the content of the io.Reader, writing
// on the server will start at the given file offset.
//
// Hint: io.Pipe() can be used if an io.Writer is required.
func (c *ServerConn) StorFrom(path string, r io.Reader, offset uint64) error {
	conn, err := c.cmdDataConnFrom(offset, "STOR %s", path)
	if err != nil {
		return err
	}

	var errs *multierror.Error

	// if the upload fails we still need to try to read the server
	// response otherwise if the failure is not due to a connection problem,
	// for example the server denied the upload for quota limits, we miss
	// the response and we cannot use the connection to send other commands.
	if n, err := io.Copy(conn, r); err != nil {
		errs = multierror.Append(errs, err)
	} else if n == 0 {
		// If we wrote no bytes and got no error, make sure we call
		// tls.Handshake on the connection as it won't get called
		// unless Write() is called. (See comment in openDataConn()).
		//
		// ProFTP doesn't like this and returns "Unable to build data
		// connection: Operation not permitted" when trying to upload
		// an empty file without this.
		if do, ok := conn.(interface{ Handshake() error }); ok {
			if err := do.Handshake(); err != nil {
				errs = multierror.Append(errs, err)
			}
		}
	}

	if err := conn.Close(); err != nil {
		errs = multierror.Append(errs, err)
	}

	if err := c.checkDataShut(); err != nil {
		errs = multierror.Append(errs, err)
	}

	return errs.ErrorOrNil()
}

// Append issues a APPE FTP command to store a file to the remote FTP server.
// If a file already exists with the given path, then the content of the
// io.Reader is appended. Otherwise, a new file is created with that content.
//
// Hint: io.Pipe() can be used if an io.Writer is required.
func (c *ServerConn) Append(path string, r io.Reader) error {
	conn, err := c.cmdDataConnFrom(0, "APPE %s", path)
	if err != nil {
		return err
	}

	var errs *multierror.Error

	if _, err := io.Copy(conn, r); err != nil {
		errs = multierror.Append(errs, err)
	}

	if err := conn.Close(); err != nil {
		errs = multierror.Append(errs, err)
	}

	if err := c.checkDataShut(); err != nil {
		errs = multierror.Append(errs, err)
	}

	return errs.ErrorOrNil()
}

// Rename renames a file on the remote FTP server.
func (c *ServerConn) Rename(from, to string) error {
	_, _, err := c.cmd(StatusRequestFilePending, "RNFR %s", from)
	if err != nil {
		return err
	}

	_, _, err = c.cmd(StatusRequestedFileActionOK, "RNTO %s", to)
	return err
}

// Delete issues a DELE FTP command to delete the specified file from the
// remote FTP server.
func (c *ServerConn) Delete(path string) error {
	_, _, err := c.cmd(StatusRequestedFileActionOK, "DELE %s", path)
	return err
}

// RemoveDirRecur deletes a non-empty folder recursively using
// RemoveDir and Delete
func (c *ServerConn) RemoveDirRecur(path string) error {
	err := c.ChangeDir(path)
	if err != nil {
		return err
	}
	currentDir, err := c.CurrentDir()
	if err != nil {
		return err
	}

	entries, err := c.List(currentDir)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		if entry.Name != ".." && entry.Name != "." {
			if entry.Type == EntryTypeFolder {
				err = c.RemoveDirRecur(currentDir + "/" + entry.Name)
				if err != nil {
					return err
				}
			} else {
				err = c.Delete(entry.Name)
				if err != nil {
					return err
				}
			}
		}
	}
	err = c.ChangeDirToParent()
	if err != nil {
		return err
	}
	err = c.RemoveDir(currentDir)
	return err
}

// MakeDir issues a MKD FTP command to create the specified directory on the
// remote FTP server.
func (c *ServerConn) MakeDir(path string) error {
	_, _, err := c.cmd(StatusPathCreated, "MKD %s", path)
	return err
}

// RemoveDir issues a RMD FTP command to remove the specified directory from
// the remote FTP server.
func (c *ServerConn) RemoveDir(path string) error {
	_, _, err := c.cmd(StatusRequestedFileActionOK, "RMD %s", path)
	return err
}

// Walk prepares the internal walk function so that the caller can begin traversing the directory
func (c *ServerConn) Walk(root string) *Walker {
	w := new(Walker)
	w.serverConn = c

	if !strings.HasSuffix(root, "/") {
		root += "/"
	}

	w.root = root
	w.descend = true

	return w
}

// NoOp issues a NOOP FTP command.
// NOOP has no effects and is usually used to prevent the remote FTP server to
// close the otherwise idle connection.
func (c *ServerConn) NoOp() error {
	_, _, err := c.cmd(StatusCommandOK, "NOOP")
	return err
}

// Logout issues a REIN FTP command to logout the current user.
func (c *ServerConn) Logout() error {
	_, _, err := c.cmd(StatusReady, "REIN")
	return err
}

// Quit issues a QUIT FTP command to properly close the connection from the
// remote FTP server.
func (c *ServerConn) Quit() error {
	var errs *multierror.Error

	if _, err := c.conn.Cmd("QUIT"); err != nil {
		errs = multierror.Append(errs, err)
	}

	if err := c.conn.Close(); err != nil {
		errs = multierror.Append(errs, err)
	}

	return errs.ErrorOrNil()
}

// Read implements the io.Reader interface on a FTP data connection.
func (r *Response) Read(buf []byte) (int, error) {
	return r.conn.Read(buf)
}

// Close implements the io.Closer interface on a FTP data connection.
// After the first call, Close will do nothing and return nil.
func (r *Response) Close() error {
	if r.closed {
		return nil
	}

	var errs *multierror.Error

	if err := r.conn.Close(); err != nil {
		errs = multierror.Append(errs, err)
	}

	if err := r.c.checkDataShut(); err != nil {
		errs = multierror.Append(errs, err)
	}

	r.closed = true
	return errs.ErrorOrNil()
}

// SetDeadline sets the deadlines associated with the connection.
func (r *Response) SetDeadline(t time.Time) error {
	return r.conn.SetDeadline(t)
}

// String returns the string representation of EntryType t.
func (t EntryType) String() string {
	return [...]string{"file", "folder", "link"}[t]
}
