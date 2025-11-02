// Package obscure provides the obscure command.
package obscure

import (
	"bufio"
	"fmt"

	"os"

	"github.com/rclone/rclone/cmd"
	"github.com/rclone/rclone/fs/config/obscure"
	"github.com/spf13/cobra"
)

func init() {
	cmd.Root.AddCommand(commandDefinition)
}

var commandDefinition = &cobra.Command{
	Use:   "obscure password",
	Short: `Obscure password for use in the rclone config file.`,
	Long: `In the rclone config file, human-readable passwords are
obscured. Obscuring them is done by encrypting them and writing them
out in base64. This is **not** a secure way of encrypting these
passwords as rclone can decrypt them - it is to prevent "eyedropping" -
namely someone seeing a password in the rclone config file by accident.

Many equally important things (like access tokens) are not obscured in
the config file. However it is very hard to shoulder surf a 64
character hex token.

This command can also accept a password through STDIN instead of an
argument by passing a hyphen as an argument. This will use the first
line of STDIN as the password not including the trailing newline.

` + "```console" + `
echo "secretpassword" | rclone obscure -
` + "```" + `

If there is no data on STDIN to read, rclone obscure will default to
obfuscating the hyphen itself.

If you want to encrypt the config file then please use config file
encryption - see [rclone config](/commands/rclone_config/) for more
info.`,
	Annotations: map[string]string{
		"versionIntroduced": "v1.36",
	},
	RunE: func(command *cobra.Command, args []string) error {
		cmd.CheckArgs(1, 1, command, args)
		var password string
		fi, _ := os.Stdin.Stat()
		if args[0] == "-" && (fi.Mode()&os.ModeCharDevice) == 0 {
			scanner := bufio.NewScanner(os.Stdin)
			if scanner.Scan() {
				password = scanner.Text()
			}
			if err := scanner.Err(); err != nil {
				return err
			}
		} else {
			password = args[0]
		}
		cmd.Run(false, false, command, func() error {
			obscured := obscure.MustObscure(password)
			fmt.Println(obscured)
			return nil
		})
		return nil
	},
}
