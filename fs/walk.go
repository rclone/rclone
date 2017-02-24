// Walking directories

package fs

import (
	"sync"

	"github.com/pkg/errors"
)

// ErrorSkipDir is used as a return value from Walk to indicate that the
// directory named in the call is to be skipped. It is not returned as
// an error by any function.
var ErrorSkipDir = errors.New("skip this directory")

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
// NB (f, path) to be replaced by fs.Dir at some point
func Walk(f Fs, path string, includeAll bool, maxLevel int, fn WalkFunc) error {
	return walk(f, path, includeAll, maxLevel, fn, ListDirSorted)
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
					mu.Lock()
					err = fn(job.remote, entries, err)
					mu.Unlock()
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
					var jobs []listJob
					if job.depth != 0 && err == nil {
						entries.ForDir(func(dir *Dir) {
							// Recurse for the directory
							jobs = append(jobs, listJob{
								remote: dir.Remote(),
								depth:  job.depth - 1,
							})
						})
					}
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

// WalkGetAll runs Walk getting all the results
func WalkGetAll(f Fs, path string, includeAll bool, maxLevel int) (objs []Object, dirs []*Dir, err error) {
	err = Walk(f, path, includeAll, maxLevel, func(dirPath string, entries DirEntries, err error) error {
		if err != nil {
			return err
		}
		for _, entry := range entries {
			switch x := entry.(type) {
			case Object:
				objs = append(objs, x)
			case *Dir:
				dirs = append(dirs, x)
			}
		}
		return nil
	})
	return
}
