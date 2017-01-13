// Package local provides a filesystem interface
package local

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"golang.org/x/text/unicode/norm"

	"github.com/ncw/rclone/fs"
	"github.com/pkg/errors"
)

// Constants
const devUnset = 0xdeadbeefcafebabe // a device id meaning it is unset

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
	root        string              // The root directory (OS path)
	features    *fs.Features        // optional features
	dev         uint64              // device number of root node
	precisionOk sync.Once           // Whether we need to read the precision
	precision   time.Duration       // precision of local filesystem
	wmu         sync.Mutex          // used for locking access to 'warned'.
	warned      map[string]struct{} // whether we have warned about this string
	nounc       bool                // Skip UNC conversion on Windows
}

// Object represents a local filesystem object
type Object struct {
	fs     *Fs                    // The Fs this object is part of
	remote string                 // The remote path - properly UTF-8 encoded - for rclone
	path   string                 // The local path - may not be properly UTF-8 encoded - for OS
	info   os.FileInfo            // Interface for file info (always present)
	hashes map[fs.HashType]string // Hashes
}

// ------------------------------------------------------------

// NewFs constructs an Fs from the path
func NewFs(name, root string) (fs.Fs, error) {
	var err error

	nounc := fs.ConfigFileGet(name, "nounc")
	f := &Fs{
		name:   name,
		warned: make(map[string]struct{}),
		nounc:  nounc == "true",
		dev:    devUnset,
	}
	f.root = f.cleanPath(root)
	f.features = (&fs.Features{CaseInsensitive: f.caseInsensitive()}).Fill(f)

	// Check to see if this points to a file
	fi, err := os.Lstat(f.root)
	if err == nil {
		f.dev = readDevice(fi)
	}
	if err == nil && fi.Mode().IsRegular() {
		// It is a file, so use the parent as the root
		f.root, _ = getDirFile(f.root)
		// return an error with an fs which points to the parent
		return f, fs.ErrorIsFile
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

// Features returns the optional features of this Fs
func (f *Fs) Features() *fs.Features {
	return f.features
}

// caseInsenstive returns whether the remote is case insensitive or not
func (f *Fs) caseInsensitive() bool {
	// FIXME not entirely accurate since you can have case
	// sensitive Fses on darwin and case insenstive Fses on linux.
	// Should probably check but that would involve creating a
	// file in the remote to be most accurate which probably isn't
	// desirable.
	return runtime.GOOS == "windows" || runtime.GOOS == "darwin"
}

// newObject makes a half completed Object
func (f *Fs) newObject(remote string) *Object {
	dstPath := f.cleanPath(filepath.Join(f.root, remote))
	remote = f.cleanRemote(remote)
	return &Object{
		fs:     f,
		remote: remote,
		path:   dstPath,
	}
}

// Return an Object from a path
//
// May return nil if an error occurred
func (f *Fs) newObjectWithInfo(remote string, info os.FileInfo) (fs.Object, error) {
	o := f.newObject(remote)
	if info != nil {
		o.info = info
	} else {
		err := o.lstat()
		if err != nil {
			if os.IsNotExist(err) {
				return nil, fs.ErrorObjectNotFound
			}
			return nil, err
		}
	}
	return o, nil
}

// NewObject finds the Object at remote.  If it can't be found
// it returns the error ErrorObjectNotFound.
func (f *Fs) NewObject(remote string) (fs.Object, error) {
	return f.newObjectWithInfo(remote, nil)
}

// listArgs is the arguments that a new list takes
type listArgs struct {
	remote  string
	dirpath string
	level   int
}

// list traverses the directory passed in, listing to out.
// it returns a boolean whether it is finished or not.
func (f *Fs) list(out fs.ListOpts, remote string, dirpath string, level int) (subdirs []listArgs) {
	fd, err := os.Open(dirpath)
	if err != nil {
		out.SetError(errors.Wrapf(err, "failed to open directory %q", dirpath))
		return nil
	}

	defer func() {
		err := fd.Close()
		if err != nil {
			out.SetError(errors.Wrapf(err, "failed to close directory %q:", dirpath))
		}
	}()

	for {
		fis, err := fd.Readdir(1024)
		if err == io.EOF && len(fis) == 0 {
			break
		}
		if err != nil {
			out.SetError(errors.Wrapf(err, "failed to read directory %q", dirpath))

			return nil
		}

		for _, fi := range fis {
			name := fi.Name()
			newRemote := path.Join(remote, name)
			newPath := filepath.Join(dirpath, name)
			if fi.IsDir() {
				// Ignore directories which are symlinks.  These are junction points under windows which
				// are kind of a souped up symlink. Unix doesn't have directories which are symlinks.
				if (fi.Mode()&os.ModeSymlink) == 0 && out.IncludeDirectory(newRemote) {
					dir := &fs.Dir{
						Name:  f.cleanRemote(newRemote),
						When:  fi.ModTime(),
						Bytes: 0,
						Count: 0,
					}
					if out.AddDir(dir) {
						return nil
					}
					if level > 0 && f.dev == readDevice(fi) {
						subdirs = append(subdirs, listArgs{remote: newRemote, dirpath: newPath, level: level - 1})
					}
				}
			} else {
				fso, err := f.newObjectWithInfo(newRemote, fi)
				if err != nil {
					out.SetError(err)
					return nil
				}
				if fso.Storable() && out.Add(fso) {
					return nil
				}
			}
		}
	}
	return subdirs
}

// List the path into out
//
// Ignores everything which isn't Storable, eg links etc
func (f *Fs) List(out fs.ListOpts, dir string) {
	defer out.Finished()
	root := f.cleanPath(filepath.Join(f.root, dir))
	dir = f.cleanRemote(dir)
	_, err := os.Stat(root)
	if err != nil {
		out.SetError(fs.ErrorDirNotFound)
		return
	}

	in := make(chan listArgs, out.Buffer())
	var wg sync.WaitGroup         // sync closing of go routines
	var traversing sync.WaitGroup // running directory traversals

	// Start the process
	traversing.Add(1)
	in <- listArgs{remote: dir, dirpath: root, level: out.Level() - 1}
	for i := 0; i < fs.Config.Checkers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for job := range in {
				if out.IsFinished() {
					continue
				}
				newJobs := f.list(out, job.remote, job.dirpath, job.level)
				// Now we have traversed this directory, send
				// these ones off for traversal
				if len(newJobs) != 0 {
					traversing.Add(len(newJobs))
					go func() {
						for _, newJob := range newJobs {
							in <- newJob
						}
					}()
				}
				traversing.Done()
			}
		}()
	}

	// Wait for traversal to finish
	traversing.Wait()
	close(in)
	wg.Wait()
}

// cleanRemote makes string a valid UTF-8 string for remote strings.
//
// Any invalid UTF-8 characters will be replaced with utf8.RuneError
// It also normalises the UTF-8 and converts the slashes if necessary.
func (f *Fs) cleanRemote(name string) string {
	if !utf8.ValidString(name) {
		f.wmu.Lock()
		if _, ok := f.warned[name]; !ok {
			fs.Debug(f, "Replacing invalid UTF-8 characters in %q", name)
			f.warned[name] = struct{}{}
		}
		f.wmu.Unlock()
		name = string([]rune(name))
	}
	name = norm.NFC.String(name)
	name = filepath.ToSlash(name)
	return name
}

// Put the Object to the local filesystem
func (f *Fs) Put(in io.Reader, src fs.ObjectInfo) (fs.Object, error) {
	remote := src.Remote()
	// Temporary Object under construction - info filled in by Update()
	o := f.newObject(remote)
	err := o.Update(in, src)
	if err != nil {
		return nil, err
	}
	return o, nil
}

// Mkdir creates the directory if it doesn't exist
func (f *Fs) Mkdir(dir string) error {
	// FIXME: https://github.com/syncthing/syncthing/blob/master/lib/osutil/mkdirall_windows.go
	root := f.cleanPath(filepath.Join(f.root, dir))
	err := os.MkdirAll(root, 0777)
	if err != nil {
		return err
	}
	if dir == "" {
		fi, err := os.Lstat(root)
		if err != nil {
			return err
		}
		f.dev = readDevice(fi)
	}
	return nil
}

// Rmdir removes the directory
//
// If it isn't empty it will return an error
func (f *Fs) Rmdir(dir string) error {
	root := f.cleanPath(filepath.Join(f.root, dir))
	return os.Remove(root)
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
		return errors.Errorf("can't purge non directory: %q", f.root)
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

	// Temporary Object under construction
	dstObj := f.newObject(remote)

	// Check it is a file if it exists
	err := dstObj.lstat()
	if os.IsNotExist(err) {
		// OK
	} else if err != nil {
		return nil, err
	} else if !dstObj.info.Mode().IsRegular() {
		// It isn't a file
		return nil, errors.New("can't move file onto non-file")
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
	return o.remote
}

// Hash returns the requested hash of a file as a lowercase hex string
func (o *Object) Hash(r fs.HashType) (string, error) {
	// Check that the underlying file hasn't changed
	oldtime := o.info.ModTime()
	oldsize := o.info.Size()
	err := o.lstat()
	if err != nil {
		return "", errors.Wrap(err, "hash: failed to stat")
	}

	if !o.info.ModTime().Equal(oldtime) || oldsize != o.info.Size() {
		o.hashes = nil
	}

	if o.hashes == nil {
		o.hashes = make(map[fs.HashType]string)
		in, err := os.Open(o.path)
		if err != nil {
			return "", errors.Wrap(err, "hash: failed to open")
		}
		o.hashes, err = fs.HashStream(in)
		closeErr := in.Close()
		if err != nil {
			return "", errors.Wrap(err, "hash: failed to read")
		}
		if closeErr != nil {
			return "", errors.Wrap(closeErr, "hash: failed to close")
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
func (o *Object) SetModTime(modTime time.Time) error {
	err := os.Chtimes(o.path, modTime, modTime)
	if err != nil {
		return err
	}
	// Re-read metadata
	return o.lstat()
}

// Storable returns a boolean showing if this object is storable
func (o *Object) Storable() bool {
	// Check for control characters in the remote name and show non storable
	for _, c := range o.Remote() {
		if c >= 0x00 && c < 0x20 || c == 0x7F {
			fs.Debug(o.fs, "Can't store file with control characters: %q", o.Remote())
			return false
		}
	}
	mode := o.info.Mode()
	// On windows a file with os.ModeSymlink represents a file with reparse points
	if runtime.GOOS == "windows" && (mode&os.ModeSymlink) != 0 {
		fs.Debug(o, "Clearing symlink bit to allow a file with reparse points to be copied")
		mode &^= os.ModeSymlink
	}
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

// Close the object and update the hashes
func (file *localOpenFile) Close() (err error) {
	err = file.in.Close()
	if err == nil {
		if file.hash.Size() == file.o.Size() {
			file.o.hashes = file.hash.Sums()
		}
	}
	return err
}

// Open an object for read
func (o *Object) Open(options ...fs.OpenOption) (in io.ReadCloser, err error) {
	var offset int64
	for _, option := range options {
		switch x := option.(type) {
		case *fs.SeekOption:
			offset = x.Offset
		default:
			if option.Mandatory() {
				fs.Log(o, "Unsupported mandatory option: %v", option)
			}
		}
	}

	fd, err := os.Open(o.path)
	if err != nil {
		return
	}
	if offset != 0 {
		// seek the object
		_, err = fd.Seek(offset, 0)
		// don't attempt to make checksums
		return fd, err
	}
	// Update the md5sum as we go along
	in = &localOpenFile{
		o:    o,
		in:   fd,
		hash: fs.NewMultiHasher(),
	}
	return in, nil
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
	closeErr := out.Close()
	if err == nil {
		err = closeErr
	}
	if err != nil {
		fs.Debug(o, "Removing partially written file on error: %v", err)
		if removeErr := os.Remove(o.path); removeErr != nil {
			fs.ErrorLog(o, "Failed to remove partially written file: %v", removeErr)
		}
		return err
	}

	// All successful so update the hashes
	o.hashes = hash.Sums()

	// Set the mtime
	err = o.SetModTime(src.ModTime())
	if err != nil {
		return err
	}

	// ReRead info now that we have finished
	return o.lstat()
}

// Stat a Object into info
func (o *Object) lstat() error {
	info, err := os.Lstat(o.path)
	o.info = info
	return err
}

// Remove an object
func (o *Object) Remove() error {
	return os.Remove(o.path)
}

// Return the directory and file from an OS path. Assumes
// os.PathSeparator is used.
func getDirFile(s string) (string, string) {
	i := strings.LastIndex(s, string(os.PathSeparator))
	dir, file := s[:i], s[i+1:]
	if dir == "" {
		dir = string(os.PathSeparator)
	}
	return dir, file
}

// cleanPathFragment cleans an OS path fragment which is part of a
// bigger path and not necessarily absolute
func cleanPathFragment(s string) string {
	if s == "" {
		return s
	}
	s = filepath.Clean(s)
	if runtime.GOOS == "windows" {
		s = strings.Replace(s, `/`, `\`, -1)
	}
	return s
}

// cleanPath cleans and makes absolute the path passed in and returns
// an OS path.
//
// The input might be in OS form or rclone form or a mixture, but the
// output is in OS form.
//
// On windows it makes the path UNC also and replaces any characters
// Windows can't deal with with their replacements.
func (f *Fs) cleanPath(s string) string {
	s = cleanPathFragment(s)
	if runtime.GOOS == "windows" {
		if !filepath.IsAbs(s) && !strings.HasPrefix(s, "\\") {
			s2, err := filepath.Abs(s)
			if err == nil {
				s = s2
			}
		}
		if !f.nounc {
			// Convert to UNC
			s = uncPath(s)
		}
		s = cleanWindowsName(f, s)
	} else {
		if !filepath.IsAbs(s) {
			s2, err := filepath.Abs(s)
			if err == nil {
				s = s2
			}
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

// cleanWindowsName will clean invalid Windows characters replacing them with _
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
