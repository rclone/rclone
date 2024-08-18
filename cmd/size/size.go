// Package size provides the size command.
package size

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strconv"

	"github.com/rclone/rclone/cmd"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config/flags"
	"github.com/rclone/rclone/fs/operations"
	"github.com/spf13/cobra"
)

var jsonOutput bool

func init() {
	cmd.Root.AddCommand(commandDefinition)
	cmdFlags := commandDefinition.Flags()
	flags.BoolVarP(cmdFlags, &jsonOutput, "json", "", false, "Format output as JSON", "")
}

var commandDefinition = &cobra.Command{
	Use:   "size remote:path",
	Short: `Prints the total size and number of objects in remote:path.`,
	Long: `Counts objects in the path and calculates the total size. Prints the
result to standard output.

By default the output is in human-readable format, but shows values in
both human-readable format as well as the raw numbers (global option
` + "`--human-readable`" + ` is not considered). Use option ` + "`--json`" + `
to format output as JSON instead.

Recurses by default, use ` + "`--max-depth 1`" + ` to stop the
recursion.

Some backends do not always provide file sizes, see for example
[Google Photos](/googlephotos/#size) and
[Google Docs](/drive/#limitations-of-google-docs).
Rclone will then show a notice in the log indicating how many such
files were encountered, and count them in as empty files in the output
of the size command.
`,
	Annotations: map[string]string{
		"versionIntroduced": "v1.23",
		"groups":            "Filter,Listing",
	},
	Run: func(command *cobra.Command, args []string) {
		cmd.CheckArgs(1, 1, command, args)
		fsrc := cmd.NewFsSrc(args)
		cmd.Run(false, false, command, func() error {
			var err error
			var results struct {
				Count    int64 `json:"count"`
				Bytes    int64 `json:"bytes"`
				Sizeless int64 `json:"sizeless"`
			}

			results.Count, results.Bytes, results.Sizeless, err = operations.Count(context.Background(), fsrc)
			if err != nil {
				return err
			}
			if results.Sizeless > 0 {
				fs.Logf(fsrc, "Size may be underestimated due to %d objects with unknown size", results.Sizeless)
			}
			if jsonOutput {
				return json.NewEncoder(os.Stdout).Encode(results)
			}
			count := strconv.FormatInt(results.Count, 10)
			countSuffix := fs.CountSuffix(results.Count).String()
			if count == countSuffix {
				fmt.Printf("Total objects: %s\n", count)
			} else {
				fmt.Printf("Total objects: %s (%s)\n", countSuffix, count)
			}
			fmt.Printf("Total size: %s (%d Byte)\n", fs.SizeSuffix(results.Bytes).ByteUnit(), results.Bytes)
			if results.Sizeless > 0 {
				fmt.Printf("Total objects with unknown size: %s (%d)\n", fs.CountSuffix(results.Sizeless), results.Sizeless)
			}
			return nil
		})
	},
}
