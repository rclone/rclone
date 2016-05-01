// +build !darwin

package local

// normString normalises the remote name if necessary
func normString(remote string) string {
	return remote
}
