package filelu

import (
	"fmt"
)

// parseStorageToBytes converts a storage string (e.g., "10") to bytes
func parseStorageToBytes(storage string) (int64, error) {
	var gb float64
	_, err := fmt.Sscanf(storage, "%f", &gb)
	if err != nil {
		return 0, fmt.Errorf("failed to parse storage: %w", err)
	}
	return int64(gb * 1024 * 1024 * 1024), nil
}
