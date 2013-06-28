// Local filesystem interface
package local

import (
	"../fs"
	"crypto/md5"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sync"
	"time"
)

// Pattern to match a local url (matches anything)
var Match = regexp.MustCompile(``)

// Register with Fs
func init() {
	fs.Register(Match, NewFs)
}

// FsLocal represents a local filesystem rooted at root
type FsLocal struct {
	root        string        // The root directory
	precisionOk sync.Once     // Whether we need to read the precision
	precision   time.Duration // precision of local filesystem
}

// FsObjectLocal represents a local filesystem object
type FsObjectLocal struct {
	remote string      // The remote path
	path   string      // The local path
	info   os.FileInfo // Interface for file info
}

// ------------------------------------------------------------

// NewFs contstructs an FsLocal from the path
func NewFs(root string) (fs.Fs, error) {
	root = path.Clean(root)
	f := &FsLocal{root: root}
	return f, nil
}

// String converts this FsLocal to a string
func (f *FsLocal) String() string {
	return fmt.Sprintf("Local file system at %s", f.root)
}

// Return an FsObject from a path
//
// May return nil if an error occurred
func (f *FsLocal) NewFsObjectWithInfo(remote string, info os.FileInfo) fs.Object {
	path := filepath.Join(f.root, remote)
	o := &FsObjectLocal{remote: remote, path: path}
	if info != nil {
		o.info = info
	} else {
		err := o.lstat()
		if err != nil {
			fs.Debug(o, "Failed to stat %s: %s", path, err)
			return nil
		}
	}
	return o
}

// Return an FsObject from a path
//
// May return nil if an error occurred
func (f *FsLocal) NewFsObject(remote string) fs.Object {
	return f.NewFsObjectWithInfo(remote, nil)
}

// List the path returning a channel of FsObjects
//
// Ignores everything which isn't Storable, eg links etc
func (f *FsLocal) List() fs.ObjectsChan {
	out := make(fs.ObjectsChan, fs.Config.Checkers)
	go func() {
		err := filepath.Walk(f.root, func(path string, fi os.FileInfo, err error) error {
			if err != nil {
				fs.Stats.Error()
				log.Printf("Failed to open directory: %s: %s", path, err)
			} else {
				remote, err := filepath.Rel(f.root, path)
				if err != nil {
					fs.Stats.Error()
					log.Printf("Failed to get relative path %s: %s", path, err)
					return nil
				}
				if remote == "." {
					return nil
					// remote = ""
				}
				if fs := f.NewFsObjectWithInfo(remote, fi); fs != nil {
					if fs.Storable() {
						out <- fs
					}
				}
			}
			return nil
		})
		if err != nil {
			fs.Stats.Error()
			log.Printf("Failed to open directory: %s: %s", f.root, err)
		}
		close(out)
	}()
	return out
}

// Walk the path returning a channel of FsObjects
func (f *FsLocal) ListDir() fs.DirChan {
	out := make(fs.DirChan, fs.Config.Checkers)
	go func() {
		defer close(out)
		items, err := ioutil.ReadDir(f.root)
		if err != nil {
			fs.Stats.Error()
			log.Printf("Couldn't find read directory: %s", err)
		} else {
			for _, item := range items {
				if item.IsDir() {
					dir := &fs.Dir{
						Name:  item.Name(),
						When:  item.ModTime(),
						Bytes: 0,
						Count: 0,
					}
					// Go down the tree to count the files and directories
					dirpath := path.Join(f.root, item.Name())
					err := filepath.Walk(dirpath, func(path string, fi os.FileInfo, err error) error {
						if err != nil {
							fs.Stats.Error()
							log.Printf("Failed to open directory: %s: %s", path, err)
						} else {
							dir.Count += 1
							dir.Bytes += fi.Size()
						}
						return nil
					})
					if err != nil {
						fs.Stats.Error()
						log.Printf("Failed to open directory: %s: %s", dirpath, err)
					}
					out <- dir
				}
			}
		}
		// err := f.findRoot(false)
	}()
	return out
}

// Puts the FsObject to the local filesystem
func (f *FsLocal) Put(in io.Reader, remote string, modTime time.Time, size int64) (fs.Object, error) {
	dstPath := filepath.Join(f.root, remote)
	// Temporary FsObject under construction
	fs := &FsObjectLocal{remote: remote, path: dstPath}

	dir := path.Dir(dstPath)
	err := os.MkdirAll(dir, 0770)
	if err != nil {
		return fs, err
	}

	out, err := os.Create(dstPath)
	if err != nil {
		return fs, err
	}

	_, err = io.Copy(out, in)
	outErr := out.Close()
	if err != nil {
		return fs, err
	}
	if outErr != nil {
		return fs, outErr
	}

	// Set the mtime
	fs.SetModTime(modTime)
	return fs, err
}

// Mkdir creates the directory if it doesn't exist
func (f *FsLocal) Mkdir() error {
	return os.MkdirAll(f.root, 0770)
}

// Rmdir removes the directory
//
// If it isn't empty it will return an error
func (f *FsLocal) Rmdir() error {
	return os.Remove(f.root)
}

// Return the precision
func (f *FsLocal) Precision() (precision time.Duration) {
	f.precisionOk.Do(func() {
		f.precision = f.readPrecision()
	})
	return f.precision
}

// Read the precision
func (f *FsLocal) readPrecision() (precision time.Duration) {
	// Default precision of 1s
	precision = time.Second

	// Create temporary file and test it
	fd, err := ioutil.TempFile("", "rclone")
	if err != nil {
		// If failed return 1s
		// fmt.Println("Failed to create temp file", err)
		return time.Second
	}
	path := fd.Name()
	// fmt.Println("Created temp file", path)
	fd.Close()

	// Delete it on return
	defer func() {
		// fmt.Println("Remove temp file")
		os.Remove(path)
	}()

	// Find the minimum duration we can detect
	for duration := time.Duration(1); duration < time.Second; duration *= 10 {
		// Current time with delta
		t := time.Unix(time.Now().Unix(), int64(duration))
		err := os.Chtimes(path, t, t)
		if err != nil {
			// fmt.Println("Failed to Chtimes", err)
			break
		}

		// Read the actual time back
		fi, err := os.Stat(path)
		if err != nil {
			// fmt.Println("Failed to Stat", err)
			break
		}

		// If it matches - have found the precision
		// fmt.Println("compare", fi.ModTime(), t)
		if fi.ModTime() == t {
			// fmt.Println("Precision detected as", duration)
			return duration
		}
	}
	return
}

// ------------------------------------------------------------

// Return the remote path
func (o *FsObjectLocal) Remote() string {
	return o.remote
}

// Md5sum calculates the Md5sum of a file returning a lowercase hex string
func (o *FsObjectLocal) Md5sum() (string, error) {
	in, err := os.Open(o.path)
	if err != nil {
		fs.Stats.Error()
		fs.Log(o, "Failed to open: %s", err)
		return "", err
	}
	defer in.Close() // FIXME ignoring error
	hash := md5.New()
	_, err = io.Copy(hash, in)
	if err != nil {
		fs.Stats.Error()
		fs.Log(o, "Failed to read: %s", err)
		return "", err
	}
	return fmt.Sprintf("%x", hash.Sum(nil)), nil
}

// Size returns the size of an object in bytes
func (o *FsObjectLocal) Size() int64 {
	return o.info.Size()
}

// ModTime returns the modification time of the object
func (o *FsObjectLocal) ModTime() time.Time {
	return o.info.ModTime()
}

// Sets the modification time of the local fs object
func (o *FsObjectLocal) SetModTime(modTime time.Time) {
	err := os.Chtimes(o.path, modTime, modTime)
	if err != nil {
		fs.Debug(o, "Failed to set mtime on file: %s", err)
	}
}

// Is this object storable
func (o *FsObjectLocal) Storable() bool {
	mode := o.info.Mode()
	if mode&(os.ModeSymlink|os.ModeNamedPipe|os.ModeSocket|os.ModeDevice) != 0 {
		fs.Debug(o, "Can't transfer non file/directory")
		return false
	} else if mode&os.ModeDir != 0 {
		fs.Debug(o, "FIXME Skipping directory")
		return false
	}
	return true
}

// Open an object for read
func (o *FsObjectLocal) Open() (in io.ReadCloser, err error) {
	in, err = os.Open(o.path)
	return
}

// Stat a FsObject into info
func (o *FsObjectLocal) lstat() error {
	info, err := os.Lstat(o.path)
	o.info = info
	return err
}

// Remove an object
func (o *FsObjectLocal) Remove() error {
	return os.Remove(o.path)
}

// Check the interfaces are satisfied
var _ fs.Fs = &FsLocal{}
var _ fs.Object = &FsObjectLocal{}
