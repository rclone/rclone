package copy

import (
	"github.com/ncw/rclone/cmd"
	"github.com/ncw/rclone/fs"
	"github.com/spf13/cobra"
)

func init() {
	cmd.Root.AddCommand(commandDefintion)
}

var commandDefintion = &cobra.Command{
	Use:   "copy source:path dest:path",
	Short: `Copy files from source to dest, skipping already copied`,
	Long: `
Copy the source to the destination.  Doesn't transfer
unchanged files, testing by size and modification time or
MD5SUM.  Doesn't delete files from the destination.

Note that it is always the contents of the directory that is synced,
not the directory so when source:path is a directory, it's the
contents of source:path that are copied, not the directory name and
contents.

If dest:path doesn't exist, it is created and the source:path contents
go there.

For example

    rclone copy source:sourcepath dest:destpath

Let's say there are two files in sourcepath

    sourcepath/one.txt
    sourcepath/two.txt

This copies them to

    destpath/one.txt
    destpath/two.txt

Not to

    destpath/sourcepath/one.txt
    destpath/sourcepath/two.txt

If you are familiar with ` + "`rsync`" + `, rclone always works as if you had
written a trailing / - meaning "copy the contents of this directory".
This applies to all commands and whether you are talking about the
source or destination.

See the ` + "`--no-traverse`" + ` option for controlling whether rclone lists
the destination directory or not.
`,
	Run: func(command *cobra.Command, args []string) {
		cmd.CheckArgs(2, 2, command, args)
		fsrc, fdst := cmd.NewFsSrcDst(args)
		cmd.Run(true, true, command, func() error {
			return fs.CopyDir(fdst, fsrc)
		})
	},
}
