// Package transformflags implements command line flags to set up a transform
package transformflags

import (
	"github.com/rclone/rclone/fs/config/flags"
	"github.com/rclone/rclone/lib/transform"

	"github.com/spf13/pflag"
)

// AddFlags adds the transform flags to the command
func AddFlags(flagSet *pflag.FlagSet) {
	flags.AddFlagsFromOptions(flagSet, "", transform.OptionsInfo)
}
