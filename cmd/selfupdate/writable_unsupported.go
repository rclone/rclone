//go:build (plan9 || js || wasm) && !noselfupdate

package selfupdate

func writable(path string) bool {
	return true
}
