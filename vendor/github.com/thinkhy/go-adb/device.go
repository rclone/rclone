package adb

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/thinkhy/go-adb/internal/errors"
	"github.com/thinkhy/go-adb/wire"
)

// MtimeOfClose should be passed to OpenWrite to set the file modification time to the time the Close
// method is called.
var MtimeOfClose = time.Time{}

// Device communicates with a specific Android device.
// To get an instance, call Device() on an Adb.
type Device struct {
	server     server
	descriptor DeviceDescriptor

	// Used to get device info.
	deviceListFunc func() ([]*DeviceInfo, error)
}

func (c *Device) String() string {
	return c.descriptor.String()
}

// get-product is documented, but not implemented, in the server.
// TODO(z): Make product exported if get-product is ever implemented in adb.
func (c *Device) product() (string, error) {
	attr, err := c.getAttribute("get-product")
	return attr, wrapClientError(err, c, "Product")
}

func (c *Device) Serial() (string, error) {
	attr, err := c.getAttribute("get-serialno")
	return attr, wrapClientError(err, c, "Serial")
}

func (c *Device) DevicePath() (string, error) {
	attr, err := c.getAttribute("get-devpath")
	return attr, wrapClientError(err, c, "DevicePath")
}

func (c *Device) State() (DeviceState, error) {
	attr, err := c.getAttribute("get-state")
	state, err := parseDeviceState(attr)
	return state, wrapClientError(err, c, "State")
}

var (
	FProtocolTcp        = "tcp"
	FProtocolAbstract   = "localabstract"
	FProtocolReserved   = "localreserved"
	FProtocolFilesystem = "localfilesystem"
)

type ForwardSpec struct {
	Protocol   string
	PortOrName string
}

func (f ForwardSpec) String() string {
	return fmt.Sprintf("%s:%s", f.Protocol, f.PortOrName)
}

func (f ForwardSpec) Port() (int, error) {
	if f.Protocol != FProtocolTcp {
		return 0, fmt.Errorf("protocal is not tcp")
	}
	return strconv.Atoi(f.PortOrName)
}

func (f *ForwardSpec) parseString(s string) error {
	fields := strings.Split(s, ":")
	if len(fields) != 2 {
		return fmt.Errorf("expect string contains only one ':', str = %s", s)
	}
	f.Protocol = fields[0]
	f.PortOrName = fields[1]
	return nil
}

type ForwardPair struct {
	Serial string
	Local  ForwardSpec
	Remote ForwardSpec
}

// ForwardList returns list with struct ForwardPair
// If no device serial specified all devices's forward list will returned
func (c *Device) ForwardList() (fs []ForwardPair, err error) {
	attr, err := c.getAttribute("list-forward")
	if err != nil {
		return nil, err
	}
	fields := strings.Fields(attr)
	if len(fields)%3 != 0 {
		return nil, fmt.Errorf("list forward parse error")
	}
	fs = make([]ForwardPair, 0)
	for i := 0; i < len(fields)/3; i++ {
		var local, remote ForwardSpec
		var serial = fields[i*3]
		// skip other device serial forwards
		if c.descriptor.descriptorType == DeviceSerial && c.descriptor.serial != serial {
			continue
		}
		if err = local.parseString(fields[i*3+1]); err != nil {
			return nil, err
		}
		if err = remote.parseString(fields[i*3+2]); err != nil {
			return nil, err
		}
		fs = append(fs, ForwardPair{serial, local, remote})
	}
	return fs, nil
}

// ForwardRemove specified forward
func (c *Device) ForwardRemove(local ForwardSpec) error {
	err := roundTripSingleNoResponse(c.server,
		fmt.Sprintf("%s:killforward:%v", c.descriptor.getHostPrefix(), local))
	return wrapClientError(err, c, "ForwardRemove")
}

// ForwardRemoveAll cancel all exists forwards
func (c *Device) ForwardRemoveAll() error {
	err := roundTripSingleNoResponse(c.server,
		fmt.Sprintf("%s:killforward-all", c.descriptor.getHostPrefix()))
	return wrapClientError(err, c, "ForwardRemoveAll")
}

// Forward remote connection to local
func (c *Device) Forward(local, remote ForwardSpec) error {
	err := roundTripSingleNoResponse(c.server,
		fmt.Sprintf("%s:forward:%v;%v", c.descriptor.getHostPrefix(), local, remote))
	return wrapClientError(err, c, "Forward")
}

// ForwardToFreePort return random generated port
// If forward already exists, just return current forworded port
func (c *Device) ForwardToFreePort(remote ForwardSpec) (port int, err error) {
	fws, err := c.ForwardList()
	if err != nil {
		return
	}
	for _, fw := range fws {
		if fw.Remote == remote {
			return fw.Local.Port()
		}
	}
	port, err = getFreePort()
	if err != nil {
		return
	}
	err = c.Forward(ForwardSpec{FProtocolTcp, strconv.Itoa(port)}, remote)
	return
}

func (c *Device) DeviceInfo() (*DeviceInfo, error) {
	// Adb doesn't actually provide a way to get this for an individual device,
	// so we have to just list devices and find ourselves.

	serial, err := c.Serial()
	if err != nil {
		return nil, wrapClientError(err, c, "GetDeviceInfo(GetSerial)")
	}

	devices, err := c.deviceListFunc()
	if err != nil {
		return nil, wrapClientError(err, c, "DeviceInfo(ListDevices)")
	}

	for _, deviceInfo := range devices {
		if deviceInfo.Serial == serial {
			return deviceInfo, nil
		}
	}

	err = errors.Errorf(errors.DeviceNotFound, "device list doesn't contain serial %s", serial)
	return nil, wrapClientError(err, c, "DeviceInfo")
}

/*
RunCommand runs the specified commands on a shell on the device.

From the Android docs:
	Run 'command arg1 arg2 ...' in a shell on the device, and return
	its output and error streams. Note that arguments must be separated
	by spaces. If an argument contains a space, it must be quoted with
	double-quotes. Arguments cannot contain double quotes or things
	will go very wrong.

	Note that this is the non-interactive version of "adb shell"
Source: https://android.googlesource.com/platform/system/core/+/master/adb/SERVICES.TXT

This method quotes the arguments for you, and will return an error if any of them
contain double quotes.

Because the adb shell converts all "\n" into "\r\n",
so here we convert it back (maybe not good for binary output)
*/
func (c *Device) RunCommand(cmd string, args ...string) (string, error) {
	conn, err := c.OpenCommand(cmd, args...)
	if err != nil {
		return "", err
	}

	defer conn.Close()
	resp, err := conn.ReadUntilEof()
	if err != nil {
		return "", wrapClientError(err, c, "RunCommand")
	}
	outStr := strings.Replace(string(resp), "\r\n", "\n", -1)
	return outStr, nil
}

func (c *Device) OpenCommand(cmd string, args ...string) (conn *wire.Conn, err error) {
	cmd, err = prepareCommandLine(cmd, args...)
	if err != nil {
		return nil, wrapClientError(err, c, "RunCommand")
	}
	conn, err = c.dialDevice()
	if err != nil {
		return nil, wrapClientError(err, c, "RunCommand")
	}
	defer func() {
		if err != nil && conn != nil {
			conn.Close()
		}
	}()

	req := fmt.Sprintf("shell:%s", cmd)

	// Shell responses are special, they don't include a length header.
	// We read until the stream is closed.
	// So, we can't use conn.RoundTripSingleResponse.
	if err = conn.SendMessage([]byte(req)); err != nil {
		return nil, wrapClientError(err, c, "Command")
	}
	if _, err = conn.ReadStatus(req); err != nil {
		return nil, wrapClientError(err, c, "Command")
	}
	return conn, nil
}

func (c *Device) RunCommandInContext(conn *wire.Conn, cmd string, args ...string) (*wire.Conn, string, error) {
	conn, err := c.OpenCmdInContext(conn, cmd, args...)
	if err != nil {
		return nil, "", err
	}

	resp, err := conn.ReadUntilEof()
	if err != nil {
		return nil, "", wrapClientError(err, c, "RunCommand")
	}
	outStr := strings.Replace(string(resp), "\r\n", "\n", -1)
	return conn, outStr, nil
}

func (c *Device) OpenCmdInContext(conn *wire.Conn, cmd string, args ...string) (*wire.Conn, error) {
	cmd, err := prepareCommandLine(cmd, args...)
	if err != nil {
		return nil, wrapClientError(err, c, "RunCommand")
	}
	if conn == nil {
		conn, err = c.dialDevice()
	}
	if err != nil {
		return nil, wrapClientError(err, c, "RunCommand")
	}

	req := fmt.Sprintf("shell:%s", cmd)

	// Shell responses are special, they don't include a length header.
	// We read until the stream is closed.
	// So, we can't use conn.RoundTripSingleResponse.
	if err = conn.SendMessage([]byte(req)); err != nil {
		return nil, wrapClientError(err, c, "Command")
	}
	if _, err = conn.ReadStatus(req); err != nil {
		return nil, wrapClientError(err, c, "Command")
	}

	return conn, nil
}

/*
Remount, from the official adb command’s docs:
	Ask adbd to remount the device's filesystem in read-write mode,
	instead of read-only. This is usually necessary before performing
	an "adb sync" or "adb push" request.
	This request may not succeed on certain builds which do not allow
	that.
Source: https://android.googlesource.com/platform/system/core/+/master/adb/SERVICES.TXT
*/
func (c *Device) Remount() (string, error) {
	conn, err := c.dialDevice()
	if err != nil {
		return "", wrapClientError(err, c, "Remount")
	}
	defer conn.Close()

	resp, err := conn.RoundTripSingleResponse([]byte("remount"))
	return string(resp), wrapClientError(err, c, "Remount")
}

func (c *Device) ListDirEntries(path string) (*DirEntries, error) {
	conn, err := c.getSyncConn()
	if err != nil {
		return nil, wrapClientError(err, c, "ListDirEntries(%s)", path)
	}

	entries, err := listDirEntries(conn, path)
	return entries, wrapClientError(err, c, "ListDirEntries(%s)", path)
}

func (c *Device) Stat(path string) (*DirEntry, error) {
	conn, err := c.getSyncConn()
	if err != nil {
		return nil, wrapClientError(err, c, "Stat(%s)", path)
	}
	defer conn.Close()

	entry, err := stat(conn, path)
	return entry, wrapClientError(err, c, "Stat(%s)", path)
}

func (c *Device) OpenRead(path string) (io.ReadCloser, error) {
	conn, err := c.getSyncConn()
	if err != nil {
		return nil, wrapClientError(err, c, "OpenRead(%s)", path)
	}

	reader, err := receiveFile(conn, path)
	return reader, wrapClientError(err, c, "OpenRead(%s)", path)
}

// OpenWrite opens the file at path on the device, creating it with the permissions specified
// by perms if necessary, and returns a writer that writes to the file.
// The files modification time will be set to mtime when the WriterCloser is closed. The zero value
// is TimeOfClose, which will use the time the Close method is called as the modification time.
func (c *Device) OpenWrite(path string, perms os.FileMode, mtime time.Time) (io.WriteCloser, error) {
	conn, err := c.getSyncConn()
	if err != nil {
		return nil, wrapClientError(err, c, "OpenWrite(%s)", path)
	}

	writer, err := sendFile(conn, path, perms, mtime)
	return writer, wrapClientError(err, c, "OpenWrite(%s)", path)
}

// getAttribute returns the first message returned by the server by running
// <host-prefix>:<attr>, where host-prefix is determined from the DeviceDescriptor.
func (c *Device) getAttribute(attr string) (string, error) {
	resp, err := roundTripSingleResponse(c.server,
		fmt.Sprintf("%s:%s", c.descriptor.getHostPrefix(), attr))
	if err != nil {
		return "", err
	}
	return string(resp), nil
}

func (c *Device) getSyncConn() (*wire.SyncConn, error) {
	conn, err := c.dialDevice()
	if err != nil {
		return nil, err
	}

	// Switch the connection to sync mode.
	if err := wire.SendMessageString(conn, "sync:"); err != nil {
		return nil, err
	}
	if _, err := conn.ReadStatus("sync"); err != nil {
		return nil, err
	}

	return conn.NewSyncConn(), nil
}

// dialDevice switches the connection to communicate directly with the device
// by requesting the transport defined by the DeviceDescriptor.
func (c *Device) dialDevice() (*wire.Conn, error) {
	conn, err := c.server.Dial()
	if err != nil {
		return nil, err
	}

	req := fmt.Sprintf("host:%s", c.descriptor.getTransportDescriptor())
	if err = wire.SendMessageString(conn, req); err != nil {
		conn.Close()
		return nil, errors.WrapErrf(err, "error connecting to device '%s'", c.descriptor)
	}

	if _, err = conn.ReadStatus(req); err != nil {
		conn.Close()
		return nil, err
	}

	return conn, nil
}

func (c *Device) Dial() (*wire.Conn, error) {
	return c.dialDevice()
}

// prepareCommandLine validates the command and argument strings, quotes
// arguments if required, and joins them into a valid adb command string.
func prepareCommandLine(cmd string, args ...string) (string, error) {
	if isBlank(cmd) {
		return "", errors.AssertionErrorf("command cannot be empty")
	}

	for i, arg := range args {
		if strings.ContainsRune(arg, '"') {
			return "", errors.Errorf(errors.ParseError, "arg at index %d contains an invalid double quote: %s", i, arg)
		}
		if containsWhitespace(arg) {
			args[i] = fmt.Sprintf("\"%s\"", arg)
		}
	}

	// Prepend the command to the args array.
	if len(args) > 0 {
		cmd = fmt.Sprintf("%s %s", cmd, strings.Join(args, " "))
	}

	return cmd, nil
}

func (c *Device) Push(localPath, remotePath string) int64 {
	if remotePath == "" {
		return 1
	}

	var (
		localFile io.ReadCloser
		size      int
		perms     os.FileMode
		mtime     time.Time
	)
	if localPath == "" {
		localFile = os.Stdin
		// 0 size will hide the progress bar.
		perms = os.FileMode(0660)
		mtime = MtimeOfClose
	} else {
		var err error
		localFile, err = os.Open(localPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error opening local file %s: %s\n", localPath, err)
			return 1
		}
		info, err := os.Stat(localPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error reading local file %s: %s\n", localPath, err)
			return 1
		}
		size = int(info.Size())
		perms = info.Mode().Perm()
		mtime = info.ModTime()
	}
	defer localFile.Close()

	client := c
	writer, err := client.OpenWrite(remotePath, perms, mtime)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error opening remote file %s: %s\n", remotePath, err)
		return 1
	}
	defer writer.Close()

	n, err := io.Copy(writer, localFile)
	/*if err := copyWithProgressAndStats(writer, localFile, size, showProgress); err != nil {
		fmt.Fprintln(os.Stderr, "error pushing file:", err)
		return 1
	}*/
	if n == int64(size) {
		return 0
	} else {
		return n
	}
}

func (c *Device) Pull(remotePath, localPath string) (int64, error) {
	if remotePath == "" {
		fmt.Fprintln(os.Stderr, "error: must specify remote file")
		return 0, fmt.Errorf("error: must specify remote file")
	}

	if localPath == "" {
		localPath = filepath.Base(remotePath)
	}

	client := c
	// client := client.Device(device)

	info, err := client.Stat(remotePath)
	if HasErrCode(err, ErrCode(FileNoExistError)) {
		// fmt.Fprintf(os.Stderr, "file %v does not exist on device ", remotePath)
		return 0, err
	} else if err != nil {
		// fmt.Fprintf(os.Stderr, "error reading remote file %s: %s\n", remotePath, err)
		return 0, err
	}

	remoteFile, err := client.OpenRead(remotePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error opening remote file %s: %s\n", remotePath, ErrorWithCauseChain(err))
		return 0, err
	}
	defer remoteFile.Close()

	var localFile io.WriteCloser
	localFile, err = os.Create(localPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error opening local file %s: %s\n", localPath, err)
		return 0, err
	}
	defer localFile.Close()

	n, err := io.Copy(localFile, remoteFile)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error pulling file:", err)
		return 0, err
	}

	if n == int64(info.Size) {
		return int64(info.Size), nil
	} else {
		return n, fmt.Errorf("error: file size: %v, pull size: %v", info.Size, n)
	}
}
