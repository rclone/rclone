// Package mockdir makes a mock fs.Directory object
package mockdir

import (
	"time"

	"github.com/rclone/rclone/fs"
)

// New makes a mock directory object with the name given
func New(name string) fs.Directory {
	return fs.NewDir(name, time.Time{})
}
