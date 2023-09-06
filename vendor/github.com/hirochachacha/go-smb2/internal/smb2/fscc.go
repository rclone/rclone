// ref: MS-FSCC

package smb2

import (
	"github.com/hirochachacha/go-smb2/internal/utf16le"
)

const (
	IO_REPARSE_TAG_RESERVED_ZERO   = 0x00000000
	IO_REPARSE_TAG_RESERVED_ONE    = 0x00000001
	IO_REPARSE_TAG_MOUNT_POINT     = 0xA0000003
	IO_REPARSE_TAG_HSM             = 0xC0000004
	IO_REPARSE_TAG_HSM2            = 0x80000006
	IO_REPARSE_TAG_DRIVER_EXTENDER = 0x80000005
	IO_REPARSE_TAG_SIS             = 0x80000007
	IO_REPARSE_TAG_DFS             = 0x8000000A
	IO_REPARSE_TAG_DFSR            = 0x80000012
	IO_REPARSE_TAG_FILTER_MANAGER  = 0x8000000B
	IO_REPARSE_TAG_SYMLINK         = 0xA000000C
)

const (
	FSCTL_DFS_GET_REFERRALS            = 0x00060194
	FSCTL_PIPE_PEEK                    = 0x0011400C
	FSCTL_PIPE_WAIT                    = 0x00110018
	FSCTL_PIPE_TRANSCEIVE              = 0x0011C017
	FSCTL_SRV_COPYCHUNK                = 0x001440F2
	FSCTL_SRV_ENUMERATE_SNAPSHOTS      = 0x00144064
	FSCTL_SRV_REQUEST_RESUME_KEY       = 0x00140078
	FSCTL_SRV_READ_HASH                = 0x001441bb
	FSCTL_SRV_COPYCHUNK_WRITE          = 0x001480F2
	FSCTL_LMR_REQUEST_RESILIENCY       = 0x001401D4
	FSCTL_QUERY_NETWORK_INTERFACE_INFO = 0x001401FC
	FSCTL_GET_REPARSE_POINT            = 0x000900A8
	FSCTL_SET_REPARSE_POINT            = 0x000900A4
	FSCTL_DFS_GET_REFERRALS_EX         = 0x000601B0
	FSCTL_FILE_LEVEL_TRIM              = 0x00098208
	FSCTL_VALIDATE_NEGOTIATE_INFO      = 0x00140204
)

type SymbolicLinkReparseDataBuffer struct {
	Flags          uint32
	SubstituteName string
	PrintName      string
}

func (c *SymbolicLinkReparseDataBuffer) Size() int {
	return 20 + utf16le.EncodedStringLen(c.SubstituteName) + utf16le.EncodedStringLen(c.PrintName)
}

func (c *SymbolicLinkReparseDataBuffer) Encode(p []byte) {
	slen := utf16le.EncodeString(p[20:], c.SubstituteName)
	plen := utf16le.EncodeString(p[20+slen:], c.PrintName)

	le.PutUint32(p[:4], IO_REPARSE_TAG_SYMLINK)
	le.PutUint16(p[4:6], uint16(len(p)-8)) // ReparseDataLength
	le.PutUint16(p[8:10], 0)               // SubstituteNameOffset
	le.PutUint16(p[10:12], uint16(slen))   // SubstituteNameLength
	le.PutUint16(p[14:16], uint16(plen))   // PrintNameLength
	le.PutUint16(p[12:14], uint16(slen))   // PrintNameOffset
	le.PutUint32(p[16:20], c.Flags)
}

type SymbolicLinkReparseDataBufferDecoder []byte

func (c SymbolicLinkReparseDataBufferDecoder) IsInvalid() bool {
	if len(c) < 20 {
		return true
	}

	if c.ReparseTag() != IO_REPARSE_TAG_SYMLINK {
		return true
	}

	rlen := int(c.ReparseDataLength())
	soff := int(c.SubstituteNameOffset())
	slen := int(c.SubstituteNameLength())
	poff := int(c.PrintNameOffset())
	plen := int(c.PrintNameLength())

	if (soff&1 | poff&1) != 0 {
		return true
	}

	if len(c) < 8+rlen {
		return true
	}

	if rlen < 12+soff+slen || rlen < 12+poff+plen {
		return true
	}

	return false
}

func (c SymbolicLinkReparseDataBufferDecoder) ReparseTag() uint32 {
	return le.Uint32(c[:4])
}

func (c SymbolicLinkReparseDataBufferDecoder) ReparseDataLength() uint16 {
	return le.Uint16(c[4:6])
}

func (c SymbolicLinkReparseDataBufferDecoder) SubstituteNameOffset() uint16 {
	return le.Uint16(c[8:10])
}

func (c SymbolicLinkReparseDataBufferDecoder) SubstituteNameLength() uint16 {
	return le.Uint16(c[10:12])
}

func (c SymbolicLinkReparseDataBufferDecoder) PrintNameOffset() uint16 {
	return le.Uint16(c[12:14])
}

func (c SymbolicLinkReparseDataBufferDecoder) PrintNameLength() uint16 {
	return le.Uint16(c[14:16])
}

func (c SymbolicLinkReparseDataBufferDecoder) Flags() uint32 {
	return le.Uint32(c[16:20])
}

func (c SymbolicLinkReparseDataBufferDecoder) PathBuffer() []byte {
	return c[20:]
}

func (c SymbolicLinkReparseDataBufferDecoder) SubstituteName() string {
	off := c.SubstituteNameOffset()
	len := c.SubstituteNameLength()
	return utf16le.DecodeToString(c.PathBuffer()[off : off+len])
}

func (c SymbolicLinkReparseDataBufferDecoder) PrintName() string {
	off := c.PrintNameOffset()
	len := c.PrintNameLength()
	return utf16le.DecodeToString(c.PathBuffer()[off : off+len])
}

type SrvRequestResumeKeyResponseDecoder []byte

func (c SrvRequestResumeKeyResponseDecoder) IsInvalid() bool {
	if len(c) < int(28+c.ContextLength()) {
		return true
	}
	return false
}

func (c SrvRequestResumeKeyResponseDecoder) ResumeKey() []byte {
	return c[:24]
}

func (c SrvRequestResumeKeyResponseDecoder) ContextLength() uint32 {
	return le.Uint32(c[24:28])
}

func (c SrvRequestResumeKeyResponseDecoder) Context() []byte {
	return c[28 : 28+c.ContextLength()]
}

type SrvCopychunkCopy struct {
	SourceKey [24]byte
	Chunks    []*SrvCopychunk
}

func (c *SrvCopychunkCopy) Size() int {
	return 32 + len(c.Chunks)*24
}

func (c *SrvCopychunkCopy) Encode(p []byte) {
	copy(p[:24], c.SourceKey[:])
	le.PutUint32(p[24:28], uint32(len(c.Chunks)))
	off := 32
	for i, chunk := range c.Chunks {
		chunk.Encode(p[off+i*24 : off+i*24+24])
	}
}

type SrvCopychunk struct {
	SourceOffset int64
	TargetOffset int64
	Length       uint32
}

func (c *SrvCopychunk) Size() int {
	return 24
}

func (c *SrvCopychunk) Encode(p []byte) {
	le.PutUint64(p[:8], uint64(c.SourceOffset))
	le.PutUint64(p[8:16], uint64(c.TargetOffset))
	le.PutUint32(p[16:20], c.Length)
}

type SrvCopychunkResponseDecoder []byte

func (c SrvCopychunkResponseDecoder) IsInvalid() bool {
	return len(c) < 12
}

func (c SrvCopychunkResponseDecoder) ChunksWritten() uint32 {
	return le.Uint32(c[:4])
}

func (c SrvCopychunkResponseDecoder) ChunksBytesWritten() uint32 {
	return le.Uint32(c[4:8])
}

func (c SrvCopychunkResponseDecoder) TotalBytesWritten() uint32 {
	return le.Uint32(c[8:12])
}

const (
	FILE_ATTRIBUTE_ARCHIVE             = 0x20
	FILE_ATTRIBUTE_COMPRESSED          = 0x800
	FILE_ATTRIBUTE_DIRECTORY           = 0x10
	FILE_ATTRIBUTE_ENCRYPTED           = 0x4000
	FILE_ATTRIBUTE_HIDDEN              = 0x2
	FILE_ATTRIBUTE_NORMAL              = 0x80
	FILE_ATTRIBUTE_NOT_CONTENT_INDEXED = 0x2000
	FILE_ATTRIBUTE_OFFLINE             = 0x1000
	FILE_ATTRIBUTE_READONLY            = 0x1
	FILE_ATTRIBUTE_REPARSE_POINT       = 0x400
	FILE_ATTRIBUTE_SPARSE_FILE         = 0x200
	FILE_ATTRIBUTE_SYSTEM              = 0x4
	FILE_ATTRIBUTE_TEMPORARY           = 0x100
	FILE_ATTRIBUTE_INTEGRITY_STREAM    = 0x8000
	FILE_ATTRIBUTE_NO_SCRUB_DATA       = 0x20000
)

const (
	FileDirectoryInformation           = 1 + iota // 1
	FileFullDirectoryInformation                  // 2
	FileBothDirectoryInformation                  // 3
	FileBasicInformation                          // 4
	FileStandardInformation                       // 5
	FileInternalInformation                       // 6
	FileEaInformation                             // 7
	FileAccessInformation                         // 8
	FileNameInformation                           // 9
	FileRenameInformation                         // 10
	FileLinkInformation                           // 11
	FileNamesInformation                          // 12
	FileDispositionInformation                    // 13
	FilePositionInformation                       // 14
	FileFullEaInformation                         // 15
	FileModeInformation                           // 16
	FileAlignmentInformation                      // 17
	FileAllInformation                            // 18
	FileAllocationInformation                     // 19
	FileEndOfFileInformation                      // 20
	FileAlternateNameInformation                  // 21
	FileStreamInformation                         // 22
	FilePipeInformation                           // 23
	FilePipeLocalInformation                      // 24
	FilePipeRemoteInformation                     // 25
	FileMailslotQueryInformation                  // 26
	FileMailslotSetInformation                    // 27
	FileCompressionInformation                    // 28
	FileObjectIdInformation                       // 29
	_                                             // 30
	FileMoveClusterInformation                    // 31
	FileQuotaInformation                          // 32
	FileReparsePointInformation                   // 33
	FileNetworkOpenInformation                    // 34
	FileAttributeTagInformation                   // 35
	FileTrackingInformation                       // 36
	FileIdBothDirectoryInformation                // 37
	FileIdFullDirectoryInformation                // 38
	FileValidDataLengthInformation                // 39
	FileShortNameInformation                      // 40
	_                                             // 41
	_                                             // 42
	_                                             // 43
	FileSfioReserveInformation                    // 44
	FileSfioVolumeInformation                     // 45
	FileHardLinkInformation                       // 46
	_                                             // 47
	FileNormalizedNameInformation                 // 48
	_                                             // 49
	FildIdGlobalTxDirectoryInformation            // 50
	_                                             // 51
	_                                             // 52
	_                                             // 53
	FileStardardLinkInformation                   // 54
)

const (
	FileFsVolumeInformation = 1 + iota
	FileFsLabelInformation
	FileFsSizeInformation
	FileFsDeviceInformation
	FileFsAttributeInformation
	FileFsControlInformation
	FileFsFullSizeInformation
	FileFsObjectIdInformation
	FileFsDriverPathInformation
	FileFsVolumeFlagsInformation
	FileFsSectorSizeInformation
)

type FileDirectoryInformationDecoder []byte

func (c FileDirectoryInformationDecoder) IsInvalid() bool {
	return len(c) < int(64+c.FileNameLength())
}

func (c FileDirectoryInformationDecoder) NextEntryOffset() uint32 {
	return le.Uint32(c[:4])
}

func (c FileDirectoryInformationDecoder) FileIndex() uint32 {
	return le.Uint32(c[4:8])
}

func (c FileDirectoryInformationDecoder) CreationTime() FiletimeDecoder {
	return FiletimeDecoder(c[8:16])
}

func (c FileDirectoryInformationDecoder) LastAccessTime() FiletimeDecoder {
	return FiletimeDecoder(c[16:24])
}

func (c FileDirectoryInformationDecoder) LastWriteTime() FiletimeDecoder {
	return FiletimeDecoder(c[24:32])
}

func (c FileDirectoryInformationDecoder) ChangeTime() FiletimeDecoder {
	return FiletimeDecoder(c[32:40])
}

func (c FileDirectoryInformationDecoder) EndOfFile() int64 {
	return int64(le.Uint64(c[40:48]))
}

func (c FileDirectoryInformationDecoder) AllocationSize() int64 {
	return int64(le.Uint64(c[48:56]))
}

func (c FileDirectoryInformationDecoder) FileAttributes() uint32 {
	return le.Uint32(c[56:60])
}

func (c FileDirectoryInformationDecoder) FileNameLength() uint32 {
	return le.Uint32(c[60:64])
}

func (c FileDirectoryInformationDecoder) FileName() string {
	return utf16le.DecodeToString(c[64 : 64+c.FileNameLength()])
}

type FileRenameInformationType2Encoder struct {
	ReplaceIfExists uint8
	RootDirectory   uint64
	FileName        string
}

func (c *FileRenameInformationType2Encoder) Size() int {
	return 20 + utf16le.EncodedStringLen(c.FileName)
}

func (c *FileRenameInformationType2Encoder) Encode(p []byte) {
	flen := utf16le.EncodeString(p[20:], c.FileName)

	p[0] = c.ReplaceIfExists
	le.PutUint64(p[8:16], c.RootDirectory)
	le.PutUint32(p[16:20], uint32(flen))
}

type FileLinkInformationType2Encoder struct {
	ReplaceIfExists uint8
	RootDirectory   uint64
	FileName        string
}

func (c *FileLinkInformationType2Encoder) Size() int {
	return 20 + utf16le.EncodedStringLen(c.FileName)
}

func (c *FileLinkInformationType2Encoder) Encode(p []byte) {
	flen := utf16le.EncodeString(p[20:], c.FileName)

	p[0] = c.ReplaceIfExists
	le.PutUint64(p[8:16], c.RootDirectory)
	le.PutUint32(p[16:20], uint32(flen))
}

type FileDispositionInformationEncoder struct {
	DeletePending uint8
}

func (c *FileDispositionInformationEncoder) Size() int {
	return 4
}

func (c *FileDispositionInformationEncoder) Encode(p []byte) {
	p[0] = c.DeletePending
}

type FilePositionInformationEncoder struct {
	CurrentByteOffset int64
}

func (c *FilePositionInformationEncoder) Size() int {
	return 8
}

func (c *FilePositionInformationEncoder) Encode(p []byte) {
	le.PutUint64(p[:8], uint64(c.CurrentByteOffset))
}

type FileFsFullSizeInformationDecoder []byte

func (c FileFsFullSizeInformationDecoder) IsInvalid() bool {
	return len(c) < 32
}

func (c FileFsFullSizeInformationDecoder) TotalAllocationUnits() int64 {
	return int64(le.Uint64(c[:8]))
}

func (c FileFsFullSizeInformationDecoder) CallerAvailableAllocationUnits() int64 {
	return int64(le.Uint64(c[8:16]))
}

func (c FileFsFullSizeInformationDecoder) ActualAvailableAllocationUnits() int64 {
	return int64(le.Uint64(c[16:24]))
}

func (c FileFsFullSizeInformationDecoder) SectorsPerAllocationUnit() uint32 {
	return le.Uint32(c[24:28])
}

func (c FileFsFullSizeInformationDecoder) BytesPerSector() uint32 {
	return le.Uint32(c[28:32])
}

type FileQuotaInformationDecoder []byte

func (c FileQuotaInformationDecoder) IsInvalid() bool {
	return len(c) < int(40+c.SidLength())
}

func (c FileQuotaInformationDecoder) NextEntryOffset() uint32 {
	return le.Uint32(c[:4])
}

func (c FileQuotaInformationDecoder) SidLength() uint32 {
	return le.Uint32(c[4:8])
}

func (c FileQuotaInformationDecoder) ChangeTime() FiletimeDecoder {
	return FiletimeDecoder(c[8:16])
}

func (c FileQuotaInformationDecoder) QuotaUsed() int64 {
	return int64(le.Uint64(c[16:24]))
}

func (c FileQuotaInformationDecoder) QuotaThreshold() int64 {
	return int64(le.Uint64(c[24:32]))
}

func (c FileQuotaInformationDecoder) QuotaLimit() int64 {
	return int64(le.Uint64(c[32:40]))
}

func (c FileQuotaInformationDecoder) Sid() SidDecoder {
	return SidDecoder(c[40 : 40+c.SidLength()])
}

type FileEndOfFileInformationEncoder struct {
	EndOfFile int64
}

func (c *FileEndOfFileInformationEncoder) Size() int {
	return 8
}

func (c *FileEndOfFileInformationEncoder) Encode(p []byte) {
	le.PutUint64(p[:8], uint64(c.EndOfFile))
}

type FileEndOfFileInformationDecoder []byte

func (c FileEndOfFileInformationDecoder) IsInvalid() bool {
	return len(c) < 8
}

func (c FileEndOfFileInformationDecoder) EndOfFile() int64 {
	return int64(le.Uint64(c[:8]))
}

type FileAllInformationDecoder []byte

func (c FileAllInformationDecoder) IsInvalid() bool {
	return len(c) < 96
}

func (c FileAllInformationDecoder) BasicInformation() FileBasicInformationDecoder {
	return FileBasicInformationDecoder(c[:40])
}

func (c FileAllInformationDecoder) StandardInformation() FileStandardInformationDecoder {
	return FileStandardInformationDecoder(c[40:64])
}

func (c FileAllInformationDecoder) InternalInformation() FileInternalInformationDecoder {
	return FileInternalInformationDecoder(c[64:72])
}

func (c FileAllInformationDecoder) EaInformation() FileEaInformationDecoder {
	return FileEaInformationDecoder(c[72:76])
}

func (c FileAllInformationDecoder) AccessInformation() FileAccessInformationDecoder {
	return FileAccessInformationDecoder(c[76:80])
}

func (c FileAllInformationDecoder) PositionInformation() FilePositionInformationDecoder {
	return FilePositionInformationDecoder(c[80:88])
}

func (c FileAllInformationDecoder) ModeInformation() FileModeInformationDecoder {
	return FileModeInformationDecoder(c[88:92])
}

func (c FileAllInformationDecoder) AlignmentInformation() FileAlignmentInformationDecoder {
	return FileAlignmentInformationDecoder(c[92:96])
}

func (c FileAllInformationDecoder) NameInformation() FileNameInformationDecoder {
	return FileNameInformationDecoder(c[96:])
}

type FileBasicInformationEncoder struct {
	CreationTime   *Filetime
	LastAccessTime *Filetime
	LastWriteTime  *Filetime
	ChangeTime     *Filetime
	FileAttributes uint32
}

func (c *FileBasicInformationEncoder) Size() int {
	return 40
}

func (c *FileBasicInformationEncoder) Encode(p []byte) {
	if c.CreationTime != nil {
		c.CreationTime.Encode(p[:8])
	}
	if c.LastAccessTime != nil {
		c.LastAccessTime.Encode(p[8:16])
	}
	if c.LastWriteTime != nil {
		c.LastWriteTime.Encode(p[16:24])
	}
	if c.ChangeTime != nil {
		c.ChangeTime.Encode(p[24:32])
	}
	le.PutUint32(p[32:36], c.FileAttributes)
}

type FileBasicInformationDecoder []byte

func (c FileBasicInformationDecoder) IsInvalid() bool {
	return len(c) < 40
}

func (c FileBasicInformationDecoder) CreationTime() FiletimeDecoder {
	return FiletimeDecoder(c[:8])
}

func (c FileBasicInformationDecoder) LastAccessTime() FiletimeDecoder {
	return FiletimeDecoder(c[8:16])
}

func (c FileBasicInformationDecoder) LastWriteTime() FiletimeDecoder {
	return FiletimeDecoder(c[16:24])
}

func (c FileBasicInformationDecoder) ChangeTime() FiletimeDecoder {
	return FiletimeDecoder(c[24:32])
}

func (c FileBasicInformationDecoder) FileAttributes() uint32 {
	return le.Uint32(c[32:36])
}

type FileStandardInformationDecoder []byte

func (c FileStandardInformationDecoder) IsInvalid() bool {
	return len(c) < 24
}

func (c FileStandardInformationDecoder) AllocationSize() int64 {
	return int64(le.Uint64(c[:8]))
}

func (c FileStandardInformationDecoder) EndOfFile() int64 {
	return int64(le.Uint64(c[8:16]))
}

func (c FileStandardInformationDecoder) NumberOfLinks() uint32 {
	return le.Uint32(c[16:20])
}

func (c FileStandardInformationDecoder) DeletePending() uint8 {
	return c[20]
}

func (c FileStandardInformationDecoder) Directory() uint8 {
	return c[21]
}

type FileInternalInformationDecoder []byte

func (c FileInternalInformationDecoder) IsInvalid() bool {
	return len(c) < 8
}

func (c FileInternalInformationDecoder) IndexNumber() int64 {
	return int64(le.Uint64(c[:8]))
}

type FileEaInformationDecoder []byte

func (c FileEaInformationDecoder) IsInvalid() bool {
	return len(c) < 4
}

func (c FileEaInformationDecoder) EaSize() uint32 {
	return le.Uint32(c[:4])
}

type FileAccessInformationDecoder []byte

func (c FileAccessInformationDecoder) IsInvalid() bool {
	return len(c) < 4
}

func (c FileAccessInformationDecoder) AccessFlags() uint32 {
	return le.Uint32(c[:4])
}

type FilePositionInformationDecoder []byte

func (c FilePositionInformationDecoder) IsInvalid() bool {
	return len(c) < 8
}

func (c FilePositionInformationDecoder) CurrentByteOffset() int64 {
	return int64(le.Uint64(c[:8]))
}

type FileModeInformationDecoder []byte

func (c FileModeInformationDecoder) IsInvalid() bool {
	return len(c) < 4
}

func (c FileModeInformationDecoder) Mode() uint32 {
	return le.Uint32(c[:4])
}

type FileAlignmentInformationDecoder []byte

func (c FileAlignmentInformationDecoder) IsInvalid() bool {
	return len(c) < 4
}

func (c FileAlignmentInformationDecoder) AlignmentRequirement() uint32 {
	return le.Uint32(c[:4])
}

type FileNameInformationDecoder []byte

func (c FileNameInformationDecoder) IsInvalid() bool {
	if len(c) < 4 {
		return true
	}

	if len(c) < int(4+c.FileNameLength()) {
		return true
	}

	return false
}

func (c FileNameInformationDecoder) FileNameLength() uint32 {
	return le.Uint32(c[:4])
}

func (c FileNameInformationDecoder) FileName() string {
	return utf16le.DecodeToString(c[4 : 4+c.FileNameLength()])
}
