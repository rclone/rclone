package gitannex

import (
	"bufio"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"testing"

	// Without this import, the local filesystem backend would be unavailable.
	// It looks unused, but the act of importing it runs its `init()` function.
	_ "github.com/rclone/rclone/backend/local"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/cache"
	"github.com/rclone/rclone/fs/config"
	"github.com/rclone/rclone/fs/config/configfile"
	"github.com/rclone/rclone/fstest/mockfs"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFixArgsForSymlinkIdentity(t *testing.T) {
	for _, argList := range [][]string{
		[]string{},
		[]string{"foo"},
		[]string{"foo", "bar"},
		[]string{"foo", "bar", "baz"},
	} {
		assert.Equal(t, maybeTransformArgs(argList), argList)
	}
}

func TestFixArgsForSymlinkCorrectName(t *testing.T) {
	assert.Equal(t,
		maybeTransformArgs([]string{"git-annex-remote-rclone-builtin"}),
		[]string{"git-annex-remote-rclone-builtin", "gitannex"})
	assert.Equal(t,
		maybeTransformArgs([]string{"/path/to/git-annex-remote-rclone-builtin"}),
		[]string{"/path/to/git-annex-remote-rclone-builtin", "gitannex"})
}

type messageParserTestCase struct {
	label    string
	testFunc func(*testing.T)
}

var messageParserTestCases = []messageParserTestCase{
	{
		"OneParam",
		func(t *testing.T) {
			m := messageParser{"foo\n"}

			param, err := m.nextSpaceDelimitedParameter()
			assert.NoError(t, err)
			assert.Equal(t, param, "foo")

			param, err = m.nextSpaceDelimitedParameter()
			assert.Error(t, err)
			assert.Equal(t, param, "")

			param = m.finalParameter()
			assert.Equal(t, param, "")

			param = m.finalParameter()
			assert.Equal(t, param, "")

			param, err = m.nextSpaceDelimitedParameter()
			assert.Error(t, err)
			assert.Equal(t, param, "")

		},
	},
	{
		"TwoParams",
		func(t *testing.T) {
			m := messageParser{"foo bar\n"}

			param, err := m.nextSpaceDelimitedParameter()
			assert.NoError(t, err)
			assert.Equal(t, param, "foo")

			param, err = m.nextSpaceDelimitedParameter()
			assert.NoError(t, err)
			assert.Equal(t, param, "bar")

			param, err = m.nextSpaceDelimitedParameter()
			assert.Error(t, err)
			assert.Equal(t, param, "")

			param = m.finalParameter()
			assert.Equal(t, param, "")
		},
	},
	{
		"TwoParamsNoTrailingNewline",

		func(t *testing.T) {
			m := messageParser{"foo bar"}

			param, err := m.nextSpaceDelimitedParameter()
			assert.NoError(t, err)
			assert.Equal(t, param, "foo")

			param, err = m.nextSpaceDelimitedParameter()
			assert.NoError(t, err)
			assert.Equal(t, param, "bar")

			param, err = m.nextSpaceDelimitedParameter()
			assert.Error(t, err)
			assert.Equal(t, param, "")

			param = m.finalParameter()
			assert.Equal(t, param, "")
		},
	},
	{
		"ThreeParamsWhereFinalParamContainsSpaces",
		func(t *testing.T) {
			m := messageParser{"firstparam secondparam final param with spaces"}

			param, err := m.nextSpaceDelimitedParameter()
			assert.NoError(t, err)
			assert.Equal(t, param, "firstparam")

			param, err = m.nextSpaceDelimitedParameter()
			assert.NoError(t, err)
			assert.Equal(t, param, "secondparam")

			param = m.finalParameter()
			assert.Equal(t, param, "final param with spaces")
		},
	},
	{
		"OneLongFinalParameter",
		func(t *testing.T) {
			for _, lineEnding := range []string{"", "\n", "\r", "\r\n", "\n\r"} {
				lineEnding := lineEnding
				testName := fmt.Sprintf("lineEnding%x", lineEnding)

				t.Run(testName, func(t *testing.T) {
					m := messageParser{"one long final parameter" + lineEnding}

					param := m.finalParameter()
					assert.Equal(t, param, "one long final parameter")

					param = m.finalParameter()
					assert.Equal(t, param, "")
				})

			}
		},
	},
	{
		"MultipleSpaces",
		func(t *testing.T) {
			m := messageParser{"foo  bar\n\r"}

			param, err := m.nextSpaceDelimitedParameter()
			assert.NoError(t, err)
			assert.Equal(t, param, "foo")

			param, err = m.nextSpaceDelimitedParameter()
			assert.Error(t, err, "blah")
			assert.Equal(t, param, "")
		},
	},
	{
		"StartsWithSpace",
		func(t *testing.T) {
			m := messageParser{" foo"}

			param, err := m.nextSpaceDelimitedParameter()
			assert.Error(t, err, "blah")
			assert.Equal(t, param, "")
		},
	},
}

func TestMessageParser(t *testing.T) {
	for _, testCase := range messageParserTestCases {
		testCase := testCase
		t.Run(testCase.label, func(t *testing.T) {
			t.Parallel()
			testCase.testFunc(t)
		})
	}
}

func TestConfigDefinitionOneName(t *testing.T) {
	var parsed string
	var defaultValue = "abc"

	configFoo := configDefinition{
		names:        []string{"foo"},
		description:  "The foo config is utterly useless.",
		destination:  &parsed,
		defaultValue: &defaultValue,
	}

	assert.Equal(t, "foo",
		configFoo.getCanonicalName())

	assert.Equal(t,
		configFoo.description,
		configFoo.fullDescription())
}

func TestConfigDefinitionTwoNames(t *testing.T) {
	var parsed string
	var defaultValue = "abc"

	configFoo := configDefinition{
		names:        []string{"foo", "bar"},
		description:  "The foo config is utterly useless.",
		destination:  &parsed,
		defaultValue: &defaultValue,
	}

	assert.Equal(t, "foo",
		configFoo.getCanonicalName())

	assert.Equal(t,
		"(synonyms: bar) The foo config is utterly useless.",
		configFoo.fullDescription())
}

func TestConfigDefinitionThreeNames(t *testing.T) {
	var parsed string
	var defaultValue = "abc"

	configFoo := configDefinition{
		names:        []string{"foo", "bar", "baz"},
		description:  "The foo config is utterly useless.",
		destination:  &parsed,
		defaultValue: &defaultValue,
	}

	assert.Equal(t, "foo",
		configFoo.getCanonicalName())

	assert.Equal(t,
		`(synonyms: bar, baz) The foo config is utterly useless.`,
		configFoo.fullDescription())
}

type testState struct {
	t                *testing.T
	server           *server
	mockStdinW       *io.PipeWriter
	mockStdoutReader *bufio.Reader

	localFsDir string
	configPath string
	remoteName string
}

func makeTestState(t *testing.T) testState {
	stdinR, stdinW := io.Pipe()
	stdoutR, stdoutW := io.Pipe()

	return testState{
		t: t,
		server: &server{
			reader: bufio.NewReader(stdinR),
			writer: stdoutW,
		},
		mockStdinW:       stdinW,
		mockStdoutReader: bufio.NewReader(stdoutR),
	}
}

func (h *testState) requireReadLineExact(line string) {
	receivedLine, err := h.mockStdoutReader.ReadString('\n')
	require.NoError(h.t, err)
	require.Equal(h.t, line+"\n", receivedLine)
}

func (h *testState) requireReadLine() string {
	receivedLine, err := h.mockStdoutReader.ReadString('\n')
	require.NoError(h.t, err)
	return receivedLine
}

func (h *testState) requireWriteLine(line string) {
	_, err := h.mockStdinW.Write([]byte(line + "\n"))
	require.NoError(h.t, err)
}

// Preconfigure the handle. This enables the calling test to skip the PREPARE
// handshake.
func (h *testState) preconfigureServer() {
	h.server.configPrefix = h.localFsDir
	h.server.configRcloneRemoteName = h.remoteName
	h.server.configRcloneLayout = string(layoutModeNodir)
	h.server.configsDone = true
}

// getUniqueRemoteName returns a valid remote name derived from the given test's
// name. This is necessary because when a test registers a second remote with
// the same name, the original remote appears to take precedence. This function
// is injective, so each test gets a unique remote name. Returned strings
// contain no spaces.
func getUniqueRemoteName(t *testing.T) string {
	// Using sha256 as a hack to ensure injectivity without adding a global
	// variable.
	return fmt.Sprintf("remote-%x", sha256.Sum256([]byte(t.Name())))
}

type testCase struct {
	label            string
	testProtocolFunc func(*testing.T, *testState)
	expectedError    string
}

// These test cases run against the "local" backend.
var localBackendTestCases = []testCase{
	{
		label: "HandlesInit",
		testProtocolFunc: func(t *testing.T, h *testState) {
			h.preconfigureServer()

			h.requireReadLineExact("VERSION 1")
			h.requireWriteLine("INITREMOTE")
			h.requireReadLineExact("INITREMOTE-SUCCESS")

			require.NoError(t, h.mockStdinW.Close())
		},
	},
	{
		label: "HandlesListConfigs",
		testProtocolFunc: func(t *testing.T, h *testState) {
			h.preconfigureServer()

			h.requireReadLineExact("VERSION 1")
			h.requireWriteLine("INITREMOTE")
			h.requireReadLineExact("INITREMOTE-SUCCESS")

			h.requireWriteLine("LISTCONFIGS")

			require.Regexp(t,
				regexp.MustCompile(`^CONFIG rcloneremotename \(synonyms: target\) (.|\n)*$`),
				h.requireReadLine(),
			)
			require.Regexp(t,
				regexp.MustCompile(`^CONFIG rcloneprefix \(synonyms: prefix\) (.|\n)*$`),
				h.requireReadLine(),
			)
			require.Regexp(t,
				regexp.MustCompile(`^CONFIG rclonelayout \(synonyms: rclone_layout\) (.|\n)*$`),
				h.requireReadLine(),
			)
			h.requireReadLineExact("CONFIGEND")

			require.NoError(t, h.mockStdinW.Close())
		},
	},
	{
		label: "HandlesPrepare",
		testProtocolFunc: func(t *testing.T, h *testState) {
			h.requireReadLineExact("VERSION 1")
			h.requireWriteLine("EXTENSIONS INFO") // Advertise that we support the INFO extension
			h.requireReadLineExact("EXTENSIONS")

			if !h.server.extensionInfo {
				t.Errorf("expected INFO extension to be enabled")
				return
			}

			h.requireWriteLine("PREPARE")
			h.requireReadLineExact("GETCONFIG rcloneremotename")
			h.requireWriteLine("VALUE " + h.remoteName)
			h.requireReadLineExact("GETCONFIG rcloneprefix")
			h.requireWriteLine("VALUE " + h.localFsDir)
			h.requireReadLineExact("GETCONFIG rclonelayout")
			h.requireWriteLine("VALUE foo")
			h.requireReadLineExact("PREPARE-SUCCESS")

			require.Equal(t, h.server.configRcloneRemoteName, h.remoteName)
			require.Equal(t, h.server.configPrefix, h.localFsDir)
			require.True(t, h.server.configsDone)

			require.NoError(t, h.mockStdinW.Close())
		},
	},
	{
		label: "HandlesPrepareWithSynonyms",
		testProtocolFunc: func(t *testing.T, h *testState) {
			h.requireReadLineExact("VERSION 1")
			h.requireWriteLine("EXTENSIONS INFO") // Advertise that we support the INFO extension
			h.requireReadLineExact("EXTENSIONS")

			if !h.server.extensionInfo {
				t.Errorf("expected INFO extension to be enabled")
				return
			}

			h.requireWriteLine("PREPARE")
			h.requireReadLineExact("GETCONFIG rcloneremotename")
			// TODO check what git-annex does when asked for a config value it does not have.
			h.requireWriteLine("VALUE")
			h.requireReadLineExact("GETCONFIG target")
			h.requireWriteLine("VALUE " + h.remoteName)

			h.requireReadLineExact("GETCONFIG rcloneprefix")
			h.requireWriteLine("VALUE " + h.localFsDir)
			h.requireReadLineExact("GETCONFIG rclonelayout")
			h.requireWriteLine("VALUE foo")
			h.requireReadLineExact("PREPARE-SUCCESS")

			require.Equal(t, h.server.configRcloneRemoteName, h.remoteName)
			require.Equal(t, h.server.configPrefix, h.localFsDir)
			require.True(t, h.server.configsDone)

			require.NoError(t, h.mockStdinW.Close())
		},
	},
	{
		label: "HandlesPrepareAndDoesNotTrimWhitespaceFromValue",
		testProtocolFunc: func(t *testing.T, h *testState) {
			h.requireReadLineExact("VERSION 1")
			h.requireWriteLine("EXTENSIONS INFO") // Advertise that we support the INFO extension
			h.requireReadLineExact("EXTENSIONS")

			if !h.server.extensionInfo {
				t.Errorf("expected INFO extension to be enabled")
				return
			}

			h.requireWriteLine("PREPARE")
			h.requireReadLineExact("GETCONFIG rcloneremotename")

			remoteNameWithSpaces := fmt.Sprintf(" %s ", h.remoteName)
			localFsDirWithSpaces := fmt.Sprintf(" %s\t", h.localFsDir)

			h.requireWriteLine(fmt.Sprintf("VALUE %s", remoteNameWithSpaces))

			h.requireReadLineExact("GETCONFIG rcloneprefix")
			h.requireWriteLine(fmt.Sprintf("VALUE %s", localFsDirWithSpaces))

			h.requireReadLineExact("GETCONFIG rclonelayout")
			h.requireWriteLine("VALUE")
			h.requireReadLineExact("GETCONFIG rclone_layout")
			h.requireWriteLine("VALUE")

			h.requireReadLineExact("PREPARE-SUCCESS")

			require.Equal(t, h.server.configRcloneRemoteName, remoteNameWithSpaces)
			require.Equal(t, h.server.configPrefix, localFsDirWithSpaces)
			require.True(t, h.server.configsDone)

			require.NoError(t, h.mockStdinW.Close())
		},
	},
	{
		label: "HandlesEarlyError",
		testProtocolFunc: func(t *testing.T, h *testState) {
			h.preconfigureServer()

			h.requireReadLineExact("VERSION 1")
			h.requireWriteLine("ERROR foo")

			require.NoError(t, h.mockStdinW.Close())
		},
		expectedError: "received error message from git-annex: foo",
	},
	// Test what happens when the git-annex client sends "GETCONFIG", but
	// doesn't understand git-annex's response.
	{
		label: "ConfigFail",
		testProtocolFunc: func(t *testing.T, h *testState) {
			h.requireReadLineExact("VERSION 1")
			h.requireWriteLine("EXTENSIONS INFO") // Advertise that we support the INFO extension
			h.requireReadLineExact("EXTENSIONS")
			require.True(t, h.server.extensionInfo)

			h.requireWriteLine("PREPARE")
			h.requireReadLineExact("GETCONFIG rcloneremotename")
			h.requireWriteLine("ERROR ineffable error")
			h.requireReadLineExact("PREPARE-FAILURE Error getting configs")

			require.NoError(t, h.mockStdinW.Close())
		},
		expectedError: "failed to parse config value: ERROR ineffable error",
	},
	{
		label: "TransferStoreEmptyPath",
		testProtocolFunc: func(t *testing.T, h *testState) {
			h.preconfigureServer()

			h.requireReadLineExact("VERSION 1")
			h.requireWriteLine("INITREMOTE")
			h.requireReadLineExact("INITREMOTE-SUCCESS")

			// Note the whitespace following the key.
			h.requireWriteLine("TRANSFER STORE Key ")
			h.requireReadLineExact("TRANSFER-FAILURE failed to parse file path")

			require.NoError(t, h.mockStdinW.Close())
		},
		expectedError: "failed to parse file",
	},
	// Repeated EXTENSIONS messages add to each other rather than overriding
	// prior advertised extensions. This behavior is not mandated by the
	// protocol design.
	{
		label: "ExtensionsCompound",
		testProtocolFunc: func(t *testing.T, h *testState) {
			h.preconfigureServer()

			h.requireReadLineExact("VERSION 1")
			h.requireWriteLine("INITREMOTE")
			h.requireReadLineExact("INITREMOTE-SUCCESS")

			h.requireWriteLine("EXTENSIONS")
			h.requireReadLineExact("EXTENSIONS")
			require.False(t, h.server.extensionInfo)
			require.False(t, h.server.extensionAsync)
			require.False(t, h.server.extensionGetGitRemoteName)
			require.False(t, h.server.extensionUnavailableResponse)

			h.requireWriteLine("EXTENSIONS INFO")
			h.requireReadLineExact("EXTENSIONS")
			require.True(t, h.server.extensionInfo)
			require.False(t, h.server.extensionAsync)
			require.False(t, h.server.extensionGetGitRemoteName)
			require.False(t, h.server.extensionUnavailableResponse)

			h.requireWriteLine("EXTENSIONS ASYNC")
			h.requireReadLineExact("EXTENSIONS")
			require.True(t, h.server.extensionInfo)
			require.True(t, h.server.extensionAsync)
			require.False(t, h.server.extensionGetGitRemoteName)
			require.False(t, h.server.extensionUnavailableResponse)

			h.requireWriteLine("EXTENSIONS GETGITREMOTENAME")
			h.requireReadLineExact("EXTENSIONS")
			require.True(t, h.server.extensionInfo)
			require.True(t, h.server.extensionAsync)
			require.True(t, h.server.extensionGetGitRemoteName)
			require.False(t, h.server.extensionUnavailableResponse)

			h.requireWriteLine("EXTENSIONS UNAVAILABLERESPONSE")
			h.requireReadLineExact("EXTENSIONS")
			require.True(t, h.server.extensionInfo)
			require.True(t, h.server.extensionAsync)
			require.True(t, h.server.extensionGetGitRemoteName)
			require.True(t, h.server.extensionUnavailableResponse)

			require.NoError(t, h.mockStdinW.Close())
		},
	},
	{
		label: "ExtensionsIdempotent",
		testProtocolFunc: func(t *testing.T, h *testState) {
			h.preconfigureServer()

			h.requireReadLineExact("VERSION 1")
			h.requireWriteLine("INITREMOTE")
			h.requireReadLineExact("INITREMOTE-SUCCESS")

			h.requireWriteLine("EXTENSIONS")
			h.requireReadLineExact("EXTENSIONS")
			require.False(t, h.server.extensionInfo)
			require.False(t, h.server.extensionAsync)
			require.False(t, h.server.extensionGetGitRemoteName)
			require.False(t, h.server.extensionUnavailableResponse)

			h.requireWriteLine("EXTENSIONS")
			h.requireReadLineExact("EXTENSIONS")
			require.False(t, h.server.extensionInfo)
			require.False(t, h.server.extensionAsync)
			require.False(t, h.server.extensionGetGitRemoteName)
			require.False(t, h.server.extensionUnavailableResponse)

			h.requireWriteLine("EXTENSIONS INFO")
			h.requireReadLineExact("EXTENSIONS")
			require.True(t, h.server.extensionInfo)
			require.False(t, h.server.extensionAsync)
			require.False(t, h.server.extensionGetGitRemoteName)
			require.False(t, h.server.extensionUnavailableResponse)

			h.requireWriteLine("EXTENSIONS INFO")
			h.requireReadLineExact("EXTENSIONS")
			require.True(t, h.server.extensionInfo)
			require.False(t, h.server.extensionAsync)
			require.False(t, h.server.extensionGetGitRemoteName)
			require.False(t, h.server.extensionUnavailableResponse)

			h.requireWriteLine("EXTENSIONS ASYNC ASYNC")
			h.requireReadLineExact("EXTENSIONS")
			require.True(t, h.server.extensionInfo)
			require.True(t, h.server.extensionAsync)
			require.False(t, h.server.extensionGetGitRemoteName)
			require.False(t, h.server.extensionUnavailableResponse)

			require.NoError(t, h.mockStdinW.Close())
		},
	},
	{
		label: "ExtensionsSupportsMultiple",
		testProtocolFunc: func(t *testing.T, h *testState) {
			h.preconfigureServer()

			h.requireReadLineExact("VERSION 1")
			h.requireWriteLine("INITREMOTE")
			h.requireReadLineExact("INITREMOTE-SUCCESS")

			h.requireWriteLine("EXTENSIONS")
			h.requireReadLineExact("EXTENSIONS")
			require.False(t, h.server.extensionInfo)
			require.False(t, h.server.extensionAsync)
			require.False(t, h.server.extensionGetGitRemoteName)
			require.False(t, h.server.extensionUnavailableResponse)

			h.requireWriteLine("EXTENSIONS INFO ASYNC")
			h.requireReadLineExact("EXTENSIONS")
			require.True(t, h.server.extensionInfo)
			require.True(t, h.server.extensionAsync)
			require.False(t, h.server.extensionGetGitRemoteName)
			require.False(t, h.server.extensionUnavailableResponse)

			require.NoError(t, h.mockStdinW.Close())
		},
	},
	{
		label: "TransferStoreAbsolute",
		testProtocolFunc: func(t *testing.T, h *testState) {
			h.preconfigureServer()

			h.requireReadLineExact("VERSION 1")
			h.requireWriteLine("INITREMOTE")
			h.requireReadLineExact("INITREMOTE-SUCCESS")

			// Create temp file for transfer with an absolute path.
			fileToTransfer := filepath.Join(t.TempDir(), "file.txt")
			require.NoError(t, os.WriteFile(fileToTransfer, []byte("HELLO"), 0600))
			require.FileExists(t, fileToTransfer)
			require.True(t, filepath.IsAbs(fileToTransfer))

			// Specify an absolute path to transfer.
			h.requireWriteLine("TRANSFER STORE KeyAbsolute " + fileToTransfer)
			h.requireReadLineExact("TRANSFER-SUCCESS STORE KeyAbsolute")
			require.FileExists(t, filepath.Join(h.localFsDir, "KeyAbsolute"))

			// Transfer the same absolute path a second time, but with a different key.
			h.requireWriteLine("TRANSFER STORE KeyAbsolute2 " + fileToTransfer)
			h.requireReadLineExact("TRANSFER-SUCCESS STORE KeyAbsolute2")
			require.FileExists(t, filepath.Join(h.localFsDir, "KeyAbsolute2"))

			h.requireWriteLine("CHECKPRESENT KeyAbsolute2")
			h.requireReadLineExact("CHECKPRESENT-SUCCESS KeyAbsolute2")

			h.requireWriteLine("CHECKPRESENT KeyThatDoesNotExist")
			h.requireReadLineExact("CHECKPRESENT-FAILURE KeyThatDoesNotExist")

			require.NoError(t, h.mockStdinW.Close())
		},
	},
	// Test that the TRANSFER command understands simple relative paths
	// consisting only of a file name.
	{
		label: "TransferStoreRelative",
		testProtocolFunc: func(t *testing.T, h *testState) {
			h.preconfigureServer()

			// Save the current working directory so we can restore it when this
			// test ends.
			cwd, err := os.Getwd()
			require.NoError(t, err)

			require.NoError(t, os.Chdir(t.TempDir()))
			t.Cleanup(func() { require.NoError(t, os.Chdir(cwd)) })

			h.requireReadLineExact("VERSION 1")
			h.requireWriteLine("INITREMOTE")
			h.requireReadLineExact("INITREMOTE-SUCCESS")

			// Create temp file for transfer with a relative path.
			fileToTransfer := "file.txt"
			require.NoError(t, os.WriteFile(fileToTransfer, []byte("HELLO"), 0600))
			require.FileExists(t, fileToTransfer)
			require.False(t, filepath.IsAbs(fileToTransfer))

			// Specify a relative path to transfer.
			h.requireWriteLine("TRANSFER STORE KeyRelative " + fileToTransfer)
			h.requireReadLineExact("TRANSFER-SUCCESS STORE KeyRelative")
			require.FileExists(t, filepath.Join(h.localFsDir, "KeyRelative"))

			h.requireWriteLine("CHECKPRESENT KeyRelative")
			h.requireReadLineExact("CHECKPRESENT-SUCCESS KeyRelative")

			h.requireWriteLine("CHECKPRESENT KeyThatDoesNotExist")
			h.requireReadLineExact("CHECKPRESENT-FAILURE KeyThatDoesNotExist")

			require.NoError(t, h.mockStdinW.Close())
		},
	},
	{
		label: "TransferStorePathWithInteriorWhitespace",
		testProtocolFunc: func(t *testing.T, h *testState) {
			// Save the current working directory so we can restore it when this
			// test ends.
			cwd, err := os.Getwd()
			require.NoError(t, err)

			require.NoError(t, os.Chdir(t.TempDir()))
			t.Cleanup(func() { require.NoError(t, os.Chdir(cwd)) })

			h.preconfigureServer()

			h.requireReadLineExact("VERSION 1")
			h.requireWriteLine("INITREMOTE")
			h.requireReadLineExact("INITREMOTE-SUCCESS")

			// Create temp file for transfer.
			fileToTransfer := "filename with spaces.txt"
			require.NoError(t, os.WriteFile(fileToTransfer, []byte("HELLO"), 0600))
			require.FileExists(t, fileToTransfer)
			require.False(t, filepath.IsAbs(fileToTransfer))

			// Specify a relative path to transfer.
			h.requireWriteLine("TRANSFER STORE KeyRelative " + fileToTransfer)
			h.requireReadLineExact("TRANSFER-SUCCESS STORE KeyRelative")
			require.FileExists(t, filepath.Join(h.localFsDir, "KeyRelative"))

			h.requireWriteLine("CHECKPRESENT KeyRelative")
			h.requireReadLineExact("CHECKPRESENT-SUCCESS KeyRelative")

			h.requireWriteLine("CHECKPRESENT KeyThatDoesNotExist")
			h.requireReadLineExact("CHECKPRESENT-FAILURE KeyThatDoesNotExist")

			require.NoError(t, h.mockStdinW.Close())
		},
	},
	{
		label: "CheckPresentAndTransfer",
		testProtocolFunc: func(t *testing.T, h *testState) {
			h.preconfigureServer()

			// Create temp file for transfer.
			fileToTransfer := filepath.Join(t.TempDir(), "file.txt")
			require.NoError(t, os.WriteFile(fileToTransfer, []byte("HELLO"), 0600))

			h.requireReadLineExact("VERSION 1")
			h.requireWriteLine("INITREMOTE")
			h.requireReadLineExact("INITREMOTE-SUCCESS")

			h.requireWriteLine("CHECKPRESENT KeyThatDoesNotExist")
			h.requireReadLineExact("CHECKPRESENT-FAILURE KeyThatDoesNotExist")

			// Specify an absolute path to transfer.
			require.True(t, filepath.IsAbs(fileToTransfer))
			h.requireWriteLine("TRANSFER STORE KeyAbsolute " + fileToTransfer)
			h.requireReadLineExact("TRANSFER-SUCCESS STORE KeyAbsolute")
			require.FileExists(t, filepath.Join(h.localFsDir, "KeyAbsolute"))

			require.NoError(t, h.mockStdinW.Close())
		},
	},
	// Check whether a key is present, transfer a file with that key, then check
	// again whether it is present.
	//
	// This is a regression test for a bug where the second CHECKPRESENT would
	// generate the following response:
	//
	//	CHECKPRESENT-UNKNOWN ${key} failed to read directory entry: readdirent ${filepath}: not a directory
	//
	// This message was generated by the local backend's `List()` function. When
	// checking whether a file exists, we were erroneously listing its contents as
	// if it were a directory.
	{
		label: "CheckpresentTransferCheckpresent",
		testProtocolFunc: func(t *testing.T, h *testState) {
			h.preconfigureServer()

			// Create temp file for transfer.
			fileToTransfer := filepath.Join(t.TempDir(), "file.txt")
			require.NoError(t, os.WriteFile(fileToTransfer, []byte("HELLO"), 0600))

			h.requireReadLineExact("VERSION 1")
			h.requireWriteLine("INITREMOTE")
			h.requireReadLineExact("INITREMOTE-SUCCESS")

			h.requireWriteLine("CHECKPRESENT foo")
			h.requireReadLineExact("CHECKPRESENT-FAILURE foo")

			h.requireWriteLine("TRANSFER STORE foo " + fileToTransfer)
			h.requireReadLineExact("TRANSFER-SUCCESS STORE foo")
			require.FileExists(t, filepath.Join(h.localFsDir, "foo"))

			h.requireWriteLine("CHECKPRESENT foo")
			h.requireReadLineExact("CHECKPRESENT-SUCCESS foo")

			require.NoError(t, h.mockStdinW.Close())
		},
	},
	{
		label: "TransferAndCheckpresentWithRealisticKey",
		testProtocolFunc: func(t *testing.T, h *testState) {
			h.preconfigureServer()

			// Create temp file for transfer.
			fileToTransfer := filepath.Join(t.TempDir(), "file.txt")
			require.NoError(t, os.WriteFile(fileToTransfer, []byte("HELLO"), 0600))

			h.requireReadLineExact("VERSION 1")
			h.requireWriteLine("INITREMOTE")
			h.requireReadLineExact("INITREMOTE-SUCCESS")

			realisticKey := "SHA256E-s1048576--7ba87e06b9b7903cfbaf4a38736766c161e3e7b42f06fe57f040aa410a8f0701.this-is-a-test-key"

			// Specify an absolute path to transfer.
			require.True(t, filepath.IsAbs(fileToTransfer))
			h.requireWriteLine(fmt.Sprintf("TRANSFER STORE %s %s", realisticKey, fileToTransfer))
			h.requireReadLineExact("TRANSFER-SUCCESS STORE " + realisticKey)
			require.FileExists(t, filepath.Join(h.localFsDir, realisticKey))

			h.requireWriteLine("CHECKPRESENT " + realisticKey)
			h.requireReadLineExact("CHECKPRESENT-SUCCESS " + realisticKey)

			require.NoError(t, h.mockStdinW.Close())
		},
	},
	{
		label: "RetrieveNonexistentFile",
		testProtocolFunc: func(t *testing.T, h *testState) {
			h.preconfigureServer()

			h.requireReadLineExact("VERSION 1")
			h.requireWriteLine("INITREMOTE")
			h.requireReadLineExact("INITREMOTE-SUCCESS")

			h.requireWriteLine("TRANSFER RETRIEVE SomeKey path")
			h.requireReadLineExact("TRANSFER-FAILURE RETRIEVE SomeKey not found")

			require.NoError(t, h.mockStdinW.Close())
		},
	},
	{
		label: "StoreCheckpresentRetrieve",
		testProtocolFunc: func(t *testing.T, h *testState) {
			h.preconfigureServer()

			// Create temp file for transfer.
			fileToTransfer := filepath.Join(t.TempDir(), "file.txt")
			require.NoError(t, os.WriteFile(fileToTransfer, []byte("HELLO"), 0600))

			h.requireReadLineExact("VERSION 1")
			h.requireWriteLine("INITREMOTE")
			h.requireReadLineExact("INITREMOTE-SUCCESS")

			// Specify an absolute path to transfer.
			require.True(t, filepath.IsAbs(fileToTransfer))
			h.requireWriteLine("TRANSFER STORE SomeKey " + fileToTransfer)
			h.requireReadLineExact("TRANSFER-SUCCESS STORE SomeKey")
			require.FileExists(t, filepath.Join(h.localFsDir, "SomeKey"))

			h.requireWriteLine("CHECKPRESENT SomeKey")
			h.requireReadLineExact("CHECKPRESENT-SUCCESS SomeKey")

			retrievedFilePath := fileToTransfer + ".retrieved"
			require.NoFileExists(t, retrievedFilePath)
			h.requireWriteLine("TRANSFER RETRIEVE SomeKey " + retrievedFilePath)
			h.requireReadLineExact("TRANSFER-SUCCESS RETRIEVE SomeKey")
			require.FileExists(t, retrievedFilePath)

			require.NoError(t, h.mockStdinW.Close())
		},
	},
	{
		label: "RemovePreexistingFile",
		testProtocolFunc: func(t *testing.T, h *testState) {
			h.preconfigureServer()

			// Write a file into the remote without using the git-annex
			// protocol.
			remoteFilePath := filepath.Join(h.localFsDir, "SomeKey")
			require.NoError(t, os.WriteFile(remoteFilePath, []byte("HELLO"), 0600))
			require.FileExists(t, remoteFilePath)

			h.requireReadLineExact("VERSION 1")
			h.requireWriteLine("INITREMOTE")
			h.requireReadLineExact("INITREMOTE-SUCCESS")

			h.requireWriteLine("CHECKPRESENT SomeKey")
			h.requireReadLineExact("CHECKPRESENT-SUCCESS SomeKey")
			require.FileExists(t, remoteFilePath)

			h.requireWriteLine("REMOVE SomeKey")
			h.requireReadLineExact("REMOVE-SUCCESS SomeKey")
			require.NoFileExists(t, remoteFilePath)

			h.requireWriteLine("CHECKPRESENT SomeKey")
			h.requireReadLineExact("CHECKPRESENT-FAILURE SomeKey")
			require.NoFileExists(t, remoteFilePath)

			require.NoError(t, h.mockStdinW.Close())
		},
	},
	{
		label: "Remove",
		testProtocolFunc: func(t *testing.T, h *testState) {
			h.preconfigureServer()

			// Create temp file for transfer.
			fileToTransfer := filepath.Join(t.TempDir(), "file.txt")
			require.NoError(t, os.WriteFile(fileToTransfer, []byte("HELLO"), 0600))

			h.requireReadLineExact("VERSION 1")
			h.requireWriteLine("INITREMOTE")
			h.requireReadLineExact("INITREMOTE-SUCCESS")

			h.requireWriteLine("CHECKPRESENT SomeKey")
			h.requireReadLineExact("CHECKPRESENT-FAILURE SomeKey")

			// Specify an absolute path to transfer.
			require.True(t, filepath.IsAbs(fileToTransfer))
			h.requireWriteLine("TRANSFER STORE SomeKey " + fileToTransfer)
			h.requireReadLineExact("TRANSFER-SUCCESS STORE SomeKey")
			require.FileExists(t, filepath.Join(h.localFsDir, "SomeKey"))

			h.requireWriteLine("CHECKPRESENT SomeKey")
			h.requireReadLineExact("CHECKPRESENT-SUCCESS SomeKey")

			h.requireWriteLine("REMOVE SomeKey")
			h.requireReadLineExact("REMOVE-SUCCESS SomeKey")
			require.NoFileExists(t, filepath.Join(h.localFsDir, "SomeKey"))

			h.requireWriteLine("CHECKPRESENT SomeKey")
			h.requireReadLineExact("CHECKPRESENT-FAILURE SomeKey")

			require.NoError(t, h.mockStdinW.Close())
		},
	},
	{
		label: "RemoveNonexistentFile",
		testProtocolFunc: func(t *testing.T, h *testState) {
			h.preconfigureServer()

			// Create temp file for transfer.
			fileToTransfer := filepath.Join(t.TempDir(), "file.txt")
			require.NoError(t, os.WriteFile(fileToTransfer, []byte("HELLO"), 0600))

			h.requireReadLineExact("VERSION 1")
			h.requireWriteLine("INITREMOTE")
			h.requireReadLineExact("INITREMOTE-SUCCESS")

			h.requireWriteLine("CHECKPRESENT SomeKey")
			h.requireReadLineExact("CHECKPRESENT-FAILURE SomeKey")

			require.NoFileExists(t, filepath.Join(h.localFsDir, "SomeKey"))
			h.requireWriteLine("REMOVE SomeKey")
			h.requireReadLineExact("REMOVE-SUCCESS SomeKey")
			require.NoFileExists(t, filepath.Join(h.localFsDir, "SomeKey"))

			h.requireWriteLine("CHECKPRESENT SomeKey")
			h.requireReadLineExact("CHECKPRESENT-FAILURE SomeKey")

			require.NoError(t, h.mockStdinW.Close())
		},
	},
	{
		label: "ExportNotSupported",
		testProtocolFunc: func(t *testing.T, h *testState) {
			h.preconfigureServer()

			h.requireReadLineExact("VERSION 1")
			h.requireWriteLine("INITREMOTE")
			h.requireReadLineExact("INITREMOTE-SUCCESS")

			h.requireWriteLine("EXPORTSUPPORTED")
			h.requireReadLineExact("EXPORTSUPPORTED-FAILURE")

			require.NoError(t, h.mockStdinW.Close())
		},
	},
}

func TestGitAnnexLocalBackendCases(t *testing.T) {
	for _, testCase := range localBackendTestCases {
		// Clear global state left behind by tests that chdir to a temp directory.
		cache.Clear()

		// TODO: Remove this when rclone requires a Go version >= 1.22. Future
		// versions of Go fix the semantics of capturing a range variable.
		// https://go.dev/blog/loopvar-preview
		testCase := testCase

		t.Run(testCase.label, func(t *testing.T) {
			tempDir := t.TempDir()

			// Create temp dir for an rclone remote pointing at local filesystem.
			localFsDir := filepath.Join(tempDir, "remoteTarget")
			require.NoError(t, os.Mkdir(localFsDir, 0700))

			// Create temp config
			remoteName := getUniqueRemoteName(t)
			configLines := []string{
				fmt.Sprintf("[%s]", remoteName),
				"type = local",
				fmt.Sprintf("remote = %s", localFsDir),
			}
			configContents := strings.Join(configLines, "\n")

			configPath := filepath.Join(tempDir, "rclone.conf")
			require.NoError(t, os.WriteFile(configPath, []byte(configContents), 0600))
			require.NoError(t, config.SetConfigPath(configPath))

			// The custom config file will be ignored unless we install the
			// global config file handler.
			configfile.Install()

			handle := makeTestState(t)
			handle.localFsDir = localFsDir
			handle.configPath = configPath
			handle.remoteName = remoteName

			var wg sync.WaitGroup
			wg.Add(1)

			go func() {
				err := handle.server.run()

				if testCase.expectedError == "" {
					require.NoError(t, err)
				} else {
					require.ErrorContains(t, err, testCase.expectedError)
				}

				wg.Done()
			}()
			defer wg.Wait()

			testCase.testProtocolFunc(t, &handle)
		})
	}
}

// Configure the git-annex client with a mockfs backend and send it the
// "INITREMOTE" command over mocked stdin. This should fail because mockfs does
// not support empty directories.
func TestGitAnnexHandleInitRemoteBackendDoesNotSupportEmptyDirectories(t *testing.T) {
	tempDir := t.TempDir()

	// Temporarily override the filesystem registry.
	oldRegistry := fs.Registry
	mockfs.Register()
	defer func() { fs.Registry = oldRegistry }()

	// Create temp dir for an rclone remote pointing at local filesystem.
	localFsDir := filepath.Join(tempDir, "remoteTarget")
	require.NoError(t, os.Mkdir(localFsDir, 0700))

	// Create temp config
	remoteName := getUniqueRemoteName(t)
	configLines := []string{
		fmt.Sprintf("[%s]", remoteName),
		"type = mockfs",
		fmt.Sprintf("remote = %s", localFsDir),
	}
	configContents := strings.Join(configLines, "\n")

	configPath := filepath.Join(tempDir, "rclone.conf")
	require.NoError(t, os.WriteFile(configPath, []byte(configContents), 0600))

	// The custom config file will be ignored unless we install the global
	// config file handler.
	configfile.Install()
	require.NoError(t, config.SetConfigPath(configPath))

	handle := makeTestState(t)
	handle.server.configPrefix = localFsDir
	handle.server.configRcloneRemoteName = remoteName
	handle.server.configsDone = true

	var wg sync.WaitGroup
	wg.Add(1)

	go func() {
		require.NotNil(t, handle.server.run())
		wg.Done()
	}()
	defer wg.Wait()

	handle.requireReadLineExact("VERSION 1")
	handle.requireWriteLine("INITREMOTE")
	handle.requireReadLineExact("INITREMOTE-FAILURE this rclone remote does not support empty directories")
}
