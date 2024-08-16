// Package bisync implements bisync
// Copyright (c) 2017-2020 Chris Nelson
package bisync

import (
	"context"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/rclone/rclone/cmd/bisync/bilib"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/filter"
	"github.com/rclone/rclone/lib/terminal"
	"golang.org/x/text/unicode/norm"
)

// delta
type delta uint8

const (
	deltaZero delta = 0
	deltaNew  delta = 1 << iota
	deltaNewer
	deltaOlder
	deltaLarger
	deltaSmaller
	deltaHash
	deltaDeleted
)

const (
	deltaSize     delta = deltaLarger | deltaSmaller
	deltaTime     delta = deltaNewer | deltaOlder
	deltaModified delta = deltaTime | deltaSize | deltaHash
	deltaOther    delta = deltaNew | deltaTime | deltaSize | deltaHash
)

func (d delta) is(cond delta) bool {
	return d&cond != 0
}

// deltaSet
type deltaSet struct {
	deltas     map[string]delta
	size       map[string]int64
	time       map[string]time.Time
	hash       map[string]string
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
	nMod := 0
	nTime := 0
	nNewer := 0
	nOlder := 0
	nSize := 0
	nLarger := 0
	nSmaller := 0
	nHash := 0
	nDeleted := 0
	for _, d := range ds.deltas {
		if d.is(deltaNew) {
			nNew++
		}
		if d.is(deltaModified) {
			nMod++
		}
		if d.is(deltaTime) {
			nTime++
		}
		if d.is(deltaNewer) {
			nNewer++
		}
		if d.is(deltaOlder) {
			nOlder++
		}
		if d.is(deltaSize) {
			nSize++
		}
		if d.is(deltaLarger) {
			nLarger++
		}
		if d.is(deltaSmaller) {
			nSmaller++
		}
		if d.is(deltaHash) {
			nHash++
		}
		if d.is(deltaDeleted) {
			nDeleted++
		}
	}
	if nAll != nNew+nMod+nDeleted {
		fs.Errorf(nil, "something doesn't add up! %4d != %4d + %4d + %4d", nAll, nNew, nMod, nDeleted)
	}
	fs.Infof(nil, "%s: %4d changes: "+Color(terminal.GreenFg, "%4d new")+", "+Color(terminal.YellowFg, "%4d modified")+", "+Color(terminal.RedFg, "%4d deleted"),
		ds.msg, nAll, nNew, nMod, nDeleted)
	if nMod > 0 {
		details := []string{}
		if nTime > 0 {
			details = append(details, fmt.Sprintf(Color(terminal.CyanFg, "%4d newer"), nNewer))
			details = append(details, fmt.Sprintf(Color(terminal.BlueFg, "%4d older"), nOlder))
		}
		if nSize > 0 {
			details = append(details, fmt.Sprintf(Color(terminal.CyanFg, "%4d larger"), nLarger))
			details = append(details, fmt.Sprintf(Color(terminal.BlueFg, "%4d smaller"), nSmaller))
		}
		if nHash > 0 {
			details = append(details, fmt.Sprintf(Color(terminal.CyanFg, "%4d hash differs"), nHash))
		}
		if (nNewer+nOlder != nTime) || (nLarger+nSmaller != nSize) || (nMod > nTime+nSize+nHash) {
			fs.Errorf(nil, "something doesn't add up!")
		}

		fs.Infof(nil, "(%s: %s)", Color(terminal.YellowFg, "Modified"), strings.Join(details, ", "))
	}
}

// findDeltas
func (b *bisyncRun) findDeltas(fctx context.Context, f fs.Fs, oldListing string, now *fileList, msg string) (ds *deltaSet, err error) {
	var old *fileList
	newListing := oldListing + "-new"

	old, err = b.loadListing(oldListing)
	if err != nil {
		fs.Errorf(nil, "Failed loading prior %s listing: %s", msg, oldListing)
		b.abort = true
		return
	}
	if err = b.checkListing(old, oldListing, "prior "+msg); err != nil {
		return
	}

	if err == nil {
		err = b.checkListing(now, newListing, "current "+msg)
	}
	if err != nil {
		return
	}

	ds = &deltaSet{
		deltas:     map[string]delta{},
		size:       map[string]int64{},
		time:       map[string]time.Time{},
		hash:       map[string]string{},
		fs:         f,
		msg:        msg,
		oldCount:   len(old.list),
		opt:        b.opt,
		checkFiles: bilib.Names{},
	}

	for _, file := range old.list {
		// REMEMBER: this section is only concerned with comparing listings from the same side (not different sides)
		d := deltaZero
		s := int64(0)
		h := ""
		var t time.Time
		if !now.has(file) {
			b.indent(msg, file, Color(terminal.RedFg, "File was deleted"))
			ds.deleted++
			d |= deltaDeleted
		} else if !now.isDir(file) {
			// skip dirs here, as we only care if they are new/deleted, not newer/older
			whatchanged := []string{}
			if b.opt.Compare.Size {
				if sizeDiffers(old.getSize(file), now.getSize(file)) {
					fs.Debugf(file, "(old: %v current: %v)", old.getSize(file), now.getSize(file))
					if now.getSize(file) > old.getSize(file) {
						whatchanged = append(whatchanged, Color(terminal.MagentaFg, "size (larger)"))
						d |= deltaLarger
					} else {
						whatchanged = append(whatchanged, Color(terminal.MagentaFg, "size (smaller)"))
						d |= deltaSmaller
					}
					s = now.getSize(file)
				}
			}
			if b.opt.Compare.Modtime {
				if timeDiffers(fctx, old.getTime(file), now.getTime(file), f, f) {
					if old.beforeOther(now, file) {
						fs.Debugf(file, "(old: %v current: %v)", old.getTime(file), now.getTime(file))
						whatchanged = append(whatchanged, Color(terminal.MagentaFg, "time (newer)"))
						d |= deltaNewer
					} else { // Current version is older than prior sync.
						fs.Debugf(file, "(old: %v current: %v)", old.getTime(file), now.getTime(file))
						whatchanged = append(whatchanged, Color(terminal.MagentaFg, "time (older)"))
						d |= deltaOlder
					}
					t = now.getTime(file)
				}
			}
			if b.opt.Compare.Checksum {
				if hashDiffers(old.getHash(file), now.getHash(file), old.hash, now.hash, old.getSize(file), now.getSize(file)) {
					fs.Debugf(file, "(old: %v current: %v)", old.getHash(file), now.getHash(file))
					whatchanged = append(whatchanged, Color(terminal.MagentaFg, "hash"))
					d |= deltaHash
					h = now.getHash(file)
				}
			}
			// concat changes and print log
			if d.is(deltaModified) {
				summary := fmt.Sprintf(Color(terminal.YellowFg, "File changed: %s"), strings.Join(whatchanged, ", "))
				b.indent(msg, file, summary)
			}
		}

		if d.is(deltaModified) {
			ds.deltas[file] = d
			if b.opt.Compare.Size {
				ds.size[file] = s
			}
			if b.opt.Compare.Modtime {
				ds.time[file] = t
			}
			if b.opt.Compare.Checksum {
				ds.hash[file] = h
			}
		} else if d.is(deltaDeleted) {
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
			b.indent(msg, file, Color(terminal.GreenFg, "File is new"))
			ds.deltas[file] = deltaNew
			if b.opt.Compare.Size {
				ds.size[file] = now.getSize(file)
			}
			if b.opt.Compare.Modtime {
				ds.time[file] = now.getTime(file)
			}
			if b.opt.Compare.Checksum {
				ds.hash[file] = now.getHash(file)
			}
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
func (b *bisyncRun) applyDeltas(ctx context.Context, ds1, ds2 *deltaSet) (changes1, changes2 bool, results2to1, results1to2 []Results, queues queues, err error) {
	path1 := bilib.FsPath(b.fs1)
	path2 := bilib.FsPath(b.fs2)

	copy1to2 := bilib.Names{}
	copy2to1 := bilib.Names{}
	delete1 := bilib.Names{}
	delete2 := bilib.Names{}
	handled := bilib.Names{}
	renameSkipped := bilib.Names{}
	deletedonboth := bilib.Names{}
	skippedDirs1 := newFileList()
	skippedDirs2 := newFileList()
	b.renames = renames{}

	ctxMove := b.opt.setDryRun(ctx)

	// update AliasMap for deleted files, as march does not know about them
	b.updateAliases(ctx, ds1, ds2)

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
		alias := b.aliases.Alias(file)
		d1 := ds1.deltas[file]
		if d1.is(deltaOther) {
			d2, in2 := ds2.deltas[file]
			file2 := file
			if !in2 && file != alias {
				d2 = ds2.deltas[alias]
				file2 = alias
			}
			if d2.is(deltaOther) {
				// if size or hash differ, skip this, as we already know they're not equal
				if (b.opt.Compare.Size && sizeDiffers(ds1.size[file], ds2.size[file2])) ||
					(b.opt.Compare.Checksum && hashDiffers(ds1.hash[file], ds2.hash[file2], b.opt.Compare.HashType1, b.opt.Compare.HashType2, ds1.size[file], ds2.size[file2])) {
					fs.Debugf(file, "skipping equality check as size/hash definitely differ")
				} else {
					checkit := func(filename string) {
						if err := filterCheck.AddFile(filename); err != nil {
							fs.Debugf(nil, "Non-critical error adding file to list of potential conflicts to check: %s", err)
						} else {
							fs.Debugf(nil, "Added file to list of potential conflicts to check: %s", filename)
						}
					}
					checkit(file)
					if file != alias {
						checkit(alias)
					}
				}
			}
		}
	}

	//if there are potential conflicts to check, check them all here (outside the loop) in one fell swoop
	matches, err := b.checkconflicts(ctxCheck, filterCheck, b.fs1, b.fs2)

	for _, file := range ds1.sort() {
		alias := b.aliases.Alias(file)
		p1 := path1 + file
		p2 := path2 + alias
		d1 := ds1.deltas[file]

		if d1.is(deltaOther) {
			d2, in2 := ds2.deltas[file]
			// try looking under alternate name
			if !in2 && file != alias {
				d2, in2 = ds2.deltas[alias]
			}
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
				if (dirs1.has(file) || dirs1.has(alias)) && (dirs2.has(file) || dirs2.has(alias)) {
					fs.Infof(nil, "This is a directory, not a file. Skipping equality check and will not rename: %s", file)
					ls1.getPut(file, skippedDirs1)
					ls2.getPut(file, skippedDirs2)
					b.debugFn(file, func() {
						b.debug(file, fmt.Sprintf("deltas dir: %s, ls1 has name?: %v, ls2 has name?: %v", file, ls1.has(b.DebugName), ls2.has(b.DebugName)))
					})
				} else {
					equal := matches.Has(file)
					if !equal {
						equal = matches.Has(alias)
					}
					if equal {
						if ciCheck.FixCase && file != alias {
							// the content is equal but filename still needs to be FixCase'd, so copy1to2
							// the Path1 version is deemed "correct" in this scenario
							fs.Infof(alias, "Files are equal but will copy anyway to fix case to %s", file)
							copy1to2.Add(file)
						} else if b.opt.Compare.Modtime && timeDiffers(ctx, ls1.getTime(ls1.getTryAlias(file, alias)), ls2.getTime(ls2.getTryAlias(file, alias)), b.fs1, b.fs2) {
							fs.Infof(file, "Files are equal but will copy anyway to update modtime (will not rename)")
							if ls1.getTime(ls1.getTryAlias(file, alias)).Before(ls2.getTime(ls2.getTryAlias(file, alias))) {
								// Path2 is newer
								b.indent("Path2", p1, "Queue copy to Path1")
								copy2to1.Add(ls2.getTryAlias(file, alias))
							} else {
								// Path1 is newer
								b.indent("Path1", p2, "Queue copy to Path2")
								copy1to2.Add(ls1.getTryAlias(file, alias))
							}
						} else {
							fs.Infof(nil, "Files are equal! Skipping: %s", file)
							renameSkipped.Add(file)
							renameSkipped.Add(alias)
						}
					} else {
						fs.Debugf(nil, "Files are NOT equal: %s", file)
						err = b.resolve(ctxMove, path1, path2, file, alias, &renameSkipped, &copy1to2, &copy2to1, ds1, ds2)
						if err != nil {
							return
						}
					}
				}
				handled.Add(file)
			}
		} else {
			// Path1 deleted
			d2, in2 := ds2.deltas[file]
			// try looking under alternate name
			fs.Debugf(file, "alias: %s, in2: %v", alias, in2)
			if !in2 && file != alias {
				fs.Debugf(file, "looking for alias: %s", alias)
				d2, in2 = ds2.deltas[alias]
				if in2 {
					fs.Debugf(file, "detected alias: %s", alias)
				}
			}
			if !in2 {
				b.indent("Path2", p2, "Queue delete")
				delete2.Add(file)
				copy1to2.Add(file)
			} else if d2.is(deltaOther) {
				b.indent("Path2", p1, "Queue copy to Path1")
				copy2to1.Add(file)
				handled.Add(file)
			} else if d2.is(deltaDeleted) {
				handled.Add(file)
				deletedonboth.Add(file)
				deletedonboth.Add(alias)
			}
		}
	}

	for _, file := range ds2.sort() {
		alias := b.aliases.Alias(file)
		p1 := path1 + alias
		d2 := ds2.deltas[file]

		if handled.Has(file) || handled.Has(alias) {
			continue
		}
		if d2.is(deltaOther) {
			b.indent("Path2", p1, "Queue copy to Path1")
			copy2to1.Add(file)
		} else {
			// Deleted
			b.indent("Path1", p1, "Queue delete")
			delete1.Add(file)
			copy2to1.Add(file)
		}
	}

	// Do the batch operation
	if copy2to1.NotEmpty() && !b.InGracefulShutdown {
		changes1 = true
		b.indent("Path2", "Path1", "Do queued copies to")
		ctx = b.setBackupDir(ctx, 1)
		results2to1, err = b.fastCopy(ctx, b.fs2, b.fs1, copy2to1, "copy2to1")

		// retries, if any
		results2to1, err = b.retryFastCopy(ctx, b.fs2, b.fs1, copy2to1, "copy2to1", results2to1, err)

		if !b.InGracefulShutdown && err != nil {
			return
		}

		//copy empty dirs from path2 to path1 (if --create-empty-src-dirs)
		b.syncEmptyDirs(ctx, b.fs1, copy2to1, dirs2, &results2to1, "make")
	}

	if copy1to2.NotEmpty() && !b.InGracefulShutdown {
		changes2 = true
		b.indent("Path1", "Path2", "Do queued copies to")
		ctx = b.setBackupDir(ctx, 2)
		results1to2, err = b.fastCopy(ctx, b.fs1, b.fs2, copy1to2, "copy1to2")

		// retries, if any
		results1to2, err = b.retryFastCopy(ctx, b.fs1, b.fs2, copy1to2, "copy1to2", results1to2, err)

		if !b.InGracefulShutdown && err != nil {
			return
		}

		//copy empty dirs from path1 to path2 (if --create-empty-src-dirs)
		b.syncEmptyDirs(ctx, b.fs2, copy1to2, dirs1, &results1to2, "make")
	}

	if delete1.NotEmpty() && !b.InGracefulShutdown {
		if err = b.saveQueue(delete1, "delete1"); err != nil {
			return
		}
		//propagate deletions of empty dirs from path2 to path1 (if --create-empty-src-dirs)
		b.syncEmptyDirs(ctx, b.fs1, delete1, dirs1, &results2to1, "remove")
	}

	if delete2.NotEmpty() && !b.InGracefulShutdown {
		if err = b.saveQueue(delete2, "delete2"); err != nil {
			return
		}
		//propagate deletions of empty dirs from path1 to path2 (if --create-empty-src-dirs)
		b.syncEmptyDirs(ctx, b.fs2, delete2, dirs2, &results1to2, "remove")
	}

	queues.copy1to2 = copy1to2
	queues.copy2to1 = copy2to1
	queues.renameSkipped = renameSkipped
	queues.deletedonboth = deletedonboth
	queues.skippedDirs1 = skippedDirs1
	queues.skippedDirs2 = skippedDirs2

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

// normally we build the AliasMap from march results,
// however, march does not know about deleted files, so need to manually check them for aliases
func (b *bisyncRun) updateAliases(ctx context.Context, ds1, ds2 *deltaSet) {
	ci := fs.GetConfig(ctx)
	// skip if not needed
	if ci.NoUnicodeNormalization && !ci.IgnoreCaseSync && !b.fs1.Features().CaseInsensitive && !b.fs2.Features().CaseInsensitive {
		return
	}
	if ds1.deleted < 1 && ds2.deleted < 1 {
		return
	}

	fs.Debugf(nil, "Updating AliasMap")

	transform := func(s string) string {
		if !ci.NoUnicodeNormalization {
			s = norm.NFC.String(s)
		}
		// note: march only checks the dest, but we check both here
		if ci.IgnoreCaseSync || b.fs1.Features().CaseInsensitive || b.fs2.Features().CaseInsensitive {
			s = strings.ToLower(s)
		}
		return s
	}

	delMap1 := map[string]string{}  // [transformedname]originalname
	delMap2 := map[string]string{}  // [transformedname]originalname
	fullMap1 := map[string]string{} // [transformedname]originalname
	fullMap2 := map[string]string{} // [transformedname]originalname

	for _, name := range ls1.list {
		fullMap1[transform(name)] = name
	}
	for _, name := range ls2.list {
		fullMap2[transform(name)] = name
	}

	addDeletes := func(ds *deltaSet, delMap, fullMap map[string]string) {
		for _, file := range ds.sort() {
			d := ds.deltas[file]
			if d.is(deltaDeleted) {
				delMap[transform(file)] = file
				fullMap[transform(file)] = file
			}
		}
	}
	addDeletes(ds1, delMap1, fullMap1)
	addDeletes(ds2, delMap2, fullMap2)

	addAliases := func(delMap, fullMap map[string]string) {
		for transformedname, name := range delMap {
			matchedName, found := fullMap[transformedname]
			if found && name != matchedName {
				fs.Debugf(name, "adding alias %s", matchedName)
				b.aliases.Add(name, matchedName)
			}
		}
	}
	addAliases(delMap1, fullMap2)
	addAliases(delMap2, fullMap1)
}
