// Package fs is a generic file system interface for rclone object storage systems
package fs

import (
	"fmt"
	"io"
	"log"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"time"

	"github.com/pkg/errors"
)

// Constants
const (
	// ModTimeNotSupported is a very large precision value to show
	// mod time isn't supported on this Fs
	ModTimeNotSupported = 100 * 365 * 24 * time.Hour
	// MaxLevel is a sentinel representing an infinite depth for listings
	MaxLevel = math.MaxInt32
)

// Globals
var (
	// UserAgent for Fs which can set it
	UserAgent = "rclone/" + Version
	// Filesystem registry
	fsRegistry []*RegInfo
	// ErrorNotFoundInConfigFile is returned by NewFs if not found in config file
	ErrorNotFoundInConfigFile = errors.New("didn't find section in config file")
	ErrorCantPurge            = errors.New("can't purge directory")
	ErrorCantCopy             = errors.New("can't copy object - incompatible remotes")
	ErrorCantMove             = errors.New("can't move object - incompatible remotes")
	ErrorCantDirMove          = errors.New("can't move directory - incompatible remotes")
	ErrorDirExists            = errors.New("can't copy directory - destination already exists")
	ErrorCantSetModTime       = errors.New("can't set modified time")
	ErrorDirNotFound          = errors.New("directory not found")
	ErrorObjectNotFound       = errors.New("object not found")
	ErrorLevelNotSupported    = errors.New("level value not supported")
	ErrorListAborted          = errors.New("list aborted")
	ErrorListOnlyRoot         = errors.New("can only list from root")
	ErrorIsFile               = errors.New("is a file not a directory")
	ErrorNotDeleting          = errors.New("not deleting files as there were IO errors")
	ErrorCantMoveOverlapping  = errors.New("can't move files on overlapping remotes")
)

// RegInfo provides information about a filesystem
type RegInfo struct {
	// Name of this fs
	Name string
	// Description of this fs - defaults to Name
	Description string
	// Create a new file system.  If root refers to an existing
	// object, then it should return a Fs which which points to
	// the parent of that object and ErrorIsFile.
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

// ListFser is the interface for listing a remote Fs
type ListFser interface {
	// List the objects and directories of the Fs starting from dir
	//
	// dir should be "" to start from the root, and should not
	// have trailing slashes.
	//
	// This should return ErrDirNotFound (using out.SetError())
	// if the directory isn't found.
	//
	// Fses must support recursion levels of fs.MaxLevel and 1.
	// They may return ErrorLevelNotSupported otherwise.
	List(out ListOpts, dir string)
}

// Fs is the interface a cloud storage system must provide
type Fs interface {
	Info
	ListFser

	// NewObject finds the Object at remote.  If it can't be found
	// it returns the error ErrorObjectNotFound.
	NewObject(remote string) (Object, error)

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
	BasicInfo

	// Fs returns read only access to the Fs that this object is part of
	Fs() Info

	// Hash returns the selected checksum of the file
	// If no checksum is available it returns ""
	Hash(HashType) (string, error)

	// Storable says whether this object can be stored
	Storable() bool
}

// BasicInfo common interface for Dir and Object providing the very
// basic attributes of an object.
type BasicInfo interface {
	// Remote returns the remote path
	Remote() string

	// ModTime returns the modification date of the file
	// It should return a best guess if one isn't available
	ModTime() time.Time

	// Size returns the size of the file
	Size() int64
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

// PutUncheckeder is an optional interface for Fs
type PutUncheckeder interface {
	// Put in to the remote path with the modTime given of the given size
	//
	// May create the object even if it returns an error - if so
	// will return the object and the error, otherwise will return
	// nil and the error
	//
	// May create duplicates or return errors if src already
	// exists.
	PutUnchecked(in io.Reader, src ObjectInfo) (Object, error)
}

// CleanUpper is an optional interfaces for Fs
type CleanUpper interface {
	// CleanUp the trash in the Fs
	//
	// Implement this if you have a way of emptying the trash or
	// otherwise cleaning up old versions of files.
	CleanUp() error
}

// ObjectsChan is a channel of Objects
type ObjectsChan chan Object

// ListOpts describes the interface used for Fs.List operations
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

// Remote returns the remote path
func (d *Dir) Remote() string {
	return d.Name
}

// ModTime returns the modification date of the file
// It should return a best guess if one isn't available
func (d *Dir) ModTime() time.Time {
	if !d.When.IsZero() {
		return d.When
	}
	return time.Now()
}

// Size returns the size of the file
func (d *Dir) Size() int64 {
	return d.Bytes
}

// Check interface
var _ BasicInfo = (*Dir)(nil)

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
	return nil, errors.Errorf("didn't find filing system for %q", name)
}

// Pattern to match an rclone url
var matcher = regexp.MustCompile(`^([\w_ -]+):(.*)$`)

// ParseRemote deconstructs a path into configName, fsPath, looking up
// the fsName in the config file (returning NotFoundInConfigFile if not found)
func ParseRemote(path string) (fsInfo *RegInfo, configName, fsPath string, err error) {
	parts := matcher.FindStringSubmatch(path)
	var fsName string
	fsName, configName, fsPath = "local", "local", path
	if parts != nil && !isDriveLetter(parts[1]) {
		configName, fsPath = parts[1], parts[2]
		var err error
		fsName, err = ConfigFile.GetValue(configName, "type")
		if err != nil {
			return nil, "", "", ErrorNotFoundInConfigFile
		}
	}
	// change native directory separators to / if there are any
	fsPath = filepath.ToSlash(fsPath)
	fsInfo, err = Find(fsName)
	return fsInfo, configName, fsPath, err
}

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
	fsInfo, configName, fsPath, err := ParseRemote(path)
	if err != nil {
		return nil, err
	}
	return fsInfo.NewFs(configName, fsPath)
}

// DebugLogger - logs to Stdout
var DebugLogger = log.New(os.Stdout, "", log.LstdFlags)

// makeLog produces a log string from the arguments passed in
func makeLog(o interface{}, text string, args ...interface{}) string {
	out := fmt.Sprintf(text, args...)
	if o == nil {
		return out
	}
	return fmt.Sprintf("%v: %s", o, out)
}

// Debug writes debugging output for this Object or Fs
func Debug(o interface{}, text string, args ...interface{}) {
	if Config.Verbose {
		DebugLogger.Print(makeLog(o, text, args...))
	}
}

// Log writes log output for this Object or Fs.  This should be
// considered to be Info level logging.
func Log(o interface{}, text string, args ...interface{}) {
	if !Config.Quiet {
		log.Print(makeLog(o, text, args...))
	}
}

// ErrorLog writes error log output for this Object or Fs.  It
// unconditionally logs a message regardless of Config.Quiet or
// Config.Verbose.
func ErrorLog(o interface{}, text string, args ...interface{}) {
	log.Print(makeLog(o, text, args...))
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
