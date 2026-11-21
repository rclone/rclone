package file

import "errors"

// ErrXFSCommitRangeUnsupported is returned when XFS conditional range commits
// are not available on this platform or filesystem.
var ErrXFSCommitRangeUnsupported = errors.New("xfs commit range unsupported")
