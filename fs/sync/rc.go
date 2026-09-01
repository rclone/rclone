package sync

import (
	"context"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/operations"
	"github.com/rclone/rclone/fs/rc"
)

func init() {
	for _, name := range []string{"sync", "copy", "move"} {
		moveHelp := ""
		if name == "move" {
			moveHelp = "- deleteEmptySrcDirs - delete empty src directories if set\n"
		}
		rc.Add(rc.Call{
			Path: "sync/" + name,
			Fn: func(ctx context.Context, in rc.Params) (rc.Params, error) {
				return rcSyncCopyMove(ctx, in, name)
			},
			Title: name + " a directory from source remote to destination remote",
			Help: `This takes the following parameters:

- srcFs - a remote name string e.g. "drive:src" for the source
- dstFs - a remote name string e.g. "drive:dst" for the destination
- createEmptySrcDirs - create empty src directories on destination if set
` + moveHelp + `- combined - make a combined report of changes (default false)
- missingOnSrc - report all files missing from the source (default false)
- missingOnDst - report all files missing from the destination (default false)
- match - report all matching files (default false)
- differ - report all non-matching files (default false)
- error - report all files with errors (hashing or reading) (default false)
- destAfter - report all files that exist on the destination post-` + name + ` (default false)

Returns:

- combined - array of strings of combined report of changes
- missingOnSrc - array of strings of all files missing from the source
- missingOnDst - array of strings of all files missing from the destination
- match - array of strings of all matching files
- differ - array of strings of all non-matching files
- error - array of strings of all files with errors (hashing or reading)
- destAfter - array of strings of all files that exist on the destination post-` + name + `

Each report is only returned if its parameter is set to true. If the
operation fails the reports are only available if it was run with
` + "`_async`" + `, as part of the job output.

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
	ctx, out = rcLogger(ctx, in, dstFs)
	switch name {
	case "sync":
		return out, Sync(ctx, dstFs, srcFs, createEmptySrcDirs)
	case "copy":
		return out, CopyDir(ctx, dstFs, srcFs, createEmptySrcDirs)
	case "move":
		deleteEmptySrcDirs, err := in.GetBool("deleteEmptySrcDirs")
		if rc.NotErrParamNotFound(err) {
			return nil, err
		}
		return out, MoveDir(ctx, dstFs, srcFs, deleteEmptySrcDirs, createEmptySrcDirs)
	}
	panic("unknown rcSyncCopyMove type")
}

// rcLogger returns ctx with a sync logger which collects the reports
// requested in in into the returned out, which is empty if none were.
func rcLogger(ctx context.Context, in rc.Params, fdst fs.Fs) (context.Context, rc.Params) {
	out := rc.Params{}
	opt := operations.NewSyncLoggerOpt()
	opt.Combined = operations.RcReportWriter(in, out, "combined", false)
	opt.MissingOnSrc = operations.RcReportWriter(in, out, "missingOnSrc", false)
	opt.MissingOnDst = operations.RcReportWriter(in, out, "missingOnDst", false)
	opt.Match = operations.RcReportWriter(in, out, "match", false)
	opt.Differ = operations.RcReportWriter(in, out, "differ", false)
	opt.Error = operations.RcReportWriter(in, out, "error", false)
	opt.DestAfter = operations.RcReportWriter(in, out, "destAfter", false)
	if len(out) == 0 {
		return ctx, out
	}
	opt.LoggerFn = operations.NewDefaultLoggerFn(&opt)
	opt.Init(ctx, fdst, nil)
	return operations.WithSyncLogger(ctx, opt), out
}
