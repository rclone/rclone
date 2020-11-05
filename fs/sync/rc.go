package sync

import (
	"context"

	"github.com/rclone/rclone/fs/rc"
)

func init() {
	for _, name := range []string{"sync", "copy", "move"} {
		name := name
		moveHelp := ""
		if name == "move" {
			moveHelp = "- deleteEmptySrcDirs - delete empty src directories if set\n"
		}
		rc.Add(rc.Call{
			Path:         "sync/" + name,
			AuthRequired: true,
			Fn: func(ctx context.Context, in rc.Params) (rc.Params, error) {
				return rcSyncCopyMove(ctx, in, name)
			},
			Title: name + " a directory from source remote to destination remote",
			Help: `This takes the following parameters

- srcFs - a remote name string e.g. "drive:src" for the source
- dstFs - a remote name string e.g. "drive:dst" for the destination
` + moveHelp + `

See the [` + name + ` command](/commands/rclone_` + name + `/) command for more information on the above.`,
		})
	}
}

// Sync/Copy/Move a file
func rcSyncCopyMove(ctx context.Context, in rc.Params, name string) (out rc.Params, err error) {
	srcFs, err := rc.GetFsNamed(ctx, in, "srcFs")
	if err != nil {
		return nil, err
	}
	dstFs, err := rc.GetFsNamed(ctx, in, "dstFs")
	if err != nil {
		return nil, err
	}
	createEmptySrcDirs, err := in.GetBool("createEmptySrcDirs")
	if rc.NotErrParamNotFound(err) {
		return nil, err
	}
	switch name {
	case "sync":
		return nil, Sync(ctx, dstFs, srcFs, createEmptySrcDirs)
	case "copy":
		return nil, CopyDir(ctx, dstFs, srcFs, createEmptySrcDirs)
	case "move":
		deleteEmptySrcDirs, err := in.GetBool("deleteEmptySrcDirs")
		if rc.NotErrParamNotFound(err) {
			return nil, err
		}
		return nil, MoveDir(ctx, dstFs, srcFs, deleteEmptySrcDirs, createEmptySrcDirs)
	}
	panic("unknown rcSyncCopyMove type")
}
