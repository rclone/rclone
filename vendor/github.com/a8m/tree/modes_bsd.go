//+build dragonfly freebsd openbsd solaris windows

package tree

import "syscall"

const modeExecute = syscall.S_IXUSR
