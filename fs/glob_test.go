package fs

import (
	"strings"
	"testing"
)

func TestGlobToRegexp(t *testing.T) {
	for _, test := range []struct {
		in    string
		want  string
		error string
	}{
		{``, `(^|/)$`, ``},
		{`potato`, `(^|/)potato$`, ``},
		{`potato,sausage`, `(^|/)potato,sausage$`, ``},
		{`/potato`, `^potato$`, ``},
		{`potato?sausage`, `(^|/)potato[^/]sausage$`, ``},
		{`potat[oa]`, `(^|/)potat[oa]$`, ``},
		{`potat[a-z]or`, `(^|/)potat[a-z]or$`, ``},
		{`potat[[:alpha:]]or`, `(^|/)potat[[:alpha:]]or$`, ``},
		{`'.' '+' '(' ')' '|' '^' '$'`, `(^|/)'\.' '\+' '\(' '\)' '\|' '\^' '\$'$`, ``},
		{`*.jpg`, `(^|/)[^/]*\.jpg$`, ``},
		{`a{b,c,d}e`, `(^|/)a(b|c|d)e$`, ``},
		{`potato**`, `(^|/)potato.*$`, ``},
		{`potato**sausage`, `(^|/)potato.*sausage$`, ``},
		{`*.p[lm]`, `(^|/)[^/]*\.p[lm]$`, ``},
		{`[\[\]]`, `(^|/)[\[\]]$`, ``},
		{`***potato`, `(^|/)`, `too many stars`},
		{`***`, `(^|/)`, `too many stars`},
		{`ab]c`, `(^|/)`, `mismatched ']'`},
		{`ab[c`, `(^|/)`, `mismatched '[' and ']'`},
		{`ab{{cd`, `(^|/)`, `can't nest`},
		{`ab{}}cd`, `(^|/)`, `mismatched '{' and '}'`},
		{`ab}c`, `(^|/)`, `mismatched '{' and '}'`},
		{`ab{c`, `(^|/)`, `mismatched '{' and '}'`},
		{`*.{jpg,png,gif}`, `(^|/)[^/]*\.(jpg|png|gif)$`, ``},
		{`[a--b]`, `(^|/)`, `Bad glob pattern`},
		{`a\*b`, `(^|/)a\*b$`, ``},
		{`a\\b`, `(^|/)a\\b$`, ``},
	} {
		gotRe, err := globToRegexp(test.in)
		if test.error == "" {
			if err != nil {
				t.Errorf("%q: not expecting error: %v", test.in, err)
			} else {
				got := gotRe.String()
				if test.want != got {
					t.Errorf("%q: want %q got %q", test.in, test.want, got)
				}
			}
		} else {
			if err == nil {
				t.Errorf("%q: expecting error but didn't get one", test.in)
			} else {
				got := err.Error()
				if !strings.Contains(got, test.error) {
					t.Errorf("%q: want error %q got %q", test.in, test.error, got)
				}
			}
		}
	}

}
