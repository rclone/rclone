// +build !linux,!darwin,!freebsd

package vfs

import (
	"github.com/spf13/pflag"
)

// add any extra platform specific flags
func platformFlags(flags *pflag.FlagSet) {
}
