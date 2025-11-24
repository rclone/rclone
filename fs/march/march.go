// Package march traverses two directories in lock step
package march

import (
	"cmp"
	"context"
	"fmt"
	"path"
	"slices"
	"strings"
	"sync"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/dirtree"
	"github.com/rclone/rclone/fs/filter"
	"github.com/rclone/rclone/fs/list"
	"github.com/rclone/rclone/fs/walk"
	"github.com/rclone/rclone/lib/transform"
	"golang.org/x/text/unicode/norm"
)

// matchTransformFn converts a name into a form which is used for
// comparison in matchListings.
type matchTransformFn func(name string) string

// list a directory into callback returning err
type listDirFn func(dir string, callback fs.ListRCallback) (err error)

// March holds the data used to traverse two Fs simultaneously,
// calling Callback for each match
type March struct {
	// parameters
	Ctx                    context.Context // context for background goroutines
	Fdst                   fs.Fs           // source Fs
	Fsrc                   fs.Fs           // dest Fs
	Dir                    string          // directory
	NoTraverse             bool            // don't traverse the destination
	SrcIncludeAll          bool            // don't include all files in the src
	DstIncludeAll          bool            // don't include all files in the destination
	Callback               Marcher         // object to call with results
	NoCheckDest            bool            // transfer all objects regardless without checking dst
	NoUnicodeNormalization bool            // don't normalize unicode characters in filenames
	// internal state
	srcListDir listDirFn // function to call to list a directory in the src
	dstListDir listDirFn // function to call to list a directory in the dst
	transforms []matchTransformFn
}

// Marcher is called on each match
type Marcher interface {
	// SrcOnly is called for a DirEntry found only in the source
	SrcOnly(src fs.DirEntry) (recurse bool)
	// DstOnly is called for a DirEntry found only in the destination
	DstOnly(dst fs.DirEntry) (recurse bool)
	// Match is called for a DirEntry found both in the source and destination
	Match(ctx context.Context, dst, src fs.DirEntry) (recurse bool)
}

// init sets up a march over opt.Fsrc, and opt.Fdst calling back callback for each match
// Note: this will flag filter-aware backends on the source side
func (m *March) init(ctx context.Context) {
	ci := fs.GetConfig(ctx)
	m.srcListDir = m.makeListDir(ctx, m.Fsrc, m.SrcIncludeAll, m.srcKey)
	if !m.NoTraverse {
		m.dstListDir = m.makeListDir(ctx, m.Fdst, m.DstIncludeAll, m.dstKey)
	}
	// Now create the matching transform
	// ..normalise the UTF8 first
	if !m.NoUnicodeNormalization {
		m.transforms = append(m.transforms, norm.NFC.String)
	}
	// ..if destination is caseInsensitive then make it lower case
	// case Insensitive | src | dst | lower case compare |
	//                  | No  | No  | No                 |
	//                  | Yes | No  | No                 |
	//                  | No  | Yes | Yes                |
	//                  | Yes | Yes | Yes                |
	if m.Fdst.Features().CaseInsensitive || ci.IgnoreCaseSync {
		m.transforms = append(m.transforms, strings.ToLower)
	}
}

// srcOrDstKey turns a directory entry into a sort key using the defined transforms.
func (m *March) srcOrDstKey(entry fs.DirEntry, isSrc bool) string {
	if entry == nil {
		return ""
	}
	name := path.Base(entry.Remote())
	_, isDirectory := entry.(fs.Directory)
	if isSrc {
		name = transform.Path(m.Ctx, name, isDirectory)
	}
	for _, transform := range m.transforms {
		name = transform(name)
	}
	// Suffix entries to make identically named files and
	// directories sort consistently with directories first.
	if isDirectory {
		name += "D"
	} else {
		name += "F"
	}
	return name
}

// srcKey turns a directory entry into a sort key using the defined transforms.
func (m *March) srcKey(entry fs.DirEntry) string {
	return m.srcOrDstKey(entry, true)
}

// dstKey turns a directory entry into a sort key using the defined transforms.
func (m *March) dstKey(entry fs.DirEntry) string {
	return m.srcOrDstKey(entry, false)
}

// makeListDir makes constructs a listing function for the given fs
// and includeAll flags for marching through the file system.
// Note: this will optionally flag filter-aware backends!
func (m *March) makeListDir(ctx context.Context, f fs.Fs, includeAll bool, keyFn list.KeyFn) listDirFn {
	ci := fs.GetConfig(ctx)
	fi := filter.GetConfig(ctx)
	if !(ci.UseListR && f.Features().ListR != nil) && // !--fast-list active and
		!(ci.NoTraverse && fi.HaveFilesFrom()) { // !(--files-from and --no-traverse)
		return func(dir string, callback fs.ListRCallback) (err error) {
			dirCtx := filter.SetUseFilter(m.Ctx, f.Features().FilterAware && !includeAll) // make filter-aware backends constrain List
			return list.DirSortedFn(dirCtx, f, includeAll, dir, callback, keyFn)
		}
	}

	// This returns a closure for use when --fast-list is active or for when
	// --files-from and --no-traverse is set
	var (
		mu      sync.Mutex
		started bool
		dirs    dirtree.DirTree
		dirsErr error
	)
	return func(dir string, callback fs.ListRCallback) (err error) {
		mu.Lock()
		if !started {
			dirCtx := filter.SetUseFilter(m.Ctx, f.Features().FilterAware && !includeAll) // make filter-aware backends constrain List
			dirs, dirsErr = walk.NewDirTree(dirCtx, f, m.Dir, includeAll, ci.MaxDepth)
			started = true
		}
		if dirsErr != nil {
			mu.Unlock()
			return dirsErr
		}
		entries, ok := dirs[dir]
		if !ok {
			mu.Unlock()
			return fs.ErrorDirNotFound
		}
		delete(dirs, dir)
		mu.Unlock()

		// We use a stable sort here just in case there are
		// duplicates. Assuming the remote delivers the entries in a
		// consistent order, this will give the best user experience
		// in syncing as it will use the first entry for the sync
		// comparison.
		slices.SortStableFunc(entries, func(a, b fs.DirEntry) int {
			return cmp.Compare(keyFn(a), keyFn(b))
		})
		return callback(entries)
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

// Run starts the matching process off
func (m *March) Run(ctx context.Context) error {
	ci := fs.GetConfig(ctx)
	fi := filter.GetConfig(ctx)
	m.init(ctx)

	srcDepth := ci.MaxDepth
	if srcDepth < 0 {
		srcDepth = fs.MaxLevel
	}
	dstDepth := srcDepth
	if fi.Opt.DeleteExcluded {
		dstDepth = fs.MaxLevel
	}

	var mu sync.Mutex // Protects vars below
	var jobError error
	var errCount int

	// Start some directory listing go routines
	var wg sync.WaitGroup         // sync closing of go routines
	var traversing sync.WaitGroup // running directory traversals
	checkers := ci.Checkers
	in := make(chan listDirJob, checkers)
	for range checkers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-m.Ctx.Done():
					return
				case job, ok := <-in:
					if !ok {
						return
					}
					jobs, err := m.processJob(job)
					if err != nil {
						mu.Lock()
						// Keep reference only to the first encountered error
						if jobError == nil {
							jobError = err
						}
						errCount++
						mu.Unlock()
					}
					if len(jobs) > 0 {
						traversing.Add(len(jobs))
						go func() {
							// Now we have traversed this directory, send these
							// jobs off for traversal in the background
							for _, newJob := range jobs {
								select {
								case <-m.Ctx.Done():
									// discard job if finishing
									traversing.Done()
								case in <- newJob:
								}
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
		srcRemote: m.Dir,
		srcDepth:  srcDepth - 1,
		dstRemote: m.Dir,
		dstDepth:  dstDepth - 1,
		noDst:     m.NoCheckDest,
	}
	go func() {
		// when the context is cancelled discard the remaining jobs
		<-m.Ctx.Done()
		for range in {
			traversing.Done()
		}
	}()
	traversing.Wait()
	close(in)
	wg.Wait()

	if errCount > 1 {
		return fmt.Errorf("march failed with %d error(s): first error: %w", errCount, jobError)
	}
	return jobError
}

// Check to see if the context has been cancelled
func (m *March) aborting() bool {
	select {
	case <-m.Ctx.Done():
		return true
	default:
	}
	return false
}

// Process the two listings, matching up the items in the two slices
// using the transform function on each name first.
//
// Into srcOnly go Entries which only exist in the srcList
// Into dstOnly go Entries which only exist in the dstList
// Into match go matchPair's of src and dst which have the same name
//
// This checks for duplicates and checks the list is sorted.
func (m *March) matchListings(srcChan, dstChan <-chan fs.DirEntry, srcOnly, dstOnly func(fs.DirEntry), match func(dst, src fs.DirEntry)) error {
	var (
		srcPrev, dstPrev         fs.DirEntry
		srcPrevName, dstPrevName string
		src, dst                 fs.DirEntry
		srcHasMore, dstHasMore   = true, true
		srcName, dstName         string
	)
	srcDone := func() {
		srcPrevName = srcName
		srcPrev = src
		src = nil
		srcName = ""
	}
	dstDone := func() {
		dstPrevName = dstName
		dstPrev = dst
		dst = nil
		dstName = ""
	}
	for {
		if m.aborting() {
			return m.Ctx.Err()
		}
		// Reload src and dst if needed - we set them to nil if used
		if src == nil {
			src, srcHasMore = <-srcChan
			srcName = m.srcKey(src)
		}
		if dst == nil {
			dst, dstHasMore = <-dstChan
			dstName = m.dstKey(dst)
		}
		if !srcHasMore && !dstHasMore {
			break
		}
		if src != nil && srcPrev != nil {
			if srcName == srcPrevName && fs.DirEntryType(srcPrev) == fs.DirEntryType(src) {
				fs.Logf(src, "Duplicate %s found in source - ignoring", fs.DirEntryType(src))
				srcDone() // skip the src and retry the dst
				continue
			} else if srcName < srcPrevName {
				// this should never happen since we sort the listings
				panic("Out of order listing in source")
			}
		}
		if dst != nil && dstPrev != nil {
			if dstName == dstPrevName && fs.DirEntryType(dst) == fs.DirEntryType(dstPrev) {
				fs.Logf(dst, "Duplicate %s found in destination - ignoring", fs.DirEntryType(dst))
				dstDone() // skip the dst and retry the src
				continue
			} else if dstName < dstPrevName {
				// this should never happen since we sort the listings
				panic("Out of order listing in destination")
			}
		}
		switch {
		case src != nil && dst != nil:
			// we can't use CompareDirEntries because srcName, dstName could
			// be different from src.Remote() or dst.Remote()
			srcType := fs.DirEntryType(src)
			dstType := fs.DirEntryType(dst)
			if srcName > dstName || (srcName == dstName && srcType > dstType) {
				dstOnly(dst)
				dstDone()
			} else if srcName < dstName || (srcName == dstName && srcType < dstType) {
				srcOnly(src)
				srcDone()
			} else {
				match(dst, src)
				dstDone()
				srcDone()
			}
		case src == nil:
			dstOnly(dst)
			dstDone()
		case dst == nil:
			srcOnly(src)
			srcDone()
		}
	}
	return nil
}

// processJob processes a listDirJob listing the source and
// destination directories, comparing them and returning a slice of
// more jobs
//
// returns errors using processError
func (m *March) processJob(job listDirJob) ([]listDirJob, error) {
	var (
		jobs                   []listDirJob
		srcChan                = make(chan fs.DirEntry, 100)
		dstChan                = make(chan fs.DirEntry, 100)
		srcListErr, dstListErr error
		wg                     sync.WaitGroup
		ci                     = fs.GetConfig(m.Ctx)
	)

	// List the src and dst directories
	if !job.noSrc {
		srcChan := srcChan // duplicate this as we may override it later
		wg.Add(1)
		go func() {
			defer wg.Done()
			srcListErr = m.srcListDir(job.srcRemote, func(entries fs.DirEntries) error {
				for _, entry := range entries {
					srcChan <- entry
				}
				return nil
			})
			close(srcChan)
		}()
	} else {
		close(srcChan)
	}
	startedDst := false
	if !m.NoTraverse && !job.noDst {
		startedDst = true
		wg.Add(1)
		go func() {
			defer wg.Done()
			dstListErr = m.dstListDir(job.dstRemote, func(entries fs.DirEntries) error {
				for _, entry := range entries {
					dstChan <- entry
				}
				return nil
			})
			close(dstChan)
		}()
	}
	// If NoTraverse is set, then try to find a matching object
	// for each item in the srcList to head dst object
	if m.NoTraverse && !m.NoCheckDest {
		startedDst = true
		workers := ci.Checkers
		originalSrcChan := srcChan
		srcChan = make(chan fs.DirEntry, 100)

		type matchTask struct {
			src      fs.DirEntry        // src object to find in destination
			dstMatch chan<- fs.DirEntry // channel to receive matching dst object or nil
		}
		matchTasks := make(chan matchTask, workers)
		dstMatches := make(chan (<-chan fs.DirEntry), workers)

		// Create the tasks from the originalSrcChan. These are put into matchTasks for
		// processing and dstMatches so they can be retrieved in order.
		go func() {
			for src := range originalSrcChan {
				srcChan <- src
				dstMatch := make(chan fs.DirEntry, 1)
				matchTasks <- matchTask{
					src:      src,
					dstMatch: dstMatch,
				}
				dstMatches <- dstMatch
			}
			close(matchTasks)
		}()

		// Get the tasks from the queue and find a matching object.
		var workerWg sync.WaitGroup
		for range workers {
			workerWg.Add(1)
			go func() {
				defer workerWg.Done()
				for t := range matchTasks {
					// Can't match directories with NewObject
					if _, ok := t.src.(fs.Object); !ok {
						t.dstMatch <- nil
						continue
					}
					leaf := path.Base(t.src.Remote())
					dst, err := m.Fdst.NewObject(m.Ctx, path.Join(job.dstRemote, leaf))
					if err != nil {
						dst = nil
					}
					t.dstMatch <- dst
				}
			}()
		}

		// Close dstResults when all the workers have finished
		go func() {
			workerWg.Wait()
			close(dstMatches)
		}()

		// Read the matches in order and send them to dstChan if found.
		wg.Add(1)
		go func() {
			defer wg.Done()
			for dstMatch := range dstMatches {
				dst := <-dstMatch
				// Note that dst may be nil here
				// We send these on so we don't deadlock the reader
				dstChan <- dst
			}
			close(srcChan)
			close(dstChan)
		}()
	}
	if !startedDst {
		close(dstChan)
	}

	// Work out what to do and do it
	err := m.matchListings(srcChan, dstChan, func(src fs.DirEntry) {
		recurse := m.Callback.SrcOnly(src)
		if recurse && job.srcDepth > 0 {
			jobs = append(jobs, listDirJob{
				srcRemote: src.Remote(),
				dstRemote: src.Remote(),
				srcDepth:  job.srcDepth - 1,
				noDst:     true,
			})
		}
	}, func(dst fs.DirEntry) {
		recurse := m.Callback.DstOnly(dst)
		if recurse && job.dstDepth > 0 {
			jobs = append(jobs, listDirJob{
				srcRemote: dst.Remote(),
				dstRemote: dst.Remote(),
				dstDepth:  job.dstDepth - 1,
				noSrc:     true,
			})
		}
	}, func(dst, src fs.DirEntry) {
		recurse := m.Callback.Match(m.Ctx, dst, src)
		if recurse && job.srcDepth > 0 && job.dstDepth > 0 {
			jobs = append(jobs, listDirJob{
				srcRemote: src.Remote(),
				dstRemote: dst.Remote(),
				srcDepth:  job.srcDepth - 1,
				dstDepth:  job.dstDepth - 1,
			})
		}
	})
	if err != nil {
		return nil, err
	}

	// Wait for listings to complete and report errors
	wg.Wait()
	if srcListErr != nil {
		if job.srcRemote != "" {
			fs.Errorf(job.srcRemote, "error reading source directory: %v", srcListErr)
		} else {
			fs.Errorf(m.Fsrc, "error reading source root directory: %v", srcListErr)
		}
		srcListErr = fs.CountError(m.Ctx, srcListErr)
		return nil, srcListErr
	}
	if dstListErr == fs.ErrorDirNotFound {
		// Copy the stuff anyway
	} else if dstListErr != nil {
		if job.dstRemote != "" {
			fs.Errorf(job.dstRemote, "error reading destination directory: %v", dstListErr)
		} else {
			fs.Errorf(m.Fdst, "error reading destination root directory: %v", dstListErr)
		}
		dstListErr = fs.CountError(m.Ctx, dstListErr)
		return nil, dstListErr
	}

	return jobs, nil
}
