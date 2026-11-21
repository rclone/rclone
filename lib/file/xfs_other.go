//go:build !linux || mips || mipsle || mips64 || mips64le || ppc64 || ppc64le

package file

import "os"

// XFSCommitToken is unavailable on platforms without the supported Linux XFS ABI.
type XFSCommitToken struct{}

// IsXFS reports false on platforms without the supported Linux XFS API.
func IsXFS(path string) (bool, error) {
	return false, nil
}

// StartXFSCommit reports that XFS conditional range commits are unavailable.
func StartXFSCommit(target *os.File) (*XFSCommitToken, error) {
	return nil, ErrXFSCommitRangeUnsupported
}

// CommitXFSRange reports that XFS conditional range commits are unavailable.
func CommitXFSRange(target, staging *os.File, token *XFSCommitToken, dsync bool) error {
	return ErrXFSCommitRangeUnsupported
}

// SupportsXFSExchange reports false on platforms without the supported Linux XFS API.
func SupportsXFSExchange(path string) (bool, error) { return false, nil }
