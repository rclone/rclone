package fs

import (
	"strconv"
	"strings"
	"time"
)

// Duration is a time.Duration with some more parsing options
type Duration time.Duration

// Turn Duration into a string
func (d Duration) String() string {
	return time.Duration(d).String()
}

// We use time conventions
var ageSuffixes = []struct {
	Suffix     string
	Multiplier time.Duration
}{
	{Suffix: "ms", Multiplier: time.Millisecond},
	{Suffix: "s", Multiplier: time.Second},
	{Suffix: "m", Multiplier: time.Minute},
	{Suffix: "h", Multiplier: time.Hour},
	{Suffix: "d", Multiplier: time.Hour * 24},
	{Suffix: "w", Multiplier: time.Hour * 24 * 7},
	{Suffix: "M", Multiplier: time.Hour * 24 * 30},
	{Suffix: "y", Multiplier: time.Hour * 24 * 365},

	// Default to second
	{Suffix: "", Multiplier: time.Second},
}

// ParseDuration parses a duration string. Accept ms|s|m|h|d|w|M|y suffixes. Defaults to second if not provided
func ParseDuration(age string) (time.Duration, error) {
	var period float64

	for _, ageSuffix := range ageSuffixes {
		if strings.HasSuffix(age, ageSuffix.Suffix) {
			numberString := age[:len(age)-len(ageSuffix.Suffix)]
			var err error
			period, err = strconv.ParseFloat(numberString, 64)
			if err != nil {
				return time.Duration(0), err
			}
			period *= float64(ageSuffix.Multiplier)
			break
		}
	}

	return time.Duration(period), nil
}

// Set a Duration
func (d *Duration) Set(s string) error {
	duration, err := ParseDuration(s)
	if err != nil {
		return err
	}
	*d = Duration(duration)
	return nil
}

// Type of the value
func (d Duration) Type() string {
	return "time.Duration"
}
