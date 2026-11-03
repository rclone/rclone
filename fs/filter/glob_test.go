package filter

import (
	"regexp"
	"regexp/syntax"
	"testing"
	"unicode"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGlobStringToRegexp(t *testing.T) {
	for _, test := range []struct {
		in    string
		want  string
		error string
	}{
		{``, ``, ``},
		{`potato`, `potato`, ``},
		{`potato,sausage`, `potato,sausage`, ``},
		{`/potato`, `/potato`, ``},
		{`potato?sausage`, `potato.sausage`, ``},
		{`potat[oa]`, `potat[oa]`, ``},
		{`potat[a-z]or`, `potat[a-z]or`, ``},
		{`potat[[:alpha:]]or`, `potat[[:alpha:]]or`, ``},
		{`'.' '+' '(' ')' '|' '^' '$'`, `'\.' '\+' '\(' '\)' '\|' '\^' '\$'`, ``},
		{`*.jpg`, `.*\.jpg`, ``},
		{`a{b,c,d}e`, `a(b|c|d)e`, ``},
		{`a{b,{c,d}}e`, `a(b|(c|d))e`, ``},
		{`{a,{b,c},{d,{e,f}}}`, `(a|(b|c)|(d|(e|f)))`, ``},
		{`potato**`, ``, `too many stars`},
		{`potato**sausage`, ``, `too many stars`},
		{`*.p[lm]`, `.*\.p[lm]`, ``},
		{`[\[\]]`, `[\[\]]`, ``},
		{`***potato`, ``, `too many stars`},
		{`***`, ``, `too many stars`},
		{`ab]c`, ``, `mismatched ']'`},
		{`ab[c`, ``, `mismatched '[' and ']'`},
		{`ab{x{cd`, ``, `mismatched '{' and '}'`},
		{`ab{}}cd`, ``, `mismatched '{' and '}'`},
		{`ab}c`, ``, `mismatched '{' and '}'`},
		{`ab{c`, ``, `mismatched '{' and '}'`},
		{`*.{jpg,png,gif}`, `.*\.(jpg|png|gif)`, ``},
		{`[a--b]`, ``, `bad glob pattern`},
		{`a\*b`, `a\*b`, ``},
		{`a\\b`, `a\\b`, ``},
		{`a{{.*}}b`, `a(.*)b`, ``},
		{`a{{.*}`, ``, `mismatched '{{' and '}}'`},
		{`{{regexp}}`, `(regexp)`, ``},
		{`\{{{regexp}}`, `\{(regexp)`, ``},
		{`/{{regexp}}`, `/(regexp)`, ``},
		{`/{{\d{8}}}`, `/(\d{8})`, ``},
		{`/{{\}}}`, `/(\})`, ``},
		{`{{(?i)regexp}}`, `((?i)regexp)`, ``},
	} {
		for _, ignoreCase := range []bool{false, true} {
			for _, addAnchors := range []bool{false, true} {
				gotRe, err := GlobStringToRegexp(test.in, addAnchors, ignoreCase)
				if test.error == "" {
					require.NoError(t, err, test.in)
					prefix := ""
					suffix := ""
					if ignoreCase {
						prefix += "(?i)"
					}
					if addAnchors {
						prefix += "^"
						suffix += "$"
					}
					got := gotRe.String()
					assert.Equal(t, prefix+test.want+suffix, got, test.in)
				} else {
					require.Error(t, err, test.in)
					assert.Contains(t, err.Error(), test.error, test.in)
					assert.Nil(t, gotRe)
				}
			}
		}
	}
}

func TestGlobPathToRegexp(t *testing.T) {
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
		{`a{b,{c,d}}e`, `(^|/)a(b|(c|d))e$`, ``},
		{`{a,b/{c,d}/**}`, `(^|/)(a|b/(c|d)/.*)$`, ``},
		{`potato**`, `(^|/)potato.*$`, ``},
		{`potato**sausage`, `(^|/)potato.*sausage$`, ``},
		{`*.p[lm]`, `(^|/)[^/]*\.p[lm]$`, ``},
		{`[\[\]]`, `(^|/)[\[\]]$`, ``},
		{`***potato`, ``, `too many stars`},
		{`***`, ``, `too many stars`},
		{`ab]c`, ``, `mismatched ']'`},
		{`ab[c`, ``, `mismatched '[' and ']'`},
		{`ab{x{cd`, ``, `mismatched '{' and '}'`},
		{`ab{}}cd`, ``, `mismatched '{' and '}'`},
		{`ab}c`, ``, `mismatched '{' and '}'`},
		{`ab{c`, ``, `mismatched '{' and '}'`},
		{`*.{jpg,png,gif}`, `(^|/)[^/]*\.(jpg|png|gif)$`, ``},
		{`[a--b]`, ``, `bad glob pattern`},
		{`a\*b`, `(^|/)a\*b$`, ``},
		{`a\\b`, `(^|/)a\\b$`, ``},
		{`a{{.*}}b`, `(^|/)a(.*)b$`, ``},
		{`a{{.*}`, ``, `mismatched '{{' and '}}'`},
		{`{{regexp}}`, `(^|/)(regexp)$`, ``},
		{`\{{{regexp}}`, `(^|/)\{(regexp)$`, ``},
		{`/{{regexp}}`, `^(regexp)$`, ``},
		{`/{{\d{8}}}`, `^(\d{8})$`, ``},
		{`/{{\}}}`, `^(\})$`, ``},
		{`{{(?i)regexp}}`, `(^|/)((?i)regexp)$`, ``},
	} {
		for _, ignoreCase := range []bool{false, true} {
			gotRe, err := GlobPathToRegexp(test.in, ignoreCase)
			if test.error == "" {
				require.NoError(t, err, test.in)
				prefix := ""
				if ignoreCase {
					prefix = "(?i)"
				}
				got := gotRe.String()
				assert.Equal(t, prefix+test.want, got, test.in)
			} else {
				require.Error(t, err, test.in)
				assert.Contains(t, err.Error(), test.error, test.in)
				assert.Nil(t, gotRe)
			}
		}
	}
}

func TestGlobToDirGlobs(t *testing.T) {
	for _, test := range []struct {
		in   string
		want []string
	}{
		{`*`, []string{"/**"}},
		{`/*`, []string{"/"}},
		{`*.jpg`, []string{"/**"}},
		{`/*.jpg`, []string{"/"}},
		{`//*.jpg`, []string{"/"}},
		{`///*.jpg`, []string{"/"}},
		{`/a/*.jpg`, []string{"/a/", "/"}},
		{`/a//*.jpg`, []string{"/a/", "/"}},
		{`/a///*.jpg`, []string{"/a/", "/"}},
		{`/a/b/*.jpg`, []string{"/a/b/", "/a/", "/"}},
		{`a/*.jpg`, []string{"a/"}},
		{`a/b/*.jpg`, []string{"a/b/", "a/"}},
		{`*/*/*.jpg`, []string{"*/*/", "*/"}},
		{`a/b/`, []string{"a/b/", "a/"}},
		{`a/b`, []string{"a/"}},
		{`a/b/*.{jpg,png,gif}`, []string{"a/b/", "a/"}},
		{`/a/{jpg,png,gif}/*.{jpg,png,gif}`, []string{"/a/{jpg,png,gif}/", "/a/", "/"}},
		{`a/{b,{c,d}}/*.jpg`, []string{"/**"}},
		{`a/{a,a*b,a**c}/d/`, []string{"/**"}},
		{`/a/{a,a*b,a/c,d}/d/`, []string{"/**"}},
		{`/a/{{.*}}/d/`, []string{"/**"}},
		{`**`, []string{"**/"}},
		{`a**`, []string{"a**/"}},
		{`a**b`, []string{"a**/"}},
		{`a**b**c**d`, []string{"a**b**c**/", "a**b**/", "a**/"}},
		{`a**b/c**d`, []string{"a**b/c**/", "a**b/", "a**/"}},
		{`/A/a**b/B/c**d/C/`, []string{"/A/a**b/B/c**d/C/", "/A/a**b/B/c**d/", "/A/a**b/B/c**/", "/A/a**b/B/", "/A/a**b/", "/A/a**/", "/A/", "/"}},
		{`/var/spool/**/ncw`, []string{"/var/spool/**/", "/var/spool/", "/var/", "/"}},
		{`var/spool/**/ncw/`, []string{"var/spool/**/ncw/", "var/spool/**/", "var/spool/", "var/"}},
		{"/file1.jpg", []string{`/`}},
		{"/file2.png", []string{`/`}},
		{"/*.jpg", []string{`/`}},
		{"/*.png", []string{`/`}},
		{"/potato", []string{`/`}},
		{"/sausage1", []string{`/`}},
		{"/sausage2*", []string{`/`}},
		{"/sausage3**", []string{`/sausage3**/`, "/"}},
		{"/a/*.jpg", []string{`/a/`, "/"}},
	} {
		_, err := GlobPathToRegexp(test.in, false)
		assert.NoError(t, err)
		got := globToDirGlobs(test.in)
		assert.Equal(t, test.want, got, test.in)
	}
}

// literal returns the one string re matches, or false if it can match more
func literal(t *testing.T, re *regexp.Regexp) (string, bool) {
	p, err := syntax.Parse(re.String(), syntax.Perl)
	require.NoError(t, err)
	p = p.Simplify()
	if p.Op != syntax.OpConcat || len(p.Sub) != 3 || p.Sub[0].Op != syntax.OpBeginText ||
		p.Sub[1].Op != syntax.OpLiteral || p.Sub[1].Flags&syntax.FoldCase != 0 || p.Sub[2].Op != syntax.OpEndText {
		return "", false
	}
	return string(p.Sub[1].Rune), true
}

func TestGlobEscape(t *testing.T) {
	var chars []rune
	for c := rune(' '); c <= '~'; c++ {
		if !unicode.IsLetter(c) && !unicode.IsDigit(c) {
			chars = append(chars, c)
		}
	}
	chars = append(chars, '\u00a0', '日')
	for _, c := range chars {
		for _, s := range []string{string(c), "a" + string(c) + "b", string(c) + string(c)} {
			glob := GlobEscape(s)
			re, err := GlobPathToRegexp("/"+glob, false)
			require.NoError(t, err, glob)
			got, ok := literal(t, re)
			assert.True(t, ok, "path glob %q matches more than %q", glob, s)
			assert.Equal(t, s, got)
			re, err = GlobStringToRegexp(glob, true, false)
			require.NoError(t, err, glob)
			got, ok = literal(t, re)
			assert.True(t, ok, "string glob %q matches more than %q", glob, s)
			assert.Equal(t, s, got)
		}
	}
	assert.Equal(t, `a\*b\?\[c\]\{d\}\\e`, GlobEscape(`a*b?[c]{d}\e`))
}
