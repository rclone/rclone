package rest

import (
	"fmt"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestURLJoin(t *testing.T) {
	for i, test := range []struct {
		base   string
		path   string
		wantOK bool
		want   string
	}{
		{"http://example.com/", "potato", true, "http://example.com/potato"},
		{"http://example.com/dir/", "potato", true, "http://example.com/dir/potato"},
		{"http://example.com/dir/", "../dir/potato", true, "http://example.com/dir/potato"},
		{"http://example.com/dir/", "..", true, "http://example.com/"},
		{"http://example.com/dir/", "http://example.com/", true, "http://example.com/"},
		{"http://example.com/dir/", "http://example.com/dir/", true, "http://example.com/dir/"},
		{"http://example.com/dir/", "http://example.com/dir/potato", true, "http://example.com/dir/potato"},
		{"http://example.com/dir/", "/dir/", true, "http://example.com/dir/"},
		{"http://example.com/dir/", "/dir/potato", true, "http://example.com/dir/potato"},
		{"http://example.com/dir/", "subdir/potato", true, "http://example.com/dir/subdir/potato"},
		{"http://example.com/dir/", "With percent %25.txt", true, "http://example.com/dir/With%20percent%20%25.txt"},
		{"http://example.com/dir/", "With colon :", false, ""},
		{"http://example.com/dir/", URLPathEscape("With colon :"), true, "http://example.com/dir/With%20colon%20:"},
	} {
		u, err := url.Parse(test.base)
		require.NoError(t, err)
		got, err := URLJoin(u, test.path)
		gotOK := err == nil
		what := fmt.Sprintf("test %d base=%q, val=%q", i, test.base, test.path)
		assert.Equal(t, test.wantOK, gotOK, what)
		var gotString string
		if gotOK {
			gotString = got.String()
		}
		assert.Equal(t, test.want, gotString, what)
	}
}

func TestURLPathEscape(t *testing.T) {
	for i, test := range []struct {
		path string
		want string
	}{
		{"", ""},
		{"/hello.txt", "/hello.txt"},
		{"With Space", "With%20Space"},
		{"With Colon:", "./With%20Colon:"},
		{"With Percent%", "With%20Percent%25"},
	} {
		got := URLPathEscape(test.path)
		assert.Equal(t, test.want, got, fmt.Sprintf("Test %d path = %q", i, test.path))
	}
}

func TestURLPathEscapeAll(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"", ""},
		{"/hello.txt", "/hello%2Etxt"},
		{"With Space", "With%20Space"},
		{"With Colon:", "With%20Colon%3A"},
		{"With Percent%", "With%20Percent%25"},
		{"abc/XYZ123", "abc/XYZ123"},
		{"hello world", "hello%20world"},
		{"$test", "%24test"},
		{"ümlaut", "%C3%BCmlaut"},
		{"", ""},
		{" /?", "%20/%3F"},
	}

	for _, test := range tests {
		got := URLPathEscapeAll(test.in)
		assert.Equal(t, test.want, got)
	}
}
