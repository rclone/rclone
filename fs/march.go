package fs

import (
	"sync"

	"golang.org/x/net/context"
)

// march traverses two Fs simultaneously, calling walker for each match
type march struct {
	// parameters
	ctx      context.Context
	fdst     Fs
	fsrc     Fs
	dir      string
	callback marcher
	// internal state
	srcListDir listDirFn // function to call to list a directory in the src
	dstListDir listDirFn // function to call to list a directory in the dst
}

// marcher is called on each match
type marcher interface {
	// SrcOnly is called for a DirEntry found only in the source
	SrcOnly(src DirEntry) (recurse bool)
	// DstOnly is called for a DirEntry found only in the destination
	DstOnly(dst DirEntry) (recurse bool)
	// Match is called for a DirEntry found both in the source and destination
	Match(dst, src DirEntry) (recurse bool)
}

// newMarch sets up a march over fsrc, and fdst calling back callback for each match
func newMarch(ctx context.Context, fdst, fsrc Fs, dir string, callback marcher) *march {
	m := &march{
		ctx:      ctx,
		fdst:     fdst,
		fsrc:     fsrc,
		dir:      dir,
		callback: callback,
	}
	m.srcListDir = m.makeListDir(fsrc, false)
	m.dstListDir = m.makeListDir(fdst, Config.Filter.DeleteExcluded)
	return m
}

// list a directory into entries, err
type listDirFn func(dir string) (entries DirEntries, err error)

// makeListDir makes a listing function for the given fs and includeAll flags
func (m *march) makeListDir(f Fs, includeAll bool) listDirFn {
	if !Config.UseListR || f.Features().ListR == nil {
		return func(dir string) (entries DirEntries, err error) {
			return ListDirSorted(f, includeAll, dir)
		}
	}
	var (
		mu      sync.Mutex
		started bool
		dirs    DirTree
		dirsErr error
	)
	return func(dir string) (entries DirEntries, err error) {
		mu.Lock()
		defer mu.Unlock()
		if !started {
			dirs, dirsErr = NewDirTree(f, m.dir, includeAll, Config.MaxDepth)
			started = true
		}
		if dirsErr != nil {
			return nil, dirsErr
		}
		entries, ok := dirs[dir]
		if !ok {
			err = ErrorDirNotFound
		} else {
			delete(dirs, dir)
		}
		return entries, err
	}
}

// listDirJob describe a directory listing that needs to be done
type listDirJob struct {
	srcRemote string
	dstRemote string
	srcDepth  int
	dstDepth  int
	noSrc     bool
	noDst     bool
}

// run starts the matching process off
func (m *march) run() {
	srcDepth := Config.MaxDepth
	if srcDepth < 0 {
		srcDepth = MaxLevel
	}
	dstDepth := srcDepth
	if Config.Filter.DeleteExcluded {
		dstDepth = MaxLevel
	}

	// Start some directory listing go routines
	var wg sync.WaitGroup         // sync closing of go routines
	var traversing sync.WaitGroup // running directory traversals
	in := make(chan listDirJob, Config.Checkers)
	for i := 0; i < Config.Checkers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-m.ctx.Done():
					return
				case job, ok := <-in:
					if !ok {
						return
					}
					jobs := m.processJob(job)
					if len(jobs) > 0 {
						traversing.Add(len(jobs))
						go func() {
							// Now we have traversed this directory, send these
							// jobs off for traversal in the background
							for _, newJob := range jobs {
								in <- newJob
							}
						}()
					}
					traversing.Done()
				}
			}
		}()
	}

	// Start the process
	traversing.Add(1)
	in <- listDirJob{
		srcRemote: m.dir,
		srcDepth:  srcDepth - 1,
		dstRemote: m.dir,
		dstDepth:  dstDepth - 1,
	}
	traversing.Wait()
	close(in)
	wg.Wait()
}

// Check to see if the context has been cancelled
func (m *march) aborting() bool {
	select {
	case <-m.ctx.Done():
		return true
	default:
	}
	return false
}

type matchPair struct {
	src, dst DirEntry
}

// Process the two sorted listings, matching up the items in the two
// sorted slices
//
// Into srcOnly go Entries which only exist in the srcList
// Into dstOnly go Entries which only exist in the dstList
// Into matches go matchPair's of src and dst which have the same name
//
// This checks for duplicates and checks the list is sorted.
func matchListings(srcList, dstList DirEntries) (srcOnly DirEntries, dstOnly DirEntries, matches []matchPair) {
	for iSrc, iDst := 0, 0; ; iSrc, iDst = iSrc+1, iDst+1 {
		var src, dst DirEntry
		var srcRemote, dstRemote string
		if iSrc < len(srcList) {
			src = srcList[iSrc]
			srcRemote = src.Remote()
		}
		if iDst < len(dstList) {
			dst = dstList[iDst]
			dstRemote = dst.Remote()
		}
		if src == nil && dst == nil {
			break
		}
		if src != nil && iSrc > 0 {
			prev := srcList[iSrc-1].Remote()
			if srcRemote == prev {
				Logf(src, "Duplicate %s found in source - ignoring", DirEntryType(src))
				src = nil // ignore the src
			} else if srcRemote < prev {
				Errorf(src, "Out of order listing in source")
				src = nil // ignore the src
			}
		}
		if dst != nil && iDst > 0 {
			prev := dstList[iDst-1].Remote()
			if dstRemote == prev {
				Logf(dst, "Duplicate %s found in destination - ignoring", DirEntryType(dst))
				dst = nil // ignore the dst
			} else if dstRemote < prev {
				Errorf(dst, "Out of order listing in destination")
				dst = nil // ignore the dst
			}
		}
		if src != nil && dst != nil {
			if srcRemote < dstRemote {
				dst = nil
				iDst-- // retry the dst
			} else if srcRemote > dstRemote {
				src = nil
				iSrc-- // retry the src
			}
		}
		// Debugf(nil, "src = %v, dst = %v", src, dst)
		switch {
		case src == nil && dst == nil:
			// do nothing
		case src == nil:
			dstOnly = append(dstOnly, dst)
		case dst == nil:
			srcOnly = append(srcOnly, src)
		default:
			matches = append(matches, matchPair{src: src, dst: dst})
		}
	}
	return
}

// processJob processes a listDirJob listing the source and
// destination directories, comparing them and returning a slice of
// more jobs
//
// returns errors using processError
func (m *march) processJob(job listDirJob) (jobs []listDirJob) {
	var (
		srcList, dstList       DirEntries
		srcListErr, dstListErr error
		wg                     sync.WaitGroup
	)

	// List the src and dst directories
	if !job.noSrc {
		wg.Add(1)
		go func() {
			defer wg.Done()
			srcList, srcListErr = m.srcListDir(job.srcRemote)
		}()
	}
	if !job.noDst {
		wg.Add(1)
		go func() {
			defer wg.Done()
			dstList, dstListErr = m.dstListDir(job.dstRemote)
		}()
	}

	// Wait for listings to complete and report errors
	wg.Wait()
	if srcListErr != nil {
		Errorf(job.srcRemote, "error reading source directory: %v", srcListErr)
		Stats.Error()
		return nil
	}
	if dstListErr == ErrorDirNotFound {
		// Copy the stuff anyway
	} else if dstListErr != nil {
		Errorf(job.dstRemote, "error reading destination directory: %v", dstListErr)
		Stats.Error()
		return nil
	}

	// Work out what to do and do it
	srcOnly, dstOnly, matches := matchListings(srcList, dstList)
	for _, src := range srcOnly {
		if m.aborting() {
			return nil
		}
		recurse := m.callback.SrcOnly(src)
		if recurse && job.srcDepth > 0 {
			jobs = append(jobs, listDirJob{
				srcRemote: src.Remote(),
				srcDepth:  job.srcDepth - 1,
				noDst:     true,
			})
		}

	}
	for _, dst := range dstOnly {
		if m.aborting() {
			return nil
		}
		recurse := m.callback.DstOnly(dst)
		if recurse && job.dstDepth > 0 {
			jobs = append(jobs, listDirJob{
				dstRemote: dst.Remote(),
				dstDepth:  job.dstDepth - 1,
				noSrc:     true,
			})
		}
	}
	for _, match := range matches {
		if m.aborting() {
			return nil
		}
		recurse := m.callback.Match(match.dst, match.src)
		if recurse && job.srcDepth > 0 && job.dstDepth > 0 {
			jobs = append(jobs, listDirJob{
				srcRemote: match.src.Remote(),
				dstRemote: match.dst.Remote(),
				srcDepth:  job.srcDepth - 1,
				dstDepth:  job.dstDepth - 1,
			})
		}
	}
	return jobs
}
