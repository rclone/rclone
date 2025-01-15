package s3

import (
	"fmt"

	"github.com/rclone/gofakes3"
	"github.com/rclone/rclone/fs"
)

// logger output formatted message
type logger struct{}

// print log message
func (l logger) Print(level gofakes3.LogLevel, v ...interface{}) {
	var s string
	if len(v) == 0 {
		s = ""
	} else {
		var ok bool
		s, ok = v[0].(string)
		if !ok {
			s = fmt.Sprint(v[0])
		}
		v = v[1:]
	}
	switch level {
	default:
		fallthrough
	case gofakes3.LogErr:
		fs.Errorf("serve s3", s, v...)
	case gofakes3.LogWarn:
		fs.Infof("serve s3", s, v...)
	case gofakes3.LogInfo:
		fs.Debugf("serve s3", s, v...)
	}
}
