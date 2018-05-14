package configstruct

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCamelToSnake(t *testing.T) {
	for _, test := range []struct {
		in   string
		want string
	}{
		{"", ""},
		{"Type", "type"},
		{"AuthVersion", "auth_version"},
		{"AccessKeyID", "access_key_id"},
	} {
		got := camelToSnake(test.in)
		assert.Equal(t, test.want, got, test.in)
	}
}

func TestStringToInterface(t *testing.T) {
	item := struct{ A int }{2}
	for _, test := range []struct {
		in   string
		def  interface{}
		want interface{}
		err  string
	}{
		{"", string(""), "", ""},
		{"   string   ", string(""), "   string   ", ""},
		{"123", int(0), int(123), ""},
		{"0x123", int(0), int(0x123), ""},
		{"   0x123   ", int(0), int(0x123), ""},
		{"-123", int(0), int(-123), ""},
		{"0", false, false, ""},
		{"1", false, true, ""},
		{"FALSE", false, false, ""},
		{"true", false, true, ""},
		{"123", uint(0), uint(123), ""},
		{"123", int64(0), int64(123), ""},
		{"123x", int64(0), nil, "parsing \"123x\" as int64 failed: expected newline"},
		{"truth", false, nil, "parsing \"truth\" as bool failed: syntax error scanning boolean"},
		{"struct", item, nil, "parsing \"struct\" as struct { A int } failed: can't scan type: *struct { A int }"},
	} {
		what := fmt.Sprintf("parse %q as %T", test.in, test.def)
		got, err := StringToInterface(test.def, test.in)
		if test.err == "" {
			require.NoError(t, err, what)
			assert.Equal(t, test.want, got, what)
		} else {
			assert.Nil(t, got)
			assert.EqualError(t, err, test.err, what)
		}
	}
}
