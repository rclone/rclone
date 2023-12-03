package bisync

import (
	"context"
	"sync"
	"time"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/accounting"
	"github.com/rclone/rclone/fs/filter"
	"github.com/rclone/rclone/fs/hash"
	"github.com/rclone/rclone/fs/march"
)

var ls1 = newFileList()
var ls2 = newFileList()
var err error
var firstErr error
var marchAliasLock sync.Mutex
var marchLsLock sync.Mutex
var marchErrLock sync.Mutex
var marchCtx context.Context

func (b *bisyncRun) makeMarchListing(ctx context.Context) (*fileList, *fileList, error) {
	ci := fs.GetConfig(ctx)
	marchCtx = ctx
	b.setupListing()
	fs.Debugf(b, "starting to march!")

	// set up a march over fdst (Path2) and fsrc (Path1)
	m := &march.March{
		Ctx:                    ctx,
		Fdst:                   b.fs2,
		Fsrc:                   b.fs1,
		Dir:                    "",
		NoTraverse:             false,
		Callback:               b,
		DstIncludeAll:          false,
		NoCheckDest:            false,
		NoUnicodeNormalization: ci.NoUnicodeNormalization,
	}
	err = m.Run(ctx)

	fs.Debugf(b, "march completed. err: %v", err)
	if err == nil {
		err = firstErr
	}
	if err != nil {
		b.handleErr("march", "error during march", err, true, true)
		b.abort = true
		return ls1, ls2, err
	}

	// save files
	if b.opt.Compare.DownloadHash && ls1.hash == hash.None {
		ls1.hash = hash.MD5
	}
	if b.opt.Compare.DownloadHash && ls2.hash == hash.None {
		ls2.hash = hash.MD5
	}
	err = ls1.save(ctx, b.newListing1)
	b.handleErr(ls1, "error saving ls1 from march", err, true, true)
	err = ls2.save(ctx, b.newListing2)
	b.handleErr(ls2, "error saving ls2 from march", err, true, true)

	return ls1, ls2, err
}

// SrcOnly have an object which is on path1 only
func (b *bisyncRun) SrcOnly(o fs.DirEntry) (recurse bool) {
	fs.Debugf(o, "path1 only")
	b.parse(o, true)
	return isDir(o)
}

// DstOnly have an object which is on path2 only
func (b *bisyncRun) DstOnly(o fs.DirEntry) (recurse bool) {
	fs.Debugf(o, "path2 only")
	b.parse(o, false)
	return isDir(o)
}

// Match is called when object exists on both path1 and path2 (whether equal or not)
func (b *bisyncRun) Match(ctx context.Context, o2, o1 fs.DirEntry) (recurse bool) {
	fs.Debugf(o1, "both path1 and path2")
	marchAliasLock.Lock()
	b.aliases.Add(o1.Remote(), o2.Remote())
	marchAliasLock.Unlock()
	b.parse(o1, true)
	b.parse(o2, false)
	return isDir(o1)
}

func isDir(e fs.DirEntry) bool {
	switch x := e.(type) {
	case fs.Object:
		fs.Debugf(x, "is Object")
		return false
	case fs.Directory:
		fs.Debugf(x, "is Dir")
		return true
	default:
		fs.Debugf(e, "is unknown")
	}
	return false
}

func (b *bisyncRun) parse(e fs.DirEntry, isPath1 bool) {
	switch x := e.(type) {
	case fs.Object:
		b.ForObject(x, isPath1)
	case fs.Directory:
		if b.opt.CreateEmptySrcDirs {
			b.ForDir(x, isPath1)
		}
	default:
		fs.Debugf(e, "is unknown")
	}
}

func (b *bisyncRun) setupListing() {
	ls1 = newFileList()
	ls2 = newFileList()

	// note that --ignore-listing-checksum is different from --ignore-checksum
	// and we already checked it when we set b.opt.Compare.HashType1 and 2
	ls1.hash = b.opt.Compare.HashType1
	ls2.hash = b.opt.Compare.HashType2
}

func (b *bisyncRun) ForObject(o fs.Object, isPath1 bool) {
	tr := accounting.Stats(marchCtx).NewCheckingTransfer(o, "listing file - "+whichPath(isPath1))
	defer func() {
		tr.Done(marchCtx, nil)
	}()
	var (
		hashVal string
		hashErr error
	)
	ls := whichLs(isPath1)
	hashType := ls.hash
	if hashType != hash.None {
		hashVal, hashErr = o.Hash(marchCtx, hashType)
		marchErrLock.Lock()
		if firstErr == nil {
			firstErr = hashErr
		}
		marchErrLock.Unlock()
	}
	hashVal, hashErr = tryDownloadHash(marchCtx, o, hashVal)
	marchErrLock.Lock()
	if firstErr == nil {
		firstErr = hashErr
	}
	if firstErr != nil {
		b.handleErr(hashType, "error hashing during march", firstErr, false, true)
	}
	marchErrLock.Unlock()

	var modtime time.Time
	if b.opt.Compare.Modtime {
		modtime = o.ModTime(marchCtx).In(TZ)
	}
	id := ""     // TODO: ID(o)
	flags := "-" // "-" for a file and "d" for a directory
	marchLsLock.Lock()
	ls.put(o.Remote(), o.Size(), modtime, hashVal, id, flags)
	marchLsLock.Unlock()
}

func (b *bisyncRun) ForDir(o fs.Directory, isPath1 bool) {
	tr := accounting.Stats(marchCtx).NewCheckingTransfer(o, "listing dir - "+whichPath(isPath1))
	defer func() {
		tr.Done(marchCtx, nil)
	}()
	ls := whichLs(isPath1)
	var modtime time.Time
	if b.opt.Compare.Modtime {
		modtime = o.ModTime(marchCtx).In(TZ)
	}
	id := ""     // TODO
	flags := "d" // "-" for a file and "d" for a directory
	marchLsLock.Lock()
	ls.put(o.Remote(), -1, modtime, "", id, flags)
	marchLsLock.Unlock()
}

func whichLs(isPath1 bool) *fileList {
	ls := ls1
	if !isPath1 {
		ls = ls2
	}
	return ls
}

func whichPath(isPath1 bool) string {
	s := "Path1"
	if !isPath1 {
		s = "Path2"
	}
	return s
}

func (b *bisyncRun) findCheckFiles(ctx context.Context) (*fileList, *fileList, error) {
	ctxCheckFile, filterCheckFile := filter.AddConfig(ctx)
	b.handleErr(b.opt.CheckFilename, "error adding CheckFilename to filter", filterCheckFile.Add(true, b.opt.CheckFilename), true, true)
	b.handleErr(b.opt.CheckFilename, "error adding ** exclusion to filter", filterCheckFile.Add(false, "**"), true, true)
	ci := fs.GetConfig(ctxCheckFile)
	marchCtx = ctxCheckFile

	b.setupListing()
	fs.Debugf(b, "starting to march!")

	// set up a march over fdst (Path2) and fsrc (Path1)
	m := &march.March{
		Ctx:                    ctxCheckFile,
		Fdst:                   b.fs2,
		Fsrc:                   b.fs1,
		Dir:                    "",
		NoTraverse:             false,
		Callback:               b,
		DstIncludeAll:          false,
		NoCheckDest:            false,
		NoUnicodeNormalization: ci.NoUnicodeNormalization,
	}
	err = m.Run(ctxCheckFile)

	fs.Debugf(b, "march completed. err: %v", err)
	if err == nil {
		err = firstErr
	}
	if err != nil {
		b.handleErr("march", "error during findCheckFiles", err, true, true)
		b.abort = true
	}

	return ls1, ls2, err
}

// ID returns the ID of the Object if known, or "" if not
func ID(o fs.Object) string {
	do, ok := o.(fs.IDer)
	if !ok {
		return ""
	}
	return do.ID()
}
