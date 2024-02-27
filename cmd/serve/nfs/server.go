//go:build unix
// +build unix

package nfs

import (
	"context"
	"net"

	nfs "github.com/willscott/go-nfs"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/vfs"
	"github.com/rclone/rclone/vfs/vfscommon"
)

// Server contains everything to run the Server
type Server struct {
	opt                 Options
	handler             nfs.Handler
	ctx                 context.Context // for global config
	listener            net.Listener
	UnmountedExternally bool
}

// NewServer creates a new server
func NewServer(ctx context.Context, vfs *vfs.VFS, opt *Options) (s *Server, err error) {
	if vfs.Opt.CacheMode == vfscommon.CacheModeOff {
		fs.LogPrintf(fs.LogLevelWarning, ctx, "NFS writes don't work without a cache, the filesystem will be served read-only")
	}
	// Our NFS server doesn't have any authentication, we run it on localhost and random port by default
	if opt.ListenAddr == "" {
		opt.ListenAddr = "localhost:"
	}

	s = &Server{
		ctx: ctx,
		opt: *opt,
	}
	s.handler = newHandler(vfs, opt)
	s.listener, err = net.Listen("tcp", s.opt.ListenAddr)
	if err != nil {
		fs.Errorf(nil, "NFS server failed to listen: %v\n", err)
	}
	return
}

// Addr returns the listening address of the server
func (s *Server) Addr() net.Addr {
	return s.listener.Addr()
}

// Shutdown stops the server
func (s *Server) Shutdown() error {
	return s.listener.Close()
}

// Serve starts the server
func (s *Server) Serve() (err error) {
	fs.Logf(nil, "NFS Server running at %s\n", s.listener.Addr())
	return nfs.Serve(s.listener, s.handler)
}
