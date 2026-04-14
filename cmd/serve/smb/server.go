//go:build !windows && !plan9

package smb

import (
	"context"
	"errors"
	"fmt"
	"net"

	smbserver "github.com/macos-fuse-t/go-smb2/server"
	smbvfs "github.com/macos-fuse-t/go-smb2/vfs"
	"github.com/rclone/rclone/cmd/serve/proxy"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/vfs"
	"github.com/rclone/rclone/vfs/vfscommon"
)

// server contains everything to run the SMB server
type server struct {
	f        fs.Fs
	opt      Options
	vfs      *vfs.VFS
	ctx      context.Context // for global config
	proxy    *proxy.Proxy
	listener net.Listener
	srv      *smbserver.Server
	stopped  chan struct{} // for waiting on the server to stop
}

func newServer(ctx context.Context, f fs.Fs, opt *Options, vfsOpt *vfscommon.Options, proxyOpt *proxy.Options) (*server, error) {
	s := &server{
		f:       f,
		ctx:     ctx,
		opt:     *opt,
		stopped: make(chan struct{}),
	}
	if proxyOpt.AuthProxy != "" {
		s.proxy = proxy.New(ctx, proxyOpt, vfsOpt)
	} else {
		s.vfs = vfs.New(ctx, f, vfsOpt)
	}

	if !s.opt.NoAuth && s.opt.User == "" && s.opt.Pass == "" && s.proxy == nil {
		return nil, errors.New("no authorization found, use --user/--pass or --no-auth or --auth-proxy")
	}

	// Listen on the configured address
	var err error
	s.listener, err = net.Listen("tcp", s.opt.ListenAddr)
	if err != nil {
		return nil, fmt.Errorf("failed to listen: %w", err)
	}

	// Create the SMB VFS bridge.
	// When using auth proxy, use a proxy-aware VFS that lazily calls
	// the proxy to get a VFS on the first operation. NTLM doesn't
	// provide plaintext passwords so we can't do per-user auth proxy
	// in the same way as HTTP/SFTP. Instead we allow guest access
	// at the NTLM level and let the proxy create the VFS.
	var smbFS smbvfs.VFSFileSystem
	if s.proxy != nil {
		smbFS = newProxySMBVFS(s.proxy, s.opt.User, s.opt.Pass)
	} else {
		smbFS = newSMBVFS(s.vfs)
	}

	// Create shares map
	shares := map[string]smbvfs.VFSFileSystem{
		s.opt.ShareName: smbFS,
	}

	// Create NTLM authenticator
	auth := &smbserver.NTLMAuthenticator{
		UserPassword: map[string]string{},
		AllowGuest:   s.opt.NoAuth || s.proxy != nil,
	}
	if s.opt.User != "" && s.opt.Pass != "" {
		auth.UserPassword[s.opt.User] = s.opt.Pass
	}

	// Configure the SMB server
	cfg := &smbserver.ServerConfig{
		AllowGuest: s.opt.NoAuth || s.proxy != nil,
	}

	s.srv = smbserver.NewServer(cfg, auth, shares)

	return s, nil
}

// Serve starts the SMB server - blocks until Shutdown is called
func (s *server) Serve() error {
	fs.Logf(nil, "SMB server listening on %v\n", s.listener.Addr())
	err := s.srv.ServeListener(s.listener)
	close(s.stopped)
	return err
}

// Addr returns the address the server is listening on
func (s *server) Addr() net.Addr {
	return s.listener.Addr()
}

// Shutdown stops the SMB server
func (s *server) Shutdown() error {
	s.srv.Shutdown()
	<-s.stopped
	return nil
}
