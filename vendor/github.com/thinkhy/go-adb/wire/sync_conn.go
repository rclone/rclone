package wire

import "github.com/thinkhy/go-adb/internal/errors"

const (
	// Chunks cannot be longer than 64k.
	SyncMaxChunkSize = 64 * 1024
)

/*
SyncConn is a connection to the adb server in sync mode.
Assumes the connection has been put into sync mode (by sending "sync" in transport mode).

The adb sync protocol is defined at
https://android.googlesource.com/platform/system/core/+/master/adb/SYNC.TXT.

Unlike the normal adb protocol (implemented in Conn), the sync protocol is binary.
Lengths are binary-encoded (little-endian) instead of hex.

Notes on Encoding

Length headers and other integers are encoded in little-endian, with 32 bits.

File mode seems to be encoded as POSIX file mode.

Modification time seems to be the Unix timestamp format, i.e. seconds since Epoch UTC.
*/
type SyncConn struct {
	SyncScanner
	SyncSender
}

// Close closes both the sender and the scanner, and returns any errors.
func (c SyncConn) Close() error {
	return errors.CombineErrs("error closing SyncConn", errors.NetworkError,
		c.SyncScanner.Close(), c.SyncSender.Close())
}
