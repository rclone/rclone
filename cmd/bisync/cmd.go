// Package bisync implements bisync
// Copyright (c) 2017-2020 Chris Nelson
package bisync

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/rclone/rclone/cmd"
	"github.com/rclone/rclone/cmd/bisync/bilib"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config"
	"github.com/rclone/rclone/fs/config/flags"
	"github.com/rclone/rclone/fs/filter"
	"github.com/rclone/rclone/fs/hash"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

// Options keep bisync options
type Options struct {
	Resync          bool
	CheckAccess     bool
	CheckFilename   string
	CheckSync       CheckSyncMode
	RemoveEmptyDirs bool
	MaxDelete       int // percentage from 0 to 100
	Force           bool
	FiltersFile     string
	Workdir         string
	DryRun          bool
	NoCleanup       bool
	SaveQueues      bool // save extra debugging files (test only flag)
}

// Default values
const (
	DefaultMaxDelete     int    = 50
	DefaultCheckFilename string = "RCLONE_TEST"
)

// DefaultWorkdir is default working directory
var DefaultWorkdir = filepath.Join(config.GetCacheDir(), "bisync")

// CheckSyncMode controls when to compare final listings
type CheckSyncMode int

// CheckSync modes
const (
	CheckSyncTrue  CheckSyncMode = iota // Compare final listings (default)
	CheckSyncFalse                      // Disable comparison of final listings
	CheckSyncOnly                       // Only compare listings from the last run, do not sync
)

func (x CheckSyncMode) String() string {
	switch x {
	case CheckSyncTrue:
		return "true"
	case CheckSyncFalse:
		return "false"
	case CheckSyncOnly:
		return "only"
	}
	return "unknown"
}

// Set a CheckSync mode from a string
func (x *CheckSyncMode) Set(s string) error {
	switch strings.ToLower(s) {
	case "true":
		*x = CheckSyncTrue
	case "false":
		*x = CheckSyncFalse
	case "only":
		*x = CheckSyncOnly
	default:
		return errors.Errorf("unknown check-sync mode for bisync: %q", s)
	}
	return nil
}

// Type of the CheckSync value
func (x *CheckSyncMode) Type() string {
	return "string"
}

// Opt keeps command line options
var Opt Options

func init() {
	cmd.Root.AddCommand(commandDefinition)
	cmdFlags := commandDefinition.Flags()
	flags.BoolVarP(cmdFlags, &Opt.Resync, "resync", "1", Opt.Resync, "Performs the resync run. Path1 files may overwrite Path2 versions. Consider using --verbose or --dry-run first.")
	flags.BoolVarP(cmdFlags, &Opt.CheckAccess, "check-access", "", Opt.CheckAccess, makeHelp("Ensure expected {CHECKFILE} files are found on both Path1 and Path2 filesystems, else abort."))
	flags.StringVarP(cmdFlags, &Opt.CheckFilename, "check-filename", "", Opt.CheckFilename, makeHelp("Filename for --check-access (default: {CHECKFILE})"))
	flags.BoolVarP(cmdFlags, &Opt.Force, "force", "", Opt.Force, "Bypass --max-delete safety check and run the sync. Consider using with --verbose")
	flags.FVarP(cmdFlags, &Opt.CheckSync, "check-sync", "", "Controls comparison of final listings: true|false|only (default: true)")
	flags.BoolVarP(cmdFlags, &Opt.RemoveEmptyDirs, "remove-empty-dirs", "", Opt.RemoveEmptyDirs, "Remove empty directories at the final cleanup step.")
	flags.StringVarP(cmdFlags, &Opt.FiltersFile, "filters-file", "", Opt.FiltersFile, "Read filtering patterns from a file")
	flags.StringVarP(cmdFlags, &Opt.Workdir, "workdir", "", Opt.Workdir, makeHelp("Use custom working dir - useful for testing. (default: {WORKDIR})"))
	flags.BoolVarP(cmdFlags, &tzLocal, "localtime", "", tzLocal, "Use local time in listings (default: UTC)")
	flags.BoolVarP(cmdFlags, &Opt.NoCleanup, "no-cleanup", "", Opt.NoCleanup, "Retain working files (useful for troubleshooting and testing).")
}

// bisync command definition
var commandDefinition = &cobra.Command{
	Use:   "bisync remote1:path1 remote2:path2",
	Short: shortHelp,
	Long:  longHelp,
	RunE: func(command *cobra.Command, args []string) error {
		cmd.CheckArgs(2, 2, command, args)
		fs1, file1, fs2, file2 := cmd.NewFsSrcDstFiles(args)
		if file1 != "" || file2 != "" {
			return errors.New("paths must be existing directories")
		}

		ctx := context.Background()
		opt := Opt
		opt.applyContext(ctx)

		if tzLocal {
			TZ = time.Local
		}

		commonHashes := fs1.Hashes().Overlap(fs2.Hashes())
		isDropbox1 := strings.HasPrefix(fs1.String(), "Dropbox")
		isDropbox2 := strings.HasPrefix(fs2.String(), "Dropbox")
		if commonHashes == hash.Set(0) && (isDropbox1 || isDropbox2) {
			ci := fs.GetConfig(ctx)
			if !ci.DryRun && !ci.RefreshTimes {
				fs.Debugf(nil, "Using flag --refresh-times is recommended")
			}
		}

		fs.Logf(nil, "bisync is EXPERIMENTAL. Don't use in production!")
		cmd.Run(false, true, command, func() error {
			err := Bisync(ctx, fs1, fs2, &opt)
			if err == ErrBisyncAborted {
				os.Exit(2)
			}
			return err
		})
		return nil
	},
}

func (opt *Options) applyContext(ctx context.Context) {
	maxDelete := DefaultMaxDelete
	ci := fs.GetConfig(ctx)
	if ci.MaxDelete >= 0 {
		maxDelete = int(ci.MaxDelete)
	}
	if maxDelete < 0 {
		maxDelete = 0
	}
	if maxDelete > 100 {
		maxDelete = 100
	}
	opt.MaxDelete = maxDelete
	// reset MaxDelete for fs/operations, bisync handles this parameter specially
	ci.MaxDelete = -1
	opt.DryRun = ci.DryRun
}

func (opt *Options) setDryRun(ctx context.Context) context.Context {
	ctxNew, ci := fs.AddConfig(ctx)
	ci.DryRun = opt.DryRun
	return ctxNew
}

func (opt *Options) applyFilters(ctx context.Context) (context.Context, error) {
	filtersFile := opt.FiltersFile
	if filtersFile == "" {
		return ctx, nil
	}

	f, err := os.Open(filtersFile)
	if err != nil {
		return ctx, errors.Errorf("specified filters file does not exist: %s", filtersFile)
	}

	fs.Infof(nil, "Using filters file %s", filtersFile)
	hasher := md5.New()
	if _, err := io.Copy(hasher, f); err != nil {
		_ = f.Close()
		return ctx, err
	}
	gotHash := hex.EncodeToString(hasher.Sum(nil))
	_ = f.Close()

	hashFile := filtersFile + ".md5"
	wantHash, err := ioutil.ReadFile(hashFile)
	if err != nil && !opt.Resync {
		return ctx, errors.Errorf("filters file md5 hash not found (must run --resync): %s", filtersFile)
	}

	if gotHash != string(wantHash) && !opt.Resync {
		return ctx, errors.Errorf("filters file has changed (must run --resync): %s", filtersFile)
	}

	if opt.Resync {
		fs.Infof(nil, "Storing filters file hash to %s", hashFile)
		if err := ioutil.WriteFile(hashFile, []byte(gotHash), bilib.PermSecure); err != nil {
			return ctx, err
		}
	}

	// Prepend our filter file first in the list
	filterOpt := filter.GetConfig(ctx).Opt
	filterOpt.FilterFrom = append([]string{filtersFile}, filterOpt.FilterFrom...)
	newFilter, err := filter.NewFilter(&filterOpt)
	if err != nil {
		return ctx, errors.Wrapf(err, "invalid filters file: %s", filtersFile)
	}

	return filter.ReplaceConfig(ctx, newFilter), nil
}
