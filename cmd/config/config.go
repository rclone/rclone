package config

import (
	"github.com/ncw/rclone/cmd"
	"github.com/ncw/rclone/fs/config"
	"github.com/spf13/cobra"
)

func init() {
	cmd.Root.AddCommand(configCommand)
	configCommand.AddCommand(configEditCommand)
	configCommand.AddCommand(configFileCommand)
	configCommand.AddCommand(configShowCommand)
	configCommand.AddCommand(configDumpCommand)
	configCommand.AddCommand(configProvidersCommand)
	configCommand.AddCommand(configCreateCommand)
	configCommand.AddCommand(configUpdateCommand)
	configCommand.AddCommand(configDeleteCommand)
	configCommand.AddCommand(configPasswordCommand)
}

var configCommand = &cobra.Command{
	Use:   "config",
	Short: `Enter an interactive configuration session.`,
	Long: `Enter an interactive configuration session where you can setup new
remotes and manage existing ones. You may also set or remove a
password to protect your configuration.
`,
	Run: func(command *cobra.Command, args []string) {
		cmd.CheckArgs(0, 0, command, args)
		config.EditConfig()
	},
}

var configEditCommand = &cobra.Command{
	Use:   "edit",
	Short: configCommand.Short,
	Long:  configCommand.Long,
	Run:   configCommand.Run,
}

var configFileCommand = &cobra.Command{
	Use:   "file",
	Short: `Show path of configuration file in use.`,
	Run: func(command *cobra.Command, args []string) {
		cmd.CheckArgs(0, 0, command, args)
		config.ShowConfigLocation()
	},
}

var configShowCommand = &cobra.Command{
	Use:   "show [<remote>]",
	Short: `Print (decrypted) config file, or the config for a single remote.`,
	Run: func(command *cobra.Command, args []string) {
		cmd.CheckArgs(0, 1, command, args)
		if len(args) == 0 {
			config.ShowConfig()
		} else {
			config.ShowRemote(args[0])
		}
	},
}

var configDumpCommand = &cobra.Command{
	Use:   "dump",
	Short: `Dump the config file as JSON.`,
	RunE: func(command *cobra.Command, args []string) error {
		cmd.CheckArgs(0, 0, command, args)
		return config.Dump()
	},
}

var configProvidersCommand = &cobra.Command{
	Use:   "providers",
	Short: `List in JSON format all the providers and options.`,
	RunE: func(command *cobra.Command, args []string) error {
		cmd.CheckArgs(0, 0, command, args)
		return config.JSONListProviders()
	},
}

var configCreateCommand = &cobra.Command{
	Use:   "create <name> <type> [<key> <value>]*",
	Short: `Create a new remote with name, type and options.`,
	Long: `
Create a new remote of <name> with <type> and options.  The options
should be passed in in pairs of <key> <value>.

For example to make a swift remote of name myremote using auto config
you would do:

    rclone config create myremote swift env_auth true
`,
	RunE: func(command *cobra.Command, args []string) error {
		cmd.CheckArgs(2, 256, command, args)
		return config.CreateRemote(args[0], args[1], args[2:])
	},
}

var configUpdateCommand = &cobra.Command{
	Use:   "update <name> [<key> <value>]+",
	Short: `Update options in an existing remote.`,
	Long: `
Update an existing remote's options. The options should be passed in
in pairs of <key> <value>.

For example to update the env_auth field of a remote of name myremote you would do:

    rclone config update myremote swift env_auth true
`,
	RunE: func(command *cobra.Command, args []string) error {
		cmd.CheckArgs(3, 256, command, args)
		return config.UpdateRemote(args[0], args[1:])
	},
}

var configDeleteCommand = &cobra.Command{
	Use:   "delete <name>",
	Short: `Delete an existing remote <name>.`,
	Run: func(command *cobra.Command, args []string) {
		cmd.CheckArgs(1, 1, command, args)
		config.DeleteRemote(args[0])
	},
}

var configPasswordCommand = &cobra.Command{
	Use:   "password <name> [<key> <value>]+",
	Short: `Update password in an existing remote.`,
	Long: `
Update an existing remote's password. The password
should be passed in in pairs of <key> <value>.

For example to set password of a remote of name myremote you would do:

    rclone config password myremote fieldname mypassword
`,
	RunE: func(command *cobra.Command, args []string) error {
		cmd.CheckArgs(3, 256, command, args)
		return config.PasswordRemote(args[0], args[1:])
	},
}
