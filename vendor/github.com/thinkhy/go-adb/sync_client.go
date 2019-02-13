package adb

import (
	"io"
	"os"
	"time"

	"github.com/thinkhy/go-adb/internal/errors"
	"github.com/thinkhy/go-adb/wire"
)

var zeroTime = time.Unix(0, 0).UTC()

func stat(conn *wire.SyncConn, path string) (*DirEntry, error) {
	if err := conn.SendOctetString("STAT"); err != nil {
		return nil, err
	}
	if err := conn.SendBytes([]byte(path)); err != nil {
		return nil, err
	}

	id, err := conn.ReadStatus("stat")
	if err != nil {
		return nil, err
	}
	if id != "STAT" {
		return nil, errors.Errorf(errors.AssertionError, "expected stat ID 'STAT', but got '%s'", id)
	}

	return readStat(conn)
}

func listDirEntries(conn *wire.SyncConn, path string) (entries *DirEntries, err error) {
	if err = conn.SendOctetString("LIST"); err != nil {
		return
	}
	if err = conn.SendBytes([]byte(path)); err != nil {
		return
	}

	return &DirEntries{scanner: conn}, nil
}

func receiveFile(conn *wire.SyncConn, path string) (io.ReadCloser, error) {
	if err := conn.SendOctetString("RECV"); err != nil {
		return nil, err
	}
	if err := conn.SendBytes([]byte(path)); err != nil {
		return nil, err
	}
	return newSyncFileReader(conn)
}

// sendFile returns a WriteCloser than will write to the file at path on device.
// The file will be created with permissions specified by mode.
// The file's modified time will be set to mtime, unless mtime is 0, in which case the time the writer is
// closed will be used.
func sendFile(conn *wire.SyncConn, path string, mode os.FileMode, mtime time.Time) (io.WriteCloser, error) {
	if err := conn.SendOctetString("SEND"); err != nil {
		return nil, err
	}

	pathAndMode := encodePathAndMode(path, mode)
	if err := conn.SendBytes(pathAndMode); err != nil {
		return nil, err
	}

	return newSyncFileWriter(conn, mtime), nil
}

func readStat(s wire.SyncScanner) (entry *DirEntry, err error) {
	mode, err := s.ReadFileMode()
	if err != nil {
		err = errors.WrapErrf(err, "error reading file mode: %v", err)
		return
	}
	size, err := s.ReadInt32()
	if err != nil {
		err = errors.WrapErrf(err, "error reading file size: %v", err)
		return
	}
	mtime, err := s.ReadTime()
	if err != nil {
		err = errors.WrapErrf(err, "error reading file time: %v", err)
		return
	}

	// adb doesn't indicate when a file doesn't exist, but will return all zeros.
	// Theoretically this could be an actual file, but that's very unlikely.
	if mode == os.FileMode(0) && size == 0 && mtime == zeroTime {
		return nil, errors.Errorf(errors.FileNoExistError, "file doesn't exist")
	}

	entry = &DirEntry{
		Mode:       mode,
		Size:       size,
		ModifiedAt: mtime,
	}
	return
}
