//go:build !go1.20
// +build !go1.20

package fs

// Upgrade to Go version 1.20 to compile rclone - latest stable go
// compiler recommended.
func init() { Go_version_1_20_required_for_compilation() }
