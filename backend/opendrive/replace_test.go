package opendrive

import "testing"

func TestReplace(t *testing.T) {
	for _, test := range []struct {
		in  string
		out string
	}{
		{"", ""},
		{"abc 123", "abc 123"},
		{`\*<>?:|#%".~`, `＼＊＜＞？：｜#%＂.~`},
		{`\*<>?:|#%".~/\*<>?:|#%".~`, `＼＊＜＞？：｜#%＂.~/＼＊＜＞？：｜#%＂.~`},
		{" leading space", "␠leading space"},
		{" path/  leading spaces", "␠path/␠ leading spaces"},
		{"trailing space ", "trailing space␠"},
		{"trailing spaces  /path ", "trailing spaces ␠/path␠"},
	} {
		got := replaceReservedChars(test.in)
		if got != test.out {
			t.Errorf("replaceReservedChars(%q) want %q got %q", test.in, test.out, got)
		}
		got2 := restoreReservedChars(got)
		if got2 != test.in {
			t.Errorf("restoreReservedChars(%q) want %q got %q", got, test.in, got2)
		}
	}
}
