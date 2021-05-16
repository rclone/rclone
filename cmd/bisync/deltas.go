// Package bisync implements bisync
// Copyright (c) 2017-2020 Chris Nelson
package bisync

import (
	"context"
	"path/filepath"
	"sort"

	"github.com/pkg/errors"
	"github.com/rclone/rclone/cmd/bisync/bilib"
	"github.com/rclone/rclone/fs"
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
				b.indent("!Path1", p1+"..path1", "Renaming Path1 copy")
				if err = operations.MoveFile(ctxMove, b.fs1, b.fs1, file+"..path1", file); err != nil {
					err = errors.Wrapf(err, "path1 rename failed for %s", p1)
					b.critical = true
					return
				}
				b.indent("!Path1", p2+"..path1", "Queue copy to Path2")
				copy1to2.Add(file + "..path1")

				b.indent("!Path2", p2+"..path2", "Renaming Path2 copy")
				if err = operations.MoveFile(ctxMove, b.fs2, b.fs2, file+"..path2", file); err != nil {
					err = errors.Wrapf(err, "path2 rename failed for %s", file)
					return
				}
				b.indent("!Path2", p1+"..path2", "Queue copy to Path1")
				copy2to1.Add(file + "..path2")
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
	}

	if copy1to2.NotEmpty() {
		changes2 = true
		b.indent("Path1", "Path2", "Do queued copies to")
		err = b.fastCopy(ctx, b.fs1, b.fs2, copy1to2, "copy1to2")
		if err != nil {
			return
		}
	}

	if delete1.NotEmpty() {
		changes1 = true
		b.indent("", "Path1", "Do queued deletes on")
		err = b.fastDelete(ctx, b.fs1, delete1, "delete1")
		if err != nil {
			return
		}
	}

	if delete2.NotEmpty() {
		changes2 = true
		b.indent("", "Path2", "Do queued deletes on")
		err = b.fastDelete(ctx, b.fs2, delete2, "delete2")
		if err != nil {
			return
		}
	}

	return
}

// exccessDeletes checks whether number of deletes is within allowed range
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
