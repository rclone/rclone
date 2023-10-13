package nfs

// NFSProcedure is the valid RPC calls for the nfs service.
type NFSProcedure uint32

// NfsProcedure Codes
const (
	NFSProcedureNull NFSProcedure = iota
	NFSProcedureGetAttr
	NFSProcedureSetAttr
	NFSProcedureLookup
	NFSProcedureAccess
	NFSProcedureReadlink
	NFSProcedureRead
	NFSProcedureWrite
	NFSProcedureCreate
	NFSProcedureMkDir
	NFSProcedureSymlink
	NFSProcedureMkNod
	NFSProcedureRemove
	NFSProcedureRmDir
	NFSProcedureRename
	NFSProcedureLink
	NFSProcedureReadDir
	NFSProcedureReadDirPlus
	NFSProcedureFSStat
	NFSProcedureFSInfo
	NFSProcedurePathConf
	NFSProcedureCommit
)

func (n NFSProcedure) String() string {
	switch n {
	case NFSProcedureNull:
		return "Null"
	case NFSProcedureGetAttr:
		return "GetAttr"
	case NFSProcedureSetAttr:
		return "SetAttr"
	case NFSProcedureLookup:
		return "Lookup"
	case NFSProcedureAccess:
		return "Access"
	case NFSProcedureReadlink:
		return "ReadLink"
	case NFSProcedureRead:
		return "Read"
	case NFSProcedureWrite:
		return "Write"
	case NFSProcedureCreate:
		return "Create"
	case NFSProcedureMkDir:
		return "Mkdir"
	case NFSProcedureSymlink:
		return "Symlink"
	case NFSProcedureMkNod:
		return "Mknod"
	case NFSProcedureRemove:
		return "Remove"
	case NFSProcedureRmDir:
		return "Rmdir"
	case NFSProcedureRename:
		return "Rename"
	case NFSProcedureLink:
		return "Link"
	case NFSProcedureReadDir:
		return "ReadDir"
	case NFSProcedureReadDirPlus:
		return "ReadDirPlus"
	case NFSProcedureFSStat:
		return "FSStat"
	case NFSProcedureFSInfo:
		return "FSInfo"
	case NFSProcedurePathConf:
		return "PathConf"
	case NFSProcedureCommit:
		return "Commit"
	default:
		return "Unknown"
	}
}

// NFSStatus (nfsstat3) is a result code for nfs rpc calls
type NFSStatus uint32

// NFSStatus codes
const (
	NFSStatusOk          NFSStatus = 0
	NFSStatusPerm        NFSStatus = 1
	NFSStatusNoEnt       NFSStatus = 2
	NFSStatusIO          NFSStatus = 5
	NFSStatusNXIO        NFSStatus = 6
	NFSStatusAccess      NFSStatus = 13
	NFSStatusExist       NFSStatus = 17
	NFSStatusXDev        NFSStatus = 18
	NFSStatusNoDev       NFSStatus = 19
	NFSStatusNotDir      NFSStatus = 20
	NFSStatusIsDir       NFSStatus = 21
	NFSStatusInval       NFSStatus = 22
	NFSStatusFBig        NFSStatus = 27
	NFSStatusNoSPC       NFSStatus = 28
	NFSStatusROFS        NFSStatus = 30
	NFSStatusMlink       NFSStatus = 31
	NFSStatusNameTooLong NFSStatus = 63
	NFSStatusNotEmpty    NFSStatus = 66
	NFSStatusDQuot       NFSStatus = 69
	NFSStatusStale       NFSStatus = 70
	NFSStatusRemote      NFSStatus = 71
	NFSStatusBadHandle   NFSStatus = 10001
	NFSStatusNotSync     NFSStatus = 10002
	NFSStatusBadCookie   NFSStatus = 10003
	NFSStatusNotSupp     NFSStatus = 10004
	NFSStatusTooSmall    NFSStatus = 10005
	NFSStatusServerFault NFSStatus = 10006
	NFSStatusBadType     NFSStatus = 10007
	NFSStatusJukebox     NFSStatus = 10008
)

func (s NFSStatus) String() string {
	switch s {
	case NFSStatusOk:
		return "Call Completed Successfull"
	case NFSStatusPerm:
		return "Not Owner"
	case NFSStatusNoEnt:
		return "No such file or directory"
	case NFSStatusIO:
		return "I/O error"
	case NFSStatusNXIO:
		return "I/O error: No such device"
	case NFSStatusAccess:
		return "Permission denied"
	case NFSStatusExist:
		return "File exists"
	case NFSStatusXDev:
		return "Attempt to do a cross device hard link"
	case NFSStatusNoDev:
		return "No such device"
	case NFSStatusNotDir:
		return "Not a directory"
	case NFSStatusIsDir:
		return "Is a directory"
	case NFSStatusInval:
		return "Invalid argument"
	case NFSStatusFBig:
		return "File too large"
	case NFSStatusNoSPC:
		return "No space left on device"
	case NFSStatusROFS:
		return "Read only file system"
	case NFSStatusMlink:
		return "Too many hard links"
	case NFSStatusNameTooLong:
		return "Name too long"
	case NFSStatusNotEmpty:
		return "Not empty"
	case NFSStatusDQuot:
		return "Resource quota exceeded"
	case NFSStatusStale:
		return "Invalid file handle"
	case NFSStatusRemote:
		return "Too many levels of remote in path"
	case NFSStatusBadHandle:
		return "Illegal NFS file handle"
	case NFSStatusNotSync:
		return "Synchronization mismatch"
	case NFSStatusBadCookie:
		return "Cookie is Stale"
	case NFSStatusNotSupp:
		return "Operation not supported"
	case NFSStatusTooSmall:
		return "Buffer or request too small"
	case NFSStatusServerFault:
		return "Unmapped error (EIO)"
	case NFSStatusBadType:
		return "Type not supported"
	case NFSStatusJukebox:
		return "Initiated, but too slow. Try again with new txn"
	default:
		return "unknown"
	}
}

// DirOpArg is a common serialization used for referencing an object in a directory
type DirOpArg struct {
	Handle   []byte
	Filename []byte
}
