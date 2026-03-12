package s3

import (
	"context"
	"fmt"
	"strings"

	"github.com/rclone/gofakes3"
	"github.com/rclone/rclone/fs"
)

// logger output formatted message
type logger struct{}

// print log message
func (l logger) Print(level gofakes3.LogLevel, v ...any) {
	var b strings.Builder
	for i := range v {
		if i > 0 {
			fmt.Fprintf(&b, " ")
		}
		fmt.Fprint(&b, v[i])
	}
	s := b.String()

	switch level {
	default:
		fallthrough
	case gofakes3.LogErr:
		fs.ErrorfCtx(context.Background(), "serve s3", s)
	case gofakes3.LogWarn:
		fs.InfofCtx(context.Background(), "serve s3", s)
	case gofakes3.LogInfo:
		fs.DebugfCtx(context.Background(), "serve s3", s)
	}
}
