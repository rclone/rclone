package copyurl

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/rclone/rclone/cmd"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config/flags"
	"github.com/rclone/rclone/fs/operations"
	"github.com/spf13/cobra"
)

var (
	autoFilename  = false
	printFilename = false
	stdout        = false
	noClobber     = false
)

func init() {
	cmd.Root.AddCommand(commandDefinition)
	cmdFlags := commandDefinition.Flags()
	flags.BoolVarP(cmdFlags, &autoFilename, "auto-filename", "a", autoFilename, "Get the file name from the URL and use it for destination file path")
	flags.BoolVarP(cmdFlags, &printFilename, "print-filename", "p", printFilename, "Print the resulting name from --auto-filename")
	flags.BoolVarP(cmdFlags, &noClobber, "no-clobber", "", noClobber, "Prevent overwriting file with same name")
	flags.BoolVarP(cmdFlags, &stdout, "stdout", "", stdout, "Write the output to stdout rather than a file")
}

var commandDefinition = &cobra.Command{
	Use:   "copyurl https://example.com dest:path",
	Short: `Copy url content to dest.`,
	Long: `
Download a URL's content and copy it to the destination without saving
it in temporary storage.

Setting ` + "`--auto-filename`" + ` will cause the file name to be retrieved from
the from URL (after any redirections) and used in the destination
path. With ` + "`--print-filename`" + ` in addition, the resuling file name will
be printed.

Setting ` + "`--no-clobber`" + ` will prevent overwriting file on the 
destination if there is one with the same name.

Setting ` + "`--stdout`" + ` or making the output file name ` + "`-`" + `
will cause the output to be written to standard output.
`,
	RunE: func(command *cobra.Command, args []string) (err error) {
		cmd.CheckArgs(1, 2, command, args)

		var dstFileName string
		var fsdst fs.Fs
		if !stdout {
			if len(args) < 2 {
				return errors.New("need 2 arguments if not using --stdout")
			}
			if args[1] == "-" {
				stdout = true
			} else if autoFilename {
				fsdst = cmd.NewFsDir(args[1:])
			} else {
				fsdst, dstFileName = cmd.NewFsDstFile(args[1:])
			}
		}
		cmd.Run(true, true, command, func() error {
			var dst fs.Object
			if stdout {
				err = operations.CopyURLToWriter(context.Background(), args[0], os.Stdout)
			} else {
				dst, err = operations.CopyURL(context.Background(), fsdst, dstFileName, args[0], autoFilename, noClobber)
				if printFilename && err == nil && dst != nil {
					fmt.Println(dst.Remote())
				}
			}
			return err
		})
		return nil
	},
}
