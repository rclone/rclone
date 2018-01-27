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
