// For unsupported platforms (windows, plan9)
//go:build windows || plan9

// Package smb is not supported on Windows or Plan 9
package smb

import (
	"github.com/spf13/cobra"
)

// Command is just nil for unsupported platforms
var Command *cobra.Command
