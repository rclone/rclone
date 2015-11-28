// Package fs is a generic file system interface for rclone object storage systems
package fs

import (
	"fmt"
	"io"
	"log"
	"path/filepath"
	"regexp"
	"time"
)

// Constants
const (
	// UserAgent for Fs which can set it
	UserAgent = "rclone/" + Version
	// ModTimeNotSupported is a very large precision value to show
	// mod time isn't supported on this Fs
	ModTimeNotSupported = 100 * 365 * 24 * time.Hour
)

// Globals
var (
	// Filesystem registry
	fsRegistry []*Info
	// ErrorNotFoundInConfigFile is returned by NewFs if not found in config file
	ErrorNotFoundInConfigFile = fmt.Errorf("Didn't find section in config file")
	ErrorCantPurge            = fmt.Errorf("Can't purge directory")
	ErrorCantCopy             = fmt.Errorf("Can't copy object - incompatible remotes")
	ErrorCantMove             = fmt.Errorf("Can't copy object - incompatible remotes")
	ErrorCantDirMove          = fmt.Errorf("Can't copy directory - incompatible remotes")
	ErrorDirExists            = fmt.Errorf("Can't copy directory - destination already exists")
)

// Info information about a filesystem
type Info struct {
	// Name of this fs
	Name string
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
	Examples []OptionExample
}

// OptionExample describes an example for an Option
type OptionExample struct {
	Value string
	Help  string
}

// Register a filesystem
//
// Fs modules  should use this in an init() function
func Register(info *Info) {
	fsRegistry = append(fsRegistry, info)
}

// Fs is the interface a cloud storage system must provide
type Fs interface {
	// Name of the remote (as passed into NewFs)
	Name() string

	// Root of the remote (as passed into NewFs)
	Root() string

	// String returns a description of the FS
	String() string

	// List the Fs into a channel
	List() ObjectsChan

	// ListDir lists the Fs directories/buckets/containers into a channel
	ListDir() DirChan

	// NewFsObject finds the Object at remote.  Returns nil if can't be found
	NewFsObject(remote string) Object

	// Put in to the remote path with the modTime given of the given size
	//
	// May create the object even if it returns an error - if so
	// will return the object and the error, otherwise will return
	// nil and the error
	Put(in io.Reader, remote string, modTime time.Time, size int64) (Object, error)

	// Mkdir makes the directory (container, bucket)
	//
	// Shouldn't return an error if it already exists
	Mkdir() error

	// Rmdir removes the directory (container, bucket) if empty
	//
	// Return an error if it doesn't exist or isn't empty
	Rmdir() error

	// Precision of the ModTimes in this Fs
	Precision() time.Duration
}

// Object is a filesystem like object provided by an Fs
type Object interface {
	// String returns a description of the Object
	String() string

	// Fs returns the Fs that this object is part of
	Fs() Fs

	// Remote returns the remote path
	Remote() string

	// Md5sum returns the md5 checksum of the file
	// If no Md5sum is available it returns ""
	Md5sum() (string, error)

	// ModTime returns the modification date of the file
	// It should return a best guess if one isn't available
	ModTime() time.Time

	// SetModTime sets the metadata on the object to set the modification date
	SetModTime(time.Time)

	// Size returns the size of the file
	Size() int64

	// Open opens the file for read.  Call Close() on the returned io.ReadCloser
	Open() (io.ReadCloser, error)

	// Update in to the object with the modTime given of the given size
	Update(in io.Reader, modTime time.Time, size int64) error

	// Storable says whether this object can be stored
	Storable() bool

	// Removes this object
	Remove() error
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
func Find(name string) (*Info, error) {
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
