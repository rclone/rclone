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
	"strconv"
	gosync "sync"

	"github.com/rclone/rclone/cmd/bisync/bilib"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/filter"
	"github.com/rclone/rclone/fs/operations"
	"github.com/rclone/rclone/lib/atexit"
	"github.com/rclone/rclone/lib/terminal"
)

// ErrBisyncAborted signals that bisync is aborted and forces exit code 2
var ErrBisyncAborted = errors.New("bisync aborted")

// bisyncRun keeps bisync runtime state
type bisyncRun struct {
	fs1         fs.Fs
	fs2         fs.Fs
	abort       bool
	critical    bool
	retryable   bool
	basePath    string
	workDir     string
	listing1    string
	listing2    string
	newListing1 string
	newListing2 string
	aliases     bilib.AliasMap
	opt         *Options
	octx        context.Context
	fctx        context.Context
}

type queues struct {
	copy1to2      bilib.Names
	copy2to1      bilib.Names
	renamed1      bilib.Names // renamed on 1 and copied to 2
	renamed2      bilib.Names // renamed on 2 and copied to 1
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
		fs1: fs1,
		fs2: fs2,
		opt: &opt,
	}

	if opt.CheckFilename == "" {
		opt.CheckFilename = DefaultCheckFilename
	}
	if opt.Workdir == "" {
		opt.Workdir = DefaultWorkdir
	}
	ci := fs.GetConfig(ctx)
	opt.OrigBackupDir = ci.BackupDir

	err = b.setCompareDefaults(ctx)
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

	// Handle lock file
	lockFile := ""
	if !opt.DryRun {
		lockFile = b.basePath + ".lck"
		if bilib.FileExists(lockFile) {
			errTip := Color(terminal.MagentaFg, "Tip: this indicates that another bisync run (of these same paths) either is still running or was interrupted before completion. \n")
			errTip += Color(terminal.MagentaFg, "If you're SURE you want to override this safety feature, you can delete the lock file with the following command, then run bisync again: \n")
			errTip += fmt.Sprintf(Color(terminal.HiRedFg, "rclone deletefile \"%s\""), lockFile)
			return fmt.Errorf(Color(terminal.RedFg, "prior lock file found: %s \n")+errTip, Color(terminal.HiYellowFg, lockFile))
		}

		pidStr := []byte(strconv.Itoa(os.Getpid()))
		if err = os.WriteFile(lockFile, pidStr, bilib.PermSecure); err != nil {
			return fmt.Errorf("cannot create lock file: %s: %w", lockFile, err)
		}
		fs.Debugf(nil, "Lock file created: %s", lockFile)
	}

	// Handle SIGINT
	var finaliseOnce gosync.Once
	markFailed := func(file string) {
		failFile := file + "-err"
		if bilib.FileExists(file) {
			_ = os.Remove(failFile)
			_ = os.Rename(file, failFile)
		}
	}
	finalise := func() {
		finaliseOnce.Do(func() {
			if atexit.Signalled() {
				fs.Logf(nil, "Bisync interrupted. Must run --resync to recover.")
				markFailed(b.listing1)
				markFailed(b.listing2)
				_ = os.Remove(lockFile)
			}
		})
	}
	fnHandle := atexit.Register(finalise)
	defer atexit.Unregister(fnHandle)

	// run bisync
	err = b.runLocked(ctx)

	if lockFile != "" {
		errUnlock := os.Remove(lockFile)
		if errUnlock == nil {
			fs.Debugf(nil, "Lock file removed: %s", lockFile)
		} else if err == nil {
			err = errUnlock
		} else {
			fs.Errorf(nil, "cannot remove lockfile %s: %v", lockFile, errUnlock)
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
	if b.abort {
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

	// Generate Path1 and Path2 listings and copy any unique Path2 files to Path1
	if opt.Resync {
		return b.resync(octx, fctx)
	}

	// Check for existence of prior Path1 and Path2 listings
	if !bilib.FileExists(b.listing1) || !bilib.FileExists(b.listing2) {
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

	fs.Infof(nil, "Building Path1 and Path2 listings")
	ls1, ls2, err = b.makeMarchListing(fctx)
	if err != nil {
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
			b.critical = true
			// b.retryable = true // not sure about this one
			return err
		}
	}

	// Clean up and check listings integrity
	fs.Infof(nil, "Updating listings")
	var err1, err2 error
	b.saveOldListings()
	// save new listings
	if noChanges {
		b.replaceCurrentListings()
	} else {
		if changes1 { // 2to1
			err1 = b.modifyListing(fctx, b.fs2, b.fs1, results2to1, queues, false)
		} else {
			err1 = bilib.CopyFileIfExists(b.newListing1, b.listing1)
		}
		if changes2 { // 1to2
			err2 = b.modifyListing(fctx, b.fs1, b.fs2, results1to2, queues, true)
		} else {
			err2 = bilib.CopyFileIfExists(b.newListing2, b.listing2)
		}
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

// resync implements the --resync mode.
// It will generate path1 and path2 listings
// and copy any unique path2 files to path1.
func (b *bisyncRun) resync(octx, fctx context.Context) error {
	fs.Infof(nil, "Copying unique Path2 files to Path1")

	// Save blank filelists (will be filled from sync results)
	var ls1 = newFileList()
	var ls2 = newFileList()
	err = ls1.save(fctx, b.newListing1)
	if err != nil {
		b.handleErr(ls1, "error saving ls1 from resync", err, true, true)
		b.abort = true
	}
	err = ls2.save(fctx, b.newListing2)
	if err != nil {
		b.handleErr(ls2, "error saving ls2 from resync", err, true, true)
		b.abort = true
	}

	// Check access health on the Path1 and Path2 filesystems
	// enforce even though this is --resync
	if b.opt.CheckAccess {
		fs.Infof(nil, "Checking access health")

		filesNow1, filesNow2, err := b.findCheckFiles(fctx)
		if err != nil {
			b.critical = true
			b.retryable = true
			return err
		}

		ds1 := &deltaSet{
			checkFiles: bilib.Names{},
		}

		ds2 := &deltaSet{
			checkFiles: bilib.Names{},
		}

		for _, file := range filesNow1.list {
			if filepath.Base(file) == b.opt.CheckFilename {
				ds1.checkFiles.Add(file)
			}
		}

		for _, file := range filesNow2.list {
			if filepath.Base(file) == b.opt.CheckFilename {
				ds2.checkFiles.Add(file)
			}
		}

		err = b.checkAccess(ds1.checkFiles, ds2.checkFiles)
		if err != nil {
			b.critical = true
			b.retryable = true
			return err
		}
	}

	var results2to1 []Results
	var results1to2 []Results
	queues := queues{}

	b.indent("Path2", "Path1", "Resync is copying UNIQUE files to")
	ctxRun := b.opt.setDryRun(fctx)
	// fctx has our extra filters added!
	ctxSync, filterSync := filter.AddConfig(ctxRun)
	if filterSync.Opt.MinSize == -1 {
		fs.Debugf(nil, "filterSync.Opt.MinSize: %v", filterSync.Opt.MinSize)
	}
	ci := fs.GetConfig(ctxSync)
	ci.IgnoreExisting = true
	ctxSync = b.setBackupDir(ctxSync, 1)
	// 2 to 1
	if results2to1, err = b.resyncDir(ctxSync, b.fs2, b.fs1); err != nil {
		b.critical = true
		return err
	}

	b.indent("Path1", "Path2", "Resync is copying UNIQUE OR DIFFERING files to")
	ci.IgnoreExisting = false
	ctxSync = b.setBackupDir(ctxSync, 2)
	// 1 to 2
	if results1to2, err = b.resyncDir(ctxSync, b.fs1, b.fs2); err != nil {
		b.critical = true
		return err
	}

	fs.Infof(nil, "Resync updating listings")
	b.saveOldListings() // may not exist, as this is --resync
	b.replaceCurrentListings()

	resultsToQueue := func(results []Results) bilib.Names {
		names := bilib.Names{}
		for _, result := range results {
			if result.Name != "" &&
				(result.Flags != "d" || b.opt.CreateEmptySrcDirs) &&
				result.IsSrc && result.Src != "" &&
				(result.Winner.Err == nil || result.Flags == "d") {
				names.Add(result.Name)
			}
		}
		return names
	}

	// resync 2to1
	queues.copy2to1 = resultsToQueue(results2to1)
	if err = b.modifyListing(fctx, b.fs2, b.fs1, results2to1, queues, false); err != nil {
		b.critical = true
		return err
	}

	// resync 1to2
	queues.copy1to2 = resultsToQueue(results1to2)
	if err = b.modifyListing(fctx, b.fs1, b.fs2, results1to2, queues, true); err != nil {
		b.critical = true
		return err
	}

	if !b.opt.NoCleanup {
		_ = os.Remove(b.newListing1)
		_ = os.Remove(b.newListing2)
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
		ci.BackupDir = b.opt.BackupDir1
	}
	fs.Debugf(ci.BackupDir, "updated backup-dir for Path%d", destPath)
	return ctx
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
