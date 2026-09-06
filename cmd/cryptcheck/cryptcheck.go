// Package cryptcheck provides the cryptcheck command.
package cryptcheck

import (
	"context"
	"fmt"

	"github.com/rclone/rclone/backend/crypt"
	"github.com/rclone/rclone/cmd"
	"github.com/rclone/rclone/cmd/check"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/hash"
	"github.com/rclone/rclone/fs/operations"
	"github.com/spf13/cobra"
)

func init() {
	cmd.Root.AddCommand(commandDefinition)
	cmdFlag := commandDefinition.Flags()
	check.AddFlags(cmdFlag)
}

var commandDefinition = &cobra.Command{
	Use:   "cryptcheck remote:path cryptedremote:path",
	Short: `Cryptcheck checks the integrity of an encrypted remote.`,
	Long: `Checks a remote against an [encrypted](/crypt/) remote. This is the equivalent
of running rclone [check](/commands/rclone_check/), but able to check the
checksums of the encrypted remote.

For it to work the underlying remote of the cryptedremote must support
some kind of checksum.

It works by reading the nonce from each file on the cryptedremote: and
using that to encrypt each file on the remote:.  It then checks the
checksum of the underlying file on the cryptedremote: against the
checksum of the file it has just encrypted.

Use it like this

` + "```console" + `
rclone cryptcheck /path/to/files encryptedremote:path
` + "```" + `

You can use it like this also, but that will involve downloading all
the files in ` + "`remote:path`" + `.

` + "```console" + `
rclone cryptcheck remote:path encryptedremote:path
` + "```" + `

After it has run it will log the status of the ` + "`encryptedremote:`" + `.
` + check.FlagsHelp,
	Annotations: map[string]string{
		"versionIntroduced": "v1.36",
		"groups":            "Filter,Listing,Check",
	},
	Run: func(command *cobra.Command, args []string) {
		cmd.CheckArgs(2, 2, command, args)
		fsrc, fdst := cmd.NewFsSrcDst(args)
		cmd.Run(false, true, command, func() error {
			return cryptCheck(context.Background(), fdst, fsrc)
		})
	},
}

// cryptCheck checks the integrity of an encrypted remote
func cryptCheck(ctx context.Context, fdst, fsrc fs.Fs) error {
	fcrypt, ok := fdst.(*crypt.Fs)
	if !ok {
		return fmt.Errorf("%s:%s is not a crypt remote", fdst.Name(), fdst.Root())
	}
	funderlying := fcrypt.UnWrap()
	if funderlying.Hashes().GetOne() == hash.None {
		return fmt.Errorf("%s:%s does not support any hashes", funderlying.Name(), funderlying.Root())
	}

	opt, close, err := check.GetCheckOpt(fsrc, fdst)
	if err != nil {
		return err
	}
	defer close()

	_, err = operations.CryptCheck(ctx, opt)
	return err
}
