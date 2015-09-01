package fs

import "testing"

func TestSizeSuffixString(t *testing.T) {
	for _, test := range []struct {
		in   float64
		want string
	}{
		{0, "0"},
		{102, "0.100k"},
		{1024, "1k"},
		{1024 * 1024, "1M"},
		{1024 * 1024 * 1024, "1G"},
		{10 * 1024 * 1024 * 1024, "10G"},
		{10.1 * 1024 * 1024 * 1024, "10.100G"},
	} {
		ss := SizeSuffix(test.in)
		got := ss.String()
		if test.want != got {
			t.Errorf("Want %v got %v", test.want, got)
		}
	}
}

func TestSizeSuffixSet(t *testing.T) {
	for i, test := range []struct {
		in   string
		want int64
		err  bool
	}{
		{"0", 0, false},
		{"0.1k", 102, false},
		{"0.1", 102, false},
		{"1K", 1024, false},
		{"1", 1024, false},
		{"2.5", 1024 * 2.5, false},
		{"1M", 1024 * 1024, false},
		{"1.g", 1024 * 1024 * 1024, false},
		{"10G", 10 * 1024 * 1024 * 1024, false},
		{"", 0, true},
		{"1p", 0, true},
		{"1.p", 0, true},
		{"1p", 0, true},
		{"-1K", 0, true},
	} {
		ss := SizeSuffix(0)
		err := ss.Set(test.in)
		if (err != nil) != test.err {
			t.Errorf("%d: Expecting error %v but got error %v", i, test.err, err)
		}
		got := int64(ss)
		if test.want != got {
			t.Errorf("%d: Want %v got %v", i, test.want, got)
		}
	}
}

func TestReveal(t *testing.T) {
	for _, test := range []struct {
		in   string
		want string
	}{
		{"", ""},
		{"2sTcyNrA", "potato"},
	} {
		got := Reveal(test.in)
		if got != test.want {
			t.Errorf("%q: want %q got %q", test.in, test.want, got)
		}
		if Obscure(got) != test.in {
			t.Errorf("%q: wasn't bidirectional", test.in)
		}
	}
}
