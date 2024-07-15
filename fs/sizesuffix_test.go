package fs

import (
	"encoding/json"
	"fmt"
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
