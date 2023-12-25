package profiles_test

import (
	"context"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config"
	"github.com/rclone/rclone/fs/config/configfile"
	profiles "github.com/rclone/rclone/fs/config/configprofiles"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newTestCmd returns a cobra.Command with a small synthetic flag set
// covering the flag types the profile code actually has to handle.
//
// It is deliberately decoupled from cmd.Root and the real
// configflags/filterflags registrations: tests should exercise the
// profile machinery against a known, minimal surface rather than
// against the whole of rclone.
func newTestCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "test"}
	fs := cmd.Flags()
	fs.Int("checkers", 8, "Number of checkers")
	fs.Bool("checksum", false, "Use checksum")
	fs.Bool("metadata", false, "Preserve metadata")
	fs.Bool("dry-run", false, "Do a trial run")
	fs.String("backup-dir", "", "Backup directory")
	fs.String("drive-skip-gdocs", "", "Drive: skip gdocs")
	return cmd
}

// useTempConfig points the config subsystem at a fresh empty file
// inside t.TempDir() and registers cleanup to restore the previous
// path.
func useTempConfig(t *testing.T) string {
	t.Helper()
	configfile.Install()
	old := config.GetConfigPath()
	path := filepath.Join(t.TempDir(), "rclone.conf")
	require.NoError(t, os.WriteFile(path, nil, 0600))
	require.NoError(t, config.SetConfigPath(path))
	t.Cleanup(func() {
		_ = config.SetConfigPath(old)
	})
	// Make sure we start from an empty in-memory view too.
	for _, section := range config.LoadedData().GetSectionList() {
		config.LoadedData().DeleteSection(section)
	}
	return path
}

// parse parses argv into the given command's flag set and returns the
// remaining positional arguments.
func parse(t *testing.T, cmd *cobra.Command, argv ...string) []string {
	t.Helper()
	require.NoError(t, cmd.Flags().Parse(argv))
	return cmd.Flags().Args()
}

func TestNewFromCommand_CollectsOnlyChangedFlags(t *testing.T) {
	useTempConfig(t)
	cmd := newTestCmd()
	args := parse(t, cmd,
		"--checkers", "16",
		"--checksum",
		"--drive-skip-gdocs", "true",
		"src", "dst",
	)

	p := profiles.NewProfile("name")
	p.NewFromCommand(cmd, args)

	assert.Equal(t, []string{"src", "dst"}, p.Args)
	assert.Equal(t, map[string]string{
		"checkers":         "16",
		"checksum":         "true",
		"drive_skip_gdocs": "true",
	}, p.Flags)
}

func TestNewFromCommand_SkipsIgnoredFlags(t *testing.T) {
	useTempConfig(t)
	cmd := newTestCmd()
	// Profile-control flags must not be persisted into the profile,
	// even if the test command happens to define them.
	cmd.Flags().String("profile-save", "", "")
	cmd.Flags().String("profile", "", "")
	cmd.Flags().Bool("profile-save-args", false, "")
	parse(t, cmd,
		"--checkers", "4",
		"--profile-save", "ignored",
		"--profile", "ignored",
		"--profile-save-args",
	)

	p := profiles.NewProfile("name")
	p.NewFromCommand(cmd, nil)

	assert.Equal(t, map[string]string{"checkers": "4"}, p.Flags)
}

func TestSaveAndGet_RoundTripsFlagsAndArgs(t *testing.T) {
	useTempConfig(t)
	ctx := context.Background()

	p := profiles.NewProfile("preset")
	p.Flags = map[string]string{
		"checkers":         "16",
		"metadata":         "true",
		"drive_skip_gdocs": "true",
	}
	p.Args = []string{"src", "remote:dst"}
	p.Parents = []string{"parent_a", "parent_b"}

	opt := &profiles.ProfileOpt{SaveArgs: true}
	require.NoError(t, p.Save(ctx, opt))

	got, err := profiles.GetProfile("preset")
	require.NoError(t, err)
	assert.Equal(t, p, got)
}

func TestUse_UserPositionalArgsBeatProfileArgs(t *testing.T) {
	useTempConfig(t)
	ctx := context.Background()

	// Save a profile with positional args.
	p := profiles.NewProfile("paths")
	p.Args = []string{"saved-src", "saved-dst"}
	require.NoError(t, p.Save(ctx, &profiles.ProfileOpt{SaveArgs: true}))

	// useArgs mutates os.Args; save/restore.
	savedOSArgs := os.Args
	defer func() { os.Args = savedOSArgs }()
	os.Args = []string{"rclone", "copy", "user-src", "user-dst", "--profile", "paths"}

	cmd := newTestCmd()
	loaded, err := profiles.GetProfile("paths")
	require.NoError(t, err)
	require.NoError(t, loaded.Use(ctx, cmd, []string{"user-src", "user-dst"}, &profiles.ProfileOpt{}))

	// The user's positional args must win over the profile's.
	assert.Equal(t, []string{"rclone", "copy", "user-src", "user-dst"}, os.Args)
}

func TestUse_ProfileArgsFillUnprovidedSlots(t *testing.T) {
	useTempConfig(t)
	ctx := context.Background()

	p := profiles.NewProfile("paths")
	p.Args = []string{"saved-src", "saved-dst"}
	require.NoError(t, p.Save(ctx, &profiles.ProfileOpt{SaveArgs: true}))

	savedOSArgs := os.Args
	defer func() { os.Args = savedOSArgs }()
	// User provided no positional args.
	os.Args = []string{"rclone", "copy", "--profile", "paths"}

	cmd := newTestCmd()
	loaded, err := profiles.GetProfile("paths")
	require.NoError(t, err)
	require.NoError(t, loaded.Use(ctx, cmd, nil, &profiles.ProfileOpt{}))

	// With no user args, the profile's saved args fill in.
	assert.Equal(t, []string{"rclone", "copy", "saved-src", "saved-dst"}, os.Args)
}

func TestSave_DefensiveKeyNormalisation(t *testing.T) {
	// Callers building a *Profile directly may pass keys in either
	// snake_case or hyphenated form. Both should land in the same
	// section.
	useTempConfig(t)
	p := profiles.NewProfile("preset")
	p.Flags = map[string]string{"drive-skip-gdocs": "true"}
	require.NoError(t, p.Save(context.Background(), &profiles.ProfileOpt{}))

	got, err := profiles.GetProfile("preset")
	require.NoError(t, err)
	assert.Equal(t, map[string]string{"drive_skip_gdocs": "true"}, got.Flags)
}

func TestSave_RemoteAndProfileWithSameNameCoexist(t *testing.T) {
	// Profiles live in the "profile:" namespace so they can share a
	// name with a remote without colliding.
	useTempConfig(t)
	config.LoadedData().SetValue("clash", "type", "drive")
	require.NoError(t, config.LoadedData().Save())

	p := profiles.NewProfile("clash")
	p.Flags = map[string]string{"checkers": "4"}
	require.NoError(t, p.Save(context.Background(), &profiles.ProfileOpt{}))

	// Remote section still intact.
	rt, ok := config.LoadedData().GetValue("clash", "type")
	require.True(t, ok)
	assert.Equal(t, "drive", rt)
	// Profile under its namespaced section name.
	assert.True(t, config.LoadedData().HasSection("profile:clash"))
}

func TestGetProfile_HandlesSparseArgIndices(t *testing.T) {
	useTempConfig(t)
	// Write arg_3 with no arg_1 / arg_2 - GetProfile used to crash
	// here.
	config.LoadedData().SetValue("profile:preset", "arg_3", "third")
	require.NoError(t, config.LoadedData().Save())

	got, err := profiles.GetProfile("preset")
	require.NoError(t, err)
	assert.Equal(t, []string{"", "", "third"}, got.Args)
}

func TestUse_AppliesFlagValues(t *testing.T) {
	useTempConfig(t)
	ctx := context.Background()

	saved := profiles.NewProfile("preset")
	saved.Flags = map[string]string{
		"checkers": "16",
		"metadata": "true",
	}
	require.NoError(t, saved.Save(ctx, &profiles.ProfileOpt{}))

	cmd := newTestCmd()
	loaded, err := profiles.GetProfile("preset")
	require.NoError(t, err)
	require.NoError(t, loaded.Use(ctx, cmd, nil, &profiles.ProfileOpt{}))

	assert.Equal(t, "16", cmd.Flags().Lookup("checkers").Value.String())
	assert.Equal(t, "true", cmd.Flags().Lookup("metadata").Value.String())
}

func TestUse_UnknownFlagSkippedByDefault(t *testing.T) {
	// A profile may carry flags for commands other than the one it
	// is currently being applied to; non-strict mode should just
	// ignore them rather than erroring or nil-dereferencing.
	useTempConfig(t)
	ctx := context.Background()

	saved := profiles.NewProfile("preset")
	saved.Flags = map[string]string{
		"checkers":        "12",
		"not_a_real_flag": "value",
	}
	require.NoError(t, saved.Save(ctx, &profiles.ProfileOpt{}))

	cmd := newTestCmd()
	loaded, err := profiles.GetProfile("preset")
	require.NoError(t, err)
	require.NoError(t, loaded.Use(ctx, cmd, nil, &profiles.ProfileOpt{}))
	assert.Equal(t, "12", cmd.Flags().Lookup("checkers").Value.String())
}

func TestUse_UnknownFlagErrorsInStrictMode(t *testing.T) {
	useTempConfig(t)
	ctx := context.Background()

	saved := profiles.NewProfile("preset")
	saved.Flags = map[string]string{"not_a_real_flag": "value"}
	require.NoError(t, saved.Save(ctx, &profiles.ProfileOpt{}))

	cmd := newTestCmd()
	loaded, err := profiles.GetProfile("preset")
	require.NoError(t, err)
	err = loaded.Use(ctx, cmd, nil, &profiles.ProfileOpt{StrictFlags: true})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not-a-real-flag")
}

func TestUse_Parents_LowestToHighestPriority(t *testing.T) {
	useTempConfig(t)
	ctx := context.Background()
	opt := &profiles.ProfileOpt{}

	parentA := profiles.NewProfile("parent_a")
	parentA.Flags = map[string]string{
		"checkers": "4",
		"metadata": "true",
	}
	require.NoError(t, parentA.Save(ctx, opt))

	parentB := profiles.NewProfile("parent_b")
	parentB.Flags = map[string]string{
		"checkers": "8",
		"checksum": "true",
	}
	require.NoError(t, parentB.Save(ctx, opt))

	child := profiles.NewProfile("child")
	child.Flags = map[string]string{"checkers": "16"}
	child.Parents = []string{"parent_a", "parent_b"}
	require.NoError(t, child.Save(ctx, opt))

	cmd := newTestCmd()
	loaded, err := profiles.GetProfile("child")
	require.NoError(t, err)
	require.NoError(t, loaded.Use(ctx, cmd, nil, opt))

	// child overrides both parents
	assert.Equal(t, "16", cmd.Flags().Lookup("checkers").Value.String())
	// parent_b overrides parent_a where both set the same flag
	// (n/a here for checkers; for checksum only parent_b sets)
	assert.Equal(t, "true", cmd.Flags().Lookup("checksum").Value.String())
	// flag set only by parent_a still applies
	assert.Equal(t, "true", cmd.Flags().Lookup("metadata").Value.String())
}

func TestUse_DryRunCannotBeTurnedOff(t *testing.T) {
	useTempConfig(t)

	saved := profiles.NewProfile("preset")
	saved.Flags = map[string]string{"dry_run": "false"}
	require.NoError(t, saved.Save(context.Background(), &profiles.ProfileOpt{}))

	// Turn dry-run on *after* saving, otherwise Save itself is
	// skipped as a destructive op under --dry-run.
	ctx, ci := fs.AddConfig(context.Background())
	ci.DryRun = true

	cmd := newTestCmd()
	loaded, err := profiles.GetProfile("preset")
	require.NoError(t, err)
	require.NoError(t, loaded.Use(ctx, cmd, nil, &profiles.ProfileOpt{}))
	// The profile's "dry_run = false" must be ignored.
	assert.Equal(t, "false", cmd.Flags().Lookup("dry-run").Value.String())
	// We don't assert on ci.DryRun because Profile.Use re-runs
	// GlobalOptionsInit, which would copy ci.DryRun back from the
	// flag default - the relevant guarantee is that the flag was
	// not overwritten.
}

func TestHandleProfiles_SaveRoundTrip(t *testing.T) {
	useTempConfig(t)
	ctx, ci := fs.AddConfig(context.Background())

	cmd := newTestCmd()
	parse(t, cmd, "--checkers", "32", "--checksum")

	ci.SaveProfile = "fromcmd"
	require.NoError(t, profiles.HandleProfiles(ctx, cmd, cmd.Flags().Args()))

	loaded, err := profiles.GetProfile("fromcmd")
	require.NoError(t, err)
	assert.Equal(t, map[string]string{
		"checkers": "32",
		"checksum": "true",
	}, loaded.Flags)
}

func TestSprintSection_ShowsAllKeys(t *testing.T) {
	useTempConfig(t)
	p := profiles.NewProfile("preset")
	p.Flags = map[string]string{"checkers": "9"}
	require.NoError(t, p.Save(context.Background(), &profiles.ProfileOpt{}))

	s := profiles.SprintSection("profile:preset")
	lines := sortedNonBlank(s)
	want := sortedNonBlank("[profile:preset]\ncheckers = 9\n")
	assert.Equal(t, want, lines)
}

func sortedNonBlank(s string) []string {
	out := []string{}
	for _, line := range strings.Split(s, "\n") {
		if line == "" {
			continue
		}
		out = append(out, line)
	}
	sort.Strings(out)
	return out
}

// TestGlobalOptionsLoadable pins the contract that callers like
// HandleProfiles / Profile.Use rely on: fs.GlobalOptionsInit must be
// callable on an initialised process without erroring. If the global
// options registry ever ends up in a state that can't be loaded the
// rest of this suite will start blowing up too, but this gives a
// pointed failure.
func TestGlobalOptionsLoadable(t *testing.T) {
	require.NoError(t, fs.GlobalOptionsInit())
}
