package encoder

import (
	"regexp"
	"strconv"
	"strings"
	"testing"
)

type testCase struct {
	mask uint
	in   string
	out  string
}

func TestEncodeSingleMask(t *testing.T) {
	for i, tc := range testCasesSingle {
		e := MultiEncoder(tc.mask)
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
		e := MultiEncoder(tc.mask)
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
		e := MultiEncoder(tc.mask)
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
			mask: 0,
			in:   ".",
			out:  ".",
		}, {
			mask: EncodeDot,
			in:   ".",
			out:  "．",
		}, {
			mask: 0,
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
		e := MultiEncoder(tc.mask)
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
			mask: 0,
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
		e := MultiEncoder(tc.mask)
		t.Run(strconv.FormatInt(int64(i), 10), func(t *testing.T) {
			got := e.Decode(tc.in)
			if got != tc.out {
				t.Errorf("Decode(%q) want %q got %q", tc.in, tc.out, got)
			}
		})
	}
}

const oneDrive = MultiEncoder(
	uint(Standard) |
		EncodeWin |
		EncodeBackSlash |
		EncodeHashPercent |
		EncodeDel |
		EncodeCtl |
		EncodeLeftTilde |
		EncodeRightSpace |
		EncodeRightPeriod)

var benchTests = []struct {
	in  string
	out string
}{
	{"", ""},
	{"abc 123", "abc 123"},
	{`\*<>?:|#%".~`, `＼＊＜＞？：｜＃％＂.~`},
	{`\*<>?:|#%".~/\*<>?:|#%".~`, `＼＊＜＞？：｜＃％＂.~/＼＊＜＞？：｜＃％＂.~`},
	{" leading space", " leading space"},
	{"~leading tilde", "～leading tilde"},
	{"trailing dot.", "trailing dot．"},
	{" leading space/ leading space/ leading space", " leading space/ leading space/ leading space"},
	{"~leading tilde/~leading tilde/~leading tilde", "～leading tilde/～leading tilde/～leading tilde"},
	{"leading tilde/~leading tilde", "leading tilde/～leading tilde"},
	{"trailing dot./trailing dot./trailing dot.", "trailing dot．/trailing dot．/trailing dot．"},
}

func benchReplace(b *testing.B, f func(string) string) {
	for range make([]struct{}, b.N) {
		for _, test := range benchTests {
			got := f(test.in)
			if got != test.out {
				b.Errorf("Encode(%q) want %q got %q", test.in, test.out, got)
			}
		}
	}
}

func benchRestore(b *testing.B, f func(string) string) {
	for range make([]struct{}, b.N) {
		for _, test := range benchTests {
			got := f(test.out)
			if got != test.in {
				b.Errorf("Decode(%q) want %q got %q", got, test.in, got)
			}
		}
	}
}
func BenchmarkOneDriveReplaceNew(b *testing.B) {
	benchReplace(b, oneDrive.Encode)
}
func BenchmarkOneDriveReplaceOld(b *testing.B) {
	benchReplace(b, replaceReservedChars)
}
func BenchmarkOneDriveRestoreNew(b *testing.B) {
	benchRestore(b, oneDrive.Decode)
}
func BenchmarkOneDriveRestoreOld(b *testing.B) {
	benchRestore(b, restoreReservedChars)
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
