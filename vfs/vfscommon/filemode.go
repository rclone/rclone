package vfscommon

import (
	"fmt"
	"os"
	"strconv"

	"github.com/rclone/rclone/fs"
)

// FileMode is a command line friendly os.FileMode
type FileMode os.FileMode

// String turns FileMode into a string
func (x FileMode) String() string {
	return fmt.Sprintf("%03o", x)
}

// Set a FileMode
func (x *FileMode) Set(s string) error {
	i, err := strconv.ParseInt(s, 8, 32)
	if err != nil {
		return fmt.Errorf("bad FileMode - must be octal digits: %w", err)
	}
	*x = (FileMode)(i)
	return nil
}

// Type of the value
func (x FileMode) Type() string {
	return "FileMode"
}

// UnmarshalJSON makes sure the value can be parsed as a string or integer in JSON
func (x *FileMode) UnmarshalJSON(in []byte) error {
	return fs.UnmarshalJSONFlag(in, x, func(i int64) error {
		*x = FileMode(i)
		return nil
	})
}
