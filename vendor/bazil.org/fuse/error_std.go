package fuse

// There is very little commonality in extended attribute errors
// across platforms.
//
// getxattr return value for "extended attribute does not exist" is
// ENOATTR on OS X, and ENODATA on Linux and apparently at least
// NetBSD. There may be a #define ENOATTR on Linux too, but the value
// is ENODATA in the actual syscalls. FreeBSD and OpenBSD have no
// ENODATA, only ENOATTR. ENOATTR is not in any of the standards,
// ENODATA exists but is only used for STREAMs.
//
// Each platform will define it a errNoXattr constant, and this file
// will enforce that it implements the right interfaces and hide the
// implementation.
//
// https://developer.apple.com/library/mac/documentation/Darwin/Reference/ManPages/man2/getxattr.2.html
// http://mail-index.netbsd.org/tech-kern/2012/04/30/msg013090.html
// http://mail-index.netbsd.org/tech-kern/2012/04/30/msg013097.html
// http://pubs.opengroup.org/onlinepubs/9699919799/basedefs/errno.h.html
// http://www.freebsd.org/cgi/man.cgi?query=extattr_get_file&sektion=2
// http://nixdoc.net/man-pages/openbsd/man2/extattr_get_file.2.html

// ErrNoXattr is a platform-independent error value meaning the
// extended attribute was not found. It can be used to respond to
// GetxattrRequest and such.
const ErrNoXattr = errNoXattr

var _ error = ErrNoXattr
var _ Errno = ErrNoXattr
var _ ErrorNumber = ErrNoXattr
