// Package moveto provides the moveto command.
package moveto

import (
	"context"

	"github.com/rclone/rclone/cmd"
	"github.com/rclone/rclone/fs/operations"
	"github.com/rclone/rclone/fs/operations/operationsflags"
	"github.com/rclone/rclone/fs/sync"
	"github.com/spf13/cobra"
)

var (
	loggerOpt      = operations.LoggerOpt{}
	loggerFlagsOpt = operationsflags.AddLoggerFlagsOptions{}
)

func init() {
	cmd.Root.AddCommand(commandDefinition)
	cmdFlags := commandDefinition.Flags()
	operationsflags.AddLoggerFlags(cmdFlags, &loggerOpt, &loggerFlagsOpt)
	loggerOpt.LoggerFn = operations.NewDefaultLoggerFn(&loggerOpt)
}

var commandDefinition = &cobra.Command{
	Use:   "moveto source:path dest:path",
	Short: `Move file or directory from source to dest.`,
	Long: `If source:path is a file or directory then it moves it to a file or
directory named dest:path.

This can be used to rename files or upload single files to other than
their existing name.  If the source is a directory then it acts exactly
like the [move](/commands/rclone_move/) command.

So

` + "```console" + `
rclone moveto src dst
` + "```" + `

where src and dst are rclone paths, either remote:path or
/path/to/local or C:\windows\path\if\on\windows.

This will:

` + "```text" + `
if src is file
    move it to dst, overwriting an existing file if it exists
if src is directory
    move it to dst, overwriting existing files if they exist
    see move command for full details
` + "```" + `

This doesn't transfer files that are identical on src and dst, testing
by size and modification time or MD5SUM.  src will be deleted on
successful transfer.

**Important**: Since this can cause data loss, test first with the
` + "`--dry-run` or the `--interactive`/`-i`" + ` flag.

**Note**: Use the ` + "`-P`" + `/` + "`--progress`" + ` flag to view real-time transfer statistics.

` + operationsflags.Help(),
	Annotations: map[string]string{
		"versionIntroduced": "v1.35",
		"groups":            "Filter,Listing,Important,Copy",
	},
	Run: func(command *cobra.Command, args []string) {
		cmd.CheckArgs(2, 2, command, args)
		fsrc, srcFileName, fdst, dstFileName := cmd.NewFsSrcDstFiles(args)

		cmd.Run(true, true, command, func() error {
			ctx := context.Background()
			close, err := operationsflags.ConfigureLoggers(ctx, fdst, command, &loggerOpt, loggerFlagsOpt)
			if err != nil {
				return err
			}
			defer close()

			if loggerFlagsOpt.AnySet() {
				ctx = operations.WithSyncLogger(ctx, loggerOpt)
			}

			if srcFileName == "" {
				return sync.MoveDir(ctx, fdst, fsrc, false, false)
			}
			return operations.MoveFile(ctx, fdst, fsrc, dstFileName, srcFileName)
		})
	},
}
