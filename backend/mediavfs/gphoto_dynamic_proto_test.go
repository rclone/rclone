package mediavfs

import (
	"testing"
)

func TestIsProbablyString(t *testing.T) {
	tests := []struct {
		name     string
		input    []byte
		expected bool
	}{
		{
			name:     "empty string",
			input:    []byte{},
			expected: true,
		},
		{
			name:     "simple filename",
			input:    []byte("test.mp4"),
			expected: true,
		},
		{
			name:     "filename with dots and numbers",
			input:    []byte("analvids.20.01.13.lauren.phillips.gio1281.piss.version.4k.mp4"),
			expected: true,
		},
		{
			name:     "filename with spaces",
			input:    []byte("my video file.mp4"),
			expected: true,
		},
		{
			name:     "filename with unicode",
			input:    []byte("日本語ファイル.mp4"),
			expected: true,
		},
		{
			name:     "binary data with null bytes",
			input:    []byte{0x00, 0x01, 0x02, 0x03, 0x04, 0x05},
			expected: false,
		},
		{
			name:     "protobuf-like binary",
			input:    []byte{0x08, 0x01, 0x10, 0x02, 0x18, 0x03},
			expected: false,
		},
		{
			name:     "mixed printable and binary",
			input:    []byte{0x61, 0x00, 0x62, 0x00, 0x63, 0x00}, // a\0b\0c\0
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isProbablyString(tt.input)
			if result != tt.expected {
				t.Errorf("isProbablyString(%q) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}

func TestDecodeDynamicMessage_FilenameNotCorrupted(t *testing.T) {
	// This test verifies that a filename string is not incorrectly parsed as a nested message
	// The filename "analvids.20.01.13..." should remain a string, not become a map

	filename := "analvids.20.01.13.lauren.phillips.gio1281.piss.version.4k.mp4"

	// Encode a simple message with field 4 containing the filename
	msg := map[string]interface{}{
		"4": filename,
	}

	encoded, err := EncodeDynamicMessage(msg)
	if err != nil {
		t.Fatalf("EncodeDynamicMessage failed: %v", err)
	}

	// Decode it back
	decoded, err := DecodeDynamicMessage(encoded)
	if err != nil {
		t.Fatalf("DecodeDynamicMessage failed: %v", err)
	}

	// Field 4 should still be a string, not a map
	field4 := decoded["4"]
	decodedFilename, ok := field4.(string)
	if !ok {
		t.Fatalf("Field 4 should be a string, got %T: %v", field4, field4)
	}

	if decodedFilename != filename {
		t.Errorf("Filename corrupted: got %q, want %q", decodedFilename, filename)
	}
}
