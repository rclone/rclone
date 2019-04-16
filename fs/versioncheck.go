//+build !go1.9

package fs

// Upgrade to Go version 1.9 to compile rclone - latest stable go
// compiler recommended.
func init() { Go_version_1_9_required_for_compilation() }
