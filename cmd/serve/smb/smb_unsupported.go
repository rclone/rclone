// For unsupported platforms (windows, plan9, 32-bit linux)
//go:build windows || plan9 || (linux && (386 || arm || mips || mipsle))

// Package smb is not supported on Windows, Plan 9, or 32-bit Linux
package smb

import (
	"github.com/spf13/cobra"
)

// Command is just nil for unsupported platforms
var Command *cobra.Command
