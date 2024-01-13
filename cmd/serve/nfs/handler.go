//go:build unix
// +build unix

package nfs

import (
	"context"
	"net"

	"github.com/go-git/go-billy/v5"
	"github.com/rclone/rclone/vfs"
	"github.com/willscott/go-nfs"
	nfshelper "github.com/willscott/go-nfs/helpers"
)

// NewBackendAuthHandler creates a handler for the provided filesystem
func NewBackendAuthHandler(vfs *vfs.VFS) nfs.Handler {
	return &BackendAuthHandler{vfs}
}

// BackendAuthHandler returns a NFS backing that exposes a given file system in response to all mount requests.
type BackendAuthHandler struct {
	vfs *vfs.VFS
}

// Mount backs Mount RPC Requests, allowing for access control policies.
func (h *BackendAuthHandler) Mount(ctx context.Context, conn net.Conn, req nfs.MountRequest) (status nfs.MountStatus, hndl billy.Filesystem, auths []nfs.AuthFlavor) {
	status = nfs.MountStatusOk
	hndl = &FS{vfs: h.vfs}
	auths = []nfs.AuthFlavor{nfs.AuthFlavorNull}
	return
}

// Change provides an interface for updating file attributes.
func (h *BackendAuthHandler) Change(fs billy.Filesystem) billy.Change {
	if c, ok := fs.(billy.Change); ok {
		return c
	}
	return nil
}

// FSStat provides information about a filesystem.
func (h *BackendAuthHandler) FSStat(ctx context.Context, f billy.Filesystem, s *nfs.FSStat) error {
	total, _, free := h.vfs.Statfs()
	s.TotalSize = uint64(total)
	s.FreeSize = uint64(free)
	s.AvailableSize = uint64(free)
	return nil
}

// ToHandle handled by CachingHandler
func (h *BackendAuthHandler) ToHandle(f billy.Filesystem, s []string) []byte {
	return []byte{}
}

// FromHandle handled by CachingHandler
func (h *BackendAuthHandler) FromHandle([]byte) (billy.Filesystem, []string, error) {
	return nil, []string{}, nil
}

// HandleLimit handled by cachingHandler
func (h *BackendAuthHandler) HandleLimit() int {
	return -1
}

// InvalidateHandle is called on removes or renames
func (h *BackendAuthHandler) InvalidateHandle(billy.Filesystem, []byte) error {
	return nil
}

func newHandler(vfs *vfs.VFS) nfs.Handler {
	handler := NewBackendAuthHandler(vfs)
	cacheHelper := nfshelper.NewCachingHandler(handler, 1024)
	return cacheHelper
}
