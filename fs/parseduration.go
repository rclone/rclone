package fs

import (
	"encoding/json"
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

// parse the age as suffixed ages
func parseDurationSuffixes(age string) (time.Duration, error) {
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

// time formats to try parsing ages as - in order
var timeFormats = []string{
	time.RFC3339,
	"2006-01-02T15:04:05",
	"2006-01-02 15:04:05",
	"2006-01-02",
}

// parse the date as time in various date formats
func parseTimeDates(date string) (t time.Time, err error) {
	var instant time.Time
	for _, timeFormat := range timeFormats {
		instant, err = time.ParseInLocation(timeFormat, date, time.Local)
		if err == nil {
			return instant, nil
		}
	}
	return t, err
}

// parse the age as time before the epoch in various date formats
func parseDurationDates(age string, epoch time.Time) (d time.Duration, err error) {
	instant, err := parseTimeDates(age)
	if err != nil {
		return d, err
	}

	return epoch.Sub(instant), nil
}

// parseDurationFromNow parses a duration string. Allows ParseDuration to match the time
// package and easier testing within the fs package.
func parseDurationFromNow(age string, getNow func() time.Time) (d time.Duration, err error) {
	if age == "off" {
		return time.Duration(DurationOff), nil
	}

	// Attempt to parse as a time.Duration first
	d, err = time.ParseDuration(age)
	if err == nil {
		return d, nil
	}

	d, err = parseDurationSuffixes(age)
	if err == nil {
		return d, nil
	}

	d, err = parseDurationDates(age, getNow())
	if err == nil {
		return d, nil
	}

	return d, err
}

// ParseDuration parses a duration string. Accept ms|s|m|h|d|w|M|y suffixes. Defaults to second if not provided
func ParseDuration(age string) (time.Duration, error) {
	return parseDurationFromNow(age, timeNowFunc)
}

// ReadableString parses d into a human-readable duration with units.
// Examples: "3s", "1d2h23m20s", "292y24w3d23h47m16s".
func (d Duration) ReadableString() string {
	return d.readableString(0)
}

// ShortReadableString parses d into a human-readable duration with units.
// This method returns it in short format, including the 3 most significant
// units only, sacrificing precision if necessary. E.g. returns "292y24w3d"
// instead of "292y24w3d23h47m16s", and "3d23h47m" instead of "3d23h47m16s".
func (d Duration) ShortReadableString() string {
	return d.readableString(3)
}

// readableString parses d into a human-readable duration with units.
// Parameter maxNumberOfUnits limits number of significant units to include,
// sacrificing precision. E.g. with argument 3 it returns "292y24w3d" instead
// of "292y24w3d23h47m16s", and "3d23h47m" instead of "3d23h47m16s". Zero or
// negative argument means include all.
// Based on https://github.com/hako/durafmt
func (d Duration) readableString(maxNumberOfUnits int) string {
	switch d {
	case DurationOff:
		return "off"
	case 0:
		return "0s"
	}

	var readableString strings.Builder

	// Check for minus durations.
	if d < 0 {
		readableString.WriteString("-")
	}

	duration := time.Duration(math.Abs(float64(d)))

	// Convert duration.
	seconds := int64(duration.Seconds()) % 60
	minutes := int64(duration.Minutes()) % 60
	hours := int64(duration.Hours()) % 24
	days := int64(duration/(24*time.Hour)) % 365 % 7

	// Edge case between 364 and 365 days.
	// We need to calculate weeks from what is left from years
	leftYearDays := int64(duration/(24*time.Hour)) % 365
	weeks := leftYearDays / 7
	if leftYearDays >= 364 && leftYearDays < 365 {
		weeks = 52
	}

	years := int64(duration/(24*time.Hour)) / 365
	milliseconds := int64(duration/time.Millisecond) -
		(seconds * 1000) - (minutes * 60000) - (hours * 3600000) -
		(days * 86400000) - (weeks * 604800000) - (years * 31536000000)

	// Create a map of the converted duration time.
	durationMap := map[string]int64{
		"ms": milliseconds,
		"s":  seconds,
		"m":  minutes,
		"h":  hours,
		"d":  days,
		"w":  weeks,
		"y":  years,
	}

	// Construct duration string.
	numberOfUnits := 0
	for _, u := range [...]string{"y", "w", "d", "h", "m", "s", "ms"} {
		v := durationMap[u]
		strval := strconv.FormatInt(v, 10)
		if v == 0 {
			continue
		}
		readableString.WriteString(strval + u)
		numberOfUnits++
		if maxNumberOfUnits > 0 && numberOfUnits >= maxNumberOfUnits {
			break
		}
	}

	return readableString.String()
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

// UnmarshalJSON makes sure the value can be parsed as a string or integer in JSON
func (d *Duration) UnmarshalJSON(in []byte) error {
	// Check if the input is a string value.
	if len(in) >= 2 && in[0] == '"' && in[len(in)-1] == '"' {
		strVal := string(in[1 : len(in)-1]) // Remove the quotes

		// Attempt to parse the string as a duration.
		parsedDuration, err := ParseDuration(strVal)
		if err != nil {
			return err
		}
		*d = Duration(parsedDuration)
		return nil
	}
	// Handle numeric values.
	var i int64
	err := json.Unmarshal(in, &i)
	if err != nil {
		return err
	}
	*d = Duration(i)
	return nil
}

// Scan implements the fmt.Scanner interface
func (d *Duration) Scan(s fmt.ScanState, ch rune) error {
	token, err := s.Token(true, func(rune) bool { return true })
	if err != nil {
		return err
	}
	return d.Set(string(token))
}
