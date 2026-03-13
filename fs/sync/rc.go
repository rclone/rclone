package sync

import (
	"context"
	"time"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/fserrors"
	"github.com/rclone/rclone/fs/rc"
)

func init() {
	for _, name := range []string{"sync", "copy", "move"} {
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
			Help: `This takes the following parameters:

- srcFs - a remote name string e.g. "drive:src" for the source
- dstFs - a remote name string e.g. "drive:dst" for the destination
- createEmptySrcDirs - create empty src directories on destination if set
` + moveHelp + `

See the [` + name + `](/commands/rclone_` + name + `/) command for more information on the above.`,
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

	ci := fs.GetConfig(ctx)

	for try := 1; try <= ci.Retries; try++ {
		switch name {
		case "sync":
			err = Sync(ctx, dstFs, srcFs, createEmptySrcDirs)
		case "copy":
			err = CopyDir(ctx, dstFs, srcFs, createEmptySrcDirs)
		case "move":
			deleteEmptySrcDirs, err := in.GetBool("deleteEmptySrcDirs")
			if rc.NotErrParamNotFound(err) {
				return nil, err
			}
			return nil, MoveDir(ctx, dstFs, srcFs, deleteEmptySrcDirs, createEmptySrcDirs)
		default:
			panic("unknown rcSyncCopyMove type")
		}

		if err == nil {
			if try > 1 {
				fs.Logf(nil, "rc %s: Attempt %d/%d succeeded", name, try, ci.Retries)
			}
			break
		}

		if fserrors.IsFatalError(err) {
			fs.Errorf(nil, "rc %s: Fatal error received - not attempting retries: %v", name, err)
			break
		}

		if fserrors.IsNoRetryError(err) {
			fs.Errorf(nil, "rc %s: Non-retryable error - not attempting retries: %v", name, err)
			break
		}

		fs.Errorf(nil, "rc %s: Attempt %d/%d failed: %v", name, try, ci.Retries, err)

		if fserrors.IsRetryAfterError(err) {
			retryAfter := fserrors.RetryAfterErrorTime(err)
			d := time.Until(retryAfter)
			if d > 0 {
				fs.Logf(nil, "rc %s: Retry-after error - sleeping until %s (%v)", name, retryAfter.Format(time.RFC3339Nano), d)
				time.Sleep(d)
				continue
			}
		}

		if ci.RetriesInterval > 0 {
			fs.Logf(nil, "rc %s: Sleeping %v before retry %d/%d", name, ci.RetriesInterval, try+1, ci.Retries)
			time.Sleep(time.Duration(ci.RetriesInterval))
		}
	}

	return nil, err
}
