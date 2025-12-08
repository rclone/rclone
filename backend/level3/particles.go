// Package level3 implements a backend that splits data across two remotes using byte-level striping
package level3

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/operations"
)

// SplitBytes splits data into even and odd indexed bytes
func SplitBytes(data []byte) (even []byte, odd []byte) {
	evenLen := (len(data) + 1) / 2
	oddLen := len(data) / 2

	even = make([]byte, evenLen)
	odd = make([]byte, oddLen)

	for i := 0; i < len(data); i++ {
		if i%2 == 0 {
			even[i/2] = data[i]
		} else {
			odd[i/2] = data[i]
		}
	}
	return even, odd
}

// MergeBytes merges even and odd indexed bytes back into original data
func MergeBytes(even []byte, odd []byte) ([]byte, error) {
	// Validate sizes: even should equal odd or be one byte larger
	if len(even) != len(odd) && len(even) != len(odd)+1 {
		return nil, fmt.Errorf("invalid particle sizes: even=%d, odd=%d (expected even=odd or even=odd+1)", len(even), len(odd))
	}

	result := make([]byte, len(even)+len(odd))
	for i := 0; i < len(result); i++ {
		if i%2 == 0 {
			result[i] = even[i/2]
		} else {
			result[i] = odd[i/2]
		}
	}
	return result, nil
}

// CalculateParity calculates XOR parity for even and odd particles
// For even-length data: parity[i] = even[i] XOR odd[i]
// For odd-length data: parity[i] = even[i] XOR odd[i], except last byte which is just even[last]
func CalculateParity(even []byte, odd []byte) []byte {
	parityLen := len(even) // Parity size always equals even size
	parity := make([]byte, parityLen)

	// XOR pairs of even and odd bytes
	for i := 0; i < len(odd); i++ {
		parity[i] = even[i] ^ odd[i]
	}

	// If odd length, last parity byte is just the last even byte (no XOR partner)
	if len(even) > len(odd) {
		parity[len(even)-1] = even[len(even)-1]
	}

	return parity
}

// ReconstructFromEvenAndParity reconstructs the original data from even + parity.
// If isOddLength is true, the last even byte equals the last parity byte.
func ReconstructFromEvenAndParity(even []byte, parity []byte, isOddLength bool) ([]byte, error) {
	if len(even) != len(parity) {
		return nil, fmt.Errorf("invalid sizes for reconstruction (even=%d parity=%d)", len(even), len(parity))
	}

	// Reconstruct odd bytes from parity ^ even
	odd := make([]byte, len(even))
	for i := 0; i < len(even); i++ {
		odd[i] = parity[i] ^ even[i]
	}
	// If original length was odd, odd has one fewer byte
	if isOddLength {
		odd = odd[:len(odd)-1]
	}

	return MergeBytes(even, odd)
}

// ReconstructFromOddAndParity reconstructs the original data from odd + parity.
// If isOddLength is true, the last even byte equals the last parity byte.
func ReconstructFromOddAndParity(odd []byte, parity []byte, isOddLength bool) ([]byte, error) {
	// Even length equals parity length. For odd original, even has one extra byte.
	even := make([]byte, len(parity))
	// Reconstruct pairs where odd exists
	for i := 0; i < len(odd); i++ {
		even[i] = parity[i] ^ odd[i]
	}
	// If original length was odd, last even byte equals last parity byte
	if isOddLength {
		even[len(even)-1] = parity[len(parity)-1]
	}

	return MergeBytes(even, odd)
}

// GetParityFilename returns the parity filename with appropriate suffix
func GetParityFilename(original string, isOddLength bool) string {
	if isOddLength {
		return original + ".parity-ol"
	}
	return original + ".parity-el"
}

// StripParitySuffix removes the parity suffix from a filename
func StripParitySuffix(filename string) (string, bool, bool) {
	if strings.HasSuffix(filename, ".parity-ol") {
		return strings.TrimSuffix(filename, ".parity-ol"), true, true
	}
	if strings.HasSuffix(filename, ".parity-el") {
		return strings.TrimSuffix(filename, ".parity-el"), true, false
	}
	return filename, false, false
}

// IsTempFile checks if a filename is a temporary file created during Update rollback
// Temporary files have suffixes like .tmp.even, .tmp.odd, .tmp.parity
func IsTempFile(filename string) bool {
	return strings.HasSuffix(filename, ".tmp.even") ||
		strings.HasSuffix(filename, ".tmp.odd") ||
		strings.HasSuffix(filename, ".tmp.parity")
}

// ValidateParticleSizes checks if particle sizes are valid
func ValidateParticleSizes(evenSize, oddSize int64) bool {
	return evenSize == oddSize || evenSize == oddSize+1
}

// particleInfo holds information about which particles exist for an object
type particleInfo struct {
	remote       string
	evenExists   bool
	oddExists    bool
	parityExists bool
	count        int
	size         int64 // Size of single particle (for broken objects)
}

// countParticles counts the number of particles on a backend
func (f *Fs) countParticles(ctx context.Context, backend fs.Fs) (int64, error) {
	var count int64
	err := operations.ListFn(ctx, backend, func(obj fs.Object) {
		count++
	})
	if err != nil && !errors.Is(err, fs.ErrorDirNotFound) {
		return 0, err
	}
	return count, nil
}

// reconstructParityParticle reconstructs a parity particle from even + odd
func (f *Fs) reconstructParityParticle(ctx context.Context, evenFs, oddFs fs.Fs, remote string) ([]byte, error) {
	// Read even particle
	evenObj, err := evenFs.NewObject(ctx, remote)
	if err != nil {
		return nil, fmt.Errorf("even particle not found: %w", err)
	}
	evenReader, err := evenObj.Open(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to open even particle: %w", err)
	}
	evenData, err := io.ReadAll(evenReader)
	evenReader.Close()
	if err != nil {
		return nil, fmt.Errorf("failed to read even particle: %w", err)
	}

	// Read odd particle
	oddObj, err := oddFs.NewObject(ctx, remote)
	if err != nil {
		return nil, fmt.Errorf("odd particle not found: %w", err)
	}
	oddReader, err := oddObj.Open(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to open odd particle: %w", err)
	}
	oddData, err := io.ReadAll(oddReader)
	oddReader.Close()
	if err != nil {
		return nil, fmt.Errorf("failed to read odd particle: %w", err)
	}

	// Calculate parity
	parityData := CalculateParity(evenData, oddData)
	return parityData, nil
}

// reconstructDataParticle reconstructs a data particle (even or odd) from the other data + parity
func (f *Fs) reconstructDataParticle(ctx context.Context, dataFs, parityFs fs.Fs, remote string, targetType string) ([]byte, error) {
	// For data particles, we need to read from parity backend with suffix
	// First, try to find the parity file
	parityOdd := GetParityFilename(remote, true)
	parityEven := GetParityFilename(remote, false)

	var parityObj fs.Object
	var isOddLength bool
	var err error

	parityObj, err = parityFs.NewObject(ctx, parityOdd)
	if err == nil {
		isOddLength = true
	} else {
		parityObj, err = parityFs.NewObject(ctx, parityEven)
		if err == nil {
			isOddLength = false
		} else {
			return nil, fmt.Errorf("parity particle not found (tried both suffixes): %w", err)
		}
	}

	// Read parity data
	parityReader, err := parityObj.Open(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to open parity particle: %w", err)
	}
	parityData, err := io.ReadAll(parityReader)
	parityReader.Close()
	if err != nil {
		return nil, fmt.Errorf("failed to read parity particle: %w", err)
	}

	// Read the available data particle
	dataObj, err := dataFs.NewObject(ctx, remote)
	if err != nil {
		return nil, fmt.Errorf("data particle not found: %w", err)
	}
	dataReader, err := dataObj.Open(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to open data particle: %w", err)
	}
	dataData, err := io.ReadAll(dataReader)
	dataReader.Close()
	if err != nil {
		return nil, fmt.Errorf("failed to read data particle: %w", err)
	}

	// Reconstruct missing particle
	if targetType == "even" {
		// Reconstruct even from odd + parity
		reconstructed, err := ReconstructFromOddAndParity(dataData, parityData, isOddLength)
		if err != nil {
			return nil, fmt.Errorf("failed to reconstruct even particle: %w", err)
		}
		evenData, _ := SplitBytes(reconstructed)
		return evenData, nil
	} else {
		// Reconstruct odd from even + parity
		reconstructed, err := ReconstructFromEvenAndParity(dataData, parityData, isOddLength)
		if err != nil {
			return nil, fmt.Errorf("failed to reconstruct odd particle: %w", err)
		}
		_, oddData := SplitBytes(reconstructed)
		return oddData, nil
	}
}

// countParticlesSync counts how many particles exist for an object (0-3)
// This is used by List() when auto_cleanup is enabled
func (f *Fs) countParticlesSync(ctx context.Context, remote string) int {
	type result struct {
		name   string
		exists bool
	}
	resultCh := make(chan result, 3)

	// Check even particle
	go func() {
		_, err := f.even.NewObject(ctx, remote)
		resultCh <- result{"even", err == nil}
	}()

	// Check odd particle
	go func() {
		_, err := f.odd.NewObject(ctx, remote)
		resultCh <- result{"odd", err == nil}
	}()

	// Check parity particle (both suffixes)
	go func() {
		_, errOL := f.parity.NewObject(ctx, GetParityFilename(remote, true))
		_, errEL := f.parity.NewObject(ctx, GetParityFilename(remote, false))
		resultCh <- result{"parity", errOL == nil || errEL == nil}
	}()

	// Collect results
	count := 0
	for i := 0; i < 3; i++ {
		res := <-resultCh
		if res.exists {
			count++
		}
	}

	return count
}

// particleInfoForObject inspects a single object and returns which particles exist.
func (f *Fs) particleInfoForObject(ctx context.Context, remote string) (particleInfo, error) {
	pi := particleInfo{remote: remote}

	// Check even
	if _, err := f.even.NewObject(ctx, remote); err == nil {
		pi.evenExists = true
	}

	// Check odd
	if _, err := f.odd.NewObject(ctx, remote); err == nil {
		pi.oddExists = true
	}

	// Check parity (both suffixes)
	if _, errOL := f.parity.NewObject(ctx, GetParityFilename(remote, true)); errOL == nil {
		pi.parityExists = true
	} else if _, errEL := f.parity.NewObject(ctx, GetParityFilename(remote, false)); errEL == nil {
		pi.parityExists = true
	}

	pi.count = 0
	if pi.evenExists {
		pi.count++
	}
	if pi.oddExists {
		pi.count++
	}
	if pi.parityExists {
		pi.count++
	}

	return pi, nil
}

// scanParticles scans a directory and returns particle information for all objects
// This is used by the Cleanup() command to identify broken objects
func (f *Fs) scanParticles(ctx context.Context, dir string) ([]particleInfo, error) {
	// Collect all entries from all backends (without filtering)
	entriesEven, _ := f.even.List(ctx, dir)
	entriesOdd, _ := f.odd.List(ctx, dir)
	entriesParity, _ := f.parity.List(ctx, dir)

	// Build map of all unique object paths
	objectMap := make(map[string]*particleInfo)

	// Process even particles
	for _, entry := range entriesEven {
		if _, ok := entry.(fs.Object); ok {
			remote := entry.Remote()
			if objectMap[remote] == nil {
				objectMap[remote] = &particleInfo{remote: remote}
			}
			objectMap[remote].evenExists = true
		}
	}

	// Process odd particles
	for _, entry := range entriesOdd {
		if _, ok := entry.(fs.Object); ok {
			remote := entry.Remote()
			if objectMap[remote] == nil {
				objectMap[remote] = &particleInfo{remote: remote}
			}
			objectMap[remote].oddExists = true
		}
	}

	// Process parity particles
	for _, entry := range entriesParity {
		if _, ok := entry.(fs.Object); ok {
			remote := entry.Remote()
			// Strip parity suffix to get base object name
			baseRemote, isParity, _ := StripParitySuffix(remote)
			if isParity {
				// Proper parity file with suffix
				if objectMap[baseRemote] == nil {
					objectMap[baseRemote] = &particleInfo{remote: baseRemote}
				}
				objectMap[baseRemote].parityExists = true
			} else {
				// File in parity remote without suffix (orphaned/manually created)
				// Still track it as it might be a broken object
				if objectMap[remote] == nil {
					objectMap[remote] = &particleInfo{remote: remote}
				}
				objectMap[remote].parityExists = true
			}
		}
	}

	// Calculate counts
	result := make([]particleInfo, 0, len(objectMap))
	for _, info := range objectMap {
		if info.evenExists {
			info.count++
		}
		if info.oddExists {
			info.count++
		}
		if info.parityExists {
			info.count++
		}
		result = append(result, *info)
	}

	return result, nil
}
