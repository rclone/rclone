package helpers

import (
	"context"
	"net"

	"github.com/go-git/go-billy/v5"
	"github.com/willscott/go-nfs"
)

// NewNullAuthHandler creates a handler for the provided filesystem
func NewNullAuthHandler(fs billy.Filesystem) nfs.Handler {
	return &NullAuthHandler{fs}
}

// NullAuthHandler returns a NFS backing that exposes a given file system in response to all mount requests.
type NullAuthHandler struct {
	fs billy.Filesystem
}

// Mount backs Mount RPC Requests, allowing for access control policies.
func (h *NullAuthHandler) Mount(ctx context.Context, conn net.Conn, req nfs.MountRequest) (status nfs.MountStatus, hndl billy.Filesystem, auths []nfs.AuthFlavor) {
	status = nfs.MountStatusOk
	hndl = h.fs
	auths = []nfs.AuthFlavor{nfs.AuthFlavorNull}
	return
}

// Change provides an interface for updating file attributes.
func (h *NullAuthHandler) Change(fs billy.Filesystem) billy.Change {
	if c, ok := h.fs.(billy.Change); ok {
		return c
	}
	return nil
}

// FSStat provides information about a filesystem.
func (h *NullAuthHandler) FSStat(ctx context.Context, f billy.Filesystem, s *nfs.FSStat) error {
	return nil
}

// ToHandle handled by CachingHandler
func (h *NullAuthHandler) ToHandle(f billy.Filesystem, s []string) []byte {
	return []byte{}
}

// FromHandle handled by CachingHandler
func (h *NullAuthHandler) FromHandle([]byte) (billy.Filesystem, []string, error) {
	return nil, []string{}, nil
}

// HandleLImit handled by cachingHandler
func (h *NullAuthHandler) HandleLimit() int {
	return -1
}
