// For unsupported architectures
//go:build !unix

// Package nfs is not supported on non-Unix platforms
package nfs

import (
	"github.com/spf13/cobra"
)

// Command is just nil for unsupported platforms
var Command *cobra.Command
