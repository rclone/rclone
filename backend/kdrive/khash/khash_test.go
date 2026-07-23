package khash

import (
	"bytes"
	"encoding/hex"
	"io"
	"testing"

	"github.com/rclone/rclone/backend/kdrive/chunksize"
	"github.com/zeebo/xxh3"
)

func TestNew(t *testing.T) {
	h := New()
	if h == nil {
		t.Fatal("New() returned nil")
	}
	if h.Size() != 8 {
		t.Errorf("Expected Size() = 8, got %d", h.Size())
	}
}

func TestWriteAndSum(t *testing.T) {
	tests := []struct {
		name     string
		data     []byte
		expected string // hex string of expected hash
	}{
		{
			name:     "empty data",
			data:     []byte{},
			expected: "2d06800538d394c2", // XXH3 of empty data (64-bit)
		},
		{
			name:     "small data",
			data:     []byte("hello world"),
			expected: "d447b1ea40e6988b", // XXH3 of "hello world"
		},
		{
			name:     "1 MB data",
			data:     bytes.Repeat([]byte("a"), 1024*1024),
			expected: "", // Will validate against direct xxh3
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := NewWithChunkSize(chunksize.ChunkSizeConfig.DefaultChunkSize)
			n, err := h.Write(tt.data)
			if err != nil {
				t.Fatalf("Write failed: %v", err)
			}
			if n != len(tt.data) {
				t.Errorf("Write returned %d, expected %d", n, len(tt.data))
			}

			sum := h.Sum(nil)
			sumHex := hex.EncodeToString(sum)

			// For empty and small data, compare with expected
			if tt.expected != "" {
				if sumHex != tt.expected {
					t.Errorf("Hash mismatch: got %s, expected %s", sumHex, tt.expected)
				}
			} else {
				// Compare with direct xxh3 calculation
				hasher := xxh3.New()
				_, err = hasher.Write(tt.data)
				if err != nil {
					t.Fatalf("Write failed: %v", err)
				}
				expected := hex.EncodeToString(hasher.Sum(nil))
				if sumHex != expected {
					t.Errorf("Hash mismatch: got %s, expected %s", sumHex, expected)
				}
			}
		})
	}
}

func TestWriteAndSumSimple(t *testing.T) {
	// Test that New() creates a simple (non-chunked) hasher
	tests := []struct {
		name     string
		data     []byte
		expected string
	}{
		{
			name:     "empty data",
			data:     []byte{},
			expected: "2d06800538d394c2", // XXH3 of empty data (64-bit)
		},
		{
			name:     "small data",
			data:     []byte("hello world"),
			expected: "d447b1ea40e6988b", // XXH3 of "hello world"
		},
		{
			name:     "large data",
			data:     bytes.Repeat([]byte("a"), 1024*1024),
			expected: "", // Will validate against direct xxh3
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := New()
			n, err := h.Write(tt.data)
			if err != nil {
				t.Fatalf("Write failed: %v", err)
			}
			if n != len(tt.data) {
				t.Errorf("Write returned %d, expected %d", n, len(tt.data))
			}

			sum := h.Sum(nil)
			sumHex := hex.EncodeToString(sum)

			if tt.expected != "" {
				if sumHex != tt.expected {
					t.Errorf("Hash mismatch: got %s, expected %s", sumHex, tt.expected)
				}
			} else {
				hasher := xxh3.New()
				_, err = hasher.Write(tt.data)
				if err != nil {
					t.Fatalf("Write failed: %v", err)
				}
				expected := hex.EncodeToString(hasher.Sum(nil))
				if sumHex != expected {
					t.Errorf("Hash mismatch: got %s, expected %s", sumHex, expected)
				}
			}

			// Verify it's a simple hasher (no chunking)
			d := h.(*digest)
			if d.chunkSize != nil {
				t.Error("New() should create a simple hasher with nil chunkSize")
			}
		})
	}
}

func TestNestedHash(t *testing.T) {
	// Test that files larger than chunk size produce nested hash
	smallChunkSize := int64(1024) // 1KB for testing

	// Data exactly 1 chunk
	data1Chunk := bytes.Repeat([]byte("a"), 1024)
	h1 := NewWithChunkSize(smallChunkSize)
	h1.Write(data1Chunk)
	sum1 := hex.EncodeToString(h1.Sum(nil))

	// Data spanning 2 chunks
	data2Chunks := bytes.Repeat([]byte("a"), 1024+512)
	h2 := NewWithChunkSize(smallChunkSize)
	h2.Write(data2Chunks)
	sum2 := hex.EncodeToString(h2.Sum(nil))

	// Data spanning 3 chunks
	data3Chunks := bytes.Repeat([]byte("a"), 2048)
	h3 := NewWithChunkSize(smallChunkSize)
	h3.Write(data3Chunks)
	sum3 := hex.EncodeToString(h3.Sum(nil))

	// Verify they're different
	if sum1 == sum2 || sum2 == sum3 {
		t.Error("Hashes for different chunk counts should be different")
	}

	// Verify single chunk matches direct XXH3
	directHash := xxh3.New()
	_, err := directHash.Write(data1Chunk)
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}
	expected1 := hex.EncodeToString(directHash.Sum(nil))
	if sum1 != expected1 {
		t.Errorf("Single chunk hash mismatch: got %s, expected %s", sum1, expected1)
	}
}

func TestParseHash(t *testing.T) {
	tests := []struct {
		name         string
		input        string
		expectedHash string
		isNested     bool
		expectError  bool
	}{
		{
			name:         "simple hash with prefix",
			input:        "xxh3:1db797b96febd334",
			expectedHash: "1db797b96febd334",
			isNested:     false,
		},
		{
			name:         "hash with chunk count 1",
			input:        "N:xxh3:877abf3579f0a5c0",
			expectedHash: "877abf3579f0a5c0",
			isNested:     true,
		},
		{
			name:         "plain hex",
			input:        "1db797b96febd334",
			expectedHash: "1db797b96febd334",
			isNested:     false,
		},
		{
			name:         "empty string",
			input:        "",
			expectedHash: "",
			isNested:     false,
		},
		{
			name:        "invalid hex",
			input:       "xxh3:notahexvalue",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hash, isNested, err := ParseHash(tt.input)

			if tt.expectError {
				if err == nil {
					t.Error("Expected error but got none")
				}
				return
			}

			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			if hash != tt.expectedHash {
				t.Errorf("Hash: got %q, expected %q", hash, tt.expectedHash)
			}
			if isNested != tt.isNested {
				t.Errorf("ChunkCount: got %t, expected %t", isNested, tt.isNested)
			}
		})
	}
}

func TestValidateHash(t *testing.T) {
	tests := []struct {
		name        string
		localHash   string
		remoteHash  string
		expectMatch bool
		expectError bool
	}{
		{
			name:        "exact match",
			localHash:   "1db797b96febd334",
			remoteHash:  "xxh3:1db797b96febd334",
			expectMatch: true,
		},
		{
			name:        "nested hash",
			localHash:   "877abf3579f0a5c0",
			remoteHash:  "N:xxh3:877abf3579f0a5c0",
			expectMatch: true,
		},
		{
			name:        "case insensitive match",
			localHash:   "1DB797B96FEBD334",
			remoteHash:  "xxh3:1db797b96febd334",
			expectMatch: true,
		},
		{
			name:        "mismatch",
			localHash:   "1db797b96febd334",
			remoteHash:  "xxh3:0000000000000000",
			expectMatch: false,
		},
		{
			name:        "invalid remote hash",
			localHash:   "1db797b96febd334",
			remoteHash:  "xxh3:invalid",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			match, err := ValidateHash(tt.localHash, tt.remoteHash)

			if tt.expectError {
				if err == nil {
					t.Error("Expected error but got none")
				}
				return
			}

			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			if match != tt.expectMatch {
				t.Errorf("ValidateHash: got match=%v, expected %v", match, tt.expectMatch)
			}
		})
	}
}

func TestReset(t *testing.T) {
	h := New()
	h.Write([]byte("some data"))
	sum1 := hex.EncodeToString(h.Sum(nil))

	h.Reset()
	h.Write([]byte("some data"))
	sum2 := hex.EncodeToString(h.Sum(nil))

	if sum1 != sum2 {
		t.Errorf("Reset failed: got %s, expected %s", sum2, sum1)
	}
}

func TestBlockSize(t *testing.T) {
	h := New()
	bs := h.BlockSize()
	if bs <= 0 {
		t.Errorf("BlockSize returned %d, expected positive value", bs)
	}
}

func TestEmptyData(t *testing.T) {
	h := New()
	sum := h.Sum(nil)

	// Should produce a valid hash even for empty data (XXH3 of empty is not zero)
	if len(sum) != 8 {
		t.Errorf("Empty data sum has wrong length: %d", len(sum))
	}
}

func TestChunkSizeBoundary(t *testing.T) {
	// Test exactly at chunk boundary
	data := bytes.Repeat([]byte("x"), int(chunksize.ChunkSizeConfig.DefaultChunkSize))

	h := NewWithChunkSize(chunksize.ChunkSizeConfig.DefaultChunkSize)
	h.Write(data)
	sum := h.Sum(nil)

	// Should be a single hash (not nested) since it's exactly one chunk
	d := h.(*digest)
	if len(d.chunkHashes) != 8 {
		t.Errorf("Expected 1 chunk (8 bytes) at exact boundary, got %d", len(d.chunkHashes))
	}

	// Verify it matches direct XXH3
	hasher := xxh3.New()
	_, err := hasher.Write(data)
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}
	expected := hasher.Sum(nil)

	if !bytes.Equal(sum, expected) {
		t.Errorf("Boundary hash mismatch")
	}
}

func TestLargeFile(t *testing.T) {
	// Simulate a 100MB file
	data := bytes.Repeat([]byte("large file content "), 100*1024*1024/18)

	h := New()
	_, err := io.Copy(h, bytes.NewReader(data))
	if err != nil {
		t.Fatalf("Failed to hash large file: %v", err)
	}

	sum := h.Sum(nil)
	if len(sum) != 8 {
		t.Errorf("Large file hash has wrong length: %d", len(sum))
	}
}
