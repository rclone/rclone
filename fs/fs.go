// Package fs is a generic file system interface for rclone object storage systems
package fs

import (
	"fmt"
	"io"
	"log"
	"math"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
)

// Constants
const (
	// UserAgent for Fs which can set it
	UserAgent = "rclone/" + Version
	// ModTimeNotSupported is a very large precision value to show
	// mod time isn't supported on this Fs
	ModTimeNotSupported = 100 * 365 * 24 * time.Hour
	// MaxLevel is a sentinel representing an infinite depth for listings
	MaxLevel = math.MaxInt32
)

// Globals
var (
	// Filesystem registry
	fsRegistry []*RegInfo
	// ErrorNotFoundInConfigFile is returned by NewFs if not found in config file
	ErrorNotFoundInConfigFile = fmt.Errorf("Didn't find section in config file")
	ErrorCantPurge            = fmt.Errorf("Can't purge directory")
	ErrorCantCopy             = fmt.Errorf("Can't copy object - incompatible remotes")
	ErrorCantMove             = fmt.Errorf("Can't move object - incompatible remotes")
	ErrorCantDirMove          = fmt.Errorf("Can't move directory - incompatible remotes")
	ErrorDirExists            = fmt.Errorf("Can't copy directory - destination already exists")
	ErrorCantSetModTime       = fmt.Errorf("Can't set modified time")
	ErrorDirNotFound          = fmt.Errorf("Directory not found")
	ErrorLevelNotSupported    = fmt.Errorf("Level value not supported")
	ErrorListAborted          = fmt.Errorf("List aborted")
)

// RegInfo provides information about a filesystem
type RegInfo struct {
	// Name of this fs
	Name string
	// Description of this fs - defaults to Name
	Description string
	// Create a new file system.  If root refers to an existing
	// object, then it should return a Fs which only returns that
	// object.
	NewFs func(name string, root string) (Fs, error)
	// Function to call to help with config
	Config func(string)
	// Options for the Fs configuration
	Options []Option
}

// Option is describes an option for the config wizard
type Option struct {
	Name     string
	Help     string
	Optional bool
	Examples OptionExamples
}

// OptionExamples is a slice of examples
type OptionExamples []OptionExample

// Len is part of sort.Interface.
func (os OptionExamples) Len() int { return len(os) }

// Swap is part of sort.Interface.
func (os OptionExamples) Swap(i, j int) { os[i], os[j] = os[j], os[i] }

// Less is part of sort.Interface.
func (os OptionExamples) Less(i, j int) bool { return os[i].Help < os[j].Help }

// Sort sorts an OptionExamples
func (os OptionExamples) Sort() { sort.Sort(os) }

// OptionExample describes an example for an Option
type OptionExample struct {
	Value string
	Help  string
}

// Register a filesystem
//
// Fs modules  should use this in an init() function
func Register(info *RegInfo) {
	fsRegistry = append(fsRegistry, info)
}

// Fs is the interface a cloud storage system must provide
type Fs interface {
	Info

	// List the objects and directories of the Fs
	//
	// This should return ErrDirNotFound if the directory isn't found.
	List(ListOpts)

	// NewFsObject finds the Object at remote.  Returns nil if can't be found
	NewFsObject(remote string) Object

	// Put in to the remote path with the modTime given of the given size
	//
	// May create the object even if it returns an error - if so
	// will return the object and the error, otherwise will return
	// nil and the error
	Put(in io.Reader, src ObjectInfo) (Object, error)

	// Mkdir makes the directory (container, bucket)
	//
	// Shouldn't return an error if it already exists
	Mkdir() error

	// Rmdir removes the directory (container, bucket) if empty
	//
	// Return an error if it doesn't exist or isn't empty
	Rmdir() error
}

// Info provides an interface to reading information about a filesystem.
type Info interface {
	// Name of the remote (as passed into NewFs)
	Name() string

	// Root of the remote (as passed into NewFs)
	Root() string

	// String returns a description of the FS
	String() string

	// Precision of the ModTimes in this Fs
	Precision() time.Duration

	// Returns the supported hash types of the filesystem
	Hashes() HashSet
}

// Object is a filesystem like object provided by an Fs
type Object interface {
	ObjectInfo

	// String returns a description of the Object
	String() string

	// SetModTime sets the metadata on the object to set the modification date
	SetModTime(time.Time) error

	// Open opens the file for read.  Call Close() on the returned io.ReadCloser
	Open() (io.ReadCloser, error)

	// Update in to the object with the modTime given of the given size
	Update(in io.Reader, src ObjectInfo) error

	// Removes this object
	Remove() error
}

// ObjectInfo contains information about an object.
type ObjectInfo interface {
	// Fs returns read only access to the Fs that this object is part of
	Fs() Info

	// Remote returns the remote path
	Remote() string

	// Hash returns the selected checksum of the file
	// If no checksum is available it returns ""
	Hash(HashType) (string, error)

	// ModTime returns the modification date of the file
	// It should return a best guess if one isn't available
	ModTime() time.Time

	// Size returns the size of the file
	Size() int64

	// Storable says whether this object can be stored
	Storable() bool
}

// Purger is an optional interfaces for Fs
type Purger interface {
	// Purge all files in the root and the root directory
	//
	// Implement this if you have a way of deleting all the files
	// quicker than just running Remove() on the result of List()
	//
	// Return an error if it doesn't exist
	Purge() error
}

// Copier is an optional interface for Fs
type Copier interface {
	// Copy src to this remote using server side copy operations.
	//
	// This is stored with the remote path given
	//
	// It returns the destination Object and a possible error
	//
	// Will only be called if src.Fs().Name() == f.Name()
	//
	// If it isn't possible then return fs.ErrorCantCopy
	Copy(src Object, remote string) (Object, error)
}

// Mover is an optional interface for Fs
type Mover interface {
	// Move src to this remote using server side move operations.
	//
	// This is stored with the remote path given
	//
	// It returns the destination Object and a possible error
	//
	// Will only be called if src.Fs().Name() == f.Name()
	//
	// If it isn't possible then return fs.ErrorCantMove
	Move(src Object, remote string) (Object, error)
}

// DirMover is an optional interface for Fs
type DirMover interface {
	// DirMove moves src to this remote using server side move
	// operations.
	//
	// Will only be called if src.Fs().Name() == f.Name()
	//
	// If it isn't possible then return fs.ErrorCantDirMove
	//
	// If destination exists then return fs.ErrorDirExists
	DirMove(src Fs) error
}

// UnWrapper is an optional interfaces for Fs
type UnWrapper interface {
	// UnWrap returns the Fs that this Fs is wrapping
	UnWrap() Fs
}

// ObjectsChan is a channel of Objects
type ObjectsChan chan Object

type ListOpts interface {
	// Add an object to the output.
	// If the function returns true, the operation has been aborted.
	// Multiple goroutines can safely add objects concurrently.
	Add(obj Object) (abort bool)

	// Add a directory to the output.
	// If the function returns true, the operation has been aborted.
	// Multiple goroutines can safely add objects concurrently.
	AddDir(dir *Dir) (abort bool)

	// IncludeDirectory returns whether this directory should be
	// included in the listing (and recursed into or not).
	IncludeDirectory(remote string) bool

	// SetError will set an error state, and will cause the listing to
	// be aborted.
	// Multiple goroutines can set the error state concurrently,
	// but only the first will be returned to the caller.
	SetError(err error)

	// Level returns the level it should recurse to.  Fses may
	// ignore this in which case the listing will be less
	// efficient.
	Level() int

	// Buffer returns the channel depth in use
	Buffer() int

	// Finished should be called when listing is finished
	Finished()

	// IsFinished returns whether Finished or SetError have been called
	IsFinished() bool
}

// listerResult is returned by the lister methods
type listerResult struct {
	Obj Object
	Dir *Dir
	Err error
}

// Lister objects are used for controlling listing of Fs objects
type Lister struct {
	mu       sync.RWMutex
	buffer   int
	abort    bool
	results  chan listerResult
	finished sync.Once
	level    int
	filter   *Filter
}

// NewLister creates a Lister object.
//
// The default channel buffer size will be Config.Checkers unless
// overridden with SetBuffer.  The default level will be infinite.
func NewLister() *Lister {
	o := &Lister{}
	return o.SetLevel(-1).SetBuffer(Config.Checkers)
}

// Start starts a go routine listing the Fs passed in.  It returns the
// same Lister that was passed in for convenience.
func (o *Lister) Start(f Fs) *Lister {
	o.results = make(chan listerResult, o.buffer)
	go func() {
		f.List(o)
	}()
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
	remote := dir.Name
	remote = strings.Trim(remote, "/")
	dir.Name = remote
	// Check the level and ignore if too high
	slashes := strings.Count(remote, "/")
	if slashes >= o.level {
		return false
	}
	// Check if directory is included
	if !o.IncludeDirectory(remote) {
		return false
	}
	o.results <- listerResult{Dir: dir}
	return false
}

// IncludeDirectory returns whether this directory should be
// included in the listing (and recursed into or not).
func (o *Lister) IncludeDirectory(remote string) bool {
	if o.filter == nil {
		return true
	}
	return o.filter.IncludeDirectory(remote)
}

// SetError will set an error state, and will cause the listing to
// be aborted.
// Multiple goroutines can set the error state concurrently,
// but only the first will be returned to the caller.
func (o *Lister) SetError(err error) {
	o.mu.RLock()
	if err != nil && !o.abort {
		o.results <- listerResult{Err: err}
	}
	o.mu.RUnlock()
	o.Finished()
}

// Finished should be called when listing is finished
func (o *Lister) Finished() {
	o.finished.Do(func() {
		o.mu.Lock()
		o.abort = true
		close(o.results)
		o.mu.Unlock()
	})
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

// GetObject will return an object from the listing.
// It will skip over any directories.
// Will return (nil, nil) when all objects have been returned.
func (o *Lister) GetObject() (Object, error) {
	for {
		obj, dir, err := o.Get()
		if err != nil {
			return nil, err
		}
		// Check if we are finished
		if dir == nil && obj == nil {
			return nil, nil
		}
		// Ignore directories
		if dir != nil {
			continue
		}
		return obj, nil
	}
}

// GetObjects will return a slice of object from the listing.
// It will skip over any directories.
func (o *Lister) GetObjects() (objs []Object, err error) {
	for {
		obj, dir, err := o.Get()
		if err != nil {
			return nil, err
		}
		// Check if we are finished
		if dir == nil && obj == nil {
			break
		}
		if obj != nil {
			objs = append(objs, obj)
		}
	}
	return objs, nil
}

// GetDir will return a directory from the listing.
// It will skip over any objects.
// Will return (nil, nil) when all objects have been returned.
func (o *Lister) GetDir() (*Dir, error) {
	for {
		obj, dir, err := o.Get()
		if err != nil {
			return nil, err
		}
		// Check if we are finished
		if dir == nil && obj == nil {
			return nil, nil
		}
		// Ignore objects
		if obj != nil {
			continue
		}
		return dir, nil
	}
}

// GetDirs will return a slice of directories from the listing.
// It will skip over any objects.
func (o *Lister) GetDirs() (dirs []*Dir, err error) {
	for {
		obj, dir, err := o.Get()
		if err != nil {
			return nil, err
		}
		// Check if we are finished
		if dir == nil && obj == nil {
			break
		}
		if dir != nil {
			dirs = append(dirs, dir)
		}
	}
	return dirs, nil
}

// Objects is a slice of Object~s
type Objects []Object

// ObjectPair is a pair of Objects used to describe a potential copy
// operation.
type ObjectPair struct {
	src, dst Object
}

// ObjectPairChan is a channel of ObjectPair
type ObjectPairChan chan ObjectPair

// Dir describes a directory for directory/container/bucket lists
type Dir struct {
	Name  string    // name of the directory
	When  time.Time // modification or creation time - IsZero for unknown
	Bytes int64     // size of directory and contents -1 for unknown
	Count int64     // number of objects -1 for unknown
}

// DirChan is a channel of Dir objects
type DirChan chan *Dir

// Find looks for an Info object for the name passed in
//
// Services are looked up in the config file
func Find(name string) (*RegInfo, error) {
	for _, item := range fsRegistry {
		if item.Name == name {
			return item, nil
		}
	}
	return nil, fmt.Errorf("Didn't find filing system for %q", name)
}

// Pattern to match an rclone url
var matcher = regexp.MustCompile(`^([\w_ -]+):(.*)$`)

// NewFs makes a new Fs object from the path
//
// The path is of the form remote:path
//
// Remotes are looked up in the config file.  If the remote isn't
// found then NotFoundInConfigFile will be returned.
//
// On Windows avoid single character remote names as they can be mixed
// up with drive letters.
func NewFs(path string) (Fs, error) {
	parts := matcher.FindStringSubmatch(path)
	fsName, configName, fsPath := "local", "local", path
	if parts != nil && !isDriveLetter(parts[1]) {
		configName, fsPath = parts[1], parts[2]
		var err error
		fsName, err = ConfigFile.GetValue(configName, "type")
		if err != nil {
			return nil, ErrorNotFoundInConfigFile
		}
	}
	fs, err := Find(fsName)
	if err != nil {
		return nil, err
	}
	// change native directory separators to / if there are any
	fsPath = filepath.ToSlash(fsPath)
	return fs.NewFs(configName, fsPath)
}

// OutputLog logs for an object
func OutputLog(o interface{}, text string, args ...interface{}) {
	description := ""
	if o != nil {
		description = fmt.Sprintf("%v: ", o)
	}
	out := fmt.Sprintf(text, args...)
	log.Print(description + out)
}

// Debug writes debuging output for this Object or Fs
func Debug(o interface{}, text string, args ...interface{}) {
	if Config.Verbose {
		OutputLog(o, text, args...)
	}
}

// Log writes log output for this Object or Fs
func Log(o interface{}, text string, args ...interface{}) {
	if !Config.Quiet {
		OutputLog(o, text, args...)
	}
}

// ErrorLog writes error log output for this Object or Fs.  It
// unconditionally logs a message regardless of Config.Quiet or
// Config.Verbose.
func ErrorLog(o interface{}, text string, args ...interface{}) {
	OutputLog(o, text, args...)
}

// CheckClose is a utility function used to check the return from
// Close in a defer statement.
func CheckClose(c io.Closer, err *error) {
	cerr := c.Close()
	if *err == nil {
		*err = cerr
	}
}

// NewStaticObjectInfo returns a static ObjectInfo
// If hashes is nil and fs is not nil, the hash map will be replaced with
// empty hashes of the types supported by the fs.
func NewStaticObjectInfo(remote string, modTime time.Time, size int64, storable bool, hashes map[HashType]string, fs Info) ObjectInfo {
	info := &staticObjectInfo{
		remote:   remote,
		modTime:  modTime,
		size:     size,
		storable: storable,
		hashes:   hashes,
		fs:       fs,
	}
	if fs != nil && hashes == nil {
		set := fs.Hashes().Array()
		info.hashes = make(map[HashType]string)
		for _, ht := range set {
			info.hashes[ht] = ""
		}
	}
	return info
}

type staticObjectInfo struct {
	remote   string
	modTime  time.Time
	size     int64
	storable bool
	hashes   map[HashType]string
	fs       Info
}

func (i *staticObjectInfo) Fs() Info           { return i.fs }
func (i *staticObjectInfo) Remote() string     { return i.remote }
func (i *staticObjectInfo) ModTime() time.Time { return i.modTime }
func (i *staticObjectInfo) Size() int64        { return i.size }
func (i *staticObjectInfo) Storable() bool     { return i.storable }
func (i *staticObjectInfo) Hash(h HashType) (string, error) {
	if len(i.hashes) == 0 {
		return "", ErrHashUnsupported
	}
	if hash, ok := i.hashes[h]; ok {
		return hash, nil
	}
	return "", ErrHashUnsupported
}
