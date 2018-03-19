// +build !darwin,!linux

package sftp

import (
	"syscall"
)

func (p sshFxpExtendedPacketStatVFS) respond(svr *Server) error {
	return syscall.ENOTSUP
}
