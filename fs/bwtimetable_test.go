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
			"666k",
		},
		{
			"666:333",
			BwTimetable{
				BwTimeSlot{DayOfTheWeek: 0, HHMM: 0, Bandwidth: BwPair{Tx: 666 * 1024, Rx: 333 * 1024}},
			},
			false,
			"666k:333k",
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
			"Sun-10:20,666k Mon-10:20,666k Tue-10:20,666k Wed-10:20,666k Thu-10:20,666k Fri-10:20,666k Sat-10:20,666k",
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
			"Sun-10:20,666k:333k Mon-10:20,666k:333k Tue-10:20,666k:333k Wed-10:20,666k:333k Thu-10:20,666k:333k Fri-10:20,666k:333k Sat-10:20,666k:333k",
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
			"Sun-11:00,333k Mon-11:00,333k Tue-11:00,333k Wed-11:00,333k Thu-11:00,333k Fri-11:00,333k Sat-11:00,333k Sun-13:40,666k Mon-13:40,666k Tue-13:40,666k Wed-13:40,666k Thu-13:40,666k Fri-13:40,666k Sat-13:40,666k Sun-23:50,10M Mon-23:50,10M Tue-23:50,10M Wed-23:50,10M Thu-23:50,10M Fri-23:50,10M Sat-23:50,10M Sun-23:59,off Mon-23:59,off Tue-23:59,off Wed-23:59,off Thu-23:59,off Fri-23:59,off Sat-23:59,off",
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
			"Sun-11:00,333k:666k Mon-11:00,333k:666k Tue-11:00,333k:666k Wed-11:00,333k:666k Thu-11:00,333k:666k Fri-11:00,333k:666k Sat-11:00,333k:666k Sun-13:40,666k:off Mon-13:40,666k:off Tue-13:40,666k:off Wed-13:40,666k:off Thu-13:40,666k:off Fri-13:40,666k:off Sat-13:40,666k:off Sun-23:50,10M:1M Mon-23:50,10M:1M Tue-23:50,10M:1M Wed-23:50,10M:1M Thu-23:50,10M:1M Fri-23:50,10M:1M Sat-23:50,10M:1M Sun-23:59,off:10M Mon-23:59,off:10M Tue-23:59,off:10M Wed-23:59,off:10M Thu-23:59,off:10M Fri-23:59,off:10M Sat-23:59,off:10M",
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
			"Mon-11:00,333k Tue-13:40,666k:333k Fri-00:00,10M Sat-10:00,off Sun-23:00,666k",
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
			"Mon-11:00,333k Tue-13:40,666k Fri-00:00,10M Sun-00:01,off Mon-00:01,off Tue-00:01,off Wed-00:01,off Thu-00:01,off Fri-00:01,off Sat-00:01,off Sun-23:00,666k:off",
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
			"Sun-08:00,512k Mon-08:00,512k Tue-08:00,512k Wed-08:00,512k Thu-08:00,512k Fri-08:00,512k Sat-08:00,512k Sun-12:00,10M Mon-12:00,10M Tue-12:00,10M Wed-12:00,10M Thu-12:00,10M Fri-12:00,10M Sat-12:00,10M Sun-13:00,512k Mon-13:00,512k Tue-13:00,512k Wed-13:00,512k Thu-13:00,512k Fri-13:00,512k Sat-13:00,512k Sun-18:00,30M Mon-18:00,30M Tue-18:00,30M Wed-18:00,30M Thu-18:00,30M Fri-18:00,30M Sat-18:00,30M Sun-23:00,off Mon-23:00,off Tue-23:00,off Wed-23:00,off Thu-23:00,off Fri-23:00,off Sat-23:00,off",
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
			"Mon-00:00,512k Fri-23:59,10M Sat-10:00,1M Sun-20:00,off",
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
			"Mon-00:00,512k Sun-12:00,1M Mon-12:00,1M Tue-12:00,1M Wed-12:00,1M Thu-12:00,1M Fri-12:00,1M Sat-12:00,1M Sun-20:00,off",
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
			`"666k"`,
		},
		{
			BwTimetable{
				BwTimeSlot{DayOfTheWeek: 0, HHMM: 0, Bandwidth: BwPair{Tx: 666 * 1024, Rx: 333 * 1024}},
			},
			`"666k:333k"`,
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
			`"Sun-10:20,666k Mon-10:20,666k Tue-10:20,666k Wed-10:20,666k Thu-10:20,666k Fri-10:20,666k Sat-10:20,666k"`,
		},
	} {
		got, err := json.Marshal(test.in)
		require.NoError(t, err, test.want)
		assert.Equal(t, test.want, string(got))
	}
}
