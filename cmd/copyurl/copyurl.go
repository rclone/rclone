// Package copyurl provides the copyurl command.
package copyurl

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/rclone/rclone/cmd"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config/flags"
	"github.com/rclone/rclone/fs/operations"
	"github.com/spf13/cobra"
)

var (
	autoFilename   = false
	headerFilename = false
	printFilename  = false
	stdout         = false
	noClobber      = false
)

func init() {
	cmd.Root.AddCommand(commandDefinition)
	cmdFlags := commandDefinition.Flags()
	flags.BoolVarP(cmdFlags, &autoFilename, "auto-filename", "a", autoFilename, "Get the file name from the URL and use it for destination file path", "")
	flags.BoolVarP(cmdFlags, &headerFilename, "header-filename", "", headerFilename, "Get the file name from the Content-Disposition header", "")
	flags.BoolVarP(cmdFlags, &printFilename, "print-filename", "p", printFilename, "Print the resulting name from --auto-filename", "")
	flags.BoolVarP(cmdFlags, &noClobber, "no-clobber", "", noClobber, "Prevent overwriting file with same name", "")
	flags.BoolVarP(cmdFlags, &stdout, "stdout", "", stdout, "Write the output to stdout rather than a file", "")
}

var commandDefinition = &cobra.Command{
	Use:   "copyurl https://example.com dest:path",
	Short: `Copy the contents of the URL supplied content to dest:path.`,
	Long: strings.ReplaceAll(`Download a URL's content and copy it to the destination without saving
it in temporary storage.

Setting |--auto-filename| will attempt to automatically determine the
filename from the URL (after any redirections) and used in the
destination path.

With |--auto-filename-header| in addition, if a specific filename is
set in HTTP headers, it will be used instead of the name from the URL.
With |--print-filename| in addition, the resulting file name will be
printed.

Setting |--no-clobber| will prevent overwriting file on the 
destination if there is one with the same name.

Setting |--stdout| or making the output file name |-|
will cause the output to be written to standard output.

### Troublshooting

If you can't get |rclone copyurl| to work then here are some things you can try:

- |--disable-http2| rclone will use HTTP2 if available - try disabling it
- |--bind 0.0.0.0| rclone will use IPv6 if available - try disabling it
- |--bind ::0| to disable IPv4
- |--user agent curl| - some sites have whitelists for curl's user-agent - try that
- Make sure the site works with |curl| directly

`, "|", "`"),
	Annotations: map[string]string{
		"versionIntroduced": "v1.43",
		"groups":            "Important",
	},
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
				dst, err = operations.CopyURL(context.Background(), fsdst, dstFileName, args[0], autoFilename, headerFilename, noClobber)
				if printFilename && err == nil && dst != nil {
					fmt.Println(dst.Remote())
				}
			}
			return err
		})
		return nil
	},
}
