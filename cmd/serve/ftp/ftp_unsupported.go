// Build  for unsupported platforms to stop go complaining
// about "no buildable Go source files "

//go:build plan9

// Package ftp implements an FTP server for rclone
package ftp

import "github.com/spf13/cobra"

// Command definition is nil to show not implemented
var Command *cobra.Command
