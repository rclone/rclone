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

// Check it satisfies the interfaces
var (
	_ Flagger   = (*Duration)(nil)
	_ FlaggerNP = Duration(0)
)

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
		negative  bool
		in        time.Duration
		wantLong  string
		wantShort string
	}{
		// Edge Cases
		{false, time.Duration(DurationOff), "off", "off"},
		// Base Cases
		{false, time.Duration(0), "0s", "0s"},
		{true, time.Millisecond, "1ms", "1ms"},
		{true, time.Second, "1s", "1s"},
		{true, time.Minute, "1m", "1m"},
		{true, (3 * time.Minute) / 2, "1m30s", "1m30s"},
		{true, time.Hour, "1h", "1h"},
		{true, time.Hour * 24, "1d", "1d"},
		{true, time.Hour * 24 * 7, "1w", "1w"},
		{true, time.Hour * 24 * 365, "1y", "1y"},
		// Composite Cases
		{true, time.Hour + 2*time.Minute + 3*time.Second, "1h2m3s", "1h2m3s"},
		{true, time.Hour * 24 * (365 + 14), "1y2w", "1y2w"},
		{true, time.Hour*24*4 + time.Hour*3 + time.Minute*2 + time.Second, "4d3h2m1s", "4d3h2m"},
		{true, time.Hour * 24 * (365*3 + 7*2 + 1), "3y2w1d", "3y2w1d"},
		{true, time.Hour*24*(365*3+7*2+1) + time.Hour*2 + time.Second, "3y2w1d2h1s", "3y2w1d"},
		{true, time.Hour*24*(365*3+7*2+1) + time.Second, "3y2w1d1s", "3y2w1d"},
		{true, time.Hour*24*(365+7*2+3) + time.Hour*4 + time.Minute*5 + time.Second*6 + time.Millisecond*7, "1y2w3d4h5m6s7ms", "1y2w3d"},
		{true, time.Duration(DurationOff) / time.Millisecond * time.Millisecond, "292y24w3d23h47m16s853ms", "292y24w3d"}, // Should have been 854ms but some precision are lost with floating point calculations
	} {
		got := Duration(test.in).ReadableString()
		assert.Equal(t, test.wantLong, got)
		got = Duration(test.in).ShortReadableString()
		assert.Equal(t, test.wantShort, got)

		// Test Negative Case
		if test.negative {
			got = Duration(-test.in).ReadableString()
			assert.Equal(t, "-"+test.wantLong, got)
			got = Duration(-test.in).ShortReadableString()
			assert.Equal(t, "-"+test.wantShort, got)
		}
	}
}

func TestDurationScan(t *testing.T) {
	now := time.Date(2020, 9, 5, 8, 15, 5, 250, time.UTC)
	oldTimeNowFunc := timeNowFunc
	timeNowFunc = func() time.Time { return now }
	defer func() { timeNowFunc = oldTimeNowFunc }()

	for _, test := range []struct {
		in   string
		want Duration
	}{
		{"17m", Duration(17 * time.Minute)},
		{"-12h", Duration(-12 * time.Hour)},
		{"0", Duration(0)},
		{"off", DurationOff},
		{"2022-03-26T17:48:19Z", Duration(now.Sub(time.Date(2022, 03, 26, 17, 48, 19, 0, time.UTC)))},
		{"2022-03-26 17:48:19", Duration(now.Sub(time.Date(2022, 03, 26, 17, 48, 19, 0, time.Local)))},
	} {
		var got Duration
		n, err := fmt.Sscan(test.in, &got)
		require.NoError(t, err)
		assert.Equal(t, 1, n)
		assert.Equal(t, test.want, got)
	}
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

func TestUnmarshalJSON(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    Duration
		wantErr bool
	}{
		{"off string", `"off"`, DurationOff, false},
		{"max int64", `9223372036854775807`, DurationOff, false},
		{"duration string", `"1h"`, Duration(time.Hour), false},
		{"invalid string", `"invalid"`, 0, true},
		{"negative int", `-1`, Duration(-1), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var d Duration
			err := json.Unmarshal([]byte(tt.input), &d)
			if (err != nil) != tt.wantErr {
				t.Errorf("UnmarshalJSON() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if d != tt.want {
				t.Errorf("UnmarshalJSON() got = %v, want %v", d, tt.want)
			}
		})
	}
}
