//go:build !plan9

package extract

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDestPath(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		dstDir   string
		expected string
		wantErr  bool
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
			name:     "joined onto destination directory",
			input:    "file.txt",
			dstDir:   "safe/prefix",
			expected: "safe/prefix/file.txt",
		},
		{
			name:     "archive root entry skipped",
			input:    "./",
			expected: "",
		},
		{
			name:     "only single leading dot-slash stripped",
			input:    "././file.txt",
			expected: "./file.txt",
		},
		{
			name:    "leading dot-dot rejected",
			input:   "../etc/passwd",
			wantErr: true,
		},
		{
			name:    "leading dot-dot rejected with destination",
			input:   "../escaped.txt",
			dstDir:  "safe/prefix",
			wantErr: true,
		},
		{
			name:    "interior dot-dot rejected",
			input:   "dir/../../escaped.txt",
			dstDir:  "safe/prefix",
			wantErr: true,
		},
		{
			name:    "trailing dot-dot rejected",
			input:   "dir/..",
			wantErr: true,
		},
		{
			name:    "backslash dot-dot rejected",
			input:   `..\escaped.txt`,
			wantErr: true,
		},
		{
			name:    "nested backslash dot-dot rejected",
			input:   `dir\..\..\escaped.txt`,
			dstDir:  "safe/prefix",
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := destPath(tc.input, tc.dstDir)
			if tc.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.expected, got)
		})
	}
}
