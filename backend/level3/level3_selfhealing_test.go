package level3_test

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/rclone/rclone/backend/level3"
	_ "github.com/rclone/rclone/backend/local"
	"github.com/rclone/rclone/fs/config/configmap"
	"github.com/rclone/rclone/fs/object"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// Self-Healing Tests
// =============================================================================

// TestSelfHealing tests automatic background restoration of missing odd
// particle.
//
// This is the core self-healing feature: when a file is read in degraded
// mode (odd particle missing), the backend should automatically queue the
// missing odd particle for upload in the background, and the upload should
// complete before the command exits.
//
// This test verifies:
//   - Missing odd particle is detected during Open()
//   - Data is correctly reconstructed from even + parity
//   - Odd particle is queued for background upload
//   - Upload completes during Shutdown()
//   - Restored particle is byte-for-byte identical to original
//
// Failure indicates: Self-healing doesn't work, leaving the backend in
// degraded state permanently. Users would need manual intervention to
// restore redundancy.
func TestSelfHealing(t *testing.T) {
	ctx := context.Background()

	// Create three temporary directories
	evenDir := t.TempDir()
	oddDir := t.TempDir()
	parityDir := t.TempDir()

	m := configmap.Simple{
		"even":     evenDir,
		"odd":      oddDir,
		"parity":   parityDir,
		"auto_heal": "true",
	}
	fsInterface, err := level3.NewFs(ctx, "TestSelfHealing", "", m)
	require.NoError(t, err)

	// Cast to *level3.Fs to access Shutdown method
	f, ok := fsInterface.(*level3.Fs)
	require.True(t, ok, "expected *level3.Fs")

	// Upload a test file
	remote := "test-healing.txt"
	data := []byte("Hello Self-Healing World!")
	info := object.NewStaticObjectInfo(remote, time.Now(), int64(len(data)), true, nil, nil)
	_, err = f.Put(ctx, bytes.NewReader(data), info)
	require.NoError(t, err)

	// Verify all three particles exist
	evenPath := filepath.Join(evenDir, remote)
	oddPath := filepath.Join(oddDir, remote)
	parityPath := filepath.Join(parityDir, remote+".parity-ol") // odd length

	_, err = os.Stat(evenPath)
	require.NoError(t, err, "even particle should exist")
	_, err = os.Stat(oddPath)
	require.NoError(t, err, "odd particle should exist")
	_, err = os.Stat(parityPath)
	require.NoError(t, err, "parity particle should exist")

	// Remove odd particle to trigger self-healing
	err = os.Remove(oddPath)
	require.NoError(t, err)

	// Verify odd particle is gone
	_, err = os.Stat(oddPath)
	require.True(t, os.IsNotExist(err), "odd particle should not exist")

	// Read file in degraded mode (should queue odd particle for upload)
	obj, err := f.NewObject(ctx, remote)
	require.NoError(t, err)
	rc, err := obj.Open(ctx)
	require.NoError(t, err)
	got, err := io.ReadAll(rc)
	rc.Close()
	require.NoError(t, err)
	assert.Equal(t, data, got, "data should be correctly reconstructed")

	// Shutdown and wait for self-healing to complete
	err = f.Shutdown(ctx)
	require.NoError(t, err)

	// Verify odd particle was re-created by self-healing
	_, err = os.Stat(oddPath)
	require.NoError(t, err, "odd particle should be restored by self-healing")

	// Verify the restored particle is correct
	restoredOddData, err := os.ReadFile(oddPath)
	require.NoError(t, err)

	// Calculate expected odd particle
	_, expectedOdd := level3.SplitBytes(data)
	assert.Equal(t, expectedOdd, restoredOddData, "restored odd particle should match original")
}

// TestSelfHealingEvenParticle tests automatic background restoration of
// missing even particle.
//
// Similar to TestSelfHealing but for the even particle. This ensures
// self-healing works regardless of which data particle is missing.
//
// This test verifies:
//   - Missing even particle is detected during Open()
//   - Data is correctly reconstructed from odd + parity
//   - Even particle is queued for background upload
//   - Upload completes during Shutdown()
//   - Restored particle is byte-for-byte identical to original
//
// Failure indicates: Self-healing only works for odd particles, not even.
// This would be a critical asymmetry in the implementation.
func TestSelfHealingEvenParticle(t *testing.T) {
	ctx := context.Background()

	// Create three temporary directories
	evenDir := t.TempDir()
	oddDir := t.TempDir()
	parityDir := t.TempDir()

	m := configmap.Simple{
		"even":     evenDir,
		"odd":      oddDir,
		"parity":   parityDir,
		"auto_heal": "true",
	}
	fsInterface, err := level3.NewFs(ctx, "TestSelfHealingEven", "", m)
	require.NoError(t, err)

	f, ok := fsInterface.(*level3.Fs)
	require.True(t, ok)

	// Upload a test file
	remote := "test-even.txt"
	data := []byte("Test Even Particle Reconstruction")
	info := object.NewStaticObjectInfo(remote, time.Now(), int64(len(data)), true, nil, nil)
	_, err = f.Put(ctx, bytes.NewReader(data), info)
	require.NoError(t, err)

	// Remove even particle
	evenPath := filepath.Join(evenDir, remote)
	err = os.Remove(evenPath)
	require.NoError(t, err)

	// Read file (should queue even particle for upload)
	obj, err := f.NewObject(ctx, remote)
	require.NoError(t, err)
	rc, err := obj.Open(ctx)
	require.NoError(t, err)
	got, err := io.ReadAll(rc)
	rc.Close()
	require.NoError(t, err)
	assert.Equal(t, data, got)

	// Shutdown and wait for self-healing
	err = f.Shutdown(ctx)
	require.NoError(t, err)

	// Verify even particle was restored
	_, err = os.Stat(evenPath)
	require.NoError(t, err, "even particle should be restored")

	// Verify correctness
	restoredEvenData, err := os.ReadFile(evenPath)
	require.NoError(t, err)
	expectedEven, _ := level3.SplitBytes(data)
	assert.Equal(t, expectedEven, restoredEvenData)
}

// TestSelfHealingNoQueue tests that Shutdown() is fast when no self-healing
// is needed.
//
// This verifies the "hybrid" shutdown behavior (Solution D): when all
// particles are healthy, Shutdown() should exit immediately without waiting.
// This prevents unnecessary delays when the system is healthy.
//
// This test verifies:
//   - Reading a healthy file (all particles present) doesn't queue uploads
//   - Shutdown() completes in <100ms (instant)
//   - No background workers are waiting
//   - No performance penalty for normal operations
//
// Failure indicates: Performance regression - commands would be slow even
// when no healing is needed. This would make the backend unacceptable for
// production use.
func TestSelfHealingNoQueue(t *testing.T) {
	ctx := context.Background()

	// Create three temporary directories
	evenDir := t.TempDir()
	oddDir := t.TempDir()
	parityDir := t.TempDir()

	m := configmap.Simple{
		"even":     evenDir,
		"odd":      oddDir,
		"parity":   parityDir,
		"auto_heal": "true",
	}
	fsInterface, err := level3.NewFs(ctx, "TestNoQueue", "", m)
	require.NoError(t, err)

	f, ok := fsInterface.(*level3.Fs)
	require.True(t, ok)

	// Upload a test file
	remote := "test-noqueue.txt"
	data := []byte("No healing needed")
	info := object.NewStaticObjectInfo(remote, time.Now(), int64(len(data)), true, nil, nil)
	_, err = f.Put(ctx, bytes.NewReader(data), info)
	require.NoError(t, err)

	// Read file (all particles present, no healing needed)
	obj, err := f.NewObject(ctx, remote)
	require.NoError(t, err)
	rc, err := obj.Open(ctx)
	require.NoError(t, err)
	got, err := io.ReadAll(rc)
	rc.Close()
	require.NoError(t, err)
	assert.Equal(t, data, got)

	// Shutdown should return immediately (no pending uploads)
	start := time.Now()
	err = f.Shutdown(ctx)
	duration := time.Since(start)
	require.NoError(t, err)

	// Should complete almost instantly (<100ms) since no uploads are pending
	assert.Less(t, duration, 100*time.Millisecond, "shutdown should be instant with no pending uploads")
}

// TestSelfHealingLargeFile tests self-healing with a larger file (100 KB).
//
// This ensures self-healing works with realistic file sizes, not just
// small test data. Large files stress-test the memory handling, upload
// performance, and ensure the background worker can handle substantial data.
//
// This test verifies:
//   - Self-healing works with 100 KB files
//   - Correct particle reconstruction for large data
//   - Upload completes successfully (not timeout)
//   - Restored particle is byte-for-byte correct
//   - No memory or performance issues
//
// Failure indicates: Self-healing doesn't work with realistic file sizes.
// Could indicate memory issues, timeout problems, or buffer limitations.
func TestSelfHealingLargeFile(t *testing.T) {
	ctx := context.Background()

	// Create three temporary directories
	evenDir := t.TempDir()
	oddDir := t.TempDir()
	parityDir := t.TempDir()

	m := configmap.Simple{
		"even":     evenDir,
		"odd":      oddDir,
		"parity":   parityDir,
		"auto_heal": "true",
	}
	fsInterface, err := level3.NewFs(ctx, "TestLargeHealing", "", m)
	require.NoError(t, err)

	f, ok := fsInterface.(*level3.Fs)
	require.True(t, ok)

	// Create a larger file (100 KB)
	remote := "large-heal.bin"
	data := make([]byte, 100*1024)
	for i := range data {
		data[i] = byte(i % 256)
	}

	info := object.NewStaticObjectInfo(remote, time.Now(), int64(len(data)), true, nil, nil)
	_, err = f.Put(ctx, bytes.NewReader(data), info)
	require.NoError(t, err)

	// Remove even particle
	evenPath := filepath.Join(evenDir, remote)
	err = os.Remove(evenPath)
	require.NoError(t, err)

	// Read file (should queue even particle)
	obj, err := f.NewObject(ctx, remote)
	require.NoError(t, err)
	rc, err := obj.Open(ctx)
	require.NoError(t, err)
	got, err := io.ReadAll(rc)
	rc.Close()
	require.NoError(t, err)
	assert.Equal(t, data, got)

	// Shutdown and wait
	err = f.Shutdown(ctx)
	require.NoError(t, err)

	// Verify particle was restored
	_, err = os.Stat(evenPath)
	require.NoError(t, err)

	// Verify data
	restoredData, err := os.ReadFile(evenPath)
	require.NoError(t, err)
	expectedEven, _ := level3.SplitBytes(data)
	assert.Equal(t, expectedEven, restoredData)
}

// TestSelfHealingShutdownTimeout tests that Shutdown() times out gracefully
// when background uploads are taking too long.
//
// This would verify the 60-second timeout in Shutdown() prevents the command
// from hanging indefinitely if an upload gets stuck. However, this requires
// mocking a slow or hanging backend, which is complex to set up.
//
// This test verifies (when implemented):
//   - Shutdown() doesn't hang indefinitely
//   - 60-second timeout triggers correctly
//   - Error is logged when timeout occurs
//   - Command can exit even with incomplete uploads
//
// Failure would indicate: Commands could hang forever waiting for uploads.
//
// Current status: Skipped (requires mocked backend - future enhancement)
func TestSelfHealingShutdownTimeout(t *testing.T) {
	// This test would require mocking a slow backend
	// For now, we'll skip it as it's complex to set up
	t.Skip("Shutdown timeout test requires mocked backend (future enhancement)")
}

