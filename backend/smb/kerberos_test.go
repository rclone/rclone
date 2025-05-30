package smb

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/jcmturner/gokrb5/v8/client"
	"github.com/jcmturner/gokrb5/v8/config"
	"github.com/jcmturner/gokrb5/v8/credentials"
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

func TestCreateKerberosClient_ReloadOnCcacheChange(t *testing.T) {

	// Create temporary fake ccache file
	tmpFile, err := os.CreateTemp("", "krb5cc_test")
	assert.NoError(t, err)
	defer os.Remove(tmpFile.Name())

	fakeCcacheContent := []byte("CCACHE_VERSION 4\n")
	_, err = tmpFile.Write(fakeCcacheContent)
	assert.NoError(t, err)
	assert.NoError(t, tmpFile.Close())

	// Patch functions
	origLoadCCache := loadCCacheFunc
	origNewFromCCache := newClientFromCCache
	origLoadKerberosConfig := loadKrbConfig

	defer func() {
		loadCCacheFunc = origLoadCCache
		newClientFromCCache = origNewFromCCache
		loadKrbConfig = origLoadKerberosConfig
	}()

	loadCallCount := 0
	loadCCacheFunc = func(path string) (*credentials.CCache, error) {
		loadCallCount++
		return &credentials.CCache{}, nil
	}
	newClientFromCCache = func(cc *credentials.CCache, cfg *config.Config, _ ...func(*client.Settings)) (*client.Client, error) {
		return &client.Client{}, nil
	}
	loadKrbConfig = func() (*config.Config, error) {
		return &config.Config{}, nil
	}

	// First call — should trigger load
	_, err = createKerberosClient(tmpFile.Name())
	assert.NoError(t, err)
	assert.Equal(t, 1, loadCallCount, "expected 1 load call")

	// Second call — should reuse cached client
	_, err = createKerberosClient(tmpFile.Name())
	assert.NoError(t, err)
	assert.Equal(t, 1, loadCallCount, "expected reuse on unchanged ccache")

	// Simulate file update
	time.Sleep(1 * time.Second) // ensure mtime actually changes
	err = os.WriteFile(tmpFile.Name(), []byte("CCACHE_VERSION 4\n#updated"), 0600)
	assert.NoError(t, err)

	// Third call — should detect change and reload
	_, err = createKerberosClient(tmpFile.Name())
	assert.NoError(t, err)
	assert.Equal(t, 2, loadCallCount, "expected reload on changed ccache")
}
