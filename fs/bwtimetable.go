package fs

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// BwPair represents an upload and a download bandwidth
type BwPair struct {
	Tx SizeSuffix // upload bandwidth
	Rx SizeSuffix // download bandwidth
}

// String returns a printable representation of a BwPair
func (bp *BwPair) String() string {
	var out strings.Builder
	out.WriteString(bp.Tx.String())
	if bp.Rx != bp.Tx {
		out.WriteRune(':')
		out.WriteString(bp.Rx.String())
	}
	return out.String()
}

// Set the bandwidth from a string which is either
// SizeSuffix or SizeSuffix:SizeSuffix (for tx:rx bandwidth)
func (bp *BwPair) Set(s string) (err error) {
	colon := strings.Index(s, ":")
	stx, srx := s, ""
	if colon >= 0 {
		stx, srx = s[:colon], s[colon+1:]
	}
	err = bp.Tx.Set(stx)
	if err != nil {
		return err
	}
	if colon < 0 {
		bp.Rx = bp.Tx
	} else {
		err = bp.Rx.Set(srx)
		if err != nil {
			return err
		}
	}
	return nil
}

// IsSet returns true if either of the bandwidth limits are set
func (bp *BwPair) IsSet() bool {
	return bp.Tx > 0 || bp.Rx > 0
}

// BwTimeSlot represents a bandwidth configuration at a point in time.
type BwTimeSlot struct {
	DayOfTheWeek int
	HHMM         int
	Bandwidth    BwPair
}

// BwTimetable contains all configured time slots.
type BwTimetable []BwTimeSlot

// String returns a printable representation of BwTimetable.
func (x BwTimetable) String() string {
	var out strings.Builder
	bwOnly := len(x) == 1 && x[0].DayOfTheWeek == 0 && x[0].HHMM == 0
	for _, ts := range x {
		if out.Len() != 0 {
			out.WriteRune(' ')
		}
		if !bwOnly {
			_, _ = fmt.Fprintf(&out, "%s-%02d:%02d,", time.Weekday(ts.DayOfTheWeek).String()[:3], ts.HHMM/100, ts.HHMM%100)
		}
		out.WriteString(ts.Bandwidth.String())
	}
	return out.String()
}

// Basic hour format checking
func validateHour(HHMM string) error {
	if len(HHMM) != 5 {
		return fmt.Errorf("invalid time specification (hh:mm): %q", HHMM)
	}
	hh, err := strconv.Atoi(HHMM[0:2])
	if err != nil {
		return fmt.Errorf("invalid hour in time specification %q: %v", HHMM, err)
	}
	if hh < 0 || hh > 23 {
		return fmt.Errorf("invalid hour (must be between 00 and 23): %q", hh)
	}
	mm, err := strconv.Atoi(HHMM[3:])
	if err != nil {
		return fmt.Errorf("invalid minute in time specification: %q: %v", HHMM, err)
	}
	if mm < 0 || mm > 59 {
		return fmt.Errorf("invalid minute (must be between 00 and 59): %q", hh)
	}
	return nil
}

// Basic weekday format checking
func parseWeekday(dayOfWeek string) (int, error) {
	dayOfWeek = strings.ToLower(dayOfWeek)
	if dayOfWeek == "sun" || dayOfWeek == "sunday" {
		return 0, nil
	}
	if dayOfWeek == "mon" || dayOfWeek == "monday" {
		return 1, nil
	}
	if dayOfWeek == "tue" || dayOfWeek == "tuesday" {
		return 2, nil
	}
	if dayOfWeek == "wed" || dayOfWeek == "wednesday" {
		return 3, nil
	}
	if dayOfWeek == "thu" || dayOfWeek == "thursday" {
		return 4, nil
	}
	if dayOfWeek == "fri" || dayOfWeek == "friday" {
		return 5, nil
	}
	if dayOfWeek == "sat" || dayOfWeek == "saturday" {
		return 6, nil
	}
	return 0, fmt.Errorf("invalid weekday: %q", dayOfWeek)
}

// Set the bandwidth timetable.
func (x *BwTimetable) Set(s string) error {
	// The timetable is formatted as:
	// "dayOfWeek-hh:mm,bandwidth dayOfWeek-hh:mm,bandwidth..." ex: "Mon-10:00,10G Mon-11:30,1G Tue-18:00,off"
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
		ts.DayOfTheWeek = 0
		ts.HHMM = 0
		*x = BwTimetable{ts}
		return nil
	}

	// Split the timetable string by both spaces and semicolons
	for _, tok := range strings.FieldsFunc(s, func(r rune) bool {
		return r == ' ' || r == ';'
	}) {
		tv := strings.Split(tok, ",")

		// Format must be dayOfWeek-HH:MM,BW
		if len(tv) != 2 {
			return fmt.Errorf("invalid time/bandwidth specification: %q", tok)
		}

		weekday := 0
		HHMM := ""
		if !strings.Contains(tv[0], "-") {
			HHMM = tv[0]
			if err := validateHour(HHMM); err != nil {
				return err
			}
			for i := 0; i < 7; i++ {
				hh, _ := strconv.Atoi(HHMM[0:2])
				mm, _ := strconv.Atoi(HHMM[3:])
				ts := BwTimeSlot{
					DayOfTheWeek: i,
					HHMM:         (hh * 100) + mm,
				}
				if err := ts.Bandwidth.Set(tv[1]); err != nil {
					return err
				}
				*x = append(*x, ts)
			}
		} else {
			timespec := strings.Split(tv[0], "-")
			if len(timespec) != 2 {
				return fmt.Errorf("invalid time specification: %q", tv[0])
			}
			var err error
			weekday, err = parseWeekday(timespec[0])
			if err != nil {
				return err
			}
			HHMM = timespec[1]
			if err := validateHour(HHMM); err != nil {
				return err
			}

			hh, _ := strconv.Atoi(HHMM[0:2])
			mm, _ := strconv.Atoi(HHMM[3:])
			ts := BwTimeSlot{
				DayOfTheWeek: weekday,
				HHMM:         (hh * 100) + mm,
			}
			// Bandwidth limit for this time slot.
			if err := ts.Bandwidth.Set(tv[1]); err != nil {
				return err
			}
			*x = append(*x, ts)
		}
	}
	return nil
}

// Difference in minutes between lateDayOfWeekHHMM and earlyDayOfWeekHHMM
func timeDiff(lateDayOfWeekHHMM int, earlyDayOfWeekHHMM int) int {

	lateTimeMinutes := (lateDayOfWeekHHMM / 10000) * 24 * 60
	lateTimeMinutes += ((lateDayOfWeekHHMM / 100) % 100) * 60
	lateTimeMinutes += lateDayOfWeekHHMM % 100

	earlyTimeMinutes := (earlyDayOfWeekHHMM / 10000) * 24 * 60
	earlyTimeMinutes += ((earlyDayOfWeekHHMM / 100) % 100) * 60
	earlyTimeMinutes += earlyDayOfWeekHHMM % 100

	return lateTimeMinutes - earlyTimeMinutes
}

// LimitAt returns a BwTimeSlot for the time requested.
func (x BwTimetable) LimitAt(tt time.Time) BwTimeSlot {
	// If the timetable is empty, we return an unlimited BwTimeSlot starting at Sunday midnight.
	if len(x) == 0 {
		return BwTimeSlot{Bandwidth: BwPair{-1, -1}}
	}

	dayOfWeekHHMM := int(tt.Weekday())*10000 + tt.Hour()*100 + tt.Minute()

	// By default, we return the last element in the timetable. This
	// satisfies two conditions: 1) If there's only one element it
	// will always be selected, and 2) The last element of the table
	// will "wrap around" until overridden by an earlier time slot.
	// there's only one time slot in the timetable.
	ret := x[len(x)-1]
	mindif := 0
	first := true

	// Look for most recent time slot.
	for _, ts := range x {
		// Ignore the past
		if dayOfWeekHHMM < (ts.DayOfTheWeek*10000)+ts.HHMM {
			continue
		}
		dif := timeDiff(dayOfWeekHHMM, (ts.DayOfTheWeek*10000)+ts.HHMM)
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

// UnmarshalJSON unmarshals a string value
func (x *BwTimetable) UnmarshalJSON(in []byte) error {
	var s string
	err := json.Unmarshal(in, &s)
	if err != nil {
		return err
	}
	return x.Set(s)
}

// MarshalJSON marshals as a string value
func (x BwTimetable) MarshalJSON() ([]byte, error) {
	s := x.String()
	return json.Marshal(s)
}
