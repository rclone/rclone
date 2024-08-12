// Package about provides the about command.
package about

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"

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
	flags.BoolVarP(cmdFlags, &jsonOutput, "json", "", false, "Format output as JSON", "")
	flags.BoolVarP(cmdFlags, &fullOutput, "full", "", false, "Full numbers instead of human-readable", "")
}

// printValue formats uv to be output
func printValue(what string, uv *int64, isSize bool) {
	what += ":"
	if uv == nil {
		return
	}
	var val string
	if fullOutput {
		val = fmt.Sprintf("%d", *uv)
	} else if isSize {
		val = fs.SizeSuffix(*uv).ByteUnit()
	} else {
		val = fs.CountSuffix(*uv).String()
	}
	fmt.Printf("%-9s%v\n", what, val)
}

var commandDefinition = &cobra.Command{
	Use:   "about remote:",
	Short: `Get quota information from the remote.`,
	Long: `Prints quota information about a remote to standard
output. The output is typically used, free, quota and trash contents.

E.g. Typical output from ` + "`rclone about remote:`" + ` is:

    Total:   17 GiB
    Used:    7.444 GiB
    Free:    1.315 GiB
    Trashed: 100.000 MiB
    Other:   8.241 GiB

Where the fields are:

  * Total: Total size available.
  * Used: Total size used.
  * Free: Total space available to this user.
  * Trashed: Total space used by trash.
  * Other: Total amount in other storage (e.g. Gmail, Google Photos).
  * Objects: Total number of objects in the storage.

All sizes are in number of bytes.

Applying a ` + "`--full`" + ` flag to the command prints the bytes in full, e.g.

    Total:   18253611008
    Used:    7993453766
    Free:    1411001220
    Trashed: 104857602
    Other:   8849156022

A ` + "`--json`" + ` flag generates conveniently machine-readable output, e.g.

    {
        "total": 18253611008,
        "used": 7993453766,
        "trashed": 104857602,
        "other": 8849156022,
        "free": 1411001220
    }

Not all backends print all fields. Information is not included if it is not
provided by a backend. Where the value is unlimited it is omitted.

Some backends does not support the ` + "`rclone about`" + ` command at all,
see complete list in [documentation](https://rclone.org/overview/#optional-features).
`,
	Annotations: map[string]string{
		"versionIntroduced": "v1.41",
		// "groups":            "",
	},
	Run: func(command *cobra.Command, args []string) {
		cmd.CheckArgs(1, 1, command, args)
		f := cmd.NewFsSrc(args)
		cmd.Run(false, false, command, func() error {
			doAbout := f.Features().About
			if doAbout == nil {
				return fmt.Errorf("%v doesn't support about", f)
			}
			u, err := doAbout(context.Background())
			if err != nil {
				return fmt.Errorf("about call failed: %w", err)
			}
			if u == nil {
				return errors.New("nil usage returned")
			}
			if jsonOutput {
				out := json.NewEncoder(os.Stdout)
				out.SetIndent("", "\t")
				return out.Encode(u)
			}

			printValue("Total", u.Total, true)
			printValue("Used", u.Used, true)
			printValue("Free", u.Free, true)
			printValue("Trashed", u.Trashed, true)
			printValue("Other", u.Other, true)
			printValue("Objects", u.Objects, false)
			return nil
		})
	},
}
