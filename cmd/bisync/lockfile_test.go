package bisync

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/rclone/rclone/fs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestBisyncRun(t *testing.T, lockContent string, maxLock fs.Duration) *bisyncRun {
	t.Helper()
	dir := t.TempDir()
	lockPath := filepath.Join(dir, "test.lck")
	require.NoError(t, os.WriteFile(lockPath, []byte(lockContent), 0600))

	listing1 := filepath.Join(dir, "listing1")
	listing2 := filepath.Join(dir, "listing2")
	require.NoError(t, os.WriteFile(listing1, []byte(""), 0600))
	require.NoError(t, os.WriteFile(listing2, []byte(""), 0600))

	return &bisyncRun{
		lockFile: lockPath,
		opt:      &Options{MaxLock: maxLock},
		listing1: listing1,
		listing2: listing2,
	}
}

func TestLockFileIsExpired_GarbledWithMaxLock(t *testing.T) {
	b := newTestBisyncRun(t, "not json!!!", fs.Duration(5*time.Minute))
	assert.True(t, b.lockFileIsExpired(), "garbled lockfile with --max-lock set should be treated as expired")
}

func TestLockFileIsExpired_GarbledWithoutMaxLock(t *testing.T) {
	b := newTestBisyncRun(t, "not json!!!", basicallyforever)
	assert.False(t, b.lockFileIsExpired(), "garbled lockfile without --max-lock should not be treated as expired")
}

func TestLockFileIsExpired_ValidExpired(t *testing.T) {
	data := struct {
		Session     string
		PID         string
		TimeRenewed time.Time
		TimeExpires time.Time
	}{
		Session:     "test",
		PID:         "12345",
		TimeRenewed: time.Now().Add(-10 * time.Minute),
		TimeExpires: time.Now().Add(-5 * time.Minute),
	}
	content, err := json.Marshal(data)
	require.NoError(t, err)

	b := newTestBisyncRun(t, string(content), fs.Duration(5*time.Minute))
	assert.True(t, b.lockFileIsExpired(), "valid lockfile with past expiry should be expired")
}

func TestLockFileIsExpired_ValidNotExpired(t *testing.T) {
	data := struct {
		Session     string
		PID         string
		TimeRenewed time.Time
		TimeExpires time.Time
	}{
		Session:     "test",
		PID:         "12345",
		TimeRenewed: time.Now(),
		TimeExpires: time.Now().Add(10 * time.Minute),
	}
	content, err := json.Marshal(data)
	require.NoError(t, err)

	b := newTestBisyncRun(t, string(content), fs.Duration(5*time.Minute))
	assert.False(t, b.lockFileIsExpired(), "valid lockfile with future expiry should not be expired")
}
