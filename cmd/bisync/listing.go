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
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/pkg/errors"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/hash"
	"github.com/rclone/rclone/fs/walk"
)

// ListingHeader defines first line of a listing
const ListingHeader = "# bisync listing v1 from"

// lineRegex and lineFormat define listing line format
//
//   flags <- size -> <- hash -> id <------------ modtime -----------> "<----- remote"
//   -        3009805 md5:xxxxxx -  2006-01-02T15:04:05.000000000-0700 "12 - Wait.mp3"
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
	size int64
	time time.Time
	hash string
	id   string
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
	return found
}

func (ls *fileList) get(file string) *fileInfo {
	return ls.info[file]
}

func (ls *fileList) put(file string, size int64, time time.Time, hash, id string) {
	fi := ls.get(file)
	if fi != nil {
		fi.size = size
		fi.time = time
	} else {
		fi = &fileInfo{
			size: size,
			time: time,
			hash: hash,
			id:   id,
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

// save will save listing to a file.
func (ls *fileList) save(ctx context.Context, listing string) error {
	file, err := os.Create(listing)
	if err != nil {
		return err
	}

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

		flags := "-"
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

		if flags != "-" || id != "-" || sizeErr != nil || timeErr != nil || hashErr != nil || nameErr != nil {
			fs.Logf(listing, "Ignoring incorrect line: %q", line)
			continue
		}

		if ls.has(nameVal) {
			fs.Logf(listing, "Duplicate line (keeping latest): %q", line)
			if ls.afterTime(nameVal, timeVal) {
				continue
			}
		}

		ls.put(nameVal, sizeVal, timeVal.In(TZ), hashVal, id)
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
	return "", "", errors.Errorf("invalid hash %q", str)
}

// makeListing will produce listing from directory tree and write it to a file
func (b *bisyncRun) makeListing(ctx context.Context, f fs.Fs, listing string) (ls *fileList, err error) {
	ci := fs.GetConfig(ctx)
	depth := ci.MaxDepth
	hashType := hash.None
	if !ci.IgnoreChecksum {
		// Currently bisync just honors --ignore-checksum
		// TODO add full support for checksums and related flags
		hashType = f.Hashes().GetOne()
	}
	ls = newFileList()
	ls.hash = hashType
	var lock sync.Mutex
	err = walk.ListR(ctx, f, "", false, depth, walk.ListObjects, func(entries fs.DirEntries) error {
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
			id := "" // TODO
			lock.Lock()
			ls.put(o.Remote(), o.Size(), time, hashVal, id)
			lock.Unlock()
			//tr.Done(ctx, nil) // TODO
		})
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
	return errors.Errorf("empty %s listing: %s", msg, listing)
}
