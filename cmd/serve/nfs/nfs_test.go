//go:build unix

// The serving is tested in cmd/nfsmount - here we test anything else
package nfs

import (
	"testing"

	_ "github.com/PhateValleyman/rclone/backend/local"
	"github.com/PhateValleyman/rclone/cmd/serve/servetest"
	"github.com/PhateValleyman/rclone/fs/rc"
)

func TestRc(t *testing.T) {
	servetest.TestRc(t, rc.Params{
		"type":           "nfs",
		"vfs_cache_mode": "off",
	})
}
