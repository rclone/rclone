//go:build !go1.16
// +build !go1.16

package fs

// Upgrade to Go version 1.16 to compile rclone - latest stable go
// compiler recommended.
func init() { Go_version_1_16_required_for_compilation() }
