//go:build windows

package oauthutil

import (
	"errors"
	"fmt"
	"strconv"
	"syscall"
)

// bindErrorHint returns extra guidance to append to an error from binding the
// local OAuth webserver, or "" if there is nothing useful to add.
//
// On Windows, binding a port inside the dynamic/ephemeral range (49152-65535)
// fails with WSAEACCES ("An attempt was made to access a socket in a way
// forbidden by its access permissions") when Hyper-V, WSL2 or Docker Desktop
// has reserved that port, even though nothing is listening on it. The default
// port 53682 sits inside that range, so point the user at the fix.
func bindErrorHint(err error) string {
	if !errors.Is(err, syscall.WSAEACCES) {
		return ""
	}
	// Suggest moving the dynamic range to just above the port we need, which
	// frees the port for rclone (start = bindPort+1, up to the top of the range).
	start := 53683
	if port, atoiErr := strconv.Atoi(bindPort); atoiErr == nil {
		start = port + 1
	}
	num := 65536 - start
	return fmt.Sprintf(`This usually means port %s is inside the Windows dynamic port range (49152-65535)
that Hyper-V, WSL2 or Docker Desktop has reserved, which blocks the bind even
when nothing is listening on the port.

To free the port, move the dynamic range above it from an elevated Command
Prompt or PowerShell:

    netsh int ipv4 set dynamic tcp start=%d num=%d

You can check the reserved ranges with:

    netsh interface ipv4 show excludedportrange protocol=tcp`, bindPort, start, num)
}
