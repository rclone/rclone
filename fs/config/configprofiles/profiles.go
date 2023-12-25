// Package profiles handles presets for config
package profiles

import (
	"context"
	"fmt"
	"os"
	"slices"
	"strings"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config"
	"github.com/rclone/rclone/fs/config/configflags"
	"github.com/rclone/rclone/fs/operations"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// ignoredFlags are flag names (in profile-key form, i.e. snake_case)
// that must never be persisted into a profile or copied out of one.
//
// "config" is excluded because applying it from a profile would
// re-point the config-file storage mid-Use(), while we're reading
// the profile out of that very file.
var ignoredFlags = []string{"profile", "profile_save", "profile_save_args", "profile_strict_flags", "flags_from", "config"}

const (
	argPrefix = "arg_"
	parentKey = "parent_profiles"
)

// ProfileOpt stores the settings for Profiles
type ProfileOpt struct {
	SaveProfile string          // name of profile to save
	UseProfile  fs.CommaSepList // name(s) of profiles to use
	StrictFlags bool            // If set, --profile will error if any flags are invalid for this command, instead of ignoring.
	SaveArgs    bool            // When saving, also include positional args (e.g. the paths being synced)
}

// Profile represents a flag/arg configuration to save/use as a reusable preset
type Profile struct {
	Name    string            // the name of the profile
	Args    []string          // command args (ex. the paths being synced)
	Flags   map[string]string // any flags (command, config, backend...)
	Parents []string          // see p.GetParents() to get a slice of *Profiles instead
}

// NewProfile returns a new empty *Profile
func NewProfile(name string) *Profile {
	return &Profile{
		Name:    name,
		Args:    []string{},
		Flags:   map[string]string{},
		Parents: []string{},
	}
}

// ProfileOptFromCtx gets a new ProfileOpt from config settings
func ProfileOptFromCtx(ctx context.Context) ProfileOpt {
	ci := fs.GetConfig(ctx)
	return ProfileOpt{
		SaveProfile: ci.SaveProfile,
		UseProfile:  ci.UseProfile,
		StrictFlags: ci.ProfileStrictFlags,
		SaveArgs:    ci.ProfileSaveArgs,
	}
}

// NewFromCommand sets the profile's Args and Flags from the cmd and args passed in
func (p *Profile) NewFromCommand(cmd *cobra.Command, args []string) {
	p.Args = args

	setProfileVal := func(flag *pflag.Flag) {
		key := optionToProfileKey(flag.Name)
		if slices.Contains(ignoredFlags, key) {
			return
		}
		if !flag.Changed {
			return
		}
		p.Flags[key] = flag.Value.String()
	}

	// .Visit() does not visit the flags we add manually via
	// --flags-from (in `rclone config profile save`), but .VisitAll()
	// with a flag.Changed check does, so use that.
	cmd.Flags().VisitAll(setProfileVal)
}

// sectionName returns the config-file section name for the profile
// "name" (e.g. "fast" -> "profile:fast"). Profile.Name itself stays
// in its user-facing form.
func sectionName(name string) string {
	return config.ProfileSectionPrefix + name
}

// GetProfile gets a profile from the config
func GetProfile(name string) (*Profile, error) {
	p := NewProfile(name)
	section := sectionName(p.Name)
	data := config.LoadedData()
	if !data.HasSection(section) {
		return nil, fmt.Errorf("no profile named %q found in config file", p.Name)
	}

	// Args may appear out of order or be sparse (e.g. arg_3 without
	// arg_1) so collect them first then size the slice from the max
	// index.
	args := map[int]string{}

	keys := data.GetKeyList(section)
	for _, key := range keys {
		if slices.Contains(ignoredFlags, key) {
			continue
		}

		if key == parentKey {
			parentList, _ := data.GetValue(section, key)
			p.Parents = strings.Split(parentList, ",")
			continue
		}

		if strings.HasPrefix(key, argPrefix) {
			i, err := argNum(key)
			if err != nil {
				return nil, fmt.Errorf("error parsing arg %s: %v", key, err)
			}
			if i < 0 {
				return nil, fmt.Errorf("arg index out of range: %s", key)
			}
			val, _ := data.GetValue(section, key)
			args[i] = val
			continue
		}

		val, _ := data.GetValue(section, key)
		p.Flags[key] = val
	}
	if len(args) > 0 {
		maxIdx := -1
		for i := range args {
			if i > maxIdx {
				maxIdx = i
			}
		}
		p.Args = make([]string, maxIdx+1)
		for i, v := range args {
			p.Args[i] = v
		}
	}
	return p, nil
}

// GetProfiles gets a slice of profiles
func GetProfiles(names []string) ([]*Profile, error) {
	profiles := []*Profile{}
	for _, name := range names {
		p, err := GetProfile(name)
		if err != nil {
			return nil, err
		}
		profiles = append(profiles, p)
	}
	return profiles, nil
}

// UseProfiles gets one or more Profiles from opt.UseProfile and applies them to context (priority: lowest to highest)
func UseProfiles(ctx context.Context, cmd *cobra.Command, args []string, opt *ProfileOpt) error {
	for _, profileName := range opt.UseProfile {
		err := GetAndUseProfile(ctx, cmd, args, opt, profileName)
		if err != nil {
			return err
		}
	}
	return nil
}

// GetAndUseProfile gets one Profile and uses it
func GetAndUseProfile(ctx context.Context, cmd *cobra.Command, args []string, opt *ProfileOpt, profileName string) error {
	profile, err := GetProfile(profileName)
	if err != nil {
		return err
	}

	err = profile.Use(ctx, cmd, args, opt)
	if err != nil {
		return err
	}
	return nil
}

// Save saves a profile to the config file
func (p *Profile) Save(ctx context.Context, opt *ProfileOpt) error {
	var err error
	p.Name, err = validateProfileName(p.Name)
	if err != nil {
		return err
	}

	if operations.SkipDestructive(ctx, p.Name, "save profile") {
		return nil
	}

	section := sectionName(p.Name)
	data := config.LoadedData()
	data.DeleteSection(section) // clear it if already exists

	for k, v := range p.Flags {
		// Defensive normalisation: callers building a *Profile
		// directly may pass keys in either form.
		data.SetValue(section, optionToProfileKey(k), v)
	}

	if opt.SaveArgs {
		p.saveArgs()
	}

	if len(p.Parents) > 0 {
		data.SetValue(section, parentKey, strings.Join(p.Parents, ","))
	}

	if err := data.Save(); err != nil {
		return fmt.Errorf("error saving config: %v", err)
	}

	fs.Debugf(p.Name, "saved profile: %v", SprintSection(sectionName(p.Name)))
	return nil
}

func (p *Profile) saveArgs() {
	section := sectionName(p.Name)
	for i, arg := range p.Args {
		config.LoadedData().SetValue(section, argPrefix+fmt.Sprint(i+1), arg)
	}
}

// Use sets the current command args and flags from the profile struct
func (p *Profile) Use(ctx context.Context, cmd *cobra.Command, cobraArgs []string, opt *ProfileOpt) error {
	// set logging first
	ci := fs.GetConfig(ctx)
	section := sectionName(p.Name)
	if ci.LogLevel == fs.LogLevelNotice {
		if verbose, ok := config.FileGetValue(section, "verbose"); ok {
			_ = cmd.Flags().Lookup("verbose").Value.Set(verbose)
			configflags.SetFlags(ci)
		}
	}
	fs.Debugf(p.Name, "loading profile: %v", SprintSection(section))

	err := p.useParents(ctx, cmd, cobraArgs, opt)
	if err != nil {
		return err
	}

	err = p.useFlags(ctx, cmd, cobraArgs, opt)
	if err != nil {
		return err
	}

	// Apply saved args whenever the profile carries them. Whether or
	// not args were saved is a per-profile decision made at save time
	// (--profile-save-args); the user shouldn't have to know or
	// re-pass that knob at use time.
	if len(p.Args) > 0 {
		p.useArgs(ctx, cmd, cobraArgs, opt)
	}

	// Copy any option-backed flag values we just changed (e.g.
	// --checkers, --transfers, --metadata, backend flags) out of the
	// pflag/options registry and into ConfigInfo.
	if err := fs.GlobalOptionsInit(); err != nil {
		return fmt.Errorf("failed to reload global options from profile: %w", err)
	}
	// Re-run configflags.SetFlags() for the hand-rolled flags it owns
	// (-v, -q, --bind, --headers, --config, --cache-dir...).
	configflags.SetFlags(ci)
	return nil
}

func (p *Profile) useFlags(ctx context.Context, cmd *cobra.Command, cobraArgs []string, opt *ProfileOpt) error {
	ci := fs.GetConfig(ctx)
	for rawKey, val := range p.Flags {
		key := optionToProfileKey(rawKey) // tolerate hyphenated keys from external callers
		if slices.Contains(ignoredFlags, key) {
			continue
		}

		flag := cmd.Flags().Lookup(profileKeyToFlag(key))
		if flag == nil {
			if opt.StrictFlags {
				return fmt.Errorf("invalid flag: %s", profileKeyToFlag(key))
			}
			fs.Debugf(p.Name, "ignoring unknown flag %s for this command", profileKeyToFlag(key))
			continue
		}

		if ci.DryRun && key == "dry_run" && val == "false" {
			// disallow overriding --dry-run if it was specifically set
			fs.Logf(nil, "for safety, profiles cannot change --dry-run from true to false. Ignoring.")
			continue
		}
		logChange(p.Name, key, flag.Value.String(), val)
		err := flag.Value.Set(val)
		if err != nil {
			return fmt.Errorf("%s: error setting val %s from profile: %v", flag.Name, val, err)
		}
	}
	return nil
}

// useArgs fills positional args from the profile's saved Args list,
// but only into slots the user did not provide on the command line.
// We don't CheckArgs() here as the command will do it later; users
// should take care to not exceed MaxArgs.
//
// Args on parent profiles are generally a bad idea (children would
// overwrite parent), but the same precedence rule applies if you
// use them.
func (p *Profile) useArgs(ctx context.Context, cmd *cobra.Command, cobraArgs []string, opt *ProfileOpt) {
	userProvided := len(cobraArgs)
	for i, pArg := range p.Args { // we DO assume that the p.Args are in the right order here.
		if pArg == "" {
			continue
		}
		if i < userProvided {
			// User gave an explicit positional arg for this slot - keep it.
			continue
		}
		if i > len(cobraArgs)-1 {
			cobraArgs = append(cobraArgs, "")
		}
		logChange(p.Name, fmt.Sprintf("Arg %d", i+1), cobraArgs[i], pArg)
		cobraArgs[i] = pArg
	}

	// rclone command [args...]
	os.Args = append([]string{os.Args[0], os.Args[1]}, cobraArgs...)
	os.Args = cleanArgs(os.Args)
}

// allows nested profiles (priority lowest to highest, parents all lower than child)
func (p *Profile) useParents(ctx context.Context, cmd *cobra.Command, args []string, opt *ProfileOpt) error {
	if len(p.Parents) == 0 {
		return nil
	}
	newopt := *opt
	newopt.UseProfile = p.Parents
	return UseProfiles(ctx, cmd, args, &newopt)
}

// GetParents returns the parent profiles of this profile
func (p *Profile) GetParents() ([]*Profile, error) {
	return GetProfiles(p.Parents)
}

/*
TODO:
- should we be storing more in memory to cut down on config.LoadedData() calls?
- when profile/key names have illegal characters should we error or auto-convert them? currently we do some of both.
- more tests
- docs
- document handling of blank or default values
- UI could use some cleanup (and see the note there about the use of the word "remote")
- rc methods
- API for other commands (bisync) to save/use profiles
- better handle scenario where logging level passed in is higher than that of the profile we're getting/using...
...probably needs a defer to make sure debugs are seen (if requested) during profile setting, then set the real level (from profile) after
- maybe edit more of the helper functions to use the types and methods instead of modifying config directly
- the ProfileOpt logic maybe needs another look -- slightly confusing that we store the profile name both there and in the profile struct.
- search other TODOs
*/
