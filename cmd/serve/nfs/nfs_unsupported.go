// For unsupported architectures
//go:build !unix
// +build !unix

// Package nfs is not supported on non-Unix platforms
package nfs

import (
	"github.com/spf13/cobra"
)

// For unsupported platforms we just put nil
var Command *cobra.Command = nil
