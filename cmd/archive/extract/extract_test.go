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
			name:     "sanitized name returned",
			input:    "./subdir/file.txt",
			expected: "subdir/file.txt",
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
			name:    "leading dot-dot rejected",
			input:   "../etc/passwd",
			wantErr: true,
		},
		{
			name:    "interior dot-dot rejected with destination",
			input:   "dir/../../escaped.txt",
			dstDir:  "safe/prefix",
			wantErr: true,
		},
		{
			name:    "backslash dot-dot rejected",
			input:   `..\escaped.txt`,
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
