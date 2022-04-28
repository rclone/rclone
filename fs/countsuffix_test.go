package fs

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Check it satisfies the interface
var _ flagger = (*CountSuffix)(nil)

func TestCountSuffixString(t *testing.T) {
	for _, test := range []struct {
		in   float64
		want string
	}{
		{0, "0"},
		{102, "102"},
		{1000, "1k"},
		{1000 * 1000, "1M"},
		{1000 * 1000 * 1000, "1G"},
		{10 * 1000 * 1000 * 1000, "10G"},
		{10.1 * 1000 * 1000 * 1000, "10.100G"},
		{-1, "off"},
		{-100, "off"},
	} {
		ss := CountSuffix(test.in)
		got := ss.String()
		assert.Equal(t, test.want, got)
	}
}

func TestCountSuffixUnit(t *testing.T) {
	for _, test := range []struct {
		in   float64
		want string
	}{
		{0, "0 Byte"},
		{102, "102 Byte"},
		{1000, "1 kByte"},
		{1000 * 1000, "1 MByte"},
		{1000 * 1000 * 1000, "1 GByte"},
		{10 * 1000 * 1000 * 1000, "10 GByte"},
		{10.1 * 1000 * 1000 * 1000, "10.100 GByte"},
		{10 * 1000 * 1000 * 1000 * 1000, "10 TByte"},
		{10 * 1000 * 1000 * 1000 * 1000 * 1000, "10 PByte"},
		{1 * 1000 * 1000 * 1000 * 1000 * 1000 * 1000, "1 EByte"},
		{-1, "off"},
		{-100, "off"},
	} {
		ss := CountSuffix(test.in)
		got := ss.Unit("Byte")
		assert.Equal(t, test.want, got)
	}
}

func TestCountSuffixSet(t *testing.T) {
	for _, test := range []struct {
		in   string
		want int64
		err  bool
	}{
		{"0", 0, false},
		{"1b", 1, false},
		{"100B", 100, false},
		{"0.1k", 100, false},
		{"0.1", 100, false},
		{"1K", 1000, false},
		{"1k", 1000, false},
		{"1KB", 1000, false},
		{"1kB", 1000, false},
		{"1kb", 1000, false},
		{"1", 1000, false},
		{"2.5", 1000 * 2.5, false},
		{"1M", 1000 * 1000, false},
		{"1MB", 1000 * 1000, false},
		{"1.g", 1000 * 1000 * 1000, false},
		{"10G", 10 * 1000 * 1000 * 1000, false},
		{"10T", 10 * 1000 * 1000 * 1000 * 1000, false},
		{"10P", 10 * 1000 * 1000 * 1000 * 1000 * 1000, false},
		{"off", -1, false},
		{"OFF", -1, false},
		{"", 0, true},
		{"1q", 0, true},
		{"1.q", 0, true},
		{"1q", 0, true},
		{"-1K", 0, true},
		{"1i", 0, true},
		{"1iB", 0, true},
		{"1Ki", 0, true},
		{"1KiB", 0, true},
	} {
		ss := CountSuffix(0)
		err := ss.Set(test.in)
		if test.err {
			require.Error(t, err, test.in)
		} else {
			require.NoError(t, err, test.in)
		}
		assert.Equal(t, test.want, int64(ss))
	}
}

func TestCountSuffixScan(t *testing.T) {
	var v CountSuffix
	n, err := fmt.Sscan(" 17M ", &v)
	require.NoError(t, err)
	assert.Equal(t, 1, n)
	assert.Equal(t, CountSuffix(17000000), v)
}

func TestCountSuffixUnmarshalJSON(t *testing.T) {
	for _, test := range []struct {
		in   string
		want int64
		err  bool
	}{
		{`"0"`, 0, false},
		{`"102B"`, 102, false},
		{`"1K"`, 1000, false},
		{`"2.5"`, 1000 * 2.5, false},
		{`"1M"`, 1000 * 1000, false},
		{`"1.g"`, 1000 * 1000 * 1000, false},
		{`"10G"`, 10 * 1000 * 1000 * 1000, false},
		{`"off"`, -1, false},
		{`""`, 0, true},
		{`"1q"`, 0, true},
		{`"-1K"`, 0, true},
		{`0`, 0, false},
		{`102`, 102, false},
		{`1000`, 1000, false},
		{`1000000000`, 1000000000, false},
		{`1.1.1`, 0, true},
	} {
		var ss CountSuffix
		err := json.Unmarshal([]byte(test.in), &ss)
		if test.err {
			require.Error(t, err, test.in)
		} else {
			require.NoError(t, err, test.in)
		}
		assert.Equal(t, test.want, int64(ss))
	}
}
