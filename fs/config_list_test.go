package fs

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func must(err error) {
	if err != nil {
		panic(err)
	}
}

func ExampleSpaceSepList() {
	for _, s := range []string{
		`remotea:test/dir remoteb:`,
		`"remotea:test/space dir" remoteb:`,
		`"remotea:test/quote""dir" remoteb:`,
	} {
		var l SpaceSepList
		must(l.Set(s))
		fmt.Printf("%#v\n", l)
	}
	// Output:
	// fs.SpaceSepList{"remotea:test/dir", "remoteb:"}
	// fs.SpaceSepList{"remotea:test/space dir", "remoteb:"}
	// fs.SpaceSepList{"remotea:test/quote\"dir", "remoteb:"}
}

func ExampleCommaSepList() {
	for _, s := range []string{
		`remotea:test/dir,remoteb:`,
		`"remotea:test/space dir",remoteb:`,
		`"remotea:test/quote""dir",remoteb:`,
	} {
		var l CommaSepList
		must(l.Set(s))
		fmt.Printf("%#v\n", l)
	}
	// Output:
	// fs.CommaSepList{"remotea:test/dir", "remoteb:"}
	// fs.CommaSepList{"remotea:test/space dir", "remoteb:"}
	// fs.CommaSepList{"remotea:test/quote\"dir", "remoteb:"}
}

func TestSpaceSepListSet(t *testing.T) {
	type tc struct {
		in  string
		out SpaceSepList
		err string
	}
	tests := []tc{
		{``, nil, ""},
		{`\`, SpaceSepList{`\`}, ""},
		{`\\`, SpaceSepList{`\\`}, ""},
		{`potato`, SpaceSepList{`potato`}, ""},
		{`po\tato`, SpaceSepList{`po\tato`}, ""},
		{`potato\`, SpaceSepList{`potato\`}, ""},
		{`'potato`, SpaceSepList{`'potato`}, ""},
		{`pot'ato`, SpaceSepList{`pot'ato`}, ""},
		{`potato'`, SpaceSepList{`potato'`}, ""},
		{`"potato"`, SpaceSepList{`potato`}, ""},
		{`'potato'`, SpaceSepList{`'potato'`}, ""},
		{`potato apple`, SpaceSepList{`potato`, `apple`}, ""},
		{`potato\ apple`, SpaceSepList{`potato\`, `apple`}, ""},
		{`"potato  apple"`, SpaceSepList{`potato  apple`}, ""},
		{`"potato'apple"`, SpaceSepList{`potato'apple`}, ""},
		{`"potato''apple"`, SpaceSepList{`potato''apple`}, ""},
		{`"potato' 'apple"`, SpaceSepList{`potato' 'apple`}, ""},
		{`potato="apple"`, nil, `bare " in non-quoted-field`},
		{`apple "potato`, nil, "extraneous"},
		{`apple pot"ato`, nil, "bare \" in non-quoted-field"},
		{`potato"`, nil, "bare \" in non-quoted-field"},
	}
	for _, tc := range tests {
		var l SpaceSepList
		err := l.Set(tc.in)
		if tc.err == "" {
			require.NoErrorf(t, err, "input: %q", tc.in)
		} else {
			require.Containsf(t, err.Error(), tc.err, "input: %q", tc.in)
		}
		require.Equalf(t, tc.out, l, "input: %q", tc.in)
	}
}
