package config

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/pkg/errors"
	"github.com/rclone/rclone/cmd"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config"
	"github.com/rclone/rclone/fs/config/flags"
	"github.com/rclone/rclone/fs/rc"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

func init() {
	cmd.Root.AddCommand(configCommand)
	configCommand.AddCommand(configEditCommand)
	configCommand.AddCommand(configFileCommand)
	configCommand.AddCommand(configTouchCommand)
	configCommand.AddCommand(configShowCommand)
	configCommand.AddCommand(configDumpCommand)
	configCommand.AddCommand(configProvidersCommand)
	configCommand.AddCommand(configCreateCommand)
	configCommand.AddCommand(configUpdateCommand)
	configCommand.AddCommand(configDeleteCommand)
	configCommand.AddCommand(configPasswordCommand)
	configCommand.AddCommand(configReconnectCommand)
	configCommand.AddCommand(configDisconnectCommand)
	configCommand.AddCommand(configUserInfoCommand)
}

var configCommand = &cobra.Command{
	Use:   "config",
	Short: `Enter an interactive configuration session.`,
	Long: `Enter an interactive configuration session where you can setup new
remotes and manage existing ones. You may also set or remove a
password to protect your configuration.
`,
	RunE: func(command *cobra.Command, args []string) error {
		cmd.CheckArgs(0, 0, command, args)
		return config.EditConfig(context.Background())
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

var configTouchCommand = &cobra.Command{
	Use:   "touch",
	Short: `Ensure configuration file exists.`,
	Run: func(command *cobra.Command, args []string) {
		cmd.CheckArgs(0, 0, command, args)
		config.SaveConfig()
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
			name := strings.TrimRight(args[0], ":")
			config.ShowRemote(name)
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

var updateRemoteOpt config.UpdateRemoteOpt

var configPasswordHelp = strings.ReplaceAll(`
Note that if the config process would normally ask a question the
default is taken (unless |--non-interactive| is used).  Each time
that happens rclone will print or DEBUG a message saying how to
affect the value taken.

If any of the parameters passed is a password field, then rclone will
automatically obscure them if they aren't already obscured before
putting them in the config file.

**NB** If the password parameter is 22 characters or longer and
consists only of base64 characters then rclone can get confused about
whether the password is already obscured or not and put unobscured
passwords into the config file. If you want to be 100% certain that
the passwords get obscured then use the |--obscure| flag, or if you
are 100% certain you are already passing obscured passwords then use
|--no-obscure|.  You can also set obscured passwords using the
|rclone config password| command.

The flag |--non-interactive| is for use by applications that wish to
configure rclone themeselves, rather than using rclone's text based
configuration questions. If this flag is set, and rclone needs to ask
the user a question, a JSON blob will be returned with the question in
it.

This will look something like (some irrelevant detail removed):

|||
{
    "State": "*oauth-islocal,teamdrive,,",
    "Option": {
        "Name": "config_is_local",
        "Help": "Use auto config?\n * Say Y if not sure\n * Say N if you are working on a remote or headless machine\n",
        "Default": true,
        "Examples": [
            {
                "Value": "true",
                "Help": "Yes"
            },
            {
                "Value": "false",
                "Help": "No"
            }
        ],
        "Required": false,
        "IsPassword": false,
        "Type": "bool",
        "Exclusive": true,
    },
    "Error": "",
}
|||

The format of |Option| is the same as returned by |rclone config
providers|. The question should be asked to the user and returned to
rclone as the |--result| option along with the |--state| parameter.

The keys of |Option| are used as follows:

- |Name| - name of variable - show to user
- |Help| - help text. Hard wrapped at 80 chars. Any URLs should be clicky.
- |Default| - default value - return this if the user just wants the default.
- |Examples| - the user should be able to choose one of these
- |Required| - the value should be non-empty
- |IsPassword| - the value is a password and should be edited as such
- |Type| - type of value, eg |bool|, |string|, |int| and others
- |Exclusive| - if set no free-form entry allowed only the |Examples|
- Irrelevant keys |Provider|, |ShortOpt|, |Hide|, |NoPrefix|, |Advanced|

If |Error| is set then it should be shown to the user at the same
time as the question.

    rclone config update name --continue --state "*oauth-islocal,teamdrive,," --result "true"

Note that when using |--continue| all passwords should be passed in
the clear (not obscured). Any default config values should be passed
in with each invocation of |--continue|.

At the end of the non interactive process, rclone will return a result
with |State| as empty string.

If |--all| is passed then rclone will ask all the config questions,
not just the post config questions. Any parameters are used as
defaults for questions as usual.

Note that |bin/config.py| in the rclone source implements this protocol
as a readable demonstration.
`, "|", "`")
var configCreateCommand = &cobra.Command{
	Use:   "create `name` `type` [`key` `value`]*",
	Short: `Create a new remote with name, type and options.`,
	Long: strings.ReplaceAll(`
Create a new remote of |name| with |type| and options.  The options
should be passed in pairs of |key| |value| or as |key=value|.

For example to make a swift remote of name myremote using auto config
you would do:

    rclone config create myremote swift env_auth true
    rclone config create myremote swift env_auth=true

So for example if you wanted to configure a Google Drive remote but
using remote authorization you would do this:

    rclone config create mydrive drive config_is_local=false
`, "|", "`") + configPasswordHelp,
	RunE: func(command *cobra.Command, args []string) error {
		cmd.CheckArgs(2, 256, command, args)
		in, err := argsToMap(args[2:])
		if err != nil {
			return err
		}
		return doConfig(args[0], in, func(opts config.UpdateRemoteOpt) (*fs.ConfigOut, error) {
			return config.CreateRemote(context.Background(), args[0], args[1], in, opts)
		})
	},
}

func doConfig(name string, in rc.Params, do func(config.UpdateRemoteOpt) (*fs.ConfigOut, error)) error {
	out, err := do(updateRemoteOpt)
	if err != nil {
		return err
	}
	if !(updateRemoteOpt.NonInteractive || updateRemoteOpt.Continue) {
		config.ShowRemote(name)
	} else {
		if out == nil {
			out = &fs.ConfigOut{}
		}
		outBytes, err := json.MarshalIndent(out, "", "\t")
		if err != nil {
			return err
		}
		_, _ = os.Stdout.Write(outBytes)
		_, _ = os.Stdout.WriteString("\n")
	}
	return nil
}

func init() {
	for _, cmdFlags := range []*pflag.FlagSet{configCreateCommand.Flags(), configUpdateCommand.Flags()} {
		flags.BoolVarP(cmdFlags, &updateRemoteOpt.Obscure, "obscure", "", false, "Force any passwords to be obscured.")
		flags.BoolVarP(cmdFlags, &updateRemoteOpt.NoObscure, "no-obscure", "", false, "Force any passwords not to be obscured.")
		flags.BoolVarP(cmdFlags, &updateRemoteOpt.NonInteractive, "non-interactive", "", false, "Don't interact with user and return questions.")
		flags.BoolVarP(cmdFlags, &updateRemoteOpt.Continue, "continue", "", false, "Continue the configuration process with an answer.")
		flags.BoolVarP(cmdFlags, &updateRemoteOpt.All, "all", "", false, "Ask the full set of config questions.")
		flags.StringVarP(cmdFlags, &updateRemoteOpt.State, "state", "", "", "State - use with --continue.")
		flags.StringVarP(cmdFlags, &updateRemoteOpt.Result, "result", "", "", "Result - use with --continue.")
	}
}

var configUpdateCommand = &cobra.Command{
	Use:   "update `name` [`key` `value`]+",
	Short: `Update options in an existing remote.`,
	Long: strings.ReplaceAll(`
Update an existing remote's options. The options should be passed in
pairs of |key| |value| or as |key=value|.

For example to update the env_auth field of a remote of name myremote
you would do:

    rclone config update myremote env_auth true
    rclone config update myremote env_auth=true

If the remote uses OAuth the token will be updated, if you don't
require this add an extra parameter thus:

    rclone config update myremote env_auth=true config_refresh_token=false
`, "|", "`") + configPasswordHelp,
	RunE: func(command *cobra.Command, args []string) error {
		cmd.CheckArgs(1, 256, command, args)
		in, err := argsToMap(args[1:])
		if err != nil {
			return err
		}
		return doConfig(args[0], in, func(opts config.UpdateRemoteOpt) (*fs.ConfigOut, error) {
			return config.UpdateRemote(context.Background(), args[0], in, opts)
		})
	},
}

var configDeleteCommand = &cobra.Command{
	Use:   "delete `name`",
	Short: "Delete an existing remote `name`.",
	Run: func(command *cobra.Command, args []string) {
		cmd.CheckArgs(1, 1, command, args)
		config.DeleteRemote(args[0])
	},
}

var configPasswordCommand = &cobra.Command{
	Use:   "password `name` [`key` `value`]+",
	Short: `Update password in an existing remote.`,
	Long: strings.ReplaceAll(`
Update an existing remote's password. The password
should be passed in pairs of |key| |password| or as |key=password|.
The |password| should be passed in in clear (unobscured).

For example to set password of a remote of name myremote you would do:

    rclone config password myremote fieldname mypassword
    rclone config password myremote fieldname=mypassword

This command is obsolete now that "config update" and "config create"
both support obscuring passwords directly.
`, "|", "`"),
	RunE: func(command *cobra.Command, args []string) error {
		cmd.CheckArgs(1, 256, command, args)
		in, err := argsToMap(args[1:])
		if err != nil {
			return err
		}
		err = config.PasswordRemote(context.Background(), args[0], in)
		if err != nil {
			return err
		}
		config.ShowRemote(args[0])
		return nil
	},
}

// This takes a list of arguments in key value key value form, or
// key=value key=value form and converts it into a map
func argsToMap(args []string) (out rc.Params, err error) {
	out = rc.Params{}
	for i := 0; i < len(args); i++ {
		key := args[i]
		equals := strings.IndexRune(key, '=')
		var value string
		if equals >= 0 {
			key, value = key[:equals], key[equals+1:]
		} else {
			i++
			if i >= len(args) {
				return nil, errors.New("found key without value")
			}
			value = args[i]
		}
		out[key] = value
	}
	return out, nil
}

var configReconnectCommand = &cobra.Command{
	Use:   "reconnect remote:",
	Short: `Re-authenticates user with remote.`,
	Long: `
This reconnects remote: passed in to the cloud storage system.

To disconnect the remote use "rclone config disconnect".

This normally means going through the interactive oauth flow again.
`,
	RunE: func(command *cobra.Command, args []string) error {
		ctx := context.Background()
		cmd.CheckArgs(1, 1, command, args)
		fsInfo, configName, _, m, err := fs.ConfigFs(args[0])
		if err != nil {
			return err
		}
		return config.PostConfig(ctx, configName, m, fsInfo)
	},
}

var configDisconnectCommand = &cobra.Command{
	Use:   "disconnect remote:",
	Short: `Disconnects user from remote`,
	Long: `
This disconnects the remote: passed in to the cloud storage system.

This normally means revoking the oauth token.

To reconnect use "rclone config reconnect".
`,
	RunE: func(command *cobra.Command, args []string) error {
		cmd.CheckArgs(1, 1, command, args)
		f := cmd.NewFsSrc(args)
		doDisconnect := f.Features().Disconnect
		if doDisconnect == nil {
			return errors.Errorf("%v doesn't support Disconnect", f)
		}
		err := doDisconnect(context.Background())
		if err != nil {
			return errors.Wrap(err, "Disconnect call failed")
		}
		return nil
	},
}

var (
	jsonOutput bool
)

func init() {
	flags.BoolVarP(configUserInfoCommand.Flags(), &jsonOutput, "json", "", false, "Format output as JSON")
}

var configUserInfoCommand = &cobra.Command{
	Use:   "userinfo remote:",
	Short: `Prints info about logged in user of remote.`,
	Long: `
This prints the details of the person logged in to the cloud storage
system.
`,
	RunE: func(command *cobra.Command, args []string) error {
		cmd.CheckArgs(1, 1, command, args)
		f := cmd.NewFsSrc(args)
		doUserInfo := f.Features().UserInfo
		if doUserInfo == nil {
			return errors.Errorf("%v doesn't support UserInfo", f)
		}
		u, err := doUserInfo(context.Background())
		if err != nil {
			return errors.Wrap(err, "UserInfo call failed")
		}
		if jsonOutput {
			out := json.NewEncoder(os.Stdout)
			out.SetIndent("", "\t")
			return out.Encode(u)
		}
		var keys []string
		var maxKeyLen int
		for key := range u {
			keys = append(keys, key)
			if len(key) > maxKeyLen {
				maxKeyLen = len(key)
			}
		}
		sort.Strings(keys)
		for _, key := range keys {
			fmt.Printf("%*s: %s\n", maxKeyLen, key, u[key])
		}
		return nil
	},
}
