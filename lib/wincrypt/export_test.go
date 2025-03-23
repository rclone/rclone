//go:build windows
// +build windows

package wincrypt

import (
	"syscall"
)

var (
	NCrypt  = ncrypt
	Crypt32 = crypt32
)

func NCryptFreeObject(obj syscall.Handle) error {
	return ncryptFreeObject(obj)
}

func WrapError(prefix string, err error, args ...any) error {
	return wrapError(prefix, err, args...)
}
