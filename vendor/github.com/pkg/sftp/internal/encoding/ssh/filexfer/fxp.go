package filexfer

import (
	"fmt"
)

// PacketType defines the various SFTP packet types.
type PacketType uint8

// Request packet types.
const (
	// https://tools.ietf.org/html/draft-ietf-secsh-filexfer-02#section-3
	PacketTypeInit = PacketType(iota + 1)
	PacketTypeVersion
	PacketTypeOpen
	PacketTypeClose
	PacketTypeRead
	PacketTypeWrite
	PacketTypeLStat
	PacketTypeFStat
	PacketTypeSetstat
	PacketTypeFSetstat
	PacketTypeOpenDir
	PacketTypeReadDir
	PacketTypeRemove
	PacketTypeMkdir
	PacketTypeRmdir
	PacketTypeRealPath
	PacketTypeStat
	PacketTypeRename
	PacketTypeReadLink
	PacketTypeSymlink

	// https://tools.ietf.org/html/draft-ietf-secsh-filexfer-07#section-3.3
	PacketTypeV6Link

	// https://tools.ietf.org/html/draft-ietf-secsh-filexfer-08#section-3.3
	PacketTypeV6Block
	PacketTypeV6Unblock
)

// Response packet types.
const (
	// https://tools.ietf.org/html/draft-ietf-secsh-filexfer-02#section-3
	PacketTypeStatus = PacketType(iota + 101)
	PacketTypeHandle
	PacketTypeData
	PacketTypeName
	PacketTypeAttrs
)

// Extended packet types.
const (
	// https://tools.ietf.org/html/draft-ietf-secsh-filexfer-02#section-3
	PacketTypeExtended = PacketType(iota + 200)
	PacketTypeExtendedReply
)

func (f PacketType) String() string {
	switch f {
	case PacketTypeInit:
		return "SSH_FXP_INIT"
	case PacketTypeVersion:
		return "SSH_FXP_VERSION"
	case PacketTypeOpen:
		return "SSH_FXP_OPEN"
	case PacketTypeClose:
		return "SSH_FXP_CLOSE"
	case PacketTypeRead:
		return "SSH_FXP_READ"
	case PacketTypeWrite:
		return "SSH_FXP_WRITE"
	case PacketTypeLStat:
		return "SSH_FXP_LSTAT"
	case PacketTypeFStat:
		return "SSH_FXP_FSTAT"
	case PacketTypeSetstat:
		return "SSH_FXP_SETSTAT"
	case PacketTypeFSetstat:
		return "SSH_FXP_FSETSTAT"
	case PacketTypeOpenDir:
		return "SSH_FXP_OPENDIR"
	case PacketTypeReadDir:
		return "SSH_FXP_READDIR"
	case PacketTypeRemove:
		return "SSH_FXP_REMOVE"
	case PacketTypeMkdir:
		return "SSH_FXP_MKDIR"
	case PacketTypeRmdir:
		return "SSH_FXP_RMDIR"
	case PacketTypeRealPath:
		return "SSH_FXP_REALPATH"
	case PacketTypeStat:
		return "SSH_FXP_STAT"
	case PacketTypeRename:
		return "SSH_FXP_RENAME"
	case PacketTypeReadLink:
		return "SSH_FXP_READLINK"
	case PacketTypeSymlink:
		return "SSH_FXP_SYMLINK"
	case PacketTypeV6Link:
		return "SSH_FXP_LINK"
	case PacketTypeV6Block:
		return "SSH_FXP_BLOCK"
	case PacketTypeV6Unblock:
		return "SSH_FXP_UNBLOCK"
	case PacketTypeStatus:
		return "SSH_FXP_STATUS"
	case PacketTypeHandle:
		return "SSH_FXP_HANDLE"
	case PacketTypeData:
		return "SSH_FXP_DATA"
	case PacketTypeName:
		return "SSH_FXP_NAME"
	case PacketTypeAttrs:
		return "SSH_FXP_ATTRS"
	case PacketTypeExtended:
		return "SSH_FXP_EXTENDED"
	case PacketTypeExtendedReply:
		return "SSH_FXP_EXTENDED_REPLY"
	default:
		return fmt.Sprintf("SSH_FXP_UNKNOWN(%d)", f)
	}
}
