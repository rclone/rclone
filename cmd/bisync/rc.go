package bisync

import (
	"context"
	"log"

	"github.com/pkg/errors"
	"github.com/rclone/rclone/cmd/bisync/bilib"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/rc"
)

func init() {
	rc.Add(rc.Call{
		Path:         "sync/bisync",
		AuthRequired: true,
		Fn:           rcBisync,
		Title:        shortHelp,
		Help:         rcHelp,
	})
}

func rcBisync(ctx context.Context, in rc.Params) (out rc.Params, err error) {
	opt := &Options{}
	octx, ci := fs.AddConfig(ctx)

	if dryRun, err := in.GetBool("dryRun"); err == nil {
		ci.DryRun = dryRun
	} else if rc.NotErrParamNotFound(err) {
		return nil, err
	}

	if maxDelete, err := in.GetInt64("maxDelete"); err == nil {
		if maxDelete < 0 || maxDelete > 100 {
			return nil, rc.NewErrParamInvalid(errors.New("maxDelete must be a percentage between 0 and 100"))
		}
		ci.MaxDelete = maxDelete
	} else if rc.NotErrParamNotFound(err) {
		return nil, err
	}

	if opt.Resync, err = in.GetBool("resync"); rc.NotErrParamNotFound(err) {
		return
	}
	if opt.CheckAccess, err = in.GetBool("checkAccess"); rc.NotErrParamNotFound(err) {
		return
	}
	if opt.Force, err = in.GetBool("force"); rc.NotErrParamNotFound(err) {
		return
	}
	if opt.RemoveEmptyDirs, err = in.GetBool("removeEmptyDirs"); rc.NotErrParamNotFound(err) {
		return
	}
	if opt.NoCleanup, err = in.GetBool("noCleanup"); rc.NotErrParamNotFound(err) {
		return
	}

	if opt.CheckFilename, err = in.GetString("checkFilename"); rc.NotErrParamNotFound(err) {
		return
	}
	if opt.FiltersFile, err = in.GetString("filtersFile"); rc.NotErrParamNotFound(err) {
		return
	}
	if opt.Workdir, err = in.GetString("workdir"); rc.NotErrParamNotFound(err) {
		return
	}

	checkSync, err := in.GetString("checkSync")
	if rc.NotErrParamNotFound(err) {
		return nil, err
	}
	if err := opt.CheckSync.Set(checkSync); err != nil {
		return nil, err
	}

	fs1, err := rc.GetFsNamed(octx, in, "path1")
	if err != nil {
		return nil, err
	}

	fs2, err := rc.GetFsNamed(octx, in, "path2")
	if err != nil {
		return nil, err
	}

	output := bilib.CaptureOutput(func() {
		err = Bisync(octx, fs1, fs2, opt)
	})
	_, _ = log.Writer().Write(output)
	return rc.Params{"output": string(output)}, err
}
