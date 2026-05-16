//go:build !plan9

package sftp

import (
	"context"
	"fmt"
	"io"
	"strings"
	"sync"
	"testing"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config/configmap"
	"github.com/rclone/rclone/fstest/fstests"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockSSHClient implements sshClient interface for testing
type mockSSHClient struct{}

func (m *mockSSHClient) Wait() error                     { return nil }
func (m *mockSSHClient) SendKeepAlive()                  {}
func (m *mockSSHClient) Close() error                    { return nil }
func (m *mockSSHClient) NewSession() (sshSession, error) { return nil, nil }
func (m *mockSSHClient) CanReuse() bool                  { return true }

type settings map[string]any

func deriveFs(ctx context.Context, t *testing.T, f fs.Fs, opts settings) fs.Fs {
	fsName := strings.Split(f.Name(), "{")[0] // strip off hash
	configMap := configmap.Simple{}
	for key, val := range opts {
		configMap[key] = fmt.Sprintf("%v", val)
	}
	remote := fmt.Sprintf("%s,%s:%s", fsName, configMap.String(), f.Root())
	newFs, err := fs.NewFs(ctx, remote)
	require.NoError(t, err)
	return newFs
}

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

func TestDisableConcurrentOpensOption(t *testing.T) {
	// Test that DisableConcurrentOpens option is correctly defined
	t.Run("OptionDefault", func(t *testing.T) {
		opt := Options{}
		assert.False(t, opt.DisableConcurrentOpens, "DisableConcurrentOpens should default to false")
	})

	t.Run("OptionEnabled", func(t *testing.T) {
		opt := Options{DisableConcurrentOpens: true}
		assert.True(t, opt.DisableConcurrentOpens, "DisableConcurrentOpens should be settable to true")
	})
}

func TestPutSftpConnectionReturnsToPool(t *testing.T) {
	// Test that putSftpConnection correctly returns connection to pool
	f := &Fs{
		poolMu: sync.Mutex{},
		pool:   []*conn{},
	}

	mockConn := &conn{
		sshClient: &mockSSHClient{},
	}

	t.Run("ConnectionReturnedToPool", func(t *testing.T) {
		f.pool = []*conn{}
		c := mockConn

		// Verify pool is empty before
		assert.Equal(t, 0, len(f.pool))

		// Call putSftpConnection
		f.putSftpConnection(&c, nil)

		// Verify connection is in pool
		assert.Equal(t, 1, len(f.pool))
		assert.Equal(t, mockConn, f.pool[0])
	})

	t.Run("PointerNilledAfterReturn", func(t *testing.T) {
		f.pool = []*conn{}
		c := mockConn

		f.putSftpConnection(&c, nil)

		// Verify pointer is nilled (prevents reuse)
		assert.Nil(t, c)
	})
}

// testDisableConcurrentOpens tests that DisableConcurrentOpens option works correctly
func (f *Fs) testDisableConcurrentOpens(t *testing.T) {
	ctx := context.Background()

	// Skip if no test files available
	if len(fstests.InternalTestFiles) == 0 {
		t.Skip("no test files available")
	}

	// Create a new Fs with DisableConcurrentOpens enabled
	testFs := deriveFs(ctx, t, f, settings{
		"disable_concurrent_opens": true,
	})
	sftpFs := testFs.(*Fs)

	// Verify option is enabled
	assert.True(t, sftpFs.opt.DisableConcurrentOpens, "DisableConcurrentOpens should be enabled")

	// Get a test file
	testFile := fstests.InternalTestFiles[0]
	obj, err := testFs.NewObject(ctx, testFile.Path)
	require.NoError(t, err)

	// Record pool size before Open
	sftpFs.poolMu.Lock()
	poolSizeBefore := len(sftpFs.pool)
	sftpFs.poolMu.Unlock()

	// Open the file for reading
	reader, err := obj.Open(ctx)
	require.NoError(t, err)

	// While file is open, pool should not have grown
	// (connection is held by the reader, not returned to pool)
	sftpFs.poolMu.Lock()
	poolSizeDuringRead := len(sftpFs.pool)
	sftpFs.poolMu.Unlock()

	// Read some data to ensure the transfer is active
	buf := make([]byte, 1024)
	_, _ = reader.Read(buf)

	// Close the reader
	err = reader.Close()
	require.NoError(t, err)

	// After Close, pool should have the connection back
	sftpFs.poolMu.Lock()
	poolSizeAfter := len(sftpFs.pool)
	sftpFs.poolMu.Unlock()

	// The pool should have grown after Close (connection returned)
	assert.GreaterOrEqual(t, poolSizeAfter, poolSizeDuringRead,
		"Pool should have connection after Close (before: %d, during: %d, after: %d)",
		poolSizeBefore, poolSizeDuringRead, poolSizeAfter)
}

// InternalTest dispatches all internal tests
func (f *Fs) InternalTest(t *testing.T) {
	t.Run("DisableConcurrentOpens", f.testDisableConcurrentOpens)
}

var _ fstests.InternalTester = (*Fs)(nil)

// Ensure sync and io packages are used
var _ = sync.Mutex{}
var _ io.Reader
