//go:build !plan9

package extract

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestStripDotSlashPrefix(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "strip leading dot-slash from file",
			input:    "./file.txt",
			expected: "file.txt",
		},
		{
			name:     "strip leading dot-slash from nested path",
			input:    "./subdir/file.txt",
			expected: "subdir/file.txt",
		},
		{
			name:     "no prefix unchanged",
			input:    "file.txt",
			expected: "file.txt",
		},
		{
			name:     "nested path unchanged",
			input:    "dir/file.txt",
			expected: "dir/file.txt",
		},
		{
			name:     "dot-dot-slash NOT stripped (path traversal safety)",
			input:    "../etc/passwd",
			expected: "../etc/passwd",
		},
		{
			name:     "dot-slash directory entry becomes empty",
			input:    "./",
			expected: "",
		},
		{
			name:     "only single leading dot-slash stripped",
			input:    "././file.txt",
			expected: "./file.txt",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// This mirrors the stripping logic in ArchiveExtract
			got := strings.TrimPrefix(tc.input, "./")
			assert.Equal(t, tc.expected, got)
		})
	}
}
