package ftp

import "testing"

func TestParseUrlToDial(t *testing.T){
	for _, test := range []struct {
		in string
		want string
	}{
		{"ftp://foo.bar", "foo.bar:21"},
		{"http://foo.bar", "foo.bar:21"},
		{"ftp:/foo.bar:123", "foo.bar:123"},
	} {
		u := parseUrl(test.in)
		got := u.ToDial()
		if got != test.want {
			t.Logf("%q: want %q got %q", test.in, test.want, got)
		}
	}
}

func TestParseUrlPath(t *testing.T){
	for _, test := range []struct {
		in string
		want string
	}{
		{"ftp://foo.bar/", "/"},
		{"ftp://foo.bar/debian", "/debian"},
		{"ftp://foo.bar", "/"},
	} {
		u := parseUrl(test.in)
		if u.Path != test.want {
			t.Logf("%q: want %q got %q", test.in, test.want, u.Path)
		}
	}
}
