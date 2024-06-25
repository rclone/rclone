//go:build unix

package nfs

import (
	billy "github.com/go-git/go-billy/v5"
	nfshelper "github.com/willscott/go-nfs/helpers"
)

// Cache controls the file handle cache implementation
type Cache interface {
	// ToHandle takes a file and represents it with an opaque handle to reference it.
	// In stateless nfs (when it's serving a unix fs) this can be the device + inode
	// but we can generalize with a stateful local cache of handed out IDs.
	ToHandle(f billy.Filesystem, path []string) []byte

	// FromHandle converts from an opaque handle to the file it represents
	FromHandle(fh []byte) (billy.Filesystem, []string, error)

	// Invalidate the handle passed - used on rename and delete
	InvalidateHandle(fs billy.Filesystem, handle []byte) error

	// HandleLimit exports how many file handles can be safely stored by this cache.
	HandleLimit() int
}

// Set the cache of the handler to the type required by the user
func (h *Handler) setCache() (err error) {
	// The default caching handler
	h.Cache = nfshelper.NewCachingHandler(h, h.opt.HandleLimit)
	return nil
}
