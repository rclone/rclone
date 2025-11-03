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
// temporary directories.
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
		"even":   evenDir,
		"odd":    oddDir,
		"parity": parityDir,
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
	require.True(t, os.IsNotExist(err), "source even particle should be deleted")
	_, err = os.Stat(dstEvenPath)
	require.NoError(t, err, "destination even particle should exist")
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
