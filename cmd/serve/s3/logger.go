package s3

import (
	"fmt"

	"github.com/Mikubill/gofakes3"
	"github.com/rclone/rclone/fs"
)

// logger output formatted message
type logger struct{}

// print log message
func (l logger) Print(level gofakes3.LogLevel, v ...interface{}) {
	switch level {
	default:
		fallthrough
	case gofakes3.LogErr:
		fs.Errorf("serve s3", fmt.Sprintln(v...))
	case gofakes3.LogWarn:
		fs.Infof("serve s3", fmt.Sprintln(v...))
	case gofakes3.LogInfo:
		fs.Debugf("serve s3", fmt.Sprintln(v...))
	}
}
