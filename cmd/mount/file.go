// +build linux darwin freebsd

package mount

import (
	"time"

	"bazil.org/fuse"
	fusefs "bazil.org/fuse/fs"
	"github.com/ncw/rclone/cmd/mountlib"
	"github.com/ncw/rclone/fs"
	"github.com/pkg/errors"
	"golang.org/x/net/context"
)

// File represents a file
type File struct {
	*mountlib.File
	// size           int64        // size of file - read and written with atomic int64 - must be 64 bit aligned
	// d              *Dir         // parent directory - read only
	// mu             sync.RWMutex // protects the following
	// o              fs.Object    // NB o may be nil if file is being written
	// writers        int          // number of writers for this file
	// pendingModTime time.Time    // will be applied once o becomes available, i.e. after file was written
}

// Check interface satisfied
var _ fusefs.Node = (*File)(nil)

// Attr fills out the attributes for the file
func (f *File) Attr(ctx context.Context, a *fuse.Attr) (err error) {
	defer fs.Trace(f, "")("a=%+v, err=%v", a, &err)
	modTime, Size, Blocks, err := f.File.Attr(noModTime)
	if err != nil {
		return translateError(err)
	}
	a.Gid = gid
	a.Uid = uid
	a.Mode = filePerms
	a.Size = Size
	a.Atime = modTime
	a.Mtime = modTime
	a.Ctime = modTime
	a.Crtime = modTime
	a.Blocks = Blocks
	return nil
}

// Check interface satisfied
var _ fusefs.NodeSetattrer = (*File)(nil)

// Setattr handles attribute changes from FUSE. Currently supports ModTime only.
func (f *File) Setattr(ctx context.Context, req *fuse.SetattrRequest, resp *fuse.SetattrResponse) (err error) {
	defer fs.Trace(f, "a=%+v", req)("err=%v", &err)
	if noModTime {
		return nil
	}
	if req.Valid.MtimeNow() {
		err = f.File.SetModTime(time.Now())
	} else if req.Valid.Mtime() {
		err = f.File.SetModTime(req.Mtime)
	}
	return translateError(err)
}

// Check interface satisfied
var _ fusefs.NodeOpener = (*File)(nil)

// Open the file for read or write
func (f *File) Open(ctx context.Context, req *fuse.OpenRequest, resp *fuse.OpenResponse) (fh fusefs.Handle, err error) {
	defer fs.Trace(f, "flags=%v", req.Flags)("fh=%v, err=%v", &fh, &err)
	switch {
	case req.Flags.IsReadOnly():
		if noSeek {
			resp.Flags |= fuse.OpenNonSeekable
		}
		var rfh *mountlib.ReadFileHandle
		rfh, err = f.File.OpenRead()
		fh = &ReadFileHandle{rfh}
	case req.Flags.IsWriteOnly() || (req.Flags.IsReadWrite() && (req.Flags&fuse.OpenTruncate) != 0):
		resp.Flags |= fuse.OpenNonSeekable
		var wfh *mountlib.WriteFileHandle
		wfh, err = f.File.OpenWrite()
		fh = &WriteFileHandle{wfh}
	case req.Flags.IsReadWrite():
		err = errors.New("can't open for read and write simultaneously")
	default:
		err = errors.Errorf("can't figure out how to open with flags %v", req.Flags)
	}

	/*
	   // File was opened in append-only mode, all writes will go to end
	   // of file. OS X does not provide this information.
	   OpenAppend    OpenFlags = syscall.O_APPEND
	   OpenCreate    OpenFlags = syscall.O_CREAT
	   OpenDirectory OpenFlags = syscall.O_DIRECTORY
	   OpenExclusive OpenFlags = syscall.O_EXCL
	   OpenNonblock  OpenFlags = syscall.O_NONBLOCK
	   OpenSync      OpenFlags = syscall.O_SYNC
	   OpenTruncate  OpenFlags = syscall.O_TRUNC
	*/

	if err != nil {
		return nil, translateError(err)
	}
	return fh, nil
}

// Check interface satisfied
var _ fusefs.NodeFsyncer = (*File)(nil)

// Fsync the file
//
// Note that we don't do anything except return OK
func (f *File) Fsync(ctx context.Context, req *fuse.FsyncRequest) (err error) {
	defer fs.Trace(f, "")("err=%v", &err)
	return nil
}
