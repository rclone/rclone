package common

import "errors"

var (
	ErrKeyPassOrSaltedKeyPassMustBeNotNil = errors.New("either keyPass or saltedKeyPass must be not nil")
	ErrFailedToUnlockUserKeys             = errors.New("failed to unlock user keys")

	ErrUsernameAndPasswordRequired = errors.New("username and password are required")
	Err2FACodeRequired             = errors.New("this account requires a 2FA code")
)
