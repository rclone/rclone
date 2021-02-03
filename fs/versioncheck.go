//+build !go1.13

package fs

// Upgrade to Go version 1.13 to compile rclone - latest stable go
// compiler recommended.
func init() { Go_version_1_13_required_for_compilation() }
