package lsf

import (
	"fmt"
	"io"
	"os"

	"github.com/ncw/rclone/cmd"
	"github.com/ncw/rclone/fs"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

var (
	format    string
	separator string
	dirSlash  bool
	recurse   bool
)

func init() {
	cmd.Root.AddCommand(commandDefintion)
	flags := commandDefintion.Flags()
	flags.StringVarP(&format, "format", "F", "", "Output format.")
	flags.StringVarP(&separator, "separator", "s", "", "Separator.")
	flags.BoolVarP(&dirSlash, "dir-slash", "d", false, "Dir name contains slash one the end.")
	commandDefintion.Flags().BoolVarP(&recurse, "recursive", "R", false, "Recurse into the listing.")
}

var commandDefintion = &cobra.Command{
	Use:   "lsf remote:path",
	Short: `List all the objects in the path with modification time, size and path in specific format: 'p' - path, 's' - size, 't' - modification time, ex. 'tsp'. Default output contains only path. If format is empty, dir-slash flag is always true.`,
	Run: func(command *cobra.Command, args []string) {
		cmd.CheckArgs(1, 1, command, args)
		fsrc := cmd.NewFsSrc(args)
		cmd.Run(false, false, command, func() error {
			return Lsf(fsrc, os.Stdout)
		})
	},
}

//Lsf lists all the objects in the path with modification time, size and path in specific format.
func Lsf(fsrc fs.Fs, out io.Writer) error {
	return fs.Walk(fsrc, "", false, fs.ConfigMaxDepth(recurse), func(path string, entries fs.DirEntries, err error) error {
		if err != nil {
			fs.Stats.Error(err)
			fs.Errorf(path, "error listing: %v", err)
			return nil
		}
		if format == "" {
			format = "p"
			dirSlash = true
		}
		if separator == "" {
			separator = ";"
		}
		var list fs.ListFormat
		list.SetSeparator(separator)
		list.SetDirSlash(dirSlash)

		for _, char := range format {
			switch char {
			case 'p':
				list.AddPath()
			case 't':
				list.AddModTime()
			case 's':
				list.AddSize()
			default:
				return errors.Wrap(err, "failed to parse format argument")
			}
		}
		for _, entry := range entries {
			fmt.Fprintln(out, fs.ListFormatted(&entry, &list))
		}
		return nil
	})
}
