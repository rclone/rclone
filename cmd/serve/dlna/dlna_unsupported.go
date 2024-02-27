//go:build !go1.21

// Package dlna is unsupported on this platform
package dlna

import "github.com/spf13/cobra"

// Command definition is nil to show not implemented
var Command *cobra.Command
