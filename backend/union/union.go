package union

import (
	"context"
	"fmt"
	"io"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/pkg/errors"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/cache"
	"github.com/rclone/rclone/fs/config/configmap"
	"github.com/rclone/rclone/fs/config/configstruct"
	"github.com/rclone/rclone/fs/hash"
)

// Register with Fs
func init() {
	fsi := &fs.RegInfo{
		Name:        "union",
		Description: "Union merges the contents of several remotes",
		NewFs:       NewFs,
		Options: []fs.Option{{
			Name:     "remotes",
			Help:     "List of space separated remotes.\nCan be 'remotea:test/dir remoteb:', '\"remotea:test/space dir\" remoteb:', etc.\nThe last remote is used to write to.",
			Required: true,
		}},
	}
	fs.Register(fsi)
}

// Options defines the configuration for this backend
type Options struct {
	Remotes fs.SpaceSepList `config:"remotes"`
}

// Fs represents a union of remotes
type Fs struct {
	name     string       // name of this remote
	features *fs.Features // optional features
	opt      Options      // options for this Fs
	root     string       // the path we are working on
	remotes  []fs.Fs      // slice of remotes
	wr       fs.Fs        // writable remote
	hashSet  hash.Set     // intersection of hash types
}

// Object describes a union Object
//
// This is a wrapped object which returns the Union Fs as its parent
type Object struct {
	fs.Object
	fs *Fs // what this object is part of
}

// Wrap an existing object in the union Object
func (f *Fs) wrapObject(o fs.Object) *Object {
	return &Object{
		Object: o,
		fs:     f,
	}
}

// Fs returns the union Fs as the parent
func (o *Object) Fs() fs.Info {
	return o.fs
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
	return fmt.Sprintf("union root '%s'", f.root)
}

// Features returns the optional features of this Fs
func (f *Fs) Features() *fs.Features {
	return f.features
}

// Rmdir removes the root directory of the Fs object
func (f *Fs) Rmdir(ctx context.Context, dir string) error {
	return f.wr.Rmdir(ctx, dir)
}

// Hashes returns hash.HashNone to indicate remote hashing is unavailable
func (f *Fs) Hashes() hash.Set {
	return f.hashSet
}

// Mkdir makes the root directory of the Fs object
func (f *Fs) Mkdir(ctx context.Context, dir string) error {
	return f.wr.Mkdir(ctx, dir)
}

// Purge all files in the root and the root directory
//
// Implement this if you have a way of deleting all the files
// quicker than just running Remove() on the result of List()
//
// Return an error if it doesn't exist
func (f *Fs) Purge(ctx context.Context) error {
	return f.wr.Features().Purge(ctx)
}

// Copy src to this remote using server side copy operations.
//
// This is stored with the remote path given
//
// It returns the destination Object and a possible error
//
// Will only be called if src.Fs().Name() == f.Name()
//
// If it isn't possible then return fs.ErrorCantCopy
func (f *Fs) Copy(ctx context.Context, src fs.Object, remote string) (fs.Object, error) {
	if src.Fs() != f.wr {
		fs.Debugf(src, "Can't copy - not same remote type")
		return nil, fs.ErrorCantCopy
	}
	o, err := f.wr.Features().Copy(ctx, src, remote)
	if err != nil {
		return nil, err
	}
	return f.wrapObject(o), nil
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
	if src.Fs() != f.wr {
		fs.Debugf(src, "Can't move - not same remote type")
		return nil, fs.ErrorCantMove
	}
	o, err := f.wr.Features().Move(ctx, src, remote)
	if err != nil {
		return nil, err
	}
	return f.wrapObject(o), err
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
	return f.wr.Features().DirMove(ctx, srcFs.wr, srcRemote, dstRemote)
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
func (f *Fs) ChangeNotify(ctx context.Context, fn func(string, fs.EntryType), ch <-chan time.Duration) {
	var remoteChans []chan time.Duration

	for _, remote := range f.remotes {
		if ChangeNotify := remote.Features().ChangeNotify; ChangeNotify != nil {
			ch := make(chan time.Duration)
			remoteChans = append(remoteChans, ch)
			ChangeNotify(ctx, fn, ch)
		}
	}

	go func() {
		for i := range ch {
			for _, c := range remoteChans {
				c <- i
			}
		}
		for _, c := range remoteChans {
			close(c)
		}
	}()
}

// DirCacheFlush resets the directory cache - used in testing
// as an optional interface
func (f *Fs) DirCacheFlush() {
	for _, remote := range f.remotes {
		if DirCacheFlush := remote.Features().DirCacheFlush; DirCacheFlush != nil {
			DirCacheFlush()
		}
	}
}

// PutStream uploads to the remote path with the modTime given of indeterminate size
//
// May create the object even if it returns an error - if so
// will return the object and the error, otherwise will return
// nil and the error
func (f *Fs) PutStream(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error) {
	o, err := f.wr.Features().PutStream(ctx, in, src, options...)
	if err != nil {
		return nil, err
	}
	return f.wrapObject(o), err
}

// About gets quota information from the Fs
func (f *Fs) About(ctx context.Context) (*fs.Usage, error) {
	return f.wr.Features().About(ctx)
}

// Put in to the remote path with the modTime given of the given size
//
// May create the object even if it returns an error - if so
// will return the object and the error, otherwise will return
// nil and the error
func (f *Fs) Put(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error) {
	o, err := f.wr.Put(ctx, in, src, options...)
	if err != nil {
		return nil, err
	}
	return f.wrapObject(o), err
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
	set := make(map[string]fs.DirEntry)
	found := false
	for _, remote := range f.remotes {
		var remoteEntries, err = remote.List(ctx, dir)
		if err == fs.ErrorDirNotFound {
			continue
		}
		if err != nil {
			return nil, errors.Wrapf(err, "List failed on %v", remote)
		}
		found = true
		for _, remoteEntry := range remoteEntries {
			set[remoteEntry.Remote()] = remoteEntry
		}
	}
	if !found {
		return nil, fs.ErrorDirNotFound
	}
	for _, entry := range set {
		if o, ok := entry.(fs.Object); ok {
			entry = f.wrapObject(o)
		}
		entries = append(entries, entry)
	}
	return entries, nil
}

// NewObject creates a new remote union file object based on the first Object it finds (reverse remote order)
func (f *Fs) NewObject(ctx context.Context, path string) (fs.Object, error) {
	for i := range f.remotes {
		var remote = f.remotes[len(f.remotes)-i-1]
		var obj, err = remote.NewObject(ctx, path)
		if err == fs.ErrorObjectNotFound {
			continue
		}
		if err != nil {
			return nil, errors.Wrapf(err, "NewObject failed on %v", remote)
		}
		return f.wrapObject(obj), nil
	}
	return nil, fs.ErrorObjectNotFound
}

// Precision is the greatest Precision of all remotes
func (f *Fs) Precision() time.Duration {
	var greatestPrecision time.Duration
	for _, remote := range f.remotes {
		if remote.Precision() > greatestPrecision {
			greatestPrecision = remote.Precision()
		}
	}
	return greatestPrecision
}

// NewFs constructs an Fs from the path.
//
// The returned Fs is the actual Fs, referenced by remote in the config
func NewFs(name, root string, m configmap.Mapper) (fs.Fs, error) {
	// Parse config into Options struct
	opt := new(Options)
	err := configstruct.Set(m, opt)
	if err != nil {
		return nil, err
	}
	if len(opt.Remotes) == 0 {
		return nil, errors.New("union can't point to an empty remote - check the value of the remotes setting")
	}
	if len(opt.Remotes) == 1 {
		return nil, errors.New("union can't point to a single remote - check the value of the remotes setting")
	}
	for _, remote := range opt.Remotes {
		if strings.HasPrefix(remote, name+":") {
			return nil, errors.New("can't point union remote at itself - check the value of the remote setting")
		}
	}

	var remotes []fs.Fs
	for i := range opt.Remotes {
		// Last remote first so we return the correct (last) matching fs in case of fs.ErrorIsFile
		var remote = opt.Remotes[len(opt.Remotes)-i-1]
		_, configName, fsPath, err := fs.ParseRemote(remote)
		if err != nil {
			return nil, err
		}
		var rootString = path.Join(fsPath, filepath.ToSlash(root))
		if configName != "local" {
			rootString = configName + ":" + rootString
		}
		myFs, err := cache.Get(rootString)
		if err != nil {
			if err == fs.ErrorIsFile {
				return myFs, err
			}
			return nil, err
		}
		remotes = append(remotes, myFs)
	}

	// Reverse the remotes again so they are in the order as before
	for i, j := 0, len(remotes)-1; i < j; i, j = i+1, j-1 {
		remotes[i], remotes[j] = remotes[j], remotes[i]
	}

	f := &Fs{
		name:    name,
		root:    root,
		opt:     *opt,
		remotes: remotes,
		wr:      remotes[len(remotes)-1],
	}
	var features = (&fs.Features{
		CaseInsensitive:         true,
		DuplicateFiles:          false,
		ReadMimeType:            true,
		WriteMimeType:           true,
		CanHaveEmptyDirectories: true,
		BucketBased:             true,
		SetTier:                 true,
		GetTier:                 true,
	}).Fill(f)
	features = features.Mask(f.wr) // mask the features just on the writable fs

	// Really need the union of all remotes for these, so
	// re-instate and calculate separately.
	features.ChangeNotify = f.ChangeNotify
	features.DirCacheFlush = f.DirCacheFlush

	// FIXME maybe should be masking the bools here?

	// Clear ChangeNotify and DirCacheFlush if all are nil
	clearChangeNotify := true
	clearDirCacheFlush := true
	for _, remote := range f.remotes {
		remoteFeatures := remote.Features()
		if remoteFeatures.ChangeNotify != nil {
			clearChangeNotify = false
		}
		if remoteFeatures.DirCacheFlush != nil {
			clearDirCacheFlush = false
		}
	}
	if clearChangeNotify {
		features.ChangeNotify = nil
	}
	if clearDirCacheFlush {
		features.DirCacheFlush = nil
	}

	f.features = features

	// Get common intersection of hashes
	hashSet := f.remotes[0].Hashes()
	for _, remote := range f.remotes[1:] {
		hashSet = hashSet.Overlap(remote.Hashes())
	}
	f.hashSet = hashSet

	return f, nil
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
)
