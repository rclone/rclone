//go:build noselfupdate

package selfupdate

import (
	"github.com/PhateValleyman/rclone/lib/buildinfo"
)

func init() {
	buildinfo.Tags = append(buildinfo.Tags, "noselfupdate")
}
