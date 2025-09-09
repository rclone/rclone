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

func TestKerberosFactory_GetClient_ReloadOnCcacheChange(t *testing.T) {
	// Create temp ccache file
	tmpFile, err := os.CreateTemp("", "krb5cc_test")
	assert.NoError(t, err)
	defer func() {
		if err := os.Remove(tmpFile.Name()); err != nil {
			t.Logf("Failed to remove temp file %s: %v", tmpFile.Name(), err)
		}
	}()

	unixPath := filepath.ToSlash(tmpFile.Name())
	ccachePath := "FILE:" + unixPath

	initialContent := []byte("CCACHE_VERSION 4\n")
	_, err = tmpFile.Write(initialContent)
	assert.NoError(t, err)
	assert.NoError(t, tmpFile.Close())

	// Setup mocks
	loadCallCount := 0
	mockLoadCCache := func(path string) (*credentials.CCache, error) {
		loadCallCount++
		return &credentials.CCache{}, nil
	}

	mockNewClient := func(cc *credentials.CCache, cfg *config.Config, opts ...func(*client.Settings)) (*client.Client, error) {
		return &client.Client{}, nil
	}

	mockLoadConfig := func() (*config.Config, error) {
		return &config.Config{}, nil
	}
	factory := &KerberosFactory{
		loadCCache: mockLoadCCache,
		newClient:  mockNewClient,
		loadConfig: mockLoadConfig,
	}

	// First call — triggers loading
	_, err = factory.GetClient(ccachePath)
	assert.NoError(t, err)
	assert.Equal(t, 1, loadCallCount, "expected 1 load call")

	// Second call — should reuse cache, no additional load
	_, err = factory.GetClient(ccachePath)
	assert.NoError(t, err)
	assert.Equal(t, 1, loadCallCount, "expected cached reuse, no new load")

	// Simulate file update
	time.Sleep(1 * time.Second) // ensure mtime changes
	err = os.WriteFile(tmpFile.Name(), []byte("CCACHE_VERSION 4\n#updated"), 0600)
	assert.NoError(t, err)

	// Third call — should detect change, reload
	_, err = factory.GetClient(ccachePath)
	assert.NoError(t, err)
	assert.Equal(t, 2, loadCallCount, "expected reload on changed ccache")
}
