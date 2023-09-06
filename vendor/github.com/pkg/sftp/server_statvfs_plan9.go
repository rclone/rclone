package sftp

import (
	"syscall"
)

func (p *sshFxpExtendedPacketStatVFS) respond(svr *Server) responsePacket {
	return statusFromError(p.ID, syscall.EPLAN9)
}

func getStatVFSForPath(name string) (*StatVFS, error) {
	return nil, syscall.EPLAN9
}
