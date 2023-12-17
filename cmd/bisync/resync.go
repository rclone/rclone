package bisync

import (
	"context"
	"os"
	"path/filepath"

	"github.com/rclone/rclone/cmd/bisync/bilib"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/filter"
)

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
