//go:build !windows && !plan9
// +build !windows,!plan9

package sftp

import (
	"path"
)

func (s *Server) toLocalPath(p string) string {
	if s.workDir != "" && !path.IsAbs(p) {
		p = path.Join(s.workDir, p)
	}

	return p
}
