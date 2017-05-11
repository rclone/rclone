//+build !go1.6

package fs

// Upgrade to Go version 1.6 to compile rclone - latest stable go
// compiler recommended.
func init() { Go_version_1_6_required_for_compilation() }
