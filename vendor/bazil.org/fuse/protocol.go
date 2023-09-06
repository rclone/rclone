package fuse

import (
	"fmt"
)

// Protocol is a FUSE protocol version number.
type Protocol struct {
	Major uint32
	Minor uint32
}

func (p Protocol) String() string {
	return fmt.Sprintf("%d.%d", p.Major, p.Minor)
}

// LT returns whether a is less than b.
func (a Protocol) LT(b Protocol) bool {
	return a.Major < b.Major ||
		(a.Major == b.Major && a.Minor < b.Minor)
}

// GE returns whether a is greater than or equal to b.
func (a Protocol) GE(b Protocol) bool {
	return a.Major > b.Major ||
		(a.Major == b.Major && a.Minor >= b.Minor)
}

// HasAttrBlockSize returns whether Attr.BlockSize is respected by the
// kernel.
//
// Deprecated: Guaranteed to be true with our minimum supported
// protocol version.
func (a Protocol) HasAttrBlockSize() bool {
	return true
}

// HasReadWriteFlags returns whether ReadRequest/WriteRequest
// fields Flags and FileFlags are valid.
//
// Deprecated: Guaranteed to be true with our minimum supported
// protocol version.
func (a Protocol) HasReadWriteFlags() bool {
	return true
}

// HasGetattrFlags returns whether GetattrRequest field Flags is
// valid.
//
// Deprecated: Guaranteed to be true with our minimum supported
// protocol version.
func (a Protocol) HasGetattrFlags() bool {
	return true
}

// HasOpenNonSeekable returns whether OpenResponse field Flags flag
// OpenNonSeekable is supported.
//
// Deprecated: Guaranteed to be true with our minimum supported
// protocol version.
func (a Protocol) HasOpenNonSeekable() bool {
	return true
}

// HasUmask returns whether CreateRequest/MkdirRequest/MknodRequest
// field Umask is valid.
//
// Deprecated: Guaranteed to be true with our minimum supported
// protocol version.
func (a Protocol) HasUmask() bool {
	return true
}

// HasInvalidate returns whether InvalidateNode/InvalidateEntry are
// supported.
//
// Deprecated: Guaranteed to be true with our minimum supported
// protocol version.
func (a Protocol) HasInvalidate() bool {
	return true
}

// HasNotifyDelete returns whether NotifyDelete is supported.
func (a Protocol) HasNotifyDelete() bool {
	return a.GE(Protocol{7, 18})
}
