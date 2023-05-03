package fs

import (
	"encoding/json"
	"fmt"
	"time"
)

// Time is a time.Time with some more parsing options
type Time time.Time

// For overriding in unittests.
var (
	timeNowFunc = time.Now
)

// Turn Time into a string
func (t Time) String() string {
	if !t.IsSet() {
		return "off"
	}
	return time.Time(t).Format(time.RFC3339Nano)
}

// IsSet returns if the time is not zero
func (t Time) IsSet() bool {
	return !time.Time(t).IsZero()
}

// ParseTime parses a time or duration string as a Time.
func ParseTime(date string) (t time.Time, err error) {
	if date == "off" {
		return time.Time{}, nil
	}

	now := timeNowFunc()

	// Attempt to parse as a text time
	t, err = parseTimeDates(date)
	if err == nil {
		return t, nil
	}

	// Attempt to parse as a time.Duration offset from now
	d, err := time.ParseDuration(date)
	if err == nil {
		return now.Add(-d), nil
	}

	d, err = parseDurationSuffixes(date)
	if err == nil {
		return now.Add(-d), nil
	}

	return t, err
}

// Set a Time
func (t *Time) Set(s string) error {
	parsedTime, err := ParseTime(s)
	if err != nil {
		return err
	}
	*t = Time(parsedTime)
	return nil
}

// Type of the value
func (t Time) Type() string {
	return "Time"
}

// UnmarshalJSON makes sure the value can be parsed as a string in JSON
func (t *Time) UnmarshalJSON(in []byte) error {
	var s string
	err := json.Unmarshal(in, &s)
	if err != nil {
		return err
	}

	return t.Set(s)
}

// MarshalJSON marshals as a time.Time value
func (t Time) MarshalJSON() ([]byte, error) {
	return json.Marshal(time.Time(t))
}

// Scan implements the fmt.Scanner interface
func (t *Time) Scan(s fmt.ScanState, ch rune) error {
	token, err := s.Token(true, func(rune) bool { return true })
	if err != nil {
		return err
	}
	return t.Set(string(token))
}
