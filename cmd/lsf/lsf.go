package lsf

import (
	"fmt"
	"strconv"

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
	Short: `List all the objects in the path with modification time, size and path in specific format: 'p' - path, 's' - size, 't' - modification time, ex. 'tsp'.`,
	Run: func(command *cobra.Command, args []string) {
		cmd.CheckArgs(1, 1, command, args)
		fsrc := cmd.NewFsSrc(args)
		cmd.Run(false, false, command, func() error {
			return fs.Walk(fsrc, "", false, fs.ConfigMaxDepth(recurse), func(path string, entries fs.DirEntries, err error) error {
				if err != nil {
					fs.Stats.Error(err)
					fs.Errorf(path, "error listing: %v", err)
					return nil
				}
				for _, char := range format {
					switch char {
					case
						'p',
						't',
						's':
						continue
					default:
						return errors.Wrap(err, "failed to parse date/time argument")
					}
				}
				if separator == "" {
					separator = ";"
				}
				for _, entry := range entries {
					_, isDir := entry.(fs.Directory)
					var pathInformation string
					for _, char := range format {
						switch char {
						case 'p':
							pathInformation += entry.Remote()
							if isDir && dirSlash {
								pathInformation += "/"
							}
							pathInformation += separator
						case 't':
							pathInformation += entry.ModTime().Format("2006-01-02 15:04:05") + separator
						case 's':
							pathInformation += strconv.FormatInt(entry.Size(), 10) + separator
						}
					}
					fmt.Println(pathInformation[:len(pathInformation)-len(separator)])
				}
				return nil
			})
		})
	},
}
