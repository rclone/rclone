// +build darwin linux

/*
 * host_unix_test.go
 *
 * Copyright 2017 Bill Zissimopoulos
 */
/*
 * This file is part of Cgofuse.
 *
 * It is licensed under the MIT license. The full license text can be found
 * in the License.txt file at the root of this project.
 */

package fuse

import (
	"syscall"
)

func sendInterrupt() bool {
	return nil == syscall.Kill(syscall.Getpid(), syscall.SIGINT)
}
