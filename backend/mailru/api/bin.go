package api

// BIN protocol constants
const (
	BinContentType    = "application/x-www-form-urlencoded"
	TreeIDLength      = 12
	DunnoNodeIDLength = 16
)

// Operations in binary protocol
const (
	OperationAddFile           = 103 // 0x67
	OperationRename            = 105 // 0x69
	OperationCreateFolder      = 106 // 0x6A
	OperationFolderList        = 117 // 0x75
	OperationSharedFoldersList = 121 // 0x79
	// TODO investigate opcodes below
	Operation154MaybeItemInfo = 154 // 0x9A
	Operation102MaybeAbout    = 102 // 0x66
	Operation104MaybeDelete   = 104 // 0x68
)

// CreateDir protocol constants
const (
	MkdirResultOK                  = 0
	MkdirResultSourceNotExists     = 1
	MkdirResultAlreadyExists       = 4
	MkdirResultExistsDifferentCase = 9
	MkdirResultInvalidName         = 10
	MkdirResultFailed254           = 254
)

// Move result codes
const (
	MoveResultOK              = 0
	MoveResultSourceNotExists = 1
	MoveResultFailed002       = 2
	MoveResultAlreadyExists   = 4
	MoveResultFailed005       = 5
	MoveResultFailed254       = 254
)

// AddFile result codes
const (
	AddResultOK          = 0
	AddResultError01     = 1
	AddResultDunno04     = 4
	AddResultWrongPath   = 5
	AddResultNoFreeSpace = 7
	AddResultDunno09     = 9
	AddResultInvalidName = 10
	AddResultNotModified = 12
	AddResultFailedA     = 253
	AddResultFailedB     = 254
)

// List request options
const (
	ListOptTotalSpace  = 1
	ListOptDelete      = 2
	ListOptFingerprint = 4
	ListOptUnknown8    = 8
	ListOptUnknown16   = 16
	ListOptFolderSize  = 32
	ListOptUsedSpace   = 64
	ListOptUnknown128  = 128
	ListOptUnknown256  = 256
)

// ListOptDefaults ...
const ListOptDefaults = ListOptUnknown128 | ListOptUnknown256 | ListOptFolderSize | ListOptTotalSpace | ListOptUsedSpace

// List parse flags
const (
	ListParseDone      = 0
	ListParseReadItem  = 1
	ListParsePin       = 2
	ListParsePinUpper  = 3
	ListParseUnknown15 = 15
)

// List operation results
const (
	ListResultOK              = 0
	ListResultNotExists       = 1
	ListResultDunno02         = 2
	ListResultDunno03         = 3
	ListResultAlreadyExists04 = 4
	ListResultDunno05         = 5
	ListResultDunno06         = 6
	ListResultDunno07         = 7
	ListResultDunno08         = 8
	ListResultAlreadyExists09 = 9
	ListResultDunno10         = 10
	ListResultDunno11         = 11
	ListResultDunno12         = 12
	ListResultFailedB         = 253
	ListResultFailedA         = 254
)

// Directory item types
const (
	ListItemMountPoint   = 0
	ListItemFile         = 1
	ListItemFolder       = 2
	ListItemSharedFolder = 3
)
