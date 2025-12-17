// Package operationsflags defines the flags used by rclone operations.
// It is decoupled into a separate package so it can be replaced.
package operationsflags

import (
	"context"
	_ "embed"
	"io"
	"os"
	"strings"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config/flags"
	"github.com/rclone/rclone/fs/hash"
	"github.com/rclone/rclone/fs/operations"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

//go:embed operationsflags.md
var help string

// Help returns the help string cleaned up to simplify appending
func Help() string {
	return strings.TrimSpace(help)
}

// AddLoggerFlagsOptions contains options for the Logger Flags
type AddLoggerFlagsOptions struct {
	Combined     string // a file with file names with leading sigils
	MissingOnSrc string // files only in the destination
	MissingOnDst string // files only in the source
	Match        string // matching files
	Differ       string // differing files
	ErrFile      string // files with errors of some kind
	DestAfter    string // files that exist on the destination post-sync
}

// AnySet checks if any of the logger flags have a non-blank value
func (o AddLoggerFlagsOptions) AnySet() bool {
	return anyNotBlank(o.Combined, o.MissingOnSrc, o.MissingOnDst, o.Match, o.Differ, o.ErrFile, o.DestAfter)
}

func anyNotBlank(s ...string) bool {
	for _, x := range s {
		if x != "" {
			return true
		}
	}
	return false
}

// AddLoggerFlags adds the logger flags to the cmdFlags command
func AddLoggerFlags(cmdFlags *pflag.FlagSet, opt *operations.LoggerOpt, flagsOpt *AddLoggerFlagsOptions) {
	flags.StringVarP(cmdFlags, &flagsOpt.Combined, "combined", "", flagsOpt.Combined, "Make a combined report of changes to this file", "Sync")
	flags.StringVarP(cmdFlags, &flagsOpt.MissingOnSrc, "missing-on-src", "", flagsOpt.MissingOnSrc, "Report all files missing from the source to this file", "Sync")
	flags.StringVarP(cmdFlags, &flagsOpt.MissingOnDst, "missing-on-dst", "", flagsOpt.MissingOnDst, "Report all files missing from the destination to this file", "Sync")
	flags.StringVarP(cmdFlags, &flagsOpt.Match, "match", "", flagsOpt.Match, "Report all matching files to this file", "Sync")
	flags.StringVarP(cmdFlags, &flagsOpt.Differ, "differ", "", flagsOpt.Differ, "Report all non-matching files to this file", "Sync")
	flags.StringVarP(cmdFlags, &flagsOpt.ErrFile, "error", "", flagsOpt.ErrFile, "Report all files with errors (hashing or reading) to this file", "Sync")
	flags.StringVarP(cmdFlags, &flagsOpt.DestAfter, "dest-after", "", flagsOpt.DestAfter, "Report all files that exist on the dest post-sync", "Sync")

	// lsf flags for destAfter
	flags.StringVarP(cmdFlags, &opt.Format, "format", "F", "p", "Output format - see lsf help for details", "Sync")
	flags.StringVarP(cmdFlags, &opt.TimeFormat, "timeformat", "t", "", "Specify a custom time format - see docs for details (default: 2006-01-02 15:04:05)", "")
	flags.StringVarP(cmdFlags, &opt.Separator, "separator", "s", ";", "Separator for the items in the format", "Sync")
	flags.BoolVarP(cmdFlags, &opt.DirSlash, "dir-slash", "d", true, "Append a slash to directory names", "Sync")
	opt.HashType = hash.MD5
	flags.FVarP(cmdFlags, &opt.HashType, "hash", "", "Use this hash when `h` is used in the format MD5|SHA-1|DropboxHash", "Sync")
	flags.BoolVarP(cmdFlags, &opt.FilesOnly, "files-only", "", true, "Only list files", "Sync")
	flags.BoolVarP(cmdFlags, &opt.DirsOnly, "dirs-only", "", false, "Only list directories", "Sync")
	flags.BoolVarP(cmdFlags, &opt.Csv, "csv", "", false, "Output in CSV format", "Sync")
	flags.BoolVarP(cmdFlags, &opt.Absolute, "absolute", "", false, "Put a leading / in front of path names", "Sync")
	// flags.BoolVarP(cmdFlags, &recurse, "recursive", "R", false, "Recurse into the listing", "")
}

// ConfigureLoggers verifies and sets up writers for log files requested via CLI flags
func ConfigureLoggers(ctx context.Context, fdst fs.Fs, command *cobra.Command, opt *operations.LoggerOpt, flagsOpt AddLoggerFlagsOptions) (func(), error) {
	closers := []io.Closer{}

	if opt.TimeFormat == "max" {
		opt.TimeFormat = operations.FormatForLSFPrecision(fdst.Precision())
	}
	opt.SetListFormat(ctx, command.Flags())
	opt.NewListJSON(ctx, fdst, "")

	open := func(name string, pout *io.Writer) error {
		if name == "" {
			return nil
		}
		if name == "-" {
			*pout = os.Stdout
			return nil
		}
		out, err := os.Create(name)
		if err != nil {
			return err
		}
		*pout = out
		closers = append(closers, out)
		return nil
	}

	if err := open(flagsOpt.Combined, &opt.Combined); err != nil {
		return nil, err
	}
	if err := open(flagsOpt.MissingOnSrc, &opt.MissingOnSrc); err != nil {
		return nil, err
	}
	if err := open(flagsOpt.MissingOnDst, &opt.MissingOnDst); err != nil {
		return nil, err
	}
	if err := open(flagsOpt.Match, &opt.Match); err != nil {
		return nil, err
	}
	if err := open(flagsOpt.Differ, &opt.Differ); err != nil {
		return nil, err
	}
	if err := open(flagsOpt.ErrFile, &opt.Error); err != nil {
		return nil, err
	}
	if err := open(flagsOpt.DestAfter, &opt.DestAfter); err != nil {
		return nil, err
	}

	close := func() {
		for _, closer := range closers {
			err := closer.Close()
			if err != nil {
				fs.Errorf(nil, "Failed to close report output: %v", err)
			}
		}
	}

	ci := fs.GetConfig(ctx)
	if ci.NoTraverse && opt.Combined != nil {
		fs.LogPrintf(fs.LogLevelWarning, nil, "--no-traverse does not list any deletes (-) in --combined output\n")
	}
	if ci.NoTraverse && opt.MissingOnSrc != nil {
		fs.LogPrintf(fs.LogLevelWarning, nil, "--no-traverse makes --missing-on-src produce empty output\n")
	}
	if ci.NoTraverse && opt.DestAfter != nil {
		fs.LogPrintf(fs.LogLevelWarning, nil, "--no-traverse makes --dest-after produce incomplete output\n")
	}

	return close, nil
}
