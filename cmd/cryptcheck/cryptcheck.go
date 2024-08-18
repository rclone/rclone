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
	Long: `Checks a remote against a [crypted](/crypt/) remote. This is the equivalent
of running rclone [check](/commands/rclone_check/), but able to check the
checksums of the encrypted remote.

For it to work the underlying remote of the cryptedremote must support
some kind of checksum.

It works by reading the nonce from each file on the cryptedremote: and
using that to encrypt each file on the remote:.  It then checks the
checksum of the underlying file on the cryptedremote: against the
checksum of the file it has just encrypted.

Use it like this

    rclone cryptcheck /path/to/files encryptedremote:path

You can use it like this also, but that will involve downloading all
the files in remote:path.

    rclone cryptcheck remote:path encryptedremote:path

After it has run it will log the status of the encryptedremote:.
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
	// Check to see fcrypt is a crypt
	fcrypt, ok := fdst.(*crypt.Fs)
	if !ok {
		return fmt.Errorf("%s:%s is not a crypt remote", fdst.Name(), fdst.Root())
	}
	// Find a hash to use
	funderlying := fcrypt.UnWrap()
	hashType := funderlying.Hashes().GetOne()
	if hashType == hash.None {
		return fmt.Errorf("%s:%s does not support any hashes", funderlying.Name(), funderlying.Root())
	}
	fs.Infof(nil, "Using %v for hash comparisons", hashType)

	opt, close, err := check.GetCheckOpt(fsrc, fcrypt)
	if err != nil {
		return err
	}
	defer close()

	// checkIdentical checks to see if dst and src are identical
	//
	// it returns true if differences were found
	// it also returns whether it couldn't be hashed
	opt.Check = func(ctx context.Context, dst, src fs.Object) (differ bool, noHash bool, err error) {
		cryptDst := dst.(*crypt.Object)
		underlyingDst := cryptDst.UnWrap()
		underlyingHash, err := underlyingDst.Hash(ctx, hashType)
		if err != nil {
			return true, false, fmt.Errorf("error reading hash from underlying %v: %w", underlyingDst, err)
		}
		if underlyingHash == "" {
			return false, true, nil
		}
		cryptHash, err := fcrypt.ComputeHash(ctx, cryptDst, src, hashType)
		if err != nil {
			return true, false, fmt.Errorf("error computing hash: %w", err)
		}
		if cryptHash == "" {
			return false, true, nil
		}
		if cryptHash != underlyingHash {
			err = fmt.Errorf("hashes differ (%s:%s) %q vs (%s:%s) %q", fdst.Name(), fdst.Root(), cryptHash, fsrc.Name(), fsrc.Root(), underlyingHash)
			fs.Errorf(src, "%s", err.Error())
			return true, false, nil
		}
		return false, false, nil
	}

	return operations.CheckFn(ctx, opt)
}
