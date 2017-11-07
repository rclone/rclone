// This contains helper functions for managing flags

package fs

import (
	"fmt"
	"log"
	"math"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/pkg/errors"
	"github.com/spf13/pflag"
)

// SizeSuffix is parsed by flag with k/M/G suffixes
type SizeSuffix int64

// Turn SizeSuffix into a string and a suffix
func (x SizeSuffix) string() (string, string) {
	scaled := float64(0)
	suffix := ""
	switch {
	case x < 0:
		return "off", ""
	case x == 0:
		return "0", ""
	case x < 1024:
		scaled = float64(x)
		suffix = ""
	case x < 1024*1024:
		scaled = float64(x) / 1024
		suffix = "k"
	case x < 1024*1024*1024:
		scaled = float64(x) / 1024 / 1024
		suffix = "M"
	default:
		scaled = float64(x) / 1024 / 1024 / 1024
		suffix = "G"
	}
	if math.Floor(scaled) == scaled {
		return fmt.Sprintf("%.0f", scaled), suffix
	}
	return fmt.Sprintf("%.3f", scaled), suffix
}

// String turns SizeSuffix into a string
func (x SizeSuffix) String() string {
	val, suffix := x.string()
	return val + suffix
}

// Unit turns SizeSuffix into a string with a unit
func (x SizeSuffix) Unit(unit string) string {
	val, suffix := x.string()
	if val == "off" {
		return val
	}
	return val + " " + suffix + unit
}

// Set a SizeSuffix
func (x *SizeSuffix) Set(s string) error {
	if len(s) == 0 {
		return errors.New("empty string")
	}
	if strings.ToLower(s) == "off" {
		*x = -1
		return nil
	}
	suffix := s[len(s)-1]
	suffixLen := 1
	var multiplier float64
	switch suffix {
	case '0', '1', '2', '3', '4', '5', '6', '7', '8', '9', '.':
		suffixLen = 0
		multiplier = 1 << 10
	case 'b', 'B':
		multiplier = 1
	case 'k', 'K':
		multiplier = 1 << 10
	case 'm', 'M':
		multiplier = 1 << 20
	case 'g', 'G':
		multiplier = 1 << 30
	default:
		return errors.Errorf("bad suffix %q", suffix)
	}
	s = s[:len(s)-suffixLen]
	value, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return err
	}
	if value < 0 {
		return errors.Errorf("size can't be negative %q", s)
	}
	value *= multiplier
	*x = SizeSuffix(value)
	return nil
}

// Type of the value
func (x *SizeSuffix) Type() string {
	return "int64"
}

// Check it satisfies the interface
var _ pflag.Value = (*SizeSuffix)(nil)

// BwTimeSlot represents a bandwidth configuration at a point in time.
type BwTimeSlot struct {
	hhmm      int
	bandwidth SizeSuffix
}

// BwTimetable contains all configured time slots.
type BwTimetable []BwTimeSlot

// String returns a printable representation of BwTimetable.
func (x BwTimetable) String() string {
	ret := []string{}
	for _, ts := range x {
		ret = append(ret, fmt.Sprintf("%04.4d,%s", ts.hhmm, ts.bandwidth.String()))
	}
	return strings.Join(ret, " ")
}

// Set the bandwidth timetable.
func (x *BwTimetable) Set(s string) error {
	// The timetable is formatted as:
	// "hh:mm,bandwidth hh:mm,banwidth..." ex: "10:00,10G 11:30,1G 18:00,off"
	// If only a single bandwidth identifier is provided, we assume constant bandwidth.

	if len(s) == 0 {
		return errors.New("empty string")
	}
	// Single value without time specification.
	if !strings.Contains(s, " ") && !strings.Contains(s, ",") {
		ts := BwTimeSlot{}
		if err := ts.bandwidth.Set(s); err != nil {
			return err
		}
		ts.hhmm = 0
		*x = BwTimetable{ts}
		return nil
	}

	for _, tok := range strings.Split(s, " ") {
		tv := strings.Split(tok, ",")

		// Format must be HH:MM,BW
		if len(tv) != 2 {
			return errors.Errorf("invalid time/bandwidth specification: %q", tok)
		}

		// Basic timespec sanity checking
		hhmm := tv[0]
		if len(hhmm) != 5 {
			return errors.Errorf("invalid time specification (hh:mm): %q", hhmm)
		}
		hh, err := strconv.Atoi(hhmm[0:2])
		if err != nil {
			return errors.Errorf("invalid hour in time specification %q: %v", hhmm, err)
		}
		if hh < 0 || hh > 23 {
			return errors.Errorf("invalid hour (must be between 00 and 23): %q", hh)
		}
		mm, err := strconv.Atoi(hhmm[3:])
		if err != nil {
			return errors.Errorf("invalid minute in time specification: %q: %v", hhmm, err)
		}
		if mm < 0 || mm > 59 {
			return errors.Errorf("invalid minute (must be between 00 and 59): %q", hh)
		}

		ts := BwTimeSlot{
			hhmm: (hh * 100) + mm,
		}
		// Bandwidth limit for this time slot.
		if err := ts.bandwidth.Set(tv[1]); err != nil {
			return err
		}
		*x = append(*x, ts)
	}
	return nil
}

// LimitAt returns a BwTimeSlot for the time requested.
func (x BwTimetable) LimitAt(tt time.Time) BwTimeSlot {
	// If the timetable is empty, we return an unlimited BwTimeSlot starting at midnight.
	if len(x) == 0 {
		return BwTimeSlot{hhmm: 0, bandwidth: -1}
	}

	hhmm := tt.Hour()*100 + tt.Minute()

	// By default, we return the last element in the timetable. This
	// satisfies two conditions: 1) If there's only one element it
	// will always be selected, and 2) The last element of the table
	// will "wrap around" until overriden by an earlier time slot.
	// there's only one time slot in the timetable.
	ret := x[len(x)-1]

	mindif := 0
	first := true

	// Look for most recent time slot.
	for _, ts := range x {
		// Ignore the past
		if hhmm < ts.hhmm {
			continue
		}
		dif := ((hhmm / 100 * 60) + (hhmm % 100)) - ((ts.hhmm / 100 * 60) + (ts.hhmm % 100))
		if first {
			mindif = dif
			first = false
		}
		if dif <= mindif {
			mindif = dif
			ret = ts
		}
	}

	return ret
}

// Type of the value
func (x BwTimetable) Type() string {
	return "BwTimetable"
}

// Check it satisfies the interface
var _ pflag.Value = (*BwTimetable)(nil)

// optionToEnv converts an option name, eg "ignore-size" into an
// environment name "RCLONE_IGNORE_SIZE"
func optionToEnv(name string) string {
	return "RCLONE_" + strings.ToUpper(strings.Replace(name, "-", "_", -1))
}

// setDefaultFromEnv constructs a name from the flag passed in and
// sets the default from the environment if possible.
func setDefaultFromEnv(name string) {
	key := optionToEnv(name)
	newValue, found := os.LookupEnv(key)
	if found {
		flag := pflag.Lookup(name)
		if flag == nil {
			log.Fatalf("Couldn't find flag %q", name)
		}
		err := flag.Value.Set(newValue)
		if err != nil {
			log.Fatalf("Invalid value for environment variable %q: %v", key, err)
		}
		Debugf(nil, "Set default for %q from %q to %q (%v)", name, key, newValue, flag.Value)
		flag.DefValue = newValue
	}
}

// StringP defines a flag which can be overridden by an environment variable
//
// It is a thin wrapper around pflag.StringP
func StringP(name, shorthand string, value string, usage string) (out *string) {
	out = pflag.StringP(name, shorthand, value, usage)
	setDefaultFromEnv(name)
	return out
}

// StringVarP defines a flag which can be overridden by an environment variable
//
// It is a thin wrapper around pflag.StringVarP
func StringVarP(flags *pflag.FlagSet, p *string, name, shorthand string, value string, usage string) {
	flags.StringVarP(p, name, shorthand, value, usage)
	setDefaultFromEnv(name)
}

// BoolP defines a flag which can be overridden by an environment variable
//
// It is a thin wrapper around pflag.BoolP
func BoolP(name, shorthand string, value bool, usage string) (out *bool) {
	out = pflag.BoolP(name, shorthand, value, usage)
	setDefaultFromEnv(name)
	return out
}

// BoolVarP defines a flag which can be overridden by an environment variable
//
// It is a thin wrapper around pflag.BoolVarP
func BoolVarP(flags *pflag.FlagSet, p *bool, name, shorthand string, value bool, usage string) {
	flags.BoolVarP(p, name, shorthand, value, usage)
	setDefaultFromEnv(name)
}

// IntP defines a flag which can be overridden by an environment variable
//
// It is a thin wrapper around pflag.IntP
func IntP(name, shorthand string, value int, usage string) (out *int) {
	out = pflag.IntP(name, shorthand, value, usage)
	setDefaultFromEnv(name)
	return out
}

// IntVarP defines a flag which can be overridden by an environment variable
//
// It is a thin wrapper around pflag.IntVarP
func IntVarP(flags *pflag.FlagSet, p *int, name, shorthand string, value int, usage string) {
	flags.IntVarP(p, name, shorthand, value, usage)
	setDefaultFromEnv(name)
}

// Uint32VarP defines a flag which can be overridden by an environment variable
//
// It is a thin wrapper around pflag.Uint32VarP
func Uint32VarP(flags *pflag.FlagSet, p *uint32, name, shorthand string, value uint32, usage string) {
	flags.Uint32VarP(p, name, shorthand, value, usage)
	setDefaultFromEnv(name)
}

// Float64P defines a flag which can be overridden by an environment variable
//
// It is a thin wrapper around pflag.Float64P
func Float64P(name, shorthand string, value float64, usage string) (out *float64) {
	out = pflag.Float64P(name, shorthand, value, usage)
	setDefaultFromEnv(name)
	return out
}

// Float64VarP defines a flag which can be overridden by an environment variable
//
// It is a thin wrapper around pflag.Float64VarP
func Float64VarP(flags *pflag.FlagSet, p *float64, name, shorthand string, value float64, usage string) {
	flags.Float64VarP(p, name, shorthand, value, usage)
	setDefaultFromEnv(name)
}

// DurationP defines a flag which can be overridden by an environment variable
//
// It is a thin wrapper around pflag.DurationP
func DurationP(name, shorthand string, value time.Duration, usage string) (out *time.Duration) {
	out = pflag.DurationP(name, shorthand, value, usage)
	setDefaultFromEnv(name)
	return out
}

// DurationVarP defines a flag which can be overridden by an environment variable
//
// It is a thin wrapper around pflag.DurationVarP
func DurationVarP(flags *pflag.FlagSet, p *time.Duration, name, shorthand string, value time.Duration, usage string) {
	flags.DurationVarP(p, name, shorthand, value, usage)
	setDefaultFromEnv(name)
}

// VarP defines a flag which can be overridden by an environment variable
//
// It is a thin wrapper around pflag.VarP
func VarP(value pflag.Value, name, shorthand, usage string) {
	pflag.VarP(value, name, shorthand, usage)
	setDefaultFromEnv(name)
}

// FlagsVarP defines a flag which can be overridden by an environment variable
//
// It is a thin wrapper around pflag.VarP
func FlagsVarP(flags *pflag.FlagSet, value pflag.Value, name, shorthand, usage string) {
	flags.VarP(value, name, shorthand, usage)
	setDefaultFromEnv(name)
}

// StringArrayP defines a flag which can be overridden by an environment variable
//
// It sets one value only - command line flags can be used to set more.
//
// It is a thin wrapper around pflag.StringArrayP
func StringArrayP(name, shorthand string, value []string, usage string) (out *[]string) {
	out = pflag.StringArrayP(name, shorthand, value, usage)
	setDefaultFromEnv(name)
	return out
}

// StringArrayVarP defines a flag which can be overridden by an environment variable
//
// It sets one value only - command line flags can be used to set more.
//
// It is a thin wrapper around pflag.StringArrayVarP
func StringArrayVarP(flags *pflag.FlagSet, p *[]string, name, shorthand string, value []string, usage string) {
	flags.StringArrayVarP(p, name, shorthand, value, usage)
	setDefaultFromEnv(name)
}

// CountP defines a flag which can be overridden by an environment variable
//
// It is a thin wrapper around pflag.CountP
func CountP(name, shorthand string, usage string) (out *int) {
	out = pflag.CountP(name, shorthand, usage)
	setDefaultFromEnv(name)
	return out
}

// CountVarP defines a flag which can be overridden by an environment variable
//
// It is a thin wrapper around pflag.CountVarP
func CountVarP(flags *pflag.FlagSet, p *int, name, shorthand string, usage string) {
	flags.CountVarP(p, name, shorthand, usage)
	setDefaultFromEnv(name)
}
