package sftp

import (
	"io"
	"os"
)

// Interfaces are differentiated based on required returned values.
// All input arguments are to be pulled from Request (the only arg).

// FileReader should return an io.ReaderAt for the filepath
// Note in cases of an error, the error text will be sent to the client.
type FileReader interface {
	Fileread(*Request) (io.ReaderAt, error)
}

// FileWriter should return an io.WriterAt for the filepath.
//
// The request server code will call Close() on the returned io.WriterAt
// ojbect if an io.Closer type assertion succeeds.
// Note in cases of an error, the error text will be sent to the client.
type FileWriter interface {
	Filewrite(*Request) (io.WriterAt, error)
}

// FileCmder should return an error
// Note in cases of an error, the error text will be sent to the client.
type FileCmder interface {
	Filecmd(*Request) error
}

// FileLister should return an object that fulfils the ListerAt interface
// Note in cases of an error, the error text will be sent to the client.
type FileLister interface {
	Filelist(*Request) (ListerAt, error)
}

// ListerAt does for file lists what io.ReaderAt does for files.
// ListAt should return the number of entries copied and an io.EOF
// error if at end of list. This is testable by comparing how many you
// copied to how many could be copied (eg. n < len(ls) below).
// The copy() builtin is best for the copying.
// Note in cases of an error, the error text will be sent to the client.
type ListerAt interface {
	ListAt([]os.FileInfo, int64) (int, error)
}
