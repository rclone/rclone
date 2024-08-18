package bisync

import (
	"context"
	"os"
	"path/filepath"

	"github.com/rclone/rclone/cmd/bisync/bilib"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/filter"
	"github.com/rclone/rclone/lib/terminal"
)

// for backward compatibility, --resync is now equivalent to --resync-mode path1
// and either flag is sufficient without the other.
func (b *bisyncRun) setResyncDefaults() {
	if b.opt.Resync && b.opt.ResyncMode == PreferNone {
		fs.Debugf(nil, Color(terminal.Dim, "defaulting to --resync-mode path1 as --resync is set")) //nolint:govet
		b.opt.ResyncMode = PreferPath1
	}
	if b.opt.ResyncMode != PreferNone {
		b.opt.Resync = true
		Opt.Resync = true // shouldn't be using this one, but set to be safe
	}

	// checks and warnings
	if (b.opt.ResyncMode == PreferNewer || b.opt.ResyncMode == PreferOlder) && (b.fs1.Precision() == fs.ModTimeNotSupported || b.fs2.Precision() == fs.ModTimeNotSupported) {
		fs.Logf(nil, Color(terminal.YellowFg, "WARNING: ignoring --resync-mode %s as at least one remote does not support modtimes."), b.opt.ResyncMode.String())
		b.opt.ResyncMode = PreferPath1
	} else if (b.opt.ResyncMode == PreferNewer || b.opt.ResyncMode == PreferOlder) && !b.opt.Compare.Modtime {
		fs.Logf(nil, Color(terminal.YellowFg, "WARNING: ignoring --resync-mode %s as --compare does not include modtime."), b.opt.ResyncMode.String())
		b.opt.ResyncMode = PreferPath1
	}
	if (b.opt.ResyncMode == PreferLarger || b.opt.ResyncMode == PreferSmaller) && !b.opt.Compare.Size {
		fs.Logf(nil, Color(terminal.YellowFg, "WARNING: ignoring --resync-mode %s as --compare does not include size."), b.opt.ResyncMode.String())
		b.opt.ResyncMode = PreferPath1
	}
}

// resync implements the --resync mode.
// It will generate path1 and path2 listings,
// copy any unique files to the opposite path,
// and resolve any differing files according to the --resync-mode.
func (b *bisyncRun) resync(octx, fctx context.Context) error {
	fs.Infof(nil, "Copying Path2 files to Path1")

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

	b.indent("Path2", "Path1", "Resync is copying files to")
	ctxRun := b.opt.setDryRun(fctx)
	// fctx has our extra filters added!
	ctxSync, filterSync := filter.AddConfig(ctxRun)
	if filterSync.Opt.MinSize == -1 {
		fs.Debugf(nil, "filterSync.Opt.MinSize: %v", filterSync.Opt.MinSize)
	}
	b.resyncIs1to2 = false
	ctxSync = b.setResyncConfig(ctxSync)
	ctxSync = b.setBackupDir(ctxSync, 1)
	// 2 to 1
	if results2to1, err = b.resyncDir(ctxSync, b.fs2, b.fs1); err != nil {
		b.critical = true
		return err
	}

	b.indent("Path1", "Path2", "Resync is copying files to")
	b.resyncIs1to2 = true
	ctxSync = b.setResyncConfig(ctxSync)
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

	if b.opt.CheckSync == CheckSyncTrue && !b.opt.DryRun {
		path1 := bilib.FsPath(b.fs1)
		path2 := bilib.FsPath(b.fs2)
		fs.Infof(nil, "Validating listings for Path1 %s vs Path2 %s", quotePath(path1), quotePath(path2))
		if err := b.checkSync(b.listing1, b.listing2); err != nil {
			b.critical = true
			return err
		}
	}

	if !b.opt.NoCleanup {
		_ = os.Remove(b.newListing1)
		_ = os.Remove(b.newListing2)
	}
	return nil
}

/*
	 --resync-mode implementation:
		PreferPath1: set ci.IgnoreExisting true, then false
		PreferPath2: set ci.IgnoreExisting false, then true
		PreferNewer: set ci.UpdateOlder in both directions
		PreferOlder: override EqualFn to implement custom logic
		PreferLarger: override EqualFn to implement custom logic
		PreferSmaller: override EqualFn to implement custom logic
*/
func (b *bisyncRun) setResyncConfig(ctx context.Context) context.Context {
	ci := fs.GetConfig(ctx)
	switch b.opt.ResyncMode {
	case PreferPath1:
		if !b.resyncIs1to2 { // 2to1 (remember 2to1 is first)
			ci.IgnoreExisting = true
		} else { // 1to2
			ci.IgnoreExisting = false
		}
	case PreferPath2:
		if !b.resyncIs1to2 { // 2to1 (remember 2to1 is first)
			ci.IgnoreExisting = false
		} else { // 1to2
			ci.IgnoreExisting = true
		}
	case PreferNewer:
		ci.UpdateOlder = true
	}
	// for older, larger, and smaller, we return it unchanged and handle it later
	return ctx
}

func (b *bisyncRun) resyncWhichIsWhich(src, dst fs.ObjectInfo) (path1, path2 fs.ObjectInfo) {
	if b.resyncIs1to2 {
		return src, dst
	}
	return dst, src
}

// equal in this context really means "don't transfer", so we should
// return true if the files are actually equal or if dest is winner,
// false if src is winner
// When can't determine, we end up running the normal Equal() to tie-break (due to our differ functions).
func (b *bisyncRun) resyncWinningPathToEqual(winningPath int) bool {
	if b.resyncIs1to2 {
		return winningPath != 1
	}
	return winningPath != 2
}
