// Package cryptdecode provides the cryptdecode command.
package cryptdecode

import (
	"errors"
	"fmt"

	"github.com/rclone/rclone/backend/crypt"
	"github.com/rclone/rclone/cmd"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config/flags"
	"github.com/spf13/cobra"
)

// Options set by command line flags
var (
	Reverse = false
)

func init() {
	cmd.Root.AddCommand(commandDefinition)
	cmdFlags := commandDefinition.Flags()
	flags.BoolVarP(cmdFlags, &Reverse, "reverse", "", Reverse, "Reverse cryptdecode, encrypts filenames", "")
}

var commandDefinition = &cobra.Command{
	Use:   "cryptdecode encryptedremote: encryptedfilename",
	Short: `Cryptdecode returns unencrypted file names.`,
	Long: `Returns unencrypted file names when provided with a list of encrypted file
names. List limit is 10 items.

If you supply the ` + "`--reverse`" + ` flag, it will return encrypted file names.

use it like this

	rclone cryptdecode encryptedremote: encryptedfilename1 encryptedfilename2

	rclone cryptdecode --reverse encryptedremote: filename1 filename2

Another way to accomplish this is by using the ` + "`rclone backend encode` (or `decode`)" + ` command.
See the documentation on the [crypt](/crypt/) overlay for more info.
`,
	Annotations: map[string]string{
		"versionIntroduced": "v1.38",
	},
	Run: func(command *cobra.Command, args []string) {
		cmd.CheckArgs(2, 11, command, args)
		cmd.Run(false, false, command, func() error {
			fsInfo, _, _, config, err := fs.ConfigFs(args[0])
			if err != nil {
				return err
			}
			if fsInfo.Name != "crypt" {
				return errors.New("the remote needs to be of type \"crypt\"")
			}
			cipher, err := crypt.NewCipher(config)
			if err != nil {
				return err
			}
			if Reverse {
				return cryptEncode(cipher, args[1:])
			}
			return cryptDecode(cipher, args[1:])
		})
	},
}

// cryptDecode returns the unencrypted file name
func cryptDecode(cipher *crypt.Cipher, args []string) error {
	output := ""

	for _, encryptedFileName := range args {
		fileName, err := cipher.DecryptFileName(encryptedFileName)
		if err != nil {
			output += fmt.Sprintln(encryptedFileName, "\t", "Failed to decrypt")
		} else {
			output += fmt.Sprintln(encryptedFileName, "\t", fileName)
		}
	}

	fmt.Print(output)

	return nil
}

// cryptEncode returns the encrypted file name
func cryptEncode(cipher *crypt.Cipher, args []string) error {
	output := ""

	for _, fileName := range args {
		encryptedFileName := cipher.EncryptFileName(fileName)
		output += fmt.Sprintln(fileName, "\t", encryptedFileName)
	}

	fmt.Print(output)

	return nil
}
