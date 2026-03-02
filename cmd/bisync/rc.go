//go:generate go run help.go rc.md
package bisync

import (
	"context"
	_ "embed"
	"errors"
	"log"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/rclone/rclone/cmd/bisync/bilib"
	"github.com/rclone/rclone/fs"
	fslog "github.com/rclone/rclone/fs/log"
	"github.com/rclone/rclone/fs/rc"
)

func addRC() {
	rc.Add(rc.Call{
		Path:         "sync/bisync",
		AuthRequired: true,
		Fn:           rcBisync,
		Title:        shortHelp,
		Help:         rcHelp,
	})
}

//go:embed rc.md
var rcHelp string

var shortHelp = `Perform bidirectional synchronization between two paths.`

var longHelp = shortHelp + MakeHelp(`

[Bisync](https://rclone.org/bisync/) provides a
bidirectional cloud sync solution in rclone.
It retains the Path1 and Path2 filesystem listings from the prior run.
On each successive run it will:

- list files on Path1 and Path2, and check for changes on each side.
  Changes include ||New||, ||Newer||, ||Older||, and ||Deleted|| files.
- Propagate changes on Path1 to Path2, and vice-versa.

Bisync is considered an **advanced command**, so use with care.
Make sure you have read and understood the entire [manual](https://rclone.org/bisync)
(especially the [Limitations](https://rclone.org/bisync/#limitations) section)
before using, or data loss can result. Questions can be asked in the
[Rclone Forum](https://forum.rclone.org/).

See [full bisync description](https://rclone.org/bisync/) for details.
`)

// MakeHelp replaces some dynamic variables for the help docs
func MakeHelp(help string) string {
	replacer := strings.NewReplacer(
		"||", "`",
		"{MAXDELETE}", strconv.Itoa(DefaultMaxDelete),
		"{CHECKFILE}", DefaultCheckFilename,
		"{WORKDIR}", DefaultWorkdir,
	)
	return replacer.Replace(help)
}

func rcBisync(ctx context.Context, in rc.Params) (out rc.Params, err error) {
	opt := &Options{}
	octx, ci := fs.AddConfig(ctx)

	if dryRun, err := in.GetBool("dryRun"); err == nil {
		ci.DryRun = dryRun
		opt.DryRun = dryRun
	} else if rc.NotErrParamNotFound(err) {
		return nil, err
	}

	if maxDelete, err := in.GetInt64("maxDelete"); err == nil {
		if maxDelete < 0 || maxDelete > 100 {
			return nil, rc.NewErrParamInvalid(errors.New("maxDelete must be a percentage between 0 and 100"))
		}
		opt.MaxDelete = int(maxDelete)
	} else if rc.NotErrParamNotFound(err) {
		return nil, err
	}

	if opt.Resync, err = in.GetBool("resync"); rc.NotErrParamNotFound(err) {
		fs.Debugf("resync", "optional parameter is missing. using default value: %v", opt.Resync)
	}
	if opt.CheckAccess, err = in.GetBool("checkAccess"); rc.NotErrParamNotFound(err) {
		fs.Debugf("checkAccess", "optional parameter is missing. using default value: %v", opt.CheckAccess)
	}
	if opt.Force, err = in.GetBool("force"); rc.NotErrParamNotFound(err) {
		fs.Debugf("force", "optional parameter is missing. using default value: %v", opt.Force)
	}
	if opt.CreateEmptySrcDirs, err = in.GetBool("createEmptySrcDirs"); rc.NotErrParamNotFound(err) {
		fs.Debugf("createEmptySrcDirs", "optional parameter is missing. using default value: %v", opt.CreateEmptySrcDirs)
	}
	if opt.RemoveEmptyDirs, err = in.GetBool("removeEmptyDirs"); rc.NotErrParamNotFound(err) {
		fs.Debugf("removeEmptyDirs", "optional parameter is missing. using default value: %v", opt.RemoveEmptyDirs)
	}
	if opt.NoCleanup, err = in.GetBool("noCleanup"); rc.NotErrParamNotFound(err) {
		fs.Debugf("noCleanup", "optional parameter is missing. using default value: %v", opt.NoCleanup)
	}
	if opt.IgnoreListingChecksum, err = in.GetBool("ignoreListingChecksum"); rc.NotErrParamNotFound(err) {
		fs.Debugf("ignoreListingChecksum", "optional parameter is missing. using default value: %v", opt.IgnoreListingChecksum)
	}
	if opt.Resilient, err = in.GetBool("resilient"); rc.NotErrParamNotFound(err) {
		fs.Debugf("resilient", "optional parameter is missing. using default value: %v", opt.Resilient)
	}
	if opt.CheckFilename, err = in.GetString("checkFilename"); rc.NotErrParamNotFound(err) {
		opt.CheckFilename = DefaultCheckFilename
		fs.Debugf("checkFilename", "optional parameter is missing. using default value: %v", opt.CheckFilename)
	}
	if opt.FiltersFile, err = in.GetString("filtersFile"); rc.NotErrParamNotFound(err) {
		fs.Debugf("filtersFile", "optional parameter is missing. using default value: %v", opt.FiltersFile)
	}
	if opt.Workdir, err = in.GetString("workdir"); rc.NotErrParamNotFound(err) {
		// "" sets correct default later
		fs.Debugf("workdir", "optional parameter is missing. using default value: %v", opt.Workdir)
	}
	if opt.BackupDir1, err = in.GetString("backupDir1"); rc.NotErrParamNotFound(err) {
		// we accept an alternate capitalization here for backward compatibility.
		if opt.BackupDir1, err = in.GetString("backupdir1"); rc.NotErrParamNotFound(err) {
			fs.Debugf("backupDir1", "optional parameter is missing. using default value: %v", opt.BackupDir1)
		}
	}
	if opt.BackupDir2, err = in.GetString("backupDir2"); rc.NotErrParamNotFound(err) {
		// we accept an alternate capitalization here for backward compatibility.
		if opt.BackupDir2, err = in.GetString("backupdir2"); rc.NotErrParamNotFound(err) {
			fs.Debugf("backupDir2", "optional parameter is missing. using default value: %v", opt.BackupDir2)
		}
	}
	if err = setEnum(in, "checkSync", "true", opt.CheckSync.Set); err != nil {
		return nil, err
	}
	if err = setEnum(in, "resyncMode", opt.ResyncMode.String(), opt.ResyncMode.Set); err != nil {
		return nil, err
	}
	if err = setEnum(in, "conflictResolve", opt.ConflictResolve.String(), opt.ConflictResolve.Set); err != nil {
		return nil, err
	}
	if err = setEnum(in, "conflictLoser", opt.ConflictLoser.String(), opt.ConflictLoser.Set); err != nil {
		return nil, err
	}
	if opt.ConflictSuffixFlag, err = in.GetString("conflictSuffix"); rc.NotErrParamNotFound(err) {
		fs.Debugf("conflictSuffix", "optional parameter is missing. using default value: %v", opt.ConflictSuffixFlag)
	}
	if opt.Recover, err = in.GetBool("recover"); rc.NotErrParamNotFound(err) {
		fs.Debugf("recover", "optional parameter is missing. using default value: %v", opt.Recover)
	}
	if opt.CompareFlag, err = in.GetString("compare"); rc.NotErrParamNotFound(err) {
		fs.Debugf("compare", "optional parameter is missing. using default value: %v", opt.CompareFlag)
	}
	if opt.Compare.NoSlowHash, err = in.GetBool("noSlowHash"); rc.NotErrParamNotFound(err) {
		fs.Debugf("noSlowHash", "optional parameter is missing. using default value: %v", opt.Compare.NoSlowHash)
	}
	if opt.Compare.SlowHashSyncOnly, err = in.GetBool("slowHashSyncOnly"); rc.NotErrParamNotFound(err) {
		fs.Debugf("slowHashSyncOnly", "optional parameter is missing. using default value: %v", opt.Compare.SlowHashSyncOnly)
	}
	if opt.Compare.DownloadHash, err = in.GetBool("downloadHash"); rc.NotErrParamNotFound(err) {
		fs.Debugf("downloadHash", "optional parameter is missing. using default value: %v", opt.Compare.DownloadHash)
	}
	if opt.MaxLock, err = in.GetFsDuration("maxLock"); rc.NotErrParamNotFound(err) {
		opt.MaxLock = 0
		fs.Debugf("maxLock", "optional parameter is missing. using default value: %v", opt.MaxLock)
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

	workDir, _ := filepath.Abs(DefaultWorkdir)
	if opt.Workdir != "" {
		workDir, _ = filepath.Abs(opt.Workdir)
	}
	basePath := bilib.BasePath(ctx, workDir, fs1, fs2)

	_, _ = log.Writer().Write(output)
	return rc.Params{
		"output":   string(output),
		"session":  bilib.SessionName(fs1, fs2),
		"workDir":  workDir,
		"basePath": basePath,
		"listing1": basePath + ".path1.lst",
		"listing2": basePath + ".path2.lst",
		"logFile":  fslog.Opt.File,
	}, err
}

func setEnum(in rc.Params, name string, defaultVal string, set func(s string) error) error {
	v, err := in.GetString(name)
	if rc.NotErrParamNotFound(err) || v == "" {
		v = defaultVal
		fs.Debugf(name, "optional parameter is missing. using default value: %v", v)
	}
	if err := set(v); err != nil {
		return err
	}
	return nil
}
