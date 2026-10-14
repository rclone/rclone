package sanitize

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPath(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
		wantErr  bool
	}{
		{
			name:     "plain file unchanged",
			input:    "file.txt",
			expected: "file.txt",
		},
		{
			name:     "nested path unchanged",
			input:    "dir/file.txt",
			expected: "dir/file.txt",
		},
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
			name:     "strip repeated leading dot-slash",
			input:    "././file.txt",
			expected: "file.txt",
		},
		{
			name:     "strip interior dot component",
			input:    "dir/./file.txt",
			expected: "dir/file.txt",
		},
		{
			name:     "strip leading slash",
			input:    "/dir/file.txt",
			expected: "dir/file.txt",
		},
		{
			name:     "strip trailing slash from directory",
			input:    "dir/",
			expected: "dir",
		},
		{
			name:     "collapse doubled slashes",
			input:    "dir//file.txt",
			expected: "dir/file.txt",
		},
		{
			name:     "empty name is the root",
			input:    "",
			expected: "",
		},
		{
			name:     "dot-slash is the root",
			input:    "./",
			expected: "",
		},
		{
			name:     "dot is the root",
			input:    ".",
			expected: "",
		},
		{
			name:     "slash is the root",
			input:    "/",
			expected: "",
		},
		{
			name:     "three dots allowed",
			input:    "dir/...",
			expected: "dir/...",
		},
		{
			name:     "backslash kept in file name",
			input:    `dir/back\slash.txt`,
			expected: `dir/back\slash.txt`,
		},
		{
			name:    "leading dot-dot rejected",
			input:   "../etc/passwd",
			wantErr: true,
		},
		{
			name:    "interior dot-dot rejected",
			input:   "dir/../../escaped.txt",
			wantErr: true,
		},
		{
			name:    "trailing dot-dot rejected",
			input:   "dir/..",
			wantErr: true,
		},
		{
			name:    "bare dot-dot rejected",
			input:   "..",
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
			wantErr: true,
		},
		{
			name:    "mixed separator dot-dot rejected",
			input:   `dir/..\escaped.txt`,
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := Path(tc.input)
			if tc.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.expected, got)
		})
	}
}

func TestLeaf(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{name: "plain name", input: "file.txt"},
		{name: "name with dots", input: "..."},
		{name: "hidden name", input: ".hidden"},
		{name: "empty rejected", input: "", wantErr: true},
		{name: "dot rejected", input: ".", wantErr: true},
		{name: "dot-dot rejected", input: "..", wantErr: true},
		{name: "slash rejected", input: "a/b", wantErr: true},
		{name: "backslash allowed", input: `a\b`},
		{name: "leading slash rejected", input: "/etc", wantErr: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := Leaf(tc.input)
			if tc.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
		})
	}
}
