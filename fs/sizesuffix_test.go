package fs

import (
	"encoding/json"
	"fmt"
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Check it satisfies the interfaces
var (
	_ Flagger   = (*SizeSuffix)(nil)
	_ FlaggerNP = SizeSuffix(0)
)

func TestSizeSuffixString(t *testing.T) {
	for _, test := range []struct {
		in   float64
		want string
	}{
		{0, "0"},
		{102, "102"},
		{1024, "1Ki"},
		{1024 * 1024, "1Mi"},
		{1024 * 1024 * 1024, "1Gi"},
		{10 * 1024 * 1024 * 1024, "10Gi"},
		{10.1 * 1024 * 1024 * 1024, "10.100Gi"},
		{-1, "off"},
		{-100, "off"},
	} {
		ss := SizeSuffix(test.in)
		got := ss.String()
		assert.Equal(t, test.want, got)
	}
}

func TestSizeSuffixByteUnit(t *testing.T) {
	for _, test := range []struct {
		in   float64
		want string
	}{
		{0, "0 B"},
		{102, "102 B"},
		{1024, "1 KiB"},
		{1024 * 1024, "1 MiB"},
		{1024 * 1024 * 1024, "1 GiB"},
		{10 * 1024 * 1024 * 1024, "10 GiB"},
		{10.1 * 1024 * 1024 * 1024, "10.100 GiB"},
		{10 * 1024 * 1024 * 1024 * 1024, "10 TiB"},
		{10 * 1024 * 1024 * 1024 * 1024 * 1024, "10 PiB"},
		{1 * 1024 * 1024 * 1024 * 1024 * 1024 * 1024, "1 EiB"},
		{-1, "off"},
		{-100, "off"},
	} {
		ss := SizeSuffix(test.in)
		got := ss.ByteUnit()
		assert.Equal(t, test.want, got)
	}
}

func TestSizeSuffixBitRateUnit(t *testing.T) {
	for _, test := range []struct {
		in   float64
		want string
	}{
		{0, "0 bit/s"},
		{1024, "1 Kibit/s"},
		{1024 * 1024, "1 Mibit/s"},
		{1024 * 1024 * 1024, "1 Gibit/s"},
		{10 * 1024 * 1024 * 1024, "10 Gibit/s"},
		{10.1 * 1024 * 1024 * 1024, "10.100 Gibit/s"},
		{10 * 1024 * 1024 * 1024 * 1024, "10 Tibit/s"},
		{10 * 1024 * 1024 * 1024 * 1024 * 1024, "10 Pibit/s"},
		{1 * 1024 * 1024 * 1024 * 1024 * 1024 * 1024, "1 Eibit/s"},
		{-1, "off"},
		{-100, "off"},
	} {
		ss := SizeSuffix(test.in)
		got := ss.BitRateUnit()
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
		{"1k", 1024, false},
		//{"1KB", 1024, false},
		//{"1kB", 1024, false},
		//{"1kb", 1024, false},
		{"1KI", 1024, false},
		{"1Ki", 1024, false},
		{"1kI", 1024, false},
		{"1ki", 1024, false},
		{"1KiB", 1024, false},
		{"1KiB", 1024, false},
		{"1kib", 1024, false},
		{"1", 1024, false},
		{"2.5", 1024 * 2.5, false},
		{"1M", 1024 * 1024, false},
		//{"1MB", 1024 * 1024, false},
		{"1Mi", 1024 * 1024, false},
		{"1MiB", 1024 * 1024, false},
		{"1.g", 1024 * 1024 * 1024, false},
		{"10G", 10 * 1024 * 1024 * 1024, false},
		{"10T", 10 * 1024 * 1024 * 1024 * 1024, false},
		{"10P", 10 * 1024 * 1024 * 1024 * 1024 * 1024, false},
		{"7E", 7 * 1024 * 1024 * 1024 * 1024 * 1024 * 1024, false},
		{"8E", 0, true},
		{"8Ei", 0, true},
		{"8EiB", 0, true},
		{"10E", 0, true},
		{"off", -1, false},
		{"OFF", -1, false},
		{"", 0, true},
		{"1q", 0, true},
		{"1.q", 0, true},
		{"1q", 0, true},
		{"-1K", 0, true},
		{"1i", 0, true},
		{"1iB", 0, true},
		{"1MB", 0, true},
	} {
		ss := SizeSuffix(0)
		err := ss.Set(test.in)
		if test.err {
			require.Error(t, err, test.in)
		} else {
			require.NoError(t, err, test.in)
		}
		assert.Equal(t, test.want, int64(ss))
	}
}

func TestSizeSuffixSetExactInteger(t *testing.T) {
	// Integer inputs must be scaled exactly, even when they aren't
	// representable as a float64.
	for _, test := range []struct {
		in   string
		want int64
	}{
		{"123456789012345678B", 123456789012345678},
		{"9007199254740993B", 9007199254740993}, // 2^53 + 1
		{"1152921504606846977B", (1 << 60) + 1},
	} {
		ss := SizeSuffix(0)
		require.NoError(t, ss.Set(test.in), test.in)
		assert.Equal(t, test.want, int64(ss), test.in)
	}
}

func TestSizeSuffixStringRoundTrip(t *testing.T) {
	// Values whose rounded pretty form doesn't parse back to the same
	// value, plus boundary values around the suffix steps.
	for _, in := range []SizeSuffix{
		0, 1, 1023, 1024, 1025, 1046, 1047,
		123456789012345678,
		(1 << 60) + 1,
		math.MaxInt64 - 1,
	} {
		str := in.stringRoundTrip()
		var out SizeSuffix
		require.NoError(t, out.Set(str), "in=%d str=%q", int64(in), str)
		assert.Equal(t, int64(in), int64(out), "in=%d str=%q", int64(in), str)
	}
	// Every value in a dense range must round trip exactly.
	for i := SizeSuffix(0); i <= 100000; i++ {
		str := i.stringRoundTrip()
		var out SizeSuffix
		require.NoError(t, out.Set(str))
		require.Equal(t, int64(i), int64(out), "str=%q", str)
	}
	// As must samples around each power of two.
	for shift := uint(10); shift < 63; shift += 5 {
		for _, delta := range []SizeSuffix{-2, -1, 0, 1, 2, 12345} {
			in := (SizeSuffix(1) << shift) + delta
			str := in.stringRoundTrip()
			var out SizeSuffix
			require.NoError(t, out.Set(str), "in=%d str=%q", int64(in), str)
			require.Equal(t, int64(in), int64(out), "str=%q", str)
		}
	}
}

func TestSizeSuffixScan(t *testing.T) {
	var v SizeSuffix
	n, err := fmt.Sscan(" 17M ", &v)
	require.NoError(t, err)
	assert.Equal(t, 1, n)
	assert.Equal(t, SizeSuffix(17<<20), v)
}

func TestSizeSuffixUnmarshalJSON(t *testing.T) {
	for _, test := range []struct {
		in   string
		want int64
		err  bool
	}{
		{`"0"`, 0, false},
		{`"102B"`, 102, false},
		{`"1K"`, 1024, false},
		{`"2.5"`, 1024 * 2.5, false},
		{`"1M"`, 1024 * 1024, false},
		{`"1.g"`, 1024 * 1024 * 1024, false},
		{`"10G"`, 10 * 1024 * 1024 * 1024, false},
		{`"off"`, -1, false},
		{`""`, 0, true},
		{`"1q"`, 0, true},
		{`"-1K"`, 0, true},
		{`0`, 0, false},
		{`102`, 102, false},
		{`1024`, 1024, false},
		{`1000000000`, 1000000000, false},
		{`1.1.1`, 0, true},
	} {
		var ss SizeSuffix
		err := json.Unmarshal([]byte(test.in), &ss)
		if test.err {
			require.Error(t, err, test.in)
		} else {
			require.NoError(t, err, test.in)
		}
		assert.Equal(t, test.want, int64(ss))
	}
}
