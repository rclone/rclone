// Package sftp implements the SSH File Transfer Protocol as described in
// https://tools.ietf.org/html/draft-ietf-secsh-filexfer-02
package sftp

import (
	"fmt"

	"github.com/pkg/errors"
)

const (
	sshFxpInit          = 1
	sshFxpVersion       = 2
	sshFxpOpen          = 3
	sshFxpClose         = 4
	sshFxpRead          = 5
	sshFxpWrite         = 6
	sshFxpLstat         = 7
	sshFxpFstat         = 8
	sshFxpSetstat       = 9
	sshFxpFsetstat      = 10
	sshFxpOpendir       = 11
	sshFxpReaddir       = 12
	sshFxpRemove        = 13
	sshFxpMkdir         = 14
	sshFxpRmdir         = 15
	sshFxpRealpath      = 16
	sshFxpStat          = 17
	sshFxpRename        = 18
	sshFxpReadlink      = 19
	sshFxpSymlink       = 20
	sshFxpStatus        = 101
	sshFxpHandle        = 102
	sshFxpData          = 103
	sshFxpName          = 104
	sshFxpAttrs         = 105
	sshFxpExtended      = 200
	sshFxpExtendedReply = 201
)

const (
	sshFxOk               = 0
	sshFxEOF              = 1
	sshFxNoSuchFile       = 2
	sshFxPermissionDenied = 3
	sshFxFailure          = 4
	sshFxBadMessage       = 5
	sshFxNoConnection     = 6
	sshFxConnectionLost   = 7
	sshFxOPUnsupported    = 8

	// see draft-ietf-secsh-filexfer-13
	// https://tools.ietf.org/html/draft-ietf-secsh-filexfer-13#section-9.1
	sshFxInvalidHandle           = 9
	sshFxNoSuchPath              = 10
	sshFxFileAlreadyExists       = 11
	sshFxWriteProtect            = 12
	sshFxNoMedia                 = 13
	sshFxNoSpaceOnFilesystem     = 14
	sshFxQuotaExceeded           = 15
	sshFxUnlnownPrincipal        = 16
	sshFxLockConflict            = 17
	sshFxDitNotEmpty             = 18
	sshFxNotADirectory           = 19
	sshFxInvalidFilename         = 20
	sshFxLinkLoop                = 21
	sshFxCannotDelete            = 22
	sshFxInvalidParameter        = 23
	sshFxFileIsADirectory        = 24
	sshFxByteRangeLockConflict   = 25
	sshFxByteRangeLockRefused    = 26
	sshFxDeletePending           = 27
	sshFxFileCorrupt             = 28
	sshFxOwnerInvalid            = 29
	sshFxGroupInvalid            = 30
	sshFxNoMatchingByteRangeLock = 31
)

const (
	sshFxfRead   = 0x00000001
	sshFxfWrite  = 0x00000002
	sshFxfAppend = 0x00000004
	sshFxfCreat  = 0x00000008
	sshFxfTrunc  = 0x00000010
	sshFxfExcl   = 0x00000020
)

var (
	// supportedSFTPExtensions defines the supported extensions
	supportedSFTPExtensions = []sshExtensionPair{
		{"hardlink@openssh.com", "1"},
		{"posix-rename@openssh.com", "1"},
	}
	sftpExtensions = supportedSFTPExtensions
)

type fxp uint8

func (f fxp) String() string {
	switch f {
	case sshFxpInit:
		return "SSH_FXP_INIT"
	case sshFxpVersion:
		return "SSH_FXP_VERSION"
	case sshFxpOpen:
		return "SSH_FXP_OPEN"
	case sshFxpClose:
		return "SSH_FXP_CLOSE"
	case sshFxpRead:
		return "SSH_FXP_READ"
	case sshFxpWrite:
		return "SSH_FXP_WRITE"
	case sshFxpLstat:
		return "SSH_FXP_LSTAT"
	case sshFxpFstat:
		return "SSH_FXP_FSTAT"
	case sshFxpSetstat:
		return "SSH_FXP_SETSTAT"
	case sshFxpFsetstat:
		return "SSH_FXP_FSETSTAT"
	case sshFxpOpendir:
		return "SSH_FXP_OPENDIR"
	case sshFxpReaddir:
		return "SSH_FXP_READDIR"
	case sshFxpRemove:
		return "SSH_FXP_REMOVE"
	case sshFxpMkdir:
		return "SSH_FXP_MKDIR"
	case sshFxpRmdir:
		return "SSH_FXP_RMDIR"
	case sshFxpRealpath:
		return "SSH_FXP_REALPATH"
	case sshFxpStat:
		return "SSH_FXP_STAT"
	case sshFxpRename:
		return "SSH_FXP_RENAME"
	case sshFxpReadlink:
		return "SSH_FXP_READLINK"
	case sshFxpSymlink:
		return "SSH_FXP_SYMLINK"
	case sshFxpStatus:
		return "SSH_FXP_STATUS"
	case sshFxpHandle:
		return "SSH_FXP_HANDLE"
	case sshFxpData:
		return "SSH_FXP_DATA"
	case sshFxpName:
		return "SSH_FXP_NAME"
	case sshFxpAttrs:
		return "SSH_FXP_ATTRS"
	case sshFxpExtended:
		return "SSH_FXP_EXTENDED"
	case sshFxpExtendedReply:
		return "SSH_FXP_EXTENDED_REPLY"
	default:
		return "unknown"
	}
}

type fx uint8

func (f fx) String() string {
	switch f {
	case sshFxOk:
		return "SSH_FX_OK"
	case sshFxEOF:
		return "SSH_FX_EOF"
	case sshFxNoSuchFile:
		return "SSH_FX_NO_SUCH_FILE"
	case sshFxPermissionDenied:
		return "SSH_FX_PERMISSION_DENIED"
	case sshFxFailure:
		return "SSH_FX_FAILURE"
	case sshFxBadMessage:
		return "SSH_FX_BAD_MESSAGE"
	case sshFxNoConnection:
		return "SSH_FX_NO_CONNECTION"
	case sshFxConnectionLost:
		return "SSH_FX_CONNECTION_LOST"
	case sshFxOPUnsupported:
		return "SSH_FX_OP_UNSUPPORTED"
	default:
		return "unknown"
	}
}

type unexpectedPacketErr struct {
	want, got uint8
}

func (u *unexpectedPacketErr) Error() string {
	return fmt.Sprintf("sftp: unexpected packet: want %v, got %v", fxp(u.want), fxp(u.got))
}

func unimplementedPacketErr(u uint8) error {
	return errors.Errorf("sftp: unimplemented packet type: got %v", fxp(u))
}

type unexpectedIDErr struct{ want, got uint32 }

func (u *unexpectedIDErr) Error() string {
	return fmt.Sprintf("sftp: unexpected id: want %v, got %v", u.want, u.got)
}

func unimplementedSeekWhence(whence int) error {
	return errors.Errorf("sftp: unimplemented seek whence %v", whence)
}

func unexpectedCount(want, got uint32) error {
	return errors.Errorf("sftp: unexpected count: want %v, got %v", want, got)
}

type unexpectedVersionErr struct{ want, got uint32 }

func (u *unexpectedVersionErr) Error() string {
	return fmt.Sprintf("sftp: unexpected server version: want %v, got %v", u.want, u.got)
}

// A StatusError is returned when an SFTP operation fails, and provides
// additional information about the failure.
type StatusError struct {
	Code      uint32
	msg, lang string
}

func (s *StatusError) Error() string {
	return fmt.Sprintf("sftp: %q (%v)", s.msg, fx(s.Code))
}

// FxCode returns the error code typed to match against the exported codes
func (s *StatusError) FxCode() fxerr {
	return fxerr(s.Code)
}

func getSupportedExtensionByName(extensionName string) (sshExtensionPair, error) {
	for _, supportedExtension := range supportedSFTPExtensions {
		if supportedExtension.Name == extensionName {
			return supportedExtension, nil
		}
	}
	return sshExtensionPair{}, fmt.Errorf("Unsupported extension: %v", extensionName)
}

// SetSFTPExtensions allows to customize the supported server extensions.
// See the variable supportedSFTPExtensions for supported extensions.
// This method accepts a slice of sshExtensionPair names for example 'hardlink@openssh.com'.
// If an invalid extension is given an error will be returned and nothing will be changed
func SetSFTPExtensions(extensions ...string) error {
	tempExtensions := []sshExtensionPair{}
	for _, extension := range extensions {
		sftpExtension, err := getSupportedExtensionByName(extension)
		if err != nil {
			return err
		}
		tempExtensions = append(tempExtensions, sftpExtension)
	}
	sftpExtensions = tempExtensions
	return nil
}
