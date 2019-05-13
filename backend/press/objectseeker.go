// This file makes the ObjectSeeker struct, simulates a ReadSeeker for an Object
package press

import (
	"io"
	"errors"

	"github.com/ncw/rclone/fs"
)

var ErrorSeekDirectionNotSupported = errors.New("Only seek from start supported")

// ObjectSeeker struct that simulates a ReadSeeker for an object
type ObjectSeeker struct {
	o fs.Object
	cursor io.ReadCloser
	options []fs.OpenOption
}

// Creates a new ObjectSeeker from an Object
func newObjectSeeker(o fs.Object, options []fs.OpenOption) (os *ObjectSeeker, err error) {
	// Copy over object
	os = new(ObjectSeeker)
	os.o = o
	// Copy over some options
	var openOptions []fs.OpenOption = []fs.OpenOption{&fs.SeekOption{Offset: 0}}
	for _, option := range options {
		switch option.(type) {
			case *fs.SeekOption:
				continue
			case *fs.RangeOption:
				continue
			default:
				openOptions = append(openOptions, option)
		}
	}
	os.options = openOptions
	// Initialize cursor at 0
	cursor, err := o.Open(append(os.options, &fs.SeekOption{Offset: 0})...)
	if err != nil {
		return nil, err
	}
	os.cursor = cursor
	return os, nil
}

// Reads from the ObjectSeeker
func (os *ObjectSeeker) Read(p []byte) (n int, err error) {
	return os.cursor.Read(p)
}

// Seeks to another spot in the Object in the ObjectSeeker
func (os *ObjectSeeker) Seek(offset int64, whence int) (newOffset int64, err error) {
	if whence != io.SeekStart {
		return 0, ErrorSeekDirectionNotSupported
	}
	os.cursor.Close()
	os.cursor, err = os.o.Open(append(os.options, &fs.SeekOption{Offset: offset})...)
	return offset, err
}

// Closes an ObjectSeeker
func (os *ObjectSeeker) Close() error {
	return os.cursor.Close()
}