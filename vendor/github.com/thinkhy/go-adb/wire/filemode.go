package wire

import "os"

// ADB file modes seem to only be 16 bits.
// Values are taken from http://linux.die.net/include/bits/stat.h.
const (
	ModeDir        uint32 = 0040000
	ModeSymlink           = 0120000
	ModeSocket            = 0140000
	ModeFifo              = 0010000
	ModeCharDevice        = 0020000
)

func ParseFileModeFromAdb(modeFromSync uint32) (filemode os.FileMode) {
	// The ADB filemode uses the permission bits defined in Go's os package, but
	// we need to parse the other bits manually.
	switch {
	case modeFromSync&ModeSymlink == ModeSymlink:
		filemode = os.ModeSymlink
	case modeFromSync&ModeDir == ModeDir:
		filemode = os.ModeDir
	case modeFromSync&ModeSocket == ModeSocket:
		filemode = os.ModeSocket
	case modeFromSync&ModeFifo == ModeFifo:
		filemode = os.ModeNamedPipe
	case modeFromSync&ModeCharDevice == ModeCharDevice:
		filemode = os.ModeCharDevice
	}

	filemode |= os.FileMode(modeFromSync).Perm()
	return
}
