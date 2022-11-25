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

func (s *Server) serve(listener net.Listener, addr, tempFile string) error {
	if tempFile != "" {
		atexit.Register(func() {
			// remove spec file or self-created unix socket
			fs.Debugf(nil, "Removing stale file %s", tempFile)
			_ = os.Remove(tempFile)
		})
	}
	hs := (*http.Server)(s)
	return hs.Serve(listener)
}

// ServeUnix makes the handler to listen for requests in a unix socket.
// It also creates the socket file in the right directory for docker to read.
func (s *Server) ServeUnix(path string, gid int) error {
	listener, socketPath, err := newUnixListener(path, gid)
	if err != nil {
		return err
	}
	if socketPath != "" {
		path = socketPath
		fs.Infof(nil, "Serving unix socket: %s", path)
	} else {
		fs.Infof(nil, "Serving systemd socket")
	}
	return s.serve(listener, path, socketPath)
}

// ServeTCP makes the handler listen for request on a given TCP address.
// It also writes the spec file in the right directory for docker to read.
func (s *Server) ServeTCP(addr, specDir string, tlsConfig *tls.Config, noSpec bool) error {
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return err
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
			return err
		}
	}
	fs.Infof(nil, "Serving TCP socket: %s", addr)
	return s.serve(listener, addr, specFile)
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
