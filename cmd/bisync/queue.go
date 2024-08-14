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
	"github.com/rclone/rclone/fs/accounting"
	"github.com/rclone/rclone/fs/filter"
	"github.com/rclone/rclone/fs/hash"
	"github.com/rclone/rclone/fs/operations"
	"github.com/rclone/rclone/fs/sync"
	"github.com/rclone/rclone/lib/terminal"
)

// Results represents a pair of synced files, as reported by the LoggerFn
// Bisync uses this to determine what happened during the sync, and modify the listings accordingly
type Results struct {
	Src      string
	Dst      string
	Name     string
	AltName  string
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
	Origin   string
}

// ResultsSlice is a slice of Results (obviously)
type ResultsSlice []Results

func (rs *ResultsSlice) has(name string) bool {
	for _, r := range *rs {
		if r.Name == name {
			return true
		}
	}
	return false
}

var (
	logger                = operations.NewLoggerOpt()
	lock                  mutex.Mutex
	once                  mutex.Once
	ignoreListingChecksum bool
	ignoreListingModtime  bool
	hashTypes             map[string]hash.Type
	queueCI               *fs.ConfigInfo
)

// allows us to get the right hashtype during the LoggerFn without knowing whether it's Path1/Path2
func getHashType(fname string) hash.Type {
	ht, ok := hashTypes[fname]
	if ok {
		return ht
	}
	return hash.None
}

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

// returns the opposite side's name, only if different
func altName(name string, src, dst fs.DirEntry) string {
	if src != nil && dst != nil {
		if src.Remote() != dst.Remote() {
			switch name {
			case src.Remote():
				return dst.Remote()
			case dst.Remote():
				return src.Remote()
			}
		}
	}
	return ""
}

// WriteResults is Bisync's LoggerFn
func WriteResults(ctx context.Context, sigil operations.Sigil, src, dst fs.DirEntry, err error) {
	lock.Lock()
	defer lock.Unlock()

	opt := operations.GetLoggerOpt(ctx)
	result := Results{
		Sigil:  sigil,
		Src:    FsPathIfAny(src),
		Dst:    FsPathIfAny(dst),
		Err:    err,
		Origin: "sync",
	}

	result.Winner = operations.WinningSide(ctx, sigil, src, dst, err)

	fss := []fs.DirEntry{src, dst}
	for i, side := range fss {

		result.Name = resultName(result, side, src, dst)
		result.AltName = altName(result.Name, src, dst)
		result.IsSrc = i == 0
		result.IsDst = i == 1
		result.Flags = "-"
		if side != nil {
			result.Size = side.Size()
			if !ignoreListingModtime {
				result.Modtime = side.ModTime(ctx).In(TZ)
			}
			if !ignoreListingChecksum {
				sideObj, ok := side.(fs.ObjectInfo)
				if ok {
					result.Hash, _ = sideObj.Hash(ctx, getHashType(sideObj.Fs().Name()))
					result.Hash, _ = tryDownloadHash(ctx, sideObj, result.Hash)
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
			result.Size = -1
		}

		prettyprint(result, "writing result", fs.LogLevelDebug)
		if result.Size < 0 && result.Flags != "d" && ((queueCI.CheckSum && !downloadHash) || queueCI.SizeOnly) {
			once.Do(func() {
				fs.Logf(result.Name, Color(terminal.YellowFg, "Files of unknown size (such as Google Docs) do not sync reliably with --checksum or --size-only. Consider using modtime instead (the default) or --drive-skip-gdocs")) //nolint:govet
			})
		}

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
		prettyprint(r, "result", fs.LogLevelDebug)
		slice = append(slice, r)
	}
	return slice
}

// for setup code shared by both fastCopy and resyncDir
func (b *bisyncRun) preCopy(ctx context.Context) context.Context {
	queueCI = fs.GetConfig(ctx)
	ignoreListingChecksum = b.opt.IgnoreListingChecksum
	ignoreListingModtime = !b.opt.Compare.Modtime
	hashTypes = map[string]hash.Type{
		b.fs1.Name(): b.opt.Compare.HashType1,
		b.fs2.Name(): b.opt.Compare.HashType2,
	}
	logger.LoggerFn = WriteResults
	overridingEqual := false
	if (b.opt.Compare.Modtime && b.opt.Compare.Checksum) || b.opt.Compare.DownloadHash {
		overridingEqual = true
		fs.Debugf(nil, "overriding equal")
		// otherwise impossible in Sync, so override Equal
		ctx = b.EqualFn(ctx)
	}
	if b.opt.ResyncMode == PreferOlder || b.opt.ResyncMode == PreferLarger || b.opt.ResyncMode == PreferSmaller {
		overridingEqual = true
		fs.Debugf(nil, "overriding equal")
		ctx = b.EqualFn(ctx)
	}
	ctxCopyLogger := operations.WithSyncLogger(ctx, logger)
	if b.opt.Compare.Checksum && (b.opt.Compare.NoSlowHash || b.opt.Compare.SlowHashSyncOnly) && b.opt.Compare.SlowHashDetected {
		// set here in case !b.opt.Compare.Modtime
		queueCI = fs.GetConfig(ctxCopyLogger)
		if b.opt.Compare.NoSlowHash {
			queueCI.CheckSum = false
		}
		if b.opt.Compare.SlowHashSyncOnly && !overridingEqual {
			queueCI.CheckSum = true
		}
	}
	return ctxCopyLogger
}

func (b *bisyncRun) fastCopy(ctx context.Context, fsrc, fdst fs.Fs, files bilib.Names, queueName string) ([]Results, error) {
	if b.InGracefulShutdown {
		return nil, nil
	}
	ctx = b.preCopy(ctx)
	if err := b.saveQueue(files, queueName); err != nil {
		return nil, err
	}

	ctxCopy, filterCopy := filter.AddConfig(b.opt.setDryRun(ctx))
	for _, file := range files.ToList() {
		if err := filterCopy.AddFile(file); err != nil {
			return nil, err
		}
		alias := b.aliases.Alias(file)
		if alias != file {
			if err := filterCopy.AddFile(alias); err != nil {
				return nil, err
			}
		}
	}

	b.SyncCI = fs.GetConfig(ctxCopy)      // allows us to request graceful shutdown
	accounting.MaxCompletedTransfers = -1 // we need a complete list in the event of graceful shutdown
	ctxCopy, b.CancelSync = context.WithCancel(ctxCopy)
	b.testFn()
	err := sync.Sync(ctxCopy, fdst, fsrc, b.opt.CreateEmptySrcDirs)
	prettyprint(logger, "logger", fs.LogLevelDebug)

	getResults := ReadResults(logger.JSON)
	fs.Debugf(nil, "Got %v results for %v", len(getResults), queueName)

	lineFormat := "%s %8d %s %s %s %q\n"
	for _, result := range getResults {
		fs.Debugf(nil, lineFormat, result.Flags, result.Size, result.Hash, "", result.Modtime, result.Name)
	}

	return getResults, err
}

func (b *bisyncRun) retryFastCopy(ctx context.Context, fsrc, fdst fs.Fs, files bilib.Names, queueName string, results []Results, err error) ([]Results, error) {
	ci := fs.GetConfig(ctx)
	if err != nil && b.opt.Resilient && !b.InGracefulShutdown && ci.Retries > 1 {
		for tries := 1; tries <= ci.Retries; tries++ {
			fs.Logf(queueName, Color(terminal.YellowFg, "Received error: %v - retrying as --resilient is set. Retry %d/%d"), err, tries, ci.Retries)
			accounting.GlobalStats().ResetErrors()
			if retryAfter := accounting.GlobalStats().RetryAfter(); !retryAfter.IsZero() {
				d := time.Until(retryAfter)
				if d > 0 {
					fs.Logf(nil, "Received retry after error - sleeping until %s (%v)", retryAfter.Format(time.RFC3339Nano), d)
					time.Sleep(d)
				}
			}
			if ci.RetriesInterval > 0 {
				naptime(ci.RetriesInterval)
			}
			results, err = b.fastCopy(ctx, fsrc, fdst, files, queueName)
			if err == nil || b.InGracefulShutdown {
				return results, err
			}
		}
	}
	return results, err
}

func (b *bisyncRun) resyncDir(ctx context.Context, fsrc, fdst fs.Fs) ([]Results, error) {
	ctx = b.preCopy(ctx)

	err := sync.CopyDir(ctx, fdst, fsrc, b.opt.CreateEmptySrcDirs)
	prettyprint(logger, "logger", fs.LogLevelDebug)

	getResults := ReadResults(logger.JSON)
	fs.Debugf(nil, "Got %v results for %v", len(getResults), "resync")

	return getResults, err
}

// operation should be "make" or "remove"
func (b *bisyncRun) syncEmptyDirs(ctx context.Context, dst fs.Fs, candidates bilib.Names, dirsList *fileList, results *[]Results, operation string) {
	if b.InGracefulShutdown {
		return
	}
	fs.Debugf(nil, "syncing empty dirs")
	if b.opt.CreateEmptySrcDirs && (!b.opt.Resync || operation == "make") {

		candidatesList := candidates.ToList()
		if operation == "remove" {
			// reverse the sort order to ensure we remove subdirs before parent dirs
			sort.Sort(sort.Reverse(sort.StringSlice(candidatesList)))
		}

		for _, s := range candidatesList {
			var direrr error
			if dirsList.has(s) { // make sure it's a dir, not a file
				r := Results{}
				r.Name = s
				r.Size = -1
				r.Modtime = dirsList.getTime(s).In(time.UTC)
				r.Flags = "d"
				r.Err = nil
				r.Origin = "syncEmptyDirs"
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

func naptime(totalWait time.Duration) {
	expireTime := time.Now().Add(totalWait)
	fs.Logf(nil, "will retry in %v at %v", totalWait, expireTime.Format("2006-01-02 15:04:05 MST"))
	for i := 0; time.Until(expireTime) > 0; i++ {
		if i > 0 && i%10 == 0 {
			fs.Infof(nil, Color(terminal.Dim, "retrying in %v..."), time.Until(expireTime).Round(1*time.Second))
		} else {
			fs.Debugf(nil, Color(terminal.Dim, "retrying in %v..."), time.Until(expireTime).Round(1*time.Second))
		}
		time.Sleep(1 * time.Second)
	}
}
