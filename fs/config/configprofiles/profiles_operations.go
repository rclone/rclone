// Package profiles handles presets for config
package profiles

import (
	"context"
	"errors"
	"fmt"
	"os"
	"slices"
	"strconv"
	"strings"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config"
	"github.com/rclone/rclone/fs/config/configfile"
	"github.com/rclone/rclone/fs/config/configflags"
	"github.com/rclone/rclone/fs/fspath"
	fslog "github.com/rclone/rclone/fs/log"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// AddProfiles handles saving/loading of config profiles
// We do this before Root.Execute() because there's otherwise
// no good way to add args to an already-executing command
func AddProfiles(ctx context.Context, Root *cobra.Command) {
	if !profileRequested() {
		return
	}
	cmd, remaining, err := Root.Find(os.Args[1:])
	if err != nil {
		fs.Fatalf(nil, "Fatal error: %v", err)
	}
	// Anything under `rclone config` (notably `config profile save`)
	// manages profiles directly and must not be intercepted here.
	for c := cmd; c != nil; c = c.Parent() {
		if c.Name() == "config" {
			return
		}
	}

	// Start the logger so debug/info messages from the profile code
	// reach the user; the normal cobra.OnInitialize path hasn't run
	// yet because Root.Execute() hasn't been called.
	fslog.InitLogging()

	if err := cmd.Flags().Parse(remaining); err != nil {
		fs.Fatalf(nil, "Fatal error: %v", err)
	}

	ci := fs.GetConfig(ctx)
	// The hand-rolled flags (--config, --cache-dir, -v, -q, ...) live
	// on pflag.CommandLine rather than on `cmd`, so parse it as well
	// to populate the package vars configflags.SetFlags reads from.
	if err := pflag.CommandLine.Parse(os.Args[1:]); err != nil {
		fs.Fatalf(nil, "Fatal error: %v", err)
	}
	// Populate ConfigInfo from the option-registry values (covers
	// every option-backed flag including --profile, --profile-save,
	// and env vars like RCLONE_PROFILE).
	if err := fs.GlobalOptionsInit(); err != nil {
		fs.Fatalf(nil, "Failed to initialise global options: %v", err)
	}
	// Apply the hand-rolled flags last - SetFlags writes directly to
	// ConfigInfo (e.g. ci.LogLevel from -vv) and would otherwise be
	// overwritten by GlobalOptionsInit reloading from the option
	// defaults.
	configflags.SetFlags(ci)
	// Install the config file handler *after* SetFlags has applied
	// --config (or RCLONE_CONFIG), otherwise the install is a no-op
	// against an empty path and any save goes to an in-memory
	// storage that's discarded at exit.
	configfile.Install()

	if err := HandleProfiles(ctx, cmd, cmd.Flags().Args()); err != nil {
		fs.Fatalf(nil, "Fatal error: %v", err)
	}
}

// HandleProfiles handles --profile-save and --profile
func HandleProfiles(ctx context.Context, cmd *cobra.Command, args []string) error {
	opt := ProfileOptFromCtx(ctx)

	if len(opt.UseProfile) > 0 {
		err := UseProfiles(ctx, cmd, args, &opt)
		if err != nil {
			return err
		}
	}

	if opt.SaveProfile != "" {
		saveProfile := NewProfile(opt.SaveProfile)
		saveProfile.NewFromCommand(cmd, args)
		return saveProfile.Save(ctx, &opt)
	}
	return nil
}

func argNum(key string) (int, error) {
	s := strings.TrimPrefix(key, argPrefix)
	i, err := strconv.Atoi(s)
	if err != nil {
		return i, err
	}
	return i - 1, nil
}

func optionToProfileKey(name string) string {
	name = strings.TrimSpace(name)
	name = strings.ReplaceAll(name, " ", "_")
	return strings.ToLower(strings.ReplaceAll(name, "-", "_"))
}

// note that this is case-insensitive
func profileKeyToFlag(name string) string {
	return strings.ToLower(strings.ReplaceAll(name, "_", "-"))
}

func validateKeyName(name string) (convertedName string, err error) {
	convertedName = fspath.MakeConfigName(optionToProfileKey(name))
	if convertedName != name {
		return convertedName, errors.New("invalid characters detected")
	}
	return convertedName, nil
}

func validateProfileName(name string) (convertedName string, err error) {
	// Profiles live in the "profile:" namespace so there is no
	// conflict possible with remote names - we only need to validate
	// the name's character set.
	return validateKeyName(name)
}

func logChange(useProfileName, key, oldval, newval string) {
	if oldval == newval {
		return
	}
	if oldval == "" {
		oldval = "[blank]"
	}
	fs.Infof(useProfileName, "changing %s from %s to %s", key, oldval, newval)
}

// SprintSection returns a string of this section from the config file
func SprintSection(name string) string {
	keys := config.LoadedData().GetKeyList(name)
	s := fmt.Sprintf("\n[%s]\n", name)
	for _, key := range keys {
		if value, ok := config.FileGetValue(name, key); ok {
			s += fmt.Sprintf("%s%s%s\n", key, " = ", value)
		}
	}
	return s
}

func cleanArgs(s []string) []string {
	var r []string
	// s = s[:cap(s)]
	for _, str := range s {
		if str != "" {
			r = append(r, str)
		}
	}
	return r
}

// profileRequested returns true when the user has asked for profile
// machinery to fire on this invocation, via either a CLI flag or an
// env var.
//
// We need this *before* cobra/options have done any of their work,
// so we look at the raw os.Args / os.Environ directly rather than
// reading from ConfigInfo (which is still populated from defaults
// at this point).
func profileRequested() bool {
	if os.Getenv("RCLONE_PROFILE") != "" || os.Getenv("RCLONE_PROFILE_SAVE") != "" {
		return true
	}
	return slices.ContainsFunc(os.Args, func(a string) bool {
		// Exact "--profile" / "--profile-save" and their "=value" forms.
		// Anything else (e.g. --profile-save-args, --profile-strict-flags)
		// is a modifier and does not on its own trigger the profile
		// machinery.
		return a == "--profile" || strings.HasPrefix(a, "--profile=") ||
			a == "--profile-save" || strings.HasPrefix(a, "--profile-save=")
	})
}
