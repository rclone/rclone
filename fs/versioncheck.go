//go:build !go1.22

package fs

// Upgrade to Go version 1.22 to compile rclone - latest stable go
// compiler recommended.
func init() { Go_version_1_22_required_for_compilation() }
