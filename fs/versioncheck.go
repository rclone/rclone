//go:build !go1.17
// +build !go1.17

package fs

// Upgrade to Go version 1.17 to compile rclone - latest stable go
// compiler recommended.
func init() { Go_version_1_17_required_for_compilation() }
