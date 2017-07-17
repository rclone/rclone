/*
Translate file names for OpenDrive

OpenDrive reserved characters

The following characters are OpenDrive reserved characters, and can't
be used in OpenDrive folder and file names.

\\ / : * ? \" < > |"

*/

package opendrive

import (
	"regexp"
	"strings"
)

// charMap holds replacements for characters
//
// Onedrive has a restricted set of characters compared to other cloud
// storage systems, so we to map these to the FULLWIDTH unicode
// equivalents
//
// http://unicode-search.net/unicode-namesearch.pl?term=SOLIDUS
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
	fixStartingWithTilde = regexp.MustCompile(`(/|^)~`)
	fixStartingWithSpace = regexp.MustCompile(`(/|^) `)
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
	in = fixStartingWithSpace.ReplaceAllString(in, "$1"+string(charMap[' ']))
	// Replace reserved characters
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
