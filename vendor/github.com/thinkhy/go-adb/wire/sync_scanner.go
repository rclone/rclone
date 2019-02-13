package wire

import (
	"encoding/binary"
	"io"
	"os"
	"time"

	"github.com/thinkhy/go-adb/internal/errors"
)

type SyncScanner interface {
	io.Closer
	StatusReader
	ReadInt32() (int32, error)
	ReadFileMode() (os.FileMode, error)
	ReadTime() (time.Time, error)

	// Reads an octet length, followed by length bytes.
	ReadString() (string, error)

	// Reads an octet length, and returns a reader that will read length
	// bytes (see io.LimitReader). The returned reader should be fully
	// read before reading anything off the Scanner again.
	ReadBytes() (io.Reader, error)
}

type realSyncScanner struct {
	io.Reader
}

func NewSyncScanner(r io.Reader) SyncScanner {
	return &realSyncScanner{r}
}

func (s *realSyncScanner) ReadStatus(req string) (string, error) {
	return readStatusFailureAsError(s.Reader, req, readInt32)
}

func (s *realSyncScanner) ReadInt32() (int32, error) {
	value, err := readInt32(s.Reader)
	return int32(value), errors.WrapErrorf(err, errors.NetworkError, "error reading int from sync scanner")
}
func (s *realSyncScanner) ReadFileMode() (os.FileMode, error) {
	var value uint32
	err := binary.Read(s.Reader, binary.LittleEndian, &value)
	if err != nil {
		return 0, errors.WrapErrorf(err, errors.NetworkError, "error reading filemode from sync scanner")
	}
	return ParseFileModeFromAdb(value), nil

}
func (s *realSyncScanner) ReadTime() (time.Time, error) {
	seconds, err := s.ReadInt32()
	if err != nil {
		return time.Time{}, errors.WrapErrorf(err, errors.NetworkError, "error reading time from sync scanner")
	}

	return time.Unix(int64(seconds), 0).UTC(), nil
}

func (s *realSyncScanner) ReadString() (string, error) {
	length, err := s.ReadInt32()
	if err != nil {
		return "", errors.WrapErrorf(err, errors.NetworkError, "error reading length from sync scanner")
	}

	bytes := make([]byte, length)
	n, rawErr := io.ReadFull(s.Reader, bytes)
	if rawErr != nil && rawErr != io.ErrUnexpectedEOF {
		return "", errors.WrapErrorf(rawErr, errors.NetworkError, "error reading string from sync scanner")
	} else if rawErr == io.ErrUnexpectedEOF {
		return "", errIncompleteMessage("bytes", n, int(length))
	}

	return string(bytes), nil
}
func (s *realSyncScanner) ReadBytes() (io.Reader, error) {
	length, err := s.ReadInt32()
	if err != nil {
		return nil, errors.WrapErrorf(err, errors.NetworkError, "error reading bytes from sync scanner")
	}

	return io.LimitReader(s.Reader, int64(length)), nil
}

func (s *realSyncScanner) Close() error {
	if closer, ok := s.Reader.(io.Closer); ok {
		return errors.WrapErrorf(closer.Close(), errors.NetworkError, "error closing sync scanner")
	}
	return nil
}
