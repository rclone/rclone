/*
Translate file names for OpenDrive

OpenDrive reserved characters

The following characters are OpenDrive reserved characters, and can't
be used in OpenDrive folder and file names.

\ / : * ? " < > |

OpenDrive files and folders can't have leading or trailing spaces also.

*/

package opendrive

import (
	"regexp"
	"strings"
)

// charMap holds replacements for characters
//
// OpenDrive has a restricted set of characters compared to other cloud
// storage systems, so we to map these to the FULLWIDTH unicode
// equivalents
//
// http://unicode-search.net/unicode-namesearch.pl?term=SOLIDUS
var (
	charMap = map[rune]rune{
		'\\': '＼', // FULLWIDTH REVERSE SOLIDUS
		':':  '：', // FULLWIDTH COLON
		'*':  '＊', // FULLWIDTH ASTERISK
		'?':  '？', // FULLWIDTH QUESTION MARK
		'"':  '＂', // FULLWIDTH QUOTATION MARK
		'<':  '＜', // FULLWIDTH LESS-THAN SIGN
		'>':  '＞', // FULLWIDTH GREATER-THAN SIGN
		'|':  '｜', // FULLWIDTH VERTICAL LINE
		' ':  '␠', // SYMBOL FOR SPACE
	}
	fixStartingWithSpace = regexp.MustCompile(`(/|^) `)
	fixEndingWithSpace   = regexp.MustCompile(` (/|$)`)
	invCharMap           map[rune]rune
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
	// Filenames can't start with space
	in = fixStartingWithSpace.ReplaceAllString(in, "$1"+string(charMap[' ']))
	// Filenames can't end with space
	in = fixEndingWithSpace.ReplaceAllString(in, string(charMap[' '])+"$1")
	return strings.Map(func(c rune) rune {
		if replacement, ok := charMap[c]; ok && c != ' ' {
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
