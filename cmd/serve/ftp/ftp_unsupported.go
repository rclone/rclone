// Build  for unsupported platforms to stop go complaining
// about "no buildable Go source files "

// +build plan9 !go1.13

package ftp

import "github.com/spf13/cobra"

// Command definition is nil to show not implemented
var Command *cobra.Command = nil
