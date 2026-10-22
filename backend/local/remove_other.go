//go:build !windows

package local

// remove removes name with removeFn, retrying on a sharing violation
func remove(name string, removeFn func(string) error) error {
	return removeFn(name)
}
