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
func (p sshFxpOpendirPacket) getPath() string  { return p.Path }
func (p sshFxpOpenPacket) getPath() string     { return p.Path }

func (p sshFxpExtendedPacketPosixRename) getPath() string { return p.Oldpath }
func (p sshFxpExtendedPacketHardlink) getPath() string    { return p.Oldpath }

// getHandle
func (p sshFxpFstatPacket) getHandle() string    { return p.Handle }
func (p sshFxpFsetstatPacket) getHandle() string { return p.Handle }
func (p sshFxpReadPacket) getHandle() string     { return p.Handle }
func (p sshFxpWritePacket) getHandle() string    { return p.Handle }
func (p sshFxpReaddirPacket) getHandle() string  { return p.Handle }
func (p sshFxpClosePacket) getHandle() string    { return p.Handle }

// notReadOnly
func (p sshFxpWritePacket) notReadOnly()               {}
func (p sshFxpSetstatPacket) notReadOnly()             {}
func (p sshFxpFsetstatPacket) notReadOnly()            {}
func (p sshFxpRemovePacket) notReadOnly()              {}
func (p sshFxpMkdirPacket) notReadOnly()               {}
func (p sshFxpRmdirPacket) notReadOnly()               {}
func (p sshFxpRenamePacket) notReadOnly()              {}
func (p sshFxpSymlinkPacket) notReadOnly()             {}
func (p sshFxpExtendedPacketPosixRename) notReadOnly() {}
func (p sshFxpExtendedPacketHardlink) notReadOnly()    {}

// some packets with ID are missing id()
func (p sshFxpDataPacket) id() uint32   { return p.ID }
func (p sshFxpStatusPacket) id() uint32 { return p.ID }
func (p sshFxpStatResponse) id() uint32 { return p.ID }
func (p sshFxpNamePacket) id() uint32   { return p.ID }
func (p sshFxpHandlePacket) id() uint32 { return p.ID }
func (p StatVFS) id() uint32            { return p.ID }
func (p sshFxVersionPacket) id() uint32 { return 0 }

// take raw incoming packet data and build packet objects
func makePacket(p rxPacket) (requestPacket, error) {
	var pkt requestPacket
	switch p.pktType {
	case sshFxpInit:
		pkt = &sshFxInitPacket{}
	case sshFxpLstat:
		pkt = &sshFxpLstatPacket{}
	case sshFxpOpen:
		pkt = &sshFxpOpenPacket{}
	case sshFxpClose:
		pkt = &sshFxpClosePacket{}
	case sshFxpRead:
		pkt = &sshFxpReadPacket{}
	case sshFxpWrite:
		pkt = &sshFxpWritePacket{}
	case sshFxpFstat:
		pkt = &sshFxpFstatPacket{}
	case sshFxpSetstat:
		pkt = &sshFxpSetstatPacket{}
	case sshFxpFsetstat:
		pkt = &sshFxpFsetstatPacket{}
	case sshFxpOpendir:
		pkt = &sshFxpOpendirPacket{}
	case sshFxpReaddir:
		pkt = &sshFxpReaddirPacket{}
	case sshFxpRemove:
		pkt = &sshFxpRemovePacket{}
	case sshFxpMkdir:
		pkt = &sshFxpMkdirPacket{}
	case sshFxpRmdir:
		pkt = &sshFxpRmdirPacket{}
	case sshFxpRealpath:
		pkt = &sshFxpRealpathPacket{}
	case sshFxpStat:
		pkt = &sshFxpStatPacket{}
	case sshFxpRename:
		pkt = &sshFxpRenamePacket{}
	case sshFxpReadlink:
		pkt = &sshFxpReadlinkPacket{}
	case sshFxpSymlink:
		pkt = &sshFxpSymlinkPacket{}
	case sshFxpExtended:
		pkt = &sshFxpExtendedPacket{}
	default:
		return nil, errors.Errorf("unhandled packet type: %s", p.pktType)
	}
	if err := pkt.UnmarshalBinary(p.pktBytes); err != nil {
		// Return partially unpacked packet to allow callers to return
		// error messages appropriately with necessary id() method.
		return pkt, err
	}
	return pkt, nil
}
