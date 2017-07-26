//+build android darwin linux nacl netbsd

package tree

import "syscall"

const modeExecute = syscall.S_IXUSR | syscall.S_IXGRP | syscall.S_IXOTH
