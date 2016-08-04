//+build !go1.5

package cmd

// Upgrade to Go version 1.5 to compile rclone.
func init() { Go_version_1_5_required_for_compilation() }
