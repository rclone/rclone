// Package local provides a filesystem interface
package local

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unicode/utf8"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/accounting"
	"github.com/rclone/rclone/fs/config"
	"github.com/rclone/rclone/fs/config/configmap"
	"github.com/rclone/rclone/fs/config/configstruct"
	"github.com/rclone/rclone/fs/filter"
	"github.com/rclone/rclone/fs/fserrors"
	"github.com/rclone/rclone/fs/hash"
	"github.com/rclone/rclone/lib/encoder"
	"github.com/rclone/rclone/lib/file"
	"github.com/rclone/rclone/lib/readers"
	"golang.org/x/text/unicode/norm"
)

// Constants
const (
	devUnset   = 0xdeadbeefcafebabe                                     // a device id meaning it is unset
	linkSuffix = ".rclonelink"                                          // The suffix added to a translated symbolic link
	useReadDir = (runtime.GOOS == "windows" || runtime.GOOS == "plan9") // these OSes read FileInfos directly
)

// timeType allows the user to choose what exactly ModTime() returns
type timeType = fs.Enum[timeTypeChoices]

const (
	mTime timeType = iota
	aTime
	bTime
	cTime
)

type timeTypeChoices struct{}

func (timeTypeChoices) Choices() []string {
	return []string{
		mTime: "mtime",
		aTime: "atime",
		bTime: "btime",
		cTime: "ctime",
	}
}

// Register with Fs
func init() {
	fsi := &fs.RegInfo{
		Name:        "local",
		Description: "Local Disk",
		NewFs:       NewFs,
		CommandHelp: commandHelp,
		MetadataInfo: &fs.MetadataInfo{
			System: systemMetadataInfo,
			Help: `Depending on which OS is in use the local backend may return only some
of the system metadata. Setting system metadata is supported on all
OSes but setting user metadata is only supported on linux, freebsd,
netbsd, macOS and Solaris. It is **not** supported on Windows yet
([see pkg/attrs#47](https://github.com/pkg/xattr/issues/47)).

User metadata is stored as extended attributes (which may not be
supported by all file systems) under the "user.*" prefix.

Metadata is supported on files and directories.
`,
		},
		Options: []fs.Option{
			{
				Name:     "nounc",
				Help:     "Disable UNC (long path names) conversion on Windows.",
				Default:  false,
				Advanced: runtime.GOOS != "windows",
				Examples: []fs.OptionExample{{
					Value: "true",
					Help:  "Disables long file names.",
				}},
			},
			{
				Name:     "copy_links",
				Help:     "Follow symlinks and copy the pointed to item.",
				Default:  false,
				NoPrefix: true,
				ShortOpt: "L",
				Advanced: true,
			},
			{
				Name:     "links",
				Help:     "Translate symlinks to/from regular files with a '" + linkSuffix + "' extension.",
				Default:  false,
				NoPrefix: true,
				ShortOpt: "l",
				Advanced: true,
			},
			{
				Name: "skip_links",
				Help: `Don't warn about skipped symlinks.

This flag disables warning messages on skipped symlinks or junction
points, as you explicitly acknowledge that they should be skipped.`,
				Default:  false,
				NoPrefix: true,
				Advanced: true,
			},
			{
				Name: "zero_size_links",
				Help: `Assume the Stat size of links is zero (and read them instead) (deprecated).

Rclone used to use the Stat size of links as the link size, but this fails in quite a few places:

- Windows
- On some virtual filesystems (such ash LucidLink)
- Android

So rclone now always reads the link.
`,
				Default:  false,
				Advanced: true,
			},
			{
				Name: "unicode_normalization",
				Help: `Apply unicode NFC normalization to paths and filenames.

This flag can be used to normalize file names into unicode NFC form
that are read from the local filesystem.

Rclone does not normally touch the encoding of file names it reads from
the file system.

This can be useful when using macOS as it normally provides decomposed (NFD)
unicode which in some language (eg Korean) doesn't display properly on
some OSes.

Note that rclone compares filenames with unicode normalization in the sync
routine so this flag shouldn't normally be used.`,
				Default:  false,
				Advanced: true,
			},
			{
				Name: "no_check_updated",
				Help: `Don't check to see if the files change during upload.

Normally rclone checks the size and modification time of files as they
are being uploaded and aborts with a message which starts "can't copy -
source file is being updated" if the file changes during upload.

However on some file systems this modification time check may fail (e.g.
[Glusterfs #2206](https://github.com/rclone/rclone/issues/2206)) so this
check can be disabled with this flag.

If this flag is set, rclone will use its best efforts to transfer a
file which is being updated. If the file is only having things
appended to it (e.g. a log) then rclone will transfer the log file with
the size it had the first time rclone saw it.

If the file is being modified throughout (not just appended to) then
the transfer may fail with a hash check failure.

In detail, once the file has had stat() called on it for the first
time we:

- Only transfer the size that stat gave
- Only checksum the size that stat gave
- Don't update the stat info for the file

**NB** do not use this flag on a Windows Volume Shadow (VSS). For some
unknown reason, files in a VSS sometimes show different sizes from the
directory listing (where the initial stat value comes from on Windows)
and when stat is called on them directly. Other copy tools always use
the direct stat value and setting this flag will disable that.
`,
				Default:  false,
				Advanced: true,
			},
			{
				Name:     "one_file_system",
				Help:     "Don't cross filesystem boundaries (unix/macOS only).",
				Default:  false,
				NoPrefix: true,
				ShortOpt: "x",
				Advanced: true,
			},
			{
				Name: "case_sensitive",
				Help: `Force the filesystem to report itself as case sensitive.

Normally the local backend declares itself as case insensitive on
Windows/macOS and case sensitive for everything else.  Use this flag
to override the default choice.`,
				Default:  false,
				Advanced: true,
			},
			{
				Name: "case_insensitive",
				Help: `Force the filesystem to report itself as case insensitive.

Normally the local backend declares itself as case insensitive on
Windows/macOS and case sensitive for everything else.  Use this flag
to override the default choice.`,
				Default:  false,
				Advanced: true,
			},
			{
				Name: "no_clone",
				Help: `Disable reflink cloning for server-side copies.

Normally, for local-to-local transfers, rclone will "clone" the file when
possible, and fall back to "copying" only when cloning is not supported.

Cloning creates a shallow copy (or "reflink") which initially shares blocks with
the original file. Unlike a "hardlink", the two files are independent and
neither will affect the other if subsequently modified.

Cloning is usually preferable to copying, as it is much faster and is
deduplicated by default (i.e. having two identical files does not consume more
storage than having just one.)  However, for use cases where data redundancy is
preferable, --local-no-clone can be used to disable cloning and force "deep" copies.

Currently, cloning is only supported when using APFS on macOS (support for other
platforms may be added in the future.)`,
				Default:  false,
				Advanced: true,
			},
			{
				Name: "no_preallocate",
				Help: `Disable preallocation of disk space for transferred files.

Preallocation of disk space helps prevent filesystem fragmentation.
However, some virtual filesystem layers (such as Google Drive File
Stream) may incorrectly set the actual file size equal to the
preallocated space, causing checksum and file size checks to fail.
Use this flag to disable preallocation.`,
				Default:  false,
				Advanced: true,
			},
			{
				Name: "no_sparse",
				Help: `Disable sparse files for multi-thread downloads.

On Windows platforms rclone will make sparse files when doing
multi-thread downloads. This avoids long pauses on large files where
the OS zeros the file. However sparse files may be undesirable as they
cause disk fragmentation and can be slow to work with.`,
				Default:  false,
				Advanced: true,
			},
			{
				Name: "no_set_modtime",
				Help: `Disable setting modtime.

Normally rclone updates modification time of files after they are done
uploading. This can cause permissions issues on Linux platforms when 
the user rclone is running as does not own the file uploaded, such as
when copying to a CIFS mount owned by another user. If this option is 
enabled, rclone will no longer update the modtime after copying a file.`,
				Default:  false,
				Advanced: true,
			},
			{
				Name: "time_type",
				Help: `Set what kind of time is returned.

Normally rclone does all operations on the mtime or Modification time.

If you set this flag then rclone will return the Modified time as whatever
you set here. So if you use "rclone lsl --local-time-type ctime" then
you will see ctimes in the listing.

If the OS doesn't support returning the time_type specified then rclone
will silently replace it with the modification time which all OSes support.

- mtime is supported by all OSes
- atime is supported on all OSes except: plan9, js
- btime is only supported on: Windows, macOS, freebsd, netbsd
- ctime is supported on all Oses except: Windows, plan9, js

Note that setting the time will still set the modified time so this is
only useful for reading.
`,
				Default:  mTime,
				Advanced: true,
				Examples: []fs.OptionExample{{
					Value: mTime.String(),
					Help:  "The last modification time.",
				}, {
					Value: aTime.String(),
					Help:  "The last access time.",
				}, {
					Value: bTime.String(),
					Help:  "The creation time.",
				}, {
					Value: cTime.String(),
					Help:  "The last status change time.",
				}},
			},
			{
				Name:     config.ConfigEncoding,
				Help:     config.ConfigEncodingHelp,
				Advanced: true,
				Default:  encoder.OS,
			},
		},
	}
	fs.Register(fsi)
}

// Options defines the configuration for this backend
type Options struct {
	FollowSymlinks    bool                 `config:"copy_links"`
	TranslateSymlinks bool                 `config:"links"`
	SkipSymlinks      bool                 `config:"skip_links"`
	UTFNorm           bool                 `config:"unicode_normalization"`
	NoCheckUpdated    bool                 `config:"no_check_updated"`
	NoUNC             bool                 `config:"nounc"`
	OneFileSystem     bool                 `config:"one_file_system"`
	CaseSensitive     bool                 `config:"case_sensitive"`
	CaseInsensitive   bool                 `config:"case_insensitive"`
	NoPreAllocate     bool                 `config:"no_preallocate"`
	NoSparse          bool                 `config:"no_sparse"`
	NoSetModTime      bool                 `config:"no_set_modtime"`
	TimeType          timeType             `config:"time_type"`
	Enc               encoder.MultiEncoder `config:"encoding"`
	NoClone           bool                 `config:"no_clone"`
}

// Fs represents a local filesystem rooted at root
type Fs struct {
	name           string              // the name of the remote
	root           string              // The root directory (OS path)
	opt            Options             // parsed config options
	features       *fs.Features        // optional features
	dev            uint64              // device number of root node
	precisionOk    sync.Once           // Whether we need to read the precision
	precision      time.Duration       // precision of local filesystem
	warnedMu       sync.Mutex          // used for locking access to 'warned'.
	warned         map[string]struct{} // whether we have warned about this string
	xattrSupported atomic.Int32        // whether xattrs are supported

	// do os.Lstat or os.Stat
	lstat        func(name string) (os.FileInfo, error)
	objectMetaMu sync.RWMutex // global lock for Object metadata
}

// Object represents a local filesystem object
type Object struct {
	fs     *Fs    // The Fs this object is part of
	remote string // The remote path (encoded path)
	path   string // The local path (OS path)
	// When using these items the fs.objectMetaMu must be held
	size    int64 // file metadata - always present
	mode    os.FileMode
	modTime time.Time
	hashes  map[hash.Type]string // Hashes
	// these are read only and don't need the mutex held
	translatedLink bool // Is this object a translated link
}

// Directory represents a local filesystem directory
type Directory struct {
	Object
}

// ------------------------------------------------------------

var (
	errLinksAndCopyLinks = errors.New("can't use -l/--links with -L/--copy-links")
	errLinksNeedsSuffix  = errors.New("need \"" + linkSuffix + "\" suffix to refer to symlink when using -l/--links")
)

// NewFs constructs an Fs from the path
func NewFs(ctx context.Context, name, root string, m configmap.Mapper) (fs.Fs, error) {
	// Parse config into Options struct
	opt := new(Options)
	err := configstruct.Set(m, opt)
	if err != nil {
		return nil, err
	}
	if opt.TranslateSymlinks && opt.FollowSymlinks {
		return nil, errLinksAndCopyLinks
	}

	f := &Fs{
		name:   name,
		opt:    *opt,
		warned: make(map[string]struct{}),
		dev:    devUnset,
		lstat:  os.Lstat,
	}
	if xattrSupported {
		f.xattrSupported.Store(1)
	}
	f.root = cleanRootPath(root, f.opt.NoUNC, f.opt.Enc)
	f.features = (&fs.Features{
		CaseInsensitive:          f.caseInsensitive(),
		CanHaveEmptyDirectories:  true,
		IsLocal:                  true,
		SlowHash:                 true,
		ReadMetadata:             true,
		WriteMetadata:            true,
		ReadDirMetadata:          true,
		WriteDirMetadata:         true,
		WriteDirSetModTime:       true,
		UserDirMetadata:          xattrSupported, // can only R/W general purpose metadata if xattrs are supported
		DirModTimeUpdatesOnWrite: true,
		UserMetadata:             xattrSupported, // can only R/W general purpose metadata if xattrs are supported
		FilterAware:              true,
		PartialUploads:           true,
	}).Fill(ctx, f)
	if opt.FollowSymlinks {
		f.lstat = os.Stat
	}
	if opt.NoClone {
		// Disable server-side copy when --local-no-clone is set
		f.features.Copy = nil
	}

	// Check to see if this points to a file
	fi, err := f.lstat(f.root)
	if err == nil {
		f.dev = readDevice(fi, f.opt.OneFileSystem)
	}
	// Check to see if this is a .rclonelink if not found
	hasLinkSuffix := strings.HasSuffix(f.root, linkSuffix)
	if hasLinkSuffix && opt.TranslateSymlinks && os.IsNotExist(err) {
		fi, err = f.lstat(strings.TrimSuffix(f.root, linkSuffix))
	}
	if err == nil && f.isRegular(fi.Mode()) {
		// Handle the odd case, that a symlink was specified by name without the link suffix
		if !hasLinkSuffix && opt.TranslateSymlinks && fi.Mode()&os.ModeSymlink != 0 {
			return nil, errLinksNeedsSuffix
		}
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
	return f.opt.Enc.ToStandardPath(filepath.ToSlash(f.root))
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
		return nil, fs.ErrorIsDir
	}
	return o, nil
}

// NewObject finds the Object at remote.  If it can't be found
// it returns the error ErrorObjectNotFound.
func (f *Fs) NewObject(ctx context.Context, remote string) (fs.Object, error) {
	return f.newObjectWithInfo(remote, nil)
}

// Create new directory object from the info passed in
func (f *Fs) newDirectory(dir string, fi os.FileInfo) *Directory {
	o := f.newObject(dir)
	o.setMetadata(fi)
	return &Directory{
		Object: *o,
	}
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
	filter, useFilter := filter.GetConfig(ctx), filter.GetUseFilter(ctx)

	fsDirPath := f.localPath(dir)
	_, err = os.Stat(fsDirPath)
	if err != nil {
		return nil, fs.ErrorDirNotFound
	}

	fd, err := os.Open(fsDirPath)
	if err != nil {
		isPerm := os.IsPermission(err)
		err = fmt.Errorf("failed to open directory %q: %w", dir, err)
		fs.Errorf(dir, "%v", err)
		if isPerm {
			_ = accounting.Stats(ctx).Error(fserrors.NoRetryError(err))
			err = nil // ignore error but fail sync
		}
		return nil, err
	}
	defer func() {
		cerr := fd.Close()
		if cerr != nil && err == nil {
			err = fmt.Errorf("failed to close directory %q:: %w", dir, cerr)
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
					if os.IsNotExist(fierr) {
						// skip entry removed by a concurrent goroutine
						continue
					}
					if fierr != nil {
						// Don't report errors on any file names that are excluded
						if useFilter {
							newRemote := f.cleanRemote(dir, name)
							if !filter.IncludeRemote(newRemote) {
								continue
							}
						}
						fierr = fmt.Errorf("failed to get info about directory entry %q: %w", namepath, fierr)
						fs.Errorf(dir, "%v", fierr)
						_ = accounting.Stats(ctx).Error(fserrors.NoRetryError(fierr)) // fail the sync
						continue
					}
					fis = append(fis, fi)
				}
			}
		}
		if err != nil {
			return nil, fmt.Errorf("failed to read directory entry: %w", err)
		}

		for _, fi := range fis {
			name := fi.Name()
			mode := fi.Mode()
			newRemote := f.cleanRemote(dir, name)
			// Follow symlinks if required
			if f.opt.FollowSymlinks && (mode&os.ModeSymlink) != 0 {
				localPath := filepath.Join(fsDirPath, name)
				fi, err = os.Stat(localPath)
				// Quietly skip errors on excluded files and directories
				if err != nil && useFilter && !filter.IncludeRemote(newRemote) {
					continue
				}
				if os.IsNotExist(err) || isCircularSymlinkError(err) {
					// Skip bad symlinks and circular symlinks
					err = fserrors.NoRetryError(fmt.Errorf("symlink: %w", err))
					fs.Errorf(newRemote, "Listing error: %v", err)
					err = accounting.Stats(ctx).Error(err)
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
					d := f.newDirectory(newRemote, fi)
					entries = append(entries, d)
				}
			} else {
				// Check whether this link should be translated
				if f.opt.TranslateSymlinks && fi.Mode()&os.ModeSymlink != 0 {
					newRemote += linkSuffix
				}
				// Don't include non directory if not included
				// we leave directory filtering to the layer above
				if useFilter && !filter.IncludeRemote(newRemote) {
					continue
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
	if f.opt.UTFNorm {
		filename = norm.NFC.String(filename)
	}
	remote = path.Join(dir, f.opt.Enc.ToStandardName(filename))

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
	return filepath.Join(f.root, filepath.FromSlash(f.opt.Enc.FromStandardPath(name)))
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
	localPath := f.localPath(dir)
	err := file.MkdirAll(localPath, 0777)
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

// DirSetModTime sets the directory modtime for dir
func (f *Fs) DirSetModTime(ctx context.Context, dir string, modTime time.Time) error {
	o := Object{
		fs:     f,
		remote: dir,
		path:   f.localPath(dir),
	}
	return o.SetModTime(ctx, modTime)
}

// MkdirMetadata makes the directory passed in as dir.
//
// It shouldn't return an error if it already exists.
//
// If the metadata is not nil it is set.
//
// It returns the directory that was created.
func (f *Fs) MkdirMetadata(ctx context.Context, dir string, metadata fs.Metadata) (fs.Directory, error) {
	// Find and or create the directory
	localPath := f.localPath(dir)
	fi, err := f.lstat(localPath)
	if errors.Is(err, os.ErrNotExist) {
		err := f.Mkdir(ctx, dir)
		if err != nil {
			return nil, fmt.Errorf("mkdir metadata: failed make directory: %w", err)
		}
		fi, err = f.lstat(localPath)
		if err != nil {
			return nil, fmt.Errorf("mkdir metadata: failed to read info: %w", err)
		}
	} else if err != nil {
		return nil, err
	}

	// Create directory object
	d := f.newDirectory(dir, fi)

	// Set metadata on the directory object if provided
	if metadata != nil {
		err = d.writeMetadata(metadata)
		if err != nil {
			return nil, fmt.Errorf("failed to set metadata on directory: %w", err)
		}
		// Re-read info now we have finished setting stuff
		err = d.lstat()
		if err != nil {
			return nil, fmt.Errorf("mkdir metadata: failed to re-read info: %w", err)
		}
	}
	return d, nil
}

// Rmdir removes the directory
//
// If it isn't empty it will return an error
func (f *Fs) Rmdir(ctx context.Context, dir string) error {
	localPath := f.localPath(dir)
	if fi, err := os.Stat(localPath); err != nil {
		return err
	} else if !fi.IsDir() {
		return fs.ErrorIsFile
	}
	return os.Remove(localPath)
}

// Precision of the file system
func (f *Fs) Precision() (precision time.Duration) {
	if f.opt.NoSetModTime {
		return fs.ModTimeNotSupported
	}

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
	fd, err := os.CreateTemp("", "rclone")
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

// Move src to this remote using server-side move operations.
//
// This is stored with the remote path given.
//
// It returns the destination Object and a possible error.
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
	dstObj.fs.objectMetaMu.RLock()
	dstObjMode := dstObj.mode
	dstObj.fs.objectMetaMu.RUnlock()

	// Check it is a file if it exists
	err := dstObj.lstat()
	if os.IsNotExist(err) {
		// OK
	} else if err != nil {
		return nil, err
	} else if !dstObj.fs.isRegular(dstObjMode) {
		// It isn't a file
		return nil, errors.New("can't move file onto non-file")
	}

	// Create destination
	err = dstObj.mkdirAll()
	if err != nil {
		return nil, err
	}

	// Fetch metadata if --metadata is in use
	meta, err := fs.GetMetadataOptions(ctx, f, src, fs.MetadataAsOpenOptions(ctx))
	if err != nil {
		return nil, fmt.Errorf("move: failed to read metadata: %w", err)
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

	// Set metadata if --metadata is in use
	err = dstObj.writeMetadata(meta)
	if err != nil {
		return nil, fmt.Errorf("move: failed to set metadata: %w", err)
	}

	// Update the info
	err = dstObj.lstat()
	if err != nil {
		return nil, err
	}

	return dstObj, nil
}

// DirMove moves src, srcRemote to this remote at dstRemote
// using server-side move operations.
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
	err = file.MkdirAll(dstParentPath, 0777)
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

var commandHelp = []fs.CommandHelp{
	{
		Name:  "noop",
		Short: "A null operation for testing backend commands",
		Long: `This is a test command which has some options
you can try to change the output.`,
		Opts: map[string]string{
			"echo":  "echo the input arguments",
			"error": "return an error based on option value",
		},
	},
}

// Command the backend to run a named command
//
// The command run is name
// args may be used to read arguments from
// opts may be used to read optional arguments from
//
// The result should be capable of being JSON encoded
// If it is a string or a []string it will be shown to the user
// otherwise it will be JSON encoded and shown to the user like that
func (f *Fs) Command(ctx context.Context, name string, arg []string, opt map[string]string) (interface{}, error) {
	switch name {
	case "noop":
		if txt, ok := opt["error"]; ok {
			if txt == "" {
				txt = "unspecified error"
			}
			return nil, errors.New(txt)
		}
		if _, ok := opt["echo"]; ok {
			out := map[string]interface{}{}
			out["name"] = name
			out["arg"] = arg
			out["opt"] = opt
			return out, nil
		}
		return nil, nil
	default:
		return nil, fs.ErrorCommandNotFound
	}
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
	o.fs.objectMetaMu.RLock()
	oldtime := o.modTime
	oldsize := o.size
	o.fs.objectMetaMu.RUnlock()
	err := o.lstat()
	var changed bool
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			// If file not found then we assume any accumulated
			// hashes are OK - this will error on Open
			changed = true
		} else {
			return "", fmt.Errorf("hash: failed to stat: %w", err)
		}
	} else {
		o.fs.objectMetaMu.RLock()
		changed = !o.modTime.Equal(oldtime) || oldsize != o.size
		o.fs.objectMetaMu.RUnlock()
	}

	o.fs.objectMetaMu.RLock()
	hashValue, hashFound := o.hashes[r]
	o.fs.objectMetaMu.RUnlock()

	if changed || !hashFound {
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
		// If not checking for updates, only read size given
		if o.fs.opt.NoCheckUpdated {
			in = readers.NewLimitedReadCloser(in, o.size)
		}
		if err != nil {
			return "", fmt.Errorf("hash: failed to open: %w", err)
		}
		var hashes map[hash.Type]string
		hashes, err = hash.StreamTypes(readers.NewContextReader(ctx, in), hash.NewHashSet(r))
		closeErr := in.Close()
		if err != nil {
			return "", fmt.Errorf("hash: failed to read: %w", err)
		}
		if closeErr != nil {
			return "", fmt.Errorf("hash: failed to close: %w", closeErr)
		}
		hashValue = hashes[r]
		o.fs.objectMetaMu.Lock()
		if o.hashes == nil {
			o.hashes = hashes
		} else {
			o.hashes[r] = hashValue
		}
		o.fs.objectMetaMu.Unlock()
	}
	return hashValue, nil
}

// Size returns the size of an object in bytes
func (o *Object) Size() int64 {
	o.fs.objectMetaMu.RLock()
	defer o.fs.objectMetaMu.RUnlock()
	return o.size
}

// ModTime returns the modification time of the object
func (o *Object) ModTime(ctx context.Context) time.Time {
	o.fs.objectMetaMu.RLock()
	defer o.fs.objectMetaMu.RUnlock()
	return o.modTime
}

// Set the atime and ltime of the object
func (o *Object) setTimes(atime, mtime time.Time) (err error) {
	if o.translatedLink {
		err = lChtimes(o.path, atime, mtime)
	} else {
		err = os.Chtimes(o.path, atime, mtime)
	}
	return err
}

// SetModTime sets the modification time of the local fs object
func (o *Object) SetModTime(ctx context.Context, modTime time.Time) error {
	if o.fs.opt.NoSetModTime {
		return nil
	}
	err := o.setTimes(modTime, modTime)
	if err != nil {
		return err
	}
	// Re-read metadata
	return o.lstat()
}

// Storable returns a boolean showing if this object is storable
func (o *Object) Storable() bool {
	o.fs.objectMetaMu.RLock()
	mode := o.mode
	o.fs.objectMetaMu.RUnlock()
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
			return 0, fmt.Errorf("can't read status of source file while transferring: %w", err)
		}
		file.o.fs.objectMetaMu.RLock()
		oldtime := file.o.modTime
		oldsize := file.o.size
		file.o.fs.objectMetaMu.RUnlock()
		if oldsize != fi.Size() {
			return 0, fserrors.NoLowLevelRetryError(fmt.Errorf("can't copy - source file is being updated (size changed from %d to %d)", oldsize, fi.Size()))
		}
		if !oldtime.Equal(readTime(file.o.fs.opt.TimeType, fi)) {
			return 0, fserrors.NoLowLevelRetryError(fmt.Errorf("can't copy - source file is being updated (mod time changed from %v to %v)", oldtime, fi.ModTime()))
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
			file.o.fs.objectMetaMu.Lock()
			file.o.hashes = file.hash.Sums()
			file.o.fs.objectMetaMu.Unlock()
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
	return readers.NewLimitedReadCloser(io.NopCloser(strings.NewReader(linkdst[offset:])), limit), nil
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
			offset, limit = x.Decode(o.Size())
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

	// Update the file info before we start reading
	err = o.lstat()
	if err != nil {
		return nil, err
	}

	// If not checking updated then limit to current size.  This means if
	// file is being extended, readers will read a o.Size() bytes rather
	// than the new size making for a consistent upload.
	if limit < 0 && o.fs.opt.NoCheckUpdated {
		limit = o.size
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
	return file.MkdirAll(dir, 0777)
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

	// Wipe hashes before update
	o.clearHashCache()

	var symlinkData bytes.Buffer
	// If the object is a regular file, create it.
	// If it is a translated link, just read in the contents, and
	// then create a symlink
	if !o.translatedLink {
		f, err := file.OpenFile(o.path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0666)
		if err != nil {
			if runtime.GOOS == "windows" && os.IsPermission(err) {
				// If permission denied on Windows might be trying to update a
				// hidden file, in which case try opening without CREATE
				// See: https://stackoverflow.com/questions/13215716/ioerror-errno-13-permission-denied-when-trying-to-open-hidden-file-in-w-mod
				f, err = file.OpenFile(o.path, os.O_WRONLY|os.O_TRUNC, 0666)
				if err != nil {
					return err
				}
			} else {
				return err
			}
		}
		if !o.fs.opt.NoPreAllocate {
			// Pre-allocate the file for performance reasons
			err = file.PreAllocate(src.Size(), f)
			if err != nil {
				fs.Debugf(o, "Failed to pre-allocate: %v", err)
				if err == file.ErrDiskFull {
					_ = f.Close()
					return err
				}
			}
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
		o.fs.objectMetaMu.Lock()
		o.hashes = hasher.Sums()
		o.fs.objectMetaMu.Unlock()
	}

	// Set the mtime
	err = o.SetModTime(ctx, src.ModTime(ctx))
	if err != nil {
		return err
	}

	// Fetch and set metadata if --metadata is in use
	meta, err := fs.GetMetadataOptions(ctx, o.fs, src, options)
	if err != nil {
		return fmt.Errorf("failed to read metadata from source object: %w", err)
	}
	err = o.writeMetadata(meta)
	if err != nil {
		return fmt.Errorf("failed to set metadata: %w", err)
	}

	// ReRead info now that we have finished
	return o.lstat()
}

var sparseWarning sync.Once

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
	if !f.opt.NoPreAllocate {
		err = file.PreAllocate(size, out)
		if err != nil {
			fs.Debugf(o, "Failed to pre-allocate: %v", err)
		}
	}
	if !f.opt.NoSparse && file.SetSparseImplemented {
		sparseWarning.Do(func() {
			fs.Infof(nil, "Writing sparse files: use --local-no-sparse or --multi-thread-streams 0 to disable")
		})
		// Set the file to be a sparse file (important on Windows)
		err = file.SetSparse(out)
		if err != nil {
			fs.Errorf(o, "Failed to set sparse: %v", err)
		}
	}

	return out, nil
}

// setMetadata sets the file info from the os.FileInfo passed in
func (o *Object) setMetadata(info os.FileInfo) {
	// if not checking updated then don't update the stat
	if o.fs.opt.NoCheckUpdated && !o.modTime.IsZero() {
		return
	}
	o.fs.objectMetaMu.Lock()
	o.size = info.Size()
	o.modTime = readTime(o.fs.opt.TimeType, info)
	o.mode = info.Mode()
	o.fs.objectMetaMu.Unlock()
	// Read the size of the link.
	//
	// The value in info.Size() is not always correct
	// - Windows links read as 0 size
	// - Some virtual filesystems (such ash LucidLink) links read as 0 size
	// - Android - some versions the links are larger than readlink suggests
	if o.translatedLink {
		linkdst, err := os.Readlink(o.path)
		if err != nil {
			fs.Errorf(o, "Failed to read link size: %v", err)
		} else {
			o.size = int64(len(linkdst))
		}
	}
}

// clearHashCache wipes any cached hashes for the object
func (o *Object) clearHashCache() {
	o.fs.objectMetaMu.Lock()
	o.hashes = nil
	o.fs.objectMetaMu.Unlock()
}

// Stat an Object into info
func (o *Object) lstat() error {
	info, err := o.fs.lstat(o.path)
	if err == nil {
		o.setMetadata(info)
	}
	return err
}

// Remove an object
func (o *Object) Remove(ctx context.Context) error {
	o.clearHashCache()
	return remove(o.path)
}

// Metadata returns metadata for an object
//
// It should return nil if there is no Metadata
func (o *Object) Metadata(ctx context.Context) (metadata fs.Metadata, err error) {
	metadata, err = o.getXattr()
	if err != nil {
		return nil, err
	}
	err = o.readMetadataFromFile(&metadata)
	if err != nil {
		return nil, err
	}
	return metadata, nil
}

// Write the metadata on the object
func (o *Object) writeMetadata(metadata fs.Metadata) (err error) {
	err = o.setXattr(metadata)
	if err != nil {
		return err
	}
	err = o.writeMetadataToFile(metadata)
	if err != nil {
		return err
	}
	return err
}

// SetMetadata sets metadata for an Object
//
// It should return fs.ErrorNotImplemented if it can't set metadata
func (o *Object) SetMetadata(ctx context.Context, metadata fs.Metadata) error {
	err := o.writeMetadata(metadata)
	if err != nil {
		return fmt.Errorf("SetMetadata failed on Object: %w", err)
	}
	// Re-read info now we have finished setting stuff
	return o.lstat()
}

func cleanRootPath(s string, noUNC bool, enc encoder.MultiEncoder) string {
	var vol string
	if runtime.GOOS == "windows" {
		vol = filepath.VolumeName(s)
		if vol == `\\?` && len(s) >= 6 {
			// `\\?\C:`
			vol = s[:6]
		}
		s = s[len(vol):]
	}
	// Don't use FromStandardPath. Make sure Dot (`.`, `..`) as name will not be reencoded
	// Take care of the case Standard: ./．/‛． (the first dot means current directory)
	if enc != encoder.Standard {
		s = filepath.ToSlash(s)
		parts := strings.Split(s, "/")
		encoded := make([]string, len(parts))
		changed := false
		for i, p := range parts {
			if (p == ".") || (p == "..") {
				encoded[i] = p
				continue
			}
			part := enc.FromStandardName(p)
			changed = changed || part != p
			encoded[i] = part
		}
		if changed {
			s = strings.Join(encoded, "/")
		}
		s = filepath.FromSlash(s)
	}
	if runtime.GOOS == "windows" {
		s = vol + s
	}
	s2, err := filepath.Abs(s)
	if err == nil {
		s = s2
	}
	if !noUNC {
		// Convert to UNC. It does nothing on non windows platforms.
		s = file.UNCPath(s)
	}
	return s
}

// Items returns the count of items in this directory or this
// directory and subdirectories if known, -1 for unknown
func (d *Directory) Items() int64 {
	return -1
}

// ID returns the internal ID of this directory if known, or
// "" otherwise
func (d *Directory) ID() string {
	return ""
}

// SetMetadata sets metadata for a Directory
//
// It should return fs.ErrorNotImplemented if it can't set metadata
func (d *Directory) SetMetadata(ctx context.Context, metadata fs.Metadata) error {
	err := d.writeMetadata(metadata)
	if err != nil {
		return fmt.Errorf("SetMetadata failed on Directory: %w", err)
	}
	// Re-read info now we have finished setting stuff
	return d.lstat()
}

// Hash does nothing on a directory
//
// This method is implemented with the incorrect type signature to
// stop the Directory type asserting to fs.Object or fs.ObjectInfo
func (d *Directory) Hash() {
	// Does nothing
}

// Check the interfaces are satisfied
var (
	_ fs.Fs              = &Fs{}
	_ fs.PutStreamer     = &Fs{}
	_ fs.Mover           = &Fs{}
	_ fs.DirMover        = &Fs{}
	_ fs.Commander       = &Fs{}
	_ fs.OpenWriterAter  = &Fs{}
	_ fs.DirSetModTimer  = &Fs{}
	_ fs.MkdirMetadataer = &Fs{}
	_ fs.Object          = &Object{}
	_ fs.Metadataer      = &Object{}
	_ fs.SetMetadataer   = &Object{}
	_ fs.Directory       = &Directory{}
	_ fs.SetModTimer     = &Directory{}
	_ fs.SetMetadataer   = &Directory{}
)
