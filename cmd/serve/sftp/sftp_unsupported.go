// Build for sftp for unsupported platforms to stop go complaining
// about "no buildable Go source files "

//go:build plan9

// Package sftp implements an SFTP server to serve an rclone VFS
package sftp

import "github.com/spf13/cobra"

// Command definition is nil to show not implemented
var Command *cobra.Command
