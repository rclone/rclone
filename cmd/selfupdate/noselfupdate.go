//go:build noselfupdate

package selfupdate

import (
	"github.com/rclone/rclone/lib/buildinfo"
)

func init() {
	buildinfo.Tags = append(buildinfo.Tags, "noselfupdate")
}
