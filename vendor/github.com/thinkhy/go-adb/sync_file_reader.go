package adb

import (
	"io"

	"github.com/thinkhy/go-adb/internal/errors"
	"github.com/thinkhy/go-adb/wire"
)

// syncFileReader wraps a SyncConn that has requested to receive a file.
type syncFileReader struct {
	// Reader used to read data from the adb connection.
	scanner wire.SyncScanner

	// Reader for the current chunk only.
	chunkReader io.Reader

	// False until the DONE chunk is encountered.
	eof bool
}

var _ io.ReadCloser = &syncFileReader{}

func newSyncFileReader(s wire.SyncScanner) (r io.ReadCloser, err error) {
	r = &syncFileReader{
		scanner: s,
	}

	// Read the header for the first chunk to consume any errors.
	if _, err = r.Read([]byte{}); err != nil {
		if err == io.EOF {
			// EOF means the file was empty. This still means the file was opened successfully,
			// and the next time the caller does a read they'll get the EOF and handle it themselves.
			err = nil
		} else {
			r.Close()
			return nil, err
		}
	}
	return
}

func (r *syncFileReader) Read(buf []byte) (n int, err error) {
	if r.eof {
		return 0, io.EOF
	}

	if r.chunkReader == nil {
		chunkReader, err := readNextChunk(r.scanner)
		if err != nil {
			if err == io.EOF {
				// We just read the last chunk, set our flag before passing it up.
				r.eof = true
			}
			return 0, err
		}
		r.chunkReader = chunkReader
	}

	if len(buf) == 0 {
		// Read can be called with an empty buffer to read the next chunk and check for errors.
		// However, net.Conn.Read seems to return EOF when given an empty buffer, so we need to
		// handle that case ourselves.
		return 0, nil
	}

	n, err = r.chunkReader.Read(buf)
	if err == io.EOF {
		// End of current chunk, don't return an error, the next chunk will be
		// read on the next call to this method.
		r.chunkReader = nil
		return n, nil
	}

	return n, err
}

func (r *syncFileReader) Close() error {
	return r.scanner.Close()
}

// readNextChunk creates an io.LimitedReader for the next chunk of data,
// and returns io.EOF if the last chunk has been read.
func readNextChunk(r wire.SyncScanner) (io.Reader, error) {
	status, err := r.ReadStatus("read-chunk")
	if err != nil {
		if wire.IsAdbServerErrorMatching(err, readFileNotFoundPredicate) {
			return nil, errors.Errorf(errors.FileNoExistError, "no such file or directory")
		}
		return nil, err
	}

	switch status {
	case wire.StatusSyncData:
		return r.ReadBytes()
	case wire.StatusSyncDone:
		return nil, io.EOF
	default:
		return nil, errors.Errorf(errors.AssertionError, "expected chunk id '%s' or '%s', but got '%s'",
			wire.StatusSyncData, wire.StatusSyncDone, []byte(status))
	}
}

// readFileNotFoundPredicate returns true if s is the adb server error message returned
// when trying to open a file that doesn't exist.
func readFileNotFoundPredicate(s string) bool {
	return s == "No such file or directory"
}
