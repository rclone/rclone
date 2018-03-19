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
		{"0", BwTimetable{BwTimeSlot{HHMM: 0, Bandwidth: 0}}, false},
		{"666", BwTimetable{BwTimeSlot{HHMM: 0, Bandwidth: 666 * 1024}}, false},
		{"10:20,666", BwTimetable{BwTimeSlot{HHMM: 1020, Bandwidth: 666 * 1024}}, false},
		{
			"11:00,333 13:40,666 23:50,10M 23:59,off",
			BwTimetable{
				BwTimeSlot{HHMM: 1100, Bandwidth: 333 * 1024},
				BwTimeSlot{HHMM: 1340, Bandwidth: 666 * 1024},
				BwTimeSlot{HHMM: 2350, Bandwidth: 10 * 1024 * 1024},
				BwTimeSlot{HHMM: 2359, Bandwidth: -1},
			},
			false,
		},
		{"bad,bad", BwTimetable{}, true},
		{"bad bad", BwTimetable{}, true},
		{"bad", BwTimetable{}, true},
		{"1000X", BwTimetable{}, true},
		{"2401,666", BwTimetable{}, true},
		{"1061,666", BwTimetable{}, true},
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
			BwTimeSlot{HHMM: 0, Bandwidth: -1},
		},
		{
			BwTimetable{BwTimeSlot{HHMM: 1100, Bandwidth: 333 * 1024}},
			time.Date(2017, time.April, 20, 15, 0, 0, 0, time.UTC),
			BwTimeSlot{HHMM: 1100, Bandwidth: 333 * 1024},
		},
		{
			BwTimetable{
				BwTimeSlot{HHMM: 1100, Bandwidth: 333 * 1024},
				BwTimeSlot{HHMM: 1300, Bandwidth: 666 * 1024},
				BwTimeSlot{HHMM: 2301, Bandwidth: 1024 * 1024},
				BwTimeSlot{HHMM: 2350, Bandwidth: -1},
			},
			time.Date(2017, time.April, 20, 10, 15, 0, 0, time.UTC),
			BwTimeSlot{HHMM: 2350, Bandwidth: -1},
		},
		{
			BwTimetable{
				BwTimeSlot{HHMM: 1100, Bandwidth: 333 * 1024},
				BwTimeSlot{HHMM: 1300, Bandwidth: 666 * 1024},
				BwTimeSlot{HHMM: 2301, Bandwidth: 1024 * 1024},
				BwTimeSlot{HHMM: 2350, Bandwidth: -1},
			},
			time.Date(2017, time.April, 20, 11, 0, 0, 0, time.UTC),
			BwTimeSlot{HHMM: 1100, Bandwidth: 333 * 1024},
		},
		{
			BwTimetable{
				BwTimeSlot{HHMM: 1100, Bandwidth: 333 * 1024},
				BwTimeSlot{HHMM: 1300, Bandwidth: 666 * 1024},
				BwTimeSlot{HHMM: 2301, Bandwidth: 1024 * 1024},
				BwTimeSlot{HHMM: 2350, Bandwidth: -1},
			},
			time.Date(2017, time.April, 20, 13, 1, 0, 0, time.UTC),
			BwTimeSlot{HHMM: 1300, Bandwidth: 666 * 1024},
		},
		{
			BwTimetable{
				BwTimeSlot{HHMM: 1100, Bandwidth: 333 * 1024},
				BwTimeSlot{HHMM: 1300, Bandwidth: 666 * 1024},
				BwTimeSlot{HHMM: 2301, Bandwidth: 1024 * 1024},
				BwTimeSlot{HHMM: 2350, Bandwidth: -1},
			},
			time.Date(2017, time.April, 20, 23, 59, 0, 0, time.UTC),
			BwTimeSlot{HHMM: 2350, Bandwidth: -1},
		},
	} {
		slot := test.tt.LimitAt(test.now)
		assert.Equal(t, test.want, slot)
	}
}
