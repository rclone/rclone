// Package combine implents a backend to combine multipe remotes in a directory tree
package combine

/*
   Have API to add/remove branches in the combine
*/

import (
	"context"
	"errors"
	"fmt"
	"io"
	"path"
	"strings"
	"sync"
	"time"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/cache"
	"github.com/rclone/rclone/fs/config/configmap"
	"github.com/rclone/rclone/fs/config/configstruct"
	"github.com/rclone/rclone/fs/hash"
	"github.com/rclone/rclone/fs/operations"
	"github.com/rclone/rclone/fs/walk"
	"golang.org/x/sync/errgroup"
)

// Register with Fs
func init() {
	fsi := &fs.RegInfo{
		Name:        "combine",
		Description: "Combine several remotes into one",
		NewFs:       NewFs,
		Options: []fs.Option{{
			Name: "upstreams",
			Help: `Upstreams for combining

These should be in the form

    dir=remote:path dir2=remote2:path

Where before the = is specified the root directory and after is the remote to
put there.

Embedded spaces can be added using quotes

    "dir=remote:path with space" "dir2=remote2:path with space"

`,
			Required: true,
			Default:  fs.SpaceSepList(nil),
		}},
	}
	fs.Register(fsi)
}

// Options defines the configuration for this backend
type Options struct {
	Upstreams fs.SpaceSepList `config:"upstreams"`
}

// Fs represents a combine of upstreams
type Fs struct {
	name      string               // name of this remote
	features  *fs.Features         // optional features
	opt       Options              // options for this Fs
	root      string               // the path we are working on
	hashSet   hash.Set             // common hashes
	when      time.Time            // directory times
	upstreams map[string]*upstream // map of upstreams
}

// adjustment stores the info to add a prefix to a path or chop characters off
type adjustment struct {
	root            string
	rootSlash       string
	mountpoint      string
	mountpointSlash string
}

// newAdjustment makes a new path adjustment adjusting between mountpoint and root
//
// mountpoint is the point the upstream is mounted and root is the combine root
func newAdjustment(root, mountpoint string) (a adjustment) {
	return adjustment{
		root:            root,
		rootSlash:       root + "/",
		mountpoint:      mountpoint,
		mountpointSlash: mountpoint + "/",
	}
}

var errNotUnderRoot = errors.New("file not under root")

// do makes the adjustment on s, mapping an upstream path into a combine path
func (a *adjustment) do(s string) (string, error) {
	absPath := join(a.mountpoint, s)
	if a.root == "" {
		return absPath, nil
	}
	if absPath == a.root {
		return "", nil
	}
	if !strings.HasPrefix(absPath, a.rootSlash) {
		return "", errNotUnderRoot
	}
	return absPath[len(a.rootSlash):], nil
}

// undo makes the adjustment on s, mapping a combine path into an upstream path
func (a *adjustment) undo(s string) (string, error) {
	absPath := join(a.root, s)
	if absPath == a.mountpoint {
		return "", nil
	}
	if !strings.HasPrefix(absPath, a.mountpointSlash) {
		return "", errNotUnderRoot
	}
	return absPath[len(a.mountpointSlash):], nil
}

// upstream represents an upstream Fs
type upstream struct {
	f              fs.Fs
	parent         *Fs
	dir            string     // directory the upstream is mounted
	pathAdjustment adjustment // how to fiddle with the path
}

// Create an upstream from the directory it is mounted on and the remote
func (f *Fs) newUpstream(ctx context.Context, dir, remote string) (*upstream, error) {
	uFs, err := cache.Get(ctx, remote)
	if err == fs.ErrorIsFile {
		return nil, fmt.Errorf("can't combine files yet, only directories %q: %w", remote, err)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to create upstream %q: %w", remote, err)
	}
	u := &upstream{
		f:              uFs,
		parent:         f,
		dir:            dir,
		pathAdjustment: newAdjustment(f.root, dir),
	}
	return u, nil
}

// NewFs constructs an Fs from the path.
//
// The returned Fs is the actual Fs, referenced by remote in the config
func NewFs(ctx context.Context, name, root string, m configmap.Mapper) (outFs fs.Fs, err error) {
	// defer log.Trace(nil, "name=%q, root=%q, m=%v", name, root, m)("f=%+v, err=%v", &outFs, &err)
	// Parse config into Options struct
	opt := new(Options)
	err = configstruct.Set(m, opt)
	if err != nil {
		return nil, err
	}
	// Backward compatible to old config
	if len(opt.Upstreams) == 0 {
		return nil, errors.New("combine can't point to an empty upstream - check the value of the upstreams setting")
	}
	for _, u := range opt.Upstreams {
		if strings.HasPrefix(u, name+":") {
			return nil, errors.New("can't point combine remote at itself - check the value of the upstreams setting")
		}
	}
	isDir := false
	for strings.HasSuffix(root, "/") {
		root = root[:len(root)-1]
		isDir = true
	}

	f := &Fs{
		name:      name,
		root:      root,
		opt:       *opt,
		upstreams: make(map[string]*upstream, len(opt.Upstreams)),
		when:      time.Now(),
	}

	g, gCtx := errgroup.WithContext(ctx)
	var mu sync.Mutex
	for _, upstream := range opt.Upstreams {
		upstream := upstream
		g.Go(func() (err error) {
			equal := strings.IndexRune(upstream, '=')
			if equal < 0 {
				return fmt.Errorf("no \"=\" in upstream definition %q", upstream)
			}
			dir, remote := upstream[:equal], upstream[equal+1:]
			if dir == "" {
				return fmt.Errorf("empty dir in upstream definition %q", upstream)
			}
			if remote == "" {
				return fmt.Errorf("empty remote in upstream definition %q", upstream)
			}
			if strings.ContainsRune(dir, '/') {
				return fmt.Errorf("dirs can't contain / (yet): %q", dir)
			}
			u, err := f.newUpstream(gCtx, dir, remote)
			if err != nil {
				return err
			}
			mu.Lock()
			f.upstreams[dir] = u
			mu.Unlock()
			return nil
		})
	}
	err = g.Wait()
	if err != nil {
		return nil, err
	}
	// check features
	var features = (&fs.Features{
		CaseInsensitive:         true,
		DuplicateFiles:          false,
		ReadMimeType:            true,
		WriteMimeType:           true,
		CanHaveEmptyDirectories: true,
		BucketBased:             true,
		SetTier:                 true,
		GetTier:                 true,
	}).Fill(ctx, f)
	canMove := true
	for _, u := range f.upstreams {
		features = features.Mask(ctx, u.f) // Mask all upstream fs
		if !operations.CanServerSideMove(u.f) {
			canMove = false
		}
	}
	// We can move if all remotes support Move or Copy
	if canMove {
		features.Move = f.Move
	}

	// Enable ListR when upstreams either support ListR or is local
	// But not when all upstreams are local
	if features.ListR == nil {
		for _, u := range f.upstreams {
			if u.f.Features().ListR != nil {
				features.ListR = f.ListR
			} else if !u.f.Features().IsLocal {
				features.ListR = nil
				break
			}
		}
	}

	// Enable Purge when any upstreams support it
	if features.Purge == nil {
		for _, u := range f.upstreams {
			if u.f.Features().Purge != nil {
				features.Purge = f.Purge
				break
			}
		}
	}

	// Enable Shutdown when any upstreams support it
	if features.Shutdown == nil {
		for _, u := range f.upstreams {
			if u.f.Features().Shutdown != nil {
				features.Shutdown = f.Shutdown
				break
			}
		}
	}

	// Enable DirCacheFlush when any upstreams support it
	if features.DirCacheFlush == nil {
		for _, u := range f.upstreams {
			if u.f.Features().DirCacheFlush != nil {
				features.DirCacheFlush = f.DirCacheFlush
				break
			}
		}
	}

	// Enable ChangeNotify when any upstreams support it
	if features.ChangeNotify == nil {
		for _, u := range f.upstreams {
			if u.f.Features().ChangeNotify != nil {
				features.ChangeNotify = f.ChangeNotify
				break
			}
		}
	}

	f.features = features

	// Get common intersection of hashes
	var hashSet hash.Set
	var first = true
	for _, u := range f.upstreams {
		if first {
			hashSet = u.f.Hashes()
			first = false
		} else {
			hashSet = hashSet.Overlap(u.f.Hashes())
		}
	}
	f.hashSet = hashSet

	// Check to see if the root is actually a file
	if f.root != "" && !isDir {
		_, err := f.NewObject(ctx, "")
		if err != nil {
			if err == fs.ErrorObjectNotFound || err == fs.ErrorNotAFile || err == fs.ErrorIsDir {
				// File doesn't exist or is a directory so return old f
				return f, nil
			}
			return nil, err
		}

		// Check to see if the root path is actually an existing file
		f.root = path.Dir(f.root)
		if f.root == "." {
			f.root = ""
		}
		// Adjust path adjustment to remove leaf
		for _, u := range f.upstreams {
			u.pathAdjustment = newAdjustment(f.root, u.dir)
		}
		return f, fs.ErrorIsFile
	}
	return f, nil
}

// Run a function over all the upstreams in parallel
func (f *Fs) multithread(ctx context.Context, fn func(context.Context, *upstream) error) error {
	g, gCtx := errgroup.WithContext(ctx)
	for _, u := range f.upstreams {
		u := u
		g.Go(func() (err error) {
			return fn(gCtx, u)
		})
	}
	return g.Wait()
}

// join the elements together but unline path.Join return empty string
func join(elem ...string) string {
	result := path.Join(elem...)
	if result == "." {
		return ""
	}
	if len(result) > 0 && result[0] == '/' {
		result = result[1:]
	}
	return result
}

// find the upstream for the remote passed in, returning the upstream and the adjusted path
func (f *Fs) findUpstream(remote string) (u *upstream, uRemote string, err error) {
	// defer log.Trace(remote, "")("f=%v, uRemote=%q, err=%v", &u, &uRemote, &err)
	for _, u := range f.upstreams {
		uRemote, err = u.pathAdjustment.undo(remote)
		if err == nil {
			return u, uRemote, nil
		}
	}
	return nil, "", fmt.Errorf("combine for remote %q: %w", remote, fs.ErrorDirNotFound)
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
	return fmt.Sprintf("combine root '%s'", f.root)
}

// Features returns the optional features of this Fs
func (f *Fs) Features() *fs.Features {
	return f.features
}

// Rmdir removes the root directory of the Fs object
func (f *Fs) Rmdir(ctx context.Context, dir string) error {
	// The root always exists
	if f.root == "" && dir == "" {
		return nil
	}
	u, uRemote, err := f.findUpstream(dir)
	if err != nil {
		return err
	}
	return u.f.Rmdir(ctx, uRemote)
}

// Hashes returns hash.HashNone to indicate remote hashing is unavailable
func (f *Fs) Hashes() hash.Set {
	return f.hashSet
}

// Mkdir makes the root directory of the Fs object
func (f *Fs) Mkdir(ctx context.Context, dir string) error {
	// The root always exists
	if f.root == "" && dir == "" {
		return nil
	}
	u, uRemote, err := f.findUpstream(dir)
	if err != nil {
		return err
	}
	return u.f.Mkdir(ctx, uRemote)
}

// purge the upstream or fallback to a slow way
func (u *upstream) purge(ctx context.Context, dir string) (err error) {
	if do := u.f.Features().Purge; do != nil {
		err = do(ctx, dir)
	} else {
		err = operations.Purge(ctx, u.f, dir)
	}
	return err
}

// Purge all files in the directory
//
// Implement this if you have a way of deleting all the files
// quicker than just running Remove() on the result of List()
//
// Return an error if it doesn't exist
func (f *Fs) Purge(ctx context.Context, dir string) error {
	if f.root == "" && dir == "" {
		return f.multithread(ctx, func(ctx context.Context, u *upstream) error {
			return u.purge(ctx, "")
		})
	}
	u, uRemote, err := f.findUpstream(dir)
	if err != nil {
		return err
	}
	return u.purge(ctx, uRemote)
}

// Copy src to this remote using server-side copy operations.
//
// This is stored with the remote path given
//
// It returns the destination Object and a possible error
//
// Will only be called if src.Fs().Name() == f.Name()
//
// If it isn't possible then return fs.ErrorCantCopy
func (f *Fs) Copy(ctx context.Context, src fs.Object, remote string) (fs.Object, error) {
	srcObj, ok := src.(*Object)
	if !ok {
		fs.Debugf(src, "Can't copy - not same remote type")
		return nil, fs.ErrorCantCopy
	}

	dstU, dstRemote, err := f.findUpstream(remote)
	if err != nil {
		return nil, err
	}

	do := dstU.f.Features().Copy
	if do == nil {
		return nil, fs.ErrorCantCopy
	}

	o, err := do(ctx, srcObj.Object, dstRemote)
	if err != nil {
		return nil, err
	}

	return dstU.newObject(o), nil
}

// Move src to this remote using server-side move operations.
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

	dstU, dstRemote, err := f.findUpstream(remote)
	if err != nil {
		return nil, err
	}

	do := dstU.f.Features().Move
	useCopy := false
	if do == nil {
		do = dstU.f.Features().Copy
		if do == nil {
			return nil, fs.ErrorCantMove
		}
		useCopy = true
	}

	o, err := do(ctx, srcObj.Object, dstRemote)
	if err != nil {
		return nil, err
	}

	// If did Copy then remove the source object
	if useCopy {
		err = srcObj.Remove(ctx)
		if err != nil {
			return nil, err
		}
	}

	return dstU.newObject(o), nil
}

// DirMove moves src, srcRemote to this remote at dstRemote
// using server-side move operations.
//
// Will only be called if src.Fs().Name() == f.Name()
//
// If it isn't possible then return fs.ErrorCantDirMove
//
// If destination exists then return fs.ErrorDirExists
func (f *Fs) DirMove(ctx context.Context, src fs.Fs, srcRemote, dstRemote string) (err error) {
	// defer log.Trace(f, "src=%v, srcRemote=%q, dstRemote=%q", src, srcRemote, dstRemote)("err=%v", &err)
	srcFs, ok := src.(*Fs)
	if !ok {
		fs.Debugf(src, "Can't move directory - not same remote type")
		return fs.ErrorCantDirMove
	}

	dstU, dstURemote, err := f.findUpstream(dstRemote)
	if err != nil {
		return err
	}

	srcU, srcURemote, err := srcFs.findUpstream(srcRemote)
	if err != nil {
		return err
	}

	do := dstU.f.Features().DirMove
	if do == nil {
		return fs.ErrorCantDirMove
	}

	fs.Logf(dstU.f, "srcU.f=%v, srcURemote=%q, dstURemote=%q", srcU.f, srcURemote, dstURemote)
	return do(ctx, srcU.f, srcURemote, dstURemote)
}

// ChangeNotify calls the passed function with a path
// that has had changes. If the implementation
// uses polling, it should adhere to the given interval.
// At least one value will be written to the channel,
// specifying the initial value and updated values might
// follow. A 0 Duration should pause the polling.
// The ChangeNotify implementation must empty the channel
// regularly. When the channel gets closed, the implementation
// should stop polling and release resources.
func (f *Fs) ChangeNotify(ctx context.Context, notifyFunc func(string, fs.EntryType), ch <-chan time.Duration) {
	var uChans []chan time.Duration

	for _, u := range f.upstreams {
		u := u
		if do := u.f.Features().ChangeNotify; do != nil {
			ch := make(chan time.Duration)
			uChans = append(uChans, ch)
			wrappedNotifyFunc := func(path string, entryType fs.EntryType) {
				newPath, err := u.pathAdjustment.do(path)
				if err != nil {
					fs.Logf(f, "ChangeNotify: unable to process %q: %s", path, err)
					return
				}
				fs.Debugf(f, "ChangeNotify: path %q entryType %d", newPath, entryType)
				notifyFunc(newPath, entryType)
			}
			do(ctx, wrappedNotifyFunc, ch)
		}
	}

	go func() {
		for i := range ch {
			for _, c := range uChans {
				c <- i
			}
		}
		for _, c := range uChans {
			close(c)
		}
	}()
}

// DirCacheFlush resets the directory cache - used in testing
// as an optional interface
func (f *Fs) DirCacheFlush() {
	ctx := context.Background()
	_ = f.multithread(ctx, func(ctx context.Context, u *upstream) error {
		if do := u.f.Features().DirCacheFlush; do != nil {
			do()
		}
		return nil
	})
}

func (f *Fs) put(ctx context.Context, in io.Reader, src fs.ObjectInfo, stream bool, options ...fs.OpenOption) (fs.Object, error) {
	srcPath := src.Remote()
	u, uRemote, err := f.findUpstream(srcPath)
	if err != nil {
		return nil, err
	}
	uSrc := operations.NewOverrideRemote(src, uRemote)
	var o fs.Object
	if stream {
		o, err = u.f.Features().PutStream(ctx, in, uSrc, options...)
	} else {
		o, err = u.f.Put(ctx, in, uSrc, options...)
	}
	if err != nil {
		return nil, err
	}
	return u.newObject(o), nil
}

// Put in to the remote path with the modTime given of the given size
//
// May create the object even if it returns an error - if so
// will return the object and the error, otherwise will return
// nil and the error
func (f *Fs) Put(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error) {
	o, err := f.NewObject(ctx, src.Remote())
	switch err {
	case nil:
		return o, o.Update(ctx, in, src, options...)
	case fs.ErrorObjectNotFound:
		return f.put(ctx, in, src, false, options...)
	default:
		return nil, err
	}
}

// PutStream uploads to the remote path with the modTime given of indeterminate size
//
// May create the object even if it returns an error - if so
// will return the object and the error, otherwise will return
// nil and the error
func (f *Fs) PutStream(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error) {
	o, err := f.NewObject(ctx, src.Remote())
	switch err {
	case nil:
		return o, o.Update(ctx, in, src, options...)
	case fs.ErrorObjectNotFound:
		return f.put(ctx, in, src, true, options...)
	default:
		return nil, err
	}
}

// About gets quota information from the Fs
func (f *Fs) About(ctx context.Context) (*fs.Usage, error) {
	usage := &fs.Usage{
		Total:   new(int64),
		Used:    new(int64),
		Trashed: new(int64),
		Other:   new(int64),
		Free:    new(int64),
		Objects: new(int64),
	}
	for _, u := range f.upstreams {
		doAbout := u.f.Features().About
		if doAbout == nil {
			continue
		}
		usg, err := doAbout(ctx)
		if errors.Is(err, fs.ErrorDirNotFound) {
			continue
		}
		if err != nil {
			return nil, err
		}
		if usg.Total != nil && usage.Total != nil {
			*usage.Total += *usg.Total
		} else {
			usage.Total = nil
		}
		if usg.Used != nil && usage.Used != nil {
			*usage.Used += *usg.Used
		} else {
			usage.Used = nil
		}
		if usg.Trashed != nil && usage.Trashed != nil {
			*usage.Trashed += *usg.Trashed
		} else {
			usage.Trashed = nil
		}
		if usg.Other != nil && usage.Other != nil {
			*usage.Other += *usg.Other
		} else {
			usage.Other = nil
		}
		if usg.Free != nil && usage.Free != nil {
			*usage.Free += *usg.Free
		} else {
			usage.Free = nil
		}
		if usg.Objects != nil && usage.Objects != nil {
			*usage.Objects += *usg.Objects
		} else {
			usage.Objects = nil
		}
	}
	return usage, nil
}

// Wraps entries for this upstream
func (u *upstream) wrapEntries(ctx context.Context, entries fs.DirEntries) (fs.DirEntries, error) {
	for i, entry := range entries {
		switch x := entry.(type) {
		case fs.Object:
			entries[i] = u.newObject(x)
		case fs.Directory:
			newDir := fs.NewDirCopy(ctx, x)
			newPath, err := u.pathAdjustment.do(newDir.Remote())
			if err != nil {
				return nil, err
			}
			newDir.SetRemote(newPath)
			entries[i] = newDir
		default:
			return nil, fmt.Errorf("unknown entry type %T", entry)
		}
	}
	return entries, nil
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
	// defer log.Trace(f, "dir=%q", dir)("entries = %v, err=%v", &entries, &err)
	if f.root == "" && dir == "" {
		entries = make(fs.DirEntries, 0, len(f.upstreams))
		for combineDir := range f.upstreams {
			d := fs.NewDir(combineDir, f.when)
			entries = append(entries, d)
		}
		return entries, nil
	}
	u, uRemote, err := f.findUpstream(dir)
	if err != nil {
		return nil, err
	}
	entries, err = u.f.List(ctx, uRemote)
	if err != nil {
		return nil, err
	}
	return u.wrapEntries(ctx, entries)
}

// ListR lists the objects and directories of the Fs starting
// from dir recursively into out.
//
// dir should be "" to start from the root, and should not
// have trailing slashes.
//
// This should return ErrDirNotFound if the directory isn't
// found.
//
// It should call callback for each tranche of entries read.
// These need not be returned in any particular order.  If
// callback returns an error then the listing will stop
// immediately.
//
// Don't implement this unless you have a more efficient way
// of listing recursively that doing a directory traversal.
func (f *Fs) ListR(ctx context.Context, dir string, callback fs.ListRCallback) (err error) {
	// defer log.Trace(f, "dir=%q, callback=%v", dir, callback)("err=%v", &err)
	if f.root == "" && dir == "" {
		rootEntries, err := f.List(ctx, "")
		if err != nil {
			return err
		}
		err = callback(rootEntries)
		if err != nil {
			return err
		}
		var mu sync.Mutex
		syncCallback := func(entries fs.DirEntries) error {
			mu.Lock()
			defer mu.Unlock()
			return callback(entries)
		}
		err = f.multithread(ctx, func(ctx context.Context, u *upstream) error {
			return f.ListR(ctx, u.dir, syncCallback)
		})
		if err != nil {
			return err
		}
		return nil
	}
	u, uRemote, err := f.findUpstream(dir)
	if err != nil {
		return err
	}
	wrapCallback := func(entries fs.DirEntries) error {
		entries, err := u.wrapEntries(ctx, entries)
		if err != nil {
			return err
		}
		return callback(entries)
	}
	if do := u.f.Features().ListR; do != nil {
		err = do(ctx, uRemote, wrapCallback)
	} else {
		err = walk.ListR(ctx, u.f, uRemote, true, -1, walk.ListAll, wrapCallback)
	}
	if err == fs.ErrorDirNotFound {
		err = nil
	}
	return err
}

// NewObject creates a new remote combine file object
func (f *Fs) NewObject(ctx context.Context, remote string) (fs.Object, error) {
	u, uRemote, err := f.findUpstream(remote)
	if err != nil {
		return nil, err
	}
	if uRemote == "" || strings.HasSuffix(uRemote, "/") {
		return nil, fs.ErrorIsDir
	}
	o, err := u.f.NewObject(ctx, uRemote)
	if err != nil {
		return nil, err
	}
	return u.newObject(o), nil
}

// Precision is the greatest Precision of all upstreams
func (f *Fs) Precision() time.Duration {
	var greatestPrecision time.Duration
	for _, u := range f.upstreams {
		uPrecision := u.f.Precision()
		if uPrecision > greatestPrecision {
			greatestPrecision = uPrecision
		}
	}
	return greatestPrecision
}

// Shutdown the backend, closing any background tasks and any
// cached connections.
func (f *Fs) Shutdown(ctx context.Context) error {
	return f.multithread(ctx, func(ctx context.Context, u *upstream) error {
		if do := u.f.Features().Shutdown; do != nil {
			return do(ctx)
		}
		return nil
	})
}

// Object describes a wrapped Object
//
// This is a wrapped Object which knows its path prefix
type Object struct {
	fs.Object
	u *upstream
}

func (u *upstream) newObject(o fs.Object) *Object {
	return &Object{
		Object: o,
		u:      u,
	}
}

// Fs returns read only access to the Fs that this object is part of
func (o *Object) Fs() fs.Info {
	return o.u.parent
}

// String returns the remote path
func (o *Object) String() string {
	return o.Remote()
}

// Remote returns the remote path
func (o *Object) Remote() string {
	newPath, err := o.u.pathAdjustment.do(o.Object.String())
	if err != nil {
		fs.Errorf(o, "Bad object: %v", err)
		return err.Error()
	}
	return newPath
}

// MimeType returns the content type of the Object if known
func (o *Object) MimeType(ctx context.Context) (mimeType string) {
	if do, ok := o.Object.(fs.MimeTyper); ok {
		mimeType = do.MimeType(ctx)
	}
	return mimeType
}

// UnWrap returns the Object that this Object is wrapping or
// nil if it isn't wrapping anything
func (o *Object) UnWrap() fs.Object {
	return o.Object
}

// Check the interfaces are satisfied
var (
	_ fs.Fs              = (*Fs)(nil)
	_ fs.Purger          = (*Fs)(nil)
	_ fs.PutStreamer     = (*Fs)(nil)
	_ fs.Copier          = (*Fs)(nil)
	_ fs.Mover           = (*Fs)(nil)
	_ fs.DirMover        = (*Fs)(nil)
	_ fs.DirCacheFlusher = (*Fs)(nil)
	_ fs.ChangeNotifier  = (*Fs)(nil)
	_ fs.Abouter         = (*Fs)(nil)
	_ fs.ListRer         = (*Fs)(nil)
	_ fs.Shutdowner      = (*Fs)(nil)
)
