// +build plan9 js
// +build !noselfupdate

package selfupdate

func writable(path string) bool {
	return true
}
