//go:build !go1.26

package fs

// Upgrade to Go version 1.26 to compile rclone - latest stable go
// compiler recommended.
func init() { Go_version_1_26_required_for_compilation() }
