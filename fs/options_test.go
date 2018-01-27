package fs

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseRangeOption(t *testing.T) {
	for _, test := range []struct {
		in   string
		want RangeOption
		err  string
	}{
		{in: "", err: "doesn't start with bytes="},
		{in: "bytes=1-2,3-4", err: "contains multiple ranges"},
		{in: "bytes=100", err: "contains no '-'"},
		{in: "bytes=x-8", err: "bad start"},
		{in: "bytes=8-x", err: "bad end"},
		{in: "bytes=1-2", want: RangeOption{Start: 1, End: 2}},
		{in: "bytes=-123456789123456789", want: RangeOption{Start: -1, End: 123456789123456789}},
		{in: "bytes=123456789123456789-", want: RangeOption{Start: 123456789123456789, End: -1}},
		{in: "bytes=  1  -  2  ", want: RangeOption{Start: 1, End: 2}},
		{in: "bytes=-", want: RangeOption{Start: -1, End: -1}},
		{in: "bytes=  -  ", want: RangeOption{Start: -1, End: -1}},
	} {
		got, err := ParseRangeOption(test.in)
		what := fmt.Sprintf("parsing %q", test.in)
		if test.err != "" {
			require.Contains(t, err.Error(), test.err)
			require.Nil(t, got, what)
		} else {
			require.NoError(t, err, what)
			assert.Equal(t, test.want, *got, what)
		}
	}
}

func TestRangeOptionDecode(t *testing.T) {
	for _, test := range []struct {
		in         RangeOption
		size       int64
		wantOffset int64
		wantLimit  int64
	}{
		{in: RangeOption{Start: 1, End: 10}, size: 100, wantOffset: 1, wantLimit: 10},
		{in: RangeOption{Start: 10, End: 10}, size: 100, wantOffset: 10, wantLimit: 1},
		{in: RangeOption{Start: 10, End: 9}, size: 100, wantOffset: 10, wantLimit: 0},
		{in: RangeOption{Start: 1, End: -1}, size: 100, wantOffset: 1, wantLimit: -1},
		{in: RangeOption{Start: -1, End: 90}, size: 100, wantOffset: 10, wantLimit: -1},
		{in: RangeOption{Start: -1, End: -1}, size: 100, wantOffset: 0, wantLimit: -1},
	} {
		gotOffset, gotLimit := test.in.Decode(test.size)
		what := fmt.Sprintf("%+v size=%d", test.in, test.size)
		assert.Equal(t, test.wantOffset, gotOffset, "offset "+what)
		assert.Equal(t, test.wantLimit, gotLimit, "limit "+what)
	}
}
