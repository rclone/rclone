package gadb

// device.go is vendored from github.com/electricbubble/gadb at upstream
// commit 2e108649b dated 2025-03-14, MIT licensed.

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"
)

// DeviceFileInfo describes one entry returned by an ADB SYNC LIST call.
// Size is int64 so the field can carry both wire-protocol uint32 sizes and
// shell-stat uint64 sizes without truncation; callers that need to know
// whether the value originated from the SYNC wire (capped at 4 GiB-1) or a
// shell stat (no cap) should track that out-of-band.
type DeviceFileInfo struct {
	Name         string
	Mode         os.FileMode
	Size         int64
	LastModified time.Time
}

// defaultFileMode is the default mode used for new files pushed via Push.
const defaultFileMode = os.FileMode(0664)

// Device represents a single ADB-connected device.
type Device struct {
	adbClient Client
	serial    string
}

// Serial returns the device serial number.
func (d Device) Serial() string {
	return d.serial
}

// RunShellCommand runs cmd with optional args via the ADB shell: service.
// Output is returned as a string.
func (d Device) RunShellCommand(cmd string, args ...string) (string, error) {
	raw, err := d.runShellCommandWithBytes(cmd, args...)
	return string(raw), err
}

// runShellCommandWithBytes runs cmd with optional args via the ADB shell:
// service. Output is returned as raw bytes.
func (d Device) runShellCommandWithBytes(cmd string, args ...string) ([]byte, error) {
	if len(args) > 0 {
		cmd = fmt.Sprintf("%s %s", cmd, strings.Join(args, " "))
	}
	if strings.TrimSpace(cmd) == "" {
		return nil, errors.New("adb shell: command cannot be empty")
	}
	raw, err := d.executeCommand(fmt.Sprintf("shell:%s", cmd))
	return raw, err
}

func (d Device) createDeviceTransport() (tp transport, err error) {
	if tp, err = newTransport(fmt.Sprintf("%s:%d", d.adbClient.host, d.adbClient.port)); err != nil {
		return transport{}, err
	}

	if err = tp.Send(fmt.Sprintf("host:transport:%s", d.serial)); err != nil {
		return transport{}, err
	}
	err = tp.VerifyResponse()
	return
}

func (d Device) executeCommand(command string, onlyVerifyResponse ...bool) (raw []byte, err error) {
	if len(onlyVerifyResponse) == 0 {
		onlyVerifyResponse = []bool{false}
	}

	var tp transport
	if tp, err = d.createDeviceTransport(); err != nil {
		return nil, err
	}
	defer func() { _ = tp.Close() }()

	if err = tp.Send(command); err != nil {
		return nil, err
	}

	if err = tp.VerifyResponse(); err != nil {
		return nil, err
	}

	if onlyVerifyResponse[0] {
		return
	}

	raw, err = tp.ReadBytesAll()
	return
}

// List returns the directory listing at remotePath via the SYNC LIST command.
func (d Device) List(remotePath string) (devFileInfos []DeviceFileInfo, err error) {
	var tp transport
	if tp, err = d.createDeviceTransport(); err != nil {
		return nil, err
	}
	defer func() { _ = tp.Close() }()

	var sync syncTransport
	if sync, err = tp.CreateSyncTransport(); err != nil {
		return nil, err
	}
	defer func() { _ = sync.Close() }()

	if err = sync.Send("LIST", remotePath); err != nil {
		return nil, err
	}

	devFileInfos = make([]DeviceFileInfo, 0)

	var entry DeviceFileInfo
	for entry, err = sync.ReadDirectoryEntry(); err == nil; entry, err = sync.ReadDirectoryEntry() {
		if entry == (DeviceFileInfo{}) {
			break
		}
		devFileInfos = append(devFileInfos, entry)
	}

	return
}

// Push uploads source to remotePath with the given modification time and
// optional file mode (default 0664).
func (d Device) Push(source io.Reader, remotePath string, modification time.Time, mode ...os.FileMode) (err error) {
	if len(mode) == 0 {
		mode = []os.FileMode{defaultFileMode}
	}

	var tp transport
	if tp, err = d.createDeviceTransport(); err != nil {
		return err
	}
	defer func() { _ = tp.Close() }()

	var sync syncTransport
	if sync, err = tp.CreateSyncTransport(); err != nil {
		return err
	}
	defer func() { _ = sync.Close() }()

	data := fmt.Sprintf("%s,%d", remotePath, mode[0])
	if err = sync.Send("SEND", data); err != nil {
		return err
	}

	if err = sync.SendStream(source); err != nil {
		return
	}

	if err = sync.SendStatus("DONE", uint32(modification.Unix())); err != nil {
		return
	}

	if err = sync.VerifyStatus(); err != nil {
		return
	}
	return
}

// Pull downloads the file at remotePath into dest.
func (d Device) Pull(remotePath string, dest io.Writer) (err error) {
	var tp transport
	if tp, err = d.createDeviceTransport(); err != nil {
		return err
	}
	defer func() { _ = tp.Close() }()

	var sync syncTransport
	if sync, err = tp.CreateSyncTransport(); err != nil {
		return err
	}
	defer func() { _ = sync.Close() }()

	if err = sync.Send("RECV", remotePath); err != nil {
		return err
	}

	err = sync.WriteStream(dest)
	return
}
