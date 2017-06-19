// +build !linux,!darwin,!freebsd

package mountlib

import (
	"github.com/spf13/pflag"
)

// add any extra platform specific flags
func platformFlags(flags *pflag.FlagSet) {
}
