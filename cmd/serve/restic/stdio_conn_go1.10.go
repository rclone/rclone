//+build go1.10

// Deadline setting for go1.10+

package restic

import "time"

// SetDeadline sets the read/write deadline.
func (s *StdioConn) SetDeadline(t time.Time) error {
	err1 := s.stdin.SetReadDeadline(t)
	err2 := s.stdout.SetWriteDeadline(t)
	if err1 != nil {
		return err1
	}
	return err2
}

// SetReadDeadline sets the read/write deadline.
func (s *StdioConn) SetReadDeadline(t time.Time) error {
	return s.stdin.SetReadDeadline(t)
}

// SetWriteDeadline sets the read/write deadline.
func (s *StdioConn) SetWriteDeadline(t time.Time) error {
	return s.stdout.SetWriteDeadline(t)
}
