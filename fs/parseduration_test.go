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
		want float64
		err  bool
	}{
		{"0", 0, false},
		{"", 0, true},
		{"1ms", float64(time.Millisecond), false},
		{"1s", float64(time.Second), false},
		{"1m", float64(time.Minute), false},
		{"1h", float64(time.Hour), false},
		{"1d", float64(time.Hour) * 24, false},
		{"1w", float64(time.Hour) * 24 * 7, false},
		{"1M", float64(time.Hour) * 24 * 30, false},
		{"1y", float64(time.Hour) * 24 * 365, false},
		{"1.5y", float64(time.Hour) * 24 * 365 * 1.5, false},
		{"-1s", -float64(time.Second), false},
		{"1.s", float64(time.Second), false},
		{"1x", 0, true},
	} {
		duration, err := ParseDuration(test.in)
		if test.err {
			require.Error(t, err)
		} else {
			require.NoError(t, err)
		}
		assert.Equal(t, test.want, float64(duration))
	}
}
