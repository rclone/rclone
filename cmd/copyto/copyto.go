// Package copyto provides the copyto command.
package copyto

import (
	"context"

	"github.com/rclone/rclone/cmd"
	"github.com/rclone/rclone/fs/operations"
	"github.com/rclone/rclone/fs/sync"
	"github.com/spf13/cobra"
)

func init() {
	cmd.Root.AddCommand(commandDefinition)
}

var commandDefinition = &cobra.Command{
	Use:   "copyto source:path dest:path",
	Short: `Copy files from source to dest, skipping identical files.`,
	Long: `If source:path is a file or directory then it copies it to a file or
directory named dest:path.

This can be used to upload single files to other than their current
name.  If the source is a directory then it acts exactly like the
[copy](/commands/rclone_copy/) command.

So

    rclone copyto src dst

where src and dst are rclone paths, either remote:path or
/path/to/local or C:\windows\path\if\on\windows.

This will:

    if src is file
        copy it to dst, overwriting an existing file if it exists
    if src is directory
        copy it to dst, overwriting existing files if they exist
        see copy command for full details

This doesn't transfer files that are identical on src and dst, testing
by size and modification time or MD5SUM.  It doesn't delete files from
the destination.

**Note**: Use the ` + "`-P`" + `/` + "`--progress`" + ` flag to view real-time transfer statistics
`,
	Annotations: map[string]string{
		"versionIntroduced": "v1.35",
		"groups":            "Copy,Filter,Listing,Important",
	},
	Run: func(command *cobra.Command, args []string) {
		cmd.CheckArgs(2, 2, command, args)
		fsrc, srcFileName, fdst, dstFileName := cmd.NewFsSrcDstFiles(args)
		cmd.Run(true, true, command, func() error {
			if srcFileName == "" {
				return sync.CopyDir(context.Background(), fdst, fsrc, false)
			}
			return operations.CopyFile(context.Background(), fdst, fsrc, dstFileName, srcFileName)
		})
	},
}
