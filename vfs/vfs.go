// Package vfs provides a virtual filing system layer over rclone's
// native objects.
//
// It attempts to behave in a similar way to Go's filing system
// manipulation code in the os package.  The same named function
// should behave in an identical fashion.  The objects also obey Go's
// standard interfaces.
//
// Note that paths don't start or end with /, so the root directory
// may be referred to as "".  However Stat strips slashes so you can
// use paths with slashes in.
//
// It also includes directory caching
//
// The vfs package returns Error values to signal precisely which
// error conditions have ocurred.  It may also return general errors
// it receives.  It tries to use os Error values (e.g. os.ErrExist)
// where possible.

//go:generate sh -c "go run make_open_tests.go | gofmt > open_test.go"

package vfs

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/cache"
	"github.com/rclone/rclone/fs/log"
	"github.com/rclone/rclone/fs/rc"
	"github.com/rclone/rclone/fs/walk"
	"github.com/rclone/rclone/vfs/vfscache"
	"github.com/rclone/rclone/vfs/vfscommon"
)

// Node represents either a directory (*Dir) or a file (*File)
type Node interface {
	os.FileInfo
	IsFile() bool
	Inode() uint64
	SetModTime(modTime time.Time) error
	Sync() error
	Remove() error
	RemoveAll() error
	DirEntry() fs.DirEntry
	VFS() *VFS
	Open(flags int) (Handle, error)
	Truncate(size int64) error
	Path() string
	SetSys(interface{})
}

// Check interfaces
var (
	_ Node = (*File)(nil)
	_ Node = (*Dir)(nil)
)

// Nodes is a slice of Node
type Nodes []Node

// Sort functions
func (ns Nodes) Len() int           { return len(ns) }
func (ns Nodes) Swap(i, j int)      { ns[i], ns[j] = ns[j], ns[i] }
func (ns Nodes) Less(i, j int) bool { return ns[i].Path() < ns[j].Path() }

// Noder represents something which can return a node
type Noder interface {
	fmt.Stringer
	Node() Node
}

// Check interfaces
var (
	_ Noder = (*File)(nil)
	_ Noder = (*Dir)(nil)
	_ Noder = (*ReadFileHandle)(nil)
	_ Noder = (*WriteFileHandle)(nil)
	_ Noder = (*RWFileHandle)(nil)
	_ Noder = (*DirHandle)(nil)
)

// OsFiler is the methods on *os.File
type OsFiler interface {
	Chdir() error
	Chmod(mode os.FileMode) error
	Chown(uid, gid int) error
	Close() error
	Fd() uintptr
	Name() string
	Read(b []byte) (n int, err error)
	ReadAt(b []byte, off int64) (n int, err error)
	Readdir(n int) ([]os.FileInfo, error)
	Readdirnames(n int) (names []string, err error)
	Seek(offset int64, whence int) (ret int64, err error)
	Stat() (os.FileInfo, error)
	Sync() error
	Truncate(size int64) error
	Write(b []byte) (n int, err error)
	WriteAt(b []byte, off int64) (n int, err error)
	WriteString(s string) (n int, err error)
}

// Handle is the interface satisfied by open files or directories.
// It is the methods on *os.File, plus a few more useful for FUSE
// filingsystems.  Not all of them are supported.
type Handle interface {
	OsFiler
	// Additional methods useful for FUSE filesystems
	Flush() error
	Release() error
	Node() Node
	//	Size() int64
}

// baseHandle implements all the missing methods
type baseHandle struct{}

func (h baseHandle) Chdir() error                                         { return ENOSYS }
func (h baseHandle) Chmod(mode os.FileMode) error                         { return ENOSYS }
func (h baseHandle) Chown(uid, gid int) error                             { return ENOSYS }
func (h baseHandle) Close() error                                         { return ENOSYS }
func (h baseHandle) Fd() uintptr                                          { return 0 }
func (h baseHandle) Name() string                                         { return "" }
func (h baseHandle) Read(b []byte) (n int, err error)                     { return 0, ENOSYS }
func (h baseHandle) ReadAt(b []byte, off int64) (n int, err error)        { return 0, ENOSYS }
func (h baseHandle) Readdir(n int) ([]os.FileInfo, error)                 { return nil, ENOSYS }
func (h baseHandle) Readdirnames(n int) (names []string, err error)       { return nil, ENOSYS }
func (h baseHandle) Seek(offset int64, whence int) (ret int64, err error) { return 0, ENOSYS }
func (h baseHandle) Stat() (os.FileInfo, error)                           { return nil, ENOSYS }
func (h baseHandle) Sync() error                                          { return nil }
func (h baseHandle) Truncate(size int64) error                            { return ENOSYS }
func (h baseHandle) Write(b []byte) (n int, err error)                    { return 0, ENOSYS }
func (h baseHandle) WriteAt(b []byte, off int64) (n int, err error)       { return 0, ENOSYS }
func (h baseHandle) WriteString(s string) (n int, err error)              { return 0, ENOSYS }
func (h baseHandle) Flush() (err error)                                   { return ENOSYS }
func (h baseHandle) Release() (err error)                                 { return ENOSYS }
func (h baseHandle) Node() Node                                           { return nil }

//func (h baseHandle) Size() int64                                          { return 0 }

// Check interfaces
var (
	_ OsFiler = (*os.File)(nil)
	_ Handle  = (*baseHandle)(nil)
	_ Handle  = (*ReadFileHandle)(nil)
	_ Handle  = (*WriteFileHandle)(nil)
	_ Handle  = (*DirHandle)(nil)
)

// VFS represents the top level filing system
type VFS struct {
	f           fs.Fs
	root        *Dir
	Opt         vfscommon.Options
	cache       *vfscache.Cache
	cancelCache context.CancelFunc
	usageMu     sync.Mutex
	usageTime   time.Time
	usage       *fs.Usage
	pollChan    chan time.Duration
	inUse       int32 // count of number of opens accessed with atomic
}

// Keep track of active VFS keyed on fs.ConfigString(f)
var (
	activeMu sync.Mutex
	active   = map[string][]*VFS{}
)

// New creates a new VFS and root directory.  If opt is nil, then
// DefaultOpt will be used
func New(f fs.Fs, opt *vfscommon.Options) *VFS {
	fsDir := fs.NewDir("", time.Now())
	vfs := &VFS{
		f:     f,
		inUse: int32(1),
	}

	// Make a copy of the options
	if opt != nil {
		vfs.Opt = *opt
	} else {
		vfs.Opt = vfscommon.DefaultOpt
	}

	// Fill out anything else
	vfs.Opt.Init()

	// Find a VFS with the same name and options and return it if possible
	activeMu.Lock()
	defer activeMu.Unlock()
	configName := fs.ConfigString(f)
	for _, activeVFS := range active[configName] {
		if vfs.Opt == activeVFS.Opt {
			fs.Debugf(f, "Re-using VFS from active cache")
			atomic.AddInt32(&activeVFS.inUse, 1)
			return activeVFS
		}
	}
	// Put the VFS into the active cache
	active[configName] = append(active[configName], vfs)

	// Create root directory
	vfs.root = newDir(vfs, f, nil, fsDir)

	// Start polling function
	features := vfs.f.Features()
	if do := features.ChangeNotify; do != nil {
		vfs.pollChan = make(chan time.Duration)
		do(context.TODO(), vfs.root.changeNotify, vfs.pollChan)
		vfs.pollChan <- vfs.Opt.PollInterval
	} else if vfs.Opt.PollInterval > 0 {
		fs.Infof(f, "poll-interval is not supported by this remote")
	}

	// Warn if can't stream
	if !vfs.Opt.ReadOnly && vfs.Opt.CacheMode < vfscommon.CacheModeWrites && features.PutStream == nil {
		fs.Logf(f, "--vfs-cache-mode writes or full is recommended for this remote as it can't stream")
	}

	vfs.SetCacheMode(vfs.Opt.CacheMode)

	// Pin the Fs into the cache so that when we use cache.NewFs
	// with the same remote string we get this one. The Pin is
	// removed when the vfs is finalized
	cache.PinUntilFinalized(f, vfs)

	return vfs
}

// Stats returns info about the VFS
func (vfs *VFS) Stats() (out rc.Params) {
	out = make(rc.Params)
	out["fs"] = fs.ConfigString(vfs.f)
	out["opt"] = vfs.Opt
	out["inUse"] = atomic.LoadInt32(&vfs.inUse)

	var (
		dirs  int
		files int
	)
	vfs.root.walk(func(d *Dir) {
		dirs++
		files += len(d.items)
	})
	inf := make(rc.Params)
	out["metadataCache"] = inf
	inf["dirs"] = dirs
	inf["files"] = files

	if vfs.cache != nil {
		out["diskCache"] = vfs.cache.Stats()
	}
	return out
}

// Return the number of active cache entries and a VFS if any are in
// the cache.
func activeCacheEntries() (vfs *VFS, count int) {
	activeMu.Lock()
	for _, vfses := range active {
		count += len(vfses)
		if len(vfses) > 0 {
			vfs = vfses[0]
		}
	}
	activeMu.Unlock()
	return vfs, count
}

// Fs returns the Fs passed into the New call
func (vfs *VFS) Fs() fs.Fs {
	return vfs.f
}

// SetCacheMode change the cache mode
func (vfs *VFS) SetCacheMode(cacheMode vfscommon.CacheMode) {
	vfs.shutdownCache()
	vfs.cache = nil
	if cacheMode > vfscommon.CacheModeOff {
		ctx, cancel := context.WithCancel(context.Background())
		cache, err := vfscache.New(ctx, vfs.f, &vfs.Opt, vfs.AddVirtual) // FIXME pass on context or get from Opt?
		if err != nil {
			fs.Errorf(nil, "Failed to create vfs cache - disabling: %v", err)
			vfs.Opt.CacheMode = vfscommon.CacheModeOff
			cancel()
			return
		}
		vfs.Opt.CacheMode = cacheMode
		vfs.cancelCache = cancel
		vfs.cache = cache
	}
}

// shutdown the cache if it was running
func (vfs *VFS) shutdownCache() {
	if vfs.cancelCache != nil {
		vfs.cancelCache()
		vfs.cancelCache = nil
	}
}

// Shutdown stops any background go-routines and removes the VFS from
// the active ache.
func (vfs *VFS) Shutdown() {
	if atomic.AddInt32(&vfs.inUse, -1) > 0 {
		return
	}

	// Remove from active cache
	activeMu.Lock()
	configName := fs.ConfigString(vfs.f)
	activeVFSes := active[configName]
	for i, activeVFS := range activeVFSes {
		if activeVFS == vfs {
			activeVFSes[i] = nil
			active[configName] = append(activeVFSes[:i], activeVFSes[i+1:]...)
			break
		}
	}
	activeMu.Unlock()

	vfs.shutdownCache()
}

// CleanUp deletes the contents of the on disk cache
func (vfs *VFS) CleanUp() error {
	if vfs.Opt.CacheMode == vfscommon.CacheModeOff {
		return nil
	}
	return vfs.cache.CleanUp()
}

// FlushDirCache empties the directory cache
func (vfs *VFS) FlushDirCache() {
	vfs.root.ForgetAll()
}

// WaitForWriters sleeps until all writers have finished or
// time.Duration has elapsed
func (vfs *VFS) WaitForWriters(timeout time.Duration) {
	defer log.Trace(nil, "timeout=%v", timeout)("")
	tickTime := 10 * time.Millisecond
	deadline := time.NewTimer(timeout)
	defer deadline.Stop()
	tick := time.NewTimer(tickTime)
	defer tick.Stop()
	tick.Stop()
	for {
		writers := vfs.root.countActiveWriters()
		cacheInUse := 0
		if vfs.cache != nil {
			cacheInUse = vfs.cache.TotalInUse()
		}
		if writers == 0 && cacheInUse == 0 {
			return
		}
		fs.Debugf(nil, "Still %d writers active and %d cache items in use, waiting %v", writers, cacheInUse, tickTime)
		tick.Reset(tickTime)
		select {
		case <-tick.C:
			break
		case <-deadline.C:
			fs.Errorf(nil, "Exiting even though %d writers active and %d cache items in use after %v\n%s", writers, cacheInUse, timeout, vfs.cache.Dump())
			return
		}
		tickTime *= 2
		if tickTime > time.Second {
			tickTime = time.Second
		}
	}
}

// Root returns the root node
func (vfs *VFS) Root() (*Dir, error) {
	// fs.Debugf(vfs.f, "Root()")
	return vfs.root, nil
}

var inodeCount uint64

// newInode creates a new unique inode number
func newInode() (inode uint64) {
	return atomic.AddUint64(&inodeCount, 1)
}

// Stat finds the Node by path starting from the root
//
// It is the equivalent of os.Stat - Node contains the os.FileInfo
// interface.
func (vfs *VFS) Stat(path string) (node Node, err error) {
	path = strings.Trim(path, "/")
	node = vfs.root
	for path != "" {
		i := strings.IndexRune(path, '/')
		var name string
		if i < 0 {
			name, path = path, ""
		} else {
			name, path = path[:i], path[i+1:]
		}
		if name == "" {
			continue
		}
		dir, ok := node.(*Dir)
		if !ok {
			// We need to look in a directory, but found a file
			return nil, ENOENT
		}
		node, err = dir.Stat(name)
		if err != nil {
			return nil, err
		}
	}
	return
}

// StatParent finds the parent directory and the leaf name of a path
func (vfs *VFS) StatParent(name string) (dir *Dir, leaf string, err error) {
	name = strings.Trim(name, "/")
	parent, leaf := path.Split(name)
	node, err := vfs.Stat(parent)
	if err != nil {
		return nil, "", err
	}
	if node.IsFile() {
		return nil, "", os.ErrExist
	}
	dir = node.(*Dir)
	return dir, leaf, nil
}

// decodeOpenFlags returns a string representing the open flags
func decodeOpenFlags(flags int) string {
	var out []string
	rdwrMode := flags & accessModeMask
	switch rdwrMode {
	case os.O_RDONLY:
		out = append(out, "O_RDONLY")
	case os.O_WRONLY:
		out = append(out, "O_WRONLY")
	case os.O_RDWR:
		out = append(out, "O_RDWR")
	default:
		out = append(out, fmt.Sprintf("0x%X", rdwrMode))
	}
	if flags&os.O_APPEND != 0 {
		out = append(out, "O_APPEND")
	}
	if flags&os.O_CREATE != 0 {
		out = append(out, "O_CREATE")
	}
	if flags&os.O_EXCL != 0 {
		out = append(out, "O_EXCL")
	}
	if flags&os.O_SYNC != 0 {
		out = append(out, "O_SYNC")
	}
	if flags&os.O_TRUNC != 0 {
		out = append(out, "O_TRUNC")
	}
	flags &^= accessModeMask | os.O_APPEND | os.O_CREATE | os.O_EXCL | os.O_SYNC | os.O_TRUNC
	if flags != 0 {
		out = append(out, fmt.Sprintf("0x%X", flags))
	}
	return strings.Join(out, "|")
}

// OpenFile a file according to the flags and perm provided
func (vfs *VFS) OpenFile(name string, flags int, perm os.FileMode) (fd Handle, err error) {
	defer log.Trace(name, "flags=%s, perm=%v", decodeOpenFlags(flags), perm)("fd=%v, err=%v", &fd, &err)

	// http://pubs.opengroup.org/onlinepubs/7908799/xsh/open.html
	// The result of using O_TRUNC with O_RDONLY is undefined.
	// Linux seems to truncate the file, but we prefer to return EINVAL
	if flags&accessModeMask == os.O_RDONLY && flags&os.O_TRUNC != 0 {
		return nil, EINVAL
	}

	node, err := vfs.Stat(name)
	if err != nil {
		if err != ENOENT || flags&os.O_CREATE == 0 {
			return nil, err
		}
		// If not found and O_CREATE then create the file
		dir, leaf, err := vfs.StatParent(name)
		if err != nil {
			return nil, err
		}
		node, err = dir.Create(leaf, flags)
		if err != nil {
			return nil, err
		}
	}
	return node.Open(flags)
}

// Open opens the named file for reading. If successful, methods on
// the returned file can be used for reading; the associated file
// descriptor has mode O_RDONLY.
func (vfs *VFS) Open(name string) (Handle, error) {
	return vfs.OpenFile(name, os.O_RDONLY, 0)
}

// Create creates the named file with mode 0666 (before umask), truncating
// it if it already exists. If successful, methods on the returned
// File can be used for I/O; the associated file descriptor has mode
// O_RDWR.
func (vfs *VFS) Create(name string) (Handle, error) {
	return vfs.OpenFile(name, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0666)
}

// Rename oldName to newName
func (vfs *VFS) Rename(oldName, newName string) error {
	// find the parent directories
	oldDir, oldLeaf, err := vfs.StatParent(oldName)
	if err != nil {
		return err
	}
	newDir, newLeaf, err := vfs.StatParent(newName)
	if err != nil {
		return err
	}
	err = oldDir.Rename(oldLeaf, newLeaf, newDir)
	if err != nil {
		return err
	}
	return nil
}

// This works out the missing values from (total, used, free) using
// unknownFree as the intended free space
func fillInMissingSizes(total, used, free, unknownFree int64) (newTotal, newUsed, newFree int64) {
	if total < 0 {
		if free >= 0 {
			total = free
		} else {
			total = unknownFree
		}
		if used >= 0 {
			total += used
		}
	}
	// total is now defined
	if used < 0 {
		if free >= 0 {
			used = total - free
		} else {
			used = 0
		}
	}
	// used is now defined
	if free < 0 {
		free = total - used
	}
	return total, used, free
}

// If the total size isn't known then we will aim for this many bytes free (1 PiB)
const unknownFreeBytes = 1 << 50

// Statfs returns into about the filing system if known
//
// The values will be -1 if they aren't known
//
// This information is cached for the DirCacheTime interval
func (vfs *VFS) Statfs() (total, used, free int64) {
	// defer log.Trace("/", "")("total=%d, used=%d, free=%d", &total, &used, &free)
	vfs.usageMu.Lock()
	defer vfs.usageMu.Unlock()
	total, used, free = -1, -1, -1
	doAbout := vfs.f.Features().About
	if (doAbout != nil || vfs.Opt.UsedIsSize) && (vfs.usageTime.IsZero() || time.Since(vfs.usageTime) >= vfs.Opt.DirCacheTime) {
		var err error
		ctx := context.TODO()
		if doAbout == nil {
			vfs.usage = &fs.Usage{}
		} else {
			vfs.usage, err = doAbout(ctx)
		}
		if vfs.Opt.UsedIsSize {
			var usedBySizeAlgorithm int64
			// Algorithm from `rclone size`
			err = walk.ListR(ctx, vfs.f, "", true, -1, walk.ListObjects, func(entries fs.DirEntries) error {
				entries.ForObject(func(o fs.Object) {
					usedBySizeAlgorithm += o.Size()
				})
				return nil
			})
			vfs.usage.Used = &usedBySizeAlgorithm
		}
		vfs.usageTime = time.Now()
		if err != nil {
			fs.Errorf(vfs.f, "Statfs failed: %v", err)
			return
		}
	}
	if u := vfs.usage; u != nil {
		if u.Total != nil {
			total = *u.Total
		}
		if u.Free != nil {
			free = *u.Free
		}
		if u.Used != nil {
			used = *u.Used
		}
	}
	total, used, free = fillInMissingSizes(total, used, free, unknownFreeBytes)
	return
}

// Remove removes the named file or (empty) directory.
func (vfs *VFS) Remove(name string) error {
	node, err := vfs.Stat(name)
	if err != nil {
		return err
	}
	err = node.Remove()
	if err != nil {
		return err
	}
	return nil
}

// Chtimes changes the access and modification times of the named file, similar
// to the Unix utime() or utimes() functions.
//
// The underlying filesystem may truncate or round the values to a less precise
// time unit.
func (vfs *VFS) Chtimes(name string, atime time.Time, mtime time.Time) error {
	node, err := vfs.Stat(name)
	if err != nil {
		return err
	}
	err = node.SetModTime(mtime)
	if err != nil {
		return err
	}
	return nil
}

// Mkdir creates a new directory with the specified name and permission bits
// (before umask).
func (vfs *VFS) Mkdir(name string, perm os.FileMode) error {
	dir, leaf, err := vfs.StatParent(name)
	if err != nil {
		return err
	}
	_, err = dir.Mkdir(leaf)
	if err != nil {
		return err
	}
	return nil
}

// ReadDir reads the directory named by dirname and returns
// a list of directory entries sorted by filename.
func (vfs *VFS) ReadDir(dirname string) ([]os.FileInfo, error) {
	f, err := vfs.Open(dirname)
	if err != nil {
		return nil, err
	}
	list, err := f.Readdir(-1)
	closeErr := f.Close()
	if err != nil {
		return nil, err
	}
	if closeErr != nil {
		return nil, closeErr
	}
	sort.Slice(list, func(i, j int) bool { return list[i].Name() < list[j].Name() })
	return list, nil
}

// ReadFile reads the file named by filename and returns the contents.
// A successful call returns err == nil, not err == EOF. Because ReadFile
// reads the whole file, it does not treat an EOF from Read as an error
// to be reported.
func (vfs *VFS) ReadFile(filename string) (b []byte, err error) {
	f, err := vfs.Open(filename)
	if err != nil {
		return nil, err
	}
	defer fs.CheckClose(f, &err)
	return ioutil.ReadAll(f)
}

// AddVirtual adds the object (file or dir) to the directory cache
func (vfs *VFS) AddVirtual(remote string, size int64, isDir bool) error {
	dir, leaf, err := vfs.StatParent(remote)
	if err != nil {
		return err
	}
	dir.AddVirtual(leaf, size, false)
	return nil
}
