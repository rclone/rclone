package nfs

import (
	"github.com/willscott/go-nfs-client/nfs/rpc"
)

// FHSize is the maximum size of a FileHandle
const FHSize = 64

// MNTNameLen is the maximum size of a mount name
const MNTNameLen = 255

// MntPathLen is the maximum size of a mount path
const MntPathLen = 1024

// FileHandle maps to a fhandle3
type FileHandle []byte

// MountStatus defines the response to the Mount Procedure
type MountStatus uint32

// MountStatus Codes
const (
	MountStatusOk             MountStatus = 0
	MountStatusErrPerm        MountStatus = 1
	MountStatusErrNoEnt       MountStatus = 2
	MountStatusErrIO          MountStatus = 5
	MountStatusErrAcces       MountStatus = 13
	MountStatusErrNotDir      MountStatus = 20
	MountStatusErrInval       MountStatus = 22
	MountStatusErrNameTooLong MountStatus = 63
	MountStatusErrNotSupp     MountStatus = 10004
	MountStatusErrServerFault MountStatus = 10006
)

// MountProcedure is the valid RPC calls for the mount service.
type MountProcedure uint32

// MountProcedure Codes
const (
	MountProcNull MountProcedure = iota
	MountProcMount
	MountProcDump
	MountProcUmnt
	MountProcUmntAll
	MountProcExport
)

func (m MountProcedure) String() string {
	switch m {
	case MountProcNull:
		return "Null"
	case MountProcMount:
		return "Mount"
	case MountProcDump:
		return "Dump"
	case MountProcUmnt:
		return "Umnt"
	case MountProcUmntAll:
		return "UmntAll"
	case MountProcExport:
		return "Export"
	default:
		return "Unknown"
	}
}

// AuthFlavor is a form of authentication, per rfc1057 section 7.2
type AuthFlavor uint32

// AuthFlavor Codes
const (
	AuthFlavorNull  AuthFlavor = 0
	AuthFlavorUnix  AuthFlavor = 1
	AuthFlavorShort AuthFlavor = 2
	AuthFlavorDES   AuthFlavor = 3
)

// MountRequest contains the format of a client request to open a mount.
type MountRequest struct {
	rpc.Header
	Dirpath []byte
}

// MountResponse is the server's response with status `MountStatusOk`
type MountResponse struct {
	rpc.Header
	FileHandle
	AuthFlavors []int
}
