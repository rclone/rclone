package gitannex

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/lib/buildinfo"
)

// checkRcloneBinaryVersion runs whichever rclone is on the PATH and checks
// whether it reports a version that matches the test's expectations. Returns
// nil when the version is the expected version, otherwise returns an error.
func checkRcloneBinaryVersion(t *testing.T) error {
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
	if parsed.IsGit != strings.HasSuffix(fs.Version, "-DEV") {
		return errors.New("expected rclone to be a dev build")
	}
	_, tagString := buildinfo.GetLinkingAndTags()
	if parsed.GoTags != tagString {
		// TODO: Skip the test when tags do not match.
		t.Logf("expected tag string %q, but got %q. Not skipping!", tagString, parsed.GoTags)
	}
	return nil
}

// countFilesRecursively returns the number of files nested underneath `dir`. It
// counts files only and excludes directories.
func countFilesRecursively(t *testing.T, dir string) int {
	remoteFiles, err := os.ReadDir(dir)
	require.NoError(t, err)

	var count int
	for _, f := range remoteFiles {
		if f.IsDir() {
			subdir := filepath.Join(dir, f.Name())
			count += countFilesRecursively(t, subdir)
		} else {
			count++
		}
	}
	return count
}

func findFileWithContents(t *testing.T, dir string, wantContents []byte) bool {
	remoteFiles, err := os.ReadDir(dir)
	require.NoError(t, err)

	for _, f := range remoteFiles {
		fPath := filepath.Join(dir, f.Name())
		if f.IsDir() {
			if findFileWithContents(t, fPath, wantContents) {
				return true
			}
		} else {
			contents, err := os.ReadFile(fPath)
			require.NoError(t, err)
			if bytes.Equal(contents, wantContents) {
				return true
			}
		}
	}
	return false
}

type e2eTestingContext struct {
	t                *testing.T
	tempDir          string
	binDir           string
	homeDir          string
	configDir        string
	rcloneConfigDir  string
	ephemeralRepoDir string
}

// makeE2eTestingContext sets up a new e2eTestingContext rooted under
// `t.TempDir()`. It creates the skeleton directory structure shown below in the
// temp directory without creating any files.
//
//	.
//	|-- bin
//	|   `-- git-annex-remote-rclone-builtin -> ${PATH_TO_RCLONE_BINARY}
//	|-- ephemeralRepo
//	`-- user
//		`-- .config
//			`-- rclone
//				`-- rclone.conf
func makeE2eTestingContext(t *testing.T) e2eTestingContext {
	tempDir := t.TempDir()

	binDir := filepath.Join(tempDir, "bin")
	homeDir := filepath.Join(tempDir, "user")
	configDir := filepath.Join(homeDir, ".config")
	rcloneConfigDir := filepath.Join(configDir, "rclone")
	ephemeralRepoDir := filepath.Join(tempDir, "ephemeralRepo")

	for _, dir := range []string{binDir, homeDir, configDir, rcloneConfigDir, ephemeralRepoDir} {
		require.NoError(t, os.Mkdir(dir, 0700))
	}

	return e2eTestingContext{t, tempDir, binDir, homeDir, configDir, rcloneConfigDir, ephemeralRepoDir}
}

// Install the symlink that enables git-annex to invoke "rclone gitannex"
// without explicitly specifying the subcommand.
func (e *e2eTestingContext) installRcloneGitannexSymlink(t *testing.T) {
	rcloneBinaryPath, err := exec.LookPath("rclone")
	require.NoError(t, err)
	require.NoError(t, os.Symlink(
		rcloneBinaryPath,
		filepath.Join(e.binDir, "git-annex-remote-rclone-builtin")))
}

// Install a rclone.conf file in an appropriate location in the fake home
// directory. The config defines an rclone remote named "MyRcloneRemote" using
// the local backend.
func (e *e2eTestingContext) installRcloneConfig(t *testing.T) {
	// Install the rclone.conf file that defines the remote.
	rcloneConfigPath := filepath.Join(e.rcloneConfigDir, "rclone.conf")
	rcloneConfigContents := "[MyRcloneRemote]\ntype = local"
	require.NoError(t, os.WriteFile(rcloneConfigPath, []byte(rcloneConfigContents), 0600))
}

// runInRepo runs the given command from within the ephemeral repo directory. To
// prevent accidental changes in the real home directory, it sets the HOME
// variable to a subdirectory of the temp directory. It also ensures that the
// git-annex-remote-rclone-builtin symlink will be found by extending the PATH.
func (e *e2eTestingContext) runInRepo(t *testing.T, command string, args ...string) {
	if testing.Verbose() {
		t.Logf("Running %s %v\n", command, args)
	}
	cmd := exec.Command(command, args...)
	cmd.Dir = e.ephemeralRepoDir
	cmd.Env = []string{
		"HOME=" + e.homeDir,
		"PATH=" + os.Getenv("PATH") + ":" + e.binDir,
	}
	buf, err := cmd.CombinedOutput()
	require.NoError(t, err, fmt.Sprintf("+ %s %v failed:\n%s\n", command, args, buf))
}

// createGitRepo creates an empty git repository in the ephemeral repo
// directory. It makes "global" config changes that are ultimately scoped to the
// calling test thanks to runInRepo() overriding the HOME environment variable.
func (e *e2eTestingContext) createGitRepo(t *testing.T) {
	e.runInRepo(t, "git", "annex", "version")
	e.runInRepo(t, "git", "config", "--global", "user.name", "User Name")
	e.runInRepo(t, "git", "config", "--global", "user.email", "user@example.com")
	e.runInRepo(t, "git", "config", "--global", "init.defaultBranch", "main")
	e.runInRepo(t, "git", "init")
	e.runInRepo(t, "git", "annex", "init")
}

func skipE2eTestIfNecessary(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping due to short mode.")
	}

	// TODO: Support e2e tests on Windows. Need to evaluate the semantics of the
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

	if err := checkRcloneBinaryVersion(t); err != nil {
		t.Skipf("Skipping due to rclone version: %s", err)
	}

	if _, err := exec.LookPath("git-annex"); err != nil {
		t.Skipf("Skipping because git-annex was not found: %s", err)
	}
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
	skipE2eTestIfNecessary(t)

	for _, mode := range allLayoutModes() {
		mode := mode
		t.Run(string(mode), func(t *testing.T) {
			t.Parallel()

			testingContext := makeE2eTestingContext(t)
			testingContext.installRcloneGitannexSymlink(t)
			testingContext.installRcloneConfig(t)
			testingContext.createGitRepo(t)

			testingContext.runInRepo(t, "git", "annex", "initremote", "MyTestRemote",
				"type=external", "externaltype=rclone-builtin", "encryption=none",
				"rcloneremotename=MyRcloneRemote", "rcloneprefix="+testingContext.ephemeralRepoDir,
				"rclonelayout="+string(mode))

			testingContext.runInRepo(t, "git", "annex", "testremote", "MyTestRemote")
		})
	}
}

// For each layout mode, migrate a single remote from git-annex-remote-rclone to
// git-annex-remote-rclone-builtin and run `git annex testremote`.
func TestEndToEndMigration(t *testing.T) {
	skipE2eTestIfNecessary(t)

	if _, err := exec.LookPath("git-annex-remote-rclone"); err != nil {
		t.Skipf("Skipping because git-annex-remote-rclone was not found: %s", err)
	}

	for _, mode := range allLayoutModes() {
		mode := mode
		t.Run(string(mode), func(t *testing.T) {
			t.Parallel()

			tc := makeE2eTestingContext(t)
			tc.installRcloneGitannexSymlink(t)
			tc.installRcloneConfig(t)
			tc.createGitRepo(t)

			remoteStorage := filepath.Join(tc.tempDir, "remotePrefix")
			require.NoError(t, os.Mkdir(remoteStorage, 0777))

			tc.runInRepo(t,
				"git", "annex", "initremote", "MigratedRemote",
				"type=external", "externaltype=rclone", "encryption=none",
				"target=MyRcloneRemote",
				"rclone_layout="+string(mode),
				"prefix="+remoteStorage,
			)

			fooFileContents := []byte{1, 2, 3, 4}
			fooFilePath := filepath.Join(tc.ephemeralRepoDir, "foo")
			require.NoError(t, os.WriteFile(fooFilePath, fooFileContents, 0700))
			tc.runInRepo(t, "git", "annex", "add", "foo")
			tc.runInRepo(t, "git", "commit", "-m", "Add foo file")
			// Git-annex objects are not writable, which prevents `testing` from
			// cleaning up the temp directory. We can work around this by
			// explicitly dropping any files we add to the annex.
			t.Cleanup(func() { tc.runInRepo(t, "git", "annex", "drop", "--force", "foo") })

			tc.runInRepo(t, "git", "annex", "copy", "--to=MigratedRemote", "foo")
			tc.runInRepo(t, "git", "annex", "fsck", "--from=MigratedRemote", "foo")

			tc.runInRepo(t,
				"git", "annex", "enableremote", "MigratedRemote",
				"externaltype=rclone-builtin",
				"rcloneremotename=MyRcloneRemote",
				"rclonelayout="+string(mode),
				"rcloneprefix="+remoteStorage,
			)

			tc.runInRepo(t, "git", "annex", "fsck", "--from=MigratedRemote", "foo")

			tc.runInRepo(t, "git", "annex", "testremote", "MigratedRemote")
		})
	}
}

// For each layout mode, create two git-annex remotes with externaltype=rclone
// and externaltype=rclone-builtin respectively. Test that files copied to one
// remote are present on the other. Similarly, test that files deleted from one
// are removed on the other.
func TestEndToEndRepoLayoutCompat(t *testing.T) {
	skipE2eTestIfNecessary(t)

	if _, err := exec.LookPath("git-annex-remote-rclone"); err != nil {
		t.Skipf("Skipping because git-annex-remote-rclone was not found: %s", err)
	}

	for _, mode := range allLayoutModes() {
		mode := mode
		t.Run(string(mode), func(t *testing.T) {
			t.Parallel()

			tc := makeE2eTestingContext(t)
			tc.installRcloneGitannexSymlink(t)
			tc.installRcloneConfig(t)
			tc.createGitRepo(t)

			remoteStorage := filepath.Join(tc.tempDir, "remotePrefix")
			require.NoError(t, os.Mkdir(remoteStorage, 0777))

			tc.runInRepo(t,
				"git", "annex", "initremote", "Control",
				"type=external", "externaltype=rclone", "encryption=none",
				"target=MyRcloneRemote",
				"rclone_layout="+string(mode),
				"prefix="+remoteStorage)

			tc.runInRepo(t,
				"git", "annex", "initremote", "Experiment",
				"type=external", "externaltype=rclone-builtin", "encryption=none",
				"rcloneremotename=MyRcloneRemote",
				"rclonelayout="+string(mode),
				"rcloneprefix="+remoteStorage)

			fooFileContents := []byte{1, 2, 3, 4}
			fooFilePath := filepath.Join(tc.ephemeralRepoDir, "foo")
			require.NoError(t, os.WriteFile(fooFilePath, fooFileContents, 0700))
			tc.runInRepo(t, "git", "annex", "add", "foo")
			tc.runInRepo(t, "git", "commit", "-m", "Add foo file")
			// Git-annex objects are not writable, which prevents `testing` from
			// cleaning up the temp directory. We can work around this by
			// explicitly dropping any files we add to the annex.
			t.Cleanup(func() { tc.runInRepo(t, "git", "annex", "drop", "--force", "foo") })

			require.Equal(t, 0, countFilesRecursively(t, remoteStorage))
			require.False(t, findFileWithContents(t, remoteStorage, fooFileContents))

			// Copy the file to Control and verify it's present on Experiment.

			tc.runInRepo(t, "git", "annex", "copy", "--to=Control", "foo")
			require.Equal(t, 1, countFilesRecursively(t, remoteStorage))
			require.True(t, findFileWithContents(t, remoteStorage, fooFileContents))

			tc.runInRepo(t, "git", "annex", "fsck", "--from=Experiment", "foo")
			require.Equal(t, 1, countFilesRecursively(t, remoteStorage))
			require.True(t, findFileWithContents(t, remoteStorage, fooFileContents))

			// Drop the file locally and verify we can copy it back from Experiment.

			tc.runInRepo(t, "git", "annex", "drop", "--force", "foo")
			require.Equal(t, 1, countFilesRecursively(t, remoteStorage))
			require.True(t, findFileWithContents(t, remoteStorage, fooFileContents))

			tc.runInRepo(t, "git", "annex", "copy", "--from=Experiment", "foo")
			require.Equal(t, 1, countFilesRecursively(t, remoteStorage))
			require.True(t, findFileWithContents(t, remoteStorage, fooFileContents))

			// Drop the file from Experiment, copy it back to Experiment, and
			// verify it's still present on Control.

			tc.runInRepo(t, "git", "annex", "drop", "--from=Experiment", "--force", "foo")
			require.Equal(t, 0, countFilesRecursively(t, remoteStorage))
			require.False(t, findFileWithContents(t, remoteStorage, fooFileContents))

			tc.runInRepo(t, "git", "annex", "copy", "--to=Experiment", "foo")
			require.Equal(t, 1, countFilesRecursively(t, remoteStorage))
			require.True(t, findFileWithContents(t, remoteStorage, fooFileContents))

			tc.runInRepo(t, "git", "annex", "fsck", "--from=Control", "foo")
			require.Equal(t, 1, countFilesRecursively(t, remoteStorage))
			require.True(t, findFileWithContents(t, remoteStorage, fooFileContents))

			// Drop the file from Control.

			tc.runInRepo(t, "git", "annex", "drop", "--from=Control", "--force", "foo")
			require.Equal(t, 0, countFilesRecursively(t, remoteStorage))
			require.False(t, findFileWithContents(t, remoteStorage, fooFileContents))
		})
	}
}
