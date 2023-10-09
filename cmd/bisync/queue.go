package bisync

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	mutex "sync" // renamed as "sync" already in use
	"time"

	"github.com/rclone/rclone/cmd/bisync/bilib"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/filter"
	"github.com/rclone/rclone/fs/operations"
	"github.com/rclone/rclone/fs/sync"
)

// Results represents a pair of synced files, as reported by the LoggerFn
// Bisync uses this to determine what happened during the sync, and modify the listings accordingly
type Results struct {
	Src      string
	Dst      string
	Name     string
	Size     int64
	Modtime  time.Time
	Hash     string
	Flags    string
	Sigil    operations.Sigil
	Err      error
	Winner   operations.Winner
	IsWinner bool
	IsSrc    bool
	IsDst    bool
}

var logger = operations.NewLoggerOpt()
var lock mutex.Mutex
var ignoreListingChecksum bool

// FsPathIfAny handles type assertions and returns a formatted bilib.FsPath if valid, otherwise ""
func FsPathIfAny(x fs.DirEntry) string {
	obj, ok := x.(fs.Object)
	if x != nil && ok {
		return bilib.FsPath(obj.Fs())
	}
	return ""
}

func resultName(result Results, side, src, dst fs.DirEntry) string {
	if side != nil {
		return side.Remote()
	} else if result.IsSrc && dst != nil {
		return dst.Remote()
	} else if src != nil {
		return src.Remote()
	}
	return ""
}

// WriteResults is Bisync's LoggerFn
func WriteResults(ctx context.Context, sigil operations.Sigil, src, dst fs.DirEntry, err error) {
	lock.Lock()
	defer lock.Unlock()

	opt := operations.GetLoggerOpt(ctx)
	result := Results{
		Sigil: sigil,
		Src:   FsPathIfAny(src),
		Dst:   FsPathIfAny(dst),
		Err:   err,
	}

	result.Winner = operations.WinningSide(ctx, sigil, src, dst, err)

	fss := []fs.DirEntry{src, dst}
	for i, side := range fss {

		result.Name = resultName(result, side, src, dst)
		result.IsSrc = i == 0
		result.IsDst = i == 1
		result.Flags = "-"
		if side != nil {
			result.Size = side.Size()
			result.Modtime = side.ModTime(ctx).In(time.UTC)

			if !ignoreListingChecksum {
				sideObj, ok := side.(fs.ObjectInfo)
				if ok {
					result.Hash, _ = sideObj.Hash(ctx, sideObj.Fs().Hashes().GetOne())
				}
			}
		}
		result.IsWinner = result.Winner.Obj == side

		// used during resync only
		if err == fs.ErrorIsDir {
			if src != nil {
				result.Src = src.Remote()
				result.Name = src.Remote()
			} else {
				result.Dst = dst.Remote()
				result.Name = dst.Remote()
			}
			result.Flags = "d"
			result.Size = 0
		}

		fs.Debugf(nil, "writing result: %v", result)
		err := json.NewEncoder(opt.JSON).Encode(result)
		if err != nil {
			fs.Errorf(result, "Error encoding JSON: %v", err)
		}
	}
}

// ReadResults decodes the JSON data from WriteResults
func ReadResults(results io.Reader) []Results {
	dec := json.NewDecoder(results)
	var slice []Results
	for {
		var r Results
		if err := dec.Decode(&r); err == io.EOF {
			break
		}
		fs.Debugf(nil, "result: %v", r)
		slice = append(slice, r)
	}
	return slice
}

func (b *bisyncRun) fastCopy(ctx context.Context, fsrc, fdst fs.Fs, files bilib.Names, queueName string, altNames bilib.Names) ([]Results, error) {
	if err := b.saveQueue(files, queueName); err != nil {
		return nil, err
	}

	ctxCopy, filterCopy := filter.AddConfig(b.opt.setDryRun(ctx))
	for _, file := range files.ToList() {
		if err := filterCopy.AddFile(file); err != nil {
			return nil, err
		}
	}
	if altNames.NotEmpty() {
		for _, file := range altNames.ToList() {
			if err := filterCopy.AddFile(file); err != nil {
				return nil, err
			}
		}
	}

	ignoreListingChecksum = b.opt.IgnoreListingChecksum
	logger.LoggerFn = WriteResults
	ctxCopyLogger := operations.WithSyncLogger(ctxCopy, logger)
	var err error
	if b.opt.Resync {
		err = sync.CopyDir(ctxCopyLogger, fdst, fsrc, b.opt.CreateEmptySrcDirs)
	} else {
		b.testFn()
		err = sync.Sync(ctxCopyLogger, fdst, fsrc, b.opt.CreateEmptySrcDirs)
	}
	fs.Debugf(nil, "logger is: %v", logger)

	getResults := ReadResults(logger.JSON)
	fs.Debugf(nil, "Got %v results for %v", len(getResults), queueName)

	lineFormat := "%s %8d %s %s %s %q\n"
	for _, result := range getResults {
		fs.Debugf(nil, lineFormat, result.Flags, result.Size, result.Hash, "", result.Modtime, result.Name)
	}

	return getResults, err
}

func (b *bisyncRun) resyncDir(ctx context.Context, fsrc, fdst fs.Fs) ([]Results, error) {
	ignoreListingChecksum = b.opt.IgnoreListingChecksum
	logger.LoggerFn = WriteResults
	ctxCopyLogger := operations.WithSyncLogger(ctx, logger)
	err := sync.CopyDir(ctxCopyLogger, fdst, fsrc, b.opt.CreateEmptySrcDirs)
	fs.Debugf(nil, "logger is: %v", logger)

	getResults := ReadResults(logger.JSON)
	fs.Debugf(nil, "Got %v results for %v", len(getResults), "resync")

	return getResults, err
}

// operation should be "make" or "remove"
func (b *bisyncRun) syncEmptyDirs(ctx context.Context, dst fs.Fs, candidates bilib.Names, dirsList *fileList, results *[]Results, operation string) {
	if b.opt.CreateEmptySrcDirs && (!b.opt.Resync || operation == "make") {

		candidatesList := candidates.ToList()
		if operation == "remove" {
			// reverse the sort order to ensure we remove subdirs before parent dirs
			sort.Sort(sort.Reverse(sort.StringSlice(candidatesList)))
		}

		for _, s := range candidatesList {
			var direrr error
			if dirsList.has(s) { //make sure it's a dir, not a file
				r := Results{}
				r.Name = s
				r.Size = 0
				r.Modtime = dirsList.getTime(s).In(time.UTC)
				r.Flags = "d"
				r.Err = nil
				r.Winner = operations.Winner{ // note: Obj not set
					Side: "src",
					Err:  nil,
				}

				rSrc := r
				rDst := r
				rSrc.IsSrc = true
				rSrc.IsDst = false
				rDst.IsSrc = false
				rDst.IsDst = true
				rSrc.IsWinner = true
				rDst.IsWinner = false

				if operation == "remove" {
					// directories made empty by the sync will have already been deleted during the sync
					// this just catches the already-empty ones (excluded from sync by --files-from filter)
					direrr = operations.TryRmdir(ctx, dst, s)
					rSrc.Sigil = operations.MissingOnSrc
					rDst.Sigil = operations.MissingOnSrc
					rSrc.Dst = s
					rDst.Dst = s
					rSrc.Winner.Side = "none"
					rDst.Winner.Side = "none"
				} else if operation == "make" {
					direrr = operations.Mkdir(ctx, dst, s)
					rSrc.Sigil = operations.MissingOnDst
					rDst.Sigil = operations.MissingOnDst
					rSrc.Src = s
					rDst.Src = s
				} else {
					direrr = fmt.Errorf("invalid operation. Expected 'make' or 'remove', received '%q'", operation)
				}

				if direrr != nil {
					fs.Debugf(nil, "Error syncing directory: %v", direrr)
				} else {
					*results = append(*results, rSrc, rDst)
				}
			}
		}
	}
}

func (b *bisyncRun) saveQueue(files bilib.Names, jobName string) error {
	if !b.opt.SaveQueues {
		return nil
	}
	queueFile := fmt.Sprintf("%s.%s.que", b.basePath, jobName)
	return files.Save(queueFile)
}

func (b *bisyncRun) findAltNames(ctx context.Context, dst fs.Fs, queue bilib.Names, newListing string, altNames bilib.Names) {
	ci := fs.GetConfig(ctx)
	if queue.NotEmpty() && (!ci.NoUnicodeNormalization || ci.IgnoreCaseSync || b.fs1.Features().CaseInsensitive || b.fs2.Features().CaseInsensitive) {
		// search list for existing file that matches queueFile when normalized
		for _, queueFile := range queue.ToList() {
			normalizedName := ApplyTransforms(ctx, dst, queueFile)
			candidates, err := b.loadListing(newListing)
			if err != nil {
				fs.Errorf(candidates, "cannot read new listing: %v", err)
			}
			for _, filename := range candidates.list {
				if ApplyTransforms(ctx, dst, filename) == normalizedName && filename != queueFile {
					altNames.Add(filename) // original, not normalized
				}
			}
		}
	}
}
