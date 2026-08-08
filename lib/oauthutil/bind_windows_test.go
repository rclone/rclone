//go:build windows

package oauthutil

import (
	"net"
	"os"
	"strings"
	"syscall"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBindErrorHintWindows(t *testing.T) {
	// A WSAEACCES wrapped the way net.Listen returns it should be recognised.
	listenErr := &net.OpError{
		Op:   "listen",
		Net:  "tcp",
		Addr: &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 53682},
		Err:  os.NewSyscallError("bind", syscall.WSAEACCES),
	}
	hint := bindErrorHint(listenErr)
	assert.NotEmpty(t, hint)
	assert.Contains(t, hint, bindPort)
	assert.Contains(t, hint, "netsh")
	assert.Contains(t, hint, "start=53683")
	assert.Contains(t, hint, "num=11853")

	// The bare errno should also be recognised.
	assert.NotEmpty(t, bindErrorHint(syscall.WSAEACCES))

	// Unrelated bind errors get no hint. syscall.EACCES (13) is a different errno
	// from WSAEACCES (10013), so a look-alike permission error must not match.
	assert.Empty(t, bindErrorHint(&net.OpError{Op: "listen", Err: os.NewSyscallError("bind", syscall.EACCES)}))
	assert.Empty(t, bindErrorHint(os.ErrPermission))
	assert.Empty(t, bindErrorHint(nil))

	// Sanity check the netsh range covers up to the top of the ephemeral range.
	assert.True(t, strings.Contains(hint, "49152-65535"))
}
