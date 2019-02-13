package adb

import (
	"fmt"
	"io"
	"os"
	"time"

	"github.com/thinkhy/go-adb/internal/errors"
	"github.com/thinkhy/go-adb/wire"
)

// syncFileWriter wraps a SyncConn that has requested to send a file.
type syncFileWriter struct {
	// The modification time to write in the footer.
	// If 0, use the current time.
	mtime time.Time

	// Reader used to read data from the adb connection.
	sender wire.SyncSender
}

var _ io.WriteCloser = &syncFileWriter{}

func newSyncFileWriter(s wire.SyncSender, mtime time.Time) io.WriteCloser {
	return &syncFileWriter{
		mtime:  mtime,
		sender: s,
	}
}

/*
encodePathAndMode encodes a path and file mode as required for starting a send file stream.

From https://android.googlesource.com/platform/system/core/+/master/adb/SYNC.TXT:
	The remote file name is split into two parts separated by the last
	comma (","). The first part is the actual path, while the second is a decimal
	encoded file mode containing the permissions of the file on device.
*/
func encodePathAndMode(path string, mode os.FileMode) []byte {
	return []byte(fmt.Sprintf("%s,%d", path, uint32(mode.Perm())))
}

// Write writes the min of (len(buf), 64k).
func (w *syncFileWriter) Write(buf []byte) (n int, err error) {
	written := 0

	// If buf > 64k we'll have to send multiple chunks.
	// TODO Refactor this into something that can coalesce smaller writes into a single chukn.
	for len(buf) > 0 {
		// Writes < 64k have a one-to-one mapping to chunks.
		// If buffer is larger than the max, we'll return the max size and leave it up to the
		// caller to handle correctly.
		partialBuf := buf
		if len(partialBuf) > wire.SyncMaxChunkSize {
			partialBuf = partialBuf[:wire.SyncMaxChunkSize]
		}

		if err := w.sender.SendOctetString(wire.StatusSyncData); err != nil {
			return written, err
		}
		if err := w.sender.SendBytes(partialBuf); err != nil {
			return written, err
		}

		written += len(partialBuf)
		buf = buf[len(partialBuf):]
	}

	return written, nil
}

func (w *syncFileWriter) Close() error {
	if w.mtime.IsZero() {
		w.mtime = time.Now()
	}

	if err := w.sender.SendOctetString(wire.StatusSyncDone); err != nil {
		return errors.WrapErrf(err, "error sending done chunk to close stream")
	}
	if err := w.sender.SendTime(w.mtime); err != nil {
		return errors.WrapErrf(err, "error writing file modification time")
	}

	return errors.WrapErrf(w.sender.Close(), "error closing FileWriter")
}
