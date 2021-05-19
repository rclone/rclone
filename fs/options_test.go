package fs

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/rclone/rclone/fs/hash"
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

func TestRangeOption(t *testing.T) {
	opt := &RangeOption{Start: 1, End: 10}
	var _ OpenOption = opt // check interface
	assert.Equal(t, "RangeOption(1,10)", opt.String())
	key, value := opt.Header()
	assert.Equal(t, "Range", key)
	assert.Equal(t, "bytes=1-10", value)
	assert.Equal(t, true, opt.Mandatory())

	opt = &RangeOption{Start: -1, End: 10}
	assert.Equal(t, "RangeOption(-1,10)", opt.String())
	key, value = opt.Header()
	assert.Equal(t, "Range", key)
	assert.Equal(t, "bytes=-10", value)
	assert.Equal(t, true, opt.Mandatory())

	opt = &RangeOption{Start: 1, End: -1}
	assert.Equal(t, "RangeOption(1,-1)", opt.String())
	key, value = opt.Header()
	assert.Equal(t, "Range", key)
	assert.Equal(t, "bytes=1-", value)
	assert.Equal(t, true, opt.Mandatory())

	opt = &RangeOption{Start: -1, End: -1}
	assert.Equal(t, "RangeOption(-1,-1)", opt.String())
	key, value = opt.Header()
	assert.Equal(t, "Range", key)
	assert.Equal(t, "bytes=-", value)
	assert.Equal(t, true, opt.Mandatory())
}

func TestSeekOption(t *testing.T) {
	opt := &SeekOption{Offset: 1}
	var _ OpenOption = opt // check interface
	assert.Equal(t, "SeekOption(1)", opt.String())
	key, value := opt.Header()
	assert.Equal(t, "Range", key)
	assert.Equal(t, "bytes=1-", value)
	assert.Equal(t, true, opt.Mandatory())
}

func TestHTTPOption(t *testing.T) {
	opt := &HTTPOption{Key: "k", Value: "v"}
	var _ OpenOption = opt // check interface
	assert.Equal(t, `HTTPOption("k","v")`, opt.String())
	key, value := opt.Header()
	assert.Equal(t, "k", key)
	assert.Equal(t, "v", value)
	assert.Equal(t, false, opt.Mandatory())
}

func TestHashesOption(t *testing.T) {
	opt := &HashesOption{hash.Set(hash.MD5 | hash.SHA1)}
	var _ OpenOption = opt // check interface
	assert.Equal(t, `HashesOption([md5, sha1])`, opt.String())
	key, value := opt.Header()
	assert.Equal(t, "", key)
	assert.Equal(t, "", value)
	assert.Equal(t, false, opt.Mandatory())
}

func TestNullOption(t *testing.T) {
	opt := NullOption{}
	var _ OpenOption = opt // check interface
	assert.Equal(t, "NullOption()", opt.String())
	key, value := opt.Header()
	assert.Equal(t, "", key)
	assert.Equal(t, "", value)
	assert.Equal(t, false, opt.Mandatory())
}

func TestFixRangeOptions(t *testing.T) {
	for _, test := range []struct {
		name string
		in   []OpenOption
		size int64
		want []OpenOption
	}{
		{
			name: "Nil options",
			in:   nil,
			want: nil,
		},
		{
			name: "Empty options",
			in:   []OpenOption{},
			want: []OpenOption{},
		},
		{
			name: "Fetch a range with size=0",
			in: []OpenOption{
				&HTTPOption{Key: "a", Value: "1"},
				&RangeOption{Start: 1, End: 10},
				&HTTPOption{Key: "b", Value: "2"},
			},
			want: []OpenOption{
				&HTTPOption{Key: "a", Value: "1"},
				NullOption{},
				&HTTPOption{Key: "b", Value: "2"},
			},
			size: 0,
		},
		{
			name: "Fetch a range",
			in: []OpenOption{
				&HTTPOption{Key: "a", Value: "1"},
				&RangeOption{Start: 1, End: 10},
				&HTTPOption{Key: "b", Value: "2"},
			},
			want: []OpenOption{
				&HTTPOption{Key: "a", Value: "1"},
				&RangeOption{Start: 1, End: 10},
				&HTTPOption{Key: "b", Value: "2"},
			},
			size: 100,
		},
		{
			name: "Fetch to end",
			in: []OpenOption{
				&RangeOption{Start: 1, End: -1},
			},
			want: []OpenOption{
				&RangeOption{Start: 1, End: -1},
			},
			size: 100,
		},
		{
			name: "Fetch the last 10 bytes",
			in: []OpenOption{
				&RangeOption{Start: -1, End: 10},
			},
			want: []OpenOption{
				&RangeOption{Start: 90, End: -1},
			},
			size: 100,
		},
		{
			name: "Fetch with end bigger than size",
			in: []OpenOption{
				&RangeOption{Start: 10, End: 200},
			},
			want: []OpenOption{
				&RangeOption{Start: 10, End: 99},
			},
			size: 100,
		},
	} {
		FixRangeOption(test.in, test.size)
		assert.Equal(t, test.want, test.in, test.name)
	}
}

var testOpenOptions = []OpenOption{
	&HTTPOption{Key: "a", Value: "1"},
	&RangeOption{Start: 1, End: 10},
	&HTTPOption{Key: "b", Value: "2"},
	NullOption{},
	&HashesOption{hash.Set(hash.MD5 | hash.SHA1)},
}

func TestOpenOptionAddHeaders(t *testing.T) {
	m := map[string]string{}
	want := map[string]string{
		"a":     "1",
		"Range": "bytes=1-10",
		"b":     "2",
	}
	OpenOptionAddHeaders(testOpenOptions, m)
	assert.Equal(t, want, m)
}

func TestOpenOptionHeaders(t *testing.T) {
	want := map[string]string{
		"a":     "1",
		"Range": "bytes=1-10",
		"b":     "2",
	}
	m := OpenOptionHeaders(testOpenOptions)
	assert.Equal(t, want, m)
	assert.Nil(t, OpenOptionHeaders([]OpenOption{}))
}

func TestOpenOptionAddHTTPHeaders(t *testing.T) {
	headers := http.Header{}
	want := http.Header{
		"A":     {"1"},
		"Range": {"bytes=1-10"},
		"B":     {"2"},
	}
	OpenOptionAddHTTPHeaders(headers, testOpenOptions)
	assert.Equal(t, want, headers)

}
