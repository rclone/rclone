package gitannex

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/lib/buildinfo"
)

// checkRcloneBinaryVersion runs whichever rclone is on the PATH and checks
// whether it reports a version that matches the test's expectations. Returns
// nil when the version is the expected version, otherwise returns an error.
func checkRcloneBinaryVersion() error {
	// versionInfo is a subset of information produced by "core/version".
	type versionInfo struct {
		Version string
		IsGit   bool
		GoTags  string
	}

	cmd := exec.Command("rclone", "rc", "--loopback", "core/version")
	stdout, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("failed to get rclone version: %w", err)
	}

	var parsed versionInfo
	if err := json.Unmarshal(stdout, &parsed); err != nil {
		return fmt.Errorf("failed to parse rclone version: %w", err)
	}
	if parsed.Version != fs.Version {
		return fmt.Errorf("expected version %q, but got %q", fs.Version, parsed.Version)
	}
	if !parsed.IsGit {
		return errors.New("expected rclone to be a dev build")
	}
	_, tagString := buildinfo.GetLinkingAndTags()
	if parsed.GoTags != tagString {
		return fmt.Errorf("expected tag string %q, but got %q", tagString, parsed.GoTags)
	}
	return nil
}

// This end-to-end test runs `git annex testremote` in a temporary git repo.
// This test will be skipped unless the `rclone` binary on PATH reports the
// expected version.
//
// When run on CI, an rclone binary built from HEAD will be on the PATH. When
// running locally, you will likely need to ensure the current binary is on the
// PATH like so:
//
//	go build && PATH="$(realpath .):$PATH" go test -v ./cmd/gitannex/...
//
// In the future, this test will probably be extended to test a number of
// parameters like repo layouts, and runtime may suffer from a combinatorial
// explosion.
func TestEndToEnd(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping due to short mode.")
	}

	// TODO: Support this test on Windows. Need to evaluate the semantics of the
	// HOME and PATH environment variables.
	switch runtime.GOOS {
	case "darwin",
		"freebsd",
		"linux",
		"netbsd",
		"openbsd",
		"plan9",
		"solaris":
	default:
		t.Skipf("GOOS %q is not supported.", runtime.GOOS)
	}

	if err := checkRcloneBinaryVersion(); err != nil {
		t.Skipf("Skipping due to rclone version: %s", err)
	}

	if _, err := exec.LookPath("git-annex"); err != nil {
		t.Skipf("Skipping because git-annex was not found: %s", err)
	}

	// Create a temp directory and chdir there, just in case.
	originalWd, err := os.Getwd()
	require.NoError(t, err)
	tempDir := t.TempDir()
	require.NoError(t, os.Chdir(tempDir))
	defer func() { require.NoError(t, os.Chdir(originalWd)) }()

	// Flesh out subdirectories of the temp directory:
	//
	//  .
	//  |-- bin
	//  |   `-- git-annex-remote-rclone-builtin -> ${PATH_TO_RCLONE_BINARY}
	//  |-- ephemeralRepo
	//  `-- user
	//  	`-- .config
	//  		`-- rclone
	//  			`-- rclone.conf

	binDir := filepath.Join(tempDir, "bin")
	homeDir := filepath.Join(tempDir, "user")
	configDir := filepath.Join(homeDir, ".config")
	rcloneConfigDir := filepath.Join(configDir, "rclone")
	ephemeralRepoDir := filepath.Join(tempDir, "ephemeralRepo")
	for _, dir := range []string{binDir, homeDir, configDir, rcloneConfigDir, ephemeralRepoDir} {
		require.NoError(t, os.Mkdir(dir, 0700))
	}

	// Install the symlink that enables git-annex to invoke "rclone gitannex"
	// without explicitly specifying the subcommand.
	rcloneBinaryPath, err := exec.LookPath("rclone")
	require.NoError(t, err)
	require.NoError(t, os.Symlink(
		rcloneBinaryPath,
		filepath.Join(binDir, "git-annex-remote-rclone-builtin")))

	// Install the rclone.conf file that defines the remote.
	rcloneConfigPath := filepath.Join(rcloneConfigDir, "rclone.conf")
	rcloneConfigContents := "[MyRcloneRemote]\ntype = local"
	require.NoError(t, os.WriteFile(rcloneConfigPath, []byte(rcloneConfigContents), 0600))

	// NOTE: These commands must be run with HOME pointing at an ephemeral
	// directory, rather than the real home directory.
	cmds := [][]string{
		{"git", "annex", "version"},
		{"git", "config", "--global", "user.name", "User Name"},
		{"git", "config", "--global", "user.email", "user@example.com"},
		{"git", "init"},
		{"git", "annex", "init"},
		{"git", "annex", "initremote", "MyTestRemote",
			"type=external", "externaltype=rclone-builtin", "encryption=none",
			"rcloneremotename=MyRcloneRemote", "rcloneprefix=" + ephemeralRepoDir},
		{"git", "annex", "testremote", "MyTestRemote"},
	}

	for _, args := range cmds {
		fmt.Printf("+ %v\n", args)
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = ephemeralRepoDir
		cmd.Env = []string{
			"HOME=" + homeDir,
			"PATH=" + os.Getenv("PATH") + ":" + binDir,
		}
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		require.NoError(t, cmd.Run())
	}
}
