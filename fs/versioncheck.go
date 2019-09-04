//+build !go1.10

package fs

// Upgrade to Go version 1.10 to compile rclone - latest stable go
// compiler recommended.
func init() { Go_version_1_10_required_for_compilation() }
