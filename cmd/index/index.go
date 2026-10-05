// Package index provides the index command.
package index

import (
	"context"
	_ "embed"
	"fmt"
	"strings"

	"github.com/rclone/rclone/cmd"
	"github.com/rclone/rclone/fs/config/flags"
	"github.com/rclone/rclone/fs/operations"
	"github.com/spf13/cobra"
)

var (
	opt           = operations.IndexOptDefault
	printTemplate string
)

func init() {
	cmd.Root.AddCommand(commandDefinition)
	cmdFlags := commandDefinition.Flags()
	flags.StringArrayVarP(cmdFlags, &opt.Outputs, "output", "", opt.Outputs, "Listing to write in each directory as NAME=FORMAT", "")
	flags.StringArrayVarP(cmdFlags, &opt.Rules.FilterRule, "index-filter", "", nil, "Add a rule for which directories get listings", "")
	flags.StringArrayVarP(cmdFlags, &opt.Rules.FilterFrom, "index-filter-from", "", nil, "Read directory rules from a file (use - to read from stdin)", "")
	flags.StringArrayVarP(cmdFlags, &opt.Rules.ExcludeRule, "index-exclude", "", nil, "Don't write listings in directories matching pattern", "")
	flags.StringArrayVarP(cmdFlags, &opt.Rules.ExcludeFrom, "index-exclude-from", "", nil, "Read directory exclude patterns from file (use - to read from stdin)", "")
	flags.StringArrayVarP(cmdFlags, &opt.Rules.IncludeRule, "index-include", "", nil, "Only write listings in directories matching pattern", "")
	flags.StringArrayVarP(cmdFlags, &opt.Rules.IncludeFrom, "index-include-from", "", nil, "Read directory include patterns from file (use - to read from stdin)", "")
	flags.IntVarP(cmdFlags, &opt.MaxDepth, "index-max-depth", "", opt.MaxDepth, "Only write listings this many directories deep (-1 for no limit)", "")
	flags.BoolVarP(cmdFlags, &opt.LinkIndex, "link-index", "", opt.LinkIndex, "Link to dir/NAME rather than dir/ for hosts without index documents", "")
	flags.FVarP(cmdFlags, &opt.DirTime, "dir-time", "", "How to work out the time shown for a directory", "")
	flags.BoolVarP(cmdFlags, &opt.NoModTime, "no-modtime", "", opt.NoModTime, "Don't show modification times in listings", "")
	flags.BoolVarP(cmdFlags, &opt.Rewrite, "index-rewrite", "", opt.Rewrite, "Write every listing even if it is unchanged", "")
	flags.StringArrayVarP(cmdFlags, &opt.Changed, "changed", "", nil, "Only re-index directories affected by this changed path (directories end in /)", "")
	flags.StringArrayVarP(cmdFlags, &opt.ChangedFrom, "changed-from", "", nil, "Read changed paths from file, one per line (use - to read from stdin)", "")
	flags.StringArrayVarP(cmdFlags, &opt.ChangedCombined, "changed-combined", "", nil, "Read changed paths from a sync --combined report (use - to read from stdin)", "")
	flags.IntVarP(cmdFlags, &opt.ChangedMaxDirs, "changed-max-dirs", "", opt.ChangedMaxDirs, "Do a full index if a partial one would list more than this many directories (0 for no limit)", "")
	flags.StringVarP(cmdFlags, &printTemplate, "print-template", "", "", "Print the built-in template for FORMAT and exit", "")
}

//go:embed index.md
var indexHelp string

var commandDefinition = &cobra.Command{
	Use:   "index remote:path",
	Short: `Write static directory listings into a remote.`,
	Long:  strings.TrimSpace(indexHelp),
	Annotations: map[string]string{
		"versionIntroduced": "v1.76",
		"groups":            "Copy,Filter,Listing,Important",
	},
	RunE: func(command *cobra.Command, args []string) error {
		if printTemplate != "" {
			text, err := operations.IndexTemplate(printTemplate)
			if err != nil {
				return err
			}
			fmt.Print(text)
			return nil
		}
		cmd.CheckArgs(1, 1, command, args)
		fdst := cmd.NewFsDir(args)
		cmd.Run(true, true, command, func() error {
			return operations.Index(context.Background(), fdst, &opt)
		})
		return nil
	},
}
