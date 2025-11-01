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

// TestIntegration runs integration tests against a configured remote
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

// TestStandard runs standard integration tests with temporary directories
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

// Unit tests below

// TestSplitBytes tests the byte splitting function
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

// TestMergeBytes tests the byte merging function
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

// TestSplitMergeRoundtrip tests that split and merge are inverse operations
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

// TestValidateParticleSizes tests the particle size validation
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

// TestCalculateParity tests the XOR parity calculation
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

// TestParityFilenames tests the parity filename generation and parsing
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

// TestParityReconstruction tests reconstructing data from parity
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

// TestLargeDataQuick writes ~1 MiB to exercise split/parity paths without taking long.
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
