// Package ftp implements a FTP client as described in RFC 959.
package ftp

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"net"
	"net/textproto"
	"strconv"
	"strings"
	"time"
)

// EntryType describes the different types of an Entry.
type EntryType int

// The differents types of an Entry
const (
	EntryTypeFile EntryType = iota
	EntryTypeFolder
	EntryTypeLink
)

// ServerConn represents the connection to a remote FTP server.
type ServerConn struct {
	conn        *textproto.Conn
	host        string
	timeout     time.Duration
	features    map[string]string
	disableEPSV bool
}

// Entry describes a file and is returned by List().
type Entry struct {
	Name string
	Type EntryType
	Size uint64
	Time time.Time
}

// response represent a data-connection
type response struct {
	conn net.Conn
	c    *ServerConn
}

// Connect is an alias to Dial, for backward compatibility
func Connect(addr string) (*ServerConn, error) {
	return Dial(addr)
}

// Dial is like DialTimeout with no timeout
func Dial(addr string) (*ServerConn, error) {
	return DialTimeout(addr, 0)
}

// DialTimeout initializes the connection to the specified ftp server address.
//
// It is generally followed by a call to Login() as most FTP commands require
// an authenticated user.
func DialTimeout(addr string, timeout time.Duration) (*ServerConn, error) {
	tconn, err := net.DialTimeout("tcp", addr, timeout)
	if err != nil {
		return nil, err
	}

	// Use the resolved IP address in case addr contains a domain name
	// If we use the domain name, we might not resolve to the same IP.
	remoteAddr := tconn.RemoteAddr().String()
	host, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		return nil, err
	}

	conn := textproto.NewConn(tconn)

	c := &ServerConn{
		conn:     conn,
		host:     host,
		timeout:  timeout,
		features: make(map[string]string),
	}

	_, _, err = c.conn.ReadResponse(StatusReady)
	if err != nil {
		c.Quit()
		return nil, err
	}

	err = c.feat()
	if err != nil {
		c.Quit()
		return nil, err
	}

	return c, nil
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

	// Switch to binary mode
	_, _, err = c.cmd(StatusCommandOK, "TYPE I")
	if err != nil {
		return err
	}

	return nil
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

// epsv issues an "EPSV" command to get a port number for a data connection.
func (c *ServerConn) epsv() (port int, err error) {
	_, line, err := c.cmd(StatusExtendedPassiveMode, "EPSV")
	if err != nil {
		return
	}

	start := strings.Index(line, "|||")
	end := strings.LastIndex(line, "|")
	if start == -1 || end == -1 {
		err = errors.New("Invalid EPSV response format")
		return
	}
	port, err = strconv.Atoi(line[start+3 : end])
	return
}

// pasv issues a "PASV" command to get a port number for a data connection.
func (c *ServerConn) pasv() (port int, err error) {
	_, line, err := c.cmd(StatusPassiveMode, "PASV")
	if err != nil {
		return
	}

	// PASV response format : 227 Entering Passive Mode (h1,h2,h3,h4,p1,p2).
	start := strings.Index(line, "(")
	end := strings.LastIndex(line, ")")
	if start == -1 || end == -1 {
		return 0, errors.New("Invalid PASV response format")
	}

	// We have to split the response string
	pasvData := strings.Split(line[start+1:end], ",")

	if len(pasvData) < 6 {
		return 0, errors.New("Invalid PASV response format")
	}

	// Let's compute the port number
	portPart1, err1 := strconv.Atoi(pasvData[4])
	if err1 != nil {
		err = err1
		return
	}

	portPart2, err2 := strconv.Atoi(pasvData[5])
	if err2 != nil {
		err = err2
		return
	}

	// Recompose port
	port = portPart1*256 + portPart2
	return
}

// getDataConnPort returns a port for a new data connection
// it uses the best available method to do so
func (c *ServerConn) getDataConnPort() (int, error) {
	if !c.disableEPSV {
		if port, err := c.epsv(); err == nil {
			return port, nil
		}

		// if there is an error, disable EPSV for the next attempts
		c.disableEPSV = true
	}

	return c.pasv()
}

// openDataConn creates a new FTP data connection.
func (c *ServerConn) openDataConn() (net.Conn, error) {
	port, err := c.getDataConnPort()
	if err != nil {
		return nil, err
	}

	return net.DialTimeout("tcp", net.JoinHostPort(c.host, strconv.Itoa(port)), c.timeout)
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
	conn, err := c.openDataConn()
	if err != nil {
		return nil, err
	}

	if offset != 0 {
		_, _, err := c.cmd(StatusRequestFilePending, "REST %d", offset)
		if err != nil {
			conn.Close()
			return nil, err
		}
	}

	_, err = c.conn.Cmd(format, args...)
	if err != nil {
		conn.Close()
		return nil, err
	}

	code, msg, err := c.conn.ReadResponse(-1)
	if err != nil {
		conn.Close()
		return nil, err
	}
	if code != StatusAlreadyOpen && code != StatusAboutToSend {
		conn.Close()
		return nil, &textproto.Error{Code: code, Msg: msg}
	}

	return conn, nil
}

var errUnsupportedListLine = errors.New("Unsupported LIST line")

// parseRFC3659ListLine parses the style of directory line defined in RFC 3659.
func parseRFC3659ListLine(line string) (*Entry, error) {
	iSemicolon := strings.Index(line, ";")
	iWhitespace := strings.Index(line, " ")

	if iSemicolon < 0 || iSemicolon > iWhitespace {
		return nil, errUnsupportedListLine
	}

	e := &Entry{
		Name: line[iWhitespace+1:],
	}

	for _, field := range strings.Split(line[:iWhitespace-1], ";") {
		i := strings.Index(field, "=")
		if i < 1 {
			return nil, errUnsupportedListLine
		}

		key := field[:i]
		value := field[i+1:]

		switch key {
		case "modify":
			var err error
			e.Time, err = time.Parse("20060102150405", value)
			if err != nil {
				return nil, err
			}
		case "type":
			switch value {
			case "dir", "cdir", "pdir":
				e.Type = EntryTypeFolder
			case "file":
				e.Type = EntryTypeFile
			}
		case "size":
			e.setSize(value)
		}
	}
	return e, nil
}

// parse file or folder name with multiple spaces
func parseLsListLineName(line string, fields []string, offset int) string {
	if offset < 1 {
		return ""
	}

	match := fmt.Sprintf(" %s ", fields[offset-1])
	index := strings.Index(line, match)
	if index == -1 {
		return ""
	}

	index += len(match)
	return strings.TrimSpace(line[index:])
}

// parseLsListLine parses a directory line in a format based on the output of
// the UNIX ls command.
func parseLsListLine(line string) (*Entry, error) {
	fields := strings.Fields(line)
	if len(fields) >= 7 && fields[1] == "folder" && fields[2] == "0" {
		e := &Entry{
			Type: EntryTypeFolder,
			Name: strings.Join(fields[6:], " "),
		}
		if err := e.setTime(fields[3:6]); err != nil {
			return nil, err
		}

		return e, nil
	}

	if len(fields) < 8 {
		return nil, errUnsupportedListLine
	}

	if fields[1] == "0" {
		e := &Entry{
			Type: EntryTypeFile,
			Name: strings.Join(fields[7:], " "),
		}

		if err := e.setSize(fields[2]); err != nil {
			return nil, err
		}
		if err := e.setTime(fields[4:7]); err != nil {
			return nil, err
		}

		return e, nil
	}

	if len(fields) < 9 {
		return nil, errUnsupportedListLine
	}

	e := &Entry{}
	switch fields[0][0] {
	case '-':
		e.Type = EntryTypeFile
		if err := e.setSize(fields[4]); err != nil {
			return nil, err
		}
	case 'd':
		e.Type = EntryTypeFolder
	case 'l':
		e.Type = EntryTypeLink
	default:
		return nil, errors.New("Unknown entry type")
	}

	if err := e.setTime(fields[5:8]); err != nil {
		return nil, err
	}

	e.Name = parseLsListLineName(line, fields, 8)
	if len(e.Name) == 0 {
		e.Name = strings.Join(fields[8:], " ")
	}

	return e, nil
}

var dirTimeFormats = []string{
	"01-02-06  03:04PM",
	"2006-01-02  15:04",
}

// parseDirListLine parses a directory line in a format based on the output of
// the MS-DOS DIR command.
func parseDirListLine(line string) (*Entry, error) {
	e := &Entry{}
	var err error

	// Try various time formats that DIR might use, and stop when one works.
	for _, format := range dirTimeFormats {
		if len(line) > len(format) {
			e.Time, err = time.Parse(format, line[:len(format)])
			if err == nil {
				line = line[len(format):]
				break
			}
		}
	}
	if err != nil {
		// None of the time formats worked.
		return nil, errUnsupportedListLine
	}

	line = strings.TrimLeft(line, " ")
	if strings.HasPrefix(line, "<DIR>") {
		e.Type = EntryTypeFolder
		line = strings.TrimPrefix(line, "<DIR>")
	} else {
		space := strings.Index(line, " ")
		if space == -1 {
			return nil, errUnsupportedListLine
		}
		e.Size, err = strconv.ParseUint(line[:space], 10, 64)
		if err != nil {
			return nil, errUnsupportedListLine
		}
		e.Type = EntryTypeFile
		line = line[space:]
	}

	e.Name = strings.TrimLeft(line, " ")
	return e, nil
}

var listLineParsers = []func(line string) (*Entry, error){
	parseRFC3659ListLine,
	parseLsListLine,
	parseDirListLine,
}

// parseListLine parses the various non-standard format returned by the LIST
// FTP command.
func parseListLine(line string) (*Entry, error) {
	for _, f := range listLineParsers {
		e, err := f(line)
		if err == errUnsupportedListLine {
			// Try another format.
			continue
		}
		return e, err
	}
	return nil, errUnsupportedListLine
}

func (e *Entry) setSize(str string) (err error) {
	e.Size, err = strconv.ParseUint(str, 0, 64)
	return
}

func (e *Entry) setTime(fields []string) (err error) {
	var timeStr string
	if strings.Contains(fields[2], ":") { // this year
		thisYear, _, _ := time.Now().Date()
		timeStr = fields[1] + " " + fields[0] + " " + strconv.Itoa(thisYear)[2:4] + " " + fields[2] + " GMT"
	} else { // not this year
		if len(fields[2]) != 4 {
			return errors.New("Invalid year format in time string")
		}
		timeStr = fields[1] + " " + fields[0] + " " + fields[2][2:4] + " 00:00 GMT"
	}
	e.Time, err = time.Parse("_2 Jan 06 15:04 MST", timeStr)
	return
}

// NameList issues an NLST FTP command.
func (c *ServerConn) NameList(path string) (entries []string, err error) {
	conn, err := c.cmdDataConnFrom(0, "NLST %s", path)
	if err != nil {
		return
	}

	r := &response{conn, c}
	defer r.Close()

	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		entries = append(entries, scanner.Text())
	}
	if err = scanner.Err(); err != nil {
		return entries, err
	}
	return
}

// List issues a LIST FTP command.
func (c *ServerConn) List(path string) (entries []*Entry, err error) {
	conn, err := c.cmdDataConnFrom(0, "LIST %s", path)
	if err != nil {
		return
	}

	r := &response{conn, c}
	defer r.Close()

	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := scanner.Text()
		entry, err := parseListLine(line)
		if err == nil {
			entries = append(entries, entry)
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return
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
		return "", errors.New("Unsuported PWD response format")
	}

	return msg[start+1 : end], nil
}

// Retr issues a RETR FTP command to fetch the specified file from the remote
// FTP server.
//
// The returned ReadCloser must be closed to cleanup the FTP data connection.
func (c *ServerConn) Retr(path string) (io.ReadCloser, error) {
	return c.RetrFrom(path, 0)
}

// RetrFrom issues a RETR FTP command to fetch the specified file from the remote
// FTP server, the server will not send the offset first bytes of the file.
//
// The returned ReadCloser must be closed to cleanup the FTP data connection.
func (c *ServerConn) RetrFrom(path string, offset uint64) (io.ReadCloser, error) {
	conn, err := c.cmdDataConnFrom(offset, "RETR %s", path)
	if err != nil {
		return nil, err
	}

	return &response{conn, c}, nil
}

// Stor issues a STOR FTP command to store a file to the remote FTP server.
// Stor creates the specified file with the content of the io.Reader.
//
// Hint: io.Pipe() can be used if an io.Writer is required.
func (c *ServerConn) Stor(path string, r io.Reader) error {
	return c.StorFrom(path, r, 0)
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

	_, err = io.Copy(conn, r)
	conn.Close()
	if err != nil {
		return err
	}

	_, _, err = c.conn.ReadResponse(StatusClosingDataConnection)
	return err
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
	c.conn.Cmd("QUIT")
	return c.conn.Close()
}

// Read implements the io.Reader interface on a FTP data connection.
func (r *response) Read(buf []byte) (int, error) {
	return r.conn.Read(buf)
}

// Close implements the io.Closer interface on a FTP data connection.
func (r *response) Close() error {
	err := r.conn.Close()
	_, _, err2 := r.c.conn.ReadResponse(StatusClosingDataConnection)
	if err2 != nil {
		err = err2
	}
	return err
}
