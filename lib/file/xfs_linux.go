//go:build linux && !mips && !mipsle && !mips64 && !mips64le && !ppc64 && !ppc64le

package file

import (
	"errors"
	"os"
	"runtime"
	"unsafe"

	"golang.org/x/sys/unix"
)

const (
	xfsSuperMagic = 0x58465342

	// _IOR('X', 130, struct xfs_commit_range) using the asm-generic ioctl ABI.
	xfsIOCStartCommit = uintptr(0x80585882)
	// _IOW('X', 131, struct xfs_commit_range) using the asm-generic ioctl ABI.
	xfsIOCCommitRange = uintptr(0x40585883)

	xfsExchangeRangeToEOF = uint64(1 << 0)
	xfsExchangeRangeDSync = uint64(1 << 1)
)

// xfsCommitRange mirrors struct xfs_commit_range from xfs_fs.h.
// Keep this layout in sync with the kernel UAPI.
type xfsCommitRange struct {
	File1FD        int32
	Pad            uint32
	File1Offset    uint64
	File2Offset    uint64
	Length         uint64
	Flags          uint64
	File2Freshness [6]uint64
}

// XFSCommitToken contains the opaque freshness sample returned by
// XFS_IOC_START_COMMIT. The target file descriptor must remain open until the
// token is committed.
type XFSCommitToken struct {
	args xfsCommitRange
}

// IsXFS reports whether path resides on an XFS filesystem.
func IsXFS(path string) (bool, error) {
	var st unix.Statfs_t
	if err := unix.Statfs(path, &st); err != nil {
		return false, err
	}
	return uint64(st.Type) == xfsSuperMagic, nil
}

// xfsGeometry contains the flags field of struct xfs_fsop_geom.
type xfsGeometry struct {
	_     [92]byte
	Flags uint32
	_     [160]byte
}

const xfsIOCGeometry = uintptr(0x8100587e)
const xfsGeometryExchangeRange = 1 << 24

// SupportsXFSExchange checks the exchange-range bit in the filesystem geometry.
func SupportsXFSExchange(path string) (bool, error) {
	onXFS, err := IsXFS(path)
	if !onXFS || err != nil {
		return false, err
	}
	f, err := os.Open(path)
	if err != nil {
		return false, err
	}
	defer func() { _ = f.Close() }()
	var geo xfsGeometry
	_, _, errno := unix.Syscall(unix.SYS_IOCTL, f.Fd(), xfsIOCGeometry, uintptr(unsafe.Pointer(&geo)))
	runtime.KeepAlive(f)
	if isXFSCommitUnsupported(errno) {
		return false, nil
	}
	if errno != 0 {
		return false, errno
	}
	return geo.Flags&xfsGeometryExchangeRange != 0, nil
}

// StartXFSCommit samples the target file's freshness information for a later
// conditional range commit.
func StartXFSCommit(target *os.File) (*XFSCommitToken, error) {
	if target == nil {
		return nil, os.ErrInvalid
	}

	var args xfsCommitRange
	err := xfsCommitIoctl(target.Fd(), xfsIOCStartCommit, &args)
	runtime.KeepAlive(target)
	if err != nil {
		if isXFSCommitUnsupported(err) {
			return nil, ErrXFSCommitRangeUnsupported
		}
		return nil, err
	}
	return &XFSCommitToken{args: args}, nil
}

// CommitXFSRange atomically exchanges the complete contents of staging into
// target if target still matches the freshness sample in token. If dsync is
// true, XFS also flushes the exchanged data and metadata before returning.
func CommitXFSRange(target, staging *os.File, token *XFSCommitToken, dsync bool) error {
	if target == nil || staging == nil || token == nil {
		return os.ErrInvalid
	}
	if uint64(staging.Fd()) > uint64(^uint32(0)>>1) {
		return os.ErrInvalid
	}

	args := token.args
	args.File1FD = int32(staging.Fd())
	args.File1Offset = 0
	args.File2Offset = 0
	args.Length = 0
	args.Flags = xfsExchangeRangeToEOF
	if dsync {
		args.Flags |= xfsExchangeRangeDSync
	}

	err := xfsCommitIoctl(target.Fd(), xfsIOCCommitRange, &args)
	runtime.KeepAlive(target)
	runtime.KeepAlive(staging)
	if err != nil {
		if isXFSCommitUnsupported(err) {
			return ErrXFSCommitRangeUnsupported
		}
		return err
	}
	return nil
}

func xfsCommitIoctl(fd uintptr, request uintptr, args *xfsCommitRange) error {
	_, _, errno := unix.Syscall(unix.SYS_IOCTL, fd, request, uintptr(unsafe.Pointer(args)))
	runtime.KeepAlive(args)
	if errno != 0 {
		return errno
	}
	return nil
}

func isXFSCommitUnsupported(err error) bool {
	return errors.Is(err, unix.ENOTTY) || errors.Is(err, unix.EOPNOTSUPP) || errors.Is(err, unix.ENOSYS)
}
