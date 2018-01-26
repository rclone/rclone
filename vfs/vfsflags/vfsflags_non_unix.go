// +build !linux,!darwin,!freebsd linux,js

package vfsflags

import (
	"github.com/spf13/pflag"
)

// add any extra platform specific flags
func platformFlags(flags *pflag.FlagSet) {
}
