package log

import (
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/logrus"
)

// install hook in fs to call to avoid circular dependency
func init() {
	fs.InstallJSONLogger = logrus.InstallJSONLogger
}
