// Package raid3 implements a backend that splits data across three remotes using byte-level striping
package raid3

// This file contains the main Fs implementation, registration, and core filesystem operations.
//
// It includes:
//   - Backend registration and configuration options (init, Options struct)
//   - Fs struct and NewFs constructor with degraded mode support
//   - Core filesystem operations: Mkdir, Rmdir, Purge, CleanUp, About, Shutdown
//   - Heal infrastructure initialization (upload queue, background workers)
//   - Cleanup operations for broken objects (CleanUp, findBrokenObjects)
//   - Directory reconstruction and orphan cleanup (reconstructMissingDirectory, cleanupOrphanedDirectory)
//
// Other operations are split into separate files:
//   - list.go: List, ListR, NewObject
//   - operations.go: Put, Move, Copy, DirMove
//   - health.go: checkAllBackendsAvailable, formatDegradedModeError, checkDirectoryExists
//   - metadata.go: MkdirMetadata, DirSetModTime

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path"
	"strings"
	"sync"
	"time"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/cache"
	"github.com/rclone/rclone/fs/config/configmap"
	"github.com/rclone/rclone/fs/config/configstruct"
	"github.com/rclone/rclone/fs/fspath"
	"github.com/rclone/rclone/fs/hash"
	"github.com/rclone/rclone/fs/operations"
	"golang.org/x/sync/errgroup"
)

// Register with Fs
func init() {
	fsi := &fs.RegInfo{
		Name:        "raid3",
		Description: "RAID 3 storage with byte-level data striping across three remotes",
		NewFs:       NewFs,
		MetadataInfo: &fs.MetadataInfo{
			Help: `Any metadata supported by the underlying remotes is read and written.`,
		},
		Options: []fs.Option{{
			Name: "even",
			Help: `Remote for even-indexed bytes (indices 0, 2, 4, ...).

This should be in the form 'remote:path'.`,
			Required: true,
		}, {
			Name: "odd",
			Help: `Remote for odd-indexed bytes (indices 1, 3, 5, ...).

This should be in the form 'remote:path'.`,
			Required: true,
		}, {
			Name: "parity",
			Help: `Remote for parity data (XOR of even and odd bytes).

This should be in the form 'remote:path'.`,
			Required: true,
		}, {
			Name:    "timeout_mode",
			Help:    "Timeout behavior for backend operations",
			Default: "standard",
			Examples: []fs.OptionExample{
				{
					Value: "standard",
					Help:  "Use global timeout settings (best for local/file storage)",
				},
				{
					Value: "balanced",
					Help:  "Moderate timeouts (3 retries, 30s) - good for reliable S3",
				},
				{
					Value: "aggressive",
					Help:  "Fast failover (1 retry, 10s) - best for S3 degraded mode",
				},
			},
		}, {
			Name:     "auto_cleanup",
			Help:     "Automatically hide broken objects (only 1 particle) from listings",
			Default:  true,
			Advanced: false,
		}, {
			Name:     "auto_heal",
			Help:     "Automatically reconstruct missing particles/directories (2/3 present)",
			Default:  true,
			Advanced: false,
		}, {
			Name:     "rollback",
			Help:     "Automatically rollback successful operations if any particle operation fails (all-or-nothing guarantee)",
			Default:  true,
			Advanced: false,
		}, {
			Name:     "use_streaming",
			Help:     "Use streaming processing path. When enabled, processes files in chunks instead of loading entire file into memory.",
			Default:  true,
			Advanced: true,
		}, {
			Name:     "chunk_size",
			Help:     "Chunk size for streaming operations",
			Default:  fs.SizeSuffix(defaultChunkSize),
			Advanced: true,
		}},
		CommandHelp: commandHelp,
	}
	fs.Register(fsi)
}

// commandHelp defines the backend-specific commands
var commandHelp = []fs.CommandHelp{{
	Name:  "status",
	Short: "Show backend health and rebuild guide",
	Long: `Shows the health status of all three backends and provides step-by-step
rebuild guidance if any backend is unavailable.

This is the primary diagnostic tool for raid3 - run this first when you
encounter errors or want to check backend health.

Usage:

    rclone backend status raid3:

Output includes:
  • Health status of all three backends (even, odd, parity)
  • Impact assessment (what operations work)
  • Complete rebuild guide for degraded mode
  • Step-by-step instructions for backend replacement

This command is mentioned in error messages when writes fail in degraded mode.
`,
}, {
	Name:  "rebuild",
	Short: "Rebuild missing particles on a replacement backend",
	Long: `Rebuilds all missing particles on a backend after replacement.

Use this after replacing a failed backend with a new, empty backend. The rebuild
process reconstructs all missing particles using the other two backends and parity
information, restoring the raid3 backend to a fully healthy state.

Usage:

    rclone backend rebuild raid3: [even|odd|parity]
    
Auto-detects which backend needs rebuild if not specified:

    rclone backend rebuild raid3:

Options:

  -o check-only=true    Analyze what needs rebuild without actually rebuilding
  -o dry-run=true       Show what would be done without making changes
  -o priority=MODE      Rebuild order (auto, dirs-small, dirs, small)

Priority modes:
  auto        Smart default based on dataset (recommended)
  dirs-small  All directories first, then files by size (smallest first)
  dirs        Directories first, then files alphabetically per directory
  small       Create directories as-needed, files by size (smallest first)

Examples:

    # Check what needs rebuild
    rclone backend rebuild raid3: -o check-only=true
    
    # Rebuild with auto-detected backend
    rclone backend rebuild raid3:
    
    # Rebuild specific backend
    rclone backend rebuild raid3: odd
    
    # Rebuild with small files first
    rclone backend rebuild raid3: odd -o priority=small

The rebuild process will:
  1. Scan for missing particles
  2. Reconstruct data from other two backends
  3. Upload restored particles
  4. Show progress and ETA
  5. Verify integrity

Note: This is different from heal which happens automatically during
reads. Rebuild is a manual, complete restoration after backend replacement.
`,
}, {
	Name:  "heal",
	Short: "Heal all degraded objects (2/3 particles present)",
	Long: `Scans the entire remote and heals any objects that have exactly 2 of 3 particles.

This is an explicit, admin-driven alternative to automatic heal on read.
Use this when you want to proactively heal all degraded objects rather than
waiting for them to be accessed during normal operations.

Usage:

    rclone backend heal raid3: [file_path]

Options:

  -o dry-run=true    Show what would be healed without making changes

The heal command will:
  1. Scan all objects in the remote
  2. Identify objects with exactly 2 of 3 particles (degraded state)
  3. Reconstruct and upload the missing particle
  4. Report summary of healed objects

Output includes:
  • Total files scanned
  • Number of healthy files (3/3 particles)
  • Number of healed files (2/3→3/3)
  • Number of unrebuildable files (≤1 particle)
  • List of unrebuildable objects (if any)

Examples:

    # Heal all degraded objects
    rclone backend heal raid3:

    # Heal a specific file
    rclone backend heal raid3: path/to/file.txt

    # Dry-run: See what would be healed without making changes
    rclone backend heal raid3: -o dry-run=true

    # Example output:
    # Heal Summary
    # ══════════════════════════════════════════
    # 
    # Files scanned:      100
    # Healthy (3/3):       85
    # Healed (2/3→3/3):   12
    # Unrebuildable (≤1): 3

Note: This is different from auto_heal which heals objects automatically
during reads. The heal command proactively heals all degraded objects at once,
which is useful for:
  • Periodic maintenance
  • After rebuilding from backend failures
  • Before important operations
  • When you want to ensure all objects are fully healthy

The heal command works regardless of the auto_heal setting - it's always
available as an explicit admin command.
`,
}}

// Options defines the configuration for this backend
type Options struct {
	Even         string        `config:"even"`
	Odd          string        `config:"odd"`
	Parity       string        `config:"parity"`
	TimeoutMode  string        `config:"timeout_mode"`
	AutoCleanup  bool          `config:"auto_cleanup"`
	AutoHeal     bool          `config:"auto_heal"`
	Rollback     bool          `config:"rollback"`
	UseStreaming bool          `config:"use_streaming"`
	ChunkSize    fs.SizeSuffix `config:"chunk_size"`
}

// Fs represents a raid3 backend with striped storage and parity
type Fs struct {
	name     string       // name of this remote
	root     string       // the path we are working on
	opt      Options      // options for this Fs
	features *fs.Features // optional features
	even     fs.Fs        // remote for even-indexed bytes
	odd      fs.Fs        // remote for odd-indexed bytes
	parity   fs.Fs        // remote for parity data
	hashSet  hash.Set     // intersection of hash types

	// Self-healing infrastructure
	uploadQueue   *uploadQueue
	uploadWg      sync.WaitGroup
	uploadCtx     context.Context
	uploadCancel  context.CancelFunc
	uploadWorkers int
}

// NewFs constructs an Fs from the path.
//
// The returned Fs is the actual Fs, referenced by remote in the config
func NewFs(ctx context.Context, name, root string, m configmap.Mapper) (outFs fs.Fs, err error) {
	// Parse config into Options struct
	opt := new(Options)
	err = configstruct.Set(m, opt)
	if err != nil {
		return nil, err
	}

	// Apply defaults if not explicitly set
	// Both auto_cleanup and auto_heal default to true for best user experience.
	if _, ok := m.Get("auto_cleanup"); !ok {
		opt.AutoCleanup = true
	}
	if _, ok := m.Get("auto_heal"); !ok {
		opt.AutoHeal = true
	}
	if _, ok := m.Get("rollback"); !ok {
		opt.Rollback = true
	}
	if _, ok := m.Get("use_streaming"); !ok {
		opt.UseStreaming = true
	}
	if _, ok := m.Get("chunk_size"); !ok {
		opt.ChunkSize = fs.SizeSuffix(defaultChunkSize)
	}

	if opt.Even == "" {
		return nil, errors.New("even must be set")
	}
	if opt.Odd == "" {
		return nil, errors.New("odd must be set")
	}
	if opt.Parity == "" {
		return nil, errors.New("parity must be set")
	}
	if strings.HasPrefix(opt.Even, name+":") {
		return nil, errors.New("can't point raid3 remote at itself - check the value of the even setting")
	}
	if strings.HasPrefix(opt.Odd, name+":") {
		return nil, errors.New("can't point raid3 remote at itself - check the value of the odd setting")
	}
	if strings.HasPrefix(opt.Parity, name+":") {
		return nil, errors.New("can't point raid3 remote at itself - check the value of the parity setting")
	}

	// Trim trailing slashes
	for strings.HasSuffix(root, "/") {
		root = root[:len(root)-1]
	}

	// Apply timeout mode to context
	ctx = applyTimeoutMode(ctx, opt.TimeoutMode)

	f := &Fs{
		name: name,
		root: root,
		opt:  *opt,
	}

	var evenErr, oddErr, parityErr error

	// Create remotes concurrently to avoid blocking on unavailable backends
	// Use a timeout context to avoid waiting forever for unavailable remotes
	// Adjust timeout based on timeout_mode
	var initTimeout time.Duration
	switch opt.TimeoutMode {
	case "aggressive":
		initTimeout = aggressiveInitTimeout
	case "balanced":
		initTimeout = balancedInitTimeout
	case "standard", "":
		initTimeout = standardInitTimeout
	default:
		initTimeout = aggressiveInitTimeout
	}
	initCtx, cancel := context.WithTimeout(ctx, initTimeout)
	defer cancel()

	type fsResult struct {
		name string
		fs   fs.Fs
		err  error
	}
	fsCh := make(chan fsResult, 3)

	// Create even remote (even-indexed bytes)
	go func() {
		evenPath := fspath.JoinRootPath(opt.Even, root)
		fs, err := cache.Get(initCtx, evenPath)
		fsCh <- fsResult{"even", fs, err}
	}()

	// Create odd remote (odd-indexed bytes)
	go func() {
		oddPath := fspath.JoinRootPath(opt.Odd, root)
		fs, err := cache.Get(initCtx, oddPath)
		fsCh <- fsResult{"odd", fs, err}
	}()

	// Create parity remote
	go func() {
		parityPath := fspath.JoinRootPath(opt.Parity, root)
		fs, err := cache.Get(initCtx, parityPath)
		fsCh <- fsResult{"parity", fs, err}
	}()

	// Collect results - wait for all 3 with timeout
	// In RAID 3, we can tolerate one backend being unavailable
	successCount := 0
	for i := 0; i < 3; i++ {
		select {
		case res := <-fsCh:
			switch res.name {
			case "even":
				f.even = res.fs
				evenErr = res.err
				if evenErr == nil || evenErr == fs.ErrorIsFile {
					successCount++
				} else {
					fs.Logf(f, "Warning: even remote unavailable: %v", evenErr)
				}
			case "odd":
				f.odd = res.fs
				oddErr = res.err
				if oddErr == nil || oddErr == fs.ErrorIsFile {
					successCount++
				} else {
					fs.Logf(f, "Warning: odd remote unavailable: %v", oddErr)
				}
			case "parity":
				f.parity = res.fs
				parityErr = res.err
				if parityErr == nil || parityErr == fs.ErrorIsFile {
					successCount++
				} else {
					fs.Logf(f, "Warning: parity remote unavailable: %v", parityErr)
				}
			}
		case <-initCtx.Done():
			// Context cancelled/timeout
			fs.Logf(f, "Timeout waiting for remotes, have %d/%d available", successCount, 3)
			if successCount < 2 {
				return nil, fmt.Errorf("insufficient remotes available (%d/3): %w", successCount, initCtx.Err())
			}
			// Have at least 2, can proceed in degraded mode
			goto checkRemotes
		}
	}

checkRemotes:
	// Must have at least 2 of 3 remotes for RAID 3 to function
	if successCount < 2 {
		return nil, fmt.Errorf("need at least 2 of 3 remotes available, only have %d", successCount)
	}

	// If ErrorIsFile returned, the path points to a file.
	// We need to adjust the root to point to the parent directory.
	var returnErrorIsFile bool
	if evenErr == fs.ErrorIsFile || oddErr == fs.ErrorIsFile {
		returnErrorIsFile = true
		adjustedRoot := path.Dir(f.root)
		if adjustedRoot == "." || adjustedRoot == "/" {
			adjustedRoot = ""
		}

		// Recreate upstreams with adjusted root concurrently
		initCtx2, cancel2 := context.WithTimeout(ctx, errorIsFileRetryTimeout)
		defer cancel2()
		fsCh2 := make(chan fsResult, 3)

		go func() {
			evenPath := fspath.JoinRootPath(opt.Even, adjustedRoot)
			fs, err := cache.Get(initCtx2, evenPath)
			fsCh2 <- fsResult{"even", fs, err}
		}()

		go func() {
			oddPath := fspath.JoinRootPath(opt.Odd, adjustedRoot)
			fs, err := cache.Get(initCtx2, oddPath)
			fsCh2 <- fsResult{"odd", fs, err}
		}()

		go func() {
			parityPath := fspath.JoinRootPath(opt.Parity, adjustedRoot)
			fs, err := cache.Get(initCtx2, parityPath)
			fsCh2 <- fsResult{"parity", fs, err}
		}()

		// Collect adjusted results
		for i := 0; i < 3; i++ {
			res := <-fsCh2
			switch res.name {
			case "even":
				f.even = res.fs
				evenErr = res.err
				if evenErr != nil && evenErr != fs.ErrorIsFile {
					return nil, fmt.Errorf("failed to create even remote: %w", evenErr)
				}
			case "odd":
				f.odd = res.fs
				oddErr = res.err
				if oddErr != nil && oddErr != fs.ErrorIsFile {
					return nil, fmt.Errorf("failed to create odd remote: %w", oddErr)
				}
			case "parity":
				f.parity = res.fs
				parityErr = res.err
				if parityErr != nil && parityErr != fs.ErrorIsFile {
					return nil, fmt.Errorf("failed to create parity remote: %w", parityErr)
				}
			}
		}

		// Update root to adjusted value
		f.root = adjustedRoot
	}

	// Get the intersection of hash types
	f.hashSet = f.even.Hashes().Overlap(f.odd.Hashes()).Overlap(f.parity.Hashes())

	f.features = (&fs.Features{
		CaseInsensitive:         f.even.Features().CaseInsensitive || f.odd.Features().CaseInsensitive || f.parity.Features().CaseInsensitive,
		DuplicateFiles:          false,
		ReadMimeType:            f.even.Features().ReadMimeType && f.odd.Features().ReadMimeType,
		WriteMimeType:           f.even.Features().WriteMimeType && f.odd.Features().WriteMimeType,
		CanHaveEmptyDirectories: f.even.Features().CanHaveEmptyDirectories && f.odd.Features().CanHaveEmptyDirectories && f.parity.Features().CanHaveEmptyDirectories,
		BucketBased:             f.even.Features().BucketBased && f.odd.Features().BucketBased && f.parity.Features().BucketBased,
		About:                   f.About,
	}).Fill(ctx, f)

	// Propagate SlowHash if any backend has slow hash
	// This helps rclone optimize operations when hashing is slow
	// Safe in degraded mode: if a backend is nil/unavailable, we skip it (assume false)
	slowHash := false
	if f.even != nil && f.even.Features().SlowHash {
		slowHash = true
	}
	if f.odd != nil && f.odd.Features().SlowHash {
		slowHash = true
	}
	if f.parity != nil && f.parity.Features().SlowHash {
		slowHash = true
	}
	f.features.SlowHash = slowHash

	// Indicate that this backend wraps other backends
	// This helps rclone understand the backend architecture
	f.features.Overlay = true

	// Enable Move if all backends support Move or Copy (like union/combine backends)
	// This allows raid3 to work with backends like S3/MinIO that support Copy but not Move
	if operations.CanServerSideMove(f.even) && operations.CanServerSideMove(f.odd) && operations.CanServerSideMove(f.parity) {
		f.features.Move = f.Move
	}

	// Enable Copy if all backends support Copy
	if f.even.Features().Copy != nil && f.odd.Features().Copy != nil && f.parity.Features().Copy != nil {
		f.features.Copy = f.Copy
	}

	// Enable DirMove if all backends support it
	if f.even.Features().DirMove != nil && f.odd.Features().DirMove != nil && f.parity.Features().DirMove != nil {
		f.features.DirMove = f.DirMove
	}

	// Enable Purge if all backends support it
	if f.even.Features().Purge != nil && f.odd.Features().Purge != nil && f.parity.Features().Purge != nil {
		f.features.Purge = f.Purge
	}

	// Enable ListR when at least one backend supports ListR or is local
	// Similar to union backend: enable if any backend supports it, or all are local
	hasListR := false
	allLocal := true
	for _, backend := range []fs.Fs{f.even, f.odd, f.parity} {
		if backend != nil {
			if backend.Features().ListR != nil {
				hasListR = true
			}
			if !backend.Features().IsLocal {
				allLocal = false
			}
		}
	}
	if hasListR || allLocal {
		f.features.ListR = f.ListR
	}

	// Enable DirSetModTime if at least one backend supports it
	// Unlike other features, we enable if ANY backend supports it (not all)
	// This matches union backend behavior - it sets modtime on backends that support it
	hasDirSetModTime := false
	for _, backend := range []fs.Fs{f.even, f.odd, f.parity} {
		if backend != nil && backend.Features().DirSetModTime != nil {
			hasDirSetModTime = true
			break
		}
	}
	if hasDirSetModTime {
		f.features.DirSetModTime = f.DirSetModTime
	}

	// Enable metadata features if at least one backend supports them
	// Unlike other features, metadata is "best effort" - enable if any backend supports it
	// This matches union backend behavior
	hasReadMetadata := false
	hasWriteMetadata := false
	hasUserMetadata := false
	hasReadDirMetadata := false
	hasWriteDirMetadata := false
	hasUserDirMetadata := false

	for _, backend := range []fs.Fs{f.even, f.odd, f.parity} {
		if backend != nil {
			feat := backend.Features()
			if feat.ReadMetadata {
				hasReadMetadata = true
			}
			if feat.WriteMetadata {
				hasWriteMetadata = true
			}
			if feat.UserMetadata {
				hasUserMetadata = true
			}
			if feat.ReadDirMetadata {
				hasReadDirMetadata = true
			}
			if feat.WriteDirMetadata {
				hasWriteDirMetadata = true
			}
			if feat.UserDirMetadata {
				hasUserDirMetadata = true
			}
		}
	}

	f.features.ReadMetadata = hasReadMetadata
	f.features.WriteMetadata = hasWriteMetadata
	f.features.UserMetadata = hasUserMetadata
	f.features.ReadDirMetadata = hasReadDirMetadata
	f.features.WriteDirMetadata = hasWriteDirMetadata
	f.features.UserDirMetadata = hasUserDirMetadata

	// Enable MkdirMetadata if at least one backend supports it
	// This matches union backend behavior
	hasMkdirMetadata := false
	for _, backend := range []fs.Fs{f.even, f.odd, f.parity} {
		if backend != nil && backend.Features().MkdirMetadata != nil {
			hasMkdirMetadata = true
			break
		}
	}
	if hasMkdirMetadata {
		f.features.MkdirMetadata = f.MkdirMetadata
	}

	// Initialize heal infrastructure
	// Derive upload context from parent context for proper cancellation propagation
	// This ensures that when the parent context is cancelled, heal operations are also cancelled
	f.uploadQueue = newUploadQueue()
	f.uploadCtx, f.uploadCancel = context.WithCancel(ctx)
	f.uploadWorkers = defaultUploadWorkers

	// Start background upload workers
	for i := 0; i < f.uploadWorkers; i++ {
		go f.backgroundUploader(f.uploadCtx, i)
	}

	// Return ErrorIsFile if we adjusted the root for a file path
	if returnErrorIsFile {
		return f, fs.ErrorIsFile
	}

	return f, nil
}

// Name of the remote (as passed into NewFs)
func (f *Fs) Name() string {
	return f.name
}

// Root of the remote (as passed into NewFs)
func (f *Fs) Root() string {
	return f.root
}

// Command the backend to run a named command
//
// The command run is name
// args may be used to read arguments from
// opts may be used to read optional arguments from
//
// The result should be capable of being JSON encoded
// If it is a string or a []string it will be shown to the user
// otherwise it will be JSON encoded and shown to the user like that

// String converts this Fs to a string
func (f *Fs) String() string {
	return fmt.Sprintf("raid3 root '%s'", f.root)
}

// Features returns the optional features of this Fs
func (f *Fs) Features() *fs.Features {
	return f.features
}

// Hashes returns the supported hash types
func (f *Fs) Hashes() hash.Set {
	return f.hashSet
}

// Precision returns the precision
func (f *Fs) Precision() time.Duration {
	p1 := f.even.Precision()
	p2 := f.odd.Precision()
	p3 := f.parity.Precision()

	// Return the maximum precision
	max := p1
	if p2 > max {
		max = p2
	}
	if p3 > max {
		max = p3
	}
	return max
}

// About gets quota information for the raid3 backend by aggregating
// the underlying even/odd/parity backends.
//
// If none of the backends implement About, it returns fs.ErrorNotImplemented.
func (f *Fs) About(ctx context.Context) (*fs.Usage, error) {
	// Helper to add src into dst, treating nil as "unknown"
	add := func(dst **int64, src *int64) {
		if src == nil {
			// If any backend doesn't report this field, propagate nil
			*dst = nil
			return
		}
		if *dst == nil {
			// First value we see for this field
			v := *src
			*dst = &v
			return
		}
		**dst += *src
	}

	usage := &fs.Usage{}
	var haveUsage bool
	var lastErr error

	backends := []fs.Fs{f.even, f.odd, f.parity}
	for _, b := range backends {
		if b == nil {
			continue
		}
		aboutFn := b.Features().About
		if aboutFn == nil {
			continue
		}
		u, err := aboutFn(ctx)
		if err != nil {
			// If a backend can't report usage, remember the error but
			// keep trying others. If none succeed we'll return the last error.
			lastErr = err
			continue
		}
		haveUsage = true
		add(&usage.Total, u.Total)
		add(&usage.Used, u.Used)
		add(&usage.Trashed, u.Trashed)
		add(&usage.Other, u.Other)
		add(&usage.Free, u.Free)
		add(&usage.Objects, u.Objects)
	}

	if !haveUsage {
		if lastErr != nil {
			return nil, lastErr
		}
		return nil, fs.ErrorNotImplemented
	}

	return usage, nil
}

// disableRetriesForWrites creates a context with retries disabled to enforce
// strict write policy. This prevents rclone's command-level retry logic from
// creating degraded files when a backend is unavailable.
//
// Hardware RAID 3 blocks writes in degraded mode. By disabling retries, we
// ensure that if a write fails due to unavailable backend, it fails immediately
// without retry attempts that could create partial/degraded files.

// Shutdown waits for pending heal uploads to complete
func (f *Fs) Shutdown(ctx context.Context) error {
	// Check if there are pending uploads
	if f.uploadQueue.len() == 0 {
		f.uploadCancel() // Cancel workers
		return nil
	}

	fs.Infof(f, "Waiting for %d heal upload(s) to complete...", f.uploadQueue.len())

	// Close the job channel to signal no more jobs
	close(f.uploadQueue.jobs)

	// Wait for pending uploads with timeout
	done := make(chan struct{})
	go func() {
		f.uploadWg.Wait()
		close(done)
	}()

	select {
	case <-done:
		fs.Infof(f, "Heal complete")
		f.uploadCancel()
		return nil
	case <-time.After(defaultShutdownTimeout):
		fs.Errorf(f, "Timeout waiting for heal uploads (some may be incomplete)")
		f.uploadCancel()
		return errors.New("timeout waiting for heal uploads")
	case <-ctx.Done():
		fs.Errorf(f, "Context cancelled while waiting for heal uploads")
		f.uploadCancel()
		return ctx.Err()
	}
}

// Helper functions for rebuild

// countParticles counts the number of particles on a backend

// List the objects and directories in dir into entries

// Mkdir makes a directory
func (f *Fs) Mkdir(ctx context.Context, dir string) error {
	// Pre-flight health check: Enforce strict RAID 3 write policy
	// Consistent with Put/Update/Move operations
	if err := f.checkAllBackendsAvailable(ctx); err != nil {
		return err // Returns enhanced error with rebuild guidance
	}

	g, gCtx := errgroup.WithContext(ctx)

	g.Go(func() error {
		err := f.even.Mkdir(gCtx, dir)
		if err != nil {
			return fmt.Errorf("%s: mkdir failed: %w", f.even.Name(), err)
		}
		return nil
	})

	g.Go(func() error {
		err := f.odd.Mkdir(gCtx, dir)
		if err != nil {
			return fmt.Errorf("%s: mkdir failed: %w", f.odd.Name(), err)
		}
		return nil
	})

	g.Go(func() error {
		err := f.parity.Mkdir(gCtx, dir)
		if err != nil {
			return fmt.Errorf("%s: mkdir failed: %w", f.parity.Name(), err)
		}
		return nil
	})

	return g.Wait()
}

// Rmdir removes a directory
func (f *Fs) Rmdir(ctx context.Context, dir string) error {
	// Check if directory exists before health check (union backend pattern)
	// This ensures we return fs.ErrorDirNotFound immediately when directory doesn't exist,
	// matching test expectations (TestStandard/FsRmdirNotFound)
	// We check existence first to avoid side effects from health check
	dirExists, err := f.checkDirectoryExists(ctx, dir)
	if err != nil {
		return fmt.Errorf("failed to check directory existence: %w", err)
	}
	if !dirExists {
		return fs.ErrorDirNotFound
	}

	// Pre-flight check: Enforce strict RAID 3 delete policy
	// Fail immediately if any backend is unavailable to prevent partial deletes
	// Note: The health check only creates a test directory, not the target directory
	if err := f.checkAllBackendsAvailable(ctx); err != nil {
		return fmt.Errorf("rmdir blocked in degraded mode (RAID 3 policy): %w", err)
	}

	// Directory exists, proceed with removal
	// Use regular goroutines to collect all errors (not errgroup to avoid context cancellation)
	type rmdirResult struct {
		err error
	}
	results := make(chan rmdirResult, 3)

	go func() {
		results <- rmdirResult{err: f.even.Rmdir(ctx, dir)}
	}()
	go func() {
		results <- rmdirResult{err: f.odd.Rmdir(ctx, dir)}
	}()
	go func() {
		results <- rmdirResult{err: f.parity.Rmdir(ctx, dir)}
	}()

	var evenErr, oddErr, parityErr error
	for i := 0; i < 3; i++ {
		result := <-results
		switch i {
		case 0:
			evenErr = result.err
		case 1:
			oddErr = result.err
		case 2:
			parityErr = result.err
		}
	}

	// Collect non-nil errors
	var allErrors []error
	if evenErr != nil {
		allErrors = append(allErrors, evenErr)
	}
	if oddErr != nil {
		allErrors = append(allErrors, oddErr)
	}
	if parityErr != nil {
		allErrors = append(allErrors, parityErr)
	}

	// Convert individual ErrorDirNotFound to nil (idempotent behavior)
	// This handles the case where some backends succeed and some return ErrorDirNotFound
	// (e.g., if directory was removed on one backend but not others)
	var filteredErrors []error
	for _, err := range allErrors {
		if !errors.Is(err, fs.ErrorDirNotFound) && !os.IsNotExist(err) {
			filteredErrors = append(filteredErrors, err)
		}
	}

	// If any non-ErrorDirNotFound error exists, return first error
	if len(filteredErrors) > 0 {
		return filteredErrors[0]
	}

	// All succeeded or all returned ErrorDirNotFound (idempotent)
	return nil
}

// Purge deletes all the files and directories in the given directory
//
// Optional interface: Only implement this if you have a way of
// deleting all the files quicker than just running Remove() on the
// result of List()
func (f *Fs) Purge(ctx context.Context, dir string) error {
	// Pre-flight check: Enforce strict RAID 3 delete policy
	// Fail immediately if any backend is unavailable to prevent partial purges
	if err := f.checkAllBackendsAvailable(ctx); err != nil {
		return fmt.Errorf("purge blocked in degraded mode (RAID 3 policy): %w", err)
	}

	// Check if all backends support Purge
	evenPurge := f.even.Features().Purge
	oddPurge := f.odd.Features().Purge
	parityPurge := f.parity.Features().Purge

	// If all backends support Purge, use it
	if evenPurge != nil && oddPurge != nil && parityPurge != nil {
		g, gCtx := errgroup.WithContext(ctx)

		var evenErr, oddErr, parityErr error

		g.Go(func() error {
			evenErr = evenPurge(gCtx, dir)
			return nil
		})

		g.Go(func() error {
			oddErr = oddPurge(gCtx, dir)
			return nil
		})

		g.Go(func() error {
			parityErr = parityPurge(gCtx, dir)
			return nil
		})

		if err := g.Wait(); err != nil {
			return err
		}

		// Check if all backends returned ErrorDirNotFound
		allNotFound := errors.Is(evenErr, fs.ErrorDirNotFound) &&
			errors.Is(oddErr, fs.ErrorDirNotFound) &&
			errors.Is(parityErr, fs.ErrorDirNotFound)

		if allNotFound {
			// If all backends return ErrorDirNotFound, return ErrorDirNotFound
			// The test suite expects this error when purging a non-existent directory
			return fs.ErrorDirNotFound
		}

		// Convert individual ErrorDirNotFound to nil (idempotent behavior)
		// This handles the case where some backends succeed and some return ErrorDirNotFound
		if errors.Is(evenErr, fs.ErrorDirNotFound) {
			evenErr = nil
		}
		if errors.Is(oddErr, fs.ErrorDirNotFound) {
			oddErr = nil
		}
		if errors.Is(parityErr, fs.ErrorDirNotFound) {
			parityErr = nil
		}

		// Collect non-nil errors
		var allErrors []error
		if evenErr != nil {
			allErrors = append(allErrors, fmt.Errorf("%s: purge failed: %w", f.even.Name(), evenErr))
		}
		if oddErr != nil {
			allErrors = append(allErrors, fmt.Errorf("%s: purge failed: %w", f.odd.Name(), oddErr))
		}
		if parityErr != nil {
			allErrors = append(allErrors, fmt.Errorf("%s: purge failed: %w", f.parity.Name(), parityErr))
		}

		// Return first error if any, otherwise success
		if len(allErrors) > 0 {
			return allErrors[0]
		}

		return nil
	}

	// Fall back to fs.ErrorCantPurge if not all backends support it
	// But first check if the directory exists and has content - if it doesn't exist or is empty,
	// return ErrorDirNotFound. This is required because the fallback path (List + Delete + Rmdir)
	// is idempotent and returns nil for non-existent/empty directories, but the test suite expects
	// ErrorDirNotFound when purging an already-purged directory.
	entries, err := f.List(ctx, dir)
	if err != nil {
		// If directory doesn't exist, return ErrorDirNotFound
		if errors.Is(err, fs.ErrorDirNotFound) {
			return fs.ErrorDirNotFound
		}
		return fmt.Errorf("failed to list directory: %w", err)
	}
	// If directory is empty (no entries), return ErrorDirNotFound
	// This handles the case where the directory was already purged
	if len(entries) == 0 {
		return fs.ErrorDirNotFound
	}
	// Directory exists and has content, fall back to operations.Purge which will use List + Delete + Rmdir
	return fs.ErrorCantPurge
}

// CleanUp removes broken objects (only 1 particle present)
// This implements the optional fs.CleanUpper interface
func (f *Fs) CleanUp(ctx context.Context) error {
	// Pre-flight check: Enforce strict RAID 3 delete policy
	// Fail immediately if any backend is unavailable to prevent partial cleanup
	if err := f.checkAllBackendsAvailable(ctx); err != nil {
		return fmt.Errorf("cleanup blocked in degraded mode (RAID 3 policy): %w", err)
	}

	fs.Infof(f, "Scanning for broken objects...")

	// Scan root directory recursively
	brokenObjects, totalSize, err := f.findBrokenObjects(ctx, "")
	if err != nil {
		return fmt.Errorf("failed to scan for broken objects: %w", err)
	}

	if len(brokenObjects) == 0 {
		fs.Infof(f, "No broken objects found")
		return nil
	}

	fs.Infof(f, "Found %d broken objects (total size: %s)",
		len(brokenObjects), fs.SizeSuffix(totalSize))

	// Remove broken objects
	var cleanedCount int
	var cleanedSize int64
	for _, obj := range brokenObjects {
		fs.Infof(f, "Cleaning up broken object: %s (%d particle)",
			obj.remote, obj.count)

		err := f.removeBrokenObject(ctx, obj)
		if err != nil {
			fs.Errorf(f, "Failed to clean up %s: %v", obj.remote, err)
			continue
		}

		cleanedCount++
		cleanedSize += obj.size
	}

	fs.Infof(f, "Cleaned up %d broken objects (freed %s)",
		cleanedCount, fs.SizeSuffix(cleanedSize))

	return nil
}

// findBrokenObjects recursively finds all objects with only 1 particle
func (f *Fs) findBrokenObjects(ctx context.Context, dir string) ([]particleInfo, int64, error) {
	var brokenObjects []particleInfo
	var totalSize int64

	// Use scanParticles to get all objects from all backends (including broken ones)
	// This directly lists from all three backends without filtering
	allParticles, err := f.scanParticles(ctx, dir)
	if err != nil {
		return nil, 0, err
	}

	// Filter for broken objects (less than 2 particles)
	for _, pi := range allParticles {
		if pi.count < 2 {
			brokenObjects = append(brokenObjects, pi)
			totalSize += f.getBrokenObjectSize(ctx, pi)
		}
	}

	// Recursively scan subdirectories
	entries, err := f.listDirectories(ctx, dir)
	if err != nil {
		fs.Debugf(f, "Failed to list directories in %s: %v", dir, err)
	} else {
		for _, entry := range entries {
			if dirEntry, ok := entry.(fs.Directory); ok {
				// Recursively scan subdirectory
				subBroken, subSize, err := f.findBrokenObjects(ctx, dirEntry.Remote())
				if err != nil {
					fs.Errorf(f, "Failed to scan directory %s: %v", dirEntry.Remote(), err)
					continue
				}
				brokenObjects = append(brokenObjects, subBroken...)
				totalSize += subSize
			}
		}
	}

	return brokenObjects, totalSize, nil
}

// getBrokenObjectSize gets the size of a broken object's single particle
func (f *Fs) getBrokenObjectSize(ctx context.Context, p particleInfo) int64 {
	if p.evenExists {
		obj, err := f.even.NewObject(ctx, p.remote)
		if err == nil {
			return obj.Size()
		}
	}
	if p.oddExists {
		obj, err := f.odd.NewObject(ctx, p.remote)
		if err == nil {
			return obj.Size()
		}
	}
	if p.parityExists {
		// Try both parity suffixes
		obj, err := f.parity.NewObject(ctx, GetParityFilename(p.remote, true))
		if err == nil {
			return obj.Size()
		}
		obj, err = f.parity.NewObject(ctx, GetParityFilename(p.remote, false))
		if err == nil {
			return obj.Size()
		}
	}
	return 0
}

// removeBrokenObject removes a broken object from all backends where it exists
func (f *Fs) removeBrokenObject(ctx context.Context, p particleInfo) error {
	g, gCtx := errgroup.WithContext(ctx)

	if p.evenExists {
		g.Go(func() error {
			obj, err := f.even.NewObject(gCtx, p.remote)
			if err != nil {
				return nil // Already removed or doesn't exist
			}
			return obj.Remove(gCtx)
		})
	}

	if p.oddExists {
		g.Go(func() error {
			obj, err := f.odd.NewObject(gCtx, p.remote)
			if err != nil {
				return nil // Already removed or doesn't exist
			}
			return obj.Remove(gCtx)
		})
	}

	if p.parityExists {
		g.Go(func() error {
			// Try both parity suffixes first (normal case)
			obj, err := f.parity.NewObject(gCtx, GetParityFilename(p.remote, true))
			if err == nil {
				return obj.Remove(gCtx)
			}
			obj, err = f.parity.NewObject(gCtx, GetParityFilename(p.remote, false))
			if err == nil {
				return obj.Remove(gCtx)
			}
			// Also try the original name (for orphaned files without suffix)
			obj, err = f.parity.NewObject(gCtx, p.remote)
			if err == nil {
				return obj.Remove(gCtx)
			}
			return nil // No parity found
		})
	}

	return g.Wait()
}

// reconstructMissingDirectory creates missing directories when directory exists on 2/3 backends
// This implements heal for 1dm case (directory degraded but reconstructable)
// Called by List() when auto_cleanup is enabled
func (f *Fs) reconstructMissingDirectory(ctx context.Context, dir string, errEven, errOdd error) {
	// Check which backends have the directory
	evenExists := errEven == nil
	oddExists := errOdd == nil

	// Check parity separately
	_, errParity := f.parity.List(ctx, dir)
	parityExists := errParity == nil

	// Count how many backends have this directory
	count := 0
	if evenExists {
		count++
	}
	if oddExists {
		count++
	}
	if parityExists {
		count++
	}

	// If directory exists on exactly 2/3 backends, reconstruct the missing one
	if count == 2 {
		dirPath := dir
		if dirPath == "" {
			dirPath = f.root
		}

		// Create missing directory on the backend that doesn't have it
		if !evenExists {
			fs.Infof(f, "Reconstructing missing directory %q on even backend (2/3 → 3/3)", dirPath)
			err := f.even.Mkdir(ctx, dir)
			if err != nil && !errors.Is(err, fs.ErrorDirExists) {
				fs.Debugf(f, "Failed to reconstruct directory on even: %v", err)
			}
		}
		if !oddExists {
			fs.Infof(f, "Reconstructing missing directory %q on odd backend (2/3 → 3/3)", dirPath)
			err := f.odd.Mkdir(ctx, dir)
			if err != nil && !errors.Is(err, fs.ErrorDirExists) {
				fs.Debugf(f, "Failed to reconstruct directory on odd: %v", err)
			}
		}
		if !parityExists {
			fs.Infof(f, "Reconstructing missing directory %q on parity backend (2/3 → 3/3)", dirPath)
			err := f.parity.Mkdir(ctx, dir)
			if err != nil && !errors.Is(err, fs.ErrorDirExists) {
				fs.Debugf(f, "Failed to reconstruct directory on parity: %v", err)
			}
		}
	}
}

// checkSubdirectoryExists checks if a subdirectory exists by listing the parent directory
// and searching for it. This follows union backend's findEntry pattern.
// Returns true if subdirectory exists, false otherwise.
func (f *Fs) checkSubdirectoryExists(ctx context.Context, backend fs.Fs, subDirName string) bool {
	// Get the full remote string for this backend
	remoteString := fs.ConfigString(backend)

	// Split into remote name and path
	remoteName, remotePath, err := fspath.SplitFs(remoteString)
	if err != nil {
		fs.Debugf(f, "Rmdir: Failed to split remote string %q: %v", remoteString, err)
		// Fallback to direct List() check
		_, listErr := backend.List(ctx, "")
		return listErr == nil
	}

	// Get parent path
	parentPath := path.Dir(remotePath)
	if parentPath == "." {
		parentPath = ""
	}

	// Construct parent remote string
	parentRemote := remoteName
	if parentRemote != "" {
		parentRemote += ":"
	}
	parentRemote += parentPath

	// Get parent Fs
	parentFs, err := cache.Get(ctx, parentRemote)
	if err != nil {
		fs.Debugf(f, "Rmdir: Failed to get parent Fs %q: %v", parentRemote, err)
		// Fallback to direct List() check
		_, listErr := backend.List(ctx, "")
		return listErr == nil
	}

	// List parent directory and search for subdirectory
	entries, err := parentFs.List(ctx, "")
	if err != nil {
		fs.Debugf(f, "Rmdir: Failed to list parent directory %q: %v", parentRemote, err)
		return false
	}

	// Search for subdirectory in entries
	caseInsensitive := parentFs.Features().CaseInsensitive
	for _, entry := range entries {
		entryRemote := entry.Remote()
		found := false
		if caseInsensitive {
			found = strings.EqualFold(entryRemote, subDirName)
		} else {
			found = (entryRemote == subDirName)
		}
		if found {
			return true
		}
	}

	return false
}

// cleanupOrphanedDirectory removes directories that exist on < 2 backends
// This is called by List() when auto_cleanup is enabled and the directory is empty
// errEven and errOdd are the errors from the List operations (already performed)
func (f *Fs) cleanupOrphanedDirectory(ctx context.Context, dir string, errEven, errOdd error) {
	// Determine if directory exists on each backend based on List errors
	// Directory exists if List succeeded (err == nil) or returned empty
	// Directory doesn't exist if List returned ErrorDirNotFound
	evenExists := errEven == nil
	oddExists := errOdd == nil

	// For parity, we need to check separately since we don't have its error
	type dirResult struct {
		exists bool
	}
	resultCh := make(chan dirResult, 1)
	go func() {
		_, err := f.parity.List(ctx, dir)
		exists := err == nil
		resultCh <- dirResult{exists}
	}()
	res := <-resultCh
	parityExists := res.exists

	// Count how many backends have this directory
	count := 0
	if evenExists {
		count++
	}
	if oddExists {
		count++
	}
	if parityExists {
		count++
	}

	// If directory exists on < 2 backends, it's orphaned - remove it
	if count < 2 && count > 0 {
		dirPath := dir
		if dirPath == "" {
			dirPath = f.root
		}
		fs.Infof(f, "Auto-cleanup: removing orphaned directory %q (exists on %d/3 backends)", dirPath, count)

		// Remove from all backends where it exists (best effort)
		if evenExists {
			err := f.even.Rmdir(ctx, dir)
			if err != nil && !errors.Is(err, fs.ErrorDirNotFound) {
				fs.Debugf(f, "Auto-cleanup: failed to remove %q from even: %v", dir, err)
			} else {
				fs.Debugf(f, "Auto-cleanup: removed %q from even", dir)
			}
		}
		if oddExists {
			err := f.odd.Rmdir(ctx, dir)
			if err != nil && !errors.Is(err, fs.ErrorDirNotFound) {
				fs.Debugf(f, "Auto-cleanup: failed to remove %q from odd: %v", dir, err)
			} else {
				fs.Debugf(f, "Auto-cleanup: removed %q from odd", dir)
			}
		}
		if parityExists {
			err := f.parity.Rmdir(ctx, dir)
			if err != nil && !errors.Is(err, fs.ErrorDirNotFound) {
				fs.Debugf(f, "Auto-cleanup: failed to remove %q from parity: %v", dir, err)
			} else {
				fs.Debugf(f, "Auto-cleanup: removed %q from parity", dir)
			}
		}
	}
}

// Check the interfaces are satisfied
var (
	_ fs.Fs              = (*Fs)(nil)
	_ fs.CleanUpper      = (*Fs)(nil)
	_ fs.DirMover        = (*Fs)(nil)
	_ fs.DirSetModTimer  = (*Fs)(nil)
	_ fs.MkdirMetadataer = (*Fs)(nil)
	_ fs.ListRer         = (*Fs)(nil)
	_ fs.Object          = (*Object)(nil)
	_ fs.Metadataer      = (*Object)(nil)
	_ fs.SetMetadataer   = (*Object)(nil)
	_ fs.Directory       = (*Directory)(nil)
	_ fs.Metadataer      = (*Directory)(nil)
	_ fs.SetMetadataer   = (*Directory)(nil)
	_ fs.SetModTimer     = (*Directory)(nil)
)
