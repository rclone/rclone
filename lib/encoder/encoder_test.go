package encoder

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/spf13/pflag"
	"github.com/stretchr/testify/assert"
)

// Check it satisfies the interfaces
var (
	_ pflag.Value = (*MultiEncoder)(nil)
	_ fmt.Scanner = (*MultiEncoder)(nil)
)

func TestEncodeString(t *testing.T) {
	for _, test := range []struct {
		mask MultiEncoder
		want string
	}{
		{EncodeRaw, "Raw"},
		{EncodeZero, "None"},
		{EncodeDoubleQuote, "DoubleQuote"},
		{EncodeDot, "Dot"},
		{EncodeWin, "LtGt,DoubleQuote,Colon,Question,Asterisk,Pipe"},
		{EncodeHashPercent, "Hash,Percent"},
		{EncodeSlash | EncodeDollar | EncodeColon, "Slash,Dollar,Colon"},
		{EncodeSlash | (1 << 31), "Slash,0x80000000"},
	} {
		got := test.mask.String()
		assert.Equal(t, test.want, got)
	}

}

func TestEncodeSet(t *testing.T) {
	for _, test := range []struct {
		in      string
		want    MultiEncoder
		wantErr bool
	}{
		{"", 0, true},
		{"Raw", EncodeRaw, false},
		{"None", EncodeZero, false},
		{"DoubleQuote", EncodeDoubleQuote, false},
		{"Dot", EncodeDot, false},
		{"LtGt,DoubleQuote,Colon,Question,Asterisk,Pipe", EncodeWin, false},
		{"Hash,Percent", EncodeHashPercent, false},
		{"Slash,Dollar,Colon", EncodeSlash | EncodeDollar | EncodeColon, false},
		{"Slash,0x80000000", EncodeSlash | (1 << 31), false},
		{"Blerp", 0, true},
		{"0xFGFFF", 0, true},
	} {
		var got MultiEncoder
		err := got.Set(test.in)
		assert.Equal(t, test.wantErr, err != nil, err)
		assert.Equal(t, test.want, got, test.in)
	}

}

type testCase struct {
	mask MultiEncoder
	in   string
	out  string
}

func TestEncodeSingleMask(t *testing.T) {
	for i, tc := range testCasesSingle {
		e := tc.mask
		t.Run(strconv.FormatInt(int64(i), 10), func(t *testing.T) {
			got := e.Encode(tc.in)
			if got != tc.out {
				t.Errorf("Encode(%q) want %q got %q", tc.in, tc.out, got)
			}
			got2 := e.Decode(got)
			if got2 != tc.in {
				t.Errorf("Decode(%q) want %q got %q", got, tc.in, got2)
			}
		})
	}
}

func TestEncodeSingleMaskEdge(t *testing.T) {
	for i, tc := range testCasesSingleEdge {
		e := tc.mask
		t.Run(strconv.FormatInt(int64(i), 10), func(t *testing.T) {
			got := e.Encode(tc.in)
			if got != tc.out {
				t.Errorf("Encode(%q) want %q got %q", tc.in, tc.out, got)
			}
			got2 := e.Decode(got)
			if got2 != tc.in {
				t.Errorf("Decode(%q) want %q got %q", got, tc.in, got2)
			}
		})
	}
}

func TestEncodeDoubleMaskEdge(t *testing.T) {
	for i, tc := range testCasesDoubleEdge {
		e := tc.mask
		t.Run(strconv.FormatInt(int64(i), 10), func(t *testing.T) {
			got := e.Encode(tc.in)
			if got != tc.out {
				t.Errorf("Encode(%q) want %q got %q", tc.in, tc.out, got)
			}
			got2 := e.Decode(got)
			if got2 != tc.in {
				t.Errorf("Decode(%q) want %q got %q", got, tc.in, got2)
			}
		})
	}
}

func TestEncodeInvalidUnicode(t *testing.T) {
	for i, tc := range []testCase{
		{
			mask: EncodeInvalidUtf8,
			in:   "\xBF",
			out:  "‛BF",
		}, {
			mask: EncodeInvalidUtf8,
			in:   "\xBF\xFE",
			out:  "‛BF‛FE",
		}, {
			mask: EncodeInvalidUtf8,
			in:   "a\xBF\xFEb",
			out:  "a‛BF‛FEb",
		}, {
			mask: EncodeInvalidUtf8,
			in:   "a\xBFξ\xFEb",
			out:  "a‛BFξ‛FEb",
		}, {
			mask: EncodeInvalidUtf8 | EncodeBackSlash,
			in:   "a\xBF\\\xFEb",
			out:  "a‛BF＼‛FEb",
		}, {
			mask: 0,
			in:   "\xBF",
			out:  "\xBF",
		}, {
			mask: 0,
			in:   "\xBF\xFE",
			out:  "\xBF\xFE",
		}, {
			mask: 0,
			in:   "a\xBF\xFEb",
			out:  "a\xBF\xFEb",
		}, {
			mask: 0,
			in:   "a\xBFξ\xFEb",
			out:  "a\xBFξ\xFEb",
		}, {
			mask: EncodeBackSlash,
			in:   "a\xBF\\\xFEb",
			out:  "a\xBF＼\xFEb",
		},
	} {
		e := tc.mask
		t.Run(strconv.FormatInt(int64(i), 10), func(t *testing.T) {
			got := e.Encode(tc.in)
			if got != tc.out {
				t.Errorf("Encode(%q) want %q got %q", tc.in, tc.out, got)
			}
			got2 := e.Decode(got)
			if got2 != tc.in {
				t.Errorf("Decode(%q) want %q got %q", got, tc.in, got2)
			}
		})
	}
}

func TestEncodeDot(t *testing.T) {
	for i, tc := range []testCase{
		{
			mask: EncodeZero,
			in:   ".",
			out:  ".",
		}, {
			mask: EncodeDot,
			in:   ".",
			out:  "．",
		}, {
			mask: EncodeZero,
			in:   "..",
			out:  "..",
		}, {
			mask: EncodeDot,
			in:   "..",
			out:  "．．",
		}, {
			mask: EncodeDot,
			in:   "...",
			out:  "...",
		}, {
			mask: EncodeDot,
			in:   ". .",
			out:  ". .",
		},
	} {
		e := tc.mask
		t.Run(strconv.FormatInt(int64(i), 10), func(t *testing.T) {
			got := e.Encode(tc.in)
			if got != tc.out {
				t.Errorf("Encode(%q) want %q got %q", tc.in, tc.out, got)
			}
			got2 := e.Decode(got)
			if got2 != tc.in {
				t.Errorf("Decode(%q) want %q got %q", got, tc.in, got2)
			}
		})
	}
}

func TestDecodeHalf(t *testing.T) {
	for i, tc := range []testCase{
		{
			mask: 0,
			in:   "‛",
			out:  "‛",
		}, {
			mask: EncodeZero,
			in:   "‛‛",
			out:  "‛",
		}, {
			mask: 0,
			in:   "‛a‛",
			out:  "‛a‛",
		}, {
			mask: EncodeInvalidUtf8,
			in:   "a‛B‛Eg",
			out:  "a‛B‛Eg",
		}, {
			mask: EncodeInvalidUtf8,
			in:   "a‛B＼‛Eg",
			out:  "a‛B＼‛Eg",
		}, {
			mask: EncodeInvalidUtf8 | EncodeBackSlash,
			in:   "a‛B＼‛Eg",
			out:  "a‛B\\‛Eg",
		},
	} {
		e := tc.mask
		t.Run(strconv.FormatInt(int64(i), 10), func(t *testing.T) {
			got := e.Decode(tc.in)
			if got != tc.out {
				t.Errorf("Decode(%q) want %q got %q", tc.in, tc.out, got)
			}
		})
	}
}

const oneDrive = (Standard |
	EncodeWin |
	EncodeBackSlash |
	EncodeHashPercent |
	EncodeDel |
	EncodeCtl |
	EncodeLeftTilde |
	EncodeRightSpace |
	EncodeRightPeriod)

var benchTests = []struct {
	in     string
	outOld string
	outNew string
}{
	{
		"",
		"",
		"",
	},
	{
		"abc 123",
		"abc 123",
		"abc 123",
	},
	{
		`\*<>?:|#%".~`,
		`＼＊＜＞？：｜＃％＂.~`,
		`＼＊＜＞？：｜＃％＂.~`,
	},
	{
		`\*<>?:|#%".~/\*<>?:|#%".~`,
		`＼＊＜＞？：｜＃％＂.~/＼＊＜＞？：｜＃％＂.~`,
		`＼＊＜＞？：｜＃％＂.~／＼＊＜＞？：｜＃％＂.~`,
	},
	{
		" leading space",
		" leading space",
		" leading space",
	},
	{
		"~leading tilde",
		"～leading tilde",
		"～leading tilde",
	},
	{
		"trailing dot.",
		"trailing dot．",
		"trailing dot．",
	},
	{
		" leading space/ leading space/ leading space",
		" leading space/ leading space/ leading space",
		" leading space／ leading space／ leading space",
	},
	{
		"~leading tilde/~leading tilde/~leading tilde",
		"～leading tilde/～leading tilde/～leading tilde",
		"～leading tilde／~leading tilde／~leading tilde",
	},
	{
		"leading tilde/~leading tilde",
		"leading tilde/～leading tilde",
		"leading tilde／~leading tilde",
	},
	{
		"trailing dot./trailing dot./trailing dot.",
		"trailing dot．/trailing dot．/trailing dot．",
		"trailing dot.／trailing dot.／trailing dot．",
	},
}

func benchReplace(b *testing.B, f func(string) string, old bool) {
	for range make([]struct{}, b.N) {
		for _, test := range benchTests {
			got := f(test.in)
			out := test.outNew
			if old {
				out = test.outOld
			}
			if got != out {
				b.Errorf("Encode(%q) want %q got %q", test.in, out, got)
			}
		}
	}
}

func benchRestore(b *testing.B, f func(string) string, old bool) {
	for range make([]struct{}, b.N) {
		for _, test := range benchTests {
			out := test.outNew
			if old {
				out = test.outOld
			}
			got := f(out)
			if got != test.in {
				b.Errorf("Decode(%q) want %q got %q", out, test.in, got)
			}
		}
	}
}

func BenchmarkOneDriveReplaceNew(b *testing.B) {
	benchReplace(b, oneDrive.Encode, false)
}

func BenchmarkOneDriveReplaceOld(b *testing.B) {
	benchReplace(b, replaceReservedChars, true)
}

func BenchmarkOneDriveRestoreNew(b *testing.B) {
	benchRestore(b, oneDrive.Decode, false)
}

func BenchmarkOneDriveRestoreOld(b *testing.B) {
	benchRestore(b, restoreReservedChars, true)
}

var (
	charMap = map[rune]rune{
		'\\': '＼', // FULLWIDTH REVERSE SOLIDUS
		'*':  '＊', // FULLWIDTH ASTERISK
		'<':  '＜', // FULLWIDTH LESS-THAN SIGN
		'>':  '＞', // FULLWIDTH GREATER-THAN SIGN
		'?':  '？', // FULLWIDTH QUESTION MARK
		':':  '：', // FULLWIDTH COLON
		'|':  '｜', // FULLWIDTH VERTICAL LINE
		'#':  '＃', // FULLWIDTH NUMBER SIGN
		'%':  '％', // FULLWIDTH PERCENT SIGN
		'"':  '＂', // FULLWIDTH QUOTATION MARK - not on the list but seems to be reserved
		'.':  '．', // FULLWIDTH FULL STOP
		'~':  '～', // FULLWIDTH TILDE
		' ':  '␠', // SYMBOL FOR SPACE
	}
	invCharMap           map[rune]rune
	fixEndingInPeriod    = regexp.MustCompile(`\.(/|$)`)
	fixEndingWithSpace   = regexp.MustCompile(` (/|$)`)
	fixStartingWithTilde = regexp.MustCompile(`(/|^)~`)
)

func init() {
	// Create inverse charMap
	invCharMap = make(map[rune]rune, len(charMap))
	for k, v := range charMap {
		invCharMap[v] = k
	}
}

// replaceReservedChars takes a path and substitutes any reserved
// characters in it
func replaceReservedChars(in string) string {
	// Folder names can't end with a period '.'
	in = fixEndingInPeriod.ReplaceAllString(in, string(charMap['.'])+"$1")
	// OneDrive for Business file or folder names cannot begin with a tilde '~'
	in = fixStartingWithTilde.ReplaceAllString(in, "$1"+string(charMap['~']))
	// Apparently file names can't start with space either
	in = fixEndingWithSpace.ReplaceAllString(in, string(charMap[' '])+"$1")
	// Encode reserved characters
	return strings.Map(func(c rune) rune {
		if replacement, ok := charMap[c]; ok && c != '.' && c != '~' && c != ' ' {
			return replacement
		}
		return c
	}, in)
}

// restoreReservedChars takes a path and undoes any substitutions
// made by replaceReservedChars
func restoreReservedChars(in string) string {
	return strings.Map(func(c rune) rune {
		if replacement, ok := invCharMap[c]; ok {
			return replacement
		}
		return c
	}, in)
}
