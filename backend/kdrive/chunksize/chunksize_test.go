package chunksize

import (
	"testing"
)

func TestCalculateChunkSizeFromSize(t *testing.T) {
	tests := []struct {
		name     string
		fileSize int64
		expected int64
	}{
		{
			name:     "zero size",
			fileSize: 0,
			expected: ChunkSizeConfig.DefaultChunkSize, // 20MB
		},
		{
			name:     "small file",
			fileSize: 100 * 1024 * 1024,                // 100MB
			expected: ChunkSizeConfig.DefaultChunkSize, // 20MB
		},
		{
			name:     "medium file",
			fileSize: 500 * 1024 * 1024,                // 500MB
			expected: ChunkSizeConfig.DefaultChunkSize, // 20MB (500MB/20MB = 25 chunks)
		},
		{
			name:     "file exceeding limit",
			fileSize: ChunkSizeConfig.DefaultChunkSize*ChunkSizeConfig.MaxChunks + 1, // 200GB + 1 byte
			expected: ChunkSizeConfig.DefaultChunkSize + ChunkSizeConfig.mebi,        // 21MB (20MB + 1MiB rounded)
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CalculateChunkSize(tt.fileSize, ChunkSizeConfig.DefaultChunkSize)
			if result != tt.expected {
				t.Errorf("CalculateChunkSizeFromSize(%d) = %d, expected %d", tt.fileSize, result, tt.expected)
			}
		})
	}
}

func TestCalculateChunkSizeFromChunks(t *testing.T) {
	tests := []struct {
		name       string
		fileSize   int64
		chunkCount int64
		expected   int64
	}{
		{
			name:       "single chunk",
			fileSize:   100 * 1024 * 1024, // 100MB
			chunkCount: 1,
			expected:   100 * 1024 * 1024, // 100MB (no rounding needed, fits perfectly)
		},
		{
			name:       "five chunks",
			fileSize:   100 * 1024 * 1024, // 100MB
			chunkCount: 5,
			expected:   20971520, // 20MB (100MB / 5 = 20MB, rounded to MiB)
		},
		{
			name:       "at max chunks",
			fileSize:   1 * 1024 * 1024 * 1024 * 1024, // 1TB
			chunkCount: 10000,
			expected:   110100480, // ~105MB per chunk (1TB / 10000 = 100MB, rounded up to 105MiB due to fileSize % chunkCount logic)
		},
		{
			name:       "zero file size",
			fileSize:   0,
			chunkCount: 5,
			expected:   ChunkSizeConfig.DefaultChunkSize, // fallback to default
		},
		{
			name:       "zero chunks",
			fileSize:   100 * 1024 * 1024,
			chunkCount: 0,
			expected:   ChunkSizeConfig.DefaultChunkSize, // fallback to default
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CalculateChunkSizeFromChunks(tt.fileSize, tt.chunkCount)
			if result != tt.expected {
				t.Errorf("CalculateChunkSizeFromChunks(%d, %d) = %d, expected %d", tt.fileSize, tt.chunkCount, result, tt.expected)
			}
		})
	}
}
