package sftp

import (
	"io"
	"os"
)

// Interfaces are differentiated based on required returned values.
// All input arguments are to be pulled from Request (the only arg).

// FileReader should return an io.Reader for the filepath
type FileReader interface {
	Fileread(*Request) (io.ReaderAt, error)
}

// FileWriter should return an io.Writer for the filepath
type FileWriter interface {
	Filewrite(*Request) (io.WriterAt, error)
}

// FileCmder should return an error (rename, remove, setstate, etc.)
type FileCmder interface {
	Filecmd(*Request) error
}

// FileLister should return file info interface and errors (readdir, stat)
type FileLister interface {
	Filelist(*Request) (ListerAt, error)
}

// ListerAt does for file lists what io.ReaderAt does for files.
// ListAt should return the number of entries copied and an io.EOF
// error if at end of list. This is testable by comparing how many you
// copied to how many could be copied (eg. n < len(ls) below).
// The copy() builtin is best for the copying.
type ListerAt interface {
	ListAt([]os.FileInfo, int64) (int, error)
}
