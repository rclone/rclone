//+build !go1.12

package fs

// Upgrade to Go version 1.12 to compile rclone - latest stable go
// compiler recommended.
func init() { Go_version_1_12_required_for_compilation() }
