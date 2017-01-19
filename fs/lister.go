// This file implements the Lister object

package fs

import "sync"

// listerResult is returned by the lister methods
type listerResult struct {
	Obj Object
	Dir *Dir
	Err error
}

// Lister objects are used for controlling listing of Fs objects
type Lister struct {
	mu        sync.RWMutex
	buffer    int
	abort     bool
	results   chan listerResult
	closeOnce sync.Once
	level     int
	filter    *Filter
	err       error
}

// NewLister creates a Lister object.
//
// The default channel buffer size will be Config.Checkers unless
// overridden with SetBuffer.  The default level will be infinite.
func NewLister() *Lister {
	o := &Lister{}
	return o.SetLevel(-1).SetBuffer(Config.Checkers)
}

// Finds and lists the files passed in
//
// Note we ignore the dir and just return all the files in the list
func (o *Lister) listFiles(f ListFser, dir string, files FilesMap) {
	buffer := o.Buffer()
	jobs := make(chan string, buffer)
	var wg sync.WaitGroup

	// Start some listing go routines so we find those name in parallel
	wg.Add(buffer)
	for i := 0; i < buffer; i++ {
		go func() {
			defer wg.Done()
			for remote := range jobs {
				obj, err := f.NewObject(remote)
				if err == ErrorObjectNotFound {
					// silently ignore files that aren't found in the files list
				} else if err != nil {
					o.SetError(err)
				} else {
					o.Add(obj)
				}
			}
		}()
	}

	// Pump the names in
	for name := range files {
		jobs <- name
		if o.IsFinished() {
			break
		}
	}
	close(jobs)
	wg.Wait()

	// Signal that this listing is over
	o.Finished()
}

// Start starts a go routine listing the Fs passed in.  It returns the
// same Lister that was passed in for convenience.
func (o *Lister) Start(f ListFser, dir string) *Lister {
	o.results = make(chan listerResult, o.buffer)
	if o.filter != nil && o.filter.Files() != nil {
		go o.listFiles(f, dir, o.filter.Files())
	} else {
		go f.List(o, dir)
	}
	return o
}

// SetLevel sets the level to recurse to.  It returns same Lister that
// was passed in for convenience.  If Level is < 0 then it sets it to
// infinite.  Must be called before Start().
func (o *Lister) SetLevel(level int) *Lister {
	if level < 0 {
		o.level = MaxLevel
	} else {
		o.level = level
	}
	return o
}

// SetFilter sets the Filter that is in use.  It defaults to no
// filtering.  Must be called before Start().
func (o *Lister) SetFilter(filter *Filter) *Lister {
	o.filter = filter
	return o
}

// Level gets the recursion level for this listing.
//
// Fses may ignore this, but should implement it for improved efficiency if possible.
//
// Level 1 means list just the contents of the directory
//
// Each returned item must have less than level `/`s in.
func (o *Lister) Level() int {
	return o.level
}

// SetBuffer sets the channel buffer size in use.  Must be called
// before Start().
func (o *Lister) SetBuffer(buffer int) *Lister {
	if buffer < 1 {
		buffer = 1
	}
	o.buffer = buffer
	return o
}

// Buffer gets the channel buffer size in use
func (o *Lister) Buffer() int {
	return o.buffer
}

// Add an object to the output.
// If the function returns true, the operation has been aborted.
// Multiple goroutines can safely add objects concurrently.
func (o *Lister) Add(obj Object) (abort bool) {
	o.mu.RLock()
	defer o.mu.RUnlock()
	if o.abort {
		return true
	}
	o.results <- listerResult{Obj: obj}
	return false
}

// AddDir will a directory to the output.
// If the function returns true, the operation has been aborted.
// Multiple goroutines can safely add objects concurrently.
func (o *Lister) AddDir(dir *Dir) (abort bool) {
	o.mu.RLock()
	defer o.mu.RUnlock()
	if o.abort {
		return true
	}
	o.results <- listerResult{Dir: dir}
	return false
}

// Error returns a globally application error that's been set on the Lister
// object.
func (o *Lister) Error() error {
	o.mu.RLock()
	defer o.mu.RUnlock()
	return o.err
}

// IncludeDirectory returns whether this directory should be
// included in the listing (and recursed into or not).
func (o *Lister) IncludeDirectory(remote string) bool {
	if o.filter == nil {
		return true
	}
	return o.filter.IncludeDirectory(remote)
}

// finished closes the results channel and sets abort - must be called
// with o.mu held.
func (o *Lister) finished() {
	o.closeOnce.Do(func() {
		close(o.results)
		o.abort = true
	})
}

// SetError will set an error state, and will cause the listing to
// be aborted.
// Multiple goroutines can set the error state concurrently,
// but only the first will be returned to the caller.
func (o *Lister) SetError(err error) {
	o.mu.Lock()
	if err != nil && !o.abort {
		o.err = err
		o.results <- listerResult{Err: err}
		o.finished()
	}
	o.mu.Unlock()
}

// Finished should be called when listing is finished
func (o *Lister) Finished() {
	o.mu.Lock()
	o.finished()
	o.mu.Unlock()
}

// IsFinished returns whether the directory listing is finished or not
func (o *Lister) IsFinished() bool {
	o.mu.RLock()
	defer o.mu.RUnlock()
	return o.abort
}

// Get an object from the listing.
// Will return either an object or a directory, never both.
// Will return (nil, nil, nil) when all objects have been returned.
func (o *Lister) Get() (Object, *Dir, error) {
	select {
	case r := <-o.results:
		return r.Obj, r.Dir, r.Err
	}
}

// GetAll gets all the objects and dirs from the listing.
func (o *Lister) GetAll() (objs []Object, dirs []*Dir, err error) {
	for {
		obj, dir, err := o.Get()
		switch {
		case err != nil:
			return nil, nil, err
		case obj != nil:
			objs = append(objs, obj)
		case dir != nil:
			dirs = append(dirs, dir)
		default:
			return objs, dirs, nil
		}
	}
}

// GetObject will return an object from the listing.
// It will skip over any directories.
// Will return (nil, nil) when all objects have been returned.
func (o *Lister) GetObject() (Object, error) {
	for {
		obj, dir, err := o.Get()
		switch {
		case err != nil:
			return nil, err
		case obj != nil:
			return obj, nil
		case dir != nil:
			// ignore
		default:
			return nil, nil
		}
	}
}

// GetObjects will return a slice of object from the listing.
// It will skip over any directories.
func (o *Lister) GetObjects() (objs []Object, err error) {
	for {
		obj, dir, err := o.Get()
		switch {
		case err != nil:
			return nil, err
		case obj != nil:
			objs = append(objs, obj)
		case dir != nil:
			// ignore
		default:
			return objs, nil
		}
	}
}

// GetDir will return a directory from the listing.
// It will skip over any objects.
// Will return (nil, nil) when all objects have been returned.
func (o *Lister) GetDir() (*Dir, error) {
	for {
		obj, dir, err := o.Get()
		switch {
		case err != nil:
			return nil, err
		case obj != nil:
			// ignore
		case dir != nil:
			return dir, nil
		default:
			return nil, nil
		}
	}
}

// GetDirs will return a slice of directories from the listing.
// It will skip over any objects.
func (o *Lister) GetDirs() (dirs []*Dir, err error) {
	for {
		obj, dir, err := o.Get()
		switch {
		case err != nil:
			return nil, err
		case obj != nil:
			// ignore
		case dir != nil:
			dirs = append(dirs, dir)
		default:
			return dirs, nil
		}
	}
}

// Check interface
var _ ListOpts = (*Lister)(nil)
