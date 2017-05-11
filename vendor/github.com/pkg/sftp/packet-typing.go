package sftp

import (
	"encoding"

	"github.com/pkg/errors"
)

// all incoming packets
type requestPacket interface {
	encoding.BinaryUnmarshaler
	id() uint32
}

type requestChan chan requestPacket

type responsePacket interface {
	encoding.BinaryMarshaler
	id() uint32
}

// interfaces to group types
type hasPath interface {
	requestPacket
	getPath() string
}

type hasHandle interface {
	requestPacket
	getHandle() string
}

type isOpener interface {
	hasPath
	isOpener()
}

type notReadOnly interface {
	notReadOnly()
}

//// define types by adding methods
// hasPath
func (p sshFxpLstatPacket) getPath() string    { return p.Path }
func (p sshFxpStatPacket) getPath() string     { return p.Path }
func (p sshFxpRmdirPacket) getPath() string    { return p.Path }
func (p sshFxpReadlinkPacket) getPath() string { return p.Path }
func (p sshFxpRealpathPacket) getPath() string { return p.Path }
func (p sshFxpMkdirPacket) getPath() string    { return p.Path }
func (p sshFxpSetstatPacket) getPath() string  { return p.Path }
func (p sshFxpStatvfsPacket) getPath() string  { return p.Path }
func (p sshFxpRemovePacket) getPath() string   { return p.Filename }
func (p sshFxpRenamePacket) getPath() string   { return p.Oldpath }
func (p sshFxpSymlinkPacket) getPath() string  { return p.Targetpath }

// Openers implement hasPath and isOpener
func (p sshFxpOpendirPacket) getPath() string { return p.Path }
func (p sshFxpOpendirPacket) isOpener()       {}
func (p sshFxpOpenPacket) getPath() string    { return p.Path }
func (p sshFxpOpenPacket) isOpener()          {}

// hasHandle
func (p sshFxpFstatPacket) getHandle() string    { return p.Handle }
func (p sshFxpFsetstatPacket) getHandle() string { return p.Handle }
func (p sshFxpReadPacket) getHandle() string     { return p.Handle }
func (p sshFxpWritePacket) getHandle() string    { return p.Handle }
func (p sshFxpReaddirPacket) getHandle() string  { return p.Handle }

// notReadOnly
func (p sshFxpWritePacket) notReadOnly()    {}
func (p sshFxpSetstatPacket) notReadOnly()  {}
func (p sshFxpFsetstatPacket) notReadOnly() {}
func (p sshFxpRemovePacket) notReadOnly()   {}
func (p sshFxpMkdirPacket) notReadOnly()    {}
func (p sshFxpRmdirPacket) notReadOnly()    {}
func (p sshFxpRenamePacket) notReadOnly()   {}
func (p sshFxpSymlinkPacket) notReadOnly()  {}

// this has a handle, but is only used for close
func (p sshFxpClosePacket) getHandle() string { return p.Handle }

// some packets with ID are missing id()
func (p sshFxpDataPacket) id() uint32   { return p.ID }
func (p sshFxpStatusPacket) id() uint32 { return p.ID }
func (p sshFxpStatResponse) id() uint32 { return p.ID }
func (p sshFxpNamePacket) id() uint32   { return p.ID }
func (p sshFxpHandlePacket) id() uint32 { return p.ID }
func (p sshFxVersionPacket) id() uint32 { return 0 }

// take raw incoming packet data and build packet objects
func makePacket(p rxPacket) (requestPacket, error) {
	var pkt requestPacket
	switch p.pktType {
	case ssh_FXP_INIT:
		pkt = &sshFxInitPacket{}
	case ssh_FXP_LSTAT:
		pkt = &sshFxpLstatPacket{}
	case ssh_FXP_OPEN:
		pkt = &sshFxpOpenPacket{}
	case ssh_FXP_CLOSE:
		pkt = &sshFxpClosePacket{}
	case ssh_FXP_READ:
		pkt = &sshFxpReadPacket{}
	case ssh_FXP_WRITE:
		pkt = &sshFxpWritePacket{}
	case ssh_FXP_FSTAT:
		pkt = &sshFxpFstatPacket{}
	case ssh_FXP_SETSTAT:
		pkt = &sshFxpSetstatPacket{}
	case ssh_FXP_FSETSTAT:
		pkt = &sshFxpFsetstatPacket{}
	case ssh_FXP_OPENDIR:
		pkt = &sshFxpOpendirPacket{}
	case ssh_FXP_READDIR:
		pkt = &sshFxpReaddirPacket{}
	case ssh_FXP_REMOVE:
		pkt = &sshFxpRemovePacket{}
	case ssh_FXP_MKDIR:
		pkt = &sshFxpMkdirPacket{}
	case ssh_FXP_RMDIR:
		pkt = &sshFxpRmdirPacket{}
	case ssh_FXP_REALPATH:
		pkt = &sshFxpRealpathPacket{}
	case ssh_FXP_STAT:
		pkt = &sshFxpStatPacket{}
	case ssh_FXP_RENAME:
		pkt = &sshFxpRenamePacket{}
	case ssh_FXP_READLINK:
		pkt = &sshFxpReadlinkPacket{}
	case ssh_FXP_SYMLINK:
		pkt = &sshFxpSymlinkPacket{}
	case ssh_FXP_EXTENDED:
		pkt = &sshFxpExtendedPacket{}
	default:
		return nil, errors.Errorf("unhandled packet type: %s", p.pktType)
	}
	if err := pkt.UnmarshalBinary(p.pktBytes); err != nil {
		return nil, err
	}
	return pkt, nil
}
