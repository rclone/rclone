/*
Translate file names for usage on restrictive storage systems

The restricted set of characters are mapped to a unicode equivalent version
(most to their FULLWIDTH variant) to increase compatability with other
storage systems.
See: http://unicode-search.net/unicode-namesearch.pl?term=FULLWIDTH

Encoders will also quote reserved characters to differentiate between
the raw and encoded forms.
*/

package encoder

import (
	"bytes"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"
)

const (
	// adding this to any printable ASCII character turns it into the
	// FULLWIDTH variant
	fullOffset = 0xFEE0
	// the first rune of the SYMBOL FOR block for control characters
	symbolOffset = '␀' // SYMBOL FOR NULL
	// QuoteRune is the rune used for quoting reserved characters
	QuoteRune = '‛' // SINGLE HIGH-REVERSED-9 QUOTATION MARK
)

// NB keep the tests in fstests/fstests/fstests.go FsEncoding up to date with this
// NB keep the aliases up to date below also

// Possible flags for the MultiEncoder
const (
	EncodeZero          MultiEncoder = 0         // NUL(0x00)
	EncodeSlash         MultiEncoder = 1 << iota // /
	EncodeLtGt                                   // <>
	EncodeDoubleQuote                            // "
	EncodeSingleQuote                            // '
	EncodeBackQuote                              // `
	EncodeDollar                                 // $
	EncodeColon                                  // :
	EncodeQuestion                               // ?
	EncodeAsterisk                               // *
	EncodePipe                                   // |
	EncodeHash                                   // #
	EncodePercent                                // %
	EncodeBackSlash                              // \
	EncodeCrLf                                   // CR(0x0D), LF(0x0A)
	EncodeDel                                    // DEL(0x7F)
	EncodeCtl                                    // CTRL(0x01-0x1F)
	EncodeLeftSpace                              // Leading SPACE
	EncodeLeftPeriod                             // Leading .
	EncodeLeftTilde                              // Leading ~
	EncodeLeftCrLfHtVt                           // Leading CR LF HT VT
	EncodeRightSpace                             // Trailing SPACE
	EncodeRightPeriod                            // Trailing .
	EncodeRightCrLfHtVt                          // Trailing CR LF HT VT
	EncodeInvalidUtf8                            // Invalid UTF-8 bytes
	EncodeDot                                    // . and .. names

	// Synthetic
	EncodeWin         = EncodeColon | EncodeQuestion | EncodeDoubleQuote | EncodeAsterisk | EncodeLtGt | EncodePipe // :?"*<>|
	EncodeHashPercent = EncodeHash | EncodePercent                                                                  // #%
)

// Has returns true if flag is contained in mask
func (mask MultiEncoder) Has(flag MultiEncoder) bool {
	return mask&flag != 0
}

// Encoder can transform names to and from the original and translated version.
type Encoder interface {
	// Encode takes a raw name and substitutes any reserved characters and
	// patterns in it
	Encode(string) string
	// Decode takes a name and undoes any substitutions made by Encode
	Decode(string) string

	// FromStandardPath takes a / separated path in Standard encoding
	// and converts it to a / separated path in this encoding.
	FromStandardPath(string) string
	// FromStandardName takes name in Standard encoding and converts
	// it in this encoding.
	FromStandardName(string) string
	// ToStandardPath takes a / separated path in this encoding
	// and converts it to a / separated path in Standard encoding.
	ToStandardPath(string) string
	// ToStandardName takes name in this encoding and converts
	// it in Standard encoding.
	ToStandardName(string) string
}

// MultiEncoder is a configurable Encoder. The Encode* constants in this
// package can be combined using bitwise or (|) to enable handling of multiple
// character classes
type MultiEncoder uint

// Aliases maps encodings to names and vice versa
var (
	encodingToName = map[MultiEncoder]string{}
	nameToEncoding = map[string]MultiEncoder{}
)

// alias adds an alias for MultiEncoder.String() and MultiEncoder.Set()
func alias(name string, mask MultiEncoder) {
	nameToEncoding[name] = mask
	// don't overwrite existing reverse translations
	if _, ok := encodingToName[mask]; !ok {
		encodingToName[mask] = name
	}
}

func init() {
	alias("None", EncodeZero)
	alias("Slash", EncodeSlash)
	alias("LtGt", EncodeLtGt)
	alias("DoubleQuote", EncodeDoubleQuote)
	alias("SingleQuote", EncodeSingleQuote)
	alias("BackQuote", EncodeBackQuote)
	alias("Dollar", EncodeDollar)
	alias("Colon", EncodeColon)
	alias("Question", EncodeQuestion)
	alias("Asterisk", EncodeAsterisk)
	alias("Pipe", EncodePipe)
	alias("Hash", EncodeHash)
	alias("Percent", EncodePercent)
	alias("BackSlash", EncodeBackSlash)
	alias("CrLf", EncodeCrLf)
	alias("Del", EncodeDel)
	alias("Ctl", EncodeCtl)
	alias("LeftSpace", EncodeLeftSpace)
	alias("LeftPeriod", EncodeLeftPeriod)
	alias("LeftTilde", EncodeLeftTilde)
	alias("LeftCrLfHtVt", EncodeLeftCrLfHtVt)
	alias("RightSpace", EncodeRightSpace)
	alias("RightPeriod", EncodeRightPeriod)
	alias("RightCrLfHtVt", EncodeRightCrLfHtVt)
	alias("InvalidUtf8", EncodeInvalidUtf8)
	alias("Dot", EncodeDot)
}

// validStrings returns all the valid MultiEncoder strings
func validStrings() string {
	var out []string
	for k := range nameToEncoding {
		out = append(out, k)
	}
	sort.Strings(out)
	return strings.Join(out, ", ")
}

// String converts the MultiEncoder into text
func (mask MultiEncoder) String() string {
	// See if there is an exact translation - if so return that
	if name, ok := encodingToName[mask]; ok {
		return name
	}
	var out []string
	// Otherwise decompose bit by bit
	for bit := MultiEncoder(1); bit != 0; bit *= 2 {
		if (mask & bit) != 0 {
			if name, ok := encodingToName[bit]; ok {
				out = append(out, name)
			} else {
				out = append(out, fmt.Sprintf("0x%X", uint(bit)))
			}
		}
	}
	return strings.Join(out, ",")
}

// Set converts a string into a MultiEncoder
func (mask *MultiEncoder) Set(in string) error {
	var out MultiEncoder
	parts := strings.Split(in, ",")
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if bits, ok := nameToEncoding[part]; ok {
			out |= bits
		} else {
			i, err := strconv.ParseInt(part, 0, 64)
			if err != nil {
				return fmt.Errorf("bad encoding %q: possible values are: %s", part, validStrings())
			}
			out |= MultiEncoder(i)
		}
	}
	*mask = out
	return nil
}

// Type returns a textual type of the MultiEncoder to satsify the pflag.Value interface
func (mask MultiEncoder) Type() string {
	return "Encoding"
}

// Scan implements the fmt.Scanner interface
func (mask *MultiEncoder) Scan(s fmt.ScanState, ch rune) error {
	token, err := s.Token(true, nil)
	if err != nil {
		return err
	}
	return mask.Set(string(token))
}

// Encode takes a raw name and substitutes any reserved characters and
// patterns in it
func (mask MultiEncoder) Encode(in string) string {
	if in == "" {
		return ""
	}

	if mask.Has(EncodeDot) {
		switch in {
		case ".":
			return "．"
		case "..":
			return "．．"
		case "．":
			return string(QuoteRune) + "．"
		case "．．":
			return string(QuoteRune) + "．" + string(QuoteRune) + "．"
		}
	}

	// handle prefix only replacements
	prefix := ""
	if mask.Has(EncodeLeftSpace) { // Leading SPACE
		if in[0] == ' ' {
			prefix, in = "␠", in[1:] // SYMBOL FOR SPACE
		} else if r, l := utf8.DecodeRuneInString(in); r == '␠' { // SYMBOL FOR SPACE
			prefix, in = string(QuoteRune)+"␠", in[l:] // SYMBOL FOR SPACE
		}
	}
	if mask.Has(EncodeLeftPeriod) && prefix == "" { // Leading PERIOD
		if in[0] == '.' {
			prefix, in = "．", in[1:] // FULLWIDTH FULL STOP
		} else if r, l := utf8.DecodeRuneInString(in); r == '．' { // FULLWIDTH FULL STOP
			prefix, in = string(QuoteRune)+"．", in[l:] //  FULLWIDTH FULL STOP
		}
	}
	if mask.Has(EncodeLeftTilde) && prefix == "" { // Leading ~
		if in[0] == '~' {
			prefix, in = string('~'+fullOffset), in[1:] // FULLWIDTH TILDE
		} else if r, l := utf8.DecodeRuneInString(in); r == '~'+fullOffset {
			prefix, in = string(QuoteRune)+string('~'+fullOffset), in[l:] // FULLWIDTH TILDE
		}
	}
	if mask.Has(EncodeLeftCrLfHtVt) && prefix == "" { // Leading CR LF HT VT
		switch c := in[0]; c {
		case '\t', '\n', '\v', '\r':
			prefix, in = string('␀'+rune(c)), in[1:] // SYMBOL FOR NULL
		default:
			switch r, l := utf8.DecodeRuneInString(in); r {
			case '␀' + '\t', '␀' + '\n', '␀' + '\v', '␀' + '\r':
				prefix, in = string(QuoteRune)+string(r), in[l:]
			}
		}
	}
	// handle suffix only replacements
	suffix := ""
	if in != "" {
		if mask.Has(EncodeRightSpace) { // Trailing SPACE
			if in[len(in)-1] == ' ' {
				suffix, in = "␠", in[:len(in)-1] // SYMBOL FOR SPACE
			} else if r, l := utf8.DecodeLastRuneInString(in); r == '␠' {
				suffix, in = string(QuoteRune)+"␠", in[:len(in)-l] // SYMBOL FOR SPACE
			}
		}
		if mask.Has(EncodeRightPeriod) && suffix == "" { // Trailing .
			if in[len(in)-1] == '.' {
				suffix, in = "．", in[:len(in)-1] // FULLWIDTH FULL STOP
			} else if r, l := utf8.DecodeLastRuneInString(in); r == '．' {
				suffix, in = string(QuoteRune)+"．", in[:len(in)-l] // FULLWIDTH FULL STOP
			}
		}
		if mask.Has(EncodeRightCrLfHtVt) && suffix == "" { // Trailing .
			switch c := in[len(in)-1]; c {
			case '\t', '\n', '\v', '\r':
				suffix, in = string('␀'+rune(c)), in[:len(in)-1] // FULLWIDTH FULL STOP
			default:
				switch r, l := utf8.DecodeLastRuneInString(in); r {
				case '␀' + '\t', '␀' + '\n', '␀' + '\v', '␀' + '\r':
					suffix, in = string(QuoteRune)+string(r), in[:len(in)-l]
				}
			}
		}
	}

	index := 0
	if prefix == "" && suffix == "" {
		// find the first rune which (most likely) needs to be replaced
		index = strings.IndexFunc(in, func(r rune) bool {
			switch r {
			case 0, '␀', QuoteRune, utf8.RuneError:
				return true
			}
			if mask.Has(EncodeAsterisk) { // *
				switch r {
				case '*',
					'＊':
					return true
				}
			}
			if mask.Has(EncodeLtGt) { // <>
				switch r {
				case '<', '>',
					'＜', '＞':
					return true
				}
			}
			if mask.Has(EncodeQuestion) { // ?
				switch r {
				case '?',
					'？':
					return true
				}
			}
			if mask.Has(EncodeColon) { // :
				switch r {
				case ':',
					'：':
					return true
				}
			}
			if mask.Has(EncodePipe) { // |
				switch r {
				case '|',
					'｜':
					return true
				}
			}
			if mask.Has(EncodeDoubleQuote) { // "
				switch r {
				case '"',
					'＂':
					return true
				}
			}
			if mask.Has(EncodeSingleQuote) { // '
				switch r {
				case '\'',
					'＇':
					return true
				}
			}
			if mask.Has(EncodeBackQuote) { // `
				switch r {
				case '`',
					'｀':
					return true
				}
			}
			if mask.Has(EncodeDollar) { // $
				switch r {
				case '$',
					'＄':
					return true
				}
			}
			if mask.Has(EncodeSlash) { // /
				switch r {
				case '/',
					'／':
					return true
				}
			}
			if mask.Has(EncodeBackSlash) { // \
				switch r {
				case '\\',
					'＼':
					return true
				}
			}
			if mask.Has(EncodeCrLf) { // CR LF
				switch r {
				case rune(0x0D), rune(0x0A),
					'␍', '␊':
					return true
				}
			}
			if mask.Has(EncodeHash) { // #
				switch r {
				case '#',
					'＃':
					return true
				}
			}
			if mask.Has(EncodePercent) { // %
				switch r {
				case '%',
					'％':
					return true
				}
			}
			if mask.Has(EncodeDel) { // DEL(0x7F)
				switch r {
				case rune(0x7F), '␡':
					return true
				}
			}
			if mask.Has(EncodeCtl) { // CTRL(0x01-0x1F)
				if r >= 1 && r <= 0x1F {
					return true
				} else if r > symbolOffset && r <= symbolOffset+0x1F {
					return true
				}
			}
			return false
		})
	}
	// nothing to replace, return input
	if index == -1 {
		return in
	}

	var out bytes.Buffer
	out.Grow(len(in) + len(prefix) + len(suffix))
	out.WriteString(prefix)
	// copy the clean part of the input and skip it
	out.WriteString(in[:index])
	in = in[index:]

	for i, r := range in {
		switch r {
		case 0:
			out.WriteRune(symbolOffset)
			continue
		case '␀', QuoteRune:
			out.WriteRune(QuoteRune)
			out.WriteRune(r)
			continue
		case utf8.RuneError:
			if mask.Has(EncodeInvalidUtf8) {
				// only encode invalid sequences and not utf8.RuneError
				if i+3 > len(in) || in[i:i+3] != string(utf8.RuneError) {
					_, l := utf8.DecodeRuneInString(in[i:])
					appendQuotedBytes(&out, in[i:i+l])
					continue
				}
			} else {
				// append the real bytes instead of utf8.RuneError
				_, l := utf8.DecodeRuneInString(in[i:])
				out.WriteString(in[i : i+l])
				continue
			}
		}
		if mask.Has(EncodeAsterisk) { // *
			switch r {
			case '*':
				out.WriteRune(r + fullOffset)
				continue
			case '＊':
				out.WriteRune(QuoteRune)
				out.WriteRune(r)
				continue
			}
		}
		if mask.Has(EncodeLtGt) { // <>
			switch r {
			case '<', '>':
				out.WriteRune(r + fullOffset)
				continue
			case '＜', '＞':
				out.WriteRune(QuoteRune)
				out.WriteRune(r)
				continue
			}
		}
		if mask.Has(EncodeQuestion) { // ?
			switch r {
			case '?':
				out.WriteRune(r + fullOffset)
				continue
			case '？':
				out.WriteRune(QuoteRune)
				out.WriteRune(r)
				continue
			}
		}
		if mask.Has(EncodeColon) { // :
			switch r {
			case ':':
				out.WriteRune(r + fullOffset)
				continue
			case '：':
				out.WriteRune(QuoteRune)
				out.WriteRune(r)
				continue
			}
		}
		if mask.Has(EncodePipe) { // |
			switch r {
			case '|':
				out.WriteRune(r + fullOffset)
				continue
			case '｜':
				out.WriteRune(QuoteRune)
				out.WriteRune(r)
				continue
			}
		}
		if mask.Has(EncodeDoubleQuote) { // "
			switch r {
			case '"':
				out.WriteRune(r + fullOffset)
				continue
			case '＂':
				out.WriteRune(QuoteRune)
				out.WriteRune(r)
				continue
			}
		}
		if mask.Has(EncodeSingleQuote) { // '
			switch r {
			case '\'':
				out.WriteRune(r + fullOffset)
				continue
			case '＇':
				out.WriteRune(QuoteRune)
				out.WriteRune(r)
				continue
			}
		}
		if mask.Has(EncodeBackQuote) { // `
			switch r {
			case '`':
				out.WriteRune(r + fullOffset)
				continue
			case '｀':
				out.WriteRune(QuoteRune)
				out.WriteRune(r)
				continue
			}
		}
		if mask.Has(EncodeDollar) { // $
			switch r {
			case '$':
				out.WriteRune(r + fullOffset)
				continue
			case '＄':
				out.WriteRune(QuoteRune)
				out.WriteRune(r)
				continue
			}
		}
		if mask.Has(EncodeSlash) { // /
			switch r {
			case '/':
				out.WriteRune(r + fullOffset)
				continue
			case '／':
				out.WriteRune(QuoteRune)
				out.WriteRune(r)
				continue
			}
		}
		if mask.Has(EncodeBackSlash) { // \
			switch r {
			case '\\':
				out.WriteRune(r + fullOffset)
				continue
			case '＼':
				out.WriteRune(QuoteRune)
				out.WriteRune(r)
				continue
			}
		}
		if mask.Has(EncodeCrLf) { // CR LF
			switch r {
			case rune(0x0D), rune(0x0A):
				out.WriteRune(r + symbolOffset)
				continue
			case '␍', '␊':
				out.WriteRune(QuoteRune)
				out.WriteRune(r)
				continue
			}
		}
		if mask.Has(EncodeHash) { // #
			switch r {
			case '#':
				out.WriteRune(r + fullOffset)
				continue
			case '＃':
				out.WriteRune(QuoteRune)
				out.WriteRune(r)
				continue
			}
		}
		if mask.Has(EncodePercent) { // %
			switch r {
			case '%':
				out.WriteRune(r + fullOffset)
				continue
			case '％':
				out.WriteRune(QuoteRune)
				out.WriteRune(r)
				continue
			}
		}
		if mask.Has(EncodeDel) { // DEL(0x7F)
			switch r {
			case rune(0x7F):
				out.WriteRune('␡') // SYMBOL FOR DELETE
				continue
			case '␡':
				out.WriteRune(QuoteRune)
				out.WriteRune(r)
				continue
			}
		}
		if mask.Has(EncodeCtl) { // CTRL(0x01-0x1F)
			if r >= 1 && r <= 0x1F {
				out.WriteRune('␀' + r) // SYMBOL FOR NULL
				continue
			} else if r > symbolOffset && r <= symbolOffset+0x1F {
				out.WriteRune(QuoteRune)
				out.WriteRune(r)
				continue
			}
		}
		out.WriteRune(r)
	}
	out.WriteString(suffix)
	return out.String()
}

// Decode takes a name and undoes any substitutions made by Encode
func (mask MultiEncoder) Decode(in string) string {
	if mask.Has(EncodeDot) {
		switch in {
		case "．":
			return "."
		case "．．":
			return ".."
		case string(QuoteRune) + "．":
			return "．"
		case string(QuoteRune) + "．" + string(QuoteRune) + "．":
			return "．．"
		}
	}

	// handle prefix only replacements
	prefix := ""
	if r, l1 := utf8.DecodeRuneInString(in); mask.Has(EncodeLeftSpace) && r == '␠' { // SYMBOL FOR SPACE
		prefix, in = " ", in[l1:]
	} else if mask.Has(EncodeLeftPeriod) && r == '．' { // FULLWIDTH FULL STOP
		prefix, in = ".", in[l1:]
	} else if mask.Has(EncodeLeftTilde) && r == '～' { // FULLWIDTH TILDE
		prefix, in = "~", in[l1:]
	} else if mask.Has(EncodeLeftCrLfHtVt) && (r == '␀'+'\t' || r == '␀'+'\n' || r == '␀'+'\v' || r == '␀'+'\r') {
		prefix, in = string(r-'␀'), in[l1:]
	} else if r == QuoteRune {
		if r, l2 := utf8.DecodeRuneInString(in[l1:]); mask.Has(EncodeLeftSpace) && r == '␠' { // SYMBOL FOR SPACE
			prefix, in = "␠", in[l1+l2:]
		} else if mask.Has(EncodeLeftPeriod) && r == '．' { // FULLWIDTH FULL STOP
			prefix, in = "．", in[l1+l2:]
		} else if mask.Has(EncodeLeftTilde) && r == '～' { // FULLWIDTH TILDE
			prefix, in = "～", in[l1+l2:]
		} else if mask.Has(EncodeLeftCrLfHtVt) && (r == '␀'+'\t' || r == '␀'+'\n' || r == '␀'+'\v' || r == '␀'+'\r') {
			prefix, in = string(r), in[l1+l2:]
		}
	}

	// handle suffix only replacements
	suffix := ""
	if r, l := utf8.DecodeLastRuneInString(in); mask.Has(EncodeRightSpace) && r == '␠' { // SYMBOL FOR SPACE
		in = in[:len(in)-l]
		if q, l2 := utf8.DecodeLastRuneInString(in); q == QuoteRune {
			suffix, in = "␠", in[:len(in)-l2]
		} else {
			suffix = " "
		}
	} else if mask.Has(EncodeRightPeriod) && r == '．' { // FULLWIDTH FULL STOP
		in = in[:len(in)-l]
		if q, l2 := utf8.DecodeLastRuneInString(in); q == QuoteRune {
			suffix, in = "．", in[:len(in)-l2]
		} else {
			suffix = "."
		}
	} else if mask.Has(EncodeRightCrLfHtVt) && (r == '␀'+'\t' || r == '␀'+'\n' || r == '␀'+'\v' || r == '␀'+'\r') {
		in = in[:len(in)-l]
		if q, l2 := utf8.DecodeLastRuneInString(in); q == QuoteRune {
			suffix, in = string(r), in[:len(in)-l2]
		} else {
			suffix = string(r - '␀')
		}
	}
	index := 0
	if prefix == "" && suffix == "" {
		// find the first rune which (most likely) needs to be replaced
		index = strings.IndexFunc(in, func(r rune) bool {
			switch r {
			case '␀', QuoteRune:
				return true
			}
			if mask.Has(EncodeAsterisk) { // *
				switch r {
				case '＊':
					return true
				}
			}
			if mask.Has(EncodeLtGt) { // <>
				switch r {
				case '＜', '＞':
					return true
				}
			}
			if mask.Has(EncodeQuestion) { // ?
				switch r {
				case '？':
					return true
				}
			}
			if mask.Has(EncodeColon) { // :
				switch r {
				case '：':
					return true
				}
			}
			if mask.Has(EncodePipe) { // |
				switch r {
				case '｜':
					return true
				}
			}
			if mask.Has(EncodeDoubleQuote) { // "
				switch r {
				case '＂':
					return true
				}
			}
			if mask.Has(EncodeSingleQuote) { // '
				switch r {
				case '＇':
					return true
				}
			}
			if mask.Has(EncodeBackQuote) { // `
				switch r {
				case '｀':
					return true
				}
			}
			if mask.Has(EncodeDollar) { // $
				switch r {
				case '＄':
					return true
				}
			}
			if mask.Has(EncodeSlash) { // /
				switch r {
				case '／':
					return true
				}
			}
			if mask.Has(EncodeBackSlash) { // \
				switch r {
				case '＼':
					return true
				}
			}
			if mask.Has(EncodeCrLf) { // CR LF
				switch r {
				case '␍', '␊':
					return true
				}
			}
			if mask.Has(EncodeHash) { // #
				switch r {
				case '＃':
					return true
				}
			}
			if mask.Has(EncodePercent) { // %
				switch r {
				case '％':
					return true
				}
			}
			if mask.Has(EncodeDel) { // DEL(0x7F)
				switch r {
				case '␡':
					return true
				}
			}
			if mask.Has(EncodeCtl) { // CTRL(0x01-0x1F)
				if r > symbolOffset && r <= symbolOffset+0x1F {
					return true
				}
			}

			return false
		})
	}
	// nothing to replace, return input
	if index == -1 {
		return in
	}

	var out bytes.Buffer
	out.Grow(len(in))
	out.WriteString(prefix)
	// copy the clean part of the input and skip it
	out.WriteString(in[:index])
	in = in[index:]
	var unquote, unquoteNext, skipNext bool

	for i, r := range in {
		if skipNext {
			skipNext = false
			continue
		}
		unquote, unquoteNext = unquoteNext, false
		switch r {
		case '␀': // SYMBOL FOR NULL
			if unquote {
				out.WriteRune(r)
			} else {
				out.WriteRune(0)
			}
			continue
		case QuoteRune:
			if unquote {
				out.WriteRune(r)
			} else {
				unquoteNext = true
			}
			continue
		}
		if mask.Has(EncodeAsterisk) { // *
			switch r {
			case '＊':
				if unquote {
					out.WriteRune(r)
				} else {
					out.WriteRune(r - fullOffset)
				}
				continue
			}
		}
		if mask.Has(EncodeLtGt) { // <>
			switch r {
			case '＜', '＞':
				if unquote {
					out.WriteRune(r)
				} else {
					out.WriteRune(r - fullOffset)
				}
				continue
			}
		}
		if mask.Has(EncodeQuestion) { // ?
			switch r {
			case '？':
				if unquote {
					out.WriteRune(r)
				} else {
					out.WriteRune(r - fullOffset)
				}
				continue
			}
		}
		if mask.Has(EncodeColon) { // :
			switch r {
			case '：':
				if unquote {
					out.WriteRune(r)
				} else {
					out.WriteRune(r - fullOffset)
				}
				continue
			}
		}
		if mask.Has(EncodePipe) { // |
			switch r {
			case '｜':
				if unquote {
					out.WriteRune(r)
				} else {
					out.WriteRune(r - fullOffset)
				}
				continue
			}
		}
		if mask.Has(EncodeDoubleQuote) { // "
			switch r {
			case '＂':
				if unquote {
					out.WriteRune(r)
				} else {
					out.WriteRune(r - fullOffset)
				}
				continue
			}
		}
		if mask.Has(EncodeSingleQuote) { // '
			switch r {
			case '＇':
				if unquote {
					out.WriteRune(r)
				} else {
					out.WriteRune(r - fullOffset)
				}
				continue
			}
		}
		if mask.Has(EncodeBackQuote) { // `
			switch r {
			case '｀':
				if unquote {
					out.WriteRune(r)
				} else {
					out.WriteRune(r - fullOffset)
				}
				continue
			}
		}
		if mask.Has(EncodeDollar) { // $
			switch r {
			case '＄':
				if unquote {
					out.WriteRune(r)
				} else {
					out.WriteRune(r - fullOffset)
				}
				continue
			}
		}
		if mask.Has(EncodeSlash) { // /
			switch r {
			case '／': // FULLWIDTH SOLIDUS
				if unquote {
					out.WriteRune(r)
				} else {
					out.WriteRune(r - fullOffset)
				}
				continue
			}
		}
		if mask.Has(EncodeBackSlash) { // \
			switch r {
			case '＼': // FULLWIDTH REVERSE SOLIDUS
				if unquote {
					out.WriteRune(r)
				} else {
					out.WriteRune(r - fullOffset)
				}
				continue
			}
		}
		if mask.Has(EncodeCrLf) { // CR LF
			switch r {
			case '␍', '␊':
				if unquote {
					out.WriteRune(r)
				} else {
					out.WriteRune(r - symbolOffset)
				}
				continue
			}
		}
		if mask.Has(EncodeHash) { // %
			switch r {
			case '＃':
				if unquote {
					out.WriteRune(r)
				} else {
					out.WriteRune(r - fullOffset)
				}
				continue
			}
		}
		if mask.Has(EncodePercent) { // %
			switch r {
			case '％':
				if unquote {
					out.WriteRune(r)
				} else {
					out.WriteRune(r - fullOffset)
				}
				continue
			}
		}
		if mask.Has(EncodeDel) { // DEL(0x7F)
			switch r {
			case '␡': // SYMBOL FOR DELETE
				if unquote {
					out.WriteRune(r)
				} else {
					out.WriteRune(0x7F)
				}
				continue
			}
		}
		if mask.Has(EncodeCtl) { // CTRL(0x01-0x1F)
			if r > symbolOffset && r <= symbolOffset+0x1F {
				if unquote {
					out.WriteRune(r)
				} else {
					out.WriteRune(r - symbolOffset)
				}
				continue
			}
		}
		if unquote {
			if mask.Has(EncodeInvalidUtf8) {
				skipNext = appendUnquotedByte(&out, in[i:])
				if skipNext {
					continue
				}
			}
			out.WriteRune(QuoteRune)
		}
		switch r {
		case utf8.RuneError:
			// append the real bytes instead of utf8.RuneError
			_, l := utf8.DecodeRuneInString(in[i:])
			out.WriteString(in[i : i+l])
			continue
		}

		out.WriteRune(r)
	}
	if unquoteNext {
		out.WriteRune(QuoteRune)
	}
	out.WriteString(suffix)
	return out.String()
}

// FromStandardPath takes a / separated path in Standard encoding
// and converts it to a / separated path in this encoding.
func (mask MultiEncoder) FromStandardPath(s string) string {
	return FromStandardPath(mask, s)
}

// FromStandardName takes name in Standard encoding and converts
// it in this encoding.
func (mask MultiEncoder) FromStandardName(s string) string {
	return FromStandardName(mask, s)
}

// ToStandardPath takes a / separated path in this encoding
// and converts it to a / separated path in Standard encoding.
func (mask MultiEncoder) ToStandardPath(s string) string {
	return ToStandardPath(mask, s)
}

// ToStandardName takes name in this encoding and converts
// it in Standard encoding.
func (mask MultiEncoder) ToStandardName(s string) string {
	return ToStandardName(mask, s)
}

func appendQuotedBytes(w io.Writer, s string) {
	for _, b := range []byte(s) {
		_, _ = fmt.Fprintf(w, string(QuoteRune)+"%02X", b)
	}
}
func appendUnquotedByte(w io.Writer, s string) bool {
	if len(s) < 2 {
		return false
	}
	u, err := strconv.ParseUint(s[:2], 16, 8)
	if err != nil {
		return false
	}
	n, _ := w.Write([]byte{byte(u)})
	return n == 1
}

type identity struct{}

func (identity) Encode(in string) string { return in }
func (identity) Decode(in string) string { return in }

func (i identity) FromStandardPath(s string) string {
	return FromStandardPath(i, s)
}
func (i identity) FromStandardName(s string) string {
	return FromStandardName(i, s)
}
func (i identity) ToStandardPath(s string) string {
	return ToStandardPath(i, s)
}
func (i identity) ToStandardName(s string) string {
	return ToStandardName(i, s)
}

// Identity returns a Encoder that always returns the input value
func Identity() Encoder {
	return identity{}
}

// FromStandardPath takes a / separated path in Standard encoding
// and converts it to a / separated path in the given encoding.
func FromStandardPath(e Encoder, s string) string {
	if e == Standard {
		return s
	}
	parts := strings.Split(s, "/")
	encoded := make([]string, len(parts))
	changed := false
	for i, p := range parts {
		enc := FromStandardName(e, p)
		changed = changed || enc != p
		encoded[i] = enc
	}
	if !changed {
		return s
	}
	return strings.Join(encoded, "/")
}

// FromStandardName takes name in Standard encoding and converts
// it in the given encoding.
func FromStandardName(e Encoder, s string) string {
	if e == Standard {
		return s
	}
	return e.Encode(Standard.Decode(s))
}

// ToStandardPath takes a / separated path in the given encoding
// and converts it to a / separated path in Standard encoding.
func ToStandardPath(e Encoder, s string) string {
	if e == Standard {
		return s
	}
	parts := strings.Split(s, "/")
	encoded := make([]string, len(parts))
	changed := false
	for i, p := range parts {
		dec := ToStandardName(e, p)
		changed = changed || dec != p
		encoded[i] = dec
	}
	if !changed {
		return s
	}
	return strings.Join(encoded, "/")
}

// ToStandardName takes name in the given encoding and converts
// it in Standard encoding.
func ToStandardName(e Encoder, s string) string {
	if e == Standard {
		return s
	}
	return Standard.Encode(e.Decode(s))
}
