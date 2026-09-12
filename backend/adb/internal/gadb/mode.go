package gadb

// mode.go is a modification helper, not from upstream gadb. It exists to fix
// the os.FileMode mode_t translation bug at the source. The vendored gadb
// subset is at upstream commit 2e108649b dated 2025-03-14, MIT licensed.

import "os"

// fixupMode translates the Unix mode_t bits returned by the ADB SYNC
// protocol into Go's os.FileMode bit positions. The wire format puts
// type bits at 0x4000 S_IFDIR, 0xa000 S_IFLNK, 0x8000 S_IFREG, 0x2000
// S_IFCHR, 0x6000 S_IFBLK, 0x1000 S_IFIFO, 0xc000 S_IFSOCK. Go's
// os.FileMode puts type bits at the high end (ModeDir is 1 << 31, etc.).
// The translation is necessary to make Mode.IsDir() and Mode & os.ModeDir
// return correct results for entries returned via SYNC LIST.
func fixupMode(raw uint32) os.FileMode {
	const (
		unixIFMT   = 0xf000
		unixIFDIR  = 0x4000
		unixIFLNK  = 0xa000
		unixIFREG  = 0x8000
		unixIFCHR  = 0x2000
		unixIFBLK  = 0x6000
		unixIFIFO  = 0x1000
		unixIFSOCK = 0xc000
	)
	mode := os.FileMode(raw & 0o777)
	switch raw & unixIFMT {
	case unixIFDIR:
		mode |= os.ModeDir
	case unixIFLNK:
		mode |= os.ModeSymlink
	case unixIFREG:
		// regular file, no type bit in os.FileMode
	case unixIFCHR:
		mode |= os.ModeDevice | os.ModeCharDevice
	case unixIFBLK:
		mode |= os.ModeDevice
	case unixIFIFO:
		mode |= os.ModeNamedPipe
	case unixIFSOCK:
		mode |= os.ModeSocket
	}
	return mode
}
