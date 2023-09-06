//go:build freebsd || netbsd
// +build freebsd netbsd

package xattr

import (
	"os"
	"syscall"
	"unsafe"
)

const (
	// XATTR_SUPPORTED will be true if the current platform is supported
	XATTR_SUPPORTED = true

	EXTATTR_NAMESPACE_USER = 1

	// ENOATTR is not exported by the syscall package on Linux, because it is
	// an alias for ENODATA. We export it here so it is available on all
	// our supported platforms.
	ENOATTR = syscall.ENOATTR
)

func getxattr(path string, name string, data []byte) (int, error) {
	return sysGet(syscall.SYS_EXTATTR_GET_FILE, path, name, data)
}

func lgetxattr(path string, name string, data []byte) (int, error) {
	return sysGet(syscall.SYS_EXTATTR_GET_LINK, path, name, data)
}

func fgetxattr(f *os.File, name string, data []byte) (int, error) {
	return getxattr(f.Name(), name, data)
}

// sysGet is called by getxattr and lgetxattr with the appropriate syscall
// number. This works because syscalls have the same signature and return
// values.
func sysGet(syscallNum uintptr, path string, name string, data []byte) (int, error) {
	ptr, nbytes := bytePtrFromSlice(data)
	/*
		ssize_t extattr_get_file(
			const char *path,
			int attrnamespace,
			const char *attrname,
			void *data,
			size_t nbytes);

		ssize_t extattr_get_link(
			const char *path,
			int attrnamespace,
			const char *attrname,
			void *data,
			size_t nbytes);
	*/
	r0, _, err := syscall.Syscall6(syscallNum, uintptr(unsafe.Pointer(syscall.StringBytePtr(path))),
		EXTATTR_NAMESPACE_USER, uintptr(unsafe.Pointer(syscall.StringBytePtr(name))),
		uintptr(unsafe.Pointer(ptr)), uintptr(nbytes), 0)
	if err != syscall.Errno(0) {
		return int(r0), err
	}
	return int(r0), nil
}

func setxattr(path string, name string, data []byte, flags int) error {
	return sysSet(syscall.SYS_EXTATTR_SET_FILE, path, name, data)
}

func lsetxattr(path string, name string, data []byte, flags int) error {
	return sysSet(syscall.SYS_EXTATTR_SET_LINK, path, name, data)
}

func fsetxattr(f *os.File, name string, data []byte, flags int) error {
	return setxattr(f.Name(), name, data, flags)
}

// sysSet is called by setxattr and lsetxattr with the appropriate syscall
// number. This works because syscalls have the same signature and return
// values.
func sysSet(syscallNum uintptr, path string, name string, data []byte) error {
	ptr, nbytes := bytePtrFromSlice(data)
	/*
		ssize_t extattr_set_file(
			const char *path,
			int attrnamespace,
			const char *attrname,
			const void *data,
			size_t nbytes
		);

		ssize_t extattr_set_link(
			const char *path,
			int attrnamespace,
			const char *attrname,
			const void *data,
			size_t nbytes
		);
	*/
	r0, _, err := syscall.Syscall6(syscallNum, uintptr(unsafe.Pointer(syscall.StringBytePtr(path))),
		EXTATTR_NAMESPACE_USER, uintptr(unsafe.Pointer(syscall.StringBytePtr(name))),
		uintptr(unsafe.Pointer(ptr)), uintptr(nbytes), 0)
	if err != syscall.Errno(0) {
		return err
	}
	if int(r0) != nbytes {
		return syscall.E2BIG
	}
	return nil
}

func removexattr(path string, name string) error {
	return sysRemove(syscall.SYS_EXTATTR_DELETE_FILE, path, name)
}

func lremovexattr(path string, name string) error {
	return sysRemove(syscall.SYS_EXTATTR_DELETE_LINK, path, name)
}

func fremovexattr(f *os.File, name string) error {
	return removexattr(f.Name(), name)
}

// sysSet is called by removexattr and lremovexattr with the appropriate syscall
// number. This works because syscalls have the same signature and return
// values.
func sysRemove(syscallNum uintptr, path string, name string) error {
	/*
		int extattr_delete_file(
			const char *path,
			int attrnamespace,
			const char *attrname
		);

		int extattr_delete_link(
			const char *path,
			int attrnamespace,
			const char *attrname
		);
	*/
	_, _, err := syscall.Syscall(syscallNum, uintptr(unsafe.Pointer(syscall.StringBytePtr(path))),
		EXTATTR_NAMESPACE_USER, uintptr(unsafe.Pointer(syscall.StringBytePtr(name))),
	)
	if err != syscall.Errno(0) {
		return err
	}
	return nil
}

func listxattr(path string, data []byte) (int, error) {
	return sysList(syscall.SYS_EXTATTR_LIST_FILE, path, data)
}

func llistxattr(path string, data []byte) (int, error) {
	return sysList(syscall.SYS_EXTATTR_LIST_LINK, path, data)
}

func flistxattr(f *os.File, data []byte) (int, error) {
	return listxattr(f.Name(), data)
}

// sysSet is called by listxattr and llistxattr with the appropriate syscall
// number. This works because syscalls have the same signature and return
// values.
func sysList(syscallNum uintptr, path string, data []byte) (int, error) {
	ptr, nbytes := bytePtrFromSlice(data)
	/*
		ssize_t extattr_list_file(
				const char *path,
				int attrnamespace,
				void *data,
				size_t nbytes
			);

		ssize_t extattr_list_link(
			const char *path,
			int attrnamespace,
			void *data,
			size_t nbytes
		);
	*/
	r0, _, err := syscall.Syscall6(syscallNum, uintptr(unsafe.Pointer(syscall.StringBytePtr(path))),
		EXTATTR_NAMESPACE_USER, uintptr(unsafe.Pointer(ptr)), uintptr(nbytes), 0, 0)
	if err != syscall.Errno(0) {
		return int(r0), err
	}
	return int(r0), nil
}

// stringsFromByteSlice converts a sequence of attributes to a []string.
// On FreeBSD, each entry consists of a single byte containing the length
// of the attribute name, followed by the attribute name.
// The name is _not_ terminated by NULL.
func stringsFromByteSlice(buf []byte) (result []string) {
	index := 0
	for index < len(buf) {
		next := index + 1 + int(buf[index])
		result = append(result, string(buf[index+1:next]))
		index = next
	}
	return
}
