package fs

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/pkg/errors"
)

// BwTimeSlot represents a bandwidth configuration at a point in time.
type BwTimeSlot struct {
	HHMM      int
	Bandwidth SizeSuffix
}

// BwTimetable contains all configured time slots.
type BwTimetable []BwTimeSlot

// String returns a printable representation of BwTimetable.
func (x BwTimetable) String() string {
	ret := []string{}
	for _, ts := range x {
		ret = append(ret, fmt.Sprintf("%04.4d,%s", ts.HHMM, ts.Bandwidth.String()))
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
		if err := ts.Bandwidth.Set(s); err != nil {
			return err
		}
		ts.HHMM = 0
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
		HHMM := tv[0]
		if len(HHMM) != 5 {
			return errors.Errorf("invalid time specification (hh:mm): %q", HHMM)
		}
		hh, err := strconv.Atoi(HHMM[0:2])
		if err != nil {
			return errors.Errorf("invalid hour in time specification %q: %v", HHMM, err)
		}
		if hh < 0 || hh > 23 {
			return errors.Errorf("invalid hour (must be between 00 and 23): %q", hh)
		}
		mm, err := strconv.Atoi(HHMM[3:])
		if err != nil {
			return errors.Errorf("invalid minute in time specification: %q: %v", HHMM, err)
		}
		if mm < 0 || mm > 59 {
			return errors.Errorf("invalid minute (must be between 00 and 59): %q", hh)
		}

		ts := BwTimeSlot{
			HHMM: (hh * 100) + mm,
		}
		// Bandwidth limit for this time slot.
		if err := ts.Bandwidth.Set(tv[1]); err != nil {
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
		return BwTimeSlot{HHMM: 0, Bandwidth: -1}
	}

	HHMM := tt.Hour()*100 + tt.Minute()

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
		if HHMM < ts.HHMM {
			continue
		}
		dif := ((HHMM / 100 * 60) + (HHMM % 100)) - ((ts.HHMM / 100 * 60) + (ts.HHMM % 100))
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
