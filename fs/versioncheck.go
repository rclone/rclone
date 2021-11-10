//go:build !go1.15
// +build !go1.15

package fs

// Upgrade to Go version 1.15 to compile rclone - latest stable go
// compiler recommended.
func init() { Go_version_1_15_required_for_compilation() }
