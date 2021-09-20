//go:build !plan9
// +build !plan9

// Package tardigrade provides an interface to Tardigrade decentralized object storage.
package tardigrade

import (
	"github.com/rclone/rclone/backend/storj"
	"github.com/rclone/rclone/fs"
)

// Register with Fs
func init() {
	fs.Register(&fs.RegInfo{
		Name:        "tardigrade",
		Description: "Tardigrade (DEPRECATED, use Storj instead)",
		NewFs:       storj.NewFs,
		Config:      storj.BackendConfig,
		Options:     storj.BackendOptions,
	})
}
