package api

import (
	"errors"
	"fmt"
	"os"
)

var (
	// ErrTryAgainLater hit frequency limit
	ErrTryAgainLater = errors.New("hit frequency limit, try again later")
	// ErrAuthenticationFailed token invalid
	ErrAuthenticationFailed = errors.New("authentication failed")
	// ErrIllegalFilename filename is illegal
	ErrIllegalFilename = errors.New("illegal filename")
)

// Err convert number to error
func Err(errno int) error {
	switch errno {
	case -6:
		fallthrough
	case 111:
		return ErrAuthenticationFailed
	case 31034:
		return ErrTryAgainLater
	case -3:
		fallthrough
	case -31066:
		fallthrough
	case 31066:
		fallthrough
	case -9:
		return os.ErrNotExist
	case 2:
		fallthrough
	case 31023:
		return os.ErrInvalid
	case -7:
		fallthrough
	case 31062:
		return ErrIllegalFilename
	case 31061:
		return os.ErrExist
	default:
		return fmt.Errorf("unknown error. code %d", errno)
	}
}
