package cryptdecode

import (
	"fmt"

	"github.com/ncw/rclone/backend/crypt"
	"github.com/ncw/rclone/cmd"
	"github.com/ncw/rclone/fs"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

// Options set by command line flags
var (
	Reverse = false
)

func init() {
	cmd.Root.AddCommand(commandDefinition)
	flags := commandDefinition.Flags()
	fs.BoolVarP(flags, &Reverse, "reverse", "", Reverse, "Reverse cryptdecode, encrypts filenames")
}

var commandDefinition = &cobra.Command{
	Use:   "cryptdecode encryptedremote: encryptedfilename",
	Short: `Cryptdecode returns unencrypted file names.`,
	Long: `
rclone cryptdecode returns unencrypted file names when provided with
a list of encrypted file names. List limit is 10 items.

If you supply the --reverse flag, it will return encrypted file names.

use it like this

	rclone cryptdecode encryptedremote: encryptedfilename1 encryptedfilename2

	rclone cryptdecode --reverse encryptedremote: filename1 filename2
`,
	Run: func(command *cobra.Command, args []string) {
		cmd.CheckArgs(2, 11, command, args)
		fsrc := cmd.NewFsSrc(args)
		if Reverse {
			cmd.Run(false, false, command, func() error {
				return cryptEncode(fsrc, args[1:])
			})
		} else {
			cmd.Run(false, false, command, func() error {
				return cryptDecode(fsrc, args[1:])
			})
		}
	},
}

// Check if fsrc is a crypt
func assertCryptFs(fsrc fs.Fs) (*crypt.Fs, error) {
	fcrypt, ok := fsrc.(*crypt.Fs)
	if !ok {
		return nil, errors.Errorf("%s:%s is not a crypt remote", fsrc.Name(), fsrc.Root())
	}
	return fcrypt, nil
}

// cryptDecode returns the unencrypted file name
func cryptDecode(fsrc fs.Fs, args []string) error {
	fcrypt, err := assertCryptFs(fsrc)

	if err != nil {
		return err
	}

	output := ""

	for _, encryptedFileName := range args {
		fileName, err := fcrypt.DecryptFileName(encryptedFileName)
		if err != nil {
			output += fmt.Sprintln(encryptedFileName, "\t", "Failed to decrypt")
		} else {
			output += fmt.Sprintln(encryptedFileName, "\t", fileName)
		}
	}

	fmt.Printf(output)

	return nil
}

// cryptEncode returns the encrypted file name
func cryptEncode(fsrc fs.Fs, args []string) error {
	fcrypt, err := assertCryptFs(fsrc)

	if err != nil {
		return err
	}

	output := ""

	for _, fileName := range args {
		encryptedFileName := fcrypt.EncryptFileName(fileName)
		output += fmt.Sprintln(fileName, "\t", encryptedFileName)
	}

	fmt.Printf(output)

	return nil
}
