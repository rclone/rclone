// ref: MS-SMB2

package smb2

const (
	MAGIC  = "\xfeSMB"
	MAGIC2 = "\xfdSMB"
)

// ----------------------------------------------------------------------------
// SMB2 Packet Header
//

// Command
const (
	SMB2_NEGOTIATE = iota
	SMB2_SESSION_SETUP
	SMB2_LOGOFF
	SMB2_TREE_CONNECT
	SMB2_TREE_DISCONNECT
	SMB2_CREATE
	SMB2_CLOSE
	SMB2_FLUSH
	SMB2_READ
	SMB2_WRITE
	SMB2_LOCK
	SMB2_IOCTL
	SMB2_CANCEL
	SMB2_ECHO
	SMB2_QUERY_DIRECTORY
	SMB2_CHANGE_NOTIFY
	SMB2_QUERY_INFO
	SMB2_SET_INFO
	SMB2_OPLOCK_BREAK
)

// Flags
const (
	SMB2_FLAGS_SERVER_TO_REDIR = 1 << iota
	SMB2_FLAGS_ASYNC_COMMAND
	SMB2_FLAGS_RELATED_OPERATIONS
	SMB2_FLAGS_SIGNED

	SMB2_FLAGS_PRIORITY_MASK     = 0x70
	SMB2_FLAGS_DFS_OPERATIONS    = 0x10000000
	SMB2_FLAGS_REPLAY_OPERATIONS = 0x20000000
)

// ----------------------------------------------------------------------------
// SMB2 TRANSFORM_HEADER
//

// From SMB3

// EncryptionAlgorithm
const (
	SMB2_ENCRYPTION_AES128_CCM = 1 << iota
)

// From SMB311

// Flags
const (
	Encrypted = 1 << iota
)

// ----------------------------------------------------------------------------
// SMB2 Error Response
//

// ErrorId
const (
	SMB2_ERROR_ID_DEFAULT = 0x0
)

// Flags
const (
	SYMLINK_FLAG_RELATIVE = 0x1
)

// ----------------------------------------------------------------------------
// SMB2 NEGOTIATE Request and Response
//

// SecurityMode
const (
	SMB2_NEGOTIATE_SIGNING_ENABLED = 1 << iota
	SMB2_NEGOTIATE_SIGNING_REQUIRED
)

// Capabilities
const (
	SMB2_GLOBAL_CAP_DFS = 1 << iota
	SMB2_GLOBAL_CAP_LEASING
	SMB2_GLOBAL_CAP_LARGE_MTU
	SMB2_GLOBAL_CAP_MULTI_CHANNEL
	SMB2_GLOBAL_CAP_PERSISTENT_HANDLES
	SMB2_GLOBAL_CAP_DIRECTORY_LEASING
	SMB2_GLOBAL_CAP_ENCRYPTION
)

// Dialects
const (
	UnknownSMB = 0x0
	SMB2       = 0x2FF
	SMB202     = 0x202
	SMB210     = 0x210
	SMB300     = 0x300
	SMB302     = 0x302
	SMB311     = 0x311
)

//

// SecurityMode
const (
// SMB2_NEGOTIATE_SIGNING_ENABLED = 1 << iota
// SMB2_NEGOTIATE_SIGNING_REQUIRED
)

// DialectRevision
const (
// SMB2   = 0x2FF
// SMB202 = 0x202
// SMB210 = 0x210
// SMB300 = 0x300
// SMB302 = 0x302
// SMB311 = 0x311
)

// Capabilities
const (
// SMB2_GLOBAL_CAP_DFS = 1 << iota
// SMB2_GLOBAL_CAP_LEASING
// SMB2_GLOBAL_CAP_LARGE_MTU
// SMB2_GLOBAL_CAP_MULTI_CHANNEL
// SMB2_GLOBAL_CAP_PERSISTENT_HANDLES
// SMB2_GLOBAL_CAP_DIRECTORY_LEASING
// SMB2_GLOBAL_CAP_ENCRYPTION
)

// ----------------------------------------------------------------------------
// SMB2 NEGOTIATE Contexts
//

// From SMB311

// ContextType
const (
	SMB2_PREAUTH_INTEGRITY_CAPABILITIES = 1 << iota
	SMB2_ENCRYPTION_CAPABILITIES
)

// HashAlgorithms
const (
	SHA512 = 0x1
)

// Ciphers
const (
	AES128CCM = 1 << iota
	AES128GCM
)

// ----------------------------------------------------------------------------
// SMB2 SESSION_SETUP Request and Response
//

// Flags
const (
	SMB2_SESSION_FLAG_BINDING = 0x1
)

// SecurityMode
const (
// SMB2_NEGOTIATE_SIGNING_ENABLED = 1 << iota
// SMB2_NEGOTIATE_SIGNING_REQUIRED
)

// Capabilities
const (
// SMB2_GLOBAL_CAP_DFS = 1 << iota
// SMB2_GLOBAL_CAP_UNUSED1
// SMB2_GLOBAL_CAP_UNUSED2
// SMB2_GLOBAL_CAP_UNUSED3
)

//

// SessionFlags
const (
	SMB2_SESSION_FLAG_IS_GUEST = 1 << iota
	SMB2_SESSION_FLAG_IS_NULL
	SMB2_SESSION_FLAG_ENCRYPT_DATA
)

// ----------------------------------------------------------------------------
// SMB2 LOGOFF Request and Response
//

//

// ----------------------------------------------------------------------------
// SMB2 TREE_CONNECT Request and Response
//

// From SMB311

// Flags
const (
	SMB2_TREE_CONNECT_FLAG_CLUSTER_RECONNECT = 0x1
)

//

// ShareType
const (
	SMB2_SHARE_TYPE_DISK = 1 + iota
	SMB2_SHARE_TYPE_PIPE
	SMB2_SHARE_TYPE_PRINT
)

// ShareFlags
const (
	SMB2_SHAREFLAG_MANUAL_CACHING              = 0x0
	SMB2_SHAREFLAG_AUTO_CACHING                = 0x10
	SMB2_SHAREFLAG_VDO_CACHING                 = 0x20
	SMB2_SHAREFLAG_NO_CACHING                  = 0x30
	SMB2_SHAREFLAG_DFS                         = 0x1
	SMB2_SHAREFLAG_DFS_ROOT                    = 0x2
	SMB2_SHAREFLAG_RESTRICT_EXCLUSIVE_OPENS    = 0x100
	SMB2_SHAREFLAG_FORCE_SHARED_DELETE         = 0x200
	SMB2_SHAREFLAG_ALLOW_NAMESPACE_CACHING     = 0x400
	SMB2_SHAREFLAG_ACCESS_BASED_DIRECTORY_ENUM = 0x800
	SMB2_SHAREFLAG_FORCE_LEVELII_OPLOCK        = 0x1000
	SMB2_SHAREFLAG_ENABLE_HASH_V1              = 0x2000
	SMB2_SHAREFLAG_ENABLE_HASH_V2              = 0x4000
	SMB2_SHAREFLAG_ENCRYPT_DATA                = 0x8000
)

// Capabilities
const (
	SMB2_SHARE_CAP_DFS = 0x8 << iota
	SMB2_SHARE_CAP_CONTINUOUS_AVAILABILITY
	SMB2_SHARE_CAP_SCALEOUT
	SMB2_SHARE_CAP_CLUSTER
	SMB2_SHARE_CAP_ASYMMETRIC
)

// ----------------------------------------------------------------------------
// SMB2 TREE_DISCONNECT Request and Response
//

//

// ----------------------------------------------------------------------------
// SMB2 CREATE Request and Response
//

// RequestedOplockLevel
const (
	SMB2_OPLOCK_LEVEL_NONE      = 0x0
	SMB2_OPLOCK_LEVEL_II        = 0x1
	SMB2_OPLOCK_LEVEL_EXCLUSIVE = 0x8
	SMB2_OPLOCK_LEVEL_BATCH     = 0x9
	SMB2_OPLOCK_LEVEL_LEASE     = 0xff
)

// ImpersonationLevel
const (
	Anonymous = iota
	Identification
	Impersonation
	Delegate
)

// DesiredAccess
const (
	// for file, pipe, printer
	FILE_READ_DATA = 1 << iota
	FILE_WRITE_DATA
	FILE_APPEND_DATA
	FILE_READ_EA
	FILE_WRITE_EA
	FILE_EXECUTE
	FILE_DELETE_CHILD
	FILE_READ_ATTRIBUTES
	FILE_WRITE_ATTRIBUTES

	// for directory
	FILE_LIST_DIRECTORY = 1 << iota
	FILE_ADD_FILE
	FILE_ADD_SUBDIRECTORY
	_ // FILE_READ_EA
	_ // FILE_WRITE_EA
	FILE_TRAVERSE
	_ // FILE_DELETE_CHILD
	_ // FILE_READ_ATTRIBUTES
	_ // FILE_WRITE_ATTRIBUTES

	// common
	DELETE                 = 0x10000
	READ_CONTROL           = 0x20000
	WRITE_DAC              = 0x40000
	WRITE_OWNER            = 0x80000
	SYNCHRONIZE            = 0x100000
	ACCESS_SYSTEM_SECURITY = 0x1000000
	MAXIMUM_ALLOWED        = 0x2000000
	GENERIC_ALL            = 0x10000000
	GENERIC_EXECUTE        = 0x20000000
	GENERIC_WRITE          = 0x40000000
	GENERIC_READ           = 0x80000000
)

// FileAttributes (from MS-FSCC)
const (
// FILE_ATTRIBUTE_ARCHIVE             = 0x20
// FILE_ATTRIBUTE_COMPRESSED          = 0x800
// FILE_ATTRIBUTE_DIRECTORY           = 0x10
// FILE_ATTRIBUTE_ENCRYPTED           = 0x4000
// FILE_ATTRIBUTE_HIDDEN              = 0x2
// FILE_ATTRIBUTE_NORMAL              = 0x80
// FILE_ATTRIBUTE_NOT_CONTENT_INDEXED = 0x2000
// FILE_ATTRIBUTE_OFFLINE             = 0x1000
// FILE_ATTRIBUTE_READONLY            = 0x1
// FILE_ATTRIBUTE_REPARSE_POINT       = 0x400
// FILE_ATTRIBUTE_SPARSE_FILE         = 0x200
// FILE_ATTRIBUTE_SYSTEM              = 0x4
// FILE_ATTRIBUTE_TEMPORARY           = 0x100
// FILE_ATTRIBUTE_INTEGRITY_STREAM    = 0x8000
// FILE_ATTRIBUTE_NO_SCRUB_DATA       = 0x20000
)

// ShareAccess
const (
	FILE_SHARE_READ = 1 << iota
	FILE_SHARE_WRITE
	FILE_SHARE_DELETE
)

// CreateDisposition
const (
	FILE_SUPERSEDE = iota
	FILE_OPEN
	FILE_CREATE
	FILE_OPEN_IF
	FILE_OVERWRITE
	FILE_OVERWRITE_IF
)

// CreateOptions
const (
	FILE_DIRECTORY_FILE = 1 << iota
	FILE_WRITE_THROUGH
	FILE_SEQUENTIAL_ONLY
	FILE_NO_INTERMEDIATE_BUFFERING
	FILE_SYNCHRONOUS_IO_ALERT
	FILE_SYNCHRONOUS_IO_NONALERT
	FILE_NON_DIRECTORY_FILE
	_
	FILE_COMPLETE_IF_OPLOCKED
	FILE_NO_EA_KNOWLEDGE
	FILE_OPEN_REMOTE_INSTANCE
	FILE_RANDOM_ACCESS
	FILE_DELETE_ON_CLOSE
	FILE_OPEN_BY_FILE_ID
	FILE_OPEN_FOR_BACKUP_INTENT
	FILE_NO_COMPRESSION
	FILE_OPEN_REQUIRING_OPLOCK
	FILE_DISALLOW_EXCLUSIVE
	_
	_
	FILE_RESERVE_OPFILTER
	FILE_OPEN_REPARSE_POINT
	FILE_OPEN_NO_RECALL
	FILE_OPEN_FOR_FREE_SPACE_QUERY
)

//

// OplockLevel
const (
// SMB2_OPLOCK_LEVEL_NONE      = 0x0
// SMB2_OPLOCK_LEVEL_II        = 0x1
// SMB2_OPLOCK_LEVEL_EXCLUSIVE = 0x8
// SMB2_OPLOCK_LEVEL_BATCH     = 0x9
// SMB2_OPLOCK_LEVEL_LEASE     = 0xff
)

// Flags
const (
	SMB2_CREATE_FLAG_REPARSEPOINT = 1 << iota
)

// CreateAction
const (
// FILE_SUPERSEDE = iota
// FILE_OPEN
// FILE_CREATE
// FILE_OPEN_IF
// FILE_OVERWRITE
)

// FileAttributes (from MS-FSCC)
const (
// FILE_ATTRIBUTE_ARCHIVE             = 0x20
// FILE_ATTRIBUTE_COMPRESSED          = 0x800
// FILE_ATTRIBUTE_DIRECTORY           = 0x10
// FILE_ATTRIBUTE_ENCRYPTED           = 0x4000
// FILE_ATTRIBUTE_HIDDEN              = 0x2
// FILE_ATTRIBUTE_NORMAL              = 0x80
// FILE_ATTRIBUTE_NOT_CONTENT_INDEXED = 0x2000
// FILE_ATTRIBUTE_OFFLINE             = 0x1000
// FILE_ATTRIBUTE_READONLY            = 0x1
// FILE_ATTRIBUTE_REPARSE_POINT       = 0x400
// FILE_ATTRIBUTE_SPARSE_FILE         = 0x200
// FILE_ATTRIBUTE_SYSTEM              = 0x4
// FILE_ATTRIBUTE_TEMPORARY           = 0x100
// FILE_ATTRIBUTE_INTEGRITY_STREAM    = 0x8000
// FILE_ATTRIBUTE_NO_SCRUB_DATA       = 0x20000
)

// ----------------------------------------------------------------------------
// SMB2 CLOSE Request and Response
//

// Flags
const (
	SMB2_CLOSE_FLAG_POSTQUERY_ATTRIB = 1 << iota
)

//

// Flags
const (
// SMB2_CLOSE_FLAG_POSTQUERY_ATTRIB = 1 << iota
)

// FileAttributes (from MS-FSCC)
const (
// FILE_ATTRIBUTE_ARCHIVE             = 0x20
// FILE_ATTRIBUTE_COMPRESSED          = 0x800
// FILE_ATTRIBUTE_DIRECTORY           = 0x10
// FILE_ATTRIBUTE_ENCRYPTED           = 0x4000
// FILE_ATTRIBUTE_HIDDEN              = 0x2
// FILE_ATTRIBUTE_NORMAL              = 0x80
// FILE_ATTRIBUTE_NOT_CONTENT_INDEXED = 0x2000
// FILE_ATTRIBUTE_OFFLINE             = 0x1000
// FILE_ATTRIBUTE_READONLY            = 0x1
// FILE_ATTRIBUTE_REPARSE_POINT       = 0x400
// FILE_ATTRIBUTE_SPARSE_FILE         = 0x200
// FILE_ATTRIBUTE_SYSTEM              = 0x4
// FILE_ATTRIBUTE_TEMPORARY           = 0x100
// FILE_ATTRIBUTE_INTEGRITY_STREAM    = 0x8000
// FILE_ATTRIBUTE_NO_SCRUB_DATA       = 0x20000
)

// ----------------------------------------------------------------------------
// SMB2 FLUSH Request and Response
//

//

// ----------------------------------------------------------------------------
// SMB2 READ Request and Response
//

// Flags
const (
	SMB2_READFLAG_READ_UNBUFFERED = 1 << iota
)

// Channel
const (
	SMB2_CHANNEL_NONE = iota
	SMB2_CHANNEL_RDMA_V1
	SMB2_CHANNEL_RDMA_V1_INVALIDATE
)

//

// ----------------------------------------------------------------------------
// SMB2 WRITE Request and Response
//

// Channel
const (
// SMB2_CHANNEL_NONE = iota
// SMB2_CHANNEL_RDMA_V1
// SMB2_CHANNEL_RDMA_V1_INVALIDATE
)

// Flags
const (
	SMB2_WRITEFLAG_WRITE_THROUGH = 1 << iota
	SMB2_WRITEFLAG_WRITE_UNBUFFERED
)

//

// ----------------------------------------------------------------------------
// SMB2 OPLOCK_BREAK Notification, Acknowledgement and Response
//

//

//

// ----------------------------------------------------------------------------
// SMB2 LOCK Request and Response
//

//

// ----------------------------------------------------------------------------
// SMB2 CANCEL Request
//

// ----------------------------------------------------------------------------
// SMB2 ECHO Request and Response
//

//

// ----------------------------------------------------------------------------
// SMB2 IOCTL Request and Response
//

// CtlCode (from MS-FSCC)
const (
// FSCTL_DFS_GET_REFERRALS            = 0x00060194
// FSCTL_PIPE_PEEK                    = 0x0011400C
// FSCTL_PIPE_WAIT                    = 0x00110018
// FSCTL_PIPE_TRANSCEIVE              = 0x0011C017
// FSCTL_SRV_COPYCHUNK                = 0x001440F2
// FSCTL_SRV_ENUMERATE_SNAPSHOTS      = 0x00144064
// FSCTL_SRV_REQUEST_RESUME_KEY       = 0x00140078
// FSCTL_SRV_READ_HASH                = 0x001441bb
// FSCTL_SRV_COPYCHUNK_WRITE          = 0x001480F2
// FSCTL_LMR_REQUEST_RESILIENCY       = 0x001401D4
// FSCTL_QUERY_NETWORK_INTERFACE_INFO = 0x001401FC
// FSCTL_GET_REPARSE_POINT            = 0x000900A8
// FSCTL_SET_REPARSE_POINT            = 0x000900A4
// FSCTL_DFS_GET_REFERRALS_EX         = 0x000601B0
// FSCTL_FILE_LEVEL_TRIM              = 0x00098208
// FSCTL_VALIDATE_NEGOTIATE_INFO      = 0x00140204
)

// Flags
const (
	SMB2_0_IOCTL_IS_FSCTL = 0x1
)

//

// CtlCode (from MS-FSCC)
const (
// FSCTL_DFS_GET_REFERRALS            = 0x00060194
// FSCTL_PIPE_PEEK                    = 0x0011400C
// FSCTL_PIPE_WAIT                    = 0x00110018
// FSCTL_PIPE_TRANSCEIVE              = 0x0011C017
// FSCTL_SRV_COPYCHUNK                = 0x001440F2
// FSCTL_SRV_ENUMERATE_SNAPSHOTS      = 0x00144064
// FSCTL_SRV_REQUEST_RESUME_KEY       = 0x00140078
// FSCTL_SRV_READ_HASH                = 0x001441bb
// FSCTL_SRV_COPYCHUNK_WRITE          = 0x001480F2
// FSCTL_LMR_REQUEST_RESILIENCY       = 0x001401D4
// FSCTL_QUERY_NETWORK_INTERFACE_INFO = 0x001401FC
// FSCTL_SET_REPARSE_POINT            = 0x000900A4
// FSCTL_DFS_GET_REFERRALS_EX         = 0x000601B0
// FSCTL_FILE_LEVEL_TRIM              = 0x00098208
// FSCTL_VALIDATE_NEGOTIATE_INFO      = 0x00140204
)

// ----------------------------------------------------------------------------
// SMB2 QUERY_DIRECTORY Request and Response
//

// FileInformationClass (from MS-FSCC)
const (
// FileDirectoryInformation = 0x1
// FileFullDirectoryInformation = 0x2
// FileIdFullDirectoryInformation = 0x26
// FileBothDirectoryInformation = 0x3
// FileIdBothDirectoryInformation = 0x25
// FileNamesInformation = 0xc
)

// Flags
const (
	RESTART_SCANS = 1 << iota
	RETURN_SINGLE_ENTRY
	INDEX_SPECIFIED
	_
	REOPEN
)

//

// ----------------------------------------------------------------------------
// SMB2 CHANGE_NOTIFY Request and Response
//

//

// ----------------------------------------------------------------------------
// SMB2 QUERY_INFO Request and Response
//

// InfoType
const (
	INFO_FILE = 1 + iota
	INFO_FILESYSTEM
	INFO_SECURITY
	INFO_QUOTA
)

// FileInfoClass (from MS-FSCC)
const (
// FileAccessInformation
// FileAlignmentInformation
// FileAllInformation
// FileAlternateNameInformation
// FileAttributeTagInformation
// FileBasicInformation
// FileCompressionInformation
// FileEaInformation
// FileFullEaInformation
// FileInternalInformation
// FileModeInformation
// FileNetworkOpenInformation
// FilePipeInformation
// FilePipeLocalInformation
// FilePipeRemoteInformation
// FilePositionInformation
// FileStandardInformation
// FileStreamInformation

// FileFsAttributeInformation
// FileFsControlInformation
// FileFsDeviceInformation
// FileFsFullSizeInformation
// FileFsObjectIdInformation
// FileFsSectorSizeInformation
// FileFsSizeInformation
// FileFsVolumeInformation
)

// AdditionalInformation
const (
	OWNER_SECURITY_INFORMATION = 1 << iota
	GROUP_SECUIRTY_INFORMATION
	DACL_SECUIRTY_INFORMATION
	SACL_SECUIRTY_INFORMATION
	LABEL_SECUIRTY_INFORMATION
	ATTRIBUTE_SECUIRTY_INFORMATION
	SCOPE_SECUIRTY_INFORMATION

	BACKUP_SECUIRTY_INFORMATION = 0x10000
)

// Flags
const (
	SL_RESTART_SCAN = 1 << iota
	SL_RETURN_SINGLE_ENTRY
	SL_INDEX_SPECIFIED
)

//

// ----------------------------------------------------------------------------
// SMB2 SET_INFO Request and Response
//

// InfoType
const (
	SMB2_0_INFO_FILE = 1 + iota
	SMB2_0_INFO_FILESYSTEM
	SMB2_0_INFO_SECURITY
	SMB2_0_INFO_QUOTA
)

// FileInfoClass
const (
// FileAllocationInformation
// FileBasicInformation
// FileDispositionInformation
// FileEndOfFileInformation
// FileFullEaInformation
// FileLinkInformation
// FileModeInformation
// FilePipeInformation
// FilePositionInformation
// FileRenameInformation
// FileShortNameInformation
// FileValidDataLengthInformation

// FileFsControlInformation
// FileFsObjectIdInformation
)

// AdditionalInformation
const (
// OWNER_SECURITY_INFORMATION = 1 << iota
// GROUP_SECUIRTY_INFORMATION
// DACL_SECUIRTY_INFORMATION
// SACL_SECUIRTY_INFORMATION
// LABEL_SECUIRTY_INFORMATION
// ATTRIBUTE_SECUIRTY_INFORMATION
// SCOPE_SECUIRTY_INFORMATION

// BACKUP_SECUIRTY_INFORMATION = 0x10000
)

//
