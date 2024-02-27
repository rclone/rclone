// Package bisync implements bisync
// Copyright (c) 2017-2020 Chris Nelson
package bisync

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
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

	"github.com/spf13/cobra"
)

// TestFunc allows mocking errors during tests
type TestFunc func()

// Options keep bisync options
type Options struct {
	Resync                bool   // whether or not this is a resync
	ResyncMode            Prefer // which mode to use for resync
	CheckAccess           bool
	CheckFilename         string
	CheckSync             CheckSyncMode
	CreateEmptySrcDirs    bool
	RemoveEmptyDirs       bool
	MaxDelete             int // percentage from 0 to 100
	Force                 bool
	FiltersFile           string
	Workdir               string
	OrigBackupDir         string
	BackupDir1            string
	BackupDir2            string
	DryRun                bool
	NoCleanup             bool
	SaveQueues            bool // save extra debugging files (test only flag)
	IgnoreListingChecksum bool
	Resilient             bool
	Recover               bool
	TestFn                TestFunc // test-only option, for mocking errors
	Compare               CompareOpt
	CompareFlag           string
	DebugName             string
	MaxLock               time.Duration
	ConflictResolve       Prefer
	ConflictLoser         ConflictLoserAction
	ConflictSuffixFlag    string
	ConflictSuffix1       string
	ConflictSuffix2       string
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
		return fmt.Errorf("unknown check-sync mode for bisync: %q", s)
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
	Opt.MaxLock = 0
	cmd.Root.AddCommand(commandDefinition)
	cmdFlags := commandDefinition.Flags()
	// when adding new flags, remember to also update the rc params:
	// cmd/bisync/rc.go cmd/bisync/help.go (not docs/content/rc.md)
	// and the Command line syntax section of docs/content/bisync.md (it doesn't update automatically)
	flags.BoolVarP(cmdFlags, &Opt.Resync, "resync", "1", Opt.Resync, "Performs the resync run. Equivalent to --resync-mode path1. Consider using --verbose or --dry-run first.", "")
	flags.FVarP(cmdFlags, &Opt.ResyncMode, "resync-mode", "", "During resync, prefer the version that is: path1, path2, newer, older, larger, smaller (default: path1 if --resync, otherwise none for no resync.)", "")
	flags.BoolVarP(cmdFlags, &Opt.CheckAccess, "check-access", "", Opt.CheckAccess, makeHelp("Ensure expected {CHECKFILE} files are found on both Path1 and Path2 filesystems, else abort."), "")
	flags.StringVarP(cmdFlags, &Opt.CheckFilename, "check-filename", "", Opt.CheckFilename, makeHelp("Filename for --check-access (default: {CHECKFILE})"), "")
	flags.BoolVarP(cmdFlags, &Opt.Force, "force", "", Opt.Force, "Bypass --max-delete safety check and run the sync. Consider using with --verbose", "")
	flags.FVarP(cmdFlags, &Opt.CheckSync, "check-sync", "", "Controls comparison of final listings: true|false|only (default: true)", "")
	flags.BoolVarP(cmdFlags, &Opt.CreateEmptySrcDirs, "create-empty-src-dirs", "", Opt.CreateEmptySrcDirs, "Sync creation and deletion of empty directories. (Not compatible with --remove-empty-dirs)", "")
	flags.BoolVarP(cmdFlags, &Opt.RemoveEmptyDirs, "remove-empty-dirs", "", Opt.RemoveEmptyDirs, "Remove ALL empty directories at the final cleanup step.", "")
	flags.StringVarP(cmdFlags, &Opt.FiltersFile, "filters-file", "", Opt.FiltersFile, "Read filtering patterns from a file", "")
	flags.StringVarP(cmdFlags, &Opt.Workdir, "workdir", "", Opt.Workdir, makeHelp("Use custom working dir - useful for testing. (default: {WORKDIR})"), "")
	flags.StringVarP(cmdFlags, &Opt.BackupDir1, "backup-dir1", "", Opt.BackupDir1, "--backup-dir for Path1. Must be a non-overlapping path on the same remote.", "")
	flags.StringVarP(cmdFlags, &Opt.BackupDir2, "backup-dir2", "", Opt.BackupDir2, "--backup-dir for Path2. Must be a non-overlapping path on the same remote.", "")
	flags.StringVarP(cmdFlags, &Opt.DebugName, "debugname", "", Opt.DebugName, "Debug by tracking one file at various points throughout a bisync run (when -v or -vv)", "")
	flags.BoolVarP(cmdFlags, &tzLocal, "localtime", "", tzLocal, "Use local time in listings (default: UTC)", "")
	flags.BoolVarP(cmdFlags, &Opt.NoCleanup, "no-cleanup", "", Opt.NoCleanup, "Retain working files (useful for troubleshooting and testing).", "")
	flags.BoolVarP(cmdFlags, &Opt.IgnoreListingChecksum, "ignore-listing-checksum", "", Opt.IgnoreListingChecksum, "Do not use checksums for listings (add --ignore-checksum to additionally skip post-copy checksum checks)", "")
	flags.BoolVarP(cmdFlags, &Opt.Resilient, "resilient", "", Opt.Resilient, "Allow future runs to retry after certain less-serious errors, instead of requiring --resync. Use at your own risk!", "")
	flags.BoolVarP(cmdFlags, &Opt.Recover, "recover", "", Opt.Recover, "Automatically recover from interruptions without requiring --resync.", "")
	flags.StringVarP(cmdFlags, &Opt.CompareFlag, "compare", "", Opt.CompareFlag, "Comma-separated list of bisync-specific compare options ex. 'size,modtime,checksum' (default: 'size,modtime')", "")
	flags.BoolVarP(cmdFlags, &Opt.Compare.NoSlowHash, "no-slow-hash", "", Opt.Compare.NoSlowHash, "Ignore listing checksums only on backends where they are slow", "")
	flags.BoolVarP(cmdFlags, &Opt.Compare.SlowHashSyncOnly, "slow-hash-sync-only", "", Opt.Compare.SlowHashSyncOnly, "Ignore slow checksums for listings and deltas, but still consider them during sync calls.", "")
	flags.BoolVarP(cmdFlags, &Opt.Compare.DownloadHash, "download-hash", "", Opt.Compare.DownloadHash, "Compute hash by downloading when otherwise unavailable. (warning: may be slow and use lots of data!)", "")
	flags.DurationVarP(cmdFlags, &Opt.MaxLock, "max-lock", "", Opt.MaxLock, "Consider lock files older than this to be expired (default: 0 (never expire)) (minimum: 2m)", "")
	flags.FVarP(cmdFlags, &Opt.ConflictResolve, "conflict-resolve", "", "Automatically resolve conflicts by preferring the version that is: "+ConflictResolveList+" (default: none)", "")
	flags.FVarP(cmdFlags, &Opt.ConflictLoser, "conflict-loser", "", "Action to take on the loser of a sync conflict (when there is a winner) or on both files (when there is no winner): "+ConflictLoserList+" (default: num)", "")
	flags.StringVarP(cmdFlags, &Opt.ConflictSuffixFlag, "conflict-suffix", "", Opt.ConflictSuffixFlag, "Suffix to use when renaming a --conflict-loser. Can be either one string or two comma-separated strings to assign different suffixes to Path1/Path2. (default: 'conflict')", "")
	_ = cmdFlags.MarkHidden("debugname")
	_ = cmdFlags.MarkHidden("localtime")
}

// bisync command definition
var commandDefinition = &cobra.Command{
	Use:   "bisync remote1:path1 remote2:path2",
	Short: shortHelp,
	Long:  longHelp,
	Annotations: map[string]string{
		"versionIntroduced": "v1.58",
		"groups":            "Filter,Copy,Important",
		"status":            "Beta",
	},
	RunE: func(command *cobra.Command, args []string) error {
		// NOTE: avoid putting too much handling here, as it won't apply to the rc.
		// Generally it's best to put init-type stuff in Bisync() (operations.go)
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

		fs.Logf(nil, "bisync is IN BETA. Don't use in production!")
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
		return ctx, fmt.Errorf("specified filters file does not exist: %s", filtersFile)
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
	wantHash, err := os.ReadFile(hashFile)
	if err != nil && !opt.Resync {
		return ctx, fmt.Errorf("filters file md5 hash not found (must run --resync): %s", filtersFile)
	}

	if gotHash != string(wantHash) && !opt.Resync {
		return ctx, fmt.Errorf("filters file has changed (must run --resync): %s", filtersFile)
	}

	if opt.Resync {
		if opt.DryRun {
			fs.Infof(nil, "Skipped storing filters file hash to %s as --dry-run is set", hashFile)
		} else {
			fs.Infof(nil, "Storing filters file hash to %s", hashFile)
			if err := os.WriteFile(hashFile, []byte(gotHash), bilib.PermSecure); err != nil {
				return ctx, err
			}
		}
	}

	// Prepend our filter file first in the list
	filterOpt := filter.GetConfig(ctx).Opt
	filterOpt.FilterFrom = append([]string{filtersFile}, filterOpt.FilterFrom...)
	newFilter, err := filter.NewFilter(&filterOpt)
	if err != nil {
		return ctx, fmt.Errorf("invalid filters file: %s: %w", filtersFile, err)
	}

	return filter.ReplaceConfig(ctx, newFilter), nil
}
