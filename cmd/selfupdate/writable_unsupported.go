//go:build (plan9 || js) && !noselfupdate

package selfupdate

func writable(path string) bool {
	return true
}
