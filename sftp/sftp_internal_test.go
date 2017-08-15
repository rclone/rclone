package sftp

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestShellEscape(t *testing.T) {
	for i, test := range []struct {
		unescaped, escaped string
	}{
		{"", ""},
		{"/this/is/harmless", "/this/is/harmless"},
		{"$(rm -rf /)", "\\$\\(rm\\ -rf\\ /\\)"},
		{"/test/\n", "/test/'\n'"},
		{":\"'", ":\\\"\\'"},
	} {
		got := shellEscape(test.unescaped)
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
