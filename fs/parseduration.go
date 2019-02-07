package fs

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"
)

// Duration is a time.Duration with some more parsing options
type Duration time.Duration

// DurationOff is the default value for flags which can be turned off
const DurationOff = Duration((1 << 63) - 1)

// Turn Duration into a string
func (d Duration) String() string {
	if d == DurationOff {
		return "off"
	}
	for i := len(ageSuffixes) - 2; i >= 0; i-- {
		ageSuffix := &ageSuffixes[i]
		if math.Abs(float64(d)) >= float64(ageSuffix.Multiplier) {
			timeUnits := float64(d) / float64(ageSuffix.Multiplier)
			return strconv.FormatFloat(timeUnits, 'f', -1, 64) + ageSuffix.Suffix
		}
	}
	return time.Duration(d).String()
}

// IsSet returns if the duration is != DurationOff
func (d Duration) IsSet() bool {
	return d != DurationOff
}

// We use time conventions
var ageSuffixes = []struct {
	Suffix     string
	Multiplier time.Duration
}{
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

	if age == "off" {
		return time.Duration(DurationOff), nil
	}

	// Attempt to parse as a time.Duration first
	d, err := time.ParseDuration(age)
	if err == nil {
		return d, nil
	}

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
	return "Duration"
}

// Scan implements the fmt.Scanner interface
func (d *Duration) Scan(s fmt.ScanState, ch rune) error {
	token, err := s.Token(true, nil)
	if err != nil {
		return err
	}
	return d.Set(string(token))
}
