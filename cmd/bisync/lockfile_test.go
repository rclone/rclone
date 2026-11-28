package bisync

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/rclone/rclone/fs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestLockfileBisyncRun(t *testing.T, lockContent string, maxLock fs.Duration) *bisyncRun {
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

func TestLockfileIsExpired_UnreadableWithMaxLock(t *testing.T) {
	b := newTestLockfileBisyncRun(t, "not json!!!", fs.Duration(5*time.Minute))
	assert.True(t, b.lockFileIsExpired(), "unreadable lockfile with --max-lock set should be treated as expired")
}

func TestLockfileIsExpired_UnreadableWithoutMaxLock(t *testing.T) {
	b := newTestLockfileBisyncRun(t, "not json!!!", basicallyforever)
	assert.False(t, b.lockFileIsExpired(), "unreadable lockfile without --max-lock should not be treated as expired")
}

func TestLockfileIsExpired_EmptyWithMaxLock(t *testing.T) {
	b := newTestLockfileBisyncRun(t, "", fs.Duration(5*time.Minute))
	assert.True(t, b.lockFileIsExpired(), "empty lockfile with --max-lock set should be treated as expired")
}

func TestLockfileIsExpired_EmptyWithoutMaxLock(t *testing.T) {
	b := newTestLockfileBisyncRun(t, "", basicallyforever)
	assert.False(t, b.lockFileIsExpired(), "empty lockfile without --max-lock should not be treated as expired")
}

func TestLockfileIsExpired_ValidExpired(t *testing.T) {
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

	b := newTestLockfileBisyncRun(t, string(content), fs.Duration(5*time.Minute))
	assert.True(t, b.lockFileIsExpired(), "valid lockfile with past expiry should be expired")
}

func TestLockfileIsExpired_ValidNotExpired(t *testing.T) {
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

	b := newTestLockfileBisyncRun(t, string(content), fs.Duration(5*time.Minute))
	assert.False(t, b.lockFileIsExpired(), "valid lockfile with future expiry should not be expired")
}

func testSetLockFileRace(t *testing.T, maxLock fs.Duration) {
	t.Helper()
	const runners = 8
	const rounds = 50
	for round := range rounds {
		dir := t.TempDir()
		base := filepath.Join(dir, "session")
		lockPath := base + ".lck"
		require.NoError(t, os.WriteFile(filepath.Join(dir, "listing1"), nil, 0600))
		require.NoError(t, os.WriteFile(filepath.Join(dir, "listing2"), nil, 0600))

		start := make(chan struct{})
		type result struct {
			b   *bisyncRun
			err error
		}
		results := make(chan result, runners)
		var wg sync.WaitGroup
		for range runners {
			wg.Add(1)
			go func() {
				defer wg.Done()
				<-start
				b := &bisyncRun{
					basePath: base,
					opt:      &Options{MaxLock: maxLock},
					listing1: filepath.Join(dir, "listing1"),
					listing2: filepath.Join(dir, "listing2"),
				}
				results <- result{b: b, err: b.setLockFile()}
			}()
		}
		close(start)
		wg.Wait()
		close(results)

		winners := 0
		for res := range results {
			if res.err == nil {
				winners++
				if res.b.lockFileOpt.stopRenewal != nil {
					res.b.lockFileOpt.stopRenewal()
				}
			} else {
				assert.Contains(t, res.err.Error(), "prior lock file found")
				assert.Contains(t, res.err.Error(), "rclone deletefile")
			}
		}
		require.Equal(t, 1, winners, "round %d: exactly one runner must acquire the lock", round)

		content, err := os.ReadFile(lockPath)
		require.NoError(t, err)
		var lockData struct {
			Session     string
			PID         string
			TimeRenewed time.Time
			TimeExpires time.Time
		}
		require.NoError(t, json.Unmarshal(content, &lockData))
		assert.Equal(t, base, lockData.Session)
		assert.Equal(t, strconv.Itoa(os.Getpid()), lockData.PID)
		assert.False(t, lockData.TimeExpires.IsZero())
		assert.True(t, lockData.TimeExpires.After(lockData.TimeRenewed))

		require.NoError(t, os.Remove(lockPath))
	}
}

func TestSetLockFileRace(t *testing.T) {
	testSetLockFileRace(t, 0)
}

func TestSetLockFileRaceMaxLock(t *testing.T) {
	testSetLockFileRace(t, fs.Duration(5*time.Minute))
}

func TestSetLockFilePriorLockError(t *testing.T) {
	dir := t.TempDir()
	base := filepath.Join(dir, "session")
	lockPath := base + ".lck"
	data := struct {
		Session     string
		PID         string
		TimeRenewed time.Time
		TimeExpires time.Time
	}{
		Session:     "active-session",
		PID:         "99999",
		TimeRenewed: time.Now(),
		TimeExpires: time.Now().Add(5 * time.Minute),
	}
	content, err := json.Marshal(data)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(lockPath, content, 0600))

	b := &bisyncRun{
		basePath: base,
		opt:      &Options{MaxLock: fs.Duration(5 * time.Minute)},
	}
	err = b.setLockFile()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "prior lock file found")
	assert.Contains(t, err.Error(), "rclone deletefile")
}

func TestSetLockFileExpiredTakeover(t *testing.T) {
	dir := t.TempDir()
	base := filepath.Join(dir, "session")
	lockPath := base + ".lck"
	data := struct {
		Session     string
		PID         string
		TimeRenewed time.Time
		TimeExpires time.Time
	}{
		Session:     "old-session",
		PID:         "99999",
		TimeRenewed: time.Now().Add(-10 * time.Minute),
		TimeExpires: time.Now().Add(-5 * time.Minute),
	}
	content, err := json.Marshal(data)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(lockPath, content, 0600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "listing1"), nil, 0600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "listing2"), nil, 0600))

	b := &bisyncRun{
		basePath: base,
		opt:      &Options{MaxLock: fs.Duration(5 * time.Minute)},
		listing1: filepath.Join(dir, "listing1"),
		listing2: filepath.Join(dir, "listing2"),
	}
	require.NoError(t, b.setLockFile())
	defer b.lockFileOpt.stopRenewal()

	newContent, err := os.ReadFile(lockPath)
	require.NoError(t, err)
	assert.Contains(t, string(newContent), strconv.Itoa(os.Getpid()))
}
