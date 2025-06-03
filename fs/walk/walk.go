// Package walk walks directories
package walk

import (
	"context"
	"errors"
	"fmt"
	"path"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/accounting"
	"github.com/rclone/rclone/fs/dirtree"
	"github.com/rclone/rclone/fs/filter"
	"github.com/rclone/rclone/fs/list"
)

// ErrorSkipDir is used as a return value from Walk to indicate that the
// directory named in the call is to be skipped. It is not returned as
// an error by any function.
var ErrorSkipDir = errors.New("skip this directory")

// ErrorCantListR is returned by WalkR if the underlying Fs isn't
// capable of doing a recursive listing.
var ErrorCantListR = errors.New("recursive directory listing not available")

// Func is the type of the function called for directory
// visited by Walk. The path argument contains remote path to the directory.
//
// If there was a problem walking to directory named by path, the
// incoming error will describe the problem and the function can
// decide how to handle that error (and Walk will not descend into
// that directory). If an error is returned, processing stops. The
// sole exception is when the function returns the special value
// ErrorSkipDir. If the function returns ErrorSkipDir, Walk skips the
// directory's contents entirely.
type Func func(path string, entries fs.DirEntries, err error) error

// Walk lists the directory.
//
// If includeAll is not set it will use the filters defined.
//
// If maxLevel is < 0 then it will recurse indefinitely, else it will
// only do maxLevel levels.
//
// It calls fn for each tranche of DirEntries read.
//
// Note that fn will not be called concurrently whereas the directory
// listing will proceed concurrently.
//
// Parent directories are always listed before their children.
//
// This is implemented by WalkR if Config.UseListR is true
// and f supports it and level > 1, or WalkN otherwise.
//
// If --files-from and --no-traverse is set then a DirTree will be
// constructed with just those files in and then walked with WalkR
//
// Note: this will flag filter-aware backends!
//
// NB (f, path) to be replaced by fs.Dir at some point
func Walk(ctx context.Context, f fs.Fs, path string, includeAll bool, maxLevel int, fn Func) error {
	ci := fs.GetConfig(ctx)
	fi := filter.GetConfig(ctx)
	ctx = filter.SetUseFilter(ctx, f.Features().FilterAware && !includeAll) // make filter-aware backends constrain List
	if ci.NoTraverse && fi.HaveFilesFrom() {
		return walkR(ctx, f, path, includeAll, maxLevel, fn, fi.MakeListR(ctx, f.NewObject))
	}
	// FIXME should this just be maxLevel < 0 - why the maxLevel > 1
	if (maxLevel < 0 || maxLevel > 1) && ci.UseListR && f.Features().ListR != nil {
		return walkListR(ctx, f, path, includeAll, maxLevel, fn)
	}
	return walkListDirSorted(ctx, f, path, includeAll, maxLevel, fn)
}

// ListType is uses to choose which combination of files or directories is requires
type ListType byte

// Types of listing for ListR
const (
	ListObjects ListType                 = 1 << iota // list objects only
	ListDirs                                         // list dirs only
	ListAll     = ListObjects | ListDirs             // list files and dirs
)

// Objects returns true if the list type specifies objects
func (l ListType) Objects() bool {
	return (l & ListObjects) != 0
}

// Dirs returns true if the list type specifies dirs
func (l ListType) Dirs() bool {
	return (l & ListDirs) != 0
}

// Filter in (inplace) to only contain the type of list entry required
func (l ListType) Filter(in *fs.DirEntries) {
	if l == ListAll {
		return
	}
	out := (*in)[:0]
	for _, entry := range *in {
		switch entry.(type) {
		case fs.Object:
			if l.Objects() {
				out = append(out, entry)
			}
		case fs.Directory:
			if l.Dirs() {
				out = append(out, entry)
			}
		default:
			fs.Errorf(nil, "Unknown object type %T", entry)
		}
	}
	*in = out
}

// ListR lists the directory recursively.
//
// If includeAll is not set it will use the filters defined.
//
// If maxLevel is < 0 then it will recurse indefinitely, else it will
// only do maxLevel levels.
//
// If synthesizeDirs is set then for bucket-based remotes it will
// synthesize directories from the file structure.  This uses extra
// memory so don't set this if you don't need directories, likewise do
// set this if you are interested in directories.
//
// It calls fn for each tranche of DirEntries read. Note that these
// don't necessarily represent a directory
//
// Note that fn will not be called concurrently whereas the directory
// listing will proceed concurrently.
//
// Directories are not listed in any particular order so you can't
// rely on parents coming before children or alphabetical ordering
//
// This is implemented by using ListR on the backend if possible and
// efficient, otherwise by Walk.
//
// Note: this will flag filter-aware backends
//
// NB (f, path) to be replaced by fs.Dir at some point
func ListR(ctx context.Context, f fs.Fs, path string, includeAll bool, maxLevel int, listType ListType, fn fs.ListRCallback) error {
	fi := filter.GetConfig(ctx)
	// FIXME disable this with --no-fast-list ??? `--disable ListR` will do it...
	doListR := f.Features().ListR

	// Can't use ListR if...
	if doListR == nil || // ...no ListR
		fi.HaveFilesFrom() || // ...using --files-from
		maxLevel >= 0 || // ...using bounded recursion
		len(fi.Opt.ExcludeFile) > 0 || // ...using --exclude-file
		fi.UsesDirectoryFilters() { // ...using any directory filters
		return listRwalk(ctx, f, path, includeAll, maxLevel, listType, fn)
	}
	ctx = filter.SetUseFilter(ctx, f.Features().FilterAware && !includeAll) // make filter-aware backends constrain List
	return listR(ctx, f, path, includeAll, listType, fn, doListR, listType.Dirs() && f.Features().BucketBased)
}

// listRwalk walks the file tree for ListR using Walk
// Note: this will flag filter-aware backends (via Walk)
func listRwalk(ctx context.Context, f fs.Fs, path string, includeAll bool, maxLevel int, listType ListType, fn fs.ListRCallback) error {
	var listErr error
	walkErr := Walk(ctx, f, path, includeAll, maxLevel, func(path string, entries fs.DirEntries, err error) error {
		// Carry on listing but return the error at the end
		if err != nil {
			listErr = err
			err = fs.CountError(ctx, err)
			fs.Errorf(path, "error listing: %v", err)
			return nil
		}
		listType.Filter(&entries)
		return fn(entries)
	})
	if listErr != nil {
		return listErr
	}
	return walkErr
}

// dirMap keeps track of directories made for bucket-based remotes.
// true => directory has been sent
// false => directory has been seen but not sent
type dirMap struct {
	mu   sync.Mutex
	m    map[string]bool
	root string
}

// make a new dirMap
func newDirMap(root string) *dirMap {
	return &dirMap{
		m:    make(map[string]bool),
		root: root,
	}
}

// add adds a directory and parents with sent
func (dm *dirMap) add(dir string, sent bool) {
	for {
		if dir == dm.root || dir == "" {
			return
		}
		currentSent, found := dm.m[dir]
		if found {
			// If it has been sent already then nothing more to do
			if currentSent {
				return
			}
			// If not sent already don't override
			if !sent {
				return
			}
			// currentSent == false && sent == true so needs overriding
		}
		dm.m[dir] = sent
		// Add parents in as unsent
		dir = parentDir(dir)
		sent = false
	}
}

// parentDir finds the parent directory of path
func parentDir(entryPath string) string {
	dirPath := path.Dir(entryPath)
	if dirPath == "." {
		dirPath = ""
	}
	return dirPath
}

// add all the directories in entries and their parents to the dirMap
func (dm *dirMap) addEntries(entries fs.DirEntries) error {
	dm.mu.Lock()
	defer dm.mu.Unlock()
	for _, entry := range entries {
		switch x := entry.(type) {
		case fs.Object:
			dm.add(parentDir(x.Remote()), false)
		case fs.Directory:
			dm.add(x.Remote(), true)
		default:
			return fmt.Errorf("unknown object type %T", entry)
		}
	}
	return nil
}

// send any missing parents to fn
func (dm *dirMap) sendEntries(fn fs.ListRCallback) (err error) {
	// Count the strings first so we allocate the minimum memory
	n := 0
	for _, sent := range dm.m {
		if !sent {
			n++
		}
	}
	if n == 0 {
		return nil
	}
	dirs := make([]string, 0, n)
	// Fill the dirs up then sort it
	for dir, sent := range dm.m {
		if !sent {
			dirs = append(dirs, dir)
		}
	}
	sort.Strings(dirs)
	// Now convert to bulkier Dir in batches and send
	now := time.Now()
	list := list.NewHelper(fn)
	for _, dir := range dirs {
		err = list.Add(fs.NewDir(dir, now))
		if err != nil {
			return err
		}
	}
	return list.Flush()
}

// listR walks the file tree using ListR
func listR(ctx context.Context, f fs.Fs, path string, includeAll bool, listType ListType, fn fs.ListRCallback, doListR fs.ListRFn, synthesizeDirs bool) error {
	fi := filter.GetConfig(ctx)
	includeDirectory := fi.IncludeDirectory(ctx, f)
	if !includeAll {
		includeAll = fi.InActive()
	}
	var dm *dirMap
	if synthesizeDirs {
		dm = newDirMap(path)
	}
	var mu sync.Mutex
	err := doListR(ctx, path, func(entries fs.DirEntries) (err error) {
		accounting.Stats(ctx).Listed(int64(len(entries)))
		if synthesizeDirs {
			err = dm.addEntries(entries)
			if err != nil {
				return err
			}
		}
		listType.Filter(&entries)
		if !includeAll {
			filteredEntries := entries[:0]
			for _, entry := range entries {
				var include bool
				switch x := entry.(type) {
				case fs.Object:
					include = fi.IncludeObject(ctx, x)
				case fs.Directory:
					include, err = includeDirectory(x.Remote())
					if err != nil {
						return err
					}
				default:
					return fmt.Errorf("unknown object type %T", entry)
				}
				if include {
					filteredEntries = append(filteredEntries, entry)
				}
			}
			entries = filteredEntries
		}
		mu.Lock()
		defer mu.Unlock()
		return fn(entries)
	})
	if err != nil {
		return err
	}
	if synthesizeDirs {
		err = dm.sendEntries(fn)
		if err != nil {
			return err
		}
	}
	return nil
}

// walkListDirSorted lists the directory.
//
// It implements Walk using non recursive directory listing.
func walkListDirSorted(ctx context.Context, f fs.Fs, path string, includeAll bool, maxLevel int, fn Func) error {
	return walk(ctx, f, path, includeAll, maxLevel, fn, list.DirSorted)
}

// walkListR lists the directory.
//
// It implements Walk using recursive directory listing if
// available, or returns ErrorCantListR if not.
func walkListR(ctx context.Context, f fs.Fs, path string, includeAll bool, maxLevel int, fn Func) error {
	listR := f.Features().ListR
	if listR == nil {
		return ErrorCantListR
	}
	return walkR(ctx, f, path, includeAll, maxLevel, fn, listR)
}

type listDirFunc func(ctx context.Context, fs fs.Fs, includeAll bool, dir string) (entries fs.DirEntries, err error)

func walk(ctx context.Context, f fs.Fs, path string, includeAll bool, maxLevel int, fn Func, listDir listDirFunc) error {
	var (
		wg         sync.WaitGroup      // sync closing of go routines
		traversing sync.WaitGroup      // running directory traversals
		doClose    sync.Once           // close the channel once
		mu         sync.Mutex          // stop fn being called concurrently
		ci         = fs.GetConfig(ctx) // current config
	)
	// listJob describe a directory listing that needs to be done
	type listJob struct {
		remote string
		depth  int
	}

	in := make(chan listJob, ci.Checkers)
	errs := make(chan error, 1)
	quit := make(chan struct{})
	closeQuit := func() {
		doClose.Do(func() {
			close(quit)
			go func() {
				for range in {
					traversing.Done()
				}
			}()
		})
	}
	for range ci.Checkers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case job, ok := <-in:
					if !ok {
						return
					}
					entries, err := listDir(ctx, f, includeAll, job.remote)
					var jobs []listJob
					if err == nil && job.depth != 0 {
						entries.ForDir(func(dir fs.Directory) {
							// Recurse for the directory
							jobs = append(jobs, listJob{
								remote: dir.Remote(),
								depth:  job.depth - 1,
							})
						})
					}
					mu.Lock()
					err = fn(job.remote, entries, err)
					mu.Unlock()
					// NB once we have passed entries to fn we mustn't touch it again
					if err != nil && err != ErrorSkipDir {
						traversing.Done()
						err = fs.CountError(ctx, err)
						fs.Errorf(job.remote, "error listing: %v", err)
						closeQuit()
						// Send error to error channel if space
						select {
						case errs <- err:
						default:
						}
						continue
					}
					if err == nil && len(jobs) > 0 {
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
				case <-quit:
					return
				}
			}
		}()
	}
	// Start the process
	traversing.Add(1)
	in <- listJob{
		remote: path,
		depth:  maxLevel - 1,
	}
	traversing.Wait()
	close(in)
	wg.Wait()
	close(errs)
	// return the first error returned or nil
	return <-errs
}

func walkRDirTree(ctx context.Context, f fs.Fs, startPath string, includeAll bool, maxLevel int, listR fs.ListRFn) (dirtree.DirTree, error) {
	fi := filter.GetConfig(ctx)
	dirs := dirtree.New()
	// Entries can come in arbitrary order. We use toPrune to keep
	// all directories to exclude later.
	toPrune := make(map[string]bool)
	includeDirectory := fi.IncludeDirectory(ctx, f)
	var mu sync.Mutex
	err := listR(ctx, startPath, func(entries fs.DirEntries) error {
		accounting.Stats(ctx).Listed(int64(len(entries)))
		mu.Lock()
		defer mu.Unlock()
		for _, entry := range entries {
			slashes := strings.Count(entry.Remote(), "/")
			excluded := true
			switch x := entry.(type) {
			case fs.Object:
				// Make sure we don't delete excluded files if not required
				if includeAll || fi.IncludeObject(ctx, x) {
					if maxLevel < 0 || slashes <= maxLevel-1 {
						dirs.Add(x)
						excluded = false
					}
				}
				// Make sure we include any parent directories of excluded objects
				if excluded {
					dirPath := parentDir(x.Remote())
					slashes--
					if maxLevel >= 0 {
						for ; slashes > maxLevel-1; slashes-- {
							dirPath = parentDir(dirPath)
						}
					}
					inc, err := includeDirectory(dirPath)
					if err != nil {
						return err
					}
					if inc || includeAll {
						// If the directory doesn't exist already, create it
						_, obj := dirs.Find(dirPath)
						if obj == nil {
							dirs.AddDir(fs.NewDir(dirPath, time.Now()))
						}
					}
				}
				// Check if we need to prune a directory later.
				if !includeAll && len(fi.Opt.ExcludeFile) > 0 {
					basename := path.Base(x.Remote())
					for _, excludeFile := range fi.Opt.ExcludeFile {
						if basename == excludeFile {
							excludeDir := parentDir(x.Remote())
							toPrune[excludeDir] = true
						}
					}
				}
			case fs.Directory:
				inc, err := includeDirectory(x.Remote())
				if err != nil {
					return err
				}
				if includeAll || inc {
					if maxLevel < 0 || slashes <= maxLevel-1 {
						if slashes == maxLevel-1 {
							// Just add the object if at maxLevel
							dirs.Add(x)
						} else {
							dirs.AddDir(x)
						}
					}
				}
			default:
				return fmt.Errorf("unknown object type %T", entry)
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	dirs.CheckParents(startPath)
	if len(dirs) == 0 {
		dirs[startPath] = nil
	}
	err = dirs.Prune(toPrune)
	if err != nil {
		return nil, err
	}
	dirs.Sort()
	return dirs, nil
}

// Create a DirTree using List
func walkNDirTree(ctx context.Context, f fs.Fs, path string, includeAll bool, maxLevel int, listDir listDirFunc) (dirtree.DirTree, error) {
	dirs := make(dirtree.DirTree)
	fn := func(dirPath string, entries fs.DirEntries, err error) error {
		if err == nil {
			dirs[dirPath] = entries
		}
		return err
	}
	err := walk(ctx, f, path, includeAll, maxLevel, fn, listDir)
	if err != nil {
		return nil, err
	}
	return dirs, nil
}

// NewDirTree returns a DirTree filled with the directory listing
// using the parameters supplied.
//
// If includeAll is not set it will use the filters defined.
//
// If maxLevel is < 0 then it will recurse indefinitely, else it will
// only do maxLevel levels.
//
// This is implemented by WalkR if f supports ListR and level > 1, or
// WalkN otherwise.
//
// If --files-from and --no-traverse is set then a DirTree will be
// constructed with just those files in.
//
// NB (f, path) to be replaced by fs.Dir at some point
func NewDirTree(ctx context.Context, f fs.Fs, path string, includeAll bool, maxLevel int) (dirtree.DirTree, error) {
	ci := fs.GetConfig(ctx)
	fi := filter.GetConfig(ctx)
	// if --no-traverse and --files-from build DirTree just from files
	if ci.NoTraverse && fi.HaveFilesFrom() {
		return walkRDirTree(ctx, f, path, includeAll, maxLevel, fi.MakeListR(ctx, f.NewObject))
	}
	// if have ListR; and recursing; and not using --files-from; then build a DirTree with ListR
	if ListR := f.Features().ListR; (maxLevel < 0 || maxLevel > 1) && ListR != nil && !fi.HaveFilesFrom() {
		return walkRDirTree(ctx, f, path, includeAll, maxLevel, ListR)
	}
	// otherwise just use List
	return walkNDirTree(ctx, f, path, includeAll, maxLevel, list.DirSorted)
}

func walkR(ctx context.Context, f fs.Fs, path string, includeAll bool, maxLevel int, fn Func, listR fs.ListRFn) error {
	dirs, err := walkRDirTree(ctx, f, path, includeAll, maxLevel, listR)
	if err != nil {
		return err
	}
	skipping := false
	skipPrefix := ""
	emptyDir := fs.DirEntries{}
	for _, dirPath := range dirs.Dirs() {
		if skipping {
			// Skip over directories as required
			if strings.HasPrefix(dirPath, skipPrefix) {
				continue
			}
			skipping = false
		}
		entries := dirs[dirPath]
		if entries == nil {
			entries = emptyDir
		}
		err = fn(dirPath, entries, nil)
		if err == ErrorSkipDir {
			skipping = true
			skipPrefix = dirPath
			if skipPrefix != "" {
				skipPrefix += "/"
			}
		} else if err != nil {
			return err
		}
	}
	return nil
}

// GetAll runs ListR getting all the results
func GetAll(ctx context.Context, f fs.Fs, path string, includeAll bool, maxLevel int) (objs []fs.Object, dirs []fs.Directory, err error) {
	err = ListR(ctx, f, path, includeAll, maxLevel, ListAll, func(entries fs.DirEntries) error {
		for _, entry := range entries {
			switch x := entry.(type) {
			case fs.Object:
				objs = append(objs, x)
			case fs.Directory:
				dirs = append(dirs, x)
			}
		}
		return nil
	})
	return
}
