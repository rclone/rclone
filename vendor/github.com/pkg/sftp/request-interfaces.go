package sftp

import (
	"io"
	"os"
)

// Interfaces are differentiated based on required returned values.
// All input arguments are to be pulled from Request (the only arg).

// The Handler interfaces all take the Request object as its only argument.
// All the data you should need to handle the call are in the Request object.
// The request.Method attribute is initially the most important one as it
// determines which Handler gets called.

// FileReader should return an io.ReaderAt for the filepath
// Note in cases of an error, the error text will be sent to the client.
// Called for Methods: Get
type FileReader interface {
	Fileread(*Request) (io.ReaderAt, error)
}

// FileWriter should return an io.WriterAt for the filepath.
//
// The request server code will call Close() on the returned io.WriterAt
// ojbect if an io.Closer type assertion succeeds.
// Note in cases of an error, the error text will be sent to the client.
// Note when receiving an Append flag it is important to not open files using
// O_APPEND if you plan to use WriteAt, as they conflict.
// Called for Methods: Put, Open
type FileWriter interface {
	Filewrite(*Request) (io.WriterAt, error)
}

// FileCmder should return an error
// Note in cases of an error, the error text will be sent to the client.
// Called for Methods: Setstat, Rename, Rmdir, Mkdir, Link, Symlink, Remove
type FileCmder interface {
	Filecmd(*Request) error
}

// FileLister should return an object that fulfils the ListerAt interface
// Note in cases of an error, the error text will be sent to the client.
// Called for Methods: List, Stat, Readlink
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

// TransferError is an optional interface that readerAt and writerAt
// can implement to be notified about the error causing Serve() to exit
// with the request still open
type TransferError interface {
	TransferError(err error)
}
