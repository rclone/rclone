// +build darwin linux

// fill in statvfs structure with OS specific values
// Statfs_t is different per-kernel, and only exists on some unixes (not Solaris for instance)

package sftp

import (
	"syscall"
)

func (p *sshFxpExtendedPacketStatVFS) respond(svr *Server) responsePacket {
	retPkt, err := getStatVFSForPath(p.Path)
	if err != nil {
		return statusFromError(p.ID, err)
	}
	retPkt.ID = p.ID

	return retPkt
}

func getStatVFSForPath(name string) (*StatVFS, error) {
	var stat syscall.Statfs_t
	if err := syscall.Statfs(name, &stat); err != nil {
		return nil, err
	}

	return statvfsFromStatfst(&stat)
}
