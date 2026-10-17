package khash

import "testing"

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
			name:         "nested hash",
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
				t.Errorf("IsNested: got %t, expected %t", isNested, tt.isNested)
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

func TestNestedChunkHash(t *testing.T) {
	// Golden values: the nested hash is the XXH3 of the concatenated
	// lowercase hex strings of the chunk hashes, in order
	chunk1 := "xxh3:d447b1ea40e6988b" // XXH3 of "hello world"
	chunk2 := "2d06800538d394c2"      // XXH3 of ""

	tests := []struct {
		name        string
		chunkHashes []string
		expected    string
		expectError bool
	}{
		{
			name:        "two chunks in order",
			chunkHashes: []string{chunk1, chunk2},
			expected:    "6d51edccc9c05a8b",
		},
		{
			name:        "order matters",
			chunkHashes: []string{chunk2, chunk1},
			expected:    "596c127c7bcc8820",
		},
		{
			name:        "single chunk",
			chunkHashes: []string{chunk1},
			expected:    "b73081dff8fe443c",
		},
		{
			name:        "mixed prefixes",
			chunkHashes: []string{chunk1, "xxh3:" + chunk2, chunk1},
			expected:    "ca0b13abcdcd1661",
		},
		{
			name:        "no chunks",
			chunkHashes: []string{},
			expectError: true,
		},
		{
			name:        "invalid chunk hash",
			chunkHashes: []string{chunk1, "notahexvalue"},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			nested, err := NestedChunkHash(tt.chunkHashes)

			if tt.expectError {
				if err == nil {
					t.Error("Expected error but got none")
				}
				return
			}

			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			if nested != tt.expected {
				t.Errorf("NestedChunkHash: got %q, expected %q", nested, tt.expected)
			}
		})
	}
}

func TestNestedChunkHashAcceptsPrefixedHashes(t *testing.T) {
	plain := []string{"d447b1ea40e6988b", "2d06800538d394c2"}
	prefixed := []string{"xxh3:d447b1ea40e6988b", "N:xxh3:2d06800538d394c2"}

	gotPlain, err := NestedChunkHash(plain)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	gotPrefixed, err := NestedChunkHash(prefixed)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if gotPlain != gotPrefixed {
		t.Errorf("Prefixed hashes should give the same nested hash: %q != %q", gotPlain, gotPrefixed)
	}
}
