//go:build !go1.21

package fs

// Upgrade to Go version 1.21 to compile rclone - latest stable go
// compiler recommended.
func init() { Go_version_1_21_required_for_compilation() }
