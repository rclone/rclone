package hashsum

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/rclone/rclone/cmd"
	"github.com/rclone/rclone/fs/config/flags"
	"github.com/rclone/rclone/fs/hash"
	"github.com/rclone/rclone/fs/operations"
	"github.com/spf13/cobra"
)

var (
	outputBase64 = false
)

func init() {
	cmd.Root.AddCommand(commandDefinition)
	cmdFlags := commandDefinition.Flags()
	flags.BoolVarP(cmdFlags, &outputBase64, "base64", "", outputBase64, "Output base64 encoded hashsum")
}

var commandDefinition = &cobra.Command{
	Use:   "hashsum <hash> remote:path",
	Short: `Produces a hashsum file for all the objects in the path.`,
	Long: `
Produces a hash file for all the objects in the path using the hash
named.  The output is in the same format as the standard
md5sum/sha1sum tool.

Run without a hash to see the list of supported hashes, eg

    $ rclone hashsum
    Supported hashes are:
      * MD5
      * SHA-1
      * DropboxHash
      * QuickXorHash

Then

    $ rclone hashsum MD5 remote:path
`,
	RunE: func(command *cobra.Command, args []string) error {
		cmd.CheckArgs(0, 2, command, args)
		if len(args) == 0 {
			fmt.Printf("Supported hashes are:\n")
			for _, ht := range hash.Supported().Array() {
				fmt.Printf("  * %v\n", ht)
			}
			return nil
		} else if len(args) == 1 {
			return errors.New("need hash type and remote")
		}
		var ht hash.Type
		err := ht.Set(args[0])
		if err != nil {
			return err
		}
		fsrc := cmd.NewFsSrc(args[1:])
		cmd.Run(false, false, command, func() error {
			if outputBase64 {
				return operations.HashListerBase64(context.Background(), ht, fsrc, os.Stdout)
			}
			return operations.HashLister(context.Background(), ht, fsrc, os.Stdout)
		})
		return nil
	},
}
