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
	// fs.Infof(nil, fmt.Sprintln(v...))
	switch level {
	case gofakes3.LogErr:
		fs.Errorf(nil, fmt.Sprintln(v...))
	case gofakes3.LogWarn:
		fs.Infof(nil, fmt.Sprintln(v...))
	case gofakes3.LogInfo:
		fs.Debugf(nil, fmt.Sprintln(v...))
	default:
		panic("unknown level")
	}
}
