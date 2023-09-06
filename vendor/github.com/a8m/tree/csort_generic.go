//+build !linux,!openbsd,!dragonfly,!android,!solaris,!darwin,!freebsd,!netbsd

package tree

// CtimeSort for unsupported OS - just compare ModTime
var CTimeSort = ModSort
