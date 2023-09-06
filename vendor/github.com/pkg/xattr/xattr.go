/*
Package xattr provides support for extended attributes on linux, darwin and freebsd.
Extended attributes are name:value pairs associated permanently with files and directories,
similar to the environment strings associated with a process.
An attribute may be defined or undefined. If it is defined, its value may be empty or non-empty.
More details you can find here: https://en.wikipedia.org/wiki/Extended_file_attributes .

All functions are provided in triples: Get/LGet/FGet, Set/LSet/FSet etc. The "L"
variant will not follow a symlink at the end of the path, and "F" variant accepts
a file descriptor instead of a path.

Example for "L" variant, assuming path is "/symlink1/symlink2", where both components are
symlinks:
Get will follow "symlink1" and "symlink2" and operate on the target of
"symlink2". LGet will follow "symlink1" but operate directly on "symlink2".
*/
package xattr

import (
	"os"
	"syscall"
)

// Error records an error and the operation, file path and attribute that caused it.
type Error struct {
	Op   string
	Path string
	Name string
	Err  error
}

func (e *Error) Unwrap() error { return e.Err }

func (e *Error) Error() (errstr string) {
	if e.Op != "" {
		errstr += e.Op
	}
	if e.Path != "" {
		if errstr != "" {
			errstr += " "
		}
		errstr += e.Path
	}
	if e.Name != "" {
		if errstr != "" {
			errstr += " "
		}
		errstr += e.Name
	}
	if e.Err != nil {
		if errstr != "" {
			errstr += ": "
		}
		errstr += e.Err.Error()
	}
	return
}

// Get retrieves extended attribute data associated with path. It will follow
// all symlinks along the path.
func Get(path, name string) ([]byte, error) {
	return get(path, name, func(name string, data []byte) (int, error) {
		return getxattr(path, name, data)
	})
}

// LGet is like Get but does not follow a symlink at the end of the path.
func LGet(path, name string) ([]byte, error) {
	return get(path, name, func(name string, data []byte) (int, error) {
		return lgetxattr(path, name, data)
	})
}

// FGet is like Get but accepts a os.File instead of a file path.
func FGet(f *os.File, name string) ([]byte, error) {
	return get(f.Name(), name, func(name string, data []byte) (int, error) {
		return fgetxattr(f, name, data)
	})
}

type getxattrFunc func(name string, data []byte) (int, error)

// get contains the buffer allocation logic used by both Get and LGet.
func get(path string, name string, getxattrFunc getxattrFunc) ([]byte, error) {
	const (
		// Start with a 1 KB buffer for the xattr value
		initialBufSize = 1024

		// The theoretical maximum xattr value size on MacOS is 64 MB. On Linux it's
		// much smaller at 64 KB. Unless the kernel is evil or buggy, we should never
		// hit the limit.
		maxBufSize = 64 * 1024 * 1024

		// Function name as reported in error messages
		myname = "xattr.get"
	)

	size := initialBufSize
	for {
		data := make([]byte, size)
		read, err := getxattrFunc(name, data)

		// If the buffer was too small to fit the value, Linux and MacOS react
		// differently:
		// Linux: returns an ERANGE error and "-1" bytes.
		// MacOS: truncates the value and returns "size" bytes. If the value
		//   happens to be exactly as big as the buffer, we cannot know if it was
		//   truncated, and we retry with a bigger buffer. Contrary to documentation,
		//   MacOS never seems to return ERANGE!
		// To keep the code simple, we always check both conditions, and sometimes
		// double the buffer size without it being strictly necessary.
		if err == syscall.ERANGE || read == size {
			// The buffer was too small. Try again.
			size <<= 1
			if size >= maxBufSize {
				return nil, &Error{myname, path, name, syscall.EOVERFLOW}
			}
			continue
		}
		if err != nil {
			return nil, &Error{myname, path, name, err}
		}
		return data[:read], nil
	}
}

// Set associates name and data together as an attribute of path.
func Set(path, name string, data []byte) error {
	if err := setxattr(path, name, data, 0); err != nil {
		return &Error{"xattr.Set", path, name, err}
	}
	return nil
}

// LSet is like Set but does not follow a symlink at
// the end of the path.
func LSet(path, name string, data []byte) error {
	if err := lsetxattr(path, name, data, 0); err != nil {
		return &Error{"xattr.LSet", path, name, err}
	}
	return nil
}

// FSet is like Set but accepts a os.File instead of a file path.
func FSet(f *os.File, name string, data []byte) error {
	if err := fsetxattr(f, name, data, 0); err != nil {
		return &Error{"xattr.FSet", f.Name(), name, err}
	}
	return nil
}

// SetWithFlags associates name and data together as an attribute of path.
// Forwards the flags parameter to the syscall layer.
func SetWithFlags(path, name string, data []byte, flags int) error {
	if err := setxattr(path, name, data, flags); err != nil {
		return &Error{"xattr.SetWithFlags", path, name, err}
	}
	return nil
}

// LSetWithFlags is like SetWithFlags but does not follow a symlink at
// the end of the path.
func LSetWithFlags(path, name string, data []byte, flags int) error {
	if err := lsetxattr(path, name, data, flags); err != nil {
		return &Error{"xattr.LSetWithFlags", path, name, err}
	}
	return nil
}

// FSetWithFlags is like SetWithFlags but accepts a os.File instead of a file path.
func FSetWithFlags(f *os.File, name string, data []byte, flags int) error {
	if err := fsetxattr(f, name, data, flags); err != nil {
		return &Error{"xattr.FSetWithFlags", f.Name(), name, err}
	}
	return nil
}

// Remove removes the attribute associated with the given path.
func Remove(path, name string) error {
	if err := removexattr(path, name); err != nil {
		return &Error{"xattr.Remove", path, name, err}
	}
	return nil
}

// LRemove is like Remove but does not follow a symlink at the end of the
// path.
func LRemove(path, name string) error {
	if err := lremovexattr(path, name); err != nil {
		return &Error{"xattr.LRemove", path, name, err}
	}
	return nil
}

// FRemove is like Remove but accepts a os.File instead of a file path.
func FRemove(f *os.File, name string) error {
	if err := fremovexattr(f, name); err != nil {
		return &Error{"xattr.FRemove", f.Name(), name, err}
	}
	return nil
}

// List retrieves a list of names of extended attributes associated
// with the given path in the file system.
func List(path string) ([]string, error) {
	return list(path, func(data []byte) (int, error) {
		return listxattr(path, data)
	})
}

// LList is like List but does not follow a symlink at the end of the
// path.
func LList(path string) ([]string, error) {
	return list(path, func(data []byte) (int, error) {
		return llistxattr(path, data)
	})
}

// FList is like List but accepts a os.File instead of a file path.
func FList(f *os.File) ([]string, error) {
	return list(f.Name(), func(data []byte) (int, error) {
		return flistxattr(f, data)
	})
}

type listxattrFunc func(data []byte) (int, error)

// list contains the buffer allocation logic used by both List and LList.
func list(path string, listxattrFunc listxattrFunc) ([]string, error) {
	myname := "xattr.list"
	// find size.
	size, err := listxattrFunc(nil)
	if err != nil {
		return nil, &Error{myname, path, "", err}
	}
	if size > 0 {
		// `size + 1` because of ERANGE error when reading
		// from a SMB1 mount point (https://github.com/pkg/xattr/issues/16).
		buf := make([]byte, size+1)
		// Read into buffer of that size.
		read, err := listxattrFunc(buf)
		if err != nil {
			return nil, &Error{myname, path, "", err}
		}
		return stringsFromByteSlice(buf[:read]), nil
	}
	return []string{}, nil
}

// bytePtrFromSlice returns a pointer to array of bytes and a size.
func bytePtrFromSlice(data []byte) (ptr *byte, size int) {
	size = len(data)
	if size > 0 {
		ptr = &data[0]
	}
	return
}
