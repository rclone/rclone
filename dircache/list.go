// Listing utility functions for fses which use dircache

package dircache

import (
	"sync"

	"github.com/ncw/rclone/fs"
)

// ListDirJob describe a directory listing that needs to be done
type ListDirJob struct {
	DirID string
	Path  string
	Depth int
}

// ListDirer describes the interface necessary to use ListDir
type ListDirer interface {
	// ListDir reads the directory specified by the job into out, returning any more jobs
	ListDir(out fs.ListOpts, job ListDirJob) (jobs []ListDirJob, err error)
}

// listDir lists the directory using a recursive list from the root
//
// It does this in parallel, calling f.ListDir to do the actual reading
func listDir(f ListDirer, out fs.ListOpts, dirID string, path string) {
	// Start some directory listing go routines
	var wg sync.WaitGroup         // sync closing of go routines
	var traversing sync.WaitGroup // running directory traversals
	buffer := out.Buffer()
	in := make(chan ListDirJob, buffer)
	for i := 0; i < buffer; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for job := range in {
				jobs, err := f.ListDir(out, job)
				if err != nil {
					out.SetError(err)
					fs.Debug(f, "Error reading %s: %s", path, err)
				} else {
					traversing.Add(len(jobs))
					go func() {
						// Now we have traversed this directory, send these
						// jobs off for traversal in the background
						for _, job := range jobs {
							in <- job
						}
					}()
				}
				traversing.Done()
			}
		}()
	}

	// Start the process
	traversing.Add(1)
	in <- ListDirJob{DirID: dirID, Path: path, Depth: out.Level() - 1}
	traversing.Wait()
	close(in)
	wg.Wait()
}

// List walks the path returning iles and directories into out
func (dc *DirCache) List(f ListDirer, out fs.ListOpts, dir string) {
	defer out.Finished()
	err := dc.FindRoot(false)
	if err != nil {
		out.SetError(err)
		return
	}
	id, err := dc.FindDir(dir, false)
	if err != nil {
		out.SetError(err)
		return
	}
	if dir != "" {
		dir += "/"
	}
	listDir(f, out, id, dir)
}
