package fs

import (
	"testing"
	"time"

	"github.com/spf13/pflag"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Check it satisfies the interface
var _ pflag.Value = (*Duration)(nil)

func TestParseDuration(t *testing.T) {
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
	} {
		duration, err := ParseDuration(test.in)
		if test.err {
			require.Error(t, err)
		} else {
			require.NoError(t, err)
		}
		assert.Equal(t, test.want, duration)
	}
}

func TestDurationString(t *testing.T) {
	for _, test := range []struct {
		in   time.Duration
		want string
	}{
		{time.Duration(0), "0s"},
		{time.Second, "1s"},
		{time.Minute, "1m0s"},
		{time.Duration(DurationOff), "off"},
	} {
		got := Duration(test.in).String()
		assert.Equal(t, test.want, got)
	}
}
