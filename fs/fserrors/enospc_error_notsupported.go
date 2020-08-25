// +build plan9

package fserrors

// IsErrNoSpace() on plan9 returns false because 
// plan9 does not support syscall.ENOSPC error.
func IsErrNoSpace(cause error) (isNoSpc bool) {
	isNoSpc = false
	return 
}
