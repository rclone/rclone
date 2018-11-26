// +build go1.9

package restic

import (
	"net"
	"os"
)

// Addr implements net.Addr for stdin/stdout.
type Addr struct{}

// Network returns the network type as a string.
func (a Addr) Network() string {
	return "stdio"
}

func (a Addr) String() string {
	return "stdio"
}

// StdioConn implements a net.Conn via stdin/stdout.
type StdioConn struct {
	stdin  *os.File
	stdout *os.File
}

func (s *StdioConn) Read(p []byte) (int, error) {
	return s.stdin.Read(p)
}

func (s *StdioConn) Write(p []byte) (int, error) {
	return s.stdout.Write(p)
}

// Close closes both streams.
func (s *StdioConn) Close() error {
	err1 := s.stdin.Close()
	err2 := s.stdout.Close()
	if err1 != nil {
		return err1
	}
	return err2
}

// LocalAddr returns nil.
func (s *StdioConn) LocalAddr() net.Addr {
	return Addr{}
}

// RemoteAddr returns nil.
func (s *StdioConn) RemoteAddr() net.Addr {
	return Addr{}
}
