// +build windows android

package sftp

import (
	"os"
)

func lsLinksUIDGID(fi os.FileInfo) (numLinks uint64, uid, gid string) {
	return 1, "0", "0"
}
