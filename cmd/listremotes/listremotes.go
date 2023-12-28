// Package ls provides the ls command.
package ls

import (
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"

	"github.com/rclone/rclone/cmd"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config"
	"github.com/rclone/rclone/fs/config/flags"
	"github.com/rclone/rclone/fs/filter"
	"github.com/spf13/cobra"
)

var (
	listLong       bool
	jsonOutput     bool
	specificType   string
	specificSource string
	orderBy        string
)

func init() {
	cmd.Root.AddCommand(commandDefinition)
	cmdFlags := commandDefinition.Flags()
	flags.BoolVarP(cmdFlags, &listLong, "long", "", false, "Show the type as well as names", "")
	flags.StringVarP(cmdFlags, &specificType, "type", "", "", "Only remotes of specific type", "")
	flags.StringVarP(cmdFlags, &specificSource, "source", "", "", "Only remotes from specific source", "")
	flags.StringVarP(cmdFlags, &orderBy, "order-by", "", "", "Instructions on how to order the result, e.g. 'type,name=descending'", "")
	flags.BoolVarP(cmdFlags, &jsonOutput, "json", "", false, "Format output as JSON", "")
}

// lessFn compares to remotes for order by
type lessFn func(a, b config.Remote) bool

// newLess returns a function for comparing remotes based on an order by string
func newLess(orderBy string) (less lessFn, err error) {
	if orderBy == "" {
		return nil, nil
	}
	parts := strings.Split(strings.ToLower(orderBy), ",")
	n := len(parts)
	for i := n - 1; i >= 0; i-- {
		fieldAndDirection := strings.SplitN(parts[i], "=", 2)

		descending := false
		if len(fieldAndDirection) > 1 {
			switch fieldAndDirection[1] {
			case "ascending", "asc":
			case "descending", "desc":
				descending = true
			default:
				return nil, fmt.Errorf("unknown --order-by direction %q", fieldAndDirection[1])
			}
		}

		var field func(o config.Remote) string
		switch fieldAndDirection[0] {
		case "name":
			field = func(o config.Remote) string {
				return o.Name
			}
		case "type":
			field = func(o config.Remote) string {
				return o.Type
			}
		case "source":
			field = func(o config.Remote) string {
				return o.Source
			}
		default:
			return nil, fmt.Errorf("unknown --order-by field %q", fieldAndDirection[0])
		}

		var thisLess lessFn
		if descending {
			thisLess = func(a, b config.Remote) bool {
				return field(a) > field(b)
			}
		} else {
			thisLess = func(a, b config.Remote) bool {
				return field(a) < field(b)
			}
		}

		if i == n-1 {
			less = thisLess
		} else {
			nextLess := less
			less = func(a, b config.Remote) bool {
				if field(a) == field(b) {
					return nextLess(a, b)
				}
				return thisLess(a, b)
			}
		}
	}
	return less, nil
}

var commandDefinition = &cobra.Command{
	Use:   "listremotes [<filter>]",
	Short: `List all the remotes in the config file and defined in environment variables.`,
	Long: `
rclone listremotes lists all the available remotes from the config file,
or the remotes matching an optional filter.

Prints the result in human-readable format by default, and as a simple list of
remote names, or if used with flag ` + "`--long`" + ` a tabular format including
the remote types. Using flag ` + "`--json`" + ` produces machine-readable output
instead, which always includes both names and types.
`,
	Annotations: map[string]string{
		"versionIntroduced": "v1.34",
	},
	RunE: func(command *cobra.Command, args []string) error {
		cmd.CheckArgs(0, 1, command, args)

		var filterRe *regexp.Regexp = nil
		if len(args) > 0 {
			var err error
			if filterRe, err = filter.GlobStringToRegexp(strings.TrimRight(args[0], ":"), false); err != nil {
				return fmt.Errorf("invalid filter argument: %w", err)
			}
			fs.Debugf(nil, "Filter: %s", filterRe.String())
		}

		remotes := config.GetRemotes()
		maxName := 0
		maxType := 0
		if filterRe != nil || specificType != "" || specificSource != "" {
			i := 0
			for _, remote := range remotes {
				if filterRe != nil && !filterRe.MatchString(remote.Name) {
					continue
				}
				if specificType != "" && specificType != remote.Type {
					continue
				}
				if specificSource != "" && specificSource != remote.Source {
					continue
				}
				if len(remote.Name) > maxName {
					maxName = len(remote.Name)
				}
				if len(remote.Type) > maxType {
					maxType = len(remote.Type)
				}
				remotes[i] = remote
				i++
			}
			remotes = remotes[:i]
		} else {
			for _, remote := range remotes {
				if len(remote.Name) > maxName {
					maxName = len(remote.Name)
				}
				if len(remote.Type) > maxType {
					maxType = len(remote.Type)
				}
			}
		}

		less, err := newLess(orderBy)
		if err != nil {
			return err
		}
		if less != nil {
			sliceLessFn := func(i, j int) bool {
				return less(remotes[i], remotes[j])
			}
			sort.SliceStable(remotes, sliceLessFn)
		}

		if jsonOutput {
			fmt.Println("[")
			first := true
			for _, remote := range remotes {
				out, err := json.Marshal(remote)
				if err != nil {
					return fmt.Errorf("failed to marshal remote object: %w", err)
				}
				if first {
					first = false
				} else {
					fmt.Print(",\n")
				}
				_, err = os.Stdout.Write(out)
				if err != nil {
					return fmt.Errorf("failed to write to output: %w", err)
				}
			}
			if !first {
				fmt.Println()
			}
			fmt.Println("]")
		} else if listLong {
			for _, remote := range remotes {
				fmt.Printf("%-*s %-*s %s\n", maxName+1, remote.Name+":", maxType, remote.Type, remote.Source)
			}
		} else {
			for _, remote := range remotes {
				fmt.Printf("%s:\n", remote.Name)
			}
		}
		return nil
	},
}
