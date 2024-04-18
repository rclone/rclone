//go:build !plan9

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
		got := shellUnEscape(test.escaped)
		assert.Equal(t, test.unescaped, got, fmt.Sprintf("Test %d unescaped = %q", i, test.unescaped))
	}
}
