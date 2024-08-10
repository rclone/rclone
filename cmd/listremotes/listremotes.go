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
	listLong          bool
	jsonOutput        bool
	filterName        string
	filterType        string
	filterSource      string
	filterDescription string
	orderBy           string
)

func init() {
	cmd.Root.AddCommand(commandDefinition)
	cmdFlags := commandDefinition.Flags()
	flags.BoolVarP(cmdFlags, &listLong, "long", "", false, "Show type and description in addition to name", "")
	flags.StringVarP(cmdFlags, &filterName, "name", "", "", "Filter remotes by name", "")
	flags.StringVarP(cmdFlags, &filterType, "type", "", "", "Filter remotes by type", "")
	flags.StringVarP(cmdFlags, &filterSource, "source", "", "", "Filter remotes by source, e.g. 'file' or 'environment'", "")
	flags.StringVarP(cmdFlags, &filterDescription, "description", "", "", "Filter remotes by description", "")
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
		case "description":
			field = func(o config.Remote) string {
				return o.Description
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
Lists all the available remotes from the config file, or the remotes matching
an optional filter.

Prints the result in human-readable format by default, and as a simple list of
remote names, or if used with flag ` + "`--long`" + ` a tabular format including
the remote names, types and descriptions. Using flag ` + "`--json`" + ` produces
machine-readable output instead, which always includes all attributes - including
the source (file or environment).

Result can be filtered by a filter argument which applies to all attributes,
and/or filter flags specific for each attribute. The values must be specified
according to regular rclone filtering pattern syntax.
`,
	Annotations: map[string]string{
		"versionIntroduced": "v1.34",
	},
	RunE: func(command *cobra.Command, args []string) error {
		cmd.CheckArgs(0, 1, command, args)
		var filterDefault string
		if len(args) > 0 {
			filterDefault = args[0]
		}
		filters := make(map[string]*regexp.Regexp)
		for k, v := range map[string]string{
			"all":         filterDefault,
			"name":        filterName,
			"type":        filterType,
			"source":      filterSource,
			"description": filterDescription,
		} {
			if v != "" {
				filterRe, err := filter.GlobStringToRegexp(v, false, true)
				if err != nil {
					return fmt.Errorf("invalid %s filter argument: %w", k, err)
				}
				fs.Debugf(nil, "Filter for %s: %s", k, filterRe.String())
				filters[k] = filterRe
			}
		}
		remotes := config.GetRemotes()
		maxName := 0
		maxType := 0
		i := 0
		for _, remote := range remotes {
			include := true
			for k, v := range filters {
				if k == "all" && !(v.MatchString(remote.Name) || v.MatchString(remote.Type) || v.MatchString(remote.Source) || v.MatchString(remote.Description)) {
					include = false
				} else if k == "name" && !v.MatchString(remote.Name) {
					include = false
				} else if k == "type" && !v.MatchString(remote.Type) {
					include = false
				} else if k == "source" && !v.MatchString(remote.Source) {
					include = false
				} else if k == "description" && !v.MatchString(remote.Description) {
					include = false
				}
			}
			if include {
				if len(remote.Name) > maxName {
					maxName = len(remote.Name)
				}
				if len(remote.Type) > maxType {
					maxType = len(remote.Type)
				}
				remotes[i] = remote
				i++
			}
		}
		remotes = remotes[:i]

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
				fmt.Printf("%-*s %-*s %s\n", maxName+1, remote.Name+":", maxType, remote.Type, remote.Description)
			}
		} else {
			for _, remote := range remotes {
				fmt.Printf("%s:\n", remote.Name)
			}
		}
		return nil
	},
}
