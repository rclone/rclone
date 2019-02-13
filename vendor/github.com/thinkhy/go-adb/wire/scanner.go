package wire

import (
	"encoding/binary"
	"io"
	"io/ioutil"
	"strconv"

	"github.com/thinkhy/go-adb/internal/errors"
)

// TODO(zach): All EOF errors returned from networoking calls should use ConnectionResetError.

// StatusCodes are returned by the server. If the code indicates failure, the
// next message will be the error.
const (
	StatusSuccess  string = "OKAY"
	StatusFailure         = "FAIL"
	StatusSyncData        = "DATA"
	StatusSyncDone        = "DONE"
	StatusNone            = ""
)

func isFailureStatus(status string) bool {
	return status == StatusFailure
}

type StatusReader interface {
	// Reads a 4-byte status string and returns it.
	// If the status string is StatusFailure, reads the error message from the server
	// and returns it as an AdbError.
	ReadStatus(req string) (string, error)
}

/*
Scanner reads tokens from a server.
See Conn for more details.
*/
type Scanner interface {
	io.Closer
	StatusReader
	Read([]byte) (int, error)
	ReadMessage() ([]byte, error)
	ReadUntilEof() ([]byte, error)

	NewSyncScanner() SyncScanner
}

type realScanner struct {
	reader io.ReadCloser
}

func NewScanner(r io.ReadCloser) Scanner {
	return &realScanner{r}
}

func ReadMessageString(s Scanner) (string, error) {
	msg, err := s.ReadMessage()
	if err != nil {
		return string(msg), err
	}
	return string(msg), nil
}

func (s *realScanner) ReadStatus(req string) (string, error) {
	return readStatusFailureAsError(s.reader, req, readHexLength)
}

func (s *realScanner) ReadMessage() ([]byte, error) {
	return readMessage(s.reader, readHexLength)
}

func (s *realScanner) ReadUntilEof() ([]byte, error) {
	data, err := ioutil.ReadAll(s.reader)
	if err != nil {
		return nil, errors.WrapErrorf(err, errors.NetworkError, "error reading until EOF")
	}
	return data, nil
}

// wrap for shell
func (s *realScanner) Read(p []byte) (n int, err error) {
	return s.reader.Read(p)
}

func (s *realScanner) NewSyncScanner() SyncScanner {
	return NewSyncScanner(s.reader)
}

func (s *realScanner) Close() error {
	return errors.WrapErrorf(s.reader.Close(), errors.NetworkError, "error closing scanner")
}

var _ Scanner = &realScanner{}

// lengthReader is a func that readMessage uses to read message length.
// See readHexLength and readInt32.
type lengthReader func(io.Reader) (int, error)

// Reads the status, and if failure, reads the message and returns it as an error.
// If the status is success, doesn't read the message.
// req is just used to populate the AdbError, and can be nil.
// messageLengthReader is the function passed to readMessage if the status is failure.
func readStatusFailureAsError(r io.Reader, req string, messageLengthReader lengthReader) (string, error) {
	status, err := readOctetString(req, r)
	if err != nil {
		return "", errors.WrapErrorf(err, errors.NetworkError, "error reading status for %s", req)
	}

	if isFailureStatus(status) {
		msg, err := readMessage(r, messageLengthReader)
		if err != nil {
			return "", errors.WrapErrorf(err, errors.NetworkError,
				"server returned error for %s, but couldn't read the error message", req)
		}

		return "", adbServerError(req, string(msg))
	}

	return status, nil
}

func readOctetString(description string, r io.Reader) (string, error) {
	octet := make([]byte, 4)
	n, err := io.ReadFull(r, octet)

	if err == io.ErrUnexpectedEOF {
		return "", errIncompleteMessage(description, n, 4)
	} else if err != nil {
		return "", errors.WrapErrorf(err, errors.NetworkError, "error reading "+description)
	}

	return string(octet), nil
}

// readMessage reads a length from r, then reads length bytes and returns them.
// lengthReader is the function used to read the length. Most operations encode
// length as a hex string (readHexLength), but sync operations use little-endian
// binary encoding (readInt32).
func readMessage(r io.Reader, lengthReader lengthReader) ([]byte, error) {
	var err error

	length, err := lengthReader(r)
	if err != nil {
		return nil, err
	}

	data := make([]byte, length)
	n, err := io.ReadFull(r, data)

	if err != nil && err != io.ErrUnexpectedEOF {
		return data, errors.WrapErrorf(err, errors.NetworkError, "error reading message data")
	} else if err == io.ErrUnexpectedEOF {
		return data, errIncompleteMessage("message data", n, length)
	}
	return data, nil
}

// readHexLength reads the next 4 bytes from r as an ASCII hex-encoded length and parses them into an int.
func readHexLength(r io.Reader) (int, error) {
	lengthHex := make([]byte, 4)
	n, err := io.ReadFull(r, lengthHex)
	if err != nil {
		return 0, errIncompleteMessage("length", n, 4)
	}

	length, err := strconv.ParseInt(string(lengthHex), 16, 64)
	if err != nil {
		return 0, errors.WrapErrorf(err, errors.NetworkError, "could not parse hex length %v", lengthHex)
	}

	// COMMENT(ssx): comment the below code because I encounter message length > 255
	// Clip the length to 255, as per the Google implementation.
	// if length > MaxMessageLength {
	// 	length = MaxMessageLength
	// }

	return int(length), nil
}

// readInt32 reads the next 4 bytes from r as a little-endian integer.
// Returns an int instead of an int32 to match the lengthReader type.
func readInt32(r io.Reader) (int, error) {
	var value int32
	err := binary.Read(r, binary.LittleEndian, &value)
	return int(value), err
}
