// Filesystem features and optional interfaces

package fs

import (
	"context"
	"io"
	"reflect"
	"strings"
	"time"
)

// Features describe the optional features of the Fs
type Features struct {
	// Feature flags, whether Fs
	CaseInsensitive         bool // has case insensitive files
	DuplicateFiles          bool // allows duplicate files
	ReadMimeType            bool // can read the mime type of objects
	WriteMimeType           bool // can set the mime type of objects
	CanHaveEmptyDirectories bool // can have empty directories
	BucketBased             bool // is bucket based (like s3, swift, etc.)
	BucketBasedRootOK       bool // is bucket based and can use from root
	SetTier                 bool // allows set tier functionality on objects
	GetTier                 bool // allows to retrieve storage tier of objects
	ServerSideAcrossConfigs bool // can server-side copy between different remotes of the same type
	IsLocal                 bool // is the local backend
	SlowModTime             bool // if calling ModTime() generally takes an extra transaction
	SlowHash                bool // if calling Hash() generally takes an extra transaction

	// Purge all files in the directory specified
	//
	// Implement this if you have a way of deleting all the files
	// quicker than just running Remove() on the result of List()
	//
	// Return an error if it doesn't exist
	Purge func(ctx context.Context, dir string) error

	// Copy src to this remote using server-side copy operations.
	//
	// This is stored with the remote path given
	//
	// It returns the destination Object and a possible error
	//
	// Will only be called if src.Fs().Name() == f.Name()
	//
	// If it isn't possible then return fs.ErrorCantCopy
	Copy func(ctx context.Context, src Object, remote string) (Object, error)

	// Move src to this remote using server-side move operations.
	//
	// This is stored with the remote path given
	//
	// It returns the destination Object and a possible error
	//
	// Will only be called if src.Fs().Name() == f.Name()
	//
	// If it isn't possible then return fs.ErrorCantMove
	Move func(ctx context.Context, src Object, remote string) (Object, error)

	// DirMove moves src, srcRemote to this remote at dstRemote
	// using server-side move operations.
	//
	// Will only be called if src.Fs().Name() == f.Name()
	//
	// If it isn't possible then return fs.ErrorCantDirMove
	//
	// If destination exists then return fs.ErrorDirExists
	DirMove func(ctx context.Context, src Fs, srcRemote, dstRemote string) error

	// ChangeNotify calls the passed function with a path
	// that has had changes. If the implementation
	// uses polling, it should adhere to the given interval.
	ChangeNotify func(context.Context, func(string, EntryType), <-chan time.Duration)

	// UnWrap returns the Fs that this Fs is wrapping
	UnWrap func() Fs

	// WrapFs returns the Fs that is wrapping this Fs
	WrapFs func() Fs

	// SetWrapper sets the Fs that is wrapping this Fs
	SetWrapper func(f Fs)

	// DirCacheFlush resets the directory cache - used in testing
	// as an optional interface
	DirCacheFlush func()

	// PublicLink generates a public link to the remote path (usually readable by anyone)
	PublicLink func(ctx context.Context, remote string, expire Duration, unlink bool) (string, error)

	// Put in to the remote path with the modTime given of the given size
	//
	// May create the object even if it returns an error - if so
	// will return the object and the error, otherwise will return
	// nil and the error
	//
	// May create duplicates or return errors if src already
	// exists.
	PutUnchecked func(ctx context.Context, in io.Reader, src ObjectInfo, options ...OpenOption) (Object, error)

	// PutStream uploads to the remote path with the modTime given of indeterminate size
	//
	// May create the object even if it returns an error - if so
	// will return the object and the error, otherwise will return
	// nil and the error
	PutStream func(ctx context.Context, in io.Reader, src ObjectInfo, options ...OpenOption) (Object, error)

	// MergeDirs merges the contents of all the directories passed
	// in into the first one and rmdirs the other directories.
	MergeDirs func(ctx context.Context, dirs []Directory) error

	// CleanUp the trash in the Fs
	//
	// Implement this if you have a way of emptying the trash or
	// otherwise cleaning up old versions of files.
	CleanUp func(ctx context.Context) error

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
	ListR ListRFn

	// About gets quota information from the Fs
	About func(ctx context.Context) (*Usage, error)

	// OpenWriterAt opens with a handle for random access writes
	//
	// Pass in the remote desired and the size if known.
	//
	// It truncates any existing object
	OpenWriterAt func(ctx context.Context, remote string, size int64) (WriterAtCloser, error)

	// UserInfo returns info about the connected user
	UserInfo func(ctx context.Context) (map[string]string, error)

	// Disconnect the current user
	Disconnect func(ctx context.Context) error

	// Command the backend to run a named command
	//
	// The command run is name
	// args may be used to read arguments from
	// opts may be used to read optional arguments from
	//
	// The result should be capable of being JSON encoded
	// If it is a string or a []string it will be shown to the user
	// otherwise it will be JSON encoded and shown to the user like that
	Command func(ctx context.Context, name string, arg []string, opt map[string]string) (interface{}, error)

	// Shutdown the backend, closing any background tasks and any
	// cached connections.
	Shutdown func(ctx context.Context) error
}

// Disable nil's out the named feature.  If it isn't found then it
// will log a message.
func (ft *Features) Disable(name string) *Features {
	v := reflect.ValueOf(ft).Elem()
	vType := v.Type()
	for i := 0; i < v.NumField(); i++ {
		vName := vType.Field(i).Name
		field := v.Field(i)
		if strings.EqualFold(name, vName) {
			if !field.CanSet() {
				Errorf(nil, "Can't set Feature %q", name)
			} else {
				zero := reflect.Zero(field.Type())
				field.Set(zero)
				Debugf(nil, "Reset feature %q", name)
			}
		}
	}
	return ft
}

// List returns a slice of all the possible feature names
func (ft *Features) List() (out []string) {
	v := reflect.ValueOf(ft).Elem()
	vType := v.Type()
	for i := 0; i < v.NumField(); i++ {
		out = append(out, vType.Field(i).Name)
	}
	return out
}

// Enabled returns a map of features with keys showing whether they
// are enabled or not
func (ft *Features) Enabled() (features map[string]bool) {
	v := reflect.ValueOf(ft).Elem()
	vType := v.Type()
	features = make(map[string]bool, v.NumField())
	for i := 0; i < v.NumField(); i++ {
		vName := vType.Field(i).Name
		field := v.Field(i)
		if field.Kind() == reflect.Func {
			// Can't compare functions
			features[vName] = !field.IsNil()
		} else {
			zero := reflect.Zero(field.Type())
			features[vName] = field.Interface() != zero.Interface()
		}
	}
	return features
}

// DisableList nil's out the comma separated list of named features.
// If it isn't found then it will log a message.
func (ft *Features) DisableList(list []string) *Features {
	for _, feature := range list {
		ft.Disable(strings.TrimSpace(feature))
	}
	return ft
}

// Fill fills in the function pointers in the Features struct from the
// optional interfaces.  It returns the original updated Features
// struct passed in.
func (ft *Features) Fill(ctx context.Context, f Fs) *Features {
	if do, ok := f.(Purger); ok {
		ft.Purge = do.Purge
	}
	if do, ok := f.(Copier); ok {
		ft.Copy = do.Copy
	}
	if do, ok := f.(Mover); ok {
		ft.Move = do.Move
	}
	if do, ok := f.(DirMover); ok {
		ft.DirMove = do.DirMove
	}
	if do, ok := f.(ChangeNotifier); ok {
		ft.ChangeNotify = do.ChangeNotify
	}
	if do, ok := f.(UnWrapper); ok {
		ft.UnWrap = do.UnWrap
	}
	if do, ok := f.(Wrapper); ok {
		ft.WrapFs = do.WrapFs
		ft.SetWrapper = do.SetWrapper
	}
	if do, ok := f.(DirCacheFlusher); ok {
		ft.DirCacheFlush = do.DirCacheFlush
	}
	if do, ok := f.(PublicLinker); ok {
		ft.PublicLink = do.PublicLink
	}
	if do, ok := f.(PutUncheckeder); ok {
		ft.PutUnchecked = do.PutUnchecked
	}
	if do, ok := f.(PutStreamer); ok {
		ft.PutStream = do.PutStream
	}
	if do, ok := f.(MergeDirser); ok {
		ft.MergeDirs = do.MergeDirs
	}
	if do, ok := f.(CleanUpper); ok {
		ft.CleanUp = do.CleanUp
	}
	if do, ok := f.(ListRer); ok {
		ft.ListR = do.ListR
	}
	if do, ok := f.(Abouter); ok {
		ft.About = do.About
	}
	if do, ok := f.(OpenWriterAter); ok {
		ft.OpenWriterAt = do.OpenWriterAt
	}
	if do, ok := f.(UserInfoer); ok {
		ft.UserInfo = do.UserInfo
	}
	if do, ok := f.(Disconnecter); ok {
		ft.Disconnect = do.Disconnect
	}
	if do, ok := f.(Commander); ok {
		ft.Command = do.Command
	}
	if do, ok := f.(Shutdowner); ok {
		ft.Shutdown = do.Shutdown
	}
	return ft.DisableList(GetConfig(ctx).DisableFeatures)
}

// Mask the Features with the Fs passed in
//
// Only optional features which are implemented in both the original
// Fs AND the one passed in will be advertised.  Any features which
// aren't in both will be set to false/nil, except for UnWrap/Wrap which
// will be left untouched.
func (ft *Features) Mask(ctx context.Context, f Fs) *Features {
	mask := f.Features()
	ft.CaseInsensitive = ft.CaseInsensitive && mask.CaseInsensitive
	ft.DuplicateFiles = ft.DuplicateFiles && mask.DuplicateFiles
	ft.ReadMimeType = ft.ReadMimeType && mask.ReadMimeType
	ft.WriteMimeType = ft.WriteMimeType && mask.WriteMimeType
	ft.CanHaveEmptyDirectories = ft.CanHaveEmptyDirectories && mask.CanHaveEmptyDirectories
	ft.BucketBased = ft.BucketBased && mask.BucketBased
	ft.BucketBasedRootOK = ft.BucketBasedRootOK && mask.BucketBasedRootOK
	ft.SetTier = ft.SetTier && mask.SetTier
	ft.GetTier = ft.GetTier && mask.GetTier
	ft.ServerSideAcrossConfigs = ft.ServerSideAcrossConfigs && mask.ServerSideAcrossConfigs
	// ft.IsLocal = ft.IsLocal && mask.IsLocal Don't propagate IsLocal
	ft.SlowModTime = ft.SlowModTime && mask.SlowModTime
	ft.SlowHash = ft.SlowHash && mask.SlowHash

	if mask.Purge == nil {
		ft.Purge = nil
	}
	if mask.Copy == nil {
		ft.Copy = nil
	}
	if mask.Move == nil {
		ft.Move = nil
	}
	if mask.DirMove == nil {
		ft.DirMove = nil
	}
	if mask.ChangeNotify == nil {
		ft.ChangeNotify = nil
	}
	// if mask.UnWrap == nil {
	// 	ft.UnWrap = nil
	// }
	// if mask.Wrapper == nil {
	// 	ft.Wrapper = nil
	// }
	if mask.DirCacheFlush == nil {
		ft.DirCacheFlush = nil
	}
	if mask.PublicLink == nil {
		ft.PublicLink = nil
	}
	if mask.PutUnchecked == nil {
		ft.PutUnchecked = nil
	}
	if mask.PutStream == nil {
		ft.PutStream = nil
	}
	if mask.MergeDirs == nil {
		ft.MergeDirs = nil
	}
	if mask.CleanUp == nil {
		ft.CleanUp = nil
	}
	if mask.ListR == nil {
		ft.ListR = nil
	}
	if mask.About == nil {
		ft.About = nil
	}
	if mask.OpenWriterAt == nil {
		ft.OpenWriterAt = nil
	}
	if mask.UserInfo == nil {
		ft.UserInfo = nil
	}
	if mask.Disconnect == nil {
		ft.Disconnect = nil
	}
	// Command is always local so we don't mask it
	if mask.Shutdown == nil {
		ft.Shutdown = nil
	}
	return ft.DisableList(GetConfig(ctx).DisableFeatures)
}

// Wrap makes a Copy of the features passed in, overriding the UnWrap/Wrap
// method only if available in f.
func (ft *Features) Wrap(f Fs) *Features {
	ftCopy := new(Features)
	*ftCopy = *ft
	if do, ok := f.(UnWrapper); ok {
		ftCopy.UnWrap = do.UnWrap
	}
	if do, ok := f.(Wrapper); ok {
		ftCopy.WrapFs = do.WrapFs
		ftCopy.SetWrapper = do.SetWrapper
	}
	return ftCopy
}

// WrapsFs adds extra information between `f` which wraps `w`
func (ft *Features) WrapsFs(f Fs, w Fs) *Features {
	wFeatures := w.Features()
	if wFeatures.WrapFs != nil && wFeatures.SetWrapper != nil {
		wFeatures.SetWrapper(f)
	}
	return ft
}

// Purger is an optional interfaces for Fs
type Purger interface {
	// Purge all files in the directory specified
	//
	// Implement this if you have a way of deleting all the files
	// quicker than just running Remove() on the result of List()
	//
	// Return an error if it doesn't exist
	Purge(ctx context.Context, dir string) error
}

// Copier is an optional interface for Fs
type Copier interface {
	// Copy src to this remote using server-side copy operations.
	//
	// This is stored with the remote path given
	//
	// It returns the destination Object and a possible error
	//
	// Will only be called if src.Fs().Name() == f.Name()
	//
	// If it isn't possible then return fs.ErrorCantCopy
	Copy(ctx context.Context, src Object, remote string) (Object, error)
}

// Mover is an optional interface for Fs
type Mover interface {
	// Move src to this remote using server-side move operations.
	//
	// This is stored with the remote path given
	//
	// It returns the destination Object and a possible error
	//
	// Will only be called if src.Fs().Name() == f.Name()
	//
	// If it isn't possible then return fs.ErrorCantMove
	Move(ctx context.Context, src Object, remote string) (Object, error)
}

// DirMover is an optional interface for Fs
type DirMover interface {
	// DirMove moves src, srcRemote to this remote at dstRemote
	// using server-side move operations.
	//
	// Will only be called if src.Fs().Name() == f.Name()
	//
	// If it isn't possible then return fs.ErrorCantDirMove
	//
	// If destination exists then return fs.ErrorDirExists
	DirMove(ctx context.Context, src Fs, srcRemote, dstRemote string) error
}

// ChangeNotifier is an optional interface for Fs
type ChangeNotifier interface {
	// ChangeNotify calls the passed function with a path
	// that has had changes. If the implementation
	// uses polling, it should adhere to the given interval.
	// At least one value will be written to the channel,
	// specifying the initial value and updated values might
	// follow. A 0 Duration should pause the polling.
	// The ChangeNotify implementation must empty the channel
	// regularly. When the channel gets closed, the implementation
	// should stop polling and release resources.
	ChangeNotify(context.Context, func(string, EntryType), <-chan time.Duration)
}

// EntryType can be associated with remote paths to identify their type
type EntryType int

// Constants
const (
	// EntryDirectory should be used to classify remote paths in directories
	EntryDirectory EntryType = iota // 0
	// EntryObject should be used to classify remote paths in objects
	EntryObject // 1
)

// UnWrapper is an optional interfaces for Fs
type UnWrapper interface {
	// UnWrap returns the Fs that this Fs is wrapping
	UnWrap() Fs
}

// Wrapper is an optional interfaces for Fs
type Wrapper interface {
	// Wrap returns the Fs that is wrapping this Fs
	WrapFs() Fs
	// SetWrapper sets the Fs that is wrapping this Fs
	SetWrapper(f Fs)
}

// DirCacheFlusher is an optional interface for Fs
type DirCacheFlusher interface {
	// DirCacheFlush resets the directory cache - used in testing
	// as an optional interface
	DirCacheFlush()
}

// PutUncheckeder is an optional interface for Fs
type PutUncheckeder interface {
	// Put in to the remote path with the modTime given of the given size
	//
	// May create the object even if it returns an error - if so
	// will return the object and the error, otherwise will return
	// nil and the error
	//
	// May create duplicates or return errors if src already
	// exists.
	PutUnchecked(ctx context.Context, in io.Reader, src ObjectInfo, options ...OpenOption) (Object, error)
}

// PutStreamer is an optional interface for Fs
type PutStreamer interface {
	// PutStream uploads to the remote path with the modTime given of indeterminate size
	//
	// May create the object even if it returns an error - if so
	// will return the object and the error, otherwise will return
	// nil and the error
	PutStream(ctx context.Context, in io.Reader, src ObjectInfo, options ...OpenOption) (Object, error)
}

// PublicLinker is an optional interface for Fs
type PublicLinker interface {
	// PublicLink generates a public link to the remote path (usually readable by anyone)
	PublicLink(ctx context.Context, remote string, expire Duration, unlink bool) (string, error)
}

// MergeDirser is an option interface for Fs
type MergeDirser interface {
	// MergeDirs merges the contents of all the directories passed
	// in into the first one and rmdirs the other directories.
	MergeDirs(ctx context.Context, dirs []Directory) error
}

// CleanUpper is an optional interfaces for Fs
type CleanUpper interface {
	// CleanUp the trash in the Fs
	//
	// Implement this if you have a way of emptying the trash or
	// otherwise cleaning up old versions of files.
	CleanUp(ctx context.Context) error
}

// ListRer is an optional interfaces for Fs
type ListRer interface {
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
	ListR(ctx context.Context, dir string, callback ListRCallback) error
}

// RangeSeeker is the interface that wraps the RangeSeek method.
//
// Some of the returns from Object.Open() may optionally implement
// this method for efficiency purposes.
type RangeSeeker interface {
	// RangeSeek behaves like a call to Seek(offset int64, whence
	// int) with the output wrapped in an io.LimitedReader
	// limiting the total length to limit.
	//
	// RangeSeek with a limit of < 0 is equivalent to a regular Seek.
	RangeSeek(ctx context.Context, offset int64, whence int, length int64) (int64, error)
}

// Abouter is an optional interface for Fs
type Abouter interface {
	// About gets quota information from the Fs
	About(ctx context.Context) (*Usage, error)
}

// OpenWriterAter is an optional interface for Fs
type OpenWriterAter interface {
	// OpenWriterAt opens with a handle for random access writes
	//
	// Pass in the remote desired and the size if known.
	//
	// It truncates any existing object
	OpenWriterAt(ctx context.Context, remote string, size int64) (WriterAtCloser, error)
}

// UserInfoer is an optional interface for Fs
type UserInfoer interface {
	// UserInfo returns info about the connected user
	UserInfo(ctx context.Context) (map[string]string, error)
}

// Disconnecter is an optional interface for Fs
type Disconnecter interface {
	// Disconnect the current user
	Disconnect(ctx context.Context) error
}

// CommandHelp describes a single backend Command
//
// These are automatically inserted in the docs
type CommandHelp struct {
	Name  string            // Name of the command, e.g. "link"
	Short string            // Single line description
	Long  string            // Long multi-line description
	Opts  map[string]string // maps option name to a single line help
}

// Commander is an interface to wrap the Command function
type Commander interface {
	// Command the backend to run a named command
	//
	// The command run is name
	// args may be used to read arguments from
	// opts may be used to read optional arguments from
	//
	// The result should be capable of being JSON encoded
	// If it is a string or a []string it will be shown to the user
	// otherwise it will be JSON encoded and shown to the user like that
	Command(ctx context.Context, name string, arg []string, opt map[string]string) (interface{}, error)
}

// Shutdowner is an interface to wrap the Shutdown function
type Shutdowner interface {
	// Shutdown the backend, closing any background tasks and any
	// cached connections.
	Shutdown(ctx context.Context) error
}

// ObjectsChan is a channel of Objects
type ObjectsChan chan Object

// Objects is a slice of Object~s
type Objects []Object

// ObjectPair is a pair of Objects used to describe a potential copy
// operation.
type ObjectPair struct {
	Src, Dst Object
}

// UnWrapFs unwraps f as much as possible and returns the base Fs
func UnWrapFs(f Fs) Fs {
	for {
		unwrap := f.Features().UnWrap
		if unwrap == nil {
			break // not a wrapped Fs, use current
		}
		next := unwrap()
		if next == nil {
			break // no base Fs found, use current
		}
		f = next
	}
	return f
}

// UnWrapObject unwraps o as much as possible and returns the base object
func UnWrapObject(o Object) Object {
	for {
		u, ok := o.(ObjectUnWrapper)
		if !ok {
			break // not a wrapped object, use current
		}
		next := u.UnWrap()
		if next == nil {
			break // no base object found, use current
		}
		o = next
	}
	return o
}

// UnWrapObjectInfo returns the underlying Object unwrapped as much as
// possible or nil.
func UnWrapObjectInfo(oi ObjectInfo) Object {
	o, ok := oi.(Object)
	if !ok {
		return nil
	}
	return UnWrapObject(o)
}
