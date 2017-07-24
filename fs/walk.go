// Walking directories

package fs

import (
	"bytes"
	"fmt"
	"path"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/pkg/errors"
)

// ErrorSkipDir is used as a return value from Walk to indicate that the
// directory named in the call is to be skipped. It is not returned as
// an error by any function.
var ErrorSkipDir = errors.New("skip this directory")

// ErrorCantListR is returned by WalkR if the underlying Fs isn't
// capable of doing a recursive listing.
var ErrorCantListR = errors.New("recursive directory listing not available")

// WalkFunc is the type of the function called for directory
// visited by Walk. The path argument contains remote path to the directory.
//
// If there was a problem walking to directory named by path, the
// incoming error will describe the problem and the function can
// decide how to handle that error (and Walk will not descend into
// that directory). If an error is returned, processing stops. The
// sole exception is when the function returns the special value
// ErrorSkipDir. If the function returns ErrorSkipDir, Walk skips the
// directory's contents entirely.
type WalkFunc func(path string, entries DirEntries, err error) error

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
// Parent directories are always listed before their children
//
// This is implemented by WalkR if Config.UseRecursiveListing is true
// and f supports it and level > 1, or WalkN otherwise.
//
// NB (f, path) to be replaced by fs.Dir at some point
func Walk(f Fs, path string, includeAll bool, maxLevel int, fn WalkFunc) error {
	if (maxLevel < 0 || maxLevel > 1) && Config.UseListR && f.Features().ListR != nil {
		return WalkR(f, path, includeAll, maxLevel, fn)
	}
	return WalkN(f, path, includeAll, maxLevel, fn)
}

// WalkN lists the directory.
//
// It implements Walk using non recursive directory listing.
func WalkN(f Fs, path string, includeAll bool, maxLevel int, fn WalkFunc) error {
	return walk(f, path, includeAll, maxLevel, fn, ListDirSorted)
}

// WalkR lists the directory.
//
// It implements Walk using recursive directory listing if
// available, or returns ErrorCantListR if not.
func WalkR(f Fs, path string, includeAll bool, maxLevel int, fn WalkFunc) error {
	listR := f.Features().ListR
	if listR == nil {
		return ErrorCantListR
	}
	return walkR(f, path, includeAll, maxLevel, fn, listR)
}

type listDirFunc func(fs Fs, includeAll bool, dir string) (entries DirEntries, err error)

func walk(f Fs, path string, includeAll bool, maxLevel int, fn WalkFunc, listDir listDirFunc) error {
	var (
		wg         sync.WaitGroup // sync closing of go routines
		traversing sync.WaitGroup // running directory traversals
		doClose    sync.Once      // close the channel once
		mu         sync.Mutex     // stop fn being called concurrently
	)
	// listJob describe a directory listing that needs to be done
	type listJob struct {
		remote string
		depth  int
	}

	in := make(chan listJob, Config.Checkers)
	errs := make(chan error, 1)
	quit := make(chan struct{})
	closeQuit := func() {
		doClose.Do(func() {
			close(quit)
			go func() {
				for _ = range in {
					traversing.Done()
				}
			}()
		})
	}
	for i := 0; i < Config.Checkers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case job, ok := <-in:
					if !ok {
						return
					}
					entries, err := listDir(f, includeAll, job.remote)
					var jobs []listJob
					if err == nil && job.depth != 0 {
						entries.ForDir(func(dir Directory) {
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
						Stats.Error()
						Errorf(job.remote, "error listing: %v", err)
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

// DirTree is a map of directories to entries
type DirTree map[string]DirEntries

// parentDir finds the parent directory of path
func parentDir(entryPath string) string {
	dirPath := path.Dir(entryPath)
	if dirPath == "." {
		dirPath = ""
	}
	return dirPath
}

// add an entry to the tree
func (dt DirTree) add(entry DirEntry) {
	dirPath := parentDir(entry.Remote())
	dt[dirPath] = append(dt[dirPath], entry)
}

// add a directory entry to the tree
func (dt DirTree) addDir(entry DirEntry) {
	dt.add(entry)
	// create the directory itself if it doesn't exist already
	dirPath := entry.Remote()
	if _, ok := dt[dirPath]; !ok {
		dt[dirPath] = nil
	}
}

// Find returns the DirEntry for filePath or nil if not found
func (dt DirTree) Find(filePath string) (parentPath string, entry DirEntry) {
	parentPath = parentDir(filePath)
	for _, entry := range dt[parentPath] {
		if entry.Remote() == filePath {
			return parentPath, entry
		}
	}
	return parentPath, nil
}

// check that dirPath has a *Dir in its parent
func (dt DirTree) checkParent(root, dirPath string) {
	if dirPath == root {
		return
	}
	parentPath, entry := dt.Find(dirPath)
	if entry != nil {
		return
	}
	dt[parentPath] = append(dt[parentPath], NewDir(dirPath, time.Now()))
	dt.checkParent(root, parentPath)
}

// check every directory in the tree has *Dir in its parent
func (dt DirTree) checkParents(root string) {
	for dirPath := range dt {
		dt.checkParent(root, dirPath)
	}
}

// Sort sorts all the Entries
func (dt DirTree) Sort() {
	for _, entries := range dt {
		sort.Stable(entries)
	}
}

// Dirs returns the directories in sorted order
func (dt DirTree) Dirs() (dirNames []string) {
	for dirPath := range dt {
		dirNames = append(dirNames, dirPath)
	}
	sort.Strings(dirNames)
	return dirNames
}

// String emits a simple representation of the DirTree
func (dt DirTree) String() string {
	out := new(bytes.Buffer)
	for _, dir := range dt.Dirs() {
		fmt.Fprintf(out, "%s/\n", dir)
		for _, entry := range dt[dir] {
			flag := ""
			if _, ok := entry.(Directory); ok {
				flag = "/"
			}
			fmt.Fprintf(out, "  %s%s\n", path.Base(entry.Remote()), flag)
		}
	}
	return out.String()
}

func walkRDirTree(f Fs, path string, includeAll bool, maxLevel int, listR ListRFn) (DirTree, error) {
	dirs := make(DirTree)
	var mu sync.Mutex
	err := listR(path, func(entries DirEntries) error {
		mu.Lock()
		defer mu.Unlock()
		for _, entry := range entries {
			slashes := strings.Count(entry.Remote(), "/")
			switch x := entry.(type) {
			case Object:
				// Make sure we don't delete excluded files if not required
				if includeAll || Config.Filter.IncludeObject(x) {
					if maxLevel < 0 || slashes <= maxLevel-1 {
						dirs.add(x)
					} else {
						// Make sure we include any parent directories of excluded objects
						dirPath := x.Remote()
						for ; slashes > maxLevel-1; slashes-- {
							dirPath = parentDir(dirPath)
						}
						dirs.checkParent(path, dirPath)
					}
				} else {
					Debugf(x, "Excluded from sync (and deletion)")
				}
			case Directory:
				if includeAll || Config.Filter.IncludeDirectory(x.Remote()) {
					if maxLevel < 0 || slashes <= maxLevel-1 {
						if slashes == maxLevel-1 {
							// Just add the object if at maxLevel
							dirs.add(x)
						} else {
							dirs.addDir(x)
						}
					}
				} else {
					Debugf(x, "Excluded from sync (and deletion)")
				}
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	dirs.checkParents(path)
	if len(dirs) == 0 {
		dirs[path] = nil
	}
	dirs.Sort()
	return dirs, nil
}

// Create a DirTree using List
func walkNDirTree(f Fs, path string, includeAll bool, maxLevel int, listDir listDirFunc) (DirTree, error) {
	dirs := make(DirTree)
	fn := func(dirPath string, entries DirEntries, err error) error {
		if err == nil {
			dirs[dirPath] = entries
		}
		return err
	}
	err := walk(f, path, includeAll, maxLevel, fn, listDir)
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
// This is implemented by WalkR if Config.UseRecursiveListing is true
// and f supports it and level > 1, or WalkN otherwise.
//
// NB (f, path) to be replaced by fs.Dir at some point
func NewDirTree(f Fs, path string, includeAll bool, maxLevel int) (DirTree, error) {
	if ListR := f.Features().ListR; (maxLevel < 0 || maxLevel > 1) && Config.UseListR && ListR != nil {
		return walkRDirTree(f, path, includeAll, maxLevel, ListR)
	}
	return walkNDirTree(f, path, includeAll, maxLevel, ListDirSorted)
}

func walkR(f Fs, path string, includeAll bool, maxLevel int, fn WalkFunc, listR ListRFn) error {
	dirs, err := walkRDirTree(f, path, includeAll, maxLevel, listR)
	if err != nil {
		return err
	}
	skipping := false
	skipPrefix := ""
	emptyDir := DirEntries{}
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

// WalkGetAll runs Walk getting all the results
func WalkGetAll(f Fs, path string, includeAll bool, maxLevel int) (objs []Object, dirs []Directory, err error) {
	err = Walk(f, path, includeAll, maxLevel, func(dirPath string, entries DirEntries, err error) error {
		if err != nil {
			return err
		}
		for _, entry := range entries {
			switch x := entry.(type) {
			case Object:
				objs = append(objs, x)
			case Directory:
				dirs = append(dirs, x)
			}
		}
		return nil
	})
	return
}

// ListRHelper is used in the implementation of ListR to accumulate DirEntries
type ListRHelper struct {
	callback ListRCallback
	entries  DirEntries
}

// NewListRHelper should be called from ListR with the callback passed in
func NewListRHelper(callback ListRCallback) *ListRHelper {
	return &ListRHelper{
		callback: callback,
	}
}

// send sends the stored entries to the callback if there are >= max
// entries.
func (lh *ListRHelper) send(max int) (err error) {
	if len(lh.entries) >= max {
		err = lh.callback(lh.entries)
		lh.entries = lh.entries[:0]
	}
	return err
}

// Add an entry to the stored entries and send them if there are more
// than a certain amount
func (lh *ListRHelper) Add(entry DirEntry) error {
	if entry == nil {
		return nil
	}
	lh.entries = append(lh.entries, entry)
	return lh.send(100)
}

// Flush the stored entries (if any) sending them to the callback
func (lh *ListRHelper) Flush() error {
	return lh.send(1)
}
