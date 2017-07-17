package opendrive

import "testing"

func TestReplace(t *testing.T) {
	for _, test := range []struct {
		in  string
		out string
	}{
		{"", ""},
		{"abc 123", "abc 123"},
		{`\*<>?:|#%".~`, `＼＊＜＞？：｜＃％＂.~`},
		{`\*<>?:|#%".~/\*<>?:|#%".~`, `＼＊＜＞？：｜＃％＂.~/＼＊＜＞？：｜＃％＂.~`},
		{" leading space", "␠leading space"},
		{"~leading tilde", "～leading tilde"},
		{"trailing dot.", "trailing dot．"},
		{" leading space/ leading space/ leading space", "␠leading space/␠leading space/␠leading space"},
		{"~leading tilde/~leading tilde/~leading tilde", "～leading tilde/～leading tilde/～leading tilde"},
		{"trailing dot./trailing dot./trailing dot.", "trailing dot．/trailing dot．/trailing dot．"},
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
