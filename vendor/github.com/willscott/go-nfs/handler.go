package nfs

import (
	"context"
	"io/fs"
	"net"

	billy "github.com/go-git/go-billy/v5"
)

// Handler represents the interface of the file system / vfs being exposed over NFS
type Handler interface {
	// Required methods

	Mount(context.Context, net.Conn, MountRequest) (MountStatus, billy.Filesystem, []AuthFlavor)

	// Change can return 'nil' if filesystem is read-only
	Change(billy.Filesystem) billy.Change

	// Optional methods - generic helpers or trivial implementations can be sufficient depending on use case.

	// Fill in information about a file system's free space.
	FSStat(context.Context, billy.Filesystem, *FSStat) error

	// represent file objects as opaque references
	// Can be safely implemented via helpers/cachinghandler.
	ToHandle(fs billy.Filesystem, path []string) []byte
	FromHandle(fh []byte) (billy.Filesystem, []string, error)
	// How many handles can be safely maintained by the handler.
	HandleLimit() int
}

// CachingHandler represents the optional caching work that a user may wish to over-ride with
// their own implementations, but which can be otherwise provided through defaults.
type CachingHandler interface {
	VerifierFor(path string, contents []fs.FileInfo) uint64

	// fs.FileInfo needs to be sorted by Name(), nil in case of a cache-miss
	DataForVerifier(path string, verifier uint64) []fs.FileInfo
}
