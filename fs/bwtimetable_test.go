package fs

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Check it satisfies the interface
var _ flagger = (*BwTimetable)(nil)

func TestBwTimetableSet(t *testing.T) {
	for _, test := range []struct {
		in   string
		want BwTimetable
		err  bool
		out  string
	}{
		{"", BwTimetable{}, true, ""},
		{"bad,bad", BwTimetable{}, true, ""},
		{"bad bad", BwTimetable{}, true, ""},
		{"bad", BwTimetable{}, true, ""},
		{"1000X", BwTimetable{}, true, ""},
		{"2401,666", BwTimetable{}, true, ""},
		{"1061,666", BwTimetable{}, true, ""},
		{"bad-10:20,666", BwTimetable{}, true, ""},
		{"Mon-bad,666", BwTimetable{}, true, ""},
		{"Mon-10:20,bad", BwTimetable{}, true, ""},
		{
			"0",
			BwTimetable{
				BwTimeSlot{DayOfTheWeek: 0, HHMM: 0, Bandwidth: BwPair{Tx: 0, Rx: 0}},
			},
			false,
			"0",
		},
		{
			"666",
			BwTimetable{
				BwTimeSlot{DayOfTheWeek: 0, HHMM: 0, Bandwidth: BwPair{Tx: 666 * 1024, Rx: 666 * 1024}},
			},
			false,
			"666Ki",
		},
		{
			"666:333",
			BwTimetable{
				BwTimeSlot{DayOfTheWeek: 0, HHMM: 0, Bandwidth: BwPair{Tx: 666 * 1024, Rx: 333 * 1024}},
			},
			false,
			"666Ki:333Ki",
		},
		{
			"10:20,666",
			BwTimetable{
				BwTimeSlot{DayOfTheWeek: 0, HHMM: 1020, Bandwidth: BwPair{Tx: 666 * 1024, Rx: 666 * 1024}},
				BwTimeSlot{DayOfTheWeek: 1, HHMM: 1020, Bandwidth: BwPair{Tx: 666 * 1024, Rx: 666 * 1024}},
				BwTimeSlot{DayOfTheWeek: 2, HHMM: 1020, Bandwidth: BwPair{Tx: 666 * 1024, Rx: 666 * 1024}},
				BwTimeSlot{DayOfTheWeek: 3, HHMM: 1020, Bandwidth: BwPair{Tx: 666 * 1024, Rx: 666 * 1024}},
				BwTimeSlot{DayOfTheWeek: 4, HHMM: 1020, Bandwidth: BwPair{Tx: 666 * 1024, Rx: 666 * 1024}},
				BwTimeSlot{DayOfTheWeek: 5, HHMM: 1020, Bandwidth: BwPair{Tx: 666 * 1024, Rx: 666 * 1024}},
				BwTimeSlot{DayOfTheWeek: 6, HHMM: 1020, Bandwidth: BwPair{Tx: 666 * 1024, Rx: 666 * 1024}},
			},
			false,
			"Sun-10:20,666Ki Mon-10:20,666Ki Tue-10:20,666Ki Wed-10:20,666Ki Thu-10:20,666Ki Fri-10:20,666Ki Sat-10:20,666Ki",
		},
		{
			"10:20,666:333",
			BwTimetable{
				BwTimeSlot{DayOfTheWeek: 0, HHMM: 1020, Bandwidth: BwPair{Tx: 666 * 1024, Rx: 333 * 1024}},
				BwTimeSlot{DayOfTheWeek: 1, HHMM: 1020, Bandwidth: BwPair{Tx: 666 * 1024, Rx: 333 * 1024}},
				BwTimeSlot{DayOfTheWeek: 2, HHMM: 1020, Bandwidth: BwPair{Tx: 666 * 1024, Rx: 333 * 1024}},
				BwTimeSlot{DayOfTheWeek: 3, HHMM: 1020, Bandwidth: BwPair{Tx: 666 * 1024, Rx: 333 * 1024}},
				BwTimeSlot{DayOfTheWeek: 4, HHMM: 1020, Bandwidth: BwPair{Tx: 666 * 1024, Rx: 333 * 1024}},
				BwTimeSlot{DayOfTheWeek: 5, HHMM: 1020, Bandwidth: BwPair{Tx: 666 * 1024, Rx: 333 * 1024}},
				BwTimeSlot{DayOfTheWeek: 6, HHMM: 1020, Bandwidth: BwPair{Tx: 666 * 1024, Rx: 333 * 1024}},
			},
			false,
			"Sun-10:20,666Ki:333Ki Mon-10:20,666Ki:333Ki Tue-10:20,666Ki:333Ki Wed-10:20,666Ki:333Ki Thu-10:20,666Ki:333Ki Fri-10:20,666Ki:333Ki Sat-10:20,666Ki:333Ki",
		},
		{
			"11:00,333 13:40,666 23:50,10M 23:59,off",
			BwTimetable{
				BwTimeSlot{DayOfTheWeek: 0, HHMM: 1100, Bandwidth: BwPair{Tx: 333 * 1024, Rx: 333 * 1024}},
				BwTimeSlot{DayOfTheWeek: 1, HHMM: 1100, Bandwidth: BwPair{Tx: 333 * 1024, Rx: 333 * 1024}},
				BwTimeSlot{DayOfTheWeek: 2, HHMM: 1100, Bandwidth: BwPair{Tx: 333 * 1024, Rx: 333 * 1024}},
				BwTimeSlot{DayOfTheWeek: 3, HHMM: 1100, Bandwidth: BwPair{Tx: 333 * 1024, Rx: 333 * 1024}},
				BwTimeSlot{DayOfTheWeek: 4, HHMM: 1100, Bandwidth: BwPair{Tx: 333 * 1024, Rx: 333 * 1024}},
				BwTimeSlot{DayOfTheWeek: 5, HHMM: 1100, Bandwidth: BwPair{Tx: 333 * 1024, Rx: 333 * 1024}},
				BwTimeSlot{DayOfTheWeek: 6, HHMM: 1100, Bandwidth: BwPair{Tx: 333 * 1024, Rx: 333 * 1024}},
				BwTimeSlot{DayOfTheWeek: 0, HHMM: 1340, Bandwidth: BwPair{Tx: 666 * 1024, Rx: 666 * 1024}},
				BwTimeSlot{DayOfTheWeek: 1, HHMM: 1340, Bandwidth: BwPair{Tx: 666 * 1024, Rx: 666 * 1024}},
				BwTimeSlot{DayOfTheWeek: 2, HHMM: 1340, Bandwidth: BwPair{Tx: 666 * 1024, Rx: 666 * 1024}},
				BwTimeSlot{DayOfTheWeek: 3, HHMM: 1340, Bandwidth: BwPair{Tx: 666 * 1024, Rx: 666 * 1024}},
				BwTimeSlot{DayOfTheWeek: 4, HHMM: 1340, Bandwidth: BwPair{Tx: 666 * 1024, Rx: 666 * 1024}},
				BwTimeSlot{DayOfTheWeek: 5, HHMM: 1340, Bandwidth: BwPair{Tx: 666 * 1024, Rx: 666 * 1024}},
				BwTimeSlot{DayOfTheWeek: 6, HHMM: 1340, Bandwidth: BwPair{Tx: 666 * 1024, Rx: 666 * 1024}},
				BwTimeSlot{DayOfTheWeek: 0, HHMM: 2350, Bandwidth: BwPair{Tx: 10 * 1024 * 1024, Rx: 10 * 1024 * 1024}},
				BwTimeSlot{DayOfTheWeek: 1, HHMM: 2350, Bandwidth: BwPair{Tx: 10 * 1024 * 1024, Rx: 10 * 1024 * 1024}},
				BwTimeSlot{DayOfTheWeek: 2, HHMM: 2350, Bandwidth: BwPair{Tx: 10 * 1024 * 1024, Rx: 10 * 1024 * 1024}},
				BwTimeSlot{DayOfTheWeek: 3, HHMM: 2350, Bandwidth: BwPair{Tx: 10 * 1024 * 1024, Rx: 10 * 1024 * 1024}},
				BwTimeSlot{DayOfTheWeek: 4, HHMM: 2350, Bandwidth: BwPair{Tx: 10 * 1024 * 1024, Rx: 10 * 1024 * 1024}},
				BwTimeSlot{DayOfTheWeek: 5, HHMM: 2350, Bandwidth: BwPair{Tx: 10 * 1024 * 1024, Rx: 10 * 1024 * 1024}},
				BwTimeSlot{DayOfTheWeek: 6, HHMM: 2350, Bandwidth: BwPair{Tx: 10 * 1024 * 1024, Rx: 10 * 1024 * 1024}},
				BwTimeSlot{DayOfTheWeek: 0, HHMM: 2359, Bandwidth: BwPair{Tx: -1, Rx: -1}},
				BwTimeSlot{DayOfTheWeek: 1, HHMM: 2359, Bandwidth: BwPair{Tx: -1, Rx: -1}},
				BwTimeSlot{DayOfTheWeek: 2, HHMM: 2359, Bandwidth: BwPair{Tx: -1, Rx: -1}},
				BwTimeSlot{DayOfTheWeek: 3, HHMM: 2359, Bandwidth: BwPair{Tx: -1, Rx: -1}},
				BwTimeSlot{DayOfTheWeek: 4, HHMM: 2359, Bandwidth: BwPair{Tx: -1, Rx: -1}},
				BwTimeSlot{DayOfTheWeek: 5, HHMM: 2359, Bandwidth: BwPair{Tx: -1, Rx: -1}},
				BwTimeSlot{DayOfTheWeek: 6, HHMM: 2359, Bandwidth: BwPair{Tx: -1, Rx: -1}},
			},
			false,
			"Sun-11:00,333Ki Mon-11:00,333Ki Tue-11:00,333Ki Wed-11:00,333Ki Thu-11:00,333Ki Fri-11:00,333Ki Sat-11:00,333Ki Sun-13:40,666Ki Mon-13:40,666Ki Tue-13:40,666Ki Wed-13:40,666Ki Thu-13:40,666Ki Fri-13:40,666Ki Sat-13:40,666Ki Sun-23:50,10Mi Mon-23:50,10Mi Tue-23:50,10Mi Wed-23:50,10Mi Thu-23:50,10Mi Fri-23:50,10Mi Sat-23:50,10Mi Sun-23:59,off Mon-23:59,off Tue-23:59,off Wed-23:59,off Thu-23:59,off Fri-23:59,off Sat-23:59,off",
		},
		{
			"11:00,333:666 13:40,666:off 23:50,10M:1M 23:59,off:10M",
			BwTimetable{
				BwTimeSlot{DayOfTheWeek: 0, HHMM: 1100, Bandwidth: BwPair{Tx: 333 * 1024, Rx: 666 * 1024}},
				BwTimeSlot{DayOfTheWeek: 1, HHMM: 1100, Bandwidth: BwPair{Tx: 333 * 1024, Rx: 666 * 1024}},
				BwTimeSlot{DayOfTheWeek: 2, HHMM: 1100, Bandwidth: BwPair{Tx: 333 * 1024, Rx: 666 * 1024}},
				BwTimeSlot{DayOfTheWeek: 3, HHMM: 1100, Bandwidth: BwPair{Tx: 333 * 1024, Rx: 666 * 1024}},
				BwTimeSlot{DayOfTheWeek: 4, HHMM: 1100, Bandwidth: BwPair{Tx: 333 * 1024, Rx: 666 * 1024}},
				BwTimeSlot{DayOfTheWeek: 5, HHMM: 1100, Bandwidth: BwPair{Tx: 333 * 1024, Rx: 666 * 1024}},
				BwTimeSlot{DayOfTheWeek: 6, HHMM: 1100, Bandwidth: BwPair{Tx: 333 * 1024, Rx: 666 * 1024}},
				BwTimeSlot{DayOfTheWeek: 0, HHMM: 1340, Bandwidth: BwPair{Tx: 666 * 1024, Rx: -1}},
				BwTimeSlot{DayOfTheWeek: 1, HHMM: 1340, Bandwidth: BwPair{Tx: 666 * 1024, Rx: -1}},
				BwTimeSlot{DayOfTheWeek: 2, HHMM: 1340, Bandwidth: BwPair{Tx: 666 * 1024, Rx: -1}},
				BwTimeSlot{DayOfTheWeek: 3, HHMM: 1340, Bandwidth: BwPair{Tx: 666 * 1024, Rx: -1}},
				BwTimeSlot{DayOfTheWeek: 4, HHMM: 1340, Bandwidth: BwPair{Tx: 666 * 1024, Rx: -1}},
				BwTimeSlot{DayOfTheWeek: 5, HHMM: 1340, Bandwidth: BwPair{Tx: 666 * 1024, Rx: -1}},
				BwTimeSlot{DayOfTheWeek: 6, HHMM: 1340, Bandwidth: BwPair{Tx: 666 * 1024, Rx: -1}},
				BwTimeSlot{DayOfTheWeek: 0, HHMM: 2350, Bandwidth: BwPair{Tx: 10 * 1024 * 1024, Rx: 1 * 1024 * 1024}},
				BwTimeSlot{DayOfTheWeek: 1, HHMM: 2350, Bandwidth: BwPair{Tx: 10 * 1024 * 1024, Rx: 1 * 1024 * 1024}},
				BwTimeSlot{DayOfTheWeek: 2, HHMM: 2350, Bandwidth: BwPair{Tx: 10 * 1024 * 1024, Rx: 1 * 1024 * 1024}},
				BwTimeSlot{DayOfTheWeek: 3, HHMM: 2350, Bandwidth: BwPair{Tx: 10 * 1024 * 1024, Rx: 1 * 1024 * 1024}},
				BwTimeSlot{DayOfTheWeek: 4, HHMM: 2350, Bandwidth: BwPair{Tx: 10 * 1024 * 1024, Rx: 1 * 1024 * 1024}},
				BwTimeSlot{DayOfTheWeek: 5, HHMM: 2350, Bandwidth: BwPair{Tx: 10 * 1024 * 1024, Rx: 1 * 1024 * 1024}},
				BwTimeSlot{DayOfTheWeek: 6, HHMM: 2350, Bandwidth: BwPair{Tx: 10 * 1024 * 1024, Rx: 1 * 1024 * 1024}},
				BwTimeSlot{DayOfTheWeek: 0, HHMM: 2359, Bandwidth: BwPair{Tx: -1, Rx: 10 * 1024 * 1024}},
				BwTimeSlot{DayOfTheWeek: 1, HHMM: 2359, Bandwidth: BwPair{Tx: -1, Rx: 10 * 1024 * 1024}},
				BwTimeSlot{DayOfTheWeek: 2, HHMM: 2359, Bandwidth: BwPair{Tx: -1, Rx: 10 * 1024 * 1024}},
				BwTimeSlot{DayOfTheWeek: 3, HHMM: 2359, Bandwidth: BwPair{Tx: -1, Rx: 10 * 1024 * 1024}},
				BwTimeSlot{DayOfTheWeek: 4, HHMM: 2359, Bandwidth: BwPair{Tx: -1, Rx: 10 * 1024 * 1024}},
				BwTimeSlot{DayOfTheWeek: 5, HHMM: 2359, Bandwidth: BwPair{Tx: -1, Rx: 10 * 1024 * 1024}},
				BwTimeSlot{DayOfTheWeek: 6, HHMM: 2359, Bandwidth: BwPair{Tx: -1, Rx: 10 * 1024 * 1024}},
			},
			false,
			"Sun-11:00,333Ki:666Ki Mon-11:00,333Ki:666Ki Tue-11:00,333Ki:666Ki Wed-11:00,333Ki:666Ki Thu-11:00,333Ki:666Ki Fri-11:00,333Ki:666Ki Sat-11:00,333Ki:666Ki Sun-13:40,666Ki:off Mon-13:40,666Ki:off Tue-13:40,666Ki:off Wed-13:40,666Ki:off Thu-13:40,666Ki:off Fri-13:40,666Ki:off Sat-13:40,666Ki:off Sun-23:50,10Mi:1Mi Mon-23:50,10Mi:1Mi Tue-23:50,10Mi:1Mi Wed-23:50,10Mi:1Mi Thu-23:50,10Mi:1Mi Fri-23:50,10Mi:1Mi Sat-23:50,10Mi:1Mi Sun-23:59,off:10Mi Mon-23:59,off:10Mi Tue-23:59,off:10Mi Wed-23:59,off:10Mi Thu-23:59,off:10Mi Fri-23:59,off:10Mi Sat-23:59,off:10Mi",
		},
		{
			"Mon-11:00,333 Tue-13:40,666:333 Fri-00:00,10M Sat-10:00,off Sun-23:00,666",
			BwTimetable{
				BwTimeSlot{DayOfTheWeek: 1, HHMM: 1100, Bandwidth: BwPair{Tx: 333 * 1024, Rx: 333 * 1024}},
				BwTimeSlot{DayOfTheWeek: 2, HHMM: 1340, Bandwidth: BwPair{Tx: 666 * 1024, Rx: 333 * 1024}},
				BwTimeSlot{DayOfTheWeek: 5, HHMM: 0000, Bandwidth: BwPair{Tx: 10 * 1024 * 1024, Rx: 10 * 1024 * 1024}},
				BwTimeSlot{DayOfTheWeek: 6, HHMM: 1000, Bandwidth: BwPair{Tx: -1, Rx: -1}},
				BwTimeSlot{DayOfTheWeek: 0, HHMM: 2300, Bandwidth: BwPair{Tx: 666 * 1024, Rx: 666 * 1024}},
			},
			false,
			"Mon-11:00,333Ki Tue-13:40,666Ki:333Ki Fri-00:00,10Mi Sat-10:00,off Sun-23:00,666Ki",
		},
		{
			"Mon-11:00,333 Tue-13:40,666 Fri-00:00,10M 00:01,off Sun-23:00,666:off",
			BwTimetable{
				BwTimeSlot{DayOfTheWeek: 1, HHMM: 1100, Bandwidth: BwPair{Tx: 333 * 1024, Rx: 333 * 1024}},
				BwTimeSlot{DayOfTheWeek: 2, HHMM: 1340, Bandwidth: BwPair{Tx: 666 * 1024, Rx: 666 * 1024}},
				BwTimeSlot{DayOfTheWeek: 5, HHMM: 0000, Bandwidth: BwPair{Tx: 10 * 1024 * 1024, Rx: 10 * 1024 * 1024}},
				BwTimeSlot{DayOfTheWeek: 0, HHMM: 1, Bandwidth: BwPair{Tx: -1, Rx: -1}},
				BwTimeSlot{DayOfTheWeek: 1, HHMM: 1, Bandwidth: BwPair{Tx: -1, Rx: -1}},
				BwTimeSlot{DayOfTheWeek: 2, HHMM: 1, Bandwidth: BwPair{Tx: -1, Rx: -1}},
				BwTimeSlot{DayOfTheWeek: 3, HHMM: 1, Bandwidth: BwPair{Tx: -1, Rx: -1}},
				BwTimeSlot{DayOfTheWeek: 4, HHMM: 1, Bandwidth: BwPair{Tx: -1, Rx: -1}},
				BwTimeSlot{DayOfTheWeek: 5, HHMM: 1, Bandwidth: BwPair{Tx: -1, Rx: -1}},
				BwTimeSlot{DayOfTheWeek: 6, HHMM: 1, Bandwidth: BwPair{Tx: -1, Rx: -1}},
				BwTimeSlot{DayOfTheWeek: 0, HHMM: 2300, Bandwidth: BwPair{Tx: 666 * 1024, Rx: -1}},
			},
			false,
			"Mon-11:00,333Ki Tue-13:40,666Ki Fri-00:00,10Mi Sun-00:01,off Mon-00:01,off Tue-00:01,off Wed-00:01,off Thu-00:01,off Fri-00:01,off Sat-00:01,off Sun-23:00,666Ki:off",
		},
		{
			// from the docs
			"08:00,512 12:00,10M 13:00,512 18:00,30M 23:00,off",
			BwTimetable{
				BwTimeSlot{DayOfTheWeek: 0, HHMM: 800, Bandwidth: BwPair{Tx: 512 * 1024, Rx: 512 * 1024}},
				BwTimeSlot{DayOfTheWeek: 1, HHMM: 800, Bandwidth: BwPair{Tx: 512 * 1024, Rx: 512 * 1024}},
				BwTimeSlot{DayOfTheWeek: 2, HHMM: 800, Bandwidth: BwPair{Tx: 512 * 1024, Rx: 512 * 1024}},
				BwTimeSlot{DayOfTheWeek: 3, HHMM: 800, Bandwidth: BwPair{Tx: 512 * 1024, Rx: 512 * 1024}},
				BwTimeSlot{DayOfTheWeek: 4, HHMM: 800, Bandwidth: BwPair{Tx: 512 * 1024, Rx: 512 * 1024}},
				BwTimeSlot{DayOfTheWeek: 5, HHMM: 800, Bandwidth: BwPair{Tx: 512 * 1024, Rx: 512 * 1024}},
				BwTimeSlot{DayOfTheWeek: 6, HHMM: 800, Bandwidth: BwPair{Tx: 512 * 1024, Rx: 512 * 1024}},
				BwTimeSlot{DayOfTheWeek: 0, HHMM: 1200, Bandwidth: BwPair{Tx: 10 * 1024 * 1024, Rx: 10 * 1024 * 1024}},
				BwTimeSlot{DayOfTheWeek: 1, HHMM: 1200, Bandwidth: BwPair{Tx: 10 * 1024 * 1024, Rx: 10 * 1024 * 1024}},
				BwTimeSlot{DayOfTheWeek: 2, HHMM: 1200, Bandwidth: BwPair{Tx: 10 * 1024 * 1024, Rx: 10 * 1024 * 1024}},
				BwTimeSlot{DayOfTheWeek: 3, HHMM: 1200, Bandwidth: BwPair{Tx: 10 * 1024 * 1024, Rx: 10 * 1024 * 1024}},
				BwTimeSlot{DayOfTheWeek: 4, HHMM: 1200, Bandwidth: BwPair{Tx: 10 * 1024 * 1024, Rx: 10 * 1024 * 1024}},
				BwTimeSlot{DayOfTheWeek: 5, HHMM: 1200, Bandwidth: BwPair{Tx: 10 * 1024 * 1024, Rx: 10 * 1024 * 1024}},
				BwTimeSlot{DayOfTheWeek: 6, HHMM: 1200, Bandwidth: BwPair{Tx: 10 * 1024 * 1024, Rx: 10 * 1024 * 1024}},
				BwTimeSlot{DayOfTheWeek: 0, HHMM: 1300, Bandwidth: BwPair{Tx: 512 * 1024, Rx: 512 * 1024}},
				BwTimeSlot{DayOfTheWeek: 1, HHMM: 1300, Bandwidth: BwPair{Tx: 512 * 1024, Rx: 512 * 1024}},
				BwTimeSlot{DayOfTheWeek: 2, HHMM: 1300, Bandwidth: BwPair{Tx: 512 * 1024, Rx: 512 * 1024}},
				BwTimeSlot{DayOfTheWeek: 3, HHMM: 1300, Bandwidth: BwPair{Tx: 512 * 1024, Rx: 512 * 1024}},
				BwTimeSlot{DayOfTheWeek: 4, HHMM: 1300, Bandwidth: BwPair{Tx: 512 * 1024, Rx: 512 * 1024}},
				BwTimeSlot{DayOfTheWeek: 5, HHMM: 1300, Bandwidth: BwPair{Tx: 512 * 1024, Rx: 512 * 1024}},
				BwTimeSlot{DayOfTheWeek: 6, HHMM: 1300, Bandwidth: BwPair{Tx: 512 * 1024, Rx: 512 * 1024}},
				BwTimeSlot{DayOfTheWeek: 0, HHMM: 1800, Bandwidth: BwPair{Tx: 30 * 1024 * 1024, Rx: 30 * 1024 * 1024}},
				BwTimeSlot{DayOfTheWeek: 1, HHMM: 1800, Bandwidth: BwPair{Tx: 30 * 1024 * 1024, Rx: 30 * 1024 * 1024}},
				BwTimeSlot{DayOfTheWeek: 2, HHMM: 1800, Bandwidth: BwPair{Tx: 30 * 1024 * 1024, Rx: 30 * 1024 * 1024}},
				BwTimeSlot{DayOfTheWeek: 3, HHMM: 1800, Bandwidth: BwPair{Tx: 30 * 1024 * 1024, Rx: 30 * 1024 * 1024}},
				BwTimeSlot{DayOfTheWeek: 4, HHMM: 1800, Bandwidth: BwPair{Tx: 30 * 1024 * 1024, Rx: 30 * 1024 * 1024}},
				BwTimeSlot{DayOfTheWeek: 5, HHMM: 1800, Bandwidth: BwPair{Tx: 30 * 1024 * 1024, Rx: 30 * 1024 * 1024}},
				BwTimeSlot{DayOfTheWeek: 6, HHMM: 1800, Bandwidth: BwPair{Tx: 30 * 1024 * 1024, Rx: 30 * 1024 * 1024}},
				BwTimeSlot{DayOfTheWeek: 0, HHMM: 2300, Bandwidth: BwPair{Tx: -1, Rx: -1}},
				BwTimeSlot{DayOfTheWeek: 1, HHMM: 2300, Bandwidth: BwPair{Tx: -1, Rx: -1}},
				BwTimeSlot{DayOfTheWeek: 2, HHMM: 2300, Bandwidth: BwPair{Tx: -1, Rx: -1}},
				BwTimeSlot{DayOfTheWeek: 3, HHMM: 2300, Bandwidth: BwPair{Tx: -1, Rx: -1}},
				BwTimeSlot{DayOfTheWeek: 4, HHMM: 2300, Bandwidth: BwPair{Tx: -1, Rx: -1}},
				BwTimeSlot{DayOfTheWeek: 5, HHMM: 2300, Bandwidth: BwPair{Tx: -1, Rx: -1}},
				BwTimeSlot{DayOfTheWeek: 6, HHMM: 2300, Bandwidth: BwPair{Tx: -1, Rx: -1}},
			},
			false,
			"Sun-08:00,512Ki Mon-08:00,512Ki Tue-08:00,512Ki Wed-08:00,512Ki Thu-08:00,512Ki Fri-08:00,512Ki Sat-08:00,512Ki Sun-12:00,10Mi Mon-12:00,10Mi Tue-12:00,10Mi Wed-12:00,10Mi Thu-12:00,10Mi Fri-12:00,10Mi Sat-12:00,10Mi Sun-13:00,512Ki Mon-13:00,512Ki Tue-13:00,512Ki Wed-13:00,512Ki Thu-13:00,512Ki Fri-13:00,512Ki Sat-13:00,512Ki Sun-18:00,30Mi Mon-18:00,30Mi Tue-18:00,30Mi Wed-18:00,30Mi Thu-18:00,30Mi Fri-18:00,30Mi Sat-18:00,30Mi Sun-23:00,off Mon-23:00,off Tue-23:00,off Wed-23:00,off Thu-23:00,off Fri-23:00,off Sat-23:00,off",
		},
		{
			// from the docs
			"Mon-00:00,512 Fri-23:59,10M Sat-10:00,1M Sun-20:00,off",
			BwTimetable{
				BwTimeSlot{DayOfTheWeek: 1, HHMM: 0, Bandwidth: BwPair{Tx: 512 * 1024, Rx: 512 * 1024}},
				BwTimeSlot{DayOfTheWeek: 5, HHMM: 2359, Bandwidth: BwPair{Tx: 10 * 1024 * 1024, Rx: 10 * 1024 * 1024}},
				BwTimeSlot{DayOfTheWeek: 6, HHMM: 1000, Bandwidth: BwPair{Tx: 1024 * 1024, Rx: 1024 * 1024}},
				BwTimeSlot{DayOfTheWeek: 0, HHMM: 2000, Bandwidth: BwPair{Tx: -1, Rx: -1}},
			},
			false,
			"Mon-00:00,512Ki Fri-23:59,10Mi Sat-10:00,1Mi Sun-20:00,off",
		},
		{
			// from the docs
			"Mon-00:00,512 12:00,1M Sun-20:00,off",
			BwTimetable{
				BwTimeSlot{DayOfTheWeek: 1, HHMM: 0, Bandwidth: BwPair{Tx: 512 * 1024, Rx: 512 * 1024}},
				BwTimeSlot{DayOfTheWeek: 0, HHMM: 1200, Bandwidth: BwPair{Tx: 1024 * 1024, Rx: 1024 * 1024}},
				BwTimeSlot{DayOfTheWeek: 1, HHMM: 1200, Bandwidth: BwPair{Tx: 1024 * 1024, Rx: 1024 * 1024}},
				BwTimeSlot{DayOfTheWeek: 2, HHMM: 1200, Bandwidth: BwPair{Tx: 1024 * 1024, Rx: 1024 * 1024}},
				BwTimeSlot{DayOfTheWeek: 3, HHMM: 1200, Bandwidth: BwPair{Tx: 1024 * 1024, Rx: 1024 * 1024}},
				BwTimeSlot{DayOfTheWeek: 4, HHMM: 1200, Bandwidth: BwPair{Tx: 1024 * 1024, Rx: 1024 * 1024}},
				BwTimeSlot{DayOfTheWeek: 5, HHMM: 1200, Bandwidth: BwPair{Tx: 1024 * 1024, Rx: 1024 * 1024}},
				BwTimeSlot{DayOfTheWeek: 6, HHMM: 1200, Bandwidth: BwPair{Tx: 1024 * 1024, Rx: 1024 * 1024}},
				BwTimeSlot{DayOfTheWeek: 0, HHMM: 2000, Bandwidth: BwPair{Tx: -1, Rx: -1}},
			},
			false,
			"Mon-00:00,512Ki Sun-12:00,1Mi Mon-12:00,1Mi Tue-12:00,1Mi Wed-12:00,1Mi Thu-12:00,1Mi Fri-12:00,1Mi Sat-12:00,1Mi Sun-20:00,off",
		},
	} {
		tt := BwTimetable{}
		err := tt.Set(test.in)
		if test.err {
			require.Error(t, err)
		} else {
			require.NoError(t, err)
		}
		assert.Equal(t, test.want, tt)
		assert.Equal(t, test.out, tt.String())
	}
}

func TestBwTimetableLimitAt(t *testing.T) {
	for _, test := range []struct {
		tt   BwTimetable
		now  time.Time
		want BwTimeSlot
	}{
		{
			BwTimetable{},
			time.Date(2017, time.April, 20, 15, 0, 0, 0, time.UTC),
			BwTimeSlot{DayOfTheWeek: 0, HHMM: 0, Bandwidth: BwPair{Tx: -1, Rx: -1}},
		},
		{
			BwTimetable{
				BwTimeSlot{DayOfTheWeek: 0, HHMM: 1100, Bandwidth: BwPair{Tx: 333 * 1024, Rx: 333 * 1024}},
				BwTimeSlot{DayOfTheWeek: 1, HHMM: 1100, Bandwidth: BwPair{Tx: 333 * 1024, Rx: 333 * 1024}},
				BwTimeSlot{DayOfTheWeek: 2, HHMM: 1100, Bandwidth: BwPair{Tx: 333 * 1024, Rx: 333 * 1024}},
				BwTimeSlot{DayOfTheWeek: 3, HHMM: 1100, Bandwidth: BwPair{Tx: 333 * 1024, Rx: 333 * 1024}},
				BwTimeSlot{DayOfTheWeek: 4, HHMM: 1100, Bandwidth: BwPair{Tx: 333 * 1024, Rx: 666 * 1024}},
				BwTimeSlot{DayOfTheWeek: 5, HHMM: 1100, Bandwidth: BwPair{Tx: 333 * 1024, Rx: 333 * 1024}},
				BwTimeSlot{DayOfTheWeek: 6, HHMM: 1100, Bandwidth: BwPair{Tx: 333 * 1024, Rx: 333 * 1024}},
			},
			time.Date(2017, time.April, 20, 15, 0, 0, 0, time.UTC),
			BwTimeSlot{DayOfTheWeek: 4, HHMM: 1100, Bandwidth: BwPair{Tx: 333 * 1024, Rx: 666 * 1024}},
		},
		{
			BwTimetable{
				BwTimeSlot{DayOfTheWeek: 0, HHMM: 1100, Bandwidth: BwPair{Tx: 333 * 1024, Rx: 33 * 1024}},
				BwTimeSlot{DayOfTheWeek: 1, HHMM: 1100, Bandwidth: BwPair{Tx: 333 * 1024, Rx: 33 * 1024}},
				BwTimeSlot{DayOfTheWeek: 2, HHMM: 1100, Bandwidth: BwPair{Tx: 333 * 1024, Rx: 33 * 1024}},
				BwTimeSlot{DayOfTheWeek: 3, HHMM: 1100, Bandwidth: BwPair{Tx: 333 * 1024, Rx: 33 * 1024}},
				BwTimeSlot{DayOfTheWeek: 4, HHMM: 1100, Bandwidth: BwPair{Tx: 333 * 1024, Rx: 33 * 1024}},
				BwTimeSlot{DayOfTheWeek: 5, HHMM: 1100, Bandwidth: BwPair{Tx: 333 * 1024, Rx: 33 * 1024}},
				BwTimeSlot{DayOfTheWeek: 6, HHMM: 1100, Bandwidth: BwPair{Tx: 333 * 1024, Rx: 33 * 1024}},
				BwTimeSlot{DayOfTheWeek: 0, HHMM: 1300, Bandwidth: BwPair{Tx: 666 * 1024, Rx: 66 * 1024}},
				BwTimeSlot{DayOfTheWeek: 1, HHMM: 1300, Bandwidth: BwPair{Tx: 666 * 1024, Rx: 66 * 1024}},
				BwTimeSlot{DayOfTheWeek: 2, HHMM: 1300, Bandwidth: BwPair{Tx: 666 * 1024, Rx: 66 * 1024}},
				BwTimeSlot{DayOfTheWeek: 3, HHMM: 1300, Bandwidth: BwPair{Tx: 666 * 1024, Rx: 66 * 1024}},
				BwTimeSlot{DayOfTheWeek: 4, HHMM: 1300, Bandwidth: BwPair{Tx: 666 * 1024, Rx: 66 * 1024}},
				BwTimeSlot{DayOfTheWeek: 5, HHMM: 1300, Bandwidth: BwPair{Tx: 666 * 1024, Rx: 66 * 1024}},
				BwTimeSlot{DayOfTheWeek: 6, HHMM: 1300, Bandwidth: BwPair{Tx: 666 * 1024, Rx: 66 * 1024}},
				BwTimeSlot{DayOfTheWeek: 0, HHMM: 2301, Bandwidth: BwPair{Tx: 1024 * 1024, Rx: 102 * 1024}},
				BwTimeSlot{DayOfTheWeek: 1, HHMM: 2301, Bandwidth: BwPair{Tx: 1024 * 1024, Rx: 102 * 1024}},
				BwTimeSlot{DayOfTheWeek: 2, HHMM: 2301, Bandwidth: BwPair{Tx: 1024 * 1024, Rx: 102 * 1024}},
				BwTimeSlot{DayOfTheWeek: 3, HHMM: 2301, Bandwidth: BwPair{Tx: 1024 * 1024, Rx: 102 * 1024}},
				BwTimeSlot{DayOfTheWeek: 4, HHMM: 2301, Bandwidth: BwPair{Tx: 1024 * 1024, Rx: 102 * 1024}},
				BwTimeSlot{DayOfTheWeek: 5, HHMM: 2301, Bandwidth: BwPair{Tx: 1024 * 1024, Rx: 102 * 1024}},
				BwTimeSlot{DayOfTheWeek: 6, HHMM: 2301, Bandwidth: BwPair{Tx: 1024 * 1024, Rx: 102 * 1024}},
				BwTimeSlot{DayOfTheWeek: 0, HHMM: 2350, Bandwidth: BwPair{Tx: -1, Rx: 1024 * 1024}},
				BwTimeSlot{DayOfTheWeek: 1, HHMM: 2350, Bandwidth: BwPair{Tx: -1, Rx: 1024 * 1024}},
				BwTimeSlot{DayOfTheWeek: 2, HHMM: 2350, Bandwidth: BwPair{Tx: -1, Rx: 1024 * 1024}},
				BwTimeSlot{DayOfTheWeek: 3, HHMM: 2350, Bandwidth: BwPair{Tx: -1, Rx: 1024 * 1024}},
				BwTimeSlot{DayOfTheWeek: 4, HHMM: 2350, Bandwidth: BwPair{Tx: -1, Rx: 1024 * 1024}},
				BwTimeSlot{DayOfTheWeek: 5, HHMM: 2350, Bandwidth: BwPair{Tx: -1, Rx: 1024 * 1024}},
				BwTimeSlot{DayOfTheWeek: 6, HHMM: 2350, Bandwidth: BwPair{Tx: -1, Rx: 1024 * 1024}},
			},
			time.Date(2017, time.April, 20, 10, 15, 0, 0, time.UTC),
			BwTimeSlot{DayOfTheWeek: 3, HHMM: 2350, Bandwidth: BwPair{Tx: -1, Rx: 1024 * 1024}},
		},
		{
			BwTimetable{
				BwTimeSlot{DayOfTheWeek: 0, HHMM: 1100, Bandwidth: BwPair{Tx: 333 * 1024, Rx: 33 * 1024}},
				BwTimeSlot{DayOfTheWeek: 1, HHMM: 1100, Bandwidth: BwPair{Tx: 333 * 1024, Rx: 33 * 1024}},
				BwTimeSlot{DayOfTheWeek: 2, HHMM: 1100, Bandwidth: BwPair{Tx: 333 * 1024, Rx: 33 * 1024}},
				BwTimeSlot{DayOfTheWeek: 3, HHMM: 1100, Bandwidth: BwPair{Tx: 333 * 1024, Rx: 33 * 1024}},
				BwTimeSlot{DayOfTheWeek: 4, HHMM: 1100, Bandwidth: BwPair{Tx: 333 * 1024, Rx: 33 * 1024}},
				BwTimeSlot{DayOfTheWeek: 5, HHMM: 1100, Bandwidth: BwPair{Tx: 333 * 1024, Rx: 33 * 1024}},
				BwTimeSlot{DayOfTheWeek: 6, HHMM: 1100, Bandwidth: BwPair{Tx: 333 * 1024, Rx: 33 * 1024}},
				BwTimeSlot{DayOfTheWeek: 0, HHMM: 1300, Bandwidth: BwPair{Tx: 666 * 1024, Rx: 66 * 1024}},
				BwTimeSlot{DayOfTheWeek: 1, HHMM: 1300, Bandwidth: BwPair{Tx: 666 * 1024, Rx: 66 * 1024}},
				BwTimeSlot{DayOfTheWeek: 2, HHMM: 1300, Bandwidth: BwPair{Tx: 666 * 1024, Rx: 66 * 1024}},
				BwTimeSlot{DayOfTheWeek: 3, HHMM: 1300, Bandwidth: BwPair{Tx: 666 * 1024, Rx: 66 * 1024}},
				BwTimeSlot{DayOfTheWeek: 4, HHMM: 1300, Bandwidth: BwPair{Tx: 666 * 1024, Rx: 66 * 1024}},
				BwTimeSlot{DayOfTheWeek: 5, HHMM: 1300, Bandwidth: BwPair{Tx: 666 * 1024, Rx: 66 * 1024}},
				BwTimeSlot{DayOfTheWeek: 6, HHMM: 1300, Bandwidth: BwPair{Tx: 666 * 1024, Rx: 66 * 1024}},
				BwTimeSlot{DayOfTheWeek: 0, HHMM: 2301, Bandwidth: BwPair{Tx: 1024 * 1024, Rx: 102 * 1024}},
				BwTimeSlot{DayOfTheWeek: 1, HHMM: 2301, Bandwidth: BwPair{Tx: 1024 * 1024, Rx: 102 * 1024}},
				BwTimeSlot{DayOfTheWeek: 2, HHMM: 2301, Bandwidth: BwPair{Tx: 1024 * 1024, Rx: 102 * 1024}},
				BwTimeSlot{DayOfTheWeek: 3, HHMM: 2301, Bandwidth: BwPair{Tx: 1024 * 1024, Rx: 102 * 1024}},
				BwTimeSlot{DayOfTheWeek: 4, HHMM: 2301, Bandwidth: BwPair{Tx: 1024 * 1024, Rx: 102 * 1024}},
				BwTimeSlot{DayOfTheWeek: 5, HHMM: 2301, Bandwidth: BwPair{Tx: 1024 * 1024, Rx: 102 * 1024}},
				BwTimeSlot{DayOfTheWeek: 6, HHMM: 2301, Bandwidth: BwPair{Tx: 1024 * 1024, Rx: 102 * 1024}},
				BwTimeSlot{DayOfTheWeek: 0, HHMM: 2350, Bandwidth: BwPair{Tx: -1, Rx: 1024 * 1024}},
				BwTimeSlot{DayOfTheWeek: 1, HHMM: 2350, Bandwidth: BwPair{Tx: -1, Rx: 1024 * 1024}},
				BwTimeSlot{DayOfTheWeek: 2, HHMM: 2350, Bandwidth: BwPair{Tx: -1, Rx: 1024 * 1024}},
				BwTimeSlot{DayOfTheWeek: 3, HHMM: 2350, Bandwidth: BwPair{Tx: -1, Rx: 1024 * 1024}},
				BwTimeSlot{DayOfTheWeek: 4, HHMM: 2350, Bandwidth: BwPair{Tx: -1, Rx: 1024 * 1024}},
				BwTimeSlot{DayOfTheWeek: 5, HHMM: 2350, Bandwidth: BwPair{Tx: -1, Rx: 1024 * 1024}},
				BwTimeSlot{DayOfTheWeek: 6, HHMM: 2350, Bandwidth: BwPair{Tx: -1, Rx: 1024 * 1024}},
			},
			time.Date(2017, time.April, 20, 11, 0, 0, 0, time.UTC),
			BwTimeSlot{DayOfTheWeek: 4, HHMM: 1100, Bandwidth: BwPair{Tx: 333 * 1024, Rx: 33 * 1024}},
		},
		{
			BwTimetable{
				BwTimeSlot{DayOfTheWeek: 0, HHMM: 1100, Bandwidth: BwPair{Tx: 333 * 1024, Rx: 33 * 1024}},
				BwTimeSlot{DayOfTheWeek: 1, HHMM: 1100, Bandwidth: BwPair{Tx: 333 * 1024, Rx: 33 * 1024}},
				BwTimeSlot{DayOfTheWeek: 2, HHMM: 1100, Bandwidth: BwPair{Tx: 333 * 1024, Rx: 33 * 1024}},
				BwTimeSlot{DayOfTheWeek: 3, HHMM: 1100, Bandwidth: BwPair{Tx: 333 * 1024, Rx: 33 * 1024}},
				BwTimeSlot{DayOfTheWeek: 4, HHMM: 1100, Bandwidth: BwPair{Tx: 333 * 1024, Rx: 33 * 1024}},
				BwTimeSlot{DayOfTheWeek: 5, HHMM: 1100, Bandwidth: BwPair{Tx: 333 * 1024, Rx: 33 * 1024}},
				BwTimeSlot{DayOfTheWeek: 6, HHMM: 1100, Bandwidth: BwPair{Tx: 333 * 1024, Rx: 33 * 1024}},
				BwTimeSlot{DayOfTheWeek: 0, HHMM: 1300, Bandwidth: BwPair{Tx: 666 * 1024, Rx: 66 * 1024}},
				BwTimeSlot{DayOfTheWeek: 1, HHMM: 1300, Bandwidth: BwPair{Tx: 666 * 1024, Rx: 66 * 1024}},
				BwTimeSlot{DayOfTheWeek: 2, HHMM: 1300, Bandwidth: BwPair{Tx: 666 * 1024, Rx: 66 * 1024}},
				BwTimeSlot{DayOfTheWeek: 3, HHMM: 1300, Bandwidth: BwPair{Tx: 666 * 1024, Rx: 66 * 1024}},
				BwTimeSlot{DayOfTheWeek: 4, HHMM: 1300, Bandwidth: BwPair{Tx: 666 * 1024, Rx: 66 * 1024}},
				BwTimeSlot{DayOfTheWeek: 5, HHMM: 1300, Bandwidth: BwPair{Tx: 666 * 1024, Rx: 66 * 1024}},
				BwTimeSlot{DayOfTheWeek: 6, HHMM: 1300, Bandwidth: BwPair{Tx: 666 * 1024, Rx: 66 * 1024}},
				BwTimeSlot{DayOfTheWeek: 0, HHMM: 2301, Bandwidth: BwPair{Tx: 1024 * 1024, Rx: 102 * 1024}},
				BwTimeSlot{DayOfTheWeek: 1, HHMM: 2301, Bandwidth: BwPair{Tx: 1024 * 1024, Rx: 102 * 1024}},
				BwTimeSlot{DayOfTheWeek: 2, HHMM: 2301, Bandwidth: BwPair{Tx: 1024 * 1024, Rx: 102 * 1024}},
				BwTimeSlot{DayOfTheWeek: 3, HHMM: 2301, Bandwidth: BwPair{Tx: 1024 * 1024, Rx: 102 * 1024}},
				BwTimeSlot{DayOfTheWeek: 4, HHMM: 2301, Bandwidth: BwPair{Tx: 1024 * 1024, Rx: 102 * 1024}},
				BwTimeSlot{DayOfTheWeek: 5, HHMM: 2301, Bandwidth: BwPair{Tx: 1024 * 1024, Rx: 102 * 1024}},
				BwTimeSlot{DayOfTheWeek: 6, HHMM: 2301, Bandwidth: BwPair{Tx: 1024 * 1024, Rx: 102 * 1024}},
				BwTimeSlot{DayOfTheWeek: 0, HHMM: 2350, Bandwidth: BwPair{Tx: -1, Rx: 1024 * 1024}},
				BwTimeSlot{DayOfTheWeek: 1, HHMM: 2350, Bandwidth: BwPair{Tx: -1, Rx: 1024 * 1024}},
				BwTimeSlot{DayOfTheWeek: 2, HHMM: 2350, Bandwidth: BwPair{Tx: -1, Rx: 1024 * 1024}},
				BwTimeSlot{DayOfTheWeek: 3, HHMM: 2350, Bandwidth: BwPair{Tx: -1, Rx: 1024 * 1024}},
				BwTimeSlot{DayOfTheWeek: 4, HHMM: 2350, Bandwidth: BwPair{Tx: -1, Rx: 1024 * 1024}},
				BwTimeSlot{DayOfTheWeek: 5, HHMM: 2350, Bandwidth: BwPair{Tx: -1, Rx: 1024 * 1024}},
				BwTimeSlot{DayOfTheWeek: 6, HHMM: 2350, Bandwidth: BwPair{Tx: -1, Rx: 1024 * 1024}},
			},
			time.Date(2017, time.April, 20, 13, 1, 0, 0, time.UTC),
			BwTimeSlot{DayOfTheWeek: 4, HHMM: 1300, Bandwidth: BwPair{Tx: 666 * 1024, Rx: 66 * 1024}},
		},
		{
			BwTimetable{
				BwTimeSlot{DayOfTheWeek: 0, HHMM: 1100, Bandwidth: BwPair{Tx: 333 * 1024, Rx: 33 * 1024}},
				BwTimeSlot{DayOfTheWeek: 1, HHMM: 1100, Bandwidth: BwPair{Tx: 333 * 1024, Rx: 33 * 1024}},
				BwTimeSlot{DayOfTheWeek: 2, HHMM: 1100, Bandwidth: BwPair{Tx: 333 * 1024, Rx: 33 * 1024}},
				BwTimeSlot{DayOfTheWeek: 3, HHMM: 1100, Bandwidth: BwPair{Tx: 333 * 1024, Rx: 33 * 1024}},
				BwTimeSlot{DayOfTheWeek: 4, HHMM: 1100, Bandwidth: BwPair{Tx: 333 * 1024, Rx: 33 * 1024}},
				BwTimeSlot{DayOfTheWeek: 5, HHMM: 1100, Bandwidth: BwPair{Tx: 333 * 1024, Rx: 33 * 1024}},
				BwTimeSlot{DayOfTheWeek: 6, HHMM: 1100, Bandwidth: BwPair{Tx: 333 * 1024, Rx: 33 * 1024}},
				BwTimeSlot{DayOfTheWeek: 0, HHMM: 1300, Bandwidth: BwPair{Tx: 666 * 1024, Rx: 66 * 1024}},
				BwTimeSlot{DayOfTheWeek: 1, HHMM: 1300, Bandwidth: BwPair{Tx: 666 * 1024, Rx: 66 * 1024}},
				BwTimeSlot{DayOfTheWeek: 2, HHMM: 1300, Bandwidth: BwPair{Tx: 666 * 1024, Rx: 66 * 1024}},
				BwTimeSlot{DayOfTheWeek: 3, HHMM: 1300, Bandwidth: BwPair{Tx: 666 * 1024, Rx: 66 * 1024}},
				BwTimeSlot{DayOfTheWeek: 4, HHMM: 1300, Bandwidth: BwPair{Tx: 666 * 1024, Rx: 66 * 1024}},
				BwTimeSlot{DayOfTheWeek: 5, HHMM: 1300, Bandwidth: BwPair{Tx: 666 * 1024, Rx: 66 * 1024}},
				BwTimeSlot{DayOfTheWeek: 6, HHMM: 1300, Bandwidth: BwPair{Tx: 666 * 1024, Rx: 66 * 1024}},
				BwTimeSlot{DayOfTheWeek: 0, HHMM: 2301, Bandwidth: BwPair{Tx: 1024 * 1024, Rx: 102 * 1024}},
				BwTimeSlot{DayOfTheWeek: 1, HHMM: 2301, Bandwidth: BwPair{Tx: 1024 * 1024, Rx: 102 * 1024}},
				BwTimeSlot{DayOfTheWeek: 2, HHMM: 2301, Bandwidth: BwPair{Tx: 1024 * 1024, Rx: 102 * 1024}},
				BwTimeSlot{DayOfTheWeek: 3, HHMM: 2301, Bandwidth: BwPair{Tx: 1024 * 1024, Rx: 102 * 1024}},
				BwTimeSlot{DayOfTheWeek: 4, HHMM: 2301, Bandwidth: BwPair{Tx: 1024 * 1024, Rx: 102 * 1024}},
				BwTimeSlot{DayOfTheWeek: 5, HHMM: 2301, Bandwidth: BwPair{Tx: 1024 * 1024, Rx: 102 * 1024}},
				BwTimeSlot{DayOfTheWeek: 6, HHMM: 2301, Bandwidth: BwPair{Tx: 1024 * 1024, Rx: 102 * 1024}},
				BwTimeSlot{DayOfTheWeek: 0, HHMM: 2350, Bandwidth: BwPair{Tx: -1, Rx: 1024 * 1024}},
				BwTimeSlot{DayOfTheWeek: 1, HHMM: 2350, Bandwidth: BwPair{Tx: -1, Rx: 1024 * 1024}},
				BwTimeSlot{DayOfTheWeek: 2, HHMM: 2350, Bandwidth: BwPair{Tx: -1, Rx: 1024 * 1024}},
				BwTimeSlot{DayOfTheWeek: 3, HHMM: 2350, Bandwidth: BwPair{Tx: -1, Rx: 1024 * 1024}},
				BwTimeSlot{DayOfTheWeek: 4, HHMM: 2350, Bandwidth: BwPair{Tx: -1, Rx: 1024 * 1024}},
				BwTimeSlot{DayOfTheWeek: 5, HHMM: 2350, Bandwidth: BwPair{Tx: -1, Rx: 1024 * 1024}},
				BwTimeSlot{DayOfTheWeek: 6, HHMM: 2350, Bandwidth: BwPair{Tx: -1, Rx: 1024 * 1024}},
			},
			time.Date(2017, time.April, 20, 23, 59, 0, 0, time.UTC),
			BwTimeSlot{DayOfTheWeek: 4, HHMM: 2350, Bandwidth: BwPair{Tx: -1, Rx: 1024 * 1024}},
		},
		{
			BwTimetable{
				BwTimeSlot{DayOfTheWeek: 1, HHMM: 1100, Bandwidth: BwPair{Tx: 333 * 1024, Rx: 33 * 1024}},
				BwTimeSlot{DayOfTheWeek: 2, HHMM: 1340, Bandwidth: BwPair{Tx: 666 * 1024, Rx: 66 * 1024}},
				BwTimeSlot{DayOfTheWeek: 5, HHMM: 0000, Bandwidth: BwPair{Tx: 10 * 1024 * 1024, Rx: 1 * 1024 * 1024}},
				BwTimeSlot{DayOfTheWeek: 6, HHMM: 1000, Bandwidth: BwPair{Tx: -1, Rx: 100 * 1024}},
				BwTimeSlot{DayOfTheWeek: 0, HHMM: 2300, Bandwidth: BwPair{Tx: 666 * 1024, Rx: 66 * 1024}},
			},
			time.Date(2017, time.April, 20, 23, 59, 0, 0, time.UTC),
			BwTimeSlot{DayOfTheWeek: 2, HHMM: 1340, Bandwidth: BwPair{Tx: 666 * 1024, Rx: 66 * 1024}},
		},
		{
			BwTimetable{
				BwTimeSlot{DayOfTheWeek: 1, HHMM: 1100, Bandwidth: BwPair{Tx: 333 * 1024, Rx: 33 * 1024}},
				BwTimeSlot{DayOfTheWeek: 2, HHMM: 1340, Bandwidth: BwPair{Tx: 666 * 1024, Rx: 66 * 1024}},
				BwTimeSlot{DayOfTheWeek: 5, HHMM: 0000, Bandwidth: BwPair{Tx: 10 * 1024 * 1024, Rx: 1 * 1024 * 1024}},
				BwTimeSlot{DayOfTheWeek: 6, HHMM: 1000, Bandwidth: BwPair{Tx: -1, Rx: 100 * 1024}},
				BwTimeSlot{DayOfTheWeek: 0, HHMM: 2300, Bandwidth: BwPair{Tx: 666 * 1024, Rx: 66 * 1024}},
			},
			time.Date(2017, time.April, 21, 23, 59, 0, 0, time.UTC),
			BwTimeSlot{DayOfTheWeek: 5, HHMM: 0000, Bandwidth: BwPair{Tx: 10 * 1024 * 1024, Rx: 1 * 1024 * 1024}},
		},
		{
			BwTimetable{
				BwTimeSlot{DayOfTheWeek: 1, HHMM: 1100, Bandwidth: BwPair{Tx: 333 * 1024, Rx: 33 * 1024}},
				BwTimeSlot{DayOfTheWeek: 2, HHMM: 1340, Bandwidth: BwPair{Tx: 666 * 1024, Rx: 66 * 1024}},
				BwTimeSlot{DayOfTheWeek: 5, HHMM: 0000, Bandwidth: BwPair{Tx: 10 * 1024 * 1024, Rx: 1 * 1024 * 1024}},
				BwTimeSlot{DayOfTheWeek: 6, HHMM: 1000, Bandwidth: BwPair{Tx: -1, Rx: 100 * 1024}},
				BwTimeSlot{DayOfTheWeek: 0, HHMM: 2300, Bandwidth: BwPair{Tx: 666 * 1024, Rx: 66 * 1024}},
			},
			time.Date(2017, time.April, 17, 10, 59, 0, 0, time.UTC),
			BwTimeSlot{DayOfTheWeek: 0, HHMM: 2300, Bandwidth: BwPair{Tx: 666 * 1024, Rx: 66 * 1024}},
		},
	} {
		slot := test.tt.LimitAt(test.now)
		assert.Equal(t, test.want, slot)
	}
}

func TestBwTimetableUnmarshalJSON(t *testing.T) {
	for _, test := range []struct {
		in   string
		want BwTimetable
		err  bool
	}{
		{
			`"Mon-10:20,bad"`,
			BwTimetable(nil),
			true,
		},
		{
			`"0"`,
			BwTimetable{
				BwTimeSlot{DayOfTheWeek: 0, HHMM: 0, Bandwidth: BwPair{Tx: 0, Rx: 0}},
			},
			false,
		},
		{
			`"666"`,
			BwTimetable{
				BwTimeSlot{DayOfTheWeek: 0, HHMM: 0, Bandwidth: BwPair{Tx: 666 * 1024, Rx: 666 * 1024}},
			},
			false,
		},
		{
			`"666:333"`,
			BwTimetable{
				BwTimeSlot{DayOfTheWeek: 0, HHMM: 0, Bandwidth: BwPair{Tx: 666 * 1024, Rx: 333 * 1024}},
			},
			false,
		},
		{
			`"10:20,666"`,
			BwTimetable{
				BwTimeSlot{DayOfTheWeek: 0, HHMM: 1020, Bandwidth: BwPair{Tx: 666 * 1024, Rx: 666 * 1024}},
				BwTimeSlot{DayOfTheWeek: 1, HHMM: 1020, Bandwidth: BwPair{Tx: 666 * 1024, Rx: 666 * 1024}},
				BwTimeSlot{DayOfTheWeek: 2, HHMM: 1020, Bandwidth: BwPair{Tx: 666 * 1024, Rx: 666 * 1024}},
				BwTimeSlot{DayOfTheWeek: 3, HHMM: 1020, Bandwidth: BwPair{Tx: 666 * 1024, Rx: 666 * 1024}},
				BwTimeSlot{DayOfTheWeek: 4, HHMM: 1020, Bandwidth: BwPair{Tx: 666 * 1024, Rx: 666 * 1024}},
				BwTimeSlot{DayOfTheWeek: 5, HHMM: 1020, Bandwidth: BwPair{Tx: 666 * 1024, Rx: 666 * 1024}},
				BwTimeSlot{DayOfTheWeek: 6, HHMM: 1020, Bandwidth: BwPair{Tx: 666 * 1024, Rx: 666 * 1024}},
			},
			false,
		},
	} {
		var bwt BwTimetable
		err := json.Unmarshal([]byte(test.in), &bwt)
		if test.err {
			require.Error(t, err, test.in)
		} else {
			require.NoError(t, err, test.in)
		}
		assert.Equal(t, test.want, bwt)
	}
}

func TestBwTimetableMarshalJSON(t *testing.T) {
	for _, test := range []struct {
		in   BwTimetable
		want string
	}{
		{
			BwTimetable{
				BwTimeSlot{DayOfTheWeek: 0, HHMM: 0, Bandwidth: BwPair{Tx: 0, Rx: 0}},
			},
			`"0"`,
		},
		{
			BwTimetable{
				BwTimeSlot{DayOfTheWeek: 0, HHMM: 0, Bandwidth: BwPair{Tx: 666 * 1024, Rx: 666 * 1024}},
			},
			`"666Ki"`,
		},
		{
			BwTimetable{
				BwTimeSlot{DayOfTheWeek: 0, HHMM: 0, Bandwidth: BwPair{Tx: 666 * 1024, Rx: 333 * 1024}},
			},
			`"666Ki:333Ki"`,
		},
		{
			BwTimetable{
				BwTimeSlot{DayOfTheWeek: 0, HHMM: 1020, Bandwidth: BwPair{Tx: 666 * 1024, Rx: 666 * 1024}},
				BwTimeSlot{DayOfTheWeek: 1, HHMM: 1020, Bandwidth: BwPair{Tx: 666 * 1024, Rx: 666 * 1024}},
				BwTimeSlot{DayOfTheWeek: 2, HHMM: 1020, Bandwidth: BwPair{Tx: 666 * 1024, Rx: 666 * 1024}},
				BwTimeSlot{DayOfTheWeek: 3, HHMM: 1020, Bandwidth: BwPair{Tx: 666 * 1024, Rx: 666 * 1024}},
				BwTimeSlot{DayOfTheWeek: 4, HHMM: 1020, Bandwidth: BwPair{Tx: 666 * 1024, Rx: 666 * 1024}},
				BwTimeSlot{DayOfTheWeek: 5, HHMM: 1020, Bandwidth: BwPair{Tx: 666 * 1024, Rx: 666 * 1024}},
				BwTimeSlot{DayOfTheWeek: 6, HHMM: 1020, Bandwidth: BwPair{Tx: 666 * 1024, Rx: 666 * 1024}},
			},
			`"Sun-10:20,666Ki Mon-10:20,666Ki Tue-10:20,666Ki Wed-10:20,666Ki Thu-10:20,666Ki Fri-10:20,666Ki Sat-10:20,666Ki"`,
		},
	} {
		got, err := json.Marshal(test.in)
		require.NoError(t, err, test.want)
		assert.Equal(t, test.want, string(got))
	}
}
