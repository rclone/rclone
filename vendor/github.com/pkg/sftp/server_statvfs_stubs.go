// +build !darwin,!linux,!plan9

package sftp

import (
	"syscall"
)

func (p *sshFxpExtendedPacketStatVFS) respond(svr *Server) responsePacket {
	return statusFromError(p.ID, syscall.ENOTSUP)
}

func getStatVFSForPath(name string) (*StatVFS, error) {
	return nil, syscall.ENOTSUP
}
