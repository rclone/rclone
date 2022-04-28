package fs

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Check it satisfies the interface
var _ flagger = (*Duration)(nil)

func TestParseDuration(t *testing.T) {
	now := time.Date(2020, 9, 5, 8, 15, 5, 250, time.UTC)
	getNow := func() time.Time {
		return now
	}

	for _, test := range []struct {
		in   string
		want time.Duration
		err  bool
	}{
		{"0", 0, false},
		{"", 0, true},
		{"1ms", time.Millisecond, false},
		{"1s", time.Second, false},
		{"1m", time.Minute, false},
		{"1.5m", (3 * time.Minute) / 2, false},
		{"1h", time.Hour, false},
		{"1d", time.Hour * 24, false},
		{"1w", time.Hour * 24 * 7, false},
		{"1M", time.Hour * 24 * 30, false},
		{"1y", time.Hour * 24 * 365, false},
		{"1.5y", time.Hour * 24 * 365 * 3 / 2, false},
		{"-1s", -time.Second, false},
		{"1.s", time.Second, false},
		{"1x", 0, true},
		{"off", time.Duration(DurationOff), false},
		{"1h2m3s", time.Hour + 2*time.Minute + 3*time.Second, false},
		{"2001-02-03", now.Sub(time.Date(2001, 2, 3, 0, 0, 0, 0, time.Local)), false},
		{"2001-02-03 10:11:12", now.Sub(time.Date(2001, 2, 3, 10, 11, 12, 0, time.Local)), false},
		{"2001-08-03 10:11:12", now.Sub(time.Date(2001, 8, 3, 10, 11, 12, 0, time.Local)), false},
		{"2001-02-03T10:11:12", now.Sub(time.Date(2001, 2, 3, 10, 11, 12, 0, time.Local)), false},
		{"2001-02-03T10:11:12.123Z", now.Sub(time.Date(2001, 2, 3, 10, 11, 12, 123, time.UTC)), false},
		{"2001-02-03T10:11:12.123+00:00", now.Sub(time.Date(2001, 2, 3, 10, 11, 12, 123, time.UTC)), false},
	} {
		duration, err := parseDurationFromNow(test.in, getNow)
		if test.err {
			require.Error(t, err)
		} else {
			require.NoError(t, err)
		}
		if strings.HasPrefix(test.in, "2001-") {
			ok := duration > test.want-time.Second && duration < test.want+time.Second
			assert.True(t, ok, test.in)
		} else {
			assert.Equal(t, test.want, duration)
		}
	}
}

func TestDurationString(t *testing.T) {
	now := time.Date(2020, 9, 5, 8, 15, 5, 250, time.UTC)
	getNow := func() time.Time {
		return now
	}

	for _, test := range []struct {
		in   time.Duration
		want string
	}{
		{time.Duration(0), "0s"},
		{time.Second, "1s"},
		{time.Minute, "1m0s"},
		{time.Millisecond, "1ms"},
		{time.Second, "1s"},
		{(3 * time.Minute) / 2, "1m30s"},
		{time.Hour, "1h0m0s"},
		{time.Hour * 24, "1d"},
		{time.Hour * 24 * 7, "1w"},
		{time.Hour * 24 * 30, "1M"},
		{time.Hour * 24 * 365, "1y"},
		{time.Hour * 24 * 365 * 3 / 2, "1.5y"},
		{-time.Second, "-1s"},
		{time.Second, "1s"},
		{time.Duration(DurationOff), "off"},
		{time.Hour + 2*time.Minute + 3*time.Second, "1h2m3s"},
		{time.Hour * 24, "1d"},
		{time.Hour * 24 * 7, "1w"},
		{time.Hour * 24 * 30, "1M"},
		{time.Hour * 24 * 365, "1y"},
		{time.Hour * 24 * 365 * 3 / 2, "1.5y"},
		{-time.Hour * 24 * 365 * 3 / 2, "-1.5y"},
	} {
		got := Duration(test.in).String()
		assert.Equal(t, test.want, got)
		// Test the reverse
		reverse, err := parseDurationFromNow(test.want, getNow)
		assert.NoError(t, err)
		assert.Equal(t, test.in, reverse)
	}
}

func TestDurationReadableString(t *testing.T) {
	for _, test := range []struct {
		negative bool
		in       time.Duration
		want     string
	}{
		// Edge Cases
		{false, time.Duration(DurationOff), "off"},
		// Base Cases
		{false, time.Duration(0), "0s"},
		{true, time.Millisecond, "1ms"},
		{true, time.Second, "1s"},
		{true, time.Minute, "1m"},
		{true, (3 * time.Minute) / 2, "1m30s"},
		{true, time.Hour, "1h"},
		{true, time.Hour * 24, "1d"},
		{true, time.Hour * 24 * 7, "1w"},
		{true, time.Hour * 24 * 365, "1y"},
		// Composite Cases
		{true, time.Hour + 2*time.Minute + 3*time.Second, "1h2m3s"},
		{true, time.Hour * 24 * (365 + 14), "1y2w"},
		{true, time.Hour*24*4 + time.Hour*3 + time.Minute*2 + time.Second, "4d3h2m1s"},
		{true, time.Hour * 24 * (365*3 + 7*2 + 1), "3y2w1d"},
		{true, time.Hour*24*(365*3+7*2+1) + time.Hour*2 + time.Second, "3y2w1d2h1s"},
		{true, time.Hour*24*(365*3+7*2+1) + time.Second, "3y2w1d1s"},
		{true, time.Hour*24*(365+7*2+3) + time.Hour*4 + time.Minute*5 + time.Second*6 + time.Millisecond*7, "1y2w3d4h5m6s7ms"},
	} {
		got := Duration(test.in).ReadableString()
		assert.Equal(t, test.want, got)

		// Test Negative Case
		if test.negative {
			got = Duration(-test.in).ReadableString()
			assert.Equal(t, "-"+test.want, got)
		}
	}
}

func TestDurationScan(t *testing.T) {
	var v Duration
	n, err := fmt.Sscan(" 17m ", &v)
	require.NoError(t, err)
	assert.Equal(t, 1, n)
	assert.Equal(t, Duration(17*60*time.Second), v)
}

func TestParseUnmarshalJSON(t *testing.T) {
	for _, test := range []struct {
		in   string
		want time.Duration
		err  bool
	}{
		{`""`, 0, true},
		{`"0"`, 0, false},
		{`"1ms"`, time.Millisecond, false},
		{`"1s"`, time.Second, false},
		{`"1m"`, time.Minute, false},
		{`"1h"`, time.Hour, false},
		{`"1d"`, time.Hour * 24, false},
		{`"1w"`, time.Hour * 24 * 7, false},
		{`"1M"`, time.Hour * 24 * 30, false},
		{`"1y"`, time.Hour * 24 * 365, false},
		{`"off"`, time.Duration(DurationOff), false},
		{`"error"`, 0, true},
		{"0", 0, false},
		{"1000000", time.Millisecond, false},
		{"1000000000", time.Second, false},
		{"60000000000", time.Minute, false},
		{"3600000000000", time.Hour, false},
		{"9223372036854775807", time.Duration(DurationOff), false},
		{"error", 0, true},
	} {
		var duration Duration
		err := json.Unmarshal([]byte(test.in), &duration)
		if test.err {
			require.Error(t, err, test.in)
		} else {
			require.NoError(t, err, test.in)
		}
		assert.Equal(t, Duration(test.want), duration, test.in)
	}
}
