// Unit tests for internal SMB functions
package smb

import "testing"

// TestIsPathDir tests the isPathDir function logic
func TestIsPathDir(t *testing.T) {
	tests := []struct {
		path     string
		expected bool
	}{
		// Empty path should be considered a directory
		{"", true},

		// Paths with trailing slash should be directories
		{"/", true},
		{"share/", true},
		{"share/dir/", true},
		{"share/dir/subdir/", true},

		// Paths without trailing slash should not be directories
		{"share", false},
		{"share/dir", false},
		{"share/dir/file", false},
		{"share/dir/subdir/file", false},

		// Edge cases
		{"share//", true},
		{"share///", true},
		{"share/dir//", true},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			result := isPathDir(tt.path)
			if result != tt.expected {
				t.Errorf("isPathDir(%q) = %v, want %v", tt.path, result, tt.expected)
			}
		})
	}
}
