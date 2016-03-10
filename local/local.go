// Package local provides a filesystem interface
package local

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/ncw/rclone/fs"
)

// Register with Fs
func init() {
	fsi := &fs.RegInfo{
		Name:        "local",
		Description: "Local Disk",
		NewFs:       NewFs,
		Options: []fs.Option{{
			Name:     "nounc",
			Help:     "Disable UNC (long path names) conversion on Windows",
			Optional: true,
			Examples: []fs.OptionExample{{
				Value: "true",
				Help:  "Disables long file names",
			}},
		}},
	}
	fs.Register(fsi)
}

// Fs represents a local filesystem rooted at root
type Fs struct {
	name        string              // the name of the remote
	root        string              // The root directory
	precisionOk sync.Once           // Whether we need to read the precision
	precision   time.Duration       // precision of local filesystem
	wmu         sync.Mutex          // used for locking access to 'warned'.
	warned      map[string]struct{} // whether we have warned about this string
	nounc       bool                // Skip UNC conversion on Windows
}

// Object represents a local filesystem object
type Object struct {
	fs     *Fs                    // The Fs this object is part of
	remote string                 // The remote path
	path   string                 // The local path
	info   os.FileInfo            // Interface for file info (always present)
	hashes map[fs.HashType]string // Hashes
}

// ------------------------------------------------------------

// NewFs constructs an Fs from the path
func NewFs(name, root string) (fs.Fs, error) {
	var err error

	nounc, _ := fs.ConfigFile.GetValue(name, "nounc")
	f := &Fs{
		name:   name,
		warned: make(map[string]struct{}),
		nounc:  nounc == "true",
	}
	f.root = f.filterPath(f.cleanUtf8(root))

	// Check to see if this points to a file
	fi, err := os.Lstat(f.root)
	if err == nil && fi.Mode().IsRegular() {
		// It is a file, so use the parent as the root
		var remote string
		f.root, remote = getDirFile(f.root)
		obj := f.NewFsObject(remote)
		// return a Fs Limited to this object
		return fs.NewLimited(f, obj), nil
	}
	return f, nil
}

// Name of the remote (as passed into NewFs)
func (f *Fs) Name() string {
	return f.name
}

// Root of the remote (as passed into NewFs)
func (f *Fs) Root() string {
	return f.root
}

// String converts this Fs to a string
func (f *Fs) String() string {
	return fmt.Sprintf("Local file system at %s", f.root)
}

// newFsObject makes a half completed Object
func (f *Fs) newFsObject(remote string) *Object {
	remote = filepath.ToSlash(remote)
	dstPath := f.filterPath(filepath.Join(f.root, f.cleanUtf8(remote)))
	return &Object{
		fs:     f,
		remote: remote,
		path:   dstPath,
	}
}

// Return an FsObject from a path
//
// May return nil if an error occurred
func (f *Fs) newFsObjectWithInfo(remote string, info os.FileInfo) fs.Object {
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

// NewFsObject returns an FsObject from a path
//
// May return nil if an error occurred
func (f *Fs) NewFsObject(remote string) fs.Object {
	return f.newFsObjectWithInfo(remote, nil)
}

// List the path returning a channel of FsObjects
//
// Ignores everything which isn't Storable, eg links etc
func (f *Fs) List() fs.ObjectsChan {
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
func (f *Fs) cleanUtf8(name string) string {
	if !utf8.ValidString(name) {
		f.wmu.Lock()
		if _, ok := f.warned[name]; !ok {
			fs.Debug(f, "Replacing invalid UTF-8 characters in %q", name)
			f.warned[name] = struct{}{}
		}
		f.wmu.Unlock()
		name = string([]rune(name))
	}
	if runtime.GOOS == "windows" {
		name = cleanWindowsName(f, name)
	}
	return name
}

// ListDir walks the path returning a channel of FsObjects
func (f *Fs) ListDir() fs.DirChan {
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
					dirpath := f.filterPath(filepath.Join(f.root, item.Name()))
					err := filepath.Walk(dirpath, func(path string, fi os.FileInfo, err error) error {
						if err != nil {
							fs.Stats.Error()
							fs.ErrorLog(f, "Failed to open directory: %s: %s", path, err)
						} else {
							dir.Count++
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

// Put the FsObject to the local filesystem
func (f *Fs) Put(in io.Reader, src fs.ObjectInfo) (fs.Object, error) {
	remote := src.Remote()
	// Temporary FsObject under construction - info filled in by Update()
	o := f.newFsObject(remote)
	err := o.Update(in, src)
	if err != nil {
		return nil, err
	}
	return o, nil
}

// Mkdir creates the directory if it doesn't exist
func (f *Fs) Mkdir() error {
	// FIXME: https://github.com/syncthing/syncthing/blob/master/lib/osutil/mkdirall_windows.go
	return os.MkdirAll(f.root, 0777)
}

// Rmdir removes the directory
//
// If it isn't empty it will return an error
func (f *Fs) Rmdir() error {
	return os.Remove(f.root)
}

// Precision of the file system
func (f *Fs) Precision() (precision time.Duration) {
	f.precisionOk.Do(func() {
		f.precision = f.readPrecision()
	})
	return f.precision
}

// Read the precision
func (f *Fs) readPrecision() (precision time.Duration) {
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
func (f *Fs) Purge() error {
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
func (f *Fs) Move(src fs.Object, remote string) (fs.Object, error) {
	srcObj, ok := src.(*Object)
	if !ok {
		fs.Debug(src, "Can't move - not same remote type")
		return nil, fs.ErrorCantMove
	}

	// Temporary FsObject under construction
	dstObj := f.newFsObject(remote)

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

// DirMove moves src directory to this remote using server side move
// operations.
//
// Will only be called if src.Fs().Name() == f.Name()
//
// If it isn't possible then return fs.ErrorCantDirMove
//
// If destination exists then return fs.ErrorDirExists
func (f *Fs) DirMove(src fs.Fs) error {
	srcFs, ok := src.(*Fs)
	if !ok {
		fs.Debug(srcFs, "Can't move directory - not same remote type")
		return fs.ErrorCantDirMove
	}
	// Check if source exists
	sstat, err := os.Lstat(srcFs.root)
	if err != nil {
		return err
	}
	// And is a directory
	if !sstat.IsDir() {
		return fs.ErrorCantDirMove
	}

	// Check if destination exists
	_, err = os.Lstat(f.root)
	if !os.IsNotExist(err) {
		return fs.ErrorDirExists
	}

	// Do the move
	return os.Rename(srcFs.root, f.root)
}

// Hashes returns the supported hash sets.
func (f *Fs) Hashes() fs.HashSet {
	return fs.SupportedHashes
}

// ------------------------------------------------------------

// Fs returns the parent Fs
func (o *Object) Fs() fs.Info {
	return o.fs
}

// Return a string version
func (o *Object) String() string {
	if o == nil {
		return "<nil>"
	}
	return o.remote
}

// Remote returns the remote path
func (o *Object) Remote() string {
	return o.fs.cleanUtf8(o.remote)
}

// Hash returns the requested hash of a file as a lowercase hex string
func (o *Object) Hash(r fs.HashType) (string, error) {
	// Check that the underlying file hasn't changed
	oldtime := o.info.ModTime()
	oldsize := o.info.Size()
	err := o.lstat()
	if err != nil {
		fs.Stats.Error()
		fs.ErrorLog(o, "Failed to stat: %s", err)
		return "", err
	}

	if !o.info.ModTime().Equal(oldtime) || oldsize != o.info.Size() {
		o.hashes = nil
	}

	if o.hashes == nil {
		o.hashes = make(map[fs.HashType]string)
		in, err := os.Open(o.path)
		if err != nil {
			fs.Stats.Error()
			fs.ErrorLog(o, "Failed to open: %s", err)
			return "", err
		}
		o.hashes, err = fs.HashStream(in)
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
	}
	return o.hashes[r], nil
}

// Size returns the size of an object in bytes
func (o *Object) Size() int64 {
	return o.info.Size()
}

// ModTime returns the modification time of the object
func (o *Object) ModTime() time.Time {
	return o.info.ModTime()
}

// SetModTime sets the modification time of the local fs object
func (o *Object) SetModTime(modTime time.Time) {
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

// Storable returns a boolean showing if this object is storable
func (o *Object) Storable() bool {
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
	o    *Object         // object that is open
	in   io.ReadCloser   // handle we are wrapping
	hash *fs.MultiHasher // currently accumulating hashes
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
		file.o.hashes = file.hash.Sums()
	} else {
		file.o.hashes = nil
	}
	return err
}

// Open an object for read
func (o *Object) Open() (in io.ReadCloser, err error) {
	in, err = os.Open(o.path)
	if err != nil {
		return
	}
	// Update the md5sum as we go along
	in = &localOpenFile{
		o:    o,
		in:   in,
		hash: fs.NewMultiHasher(),
	}
	return
}

// mkdirAll makes all the directories needed to store the object
func (o *Object) mkdirAll() error {
	dir, _ := getDirFile(o.path)
	return os.MkdirAll(dir, 0777)
}

// Update the object from in with modTime and size
func (o *Object) Update(in io.Reader, src fs.ObjectInfo) error {
	err := o.mkdirAll()
	if err != nil {
		return err
	}

	out, err := os.Create(o.path)
	if err != nil {
		return err
	}

	// Calculate the hash of the object we are reading as we go along
	hash := fs.NewMultiHasher()
	in = io.TeeReader(in, hash)

	_, err = io.Copy(out, in)
	outErr := out.Close()
	if err != nil {
		return err
	}
	if outErr != nil {
		return outErr
	}

	// All successful so update the hashes
	o.hashes = hash.Sums()

	// Set the mtime
	o.SetModTime(src.ModTime())

	// ReRead info now that we have finished
	return o.lstat()
}

// Stat a FsObject into info
func (o *Object) lstat() error {
	info, err := os.Lstat(o.path)
	o.info = info
	return err
}

// Remove an object
func (o *Object) Remove() error {
	return os.Remove(o.path)
}

// Return the current directory and file from a path
// Assumes os.PathSeparator is used.
func getDirFile(s string) (string, string) {
	i := strings.LastIndex(s, string(os.PathSeparator))
	return s[:i], s[i+1:]
}

func (f *Fs) filterPath(s string) string {
	s = filepath.Clean(s)
	if runtime.GOOS == "windows" {
		s = strings.Replace(s, `/`, `\`, -1)

		if !filepath.IsAbs(s) && !strings.HasPrefix(s, "\\") {
			s2, err := filepath.Abs(s)
			if err == nil {
				s = s2
			}
		}

		if f.nounc {
			return s
		}
		// Convert to UNC
		return uncPath(s)
	}

	if !filepath.IsAbs(s) {
		s2, err := filepath.Abs(s)
		if err == nil {
			s = s2
		}
	}

	return s
}

// Pattern to match a windows absolute path: "c:\" and similar
var isAbsWinDrive = regexp.MustCompile(`^[a-zA-Z]\:\\`)

// uncPath converts an absolute Windows path
// to a UNC long path.
func uncPath(s string) string {
	// UNC can NOT use "/", so convert all to "\"
	s = strings.Replace(s, `/`, `\`, -1)

	// If prefix is "\\", we already have a UNC path or server.
	if strings.HasPrefix(s, `\\`) {
		// If already long path, just keep it
		if strings.HasPrefix(s, `\\?\`) {
			return s
		}

		// Trim "\\" from path and add UNC prefix.
		return `\\?\UNC\` + strings.TrimPrefix(s, `\\`)
	}
	if isAbsWinDrive.MatchString(s) {
		return `\\?\` + s
	}
	return s
}

// cleanWindowsName will clean invalid Windows characters
func cleanWindowsName(f *Fs, name string) string {
	original := name
	var name2 string
	if strings.HasPrefix(name, `\\?\`) {
		name2 = `\\?\`
		name = strings.TrimPrefix(name, `\\?\`)
	}
	if strings.HasPrefix(name, `//?/`) {
		name2 = `//?/`
		name = strings.TrimPrefix(name, `//?/`)
	}
	// Colon is allowed as part of a drive name X:\
	colonAt := strings.Index(name, ":")
	if colonAt > 0 && colonAt < 3 && len(name) > colonAt+1 {
		// Copy to name2, which is unfiltered
		name2 += name[0 : colonAt+1]
		name = name[colonAt+1:]
	}

	name2 += strings.Map(func(r rune) rune {
		switch r {
		case '<', '>', '"', '|', '?', '*', ':':
			return '_'
		}
		return r
	}, name)

	if name2 != original && f != nil {
		f.wmu.Lock()
		if _, ok := f.warned[name]; !ok {
			fs.Debug(f, "Replacing invalid characters in %q to %q", name, name2)
			f.warned[name] = struct{}{}
		}
		f.wmu.Unlock()
	}
	return name2
}

// Check the interfaces are satisfied
var (
	_ fs.Fs       = &Fs{}
	_ fs.Purger   = &Fs{}
	_ fs.Mover    = &Fs{}
	_ fs.DirMover = &Fs{}
	_ fs.Object   = &Object{}
)
