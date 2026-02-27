package raid3

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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
			gotEven, gotOdd := SplitBytes(tt.input)
			assert.Equal(t, tt.wantEven, gotEven, "even bytes mismatch")
			assert.Equal(t, tt.wantOdd, gotOdd, "odd bytes mismatch")
		})
	}
}

// TestSplitBytesWithOffset verifies that splitting with global offset produces
// correct even/odd assignment across chunk boundaries (e.g. two 1-byte reads
// at positions 0 and 1 must yield one byte to even and one to odd, not both to even).
func TestSplitBytesWithOffset(t *testing.T) {
	// Offset 0 must match SplitBytes
	t.Run("offset_zero_matches_SplitBytes", func(t *testing.T) {
		data := []byte{0x01, 0x02, 0x03, 0x04, 0x05}
		even0, odd0 := SplitBytes(data)
		even1, odd1 := SplitBytesWithOffset(data, 0)
		assert.Equal(t, even0, even1)
		assert.Equal(t, odd0, odd1)
	})
	// Two 1-byte reads: position 0 -> even, position 1 -> odd (regression test for compression even=odd+2)
	t.Run("two_one_byte_reads", func(t *testing.T) {
		e0, o0 := SplitBytesWithOffset([]byte{0xAA}, 0)
		e1, o1 := SplitBytesWithOffset([]byte{0xBB}, 1)
		require.Len(t, e0, 1)
		require.Len(t, o0, 0)
		require.Len(t, e1, 0)
		require.Len(t, o1, 1)
		assert.Equal(t, byte(0xAA), e0[0])
		assert.Equal(t, byte(0xBB), o1[0])
		// Combined: even has 1 byte, odd has 1 byte (valid; was broken as even=2, odd=0)
		merged, err := MergeBytes(append(e0, e1...), append(o0, o1...))
		require.NoError(t, err)
		assert.Equal(t, []byte{0xAA, 0xBB}, merged)
	})
	// Chunked split with offset matches single SplitBytes of full stream
	t.Run("chunked_matches_whole", func(t *testing.T) {
		full := []byte{0x10, 0x20, 0x30, 0x40, 0x50, 0x60}
		evenFull, oddFull := SplitBytes(full)
		var evenChunk, oddChunk []byte
		for off := 0; off < len(full); off += 2 {
			chunk := full[off:]
			if len(chunk) > 2 {
				chunk = chunk[:2]
			}
			e, o := SplitBytesWithOffset(chunk, off)
			evenChunk = append(evenChunk, e...)
			oddChunk = append(oddChunk, o...)
		}
		assert.Equal(t, evenFull, evenChunk)
		assert.Equal(t, oddFull, oddChunk)
	})
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
			got, err := MergeBytes(tt.even, tt.odd)
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
			even, odd := SplitBytes(data)

			// Verify sizes
			assert.Equal(t, (len(data)+1)/2, len(even), "even size")
			assert.Equal(t, len(data)/2, len(odd), "odd size")

			// Merge
			result, err := MergeBytes(even, odd)
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
			got := ValidateParticleSizes(tt.evenSize, tt.oddSize)
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
// even particle (no XOR partner). This parity enables rebuild when one
// data particle is missing.
//
// This test verifies:
//   - Correct XOR calculation for byte pairs
//   - Last byte handling for odd-length originals
//   - Empty input handling
//   - Various data patterns
//   - Real-world data (text strings)
//
// Failure indicates: Parity would be incorrect, preventing rebuild in
// degraded mode. Heal would upload wrong data.
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
			name:       "odd one larger (from SplitBytesWithOffset offset-odd chunk)",
			even:       []byte{0xAA},
			odd:        []byte{0xBB, 0xCC},
			wantParity: []byte{0xAA ^ 0xBB}, // parity length = len(even); one odd byte has no partner
		},
		{
			name:       "Hello, World!",
			even:       []byte{'H', 'l', 'o', ' ', 'o', 'l', '!'},
			odd:        []byte{'e', 'l', ',', 'W', 'r', 'd'},
			wantParity: []byte{'H' ^ 'e', 0, 'o' ^ ',', ' ' ^ 'W', 'o' ^ 'r', 'l' ^ 'd', '!'}, // 0 is 'l'^'l' (both even and odd indices are 'l')
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CalculateParity(tt.even, tt.odd)
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
			got := GetParityFilename(tt.original, tt.isOddLength)
			assert.Equal(t, tt.want, got)

			// Test strip parity suffix
			original, isParity, isOddLength := StripParitySuffix(got)
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
// rebuild in degraded mode.
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
	even, odd := SplitBytes(original)
	parity := CalculateParity(even, odd)

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
// Heal would not work for odd particles.
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
			even, odd := SplitBytes(original)
			parity := CalculateParity(even, odd)
			isOdd := len(original)%2 == 1
			got, err := ReconstructFromEvenAndParity(even, parity, isOdd)
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
// Heal would not work for even particles.
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
			even, odd := SplitBytes(original)
			parity := CalculateParity(even, odd)
			isOdd := len(original)%2 == 1
			got, err := ReconstructFromOddAndParity(odd, parity, isOdd)
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
			even, odd := SplitBytes(original)
			parity := CalculateParity(even, odd)
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
