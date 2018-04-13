package mega

import (
	"errors"
	"fmt"
)

var (
	// General errors
	EINTERNAL  = errors.New("Internal error occured")
	EARGS      = errors.New("Invalid arguments")
	EAGAIN     = errors.New("Try again")
	ERATELIMIT = errors.New("Rate limit reached")
	EBADRESP   = errors.New("Bad response from server")

	// Upload errors
	EFAILED  = errors.New("The upload failed. Please restart it from scratch")
	ETOOMANY = errors.New("Too many concurrent IP addresses are accessing this upload target URL")
	ERANGE   = errors.New("The upload file packet is out of range or not starting and ending on a chunk boundary")
	EEXPIRED = errors.New("The upload target URL you are trying to access has expired. Please request a fresh one")

	// Filesystem/Account errors
	ENOENT       = errors.New("Object (typically, node or user) not found")
	ECIRCULAR    = errors.New("Circular linkage attempted")
	EACCESS      = errors.New("Access violation")
	EEXIST       = errors.New("Trying to create an object that already exists")
	EINCOMPLETE  = errors.New("Trying to access an incomplete resource")
	EKEY         = errors.New("A decryption operation failed")
	ESID         = errors.New("Invalid or expired user session, please relogin")
	EBLOCKED     = errors.New("User blocked")
	EOVERQUOTA   = errors.New("Request over quota")
	ETEMPUNAVAIL = errors.New("Resource temporarily not available, please try again later")
	EMACMISMATCH = errors.New("MAC verification failed")
	EBADATTR     = errors.New("Bad node attribute")

	// Config errors
	EWORKER_LIMIT_EXCEEDED = errors.New("Maximum worker limit exceeded")
)

type ErrorMsg int

func parseError(errno ErrorMsg) error {
	switch {
	case errno == 0:
		return nil
	case errno == -1:
		return EINTERNAL
	case errno == -2:
		return EARGS
	case errno == -3:
		return EAGAIN
	case errno == -4:
		return ERATELIMIT
	case errno == -5:
		return EFAILED
	case errno == -6:
		return ETOOMANY
	case errno == -7:
		return ERANGE
	case errno == -8:
		return EEXPIRED
	case errno == -9:
		return ENOENT
	case errno == -10:
		return ECIRCULAR
	case errno == -11:
		return EACCESS
	case errno == -12:
		return EEXIST
	case errno == -13:
		return EINCOMPLETE
	case errno == -14:
		return EKEY
	case errno == -15:
		return ESID
	case errno == -16:
		return EBLOCKED
	case errno == -17:
		return EOVERQUOTA
	case errno == -18:
		return ETEMPUNAVAIL
	}

	return fmt.Errorf("Unknown mega error %d", errno)
}
