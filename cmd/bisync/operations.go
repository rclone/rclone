// Package bisync implements bisync
// Copyright (c) 2017-2020 Chris Nelson
// Contributions to original python version: Hildo G. Jr., e2t, kalemas, silenceleaf
package bisync

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	gosync "sync"
	"time"

	"github.com/rclone/rclone/cmd/bisync/bilib"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/accounting"
	"github.com/rclone/rclone/fs/log"
	"github.com/rclone/rclone/fs/operations"
	"github.com/rclone/rclone/lib/atexit"
	"github.com/rclone/rclone/lib/terminal"
)

// ErrBisyncAborted signals that bisync is aborted and forces exit code 2
var ErrBisyncAborted = errors.New("bisync aborted")

// bisyncRun keeps bisync runtime state
type bisyncRun struct {
	fs1                fs.Fs
	fs2                fs.Fs
	abort              bool
	critical           bool
	retryable          bool
	basePath           string
	workDir            string
	listing1           string
	listing2           string
	newListing1        string
	newListing2        string
	aliases            bilib.AliasMap
	opt                *Options
	octx               context.Context
	fctx               context.Context
	InGracefulShutdown bool
	CleanupCompleted   bool
	SyncCI             *fs.ConfigInfo
	CancelSync         context.CancelFunc
	DebugName          string
	lockFile           string
	renames            renames
	resyncIs1to2       bool
}

type queues struct {
	copy1to2      bilib.Names
	copy2to1      bilib.Names
	renameSkipped bilib.Names // not renamed because it was equal
	skippedDirs1  *fileList
	skippedDirs2  *fileList
	deletedonboth bilib.Names
}

// Bisync handles lock file, performs bisync run and checks exit status
func Bisync(ctx context.Context, fs1, fs2 fs.Fs, optArg *Options) (err error) {
	defer resetGlobals()
	opt := *optArg // ensure that input is never changed
	b := &bisyncRun{
		fs1:       fs1,
		fs2:       fs2,
		opt:       &opt,
		DebugName: opt.DebugName,
	}

	if opt.CheckFilename == "" {
		opt.CheckFilename = DefaultCheckFilename
	}
	if opt.Workdir == "" {
		opt.Workdir = DefaultWorkdir
	}
	ci := fs.GetConfig(ctx)
	opt.OrigBackupDir = ci.BackupDir

	if ci.TerminalColorMode == fs.TerminalColorModeAlways || (ci.TerminalColorMode == fs.TerminalColorModeAuto && !log.Redirected()) {
		Colors = true
	}

	err = b.setCompareDefaults(ctx)
	if err != nil {
		return err
	}

	b.setResyncDefaults()

	err = b.setResolveDefaults(ctx)
	if err != nil {
		return err
	}

	if b.workDir, err = filepath.Abs(opt.Workdir); err != nil {
		return fmt.Errorf("failed to make workdir absolute: %w", err)
	}
	if err = os.MkdirAll(b.workDir, os.ModePerm); err != nil {
		return fmt.Errorf("failed to create workdir: %w", err)
	}

	// Produce a unique name for the sync operation
	b.basePath = bilib.BasePath(ctx, b.workDir, b.fs1, b.fs2)
	b.listing1 = b.basePath + ".path1.lst"
	b.listing2 = b.basePath + ".path2.lst"
	b.newListing1 = b.listing1 + "-new"
	b.newListing2 = b.listing2 + "-new"
	b.aliases = bilib.AliasMap{}

	err = b.checkSyntax()
	if err != nil {
		return err
	}

	// Handle lock file
	err = b.setLockFile()
	if err != nil {
		return err
	}

	// Handle SIGINT
	var finaliseOnce gosync.Once

	// waitFor runs fn() until it returns true or the timeout expires
	waitFor := func(msg string, totalWait time.Duration, fn func() bool) (ok bool) {
		const individualWait = 1 * time.Second
		for i := 0; i < int(totalWait/individualWait); i++ {
			ok = fn()
			if ok {
				return ok
			}
			fs.Infof(nil, Color(terminal.YellowFg, "%s: %v"), msg, int(totalWait/individualWait)-i)
			time.Sleep(individualWait)
		}
		return false
	}
	finalise := func() {
		finaliseOnce.Do(func() {
			if atexit.Signalled() {
				if b.opt.Resync {
					fs.Logf(nil, Color(terminal.GreenFg, "No need to gracefully shutdown during --resync (just run it again.)"))
				} else {
					fs.Logf(nil, Color(terminal.YellowFg, "Attempting to gracefully shutdown. (Send exit signal again for immediate un-graceful shutdown.)"))
					b.InGracefulShutdown = true
					if b.SyncCI != nil {
						fs.Infof(nil, Color(terminal.YellowFg, "Telling Sync to wrap up early."))
						b.SyncCI.MaxTransfer = 1
						b.SyncCI.MaxDuration = 1 * time.Second
						b.SyncCI.CutoffMode = fs.CutoffModeSoft
						gracePeriod := 30 * time.Second // TODO: flag to customize this?
						if !waitFor("Canceling Sync if not done in", gracePeriod, func() bool { return b.CleanupCompleted }) {
							fs.Logf(nil, Color(terminal.YellowFg, "Canceling sync and cleaning up"))
							b.CancelSync()
							waitFor("Aborting Bisync if not done in", 60*time.Second, func() bool { return b.CleanupCompleted })
						}
					} else {
						// we haven't started to sync yet, so we're good.
						// no need to worry about the listing files, as we haven't overwritten them yet.
						b.CleanupCompleted = true
						fs.Logf(nil, Color(terminal.GreenFg, "Graceful shutdown completed successfully."))
					}
				}
				if !b.CleanupCompleted {
					if !b.opt.Resync {
						fs.Logf(nil, Color(terminal.HiRedFg, "Graceful shutdown failed."))
						fs.Logf(nil, Color(terminal.RedFg, "Bisync interrupted. Must run --resync to recover."))
					}
					markFailed(b.listing1)
					markFailed(b.listing2)
				}
				b.removeLockFile()
			}
		})
	}
	fnHandle := atexit.Register(finalise)
	defer atexit.Unregister(fnHandle)

	// run bisync
	err = b.runLocked(ctx)

	b.removeLockFile()

	b.CleanupCompleted = true
	if b.InGracefulShutdown {
		if err == context.Canceled || err == accounting.ErrorMaxTransferLimitReachedGraceful {
			err = nil
			b.critical = false
		}
		if err == nil {
			fs.Logf(nil, Color(terminal.GreenFg, "Graceful shutdown completed successfully."))
		}
	}

	if b.critical {
		if b.retryable && b.opt.Resilient {
			fs.Errorf(nil, Color(terminal.RedFg, "Bisync critical error: %v"), err)
			fs.Errorf(nil, Color(terminal.YellowFg, "Bisync aborted. Error is retryable without --resync due to --resilient mode."))
		} else {
			if bilib.FileExists(b.listing1) {
				_ = os.Rename(b.listing1, b.listing1+"-err")
			}
			if bilib.FileExists(b.listing2) {
				_ = os.Rename(b.listing2, b.listing2+"-err")
			}
			fs.Errorf(nil, Color(terminal.RedFg, "Bisync critical error: %v"), err)
			fs.Errorf(nil, Color(terminal.RedFg, "Bisync aborted. Must run --resync to recover."))
		}
		return ErrBisyncAborted
	}
	if b.abort && !b.InGracefulShutdown {
		fs.Logf(nil, Color(terminal.RedFg, "Bisync aborted. Please try again."))
	}
	if err == nil {
		fs.Infof(nil, Color(terminal.GreenFg, "Bisync successful"))
	}
	return err
}

// runLocked performs a full bisync run
func (b *bisyncRun) runLocked(octx context.Context) (err error) {
	opt := b.opt
	path1 := bilib.FsPath(b.fs1)
	path2 := bilib.FsPath(b.fs2)

	if opt.CheckSync == CheckSyncOnly {
		fs.Infof(nil, "Validating listings for Path1 %s vs Path2 %s", quotePath(path1), quotePath(path2))
		if err = b.checkSync(b.listing1, b.listing2); err != nil {
			b.critical = true
			b.retryable = true
		}
		return err
	}

	fs.Infof(nil, "Synching Path1 %s with Path2 %s", quotePath(path1), quotePath(path2))

	if opt.DryRun {
		// In --dry-run mode, preserve original listings and save updates to the .lst-dry files
		origListing1 := b.listing1
		origListing2 := b.listing2
		b.listing1 += "-dry"
		b.listing2 += "-dry"
		b.newListing1 = b.listing1 + "-new"
		b.newListing2 = b.listing2 + "-new"
		if err := bilib.CopyFileIfExists(origListing1, b.listing1); err != nil {
			return err
		}
		if err := bilib.CopyFileIfExists(origListing2, b.listing2); err != nil {
			return err
		}
	}

	// Create second context with filters
	var fctx context.Context
	if fctx, err = b.opt.applyFilters(octx); err != nil {
		b.critical = true
		b.retryable = true
		return
	}
	b.octx = octx
	b.fctx = fctx

	// overlapping paths check
	err = b.overlappingPathsCheck(fctx, b.fs1, b.fs2)
	if err != nil {
		b.critical = true
		b.retryable = true
		return err
	}

	// Generate Path1 and Path2 listings and copy any unique Path2 files to Path1
	if opt.Resync {
		return b.resync(octx, fctx)
	}

	// Check for existence of prior Path1 and Path2 listings
	if !bilib.FileExists(b.listing1) || !bilib.FileExists(b.listing2) {
		if b.opt.Recover && bilib.FileExists(b.listing1+"-old") && bilib.FileExists(b.listing2+"-old") {
			errTip := fmt.Sprintf(Color(terminal.CyanFg, "Path1: %s\n"), Color(terminal.HiBlueFg, b.listing1))
			errTip += fmt.Sprintf(Color(terminal.CyanFg, "Path2: %s"), Color(terminal.HiBlueFg, b.listing2))
			fs.Logf(nil, Color(terminal.YellowFg, "Listings not found. Reverting to prior backup as --recover is set. \n")+errTip)
			if opt.CheckSync != CheckSyncFalse {
				// Run CheckSync to ensure old listing is valid (garbage in, garbage out!)
				fs.Infof(nil, "Validating backup listings for Path1 %s vs Path2 %s", quotePath(path1), quotePath(path2))
				if err = b.checkSync(b.listing1+"-old", b.listing2+"-old"); err != nil {
					b.critical = true
					b.retryable = true
					return err
				}
				fs.Infof(nil, Color(terminal.GreenFg, "Backup listing is valid."))
			}
			b.revertToOldListings()
		} else {
			// On prior critical error abort, the prior listings are renamed to .lst-err to lock out further runs
			b.critical = true
			b.retryable = true
			errTip := Color(terminal.MagentaFg, "Tip: here are the filenames we were looking for. Do they exist? \n")
			errTip += fmt.Sprintf(Color(terminal.CyanFg, "Path1: %s\n"), Color(terminal.HiBlueFg, b.listing1))
			errTip += fmt.Sprintf(Color(terminal.CyanFg, "Path2: %s\n"), Color(terminal.HiBlueFg, b.listing2))
			errTip += Color(terminal.MagentaFg, "Try running this command to inspect the work dir: \n")
			errTip += fmt.Sprintf(Color(terminal.HiCyanFg, "rclone lsl \"%s\""), b.workDir)

			return errors.New("cannot find prior Path1 or Path2 listings, likely due to critical error on prior run \n" + errTip)
		}
	}

	fs.Infof(nil, "Building Path1 and Path2 listings")
	ls1, ls2, err = b.makeMarchListing(fctx)
	if err != nil || accounting.Stats(fctx).Errored() {
		fs.Errorf(nil, Color(terminal.RedFg, "There were errors while building listings. Aborting as it is too dangerous to continue."))
		b.critical = true
		b.retryable = true
		return err
	}

	// Check for Path1 deltas relative to the prior sync
	fs.Infof(nil, "Path1 checking for diffs")
	ds1, err := b.findDeltas(fctx, b.fs1, b.listing1, ls1, "Path1")
	if err != nil {
		return err
	}
	ds1.printStats()

	// Check for Path2 deltas relative to the prior sync
	fs.Infof(nil, "Path2 checking for diffs")
	ds2, err := b.findDeltas(fctx, b.fs2, b.listing2, ls2, "Path2")
	if err != nil {
		return err
	}
	ds2.printStats()

	// Check access health on the Path1 and Path2 filesystems
	if opt.CheckAccess {
		fs.Infof(nil, "Checking access health")
		err = b.checkAccess(ds1.checkFiles, ds2.checkFiles)
		if err != nil {
			b.critical = true
			b.retryable = true
			return
		}
	}

	// Check for too many deleted files - possible error condition.
	// Don't want to start deleting on the other side!
	if !opt.Force {
		if ds1.excessDeletes() || ds2.excessDeletes() {
			b.abort = true
			return errors.New("too many deletes")
		}
	}

	// Check for all files changed such as all dates changed due to DST change
	// to avoid errant copy everything.
	if !opt.Force {
		msg := "Safety abort: all files were changed on %s %s. Run with --force if desired."
		if !ds1.foundSame {
			fs.Errorf(nil, msg, ds1.msg, quotePath(path1))
		}
		if !ds2.foundSame {
			fs.Errorf(nil, msg, ds2.msg, quotePath(path2))
		}
		if !ds1.foundSame || !ds2.foundSame {
			b.abort = true
			return errors.New("all files were changed")
		}
	}

	// Determine and apply changes to Path1 and Path2
	noChanges := ds1.empty() && ds2.empty()
	changes1 := false // 2to1
	changes2 := false // 1to2
	results2to1 := []Results{}
	results1to2 := []Results{}

	queues := queues{}

	if noChanges {
		fs.Infof(nil, "No changes found")
	} else {
		fs.Infof(nil, "Applying changes")
		changes1, changes2, results2to1, results1to2, queues, err = b.applyDeltas(octx, ds1, ds2)
		if err != nil {
			if b.InGracefulShutdown && (err == context.Canceled || err == accounting.ErrorMaxTransferLimitReachedGraceful || strings.Contains(err.Error(), "context canceled")) {
				fs.Infof(nil, "Ignoring sync error due to Graceful Shutdown: %v", err)
			} else {
				b.critical = true
				// b.retryable = true // not sure about this one
				return err
			}
		}
	}

	// Clean up and check listings integrity
	fs.Infof(nil, "Updating listings")
	var err1, err2 error
	if b.DebugName != "" {
		l1, _ := b.loadListing(b.listing1)
		l2, _ := b.loadListing(b.listing2)
		newl1, _ := b.loadListing(b.newListing1)
		newl2, _ := b.loadListing(b.newListing2)
		b.debug(b.DebugName, fmt.Sprintf("pre-saveOldListings, ls1 has name?: %v, ls2 has name?: %v", l1.has(b.DebugName), l2.has(b.DebugName)))
		b.debug(b.DebugName, fmt.Sprintf("pre-saveOldListings, newls1 has name?: %v, newls2 has name?: %v", newl1.has(b.DebugName), newl2.has(b.DebugName)))
	}
	b.saveOldListings()
	// save new listings
	// NOTE: "changes" in this case does not mean this run vs. last run, it means start of this run vs. end of this run.
	// i.e. whether we can use the March lst-new as this side's lst without modifying it.
	if noChanges {
		b.replaceCurrentListings()
	} else {
		if changes1 || b.InGracefulShutdown { // 2to1
			err1 = b.modifyListing(fctx, b.fs2, b.fs1, results2to1, queues, false)
		} else {
			err1 = bilib.CopyFileIfExists(b.newListing1, b.listing1)
		}
		if changes2 || b.InGracefulShutdown { // 1to2
			err2 = b.modifyListing(fctx, b.fs1, b.fs2, results1to2, queues, true)
		} else {
			err2 = bilib.CopyFileIfExists(b.newListing2, b.listing2)
		}
	}
	if b.DebugName != "" {
		l1, _ := b.loadListing(b.listing1)
		l2, _ := b.loadListing(b.listing2)
		b.debug(b.DebugName, fmt.Sprintf("post-modifyListing, ls1 has name?: %v, ls2 has name?: %v", l1.has(b.DebugName), l2.has(b.DebugName)))
	}
	err = err1
	if err == nil {
		err = err2
	}
	if err != nil {
		b.critical = true
		b.retryable = true
		return err
	}

	if !opt.NoCleanup {
		_ = os.Remove(b.newListing1)
		_ = os.Remove(b.newListing2)
	}

	if opt.CheckSync == CheckSyncTrue && !opt.DryRun {
		fs.Infof(nil, "Validating listings for Path1 %s vs Path2 %s", quotePath(path1), quotePath(path2))
		if err := b.checkSync(b.listing1, b.listing2); err != nil {
			b.critical = true
			return err
		}
	}

	// Optional rmdirs for empty directories
	if opt.RemoveEmptyDirs {
		fs.Infof(nil, "Removing empty directories")
		fctx = b.setBackupDir(fctx, 1)
		err1 := operations.Rmdirs(fctx, b.fs1, "", true)
		fctx = b.setBackupDir(fctx, 2)
		err2 := operations.Rmdirs(fctx, b.fs2, "", true)
		err := err1
		if err == nil {
			err = err2
		}
		if err != nil {
			b.critical = true
			b.retryable = true
			return err
		}
	}

	return nil
}

// checkSync validates listings
func (b *bisyncRun) checkSync(listing1, listing2 string) error {
	files1, err := b.loadListing(listing1)
	if err != nil {
		return fmt.Errorf("cannot read prior listing of Path1: %w", err)
	}
	files2, err := b.loadListing(listing2)
	if err != nil {
		return fmt.Errorf("cannot read prior listing of Path2: %w", err)
	}

	ok := true
	for _, file := range files1.list {
		if !files2.has(file) && !files2.has(b.aliases.Alias(file)) {
			b.indent("ERROR", file, "Path1 file not found in Path2")
			ok = false
		} else {
			if !b.fileInfoEqual(file, files2.getTryAlias(file, b.aliases.Alias(file)), files1, files2) {
				ok = false
			}
		}
	}
	for _, file := range files2.list {
		if !files1.has(file) && !files1.has(b.aliases.Alias(file)) {
			b.indent("ERROR", file, "Path2 file not found in Path1")
			ok = false
		}
	}

	if !ok {
		return errors.New("path1 and path2 are out of sync, run --resync to recover")
	}
	return nil
}

// checkAccess validates access health
func (b *bisyncRun) checkAccess(checkFiles1, checkFiles2 bilib.Names) error {
	ok := true
	opt := b.opt
	prefix := "Access test failed:"

	numChecks1 := len(checkFiles1)
	numChecks2 := len(checkFiles2)
	if numChecks1 == 0 || numChecks1 != numChecks2 {
		if numChecks1 == 0 && numChecks2 == 0 {
			fs.Logf("--check-access", Color(terminal.RedFg, "Failed to find any files named %s\n More info: %s"), Color(terminal.CyanFg, opt.CheckFilename), Color(terminal.BlueFg, "https://rclone.org/bisync/#check-access"))
		}
		fs.Errorf(nil, "%s Path1 count %d, Path2 count %d - %s", prefix, numChecks1, numChecks2, opt.CheckFilename)
		ok = false
	}

	for file := range checkFiles1 {
		if !checkFiles2.Has(file) {
			b.indentf("ERROR", file, "%s Path1 file not found in Path2", prefix)
			ok = false
		}
	}

	for file := range checkFiles2 {
		if !checkFiles1.Has(file) {
			b.indentf("ERROR", file, "%s Path2 file not found in Path1", prefix)
			ok = false
		}
	}

	if !ok {
		return errors.New("check file check failed")
	}
	fs.Infof(nil, "Found %d matching %q files on both paths", numChecks1, opt.CheckFilename)
	return nil
}

func (b *bisyncRun) testFn() {
	if b.opt.TestFn != nil {
		b.opt.TestFn()
	}
}

func (b *bisyncRun) handleErr(o interface{}, msg string, err error, critical, retryable bool) {
	if err != nil {
		if retryable {
			b.retryable = true
		}
		if critical {
			b.critical = true
			b.abort = true
			fs.Errorf(o, "%s: %v", msg, err)
		} else {
			fs.Infof(o, "%s: %v", msg, err)
		}
	}
}

// setBackupDir overrides --backup-dir with path-specific version, if set, in each direction
func (b *bisyncRun) setBackupDir(ctx context.Context, destPath int) context.Context {
	ci := fs.GetConfig(ctx)
	ci.BackupDir = b.opt.OrigBackupDir
	if destPath == 1 && b.opt.BackupDir1 != "" {
		ci.BackupDir = b.opt.BackupDir1
	}
	if destPath == 2 && b.opt.BackupDir2 != "" {
		ci.BackupDir = b.opt.BackupDir2
	}
	fs.Debugf(ci.BackupDir, "updated backup-dir for Path%d", destPath)
	return ctx
}

func (b *bisyncRun) overlappingPathsCheck(fctx context.Context, fs1, fs2 fs.Fs) error {
	if operations.OverlappingFilterCheck(fctx, fs2, fs1) {
		err = fmt.Errorf(Color(terminal.RedFg, "Overlapping paths detected. Cannot bisync between paths that overlap, unless excluded by filters."))
		return err
	}
	// need to test our BackupDirs too, as sync will be fooled by our --files-from filters
	testBackupDir := func(ctx context.Context, destPath int) error {
		src := fs1
		dst := fs2
		if destPath == 1 {
			src = fs2
			dst = fs1
		}
		ctxBackupDir := b.setBackupDir(ctx, destPath)
		ci := fs.GetConfig(ctxBackupDir)
		if ci.BackupDir != "" {
			// operations.BackupDir should return an error if not properly excluded
			_, err = operations.BackupDir(fctx, dst, src, "")
			return err
		}
		return nil
	}
	err = testBackupDir(fctx, 1)
	if err != nil {
		return err
	}
	err = testBackupDir(fctx, 2)
	if err != nil {
		return err
	}
	return nil
}

func (b *bisyncRun) checkSyntax() error {
	// check for odd number of quotes in path, usually indicating an escaping issue
	path1 := bilib.FsPath(b.fs1)
	path2 := bilib.FsPath(b.fs2)
	if strings.Count(path1, `"`)%2 != 0 || strings.Count(path2, `"`)%2 != 0 {
		return fmt.Errorf(Color(terminal.RedFg, `detected an odd number of quotes in your path(s). This is usually a mistake indicating incorrect escaping.
			 Please check your command and try again. Note that on Windows, quoted paths must not have a trailing slash, or it will be interpreted as escaping the quote. path1: %v path2: %v`), path1, path2)
	}
	// check for other syntax issues
	_, err = os.Stat(b.basePath)
	if err != nil {
		if strings.Contains(err.Error(), "syntax is incorrect") {
			return fmt.Errorf(Color(terminal.RedFg, `syntax error detected in your path(s). Please check your command and try again.
				 Note that on Windows, quoted paths must not have a trailing slash, or it will be interpreted as escaping the quote. path1: %v path2: %v error: %v`), path1, path2, err)
		}
	}
	if runtime.GOOS == "windows" && (strings.Contains(path1, " --") || strings.Contains(path2, " --")) {
		return fmt.Errorf(Color(terminal.RedFg, `detected possible flags in your path(s). This is usually a mistake indicating incorrect escaping or quoting (possibly closing quote is missing?).
			 Please check your command and try again. Note that on Windows, quoted paths must not have a trailing slash, or it will be interpreted as escaping the quote. path1: %v path2: %v`), path1, path2)
	}
	return nil
}

func (b *bisyncRun) debug(nametocheck, msgiftrue string) {
	if b.DebugName != "" && b.DebugName == nametocheck {
		fs.Infof(Color(terminal.MagentaBg, "DEBUGNAME "+b.DebugName), Color(terminal.MagentaBg, msgiftrue))
	}
}

func (b *bisyncRun) debugFn(nametocheck string, fn func()) {
	if b.DebugName != "" && b.DebugName == nametocheck {
		fn()
	}
}

// mainly to make sure tests don't interfere with each other when running more than one
func resetGlobals() {
	downloadHash = false
	logger = operations.NewLoggerOpt()
	ignoreListingChecksum = false
	ignoreListingModtime = false
	hashTypes = nil
	queueCI = nil
	hashType = 0
	fsrc, fdst = nil, nil
	fcrypt = nil
	Opt = Options{}
	once = gosync.Once{}
	downloadHashWarn = gosync.Once{}
	firstDownloadHash = gosync.Once{}
	ls1 = newFileList()
	ls2 = newFileList()
	err = nil
	firstErr = nil
	marchCtx = nil
}
