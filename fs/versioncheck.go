//go:build !go1.19
// +build !go1.19

package fs

// Upgrade to Go version 1.19 to compile rclone - latest stable go
// compiler recommended.
func init() { Go_version_1_19_required_for_compilation() }
