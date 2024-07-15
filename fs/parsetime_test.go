package fs

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Check it satisfies the interfaces
var (
	_ Flagger   = (*Time)(nil)
	_ FlaggerNP = Time{}
)

func TestParseTime(t *testing.T) {
	now := time.Date(2020, 9, 5, 8, 15, 5, 250, time.UTC)
	oldTimeNowFunc := timeNowFunc
	timeNowFunc = func() time.Time { return now }
	defer func() { timeNowFunc = oldTimeNowFunc }()

	for _, test := range []struct {
		in   string
		want time.Time
		err  bool
	}{
		{"", time.Time{}, true},
		{"1ms", now.Add(-time.Millisecond), false},
		{"1s", now.Add(-time.Second), false},
		{"1", now.Add(-time.Second), false},
		{"1m", now.Add(-time.Minute), false},
		{"1.5m", now.Add(-(3 * time.Minute) / 2), false},
		{"1h", now.Add(-time.Hour), false},
		{"1d", now.Add(-time.Hour * 24), false},
		{"1w", now.Add(-time.Hour * 24 * 7), false},
		{"1M", now.Add(-time.Hour * 24 * 30), false},
		{"1y", now.Add(-time.Hour * 24 * 365), false},
		{"1.5y", now.Add(-time.Hour * 24 * 365 * 3 / 2), false},
		{"-1.5y", now.Add(time.Hour * 24 * 365 * 3 / 2), false},
		{"-1s", now.Add(time.Second), false},
		{"-1", now.Add(time.Second), false},
		{"0", now, false},
		{"100", now.Add(-100 * time.Second), false},
		{"-100", now.Add(100 * time.Second), false},
		{"1.s", now.Add(-time.Second), false},
		{"1x", time.Time{}, true},
		{"-1x", time.Time{}, true},
		{"off", time.Time{}, false},
		{"1h2m3s", now.Add(-(time.Hour + 2*time.Minute + 3*time.Second)), false},
		{"2001-02-03", time.Date(2001, 2, 3, 0, 0, 0, 0, time.Local), false},
		{"2001-02-03 10:11:12", time.Date(2001, 2, 3, 10, 11, 12, 0, time.Local), false},
		{"2001-08-03 10:11:12", time.Date(2001, 8, 3, 10, 11, 12, 0, time.Local), false},
		{"2001-02-03T10:11:12", time.Date(2001, 2, 3, 10, 11, 12, 0, time.Local), false},
		{"2001-02-03T10:11:12.123Z", time.Date(2001, 2, 3, 10, 11, 12, 123000000, time.UTC), false},
		{"2001-02-03T10:11:12.123+00:00", time.Date(2001, 2, 3, 10, 11, 12, 123000000, time.UTC), false},
	} {
		parsedTime, err := ParseTime(test.in)
		if test.err {
			require.Error(t, err, test.in)
		} else {
			require.NoError(t, err, test.in)
		}
		assert.True(t, test.want.Equal(parsedTime), "%v should be parsed as %v instead of %v", test.in, test.want, parsedTime)
	}
}

func TestTimeString(t *testing.T) {
	now := time.Date(2020, 9, 5, 8, 15, 5, 250, time.UTC)
	oldTimeNowFunc := timeNowFunc
	timeNowFunc = func() time.Time { return now }
	defer func() { timeNowFunc = oldTimeNowFunc }()

	for _, test := range []struct {
		in   time.Time
		want string
	}{
		{now, "2020-09-05T08:15:05.00000025Z"},
		{time.Date(2021, 8, 5, 8, 15, 5, 0, time.UTC), "2021-08-05T08:15:05Z"},
		{time.Time{}, "off"},
	} {
		got := Time(test.in).String()
		assert.Equal(t, test.want, got)
		// Test the reverse
		reverse, err := ParseTime(test.want)
		assert.NoError(t, err)
		assert.Equal(t, test.in, reverse)
	}
}

func TestTimeScan(t *testing.T) {
	now := time.Date(2020, 9, 5, 8, 15, 5, 250, time.UTC)
	oldTimeNowFunc := timeNowFunc
	timeNowFunc = func() time.Time { return now }
	defer func() { timeNowFunc = oldTimeNowFunc }()

	for _, test := range []struct {
		in   string
		want Time
	}{
		{"17m", Time(now.Add(-17 * time.Minute))},
		{"-12h", Time(now.Add(12 * time.Hour))},
		{"0", Time(now)},
		{"off", Time(time.Time{})},
		{"2022-03-26T17:48:19Z", Time(time.Date(2022, 03, 26, 17, 48, 19, 0, time.UTC))},
		{"2022-03-26 17:48:19", Time(time.Date(2022, 03, 26, 17, 48, 19, 0, time.Local))},
	} {
		var got Time
		n, err := fmt.Sscan(test.in, &got)
		require.NoError(t, err)
		assert.Equal(t, 1, n)
		assert.Equal(t, test.want, got)
	}
}

func TestParseTimeUnmarshalJSON(t *testing.T) {
	now := time.Date(2020, 9, 5, 8, 15, 5, 250, time.UTC)
	oldTimeNowFunc := timeNowFunc
	timeNowFunc = func() time.Time { return now }
	defer func() { timeNowFunc = oldTimeNowFunc }()

	for _, test := range []struct {
		in   string
		want time.Time
		err  bool
	}{
		{`""`, time.Time{}, true},
		{"0", time.Time{}, true},
		{"1", time.Time{}, true},
		{"1", time.Time{}, true},
		{`"2022-03-26T17:48:19Z"`, time.Date(2022, 03, 26, 17, 48, 19, 0, time.UTC), false},
		{`"0"`, now, false},
		{`"1ms"`, now.Add(-time.Millisecond), false},
		{`"1s"`, now.Add(-time.Second), false},
		{`"1"`, now.Add(-time.Second), false},
		{`"1m"`, now.Add(-time.Minute), false},
		{`"1h"`, now.Add(-time.Hour), false},
		{`"-1h"`, now.Add(time.Hour), false},
		{`"1d"`, now.Add(-time.Hour * 24), false},
		{`"1w"`, now.Add(-time.Hour * 24 * 7), false},
		{`"1M"`, now.Add(-time.Hour * 24 * 30), false},
		{`"1y"`, now.Add(-time.Hour * 24 * 365), false},
		{`"off"`, time.Time{}, false},
		{`"error"`, time.Time{}, true},
		{"error", time.Time{}, true},
	} {
		var parsedTime Time
		err := json.Unmarshal([]byte(test.in), &parsedTime)
		if test.err {
			require.Error(t, err, test.in)
		} else {
			require.NoError(t, err, test.in)
		}
		assert.Equal(t, Time(test.want), parsedTime, test.in)
	}
}

func TestParseTimeMarshalJSON(t *testing.T) {
	for _, test := range []struct {
		in   time.Time
		want string
		err  bool
	}{
		{time.Time{}, `"0001-01-01T00:00:00Z"`, false},
		{time.Date(2022, 03, 26, 17, 48, 19, 0, time.UTC), `"2022-03-26T17:48:19Z"`, false},
	} {
		gotBytes, err := json.Marshal(test.in)
		got := string(gotBytes)
		if test.err {
			require.Error(t, err, test.in)
		} else {
			require.NoError(t, err, test.in)
		}
		assert.Equal(t, test.want, got, test.in)
	}
}
