package fs

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewUsageValueInt64(t *testing.T) {
	for _, test := range []struct {
		in   int64
		want int64
	}{
		{0, 0},
		{1 << 60, 1 << 60},
		{math.MaxInt64, math.MaxInt64},
	} {
		assert.Equal(t, test.want, *NewUsageValue(test.in), "in=%d", test.in)
	}
}

func TestNewUsageValueUint64(t *testing.T) {
	for _, test := range []struct {
		in   uint64
		want int64
	}{
		{0, 0},
		{math.MaxInt64, math.MaxInt64},
		{math.MaxInt64 + 1, math.MaxInt64},
		{math.MaxUint64, math.MaxInt64},
	} {
		assert.Equal(t, test.want, *NewUsageValue(test.in), "in=%d", test.in)
	}
}

func TestNewUsageValueFloat64(t *testing.T) {
	// Largest float64 strictly below 2**63 - this still fits in an int64.
	const belowMax = float64(9223372036854773760)
	for _, test := range []struct {
		in   float64
		want int64
	}{
		{0, 0},
		{1e18, 1000000000000000000}, // Box reports space_amount like this
		{belowMax, 9223372036854773760},
		{float64(math.MaxInt64), math.MaxInt64}, // rounds up to 2**63
		{1e19, math.MaxInt64},
		{math.Inf(1), math.MaxInt64},
	} {
		assert.Equal(t, test.want, *NewUsageValue(test.in), "in=%v", test.in)
	}
}
