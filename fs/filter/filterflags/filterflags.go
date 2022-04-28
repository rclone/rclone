// Package filterflags implements command line flags to set up a filter
package filterflags

import (
	"context"

	"github.com/rclone/rclone/fs/config/flags"
	"github.com/rclone/rclone/fs/filter"
	"github.com/rclone/rclone/fs/rc"
	"github.com/spf13/pflag"
)

// Options set by command line flags
var (
	Opt = filter.DefaultOpt
)

// Reload the filters from the flags
func Reload(ctx context.Context) (err error) {
	fi := filter.GetConfig(ctx)
	newFilter, err := filter.NewFilter(&Opt)
	if err != nil {
		return err
	}
	*fi = *newFilter
	return nil
}

// AddFlags adds the non filing system specific flags to the command
func AddFlags(flagSet *pflag.FlagSet) {
	rc.AddOptionReload("filter", &Opt, Reload)
	flags.BoolVarP(flagSet, &Opt.DeleteExcluded, "delete-excluded", "", false, "Delete files on dest excluded from sync")
	flags.StringArrayVarP(flagSet, &Opt.FilterRule, "filter", "f", nil, "Add a file-filtering rule")
	flags.StringArrayVarP(flagSet, &Opt.FilterFrom, "filter-from", "", nil, "Read filtering patterns from a file (use - to read from stdin)")
	flags.StringArrayVarP(flagSet, &Opt.ExcludeRule, "exclude", "", nil, "Exclude files matching pattern")
	flags.StringArrayVarP(flagSet, &Opt.ExcludeFrom, "exclude-from", "", nil, "Read exclude patterns from file (use - to read from stdin)")
	flags.StringVarP(flagSet, &Opt.ExcludeFile, "exclude-if-present", "", "", "Exclude directories if filename is present")
	flags.StringArrayVarP(flagSet, &Opt.IncludeRule, "include", "", nil, "Include files matching pattern")
	flags.StringArrayVarP(flagSet, &Opt.IncludeFrom, "include-from", "", nil, "Read include patterns from file (use - to read from stdin)")
	flags.StringArrayVarP(flagSet, &Opt.FilesFrom, "files-from", "", nil, "Read list of source-file names from file (use - to read from stdin)")
	flags.StringArrayVarP(flagSet, &Opt.FilesFromRaw, "files-from-raw", "", nil, "Read list of source-file names from file without any processing of lines (use - to read from stdin)")
	flags.FVarP(flagSet, &Opt.MinAge, "min-age", "", "Only transfer files older than this in s or suffix ms|s|m|h|d|w|M|y")
	flags.FVarP(flagSet, &Opt.MaxAge, "max-age", "", "Only transfer files younger than this in s or suffix ms|s|m|h|d|w|M|y")
	flags.FVarP(flagSet, &Opt.MinSize, "min-size", "", "Only transfer files bigger than this in KiB or suffix B|K|M|G|T|P")
	flags.FVarP(flagSet, &Opt.MaxSize, "max-size", "", "Only transfer files smaller than this in KiB or suffix B|K|M|G|T|P")
	flags.BoolVarP(flagSet, &Opt.IgnoreCase, "ignore-case", "", false, "Ignore case in filters (case insensitive)")
	//cvsExclude     = BoolP("cvs-exclude", "C", false, "Exclude files in the same way CVS does")
}
