// Build for restic for unsupported platforms to stop go complaining
// about "no buildable Go source files "

//go:build !go1.17
// +build !go1.17

package restic

import (
	"github.com/spf13/cobra"
)

// Command definition for cobra
var Command *cobra.Command
