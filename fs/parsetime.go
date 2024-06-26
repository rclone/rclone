package fs

import (
	"encoding/json"
	"fmt"
	"time"
)

// For overriding in unittests.
var (
	timeNowFunc = time.Now
)

type timeInferer interface {
	isDurationInFuture() bool
}
type timeInfererFuture struct {
}

func (t timeInfererFuture) isDurationInFuture() bool {
	return true
}

type timeInfererPast struct {
}

func (t timeInfererPast) isDurationInFuture() bool {
	return false
}

type timeInfererUnion interface {
	timeInfererFuture | timeInfererPast
	timeInferer
}

type timeParent[I timeInfererUnion] time.Time

// TimeFuture is a time.Time wrapper that provides more parsing options. Duration is treated as in the future.
type TimeFuture = timeParent[timeInfererFuture]

// TimePast is a time.Time wrapper that provides more parsing options. Duration is treated as in the past.
type TimePast = timeParent[timeInfererPast]

// Time is TimePast. kept for compatibility
type Time = TimePast

// Turn Time into a string
func (t timeParent[I]) String() string {
	if !t.IsSet() {
		return "off"
	}
	return time.Time(t).Format(time.RFC3339Nano)
}

// IsSet returns if the time is not zero
func (t timeParent[I]) IsSet() bool {
	return !time.Time(t).IsZero()
}

// ParseTime parses a time or duration string as a Time.
func ParseTime(date string, durationInFuture bool) (t time.Time, err error) {
	if date == "off" {
		return time.Time{}, nil
	}

	now := timeNowFunc()

	// Attempt to parse as a text time
	t, err = parseTimeDates(date)
	if err == nil {
		return t, nil
	}

	mul := time.Duration(-1)
	if durationInFuture {
		mul = 1
	}
	// Attempt to parse as a time.Duration offset from now
	d, err := time.ParseDuration(date)
	if err == nil {
		return now.Add(d * mul), nil
	}

	d, err = parseDurationSuffixes(date)
	if err == nil {
		return now.Add(d * mul), nil
	}

	return t, err
}

// Set a Time
func (t *timeParent[I]) Set(s string) error {
	parsedTime, err := ParseTime(s, (any(new(I)).(timeInferer)).isDurationInFuture())
	if err != nil {
		return err
	}
	*t = timeParent[I](parsedTime)
	return nil
}

// Type of the value
func (t timeParent[I]) Type() string {
	return "Time"
}

// UnmarshalJSON makes sure the value can be parsed as a string in JSON
func (t *timeParent[I]) UnmarshalJSON(in []byte) error {
	var s string
	err := json.Unmarshal(in, &s)
	if err != nil {
		return err
	}

	return t.Set(s)
}

// MarshalJSON marshals as a time.Time value
func (t timeParent[I]) MarshalJSON() ([]byte, error) {
	return json.Marshal(time.Time(t))
}

// Scan implements the fmt.Scanner interface
func (t *timeParent[I]) Scan(s fmt.ScanState, ch rune) error {
	token, err := s.Token(true, func(rune) bool { return true })
	if err != nil {
		return err
	}
	return t.Set(string(token))
}
