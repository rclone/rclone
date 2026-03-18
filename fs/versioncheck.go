//go:build !go1.25

package fs

// Upgrade to Go version 1.25 to compile rclone - latest stable go
// compiler recommended.
func init() { Go_version_1_25_required_for_compilation() }
