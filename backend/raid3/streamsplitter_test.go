package raid3

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestStreamSplitter tests the StreamSplitter to ensure data integrity
func TestStreamSplitter(t *testing.T) {
	tests := []struct {
		name      string
		data      []byte
		chunkSize int
	}{
		{"empty", []byte{}, 8},
		{"single_byte", []byte{0x01}, 8},
		{"two_bytes", []byte{0x01, 0x02}, 8},
		{"three_bytes", []byte{0x01, 0x02, 0x03}, 8},
		{"four_bytes", []byte{0x01, 0x02, 0x03, 0x04}, 8},
		{"seven_bytes", []byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07}, 8},
		{"large_chunk", bytes.Repeat([]byte{0x01, 0x02, 0x03, 0x04}, 100), 8},
		{"small_chunk_size", []byte{0x01, 0x02, 0x03, 0x04, 0x05}, 2},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create buffers to capture output
			var evenBuf, oddBuf, parityBuf bytes.Buffer

			// Create splitter
			splitter := NewStreamSplitter(&evenBuf, &oddBuf, &parityBuf, tt.chunkSize, nil)

			// Split data using StreamSplitter
			reader := bytes.NewReader(tt.data)
			err := splitter.Split(reader)
			require.NoError(t, err)

			// Verify data integrity: reconstruct and compare
			evenData := evenBuf.Bytes()
			oddData := oddBuf.Bytes()
			parityData := parityBuf.Bytes()

			// Reconstruct original data
			reconstructed, err := MergeBytes(evenData, oddData)
			require.NoError(t, err)

			// Compare with original
			assert.Equal(t, tt.data, reconstructed, "Reconstructed data doesn't match original")

			// Verify parity (skip for empty case)
			if len(tt.data) > 0 {
				expectedParity := CalculateParity(evenData, oddData)
				assert.Equal(t, expectedParity, parityData, "Parity doesn't match")
			} else {
				// Empty case: parity should also be empty
				assert.Equal(t, 0, len(parityData), "Parity should be empty for empty input")
			}

			// Verify sizes
			expectedEvenLen := (len(tt.data) + 1) / 2
			expectedOddLen := len(tt.data) / 2
			assert.Equal(t, expectedEvenLen, len(evenData), "Even buffer size incorrect")
			assert.Equal(t, expectedOddLen, len(oddData), "Odd buffer size incorrect")
		})
	}
}

// TestStreamSplitterMultipleWrites tests splitting data across multiple chunks
func TestStreamSplitterMultipleWrites(t *testing.T) {
	// Create buffers to capture output
	var evenBuf, oddBuf, parityBuf bytes.Buffer

	// Create splitter
	splitter := NewStreamSplitter(&evenBuf, &oddBuf, &parityBuf, 8, nil)

	// Write data in multiple chunks
	data1 := []byte{0x01, 0x02, 0x03}
	data2 := []byte{0x04, 0x05, 0x06}
	data3 := []byte{0x07, 0x08}

	// Combine all data and split using StreamSplitter
	allData := append(append(data1, data2...), data3...)
	reader := bytes.NewReader(allData)
	err := splitter.Split(reader)
	require.NoError(t, err)

	// Reconstruct
	evenData := evenBuf.Bytes()
	oddData := oddBuf.Bytes()
	reconstructed, err := MergeBytes(evenData, oddData)
	require.NoError(t, err)

	// Expected: [0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08]
	expected := append(append(data1, data2...), data3...)
	assert.Equal(t, expected, reconstructed, "Reconstructed data doesn't match")
}

// TestStreamSplitterOddLengthDetection tests odd-length detection via channel
func TestStreamSplitterOddLengthDetection(t *testing.T) {
	// Create buffers to capture output
	var evenBuf, oddBuf, parityBuf bytes.Buffer

	// Create channel for odd-length detection
	isOddLengthCh := make(chan bool, 1)
	isOddLengthCh <- false // Default to even-length

	// Create splitter with channel
	splitter := NewStreamSplitter(&evenBuf, &oddBuf, &parityBuf, 8, isOddLengthCh)

	// Test with odd-length data (7 bytes)
	oddLengthData := []byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07}
	reader := bytes.NewReader(oddLengthData)
	err := splitter.Split(reader)
	require.NoError(t, err)

	// Check that channel was updated to indicate odd-length
	select {
	case isOddLength := <-isOddLengthCh:
		assert.True(t, isOddLength, "Should detect odd-length file")
	default:
		t.Fatal("Channel should have been updated with odd-length detection")
	}

	// Verify data integrity
	evenData := evenBuf.Bytes()
	oddData := oddBuf.Bytes()
	reconstructed, err := MergeBytes(evenData, oddData)
	require.NoError(t, err)
	assert.Equal(t, oddLengthData, reconstructed, "Reconstructed data doesn't match")
}

// TestStreamSplitterEvenLengthDetection tests that even-length files don't trigger odd-length detection
func TestStreamSplitterEvenLengthDetection(t *testing.T) {
	// Create buffers to capture output
	var evenBuf, oddBuf, parityBuf bytes.Buffer

	// Create channel for odd-length detection
	isOddLengthCh := make(chan bool, 1)
	isOddLengthCh <- false // Default to even-length

	// Create splitter with channel
	splitter := NewStreamSplitter(&evenBuf, &oddBuf, &parityBuf, 8, isOddLengthCh)

	// Test with even-length data (8 bytes)
	evenLengthData := []byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08}
	reader := bytes.NewReader(evenLengthData)
	err := splitter.Split(reader)
	require.NoError(t, err)

	// Check that channel still indicates even-length (or was not updated)
	select {
	case isOddLength := <-isOddLengthCh:
		assert.False(t, isOddLength, "Should not detect odd-length for even-length file")
	default:
		// Channel might not have been updated, which is fine for even-length
	}

	// Verify data integrity
	evenData := evenBuf.Bytes()
	oddData := oddBuf.Bytes()
	reconstructed, err := MergeBytes(evenData, oddData)
	require.NoError(t, err)
	assert.Equal(t, evenLengthData, reconstructed, "Reconstructed data doesn't match")
}
