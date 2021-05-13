package operations

import (
	"context"
	"fmt"
	"log"
	"path"
	"sort"
	"strings"

	"github.com/pkg/errors"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config"
	"github.com/rclone/rclone/fs/hash"
	"github.com/rclone/rclone/fs/walk"
)

// Recursively touch every file in fscr with time t
func TouchRecursive(ctx context.Context, fsrc fs.Fs, t time.Time) (err error) {
	timeAtr := time.Now()
	if timeAsArgument != "" {
		layout := defaultLayout
		if len(timeAsArgument) == len(layoutDateWithTime) {
			layout = layoutDateWithTime
		} else if len(timeAsArgument) > len(layoutDateWithTime) {
			layout = layoutDateWithTimeNano
		}
		var timeAtrFromFlags time.Time
		if localTime {
			timeAtrFromFlags, err = time.ParseInLocation(layout, timeAsArgument, time.Local)
		} else {
			timeAtrFromFlags, err = time.Parse(layout, timeAsArgument)
		}
		if err != nil {
			return errors.Wrap(err, "failed to parse date/time argument")
		}
		timeAtr = timeAtrFromFlags
	}

	return operations.ListFn(ctx, fsrc, func(o fs.Object) {
		err := o.SetModTime(ctx, timeAtr)
		if err != nil {
			err = fs.CountError(err)
			fs.Errorf(o, "touch: couldn't set mod time %v", err)
		}
	})
}
