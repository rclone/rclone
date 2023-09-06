package log

import (
	"fmt"
	"os"
	"time"
)

var DefaultTimeFormatter = func() string {
	return time.Now().Format(timeFmt)
}

var timeFmt string

func init() {
	var ok bool
	timeFmt, ok = os.LookupEnv("GO_LOG_TIME_FMT")
	if !ok {
		timeFmt = "2006-01-02 15:04:05 -0700"
	}
}

var started = time.Now()

var TimeFormatSecondsSinceInit = func() string {
	return fmt.Sprintf("%.3fs", time.Since(started).Seconds())
}
