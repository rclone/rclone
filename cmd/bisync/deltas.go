// Package bisync implements bisync
// Copyright (c) 2017-2020 Chris Nelson
package bisync

import (
	"bytes"
	"context"
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/rclone/rclone/cmd/bisync/bilib"
	"github.com/rclone/rclone/cmd/check"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/accounting"
	"github.com/rclone/rclone/fs/filter"
	"github.com/rclone/rclone/fs/operations"
)

// delta
type delta uint8

const (
	deltaZero delta = 0
	deltaNew  delta = 1 << iota
	deltaNewer
	deltaOlder
	deltaSize
	deltaHash
	deltaDeleted
)

const (
	deltaModified delta = deltaNewer | deltaOlder | deltaSize | deltaHash | deltaDeleted
	deltaOther    delta = deltaNew | deltaNewer | deltaOlder
)

func (d delta) is(cond delta) bool {
	return d&cond != 0
}

// deltaSet
type deltaSet struct {
	deltas     map[string]delta
	opt        *Options
	fs         fs.Fs  // base filesystem
	msg        string // filesystem name for logging
	oldCount   int    // original number of files (for "excess deletes" check)
	deleted    int    // number of deleted files (for "excess deletes" check)
	foundSame  bool   // true if found at least one unchanged file
	checkFiles bilib.Names
}

func (ds *deltaSet) empty() bool {
	return len(ds.deltas) == 0
}

func (ds *deltaSet) sort() (sorted []string) {
	if ds.empty() {
		return
	}
	sorted = make([]string, 0, len(ds.deltas))
	for file := range ds.deltas {
		sorted = append(sorted, file)
	}
	sort.Strings(sorted)
	return
}

func (ds *deltaSet) printStats() {
	if ds.empty() {
		return
	}
	nAll := len(ds.deltas)
	nNew := 0
	nNewer := 0
	nOlder := 0
	nDeleted := 0
	for _, d := range ds.deltas {
		if d.is(deltaNew) {
			nNew++
		}
		if d.is(deltaNewer) {
			nNewer++
		}
		if d.is(deltaOlder) {
			nOlder++
		}
		if d.is(deltaDeleted) {
			nDeleted++
		}
	}
	fs.Infof(nil, "%s: %4d changes: %4d new, %4d newer, %4d older, %4d deleted",
		ds.msg, nAll, nNew, nNewer, nOlder, nDeleted)
}

// check potential conflicts (to avoid renaming if already identical)
func (b *bisyncRun) checkconflicts(ctxCheck context.Context, filterCheck *filter.Filter, fs1, fs2 fs.Fs) (bilib.Names, error) {
	matches := bilib.Names{}
	if filterCheck.HaveFilesFrom() {
		fs.Debugf(nil, "There are potential conflicts to check.")

		opt, close, checkopterr := check.GetCheckOpt(b.fs1, b.fs2)
		if checkopterr != nil {
			b.critical = true
			b.retryable = true
			fs.Debugf(nil, "GetCheckOpt error: %v", checkopterr)
			return matches, checkopterr
		}
		defer close()

		opt.Match = new(bytes.Buffer)

		// TODO: consider using custom CheckFn to act like cryptcheck, if either fs is a crypt remote and -c has been passed
		// note that cryptCheck() is not currently exported

		fs.Infof(nil, "Checking potential conflicts...")
		check := operations.Check(ctxCheck, opt)
		fs.Infof(nil, "Finished checking the potential conflicts. %s", check)

		//reset error count, because we don't want to count check errors as bisync errors
		accounting.Stats(ctxCheck).ResetErrors()

		//return the list of identical files to check against later
		if len(fmt.Sprint(opt.Match)) > 0 {
			matches = bilib.ToNames(strings.Split(fmt.Sprint(opt.Match), "\n"))
		}
		if matches.NotEmpty() {
			fs.Debugf(nil, "The following potential conflicts were determined to be identical. %v", matches)
		} else {
			fs.Debugf(nil, "None of the conflicts were determined to be identical.")
		}

	}
	return matches, nil
}

// findDeltas
func (b *bisyncRun) findDeltas(fctx context.Context, f fs.Fs, oldListing, newListing, msg string) (ds *deltaSet, err error) {
	var old, now *fileList

	old, err = b.loadListing(oldListing)
	if err != nil {
		fs.Errorf(nil, "Failed loading prior %s listing: %s", msg, oldListing)
		b.abort = true
		return
	}
	if err = b.checkListing(old, oldListing, "prior "+msg); err != nil {
		return
	}

	now, err = b.makeListing(fctx, f, newListing)
	if err == nil {
		err = b.checkListing(now, newListing, "current "+msg)
	}
	if err != nil {
		return
	}

	ds = &deltaSet{
		deltas:     map[string]delta{},
		fs:         f,
		msg:        msg,
		oldCount:   len(old.list),
		opt:        b.opt,
		checkFiles: bilib.Names{},
	}

	for _, file := range old.list {
		d := deltaZero
		if !now.has(file) {
			b.indent(msg, file, "File was deleted")
			ds.deleted++
			d |= deltaDeleted
		} else {
			if old.getTime(file) != now.getTime(file) {
				if old.beforeOther(now, file) {
					b.indent(msg, file, "File is newer")
					d |= deltaNewer
				} else { // Current version is older than prior sync.
					b.indent(msg, file, "File is OLDER")
					d |= deltaOlder
				}
			}
			// TODO Compare sizes and hashes
		}

		if d.is(deltaModified) {
			ds.deltas[file] = d
		} else {
			// Once we've found at least one unchanged file,
			// we know that not everything has changed,
			// as with a DST time change
			ds.foundSame = true
		}
	}

	for _, file := range now.list {
		if !old.has(file) {
			b.indent(msg, file, "File is new")
			ds.deltas[file] = deltaNew
		}
	}

	if b.opt.CheckAccess {
		// checkFiles is a small structure compared with the `now`, so we
		// return it alone and let the full delta map be garbage collected.
		for _, file := range now.list {
			if filepath.Base(file) == b.opt.CheckFilename {
				ds.checkFiles.Add(file)
			}
		}
	}

	return
}

// applyDeltas
func (b *bisyncRun) applyDeltas(ctx context.Context, ds1, ds2 *deltaSet) (changes1, changes2 bool, err error) {
	path1 := bilib.FsPath(b.fs1)
	path2 := bilib.FsPath(b.fs2)

	copy1to2 := bilib.Names{}
	copy2to1 := bilib.Names{}
	delete1 := bilib.Names{}
	delete2 := bilib.Names{}
	handled := bilib.Names{}

	ctxMove := b.opt.setDryRun(ctx)

	// efficient isDir check
	// we load the listing just once and store only the dirs
	dirs1, dirs1Err := b.listDirsOnly(1)
	if dirs1Err != nil {
		b.critical = true
		b.retryable = true
		fs.Debugf(nil, "Error generating dirsonly list for path1: %v", dirs1Err)
		return
	}

	dirs2, dirs2Err := b.listDirsOnly(2)
	if dirs2Err != nil {
		b.critical = true
		b.retryable = true
		fs.Debugf(nil, "Error generating dirsonly list for path2: %v", dirs2Err)
		return
	}

	// build a list of only the "deltaOther"s so we don't have to check more files than necessary
	// this is essentially the same as running rclone check with a --files-from filter, then exempting the --match results from being renamed
	// we therefore avoid having to list the same directory more than once.

	// we are intentionally overriding DryRun here because we need to perform the check, even during a dry run, or the results would be inaccurate.
	// check is a read-only operation by its nature, so it's already "dry" in that sense.
	ctxNew, ciCheck := fs.AddConfig(ctx)
	ciCheck.DryRun = false

	ctxCheck, filterCheck := filter.AddConfig(ctxNew)

	for _, file := range ds1.sort() {
		d1 := ds1.deltas[file]
		if d1.is(deltaOther) {
			d2 := ds2.deltas[file]
			if d2.is(deltaOther) {
				if err := filterCheck.AddFile(file); err != nil {
					fs.Debugf(nil, "Non-critical error adding file to list of potential conflicts to check: %s", err)
				} else {
					fs.Debugf(nil, "Added file to list of potential conflicts to check: %s", file)
				}
			}
		}
	}

	//if there are potential conflicts to check, check them all here (outside the loop) in one fell swoop
	matches, err := b.checkconflicts(ctxCheck, filterCheck, b.fs1, b.fs2)

	for _, file := range ds1.sort() {
		p1 := path1 + file
		p2 := path2 + file
		d1 := ds1.deltas[file]

		if d1.is(deltaOther) {
			d2, in2 := ds2.deltas[file]
			if !in2 {
				b.indent("Path1", p2, "Queue copy to Path2")
				copy1to2.Add(file)
			} else if d2.is(deltaDeleted) {
				b.indent("Path1", p2, "Queue copy to Path2")
				copy1to2.Add(file)
				handled.Add(file)
			} else if d2.is(deltaOther) {
				b.indent("!WARNING", file, "New or changed in both paths")

				//if files are identical, leave them alone instead of renaming
				if dirs1.has(file) && dirs2.has(file) {
					fs.Debugf(nil, "This is a directory, not a file. Skipping equality check and will not rename: %s", file)
				} else {
					equal := matches.Has(file)
					if equal {
						fs.Infof(nil, "Files are equal! Skipping: %s", file)
					} else {
						fs.Debugf(nil, "Files are NOT equal: %s", file)
						b.indent("!Path1", p1+"..path1", "Renaming Path1 copy")
						if err = operations.MoveFile(ctxMove, b.fs1, b.fs1, file+"..path1", file); err != nil {
							err = fmt.Errorf("path1 rename failed for %s: %w", p1, err)
							b.critical = true
							return
						}
						b.indent("!Path1", p2+"..path1", "Queue copy to Path2")
						copy1to2.Add(file + "..path1")

						b.indent("!Path2", p2+"..path2", "Renaming Path2 copy")
						if err = operations.MoveFile(ctxMove, b.fs2, b.fs2, file+"..path2", file); err != nil {
							err = fmt.Errorf("path2 rename failed for %s: %w", file, err)
							return
						}
						b.indent("!Path2", p1+"..path2", "Queue copy to Path1")
						copy2to1.Add(file + "..path2")
					}
				}
				handled.Add(file)
			}
		} else {
			// Path1 deleted
			d2, in2 := ds2.deltas[file]
			if !in2 {
				b.indent("Path2", p2, "Queue delete")
				delete2.Add(file)
			} else if d2.is(deltaOther) {
				b.indent("Path2", p1, "Queue copy to Path1")
				copy2to1.Add(file)
				handled.Add(file)
			} else if d2.is(deltaDeleted) {
				handled.Add(file)
			}
		}
	}

	for _, file := range ds2.sort() {
		p1 := path1 + file
		d2 := ds2.deltas[file]

		if handled.Has(file) {
			continue
		}
		if d2.is(deltaOther) {
			b.indent("Path2", p1, "Queue copy to Path1")
			copy2to1.Add(file)
		} else {
			// Deleted
			b.indent("Path1", p1, "Queue delete")
			delete1.Add(file)
		}
	}

	// Do the batch operation
	if copy2to1.NotEmpty() {
		changes1 = true
		b.indent("Path2", "Path1", "Do queued copies to")
		err = b.fastCopy(ctx, b.fs2, b.fs1, copy2to1, "copy2to1")
		if err != nil {
			return
		}

		//copy empty dirs from path2 to path1 (if --create-empty-src-dirs)
		b.syncEmptyDirs(ctx, b.fs1, copy2to1, dirs2, "make")
	}

	if copy1to2.NotEmpty() {
		changes2 = true
		b.indent("Path1", "Path2", "Do queued copies to")
		err = b.fastCopy(ctx, b.fs1, b.fs2, copy1to2, "copy1to2")
		if err != nil {
			return
		}

		//copy empty dirs from path1 to path2 (if --create-empty-src-dirs)
		b.syncEmptyDirs(ctx, b.fs2, copy1to2, dirs1, "make")
	}

	if delete1.NotEmpty() {
		changes1 = true
		b.indent("", "Path1", "Do queued deletes on")
		err = b.fastDelete(ctx, b.fs1, delete1, "delete1")
		if err != nil {
			return
		}

		//propagate deletions of empty dirs from path2 to path1 (if --create-empty-src-dirs)
		b.syncEmptyDirs(ctx, b.fs1, delete1, dirs1, "remove")
	}

	if delete2.NotEmpty() {
		changes2 = true
		b.indent("", "Path2", "Do queued deletes on")
		err = b.fastDelete(ctx, b.fs2, delete2, "delete2")
		if err != nil {
			return
		}

		//propagate deletions of empty dirs from path1 to path2 (if --create-empty-src-dirs)
		b.syncEmptyDirs(ctx, b.fs2, delete2, dirs2, "remove")
	}

	return
}

// excessDeletes checks whether number of deletes is within allowed range
func (ds *deltaSet) excessDeletes() bool {
	maxDelete := ds.opt.MaxDelete
	maxRatio := float64(maxDelete) / 100.0
	curRatio := 0.0
	if ds.deleted > 0 && ds.oldCount > 0 {
		curRatio = float64(ds.deleted) / float64(ds.oldCount)
	}

	if curRatio <= maxRatio {
		return false
	}

	fs.Errorf("Safety abort",
		"too many deletes (>%d%%, %d of %d) on %s %s. Run with --force if desired.",
		maxDelete, ds.deleted, ds.oldCount, ds.msg, quotePath(bilib.FsPath(ds.fs)))
	return true
}
