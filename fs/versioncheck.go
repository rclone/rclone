//go:build !go1.18
// +build !go1.18

package fs

// Upgrade to Go version 1.18 to compile rclone - latest stable go
// compiler recommended.
func init() { Go_version_1_18_required_for_compilation() }
