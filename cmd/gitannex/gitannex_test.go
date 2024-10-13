package gitannex

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	// Without this import, the various backends would be unavailable. It looks
	// unused, but the act of importing runs the package's `init()` function.
	_ "github.com/rclone/rclone/backend/all"

	"github.com/rclone/rclone/fs/fspath"
	"github.com/rclone/rclone/fstest"

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

	fstestRun    *fstest.Run
	remoteName   string
	remotePrefix string
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

func (h *testState) requireRemoteIsEmpty() {
	h.fstestRun.CheckRemoteItems(h.t)
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
	h.server.configRcloneRemoteName = h.remoteName
	h.server.configPrefix = h.remotePrefix
	h.server.configRcloneLayout = string(layoutModeNodir)
	h.server.configsDone = true
}

// Drop-in replacement for `filepath.Rel()` that works around a Windows-specific
// quirk when one of the paths begins with `\\?\` or `//?/`. It seems that
// fstest gives us paths with this prefix on Windows, which throws a wrench in
// the gitannex tests that need to construct relative paths from absolute paths.
// For a demonstration, see `TestWindowsFilepathRelQuirk` below.
//
// The `\\?\` prefix tells Windows APIs to pass strings unmodified to the
// filesystem without additional parsing [1]. Our workaround is roughly to add
// the prefix to whichever parameter doesn't have it (when the OS is Windows).
// I'm not sure this generalizes, but it works for the the kinds of inputs we're
// throwing at it.
//
// [1]: https://learn.microsoft.com/en-us/windows/win32/fileio/naming-a-file?redirectedfrom=MSDN#win32-file-namespaces
func relativeFilepathWorkaround(basepath, targpath string) (string, error) {
	if runtime.GOOS != "windows" {
		return filepath.Rel(basepath, targpath)
	}
	// Canonicalize paths to use backslashes.
	basepath = filepath.Clean(basepath)
	targpath = filepath.Clean(targpath)

	const winFilePrefixDisableStringParsing = `\\?\`
	baseHasPrefix := strings.HasPrefix(basepath, winFilePrefixDisableStringParsing)
	targHasPrefix := strings.HasPrefix(targpath, winFilePrefixDisableStringParsing)

	if baseHasPrefix && !targHasPrefix {
		targpath = winFilePrefixDisableStringParsing + targpath
	}
	if !baseHasPrefix && targHasPrefix {
		basepath = winFilePrefixDisableStringParsing + basepath
	}
	return filepath.Rel(basepath, targpath)
}

func TestWindowsFilepathRelQuirk(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip()
	}

	t.Run("filepathRelQuirk", func(t *testing.T) {
		var err error

		_, err = filepath.Rel(`C:\foo`, `\\?\C:\foo\bar`)
		require.Error(t, err)

		_, err = filepath.Rel(`C:/foo`, `//?/C:/foo/bar`)
		require.Error(t, err)

		_, err = filepath.Rel(`\\?\C:\foo`, `C:\foo\bar`)
		require.Error(t, err)

		_, err = filepath.Rel(`//?/C:/foo`, `C:/foo/bar`)
		require.Error(t, err)

		path, err := filepath.Rel(`\\?\C:\foo`, `\\?\C:\foo\bar`)
		require.NoError(t, err)
		require.Equal(t, path, `bar`)

		path, err = filepath.Rel(`//?/C:/foo`, `//?/C:/foo/bar`)
		require.NoError(t, err)
		require.Equal(t, path, `bar`)
	})

	t.Run("fstestAndTempDirHaveDifferentPrefixes", func(t *testing.T) {
		r := fstest.NewRun(t)
		p := r.Flocal.Root()
		require.True(t, strings.HasPrefix(p, `//?/`))

		tempDir := t.TempDir()
		require.False(t, strings.HasPrefix(tempDir, `//?/`))
		require.False(t, strings.HasPrefix(tempDir, `\\?\`))
	})

	t.Run("workaroundWorks", func(t *testing.T) {
		path, err := relativeFilepathWorkaround(`C:\foo`, `\\?\C:\foo\bar`)
		require.NoError(t, err)
		require.Equal(t, path, "bar")

		path, err = relativeFilepathWorkaround(`C:/foo`, `//?/C:/foo/bar`)
		require.NoError(t, err)
		require.Equal(t, path, "bar")

		path, err = relativeFilepathWorkaround(`\\?\C:\foo`, `C:\foo\bar`)
		require.NoError(t, err)
		require.Equal(t, path, `bar`)

		path, err = relativeFilepathWorkaround(`//?/C:/foo`, `C:/foo/bar`)
		require.NoError(t, err)
		require.Equal(t, path, `bar`)

		path, err = relativeFilepathWorkaround(`\\?\C:\foo`, `\\?\C:\foo\bar`)
		require.NoError(t, err)
		require.Equal(t, path, `bar`)
	})
}

type testCase struct {
	label            string
	testProtocolFunc func(*testing.T, *testState)
	expectedError    string
}

// These test cases run against a backend selected by the `-remote` flag.
var fstestTestCases = []testCase{
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

			require.True(t, h.server.extensionInfo)

			h.requireWriteLine("PREPARE")
			h.requireReadLineExact("GETCONFIG rcloneremotename")
			h.requireWriteLine("VALUE " + h.remoteName)
			h.requireReadLineExact("GETCONFIG rcloneprefix")
			h.requireWriteLine("VALUE " + h.remotePrefix)
			h.requireReadLineExact("GETCONFIG rclonelayout")
			h.requireWriteLine("VALUE foo")
			h.requireReadLineExact("PREPARE-SUCCESS")

			require.Equal(t, h.server.configRcloneRemoteName, h.remoteName)
			require.Equal(t, h.server.configPrefix, h.remotePrefix)
			require.True(t, h.server.configsDone)

			require.NoError(t, h.mockStdinW.Close())
		},
	},
	{
		label: "HandlesPrepareWithNonexistentRemote",
		testProtocolFunc: func(t *testing.T, h *testState) {
			h.requireReadLineExact("VERSION 1")
			h.requireWriteLine("EXTENSIONS INFO") // Advertise that we support the INFO extension
			h.requireReadLineExact("EXTENSIONS")

			require.True(t, h.server.extensionInfo)

			h.requireWriteLine("PREPARE")
			h.requireReadLineExact("GETCONFIG rcloneremotename")
			h.requireWriteLine("VALUE thisRemoteDoesNotExist")
			h.requireReadLineExact("GETCONFIG rcloneprefix")
			h.requireWriteLine("VALUE " + h.remotePrefix)
			h.requireReadLineExact("GETCONFIG rclonelayout")
			h.requireWriteLine("VALUE foo")
			h.requireReadLineExact("PREPARE-SUCCESS")

			require.Equal(t, h.server.configRcloneRemoteName, "thisRemoteDoesNotExist")
			require.Equal(t, h.server.configPrefix, h.remotePrefix)
			require.True(t, h.server.configsDone)

			h.requireWriteLine("INITREMOTE")
			h.requireReadLineExact("INITREMOTE-FAILURE remote does not exist: thisRemoteDoesNotExist")

			require.NoError(t, h.mockStdinW.Close())
		},
		expectedError: "remote does not exist: thisRemoteDoesNotExist",
	},
	{
		label: "HandlesPrepareWithPathAsRemote",
		testProtocolFunc: func(t *testing.T, h *testState) {
			h.requireReadLineExact("VERSION 1")
			h.requireWriteLine("EXTENSIONS INFO") // Advertise that we support the INFO extension
			h.requireReadLineExact("EXTENSIONS")

			require.True(t, h.server.extensionInfo)

			h.requireWriteLine("PREPARE")
			h.requireReadLineExact("GETCONFIG rcloneremotename")
			h.requireWriteLine("VALUE " + h.remotePrefix)
			h.requireReadLineExact("GETCONFIG rcloneprefix")
			h.requireWriteLine("VALUE /foo")
			h.requireReadLineExact("GETCONFIG rclonelayout")
			h.requireWriteLine("VALUE foo")
			h.requireReadLineExact("PREPARE-SUCCESS")

			require.Equal(t, h.server.configRcloneRemoteName, h.remotePrefix)
			require.Equal(t, h.server.configPrefix, "/foo")
			require.True(t, h.server.configsDone)

			h.requireWriteLine("INITREMOTE")

			require.Regexp(t,
				regexp.MustCompile("^INITREMOTE-FAILURE remote does not exist: "),
				h.requireReadLine(),
			)

			require.NoError(t, h.mockStdinW.Close())
		},
		expectedError: "remote does not exist:",
	},
	{
		label: "HandlesPrepareWithSynonyms",
		testProtocolFunc: func(t *testing.T, h *testState) {
			h.requireReadLineExact("VERSION 1")
			h.requireWriteLine("EXTENSIONS INFO") // Advertise that we support the INFO extension
			h.requireReadLineExact("EXTENSIONS")

			require.True(t, h.server.extensionInfo)

			h.requireWriteLine("PREPARE")
			h.requireReadLineExact("GETCONFIG rcloneremotename")
			// TODO check what git-annex does when asked for a config value it does not have.
			h.requireWriteLine("VALUE")
			h.requireReadLineExact("GETCONFIG target")
			h.requireWriteLine("VALUE " + h.remoteName)

			h.requireReadLineExact("GETCONFIG rcloneprefix")
			h.requireWriteLine("VALUE " + h.remotePrefix)
			h.requireReadLineExact("GETCONFIG rclonelayout")
			h.requireWriteLine("VALUE foo")
			h.requireReadLineExact("PREPARE-SUCCESS")

			require.Equal(t, h.server.configRcloneRemoteName, h.remoteName)
			require.Equal(t, h.server.configPrefix, h.remotePrefix)
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

			require.True(t, h.server.extensionInfo)

			h.requireWriteLine("PREPARE")
			h.requireReadLineExact("GETCONFIG rcloneremotename")

			remoteNameWithSpaces := fmt.Sprintf(" %s ", h.remoteName)
			prefixWithWhitespace := fmt.Sprintf(" %s\t", h.remotePrefix)

			h.requireWriteLine(fmt.Sprintf("VALUE %s", remoteNameWithSpaces))

			h.requireReadLineExact("GETCONFIG rcloneprefix")
			h.requireWriteLine(fmt.Sprintf("VALUE %s", prefixWithWhitespace))

			h.requireReadLineExact("GETCONFIG rclonelayout")
			h.requireWriteLine("VALUE")
			h.requireReadLineExact("GETCONFIG rclone_layout")
			h.requireWriteLine("VALUE")

			h.requireReadLineExact("PREPARE-SUCCESS")

			require.Equal(t, h.server.configRcloneRemoteName, remoteNameWithSpaces)
			require.Equal(t, h.server.configPrefix, prefixWithWhitespace)
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
			item := h.fstestRun.WriteFile("file.txt", "HELLO", time.Now())
			absPath := filepath.Join(h.fstestRun.Flocal.Root(), item.Path)
			require.True(t, filepath.IsAbs(absPath))

			// Specify an absolute path to transfer.
			h.requireWriteLine("TRANSFER STORE KeyAbsolute " + absPath)
			h.requireReadLineExact("TRANSFER-SUCCESS STORE KeyAbsolute")

			// Check that the file was transferred.
			remoteItem := fstest.NewItem("KeyAbsolute", "HELLO", item.ModTime)
			h.fstestRun.CheckRemoteItems(t, remoteItem)

			// Transfer the same absolute path a second time, but with a different key.
			h.requireWriteLine("TRANSFER STORE KeyAbsolute2 " + absPath)
			h.requireReadLineExact("TRANSFER-SUCCESS STORE KeyAbsolute2")

			// Check that the same file was transferred to a new name.
			remoteItem2 := fstest.NewItem("KeyAbsolute2", "HELLO", item.ModTime)
			h.fstestRun.CheckRemoteItems(t, remoteItem, remoteItem2)

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
			// Save the current working directory so we can restore it when this
			// test ends.
			cwd, err := os.Getwd()
			require.NoError(t, err)

			tempDir := t.TempDir()

			require.NoError(t, os.Chdir(tempDir))
			t.Cleanup(func() { require.NoError(t, os.Chdir(cwd)) })

			h.preconfigureServer()

			h.requireReadLineExact("VERSION 1")
			h.requireWriteLine("INITREMOTE")
			h.requireReadLineExact("INITREMOTE-SUCCESS")

			item := h.fstestRun.WriteFile("file.txt", "HELLO", time.Now())
			absPath := filepath.Join(h.fstestRun.Flocal.Root(), item.Path)

			relativePath, err := relativeFilepathWorkaround(tempDir, absPath)
			require.NoError(t, err)
			require.False(t, filepath.IsAbs(relativePath))
			require.FileExists(t, relativePath)

			// Specify a relative path to transfer.
			h.requireWriteLine("TRANSFER STORE KeyRelative " + relativePath)
			h.requireReadLineExact("TRANSFER-SUCCESS STORE KeyRelative")

			remoteItem := fstest.NewItem("KeyRelative", "HELLO", item.ModTime)
			h.fstestRun.CheckRemoteItems(t, remoteItem)

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

			tempDir := t.TempDir()
			require.NoError(t, os.Chdir(tempDir))
			t.Cleanup(func() { require.NoError(t, os.Chdir(cwd)) })

			h.preconfigureServer()

			h.requireReadLineExact("VERSION 1")
			h.requireWriteLine("INITREMOTE")
			h.requireReadLineExact("INITREMOTE-SUCCESS")

			// Create temp file for transfer.
			item := h.fstestRun.WriteFile("filename with spaces.txt", "HELLO", time.Now())
			absPath := filepath.Join(h.fstestRun.Flocal.Root(), item.Path)
			relativePath, err := relativeFilepathWorkaround(tempDir, absPath)
			require.NoError(t, err)
			require.False(t, filepath.IsAbs(relativePath))
			require.FileExists(t, relativePath)

			// Specify a relative path to transfer.
			h.requireWriteLine("TRANSFER STORE KeyRelative " + relativePath)
			h.requireReadLineExact("TRANSFER-SUCCESS STORE KeyRelative")

			remoteItem := fstest.NewItem("KeyRelative", "HELLO", item.ModTime)
			h.fstestRun.CheckRemoteItems(t, remoteItem)

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
			item := h.fstestRun.WriteFile("file.txt", "HELLO", time.Now())
			absPath := filepath.Join(h.fstestRun.Flocal.Root(), item.Path)
			require.True(t, filepath.IsAbs(absPath))

			h.requireReadLineExact("VERSION 1")
			h.requireWriteLine("INITREMOTE")
			h.requireReadLineExact("INITREMOTE-SUCCESS")

			h.requireWriteLine("CHECKPRESENT KeyThatDoesNotExist")
			h.requireReadLineExact("CHECKPRESENT-FAILURE KeyThatDoesNotExist")

			// Specify an absolute path to transfer.
			h.requireWriteLine("TRANSFER STORE KeyAbsolute " + absPath)
			h.requireReadLineExact("TRANSFER-SUCCESS STORE KeyAbsolute")

			remoteItem := fstest.NewItem("KeyAbsolute", "HELLO", item.ModTime)
			h.fstestRun.CheckRemoteItems(t, remoteItem)

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
			item := h.fstestRun.WriteFile("file.txt", "HELLO", time.Now())
			absPath := filepath.Join(h.fstestRun.Flocal.Root(), item.Path)
			require.True(t, filepath.IsAbs(absPath))

			h.requireReadLineExact("VERSION 1")
			h.requireWriteLine("INITREMOTE")
			h.requireReadLineExact("INITREMOTE-SUCCESS")

			h.requireWriteLine("CHECKPRESENT foo")
			h.requireReadLineExact("CHECKPRESENT-FAILURE foo")

			h.requireWriteLine("TRANSFER STORE foo " + absPath)
			h.requireReadLineExact("TRANSFER-SUCCESS STORE foo")

			remoteItem := fstest.NewItem("foo", "HELLO", item.ModTime)
			h.fstestRun.CheckRemoteItems(t, remoteItem)

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
			item := h.fstestRun.WriteFile("file.txt", "HELLO", time.Now())
			absPath := filepath.Join(h.fstestRun.Flocal.Root(), item.Path)
			require.True(t, filepath.IsAbs(absPath))

			h.requireReadLineExact("VERSION 1")
			h.requireWriteLine("INITREMOTE")
			h.requireReadLineExact("INITREMOTE-SUCCESS")

			realisticKey := "SHA256E-s1048576--7ba87e06b9b7903cfbaf4a38736766c161e3e7b42f06fe57f040aa410a8f0701.this-is-a-test-key"

			// Specify an absolute path to transfer.
			h.requireWriteLine(fmt.Sprintf("TRANSFER STORE %s %s", realisticKey, absPath))
			h.requireReadLineExact("TRANSFER-SUCCESS STORE " + realisticKey)

			remoteItem := fstest.NewItem(realisticKey, "HELLO", item.ModTime)
			h.fstestRun.CheckRemoteItems(t, remoteItem)

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
			item := h.fstestRun.WriteFile("file.txt", "HELLO", time.Now())
			absPath := filepath.Join(h.fstestRun.Flocal.Root(), item.Path)
			require.True(t, filepath.IsAbs(absPath))

			h.requireReadLineExact("VERSION 1")
			h.requireWriteLine("INITREMOTE")
			h.requireReadLineExact("INITREMOTE-SUCCESS")

			// Specify an absolute path to transfer.
			h.requireWriteLine("TRANSFER STORE SomeKey " + absPath)
			h.requireReadLineExact("TRANSFER-SUCCESS STORE SomeKey")

			remoteItem := fstest.NewItem("SomeKey", "HELLO", item.ModTime)
			h.fstestRun.CheckRemoteItems(t, remoteItem)

			h.requireWriteLine("CHECKPRESENT SomeKey")
			h.requireReadLineExact("CHECKPRESENT-SUCCESS SomeKey")

			h.fstestRun.CheckLocalItems(t,
				fstest.NewItem("file.txt", "HELLO", item.ModTime),
			)

			retrievedFilePath := absPath + ".retrieved"
			h.requireWriteLine("TRANSFER RETRIEVE SomeKey " + retrievedFilePath)
			h.requireReadLineExact("TRANSFER-SUCCESS RETRIEVE SomeKey")

			h.fstestRun.CheckLocalItems(t,
				fstest.NewItem("file.txt", "HELLO", item.ModTime),
				fstest.NewItem("file.txt.retrieved", "HELLO", item.ModTime),
			)

			require.NoError(t, h.mockStdinW.Close())
		},
	},
	{
		label: "RemovePreexistingFile",
		testProtocolFunc: func(t *testing.T, h *testState) {
			h.preconfigureServer()

			ctx := context.WithoutCancel(context.Background())

			// Write a file into the remote without using the git-annex
			// protocol.
			remoteItem := h.fstestRun.WriteObject(ctx, "SomeKey", "HELLO", time.Now())

			h.fstestRun.CheckRemoteItems(t, remoteItem)

			h.requireReadLineExact("VERSION 1")
			h.requireWriteLine("INITREMOTE")
			h.requireReadLineExact("INITREMOTE-SUCCESS")

			h.requireWriteLine("CHECKPRESENT SomeKey")
			h.requireReadLineExact("CHECKPRESENT-SUCCESS SomeKey")

			h.fstestRun.CheckRemoteItems(t, remoteItem)

			h.requireWriteLine("REMOVE SomeKey")
			h.requireReadLineExact("REMOVE-SUCCESS SomeKey")

			h.requireRemoteIsEmpty()

			h.requireWriteLine("CHECKPRESENT SomeKey")
			h.requireReadLineExact("CHECKPRESENT-FAILURE SomeKey")

			h.requireRemoteIsEmpty()

			require.NoError(t, h.mockStdinW.Close())
		},
	},
	{
		label: "Remove",
		testProtocolFunc: func(t *testing.T, h *testState) {
			h.preconfigureServer()

			// Create temp file for transfer.
			item := h.fstestRun.WriteFile("file.txt", "HELLO", time.Now())
			absPath := filepath.Join(h.fstestRun.Flocal.Root(), item.Path)
			require.True(t, filepath.IsAbs(absPath))

			h.requireReadLineExact("VERSION 1")
			h.requireWriteLine("INITREMOTE")
			h.requireReadLineExact("INITREMOTE-SUCCESS")

			h.requireWriteLine("CHECKPRESENT SomeKey")
			h.requireReadLineExact("CHECKPRESENT-FAILURE SomeKey")

			// Specify an absolute path to transfer.
			h.requireWriteLine("TRANSFER STORE SomeKey " + absPath)
			h.requireReadLineExact("TRANSFER-SUCCESS STORE SomeKey")

			remoteItem := fstest.NewItem("SomeKey", "HELLO", item.ModTime)
			h.fstestRun.CheckRemoteItems(t, remoteItem)

			h.requireWriteLine("CHECKPRESENT SomeKey")
			h.requireReadLineExact("CHECKPRESENT-SUCCESS SomeKey")

			h.requireWriteLine("REMOVE SomeKey")
			h.requireReadLineExact("REMOVE-SUCCESS SomeKey")

			h.requireRemoteIsEmpty()

			h.requireWriteLine("CHECKPRESENT SomeKey")
			h.requireReadLineExact("CHECKPRESENT-FAILURE SomeKey")

			require.NoError(t, h.mockStdinW.Close())
		},
	},
	{
		label: "RemoveNonexistentFile",
		testProtocolFunc: func(t *testing.T, h *testState) {
			h.preconfigureServer()

			h.requireReadLineExact("VERSION 1")
			h.requireWriteLine("INITREMOTE")
			h.requireReadLineExact("INITREMOTE-SUCCESS")

			h.requireWriteLine("CHECKPRESENT SomeKey")
			h.requireReadLineExact("CHECKPRESENT-FAILURE SomeKey")

			h.requireRemoteIsEmpty()

			h.requireWriteLine("REMOVE SomeKey")
			h.requireReadLineExact("REMOVE-SUCCESS SomeKey")

			h.requireRemoteIsEmpty()

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

// TestMain drives the tests
func TestMain(m *testing.M) {
	fstest.TestMain(m)
}

// Run fstest-compatible test cases with backend selected by `-remote`.
func TestGitAnnexFstestBackendCases(t *testing.T) {

	for _, testCase := range fstestTestCases {
		// TODO: Remove this when rclone requires a Go version >= 1.22. Future
		// versions of Go fix the semantics of capturing a range variable.
		// https://go.dev/blog/loopvar-preview
		testCase := testCase

		t.Run(testCase.label, func(t *testing.T) {
			r := fstest.NewRun(t)
			t.Cleanup(func() { r.Finalise() })

			// Parse the fstest-provided remote string. It might have a path!
			remoteName, remotePath, err := fspath.SplitFs(r.FremoteName)
			require.NoError(t, err)

			// The gitannex command requires the `rcloneremotename` is the name
			// of a remote or exactly ":local", so the empty string will not
			// suffice.
			if remoteName == "" {
				require.True(t, r.Fremote.Features().IsLocal)
				remoteName = ":local"
			}

			handle := makeTestState(t)
			handle.fstestRun = r
			handle.remoteName = remoteName
			handle.remotePrefix = remotePath

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
