//go:build !linux || mips || mipsle || mips64 || mips64le || ppc64 || ppc64le

package local

import filelib "github.com/rclone/rclone/lib/file"

// XFSCommit is unavailable on this platform.
type XFSCommit struct{}

// SupportsXFSExchange reports whether the root filesystem supports content exchange.
func (*Fs) SupportsXFSExchange() (bool, error) { return false, nil }

// StartXFSCommit reports that conditional publication is unavailable.
func (*Fs) StartXFSCommit(string) (*XFSCommit, error) {
	return nil, filelib.ErrXFSCommitRangeUnsupported
}

// Publish reports that conditional publication is unavailable.
func (*XFSCommit) Publish(string) error { return filelib.ErrXFSCommitRangeUnsupported }

// Close releases the conditional publication handle.
func (*XFSCommit) Close() error { return nil }
