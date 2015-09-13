// Local filesystem interface
package local

// Note that all rclone paths should be / separated.  Anything coming
// from the filepath module will have \ separators on windows so
// should be converted using filepath.ToSlash.  Windows is quite happy
// with / separators so there is no need to convert them back.

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"hash"
	"io"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/ncw/rclone/fs"
	"strings"
	"regexp"
	"runtime"
)

// Register with Fs
func init() {
	fs.Register(&fs.FsInfo{
		Name:  "local",
		NewFs: NewFs,
	})
}

// FsLocal represents a local filesystem rooted at root
type FsLocal struct {
	name        string              // the name of the remote
	root        string              // The root directory
	precisionOk sync.Once           // Whether we need to read the precision
	precision   time.Duration       // precision of local filesystem
	warned      map[string]struct{} // whether we have warned about this string
}

// FsObjectLocal represents a local filesystem object
type FsObjectLocal struct {
	local  *FsLocal    // The Fs this object is part of
	remote string      // The remote path
	path   string      // The local path
	info   os.FileInfo // Interface for file info (always present)
	md5sum string      // the md5sum of the object or "" if not calculated
}

// ------------------------------------------------------------

// NewFs constructs an FsLocal from the path
func NewFs(name, root string) (fs.Fs, error) {
	var err error

	root = filterPath(root)

	f := &FsLocal{
		name:   name,
		root:   root,
		warned: make(map[string]struct{}),
	}
	// Check to see if this points to a file
	fi, err := os.Lstat(f.root)
	if err == nil && fi.Mode().IsRegular() {
		// It is a file, so use the parent as the root
		remote := path.Base(root)
		f.root = path.Dir(root)
		obj := f.NewFsObject(remote)
		// return a Fs Limited to this object
		return fs.NewLimited(f, obj), nil
	}
	return f, nil
}

// The name of the remote (as passed into NewFs)
func (f *FsLocal) Name() string {
	return f.name
}

// The root of the remote (as passed into NewFs)
func (f *FsLocal) Root() string {
	return f.root
}

// String converts this FsLocal to a string
func (f *FsLocal) String() string {
	return fmt.Sprintf("Local file system at %s", f.root)
}

// newFsObject makes a half completed FsObjectLocal
func (f *FsLocal) newFsObject(remote string) *FsObjectLocal {
	remote = filepath.ToSlash(remote)
	dstPath := filterPath(path.Join(f.root, f.cleanUtf8(remote)))
	return &FsObjectLocal{local: f, remote: remote, path: dstPath}
}

// Return an FsObject from a path
//
// May return nil if an error occurred
func (f *FsLocal) newFsObjectWithInfo(remote string, info os.FileInfo) fs.Object {
	o := f.newFsObject(remote)
	if info != nil {
		o.info = info
	} else {
		err := o.lstat()
		if err != nil {
			fs.Debug(o, "Failed to stat %s: %s", o.path, err)
			return nil
		}
	}
	return o
}

// Return an FsObject from a path
//
// May return nil if an error occurred
func (f *FsLocal) NewFsObject(remote string) fs.Object {
	return f.newFsObjectWithInfo(remote, nil)
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
				fs.ErrorLog(f, "Failed to open directory: %s: %s", path, err)
			} else {
				remote, err := filepath.Rel(f.root, path)
				if err != nil {
					fs.Stats.Error()
					fs.ErrorLog(f, "Failed to get relative path %s: %s", path, err)
					return nil
				}
				if remote == "." {
					return nil
					// remote = ""
				}
				if fs := f.newFsObjectWithInfo(remote, fi); fs != nil {
					if fs.Storable() {
						out <- fs
					}
				}
			}
			return nil
		})
		if err != nil {
			fs.Stats.Error()
			fs.ErrorLog(f, "Failed to open directory: %s: %s", f.root, err)
		}
		close(out)
	}()
	return out
}

// CleanUtf8 makes string a valid UTF-8 string
//
// Any invalid UTF-8 characters will be replaced with utf8.RuneError
func (f *FsLocal) cleanUtf8(name string) string {
	if !utf8.ValidString(name) {
		if _, ok := f.warned[name]; !ok {
			fs.Debug(f, "Replacing invalid UTF-8 characters in %q", name)
			f.warned[name] = struct{}{}
		}
		name = string([]rune(name))
	}
	if runtime.GOOS == "windows" {
		name2 := strings.Map(func(r rune) rune {
			switch r {
			case '<', '>', ':', '"', '|', '?', '*', '&':
				return '_'
			}
			return r
		}, name)

		if name2 != name {
			if _, ok := f.warned[name]; !ok {
				fs.Debug(f, "Replacing invalid UTF-8 characters in %q", name)
				f.warned[name] = struct {}{}
			}
			name = name2
		}
	}
	return name
}

// Walk the path returning a channel of FsObjects
func (f *FsLocal) ListDir() fs.DirChan {
	out := make(fs.DirChan, fs.Config.Checkers)
	go func() {
		defer close(out)
		items, err := ioutil.ReadDir(f.root)
		if err != nil {
			fs.Stats.Error()
			fs.ErrorLog(f, "Couldn't find read directory: %s", err)
		} else {
			for _, item := range items {
				if item.IsDir() {
					dir := &fs.Dir{
						Name:  f.cleanUtf8(item.Name()),
						When:  item.ModTime(),
						Bytes: 0,
						Count: 0,
					}
					// Go down the tree to count the files and directories
					dirpath := path.Join(f.root, item.Name())
					err := filepath.Walk(dirpath, func(path string, fi os.FileInfo, err error) error {
						if err != nil {
							fs.Stats.Error()
							fs.ErrorLog(f, "Failed to open directory: %s: %s", path, err)
						} else {
							dir.Count += 1
							dir.Bytes += fi.Size()
						}
						return nil
					})
					if err != nil {
						fs.Stats.Error()
						fs.ErrorLog(f, "Failed to open directory: %s: %s", dirpath, err)
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
	// Temporary FsObject under construction - info filled in by Update()
	o := f.newFsObject(remote)
	err := o.Update(in, modTime, size)
	if err != nil {
		return nil, err
	}
	return o, nil
}

// Mkdir creates the directory if it doesn't exist
func (f *FsLocal) Mkdir() error {
	// FIXME: https://github.com/syncthing/syncthing/blob/master/lib/osutil/mkdirall_windows.go
	return os.MkdirAll(f.root, 0777)
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
	err = fd.Close()
	if err != nil {
		return time.Second
	}

	// Delete it on return
	defer func() {
		// fmt.Println("Remove temp file")
		_ = os.Remove(path) // ignore error
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

// Purge deletes all the files and directories
//
// Optional interface: Only implement this if you have a way of
// deleting all the files quicker than just running Remove() on the
// result of List()
func (f *FsLocal) Purge() error {
	fi, err := os.Lstat(f.root)
	if err != nil {
		return err
	}
	if !fi.Mode().IsDir() {
		return fmt.Errorf("Can't Purge non directory: %q", f.root)
	}
	return os.RemoveAll(f.root)
}

// Move src to this remote using server side move operations.
//
// This is stored with the remote path given
//
// It returns the destination Object and a possible error
//
// Will only be called if src.Fs().Name() == f.Name()
//
// If it isn't possible then return fs.ErrorCantMove
func (dstFs *FsLocal) Move(src fs.Object, remote string) (fs.Object, error) {
	srcObj, ok := src.(*FsObjectLocal)
	if !ok {
		fs.Debug(src, "Can't move - not same remote type")
		return nil, fs.ErrorCantMove
	}

	// Temporary FsObject under construction
	dstObj := dstFs.newFsObject(remote)

	// Check it is a file if it exists
	err := dstObj.lstat()
	if os.IsNotExist(err) {
		// OK
	} else if err != nil {
		return nil, err
	} else if !dstObj.info.Mode().IsRegular() {
		// It isn't a file
		return nil, fmt.Errorf("Can't move file onto non-file")
	}

	// Create destination
	err = dstObj.mkdirAll()
	if err != nil {
		return nil, err
	}

	// Do the move
	err = os.Rename(srcObj.path, dstObj.path)
	if err != nil {
		return nil, err
	}

	// Update the info
	err = dstObj.lstat()
	if err != nil {
		return nil, err
	}

	return dstObj, nil
}

// Move src to this remote using server side move operations.
//
// Will only be called if src.Fs().Name() == f.Name()
//
// If it isn't possible then return fs.ErrorCantDirMove
//
// If destination exists then return fs.ErrorDirExists
func (dstFs *FsLocal) DirMove(src fs.Fs) error {
	srcFs, ok := src.(*FsLocal)
	if !ok {
		fs.Debug(srcFs, "Can't move directory - not same remote type")
		return fs.ErrorCantDirMove
	}
	// Check if destination exists
	_, err := os.Lstat(dstFs.root)
	if !os.IsNotExist(err) {
		return fs.ErrorDirExists
	}
	// Do the move
	return os.Rename(srcFs.root, dstFs.root)
}

// ------------------------------------------------------------

// Return the parent Fs
func (o *FsObjectLocal) Fs() fs.Fs {
	return o.local
}

// Return a string version
func (o *FsObjectLocal) String() string {
	if o == nil {
		return "<nil>"
	}
	return o.remote
}

// Return the remote path
func (o *FsObjectLocal) Remote() string {
	return o.local.cleanUtf8(o.remote)
}


// Md5sum calculates the Md5sum of a file returning a lowercase hex string
func (o *FsObjectLocal) Md5sum() (string, error) {
	if o.md5sum != "" {
		return o.md5sum, nil
	}
	in, err := os.Open(o.path)
	if err != nil {
		fs.Stats.Error()
		fs.ErrorLog(o, "Failed to open: %s", err)
		return "", err
	}
	hash := md5.New()
	_, err = io.Copy(hash, in)
	closeErr := in.Close()
	if err != nil {
		fs.Stats.Error()
		fs.ErrorLog(o, "Failed to read: %s", err)
		return "", err
	}
	if closeErr != nil {
		fs.Stats.Error()
		fs.ErrorLog(o, "Failed to close: %s", closeErr)
		return "", closeErr
	}
	o.md5sum = hex.EncodeToString(hash.Sum(nil))
	return o.md5sum, nil
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
		return
	}
	// Re-read metadata
	err = o.lstat()
	if err != nil {
		fs.Debug(o, "Failed to stat: %s", err)
		return
	}
}

// Is this object storable
func (o *FsObjectLocal) Storable() bool {
	mode := o.info.Mode()
	if mode&(os.ModeSymlink|os.ModeNamedPipe|os.ModeSocket|os.ModeDevice) != 0 {
		fs.Debug(o, "Can't transfer non file/directory")
		return false
	} else if mode&os.ModeDir != 0 {
		// fs.Debug(o, "Skipping directory")
		return false
	}
	return true
}

// localOpenFile wraps an io.ReadCloser and updates the md5sum of the
// object that is read
type localOpenFile struct {
	o    *FsObjectLocal // object that is open
	in   io.ReadCloser  // handle we are wrapping
	hash hash.Hash      // currently accumulating MD5
}

// Read bytes from the object - see io.Reader
func (file *localOpenFile) Read(p []byte) (n int, err error) {
	n, err = file.in.Read(p)
	if n > 0 {
		// Hash routines never return an error
		_, _ = file.hash.Write(p[:n])
	}
	return
}

// Close the object and update the md5sum
func (file *localOpenFile) Close() (err error) {
	err = file.in.Close()
	if err == nil {
		file.o.md5sum = hex.EncodeToString(file.hash.Sum(nil))
	} else {
		file.o.md5sum = ""
	}
	return err
}

// Open an object for read
func (o *FsObjectLocal) Open() (in io.ReadCloser, err error) {
	in, err = os.Open(o.path)
	if err != nil {
		return
	}
	// Update the md5sum as we go along
	in = &localOpenFile{
		o:    o,
		in:   in,
		hash: md5.New(),
	}
	return
}

// mkdirAll makes all the directories needed to store the object
func (o *FsObjectLocal) mkdirAll() error {
	dir := path.Dir(o.path)
	fmt.Printf("Create %q from %q\n", dir, o.path)
	return os.MkdirAll(dir, 0777)
}

// Update the object from in with modTime and size
func (o *FsObjectLocal) Update(in io.Reader, modTime time.Time, size int64) error {
	err := o.mkdirAll()
	if err != nil {
		return fmt.Errorf("mkdirall:%s",err)
	}

	out, err := os.Create(o.path)
	if err != nil {
		time.Sleep(5*time.Minute)
		return fmt.Errorf("oscreate:%s",err)
		return err
	}

	// Calculate the md5sum of the object we are reading as we go along
	hash := md5.New()
	in = io.TeeReader(in, hash)

	_, err = io.Copy(out, in)
	outErr := out.Close()
	if err != nil {
		return err
	}
	if outErr != nil {
		return outErr
	}

	// All successful so update the md5sum
	o.md5sum = hex.EncodeToString(hash.Sum(nil))

	// Set the mtime
	o.SetModTime(modTime)

	// ReRead info now that we have finished
	return o.lstat()
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


func filterPath(s string) string {
	s = path.Clean(s)

	if !filepath.IsAbs(s) {
		s2, err := filepath.Abs(s)
		if err == nil {
			s = s2
		}
	}

	if runtime.GOOS == "windows" {
		// Convert to UNC
		s = uncPath(s)
	}
	return s
}

// Pattern to match a windows absolute path: c:\temp path.
var isAbsWinDrive = regexp.MustCompile(`[a-zA-Z]\:\\`)

// uncPath converts an absolute Windows path
// to a UNC long path.
func uncPath(s string) string {
	// Make path use `\`
	// UNC can NOT use "/"
	s = strings.Replace(s, `/`, `\`, -1)

	// If prefix is "\\", we already have a UNC path or server.
	if strings.HasPrefix(s, `\\`) {
		// If already long path, just keep it
		if strings.HasPrefix(s,`\\?\`) {
			return s
		}
		// Trim "//" from path and add UNC prefix.
		return `\\?\UNC\`+strings.TrimPrefix(s, `\\`)
	}
	if isAbsWinDrive.Match([]byte(s)) {
		return `\\?\` + s
	}
	return s
}

// Check the interfaces are satisfied
var _ fs.Fs = &FsLocal{}
var _ fs.Purger = &FsLocal{}
var _ fs.Mover = &FsLocal{}
var _ fs.DirMover = &FsLocal{}
var _ fs.Object = &FsObjectLocal{}
