package cryptdecode

import (
	"fmt"

	"github.com/ncw/rclone/cmd"
	"github.com/ncw/rclone/crypt"
	"github.com/ncw/rclone/fs"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

func init() {
	cmd.Root.AddCommand(commandDefinition)
}

var commandDefinition = &cobra.Command{
	Use:   "cryptdecode encryptedremote: encryptedfilename",
	Short: `Cryptdecode returns unencrypted file names.`,
	Long: `
rclone cryptdecode returns unencrypted file names when provided with
a list of encrypted file names. List limit is 10 items.

use it like this

	rclone cryptdecode encryptedremote: encryptedfilename1 encryptedfilename2
`,
	Run: func(command *cobra.Command, args []string) {
		cmd.CheckArgs(2, 11, command, args)
		fsrc := cmd.NewFsSrc(args)
		cmd.Run(false, false, command, func() error {
			return cryptDecode(fsrc, args[1:])
		})
	},
}

// cryptDecode returns the unencrypted file name
func cryptDecode(fsrc fs.Fs, args []string) error {
	// Check if fsrc is a crypt
	fcrypt, ok := fsrc.(*crypt.Fs)
	if !ok {
		return errors.Errorf("%s:%s is not a crypt remote", fsrc.Name(), fsrc.Root())
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
