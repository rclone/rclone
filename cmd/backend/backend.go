// Package backend provides the backend command.
package backend

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sort"

	"github.com/rclone/rclone/cmd"
	"github.com/rclone/rclone/cmd/rc"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config/flags"
	"github.com/rclone/rclone/fs/operations"
	"github.com/spf13/cobra"
)

var (
	options []string
	useJSON bool
)

func init() {
	cmd.Root.AddCommand(commandDefinition)
	cmdFlags := commandDefinition.Flags()
	flags.StringArrayVarP(cmdFlags, &options, "option", "o", options, "Option in the form name=value or name", "")
	flags.BoolVarP(cmdFlags, &useJSON, "json", "", useJSON, "Always output in JSON format", "")
}

var commandDefinition = &cobra.Command{
	Use:   "backend <command> remote:path [opts] <args>",
	Short: `Run a backend-specific command.`,
	Long: `This runs a backend-specific command. The commands themselves (except
for "help" and "features") are defined by the backends and you should
see the backend docs for definitions.

You can discover what commands a backend implements by using

    rclone backend help remote:
    rclone backend help <backendname>

You can also discover information about the backend using (see
[operations/fsinfo](/rc/#operations-fsinfo) in the remote control docs
for more info).

    rclone backend features remote:

Pass options to the backend command with -o. This should be key=value or key, e.g.:

    rclone backend stats remote:path stats -o format=json -o long

Pass arguments to the backend by placing them on the end of the line

    rclone backend cleanup remote:path file1 file2 file3

Note to run these commands on a running backend then see
[backend/command](/rc/#backend-command) in the rc docs.
`,
	Annotations: map[string]string{
		"versionIntroduced": "v1.52",
		"groups":            "Important",
	},
	RunE: func(command *cobra.Command, args []string) error {
		cmd.CheckArgs(2, 1e6, command, args)
		name, remote := args[0], args[1]
		cmd.Run(false, false, command, func() error {
			// show help if remote is a backend name
			if name == "help" {
				fsInfo, err := fs.Find(remote)
				if err == nil {
					return showHelp(fsInfo)
				}
			}
			// Create remote
			fsInfo, configName, fsPath, config, err := fs.ConfigFs(remote)
			if err != nil {
				return err
			}
			f, err := fsInfo.NewFs(context.Background(), configName, fsPath, config)
			if err != nil {
				return err
			}
			// Run the command
			var out interface{}
			switch name {
			case "help":
				return showHelp(fsInfo)
			case "features":
				out = operations.GetFsInfo(f)
			default:
				doCommand := f.Features().Command
				if doCommand == nil {
					return fmt.Errorf("%v: doesn't support backend commands", f)
				}
				arg := args[2:]
				opt := rc.ParseOptions(options)
				out, err = doCommand(context.Background(), name, arg, opt)
			}
			if err != nil {
				if err == fs.ErrorCommandNotFound {
					extra := ""
					if f.Features().Overlay {
						extra = " (try the underlying remote)"
					}
					return fmt.Errorf("%q %w%s", name, err, extra)
				}
				return fmt.Errorf("command %q failed: %w", name, err)
			}
			// Output the result
			writeJSON := false
			if useJSON {
				writeJSON = true
			} else {
				switch x := out.(type) {
				case nil:
				case string:
					fmt.Println(out)
				case []string:
					for _, line := range x {
						fmt.Println(line)
					}
				default:
					writeJSON = true
				}
			}
			if writeJSON {
				// Write indented JSON to the output
				enc := json.NewEncoder(os.Stdout)
				enc.SetIndent("", "\t")
				err = enc.Encode(out)
				if err != nil {
					return fmt.Errorf("failed to write JSON: %w", err)
				}
			}
			return nil
		})
		return nil
	},
}

// show help for a backend
func showHelp(fsInfo *fs.RegInfo) error {
	cmds := fsInfo.CommandHelp
	name := fsInfo.Name
	if len(cmds) == 0 {
		return fmt.Errorf("%s backend has no commands", name)
	}
	fmt.Printf("## Backend commands\n\n")
	fmt.Printf(`Here are the commands specific to the %s backend.

Run them with

    rclone backend COMMAND remote:

The help below will explain what arguments each command takes.

See the [backend](/commands/rclone_backend/) command for more
info on how to pass options and arguments.

These can be run on a running backend using the rc command
[backend/command](/rc/#backend-command).

`, name)
	for _, cmd := range cmds {
		fmt.Printf("### %s\n\n", cmd.Name)
		fmt.Printf("%s\n\n", cmd.Short)
		fmt.Printf("    rclone backend %s remote: [options] [<arguments>+]\n\n", cmd.Name)
		if cmd.Long != "" {
			fmt.Printf("%s\n\n", cmd.Long)
		}
		if len(cmd.Opts) != 0 {
			fmt.Printf("Options:\n\n")

			ks := []string{}
			for k := range cmd.Opts {
				ks = append(ks, k)
			}
			sort.Strings(ks)
			for _, k := range ks {
				v := cmd.Opts[k]
				fmt.Printf("- %q: %s\n", k, v)
			}
			fmt.Printf("\n")
		}
	}
	return nil
}
