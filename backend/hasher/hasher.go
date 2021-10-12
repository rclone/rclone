// Package hasher implements a checksum handling overlay backend
package hasher

import (
	"context"
	"encoding/gob"
	"fmt"
	"io"
	"path"
	"strings"
	"sync"
	"time"

	"github.com/pkg/errors"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/cache"
	"github.com/rclone/rclone/fs/config/configmap"
	"github.com/rclone/rclone/fs/config/configstruct"
	"github.com/rclone/rclone/fs/fspath"
	"github.com/rclone/rclone/fs/hash"
	"github.com/rclone/rclone/lib/kv"
)

// Register with Fs
func init() {
	fs.Register(&fs.RegInfo{
		Name:        "hasher",
		Description: "Better checksums for other remotes",
		NewFs:       NewFs,
		CommandHelp: commandHelp,
		Options: []fs.Option{{
			Name:     "remote",
			Required: true,
			Help:     "Remote to cache checksums for (e.g. myRemote:path).",
		}, {
			Name:     "hashes",
			Default:  fs.CommaSepList{"md5", "sha1"},
			Advanced: false,
			Help:     "Comma separated list of supported checksum types.",
		}, {
			Name:     "max_age",
			Advanced: false,
			Default:  fs.DurationOff,
			Help:     "Maximum time to keep checksums in cache (0 = no cache, off = cache forever).",
		}, {
			Name:     "auto_size",
			Advanced: true,
			Default:  fs.SizeSuffix(0),
			Help:     "Auto-update checksum for files smaller than this size (disabled by default).",
		}},
	})
}

// Options defines the configuration for this backend
type Options struct {
	Remote   string          `config:"remote"`
	Hashes   fs.CommaSepList `config:"hashes"`
	AutoSize fs.SizeSuffix   `config:"auto_size"`
	MaxAge   fs.Duration     `config:"max_age"`
}

// Fs represents a wrapped fs.Fs
type Fs struct {
	fs.Fs
	name     string
	root     string
	wrapper  fs.Fs
	features *fs.Features
	opt      *Options
	db       *kv.DB
	// fingerprinting
	fpTime bool      // true if using time in fingerprints
	fpHash hash.Type // hash type to use in fingerprints or None
	// hash types triaged by groups
	suppHashes hash.Set // all supported checksum types
	passHashes hash.Set // passed directly to the base without caching
	slowHashes hash.Set // passed to the base and then cached
	autoHashes hash.Set // calculated in-house and cached
	keepHashes hash.Set // checksums to keep in cache (slow + auto)
}

var warnExperimental sync.Once

// NewFs constructs an Fs from the remote:path string
func NewFs(ctx context.Context, fsname, rpath string, cmap configmap.Mapper) (fs.Fs, error) {
	if !kv.Supported() {
		return nil, errors.New("hasher is not supported on this OS")
	}
	warnExperimental.Do(func() {
		fs.Infof(nil, "Hasher is EXPERIMENTAL!")
	})

	opt := &Options{}
	err := configstruct.Set(cmap, opt)
	if err != nil {
		return nil, err
	}

	if strings.HasPrefix(opt.Remote, fsname+":") {
		return nil, errors.New("can't point remote at itself")
	}
	remotePath := fspath.JoinRootPath(opt.Remote, rpath)
	baseFs, err := cache.Get(ctx, remotePath)
	if err != nil && err != fs.ErrorIsFile {
		return nil, errors.Wrapf(err, "failed to derive base remote %q", opt.Remote)
	}

	f := &Fs{
		Fs:   baseFs,
		name: fsname,
		root: rpath,
		opt:  opt,
	}
	baseFeatures := baseFs.Features()
	f.fpTime = baseFs.Precision() != fs.ModTimeNotSupported

	if baseFeatures.SlowHash {
		f.slowHashes = f.Fs.Hashes()
	} else {
		f.passHashes = f.Fs.Hashes()
		f.fpHash = f.passHashes.GetOne()
	}

	f.suppHashes = f.passHashes
	f.suppHashes.Add(f.slowHashes.Array()...)

	for _, hashName := range opt.Hashes {
		var ht hash.Type
		if err := ht.Set(hashName); err != nil {
			return nil, errors.Errorf("invalid token %q in hash string %q", hashName, opt.Hashes.String())
		}
		if !f.slowHashes.Contains(ht) {
			f.autoHashes.Add(ht)
		}
		f.keepHashes.Add(ht)
		f.suppHashes.Add(ht)
	}

	fs.Debugf(f, "Groups by usage: cached %s, passed %s, auto %s, slow %s, supported %s",
		f.keepHashes, f.passHashes, f.autoHashes, f.slowHashes, f.suppHashes)

	var nilSet hash.Set
	if f.keepHashes == nilSet {
		return nil, errors.New("configured hash_names have nothing to keep in cache")
	}

	if f.opt.MaxAge > 0 {
		gob.Register(hashRecord{})
		db, err := kv.Start(ctx, "hasher", f.Fs)
		if err != nil {
			return nil, err
		}
		f.db = db
	}

	stubFeatures := &fs.Features{
		CanHaveEmptyDirectories: true,
		IsLocal:                 true,
		ReadMimeType:            true,
		WriteMimeType:           true,
	}
	f.features = stubFeatures.Fill(ctx, f).Mask(ctx, f.Fs).WrapsFs(f, f.Fs)

	cache.PinUntilFinalized(f.Fs, f)
	return f, err
}

//
// Filesystem
//

// Name of the remote (as passed into NewFs)
func (f *Fs) Name() string { return f.name }

// Root of the remote (as passed into NewFs)
func (f *Fs) Root() string { return f.root }

// Features returns the optional features of this Fs
func (f *Fs) Features() *fs.Features { return f.features }

// Hashes returns the supported hash sets.
func (f *Fs) Hashes() hash.Set { return f.suppHashes }

// String returns a description of the FS
// The "hasher::" prefix is a distinctive feature.
func (f *Fs) String() string {
	return fmt.Sprintf("hasher::%s:%s", f.name, f.root)
}

// UnWrap returns the Fs that this Fs is wrapping
func (f *Fs) UnWrap() fs.Fs { return f.Fs }

// WrapFs returns the Fs that is wrapping this Fs
func (f *Fs) WrapFs() fs.Fs { return f.wrapper }

// SetWrapper sets the Fs that is wrapping this Fs
func (f *Fs) SetWrapper(wrapper fs.Fs) { f.wrapper = wrapper }

// Wrap base entries into hasher entries.
func (f *Fs) wrapEntries(baseEntries fs.DirEntries) (hashEntries fs.DirEntries, err error) {
	hashEntries = baseEntries[:0] // work inplace
	for _, entry := range baseEntries {
		switch x := entry.(type) {
		case fs.Object:
			hashEntries = append(hashEntries, f.wrapObject(x, nil))
		default:
			hashEntries = append(hashEntries, entry) // trash in - trash out
		}
	}
	return hashEntries, nil
}

// List the objects and directories in dir into entries.
func (f *Fs) List(ctx context.Context, dir string) (entries fs.DirEntries, err error) {
	if entries, err = f.Fs.List(ctx, dir); err != nil {
		return nil, err
	}
	return f.wrapEntries(entries)
}

// ListR lists the objects and directories recursively into out.
func (f *Fs) ListR(ctx context.Context, dir string, callback fs.ListRCallback) (err error) {
	return f.Fs.Features().ListR(ctx, dir, func(baseEntries fs.DirEntries) error {
		hashEntries, err := f.wrapEntries(baseEntries)
		if err != nil {
			return err
		}
		return callback(hashEntries)
	})
}

// Purge a directory
func (f *Fs) Purge(ctx context.Context, dir string) error {
	if do := f.Fs.Features().Purge; do != nil {
		if err := do(ctx, dir); err != nil {
			return err
		}
		err := f.db.Do(true, &kvPurge{
			dir: path.Join(f.Fs.Root(), dir),
		})
		if err != nil {
			fs.Errorf(f, "Failed to purge some hashes: %v", err)
		}
		return nil
	}
	return fs.ErrorCantPurge
}

// PutStream uploads to the remote path with undeterminate size.
func (f *Fs) PutStream(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error) {
	if do := f.Fs.Features().PutStream; do != nil {
		_ = f.pruneHash(src.Remote())
		oResult, err := do(ctx, in, src, options...)
		return f.wrapObject(oResult, err), err
	}
	return nil, errors.New("PutStream not supported")
}

// PutUnchecked uploads the object, allowing duplicates.
func (f *Fs) PutUnchecked(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error) {
	if do := f.Fs.Features().PutUnchecked; do != nil {
		_ = f.pruneHash(src.Remote())
		oResult, err := do(ctx, in, src, options...)
		return f.wrapObject(oResult, err), err
	}
	return nil, errors.New("PutUnchecked not supported")
}

// pruneHash deletes hash for a path
func (f *Fs) pruneHash(remote string) error {
	return f.db.Do(true, &kvPrune{
		key: path.Join(f.Fs.Root(), remote),
	})
}

// CleanUp the trash in the Fs
func (f *Fs) CleanUp(ctx context.Context) error {
	if do := f.Fs.Features().CleanUp; do != nil {
		return do(ctx)
	}
	return errors.New("CleanUp not supported")
}

// About gets quota information from the Fs
func (f *Fs) About(ctx context.Context) (*fs.Usage, error) {
	if do := f.Fs.Features().About; do != nil {
		return do(ctx)
	}
	return nil, errors.New("About not supported")
}

// ChangeNotify calls the passed function with a path that has had changes.
func (f *Fs) ChangeNotify(ctx context.Context, notifyFunc func(string, fs.EntryType), pollIntervalChan <-chan time.Duration) {
	if do := f.Fs.Features().ChangeNotify; do != nil {
		do(ctx, notifyFunc, pollIntervalChan)
	}
}

// UserInfo returns info about the connected user
func (f *Fs) UserInfo(ctx context.Context) (map[string]string, error) {
	if do := f.Fs.Features().UserInfo; do != nil {
		return do(ctx)
	}
	return nil, fs.ErrorNotImplemented
}

// Disconnect the current user
func (f *Fs) Disconnect(ctx context.Context) error {
	if do := f.Fs.Features().Disconnect; do != nil {
		return do(ctx)
	}
	return fs.ErrorNotImplemented
}

// MergeDirs merges the contents of all the directories passed
// in into the first one and rmdirs the other directories.
func (f *Fs) MergeDirs(ctx context.Context, dirs []fs.Directory) error {
	if do := f.Fs.Features().MergeDirs; do != nil {
		return do(ctx, dirs)
	}
	return errors.New("MergeDirs not supported")
}

// DirCacheFlush resets the directory cache - used in testing
// as an optional interface
func (f *Fs) DirCacheFlush() {
	if do := f.Fs.Features().DirCacheFlush; do != nil {
		do()
	}
}

// PublicLink generates a public link to the remote path (usually readable by anyone)
func (f *Fs) PublicLink(ctx context.Context, remote string, expire fs.Duration, unlink bool) (string, error) {
	if do := f.Fs.Features().PublicLink; do != nil {
		return do(ctx, remote, expire, unlink)
	}
	return "", errors.New("PublicLink not supported")
}

// Copy src to this remote using server-side copy operations.
func (f *Fs) Copy(ctx context.Context, src fs.Object, remote string) (fs.Object, error) {
	do := f.Fs.Features().Copy
	if do == nil {
		return nil, fs.ErrorCantCopy
	}
	o, ok := src.(*Object)
	if !ok {
		return nil, fs.ErrorCantCopy
	}
	oResult, err := do(ctx, o.Object, remote)
	return f.wrapObject(oResult, err), err
}

// Move src to this remote using server-side move operations.
func (f *Fs) Move(ctx context.Context, src fs.Object, remote string) (fs.Object, error) {
	do := f.Fs.Features().Move
	if do == nil {
		return nil, fs.ErrorCantMove
	}
	o, ok := src.(*Object)
	if !ok {
		return nil, fs.ErrorCantMove
	}
	oResult, err := do(ctx, o.Object, remote)
	if err != nil {
		return nil, err
	}
	_ = f.db.Do(true, &kvMove{
		src: path.Join(f.Fs.Root(), src.Remote()),
		dst: path.Join(f.Fs.Root(), remote),
		dir: false,
		fs:  f,
	})
	return f.wrapObject(oResult, nil), nil
}

// DirMove moves src, srcRemote to this remote at dstRemote using server-side move operations.
func (f *Fs) DirMove(ctx context.Context, src fs.Fs, srcRemote, dstRemote string) error {
	do := f.Fs.Features().DirMove
	if do == nil {
		return fs.ErrorCantDirMove
	}
	srcFs, ok := src.(*Fs)
	if !ok {
		return fs.ErrorCantDirMove
	}
	err := do(ctx, srcFs.Fs, srcRemote, dstRemote)
	if err == nil {
		_ = f.db.Do(true, &kvMove{
			src: path.Join(srcFs.Fs.Root(), srcRemote),
			dst: path.Join(f.Fs.Root(), dstRemote),
			dir: true,
			fs:  f,
		})
	}
	return err
}

// Shutdown the backend, closing any background tasks and any cached connections.
func (f *Fs) Shutdown(ctx context.Context) (err error) {
	err = f.db.Stop(false)
	if do := f.Fs.Features().Shutdown; do != nil {
		if err2 := do(ctx); err2 != nil {
			err = err2
		}
	}
	return
}

// NewObject finds the Object at remote.
func (f *Fs) NewObject(ctx context.Context, remote string) (fs.Object, error) {
	o, err := f.Fs.NewObject(ctx, remote)
	return f.wrapObject(o, err), err
}

//
// Object
//

// Object represents a composite file wrapping one or more data chunks
type Object struct {
	fs.Object
	f *Fs
}

// Wrap base object into hasher object
func (f *Fs) wrapObject(o fs.Object, err error) *Object {
	if err != nil || o == nil {
		return nil
	}
	return &Object{Object: o, f: f}
}

// Fs returns read only access to the Fs that this object is part of
func (o *Object) Fs() fs.Info { return o.f }

// UnWrap returns the wrapped Object
func (o *Object) UnWrap() fs.Object { return o.Object }

// Return a string version
func (o *Object) String() string {
	if o == nil {
		return "<nil>"
	}
	return o.Object.String()
}

// ID returns the ID of the Object if possible
func (o *Object) ID() string {
	if doer, ok := o.Object.(fs.IDer); ok {
		return doer.ID()
	}
	return ""
}

// GetTier returns the Tier of the Object if possible
func (o *Object) GetTier() string {
	if doer, ok := o.Object.(fs.GetTierer); ok {
		return doer.GetTier()
	}
	return ""
}

// SetTier set the Tier of the Object if possible
func (o *Object) SetTier(tier string) error {
	if doer, ok := o.Object.(fs.SetTierer); ok {
		return doer.SetTier(tier)
	}
	return errors.New("SetTier not supported")
}

// MimeType of an Object if known, "" otherwise
func (o *Object) MimeType(ctx context.Context) string {
	if doer, ok := o.Object.(fs.MimeTyper); ok {
		return doer.MimeType(ctx)
	}
	return ""
}

// Check the interfaces are satisfied
var (
	_ fs.Fs              = (*Fs)(nil)
	_ fs.Purger          = (*Fs)(nil)
	_ fs.Copier          = (*Fs)(nil)
	_ fs.Mover           = (*Fs)(nil)
	_ fs.DirMover        = (*Fs)(nil)
	_ fs.Commander       = (*Fs)(nil)
	_ fs.PutUncheckeder  = (*Fs)(nil)
	_ fs.PutStreamer     = (*Fs)(nil)
	_ fs.CleanUpper      = (*Fs)(nil)
	_ fs.UnWrapper       = (*Fs)(nil)
	_ fs.ListRer         = (*Fs)(nil)
	_ fs.Abouter         = (*Fs)(nil)
	_ fs.Wrapper         = (*Fs)(nil)
	_ fs.MergeDirser     = (*Fs)(nil)
	_ fs.DirCacheFlusher = (*Fs)(nil)
	_ fs.ChangeNotifier  = (*Fs)(nil)
	_ fs.PublicLinker    = (*Fs)(nil)
	_ fs.UserInfoer      = (*Fs)(nil)
	_ fs.Disconnecter    = (*Fs)(nil)
	_ fs.Shutdowner      = (*Fs)(nil)
	_ fs.Object          = (*Object)(nil)
	_ fs.ObjectUnWrapper = (*Object)(nil)
	_ fs.IDer            = (*Object)(nil)
	_ fs.SetTierer       = (*Object)(nil)
	_ fs.GetTierer       = (*Object)(nil)
	_ fs.MimeTyper       = (*Object)(nil)
)
