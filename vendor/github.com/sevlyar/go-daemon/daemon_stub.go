// +build !darwin,!dragonfly,!freebsd,!linux,!netbsd,!openbsd,!plan9,!solaris

package daemon

import (
	"os"
)

func (d *Context) reborn() (child *os.Process, err error) {
	return nil, errNotSupported
}

func (d *Context) search() (daemon *os.Process, err error) {
	return nil, errNotSupported
}

func (d *Context) release() (err error) {
	return nil, errNotSupported
}
