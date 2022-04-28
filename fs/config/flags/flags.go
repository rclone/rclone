// Package flags contains enhanced versions of spf13/pflag flag
// routines which will read from the environment also.
package flags

import (
	"log"
	"os"
	"time"

	"github.com/rclone/rclone/fs"
	"github.com/spf13/pflag"
)

// setValueFromEnv constructs a name from the flag passed in and
// sets the value and default from the environment if possible
// the value may be overridden when the command line is parsed
//
// Used to create non-backend flags like --stats
func setValueFromEnv(flags *pflag.FlagSet, name string) {
	envKey := fs.OptionToEnv(name)
	envValue, found := os.LookupEnv(envKey)
	if found {
		flag := flags.Lookup(name)
		if flag == nil {
			log.Fatalf("Couldn't find flag --%q", name)
		}
		err := flags.Set(name, envValue)
		if err != nil {
			log.Fatalf("Invalid value when setting --%s from environment variable %s=%q: %v", name, envKey, envValue, err)
		}
		fs.Debugf(nil, "Setting --%s %q from environment variable %s=%q", name, flag.Value, envKey, envValue)
		flag.DefValue = envValue
	}
}

// SetDefaultFromEnv constructs a name from the flag passed in and
// sets the default from the environment if possible
//
// Used to create backend flags like --skip-links
func SetDefaultFromEnv(flags *pflag.FlagSet, name string) {
	envKey := fs.OptionToEnv(name)
	envValue, found := os.LookupEnv(envKey)
	if found {
		flag := flags.Lookup(name)
		if flag == nil {
			log.Fatalf("Couldn't find flag --%q", name)
		}
		fs.Debugf(nil, "Setting default for %s=%q from environment variable %s", name, envValue, envKey)
		//err = tempValue.Set()
		flag.DefValue = envValue
	}
}

// StringP defines a flag which can be set by an environment variable
//
// It is a thin wrapper around pflag.StringP
func StringP(name, shorthand string, value string, usage string) (out *string) {
	out = pflag.StringP(name, shorthand, value, usage)
	setValueFromEnv(pflag.CommandLine, name)
	return out
}

// StringVarP defines a flag which can be set by an environment variable
//
// It is a thin wrapper around pflag.StringVarP
func StringVarP(flags *pflag.FlagSet, p *string, name, shorthand string, value string, usage string) {
	flags.StringVarP(p, name, shorthand, value, usage)
	setValueFromEnv(flags, name)
}

// BoolP defines a flag which can be set by an environment variable
//
// It is a thin wrapper around pflag.BoolP
func BoolP(name, shorthand string, value bool, usage string) (out *bool) {
	out = pflag.BoolP(name, shorthand, value, usage)
	setValueFromEnv(pflag.CommandLine, name)
	return out
}

// BoolVarP defines a flag which can be set by an environment variable
//
// It is a thin wrapper around pflag.BoolVarP
func BoolVarP(flags *pflag.FlagSet, p *bool, name, shorthand string, value bool, usage string) {
	flags.BoolVarP(p, name, shorthand, value, usage)
	setValueFromEnv(flags, name)
}

// IntP defines a flag which can be set by an environment variable
//
// It is a thin wrapper around pflag.IntP
func IntP(name, shorthand string, value int, usage string) (out *int) {
	out = pflag.IntP(name, shorthand, value, usage)
	setValueFromEnv(pflag.CommandLine, name)
	return out
}

// Int64P defines a flag which can be set by an environment variable
//
// It is a thin wrapper around pflag.IntP
func Int64P(name, shorthand string, value int64, usage string) (out *int64) {
	out = pflag.Int64P(name, shorthand, value, usage)
	setValueFromEnv(pflag.CommandLine, name)
	return out
}

// Int64VarP defines a flag which can be set by an environment variable
//
// It is a thin wrapper around pflag.Int64VarP
func Int64VarP(flags *pflag.FlagSet, p *int64, name, shorthand string, value int64, usage string) {
	flags.Int64VarP(p, name, shorthand, value, usage)
	setValueFromEnv(flags, name)
}

// IntVarP defines a flag which can be set by an environment variable
//
// It is a thin wrapper around pflag.IntVarP
func IntVarP(flags *pflag.FlagSet, p *int, name, shorthand string, value int, usage string) {
	flags.IntVarP(p, name, shorthand, value, usage)
	setValueFromEnv(flags, name)
}

// Uint32VarP defines a flag which can be set by an environment variable
//
// It is a thin wrapper around pflag.Uint32VarP
func Uint32VarP(flags *pflag.FlagSet, p *uint32, name, shorthand string, value uint32, usage string) {
	flags.Uint32VarP(p, name, shorthand, value, usage)
	setValueFromEnv(flags, name)
}

// Float64P defines a flag which can be set by an environment variable
//
// It is a thin wrapper around pflag.Float64P
func Float64P(name, shorthand string, value float64, usage string) (out *float64) {
	out = pflag.Float64P(name, shorthand, value, usage)
	setValueFromEnv(pflag.CommandLine, name)
	return out
}

// Float64VarP defines a flag which can be set by an environment variable
//
// It is a thin wrapper around pflag.Float64VarP
func Float64VarP(flags *pflag.FlagSet, p *float64, name, shorthand string, value float64, usage string) {
	flags.Float64VarP(p, name, shorthand, value, usage)
	setValueFromEnv(flags, name)
}

// DurationP defines a flag which can be set by an environment variable
//
// It is a thin wrapper around pflag.DurationP
func DurationP(name, shorthand string, value time.Duration, usage string) (out *time.Duration) {
	out = pflag.DurationP(name, shorthand, value, usage)
	setValueFromEnv(pflag.CommandLine, name)
	return out
}

// DurationVarP defines a flag which can be set by an environment variable
//
// It is a thin wrapper around pflag.DurationVarP
func DurationVarP(flags *pflag.FlagSet, p *time.Duration, name, shorthand string, value time.Duration, usage string) {
	flags.DurationVarP(p, name, shorthand, value, usage)
	setValueFromEnv(flags, name)
}

// VarP defines a flag which can be set by an environment variable
//
// It is a thin wrapper around pflag.VarP
func VarP(value pflag.Value, name, shorthand, usage string) {
	pflag.VarP(value, name, shorthand, usage)
	setValueFromEnv(pflag.CommandLine, name)
}

// FVarP defines a flag which can be set by an environment variable
//
// It is a thin wrapper around pflag.VarP
func FVarP(flags *pflag.FlagSet, value pflag.Value, name, shorthand, usage string) {
	flags.VarP(value, name, shorthand, usage)
	setValueFromEnv(flags, name)
}

// StringArrayP defines a flag which can be set by an environment variable
//
// It sets one value only - command line flags can be used to set more.
//
// It is a thin wrapper around pflag.StringArrayP
func StringArrayP(name, shorthand string, value []string, usage string) (out *[]string) {
	out = pflag.StringArrayP(name, shorthand, value, usage)
	setValueFromEnv(pflag.CommandLine, name)
	return out
}

// StringArrayVarP defines a flag which can be set by an environment variable
//
// It sets one value only - command line flags can be used to set more.
//
// It is a thin wrapper around pflag.StringArrayVarP
func StringArrayVarP(flags *pflag.FlagSet, p *[]string, name, shorthand string, value []string, usage string) {
	flags.StringArrayVarP(p, name, shorthand, value, usage)
	setValueFromEnv(flags, name)
}

// CountP defines a flag which can be set by an environment variable
//
// It is a thin wrapper around pflag.CountP
func CountP(name, shorthand string, usage string) (out *int) {
	out = pflag.CountP(name, shorthand, usage)
	setValueFromEnv(pflag.CommandLine, name)
	return out
}

// CountVarP defines a flag which can be set by an environment variable
//
// It is a thin wrapper around pflag.CountVarP
func CountVarP(flags *pflag.FlagSet, p *int, name, shorthand string, usage string) {
	flags.CountVarP(p, name, shorthand, usage)
	setValueFromEnv(flags, name)
}
