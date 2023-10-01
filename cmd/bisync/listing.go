// Package bisync implements bisync
// Copyright (c) 2017-2020 Chris Nelson
package bisync

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/filter"
	"github.com/rclone/rclone/fs/hash"
	"github.com/rclone/rclone/fs/operations"
	"github.com/rclone/rclone/fs/walk"
	"golang.org/x/exp/slices"
)

// ListingHeader defines first line of a listing
const ListingHeader = "# bisync listing v1 from"

// lineRegex and lineFormat define listing line format
//
//	flags <- size -> <- hash -> id <------------ modtime -----------> "<----- remote"
//	-        3009805 md5:xxxxxx -  2006-01-02T15:04:05.000000000-0700 "12 - Wait.mp3"
//
// flags: "-" for a file and "d" for a directory (reserved)
// hash: "type:value" or "-" (example: "md5:378840336ab14afa9c6b8d887e68a340")
// id: "-" (reserved)
const lineFormat = "%s %8d %s %s %s %q\n"

var lineRegex = regexp.MustCompile(`^(\S) +(\d+) (\S+) (\S+) (\d{4}-\d\d-\d\dT\d\d:\d\d:\d\d\.\d{9}[+-]\d{4}) (".+")$`)

// timeFormat defines time format used in listings
const timeFormat = "2006-01-02T15:04:05.000000000-0700"

// TZ defines time zone used in listings
var TZ = time.UTC
var tzLocal = false

// fileInfo describes a file
type fileInfo struct {
	size  int64
	time  time.Time
	hash  string
	id    string
	flags string
}

// fileList represents a listing
type fileList struct {
	list []string
	info map[string]*fileInfo
	hash hash.Type
}

func newFileList() *fileList {
	return &fileList{
		info: map[string]*fileInfo{},
		list: []string{},
	}
}

func (ls *fileList) empty() bool {
	return len(ls.list) == 0
}

func (ls *fileList) has(file string) bool {
	_, found := ls.info[file]
	if !found {
		//try unquoting
		file, _ = strconv.Unquote(`"` + file + `"`)
		_, found = ls.info[file]
	}
	return found
}

func (ls *fileList) get(file string) *fileInfo {
	info, found := ls.info[file]
	if !found {
		//try unquoting
		file, _ = strconv.Unquote(`"` + file + `"`)
		info = ls.info[fmt.Sprint(file)]
	}
	return info
}

func (ls *fileList) remove(file string) {
	if ls.has(file) {
		ls.list = slices.Delete(ls.list, slices.Index(ls.list, file), slices.Index(ls.list, file)+1)
		delete(ls.info, file)
	}
}

func (ls *fileList) put(file string, size int64, time time.Time, hash, id string, flags string) {
	fi := ls.get(file)
	if fi != nil {
		fi.size = size
		fi.time = time
		fi.hash = hash
		fi.id = id
		fi.flags = flags
	} else {
		fi = &fileInfo{
			size:  size,
			time:  time,
			hash:  hash,
			id:    id,
			flags: flags,
		}
		ls.info[file] = fi
		ls.list = append(ls.list, file)
	}
}

func (ls *fileList) getTime(file string) time.Time {
	fi := ls.get(file)
	if fi == nil {
		return time.Time{}
	}
	return fi.time
}

func (ls *fileList) beforeOther(other *fileList, file string) bool {
	thisTime := ls.getTime(file)
	thatTime := other.getTime(file)
	if thisTime.IsZero() || thatTime.IsZero() {
		return false
	}
	return thisTime.Before(thatTime)
}

func (ls *fileList) afterTime(file string, time time.Time) bool {
	fi := ls.get(file)
	if fi == nil {
		return false
	}
	return fi.time.After(time)
}

// sort by path name
func (ls *fileList) sort() {
	sort.SliceStable(ls.list, func(i, j int) bool {
		return ls.list[i] < ls.list[j]
	})
}

// save will save listing to a file.
func (ls *fileList) save(ctx context.Context, listing string) error {
	file, err := os.Create(listing)
	if err != nil {
		return err
	}
	ls.sort()

	hashName := ""
	if ls.hash != hash.None {
		hashName = ls.hash.String()
	}

	_, err = fmt.Fprintf(file, "%s %s\n", ListingHeader, time.Now().In(TZ).Format(timeFormat))
	if err != nil {
		_ = file.Close()
		_ = os.Remove(listing)
		return err
	}

	for _, remote := range ls.list {
		fi := ls.get(remote)

		time := fi.time.In(TZ).Format(timeFormat)

		hash := "-"
		if hashName != "" && fi.hash != "" {
			hash = hashName + ":" + fi.hash
		}

		id := fi.id
		if id == "" {
			id = "-"
		}

		flags := fi.flags
		if flags == "" {
			flags = "-"
		}

		_, err = fmt.Fprintf(file, lineFormat, flags, fi.size, hash, id, time, remote)
		if err != nil {
			_ = file.Close()
			_ = os.Remove(listing)
			return err
		}
	}

	return file.Close()
}

// loadListing will load listing from a file.
// The key is the path to the file relative to the Path1/Path2 base.
// File size of -1, as for Google Docs, prints a warning and won't be loaded.
func (b *bisyncRun) loadListing(listing string) (*fileList, error) {
	file, err := os.Open(listing)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = file.Close()
	}()

	reader := bufio.NewReader(file)
	ls := newFileList()
	lastHashName := ""

	for {
		line, err := reader.ReadString('\n')
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}

		line = strings.TrimSuffix(line, "\n")
		if line == "" || line[0] == '#' {
			continue
		}

		match := lineRegex.FindStringSubmatch(line)
		if match == nil {
			fs.Logf(listing, "Ignoring incorrect line: %q", line)
			continue
		}
		flags, sizeStr, hashStr := match[1], match[2], match[3]
		id, timeStr, nameStr := match[4], match[5], match[6]

		sizeVal, sizeErr := strconv.ParseInt(sizeStr, 10, 64)
		timeVal, timeErr := time.ParseInLocation(timeFormat, timeStr, TZ)
		nameVal, nameErr := strconv.Unquote(nameStr)

		hashName, hashVal, hashErr := parseHash(hashStr)
		if hashErr == nil && hashName != "" {
			if lastHashName == "" {
				lastHashName = hashName
				hashErr = ls.hash.Set(hashName)
			} else if hashName != lastHashName {
				fs.Logf(listing, "Inconsistent hash type in line: %q", line)
				continue
			}
		}

		if (flags != "-" && flags != "d") || id != "-" || sizeErr != nil || timeErr != nil || hashErr != nil || nameErr != nil {
			fs.Logf(listing, "Ignoring incorrect line: %q", line)
			continue
		}

		if ls.has(nameVal) {
			fs.Logf(listing, "Duplicate line (keeping latest): %q", line)
			if ls.afterTime(nameVal, timeVal) {
				continue
			}
		}

		ls.put(nameVal, sizeVal, timeVal.In(TZ), hashVal, id, flags)
	}

	return ls, nil
}

func parseHash(str string) (string, string, error) {
	if str == "-" {
		return "", "", nil
	}
	if pos := strings.Index(str, ":"); pos > 0 {
		name, val := str[:pos], str[pos+1:]
		if name != "" && val != "" {
			return name, val, nil
		}
	}
	return "", "", fmt.Errorf("invalid hash %q", str)
}

// makeListing will produce listing from directory tree and write it to a file
func (b *bisyncRun) makeListing(ctx context.Context, f fs.Fs, listing string) (ls *fileList, err error) {
	ci := fs.GetConfig(ctx)
	depth := ci.MaxDepth
	hashType := hash.None
	if !b.opt.IgnoreListingChecksum {
		// Currently bisync just honors --ignore-listing-checksum
		// (note that this is different from --ignore-checksum)
		// TODO add full support for checksums and related flags
		hashType = f.Hashes().GetOne()
	}
	ls = newFileList()
	ls.hash = hashType
	var lock sync.Mutex
	listType := walk.ListObjects
	if b.opt.CreateEmptySrcDirs {
		listType = walk.ListAll
	}
	err = walk.ListR(ctx, f, "", false, depth, listType, func(entries fs.DirEntries) error {
		var firstErr error
		entries.ForObject(func(o fs.Object) {
			//tr := accounting.Stats(ctx).NewCheckingTransfer(o) // TODO
			var (
				hashVal string
				hashErr error
			)
			if hashType != hash.None {
				hashVal, hashErr = o.Hash(ctx, hashType)
				if firstErr == nil {
					firstErr = hashErr
				}
			}
			time := o.ModTime(ctx).In(TZ)
			id := ""     // TODO
			flags := "-" // "-" for a file and "d" for a directory
			lock.Lock()
			ls.put(o.Remote(), o.Size(), time, hashVal, id, flags)
			lock.Unlock()
			//tr.Done(ctx, nil) // TODO
		})
		if b.opt.CreateEmptySrcDirs {
			entries.ForDir(func(o fs.Directory) {
				var (
					hashVal string
				)
				time := o.ModTime(ctx).In(TZ)
				id := ""     // TODO
				flags := "d" // "-" for a file and "d" for a directory
				lock.Lock()
				//record size as 0 instead of -1, so bisync doesn't think it's a google doc
				ls.put(o.Remote(), 0, time, hashVal, id, flags)
				lock.Unlock()
			})
		}
		return firstErr
	})
	if err == nil {
		err = ls.save(ctx, listing)
	}
	if err != nil {
		b.abort = true
	}
	return
}

// checkListing verifies that listing is not empty (unless resynching)
func (b *bisyncRun) checkListing(ls *fileList, listing, msg string) error {
	if b.opt.Resync || !ls.empty() {
		return nil
	}
	fs.Errorf(nil, "Empty %s listing. Cannot sync to an empty directory: %s", msg, listing)
	b.critical = true
	b.retryable = true
	return fmt.Errorf("empty %s listing: %s", msg, listing)
}

// listingNum should be 1 for path1 or 2 for path2
func (b *bisyncRun) loadListingNum(listingNum int) (*fileList, error) {
	listingpath := b.basePath + ".path1.lst-new"
	if listingNum == 2 {
		listingpath = b.basePath + ".path2.lst-new"
	}

	if b.opt.DryRun {
		listingpath = strings.Replace(listingpath, ".lst-", ".lst-dry-", 1)
	}

	fs.Debugf(nil, "loading listing for path %d at: %s", listingNum, listingpath)
	return b.loadListing(listingpath)
}

func (b *bisyncRun) listDirsOnly(listingNum int) (*fileList, error) {
	var fulllisting *fileList
	var dirsonly = newFileList()
	var err error

	if !b.opt.CreateEmptySrcDirs {
		return dirsonly, err
	}

	fulllisting, err = b.loadListingNum(listingNum)

	if err != nil {
		b.critical = true
		b.retryable = true
		fs.Debugf(nil, "Error loading listing to generate dirsonly list: %v", err)
		return dirsonly, err
	}

	for _, obj := range fulllisting.list {
		info := fulllisting.get(obj)

		if info.flags == "d" {
			fs.Debugf(nil, "found a dir: %s", obj)
			dirsonly.put(obj, info.size, info.time, info.hash, info.id, info.flags)
		} else {
			fs.Debugf(nil, "not a dir: %s", obj)
		}
	}

	return dirsonly, err
}

// ConvertPrecision returns the Modtime rounded to Dest's precision if lower, otherwise unchanged
// Need to use the other fs's precision (if lower) when copying
// Note: we need to use Truncate rather than Round so that After() is reliable.
// (2023-11-02 20:22:45.552679442 +0000 < UTC 2023-11-02 20:22:45.553 +0000 UTC)
func ConvertPrecision(Modtime time.Time, dst fs.Fs) time.Time {
	DestPrecision := dst.Precision()

	// In case it's wrapping an Fs with lower precision, try unwrapping and use the lowest.
	if Modtime.Truncate(DestPrecision).After(Modtime.Truncate(fs.UnWrapFs(dst).Precision())) {
		DestPrecision = fs.UnWrapFs(dst).Precision()
	}

	if Modtime.After(Modtime.Truncate(DestPrecision)) {
		return Modtime.Truncate(DestPrecision)
	}
	return Modtime
}

// modifyListing will modify the listing based on the results of the sync
func (b *bisyncRun) modifyListing(ctx context.Context, src fs.Fs, dst fs.Fs, srcListing string, dstListing string, results []Results, queues queues, is1to2 bool) (err error) {
	queue := queues.copy2to1
	renames := queues.renamed2
	direction := "2to1"
	if is1to2 {
		queue = queues.copy1to2
		renames = queues.renamed1
		direction = "1to2"
	}

	fs.Debugf(nil, "updating %s", direction)
	fs.Debugf(nil, "RESULTS: %v", results)
	fs.Debugf(nil, "QUEUE: %v", queue)

	srcList, err := b.loadListing(srcListing)
	if err != nil {
		return fmt.Errorf("cannot read prior listing: %w", err)
	}
	dstList, err := b.loadListing(dstListing)
	if err != nil {
		return fmt.Errorf("cannot read prior listing: %w", err)
	}
	// set list hash type
	if b.opt.Resync && !b.opt.IgnoreListingChecksum {
		srcList.hash = src.Hashes().GetOne()
		dstList.hash = dst.Hashes().GetOne()
	}

	srcWinners := newFileList()
	dstWinners := newFileList()
	errors := newFileList()
	ctxRecheck, filterRecheck := filter.AddConfig(ctx)

	for _, result := range results {
		if result.Name == "" {
			continue
		}

		if result.Flags == "d" && !b.opt.CreateEmptySrcDirs {
			continue
		}

		// build src winners list
		if result.IsSrc && result.Src != "" && (result.Winner.Err == nil || result.Flags == "d") {
			srcWinners.put(result.Name, result.Size, ConvertPrecision(result.Modtime, src), result.Hash, "-", result.Flags)
			fs.Debugf(nil, "winner: copy to src: %v", result)
		}

		// build dst winners list
		if result.IsWinner && result.Winner.Side != "none" && (result.Winner.Err == nil || result.Flags == "d") {
			dstWinners.put(result.Name, result.Size, ConvertPrecision(result.Modtime, dst), result.Hash, "-", result.Flags)
			fs.Debugf(nil, "winner: copy to dst: %v", result)
		}

		// build errors list
		if result.Err != nil || result.Winner.Err != nil {
			errors.put(result.Name, result.Size, result.Modtime, result.Hash, "-", result.Flags)
			if err := filterRecheck.AddFile(result.Name); err != nil {
				fs.Debugf(result.Name, "error adding file to recheck filter: %v", err)
			}
		}
	}

	updateLists := func(side string, winners, list *fileList) {
		// removals from side
		for _, oldFile := range queue.ToList() {
			if !winners.has(oldFile) && list.has(oldFile) && !errors.has(oldFile) {
				list.remove(oldFile)
				fs.Debugf(nil, "decision: removed from %s: %v", side, oldFile)
			} else if winners.has(oldFile) {
				// copies to side
				new := winners.get(oldFile)
				list.put(oldFile, new.size, new.time, new.hash, new.id, new.flags)
				fs.Debugf(nil, "decision: copied to %s: %v", side, oldFile)
			} else {
				fs.Debugf(oldFile, "file in queue but missing from %s transfers", side)
				if err := filterRecheck.AddFile(oldFile); err != nil {
					fs.Debugf(oldFile, "error adding file to recheck filter: %v", err)
				}
			}
		}
	}
	updateLists("src", srcWinners, srcList)
	updateLists("dst", dstWinners, dstList)

	// TODO: rollback on error

	// account for "deltaOthers" we handled separately
	if queues.deletedonboth.NotEmpty() {
		for file := range queues.deletedonboth {
			srcList.remove(file)
			dstList.remove(file)
		}
	}
	if renames.NotEmpty() && !b.opt.DryRun {
		// renamed on src and copied to dst
		renamesList := renames.ToList()
		for _, file := range renamesList {
			// we'll handle the other side when we go the other direction
			newName := file + "..path2"
			oppositeName := file + "..path1"
			if is1to2 {
				newName = file + "..path1"
				oppositeName = file + "..path2"
			}
			var new *fileInfo
			// we prefer to get the info from the ..path1 / ..path2 versions
			// since they were actually copied as opposed to operations.MoveFile()'d.
			// the size/time/hash info is therefore fresher on the renames
			// but we'll settle for the original if we have to.
			if srcList.has(newName) {
				new = srcList.get(newName)
			} else if srcList.has(oppositeName) {
				new = srcList.get(oppositeName)
			} else if srcList.has(file) {
				new = srcList.get(file)
			} else {
				if err := filterRecheck.AddFile(file); err != nil {
					fs.Debugf(file, "error adding file to recheck filter: %v", err)
				}
			}
			srcList.put(newName, new.size, new.time, new.hash, new.id, new.flags)
			dstList.put(newName, new.size, ConvertPrecision(new.time, src), new.hash, new.id, new.flags)
			srcList.remove(file)
			dstList.remove(file)
		}
	}

	// recheck the ones we skipped because they were equal
	// we never got their info because they were never synced.
	// TODO: add flag to skip this for people who don't care and would rather avoid?
	if queues.renameSkipped.NotEmpty() {
		skippedList := queues.renameSkipped.ToList()
		for _, file := range skippedList {
			if err := filterRecheck.AddFile(file); err != nil {
				fs.Debugf(file, "error adding file to recheck filter: %v", err)
			}
		}
	}

	if filterRecheck.HaveFilesFrom() {
		b.recheck(ctxRecheck, src, dst, srcList, dstList)
	}

	// update files
	err = srcList.save(ctx, srcListing)
	if err != nil {
		b.abort = true
	}
	err = dstList.save(ctx, dstListing)
	if err != nil {
		b.abort = true
	}

	return err
}

// recheck the ones we're not sure about
func (b *bisyncRun) recheck(ctxRecheck context.Context, src, dst fs.Fs, srcList, dstList *fileList) {
	var srcObjs []fs.Object
	var dstObjs []fs.Object

	if err := operations.ListFn(ctxRecheck, src, func(obj fs.Object) {
		srcObjs = append(srcObjs, obj)
	}); err != nil {
		fs.Debugf(src, "error recchecking src obj: %v", err)
	}
	if err := operations.ListFn(ctxRecheck, dst, func(obj fs.Object) {
		dstObjs = append(dstObjs, obj)
	}); err != nil {
		fs.Debugf(dst, "error recchecking dst obj: %v", err)
	}

	putObj := func(obj fs.Object, f fs.Fs, list *fileList) {
		hashVal := ""
		if !b.opt.IgnoreListingChecksum {
			hashType := f.Hashes().GetOne()
			if hashType != hash.None {
				hashVal, _ = obj.Hash(ctxRecheck, hashType)
			}
		}
		list.put(obj.Remote(), obj.Size(), obj.ModTime(ctxRecheck), hashVal, "-", "-")
	}

	for _, srcObj := range srcObjs {
		fs.Debugf(srcObj, "rechecking")
		for _, dstObj := range dstObjs {
			if srcObj.Remote() == dstObj.Remote() {
				if operations.Equal(ctxRecheck, srcObj, dstObj) || b.opt.DryRun {
					putObj(srcObj, src, srcList)
					putObj(dstObj, dst, dstList)
				} else {
					fs.Infof(srcObj, "files not equal on recheck: %v %v", srcObj, dstObj)
				}
			}
		}
	}
}
