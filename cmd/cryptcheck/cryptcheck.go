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

The encrypted remote can be given as either the source or the
destination, so this works too.

` + "```console" + `
rclone cryptcheck encryptedremote:path /path/to/files
` + "```" + `

If both remotes are encrypted then the destination is the one whose
checksums are checked.

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
	// Check to see which of fdst or fsrc is a crypt, preferring fdst
	fplain := fsrc
	fcrypt, ok := fdst.(*crypt.Fs)
	cryptIsSrc := false
	if !ok {
		fplain = fdst
		fcrypt, ok = fsrc.(*crypt.Fs)
		cryptIsSrc = true
	}
	if !ok {
		return fmt.Errorf("neither %s:%s nor %s:%s is a crypt remote", fsrc.Name(), fsrc.Root(), fdst.Name(), fdst.Root())
	}
	// Find a hash to use
	funderlying := fcrypt.UnWrap()
	hashType := funderlying.Hashes().GetOne()
	if hashType == hash.None {
		return fmt.Errorf("%s:%s does not support any hashes", funderlying.Name(), funderlying.Root())
	}
	fs.Infof(nil, "Using %v for hash comparisons", hashType)

	opt, close, err := check.GetCheckOpt(fsrc, fdst)
	if err != nil {
		return err
	}
	defer close()

	// checkIdentical checks to see if dst and src are identical
	//
	// it returns true if differences were found
	// it also returns whether it couldn't be hashed
	opt.Check = func(ctx context.Context, dst, src fs.Object) (differ bool, noHash bool, err error) {
		if cryptIsSrc {
			dst, src = src, dst
		}
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
			err = fmt.Errorf("hashes differ (%s:%s) %q vs (%s:%s) %q", fcrypt.Name(), fcrypt.Root(), cryptHash, fplain.Name(), fplain.Root(), underlyingHash)
			fs.Errorf(src, "%s", err.Error())
			return true, false, nil
		}
		return false, false, nil
	}

	return operations.CheckFn(ctx, opt)
}
