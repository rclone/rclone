package sftp

// Error types that match the SFTP's SSH_FXP_STATUS codes. Gives you more
// direct control of the errors being sent vs. letting the library work them
// out from the standard os/io errors.

type fxerr uint32

const (
	ErrSshFxOk               = fxerr(ssh_FX_OK)
	ErrSshFxEof              = fxerr(ssh_FX_EOF)
	ErrSshFxNoSuchFile       = fxerr(ssh_FX_NO_SUCH_FILE)
	ErrSshFxPermissionDenied = fxerr(ssh_FX_PERMISSION_DENIED)
	ErrSshFxFailure          = fxerr(ssh_FX_FAILURE)
	ErrSshFxBadMessage       = fxerr(ssh_FX_BAD_MESSAGE)
	ErrSshFxNoConnection     = fxerr(ssh_FX_NO_CONNECTION)
	ErrSshFxConnectionLost   = fxerr(ssh_FX_CONNECTION_LOST)
	ErrSshFxOpUnsupported    = fxerr(ssh_FX_OP_UNSUPPORTED)
)

func (e fxerr) Error() string {
	switch e {
	case ErrSshFxOk:
		return "OK"
	case ErrSshFxEof:
		return "EOF"
	case ErrSshFxNoSuchFile:
		return "No Such File"
	case ErrSshFxPermissionDenied:
		return "Permission Denied"
	case ErrSshFxBadMessage:
		return "Bad Message"
	case ErrSshFxNoConnection:
		return "No Connection"
	case ErrSshFxConnectionLost:
		return "Connection Lost"
	case ErrSshFxOpUnsupported:
		return "Operation Unsupported"
	default:
		return "Failure"
	}
}
