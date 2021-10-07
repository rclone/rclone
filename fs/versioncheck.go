//go:build !go1.14
// +build !go1.14

package fs

// Upgrade to Go version 1.14 to compile rclone - latest stable go
// compiler recommended.
func init() { Go_version_1_14_required_for_compilation() }
