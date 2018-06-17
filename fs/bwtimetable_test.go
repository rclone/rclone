package fs

import (
	"testing"
	"time"

	"github.com/spf13/pflag"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Check it satisfies the interface
var _ pflag.Value = (*BwTimetable)(nil)

func TestBwTimetableSet(t *testing.T) {
	for _, test := range []struct {
		in   string
		want BwTimetable
		err  bool
	}{
		{"", BwTimetable{}, true},
		{"bad,bad", BwTimetable{}, true},
		{"bad bad", BwTimetable{}, true},
		{"bad", BwTimetable{}, true},
		{"1000X", BwTimetable{}, true},
		{"2401,666", BwTimetable{}, true},
		{"1061,666", BwTimetable{}, true},
		{"bad-10:20,666", BwTimetable{}, true},
		{"Mon-bad,666", BwTimetable{}, true},
		{"Mon-10:20,bad", BwTimetable{}, true},
		{
			"0",
			BwTimetable{
				BwTimeSlot{DayOfTheWeek: 0, HHMM: 0, Bandwidth: 0},
			},
			false,
		},
		{
			"666",
			BwTimetable{
				BwTimeSlot{DayOfTheWeek: 0, HHMM: 0, Bandwidth: 666 * 1024},
			},
			false,
		},
		{
			"10:20,666",
			BwTimetable{
				BwTimeSlot{DayOfTheWeek: 0, HHMM: 1020, Bandwidth: 666 * 1024},
				BwTimeSlot{DayOfTheWeek: 1, HHMM: 1020, Bandwidth: 666 * 1024},
				BwTimeSlot{DayOfTheWeek: 2, HHMM: 1020, Bandwidth: 666 * 1024},
				BwTimeSlot{DayOfTheWeek: 3, HHMM: 1020, Bandwidth: 666 * 1024},
				BwTimeSlot{DayOfTheWeek: 4, HHMM: 1020, Bandwidth: 666 * 1024},
				BwTimeSlot{DayOfTheWeek: 5, HHMM: 1020, Bandwidth: 666 * 1024},
				BwTimeSlot{DayOfTheWeek: 6, HHMM: 1020, Bandwidth: 666 * 1024},
			},
			false,
		},
		{
			"11:00,333 13:40,666 23:50,10M 23:59,off",
			BwTimetable{
				BwTimeSlot{DayOfTheWeek: 0, HHMM: 1100, Bandwidth: 333 * 1024},
				BwTimeSlot{DayOfTheWeek: 1, HHMM: 1100, Bandwidth: 333 * 1024},
				BwTimeSlot{DayOfTheWeek: 2, HHMM: 1100, Bandwidth: 333 * 1024},
				BwTimeSlot{DayOfTheWeek: 3, HHMM: 1100, Bandwidth: 333 * 1024},
				BwTimeSlot{DayOfTheWeek: 4, HHMM: 1100, Bandwidth: 333 * 1024},
				BwTimeSlot{DayOfTheWeek: 5, HHMM: 1100, Bandwidth: 333 * 1024},
				BwTimeSlot{DayOfTheWeek: 6, HHMM: 1100, Bandwidth: 333 * 1024},
				BwTimeSlot{DayOfTheWeek: 0, HHMM: 1340, Bandwidth: 666 * 1024},
				BwTimeSlot{DayOfTheWeek: 1, HHMM: 1340, Bandwidth: 666 * 1024},
				BwTimeSlot{DayOfTheWeek: 2, HHMM: 1340, Bandwidth: 666 * 1024},
				BwTimeSlot{DayOfTheWeek: 3, HHMM: 1340, Bandwidth: 666 * 1024},
				BwTimeSlot{DayOfTheWeek: 4, HHMM: 1340, Bandwidth: 666 * 1024},
				BwTimeSlot{DayOfTheWeek: 5, HHMM: 1340, Bandwidth: 666 * 1024},
				BwTimeSlot{DayOfTheWeek: 6, HHMM: 1340, Bandwidth: 666 * 1024},
				BwTimeSlot{DayOfTheWeek: 0, HHMM: 2350, Bandwidth: 10 * 1024 * 1024},
				BwTimeSlot{DayOfTheWeek: 1, HHMM: 2350, Bandwidth: 10 * 1024 * 1024},
				BwTimeSlot{DayOfTheWeek: 2, HHMM: 2350, Bandwidth: 10 * 1024 * 1024},
				BwTimeSlot{DayOfTheWeek: 3, HHMM: 2350, Bandwidth: 10 * 1024 * 1024},
				BwTimeSlot{DayOfTheWeek: 4, HHMM: 2350, Bandwidth: 10 * 1024 * 1024},
				BwTimeSlot{DayOfTheWeek: 5, HHMM: 2350, Bandwidth: 10 * 1024 * 1024},
				BwTimeSlot{DayOfTheWeek: 6, HHMM: 2350, Bandwidth: 10 * 1024 * 1024},
				BwTimeSlot{DayOfTheWeek: 0, HHMM: 2359, Bandwidth: -1},
				BwTimeSlot{DayOfTheWeek: 1, HHMM: 2359, Bandwidth: -1},
				BwTimeSlot{DayOfTheWeek: 2, HHMM: 2359, Bandwidth: -1},
				BwTimeSlot{DayOfTheWeek: 3, HHMM: 2359, Bandwidth: -1},
				BwTimeSlot{DayOfTheWeek: 4, HHMM: 2359, Bandwidth: -1},
				BwTimeSlot{DayOfTheWeek: 5, HHMM: 2359, Bandwidth: -1},
				BwTimeSlot{DayOfTheWeek: 6, HHMM: 2359, Bandwidth: -1},
			},
			false,
		},
		{
			"Mon-11:00,333 Tue-13:40,666 Fri-00:00,10M Sat-10:00,off Sun-23:00,666",
			BwTimetable{
				BwTimeSlot{DayOfTheWeek: 1, HHMM: 1100, Bandwidth: 333 * 1024},
				BwTimeSlot{DayOfTheWeek: 2, HHMM: 1340, Bandwidth: 666 * 1024},
				BwTimeSlot{DayOfTheWeek: 5, HHMM: 0000, Bandwidth: 10 * 1024 * 1024},
				BwTimeSlot{DayOfTheWeek: 6, HHMM: 1000, Bandwidth: -1},
				BwTimeSlot{DayOfTheWeek: 0, HHMM: 2300, Bandwidth: 666 * 1024},
			},
			false,
		},
		{
			"Mon-11:00,333 Tue-13:40,666 Fri-00:00,10M 00:01,off Sun-23:00,666",
			BwTimetable{
				BwTimeSlot{DayOfTheWeek: 1, HHMM: 1100, Bandwidth: 333 * 1024},
				BwTimeSlot{DayOfTheWeek: 2, HHMM: 1340, Bandwidth: 666 * 1024},
				BwTimeSlot{DayOfTheWeek: 5, HHMM: 0000, Bandwidth: 10 * 1024 * 1024},
				BwTimeSlot{DayOfTheWeek: 0, HHMM: 1, Bandwidth: -1},
				BwTimeSlot{DayOfTheWeek: 1, HHMM: 1, Bandwidth: -1},
				BwTimeSlot{DayOfTheWeek: 2, HHMM: 1, Bandwidth: -1},
				BwTimeSlot{DayOfTheWeek: 3, HHMM: 1, Bandwidth: -1},
				BwTimeSlot{DayOfTheWeek: 4, HHMM: 1, Bandwidth: -1},
				BwTimeSlot{DayOfTheWeek: 5, HHMM: 1, Bandwidth: -1},
				BwTimeSlot{DayOfTheWeek: 6, HHMM: 1, Bandwidth: -1},
				BwTimeSlot{DayOfTheWeek: 0, HHMM: 2300, Bandwidth: 666 * 1024},
			},
			false,
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
			BwTimeSlot{DayOfTheWeek: 0, HHMM: 0, Bandwidth: -1},
		},
		{
			BwTimetable{
				BwTimeSlot{DayOfTheWeek: 0, HHMM: 1100, Bandwidth: 333 * 1024},
				BwTimeSlot{DayOfTheWeek: 1, HHMM: 1100, Bandwidth: 333 * 1024},
				BwTimeSlot{DayOfTheWeek: 2, HHMM: 1100, Bandwidth: 333 * 1024},
				BwTimeSlot{DayOfTheWeek: 3, HHMM: 1100, Bandwidth: 333 * 1024},
				BwTimeSlot{DayOfTheWeek: 4, HHMM: 1100, Bandwidth: 333 * 1024},
				BwTimeSlot{DayOfTheWeek: 5, HHMM: 1100, Bandwidth: 333 * 1024},
				BwTimeSlot{DayOfTheWeek: 6, HHMM: 1100, Bandwidth: 333 * 1024},
			},
			time.Date(2017, time.April, 20, 15, 0, 0, 0, time.UTC),
			BwTimeSlot{DayOfTheWeek: 4, HHMM: 1100, Bandwidth: 333 * 1024},
		},
		{
			BwTimetable{
				BwTimeSlot{DayOfTheWeek: 0, HHMM: 1100, Bandwidth: 333 * 1024},
				BwTimeSlot{DayOfTheWeek: 1, HHMM: 1100, Bandwidth: 333 * 1024},
				BwTimeSlot{DayOfTheWeek: 2, HHMM: 1100, Bandwidth: 333 * 1024},
				BwTimeSlot{DayOfTheWeek: 3, HHMM: 1100, Bandwidth: 333 * 1024},
				BwTimeSlot{DayOfTheWeek: 4, HHMM: 1100, Bandwidth: 333 * 1024},
				BwTimeSlot{DayOfTheWeek: 5, HHMM: 1100, Bandwidth: 333 * 1024},
				BwTimeSlot{DayOfTheWeek: 6, HHMM: 1100, Bandwidth: 333 * 1024},
				BwTimeSlot{DayOfTheWeek: 0, HHMM: 1300, Bandwidth: 666 * 1024},
				BwTimeSlot{DayOfTheWeek: 1, HHMM: 1300, Bandwidth: 666 * 1024},
				BwTimeSlot{DayOfTheWeek: 2, HHMM: 1300, Bandwidth: 666 * 1024},
				BwTimeSlot{DayOfTheWeek: 3, HHMM: 1300, Bandwidth: 666 * 1024},
				BwTimeSlot{DayOfTheWeek: 4, HHMM: 1300, Bandwidth: 666 * 1024},
				BwTimeSlot{DayOfTheWeek: 5, HHMM: 1300, Bandwidth: 666 * 1024},
				BwTimeSlot{DayOfTheWeek: 6, HHMM: 1300, Bandwidth: 666 * 1024},
				BwTimeSlot{DayOfTheWeek: 0, HHMM: 2301, Bandwidth: 1024 * 1024},
				BwTimeSlot{DayOfTheWeek: 1, HHMM: 2301, Bandwidth: 1024 * 1024},
				BwTimeSlot{DayOfTheWeek: 2, HHMM: 2301, Bandwidth: 1024 * 1024},
				BwTimeSlot{DayOfTheWeek: 3, HHMM: 2301, Bandwidth: 1024 * 1024},
				BwTimeSlot{DayOfTheWeek: 4, HHMM: 2301, Bandwidth: 1024 * 1024},
				BwTimeSlot{DayOfTheWeek: 5, HHMM: 2301, Bandwidth: 1024 * 1024},
				BwTimeSlot{DayOfTheWeek: 6, HHMM: 2301, Bandwidth: 1024 * 1024},
				BwTimeSlot{DayOfTheWeek: 0, HHMM: 2350, Bandwidth: -1},
				BwTimeSlot{DayOfTheWeek: 1, HHMM: 2350, Bandwidth: -1},
				BwTimeSlot{DayOfTheWeek: 2, HHMM: 2350, Bandwidth: -1},
				BwTimeSlot{DayOfTheWeek: 3, HHMM: 2350, Bandwidth: -1},
				BwTimeSlot{DayOfTheWeek: 4, HHMM: 2350, Bandwidth: -1},
				BwTimeSlot{DayOfTheWeek: 5, HHMM: 2350, Bandwidth: -1},
				BwTimeSlot{DayOfTheWeek: 6, HHMM: 2350, Bandwidth: -1},
			},
			time.Date(2017, time.April, 20, 10, 15, 0, 0, time.UTC),
			BwTimeSlot{DayOfTheWeek: 3, HHMM: 2350, Bandwidth: -1},
		},
		{
			BwTimetable{
				BwTimeSlot{DayOfTheWeek: 0, HHMM: 1100, Bandwidth: 333 * 1024},
				BwTimeSlot{DayOfTheWeek: 1, HHMM: 1100, Bandwidth: 333 * 1024},
				BwTimeSlot{DayOfTheWeek: 2, HHMM: 1100, Bandwidth: 333 * 1024},
				BwTimeSlot{DayOfTheWeek: 3, HHMM: 1100, Bandwidth: 333 * 1024},
				BwTimeSlot{DayOfTheWeek: 4, HHMM: 1100, Bandwidth: 333 * 1024},
				BwTimeSlot{DayOfTheWeek: 5, HHMM: 1100, Bandwidth: 333 * 1024},
				BwTimeSlot{DayOfTheWeek: 6, HHMM: 1100, Bandwidth: 333 * 1024},
				BwTimeSlot{DayOfTheWeek: 0, HHMM: 1300, Bandwidth: 666 * 1024},
				BwTimeSlot{DayOfTheWeek: 1, HHMM: 1300, Bandwidth: 666 * 1024},
				BwTimeSlot{DayOfTheWeek: 2, HHMM: 1300, Bandwidth: 666 * 1024},
				BwTimeSlot{DayOfTheWeek: 3, HHMM: 1300, Bandwidth: 666 * 1024},
				BwTimeSlot{DayOfTheWeek: 4, HHMM: 1300, Bandwidth: 666 * 1024},
				BwTimeSlot{DayOfTheWeek: 5, HHMM: 1300, Bandwidth: 666 * 1024},
				BwTimeSlot{DayOfTheWeek: 6, HHMM: 1300, Bandwidth: 666 * 1024},
				BwTimeSlot{DayOfTheWeek: 0, HHMM: 2301, Bandwidth: 1024 * 1024},
				BwTimeSlot{DayOfTheWeek: 1, HHMM: 2301, Bandwidth: 1024 * 1024},
				BwTimeSlot{DayOfTheWeek: 2, HHMM: 2301, Bandwidth: 1024 * 1024},
				BwTimeSlot{DayOfTheWeek: 3, HHMM: 2301, Bandwidth: 1024 * 1024},
				BwTimeSlot{DayOfTheWeek: 4, HHMM: 2301, Bandwidth: 1024 * 1024},
				BwTimeSlot{DayOfTheWeek: 5, HHMM: 2301, Bandwidth: 1024 * 1024},
				BwTimeSlot{DayOfTheWeek: 6, HHMM: 2301, Bandwidth: 1024 * 1024},
				BwTimeSlot{DayOfTheWeek: 0, HHMM: 2350, Bandwidth: -1},
				BwTimeSlot{DayOfTheWeek: 1, HHMM: 2350, Bandwidth: -1},
				BwTimeSlot{DayOfTheWeek: 2, HHMM: 2350, Bandwidth: -1},
				BwTimeSlot{DayOfTheWeek: 3, HHMM: 2350, Bandwidth: -1},
				BwTimeSlot{DayOfTheWeek: 4, HHMM: 2350, Bandwidth: -1},
				BwTimeSlot{DayOfTheWeek: 5, HHMM: 2350, Bandwidth: -1},
				BwTimeSlot{DayOfTheWeek: 6, HHMM: 2350, Bandwidth: -1},
			},
			time.Date(2017, time.April, 20, 11, 0, 0, 0, time.UTC),
			BwTimeSlot{DayOfTheWeek: 4, HHMM: 1100, Bandwidth: 333 * 1024},
		},
		{
			BwTimetable{
				BwTimeSlot{DayOfTheWeek: 0, HHMM: 1100, Bandwidth: 333 * 1024},
				BwTimeSlot{DayOfTheWeek: 1, HHMM: 1100, Bandwidth: 333 * 1024},
				BwTimeSlot{DayOfTheWeek: 2, HHMM: 1100, Bandwidth: 333 * 1024},
				BwTimeSlot{DayOfTheWeek: 3, HHMM: 1100, Bandwidth: 333 * 1024},
				BwTimeSlot{DayOfTheWeek: 4, HHMM: 1100, Bandwidth: 333 * 1024},
				BwTimeSlot{DayOfTheWeek: 5, HHMM: 1100, Bandwidth: 333 * 1024},
				BwTimeSlot{DayOfTheWeek: 6, HHMM: 1100, Bandwidth: 333 * 1024},
				BwTimeSlot{DayOfTheWeek: 0, HHMM: 1300, Bandwidth: 666 * 1024},
				BwTimeSlot{DayOfTheWeek: 1, HHMM: 1300, Bandwidth: 666 * 1024},
				BwTimeSlot{DayOfTheWeek: 2, HHMM: 1300, Bandwidth: 666 * 1024},
				BwTimeSlot{DayOfTheWeek: 3, HHMM: 1300, Bandwidth: 666 * 1024},
				BwTimeSlot{DayOfTheWeek: 4, HHMM: 1300, Bandwidth: 666 * 1024},
				BwTimeSlot{DayOfTheWeek: 5, HHMM: 1300, Bandwidth: 666 * 1024},
				BwTimeSlot{DayOfTheWeek: 6, HHMM: 1300, Bandwidth: 666 * 1024},
				BwTimeSlot{DayOfTheWeek: 0, HHMM: 2301, Bandwidth: 1024 * 1024},
				BwTimeSlot{DayOfTheWeek: 1, HHMM: 2301, Bandwidth: 1024 * 1024},
				BwTimeSlot{DayOfTheWeek: 2, HHMM: 2301, Bandwidth: 1024 * 1024},
				BwTimeSlot{DayOfTheWeek: 3, HHMM: 2301, Bandwidth: 1024 * 1024},
				BwTimeSlot{DayOfTheWeek: 4, HHMM: 2301, Bandwidth: 1024 * 1024},
				BwTimeSlot{DayOfTheWeek: 5, HHMM: 2301, Bandwidth: 1024 * 1024},
				BwTimeSlot{DayOfTheWeek: 6, HHMM: 2301, Bandwidth: 1024 * 1024},
				BwTimeSlot{DayOfTheWeek: 0, HHMM: 2350, Bandwidth: -1},
				BwTimeSlot{DayOfTheWeek: 1, HHMM: 2350, Bandwidth: -1},
				BwTimeSlot{DayOfTheWeek: 2, HHMM: 2350, Bandwidth: -1},
				BwTimeSlot{DayOfTheWeek: 3, HHMM: 2350, Bandwidth: -1},
				BwTimeSlot{DayOfTheWeek: 4, HHMM: 2350, Bandwidth: -1},
				BwTimeSlot{DayOfTheWeek: 5, HHMM: 2350, Bandwidth: -1},
				BwTimeSlot{DayOfTheWeek: 6, HHMM: 2350, Bandwidth: -1},
			},
			time.Date(2017, time.April, 20, 13, 1, 0, 0, time.UTC),
			BwTimeSlot{DayOfTheWeek: 4, HHMM: 1300, Bandwidth: 666 * 1024},
		},
		{
			BwTimetable{
				BwTimeSlot{DayOfTheWeek: 0, HHMM: 1100, Bandwidth: 333 * 1024},
				BwTimeSlot{DayOfTheWeek: 1, HHMM: 1100, Bandwidth: 333 * 1024},
				BwTimeSlot{DayOfTheWeek: 2, HHMM: 1100, Bandwidth: 333 * 1024},
				BwTimeSlot{DayOfTheWeek: 3, HHMM: 1100, Bandwidth: 333 * 1024},
				BwTimeSlot{DayOfTheWeek: 4, HHMM: 1100, Bandwidth: 333 * 1024},
				BwTimeSlot{DayOfTheWeek: 5, HHMM: 1100, Bandwidth: 333 * 1024},
				BwTimeSlot{DayOfTheWeek: 6, HHMM: 1100, Bandwidth: 333 * 1024},
				BwTimeSlot{DayOfTheWeek: 0, HHMM: 1300, Bandwidth: 666 * 1024},
				BwTimeSlot{DayOfTheWeek: 1, HHMM: 1300, Bandwidth: 666 * 1024},
				BwTimeSlot{DayOfTheWeek: 2, HHMM: 1300, Bandwidth: 666 * 1024},
				BwTimeSlot{DayOfTheWeek: 3, HHMM: 1300, Bandwidth: 666 * 1024},
				BwTimeSlot{DayOfTheWeek: 4, HHMM: 1300, Bandwidth: 666 * 1024},
				BwTimeSlot{DayOfTheWeek: 5, HHMM: 1300, Bandwidth: 666 * 1024},
				BwTimeSlot{DayOfTheWeek: 6, HHMM: 1300, Bandwidth: 666 * 1024},
				BwTimeSlot{DayOfTheWeek: 0, HHMM: 2301, Bandwidth: 1024 * 1024},
				BwTimeSlot{DayOfTheWeek: 1, HHMM: 2301, Bandwidth: 1024 * 1024},
				BwTimeSlot{DayOfTheWeek: 2, HHMM: 2301, Bandwidth: 1024 * 1024},
				BwTimeSlot{DayOfTheWeek: 3, HHMM: 2301, Bandwidth: 1024 * 1024},
				BwTimeSlot{DayOfTheWeek: 4, HHMM: 2301, Bandwidth: 1024 * 1024},
				BwTimeSlot{DayOfTheWeek: 5, HHMM: 2301, Bandwidth: 1024 * 1024},
				BwTimeSlot{DayOfTheWeek: 6, HHMM: 2301, Bandwidth: 1024 * 1024},
				BwTimeSlot{DayOfTheWeek: 0, HHMM: 2350, Bandwidth: -1},
				BwTimeSlot{DayOfTheWeek: 1, HHMM: 2350, Bandwidth: -1},
				BwTimeSlot{DayOfTheWeek: 2, HHMM: 2350, Bandwidth: -1},
				BwTimeSlot{DayOfTheWeek: 3, HHMM: 2350, Bandwidth: -1},
				BwTimeSlot{DayOfTheWeek: 4, HHMM: 2350, Bandwidth: -1},
				BwTimeSlot{DayOfTheWeek: 5, HHMM: 2350, Bandwidth: -1},
				BwTimeSlot{DayOfTheWeek: 6, HHMM: 2350, Bandwidth: -1},
			},
			time.Date(2017, time.April, 20, 23, 59, 0, 0, time.UTC),
			BwTimeSlot{DayOfTheWeek: 4, HHMM: 2350, Bandwidth: -1},
		},
		{
			BwTimetable{
				BwTimeSlot{DayOfTheWeek: 1, HHMM: 1100, Bandwidth: 333 * 1024},
				BwTimeSlot{DayOfTheWeek: 2, HHMM: 1340, Bandwidth: 666 * 1024},
				BwTimeSlot{DayOfTheWeek: 5, HHMM: 0000, Bandwidth: 10 * 1024 * 1024},
				BwTimeSlot{DayOfTheWeek: 6, HHMM: 1000, Bandwidth: -1},
				BwTimeSlot{DayOfTheWeek: 0, HHMM: 2300, Bandwidth: 666 * 1024},
			},
			time.Date(2017, time.April, 20, 23, 59, 0, 0, time.UTC),
			BwTimeSlot{DayOfTheWeek: 2, HHMM: 1340, Bandwidth: 666 * 1024},
		},
		{
			BwTimetable{
				BwTimeSlot{DayOfTheWeek: 1, HHMM: 1100, Bandwidth: 333 * 1024},
				BwTimeSlot{DayOfTheWeek: 2, HHMM: 1340, Bandwidth: 666 * 1024},
				BwTimeSlot{DayOfTheWeek: 5, HHMM: 0000, Bandwidth: 10 * 1024 * 1024},
				BwTimeSlot{DayOfTheWeek: 6, HHMM: 1000, Bandwidth: -1},
				BwTimeSlot{DayOfTheWeek: 0, HHMM: 2300, Bandwidth: 666 * 1024},
			},
			time.Date(2017, time.April, 21, 23, 59, 0, 0, time.UTC),
			BwTimeSlot{DayOfTheWeek: 5, HHMM: 0000, Bandwidth: 10 * 1024 * 1024},
		},
		{
			BwTimetable{
				BwTimeSlot{DayOfTheWeek: 1, HHMM: 1100, Bandwidth: 333 * 1024},
				BwTimeSlot{DayOfTheWeek: 2, HHMM: 1340, Bandwidth: 666 * 1024},
				BwTimeSlot{DayOfTheWeek: 5, HHMM: 0000, Bandwidth: 10 * 1024 * 1024},
				BwTimeSlot{DayOfTheWeek: 6, HHMM: 1000, Bandwidth: -1},
				BwTimeSlot{DayOfTheWeek: 0, HHMM: 2300, Bandwidth: 666 * 1024},
			},
			time.Date(2017, time.April, 17, 10, 59, 0, 0, time.UTC),
			BwTimeSlot{DayOfTheWeek: 0, HHMM: 2300, Bandwidth: 666 * 1024},
		},
	} {
		slot := test.tt.LimitAt(test.now)
		assert.Equal(t, test.want, slot)
	}
}
