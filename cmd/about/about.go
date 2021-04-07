package about

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/pkg/errors"
	"github.com/rclone/rclone/cmd"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config/flags"
	"github.com/spf13/cobra"
)

var (
	jsonOutput bool
	fullOutput bool
)

func init() {
	cmd.Root.AddCommand(commandDefinition)
	cmdFlags := commandDefinition.Flags()
	flags.BoolVarP(cmdFlags, &jsonOutput, "json", "", false, "Format output as JSON")
	flags.BoolVarP(cmdFlags, &fullOutput, "full", "", false, "Full numbers instead of SI units")
}

// printValue formats uv to be output
func printValue(what string, uv *int64) {
	what += ":"
	if uv == nil {
		return
	}
	var val string
	if fullOutput {
		val = fmt.Sprintf("%d", *uv)
	} else {
		val = fs.SizeSuffix(*uv).String()
	}
	fmt.Printf("%-9s%v\n", what, val)
}

var commandDefinition = &cobra.Command{
	Use:   "about remote:",
	Short: `Get quota information from the remote.`,
	Long: `
` + "`rclone about`" + ` prints quota information about a remote to standard
output. The output is typically used, free, quota and trash contents.

E.g. Typical output from ` + "`rclone about remote:`" + ` is:

    Total:   17G
    Used:    7.444G
    Free:    1.315G
    Trashed: 100.000M
    Other:   8.241G

Where the fields are:

  * Total: total size available.
  * Used: total size used
  * Free: total space available to this user.
  * Trashed: total space used by trash
  * Other: total amount in other storage (e.g. Gmail, Google Photos)
  * Objects: total number of objects in the storage

Not all backends print all fields. Information is not included if it is not
provided by a backend. Where the value is unlimited it is omitted.

Applying a ` + "`--full`" + ` flag to the command prints the bytes in full, e.g.

    Total:   18253611008
    Used:    7993453766
    Free:    1411001220
    Trashed: 104857602
    Other:   8849156022

A ` + "`--json`" + ` flag generates conveniently computer readable output, e.g.

    {
        "total": 18253611008,
        "used": 7993453766,
        "trashed": 104857602,
        "other": 8849156022,
        "free": 1411001220
    }

Not all backends support the ` + "`rclone about`" + ` command.

See [List of backends that do not support about](https://rclone.org/overview/#optional-features)
`,
	Run: func(command *cobra.Command, args []string) {
		cmd.CheckArgs(1, 1, command, args)
		f := cmd.NewFsSrc(args)
		cmd.Run(false, false, command, func() error {
			doAbout := f.Features().About
			if doAbout == nil {
				return errors.Errorf("%v doesn't support about", f)
			}
			u, err := doAbout(context.Background())
			if err != nil {
				return errors.Wrap(err, "About call failed")
			}
			if u == nil {
				return errors.New("nil usage returned")
			}
			if jsonOutput {
				out := json.NewEncoder(os.Stdout)
				out.SetIndent("", "\t")
				return out.Encode(u)
			}
			printValue("Total", u.Total)
			printValue("Used", u.Used)
			printValue("Free", u.Free)
			printValue("Trashed", u.Trashed)
			printValue("Other", u.Other)
			printValue("Objects", u.Objects)
			return nil
		})
	},
}
