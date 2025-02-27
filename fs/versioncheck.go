//go:build !go1.23

package fs

// Upgrade to Go version 1.23 to compile rclone - latest stable go
// compiler recommended.
func init() { Go_version_1_23_required_for_compilation() }
