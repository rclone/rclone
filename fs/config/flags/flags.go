// Package flags contains enhanced versions of spf13/pflag flag
// routines which will read from the environment also.
package flags

import (
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/rclone/rclone/fs"
	"github.com/spf13/pflag"
)

// Groups of Flags
type Groups struct {
	// Groups of flags
	Groups []*Group

	// GroupsMaps maps a group name to a Group
	ByName map[string]*Group
}

// Group related flags together for documentation purposes
type Group struct {
	Groups *Groups
	Name   string
	Help   string
	Flags  *pflag.FlagSet
}

// NewGroups constructs a collection of Groups
func NewGroups() *Groups {
	return &Groups{
		ByName: make(map[string]*Group),
	}
}

// NewGroup to create Group
func (gs *Groups) NewGroup(name, help string) *Group {
	group := &Group{
		Name:  name,
		Help:  help,
		Flags: pflag.NewFlagSet(name, pflag.ExitOnError),
	}
	gs.ByName[group.Name] = group
	gs.Groups = append(gs.Groups, group)
	return group
}

// Filter makes a copy of groups filtered by flagsRe
func (gs *Groups) Filter(group string, filterRe *regexp.Regexp, filterNamesOnly bool) *Groups {
	newGs := NewGroups()
	for _, g := range gs.Groups {
		if group == "" || strings.EqualFold(g.Name, group) {
			newG := newGs.NewGroup(g.Name, g.Help)
			g.Flags.VisitAll(func(f *pflag.Flag) {
				if filterRe == nil || filterRe.MatchString(f.Name) || (!filterNamesOnly && filterRe.MatchString(f.Usage)) {
					newG.Flags.AddFlag(f)
				}
			})
		}
	}
	return newGs
}

// Include makes a copy of groups only including the ones in the filter string
func (gs *Groups) Include(groupsString string) *Groups {
	if groupsString == "" {
		return gs
	}
	want := map[string]bool{}
	for _, groupName := range strings.Split(groupsString, ",") {
		_, ok := All.ByName[groupName]
		if !ok {
			fs.Fatalf(nil, "Couldn't find group %q in command annotation", groupName)
		}
		want[groupName] = true
	}
	newGs := NewGroups()
	for _, g := range gs.Groups {
		if !want[g.Name] {
			continue
		}
		newG := newGs.NewGroup(g.Name, g.Help)
		newG.Flags = g.Flags
	}
	return newGs
}

// Add a new flag to the Group
func (g *Group) Add(flag *pflag.Flag) {
	g.Flags.AddFlag(flag)
}

// AllRegistered returns all flags in a group
func (gs *Groups) AllRegistered() map[*pflag.Flag]struct{} {
	out := make(map[*pflag.Flag]struct{})
	for _, g := range gs.Groups {
		g.Flags.VisitAll(func(f *pflag.Flag) {
			out[f] = struct{}{}
		})
	}
	return out
}

// All is the global stats Groups
var All *Groups

// Groups of flags for documentation purposes
func init() {
	All = NewGroups()
	All.NewGroup("Copy", "Flags for anything which can copy a file")
	All.NewGroup("Sync", "Flags used for sync commands")
	All.NewGroup("Important", "Important flags useful for most commands")
	All.NewGroup("Check", "Flags used for check commands")
	All.NewGroup("Networking", "Flags for general networking and HTTP stuff")
	All.NewGroup("Performance", "Flags helpful for increasing performance")
	All.NewGroup("Config", "Flags for general configuration of rclone")
	All.NewGroup("Debugging", "Flags for developers")
	All.NewGroup("Filter", "Flags for filtering directory listings")
	All.NewGroup("Listing", "Flags for listing directories")
	All.NewGroup("Logging", "Flags for logging and statistics")
	All.NewGroup("Metadata", "Flags to control metadata")
	All.NewGroup("RC", "Flags to control the Remote Control API")
	All.NewGroup("Metrics", "Flags to control the Metrics HTTP endpoint.")
}

// installFlag constructs a name from the flag passed in and
// sets the value and default from the environment if possible
// the value may be overridden when the command line is parsed
//
// Used to create non-backend flags like --stats.
//
// It also adds the flag to the groups passed in.
func installFlag(flags *pflag.FlagSet, name string, groupsString string) {
	// Find flag
	flag := flags.Lookup(name)
	if flag == nil {
		fs.Fatalf(nil, "Couldn't find flag --%q", name)
	}

	// Read default from environment if possible
	envKey := fs.OptionToEnv(name)
	if envValue, envFound := os.LookupEnv(envKey); envFound {
		isStringArray := false
		opt, isOption := flag.Value.(*fs.Option)
		if isOption {
			_, isStringArray = opt.Default.([]string)
		}
		if isStringArray {
			// Treat stringArray differently, treating the environment variable as a CSV array
			var list fs.CommaSepList
			err := list.Set(envValue)
			if err != nil {
				fs.Fatalf(nil, "Invalid value when setting stringArray --%s from environment variable %s=%q: %v", name, envKey, envValue, err)
			}
			// Set both the Value (so items on the command line get added) and DefValue so the help is correct
			opt.Value = ([]string)(list)
			flag.DefValue = list.String()
			for _, v := range list {
				fs.Debugf(nil, "Setting --%s %q from environment variable %s=%q", name, v, envKey, envValue)
			}
		} else {
			err := flags.Set(name, envValue)
			if err != nil {
				fs.Fatalf(nil, "Invalid value when setting --%s from environment variable %s=%q: %v", name, envKey, envValue, err)
			}
			fs.Debugf(nil, "Setting --%s %q from environment variable %s=%q", name, flag.Value, envKey, envValue)
			flag.DefValue = envValue
		}
	}

	// Add flag to Group if it is a global flag
	if groupsString != "" && flags == pflag.CommandLine {
		for _, groupName := range strings.Split(groupsString, ",") {
			if groupName == "rc-" {
				groupName = "RC"
			}
			group, ok := All.ByName[groupName]
			if !ok {
				fs.Fatalf(nil, "Couldn't find group %q for flag --%s", groupName, name)
			}
			group.Add(flag)
		}
	}
}

// StringP defines a flag which can be set by an environment variable
//
// It is a thin wrapper around pflag.StringP
func StringP(name, shorthand string, value string, usage string, groups string) (out *string) {
	out = pflag.StringP(name, shorthand, value, usage)
	installFlag(pflag.CommandLine, name, groups)
	return out
}

// StringVarP defines a flag which can be set by an environment variable
//
// It is a thin wrapper around pflag.StringVarP
func StringVarP(flags *pflag.FlagSet, p *string, name, shorthand string, value string, usage string, groups string) {
	flags.StringVarP(p, name, shorthand, value, usage)
	installFlag(flags, name, groups)
}

// BoolP defines a flag which can be set by an environment variable
//
// It is a thin wrapper around pflag.BoolP
func BoolP(name, shorthand string, value bool, usage string, groups string) (out *bool) {
	out = pflag.BoolP(name, shorthand, value, usage)
	installFlag(pflag.CommandLine, name, groups)
	return out
}

// BoolVarP defines a flag which can be set by an environment variable
//
// It is a thin wrapper around pflag.BoolVarP
func BoolVarP(flags *pflag.FlagSet, p *bool, name, shorthand string, value bool, usage string, groups string) {
	flags.BoolVarP(p, name, shorthand, value, usage)
	installFlag(flags, name, groups)
}

// IntP defines a flag which can be set by an environment variable
//
// It is a thin wrapper around pflag.IntP
func IntP(name, shorthand string, value int, usage string, groups string) (out *int) {
	out = pflag.IntP(name, shorthand, value, usage)
	installFlag(pflag.CommandLine, name, groups)
	return out
}

// Int64P defines a flag which can be set by an environment variable
//
// It is a thin wrapper around pflag.IntP
func Int64P(name, shorthand string, value int64, usage string, groups string) (out *int64) {
	out = pflag.Int64P(name, shorthand, value, usage)
	installFlag(pflag.CommandLine, name, groups)
	return out
}

// Int64VarP defines a flag which can be set by an environment variable
//
// It is a thin wrapper around pflag.Int64VarP
func Int64VarP(flags *pflag.FlagSet, p *int64, name, shorthand string, value int64, usage string, groups string) {
	flags.Int64VarP(p, name, shorthand, value, usage)
	installFlag(flags, name, groups)
}

// IntVarP defines a flag which can be set by an environment variable
//
// It is a thin wrapper around pflag.IntVarP
func IntVarP(flags *pflag.FlagSet, p *int, name, shorthand string, value int, usage string, groups string) {
	flags.IntVarP(p, name, shorthand, value, usage)
	installFlag(flags, name, groups)
}

// Uint32VarP defines a flag which can be set by an environment variable
//
// It is a thin wrapper around pflag.Uint32VarP
func Uint32VarP(flags *pflag.FlagSet, p *uint32, name, shorthand string, value uint32, usage string, groups string) {
	flags.Uint32VarP(p, name, shorthand, value, usage)
	installFlag(flags, name, groups)
}

// Float64P defines a flag which can be set by an environment variable
//
// It is a thin wrapper around pflag.Float64P
func Float64P(name, shorthand string, value float64, usage string, groups string) (out *float64) {
	out = pflag.Float64P(name, shorthand, value, usage)
	installFlag(pflag.CommandLine, name, groups)
	return out
}

// Float64VarP defines a flag which can be set by an environment variable
//
// It is a thin wrapper around pflag.Float64VarP
func Float64VarP(flags *pflag.FlagSet, p *float64, name, shorthand string, value float64, usage string, groups string) {
	flags.Float64VarP(p, name, shorthand, value, usage)
	installFlag(flags, name, groups)
}

// DurationP defines a flag which can be set by an environment variable
//
// It wraps the duration in an fs.Duration for extra suffixes and
// passes it to pflag.VarP
func DurationP(name, shorthand string, value time.Duration, usage string, groups string) (out *time.Duration) {
	out = new(time.Duration)
	*out = value
	pflag.VarP((*fs.Duration)(out), name, shorthand, usage)
	installFlag(pflag.CommandLine, name, groups)
	return out
}

// DurationVarP defines a flag which can be set by an environment variable
//
// It wraps the duration in an fs.Duration for extra suffixes and
// passes it to pflag.VarP
func DurationVarP(flags *pflag.FlagSet, p *time.Duration, name, shorthand string, value time.Duration, usage string, groups string) {
	flags.VarP((*fs.Duration)(p), name, shorthand, usage)
	installFlag(flags, name, groups)
}

// VarP defines a flag which can be set by an environment variable
//
// It is a thin wrapper around pflag.VarP
func VarP(value pflag.Value, name, shorthand, usage string, groups string) {
	pflag.VarP(value, name, shorthand, usage)
	installFlag(pflag.CommandLine, name, groups)
}

// FVarP defines a flag which can be set by an environment variable
//
// It is a thin wrapper around pflag.VarP
func FVarP(flags *pflag.FlagSet, value pflag.Value, name, shorthand, usage string, groups string) {
	flags.VarP(value, name, shorthand, usage)
	installFlag(flags, name, groups)
}

// StringArrayP defines a flag which can be set by an environment variable
//
// It sets one value only - command line flags can be used to set more.
//
// It is a thin wrapper around pflag.StringArrayP
func StringArrayP(name, shorthand string, value []string, usage string, groups string) (out *[]string) {
	out = pflag.StringArrayP(name, shorthand, value, usage)
	installFlag(pflag.CommandLine, name, groups)
	return out
}

// StringArrayVarP defines a flag which can be set by an environment variable
//
// It sets one value only - command line flags can be used to set more.
//
// It is a thin wrapper around pflag.StringArrayVarP
func StringArrayVarP(flags *pflag.FlagSet, p *[]string, name, shorthand string, value []string, usage string, groups string) {
	flags.StringArrayVarP(p, name, shorthand, value, usage)
	installFlag(flags, name, groups)
}

// CountP defines a flag which can be set by an environment variable
//
// It is a thin wrapper around pflag.CountP
func CountP(name, shorthand string, usage string, groups string) (out *int) {
	out = pflag.CountP(name, shorthand, usage)
	installFlag(pflag.CommandLine, name, groups)
	return out
}

// CountVarP defines a flag which can be set by an environment variable
//
// It is a thin wrapper around pflag.CountVarP
func CountVarP(flags *pflag.FlagSet, p *int, name, shorthand string, usage string, groups string) {
	flags.CountVarP(p, name, shorthand, usage)
	installFlag(flags, name, groups)
}

// AddFlagsFromOptions takes a slice of fs.Option and adds flags for all of them
func AddFlagsFromOptions(flags *pflag.FlagSet, prefix string, options fs.Options) {
	done := map[string]struct{}{}
	for i := range options {
		opt := &options[i]
		// Skip if done already (e.g. with Provider options)
		if _, doneAlready := done[opt.Name]; doneAlready {
			continue
		}
		done[opt.Name] = struct{}{}
		// Make a flag from each option
		if prefix == "" {
			opt.NoPrefix = true
		}
		name := opt.FlagName(prefix)
		found := flags.Lookup(name) != nil
		if !found {
			// Take first line of help only
			help := strings.TrimSpace(opt.Help)
			if nl := strings.IndexRune(help, '\n'); nl >= 0 {
				help = help[:nl]
			}
			help = strings.TrimRight(strings.TrimSpace(help), ".!?")
			if opt.IsPassword {
				help += " (obscured)"
			}
			flag := flags.VarPF(opt, name, opt.ShortOpt, help)
			installFlag(flags, name, opt.Groups)
			if _, isBool := opt.Default.(bool); isBool {
				flag.NoOptDefVal = "true"
			}
			// Hide on the command line if requested
			if opt.Hide&fs.OptionHideCommandLine != 0 {
				flag.Hidden = true
			}
		} else {
			fs.Errorf(nil, "Not adding duplicate flag --%s", name)
		}
		// flag.Hidden = true
	}
}
