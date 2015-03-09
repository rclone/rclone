// File system interface

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
	// User agent for Fs which can set it
	UserAgent = "rclone/" + Version
)

// Globals
var (
	// Filesystem registry
	fsRegistry []*FsInfo
	// Error returned by NewFs if not found in config file
	NotFoundInConfigFile = fmt.Errorf("Didn't find section in config file")
)

// Filesystem info
type FsInfo struct {
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

// An options for a Fs
type Option struct {
	Name     string
	Help     string
	Optional bool
	Examples []OptionExample
}

// An example for an option
type OptionExample struct {
	Value string
	Help  string
}

// Register a filesystem
//
// Fs modules  should use this in an init() function
func Register(info *FsInfo) {
	fsRegistry = append(fsRegistry, info)
}

// A Filesystem, describes the local filesystem and the remote object store
type Fs interface {
	// String returns a description of the FS
	String() string

	// List the Fs into a channel
	List() ObjectsChan

	// List the Fs directories/buckets/containers into a channel
	ListDir() DirChan

	// Find the Object at remote.  Returns nil if can't be found
	NewFsObject(remote string) Object

	// Put in to the remote path with the modTime given of the given size
	//
	// May create the object even if it returns an error - if so
	// will return the object and the error, otherwise will return
	// nil and the error
	Put(in io.Reader, remote string, modTime time.Time, size int64) (Object, error)

	// Make the directory (container, bucket)
	//
	// Shouldn't return an error if it already exists
	Mkdir() error

	// Remove the directory (container, bucket) if empty
	//
	// Return an error if it doesn't exist or isn't empty
	Rmdir() error

	// Precision of the ModTimes in this Fs
	Precision() time.Duration
}

// A filesystem like object which can either be a remote object or a
// local file/directory
type Object interface {
	// String returns a description of the Object
	String() string

	// Fs returns the Fs that this object is part of
	Fs() Fs

	// Remote returns the remote path
	Remote() string

	// Md5sum returns the md5 checksum of the file
	Md5sum() (string, error)

	// ModTime returns the modification date of the file
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

// Optional interfaces
type Purger interface {
	// Purge all files in the root and the root directory
	//
	// Implement this if you have a way of deleting all the files
	// quicker than just running Remove() on the result of List()
	//
	// Return an error if it doesn't exist
	Purge() error
}

// An optional interface for error as to whether the operation should be retried
//
// This should be returned from Update or Put methods as required
type Retry interface {
	error
	Retry() bool
}

// A type of error
type retryError string

// Error interface
func (r retryError) Error() string {
	return string(r)
}

// Retry interface
func (r retryError) Retry() bool {
	return true
}

// Check interface
var _ Retry = retryError("")

// RetryErrorf makes an error which indicates it would like to be retried
func RetryErrorf(format string, a ...interface{}) error {
	return retryError(fmt.Sprintf(format, a...))
}

// A channel of Objects
type ObjectsChan chan Object

// A slice of Objects
type Objects []Object

// A pair of Objects
type ObjectPair struct {
	src, dst Object
}

// A channel of ObjectPair
type ObjectPairChan chan ObjectPair

// A structure of directory/container/bucket lists
type Dir struct {
	Name  string    // name of the directory
	When  time.Time // modification or creation time - IsZero for unknown
	Bytes int64     // size of directory and contents -1 for unknown
	Count int64     // number of objects -1 for unknown
}

// A channel of Dir objects
type DirChan chan *Dir

// Finds a FsInfo object for the name passed in
//
// Services are looked up in the config file
func Find(name string) (*FsInfo, error) {
	for _, item := range fsRegistry {
		if item.Name == name {
			return item, nil
		}
	}
	return nil, fmt.Errorf("Didn't find filing system for %q", name)
}

// Pattern to match an rclone url
var matcher = regexp.MustCompile(`^([\w_-]+):(.*)$`)

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
			return nil, NotFoundInConfigFile
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

// Outputs log for object
func OutputLog(o interface{}, text string, args ...interface{}) {
	description := ""
	if o != nil {
		description = fmt.Sprintf("%v: ", o)
	}
	out := fmt.Sprintf(text, args...)
	log.Print(description + out)
}

// Write debuging output for this Object or Fs
func Debug(o interface{}, text string, args ...interface{}) {
	if Config.Verbose {
		OutputLog(o, text, args...)
	}
}

// Write log output for this Object or Fs
func Log(o interface{}, text string, args ...interface{}) {
	if !Config.Quiet {
		OutputLog(o, text, args...)
	}
}

// checkClose is a utility function used to check the return from
// Close in a defer statement.
func checkClose(c io.Closer, err *error) {
	cerr := c.Close()
	if *err == nil {
		*err = cerr
	}
}
