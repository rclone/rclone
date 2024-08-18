//go:build !plan9

package sftp

import (
	"context"
	"io"
	"net"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/fshttp"
	"github.com/rclone/rclone/lib/proxy"
	"golang.org/x/crypto/ssh"
)

// Internal ssh connections with "golang.org/x/crypto/ssh"

type sshClientInternal struct {
	srv *ssh.Client
}

// newSSHClientInternal starts a client connection to the given SSH server. It is a
// convenience function that connects to the given network address,
// initiates the SSH handshake, and then sets up a Client.
func (f *Fs) newSSHClientInternal(ctx context.Context, network, addr string, sshConfig *ssh.ClientConfig) (sshClient, error) {

	baseDialer := fshttp.NewDialer(ctx)
	var (
		conn net.Conn
		err  error
	)
	if f.opt.SocksProxy != "" {
		conn, err = proxy.SOCKS5Dial(network, addr, f.opt.SocksProxy, baseDialer)
	} else {
		conn, err = baseDialer.Dial(network, addr)
	}
	if err != nil {
		return nil, err
	}
	c, chans, reqs, err := ssh.NewClientConn(conn, addr, sshConfig)
	if err != nil {
		return nil, err
	}
	fs.Debugf(f, "New connection %s->%s to %q", c.LocalAddr(), c.RemoteAddr(), c.ServerVersion())
	srv := ssh.NewClient(c, chans, reqs)
	return sshClientInternal{srv}, nil
}

// Wait for connection to close
func (s sshClientInternal) Wait() error {
	return s.srv.Conn.Wait()
}

// Send a keepalive over the ssh connection
func (s sshClientInternal) SendKeepAlive() {
	_, _, err := s.srv.SendRequest("keepalive@openssh.com", true, nil)
	if err != nil {
		fs.Debugf(nil, "Failed to send keep alive: %v", err)
	}
}

// Close the connection
func (s sshClientInternal) Close() error {
	return s.srv.Close()
}

// CanReuse indicates if this client can be reused
func (s sshClientInternal) CanReuse() bool {
	return true
}

// Check interfaces
var _ sshClient = sshClientInternal{}

// Thin wrapper for *ssh.Session to implement sshSession interface
type sshSessionInternal struct {
	*ssh.Session
}

// Set the stdout
func (s sshSessionInternal) SetStdout(wr io.Writer) {
	s.Session.Stdout = wr
}

// Set the stderr
func (s sshSessionInternal) SetStderr(wr io.Writer) {
	s.Session.Stderr = wr
}

// NewSession makes an sshSession from an sshClient
func (s sshClientInternal) NewSession() (sshSession, error) {
	session, err := s.srv.NewSession()
	if err != nil {
		return nil, err
	}
	return sshSessionInternal{Session: session}, nil
}

// Check interfaces
var _ sshSession = sshSessionInternal{}
