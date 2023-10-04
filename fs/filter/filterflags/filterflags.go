// Package filterflags implements command line flags to set up a filter
package filterflags

import (
	"context"
	"fmt"

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

// AddRuleFlags add a set of rules flags with prefix
func AddRuleFlags(flagSet *pflag.FlagSet, Opt *filter.RulesOpt, what, prefix string) {
	shortFilter := ""
	if prefix == "" {
		shortFilter = "f"
	}
	group := "Filter"
	if prefix == "metadata-" {
		group += ",Metadata"
	}
	flags.StringArrayVarP(flagSet, &Opt.FilterRule, prefix+"filter", shortFilter, nil, fmt.Sprintf("Add a %s filtering rule", what), group)
	flags.StringArrayVarP(flagSet, &Opt.FilterFrom, prefix+"filter-from", "", nil, fmt.Sprintf("Read %s filtering patterns from a file (use - to read from stdin)", what), group)
	flags.StringArrayVarP(flagSet, &Opt.ExcludeRule, prefix+"exclude", "", nil, fmt.Sprintf("Exclude %ss matching pattern", what), group)
	flags.StringArrayVarP(flagSet, &Opt.ExcludeFrom, prefix+"exclude-from", "", nil, fmt.Sprintf("Read %s exclude patterns from file (use - to read from stdin)", what), group)
	flags.StringArrayVarP(flagSet, &Opt.IncludeRule, prefix+"include", "", nil, fmt.Sprintf("Include %ss matching pattern", what), group)
	flags.StringArrayVarP(flagSet, &Opt.IncludeFrom, prefix+"include-from", "", nil, fmt.Sprintf("Read %s include patterns from file (use - to read from stdin)", what), group)
}

// AddFlags adds the non filing system specific flags to the command
func AddFlags(flagSet *pflag.FlagSet) {
	rc.AddOptionReload("filter", &Opt, Reload)
	flags.BoolVarP(flagSet, &Opt.DeleteExcluded, "delete-excluded", "", false, "Delete files on dest excluded from sync", "Filter")
	AddRuleFlags(flagSet, &Opt.RulesOpt, "file", "")
	AddRuleFlags(flagSet, &Opt.MetaRules, "metadata", "metadata-")
	flags.StringArrayVarP(flagSet, &Opt.ExcludeFile, "exclude-if-present", "", nil, "Exclude directories if filename is present", "Filter")
	flags.StringArrayVarP(flagSet, &Opt.FilesFrom, "files-from", "", nil, "Read list of source-file names from file (use - to read from stdin)", "Filter")
	flags.StringArrayVarP(flagSet, &Opt.FilesFromRaw, "files-from-raw", "", nil, "Read list of source-file names from file without any processing of lines (use - to read from stdin)", "Filter")
	flags.FVarP(flagSet, &Opt.MinAge, "min-age", "", "Only transfer files older than this in s or suffix ms|s|m|h|d|w|M|y", "Filter")
	flags.FVarP(flagSet, &Opt.MaxAge, "max-age", "", "Only transfer files younger than this in s or suffix ms|s|m|h|d|w|M|y", "Filter")
	flags.FVarP(flagSet, &Opt.MinSize, "min-size", "", "Only transfer files bigger than this in KiB or suffix B|K|M|G|T|P", "Filter")
	flags.FVarP(flagSet, &Opt.MaxSize, "max-size", "", "Only transfer files smaller than this in KiB or suffix B|K|M|G|T|P", "Filter")
	flags.BoolVarP(flagSet, &Opt.IgnoreCase, "ignore-case", "", false, "Ignore case in filters (case insensitive)", "Filter")
	//cvsExclude     = BoolP("cvs-exclude", "C", false, "Exclude files in the same way CVS does")
}
