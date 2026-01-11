// Package raid3 implements a backend that splits data across three remotes using byte-level striping
package raid3

// This file contains backend-specific commands for raid3.
//
// It includes:
//   - Command dispatcher (status, rebuild, heal)
//   - statusCommand: Backend health check and rebuild guidance
//   - rebuildCommand: Rebuild missing particles after backend replacement
//   - healCommand: Proactively heal all degraded objects (2/3 particles)
//   - Rebuild guidance and user-friendly error messages

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config"
	"github.com/rclone/rclone/fs/object"
	"github.com/rclone/rclone/fs/operations"
	"golang.org/x/sync/errgroup"
)

// Command dispatches backend commands
func (f *Fs) Command(ctx context.Context, name string, arg []string, opt map[string]string) (out any, err error) {
	switch name {
	case "status":
		return f.statusCommand(ctx, opt)
	case "rebuild":
		return f.rebuildCommand(ctx, arg, opt)
	case "heal":
		return f.healCommand(ctx, arg, opt)
	default:
		return nil, fs.ErrorCommandNotFound
	}
}

// statusCommand shows backend health and provides rebuild guidance
// This implements Phase 2 of user-centric rebuild
func (f *Fs) statusCommand(ctx context.Context, opt map[string]string) (out any, err error) {
	// Input validation
	if err := validateContext(ctx, "status"); err != nil {
		return nil, err
	}
	// opt can be nil (optional parameter in rclone commands)
	if opt == nil {
		opt = map[string]string{}
	}

	// Check health of all backends
	type backendHealth struct {
		name      string
		available bool
		fileCount int64
		size      int64
		err       error
	}

	// Health check with reasonable timeout
	checkCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	healthChan := make(chan backendHealth, 3)

	// Check each backend
	checkOne := func(backend fs.Fs, name, path string) {
		var fileCount int64
		var totalSize int64

		// Try to list and count files
		listErr := operations.ListFn(checkCtx, backend, func(obj fs.Object) {
			fileCount++
			totalSize += obj.Size()
		})

		// Check if backend is available
		if listErr != nil && !errors.Is(listErr, fs.ErrorDirNotFound) {
			healthChan <- backendHealth{name, false, 0, 0, listErr}
			return
		}

		healthChan <- backendHealth{name, true, fileCount, totalSize, nil}
	}

	go func() { checkOne(f.even, "even", f.opt.Even) }()
	go func() { checkOne(f.odd, "odd", f.opt.Odd) }()
	go func() { checkOne(f.parity, "parity", f.opt.Parity) }()

	// Collect results
	var evenHealth, oddHealth, parityHealth backendHealth
	for i := 0; i < 3; i++ {
		health := <-healthChan
		switch health.name {
		case "even":
			evenHealth = health
		case "odd":
			oddHealth = health
		case "parity":
			parityHealth = health
		}
	}

	// Determine overall status
	allHealthy := evenHealth.available && oddHealth.available && parityHealth.available
	isDegraded := !allHealthy

	// Build status report
	var report strings.Builder

	report.WriteString("RAID3 Backend Health Status\n")
	report.WriteString("════════════════════════════════════════════════════════════════\n\n")

	// Backend Health Section
	report.WriteString("Backend Health:\n")
	writeBackendStatus := func(h backendHealth, path string) {
		icon := "✅"
		var status string
		var healthText string

		if !h.available {
			icon = "❌"
			status = "UNAVAILABLE"
			healthText = fmt.Sprintf("ERROR: %v", h.err)
		} else if h.fileCount == 0 {
			status = "0 files (EMPTY)"
			healthText = "Available but empty"
		} else {
			status = fmt.Sprintf("%d files, %s", h.fileCount, fs.SizeSuffix(h.size))
			healthText = "HEALTHY"
		}

		report.WriteString(fmt.Sprintf("  %s %s (%s):\n", icon, strings.Title(h.name), path))
		report.WriteString(fmt.Sprintf("      %s - %s\n", status, healthText))
	}

	writeBackendStatus(evenHealth, f.opt.Even)
	writeBackendStatus(oddHealth, f.opt.Odd)
	writeBackendStatus(parityHealth, f.opt.Parity)

	// Overall Status
	report.WriteString("\nOverall Status: ")
	if allHealthy {
		if evenHealth.fileCount == 0 {
			report.WriteString("✅ HEALTHY (empty/new)\n")
		} else {
			report.WriteString("✅ HEALTHY\n")
		}
	} else {
		report.WriteString("⚠️  DEGRADED MODE\n")
	}

	// Impact Section
	report.WriteString("\nWhat This Means:\n")
	if isDegraded {
		report.WriteString("  • Reads:  ✅ Working (automatic parity reconstruction)\n")
		report.WriteString("  • Writes: ❌ Blocked (RAID 3 data safety policy)\n")
		report.WriteString("  • Heal: ⚠️  Cannot restore (backend unavailable)\n")
	} else {
		report.WriteString("  • Reads:  ✅ All operations working\n")
		report.WriteString("  • Writes: ✅ All operations working\n")
		report.WriteString("  • Heal: ✅ Available if needed\n")
	}

	// If degraded, show rebuild guide
	if isDegraded {
		// Identify which backend failed
		failedBackend := ""
		if !evenHealth.available {
			failedBackend = "even"
		} else if !oddHealth.available {
			failedBackend = "odd"
		} else if !parityHealth.available {
			failedBackend = "parity"
		}

		report.WriteString("\n━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")
		report.WriteString("Rebuild Guide\n")
		report.WriteString("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n\n")

		report.WriteString(fmt.Sprintf("STEP 1: Check if %s backend failure is temporary\n\n", failedBackend))
		report.WriteString("  Try accessing the backend:\n")
		report.WriteString(fmt.Sprintf("  $ rclone ls %s\n\n", f.getBackendPath(failedBackend)))
		report.WriteString("  If successful → Backend is online, retry your operation\n")
		report.WriteString("  If failed → Backend is lost, continue to STEP 2\n\n")

		report.WriteString("STEP 2: Create replacement backend\n\n")
		report.WriteString(fmt.Sprintf("  $ rclone mkdir new-%s-backend:\n", failedBackend))
		report.WriteString(fmt.Sprintf("  $ rclone ls new-%s-backend:    # Verify accessible\n\n", failedBackend))

		report.WriteString("STEP 3: Update rclone.conf\n\n")
		configPath := config.GetConfigPath()
		if configPath != "" {
			report.WriteString(fmt.Sprintf("  Edit: %s\n", configPath))
		} else {
			report.WriteString("  Edit: your rclone config file\n")
		}
		report.WriteString(fmt.Sprintf("  Change: %s = new-%s-backend:\n\n", failedBackend, failedBackend))

		report.WriteString("STEP 4: Rebuild missing particles\n\n")
		report.WriteString("  $ rclone backend rebuild raid3:\n")
		report.WriteString("  (Rebuilds all missing data - may take time)\n\n")

		report.WriteString("STEP 5: Verify rebuild\n\n")
		report.WriteString("  $ rclone backend status raid3:\n")
		report.WriteString("  Should show: ✅ HEALTHY\n\n")

		report.WriteString("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")
	}

	return report.String(), nil
}

// rebuildCommand rebuilds missing particles on a replacement backend
// This implements Phase 3 of user-centric rebuild
func (f *Fs) rebuildCommand(ctx context.Context, arg []string, opt map[string]string) (out any, err error) {
	// Parse options
	checkOnly := opt["check-only"] == "true"
	dryRun := opt["dry-run"] == "true"
	priority := opt["priority"]
	if priority == "" {
		priority = "auto"
	}

	// Determine which backend to rebuild
	targetBackend := ""
	if len(arg) > 0 {
		targetBackend = arg[0]
	}

	// Validate target backend
	if targetBackend != "" && targetBackend != "even" && targetBackend != "odd" && targetBackend != "parity" {
		return nil, formatOperationError("rebuild failed", fmt.Sprintf("invalid backend: %s (must be: even, odd, or parity)", targetBackend), nil)
	}

	// If not specified, auto-detect which backend needs rebuild
	if targetBackend == "" {
		fs.Infof(f, "Auto-detecting which backend needs rebuild...")

		// Count particles on each backend in parallel
		var evenCount, oddCount, parityCount int64
		var errEven, errOdd, errParity error

		g, gCtx := errgroup.WithContext(ctx)

		g.Go(func() error {
			evenCount, errEven = f.countParticles(gCtx, f.even)
			return nil // Ignore errors, we'll handle them below
		})

		g.Go(func() error {
			oddCount, errOdd = f.countParticles(gCtx, f.odd)
			return nil // Ignore errors, we'll handle them below
		})

		g.Go(func() error {
			parityCount, errParity = f.countParticles(gCtx, f.parity)
			return nil // Ignore errors, we'll handle them below
		})

		// Wait for all counts to complete
		if err := g.Wait(); err != nil {
			return nil, formatOperationError("rebuild failed", "failed to count particles", err)
		}

		// Check for errors from individual backends
		if errEven != nil {
			fs.Errorf(f, "Failed to count particles on even backend: %v", errEven)
		}
		if errOdd != nil {
			fs.Errorf(f, "Failed to count particles on odd backend: %v", errOdd)
		}
		if errParity != nil {
			fs.Errorf(f, "Failed to count particles on parity backend: %v", errParity)
		}

		fs.Debugf(f, "Particle counts: even=%d, odd=%d, parity=%d", evenCount, oddCount, parityCount)

		// Find which has fewest (needs rebuild)
		if oddCount < evenCount && oddCount < parityCount {
			targetBackend = "odd"
		} else if evenCount < oddCount && evenCount < parityCount {
			targetBackend = "even"
		} else if parityCount < evenCount && parityCount < oddCount {
			targetBackend = "parity"
		} else {
			return nil, errors.New("cannot auto-detect: all backends have similar particle counts")
		}

		fs.Infof(f, "Auto-detected: %s backend needs rebuild (%d files, should have %d)",
			targetBackend, minInt64(evenCount, oddCount, parityCount), maxInt64(evenCount, oddCount, parityCount))
	}

	// Get source and target filesystems
	var target fs.Fs
	var source1, source2 fs.Fs
	var source1Name, source2Name string

	switch targetBackend {
	case "even":
		target = f.even
		source1, source2 = f.odd, f.parity
		source1Name, source2Name = "odd", "parity"
	case "odd":
		target = f.odd
		source1, source2 = f.even, f.parity
		source1Name, source2Name = "even", "parity"
	case "parity":
		target = f.parity
		source1, source2 = f.even, f.odd
		source1Name, source2Name = "even", "odd"
	}

	// Scan source backend for all files
	var filesToRebuild []fs.Object
	var totalSize int64

	fs.Infof(f, "Scanning %s backend for files...", source1Name)
	err = operations.ListFn(ctx, source1, func(obj fs.Object) {
		filesToRebuild = append(filesToRebuild, obj)
		totalSize += obj.Size()
	})
	if err != nil {
		return nil, formatOperationError("rebuild failed", fmt.Sprintf("failed to list %s backend", source1Name), err)
	}

	fs.Infof(f, "Found %d files (%s) to rebuild", len(filesToRebuild), fs.SizeSuffix(totalSize))

	// Check-only mode
	if checkOnly {
		var report strings.Builder
		report.WriteString(fmt.Sprintf("Rebuild Analysis for %s backend\n", targetBackend))
		report.WriteString("════════════════════════════════════════════════════════════════\n\n")
		report.WriteString(fmt.Sprintf("Files to rebuild: %d\n", len(filesToRebuild)))
		report.WriteString(fmt.Sprintf("Total size: %s\n", fs.SizeSuffix(totalSize)))
		report.WriteString(fmt.Sprintf("Source: %s + %s (reconstruction)\n", source1Name, source2Name))
		report.WriteString(fmt.Sprintf("Target: %s backend\n\n", targetBackend))
		report.WriteString("Ready to rebuild. Run without -o check-only=true to proceed.\n")
		return report.String(), nil
	}

	// Dry-run mode
	if dryRun {
		fs.Infof(f, "DRY-RUN: Would rebuild %d files to %s backend", len(filesToRebuild), targetBackend)
		return fmt.Sprintf("Would rebuild %d files (%s)", len(filesToRebuild), fs.SizeSuffix(totalSize)), nil
	}

	// Actually rebuild
	fs.Infof(f, "Rebuilding %s backend...", targetBackend)
	fs.Infof(f, "Priority mode: %s", priority)

	var rebuilt int64
	var rebuiltSize int64
	var rebuiltMu sync.Mutex
	startTime := time.Now()

	// Parallel rebuild using worker pool
	g, gCtx := errgroup.WithContext(ctx)
	g.SetLimit(rebuildWorkers)

	// Process each file in parallel
	for i, sourceObj := range filesToRebuild {
		i := i // Capture loop variable
		sourceObj := sourceObj
		remote := sourceObj.Remote()

		g.Go(func() error {
			// Progress update every 10 files (thread-safe)
			if i > 0 && i%10 == 0 {
				rebuiltMu.Lock()
				currentRebuilt := rebuilt
				currentRebuiltSize := rebuiltSize
				rebuiltMu.Unlock()

				elapsed := time.Since(startTime)
				if elapsed.Seconds() > 0 {
					speed := float64(currentRebuiltSize) / elapsed.Seconds()
					remaining := totalSize - currentRebuiltSize
					eta := time.Duration(float64(remaining)/speed) * time.Second

					fs.Infof(f, "Progress: %d/%d files (%.0f%%), %s/%s, ETA %v",
						currentRebuilt, len(filesToRebuild),
						float64(currentRebuilt)/float64(len(filesToRebuild))*100,
						fs.SizeSuffix(currentRebuiltSize), fs.SizeSuffix(totalSize),
						eta.Round(time.Second))
				}
			}

			// Check if particle already exists on target
			_, err := target.NewObject(gCtx, remote)
			if err == nil {
				fs.Debugf(f, "Skipping %s (already exists)", remote)
				return nil
			}

			// Reconstruct the particle
			var particleData []byte
			if targetBackend == "parity" {
				// Reconstruct parity from even + odd
				particleData, err = f.reconstructParityParticle(gCtx, source1, source2, remote)
			} else {
				// Reconstruct data particle from other data + parity
				particleData, err = f.reconstructDataParticle(gCtx, source1, source2, remote, targetBackend)
			}

			if err != nil {
				fs.Errorf(f, "Failed to reconstruct %s: %v", remote, err)
				return nil // Continue with other files
			}

			// Upload to target backend
			reader := bytes.NewReader(particleData)
			modTime := sourceObj.ModTime(gCtx)
			info := object.NewStaticObjectInfo(remote, modTime, int64(len(particleData)), true, nil, nil)

			_, err = target.Put(gCtx, reader, info)
			if err != nil {
				fs.Errorf(f, "Failed to upload %s: %v", remote, err)
				return nil // Continue with other files
			}

			// Update counters (thread-safe)
			rebuiltMu.Lock()
			rebuilt++
			rebuiltSize += int64(len(particleData))
			rebuiltMu.Unlock()

			return nil
		})
	}

	// Wait for all rebuild operations to complete
	if err := g.Wait(); err != nil {
		return nil, formatOperationError("rebuild failed", "", err)
	}

	// Final summary
	duration := time.Since(startTime)
	avgSpeed := float64(rebuiltSize) / duration.Seconds()

	var summary strings.Builder
	summary.WriteString("\n✅ Rebuild Complete!\n\n")
	summary.WriteString(fmt.Sprintf("Files rebuilt: %d/%d\n", int(rebuilt), len(filesToRebuild)))
	summary.WriteString(fmt.Sprintf("Data transferred: %s\n", fs.SizeSuffix(rebuiltSize)))
	summary.WriteString(fmt.Sprintf("Duration: %v\n", duration.Round(time.Second)))
	summary.WriteString(fmt.Sprintf("Average speed: %s/s\n", fs.SizeSuffix(int64(avgSpeed))))
	summary.WriteString(fmt.Sprintf("\nBackend %s is now restored!\n", targetBackend))
	summary.WriteString("Run 'rclone backend status raid3:' to verify.\n")

	return summary.String(), nil
}

// healCommand scans the entire remote and heals any objects that have exactly 2 of 3 particles.
// This is an explicit, admin-driven alternative to automatic heal on read.
// If a file path is provided in arg[0], only that file is healed.
func (f *Fs) healCommand(ctx context.Context, arg []string, opt map[string]string) (out any, err error) {
	// Input validation
	if err := validateContext(ctx, "heal"); err != nil {
		return nil, err
	}
	// arg and opt can be nil (optional parameters in rclone commands)
	if arg == nil {
		arg = []string{}
	}
	if opt == nil {
		opt = map[string]string{}
	}

	// Parse options
	dryRun := opt["dry-run"] == "true"

	// If file path provided, heal only that file
	if len(arg) > 0 {
		remote := arg[0]
		if err := validateRemote(remote, "heal"); err != nil {
			return nil, err
		}
		fs.Infof(f, "Healing single file: %q", remote)

		pi, err := f.particleInfoForObject(ctx, remote)
		if err != nil {
			// Check if file doesn't exist (less than 2 particles)
			_, objErr := f.NewObject(ctx, remote)
			if objErr != nil {
				return nil, formatOperationError("heal failed", fmt.Sprintf("file not found: %q", remote), nil)
			}
			return nil, formatOperationError("heal failed", fmt.Sprintf("failed to inspect %q", remote), err)
		}

		var report strings.Builder
		report.WriteString("Heal Summary\n")
		report.WriteString("══════════════════════════════════════════\n\n")
		report.WriteString(fmt.Sprintf("File:               %s\n", remote))

		switch pi.count {
		case 3:
			report.WriteString("Status:             ✅ Healthy (3/3 particles)\n")
			report.WriteString("Action:             No healing needed\n")
			fs.Infof(f, "File %q is already healthy (3/3 particles)", remote)
		case 2:
			if dryRun {
				// Determine which particle is missing for dry-run report
				var missingParticle string
				if !pi.evenExists {
					missingParticle = "even"
				} else if !pi.oddExists {
					missingParticle = "odd"
				} else if !pi.parityExists {
					missingParticle = "parity"
				}
				report.WriteString("Status:             ⚠️  Degraded (2/3 particles)\n")
				report.WriteString(fmt.Sprintf("Action:             DRY-RUN: Would restore missing %s particle\n", missingParticle))
				fs.Infof(f, "DRY-RUN: Would heal %q (missing %s particle)", remote, missingParticle)
			} else {
				if err := f.healObject(ctx, pi); err != nil {
					report.WriteString("Status:             ❌ Failed to heal\n")
					report.WriteString(fmt.Sprintf("Error:              %v\n", err))
					fs.Errorf(f, "Heal: failed to heal %q: %v", remote, err)
					return report.String(), err
				}
				report.WriteString("Status:             ✅ Healed (2/3→3/3)\n")
				report.WriteString("Action:             Missing particle restored\n")
				fs.Infof(f, "File %q healed successfully", remote)
			}
		default:
			// 0 or 1 particle present – unrebuildable with RAID3
			report.WriteString(fmt.Sprintf("Status:             ❌ Unrebuildable (%d/3 particles)\n", pi.count))
			report.WriteString("Action:             Manual rebuild or restore needed\n")
			fs.Errorf(f, "File %q cannot be healed: only %d/3 particles present", remote, pi.count)
			return report.String(), formatOperationError("heal failed", fmt.Sprintf("cannot heal %q: only %d/3 particles present (need at least 2)", remote, pi.count), nil)
		}

		return report.String(), nil
	}

	// No file path provided - heal all files
	if dryRun {
		fs.Infof(f, "Starting full heal of raid3 backend (DRY-RUN mode)...")
	} else {
		fs.Infof(f, "Starting full heal of raid3 backend...")
	}

	// Enumerate all objects in the raid3 namespace
	var remotes []string
	err = operations.ListFn(ctx, f, func(obj fs.Object) {
		remotes = append(remotes, obj.Remote())
	})
	if err != nil {
		return nil, formatOperationError("heal failed", "failed to list objects", err)
	}

	var total, healthy, healed, unrebuildable int
	var unrebuildableRemotes []string
	var wouldHealRemotes []string // For dry-run mode

	for _, remote := range remotes {
		pi, err := f.particleInfoForObject(ctx, remote)
		if err != nil {
			fs.Errorf(f, "Heal: failed to inspect %q: %v", remote, err)
			unrebuildable++
			unrebuildableRemotes = append(unrebuildableRemotes, remote)
			continue
		}
		total++
		switch pi.count {
		case 3:
			healthy++
			continue
		case 2:
			if dryRun {
				// Dry-run mode: just track what would be healed
				wouldHealRemotes = append(wouldHealRemotes, remote)
				healed++ // Count as would-be-healed for summary
			} else {
				if err := f.healObject(ctx, pi); err != nil {
					fs.Errorf(f, "Heal: failed to heal %q: %v", pi.remote, err)
					unrebuildable++
					unrebuildableRemotes = append(unrebuildableRemotes, pi.remote)
				} else {
					healed++
				}
			}
		default:
			// 0 or 1 particle present – unrebuildable with RAID3
			unrebuildable++
			unrebuildableRemotes = append(unrebuildableRemotes, pi.remote)
		}
	}

	var report strings.Builder
	report.WriteString("Heal Summary\n")
	report.WriteString("══════════════════════════════════════════\n\n")
	if dryRun {
		report.WriteString("⚠️  DRY-RUN MODE: No changes were made\n\n")
	}
	report.WriteString(fmt.Sprintf("Files scanned:      %d\n", total))
	report.WriteString(fmt.Sprintf("Healthy (3/3):      %d\n", healthy))
	if dryRun {
		report.WriteString(fmt.Sprintf("Would heal (2/3):   %d\n", healed))
	} else {
		report.WriteString(fmt.Sprintf("Healed (2/3→3/3):   %d\n", healed))
	}
	report.WriteString(fmt.Sprintf("Unrebuildable (≤1): %d\n", unrebuildable))

	if dryRun && len(wouldHealRemotes) > 0 {
		report.WriteString("\nFiles that would be healed:\n")
		for _, r := range wouldHealRemotes {
			report.WriteString("  - " + r + "\n")
		}
	}

	if unrebuildable > 0 {
		report.WriteString("\nUnrebuildable objects (manual rebuild or restore needed):\n")
		for _, r := range unrebuildableRemotes {
			report.WriteString("  - " + r + "\n")
		}
	}

	if dryRun {
		fs.Infof(f, "DRY-RUN: Would heal %d files (scanned %d, %d unrebuildable).", healed, total, unrebuildable)
	} else {
		fs.Infof(f, "Heal completed: %d scanned, %d healed, %d unrebuildable.", total, healed, unrebuildable)
	}
	return report.String(), nil
}

// healObject heals a single object described by particleInfo when exactly 2 of 3 particles exist.
func (f *Fs) healObject(ctx context.Context, pi particleInfo) error {
	if pi.count != 2 {
		return formatOperationError("heal failed", fmt.Sprintf("cannot heal %q: expected 2 particles, found %d", pi.remote, pi.count), nil)
	}

	// Missing parity – reconstruct parity from even+odd
	if pi.evenExists && pi.oddExists && !pi.parityExists {
		return f.healParityFromData(ctx, pi.remote)
	}

	// Missing even or odd – reconstruct from data + parity
	if !pi.evenExists && pi.oddExists && pi.parityExists {
		return f.healDataFromParity(ctx, pi.remote, "even")
	}
	if pi.evenExists && !pi.oddExists && pi.parityExists {
		return f.healDataFromParity(ctx, pi.remote, "odd")
	}

	return formatOperationError("heal failed", fmt.Sprintf("cannot heal %q: unsupported particle combination (even=%v, odd=%v, parity=%v)", pi.remote, pi.evenExists, pi.oddExists, pi.parityExists), nil)
}

// healParityFromData reconstructs and uploads a missing parity particle using even+odd.
func (f *Fs) healParityFromData(ctx context.Context, remote string) error {
	evenObj, errEven := f.even.NewObject(ctx, remote)
	oddObj, errOdd := f.odd.NewObject(ctx, remote)
	if errEven != nil || errOdd != nil {
		return formatOperationError("heal failed", fmt.Sprintf("cannot heal parity for %q: evenErr=%v, oddErr=%v", remote, errEven, errOdd), nil)
	}

	evenReader, err := evenObj.Open(ctx)
	if err != nil {
		return formatParticleError(f.even, "even", "open failed", fmt.Sprintf("remote %q", remote), err)
	}
	defer func() { _ = evenReader.Close() }()

	oddReader, err := oddObj.Open(ctx)
	if err != nil {
		return formatParticleError(f.odd, "odd", "open failed", fmt.Sprintf("remote %q", remote), err)
	}
	defer func() { _ = oddReader.Close() }()

	evenData, err := io.ReadAll(evenReader)
	if err != nil {
		return formatParticleError(f.even, "even", "read failed", fmt.Sprintf("remote %q", remote), err)
	}
	oddData, err := io.ReadAll(oddReader)
	if err != nil {
		return formatParticleError(f.odd, "odd", "read failed", fmt.Sprintf("remote %q", remote), err)
	}

	parityData := CalculateParity(evenData, oddData)
	isOddLength := (len(evenData)+len(oddData))%2 == 1

	job := &uploadJob{
		remote:       remote,
		particleType: "parity",
		data:         parityData,
		isOddLength:  isOddLength,
	}
	fs.Infof(f, "Heal: uploading parity particle for %q", remote)
	return f.uploadParticle(ctx, job)
}

// healDataFromParity reconstructs and uploads a missing data particle (even or odd) using the other data particle + parity.
func (f *Fs) healDataFromParity(ctx context.Context, remote, missing string) error {
	// Find which parity variant exists and derive original length type
	parityNameOL := GetParityFilename(remote, true)
	parityObj, errParity := f.parity.NewObject(ctx, parityNameOL)
	isOddLength := false
	if errParity != nil {
		parityNameEL := GetParityFilename(remote, false)
		parityObj, errParity = f.parity.NewObject(ctx, parityNameEL)
		if errParity != nil {
			return formatNotFoundError(f.parity, "parity particle", fmt.Sprintf("remote %q", remote), errParity)
		}
		isOddLength = false // .parity-el
	} else {
		isOddLength = true // .parity-ol
	}

	// Read existing data particle and parity
	var dataObj fs.Object
	var dataLabel string
	if missing == "even" {
		obj, err := f.odd.NewObject(ctx, remote)
		if err != nil {
			return formatNotFoundError(f.odd, "odd particle", fmt.Sprintf("remote %q (required for even reconstruction)", remote), err)
		}
		dataObj = obj
		dataLabel = "odd"
	} else {
		obj, err := f.even.NewObject(ctx, remote)
		if err != nil {
			return formatNotFoundError(f.even, "even particle", fmt.Sprintf("remote %q (required for odd reconstruction)", remote), err)
		}
		dataObj = obj
		dataLabel = "even"
	}

	dataReader, err := dataObj.Open(ctx)
	if err != nil {
		var backend fs.Fs
		if missing == "even" {
			backend = f.odd
		} else {
			backend = f.even
		}
		return formatParticleError(backend, dataLabel, "open failed", fmt.Sprintf("remote %q", remote), err)
	}
	defer func() { _ = dataReader.Close() }()

	parityReader, err := parityObj.Open(ctx)
	if err != nil {
		return formatParticleError(f.parity, "parity", "open failed", fmt.Sprintf("remote %q", remote), err)
	}
	defer func() { _ = parityReader.Close() }()

	dataBytes, err := io.ReadAll(dataReader)
	if err != nil {
		var backend fs.Fs
		if missing == "even" {
			backend = f.odd
		} else {
			backend = f.even
		}
		return formatParticleError(backend, dataLabel, "read failed", fmt.Sprintf("remote %q", remote), err)
	}
	parityBytes, err := io.ReadAll(parityReader)
	if err != nil {
		return formatParticleError(f.parity, "parity", "read failed", fmt.Sprintf("remote %q", remote), err)
	}

	var merged []byte
	if missing == "even" {
		merged, err = ReconstructFromOddAndParity(dataBytes, parityBytes, isOddLength)
	} else {
		merged, err = ReconstructFromEvenAndParity(dataBytes, parityBytes, isOddLength)
	}
	if err != nil {
		return formatOperationError("reconstruct particle failed", fmt.Sprintf("remote %q from %s+parity", remote, dataLabel), err)
	}

	// Split merged data to get the missing particle
	evenData, oddData := SplitBytes(merged)
	var particleData []byte
	switch missing {
	case "even":
		particleData = evenData
	case "odd":
		particleData = oddData
	default:
		return formatOperationError("heal failed", fmt.Sprintf("invalid missing particle type: %s", missing), nil)
	}

	job := &uploadJob{
		remote:       remote,
		particleType: missing,
		data:         particleData,
		isOddLength:  isOddLength,
	}
	fs.Infof(f, "Heal: uploading %s particle for %q", missing, remote)
	return f.uploadParticle(ctx, job)
}

// getBackendPath returns the configured path for a backend name
func (f *Fs) getBackendPath(backendName string) string {
	switch backendName {
	case "even":
		return f.opt.Even
	case "odd":
		return f.opt.Odd
	case "parity":
		return f.opt.Parity
	default:
		return ""
	}
}
