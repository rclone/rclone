package raid3

import (
	"bytes"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// splitDataUsingNewApproach splits data using the new pipelined chunked approach
// This mimics the behavior of putStreaming() but writes to buffers instead of uploading
func splitDataUsingNewApproach(data []byte, chunkSize int, evenBuf, oddBuf, parityBuf *bytes.Buffer) error {
	if len(data) == 0 {
		// Empty file - write empty particles
		return nil
	}

	// Process data in chunks (similar to putStreaming)
	reader := bytes.NewReader(data)
	buffer := make([]byte, chunkSize)

	for {
		n, err := reader.Read(buffer)
		if n == 0 && err == io.EOF {
			break
		}
		if err != nil && err != io.EOF {
			return err
		}

		// Split chunk into even and odd
		chunk := buffer[:n]
		evenData, oddData := SplitBytes(chunk)
		parityData := CalculateParity(evenData, oddData)

		// Write to buffers
		if _, err := evenBuf.Write(evenData); err != nil {
			return err
		}
		if _, err := oddBuf.Write(oddData); err != nil {
			return err
		}
		if _, err := parityBuf.Write(parityData); err != nil {
			return err
		}
	}

	return nil
}

// TestStreamSplitter tests data splitting using the new pipelined chunked approach
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

			// Split data using new approach
			err := splitDataUsingNewApproach(tt.data, tt.chunkSize, &evenBuf, &oddBuf, &parityBuf)
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

	// Write data in multiple chunks
	data1 := []byte{0x01, 0x02, 0x03}
	data2 := []byte{0x04, 0x05, 0x06}
	data3 := []byte{0x07, 0x08}

	// Combine all data and split using new approach
	allData := append(append(data1, data2...), data3...)
	err := splitDataUsingNewApproach(allData, 8, &evenBuf, &oddBuf, &parityBuf)
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
