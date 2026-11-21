package s3

import (
	"errors"

	"github.com/rclone/rclone/backend/local"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/operations"
	filelib "github.com/rclone/rclone/lib/file"
	"github.com/rclone/rclone/vfs"
	"github.com/rclone/rclone/vfs/vfscommon"
)

func xfsCommitRangeCapability(fsys fs.Fs) (bool, error) {
	f, ok := fsys.(*local.Fs)
	if !ok {
		return false, nil
	}
	return f.SupportsXFSExchange()
}

func startXFSCommitGuard(fsys fs.Fs, remote string) (*local.XFSCommit, error) {
	f, ok := fsys.(*local.Fs)
	if !ok {
		return nil, filelib.ErrXFSCommitRangeUnsupported
	}
	return f.StartXFSCommit(remote)
}

func xfsCommitRangeVFSCapability(v *vfs.VFS) (bool, error) {
	if v.Opt.CacheMode != vfscommon.CacheModeOff || !operations.CanServerSideMove(v.Fs()) {
		return false, nil
	}
	return xfsCommitRangeCapability(v.Fs())
}

func xfsCommitUnavailable(err error) bool {
	return errors.Is(err, filelib.ErrXFSCommitRangeUnsupported)
}
