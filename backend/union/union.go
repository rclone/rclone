// Package union implements a virtual provider to join existing remotes.
package union

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/rclone/rclone/backend/union/common"
	"github.com/rclone/rclone/backend/union/policy"
	"github.com/rclone/rclone/backend/union/upstream"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config/configmap"
	"github.com/rclone/rclone/fs/config/configstruct"
	"github.com/rclone/rclone/fs/hash"
	"github.com/rclone/rclone/fs/operations"
	"github.com/rclone/rclone/fs/walk"
)

// Register with Fs
func init() {
	fsi := &fs.RegInfo{
		Name:        "union",
		Description: "Union merges the contents of several upstream fs",
		NewFs:       NewFs,
		MetadataInfo: &fs.MetadataInfo{
			Help: `Any metadata supported by the underlying remote is read and written.`,
		},
		Options: []fs.Option{{
			Name:     "upstreams",
			Help:     "List of space separated upstreams.\n\nCan be 'upstreama:test/dir upstreamb:', '\"upstreama:test/space:ro dir\" upstreamb:', etc.",
			Required: true,
		}, {
			Name:    "action_policy",
			Help:    "Policy to choose upstream on ACTION category.",
			Default: "epall",
		}, {
			Name:    "create_policy",
			Help:    "Policy to choose upstream on CREATE category.",
			Default: "epmfs",
		}, {
			Name:    "search_policy",
			Help:    "Policy to choose upstream on SEARCH category.",
			Default: "ff",
		}, {
			Name:    "cache_time",
			Help:    "Cache time of usage and free space (in seconds).\n\nThis option is only useful when a path preserving policy is used.",
			Default: 120,
		}, {
			Name: "min_free_space",
			Help: `Minimum viable free space for lfs/eplfs policies.

If a remote has less than this much free space then it won't be
considered for use in lfs or eplfs policies.`,
			Advanced: true,
			Default:  fs.Gibi,
		}},
	}
	fs.Register(fsi)
}

// Fs represents a union of upstreams
type Fs struct {
	name         string         // name of this remote
	features     *fs.Features   // optional features
	opt          common.Options // options for this Fs
	root         string         // the path we are working on
	upstreams    []*upstream.Fs // slice of upstreams
	hashSet      hash.Set       // intersection of hash types
	actionPolicy policy.Policy  // policy for ACTION
	createPolicy policy.Policy  // policy for CREATE
	searchPolicy policy.Policy  // policy for SEARCH
}

// Wrap candidate objects in to a union Object
func (f *Fs) wrapEntries(entries ...upstream.Entry) (entry, error) {
	e, err := f.searchEntries(entries...)
	if err != nil {
		return nil, err
	}
	switch e := e.(type) {
	case *upstream.Object:
		return &Object{
			Object: e,
			fs:     f,
			co:     entries,
		}, nil
	case *upstream.Directory:
		return &Directory{
			Directory: e,
			fs:        f,
			cd:        entries,
		}, nil
	default:
		return nil, fmt.Errorf("unknown object type %T", e)
	}
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
	upstreams, err := f.action(ctx, dir)
	if err != nil {
		// If none of the backends can have empty directories then
		// don't complain about directories not being found
		if !f.features.CanHaveEmptyDirectories && err == fs.ErrorObjectNotFound {
			return nil
		}
		return err
	}
	errs := Errors(make([]error, len(upstreams)))
	multithread(len(upstreams), func(i int) {
		err := upstreams[i].Rmdir(ctx, dir)
		if err != nil {
			errs[i] = fmt.Errorf("%s: %w", upstreams[i].Name(), err)
		}
	})
	return errs.Err()
}

// Hashes returns hash.HashNone to indicate remote hashing is unavailable
func (f *Fs) Hashes() hash.Set {
	return f.hashSet
}

// mkdir makes the directory passed in and returns the upstreams used
func (f *Fs) mkdir(ctx context.Context, dir string) ([]*upstream.Fs, error) {
	upstreams, err := f.create(ctx, dir)
	if err == fs.ErrorObjectNotFound {
		parent := parentDir(dir)
		if dir != parent {
			upstreams, err = f.mkdir(ctx, parent)
		} else if dir == "" {
			// If root dirs not created then create them
			upstreams, err = f.upstreams, nil
		}
	}
	if err != nil {
		return nil, err
	}
	errs := Errors(make([]error, len(upstreams)))
	multithread(len(upstreams), func(i int) {
		err := upstreams[i].Mkdir(ctx, dir)
		if err != nil {
			errs[i] = fmt.Errorf("%s: %w", upstreams[i].Name(), err)
		}
	})
	err = errs.Err()
	if err != nil {
		return nil, err
	}
	// If created roots then choose one
	if dir == "" {
		upstreams, err = f.create(ctx, dir)
	}
	return upstreams, err
}

// Mkdir makes the root directory of the Fs object
func (f *Fs) Mkdir(ctx context.Context, dir string) error {
	_, err := f.mkdir(ctx, dir)
	return err
}

// MkdirMetadata makes the root directory of the Fs object
func (f *Fs) MkdirMetadata(ctx context.Context, dir string, metadata fs.Metadata) (fs.Directory, error) {
	upstreams, err := f.create(ctx, dir)
	if err != nil {
		return nil, err
	}
	errs := Errors(make([]error, len(upstreams)))
	entries := make([]upstream.Entry, len(upstreams))
	multithread(len(upstreams), func(i int) {
		u := upstreams[i]
		if do := u.Features().MkdirMetadata; do != nil {
			newDir, err := do(ctx, dir, metadata)
			if err != nil {
				errs[i] = fmt.Errorf("%s: %w", upstreams[i].Name(), err)
			} else {
				entries[i], err = u.WrapEntry(newDir)
				if err != nil {
					errs[i] = fmt.Errorf("%s: %w", upstreams[i].Name(), err)
				}
			}

		} else {
			// Just do Mkdir on upstreams which don't support MkdirMetadata
			err := u.Mkdir(ctx, dir)
			if err != nil {
				errs[i] = fmt.Errorf("%s: %w", upstreams[i].Name(), err)
			}
		}
	})
	err = errs.Err()
	if err != nil {
		return nil, err
	}

	entry, err := f.wrapEntries(entries...)
	if err != nil {
		return nil, err
	}
	newDir, ok := entry.(fs.Directory)
	if !ok {
		return nil, fmt.Errorf("internal error: expecting %T to be an fs.Directory", entry)
	}
	return newDir, nil
}

// Purge all files in the directory
//
// Implement this if you have a way of deleting all the files
// quicker than just running Remove() on the result of List()
//
// Return an error if it doesn't exist
func (f *Fs) Purge(ctx context.Context, dir string) error {
	for _, r := range f.upstreams {
		if r.Features().Purge == nil {
			return fs.ErrorCantPurge
		}
	}
	upstreams, err := f.action(ctx, "")
	if err != nil {
		return err
	}
	errs := Errors(make([]error, len(upstreams)))
	multithread(len(upstreams), func(i int) {
		err := upstreams[i].Features().Purge(ctx, dir)
		if errors.Is(err, fs.ErrorDirNotFound) {
			err = nil
		}
		if err != nil {
			errs[i] = fmt.Errorf("%s: %w", upstreams[i].Name(), err)
		}
	})
	return errs.Err()
}

// Copy src to this remote using server-side copy operations.
//
// This is stored with the remote path given.
//
// It returns the destination Object and a possible error.
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
	o := srcObj.UnWrapUpstream()
	su := o.UpstreamFs()
	if su.Features().Copy == nil {
		return nil, fs.ErrorCantCopy
	}
	var du *upstream.Fs
	for _, u := range f.upstreams {
		if operations.Same(u.RootFs, su.RootFs) {
			du = u
		}
	}
	if du == nil {
		return nil, fs.ErrorCantCopy
	}
	if !du.IsCreatable() {
		return nil, fs.ErrorPermissionDenied
	}
	co, err := du.Features().Copy(ctx, o, remote)
	if err != nil || co == nil {
		return nil, err
	}
	wo, err := f.wrapEntries(du.WrapObject(co))
	return wo.(*Object), err
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
	o, ok := src.(*Object)
	if !ok {
		fs.Debugf(src, "Can't move - not same remote type")
		return nil, fs.ErrorCantMove
	}
	entries, err := f.actionEntries(o.candidates()...)
	if err != nil {
		return nil, err
	}
	for _, e := range entries {
		if !operations.CanServerSideMove(e.UpstreamFs()) {
			return nil, fs.ErrorCantMove
		}
	}
	objs := make([]*upstream.Object, len(entries))
	errs := Errors(make([]error, len(entries)))
	multithread(len(entries), func(i int) {
		su := entries[i].UpstreamFs()
		o, ok := entries[i].(*upstream.Object)
		if !ok {
			errs[i] = fmt.Errorf("%s: %w", su.Name(), fs.ErrorNotAFile)
			return
		}
		var du *upstream.Fs
		for _, u := range f.upstreams {
			if operations.Same(u.RootFs, su.RootFs) {
				du = u
			}
		}
		if du == nil {
			errs[i] = fmt.Errorf("%s: %s: %w", su.Name(), remote, fs.ErrorCantMove)
			return
		}
		srcObj := o.UnWrap()
		duFeatures := du.Features()
		do := duFeatures.Move
		if duFeatures.Move == nil {
			do = duFeatures.Copy
		}
		// Do the Move or Copy
		dstObj, err := do(ctx, srcObj, remote)
		if err != nil {
			errs[i] = fmt.Errorf("%s: %w", su.Name(), err)
			return
		}
		if dstObj == nil {
			errs[i] = fmt.Errorf("%s: destination object not found", su.Name())
			return
		}
		objs[i] = du.WrapObject(dstObj)
		// Delete the source object if Copy
		if duFeatures.Move == nil {
			err = srcObj.Remove(ctx)
			if err != nil {
				errs[i] = fmt.Errorf("%s: %w", su.Name(), err)
				return
			}
		}
	})
	var en []upstream.Entry
	for _, o := range objs {
		if o != nil {
			en = append(en, o)
		}
	}
	e, err := f.wrapEntries(en...)
	if err != nil {
		return nil, err
	}
	return e.(*Object), errs.Err()
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
	sfs, ok := src.(*Fs)
	if !ok {
		fs.Debugf(src, "Can't move directory - not same remote type")
		return fs.ErrorCantDirMove
	}
	upstreams, err := sfs.action(ctx, srcRemote)
	if err != nil {
		return err
	}
	for _, u := range upstreams {
		if u.Features().DirMove == nil {
			return fs.ErrorCantDirMove
		}
	}
	errs := Errors(make([]error, len(upstreams)))
	multithread(len(upstreams), func(i int) {
		su := upstreams[i]
		var du *upstream.Fs
		for _, u := range f.upstreams {
			if operations.Same(u.RootFs, su.RootFs) {
				du = u
			}
		}
		if du == nil {
			errs[i] = fmt.Errorf("%s: %s: %w", su.Name(), su.Root(), fs.ErrorCantDirMove)
			return
		}
		err := du.Features().DirMove(ctx, su.Fs, srcRemote, dstRemote)
		if err != nil {
			errs[i] = fmt.Errorf("%s: %w", du.Name()+":"+du.Root(), err)
		}
	})
	errs = errs.FilterNil()
	if len(errs) == 0 {
		return nil
	}
	for _, e := range errs {
		if !errors.Is(e, fs.ErrorDirExists) {
			return errs
		}
	}
	return fs.ErrorDirExists
}

// DirSetModTime sets the directory modtime for dir
func (f *Fs) DirSetModTime(ctx context.Context, dir string, modTime time.Time) error {
	upstreams, err := f.action(ctx, dir)
	if err != nil {
		return err
	}
	errs := Errors(make([]error, len(upstreams)))
	multithread(len(upstreams), func(i int) {
		u := upstreams[i]
		// ignore DirSetModTime on upstreams which don't support it
		if do := u.Features().DirSetModTime; do != nil {
			err := do(ctx, dir, modTime)
			if err != nil {
				errs[i] = fmt.Errorf("%s: %w", upstreams[i].Name(), err)
			}
		}
	})
	return errs.Err()
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
	var uChans []chan time.Duration

	for _, u := range f.upstreams {
		if ChangeNotify := u.Features().ChangeNotify; ChangeNotify != nil {
			ch := make(chan time.Duration)
			uChans = append(uChans, ch)
			ChangeNotify(ctx, fn, ch)
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
	multithread(len(f.upstreams), func(i int) {
		if do := f.upstreams[i].Features().DirCacheFlush; do != nil {
			do()
		}
	})
}

// Tee in into n outputs
//
// When finished read the error from the channel
func multiReader(n int, in io.Reader) ([]io.Reader, <-chan error) {
	readers := make([]io.Reader, n)
	pipeWriters := make([]*io.PipeWriter, n)
	writers := make([]io.Writer, n)
	errChan := make(chan error, 1)
	for i := range writers {
		r, w := io.Pipe()
		bw := bufio.NewWriter(w)
		readers[i], pipeWriters[i], writers[i] = r, w, bw
	}
	go func() {
		mw := io.MultiWriter(writers...)
		es := make([]error, 2*n+1)
		_, copyErr := io.Copy(mw, in)
		es[2*n] = copyErr
		// Flush the buffers
		for i, bw := range writers {
			es[i] = bw.(*bufio.Writer).Flush()
		}
		// Close the underlying pipes
		for i, pw := range pipeWriters {
			es[2*i] = pw.CloseWithError(copyErr)
		}
		errChan <- Errors(es).Err()
	}()
	return readers, errChan
}

func (f *Fs) put(ctx context.Context, in io.Reader, src fs.ObjectInfo, stream bool, options ...fs.OpenOption) (fs.Object, error) {
	srcPath := src.Remote()
	upstreams, err := f.create(ctx, srcPath)
	if err == fs.ErrorObjectNotFound {
		upstreams, err = f.mkdir(ctx, parentDir(srcPath))
	}
	if err != nil {
		return nil, err
	}
	if len(upstreams) == 1 {
		u := upstreams[0]
		var o fs.Object
		var err error
		if stream {
			o, err = u.Features().PutStream(ctx, in, src, options...)
		} else {
			o, err = u.Put(ctx, in, src, options...)
		}
		if err != nil {
			return nil, err
		}
		e, err := f.wrapEntries(u.WrapObject(o))
		return e.(*Object), err
	}
	// Multi-threading
	readers, errChan := multiReader(len(upstreams), in)
	errs := Errors(make([]error, len(upstreams)+1))
	objs := make([]upstream.Entry, len(upstreams))
	multithread(len(upstreams), func(i int) {
		u := upstreams[i]
		var o fs.Object
		var err error
		if stream {
			o, err = u.Features().PutStream(ctx, readers[i], src, options...)
		} else {
			o, err = u.Put(ctx, readers[i], src, options...)
		}
		if err != nil {
			errs[i] = fmt.Errorf("%s: %w", u.Name(), err)
			if len(upstreams) > 1 {
				// Drain the input buffer to allow other uploads to continue
				_, _ = io.Copy(io.Discard, readers[i])
			}
			return
		}
		objs[i] = u.WrapObject(o)
	})
	errs[len(upstreams)] = <-errChan
	err = errs.Err()
	if err != nil {
		return nil, err
	}
	e, err := f.wrapEntries(objs...)
	return e.(*Object), err
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
		usg, err := u.About(ctx)
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
	entriesList := make([][]upstream.Entry, len(f.upstreams))
	errs := Errors(make([]error, len(f.upstreams)))
	multithread(len(f.upstreams), func(i int) {
		u := f.upstreams[i]
		entries, err := u.List(ctx, dir)
		if err != nil {
			errs[i] = fmt.Errorf("%s: %w", u.Name(), err)
			return
		}
		uEntries := make([]upstream.Entry, len(entries))
		for j, e := range entries {
			uEntries[j], _ = u.WrapEntry(e)
		}
		entriesList[i] = uEntries
	})
	if len(errs) == len(errs.FilterNil()) {
		errs = errs.Map(func(e error) error {
			if errors.Is(e, fs.ErrorDirNotFound) {
				return nil
			}
			return e
		})
		if len(errs) == 0 {
			return nil, fs.ErrorDirNotFound
		}
		return nil, errs.Err()
	}
	return f.mergeDirEntries(entriesList)
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
	var entriesList [][]upstream.Entry
	errs := Errors(make([]error, len(f.upstreams)))
	var mutex sync.Mutex
	multithread(len(f.upstreams), func(i int) {
		u := f.upstreams[i]
		var err error
		callback := func(entries fs.DirEntries) error {
			uEntries := make([]upstream.Entry, len(entries))
			for j, e := range entries {
				uEntries[j], _ = u.WrapEntry(e)
			}
			mutex.Lock()
			entriesList = append(entriesList, uEntries)
			mutex.Unlock()
			return nil
		}
		do := u.Features().ListR
		if do != nil {
			err = do(ctx, dir, callback)
		} else {
			err = walk.ListR(ctx, u, dir, true, -1, walk.ListAll, callback)
		}
		if err != nil {
			errs[i] = fmt.Errorf("%s: %w", u.Name(), err)
			return
		}
	})
	if len(errs) == len(errs.FilterNil()) {
		errs = errs.Map(func(e error) error {
			if errors.Is(e, fs.ErrorDirNotFound) {
				return nil
			}
			return e
		})
		if len(errs) == 0 {
			return fs.ErrorDirNotFound
		}
		return errs.Err()
	}
	entries, err := f.mergeDirEntries(entriesList)
	if err != nil {
		return err
	}
	return callback(entries)
}

// NewObject creates a new remote union file object
func (f *Fs) NewObject(ctx context.Context, remote string) (fs.Object, error) {
	objs := make([]*upstream.Object, len(f.upstreams))
	errs := Errors(make([]error, len(f.upstreams)))
	multithread(len(f.upstreams), func(i int) {
		u := f.upstreams[i]
		o, err := u.NewObject(ctx, remote)
		if err != nil && err != fs.ErrorObjectNotFound {
			errs[i] = fmt.Errorf("%s: %w", u.Name(), err)
			return
		}
		objs[i] = u.WrapObject(o)
	})
	var entries []upstream.Entry
	for _, o := range objs {
		if o != nil {
			entries = append(entries, o)
		}
	}
	if len(entries) == 0 {
		return nil, fs.ErrorObjectNotFound
	}
	e, err := f.wrapEntries(entries...)
	if err != nil {
		return nil, err
	}
	return e.(*Object), errs.Err()
}

// Precision is the greatest Precision of all upstreams
func (f *Fs) Precision() time.Duration {
	var greatestPrecision time.Duration
	for _, u := range f.upstreams {
		if u.Precision() > greatestPrecision {
			greatestPrecision = u.Precision()
		}
	}
	return greatestPrecision
}

func (f *Fs) action(ctx context.Context, path string) ([]*upstream.Fs, error) {
	return f.actionPolicy.Action(ctx, f.upstreams, path)
}

func (f *Fs) actionEntries(entries ...upstream.Entry) ([]upstream.Entry, error) {
	return f.actionPolicy.ActionEntries(entries...)
}

func (f *Fs) create(ctx context.Context, path string) ([]*upstream.Fs, error) {
	return f.createPolicy.Create(ctx, f.upstreams, path)
}

func (f *Fs) searchEntries(entries ...upstream.Entry) (upstream.Entry, error) {
	return f.searchPolicy.SearchEntries(entries...)
}

func (f *Fs) mergeDirEntries(entriesList [][]upstream.Entry) (fs.DirEntries, error) {
	entryMap := make(map[string]([]upstream.Entry))
	for _, en := range entriesList {
		if en == nil {
			continue
		}
		for _, entry := range en {
			remote := entry.Remote()
			if f.Features().CaseInsensitive {
				remote = strings.ToLower(remote)
			}
			entryMap[remote] = append(entryMap[remote], entry)
		}
	}
	var entries fs.DirEntries
	for path := range entryMap {
		e, err := f.wrapEntries(entryMap[path]...)
		if err != nil {
			return nil, err
		}
		entries = append(entries, e)
	}
	return entries, nil
}

// Shutdown the backend, closing any background tasks and any
// cached connections.
func (f *Fs) Shutdown(ctx context.Context) error {
	errs := Errors(make([]error, len(f.upstreams)))
	multithread(len(f.upstreams), func(i int) {
		u := f.upstreams[i]
		if do := u.Features().Shutdown; do != nil {
			err := do(ctx)
			if err != nil {
				errs[i] = fmt.Errorf("%s: %w", u.Name(), err)
			}
		}
	})
	return errs.Err()
}

// CleanUp the trash in the Fs
//
// Implement this if you have a way of emptying the trash or
// otherwise cleaning up old versions of files.
func (f *Fs) CleanUp(ctx context.Context) error {
	errs := Errors(make([]error, len(f.upstreams)))
	multithread(len(f.upstreams), func(i int) {
		u := f.upstreams[i]
		if do := u.Features().CleanUp; do != nil {
			err := do(ctx)
			if err != nil {
				errs[i] = fmt.Errorf("%s: %w", u.Name(), err)
			}
		}
	})
	return errs.Err()
}

// NewFs constructs an Fs from the path.
//
// The returned Fs is the actual Fs, referenced by remote in the config
func NewFs(ctx context.Context, name, root string, m configmap.Mapper) (fs.Fs, error) {
	// Parse config into Options struct
	opt := new(common.Options)
	err := configstruct.Set(m, opt)
	if err != nil {
		return nil, err
	}
	// Backward compatible to old config
	if len(opt.Upstreams) == 0 && len(opt.Remotes) > 0 {
		for i := 0; i < len(opt.Remotes)-1; i++ {
			opt.Remotes[i] += ":ro"
		}
		opt.Upstreams = opt.Remotes
	}
	if len(opt.Upstreams) == 0 {
		return nil, errors.New("union can't point to an empty upstream - check the value of the upstreams setting")
	}
	if len(opt.Upstreams) == 1 {
		return nil, errors.New("union can't point to a single upstream - check the value of the upstreams setting")
	}
	for _, u := range opt.Upstreams {
		if strings.HasPrefix(u, name+":") {
			return nil, errors.New("can't point union remote at itself - check the value of the upstreams setting")
		}
	}

	root = strings.Trim(root, "/")
	upstreams := make([]*upstream.Fs, len(opt.Upstreams))
	errs := Errors(make([]error, len(opt.Upstreams)))
	multithread(len(opt.Upstreams), func(i int) {
		u := opt.Upstreams[i]
		upstreams[i], errs[i] = upstream.New(ctx, u, root, opt)
	})
	var usedUpstreams []*upstream.Fs
	var fserr error
	for i, err := range errs {
		if err != nil && err != fs.ErrorIsFile {
			return nil, err
		}
		// Only the upstreams returns ErrorIsFile would be used if any
		if err == fs.ErrorIsFile {
			usedUpstreams = append(usedUpstreams, upstreams[i])
			fserr = fs.ErrorIsFile
		}
	}
	if fserr == nil {
		usedUpstreams = upstreams
	}

	f := &Fs{
		name:      name,
		root:      root,
		opt:       *opt,
		upstreams: usedUpstreams,
	}
	// Correct root if definitely pointing to a file
	if fserr == fs.ErrorIsFile {
		f.root = path.Dir(f.root)
		if f.root == "." || f.root == "/" {
			f.root = ""
		}
	}
	err = upstream.Prepare(f.upstreams)
	if err != nil {
		return nil, err
	}
	f.actionPolicy, err = policy.Get(opt.ActionPolicy)
	if err != nil {
		return nil, err
	}
	f.createPolicy, err = policy.Get(opt.CreatePolicy)
	if err != nil {
		return nil, err
	}
	f.searchPolicy, err = policy.Get(opt.SearchPolicy)
	if err != nil {
		return nil, err
	}
	fs.Debugf(f, "actionPolicy = %T, createPolicy = %T, searchPolicy = %T", f.actionPolicy, f.createPolicy, f.searchPolicy)
	var features = (&fs.Features{
		CaseInsensitive:          true,
		DuplicateFiles:           false,
		ReadMimeType:             true,
		WriteMimeType:            true,
		CanHaveEmptyDirectories:  true,
		BucketBased:              true,
		SetTier:                  true,
		GetTier:                  true,
		ReadMetadata:             true,
		WriteMetadata:            true,
		UserMetadata:             true,
		ReadDirMetadata:          true,
		WriteDirMetadata:         true,
		WriteDirSetModTime:       true,
		UserDirMetadata:          true,
		DirModTimeUpdatesOnWrite: true,
		PartialUploads:           true,
	}).Fill(ctx, f)
	canMove, slowHash := true, false
	for _, f := range upstreams {
		features = features.Mask(ctx, f) // Mask all upstream fs
		if !operations.CanServerSideMove(f) {
			canMove = false
		}
		slowHash = slowHash || f.Features().SlowHash
	}
	// We can move if all remotes support Move or Copy
	if canMove {
		features.Move = f.Move
	}

	// If any of upstreams are SlowHash, propagate it
	features.SlowHash = slowHash

	// Enable ListR when upstreams either support ListR or is local
	// But not when all upstreams are local
	if features.ListR == nil {
		for _, u := range upstreams {
			if u.Features().ListR != nil {
				features.ListR = f.ListR
			} else if !u.Features().IsLocal {
				features.ListR = nil
				break
			}
		}
	}

	// show that we wrap other backends
	features.Overlay = true

	f.features = features

	// Get common intersection of hashes
	hashSet := f.upstreams[0].Hashes()
	for _, u := range f.upstreams[1:] {
		hashSet = hashSet.Overlap(u.Hashes())
	}
	f.hashSet = hashSet

	return f, fserr
}

func parentDir(absPath string) string {
	parent := path.Dir(strings.TrimRight(filepath.ToSlash(absPath), "/"))
	if parent == "." {
		parent = ""
	}
	return parent
}

func multithread(num int, fn func(int)) {
	var wg sync.WaitGroup
	for i := 0; i < num; i++ {
		wg.Add(1)
		i := i
		go func() {
			defer wg.Done()
			fn(i)
		}()
	}
	wg.Wait()
}

// Check the interfaces are satisfied
var (
	_ fs.Fs              = (*Fs)(nil)
	_ fs.Purger          = (*Fs)(nil)
	_ fs.PutStreamer     = (*Fs)(nil)
	_ fs.Copier          = (*Fs)(nil)
	_ fs.Mover           = (*Fs)(nil)
	_ fs.DirMover        = (*Fs)(nil)
	_ fs.DirSetModTimer  = (*Fs)(nil)
	_ fs.MkdirMetadataer = (*Fs)(nil)
	_ fs.DirCacheFlusher = (*Fs)(nil)
	_ fs.ChangeNotifier  = (*Fs)(nil)
	_ fs.Abouter         = (*Fs)(nil)
	_ fs.ListRer         = (*Fs)(nil)
	_ fs.Shutdowner      = (*Fs)(nil)
	_ fs.CleanUpper      = (*Fs)(nil)
)
