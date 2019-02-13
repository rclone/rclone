package wire

import (
	"encoding/binary"
	"io"
	"os"
	"time"

	"github.com/thinkhy/go-adb/internal/errors"
)

type SyncSender interface {
	io.Closer

	// SendOctetString sends a 4-byte string.
	SendOctetString(string) error
	SendInt32(int32) error
	SendFileMode(os.FileMode) error
	SendTime(time.Time) error

	// Sends len(data) as an octet, followed by the bytes.
	// If data is bigger than SyncMaxChunkSize, it returns an assertion error.
	SendBytes(data []byte) error
}

type realSyncSender struct {
	io.Writer
}

func NewSyncSender(w io.Writer) SyncSender {
	return &realSyncSender{w}
}

func (s *realSyncSender) SendOctetString(str string) error {
	if len(str) != 4 {
		return errors.AssertionErrorf("octet string must be exactly 4 bytes: '%s'", str)
	}

	wrappedErr := errors.WrapErrorf(writeFully(s.Writer, []byte(str)),
		errors.NetworkError, "error sending octet string on sync sender")

	return wrappedErr
}

func (s *realSyncSender) SendInt32(val int32) error {
	return errors.WrapErrorf(binary.Write(s.Writer, binary.LittleEndian, val),
		errors.NetworkError, "error sending int on sync sender")
}

func (s *realSyncSender) SendFileMode(mode os.FileMode) error {
	return errors.WrapErrorf(binary.Write(s.Writer, binary.LittleEndian, mode),
		errors.NetworkError, "error sending filemode on sync sender")
}

func (s *realSyncSender) SendTime(t time.Time) error {
	return errors.WrapErrorf(s.SendInt32(int32(t.Unix())),
		errors.NetworkError, "error sending time on sync sender")
}

func (s *realSyncSender) SendBytes(data []byte) error {
	length := len(data)
	if length > SyncMaxChunkSize {
		// This limit might not apply to filenames, but it's big enough
		// that I don't think it will be a problem.
		return errors.AssertionErrorf("data must be <= %d in length", SyncMaxChunkSize)
	}

	if err := s.SendInt32(int32(length)); err != nil {
		return errors.WrapErrorf(err, errors.NetworkError, "error sending data length on sync sender")
	}
	return writeFully(s.Writer, data)
}

func (s *realSyncSender) Close() error {
	if closer, ok := s.Writer.(io.Closer); ok {
		return errors.WrapErrorf(closer.Close(), errors.NetworkError, "error closing sync sender")
	}
	return nil
}
