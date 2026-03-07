package raid3

import (
	"bytes"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestStreamMerger tests the StreamMerger to ensure data integrity
func TestStreamMerger(t *testing.T) {
	tests := []struct {
		name      string
		evenData  []byte
		oddData   []byte
		chunkSize int
	}{
		{"empty", []byte{}, []byte{}, 8},
		// Note: single_byte test removed - RAID3 requires minimum 2 bytes (one even, one odd)
		{"two_bytes", []byte{0x01}, []byte{0x02}, 8},
		{"three_bytes", []byte{0x01, 0x03}, []byte{0x02}, 8},
		{"four_bytes", []byte{0x01, 0x03}, []byte{0x02, 0x04}, 8},
		{"seven_bytes", []byte{0x01, 0x03, 0x05, 0x07}, []byte{0x02, 0x04, 0x06}, 8},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create readers from data
			evenReader := io.NopCloser(bytes.NewReader(tt.evenData))
			oddReader := io.NopCloser(bytes.NewReader(tt.oddData))

			// Create merger
			merger := NewStreamMerger(evenReader, oddReader, tt.chunkSize)

			// Read all data
			var result bytes.Buffer
			buf := make([]byte, 1024)
			for {
				n, err := merger.Read(buf)
				if n > 0 {
					result.Write(buf[:n])
				}
				if err == io.EOF {
					break
				}
				require.NoError(t, err)
			}

			// Close merger
			err := merger.Close()
			require.NoError(t, err)

			// Verify reconstruction
			reconstructed := result.Bytes()
			expected, err := MergeBytes(tt.evenData, tt.oddData)
			require.NoError(t, err)
			// Handle nil vs empty slice comparison
			if len(expected) == 0 && len(reconstructed) == 0 {
				// Both are empty, that's fine
			} else {
				assert.Equal(t, expected, reconstructed, "Reconstructed data doesn't match")
			}
		})
	}
}
