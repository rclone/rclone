package cryptcheck

import (
	"github.com/ncw/rclone/cmd"
	"github.com/ncw/rclone/crypt"
	"github.com/ncw/rclone/fs"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

func init() {
	cmd.Root.AddCommand(commandDefintion)
}

var commandDefintion = &cobra.Command{
	Use:   "cryptcheck remote:path cryptedremote:path",
	Short: `Cryptcheck checks the integrity of a crypted remote.`,
	Long: `
rclone cryptcheck checks a remote against a crypted remote.  This is
the equivalent of running rclone check, but able to check the
checksums of the crypted remote.

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
`,
	Run: func(command *cobra.Command, args []string) {
		cmd.CheckArgs(2, 2, command, args)
		fsrc, fdst := cmd.NewFsSrcDst(args)
		cmd.Run(false, true, command, func() error {
			return cryptCheck(fdst, fsrc)
		})
	},
}

// cryptCheck checks the integrity of a crypted remote
func cryptCheck(fdst, fsrc fs.Fs) error {
	// Check to see fcrypt is a crypt
	fcrypt, ok := fdst.(*crypt.Fs)
	if !ok {
		return errors.Errorf("%s:%s is not a crypt remote", fdst.Name(), fdst.Root())
	}
	// Find a hash to use
	funderlying := fcrypt.UnWrap()
	hashType := funderlying.Hashes().GetOne()
	if hashType == fs.HashNone {
		return errors.Errorf("%s:%s does not support any hashes", funderlying.Name(), funderlying.Root())
	}
	fs.Infof(nil, "Using %v for hash comparisons", hashType)

	// checkIdentical checks to see if dst and src are identical
	//
	// it returns true if differences were found
	// it also returns whether it couldn't be hashed
	checkIdentical := func(dst, src fs.Object) (differ bool, noHash bool) {
		cryptDst := dst.(*crypt.Object)
		underlyingDst := cryptDst.UnWrap()
		underlyingHash, err := underlyingDst.Hash(hashType)
		if err != nil {
			fs.Stats.Error()
			fs.Errorf(dst, "Error reading hash from underlying %v: %v", underlyingDst, err)
			return true, false
		}
		if underlyingHash == "" {
			return false, true
		}
		cryptHash, err := fcrypt.ComputeHash(cryptDst, src, hashType)
		if err != nil {
			fs.Stats.Error()
			fs.Errorf(dst, "Error computing hash: %v", err)
			return true, false
		}
		if cryptHash == "" {
			return false, true
		}
		if cryptHash != underlyingHash {
			fs.Stats.Error()
			fs.Errorf(src, "hashes differ (%s:%s) %q vs (%s:%s) %q", fdst.Name(), fdst.Root(), cryptHash, fsrc.Name(), fsrc.Root(), underlyingHash)
			return true, false
		}
		fs.Debugf(src, "OK")
		return false, false
	}

	return fs.CheckFn(fcrypt, fsrc, checkIdentical)
}
