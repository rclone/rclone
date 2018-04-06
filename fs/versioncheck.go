//+build !go1.7

package fs

// Upgrade to Go version 1.7 to compile rclone - latest stable go
// compiler recommended.
func init() { Go_version_1_7_required_for_compilation() }
