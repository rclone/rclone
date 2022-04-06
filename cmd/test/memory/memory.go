package memory

import (
	"context"
	"runtime"
	"sync"

	"github.com/rclone/rclone/cmd"
	"github.com/rclone/rclone/cmd/test"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/operations"
	"github.com/spf13/cobra"
)

func init() {
	test.Command.AddCommand(commandDefinition)
}

var commandDefinition = &cobra.Command{
	Use:   "memory remote:path",
	Short: `Load all the objects at remote:path into memory and report memory stats.`,
	Run: func(command *cobra.Command, args []string) {
		cmd.CheckArgs(1, 1, command, args)
		fsrc := cmd.NewFsSrc(args)
		cmd.Run(false, false, command, func() error {
			ctx := context.Background()
			ci := fs.GetConfig(context.Background())
			objects, _, _, err := operations.Count(ctx, fsrc)
			if err != nil {
				return err
			}
			objs := make([]fs.Object, 0, objects)
			var before, after runtime.MemStats
			runtime.GC()
			runtime.ReadMemStats(&before)
			var mu sync.Mutex
			err = operations.ListFn(ctx, fsrc, func(o fs.Object) {
				mu.Lock()
				objs = append(objs, o)
				mu.Unlock()
			})
			if err != nil {
				return err
			}
			runtime.GC()
			runtime.ReadMemStats(&after)
			var allocChange int64
			if after.Alloc >= before.Alloc {
				allocChange = int64(after.Alloc - before.Alloc)
			} else {
				allocChange = -int64(before.Alloc - after.Alloc)
			}
			var sysChange int64
			if after.Sys >= before.Sys {
				sysChange = int64(after.Sys - before.Sys)
			} else {
				sysChange = -int64(before.Sys - after.Sys)
			}
			if ci.HumanReadable {
				objString := fs.CountSuffix(int64(len(objs)))
				var usedString string
				if after.Alloc >= before.Alloc {
					usedString = fs.SizeSuffix(int64(after.Alloc - before.Alloc)).ByteUnit()
				} else {
					usedString = "-" + fs.SizeSuffix(int64(before.Alloc-after.Alloc)).ByteUnit()
				}
				avgString := fs.SizeSuffix(allocChange / int64(len(objs))).ByteUnit()
				fs.Logf(nil, "%s objects took %s, %s/object", objString, usedString, avgString)

				var sysBeforeString string
				if before.Sys <= fs.SizeSuffixMaxValue {
					sysBeforeString = fs.SizeSuffix(int64(before.Sys)).String()
				} else {
					sysBeforeString = ">" + fs.SizeSuffixMax.String()
				}
				var sysAfterString string
				if after.Sys <= fs.SizeSuffixMaxValue {
					sysAfterString = fs.SizeSuffix(int64(after.Sys)).ByteUnit()
				} else {
					sysAfterString = ">" + fs.SizeSuffixMax.ByteUnit()
				}
				var sysUsedString string
				if after.Sys >= before.Sys {
					sysUsedString = fs.SizeSuffix(int64(after.Sys - before.Sys)).ByteUnit()
				} else {
					sysUsedString = "-" + fs.SizeSuffix(int64(before.Sys-after.Sys)).ByteUnit()
				}
				fs.Logf(nil, "System memory changed from %s to %s a change of %s", sysBeforeString, sysAfterString, sysUsedString)
			} else {
				fs.Logf(nil, "%d objects took %d bytes, %.1f bytes/object", len(objs), allocChange, float64(allocChange)/float64(len(objs)))
				fs.Logf(nil, "System memory changed from %d to %d bytes a change of %d bytes", before.Sys, after.Sys, sysChange)
			}
			return nil
		})
	},
}
