// Package checksum provides the checksum command.
package checksum

import (
	"context"
	"fmt"
	"strings"

	"github.com/rclone/rclone/cmd"
	"github.com/rclone/rclone/cmd/check" // for common flags
	"github.com/rclone/rclone/fs/config/flags"
	"github.com/rclone/rclone/fs/hash"
	"github.com/rclone/rclone/fs/operations"
	"github.com/spf13/cobra"
)

var download = false

func init() {
	cmd.Root.AddCommand(commandDefinition)
	cmdFlags := commandDefinition.Flags()
	flags.BoolVarP(cmdFlags, &download, "download", "", download, "Check by hashing the contents")
	check.AddFlags(cmdFlags)
}

var commandDefinition = &cobra.Command{
	Use:   "checksum <hash> sumfile src:path",
	Short: `Checks the files in the source against a SUM file.`,
	Long: strings.ReplaceAll(`
Checks that hashsums of source files match the SUM file.
It compares hashes (MD5, SHA1, etc) and logs a report of files which
don't match.  It doesn't alter the file system.

If you supply the |--download| flag, it will download the data from remote
and calculate the contents hash on the fly.  This can be useful for remotes
that don't support hashes or if you really want to check all the data.

Note that hash values in the SUM file are treated as case insensitive.
`, "|", "`") + check.FlagsHelp,
	RunE: func(command *cobra.Command, args []string) error {
		cmd.CheckArgs(3, 3, command, args)
		var hashType hash.Type
		if err := hashType.Set(args[0]); err != nil {
			fmt.Println(hash.HelpString(0))
			return err
		}
		fsum, sumFile, fsrc := cmd.NewFsSrcFileDst(args[1:])

		cmd.Run(false, true, command, func() error {
			opt, close, err := check.GetCheckOpt(nil, fsrc)
			if err != nil {
				return err
			}
			defer close()

			return operations.CheckSum(context.Background(), fsrc, fsum, sumFile, hashType, opt, download)
		})
		return nil
	},
}
