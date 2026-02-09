// Package raid3 implements a backend that splits data across three remotes using byte-level striping
package raid3

// This file contains listing operations for the raid3 backend.
//
// It includes:
//   - List: List objects and directories in a directory
//   - ListR: Recursive listing of objects and directories
//   - NewObject: Create a new Object from a remote path

import (
	"context"
	"errors"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/walk"
	"golang.org/x/text/unicode/norm"
)

// List the objects and directories in dir into entries
func (f *Fs) List(ctx context.Context, dir string) (entries fs.DirEntries, err error) {
	// Input validation
	if err := validateContext(ctx, "list"); err != nil {
		return nil, err
	}
	// dir can be empty for root listing, so we only validate if non-empty
	if dir != "" {
		if err := validateRemote(dir, "list"); err != nil {
			return nil, err
		}
	}

	// Get entries from all three remotes concurrently
	type listResult struct {
		name    string
		entries fs.DirEntries
		err     error
	}
	listCh := make(chan listResult, 3)

	go func() {
		entries, err := f.even.List(ctx, dir)
		listCh <- listResult{"even", entries, err}
	}()

	go func() {
		entries, err := f.odd.List(ctx, dir)
		listCh <- listResult{"odd", entries, err}
	}()

	go func() {
		entries, err := f.parity.List(ctx, dir)
		listCh <- listResult{"parity", entries, err}
	}()

	// Collect results
	var entriesEven, entriesOdd, entriesParity fs.DirEntries
	var errEven, errOdd error
	for i := 0; i < 3; i++ {
		res := <-listCh
		switch res.name {
		case "even":
			entriesEven = res.entries
			errEven = res.err
		case "odd":
			entriesOdd = res.entries
			errOdd = res.err
		case "parity":
			entriesParity = res.entries
			// Ignore parity errors
		}
	}

	// If even fails, try odd
	if errEven != nil {
		if errOdd != nil {
			// Both data backends failed - check if this is an orphaned directory
			// that should be cleaned up before returning error
			if f.opt.AutoCleanup {
				// Check parity to see if directory is orphaned (exists only on parity)
				_, errParity := f.parity.List(ctx, dir)

				// If parity exists but both data backends don't, this is orphaned
				if errParity == nil {
					dirPath := dir
					if dirPath == "" {
						dirPath = f.root
					}
					fs.Infof(f, "Auto-cleanup: removing orphaned directory %q (exists on 1/3 backends - parity only)", dirPath)
					_ = f.parity.Rmdir(ctx, dir)
					return nil, nil // Return empty list, no error
				}
			}
			return nil, errEven // Return even error
		}
		// Continue with odd entries
	}

	// Create a map to track all entries (excluding parity files with suffixes)
	entryMap := make(map[string]fs.DirEntry)

	// Add even entries (filter out temporary files created during Update rollback)
	for _, entry := range entriesEven {
		remote := entry.Remote()
		// Skip temporary files created during Update rollback
		if IsTempFile(remote) {
			fs.Debugf(f, "List: Skipping temp file %s", remote)
			continue
		}
		entryMap[remote] = entry
	}

	// Add odd entries (merge with even, filter out temporary files)
	for _, entry := range entriesOdd {
		remote := entry.Remote()
		// Skip temporary files created during Update rollback
		if IsTempFile(remote) {
			fs.Debugf(f, "List: Skipping temp file %s", remote)
			continue
		}
		if _, exists := entryMap[remote]; !exists {
			entryMap[remote] = entry
		}
	}

	// Filter out parity files from parity backend (they have .parity-el or .parity-ol suffix)
	// Also filter out temporary files created during Update rollback
	// but include directories
	for _, entry := range entriesParity {
		remote := entry.Remote()
		// Strip parity suffix if it's a parity file
		_, isParity, _ := StripParitySuffix(remote)
		if isParity {
			// Don't add parity files to the list
			continue
		}
		// Skip temporary files created during Update rollback
		if IsTempFile(remote) {
			fs.Debugf(f, "List: Skipping temp file %s", remote)
			continue
		}
		// Add non-parity entries (directories mainly)
		if _, exists := entryMap[remote]; !exists {
			entryMap[remote] = entry
		}
	}

	// Convert map back to slice
	entries = make(fs.DirEntries, 0, len(entryMap))
	for _, entry := range entryMap {
		switch e := entry.(type) {
		case fs.Object:
			// If auto_cleanup is enabled, handle broken objects (< 2 particles)
			if f.opt.AutoCleanup {
				particleCount := f.countParticlesSync(ctx, e.Remote())
				if particleCount < 2 {
					// Check if all backends are available for auto-delete
					if err := f.checkAllBackendsAvailable(ctx); err == nil {
						// All backends available - auto-delete broken object
						particleInfo, err := f.particleInfoForObject(ctx, e.Remote())
						if err == nil {
							if delErr := f.removeBrokenObject(ctx, particleInfo); delErr != nil {
								fs.Debugf(f, "List: Failed to auto-delete broken object %s: %v", e.Remote(), delErr)
							} else {
								fs.Debugf(f, "List: Auto-deleted broken object %s", e.Remote())
							}
						}
					} else {
						// Not all backends available - hide broken object (don't delete)
						fs.Debugf(f, "List: Hiding broken object %s (only %d particle, backends unavailable)", e.Remote(), particleCount)
					}
					// Hide broken object (whether deleted or not)
					continue
				}
			}
			entries = append(entries, &Object{
				fs:     f,
				remote: e.Remote(),
			})
		case fs.Directory:
			entries = append(entries, &Directory{
				fs:     f,
				remote: e.Remote(),
			})
		}
	}

	// If auto_cleanup is enabled and the directory is empty, check for orphaned buckets/directories
	// Orphaned buckets exist on < 2 backends and should be removed
	if f.opt.AutoCleanup && len(entries) == 0 {
		f.cleanupOrphanedDirectory(ctx, dir, errEven, errOdd)
	}

	// Reconstruct missing directories (1dm case: directory exists on 2/3 backends)
	// This implements heal for degraded directory state
	if f.opt.AutoHeal {
		f.reconstructMissingDirectory(ctx, dir, errEven, errOdd)
	}

	return entries, nil
}

// ListR lists the objects and directories of the Fs starting
// from dir recursively into out.
//
// dir should be "" to start from the root, and should not
// have trailing slashes.
//
// This should return ErrDirNotFound if the directory isn't
// found.
//
// It should call callback for each tranche of entries read.
// These need not be returned in any particular order.  If
// callback returns an error then the listing will stop
// immediately.
//
// Don't implement this unless you have a more efficient way
// of listing recursively that doing a directory traversal.
func (f *Fs) ListR(ctx context.Context, dir string, callback fs.ListRCallback) (err error) {
	// Collect entries from all three backends in parallel
	// Support degraded mode: works with 2/3 backends (reads work in degraded mode)
	type listRResult struct {
		name    string
		entries []fs.DirEntries
		err     error
	}
	results := make(chan listRResult, 3)

	// ListR on even backend
	go func() {
		var evenEntries []fs.DirEntries
		innerCallback := func(entries fs.DirEntries) error {
			// Make a copy of entries to avoid modification after callback returns
			entriesCopy := make(fs.DirEntries, len(entries))
			copy(entriesCopy, entries)
			evenEntries = append(evenEntries, entriesCopy)
			return nil
		}
		do := f.even.Features().ListR
		var err error
		if do != nil {
			err = do(ctx, dir, innerCallback)
		} else {
			// Fallback to walk.ListR if backend doesn't support ListR
			err = walk.ListR(ctx, f.even, dir, true, -1, walk.ListAll, innerCallback)
		}
		results <- listRResult{"even", evenEntries, err}
	}()

	// ListR on odd backend
	go func() {
		var oddEntries []fs.DirEntries
		innerCallback := func(entries fs.DirEntries) error {
			// Make a copy of entries to avoid modification after callback returns
			entriesCopy := make(fs.DirEntries, len(entries))
			copy(entriesCopy, entries)
			oddEntries = append(oddEntries, entriesCopy)
			return nil
		}
		do := f.odd.Features().ListR
		var err error
		if do != nil {
			err = do(ctx, dir, innerCallback)
		} else {
			// Fallback to walk.ListR if backend doesn't support ListR
			err = walk.ListR(ctx, f.odd, dir, true, -1, walk.ListAll, innerCallback)
		}
		results <- listRResult{"odd", oddEntries, err}
	}()

	// ListR on parity backend (errors ignored, similar to List())
	go func() {
		var parityEntries []fs.DirEntries
		innerCallback := func(entries fs.DirEntries) error {
			// Make a copy of entries to avoid modification after callback returns
			entriesCopy := make(fs.DirEntries, len(entries))
			copy(entriesCopy, entries)
			parityEntries = append(parityEntries, entriesCopy)
			return nil
		}
		do := f.parity.Features().ListR
		if do != nil {
			_ = do(ctx, dir, innerCallback) // Ignore errors (similar to List() behavior)
		} else {
			// Fallback to walk.ListR if backend doesn't support ListR
			_ = walk.ListR(ctx, f.parity, dir, true, -1, walk.ListAll, innerCallback) // Ignore errors
		}
		// Ignore parity errors (similar to List() behavior)
		results <- listRResult{"parity", parityEntries, nil}
	}()

	// Collect results
	var entriesEven, entriesOdd, entriesParity []fs.DirEntries
	var errEven, errOdd error
	for i := 0; i < 3; i++ {
		res := <-results
		switch res.name {
		case "even":
			entriesEven = res.entries
			errEven = res.err
		case "odd":
			entriesOdd = res.entries
			errOdd = res.err
		case "parity":
			entriesParity = res.entries
			// Ignore parity errors (already set to nil above)
		}
	}

	// Degraded mode support: if even fails, try odd (similar to List())
	// Only fail if both data backends fail
	// If one data backend succeeds, we can list successfully (degraded mode)
	if errEven != nil {
		if errOdd != nil {
			// Both data backends failed
			// Check if both failed with ErrorDirNotFound
			if errors.Is(errEven, fs.ErrorDirNotFound) && errors.Is(errOdd, fs.ErrorDirNotFound) {
				return fs.ErrorDirNotFound
			}
			// Return even error (prefer even over odd)
			return errEven
		}
		// Even failed but odd succeeded - this is degraded mode
		// Continue with odd entries only - listing can succeed with 2/3 backends
		// Don't return error - degraded mode is acceptable for reads
	}

	// Merge entries similar to List() method
	// Create a map to track all entries (excluding parity files with suffixes)
	// Use NFD-normalized remote path as key so that paths that differ only by
	// Unicode normalization (e.g. NFC vs NFD on macOS) are deduplicated (fixes Q24).
	// NFD is used because macOS local backend often returns NFD; NFC and NFD
	// both normalize to the same NFD string, giving one key per logical path.
	entryMap := make(map[string]fs.DirEntry)

	// Helper function to add entry to map with deduplication
	addEntry := func(entry fs.DirEntry) {
		remote := entry.Remote()
		// Skip temporary files created during Update rollback
		if IsTempFile(remote) {
			return
		}
		key := norm.NFD.String(remote)
		// Only add if not already present (deduplication by normalized path)
		if _, exists := entryMap[key]; !exists {
			entryMap[key] = entry
		}
	}

	// Process even entries
	for _, entryBatch := range entriesEven {
		for _, entry := range entryBatch {
			addEntry(entry)
		}
	}

	// Process odd entries (merge with even)
	for _, entryBatch := range entriesOdd {
		for _, entry := range entryBatch {
			addEntry(entry)
		}
	}

	// Process parity entries (filter out parity files, but include directories)
	for _, entryBatch := range entriesParity {
		for _, entry := range entryBatch {
			remote := entry.Remote()

			// Filter out parity files (they have .parity-el or .parity-ol suffix)
			_, isParity, _ := StripParitySuffix(remote)
			if isParity {
				continue
			}

			// Add non-parity entries (directories mainly)
			addEntry(entry)
		}
	}

	// Convert map to slice and convert entries to raid3 Object/Directory types.
	// Deduplicate again by NFC path so that any remaining variants (e.g. different
	// NFD forms that stayed distinct in entryMap) collapse to one entry per path (Q24).
	mergedEntries := make(fs.DirEntries, 0, len(entryMap))
	objectCount := 0
	dirCount := 0
	seenByNFC := make(map[string]bool, len(entryMap))
	emitPath := norm.NFC.String
	for remote, entry := range entryMap {
		outRemote := emitPath(remote)
		if seenByNFC[outRemote] {
			continue
		}
		seenByNFC[outRemote] = true
		var converted fs.DirEntry
		switch entry.(type) {
		case fs.Object:
			converted = &Object{
				fs:     f,
				remote: outRemote,
			}
			objectCount++
		case fs.Directory:
			converted = &Directory{
				fs:     f,
				remote: outRemote,
			}
			dirCount++
		default:
			// Unknown type, skip
			continue
		}
		mergedEntries = append(mergedEntries, converted)
	}
	fs.Infof(f, "ListR(%q): converted %d objects, %d dirs, final count: %d", dir, objectCount, dirCount, len(mergedEntries))

	// Call callback with merged entries
	// If callback returns an error, return it
	// Otherwise return nil (success) even if one backend failed (degraded mode)
	fs.Infof(f, "ListR(%q): calling callback with %d merged entries", dir, len(mergedEntries))
	if err := callback(mergedEntries); err != nil {
		fs.Infof(f, "ListR(%q): callback returned error: %v", dir, err)
		return err
	}
	fs.Infof(f, "ListR(%q): callback succeeded", dir)

	// Success - return nil even if one backend failed (degraded mode is acceptable for reads)
	// Hardware RAID 3 behavior: reads succeed with warnings in degraded mode
	// Degraded mode succeeds if:
	// - Both data backends succeed (parity can fail) - RAID 3 only needs 2/3 for reads
	// - One data backend succeeds (other data backend can fail) - still have 2/3
	// Errors from failed backends are logged as warnings, but the operation succeeds
	return nil
}

// NewObject creates a new remote Object
func (f *Fs) NewObject(ctx context.Context, remote string) (fs.Object, error) {
	// Input validation
	if err := validateContext(ctx, "newobject"); err != nil {
		return nil, err
	}
	if err := validateRemote(remote, "newobject"); err != nil {
		return nil, err
	}

	// Probe particles - must have at least 2 of 3 for RAID 3
	_, errEven := f.even.NewObject(ctx, remote)
	_, errOdd := f.odd.NewObject(ctx, remote)
	// Try both parity suffixes to detect presence
	_, errParityOL := f.parity.NewObject(ctx, GetParityFilename(remote, true))
	_, errParityEL := f.parity.NewObject(ctx, GetParityFilename(remote, false))

	parityPresent := errParityOL == nil || errParityEL == nil
	evenPresent := errEven == nil
	oddPresent := errOdd == nil

	// Allow object if any two of the three are present
	presentCount := 0
	if evenPresent {
		presentCount++
	}
	if oddPresent {
		presentCount++
	}
	if parityPresent {
		presentCount++
	}
	if presentCount < 2 {
		// File doesn't exist (less than 2/3 particles present)
		// If one backend is unavailable (connection error), but file doesn't exist on available backends,
		// return ObjectNotFound to allow move to proceed (Move() will check backend availability and handle rollback)

		// Check if we have connection errors (backend unavailable) vs ObjectNotFound (file doesn't exist)
		hasConnectionError := false

		// Check if any backend returned a connection error (not ObjectNotFound)
		if errEven != nil && !errors.Is(errEven, fs.ErrorObjectNotFound) {
			hasConnectionError = true
		}
		if errOdd != nil && !errors.Is(errOdd, fs.ErrorObjectNotFound) {
			hasConnectionError = true
		}
		// Parity is present if either check succeeds, so if presentCount < 2, both checks failed
		if !parityPresent && errParityOL != nil && errParityEL != nil {
			if !errors.Is(errParityOL, fs.ErrorObjectNotFound) || !errors.Is(errParityEL, fs.ErrorObjectNotFound) {
				hasConnectionError = true
			}
		}

		// If we have connection errors but file doesn't exist on available backends,
		// return ObjectNotFound to allow move to proceed (Move() will check backend availability)
		if hasConnectionError {
			return nil, fs.ErrorObjectNotFound
		}

		// All backends returned ObjectNotFound or file is truly missing
		// Prefer returning the first error if available, otherwise ObjectNotFound
		if errEven != nil {
			return nil, errEven
		}
		if errOdd != nil {
			return nil, errOdd
		}
		return nil, fs.ErrorObjectNotFound
	}

	return &Object{fs: f, remote: remote}, nil
}
