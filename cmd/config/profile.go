// Profile subcommands for `rclone config profile`.

package config

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/rclone/rclone/cmd"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config"
	profiles "github.com/rclone/rclone/fs/config/configprofiles"
	"github.com/rclone/rclone/fs/config/flags"
	"github.com/spf13/cobra"
)

func init() {
	configProfileCommand.AddCommand(configProfileListCommand)
	configProfileCommand.AddCommand(configProfileShowCommand)
	configProfileCommand.AddCommand(configProfileDeleteCommand)
	configProfileCommand.AddCommand(configProfileSaveCommand)

	flags.FVarP(configProfileSaveCommand.Flags(), &profileSaveFlagsFrom,
		"flags-from", "",
		"Import command-specific flags from these subcommands (comma separated)", "")
	configProfileSaveCommand.PersistentPreRunE = profileSaveAddFlagsFrom
	configProfileSaveCommand.FParseErrWhitelist.UnknownFlags = true
}

// profileSaveFlagsFrom backs --flags-from on `rclone config profile save`.
var profileSaveFlagsFrom fs.CommaSepList

var configProfileListCommand = &cobra.Command{
	Use:   "list",
	Short: "List the profiles in the config file.",
	Annotations: map[string]string{
		"versionIntroduced": "v1.72",
	},
	RunE: func(command *cobra.Command, args []string) error {
		cmd.CheckArgs(0, 0, command, args)
		profiles.ShowProfiles()
		return nil
	},
}

var configProfileShowCommand = &cobra.Command{
	Use:   "show NAME",
	Short: "Show the flags saved in a profile.",
	Annotations: map[string]string{
		"versionIntroduced": "v1.72",
	},
	RunE: func(command *cobra.Command, args []string) error {
		cmd.CheckArgs(1, 1, command, args)
		name := args[0]
		section := config.ProfileSectionPrefix + name
		if !config.LoadedData().HasSection(section) {
			return fmt.Errorf("no profile named %q", name)
		}
		fmt.Print(profiles.SprintSection(section))
		return nil
	},
}

var configProfileDeleteCommand = &cobra.Command{
	Use:   "delete NAME",
	Short: "Delete a profile.",
	Annotations: map[string]string{
		"versionIntroduced": "v1.72",
	},
	RunE: func(command *cobra.Command, args []string) error {
		cmd.CheckArgs(1, 1, command, args)
		name := args[0]
		section := config.ProfileSectionPrefix + name
		if !config.LoadedData().HasSection(section) {
			return fmt.Errorf("no profile named %q", name)
		}
		config.LoadedData().DeleteSection(section)
		if err := config.LoadedData().Save(); err != nil {
			return fmt.Errorf("failed to save config: %w", err)
		}
		return nil
	},
}

var configProfileSaveCommand = &cobra.Command{
	Use:   "save NAME [args...]",
	Short: "Save the given flags (and optional args) as a reusable profile.",
	Long: `Save a profile under NAME built from the flags (and, if
` + "`--profile-save-args`" + ` is set, the positional arguments) passed
on this command line.

Without ` + "`--flags-from`" + ` only global rclone flags can be saved. To
save flags that belong to a specific command (e.g. VFS flags from
` + "`rclone mount`" + ` or bisync-specific flags), import that command's
flag set with ` + "`--flags-from CMD[,CMD...]`" + `.
`,
	Annotations: map[string]string{
		"versionIntroduced": "v1.72",
	},
	RunE: func(command *cobra.Command, args []string) error {
		cmd.CheckArgs(1, 100, command, args)
		name := args[0]
		ctx := context.Background()
		opt := profiles.ProfileOptFromCtx(ctx)
		p := profiles.NewProfile(name)
		p.NewFromCommand(command, args[1:])
		return p.Save(ctx, &opt)
	},
}

// profileSaveAddFlagsFrom is the PersistentPreRunE for
// `rclone config profile save`. It interprets --flags-from, merges in
// the requested commands' flag sets, then re-parses the command line so
// the newly-added flags pick up their values.
func profileSaveAddFlagsFrom(saveCmd *cobra.Command, args []string) error {
	// The first parse was done with UnknownFlags=true so --verbose may
	// have been counted; preserve its current value across the reparse.
	var verboseSaved string
	if vf := saveCmd.Flags().Lookup("verbose"); vf != nil {
		verboseSaved = vf.Value.String()
		defer func() {
			if err := vf.Value.Set(verboseSaved); err != nil {
				fs.Errorf(nil, "error restoring --verbose: %v", err)
			}
		}()
	}

	for _, sub := range profileSaveFlagsFrom {
		sc, _, err := cmd.Root.Find([]string{sub})
		if err != nil {
			return fmt.Errorf("--flags-from %s: %w", sub, err)
		}
		if sc == cmd.Root {
			return fmt.Errorf("--flags-from %s: not a known subcommand", sub)
		}
		saveCmd.Flags().AddFlagSet(sc.Flags())
	}

	// Re-parse strictly now that the flag set is complete.
	saveCmd.FParseErrWhitelist.UnknownFlags = false
	flagArgs, err := argsAfterCommand(saveCmd)
	if err != nil {
		return err
	}
	return saveCmd.ParseFlags(flagArgs)
}

// argsAfterCommand returns the portion of os.Args that came after the
// chain of cobra command names leading to cmd.
func argsAfterCommand(cmd *cobra.Command) ([]string, error) {
	depth := 0
	for c := cmd; c != nil; c = c.Parent() {
		depth++
	}
	if depth > len(os.Args) {
		return nil, errors.New("internal error: command depth exceeds os.Args length")
	}
	return os.Args[depth:], nil
}
