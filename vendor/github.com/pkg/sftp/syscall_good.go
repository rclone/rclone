// +build !plan9,!windows
// +build !js !wasm

package sftp

import "syscall"

const S_IFMT = syscall.S_IFMT
