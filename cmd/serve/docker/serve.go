package docker

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"runtime"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/lib/atexit"
	"github.com/rclone/rclone/lib/file"
)

// Server connects plugin with docker daemon by protocol
type Server http.Server

// NewServer creates new docker plugin server
func NewServer(drv *Driver) *Server {
	return &Server{Handler: newRouter(drv)}
}

// Shutdown the server
func (s *Server) Shutdown(ctx context.Context) error {
	hs := (*http.Server)(s)
	return hs.Shutdown(ctx)
}

// Serve requests using the listener
func (s *Server) Serve(listener net.Listener) error {
	hs := (*http.Server)(s)
	return hs.Serve(listener)
}

// ListenUnix returns a unix socket listener.
// It also creates the socket file in the right directory for docker to read.
func (s *Server) ListenUnix(path string, gid int) (net.Listener, error) {
	listener, socketPath, err := newUnixListener(path, gid)
	if err != nil {
		return nil, err
	}
	if socketPath != "" {
		fs.Infof(nil, "Listening on unix socket: %s", socketPath)
		atexit.Register(func() {
			// remove self-created unix socket
			fs.Debugf(nil, "Removing stale unix socket file %s", socketPath)
			_ = os.Remove(socketPath)
		})
	} else {
		fs.Infof(nil, "Listening on systemd socket")
	}
	return listener, nil
}

// ListenTCP returns a TCP listener for the given TCP address.
// It also writes the spec file in the right directory for docker to read.
func (s *Server) ListenTCP(addr, specDir string, tlsConfig *tls.Config, noSpec bool) (net.Listener, error) {
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, err
	}
	if tlsConfig != nil {
		tlsConfig.NextProtos = []string{"http/1.1"}
		listener = tls.NewListener(listener, tlsConfig)
	}
	addr = listener.Addr().String()
	specFile := ""
	if !noSpec {
		specFile, err = writeSpecFile(addr, "tcp", specDir)
		if err != nil {
			return nil, err
		}
		atexit.Register(func() {
			// remove spec file
			fs.Debugf(nil, "Removing stale spec file %s", specFile)
			_ = os.Remove(specFile)
		})
	}
	fs.Infof(nil, "Listening on TCP socket: %s", addr)
	return listener, nil
}

func writeSpecFile(addr, proto, specDir string) (string, error) {
	if specDir == "" && runtime.GOOS == "windows" {
		specDir = os.TempDir()
	}
	if specDir == "" {
		specDir = defSpecDir
	}
	if err := file.MkdirAll(specDir, 0755); err != nil {
		return "", err
	}
	specFile := filepath.Join(specDir, "rclone.spec")
	url := fmt.Sprintf("%s://%s", proto, addr)
	if err := os.WriteFile(specFile, []byte(url), 0644); err != nil {
		return "", err
	}
	fs.Debugf(nil, "Plugin spec has been written to %s", specFile)
	return specFile, nil
}
