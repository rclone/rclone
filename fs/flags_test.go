package fs

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSizeSuffixString(t *testing.T) {
	for _, test := range []struct {
		in   float64
		want string
	}{
		{0, "0"},
		{102, "102"},
		{1024, "1k"},
		{1024 * 1024, "1M"},
		{1024 * 1024 * 1024, "1G"},
		{10 * 1024 * 1024 * 1024, "10G"},
		{10.1 * 1024 * 1024 * 1024, "10.100G"},
		{-1, "off"},
		{-100, "off"},
	} {
		ss := SizeSuffix(test.in)
		got := ss.String()
		assert.Equal(t, test.want, got)
	}
}

func TestSizeSuffixUnit(t *testing.T) {
	for _, test := range []struct {
		in   float64
		want string
	}{
		{0, "0 Bytes"},
		{102, "102 Bytes"},
		{1024, "1 kBytes"},
		{1024 * 1024, "1 MBytes"},
		{1024 * 1024 * 1024, "1 GBytes"},
		{10 * 1024 * 1024 * 1024, "10 GBytes"},
		{10.1 * 1024 * 1024 * 1024, "10.100 GBytes"},
		{-1, "off"},
		{-100, "off"},
	} {
		ss := SizeSuffix(test.in)
		got := ss.Unit("Bytes")
		assert.Equal(t, test.want, got)
	}
}

func TestSizeSuffixSet(t *testing.T) {
	for _, test := range []struct {
		in   string
		want int64
		err  bool
	}{
		{"0", 0, false},
		{"1b", 1, false},
		{"102B", 102, false},
		{"0.1k", 102, false},
		{"0.1", 102, false},
		{"1K", 1024, false},
		{"1", 1024, false},
		{"2.5", 1024 * 2.5, false},
		{"1M", 1024 * 1024, false},
		{"1.g", 1024 * 1024 * 1024, false},
		{"10G", 10 * 1024 * 1024 * 1024, false},
		{"off", -1, false},
		{"OFF", -1, false},
		{"", 0, true},
		{"1p", 0, true},
		{"1.p", 0, true},
		{"1p", 0, true},
		{"-1K", 0, true},
	} {
		ss := SizeSuffix(0)
		err := ss.Set(test.in)
		if test.err {
			require.Error(t, err)
		} else {
			require.NoError(t, err)
		}
		assert.Equal(t, test.want, int64(ss))
	}
}

func TestBwTimetableSet(t *testing.T) {
	for _, test := range []struct {
		in   string
		want BwTimetable
		err  bool
	}{
		{"", BwTimetable{}, true},
		{"0", BwTimetable{BwTimeSlot{hhmm: 0, bandwidth: 0}}, false},
		{"666", BwTimetable{BwTimeSlot{hhmm: 0, bandwidth: 666 * 1024}}, false},
		{"10:20,666", BwTimetable{BwTimeSlot{hhmm: 1020, bandwidth: 666 * 1024}}, false},
		{
			"11:00,333 13:40,666 23:50,10M 23:59,off",
			BwTimetable{
				BwTimeSlot{hhmm: 1100, bandwidth: 333 * 1024},
				BwTimeSlot{hhmm: 1340, bandwidth: 666 * 1024},
				BwTimeSlot{hhmm: 2350, bandwidth: 10 * 1024 * 1024},
				BwTimeSlot{hhmm: 2359, bandwidth: -1},
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
			BwTimeSlot{hhmm: 0, bandwidth: -1},
		},
		{
			BwTimetable{BwTimeSlot{hhmm: 1100, bandwidth: 333 * 1024}},
			time.Date(2017, time.April, 20, 15, 0, 0, 0, time.UTC),
			BwTimeSlot{hhmm: 1100, bandwidth: 333 * 1024},
		},
		{
			BwTimetable{
				BwTimeSlot{hhmm: 1100, bandwidth: 333 * 1024},
				BwTimeSlot{hhmm: 1300, bandwidth: 666 * 1024},
				BwTimeSlot{hhmm: 2301, bandwidth: 1024 * 1024},
				BwTimeSlot{hhmm: 2350, bandwidth: -1},
			},
			time.Date(2017, time.April, 20, 10, 15, 0, 0, time.UTC),
			BwTimeSlot{hhmm: 2350, bandwidth: -1},
		},
		{
			BwTimetable{
				BwTimeSlot{hhmm: 1100, bandwidth: 333 * 1024},
				BwTimeSlot{hhmm: 1300, bandwidth: 666 * 1024},
				BwTimeSlot{hhmm: 2301, bandwidth: 1024 * 1024},
				BwTimeSlot{hhmm: 2350, bandwidth: -1},
			},
			time.Date(2017, time.April, 20, 11, 0, 0, 0, time.UTC),
			BwTimeSlot{hhmm: 1100, bandwidth: 333 * 1024},
		},
		{
			BwTimetable{
				BwTimeSlot{hhmm: 1100, bandwidth: 333 * 1024},
				BwTimeSlot{hhmm: 1300, bandwidth: 666 * 1024},
				BwTimeSlot{hhmm: 2301, bandwidth: 1024 * 1024},
				BwTimeSlot{hhmm: 2350, bandwidth: -1},
			},
			time.Date(2017, time.April, 20, 13, 1, 0, 0, time.UTC),
			BwTimeSlot{hhmm: 1300, bandwidth: 666 * 1024},
		},
		{
			BwTimetable{
				BwTimeSlot{hhmm: 1100, bandwidth: 333 * 1024},
				BwTimeSlot{hhmm: 1300, bandwidth: 666 * 1024},
				BwTimeSlot{hhmm: 2301, bandwidth: 1024 * 1024},
				BwTimeSlot{hhmm: 2350, bandwidth: -1},
			},
			time.Date(2017, time.April, 20, 23, 59, 0, 0, time.UTC),
			BwTimeSlot{hhmm: 2350, bandwidth: -1},
		},
	} {
		slot := test.tt.LimitAt(test.now)
		assert.Equal(t, test.want, slot)
	}
}
