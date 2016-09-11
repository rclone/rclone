//+build !go1.5

package fs

// Upgrade to Go version 1.5 to compile rclone - latest stable go
// compiler recommended.
func init() { Go_version_1_5_required_for_compilation() }
