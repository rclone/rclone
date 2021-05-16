// Package bisync implements bisync
// Copyright (c) 2017-2020 Chris Nelson
// Contributions to original python version: Hildo G. Jr., e2t, kalemas, silenceleaf
package bisync

import (
	"context"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	gosync "sync"

	"github.com/pkg/errors"
	"github.com/rclone/rclone/cmd/bisync/bilib"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/filter"
	"github.com/rclone/rclone/fs/operations"
	"github.com/rclone/rclone/fs/sync"
	"github.com/rclone/rclone/lib/atexit"
)

// ErrBisyncAborted signals that bisync is aborted and forces exit code 2
var ErrBisyncAborted = errors.New("bisync aborted")

// bisyncRun keeps bisync runtime state
type bisyncRun struct {
	fs1      fs.Fs
	fs2      fs.Fs
	abort    bool
	critical bool
	basePath string
	workDir  string
	opt      *Options
}

// Bisync handles lock file, performs bisync run and checks exit status
func Bisync(ctx context.Context, fs1, fs2 fs.Fs, optArg *Options) (err error) {
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

	if !opt.DryRun && !opt.Force {
		if fs1.Precision() == fs.ModTimeNotSupported {
			return errors.New("modification time support is missing on path1")
		}
		if fs2.Precision() == fs.ModTimeNotSupported {
			return errors.New("modification time support is missing on path2")
		}
	}

	if b.workDir, err = filepath.Abs(opt.Workdir); err != nil {
		return errors.Wrap(err, "failed to make workdir absolute")
	}
	if err = os.MkdirAll(b.workDir, os.ModePerm); err != nil {
		return errors.Wrap(err, "failed to create workdir")
	}

	// Produce a unique name for the sync operation
	b.basePath = filepath.Join(b.workDir, bilib.SessionName(b.fs1, b.fs2))
	listing1 := b.basePath + ".path1.lst"
	listing2 := b.basePath + ".path2.lst"

	// Handle lock file
	lockFile := ""
	if !opt.DryRun {
		lockFile = b.basePath + ".lck"
		if bilib.FileExists(lockFile) {
			return errors.Errorf("prior lock file found: %s", lockFile)
		}

		pidStr := []byte(strconv.Itoa(os.Getpid()))
		if err = ioutil.WriteFile(lockFile, pidStr, bilib.PermSecure); err != nil {
			return errors.Wrapf(err, "cannot create lock file: %s", lockFile)
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
				markFailed(listing1)
				markFailed(listing2)
				_ = os.Remove(lockFile)
			}
		})
	}
	fnHandle := atexit.Register(finalise)
	defer atexit.Unregister(fnHandle)

	// run bisync
	err = b.runLocked(ctx, listing1, listing2)

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
		if bilib.FileExists(listing1) {
			_ = os.Rename(listing1, listing1+"-err")
		}
		if bilib.FileExists(listing2) {
			_ = os.Rename(listing2, listing2+"-err")
		}
		fs.Errorf(nil, "Bisync critical error: %v", err)
		fs.Errorf(nil, "Bisync aborted. Must run --resync to recover.")
		return ErrBisyncAborted
	}
	if b.abort {
		fs.Logf(nil, "Bisync aborted. Please try again.")
	}
	if err == nil {
		fs.Infof(nil, "Bisync successful")
	}
	return err
}

// runLocked performs a full bisync run
func (b *bisyncRun) runLocked(octx context.Context, listing1, listing2 string) (err error) {
	opt := b.opt
	path1 := bilib.FsPath(b.fs1)
	path2 := bilib.FsPath(b.fs2)

	if opt.CheckSync == CheckSyncOnly {
		fs.Infof(nil, "Validating listings for Path1 %s vs Path2 %s", quotePath(path1), quotePath(path2))
		if err = b.checkSync(listing1, listing2); err != nil {
			b.critical = true
		}
		return err
	}

	fs.Infof(nil, "Synching Path1 %s with Path2 %s", quotePath(path1), quotePath(path2))

	if opt.DryRun {
		// In --dry-run mode, preserve original listings and save updates to the .lst-dry files
		origListing1 := listing1
		origListing2 := listing2
		listing1 += "-dry"
		listing2 += "-dry"
		if err := bilib.CopyFileIfExists(origListing1, listing1); err != nil {
			return err
		}
		if err := bilib.CopyFileIfExists(origListing2, listing2); err != nil {
			return err
		}
	}

	// Create second context with filters
	var fctx context.Context
	if fctx, err = b.opt.applyFilters(octx); err != nil {
		b.critical = true
		return
	}

	// Generate Path1 and Path2 listings and copy any unique Path2 files to Path1
	if opt.Resync {
		return b.resync(octx, fctx, listing1, listing2)
	}

	// Check for existence of prior Path1 and Path2 listings
	if !bilib.FileExists(listing1) || !bilib.FileExists(listing2) {
		// On prior critical error abort, the prior listings are renamed to .lst-err to lock out further runs
		b.critical = true
		return errors.New("cannot find prior Path1 or Path2 listings, likely due to critical error on prior run")
	}

	// Check for Path1 deltas relative to the prior sync
	fs.Infof(nil, "Path1 checking for diffs")
	newListing1 := listing1 + "-new"
	ds1, err := b.findDeltas(fctx, b.fs1, listing1, newListing1, "Path1")
	if err != nil {
		return err
	}
	ds1.printStats()

	// Check for Path2 deltas relative to the prior sync
	fs.Infof(nil, "Path2 checking for diffs")
	newListing2 := listing2 + "-new"
	ds2, err := b.findDeltas(fctx, b.fs2, listing2, newListing2, "Path2")
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
	changes1 := false
	changes2 := false
	if noChanges {
		fs.Infof(nil, "No changes found")
	} else {
		fs.Infof(nil, "Applying changes")
		changes1, changes2, err = b.applyDeltas(octx, ds1, ds2)
		if err != nil {
			b.critical = true
			return err
		}
	}

	// Clean up and check listings integrity
	fs.Infof(nil, "Updating listings")
	var err1, err2 error
	if noChanges {
		err1 = bilib.CopyFileIfExists(newListing1, listing1)
		err2 = bilib.CopyFileIfExists(newListing2, listing2)
	} else {
		if changes1 {
			_, err1 = b.makeListing(fctx, b.fs1, listing1)
		} else {
			err1 = bilib.CopyFileIfExists(newListing1, listing1)
		}
		if changes2 {
			_, err2 = b.makeListing(fctx, b.fs2, listing2)
		} else {
			err2 = bilib.CopyFileIfExists(newListing2, listing2)
		}
	}
	err = err1
	if err == nil {
		err = err2
	}
	if err != nil {
		b.critical = true
		return err
	}

	if !opt.NoCleanup {
		_ = os.Remove(newListing1)
		_ = os.Remove(newListing2)
	}

	if opt.CheckSync == CheckSyncTrue && !opt.DryRun {
		fs.Infof(nil, "Validating listings for Path1 %s vs Path2 %s", quotePath(path1), quotePath(path2))
		if err := b.checkSync(listing1, listing2); err != nil {
			b.critical = true
			return err
		}
	}

	// Optional rmdirs for empty directories
	if opt.RemoveEmptyDirs {
		fs.Infof(nil, "Removing empty directories")
		err1 := operations.Rmdirs(fctx, b.fs1, "", true)
		err2 := operations.Rmdirs(fctx, b.fs2, "", true)
		err := err1
		if err == nil {
			err = err2
		}
		if err != nil {
			b.critical = true
			return err
		}
	}

	return nil
}

// resync implements the --resync mode.
// It will generate path1 and path2 listings
// and copy any unique path2 files to path1.
func (b *bisyncRun) resync(octx, fctx context.Context, listing1, listing2 string) error {
	fs.Infof(nil, "Copying unique Path2 files to Path1")

	newListing1 := listing1 + "-new"
	filesNow1, err := b.makeListing(fctx, b.fs1, newListing1)
	if err == nil {
		err = b.checkListing(filesNow1, newListing1, "current Path1")
	}
	if err != nil {
		return err
	}

	newListing2 := listing2 + "-new"
	filesNow2, err := b.makeListing(fctx, b.fs2, newListing2)
	if err == nil {
		err = b.checkListing(filesNow2, newListing2, "current Path2")
	}
	if err != nil {
		return err
	}

	copy2to1 := []string{}
	for _, file := range filesNow2.list {
		if !filesNow1.has(file) {
			b.indent("Path2", file, "Resync will copy to Path1")
			copy2to1 = append(copy2to1, file)
		}
	}

	if len(copy2to1) > 0 {
		b.indent("Path2", "Path1", "Resync is doing queued copies to")
		// octx does not have extra filters!
		err = b.fastCopy(octx, b.fs2, b.fs1, bilib.ToNames(copy2to1), "resync-copy2to1")
		if err != nil {
			b.critical = true
			return err
		}
	}

	fs.Infof(nil, "Resynching Path1 to Path2")
	ctxRun := b.opt.setDryRun(fctx)
	// fctx has our extra filters added!
	ctxSync, filterSync := filter.AddConfig(ctxRun)
	if filterSync.Opt.MinSize == -1 {
		// prevent overwriting Google Doc files (their size is -1)
		filterSync.Opt.MinSize = 0
	}
	if err = sync.Sync(ctxSync, b.fs2, b.fs1, false); err != nil {
		b.critical = true
		return err
	}

	fs.Infof(nil, "Resync updating listings")
	if _, err = b.makeListing(fctx, b.fs1, listing1); err != nil {
		b.critical = true
		return err
	}

	if _, err = b.makeListing(fctx, b.fs2, listing2); err != nil {
		b.critical = true
		return err
	}

	if !b.opt.NoCleanup {
		_ = os.Remove(newListing1)
		_ = os.Remove(newListing2)
	}
	return nil
}

// checkSync validates listings
func (b *bisyncRun) checkSync(listing1, listing2 string) error {
	files1, err := b.loadListing(listing1)
	if err != nil {
		return errors.Wrap(err, "cannot read prior listing of Path1")
	}
	files2, err := b.loadListing(listing2)
	if err != nil {
		return errors.Wrap(err, "cannot read prior listing of Path2")
	}

	ok := true
	for _, file := range files1.list {
		if !files2.has(file) {
			b.indent("ERROR", file, "Path1 file not found in Path2")
			ok = false
		}
	}
	for _, file := range files2.list {
		if !files1.has(file) {
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
