//+build !go1.8

package fs

// Upgrade to Go version 1.8 to compile rclone - latest stable go
// compiler recommended.
func init() { Go_version_1_8_required_for_compilation() }
