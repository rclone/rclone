package gadb

// client.go is vendored from github.com/electricbubble/gadb at upstream
// commit 2e108649b dated 2025-03-14, MIT licensed.

import (
	"fmt"
	"strconv"
	"strings"
)

// adbServerPort is the default TCP port for the ADB server (host).
const adbServerPort = 5037

// Client is a connection handle to the ADB server. It is safe to copy by
// value since it stores only host and port.
type Client struct {
	host string
	port int
}

// NewClientWith connects to an ADB server at the given host and optional
// port (default 5037).
func NewClientWith(host string, port ...int) (adbClient Client, err error) {
	if len(port) == 0 {
		port = []int{adbServerPort}
	}
	adbClient.host = host
	adbClient.port = port[0]

	var tp transport
	if tp, err = adbClient.createTransport(); err != nil {
		return Client{}, err
	}
	defer func() { _ = tp.Close() }()

	return
}

// ServerVersion returns the version reported by the ADB server.
func (c Client) ServerVersion() (version int, err error) {
	var resp string
	if resp, err = c.executeCommand("host:version"); err != nil {
		return 0, err
	}

	var v int64
	if v, err = strconv.ParseInt(resp, 16, 64); err != nil {
		return 0, err
	}

	version = int(v)
	return
}

// DeviceList returns the devices known to the ADB server.
func (c Client) DeviceList() (devices []Device, err error) {
	var resp string
	if resp, err = c.executeCommand("host:devices-l"); err != nil {
		return
	}

	lines := strings.Split(resp, "\n")
	devices = make([]Device, 0, len(lines))

	for i := range lines {
		line := strings.TrimSpace(lines[i])
		if line == "" {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) < 4 || len(fields[0]) == 0 {
			debugLog(fmt.Sprintf("can't parse: %s", line))
			continue
		}

		devices = append(devices, Device{adbClient: c, serial: fields[0]})
	}

	return
}

func (c Client) createTransport() (tp transport, err error) {
	return newTransport(fmt.Sprintf("%s:%d", c.host, c.port))
}

func (c Client) executeCommand(command string, onlyVerifyResponse ...bool) (resp string, err error) {
	if len(onlyVerifyResponse) == 0 {
		onlyVerifyResponse = []bool{false}
	}

	var tp transport
	if tp, err = c.createTransport(); err != nil {
		return "", err
	}
	defer func() { _ = tp.Close() }()

	if err = tp.Send(command); err != nil {
		return "", err
	}
	if err = tp.VerifyResponse(); err != nil {
		return "", err
	}

	if onlyVerifyResponse[0] {
		return
	}

	if resp, err = tp.UnpackString(); err != nil {
		return "", err
	}
	return
}
