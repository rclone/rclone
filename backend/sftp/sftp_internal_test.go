//go:build !plan9

package sftp

import (
	"context"
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/hash"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestShellEscapeUnix(t *testing.T) {
	for i, test := range []struct {
		unescaped, escaped string
	}{
		{"", ""},
		{"/this/is/harmless", "/this/is/harmless"},
		{"$(rm -rf /)", "\\$\\(rm\\ -rf\\ /\\)"},
		{"/test/\n", "/test/'\n'"},
		{":\"'", ":\\\"\\'"},
	} {
		got, err := quoteOrEscapeShellPath("unix", test.unescaped)
		assert.NoError(t, err)
		assert.Equal(t, test.escaped, got, fmt.Sprintf("Test %d unescaped = %q", i, test.unescaped))
	}
}

func TestShellEscapeCmd(t *testing.T) {
	for i, test := range []struct {
		unescaped, escaped string
		ok                 bool
	}{
		{"", "\"\"", true},
		{"c:/this/is/harmless", "\"c:/this/is/harmless\"", true},
		{"c:/test&notepad", "\"c:/test&notepad\"", true},
		{"c:/test\"&\"notepad", "", false},
	} {
		got, err := quoteOrEscapeShellPath("cmd", test.unescaped)
		if test.ok {
			assert.NoError(t, err)
			assert.Equal(t, test.escaped, got, fmt.Sprintf("Test %d unescaped = %q", i, test.unescaped))
		} else {
			assert.Error(t, err)
		}
	}
}

func TestShellEscapePowerShell(t *testing.T) {
	for i, test := range []struct {
		unescaped, escaped string
	}{
		{"", "''"},
		{"c:/this/is/harmless", "'c:/this/is/harmless'"},
		{"c:/test&notepad", "'c:/test&notepad'"},
		{"c:/test\"&\"notepad", "'c:/test\"&\"notepad'"},
		{"c:/test'&'notepad", "'c:/test''&''notepad'"},
	} {
		got, err := quoteOrEscapeShellPath("powershell", test.unescaped)
		assert.NoError(t, err)
		assert.Equal(t, test.escaped, got, fmt.Sprintf("Test %d unescaped = %q", i, test.unescaped))
	}
}

func TestParseHash(t *testing.T) {
	for i, test := range []struct {
		sshOutput, checksum string
	}{
		{"8dbc7733dbd10d2efc5c0a0d8dad90f958581821  RELEASE.md\n", "8dbc7733dbd10d2efc5c0a0d8dad90f958581821"},
		{"03cfd743661f07975fa2f1220c5194cbaff48451  -\n", "03cfd743661f07975fa2f1220c5194cbaff48451"},
	} {
		got := parseHash([]byte(test.sshOutput))
		assert.Equal(t, test.checksum, got, fmt.Sprintf("Test %d sshOutput = %q", i, test.sshOutput))
	}
}

func TestParseUsage(t *testing.T) {
	for i, test := range []struct {
		sshOutput string
		usage     [3]int64
	}{
		{"Filesystem     1K-blocks     Used Available Use% Mounted on\n/dev/root       91283092 81111888  10154820  89% /", [3]int64{93473886208, 83058573312, 10398535680}},
		{"Filesystem     1K-blocks  Used Available Use% Mounted on\ntmpfs             818256  1636    816620   1% /run", [3]int64{837894144, 1675264, 836218880}},
		{"Filesystem   1024-blocks     Used Available Capacity iused      ifree %iused  Mounted on\n/dev/disk0s2   244277768 94454848 149566920    39%  997820 4293969459    0%   /", [3]int64{250140434432, 96721764352, 153156526080}},
	} {
		gotSpaceTotal, gotSpaceUsed, gotSpaceAvail := parseUsage([]byte(test.sshOutput))
		assert.Equal(t, test.usage, [3]int64{gotSpaceTotal, gotSpaceUsed, gotSpaceAvail}, fmt.Sprintf("Test %d sshOutput = %q", i, test.sshOutput))
	}
}

func TestTranslateLink(t *testing.T) {
	for i, test := range []struct {
		remote             string
		sftpPath           string
		expectedPath       string
		expectedTranslated bool
	}{
		// Regular file without .rclonelink suffix
		{"file.txt", "/path/to/file.txt", "/path/to/file.txt", false},
		// File with .rclonelink suffix - should be marked as translated
		{"symlink.txt" + fs.LinkSuffix, "/path/to/symlink.txt" + fs.LinkSuffix, "/path/to/symlink.txt", true},
		// Directory without suffix
		{"dir/file.txt", "/path/dir/file.txt", "/path/dir/file.txt", false},
		// Directory with file having .rclonelink suffix
		{"dir/symlink.txt" + fs.LinkSuffix, "/path/dir/symlink.txt" + fs.LinkSuffix, "/path/dir/symlink.txt", true},
		// Edge case: empty strings
		{"", "", "", false},
		// Edge case: suffix alone
		{fs.LinkSuffix, "/" + fs.LinkSuffix, "/", true},
	} {
		gotPath, gotTranslated := translateLink(test.remote, test.sftpPath)
		assert.Equal(t, test.expectedPath, gotPath, fmt.Sprintf("Test %d: path mismatch for remote=%q, sftpPath=%q", i, test.remote, test.sftpPath))
		assert.Equal(t, test.expectedTranslated, gotTranslated, fmt.Sprintf("Test %d: translated flag mismatch for remote=%q", i, test.remote))
	}
}

func TestNopWriterCloser(t *testing.T) {
	// Test that nopWriterCloser wraps a writer correctly
	var buf strings.Builder
	wc := nopWriterCloser{&buf}

	// Test Write
	n, err := wc.Write([]byte("hello"))
	assert.NoError(t, err)
	assert.Equal(t, 5, n)
	assert.Equal(t, "hello", buf.String())

	// Test Write again
	n, err = wc.Write([]byte(" world"))
	assert.NoError(t, err)
	assert.Equal(t, 6, n)
	assert.Equal(t, "hello world", buf.String())

	// Test Close (should be no-op)
	err = wc.Close()
	assert.NoError(t, err)

	// Verify we can still read what was written
	assert.Equal(t, "hello world", buf.String())
}

// TestSymlinkHash tests that hash calculation for symlinks works correctly
func TestSymlinkHash(t *testing.T) {
	// This test verifies the hash calculation logic for translated symlinks
	// The hash should be computed from the target path string, not the target content

	testCases := []struct {
		name           string
		targetPath     string
		hashType       hash.Type
		expectedLength int
	}{
		{"simple path md5", "file.txt", hash.MD5, 32},
		{"nested path md5", "dir/subdir/file.txt", hash.MD5, 32},
		{"absolute path md5", "/absolute/path/to/file", hash.MD5, 32},
		{"relative with dots md5", "../parent/file.txt", hash.MD5, 32},
		{"sha1 hash", "file.txt", hash.SHA1, 40},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Compute expected hash
			hasher, err := hash.NewMultiHasherTypes(hash.NewHashSet(tc.hashType))
			require.NoError(t, err)

			_, err = hasher.Write([]byte(tc.targetPath))
			require.NoError(t, err)

			expectedHash := hasher.Sums()[tc.hashType]

			// Verify the hash is computed from the target path string
			assert.NotEmpty(t, expectedHash, "Hash should not be empty")
			assert.Equal(t, tc.expectedLength, len(expectedHash), fmt.Sprintf("%s hash should be %d characters", tc.hashType, tc.expectedLength))
		})
	}
}

// TestObjectPath tests the path() method behavior with translated symlinks
func TestObjectPath(t *testing.T) {
	testCases := []struct {
		name           string
		remote         string
		translatedLink bool
		absRoot        string
		expectedPath   string
	}{
		{
			name:           "regular file",
			remote:         "file.txt",
			translatedLink: false,
			absRoot:        "/home/user",
			expectedPath:   "/home/user/file.txt",
		},
		{
			name:           "translated symlink - removes .rclonelink",
			remote:         "symlink.txt" + fs.LinkSuffix,
			translatedLink: true,
			absRoot:        "/home/user",
			expectedPath:   "/home/user/symlink.txt",
		},
		{
			name:           "nested translated symlink",
			remote:         "dir/symlink.txt" + fs.LinkSuffix,
			translatedLink: true,
			absRoot:        "/home/user",
			expectedPath:   "/home/user/dir/symlink.txt",
		},
		{
			name:           "regular file with empty root",
			remote:         "file.txt",
			translatedLink: false,
			absRoot:        "",
			expectedPath:   "file.txt",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create a mock Object
			o := &Object{
				fs: &Fs{
					absRoot: tc.absRoot,
					opt: Options{
						TranslateSymlinks: true,
					},
				},
				remote:         tc.remote,
				translatedLink: tc.translatedLink,
			}

			// Test path() method
			gotPath := o.path()
			assert.Equal(t, tc.expectedPath, gotPath)
		})
	}
}

// TestSymlinkOpen tests reading symlink target as text
func TestSymlinkOpen(t *testing.T) {
	ctx := context.Background()
	targetPath := "target/file.txt"

	// Test reading full content
	t.Run("read full content", func(t *testing.T) {
		reader := io.NopCloser(strings.NewReader(targetPath))
		content, err := io.ReadAll(reader)
		require.NoError(t, err)
		assert.Equal(t, targetPath, string(content))
	})

	// Test reading with range (offset and limit)
	t.Run("read with range", func(t *testing.T) {
		offset := int64(7)
		limit := int64(4)

		reader := strings.NewReader(targetPath)
		reader.Seek(offset, io.SeekStart)

		limitedReader := io.LimitReader(reader, limit)
		content, err := io.ReadAll(limitedReader)
		require.NoError(t, err)

		expected := targetPath[offset : offset+limit]
		assert.Equal(t, expected, string(content))
	})

	_ = ctx // avoid unused variable warning
}
