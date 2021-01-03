package syncfiles

import (
	"context"

	"github.com/pkg/errors"
	"github.com/rclone/rclone/cmd"
	"github.com/rclone/rclone/fs/fserrors"
	"github.com/rclone/rclone/fs/syncfiles"
	"github.com/spf13/cobra"
)

func init() {
	cmd.Root.AddCommand(commandDefinition)
}

// syncFilesCmd represents the syncFiles command
var commandDefinition = &cobra.Command{
	Use:   "syncfiles [remote:file-path local-directory-path]",
	Short: "Sync a set of files with (possibly different) remote locations",
	Long: `
Syncfiles maintains local copies of files from different remote
locations. It does this by remembering the remote location for 
each file in the given local directory. Doesn't transfer unchanged
files, testing by modification time.
				
Initially, the file required to be synced has to be registered 
using the remote:file-path argument, which must specify a single
file and not a directory. It will copied to local-directory and
a configuration file '` + syncfiles.SyncConfigFile + `' will be updated to record 
the remote and local locations. In this way, all files that need 
syncing have to be registered.

When arguments are absent, all the registered files will be synced to
their respective locations. This will be a 2-way sync, in the sense
the new file is copied to the other location.

**Important**: Since this can cause data loss, test first with the
` + "`--dry-run` or the `--interactive`/`-i`" + ` flag.

    rclone syncfiles -i 

Note that files won't be changed if there were any errors at any point.

`,
	Run: func(command *cobra.Command, args []string) {
		cmd.CheckArgs(0, 2, command, args)
		ctx := context.Background()
		cmd.Run(true, true, command, func() error {
			err := syncfiles.LoadConfigFile()
			if err != nil {
				return err
			}
			if len(args) == 0 {
				// sync all the registered files
				err = syncfiles.SyncFiles(ctx)
			} else {
				// copy the given file and register it for future sync
				fsrc, srcFileName, fdst := cmd.NewFsSrcFileDst(args)

				// Source file must be given
				if srcFileName == "" {
					return fserrors.FatalError(errors.New("source filename should be specified"))
				}
				err = syncfiles.SyncFile(ctx, fsrc, fdst, srcFileName)
			}
			if err == nil {
				syncfiles.SaveConfigFile()
			}
			return nil
		})
	},
}
