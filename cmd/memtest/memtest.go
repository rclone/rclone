package memtest

import (
	"runtime"
	"sync"

	"github.com/ncw/rclone/cmd"
	"github.com/ncw/rclone/fs"
	"github.com/spf13/cobra"
)

func init() {
	cmd.Root.AddCommand(commandDefintion)
}

var commandDefintion = &cobra.Command{
	Use:    "memtest remote:path",
	Short:  `Load all the objects at remote:path and report memory stats.`,
	Hidden: true,
	Run: func(command *cobra.Command, args []string) {
		cmd.CheckArgs(1, 1, command, args)
		fsrc := cmd.NewFsSrc(args)
		cmd.Run(false, false, command, func() error {
			objects, _, err := fs.Count(fsrc)
			if err != nil {
				return err
			}
			objs := make([]fs.Object, 0, objects)
			var before, after runtime.MemStats
			runtime.GC()
			runtime.ReadMemStats(&before)
			var mu sync.Mutex
			err = fs.ListFn(fsrc, func(o fs.Object) {
				mu.Lock()
				objs = append(objs, o)
				mu.Unlock()
			})
			if err != nil {
				return err
			}
			runtime.GC()
			runtime.ReadMemStats(&after)
			usedMemory := after.Alloc - before.Alloc
			fs.Log(nil, "%d objects took %d bytes, %.1f bytes/object", len(objs), usedMemory, float64(usedMemory)/float64(len(objs)))
			fs.Log(nil, "System memory changed from %d to %d bytes a change of %d bytes", before.Sys, after.Sys, after.Sys-before.Sys)
			return nil
		})
	},
}
