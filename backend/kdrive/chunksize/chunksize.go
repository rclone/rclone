package chunksize

import "math"

type SizeConstants struct {
	MaxChunkSize     int64
	DefaultChunkSize int64
	MaxChunks        int64
	mebi             int64
}

var ChunkSizeConfig = SizeConstants{
	MaxChunkSize:     1 * 1000 * 1000 * 1000, // 1 Go (max API limit)
	DefaultChunkSize: 20 * 1024 * 1024,       // 20 MiB (default chunk size)
	MaxChunks:        10000,                  // 10000 (max chunks allowed by API)
	mebi:             1024 * 1024,            // 1 MiB (rounding unit)
}

func CalculateChunkSize(fileSize int64, preferredChunkSize int64) int64 {
	// Use preferred chunk size
	chunkSize := preferredChunkSize
	if chunkSize <= 0 {
		chunkSize = ChunkSizeConfig.DefaultChunkSize
	}

	// Round to greater MiB
	if chunkSize%ChunkSizeConfig.mebi != 0 {
		chunkSize += ChunkSizeConfig.mebi - (chunkSize % ChunkSizeConfig.mebi)
	}

	// Limit chunk size to 1 Go
	if chunkSize > ChunkSizeConfig.MaxChunkSize {
		chunkSize = ChunkSizeConfig.MaxChunkSize
	}

	// For large files, use a bigger chunk size
	requiredChunks := CalculateTotalChunks(fileSize, chunkSize)
	if requiredChunks > ChunkSizeConfig.MaxChunks {
		chunkSize = fileSize / ChunkSizeConfig.MaxChunks
		if fileSize%ChunkSizeConfig.MaxChunks != 0 {
			chunkSize++
		}

		// Round to greater MiB
		if chunkSize%ChunkSizeConfig.mebi != 0 {
			chunkSize += ChunkSizeConfig.mebi - (chunkSize % ChunkSizeConfig.mebi)
		}

		// Limit chunk size to 1 Go
		if chunkSize > ChunkSizeConfig.MaxChunkSize {
			chunkSize = ChunkSizeConfig.MaxChunkSize
		}
	}

	return chunkSize
}

// CalculateChunkSizeFromChunks calculates the chunk size from file size and chunk count.
// This is useful when you know the chunk count from the API but not the exact chunk size used.
//
// It returns the chunk size that would produce exactly chunkCount chunks for the given fileSize.
func CalculateChunkSizeFromChunks(fileSize int64, chunkCount int64) int64 {
	if fileSize <= 0 || chunkCount <= 0 {
		return ChunkSizeConfig.DefaultChunkSize
	}

	// Calculate the minimum chunk size needed to get chunkCount chunks
	chunkSize := fileSize / chunkCount
	if fileSize%chunkCount != 0 {
		chunkSize++
	}

	// Round to nearest MiB
	if chunkSize%ChunkSizeConfig.mebi != 0 {
		chunkSize += ChunkSizeConfig.mebi - (chunkSize % ChunkSizeConfig.mebi)
	}

	// Cap at MaxChunkSize
	if chunkSize > ChunkSizeConfig.MaxChunkSize {
		chunkSize = ChunkSizeConfig.MaxChunkSize
	}

	return chunkSize
}

func CalculateTotalChunks(fileSize int64, chunkSize int64) int64 {
	totalChunks := math.Ceil(float64(fileSize) / float64(chunkSize))

	return int64(totalChunks)
}
