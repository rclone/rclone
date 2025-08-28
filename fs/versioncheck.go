//go:build !go1.24

package fs

// Upgrade to Go version 1.24 to compile rclone - latest stable go
// compiler recommended.
func init() { Go_version_1_24_required_for_compilation() }
