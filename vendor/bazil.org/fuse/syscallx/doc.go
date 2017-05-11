// Package syscallx provides wrappers that make syscalls on various
// platforms more interoperable.
//
// The API intentionally omits the OS X-specific position and option
// arguments for extended attribute calls.
//
// Not having position means it might not be useful for accessing the
// resource fork. If that's needed by code inside fuse, a function
// with a different name may be added on the side.
//
// Options can be implemented with separate wrappers, in the style of
// Linux getxattr/lgetxattr/fgetxattr.
package syscallx // import "bazil.org/fuse/syscallx"
