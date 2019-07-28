// Package fs is a generic file system interface for rclone object storage systems
package fs

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"time"

	"github.com/pkg/errors"
	"github.com/rclone/rclone/fs/config/configmap"
	"github.com/rclone/rclone/fs/config/configstruct"
	"github.com/rclone/rclone/fs/fserrors"
	"github.com/rclone/rclone/fs/fspath"
	"github.com/rclone/rclone/fs/hash"
	"github.com/rclone/rclone/lib/pacer"
)

// EntryType can be associated with remote paths to identify their type
type EntryType int

// Constants
const (
	// ModTimeNotSupported is a very large precision value to show
	// mod time isn't supported on this Fs
	ModTimeNotSupported = 100 * 365 * 24 * time.Hour
	// MaxLevel is a sentinel representing an infinite depth for listings
	MaxLevel = math.MaxInt32
	// EntryDirectory should be used to classify remote paths in directories
	EntryDirectory EntryType = iota // 0
	// EntryObject should be used to classify remote paths in objects
	EntryObject // 1
)

// Globals
var (
	// Filesystem registry
	Registry []*RegInfo
	// ErrorNotFoundInConfigFile is returned by NewFs if not found in config file
	ErrorNotFoundInConfigFile        = errors.New("didn't find section in config file")
	ErrorCantPurge                   = errors.New("can't purge directory")
	ErrorCantCopy                    = errors.New("can't copy object - incompatible remotes")
	ErrorCantMove                    = errors.New("can't move object - incompatible remotes")
	ErrorCantDirMove                 = errors.New("can't move directory - incompatible remotes")
	ErrorCantUploadEmptyFiles        = errors.New("can't upload empty files to this remote")
	ErrorDirExists                   = errors.New("can't copy directory - destination already exists")
	ErrorCantSetModTime              = errors.New("can't set modified time")
	ErrorCantSetModTimeWithoutDelete = errors.New("can't set modified time without deleting existing object")
	ErrorDirNotFound                 = errors.New("directory not found")
	ErrorObjectNotFound              = errors.New("object not found")
	ErrorLevelNotSupported           = errors.New("level value not supported")
	ErrorListAborted                 = errors.New("list aborted")
	ErrorListBucketRequired          = errors.New("bucket or container name is needed in remote")
	ErrorIsFile                      = errors.New("is a file not a directory")
	ErrorNotAFile                    = errors.New("is a not a regular file")
	ErrorNotDeleting                 = errors.New("not deleting files as there were IO errors")
	ErrorNotDeletingDirs             = errors.New("not deleting directories as there were IO errors")
	ErrorOverlapping                 = errors.New("can't sync or move files on overlapping remotes")
	ErrorDirectoryNotEmpty           = errors.New("directory not empty")
	ErrorImmutableModified           = errors.New("immutable file modified")
	ErrorPermissionDenied            = errors.New("permission denied")
	ErrorCantShareDirectories        = errors.New("this backend can't share directories with link")
)

// RegInfo provides information about a filesystem
type RegInfo struct {
	// Name of this fs
	Name string
	// Description of this fs - defaults to Name
	Description string
	// Prefix for command line flags for this fs - defaults to Name if not set
	Prefix string
	// Create a new file system.  If root refers to an existing
	// object, then it should return a Fs which which points to
	// the parent of that object and ErrorIsFile.
	NewFs func(name string, root string, config configmap.Mapper) (Fs, error) `json:"-"`
	// Function to call to help with config
	Config func(name string, config configmap.Mapper) `json:"-"`
	// Options for the Fs configuration
	Options Options
}

// FileName returns the on disk file name for this backend
func (ri *RegInfo) FileName() string {
	return strings.Replace(ri.Name, " ", "", -1)
}

// Options is a slice of configuration Option for a backend
type Options []Option

// Set the default values for the options
func (os Options) setValues() {
	for i := range os {
		o := &os[i]
		if o.Default == nil {
			o.Default = ""
		}
	}
}

// OptionVisibility controls whether the options are visible in the
// configurator or the command line.
type OptionVisibility byte

// Constants Option.Hide
const (
	OptionHideCommandLine OptionVisibility = 1 << iota
	OptionHideConfigurator
	OptionHideBoth = OptionHideCommandLine | OptionHideConfigurator
)

// Option is describes an option for the config wizard
//
// This also describes command line options and environment variables
type Option struct {
	Name       string           // name of the option in snake_case
	Help       string           // Help, the first line only is used for the command line help
	Provider   string           // Set to filter on provider
	Default    interface{}      // default value, nil => ""
	Value      interface{}      // value to be set by flags
	Examples   OptionExamples   `json:",omitempty"` // config examples
	ShortOpt   string           // the short option for this if required
	Hide       OptionVisibility // set this to hide the config from the configurator or the command line
	Required   bool             // this option is required
	IsPassword bool             // set if the option is a password
	NoPrefix   bool             // set if the option for this should not use the backend prefix
	Advanced   bool             // set if this is an advanced config option
}

// BaseOption is an alias for Option used internally
type BaseOption Option

// MarshalJSON turns an Option into JSON
//
// It adds some generated fields for ease of use
// - DefaultStr - a string rendering of Default
// - ValueStr - a string rendering of Value
// - Type - the type of the option
func (o *Option) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		BaseOption
		DefaultStr string
		ValueStr   string
		Type       string
	}{
		BaseOption: BaseOption(*o),
		DefaultStr: fmt.Sprint(o.Default),
		ValueStr:   o.String(),
		Type:       o.Type(),
	})
}

// GetValue gets the current current value which is the default if not set
func (o *Option) GetValue() interface{} {
	val := o.Value
	if val == nil {
		val = o.Default
	}
	return val
}

// String turns Option into a string
func (o *Option) String() string {
	return fmt.Sprint(o.GetValue())
}

// Set a Option from a string
func (o *Option) Set(s string) (err error) {
	newValue, err := configstruct.StringToInterface(o.GetValue(), s)
	if err != nil {
		return err
	}
	o.Value = newValue
	return nil
}

// Type of the value
func (o *Option) Type() string {
	return reflect.TypeOf(o.GetValue()).Name()
}

// FlagName for the option
func (o *Option) FlagName(prefix string) string {
	name := strings.Replace(o.Name, "_", "-", -1) // convert snake_case to kebab-case
	if !o.NoPrefix {
		name = prefix + "-" + name
	}
	return name
}

// EnvVarName for the option
func (o *Option) EnvVarName(prefix string) string {
	return OptionToEnv(prefix + "-" + o.Name)
}

// OptionExamples is a slice of examples
type OptionExamples []OptionExample

// Len is part of sort.Interface.
func (os OptionExamples) Len() int { return len(os) }

// Swap is part of sort.Interface.
func (os OptionExamples) Swap(i, j int) { os[i], os[j] = os[j], os[i] }

// Less is part of sort.Interface.
func (os OptionExamples) Less(i, j int) bool { return os[i].Help < os[j].Help }

// Sort sorts an OptionExamples
func (os OptionExamples) Sort() { sort.Sort(os) }

// OptionExample describes an example for an Option
type OptionExample struct {
	Value    string
	Help     string
	Provider string
}

// Register a filesystem
//
// Fs modules  should use this in an init() function
func Register(info *RegInfo) {
	info.Options.setValues()
	if info.Prefix == "" {
		info.Prefix = info.Name
	}
	Registry = append(Registry, info)
}

// Fs is the interface a cloud storage system must provide
type Fs interface {
	Info

	// List the objects and directories in dir into entries.  The
	// entries can be returned in any order but should be for a
	// complete directory.
	//
	// dir should be "" to list the root, and should not have
	// trailing slashes.
	//
	// This should return ErrDirNotFound if the directory isn't
	// found.
	List(ctx context.Context, dir string) (entries DirEntries, err error)

	// NewObject finds the Object at remote.  If it can't be found
	// it returns the error ErrorObjectNotFound.
	NewObject(ctx context.Context, remote string) (Object, error)

	// Put in to the remote path with the modTime given of the given size
	//
	// When called from outside a Fs by rclone, src.Size() will always be >= 0.
	// But for unknown-sized objects (indicated by src.Size() == -1), Put should either
	// return an error or upload it properly (rather than e.g. calling panic).
	//
	// May create the object even if it returns an error - if so
	// will return the object and the error, otherwise will return
	// nil and the error
	Put(ctx context.Context, in io.Reader, src ObjectInfo, options ...OpenOption) (Object, error)

	// Mkdir makes the directory (container, bucket)
	//
	// Shouldn't return an error if it already exists
	Mkdir(ctx context.Context, dir string) error

	// Rmdir removes the directory (container, bucket) if empty
	//
	// Return an error if it doesn't exist or isn't empty
	Rmdir(ctx context.Context, dir string) error
}

// Info provides a read only interface to information about a filesystem.
type Info interface {
	// Name of the remote (as passed into NewFs)
	Name() string

	// Root of the remote (as passed into NewFs)
	Root() string

	// String returns a description of the FS
	String() string

	// Precision of the ModTimes in this Fs
	Precision() time.Duration

	// Returns the supported hash types of the filesystem
	Hashes() hash.Set

	// Features returns the optional features of this Fs
	Features() *Features
}

// Object is a filesystem like object provided by an Fs
type Object interface {
	ObjectInfo

	// SetModTime sets the metadata on the object to set the modification date
	SetModTime(ctx context.Context, t time.Time) error

	// Open opens the file for read.  Call Close() on the returned io.ReadCloser
	Open(ctx context.Context, options ...OpenOption) (io.ReadCloser, error)

	// Update in to the object with the modTime given of the given size
	//
	// When called from outside a Fs by rclone, src.Size() will always be >= 0.
	// But for unknown-sized objects (indicated by src.Size() == -1), Upload should either
	// return an error or update the object properly (rather than e.g. calling panic).
	Update(ctx context.Context, in io.Reader, src ObjectInfo, options ...OpenOption) error

	// Removes this object
	Remove(ctx context.Context) error
}

// ObjectInfo provides read only information about an object.
type ObjectInfo interface {
	DirEntry

	// Fs returns read only access to the Fs that this object is part of
	Fs() Info

	// Hash returns the selected checksum of the file
	// If no checksum is available it returns ""
	Hash(ctx context.Context, ty hash.Type) (string, error)

	// Storable says whether this object can be stored
	Storable() bool
}

// DirEntry provides read only information about the common subset of
// a Dir or Object.  These are returned from directory listings - type
// assert them into the correct type.
type DirEntry interface {
	// String returns a description of the Object
	String() string

	// Remote returns the remote path
	Remote() string

	// ModTime returns the modification date of the file
	// It should return a best guess if one isn't available
	ModTime(context.Context) time.Time

	// Size returns the size of the file
	Size() int64
}

// Directory is a filesystem like directory provided by an Fs
type Directory interface {
	DirEntry

	// Items returns the count of items in this directory or this
	// directory and subdirectories if known, -1 for unknown
	Items() int64

	// ID returns the internal ID of this directory if known, or
	// "" otherwise
	ID() string
}

// MimeTyper is an optional interface for Object
type MimeTyper interface {
	// MimeType returns the content type of the Object if
	// known, or "" if not
	MimeType(ctx context.Context) string
}

// IDer is an optional interface for Object
type IDer interface {
	// ID returns the ID of the Object if known, or "" if not
	ID() string
}

// ObjectUnWrapper is an optional interface for Object
type ObjectUnWrapper interface {
	// UnWrap returns the Object that this Object is wrapping or
	// nil if it isn't wrapping anything
	UnWrap() Object
}

// SetTierer is an optional interface for Object
type SetTierer interface {
	// SetTier performs changing storage tier of the Object if
	// multiple storage classes supported
	SetTier(tier string) error
}

// GetTierer is an optional interface for Object
type GetTierer interface {
	// GetTier returns storage tier or class of the Object
	GetTier() string
}

// ObjectOptionalInterfaces returns the names of supported and
// unsupported optional interfaces for an Object
func ObjectOptionalInterfaces(o Object) (supported, unsupported []string) {
	store := func(ok bool, name string) {
		if ok {
			supported = append(supported, name)
		} else {
			unsupported = append(unsupported, name)
		}
	}

	_, ok := o.(MimeTyper)
	store(ok, "MimeType")

	_, ok = o.(IDer)
	store(ok, "ID")

	_, ok = o.(ObjectUnWrapper)
	store(ok, "UnWrap")

	_, ok = o.(SetTierer)
	store(ok, "SetTier")

	_, ok = o.(GetTierer)
	store(ok, "GetTier")

	return supported, unsupported
}

// ListRCallback defines a callback function for ListR to use
//
// It is called for each tranche of entries read from the listing and
// if it returns an error, the listing stops.
type ListRCallback func(entries DirEntries) error

// ListRFn is defines the call used to recursively list a directory
type ListRFn func(ctx context.Context, dir string, callback ListRCallback) error

// NewUsageValue makes a valid value
func NewUsageValue(value int64) *int64 {
	p := new(int64)
	*p = value
	return p
}

// Usage is returned by the About call
//
// If a value is nil then it isn't supported by that backend
type Usage struct {
	Total   *int64 `json:"total,omitempty"`   // quota of bytes that can be used
	Used    *int64 `json:"used,omitempty"`    // bytes in use
	Trashed *int64 `json:"trashed,omitempty"` // bytes in trash
	Other   *int64 `json:"other,omitempty"`   // other usage eg gmail in drive
	Free    *int64 `json:"free,omitempty"`    // bytes which can be uploaded before reaching the quota
	Objects *int64 `json:"objects,omitempty"` // objects in the storage system
}

// WriterAtCloser wraps io.WriterAt and io.Closer
type WriterAtCloser interface {
	io.WriterAt
	io.Closer
}

// Features describe the optional features of the Fs
type Features struct {
	// Feature flags, whether Fs
	CaseInsensitive         bool // has case insensitive files
	DuplicateFiles          bool // allows duplicate files
	ReadMimeType            bool // can read the mime type of objects
	WriteMimeType           bool // can set the mime type of objects
	CanHaveEmptyDirectories bool // can have empty directories
	BucketBased             bool // is bucket based (like s3, swift etc)
	SetTier                 bool // allows set tier functionality on objects
	GetTier                 bool // allows to retrieve storage tier of objects
	ServerSideAcrossConfigs bool // can server side copy between different remotes of the same type

	// Purge all files in the root and the root directory
	//
	// Implement this if you have a way of deleting all the files
	// quicker than just running Remove() on the result of List()
	//
	// Return an error if it doesn't exist
	Purge func(ctx context.Context) error

	// Copy src to this remote using server side copy operations.
	//
	// This is stored with the remote path given
	//
	// It returns the destination Object and a possible error
	//
	// Will only be called if src.Fs().Name() == f.Name()
	//
	// If it isn't possible then return fs.ErrorCantCopy
	Copy func(ctx context.Context, src Object, remote string) (Object, error)

	// Move src to this remote using server side move operations.
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
	// using server side move operations.
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
	PublicLink func(ctx context.Context, remote string) (string, error)

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
func (ft *Features) Fill(f Fs) *Features {
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
	return ft.DisableList(Config.DisableFeatures)
}

// Mask the Features with the Fs passed in
//
// Only optional features which are implemented in both the original
// Fs AND the one passed in will be advertised.  Any features which
// aren't in both will be set to false/nil, except for UnWrap/Wrap which
// will be left untouched.
func (ft *Features) Mask(f Fs) *Features {
	mask := f.Features()
	ft.CaseInsensitive = ft.CaseInsensitive && mask.CaseInsensitive
	ft.DuplicateFiles = ft.DuplicateFiles && mask.DuplicateFiles
	ft.ReadMimeType = ft.ReadMimeType && mask.ReadMimeType
	ft.WriteMimeType = ft.WriteMimeType && mask.WriteMimeType
	ft.CanHaveEmptyDirectories = ft.CanHaveEmptyDirectories && mask.CanHaveEmptyDirectories
	ft.BucketBased = ft.BucketBased && mask.BucketBased
	ft.SetTier = ft.SetTier && mask.SetTier
	ft.GetTier = ft.GetTier && mask.GetTier

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
	return ft.DisableList(Config.DisableFeatures)
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
	// Purge all files in the root and the root directory
	//
	// Implement this if you have a way of deleting all the files
	// quicker than just running Remove() on the result of List()
	//
	// Return an error if it doesn't exist
	Purge(ctx context.Context) error
}

// Copier is an optional interface for Fs
type Copier interface {
	// Copy src to this remote using server side copy operations.
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
	// Move src to this remote using server side move operations.
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
	// using server side move operations.
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
	PublicLink(ctx context.Context, remote string) (string, error)
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

// ObjectsChan is a channel of Objects
type ObjectsChan chan Object

// Objects is a slice of Object~s
type Objects []Object

// ObjectPair is a pair of Objects used to describe a potential copy
// operation.
type ObjectPair struct {
	Src, Dst Object
}

// Find looks for an RegInfo object for the name passed in.  The name
// can be either the Name or the Prefix.
//
// Services are looked up in the config file
func Find(name string) (*RegInfo, error) {
	for _, item := range Registry {
		if item.Name == name || item.Prefix == name || item.FileName() == name {
			return item, nil
		}
	}
	return nil, errors.Errorf("didn't find backend called %q", name)
}

// MustFind looks for an Info object for the type name passed in
//
// Services are looked up in the config file
//
// Exits with a fatal error if not found
func MustFind(name string) *RegInfo {
	fs, err := Find(name)
	if err != nil {
		log.Fatalf("Failed to find remote: %v", err)
	}
	return fs
}

// ParseRemote deconstructs a path into configName, fsPath, looking up
// the fsName in the config file (returning NotFoundInConfigFile if not found)
func ParseRemote(path string) (fsInfo *RegInfo, configName, fsPath string, err error) {
	configName, fsPath = fspath.Parse(path)
	var fsName string
	var ok bool
	if configName != "" {
		if strings.HasPrefix(configName, ":") {
			fsName = configName[1:]
		} else {
			m := ConfigMap(nil, configName)
			fsName, ok = m.Get("type")
			if !ok {
				return nil, "", "", ErrorNotFoundInConfigFile
			}
		}
	} else {
		fsName = "local"
		configName = "local"
	}
	fsInfo, err = Find(fsName)
	return fsInfo, configName, fsPath, err
}

// A configmap.Getter to read from the environment RCLONE_CONFIG_backend_option_name
type configEnvVars string

// Get a config item from the environment variables if possible
func (configName configEnvVars) Get(key string) (value string, ok bool) {
	return os.LookupEnv(ConfigToEnv(string(configName), key))
}

// A configmap.Getter to read from the environment RCLONE_option_name
type optionEnvVars string

// Get a config item from the option environment variables if possible
func (prefix optionEnvVars) Get(key string) (value string, ok bool) {
	return os.LookupEnv(OptionToEnv(string(prefix) + "-" + key))
}

// A configmap.Getter to read either the default value or the set
// value from the RegInfo.Options
type regInfoValues struct {
	fsInfo     *RegInfo
	useDefault bool
}

// override the values in configMap with the either the flag values or
// the default values
func (r *regInfoValues) Get(key string) (value string, ok bool) {
	for i := range r.fsInfo.Options {
		o := &r.fsInfo.Options[i]
		if o.Name == key {
			if r.useDefault || o.Value != nil {
				return o.String(), true
			}
			break
		}
	}
	return "", false
}

// A configmap.Setter to read from the config file
type setConfigFile string

// Set a config item into the config file
func (section setConfigFile) Set(key, value string) {
	Debugf(nil, "Saving config %q = %q in section %q of the config file", key, value, section)
	ConfigFileSet(string(section), key, value)
}

// A configmap.Getter to read from the config file
type getConfigFile string

// Get a config item from the config file
func (section getConfigFile) Get(key string) (value string, ok bool) {
	value, ok = ConfigFileGet(string(section), key)
	// Ignore empty lines in the config file
	if value == "" {
		ok = false
	}
	return value, ok
}

// ConfigMap creates a configmap.Map from the *RegInfo and the
// configName passed in.
//
// If fsInfo is nil then the returned configmap.Map should only be
// used for reading non backend specific parameters, such as "type".
func ConfigMap(fsInfo *RegInfo, configName string) (config *configmap.Map) {
	// Create the config
	config = configmap.New()

	// Read the config, more specific to least specific

	// flag values
	if fsInfo != nil {
		config.AddGetter(&regInfoValues{fsInfo, false})
	}

	// remote specific environment vars
	config.AddGetter(configEnvVars(configName))

	// backend specific environment vars
	if fsInfo != nil {
		config.AddGetter(optionEnvVars(fsInfo.Prefix))
	}

	// config file
	config.AddGetter(getConfigFile(configName))

	// default values
	if fsInfo != nil {
		config.AddGetter(&regInfoValues{fsInfo, true})
	}

	// Set Config
	config.AddSetter(setConfigFile(configName))
	return config
}

// ConfigFs makes the config for calling NewFs with.
//
// It parses the path which is of the form remote:path
//
// Remotes are looked up in the config file.  If the remote isn't
// found then NotFoundInConfigFile will be returned.
func ConfigFs(path string) (fsInfo *RegInfo, configName, fsPath string, config *configmap.Map, err error) {
	// Parse the remote path
	fsInfo, configName, fsPath, err = ParseRemote(path)
	if err != nil {
		return
	}
	config = ConfigMap(fsInfo, configName)
	return
}

// NewFs makes a new Fs object from the path
//
// The path is of the form remote:path
//
// Remotes are looked up in the config file.  If the remote isn't
// found then NotFoundInConfigFile will be returned.
//
// On Windows avoid single character remote names as they can be mixed
// up with drive letters.
func NewFs(path string) (Fs, error) {
	fsInfo, configName, fsPath, config, err := ConfigFs(path)
	if err != nil {
		return nil, err
	}
	return fsInfo.NewFs(configName, fsPath, config)
}

// TemporaryLocalFs creates a local FS in the OS's temporary directory.
//
// No cleanup is performed, the caller must call Purge on the Fs themselves.
func TemporaryLocalFs() (Fs, error) {
	path, err := ioutil.TempDir("", "rclone-spool")
	if err == nil {
		err = os.Remove(path)
	}
	if err != nil {
		return nil, err
	}
	path = filepath.ToSlash(path)
	return NewFs(path)
}

// CheckClose is a utility function used to check the return from
// Close in a defer statement.
func CheckClose(c io.Closer, err *error) {
	cerr := c.Close()
	if *err == nil {
		*err = cerr
	}
}

// FileExists returns true if a file remote exists.
// If remote is a directory, FileExists returns false.
func FileExists(ctx context.Context, fs Fs, remote string) (bool, error) {
	_, err := fs.NewObject(ctx, remote)
	if err != nil {
		if err == ErrorObjectNotFound || err == ErrorNotAFile || err == ErrorPermissionDenied {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// GetModifyWindow calculates the maximum modify window between the given Fses
// and the Config.ModifyWindow parameter.
func GetModifyWindow(fss ...Info) time.Duration {
	window := Config.ModifyWindow
	for _, f := range fss {
		if f != nil {
			precision := f.Precision()
			if precision == ModTimeNotSupported {
				return ModTimeNotSupported
			}
			if precision > window {
				window = precision
			}
		}
	}
	return window
}

// Pacer is a simple wrapper around a pacer.Pacer with logging.
type Pacer struct {
	*pacer.Pacer
}

type logCalculator struct {
	pacer.Calculator
}

// NewPacer creates a Pacer for the given Fs and Calculator.
func NewPacer(c pacer.Calculator) *Pacer {
	p := &Pacer{
		Pacer: pacer.New(
			pacer.InvokerOption(pacerInvoker),
			pacer.MaxConnectionsOption(Config.Checkers+Config.Transfers),
			pacer.RetriesOption(Config.LowLevelRetries),
			pacer.CalculatorOption(c),
		),
	}
	p.SetCalculator(c)
	return p
}

func (d *logCalculator) Calculate(state pacer.State) time.Duration {
	oldSleepTime := state.SleepTime
	newSleepTime := d.Calculator.Calculate(state)
	if state.ConsecutiveRetries > 0 {
		if newSleepTime != oldSleepTime {
			Debugf("pacer", "Rate limited, increasing sleep to %v", newSleepTime)
		}
	} else {
		if newSleepTime != oldSleepTime {
			Debugf("pacer", "Reducing sleep to %v", newSleepTime)
		}
	}
	return newSleepTime
}

// SetCalculator sets the pacing algorithm. Don't modify the Calculator object
// afterwards, use the ModifyCalculator method when needed.
//
// It will choose the default algorithm if nil is passed in.
func (p *Pacer) SetCalculator(c pacer.Calculator) {
	switch c.(type) {
	case *logCalculator:
		Logf("pacer", "Invalid Calculator in fs.Pacer.SetCalculator")
	case nil:
		c = &logCalculator{pacer.NewDefault()}
	default:
		c = &logCalculator{c}
	}

	p.Pacer.SetCalculator(c)
}

// ModifyCalculator calls the given function with the currently configured
// Calculator and the Pacer lock held.
func (p *Pacer) ModifyCalculator(f func(pacer.Calculator)) {
	p.ModifyCalculator(func(c pacer.Calculator) {
		switch _c := c.(type) {
		case *logCalculator:
			f(_c.Calculator)
		default:
			Logf("pacer", "Invalid Calculator in fs.Pacer: %t", c)
			f(c)
		}
	})
}

func pacerInvoker(try, retries int, f pacer.Paced) (retry bool, err error) {
	retry, err = f()
	if retry {
		Debugf("pacer", "low level retry %d/%d (error %v)", try, retries, err)
		err = fserrors.RetryError(err)
	}
	return
}
