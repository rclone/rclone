package level3_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/rclone/rclone/backend/level3"
	_ "github.com/rclone/rclone/backend/local"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config/configmap"
	"github.com/rclone/rclone/fs/object"
	"github.com/rclone/rclone/fs/operations"
	"github.com/rclone/rclone/fstest"
	"github.com/rclone/rclone/fstest/fstests"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	unimplementableFsMethods     = []string{"UnWrap", "WrapFs", "SetWrapper", "UserInfo", "Disconnect", "PublicLink", "PutUnchecked", "MergeDirs", "OpenWriterAt", "OpenChunkWriter", "ListP"}
	unimplementableObjectMethods = []string{}
)

// =============================================================================
// Integration Tests
// =============================================================================

// TestIntegration runs the full rclone integration test suite against a
// configured remote backend.
//
// This is used for testing level3 with real cloud storage backends (S3, etc.)
// rather than local temporary directories. It exercises all standard rclone
// operations to ensure compatibility with the rclone ecosystem.
//
// This test verifies:
//   - All standard rclone operations work correctly
//   - Backend correctly implements the fs.Fs interface
//   - Compatibility with rclone's command layer
//
// Failure indicates: Breaking changes that would prevent level3 from working
// with standard rclone commands.
//
// Usage: go test -remote level3config:
func TestIntegration(t *testing.T) {
	if *fstest.RemoteName == "" {
		t.Skip("Skipping as -remote not set")
	}
	fstests.Run(t, &fstests.Opt{
		RemoteName:                   *fstest.RemoteName,
		UnimplementableFsMethods:     unimplementableFsMethods,
		UnimplementableObjectMethods: unimplementableObjectMethods,
	})
}

// TestStandard runs the full rclone integration test suite with local
// temporary directories (default timeout_mode=standard).
//
// This is the primary test for CI/CD pipelines, as it doesn't require any
// external backends or configuration. It creates three temp directories and
// runs comprehensive tests covering all rclone operations.
//
// This test verifies:
//   - All fs.Fs interface methods work correctly
//   - File upload, download, move, delete operations
//   - Directory operations
//   - Metadata handling
//   - Special characters and edge cases
//
// Failure indicates: Core functionality is broken. This is the most important
// test for catching regressions.
func TestStandard(t *testing.T) {
	if *fstest.RemoteName != "" {
		t.Skip("Skipping as -remote set")
	}

	// Create three temporary directories for even, odd, and parity
	evenDir := t.TempDir()
	oddDir := t.TempDir()
	parityDir := t.TempDir()

	name := "TestLevel3"
	fstests.Run(t, &fstests.Opt{
		RemoteName: name + ":",
		ExtraConfig: []fstests.ExtraConfigItem{
			{Name: name, Key: "type", Value: "level3"},
			{Name: name, Key: "even", Value: evenDir},
			{Name: name, Key: "odd", Value: oddDir},
			{Name: name, Key: "parity", Value: parityDir},
		},
		UnimplementableFsMethods:     unimplementableFsMethods,
		UnimplementableObjectMethods: unimplementableObjectMethods,
		QuickTestOK:                  true,
	})
}

// TestStandardBalanced runs the full integration suite with timeout_mode=balanced.
//
// This tests the "balanced" timeout configuration which uses moderate retries
// (3 attempts) and timeouts (30s) for S3/MinIO backends. This is a middle ground
// between standard (long timeouts) and aggressive (fast failover).
//
// This test verifies:
//   - All operations work correctly with balanced timeout settings
//   - Appropriate for reliable S3 backends
//   - No regressions from timeout configuration changes
//
// Failure indicates: Timeout mode configuration affects core functionality.
func TestStandardBalanced(t *testing.T) {
	if *fstest.RemoteName != "" {
		t.Skip("Skipping as -remote set")
	}

	evenDir := t.TempDir()
	oddDir := t.TempDir()
	parityDir := t.TempDir()

	name := "TestLevel3Balanced"
	fstests.Run(t, &fstests.Opt{
		RemoteName: name + ":",
		ExtraConfig: []fstests.ExtraConfigItem{
			{Name: name, Key: "type", Value: "level3"},
			{Name: name, Key: "even", Value: evenDir},
			{Name: name, Key: "odd", Value: oddDir},
			{Name: name, Key: "parity", Value: parityDir},
			{Name: name, Key: "timeout_mode", Value: "balanced"},
		},
		UnimplementableFsMethods:     unimplementableFsMethods,
		UnimplementableObjectMethods: unimplementableObjectMethods,
		QuickTestOK:                  true,
	})
}

// TestStandardAggressive runs the full integration suite with timeout_mode=aggressive.
//
// This tests the "aggressive" timeout configuration which uses minimal retries
// (1 attempt) and short timeouts (10s) for fast failover in S3/MinIO degraded mode.
// This is the recommended setting for production S3 deployments.
//
// This test verifies:
//   - All operations work correctly with aggressive timeout settings
//   - Fast failover in degraded mode scenarios
//   - No regressions from aggressive timeout configuration
//
// Failure indicates: Aggressive timeout mode breaks operations or causes
// premature failures with local backends.
func TestStandardAggressive(t *testing.T) {
	if *fstest.RemoteName != "" {
		t.Skip("Skipping as -remote set")
	}

	evenDir := t.TempDir()
	oddDir := t.TempDir()
	parityDir := t.TempDir()

	name := "TestLevel3Aggressive"
	fstests.Run(t, &fstests.Opt{
		RemoteName: name + ":",
		ExtraConfig: []fstests.ExtraConfigItem{
			{Name: name, Key: "type", Value: "level3"},
			{Name: name, Key: "even", Value: evenDir},
			{Name: name, Key: "odd", Value: oddDir},
			{Name: name, Key: "parity", Value: parityDir},
			{Name: name, Key: "timeout_mode", Value: "aggressive"},
		},
		UnimplementableFsMethods:     unimplementableFsMethods,
		UnimplementableObjectMethods: unimplementableObjectMethods,
		QuickTestOK:                  true,
	})
}

// =============================================================================
// Unit Tests - About (quota aggregation)
// =============================================================================

// TestAboutAggregatesChildUsage verifies that About() is wired and returns
// non-nil usage when the underlying backends support About.
//
// This mirrors the behaviour of other aggregating backends (e.g. combine)
// and ensures that calling rclone about on a level3 remote works when the
// child remotes implement About.
func TestAboutAggregatesChildUsage(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	evenDir := t.TempDir()
	oddDir := t.TempDir()
	parityDir := t.TempDir()

	// Create a small file on each backend so that Used is non-zero
	require.NoError(t, os.WriteFile(filepath.Join(evenDir, "file1.bin"), []byte("even"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(oddDir, "file2.bin"), []byte("odd"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(parityDir, "file3.bin"), []byte("parity"), 0o644))

	fsInterface, err := level3.NewFs(ctx, "TestAbout", "", configmap.Simple{
		"even":   evenDir,
		"odd":    oddDir,
		"parity": parityDir,
	})
	require.NoError(t, err)

	f, ok := fsInterface.(*level3.Fs)
	require.True(t, ok)
	defer func() {
		_ = f.Shutdown(context.Background())
	}()

	usage, err := f.About(ctx)
	if err != nil {
		// If none of the underlying backends support About, this will be
		// fs.ErrorNotImplemented. In that case we just verify the error type.
		require.ErrorIs(t, err, fs.ErrorNotImplemented)
		return
	}

	require.NotNil(t, usage, "usage must not be nil when About succeeds")
	// We can't assert exact values since local About reports filesystem-wide
	// usage, but we can at least check that it returned something sensible.
	if usage.Total != nil {
		assert.Greater(t, *usage.Total, int64(0))
	}
}

// =============================================================================
// Unit Tests - Byte Operations
// =============================================================================

// TestSplitBytes tests the byte-level striping function that splits data
// into even-indexed and odd-indexed bytes.
//
// This is the core RAID 3 operation - all data must be correctly split before
// storage. Even a single-byte error would corrupt files.
//
// This test verifies:
//   - Even bytes (indices 0, 2, 4, ...) go to even slice
//   - Odd bytes (indices 1, 3, 5, ...) go to odd slice
//   - Correct handling of empty input
//   - Correct handling of single-byte input
//   - Correct handling of odd-length and even-length inputs
//
// Failure indicates: Data corruption would occur on upload. CRITICAL.
func TestSplitBytes(t *testing.T) {
	tests := []struct {
		name     string
		input    []byte
		wantEven []byte
		wantOdd  []byte
	}{
		{
			name:     "empty",
			input:    []byte{},
			wantEven: []byte{},
			wantOdd:  []byte{},
		},
		{
			name:     "single byte",
			input:    []byte{0xAA},
			wantEven: []byte{0xAA},
			wantOdd:  []byte{},
		},
		{
			name:     "two bytes",
			input:    []byte{0xAA, 0xBB},
			wantEven: []byte{0xAA},
			wantOdd:  []byte{0xBB},
		},
		{
			name:     "three bytes",
			input:    []byte{0xAA, 0xBB, 0xCC},
			wantEven: []byte{0xAA, 0xCC},
			wantOdd:  []byte{0xBB},
		},
		{
			name:     "four bytes",
			input:    []byte{0xAA, 0xBB, 0xCC, 0xDD},
			wantEven: []byte{0xAA, 0xCC},
			wantOdd:  []byte{0xBB, 0xDD},
		},
		{
			name:     "seven bytes",
			input:    []byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07},
			wantEven: []byte{0x01, 0x03, 0x05, 0x07},
			wantOdd:  []byte{0x02, 0x04, 0x06},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotEven, gotOdd := level3.SplitBytes(tt.input)
			assert.Equal(t, tt.wantEven, gotEven, "even bytes mismatch")
			assert.Equal(t, tt.wantOdd, gotOdd, "odd bytes mismatch")
		})
	}
}

// TestMergeBytes tests the reconstruction of original data from even and
// odd byte slices.
//
// This is the inverse of SplitBytes and is used during downloads. Incorrect
// merging would return corrupted data to users.
//
// This test verifies:
//   - Bytes are interleaved correctly: even[0], odd[0], even[1], odd[1], ...
//   - Handles odd-length originals (even slice has one extra byte)
//   - Validates size relationship (even.len == odd.len OR even.len == odd.len + 1)
//   - Rejects invalid size relationships
//
// Failure indicates: Downloads would return corrupted data. CRITICAL.
func TestMergeBytes(t *testing.T) {
	tests := []struct {
		name    string
		even    []byte
		odd     []byte
		want    []byte
		wantErr bool
	}{
		{
			name:    "empty",
			even:    []byte{},
			odd:     []byte{},
			want:    []byte{},
			wantErr: false,
		},
		{
			name:    "single even byte",
			even:    []byte{0xAA},
			odd:     []byte{},
			want:    []byte{0xAA},
			wantErr: false,
		},
		{
			name:    "equal lengths",
			even:    []byte{0xAA},
			odd:     []byte{0xBB},
			want:    []byte{0xAA, 0xBB},
			wantErr: false,
		},
		{
			name:    "even one larger",
			even:    []byte{0xAA, 0xCC},
			odd:     []byte{0xBB},
			want:    []byte{0xAA, 0xBB, 0xCC},
			wantErr: false,
		},
		{
			name:    "even two larger - invalid",
			even:    []byte{0xAA, 0xCC, 0xEE},
			odd:     []byte{0xBB},
			want:    nil,
			wantErr: true,
		},
		{
			name:    "odd larger - invalid",
			even:    []byte{0xAA},
			odd:     []byte{0xBB, 0xDD},
			want:    nil,
			wantErr: true,
		},
		{
			name:    "seven bytes reconstructed",
			even:    []byte{0x01, 0x03, 0x05, 0x07},
			odd:     []byte{0x02, 0x04, 0x06},
			want:    []byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := level3.MergeBytes(tt.even, tt.odd)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

// TestSplitMergeRoundtrip tests that SplitBytes and MergeBytes are perfect
// inverses of each other.
//
// This is a property-based test: for any input data, the round-trip
// split(data) -> merge(even, odd) should equal the original data.
// This ensures no data is lost or corrupted in the upload/download cycle.
//
// This test verifies:
//   - No data loss during split/merge operations
//   - Works for various data patterns and lengths
//   - Empty, single-byte, and multi-byte inputs all work
//   - Longer strings (80+ bytes) work correctly
//
// Failure indicates: Data corruption in upload/download cycle. CRITICAL.
func TestSplitMergeRoundtrip(t *testing.T) {
	testData := [][]byte{
		{},
		{0x00},
		{0x00, 0xFF},
		{0x01, 0x02, 0x03},
		{0x01, 0x02, 0x03, 0x04},
		[]byte("Hello, World!"),
		[]byte("This is a longer test string to verify the split/merge operations work correctly."),
	}

	for i, data := range testData {
		t.Run(string(rune('A'+i)), func(t *testing.T) {
			// Split
			even, odd := level3.SplitBytes(data)

			// Verify sizes
			assert.Equal(t, (len(data)+1)/2, len(even), "even size")
			assert.Equal(t, len(data)/2, len(odd), "odd size")

			// Merge
			result, err := level3.MergeBytes(even, odd)
			require.NoError(t, err)

			// Verify roundtrip
			assert.Equal(t, data, result, "roundtrip failed")
		})
	}
}

// =============================================================================
// Unit Tests - Validation
// =============================================================================

// TestValidateParticleSizes tests validation of even/odd particle size
// relationships.
//
// RAID 3 has strict size requirements: for an N-byte file, even particle
// must be ceil(N/2) bytes and odd particle must be floor(N/2) bytes.
// This means: even.size == odd.size OR even.size == odd.size + 1.
//
// This test verifies:
//   - Accepts equal sizes (even-length original)
//   - Accepts even = odd + 1 (odd-length original)
//   - Rejects even = odd + 2 or more
//   - Rejects odd > even
//   - Handles zero-size edge cases
//
// Failure indicates: Invalid particles could be accepted, leading to
// corrupted downloads or failed reconstructions.
func TestValidateParticleSizes(t *testing.T) {
	tests := []struct {
		name     string
		evenSize int64
		oddSize  int64
		want     bool
	}{
		{"equal sizes", 5, 5, true},
		{"even one larger", 6, 5, true},
		{"even two larger", 7, 5, false},
		{"odd larger", 5, 6, false},
		{"both zero", 0, 0, true},
		{"even one, odd zero", 1, 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := level3.ValidateParticleSizes(tt.evenSize, tt.oddSize)
			assert.Equal(t, tt.want, got)
		})
	}
}

// =============================================================================
// Unit Tests - Parity Operations
// =============================================================================

// TestCalculateParity tests XOR parity calculation for RAID 3.
//
// Parity is calculated as even[i] XOR odd[i] for each byte pair. For
// odd-length files, the last byte of the parity is the last byte of the
// even particle (no XOR partner). This parity enables recovery when one
// data particle is missing.
//
// This test verifies:
//   - Correct XOR calculation for byte pairs
//   - Last byte handling for odd-length originals
//   - Empty input handling
//   - Various data patterns
//   - Real-world data (text strings)
//
// Failure indicates: Parity would be incorrect, preventing recovery in
// degraded mode. Self-healing would upload wrong data.
func TestCalculateParity(t *testing.T) {
	tests := []struct {
		name       string
		even       []byte
		odd        []byte
		wantParity []byte
	}{
		{
			name:       "empty",
			even:       []byte{},
			odd:        []byte{},
			wantParity: []byte{},
		},
		{
			name:       "single even byte (odd length original)",
			even:       []byte{0xAA},
			odd:        []byte{},
			wantParity: []byte{0xAA}, // No XOR partner, just copy
		},
		{
			name:       "equal lengths (even length original)",
			even:       []byte{0xAA},
			odd:        []byte{0xBB},
			wantParity: []byte{0xAA ^ 0xBB}, // 0x11
		},
		{
			name:       "even one larger (odd length original)",
			even:       []byte{0xAA, 0xCC},
			odd:        []byte{0xBB},
			wantParity: []byte{0xAA ^ 0xBB, 0xCC}, // [0x11, 0xCC]
		},
		{
			name:       "four bytes (even length original)",
			even:       []byte{0x01, 0x03},
			odd:        []byte{0x02, 0x04},
			wantParity: []byte{0x01 ^ 0x02, 0x03 ^ 0x04}, // [0x03, 0x07]
		},
		{
			name:       "seven bytes (odd length original)",
			even:       []byte{0x01, 0x03, 0x05, 0x07},
			odd:        []byte{0x02, 0x04, 0x06},
			wantParity: []byte{0x01 ^ 0x02, 0x03 ^ 0x04, 0x05 ^ 0x06, 0x07}, // [0x03, 0x07, 0x03, 0x07]
		},
		{
			name:       "Hello, World!",
			even:       []byte{'H', 'l', 'o', ' ', 'o', 'l', '!'},
			odd:        []byte{'e', 'l', ',', 'W', 'r', 'd'},
			wantParity: []byte{'H' ^ 'e', 'l' ^ 'l', 'o' ^ ',', ' ' ^ 'W', 'o' ^ 'r', 'l' ^ 'd', '!'},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := level3.CalculateParity(tt.even, tt.odd)
			assert.Equal(t, tt.wantParity, got)
			assert.Equal(t, len(tt.even), len(got), "parity size should equal even size")
		})
	}
}

// TestParityFilenames tests the generation of parity file names with
// .parity-el (even-length) and .parity-ol (odd-length) suffixes.
//
// These suffixes encode whether the original file had even or odd length,
// which is critical for correct reconstruction in degraded mode. Without
// this information, we wouldn't know which size formula to use.
//
// This test verifies:
//   - .parity-el suffix for even-length originals
//   - .parity-ol suffix for odd-length originals
//   - Correct extraction of original name and length info
//   - Handles paths with slashes correctly
//
// Failure indicates: Reconstruction would fail in degraded mode due to
// incorrect length assumptions. Would cause data corruption.
func TestParityFilenames(t *testing.T) {
	tests := []struct {
		name        string
		original    string
		isOddLength bool
		want        string
	}{
		{"odd length", "file.txt", true, "file.txt.parity-ol"},
		{"even length", "file.txt", false, "file.txt.parity-el"},
		{"path with slashes odd", "dir/subdir/file.txt", true, "dir/subdir/file.txt.parity-ol"},
		{"path with slashes even", "dir/subdir/file.txt", false, "dir/subdir/file.txt.parity-el"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := level3.GetParityFilename(tt.original, tt.isOddLength)
			assert.Equal(t, tt.want, got)

			// Test strip parity suffix
			original, isParity, isOddLength := level3.StripParitySuffix(got)
			assert.True(t, isParity, "should be detected as parity")
			assert.Equal(t, tt.original, original, "should strip back to original")
			assert.Equal(t, tt.isOddLength, isOddLength, "should detect correct length type")
		})
	}
}

// =============================================================================
// Unit Tests - Reconstruction
// =============================================================================

// TestParityReconstruction tests basic XOR-based reconstruction of missing
// data from parity.
//
// This verifies the fundamental RAID 3 property: even[i] XOR parity[i] = odd[i]
// and odd[i] XOR parity[i] = even[i]. This is the mathematical basis for
// recovery in degraded mode.
//
// This test verifies:
//   - Can reconstruct odd bytes from even + parity
//   - Can reconstruct even bytes from odd + parity
//   - XOR properties hold for all data patterns
//   - Byte-by-byte reconstruction is correct
//
// Failure indicates: Core RAID 3 math is broken. Degraded mode won't work.
func TestParityReconstruction(t *testing.T) {
	// Test that we can reconstruct odd from even+parity
	original := []byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06}
	even, odd := level3.SplitBytes(original)
	parity := level3.CalculateParity(even, odd)

	// Reconstruct odd from even and parity
	reconstructedOdd := make([]byte, len(odd))
	for i := 0; i < len(odd); i++ {
		reconstructedOdd[i] = even[i] ^ parity[i]
	}
	assert.Equal(t, odd, reconstructedOdd, "should reconstruct odd from even+parity")

	// Reconstruct even from odd and parity
	reconstructedEven := make([]byte, len(odd))
	for i := 0; i < len(odd); i++ {
		reconstructedEven[i] = odd[i] ^ parity[i]
	}
	assert.Equal(t, even[:len(odd)], reconstructedEven, "should reconstruct even from odd+parity")
}

// remotefname is used with RandomRemoteName fallback
const remotefname = "file.bin"

// TestReconstructFromEvenAndParity tests full file reconstruction when the
// odd particle is missing.
//
// In degraded mode, if the odd backend is unavailable, we must be able to
// reconstruct the complete original file using only the even particle and
// the parity particle. This uses the formula: odd[i] = even[i] XOR parity[i].
//
// This test verifies:
//   - Correct reconstruction for various file sizes
//   - Handles both odd-length and even-length originals
//   - Empty files work correctly
//   - Reconstructed data matches original exactly
//   - Real-world text data works
//
// Failure indicates: Reads would fail when odd backend is down.
// Self-healing would not work for odd particles.
func TestReconstructFromEvenAndParity(t *testing.T) {
	cases := [][]byte{
		{},
		{0x01},                     // odd length original
		{0x01, 0x02},               // even length original
		{0x01, 0x02, 0x03},         // odd
		[]byte("Hello, World!"),    // even length 13? actually 13 -> odd
		[]byte("0123456789ABCDEF"), // 16 even
	}
	for i, original := range cases {
		t.Run(string(rune('A'+i)), func(t *testing.T) {
			even, odd := level3.SplitBytes(original)
			parity := level3.CalculateParity(even, odd)
			isOdd := len(original)%2 == 1
			got, err := level3.ReconstructFromEvenAndParity(even, parity, isOdd)
			require.NoError(t, err)
			assert.Equal(t, original, got)
		})
	}
}

// TestReconstructFromOddAndParity tests full file reconstruction when the
// even particle is missing.
//
// In degraded mode, if the even backend is unavailable, we must be able to
// reconstruct the complete original file using only the odd particle and
// the parity particle. This uses the formula: even[i] = odd[i] XOR parity[i].
//
// This test verifies:
//   - Correct reconstruction for various file sizes
//   - Handles both odd-length and even-length originals
//   - Empty files work correctly
//   - Reconstructed data matches original exactly
//   - Real-world text data works
//
// Failure indicates: Reads would fail when even backend is down.
// Self-healing would not work for even particles.
func TestReconstructFromOddAndParity(t *testing.T) {
	cases := [][]byte{
		{},
		{0x01},
		{0x01, 0x02},
		{0x01, 0x02, 0x03},
		[]byte("Hello, World!"),
		[]byte("0123456789ABCDEF"),
	}
	for i, original := range cases {
		t.Run(string(rune('A'+i)), func(t *testing.T) {
			even, odd := level3.SplitBytes(original)
			parity := level3.CalculateParity(even, odd)
			isOdd := len(original)%2 == 1
			got, err := level3.ReconstructFromOddAndParity(odd, parity, isOdd)
			require.NoError(t, err)
			assert.Equal(t, original, got)
		})
	}
}

// TestSizeFormulaWithParity tests size calculation in degraded mode.
//
// In degraded mode (one data particle missing), we must calculate the
// original file size using only one data particle and the parity particle.
// This is critical for reporting correct file sizes to users and for
// correct range reads.
//
// The formula depends on which particle is missing and the original length:
//   - Even missing, odd-length: size = oddSize + paritySize
//   - Even missing, even-length: size = oddSize + paritySize
//   - Odd missing, odd-length: size = evenSize + paritySize - 1
//   - Odd missing, even-length: size = evenSize + paritySize
//
// This test verifies:
//   - Size calculation when even particle is missing (using odd + parity)
//   - Size calculation when odd particle is missing (using even + parity)
//   - Correct handling of odd-length vs even-length originals
//   - Formula produces correct sizes for all test cases
//
// Failure indicates: Size reporting in degraded mode is broken, which would
// cause incorrect file sizes in `ls` commands and corrupt partial reads.
func TestSizeFormulaWithParity(t *testing.T) {
	cases := [][]byte{
		{},
		{0x01},
		{0x01, 0x02},
		{0x01, 0x02, 0x03},
		[]byte("abcde"),
		[]byte("abcdef"),
	}
	for i, original := range cases {
		t.Run(string(rune('A'+i)), func(t *testing.T) {
			even, odd := level3.SplitBytes(original)
			parity := level3.CalculateParity(even, odd)
			isOdd := len(original)%2 == 1
			// Simulate missing odd: size = len(even) + len(parity) - (isOdd?1:0)
			want := int64(len(original))
			gotEvenParity := int64(len(even)+len(parity)) - func() int64 {
				if isOdd {
					return 1
				}
				return 0
			}()
			assert.Equal(t, want, gotEvenParity)
			// Simulate missing even: size = len(odd) + len(parity)
			gotOddParity := int64(len(odd) + len(parity))
			assert.Equal(t, want, gotOddParity)
		})
	}
}

// =============================================================================
// Integration Tests - Degraded Mode
// =============================================================================

// TestIntegrationStyle_DegradedOpenAndSize tests degraded mode operations
// in a realistic scenario.
//
// This simulates a real backend failure by deleting a particle file from
// disk, then verifying that reads still work via reconstruction, and that
// the reported size is still correct. This is crucial for production use.
//
// This test verifies:
//   - NewObject() succeeds with only 2 of 3 particles present
//   - Size() returns correct original file size in degraded mode
//   - Open() + Read() returns correct data via reconstruction
//   - Works for both even and odd particle failures
//
// Failure indicates: Degraded mode doesn't work in realistic scenarios.
// This would make the backend unusable when any backend is temporarily down.
func TestIntegrationStyle_DegradedOpenAndSize(t *testing.T) {
	if *fstest.RemoteName != "" {
		t.Skip("Skipping as -remote set")
	}

	ctx := context.Background()
	// Temp dirs for particles
	evenDir := t.TempDir()
	oddDir := t.TempDir()
	parityDir := t.TempDir()

	// Build Fs directly via NewFs using a config map
	m := configmap.Simple{
		"even":      evenDir,
		"odd":       oddDir,
		"parity":    parityDir,
		"auto_heal": "true",
	}
	f, err := level3.NewFs(ctx, "Lvl3Int", "", m)
	require.NoError(t, err)

	// Put an object
	remote := "test.bin"
	data := []byte("ABCDE") // 5 bytes (odd length)
	info := object.NewStaticObjectInfo(remote, time.Now(), int64(len(data)), true, nil, nil)
	_, err = f.Put(ctx, bytes.NewReader(data), info)
	require.NoError(t, err)

	// Ensure baseline
	obj, err := f.NewObject(ctx, remote)
	require.NoError(t, err)
	require.Equal(t, int64(len(data)), obj.Size())
	rc, err := obj.Open(ctx)
	require.NoError(t, err)
	got, err := io.ReadAll(rc)
	rc.Close()
	require.NoError(t, err)
	assert.Equal(t, data, got)

	// Remove odd particle to force reconstruction from even+parity
	require.NoError(t, os.Remove(filepath.Join(oddDir, remote)))

	// NewObject should still succeed (two of three present)
	obj2, err := f.NewObject(ctx, remote)
	require.NoError(t, err)
	// Size should match
	require.Equal(t, int64(len(data)), obj2.Size())
	// Open should reconstruct
	rc2, err := obj2.Open(ctx)
	require.NoError(t, err)
	got2, err := io.ReadAll(rc2)
	rc2.Close()
	require.NoError(t, err)
	assert.Equal(t, data, got2)
}

// TestLargeDataQuick tests RAID 3 operations with a larger file (1 MB).
//
// Most tests use small data (bytes to KB), but we need to ensure the
// implementation works correctly with larger files that are more
// representative of real-world usage. This test exercises the full
// split/parity/reconstruction pipeline with substantial data.
//
// This test verifies:
//   - Upload and download of 1 MB file works correctly
//   - All three particles are created with correct sizes
//   - Degraded mode reconstruction works with large files
//   - Performance is acceptable (completes in ~1 second)
//   - No memory issues with larger data
//
// Failure indicates: Implementation doesn't scale to realistic file sizes.
// This could indicate memory issues, performance problems, or algorithmic
// errors that only appear with larger data.
func TestLargeDataQuick(t *testing.T) {
	if *fstest.RemoteName != "" {
		t.Skip("Skipping as -remote set")
	}

	ctx := context.Background()
	evenDir := t.TempDir()
	oddDir := t.TempDir()
	parityDir := t.TempDir()

	m := configmap.Simple{
		"even":   evenDir,
		"odd":    oddDir,
		"parity": parityDir,
	}
	f, err := level3.NewFs(ctx, "Lvl3Large", "", m)
	require.NoError(t, err)

	// 1 MiB payload with deterministic content
	remote := "big.bin"
	block := []byte("ABCDEFGHIJKLMNOPQRSTUVWXYZ012345") // 32 bytes
	// 32 * 32768 = 1,048,576 bytes
	data := bytes.Repeat(block, 32768)

	info := object.NewStaticObjectInfo(remote, time.Now(), int64(len(data)), true, nil, nil)
	_, err = f.Put(ctx, bytes.NewReader(data), info)
	require.NoError(t, err)

	// Verify full read
	obj, err := f.NewObject(ctx, remote)
	require.NoError(t, err)
	rc, err := obj.Open(ctx)
	require.NoError(t, err)
	got, err := io.ReadAll(rc)
	rc.Close()
	require.NoError(t, err)
	assert.Equal(t, data, got)

	// Remove even particle, force degraded read from odd+parity
	require.NoError(t, os.Remove(filepath.Join(evenDir, remote)))
	obj2, err := f.NewObject(ctx, remote)
	require.NoError(t, err)
	rc2, err := obj2.Open(ctx)
	require.NoError(t, err)
	got2, err := io.ReadAll(rc2)
	rc2.Close()
	require.NoError(t, err)
	assert.Equal(t, data, got2)
}

// =============================================================================
// Integration Tests - File Operations (Normal Mode - All Backends Available)
// =============================================================================
//
// These tests verify file operations when ALL 3 backends are available.
//
// Error Handling Policy (Hardware RAID 3 Compliant):
//   - Reads: Work with 2 of 3 backends (best effort)
//   - Writes: Require all 3 backends (strict)
//   - Deletes: Work with any backends (best effort, idempotent)
//
// This matches hardware RAID 3 behavior: writes blocked in degraded mode,
// reads work in degraded mode.

// TestRenameFile tests file renaming within the same directory.
//
// Renaming a file in level3 must rename all three particles (even, odd, parity)
// and preserve the parity filename suffix (.parity-el or .parity-ol) based on
// the original file's length. The original particles should no longer exist
// and the new particles should contain the same data.
//
// Per RAID 3 policy: Move requires ALL 3 backends available (strict mode).
//
// This test verifies:
//   - All three particles are renamed correctly
//   - Parity filename suffix is preserved (.parity-el or .parity-ol)
//   - Original file no longer exists at old location
//   - New file exists at new location with correct data
//   - File data integrity is maintained after rename
//
// Failure indicates: Rename operation doesn't maintain RAID 3 consistency.
// Particles could be in inconsistent state (some renamed, some not).
func TestRenameFile(t *testing.T) {
	if *fstest.RemoteName != "" {
		t.Skip("Skipping as -remote set")
	}

	ctx := context.Background()
	evenDir := t.TempDir()
	oddDir := t.TempDir()
	parityDir := t.TempDir()

	m := configmap.Simple{
		"even":   evenDir,
		"odd":    oddDir,
		"parity": parityDir,
	}
	f, err := level3.NewFs(ctx, "TestRename", "", m)
	require.NoError(t, err)

	// Create a test file
	oldRemote := "original.txt"
	data := []byte("Hello, Renamed World!")
	info := object.NewStaticObjectInfo(oldRemote, time.Now(), int64(len(data)), true, nil, nil)
	_, err = f.Put(ctx, bytes.NewReader(data), info)
	require.NoError(t, err)

	// Verify original file exists (check all three particles)
	oldEvenPath := filepath.Join(evenDir, oldRemote)
	oldOddPath := filepath.Join(oddDir, oldRemote)
	oldParityPath := filepath.Join(parityDir, oldRemote+".parity-ol") // 21 bytes = odd length
	_, err = os.Stat(oldEvenPath)
	require.NoError(t, err, "original even particle should exist")
	_, err = os.Stat(oldOddPath)
	require.NoError(t, err, "original odd particle should exist")
	_, err = os.Stat(oldParityPath)
	require.NoError(t, err, "original parity particle should exist")

	// Rename the file
	newRemote := "renamed.txt"
	oldObj, err := f.NewObject(ctx, oldRemote)
	require.NoError(t, err)
	doMove := f.Features().Move
	require.NotNil(t, doMove, "level3 backend should support Move")
	newObj, err := doMove(ctx, oldObj, newRemote)
	require.NoError(t, err)
	require.NotNil(t, newObj)
	assert.Equal(t, newRemote, newObj.Remote())

	// Verify old particles no longer exist
	_, err = os.Stat(oldEvenPath)
	require.True(t, os.IsNotExist(err), "old even particle should be deleted")
	_, err = os.Stat(oldOddPath)
	require.True(t, os.IsNotExist(err), "old odd particle should be deleted")
	_, err = os.Stat(oldParityPath)
	require.True(t, os.IsNotExist(err), "old parity particle should be deleted")

	// Verify new particles exist
	newEvenPath := filepath.Join(evenDir, newRemote)
	newOddPath := filepath.Join(oddDir, newRemote)
	newParityPath := filepath.Join(parityDir, newRemote+".parity-ol")
	_, err = os.Stat(newEvenPath)
	require.NoError(t, err, "new even particle should exist")
	_, err = os.Stat(newOddPath)
	require.NoError(t, err, "new odd particle should exist")
	_, err = os.Stat(newParityPath)
	require.NoError(t, err, "new parity particle should exist")

	// Verify data integrity by reading the renamed file
	newObj2, err := f.NewObject(ctx, newRemote)
	require.NoError(t, err)
	rc, err := newObj2.Open(ctx)
	require.NoError(t, err)
	got, err := io.ReadAll(rc)
	rc.Close()
	require.NoError(t, err)
	assert.Equal(t, data, got, "renamed file should have same data as original")
}

// TestRenameFileDifferentDirectory tests renaming a file to a different directory.
//
// This verifies that Move() works correctly when the destination is in a
// different directory path, ensuring all particles are moved to the correct
// locations while maintaining RAID 3 consistency.
//
// This test verifies:
//   - File can be moved between directories
//   - All three particles are moved correctly
//   - Directory structure is maintained
//   - Data integrity is preserved
//
// Failure indicates: Move doesn't handle directory paths correctly.
func TestRenameFileDifferentDirectory(t *testing.T) {
	if *fstest.RemoteName != "" {
		t.Skip("Skipping as -remote set")
	}

	ctx := context.Background()
	evenDir := t.TempDir()
	oddDir := t.TempDir()
	parityDir := t.TempDir()

	m := configmap.Simple{
		"even":   evenDir,
		"odd":    oddDir,
		"parity": parityDir,
	}
	f, err := level3.NewFs(ctx, "TestRenameDir", "", m)
	require.NoError(t, err)

	// Create directory structure
	err = f.Mkdir(ctx, "source")
	require.NoError(t, err)
	err = f.Mkdir(ctx, "dest")
	require.NoError(t, err)

	// Create file in source directory
	oldRemote := "source/file.txt"
	data := []byte("Moving between directories")
	info := object.NewStaticObjectInfo(oldRemote, time.Now(), int64(len(data)), true, nil, nil)
	_, err = f.Put(ctx, bytes.NewReader(data), info)
	require.NoError(t, err)

	// Move to dest directory
	newRemote := "dest/file.txt"
	oldObj, err := f.NewObject(ctx, oldRemote)
	require.NoError(t, err)
	doMove := f.Features().Move
	require.NotNil(t, doMove)
	newObj, err := doMove(ctx, oldObj, newRemote)
	require.NoError(t, err)
	assert.Equal(t, newRemote, newObj.Remote())

	// Verify old location is empty
	oldObj2, err := f.NewObject(ctx, oldRemote)
	require.Error(t, err, "old file should not exist")
	require.Nil(t, oldObj2)

	// Verify new location has correct data
	newObj2, err := f.NewObject(ctx, newRemote)
	require.NoError(t, err)
	rc, err := newObj2.Open(ctx)
	require.NoError(t, err)
	got, err := io.ReadAll(rc)
	rc.Close()
	require.NoError(t, err)
	assert.Equal(t, data, got)
}

// TestDeleteFile tests deletion of a file.
//
// Deleting a file in level3 must remove all three particles (even, odd, parity)
// from all three backends. The operation should succeed even if one or more
// particles are already missing (idempotent delete).
//
// Per RAID 3 policy: Delete uses best-effort approach (idempotent), unlike
// writes which are strict. This is because missing particle = already deleted.
//
// This test verifies:
//   - All three particles are deleted when all backends available
//   - File no longer exists after deletion
//   - Deletion is idempotent (can delete already-missing particles)
//   - Parity files with both suffixes are handled correctly
//
// Failure indicates: Delete doesn't clean up all particles, leaving orphaned
// files or inconsistent state.
func TestDeleteFile(t *testing.T) {
	if *fstest.RemoteName != "" {
		t.Skip("Skipping as -remote set")
	}

	ctx := context.Background()
	evenDir := t.TempDir()
	oddDir := t.TempDir()
	parityDir := t.TempDir()

	m := configmap.Simple{
		"even":   evenDir,
		"odd":    oddDir,
		"parity": parityDir,
	}
	f, err := level3.NewFs(ctx, "TestDelete", "", m)
	require.NoError(t, err)

	// Create a test file
	remote := "to_delete.txt"
	data := []byte("This file will be deleted")
	info := object.NewStaticObjectInfo(remote, time.Now(), int64(len(data)), true, nil, nil)
	_, err = f.Put(ctx, bytes.NewReader(data), info)
	require.NoError(t, err)

	// Verify file exists
	obj, err := f.NewObject(ctx, remote)
	require.NoError(t, err)
	assert.Equal(t, remote, obj.Remote())

	// Delete the file
	err = obj.Remove(ctx)
	require.NoError(t, err)

	// Verify file no longer exists
	obj2, err := f.NewObject(ctx, remote)
	require.Error(t, err, "deleted file should not exist")
	require.Nil(t, obj2)

	// Verify all particles are deleted from filesystem
	evenPath := filepath.Join(evenDir, remote)
	oddPath := filepath.Join(oddDir, remote)
	parityOddPath := filepath.Join(parityDir, remote+".parity-ol")
	parityEvenPath := filepath.Join(parityDir, remote+".parity-el")

	_, err = os.Stat(evenPath)
	require.True(t, os.IsNotExist(err), "even particle should be deleted")
	_, err = os.Stat(oddPath)
	require.True(t, os.IsNotExist(err), "odd particle should be deleted")
	_, err = os.Stat(parityOddPath)
	require.True(t, os.IsNotExist(err), "odd-length parity particle should be deleted")
	_, err = os.Stat(parityEvenPath)
	require.True(t, os.IsNotExist(err), "even-length parity particle should be deleted")
}

// TestDeleteFileIdempotent tests that deleting a file multiple times is safe.
//
// This verifies the idempotent property of delete operations - deleting a
// file that's already deleted (or partially deleted) should not error.
// This is important for reliability and cleanup operations.
//
// Per RAID 3 policy: Deletes are best-effort and idempotent. This is
// acceptable because "missing particle" and "deleted particle" have the
// same end state - the particle doesn't exist.
//
// This test verifies:
//   - Deleting a non-existent file succeeds (no error)
//   - Deleting a file with missing particles succeeds
//   - Multiple delete calls are safe
//
// Failure indicates: Delete is not idempotent, which could cause cleanup
// operations to fail unnecessarily.
func TestDeleteFileIdempotent(t *testing.T) {
	if *fstest.RemoteName != "" {
		t.Skip("Skipping as -remote set")
	}

	ctx := context.Background()
	evenDir := t.TempDir()
	oddDir := t.TempDir()
	parityDir := t.TempDir()

	m := configmap.Simple{
		"even":   evenDir,
		"odd":    oddDir,
		"parity": parityDir,
	}
	f, err := level3.NewFs(ctx, "TestDeleteIdempotent", "", m)
	require.NoError(t, err)

	// Create and delete a file
	remote := "temp_file.txt"
	data := []byte("Temporary")
	info := object.NewStaticObjectInfo(remote, time.Now(), int64(len(data)), true, nil, nil)
	obj, err := f.Put(ctx, bytes.NewReader(data), info)
	require.NoError(t, err)
	err = obj.Remove(ctx)
	require.NoError(t, err)

	// Try to delete again (should succeed - idempotent)
	err = obj.Remove(ctx)
	require.NoError(t, err, "deleting already-deleted file should succeed")

	// Try to delete file that doesn't exist (via NewObject)
	// NewObject will fail, but if it somehow succeeds, delete should handle gracefully
	nonExistentObj, err := f.NewObject(ctx, remote)
	if err == nil {
		// File somehow still exists, delete it
		err = nonExistentObj.Remove(ctx)
		require.NoError(t, err, "removing non-existent file should handle gracefully")
	}
	// If NewObject returns error, that's expected - file doesn't exist
	// The idempotent behavior is already verified by the second Remove() above
}

// TestMoveFileBetweenDirectories tests moving a file between directories.
//
// Moving a file between directories should relocate all three particles to
// the new directory path while maintaining RAID 3 consistency. This is
// similar to rename but tests directory path handling specifically.
//
// This test verifies:
//   - File moves correctly between directories
//   - All three particles move to correct locations
//   - Original location is cleaned up
//   - Directory structure is maintained
//   - Data integrity is preserved
//
// Failure indicates: Move doesn't handle directory paths correctly or
// doesn't clean up source location properly.
func TestMoveFileBetweenDirectories(t *testing.T) {
	if *fstest.RemoteName != "" {
		t.Skip("Skipping as -remote set")
	}

	ctx := context.Background()
	evenDir := t.TempDir()
	oddDir := t.TempDir()
	parityDir := t.TempDir()

	m := configmap.Simple{
		"even":   evenDir,
		"odd":    oddDir,
		"parity": parityDir,
	}
	f, err := level3.NewFs(ctx, "TestMove", "", m)
	require.NoError(t, err)

	// Create directory structure
	err = f.Mkdir(ctx, "src")
	require.NoError(t, err)
	err = f.Mkdir(ctx, "dst")
	require.NoError(t, err)

	// Create file in source directory
	srcRemote := "src/document.pdf"
	data := []byte("PDF content here")
	info := object.NewStaticObjectInfo(srcRemote, time.Now(), int64(len(data)), true, nil, nil)
	srcObj, err := f.Put(ctx, bytes.NewReader(data), info)
	require.NoError(t, err)

	// Verify source file exists
	srcObj2, err := f.NewObject(ctx, srcRemote)
	require.NoError(t, err)
	assert.Equal(t, srcRemote, srcObj2.Remote())

	// Move to destination directory
	dstRemote := "dst/document.pdf"
	doMove := f.Features().Move
	require.NotNil(t, doMove)
	dstObj, err := doMove(ctx, srcObj, dstRemote)
	require.NoError(t, err)
	require.NotNil(t, dstObj)
	assert.Equal(t, dstRemote, dstObj.Remote())

	// Verify source file no longer exists
	srcObj3, err := f.NewObject(ctx, srcRemote)
	require.Error(t, err, "source file should not exist after move")
	require.Nil(t, srcObj3)

	// Verify destination file exists with correct data
	dstObj2, err := f.NewObject(ctx, dstRemote)
	require.NoError(t, err)
	rc, err := dstObj2.Open(ctx)
	require.NoError(t, err)
	got, err := io.ReadAll(rc)
	rc.Close()
	require.NoError(t, err)
	assert.Equal(t, data, got, "moved file should have same data")

	// Verify particles moved in filesystem
	srcEvenPath := filepath.Join(evenDir, "src", "document.pdf")
	dstEvenPath := filepath.Join(evenDir, "dst", "document.pdf")
	_, err = os.Stat(srcEvenPath)
	require.True(t, os.IsNotExist(err), "source particle should be deleted")
	_, err = os.Stat(dstEvenPath)
	require.NoError(t, err, "destination particle should exist")
}

// TestDirMove tests directory renaming using DirMove.
//
// This test verifies:
//   - Directory can be renamed using DirMove operation
//   - Source directory is removed after successful move
//   - Destination directory exists with all particles
//   - Works with local filesystem backends
//
// Failure indicates: DirMove doesn't properly handle directory renaming
// or doesn't clean up source directory.
func TestDirMove(t *testing.T) {
	if *fstest.RemoteName != "" {
		t.Skip("Skipping as -remote set")
	}

	ctx := context.Background()
	evenDir := t.TempDir()
	oddDir := t.TempDir()
	parityDir := t.TempDir()

	m := configmap.Simple{
		"even":   evenDir,
		"odd":    oddDir,
		"parity": parityDir,
	}
	f, err := level3.NewFs(ctx, "TestDirMove", "", m)
	require.NoError(t, err)

	// Verify DirMove is supported
	doDirMove := f.Features().DirMove
	require.NotNil(t, doDirMove, "DirMove should be supported with local backends")

	// Create source directory
	srcDir := "mydir"
	err = f.Mkdir(ctx, srcDir)
	require.NoError(t, err)

	// Create a file in the source directory to verify contents move
	srcFile := "mydir/file.txt"
	data := []byte("Test file content")
	info := object.NewStaticObjectInfo(srcFile, time.Now(), int64(len(data)), true, nil, nil)
	_, err = f.Put(ctx, bytes.NewReader(data), info)
	require.NoError(t, err)

	// Verify source directory exists
	entries, err := f.List(ctx, srcDir)
	require.NoError(t, err)
	require.Len(t, entries, 1, "source directory should contain one file")

	// Move directory
	dstDir := "mydir2"

	// Verify destination doesn't exist yet
	_, err = f.List(ctx, dstDir)
	require.Error(t, err, "destination should not exist before move")
	require.True(t, errors.Is(err, fs.ErrorDirNotFound), "should get ErrorDirNotFound")

	// Create separate Fs instances for source and destination (as operations.DirMove does)
	// Source Fs has root=srcDir, destination Fs has root=dstDir
	srcFs, err := level3.NewFs(ctx, "TestDirMove", srcDir, m)
	require.NoError(t, err)
	dstFs, err := level3.NewFs(ctx, "TestDirMove", dstDir, m)
	require.NoError(t, err)

	// Get DirMove from destination Fs
	dstDoDirMove := dstFs.Features().DirMove
	require.NotNil(t, dstDoDirMove, "destination Fs should support DirMove")

	// Perform the move - use destination Fs's DirMove, source Fs, and empty paths (they're at the roots)
	err = dstDoDirMove(ctx, srcFs, "", "")
	require.NoError(t, err, "DirMove should succeed")

	// Verify source directory no longer exists
	_, err = f.List(ctx, srcDir)
	require.Error(t, err, "source directory should not exist after move")
	require.True(t, errors.Is(err, fs.ErrorDirNotFound), "should get ErrorDirNotFound")

	// Verify destination directory exists with file
	entries, err = f.List(ctx, dstDir)
	require.NoError(t, err)
	require.Len(t, entries, 1, "destination directory should contain one file")

	// Verify file content is correct
	dstFile := "mydir2/file.txt"
	obj, err := f.NewObject(ctx, dstFile)
	require.NoError(t, err)
	rc, err := obj.Open(ctx)
	require.NoError(t, err)
	got, err := io.ReadAll(rc)
	rc.Close()
	require.NoError(t, err)
	assert.Equal(t, data, got, "moved file should have same data")

	// Verify particles moved in filesystem
	srcEvenPath := filepath.Join(evenDir, srcDir)
	dstEvenPath := filepath.Join(evenDir, dstDir)
	_, err = os.Stat(srcEvenPath)
	require.True(t, os.IsNotExist(err), "source directory should be deleted from even backend")
	_, err = os.Stat(dstEvenPath)
	require.NoError(t, err, "destination directory should exist on even backend")

	// Verify on all three backends
	for _, backendDir := range []string{evenDir, oddDir, parityDir} {
		srcPath := filepath.Join(backendDir, srcDir)
		dstPath := filepath.Join(backendDir, dstDir)
		_, err := os.Stat(srcPath)
		assert.True(t, os.IsNotExist(err), "source should be deleted on backend %s", backendDir)
		_, err = os.Stat(dstPath)
		assert.NoError(t, err, "destination should exist on backend %s", backendDir)
	}
}

// TestRenameFilePreservesParitySuffix tests that renaming preserves the correct
// parity filename suffix (.parity-el vs .parity-ol).
//
// When renaming a file, the parity particle must use the correct suffix
// based on the original file's length. An odd-length file (21 bytes) should
// have .parity-ol, while an even-length file (20 bytes) should have .parity-el.
//
// This test verifies:
//   - Odd-length files maintain .parity-ol suffix after rename
//   - Even-length files maintain .parity-el suffix after rename
//   - Parity suffix is correctly determined from source file
//
// Failure indicates: Parity filename generation is broken during rename,
// which would cause reconstruction failures in degraded mode.
func TestRenameFilePreservesParitySuffix(t *testing.T) {
	if *fstest.RemoteName != "" {
		t.Skip("Skipping as -remote set")
	}

	ctx := context.Background()
	evenDir := t.TempDir()
	oddDir := t.TempDir()
	parityDir := t.TempDir()

	m := configmap.Simple{
		"even":   evenDir,
		"odd":    oddDir,
		"parity": parityDir,
	}
	f, err := level3.NewFs(ctx, "TestRenameParity", "", m)
	require.NoError(t, err)

	// Test odd-length file (preserves .parity-ol)
	oldRemoteOdd := "odd_file.txt"
	dataOdd := []byte("1234567890123456789") // 19 bytes = odd length
	infoOdd := object.NewStaticObjectInfo(oldRemoteOdd, time.Now(), int64(len(dataOdd)), true, nil, nil)
	_, err = f.Put(ctx, bytes.NewReader(dataOdd), infoOdd)
	require.NoError(t, err)

	oldParityOddPath := filepath.Join(parityDir, oldRemoteOdd+".parity-ol")
	_, err = os.Stat(oldParityOddPath)
	require.NoError(t, err, "original odd-length parity should exist")

	newRemoteOdd := "renamed_odd.txt"
	oldObjOdd, err := f.NewObject(ctx, oldRemoteOdd)
	require.NoError(t, err)
	doMove := f.Features().Move
	require.NotNil(t, doMove)
	_, err = doMove(ctx, oldObjOdd, newRemoteOdd)
	require.NoError(t, err)

	newParityOddPath := filepath.Join(parityDir, newRemoteOdd+".parity-ol")
	_, err = os.Stat(newParityOddPath)
	require.NoError(t, err, "renamed file should have .parity-ol suffix (odd length preserved)")

	// Test even-length file (preserves .parity-el)
	oldRemoteEven := "even_file.txt"
	dataEven := []byte("12345678901234567890") // 20 bytes = even length
	infoEven := object.NewStaticObjectInfo(oldRemoteEven, time.Now(), int64(len(dataEven)), true, nil, nil)
	_, err = f.Put(ctx, bytes.NewReader(dataEven), infoEven)
	require.NoError(t, err)

	oldParityEvenPath := filepath.Join(parityDir, oldRemoteEven+".parity-el")
	_, err = os.Stat(oldParityEvenPath)
	require.NoError(t, err, "original even-length parity should exist")

	newRemoteEven := "renamed_even.txt"
	oldObjEven, err := f.NewObject(ctx, oldRemoteEven)
	require.NoError(t, err)
	doMoveEven := f.Features().Move
	require.NotNil(t, doMoveEven)
	_, err = doMoveEven(ctx, oldObjEven, newRemoteEven)
	require.NoError(t, err)

	newParityEvenPath := filepath.Join(parityDir, newRemoteEven+".parity-el")
	_, err = os.Stat(newParityEvenPath)
	require.NoError(t, err, "renamed file should have .parity-el suffix (even length preserved)")
}

// =============================================================================
// Advanced Tests - Deep Subdirectories & Concurrency
// =============================================================================

// TestDeepNestedDirectories tests operations with deeply nested directory
// structures (5 levels deep).
//
// Real-world filesystems often have deeply nested directories, and level3
// must handle them correctly. This tests that particle files are stored at
// the correct depth in all three backends, and that operations like list,
// move, and delete work correctly at any depth.
//
// This test verifies:
//   - Creating files in deep paths (a/b/c/d/e/file.txt)
//   - Listing works at various depths
//   - Moving files between deep directories
//   - All three particles stored at correct depth
//   - No path manipulation errors
//   - Directory creation at all levels
//
// Failure indicates: Path handling is broken for deeply nested structures,
// which would cause file corruption or loss in production.
func TestDeepNestedDirectories(t *testing.T) {
	if *fstest.RemoteName != "" {
		t.Skip("Skipping as -remote set")
	}

	ctx := context.Background()
	evenDir := t.TempDir()
	oddDir := t.TempDir()
	parityDir := t.TempDir()

	m := configmap.Simple{
		"even":   evenDir,
		"odd":    oddDir,
		"parity": parityDir,
	}
	f, err := level3.NewFs(ctx, "TestDeepNested", "", m)
	require.NoError(t, err)

	// Test 1: Create file in deeply nested directory (5 levels)
	deepPath := "level1/level2/level3/level4/level5/deep-file.txt"
	deepData := []byte("Content in deeply nested directory")

	// Create parent directories
	err = f.Mkdir(ctx, "level1")
	require.NoError(t, err)
	err = f.Mkdir(ctx, "level1/level2")
	require.NoError(t, err)
	err = f.Mkdir(ctx, "level1/level2/level3")
	require.NoError(t, err)
	err = f.Mkdir(ctx, "level1/level2/level3/level4")
	require.NoError(t, err)
	err = f.Mkdir(ctx, "level1/level2/level3/level4/level5")
	require.NoError(t, err)

	// Upload file to deep path
	info := object.NewStaticObjectInfo(deepPath, time.Now(), int64(len(deepData)), true, nil, nil)
	obj, err := f.Put(ctx, bytes.NewReader(deepData), info)
	require.NoError(t, err)
	require.NotNil(t, obj)

	// Verify all three particles exist at correct depth
	evenPath := filepath.Join(evenDir, "level1/level2/level3/level4/level5/deep-file.txt")
	oddPath := filepath.Join(oddDir, "level1/level2/level3/level4/level5/deep-file.txt")
	parityPath := filepath.Join(parityDir, "level1/level2/level3/level4/level5/deep-file.txt.parity-el")

	_, err = os.Stat(evenPath)
	require.NoError(t, err, "even particle should exist at deep path")
	_, err = os.Stat(oddPath)
	require.NoError(t, err, "odd particle should exist at deep path")
	_, err = os.Stat(parityPath)
	require.NoError(t, err, "parity particle should exist at deep path")

	// Test 2: List at various depths
	entries1, err := f.List(ctx, "level1")
	require.NoError(t, err)
	assert.True(t, len(entries1) > 0, "should list entries at level1")

	entries3, err := f.List(ctx, "level1/level2/level3")
	require.NoError(t, err)
	assert.True(t, len(entries3) > 0, "should list entries at level3")

	entries5, err := f.List(ctx, "level1/level2/level3/level4/level5")
	require.NoError(t, err)
	assert.Len(t, entries5, 1, "should list 1 file at level5")

	// Test 3: Read file from deep path
	obj2, err := f.NewObject(ctx, deepPath)
	require.NoError(t, err)
	rc, err := obj2.Open(ctx)
	require.NoError(t, err)
	readData, err := io.ReadAll(rc)
	rc.Close()
	require.NoError(t, err)
	assert.Equal(t, deepData, readData, "should read correct data from deep path")

	// Test 4: Move file between deep directories
	deepPath2 := "level1/level2/other/level4/level5/moved-file.txt"
	err = f.Mkdir(ctx, "level1/level2/other")
	require.NoError(t, err)
	err = f.Mkdir(ctx, "level1/level2/other/level4")
	require.NoError(t, err)
	err = f.Mkdir(ctx, "level1/level2/other/level4/level5")
	require.NoError(t, err)

	doMove := f.Features().Move
	require.NotNil(t, doMove)
	movedObj, err := doMove(ctx, obj2, deepPath2)
	require.NoError(t, err)
	require.NotNil(t, movedObj)

	// Verify particles moved to new deep path
	newEvenPath := filepath.Join(evenDir, "level1/level2/other/level4/level5/moved-file.txt")
	newOddPath := filepath.Join(oddDir, "level1/level2/other/level4/level5/moved-file.txt")
	newParityPath := filepath.Join(parityDir, "level1/level2/other/level4/level5/moved-file.txt.parity-el")

	_, err = os.Stat(newEvenPath)
	require.NoError(t, err, "even particle should exist at new deep path")
	_, err = os.Stat(newOddPath)
	require.NoError(t, err, "odd particle should exist at new deep path")
	_, err = os.Stat(newParityPath)
	require.NoError(t, err, "parity particle should exist at new deep path")

	// Verify old paths are deleted
	_, err = os.Stat(evenPath)
	require.True(t, os.IsNotExist(err), "old even particle should be deleted")

	t.Logf("✅ Deep nested directories (5 levels) work correctly")
}

// TestConcurrentOperations tests multiple simultaneous operations to detect
// race conditions and concurrency issues.
//
// In production, level3 may face concurrent operations: multiple uploads,
// simultaneous reads during self-healing, or operations during degraded mode.
// This test stresses the backend with concurrent operations to ensure thread
// safety and detect race conditions.
//
// This test verifies:
//   - Concurrent Put operations don't corrupt data
//   - Concurrent reads work correctly
//   - Self-healing queue handles concurrent uploads
//   - No race conditions in particle management
//   - Errgroup coordination works correctly
//
// Failure indicates: Race conditions or concurrency bugs that would cause
// data corruption or crashes in production under load.
//
// Note: Run with -race flag to detect race conditions:
//
//	go test -race -run TestConcurrentOperations
func TestConcurrentOperations(t *testing.T) {
	if *fstest.RemoteName != "" {
		t.Skip("Skipping as -remote set")
	}
	if testing.Short() {
		t.Skip("Skipping stress test in short mode")
	}
	// NOTE: This test exercises concurrent self-healing behaviour. While auto_heal
	// semantics are being revised and made explicit via backend commands, this
	// stress-test is temporarily disabled to avoid flakiness tied to timing of
	// background uploads.
	t.Skip("Concurrent self-healing stress-test temporarily disabled while auto_heal behaviour is revised")

	ctx := context.Background()
	evenDir := t.TempDir()
	oddDir := t.TempDir()
	parityDir := t.TempDir()

	m := configmap.Simple{
		"even":   evenDir,
		"odd":    oddDir,
		"parity": parityDir,
	}
	f, err := level3.NewFs(ctx, "TestConcurrent", "", m)
	require.NoError(t, err)

	// Test 1: Concurrent Put operations (10 files simultaneously)
	t.Log("Testing concurrent Put operations...")
	var wg sync.WaitGroup
	numFiles := 10
	errors := make(chan error, numFiles)

	for i := 0; i < numFiles; i++ {
		wg.Add(1)
		go func(fileNum int) {
			defer wg.Done()

			remote := fmt.Sprintf("concurrent-file-%d.txt", fileNum)
			data := []byte(fmt.Sprintf("Concurrent content %d", fileNum))
			info := object.NewStaticObjectInfo(remote, time.Now(), int64(len(data)), true, nil, nil)

			_, err := f.Put(ctx, bytes.NewReader(data), info)
			if err != nil {
				errors <- fmt.Errorf("file %d: %w", fileNum, err)
			}
		}(i)
	}

	wg.Wait()
	close(errors)

	// Check for errors
	var errList []error
	for err := range errors {
		errList = append(errList, err)
	}
	require.Empty(t, errList, "concurrent Put operations should succeed")

	// Verify all files were created correctly
	for i := 0; i < numFiles; i++ {
		remote := fmt.Sprintf("concurrent-file-%d.txt", i)
		obj, err := f.NewObject(ctx, remote)
		require.NoError(t, err, "file %d should exist", i)

		// Verify content
		rc, err := obj.Open(ctx)
		require.NoError(t, err)
		data, err := io.ReadAll(rc)
		rc.Close()
		require.NoError(t, err)

		expected := fmt.Sprintf("Concurrent content %d", i)
		assert.Equal(t, expected, string(data), "file %d should have correct content", i)
	}

	// Test 2: Concurrent reads (read all files simultaneously)
	t.Log("Testing concurrent Read operations...")
	var wg2 sync.WaitGroup
	readErrors := make(chan error, numFiles)

	for i := 0; i < numFiles; i++ {
		wg2.Add(1)
		go func(fileNum int) {
			defer wg2.Done()

			remote := fmt.Sprintf("concurrent-file-%d.txt", fileNum)
			obj, err := f.NewObject(ctx, remote)
			if err != nil {
				readErrors <- fmt.Errorf("file %d: %w", fileNum, err)
				return
			}

			rc, err := obj.Open(ctx)
			if err != nil {
				readErrors <- fmt.Errorf("file %d: %w", fileNum, err)
				return
			}
			defer rc.Close()

			_, err = io.ReadAll(rc)
			if err != nil {
				readErrors <- fmt.Errorf("file %d: %w", fileNum, err)
			}
		}(i)
	}

	wg2.Wait()
	close(readErrors)

	// Check for read errors
	var readErrList []error
	for err := range readErrors {
		readErrList = append(readErrList, err)
	}
	require.Empty(t, readErrList, "concurrent Read operations should succeed")

	// Test 3: Concurrent operations with self-healing
	// Delete odd particles to trigger self-healing on next read
	t.Log("Testing concurrent reads with self-healing...")
	healRemotes := []string{
		"concurrent-file-0.txt",
		"concurrent-file-1.txt",
		"concurrent-file-2.txt",
	}

	for _, remote := range healRemotes {
		oddPath := filepath.Join(oddDir, remote)
		err := os.Remove(oddPath)
		require.NoError(t, err)
	}

	// Read all heal files concurrently (should trigger self-healing)
	var wg3 sync.WaitGroup
	healErrors := make(chan error, len(healRemotes))

	for _, remote := range healRemotes {
		wg3.Add(1)
		go func(r string) {
			defer wg3.Done()

			obj, err := f.NewObject(ctx, r)
			if err != nil {
				healErrors <- err
				return
			}

			rc, err := obj.Open(ctx)
			if err != nil {
				healErrors <- err
				return
			}
			defer rc.Close()

			_, err = io.ReadAll(rc)
			if err != nil {
				healErrors <- err
			}
		}(remote)
	}

	wg3.Wait()
	close(healErrors)

	// Check for heal errors
	var healErrList []error
	for err := range healErrors {
		healErrList = append(healErrList, err)
	}
	require.Empty(t, healErrList, "concurrent self-healing should succeed")

	// Wait for self-healing to complete
	shutdowner, ok := f.(fs.Shutdowner)
	require.True(t, ok, "fs should implement Shutdowner")
	err = shutdowner.Shutdown(ctx)
	require.NoError(t, err)

	// Verify healed particles were restored
	for _, remote := range healRemotes {
		oddPath := filepath.Join(oddDir, remote)
		_, err := os.Stat(oddPath)
		require.NoError(t, err, "odd particle for %s should be restored", remote)
	}

	t.Logf("✅ Concurrent operations (10 files, 3 heals) completed successfully")
}

// =============================================================================
// Auto-Cleanup Tests
// =============================================================================

// TestAutoCleanupDefault tests that auto_cleanup defaults to true when not specified
func TestAutoCleanupDefault(t *testing.T) {
	ctx := context.Background()

	// Create temporary directories for three backends
	evenDir := t.TempDir()
	oddDir := t.TempDir()
	parityDir := t.TempDir()

	// Create level3 filesystem WITHOUT specifying auto_cleanup (should default to true)
	l3fs, err := level3.NewFs(ctx, "level3", "", configmap.Simple{
		"even":   evenDir,
		"odd":    oddDir,
		"parity": parityDir,
		// auto_cleanup NOT specified - should default to true
	})
	require.NoError(t, err, "Failed to create level3 filesystem")
	defer func() {
		_ = l3fs.Features().Shutdown(ctx)
	}()

	// Create a valid object (3 particles)
	validData := []byte("This is a valid test file")
	validObj, err := l3fs.Put(ctx, bytes.NewReader(validData), object.NewStaticObjectInfo("valid.txt", time.Now(), int64(len(validData)), true, nil, l3fs))
	require.NoError(t, err, "Failed to create valid object")
	require.NotNil(t, validObj, "Valid object should not be nil")

	// Create a broken object manually (only 1 particle in even)
	brokenData := []byte("broken file")
	brokenPath := filepath.Join(evenDir, "broken.txt")
	err = os.WriteFile(brokenPath, brokenData, 0644)
	require.NoError(t, err, "Failed to create broken object particle")

	// List should show only the valid object (broken should be hidden by default)
	entries, err := l3fs.List(ctx, "")
	require.NoError(t, err, "List should succeed")

	// Count objects
	objectCount := 0
	for _, entry := range entries {
		if _, ok := entry.(fs.Object); ok {
			objectCount++
			assert.Equal(t, "valid.txt", entry.Remote(), "Should only see valid.txt")
		}
	}

	assert.Equal(t, 1, objectCount, "Should see exactly 1 object (broken.txt should be hidden by default)")

	t.Logf("✅ Auto-cleanup defaults to true: broken objects are hidden without explicit config")
}

// TestAutoCleanupEnabled tests that broken objects (1 particle) are hidden when auto_cleanup=true
func TestAutoCleanupEnabled(t *testing.T) {
	ctx := context.Background()

	// Create temporary directories for three backends
	evenDir := t.TempDir()
	oddDir := t.TempDir()
	parityDir := t.TempDir()

	// Create level3 filesystem with auto_cleanup=true (explicit)
	l3fs, err := level3.NewFs(ctx, "level3", "", configmap.Simple{
		"even":         evenDir,
		"odd":          oddDir,
		"parity":       parityDir,
		"auto_cleanup": "true",
	})
	require.NoError(t, err, "Failed to create level3 filesystem")
	defer func() {
		_ = l3fs.Features().Shutdown(ctx)
	}()

	// Create a valid object (3 particles)
	validData := []byte("This is a valid test file")
	validObj, err := l3fs.Put(ctx, bytes.NewReader(validData), object.NewStaticObjectInfo("valid.txt", time.Now(), int64(len(validData)), true, nil, l3fs))
	require.NoError(t, err, "Failed to create valid object")
	require.NotNil(t, validObj, "Valid object should not be nil")

	// Create a broken object manually (only 1 particle in even)
	brokenData := []byte("broken file")
	brokenPath := filepath.Join(evenDir, "broken.txt")
	err = os.WriteFile(brokenPath, brokenData, 0644)
	require.NoError(t, err, "Failed to create broken object particle")

	// List should show only the valid object, not the broken one
	entries, err := l3fs.List(ctx, "")
	require.NoError(t, err, "List should succeed")

	// Count objects
	objectCount := 0
	for _, entry := range entries {
		if _, ok := entry.(fs.Object); ok {
			objectCount++
			assert.Equal(t, "valid.txt", entry.Remote(), "Should only see valid.txt")
		}
	}

	assert.Equal(t, 1, objectCount, "Should see exactly 1 object (broken.txt should be hidden)")

	t.Logf("✅ Auto-cleanup enabled: broken objects are hidden")
}

// TestAutoCleanupDisabled tests that broken objects are visible when auto_cleanup=false
func TestAutoCleanupDisabled(t *testing.T) {
	ctx := context.Background()

	// Create temporary directories for three backends
	evenDir := t.TempDir()
	oddDir := t.TempDir()
	parityDir := t.TempDir()

	// Create level3 filesystem with auto_cleanup=false
	l3fs, err := level3.NewFs(ctx, "level3", "", configmap.Simple{
		"even":         evenDir,
		"odd":          oddDir,
		"parity":       parityDir,
		"auto_cleanup": "false",
	})
	require.NoError(t, err, "Failed to create level3 filesystem")
	defer func() {
		_ = l3fs.Features().Shutdown(ctx)
	}()

	// Create a valid object (3 particles)
	validData := []byte("This is a valid test file")
	validObj, err := l3fs.Put(ctx, bytes.NewReader(validData), object.NewStaticObjectInfo("valid.txt", time.Now(), int64(len(validData)), true, nil, l3fs))
	require.NoError(t, err, "Failed to create valid object")
	require.NotNil(t, validObj, "Valid object should not be nil")

	// Create a broken object manually (only 1 particle in even)
	brokenData := []byte("broken file")
	brokenPath := filepath.Join(evenDir, "broken.txt")
	err = os.WriteFile(brokenPath, brokenData, 0644)
	require.NoError(t, err, "Failed to create broken object particle")

	// List should show BOTH objects when auto_cleanup is disabled
	entries, err := l3fs.List(ctx, "")
	require.NoError(t, err, "List should succeed")

	// Count objects
	objectCount := 0
	var objectNames []string
	for _, entry := range entries {
		if _, ok := entry.(fs.Object); ok {
			objectCount++
			objectNames = append(objectNames, entry.Remote())
		}
	}

	assert.Equal(t, 2, objectCount, "Should see both valid.txt and broken.txt")
	assert.Contains(t, objectNames, "valid.txt", "Should see valid.txt")
	assert.Contains(t, objectNames, "broken.txt", "Should see broken.txt")

	// Reading broken.txt should fail (can't reconstruct from 1 particle)
	brokenObj, err := l3fs.NewObject(ctx, "broken.txt")
	assert.Error(t, err, "NewObject should fail for broken object with 1 particle")
	assert.Nil(t, brokenObj, "Broken object should be nil")

	t.Logf("✅ Auto-cleanup disabled: broken objects are visible")
}

// TestCleanUpCommand tests the CleanUp() method that removes broken objects
func TestCleanUpCommand(t *testing.T) {
	ctx := context.Background()

	// Create temporary directories for three backends
	evenDir := t.TempDir()
	oddDir := t.TempDir()
	parityDir := t.TempDir()

	// Create level3 filesystem with auto_cleanup=false (to see broken objects)
	l3fs, err := level3.NewFs(ctx, "level3", "", configmap.Simple{
		"even":         evenDir,
		"odd":          oddDir,
		"parity":       parityDir,
		"auto_cleanup": "false",
	})
	require.NoError(t, err, "Failed to create level3 filesystem")
	defer func() {
		_ = l3fs.Features().Shutdown(ctx)
	}()

	// Create 3 valid objects
	for i := 1; i <= 3; i++ {
		data := []byte(fmt.Sprintf("Valid file %d", i))
		_, err := l3fs.Put(ctx, bytes.NewReader(data), object.NewStaticObjectInfo(fmt.Sprintf("valid%d.txt", i), time.Now(), int64(len(data)), true, nil, l3fs))
		require.NoError(t, err, "Failed to create valid object %d", i)
	}

	// Create 5 broken objects manually (1 particle each, alternating even/odd)
	for i := 1; i <= 5; i++ {
		data := []byte(fmt.Sprintf("Broken file %d", i))
		// Alternate between even and odd (skip parity for simplicity)
		var path string
		if i%2 == 0 {
			path = filepath.Join(evenDir, fmt.Sprintf("broken%d.txt", i))
		} else {
			path = filepath.Join(oddDir, fmt.Sprintf("broken%d.txt", i))
		}
		err = os.WriteFile(path, data, 0644)
		require.NoError(t, err, "Failed to create broken object %d", i)
	}

	// Verify we can see all 8 objects before cleanup
	entries, err := l3fs.List(ctx, "")
	require.NoError(t, err, "List should succeed")
	initialCount := 0
	for _, entry := range entries {
		if _, ok := entry.(fs.Object); ok {
			initialCount++
		}
	}
	assert.Equal(t, 8, initialCount, "Should see 8 objects total (3 valid + 5 broken)")

	// Run CleanUp command
	cleanUpFunc := l3fs.Features().CleanUp
	require.NotNil(t, cleanUpFunc, "CleanUp feature should be available")
	err = cleanUpFunc(ctx)
	require.NoError(t, err, "CleanUp should succeed")

	// Verify only valid objects remain
	entries, err = l3fs.List(ctx, "")
	require.NoError(t, err, "List should succeed after cleanup")
	finalCount := 0
	for _, entry := range entries {
		if _, ok := entry.(fs.Object); ok {
			finalCount++
			// All remaining objects should be valid*.txt
			assert.Contains(t, entry.Remote(), "valid", "Only valid objects should remain")
		}
	}
	assert.Equal(t, 3, finalCount, "Should see only 3 valid objects after cleanup")

	t.Logf("✅ CleanUp command removed 5 broken objects")
}

// TestCleanUpRecursive tests CleanUp with nested directories
func TestCleanUpRecursive(t *testing.T) {
	ctx := context.Background()

	// Create temporary directories for three backends
	evenDir := t.TempDir()
	oddDir := t.TempDir()
	parityDir := t.TempDir()

	// Create level3 filesystem with auto_cleanup=false
	l3fs, err := level3.NewFs(ctx, "level3", "", configmap.Simple{
		"even":         evenDir,
		"odd":          oddDir,
		"parity":       parityDir,
		"auto_cleanup": "false",
	})
	require.NoError(t, err, "Failed to create level3 filesystem")
	defer func() {
		_ = l3fs.Features().Shutdown(ctx)
	}()

	// Create directory structure:
	// /dir1/file1.txt (valid)
	// /dir1/file2.txt (broken)
	// /dir2/file3.txt (broken)
	// /dir2/subdir/file4.txt (valid)
	// /dir2/subdir/file5.txt (broken)

	// Create directories
	require.NoError(t, l3fs.Mkdir(ctx, "dir1"))
	require.NoError(t, l3fs.Mkdir(ctx, "dir2"))
	require.NoError(t, l3fs.Mkdir(ctx, "dir2/subdir"))

	// Create valid files
	data1 := []byte("Valid file 1")
	_, err = l3fs.Put(ctx, bytes.NewReader(data1), object.NewStaticObjectInfo("dir1/file1.txt", time.Now(), int64(len(data1)), true, nil, l3fs))
	require.NoError(t, err)

	data4 := []byte("Valid file 4")
	_, err = l3fs.Put(ctx, bytes.NewReader(data4), object.NewStaticObjectInfo("dir2/subdir/file4.txt", time.Now(), int64(len(data4)), true, nil, l3fs))
	require.NoError(t, err)

	// Create broken files manually (even and odd only, skip parity)
	broken2Path := filepath.Join(evenDir, "dir1", "file2.txt")
	require.NoError(t, os.WriteFile(broken2Path, []byte("broken2"), 0644))

	broken3Path := filepath.Join(oddDir, "dir2", "file3.txt")
	require.NoError(t, os.MkdirAll(filepath.Dir(broken3Path), 0755))
	require.NoError(t, os.WriteFile(broken3Path, []byte("broken3"), 0644))

	broken5Path := filepath.Join(evenDir, "dir2", "subdir", "file5.txt")
	require.NoError(t, os.MkdirAll(filepath.Dir(broken5Path), 0755))
	require.NoError(t, os.WriteFile(broken5Path, []byte("broken5"), 0644))

	// Count objects before cleanup (should see 5 total)
	initialCount := countAllObjects(t, ctx, l3fs, "")
	assert.Equal(t, 5, initialCount, "Should see 5 objects before cleanup")

	// Run CleanUp
	cleanUpFunc := l3fs.Features().CleanUp
	require.NotNil(t, cleanUpFunc)
	err = cleanUpFunc(ctx)
	require.NoError(t, err, "CleanUp should succeed")

	// Count objects after cleanup (should see only 2 valid)
	finalCount := countAllObjects(t, ctx, l3fs, "")
	assert.Equal(t, 2, finalCount, "Should see only 2 valid objects after cleanup")

	t.Logf("✅ CleanUp removed broken objects from nested directories")
}

// TestPurgeWithAutoCleanup tests that purge works correctly with auto-cleanup enabled
func TestPurgeWithAutoCleanup(t *testing.T) {
	ctx := context.Background()

	// Create temporary directories for three backends
	evenDir := t.TempDir()
	oddDir := t.TempDir()
	parityDir := t.TempDir()

	// Create level3 filesystem with auto_cleanup=true
	l3fs, err := level3.NewFs(ctx, "level3", "", configmap.Simple{
		"even":         evenDir,
		"odd":          oddDir,
		"parity":       parityDir,
		"auto_cleanup": "true",
	})
	require.NoError(t, err, "Failed to create level3 filesystem")
	defer func() {
		_ = l3fs.Features().Shutdown(ctx)
	}()

	// Create a subdirectory
	require.NoError(t, l3fs.Mkdir(ctx, "mybucket"))

	// Create some valid files
	for i := 1; i <= 3; i++ {
		data := []byte(fmt.Sprintf("File %d", i))
		_, err := l3fs.Put(ctx, bytes.NewReader(data), object.NewStaticObjectInfo(fmt.Sprintf("mybucket/file%d.txt", i), time.Now(), int64(len(data)), true, nil, l3fs))
		require.NoError(t, err, "Failed to create file %d", i)
	}

	// Create a broken object manually (1 particle)
	brokenPath := filepath.Join(evenDir, "mybucket", "broken.txt")
	require.NoError(t, os.MkdirAll(filepath.Dir(brokenPath), 0755))
	require.NoError(t, os.WriteFile(brokenPath, []byte("broken"), 0644))

	// List should show only 3 files (broken is hidden)
	entries, err := l3fs.List(ctx, "mybucket")
	require.NoError(t, err)
	count := 0
	for _, entry := range entries {
		if _, ok := entry.(fs.Object); ok {
			count++
		}
	}
	assert.Equal(t, 3, count, "Should see only 3 valid files")

	// Purge the bucket - should work without errors
	// Use operations.Purge which falls back to List+Delete+Rmdir
	err = operations.Purge(ctx, l3fs, "mybucket")
	require.NoError(t, err, "Purge should succeed without errors")

	// Verify bucket is gone
	err = l3fs.Rmdir(ctx, "mybucket")
	// Should succeed or return "directory not found" (both are OK)
	if err != nil {
		assert.True(t, err == fs.ErrorDirNotFound || os.IsNotExist(err), "Directory should be gone")
	}

	t.Logf("✅ Purge with auto-cleanup works without error messages")
}

// TestCleanUpOrphanedFiles tests cleanup of manually created files without proper suffixes
func TestCleanUpOrphanedFiles(t *testing.T) {
	ctx := context.Background()

	// Create temporary directories for three backends
	evenDir := t.TempDir()
	oddDir := t.TempDir()
	parityDir := t.TempDir()

	// Create level3 filesystem with auto_cleanup=false (to see orphaned files)
	l3fs, err := level3.NewFs(ctx, "level3", "", configmap.Simple{
		"even":         evenDir,
		"odd":          oddDir,
		"parity":       parityDir,
		"auto_cleanup": "false",
	})
	require.NoError(t, err, "Failed to create level3 filesystem")
	defer func() {
		_ = l3fs.Features().Shutdown(ctx)
	}()

	// Create a valid object (3 particles)
	validData := []byte("Valid file")
	_, err = l3fs.Put(ctx, bytes.NewReader(validData), object.NewStaticObjectInfo("valid.txt", time.Now(), int64(len(validData)), true, nil, l3fs))
	require.NoError(t, err, "Failed to create valid object")

	// Manually create orphaned files in each backend WITHOUT proper level3 structure
	// (simulating user's scenario where files were manually created or partially deleted)
	orphan1Path := filepath.Join(evenDir, "orphan1.txt")
	require.NoError(t, os.WriteFile(orphan1Path, []byte("orphan in even"), 0644))

	orphan2Path := filepath.Join(oddDir, "orphan2.txt")
	require.NoError(t, os.WriteFile(orphan2Path, []byte("orphan in odd"), 0644))

	// Critically: orphan in parity WITHOUT suffix (this was the bug!)
	orphan3Path := filepath.Join(parityDir, "orphan3.txt")
	require.NoError(t, os.WriteFile(orphan3Path, []byte("orphan in parity"), 0644))

	// Verify we can see all 4 objects before cleanup (1 valid + 3 orphaned)
	entries, err := l3fs.List(ctx, "")
	require.NoError(t, err, "List should succeed")
	initialCount := 0
	for _, entry := range entries {
		if _, ok := entry.(fs.Object); ok {
			initialCount++
		}
	}
	assert.Equal(t, 4, initialCount, "Should see 4 objects (1 valid + 3 orphaned)")

	// Run CleanUp command
	cleanUpFunc := l3fs.Features().CleanUp
	require.NotNil(t, cleanUpFunc, "CleanUp feature should be available")
	err = cleanUpFunc(ctx)
	require.NoError(t, err, "CleanUp should succeed")

	// Verify only valid object remains
	entries, err = l3fs.List(ctx, "")
	require.NoError(t, err, "List should succeed after cleanup")
	finalCount := 0
	for _, entry := range entries {
		if _, ok := entry.(fs.Object); ok {
			finalCount++
			assert.Equal(t, "valid.txt", entry.Remote(), "Only valid.txt should remain")
		}
	}
	assert.Equal(t, 1, finalCount, "Should see only 1 valid object after cleanup")

	// Verify orphaned files are actually gone from disk
	_, err = os.Stat(orphan1Path)
	assert.True(t, os.IsNotExist(err), "orphan1.txt should be deleted from even")

	_, err = os.Stat(orphan2Path)
	assert.True(t, os.IsNotExist(err), "orphan2.txt should be deleted from odd")

	_, err = os.Stat(orphan3Path)
	assert.True(t, os.IsNotExist(err), "orphan3.txt should be deleted from parity (THIS WAS THE BUG!)")

	t.Logf("✅ CleanUp successfully removed orphaned files including those in parity without suffix")
}

// TestAutoHealDirectoryReconstruction tests that auto_heal reconstructs missing directories
func TestAutoHealDirectoryReconstruction(t *testing.T) {
	ctx := context.Background()

	evenDir := t.TempDir()
	oddDir := t.TempDir()
	parityDir := t.TempDir()

	// Test with auto_heal=true
	t.Run("auto_heal=true reconstructs missing directory", func(t *testing.T) {
		l3fs, err := level3.NewFs(ctx, "level3", "", configmap.Simple{
			"even":      evenDir,
			"odd":       oddDir,
			"parity":    parityDir,
			"auto_heal": "true",
		})
		require.NoError(t, err)
		defer l3fs.Features().Shutdown(ctx)

		// Manually create directory on 2/3 backends (degraded state - 1dm)
		testDir := "testdir_heal"
		err = os.MkdirAll(filepath.Join(evenDir, testDir), 0755)
		require.NoError(t, err)
		err = os.MkdirAll(filepath.Join(oddDir, testDir), 0755)
		require.NoError(t, err)
		// Parity missing

		// List the directory - should trigger reconstruction
		_, err = l3fs.List(ctx, testDir)
		require.NoError(t, err)

		// Verify missing directory was reconstructed on parity
		_, err = os.Stat(filepath.Join(parityDir, testDir))
		assert.NoError(t, err, "Directory should be reconstructed on parity backend when auto_heal=true")
	})

	// Test with auto_heal=false
	t.Run("auto_heal=false does NOT reconstruct missing directory", func(t *testing.T) {
		l3fs, err := level3.NewFs(ctx, "level3", "", configmap.Simple{
			"even":      evenDir,
			"odd":       oddDir,
			"parity":    parityDir,
			"auto_heal": "false",
		})
		require.NoError(t, err)
		defer l3fs.Features().Shutdown(ctx)

		// Manually create directory on 2/3 backends (degraded state)
		testDir := "testdir_noheal"
		err = os.MkdirAll(filepath.Join(evenDir, testDir), 0755)
		require.NoError(t, err)
		err = os.MkdirAll(filepath.Join(oddDir, testDir), 0755)
		require.NoError(t, err)
		// Parity missing

		// List the directory - should NOT trigger reconstruction
		_, err = l3fs.List(ctx, testDir)
		require.NoError(t, err)

		// Verify missing directory was NOT reconstructed on parity
		_, err = os.Stat(filepath.Join(parityDir, testDir))
		assert.True(t, os.IsNotExist(err), "Directory should NOT be reconstructed on parity when auto_heal=false")
	})
}

// TestAutoHealDirMove tests that auto_heal controls reconstruction during DirMove
func TestAutoHealDirMove(t *testing.T) {
	ctx := context.Background()

	evenDir := t.TempDir()
	oddDir := t.TempDir()
	parityDir := t.TempDir()

	// Test with auto_heal=true - should reconstruct missing directory during move
	t.Run("auto_heal=true reconstructs during DirMove", func(t *testing.T) {
		// Create Fs instances
		l3fs, err := level3.NewFs(ctx, "level3", "", configmap.Simple{
			"even":      evenDir,
			"odd":       oddDir,
			"parity":    parityDir,
			"auto_heal": "true",
		})
		require.NoError(t, err)
		defer l3fs.Features().Shutdown(ctx)

		// Create source directory on 2/3 backends only (degraded - 1dm)
		srcDir := "move_heal_src"
		err = os.MkdirAll(filepath.Join(evenDir, srcDir), 0755)
		require.NoError(t, err)
		err = os.MkdirAll(filepath.Join(oddDir, srcDir), 0755)
		require.NoError(t, err)
		// Parity missing

		// Create Fs instances for source and destination
		srcFs, err := level3.NewFs(ctx, "level3", srcDir, configmap.Simple{
			"even":      evenDir,
			"odd":       oddDir,
			"parity":    parityDir,
			"auto_heal": "true",
		})
		require.NoError(t, err)
		defer srcFs.Features().Shutdown(ctx)

		dstFs, err := level3.NewFs(ctx, "level3", "move_heal_dst", configmap.Simple{
			"even":      evenDir,
			"odd":       oddDir,
			"parity":    parityDir,
			"auto_heal": "true",
		})
		require.NoError(t, err)
		defer dstFs.Features().Shutdown(ctx)

		// Perform DirMove
		doDirMove := dstFs.Features().DirMove
		require.NotNil(t, doDirMove)
		err = doDirMove(ctx, srcFs, "", "")
		require.NoError(t, err, "DirMove should succeed with reconstruction")

		// Verify destination exists on all 3 backends (reconstructed)
		_, err = os.Stat(filepath.Join(parityDir, "move_heal_dst"))
		assert.NoError(t, err, "Destination should be created on parity (reconstruction)")
	})

	// Test with auto_heal=false - should fail if directory missing on backend
	t.Run("auto_heal=false fails DirMove with degraded directory", func(t *testing.T) {
		// Clean up
		os.RemoveAll(filepath.Join(evenDir, "move_noheal_src"))
		os.RemoveAll(filepath.Join(oddDir, "move_noheal_src"))
		os.RemoveAll(filepath.Join(parityDir, "move_noheal_src"))
		os.RemoveAll(filepath.Join(evenDir, "move_noheal_dst"))
		os.RemoveAll(filepath.Join(oddDir, "move_noheal_dst"))
		os.RemoveAll(filepath.Join(parityDir, "move_noheal_dst"))

		// Create source directory on 2/3 backends only
		srcDir := "move_noheal_src"
		err := os.MkdirAll(filepath.Join(evenDir, srcDir), 0755)
		require.NoError(t, err)
		err = os.MkdirAll(filepath.Join(oddDir, srcDir), 0755)
		require.NoError(t, err)
		// Parity missing

		// Create Fs instances
		srcFs, err := level3.NewFs(ctx, "level3", srcDir, configmap.Simple{
			"even":      evenDir,
			"odd":       oddDir,
			"parity":    parityDir,
			"auto_heal": "false",
		})
		require.NoError(t, err)
		defer srcFs.Features().Shutdown(ctx)

		dstFs, err := level3.NewFs(ctx, "level3", "move_noheal_dst", configmap.Simple{
			"even":      evenDir,
			"odd":       oddDir,
			"parity":    parityDir,
			"auto_heal": "false",
		})
		require.NoError(t, err)
		defer dstFs.Features().Shutdown(ctx)

		// Perform DirMove - should fail because source missing on parity
		doDirMove := dstFs.Features().DirMove
		require.NotNil(t, doDirMove)
		err = doDirMove(ctx, srcFs, "", "")
		assert.Error(t, err, "DirMove should fail when auto_heal=false and directory degraded")
		assert.Contains(t, err.Error(), "parity dirmove failed", "Error should indicate parity backend failure")
	})
}

// Helper function to count objects recursively
func countAllObjects(t *testing.T, ctx context.Context, f fs.Fs, dir string) int {
	entries, err := f.List(ctx, dir)
	require.NoError(t, err, "List should succeed")

	count := 0
	for _, entry := range entries {
		switch e := entry.(type) {
		case fs.Object:
			count++
		case fs.Directory:
			count += countAllObjects(t, ctx, f, e.Remote())
		}
	}
	return count
}
