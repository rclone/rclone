// Filesystem related types and interfaces
// Note that optional interfaces are found in features.go

package fs

import (
	"context"
	"encoding/json"
	"io"
	"time"

	"github.com/rclone/rclone/fs/hash"
)

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
	//
	// If remote points to a directory then it should return
	// ErrorIsDir if possible without doing any extra work,
	// otherwise ErrorObjectNotFound.
	NewObject(ctx context.Context, remote string) (Object, error)

	// Put in to the remote path with the modTime given of the given size
	//
	// When called from outside an Fs by rclone, src.Size() will always be >= 0.
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
	// When called from outside an Fs by rclone, src.Size() will always be >= 0.
	// But for unknown-sized objects (indicated by src.Size() == -1), Upload should either
	// return an error or update the object properly (rather than e.g. calling panic).
	Update(ctx context.Context, in io.Reader, src ObjectInfo, options ...OpenOption) error

	// Removes this object
	Remove(ctx context.Context) error
}

// ObjectInfo provides read only information about an object.
type ObjectInfo interface {
	DirEntry

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
	// Fs returns read only access to the Fs that this object is part of
	Fs() Info

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

// FullDirectory contains all the optional interfaces for Directory
//
// Use for checking making wrapping Directories implement everything
type FullDirectory interface {
	Directory
	Metadataer
	SetMetadataer
	SetModTimer
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

// ParentIDer is an optional interface for Object
type ParentIDer interface {
	// ParentID returns the ID of the parent directory if known or nil if not
	ParentID() string
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

// Metadataer is an optional interface for DirEntry
type Metadataer interface {
	// Metadata returns metadata for an DirEntry
	//
	// It should return nil if there is no Metadata
	Metadata(ctx context.Context) (Metadata, error)
}

// SetMetadataer is an optional interface for DirEntry
type SetMetadataer interface {
	// SetMetadata sets metadata for an DirEntry
	//
	// It should return fs.ErrorNotImplemented if it can't set metadata
	SetMetadata(ctx context.Context, metadata Metadata) error
}

// SetModTimer is an optional interface for Directory.
//
// Object implements this as part of its requires set of interfaces.
type SetModTimer interface {
	// SetModTime sets the metadata on the DirEntry to set the modification date
	//
	// If there is any other metadata it does not overwrite it.
	SetModTime(ctx context.Context, t time.Time) error
}

// FullObjectInfo contains all the read-only optional interfaces
//
// Use for checking making wrapping ObjectInfos implement everything
type FullObjectInfo interface {
	ObjectInfo
	MimeTyper
	IDer
	ObjectUnWrapper
	GetTierer
	Metadataer
}

// FullObject contains all the optional interfaces for Object
//
// Use for checking making wrapping Objects implement everything
type FullObject interface {
	Object
	MimeTyper
	IDer
	ObjectUnWrapper
	GetTierer
	SetTierer
	Metadataer
	SetMetadataer
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

	_, ok = o.(Metadataer)
	store(ok, "Metadata")

	_, ok = o.(SetMetadataer)
	store(ok, "SetMetadata")

	return supported, unsupported
}

// DirectoryOptionalInterfaces returns the names of supported and
// unsupported optional interfaces for a Directory
func DirectoryOptionalInterfaces(d Directory) (supported, unsupported []string) {
	store := func(ok bool, name string) {
		if ok {
			supported = append(supported, name)
		} else {
			unsupported = append(unsupported, name)
		}
	}

	_, ok := d.(Metadataer)
	store(ok, "Metadata")

	_, ok = d.(SetMetadataer)
	store(ok, "SetMetadata")

	_, ok = d.(SetModTimer)
	store(ok, "SetModTime")

	return supported, unsupported
}

// ListRCallback defines a callback function for ListR to use
//
// It is called for each tranche of entries read from the listing and
// if it returns an error, the listing stops.
type ListRCallback func(entries DirEntries) error

// ListRFn is defines the call used to recursively list a directory
type ListRFn func(ctx context.Context, dir string, callback ListRCallback) error

// Flagger describes the interface rclone config types flags must satisfy
type Flagger interface {
	// These are from pflag.Value which we don't want to pull in here
	String() string
	Set(string) error
	Type() string
	json.Unmarshaler
}

// FlaggerNP describes the interface rclone config types flags must
// satisfy as non-pointers
//
// These are from pflag.Value and need to be tested against
// non-pointer value due the the way the backend flags are inserted
// into the flags.
type FlaggerNP interface {
	String() string
	Type() string
}

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
	Other   *int64 `json:"other,omitempty"`   // other usage e.g. gmail in drive
	Free    *int64 `json:"free,omitempty"`    // bytes which can be uploaded before reaching the quota
	Objects *int64 `json:"objects,omitempty"` // objects in the storage system
}

// WriterAtCloser wraps io.WriterAt and io.Closer
type WriterAtCloser interface {
	io.WriterAt
	io.Closer
}

type unknownFs struct{}

// Name of the remote (as passed into NewFs)
func (unknownFs) Name() string { return "unknown" }

// Root of the remote (as passed into NewFs)
func (unknownFs) Root() string { return "" }

// String returns a description of the FS
func (unknownFs) String() string { return "unknown" }

// Precision of the ModTimes in this Fs
func (unknownFs) Precision() time.Duration { return ModTimeNotSupported }

// Returns the supported hash types of the filesystem
func (unknownFs) Hashes() hash.Set { return hash.Set(hash.None) }

// Features returns the optional features of this Fs
func (unknownFs) Features() *Features { return &Features{} }

// Unknown holds an Info for an unknown Fs
//
// This is used when we need an Fs but don't have one.
var Unknown Info = unknownFs{}
