package bisync

import (
	"context"
	"sync"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/accounting"
	"github.com/rclone/rclone/fs/hash"
	"github.com/rclone/rclone/fs/march"
)

var ls1 = newFileList()
var ls2 = newFileList()
var err error
var firstErr error
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
		b.abort = true
	}

	// save files
	err = ls1.save(ctx, b.newListing1)
	if err != nil {
		b.abort = true
	}
	err = ls2.save(ctx, b.newListing2)
	if err != nil {
		b.abort = true
	}

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

	hashType1 := hash.None
	hashType2 := hash.None
	if !b.opt.IgnoreListingChecksum {
		// Currently bisync just honors --ignore-listing-checksum
		// (note that this is different from --ignore-checksum)
		// TODO add full support for checksums and related flags
		hashType1 = b.fs1.Hashes().GetOne()
		hashType2 = b.fs2.Hashes().GetOne()
	}

	ls1.hash = hashType1
	ls2.hash = hashType2
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
	time := o.ModTime(marchCtx).In(TZ)
	id := ""     // TODO
	flags := "-" // "-" for a file and "d" for a directory
	marchLsLock.Lock()
	ls.put(o.Remote(), o.Size(), time, hashVal, id, flags)
	marchLsLock.Unlock()
}

func (b *bisyncRun) ForDir(o fs.Directory, isPath1 bool) {
	tr := accounting.Stats(marchCtx).NewCheckingTransfer(o, "listing dir - "+whichPath(isPath1))
	defer func() {
		tr.Done(marchCtx, nil)
	}()
	ls := whichLs(isPath1)
	time := o.ModTime(marchCtx).In(TZ)
	id := ""     // TODO
	flags := "d" // "-" for a file and "d" for a directory
	marchLsLock.Lock()
	//record size as 0 instead of -1, so bisync doesn't think it's a google doc
	ls.put(o.Remote(), 0, time, "", id, flags)
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
