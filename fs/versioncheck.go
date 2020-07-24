//+build !go1.11

package fs

// Upgrade to Go version 1.11 to compile rclone - latest stable go
// compiler recommended.
func init() { Go_version_1_11_required_for_compilation() }
