package smb

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestResolveCcachePath(t *testing.T) {
	tmpDir := t.TempDir()

	// Setup: files for FILE and DIR modes
	fileCcache := filepath.Join(tmpDir, "file_ccache")
	err := os.WriteFile(fileCcache, []byte{}, 0600)
	assert.NoError(t, err)

	dirCcache := filepath.Join(tmpDir, "dir_ccache")
	err = os.Mkdir(dirCcache, 0755)
	assert.NoError(t, err)
	err = os.WriteFile(filepath.Join(dirCcache, "primary"), []byte("ticket"), 0600)
	assert.NoError(t, err)
	dirCcacheTicket := filepath.Join(dirCcache, "ticket")
	err = os.WriteFile(dirCcacheTicket, []byte{}, 0600)
	assert.NoError(t, err)

	tests := []struct {
		name          string
		ccachePath    string
		envKRB5CCNAME string
		expected      string
		expectError   bool
	}{
		{
			name:          "FILE: prefix from env",
			ccachePath:    "",
			envKRB5CCNAME: "FILE:" + fileCcache,
			expected:      fileCcache,
		},
		{
			name:          "DIR: prefix from env",
			ccachePath:    "",
			envKRB5CCNAME: "DIR:" + dirCcache,
			expected:      dirCcacheTicket,
		},
		{
			name:          "Unsupported prefix",
			ccachePath:    "",
			envKRB5CCNAME: "MEMORY:/bad/path",
			expectError:   true,
		},
		{
			name:       "Direct file path (no prefix)",
			ccachePath: "/tmp/myccache",
			expected:   "/tmp/myccache",
		},
		{
			name:          "Default to /tmp/krb5cc_<uid>",
			ccachePath:    "",
			envKRB5CCNAME: "",
			expected:      "/tmp/krb5cc_",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("KRB5CCNAME", tt.envKRB5CCNAME)
			result, err := resolveCcachePath(tt.ccachePath)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Contains(t, result, tt.expected)
			}
		})
	}
}
