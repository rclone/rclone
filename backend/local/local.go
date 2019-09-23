// Package local provides a filesystem interface
package local

import (
	"bytes"
	"context"
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

	"github.com/pkg/errors"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/accounting"
	"github.com/rclone/rclone/fs/config/configmap"
	"github.com/rclone/rclone/fs/config/configstruct"
	"github.com/rclone/rclone/fs/fserrors"
	"github.com/rclone/rclone/fs/hash"
	"github.com/rclone/rclone/lib/file"
	"github.com/rclone/rclone/lib/readers"
)

// Constants
const devUnset = 0xdeadbeefcafebabe                                       // a device id meaning it is unset
const linkSuffix = ".rclonelink"                                          // The suffix added to a translated symbolic link
const useReadDir = (runtime.GOOS == "windows" || runtime.GOOS == "plan9") // these OSes read FileInfos directly

// Register with Fs
func init() {
	fsi := &fs.RegInfo{
		Name:        "local",
		Description: "Local Disk",
		NewFs:       NewFs,
		Options: []fs.Option{{
			Name: "nounc",
			Help: "Disable UNC (long path names) conversion on Windows",
			Examples: []fs.OptionExample{{
				Value: "true",
				Help:  "Disables long file names",
			}},
		}, {
			Name:     "copy_links",
			Help:     "Follow symlinks and copy the pointed to item.",
			Default:  false,
			NoPrefix: true,
			ShortOpt: "L",
			Advanced: true,
		}, {
			Name:     "links",
			Help:     "Translate symlinks to/from regular files with a '" + linkSuffix + "' extension",
			Default:  false,
			NoPrefix: true,
			ShortOpt: "l",
			Advanced: true,
		}, {
			Name: "skip_links",
			Help: `Don't warn about skipped symlinks.
This flag disables warning messages on skipped symlinks or junction
points, as you explicitly acknowledge that they should be skipped.`,
			Default:  false,
			NoPrefix: true,
			Advanced: true,
		}, {
			Name: "no_unicode_normalization",
			Help: `Don't apply unicode normalization to paths and filenames (Deprecated)

This flag is deprecated now.  Rclone no longer normalizes unicode file
names, but it compares them with unicode normalization in the sync
routine instead.`,
			Default:  false,
			Advanced: true,
		}, {
			Name: "no_check_updated",
			Help: `Don't check to see if the files change during upload

Normally rclone checks the size and modification time of files as they
are being uploaded and aborts with a message which starts "can't copy
- source file is being updated" if the file changes during upload.

However on some file systems this modification time check may fail (eg
[Glusterfs #2206](https://github.com/rclone/rclone/issues/2206)) so this
check can be disabled with this flag.`,
			Default:  false,
			Advanced: true,
		}, {
			Name:     "one_file_system",
			Help:     "Don't cross filesystem boundaries (unix/macOS only).",
			Default:  false,
			NoPrefix: true,
			ShortOpt: "x",
			Advanced: true,
		}, {
			Name: "case_sensitive",
			Help: `Force the filesystem to report itself as case sensitive.

Normally the local backend declares itself as case insensitive on
Windows/macOS and case sensitive for everything else.  Use this flag
to override the default choice.`,
			Default:  false,
			Advanced: true,
		}, {
			Name: "case_insensitive",
			Help: `Force the filesystem to report itself as case insensitive

Normally the local backend declares itself as case insensitive on
Windows/macOS and case sensitive for everything else.  Use this flag
to override the default choice.`,
			Default:  false,
			Advanced: true,
		}},
	}
	fs.Register(fsi)
}

// Options defines the configuration for this backend
type Options struct {
	FollowSymlinks    bool `config:"copy_links"`
	TranslateSymlinks bool `config:"links"`
	SkipSymlinks      bool `config:"skip_links"`
	NoUTFNorm         bool `config:"no_unicode_normalization"`
	NoCheckUpdated    bool `config:"no_check_updated"`
	NoUNC             bool `config:"nounc"`
	OneFileSystem     bool `config:"one_file_system"`
	CaseSensitive     bool `config:"case_sensitive"`
	CaseInsensitive   bool `config:"case_insensitive"`
}

// Fs represents a local filesystem rooted at root
type Fs struct {
	name        string              // the name of the remote
	root        string              // The root directory (OS path)
	opt         Options             // parsed config options
	features    *fs.Features        // optional features
	dev         uint64              // device number of root node
	precisionOk sync.Once           // Whether we need to read the precision
	precision   time.Duration       // precision of local filesystem
	warnedMu    sync.Mutex          // used for locking access to 'warned'.
	warned      map[string]struct{} // whether we have warned about this string

	// do os.Lstat or os.Stat
	lstat          func(name string) (os.FileInfo, error)
	objectHashesMu sync.Mutex // global lock for Object.hashes
}

// Object represents a local filesystem object
type Object struct {
	fs             *Fs    // The Fs this object is part of
	remote         string // The remote path (encoded path)
	path           string // The local path (OS path)
	size           int64  // file metadata - always present
	mode           os.FileMode
	modTime        time.Time
	hashes         map[hash.Type]string // Hashes
	translatedLink bool                 // Is this object a translated link
}

// ------------------------------------------------------------

var errLinksAndCopyLinks = errors.New("can't use -l/--links with -L/--copy-links")

// NewFs constructs an Fs from the path
func NewFs(name, root string, m configmap.Mapper) (fs.Fs, error) {
	// Parse config into Options struct
	opt := new(Options)
	err := configstruct.Set(m, opt)
	if err != nil {
		return nil, err
	}
	if opt.TranslateSymlinks && opt.FollowSymlinks {
		return nil, errLinksAndCopyLinks
	}

	if opt.NoUTFNorm {
		fs.Errorf(nil, "The --local-no-unicode-normalization flag is deprecated and will be removed")
	}

	f := &Fs{
		name:   name,
		opt:    *opt,
		warned: make(map[string]struct{}),
		dev:    devUnset,
		lstat:  os.Lstat,
	}
	f.root = cleanRootPath(root, f.opt.NoUNC)
	f.features = (&fs.Features{
		CaseInsensitive:         f.caseInsensitive(),
		CanHaveEmptyDirectories: true,
		IsLocal:                 true,
	}).Fill(f)
	if opt.FollowSymlinks {
		f.lstat = os.Stat
	}

	// Check to see if this points to a file
	fi, err := f.lstat(f.root)
	if err == nil {
		f.dev = readDevice(fi, f.opt.OneFileSystem)
	}
	if err == nil && f.isRegular(fi.Mode()) {
		// It is a file, so use the parent as the root
		f.root = filepath.Dir(f.root)
		// return an error with an fs which points to the parent
		return f, fs.ErrorIsFile
	}
	return f, nil
}

// Determine whether a file is a 'regular' file,
// Symlinks are regular files, only if the TranslateSymlink
// option is in-effect
func (f *Fs) isRegular(mode os.FileMode) bool {
	if !f.opt.TranslateSymlinks {
		return mode.IsRegular()
	}

	// fi.Mode().IsRegular() tests that all mode bits are zero
	// Since symlinks are accepted, test that all other bits are zero,
	// except the symlink bit
	return mode&os.ModeType&^os.ModeSymlink == 0
}

// Name of the remote (as passed into NewFs)
func (f *Fs) Name() string {
	return f.name
}

// Root of the remote (as passed into NewFs)
func (f *Fs) Root() string {
	return enc.ToStandardPath(filepath.ToSlash(f.root))
}

// String converts this Fs to a string
func (f *Fs) String() string {
	return fmt.Sprintf("Local file system at %s", f.Root())
}

// Features returns the optional features of this Fs
func (f *Fs) Features() *fs.Features {
	return f.features
}

// caseInsensitive returns whether the remote is case insensitive or not
func (f *Fs) caseInsensitive() bool {
	if f.opt.CaseSensitive {
		return false
	}
	if f.opt.CaseInsensitive {
		return true
	}
	// FIXME not entirely accurate since you can have case
	// sensitive Fses on darwin and case insensitive Fses on linux.
	// Should probably check but that would involve creating a
	// file in the remote to be most accurate which probably isn't
	// desirable.
	return runtime.GOOS == "windows" || runtime.GOOS == "darwin"
}

// translateLink checks whether the remote is a translated link
// and returns a new path, removing the suffix as needed,
// It also returns whether this is a translated link at all
//
// for regular files, localPath is returned unchanged
func translateLink(remote, localPath string) (newLocalPath string, isTranslatedLink bool) {
	isTranslatedLink = strings.HasSuffix(remote, linkSuffix)
	newLocalPath = strings.TrimSuffix(localPath, linkSuffix)
	return newLocalPath, isTranslatedLink
}

// newObject makes a half completed Object
func (f *Fs) newObject(remote string) *Object {
	translatedLink := false
	localPath := f.localPath(remote)

	if f.opt.TranslateSymlinks {
		// Possibly receive a new name for localPath
		localPath, translatedLink = translateLink(remote, localPath)
	}

	return &Object{
		fs:             f,
		remote:         remote,
		path:           localPath,
		translatedLink: translatedLink,
	}
}

// Return an Object from a path
//
// May return nil if an error occurred
func (f *Fs) newObjectWithInfo(remote string, info os.FileInfo) (fs.Object, error) {
	o := f.newObject(remote)
	if info != nil {
		o.setMetadata(info)
	} else {
		err := o.lstat()
		if err != nil {
			if os.IsNotExist(err) {
				return nil, fs.ErrorObjectNotFound
			}
			if os.IsPermission(err) {
				return nil, fs.ErrorPermissionDenied
			}
			return nil, err
		}
		// Handle the odd case, that a symlink was specified by name without the link suffix
		if o.fs.opt.TranslateSymlinks && o.mode&os.ModeSymlink != 0 && !o.translatedLink {
			return nil, fs.ErrorObjectNotFound
		}

	}
	if o.mode.IsDir() {
		return nil, errors.Wrapf(fs.ErrorNotAFile, "%q", remote)
	}
	return o, nil
}

// NewObject finds the Object at remote.  If it can't be found
// it returns the error ErrorObjectNotFound.
func (f *Fs) NewObject(ctx context.Context, remote string) (fs.Object, error) {
	return f.newObjectWithInfo(remote, nil)
}

// List the objects and directories in dir into entries.  The
// entries can be returned in any order but should be for a
// complete directory.
//
// dir should be "" to list the root, and should not have
// trailing slashes.
//
// This should return ErrDirNotFound if the directory isn't
// found.
func (f *Fs) List(ctx context.Context, dir string) (entries fs.DirEntries, err error) {
	fsDirPath := f.localPath(dir)
	_, err = os.Stat(fsDirPath)
	if err != nil {
		return nil, fs.ErrorDirNotFound
	}

	fd, err := os.Open(fsDirPath)
	if err != nil {
		isPerm := os.IsPermission(err)
		err = errors.Wrapf(err, "failed to open directory %q", dir)
		fs.Errorf(dir, "%v", err)
		if isPerm {
			accounting.Stats(ctx).Error(fserrors.NoRetryError(err))
			err = nil // ignore error but fail sync
		}
		return nil, err
	}
	defer func() {
		cerr := fd.Close()
		if cerr != nil && err == nil {
			err = errors.Wrapf(cerr, "failed to close directory %q:", dir)
		}
	}()

	for {
		var fis []os.FileInfo
		if useReadDir {
			// Windows and Plan9 read the directory entries with the stat information in which
			// shouldn't fail because of unreadable entries.
			fis, err = fd.Readdir(1024)
			if err == io.EOF && len(fis) == 0 {
				break
			}
		} else {
			// For other OSes we read the names only (which shouldn't fail) then stat the
			// individual ourselves so we can log errors but not fail the directory read.
			var names []string
			names, err = fd.Readdirnames(1024)
			if err == io.EOF && len(names) == 0 {
				break
			}
			if err == nil {
				for _, name := range names {
					namepath := filepath.Join(fsDirPath, name)
					fi, fierr := os.Lstat(namepath)
					if fierr != nil {
						err = errors.Wrapf(err, "failed to read directory %q", namepath)
						fs.Errorf(dir, "%v", fierr)
						accounting.Stats(ctx).Error(fserrors.NoRetryError(fierr)) // fail the sync
						continue
					}
					fis = append(fis, fi)
				}
			}
		}
		if err != nil {
			return nil, errors.Wrap(err, "failed to read directory entry")
		}

		for _, fi := range fis {
			name := fi.Name()
			mode := fi.Mode()
			newRemote := f.cleanRemote(dir, name)
			// Follow symlinks if required
			if f.opt.FollowSymlinks && (mode&os.ModeSymlink) != 0 {
				localPath := filepath.Join(fsDirPath, name)
				fi, err = os.Stat(localPath)
				if os.IsNotExist(err) {
					// Skip bad symlinks
					err = fserrors.NoRetryError(errors.Wrap(err, "symlink"))
					fs.Errorf(newRemote, "Listing error: %v", err)
					accounting.Stats(ctx).Error(err)
					continue
				}
				if err != nil {
					return nil, err
				}
				mode = fi.Mode()
			}
			if fi.IsDir() {
				// Ignore directories which are symlinks.  These are junction points under windows which
				// are kind of a souped up symlink. Unix doesn't have directories which are symlinks.
				if (mode&os.ModeSymlink) == 0 && f.dev == readDevice(fi, f.opt.OneFileSystem) {
					d := fs.NewDir(newRemote, fi.ModTime())
					entries = append(entries, d)
				}
			} else {
				// Check whether this link should be translated
				if f.opt.TranslateSymlinks && fi.Mode()&os.ModeSymlink != 0 {
					newRemote += linkSuffix
				}
				fso, err := f.newObjectWithInfo(newRemote, fi)
				if err != nil {
					return nil, err
				}
				if fso.Storable() {
					entries = append(entries, fso)
				}
			}
		}
	}
	return entries, nil
}

func (f *Fs) cleanRemote(dir, filename string) (remote string) {
	remote = path.Join(dir, enc.ToStandardName(filename))

	if !utf8.ValidString(filename) {
		f.warnedMu.Lock()
		if _, ok := f.warned[remote]; !ok {
			fs.Logf(f, "Replacing invalid UTF-8 characters in %q", remote)
			f.warned[remote] = struct{}{}
		}
		f.warnedMu.Unlock()
	}
	return
}

func (f *Fs) localPath(name string) string {
	return filepath.Join(f.root, filepath.FromSlash(enc.FromStandardPath(name)))
}

// Put the Object to the local filesystem
func (f *Fs) Put(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error) {
	// Temporary Object under construction - info filled in by Update()
	o := f.newObject(src.Remote())
	err := o.Update(ctx, in, src, options...)
	if err != nil {
		return nil, err
	}
	return o, nil
}

// PutStream uploads to the remote path with the modTime given of indeterminate size
func (f *Fs) PutStream(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error) {
	return f.Put(ctx, in, src, options...)
}

// Mkdir creates the directory if it doesn't exist
func (f *Fs) Mkdir(ctx context.Context, dir string) error {
	// FIXME: https://github.com/syncthing/syncthing/blob/master/lib/osutil/mkdirall_windows.go
	localPath := f.localPath(dir)
	err := os.MkdirAll(localPath, 0777)
	if err != nil {
		return err
	}
	if dir == "" {
		fi, err := f.lstat(localPath)
		if err != nil {
			return err
		}
		f.dev = readDevice(fi, f.opt.OneFileSystem)
	}
	return nil
}

// Rmdir removes the directory
//
// If it isn't empty it will return an error
func (f *Fs) Rmdir(ctx context.Context, dir string) error {
	return os.Remove(f.localPath(dir))
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
		// fmt.Println("compare", fi.ModTime(ctx), t)
		if fi.ModTime().Equal(t) {
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
func (f *Fs) Purge(ctx context.Context) error {
	fi, err := f.lstat(f.root)
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
func (f *Fs) Move(ctx context.Context, src fs.Object, remote string) (fs.Object, error) {
	srcObj, ok := src.(*Object)
	if !ok {
		fs.Debugf(src, "Can't move - not same remote type")
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
	} else if !dstObj.fs.isRegular(dstObj.mode) {
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
	if os.IsNotExist(err) {
		// race condition, source was deleted in the meantime
		return nil, err
	} else if os.IsPermission(err) {
		// not enough rights to write to dst
		return nil, err
	} else if err != nil {
		// not quite clear, but probably trying to move a file across file system
		// boundaries. Copying might still work.
		fs.Debugf(src, "Can't move: %v: trying copy", err)
		return nil, fs.ErrorCantMove
	}

	// Update the info
	err = dstObj.lstat()
	if err != nil {
		return nil, err
	}

	return dstObj, nil
}

// DirMove moves src, srcRemote to this remote at dstRemote
// using server side move operations.
//
// Will only be called if src.Fs().Name() == f.Name()
//
// If it isn't possible then return fs.ErrorCantDirMove
//
// If destination exists then return fs.ErrorDirExists
func (f *Fs) DirMove(ctx context.Context, src fs.Fs, srcRemote, dstRemote string) error {
	srcFs, ok := src.(*Fs)
	if !ok {
		fs.Debugf(srcFs, "Can't move directory - not same remote type")
		return fs.ErrorCantDirMove
	}
	srcPath := srcFs.localPath(srcRemote)
	dstPath := f.localPath(dstRemote)

	// Check if destination exists
	_, err := os.Lstat(dstPath)
	if !os.IsNotExist(err) {
		return fs.ErrorDirExists
	}

	// Create parent of destination
	dstParentPath := filepath.Dir(dstPath)
	err = os.MkdirAll(dstParentPath, 0777)
	if err != nil {
		return err
	}

	// Do the move
	err = os.Rename(srcPath, dstPath)
	if os.IsNotExist(err) {
		// race condition, source was deleted in the meantime
		return err
	} else if os.IsPermission(err) {
		// not enough rights to write to dst
		return err
	} else if err != nil {
		// not quite clear, but probably trying to move directory across file system
		// boundaries. Copying might still work.
		fs.Debugf(src, "Can't move dir: %v: trying copy", err)
		return fs.ErrorCantDirMove
	}
	return nil
}

// Hashes returns the supported hash sets.
func (f *Fs) Hashes() hash.Set {
	return hash.Supported()
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
func (o *Object) Hash(ctx context.Context, r hash.Type) (string, error) {
	// Check that the underlying file hasn't changed
	oldtime := o.modTime
	oldsize := o.size
	err := o.lstat()
	if err != nil {
		return "", errors.Wrap(err, "hash: failed to stat")
	}

	o.fs.objectHashesMu.Lock()
	hashes := o.hashes
	hashValue, hashFound := o.hashes[r]
	o.fs.objectHashesMu.Unlock()

	if !o.modTime.Equal(oldtime) || oldsize != o.size || hashes == nil || !hashFound {
		var in io.ReadCloser

		if !o.translatedLink {
			var fd *os.File
			fd, err = file.Open(o.path)
			if fd != nil {
				in = newFadviseReadCloser(o, fd, 0, 0)
			}
		} else {
			in, err = o.openTranslatedLink(0, -1)
		}
		if err != nil {
			return "", errors.Wrap(err, "hash: failed to open")
		}
		hashes, err = hash.StreamTypes(in, hash.NewHashSet(r))
		closeErr := in.Close()
		if err != nil {
			return "", errors.Wrap(err, "hash: failed to read")
		}
		if closeErr != nil {
			return "", errors.Wrap(closeErr, "hash: failed to close")
		}
		hashValue = hashes[r]
		o.fs.objectHashesMu.Lock()
		if o.hashes == nil {
			o.hashes = hashes
		} else {
			o.hashes[r] = hashValue
		}
		o.fs.objectHashesMu.Unlock()
	}
	return hashValue, nil
}

// Size returns the size of an object in bytes
func (o *Object) Size() int64 {
	return o.size
}

// ModTime returns the modification time of the object
func (o *Object) ModTime(ctx context.Context) time.Time {
	return o.modTime
}

// SetModTime sets the modification time of the local fs object
func (o *Object) SetModTime(ctx context.Context, modTime time.Time) error {
	var err error
	if o.translatedLink {
		err = lChtimes(o.path, modTime, modTime)
	} else {
		err = os.Chtimes(o.path, modTime, modTime)
	}
	if err != nil {
		return err
	}
	// Re-read metadata
	return o.lstat()
}

// Storable returns a boolean showing if this object is storable
func (o *Object) Storable() bool {
	mode := o.mode
	if mode&os.ModeSymlink != 0 && !o.fs.opt.TranslateSymlinks {
		if !o.fs.opt.SkipSymlinks {
			fs.Logf(o, "Can't follow symlink without -L/--copy-links")
		}
		return false
	} else if mode&(os.ModeNamedPipe|os.ModeSocket|os.ModeDevice) != 0 {
		fs.Logf(o, "Can't transfer non file/directory")
		return false
	} else if mode&os.ModeDir != 0 {
		// fs.Debugf(o, "Skipping directory")
		return false
	}
	return true
}

// localOpenFile wraps an io.ReadCloser and updates the md5sum of the
// object that is read
type localOpenFile struct {
	o    *Object           // object that is open
	in   io.ReadCloser     // handle we are wrapping
	hash *hash.MultiHasher // currently accumulating hashes
	fd   *os.File          // file object reference
}

// Read bytes from the object - see io.Reader
func (file *localOpenFile) Read(p []byte) (n int, err error) {
	if !file.o.fs.opt.NoCheckUpdated {
		// Check if file has the same size and modTime
		fi, err := file.fd.Stat()
		if err != nil {
			return 0, errors.Wrap(err, "can't read status of source file while transferring")
		}
		if file.o.size != fi.Size() {
			return 0, errors.Errorf("can't copy - source file is being updated (size changed from %d to %d)", file.o.size, fi.Size())
		}
		if !file.o.modTime.Equal(fi.ModTime()) {
			return 0, errors.Errorf("can't copy - source file is being updated (mod time changed from %v to %v)", file.o.modTime, fi.ModTime())
		}
	}

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
			file.o.fs.objectHashesMu.Lock()
			file.o.hashes = file.hash.Sums()
			file.o.fs.objectHashesMu.Unlock()
		}
	}
	return err
}

// Returns a ReadCloser() object that contains the contents of a symbolic link
func (o *Object) openTranslatedLink(offset, limit int64) (lrc io.ReadCloser, err error) {
	// Read the link and return the destination  it as the contents of the object
	linkdst, err := os.Readlink(o.path)
	if err != nil {
		return nil, err
	}
	return readers.NewLimitedReadCloser(ioutil.NopCloser(strings.NewReader(linkdst[offset:])), limit), nil
}

// Open an object for read
func (o *Object) Open(ctx context.Context, options ...fs.OpenOption) (in io.ReadCloser, err error) {
	var offset, limit int64 = 0, -1
	var hasher *hash.MultiHasher
	for _, option := range options {
		switch x := option.(type) {
		case *fs.SeekOption:
			offset = x.Offset
		case *fs.RangeOption:
			offset, limit = x.Decode(o.size)
		case *fs.HashesOption:
			if x.Hashes.Count() > 0 {
				hasher, err = hash.NewMultiHasherTypes(x.Hashes)
				if err != nil {
					return nil, err
				}
			}
		default:
			if option.Mandatory() {
				fs.Logf(o, "Unsupported mandatory option: %v", option)
			}
		}
	}

	// Handle a translated link
	if o.translatedLink {
		return o.openTranslatedLink(offset, limit)
	}

	fd, err := file.Open(o.path)
	if err != nil {
		return
	}
	wrappedFd := readers.NewLimitedReadCloser(newFadviseReadCloser(o, fd, offset, limit), limit)
	if offset != 0 {
		// seek the object
		_, err = fd.Seek(offset, io.SeekStart)
		// don't attempt to make checksums
		return wrappedFd, err
	}
	if hasher == nil {
		// no need to wrap since we don't need checksums
		return wrappedFd, nil
	}
	// Update the hashes as we go along
	in = &localOpenFile{
		o:    o,
		in:   wrappedFd,
		hash: hasher,
		fd:   fd,
	}
	return in, nil
}

// mkdirAll makes all the directories needed to store the object
func (o *Object) mkdirAll() error {
	dir := filepath.Dir(o.path)
	return os.MkdirAll(dir, 0777)
}

type nopWriterCloser struct {
	*bytes.Buffer
}

func (nwc nopWriterCloser) Close() error {
	// noop
	return nil
}

// Update the object from in with modTime and size
func (o *Object) Update(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (err error) {
	var out io.WriteCloser
	var hasher *hash.MultiHasher

	for _, option := range options {
		switch x := option.(type) {
		case *fs.HashesOption:
			if x.Hashes.Count() > 0 {
				hasher, err = hash.NewMultiHasherTypes(x.Hashes)
				if err != nil {
					return err
				}
			}
		}
	}

	err = o.mkdirAll()
	if err != nil {
		return err
	}

	var symlinkData bytes.Buffer
	// If the object is a regular file, create it.
	// If it is a translated link, just read in the contents, and
	// then create a symlink
	if !o.translatedLink {
		f, err := file.OpenFile(o.path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0666)
		if err != nil {
			return err
		}
		// Pre-allocate the file for performance reasons
		err = preAllocate(src.Size(), f)
		if err != nil {
			fs.Debugf(o, "Failed to pre-allocate: %v", err)
		}
		out = f
	} else {
		out = nopWriterCloser{&symlinkData}
	}

	// Calculate the hash of the object we are reading as we go along
	if hasher != nil {
		in = io.TeeReader(in, hasher)
	}

	_, err = io.Copy(out, in)
	closeErr := out.Close()
	if err == nil {
		err = closeErr
	}

	if o.translatedLink {
		if err == nil {
			// Remove any current symlink or file, if one exists
			if _, err := os.Lstat(o.path); err == nil {
				if removeErr := os.Remove(o.path); removeErr != nil {
					fs.Errorf(o, "Failed to remove previous file: %v", removeErr)
					return removeErr
				}
			}
			// Use the contents for the copied object to create a symlink
			err = os.Symlink(symlinkData.String(), o.path)
		}

		// only continue if symlink creation succeeded
		if err != nil {
			return err
		}
	}

	if err != nil {
		fs.Logf(o, "Removing partially written file on error: %v", err)
		if removeErr := os.Remove(o.path); removeErr != nil {
			fs.Errorf(o, "Failed to remove partially written file: %v", removeErr)
		}
		return err
	}

	// All successful so update the hashes
	if hasher != nil {
		o.fs.objectHashesMu.Lock()
		o.hashes = hasher.Sums()
		o.fs.objectHashesMu.Unlock()
	}

	// Set the mtime
	err = o.SetModTime(ctx, src.ModTime(ctx))
	if err != nil {
		return err
	}

	// ReRead info now that we have finished
	return o.lstat()
}

// OpenWriterAt opens with a handle for random access writes
//
// Pass in the remote desired and the size if known.
//
// It truncates any existing object
func (f *Fs) OpenWriterAt(ctx context.Context, remote string, size int64) (fs.WriterAtCloser, error) {
	// Temporary Object under construction
	o := f.newObject(remote)

	err := o.mkdirAll()
	if err != nil {
		return nil, err
	}

	if o.translatedLink {
		return nil, errors.New("can't open a symlink for random writing")
	}

	out, err := file.OpenFile(o.path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0666)
	if err != nil {
		return nil, err
	}
	// Pre-allocate the file for performance reasons
	err = preAllocate(size, out)
	if err != nil {
		fs.Debugf(o, "Failed to pre-allocate: %v", err)
	}
	return out, nil
}

// setMetadata sets the file info from the os.FileInfo passed in
func (o *Object) setMetadata(info os.FileInfo) {
	// Don't overwrite the info if we don't need to
	// this avoids upsetting the race detector
	if o.size != info.Size() {
		o.size = info.Size()
	}
	if !o.modTime.Equal(info.ModTime()) {
		o.modTime = info.ModTime()
	}
	if o.mode != info.Mode() {
		o.mode = info.Mode()
	}
}

// Stat a Object into info
func (o *Object) lstat() error {
	info, err := o.fs.lstat(o.path)
	if err == nil {
		o.setMetadata(info)
	}
	return err
}

// Remove an object
func (o *Object) Remove(ctx context.Context) error {
	return remove(o.path)
}

func cleanRootPath(s string, noUNC bool) string {
	if runtime.GOOS == "windows" {
		s = filepath.ToSlash(s)
		vol := filepath.VolumeName(s)
		s = vol + enc.FromStandardPath(s[len(vol):])
		s = filepath.FromSlash(s)

		if !filepath.IsAbs(s) && !strings.HasPrefix(s, "\\") {
			s2, err := filepath.Abs(s)
			if err == nil {
				s = s2
			}
		}
		if !noUNC {
			// Convert to UNC
			s = uncPath(s)
		}
		return s
	}
	if !filepath.IsAbs(s) {
		s2, err := filepath.Abs(s)
		if err == nil {
			s = s2
		}
	}
	s = enc.FromStandardPath(s)
	return s
}

// Pattern to match a windows absolute path: "c:\" and similar
var isAbsWinDrive = regexp.MustCompile(`^[a-zA-Z]\:\\`)

// uncPath converts an absolute Windows path
// to a UNC long path.
func uncPath(l string) string {
	// If prefix is "\\", we already have a UNC path or server.
	if strings.HasPrefix(l, `\\`) {
		// If already long path, just keep it
		if strings.HasPrefix(l, `\\?\`) {
			return l
		}

		// Trim "\\" from path and add UNC prefix.
		return `\\?\UNC\` + strings.TrimPrefix(l, `\\`)
	}
	if isAbsWinDrive.MatchString(l) {
		return `\\?\` + l
	}
	return l
}

// Check the interfaces are satisfied
var (
	_ fs.Fs             = &Fs{}
	_ fs.Purger         = &Fs{}
	_ fs.PutStreamer    = &Fs{}
	_ fs.Mover          = &Fs{}
	_ fs.DirMover       = &Fs{}
	_ fs.OpenWriterAter = &Fs{}
	_ fs.Object         = &Object{}
)
