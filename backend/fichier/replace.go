/*
Translate file names for 1fichier

1Fichier reserved characters

The following characters are 1Fichier reserved characters, and can't
be used in 1Fichier  folder and file names.

*/

package fichier

import (
	"regexp"
	"strings"
)

// charMap holds replacements for characters
//
// 1Fichier has a restricted set of characters compared to other cloud
// storage systems, so we to map these to the FULLWIDTH unicode
// equivalents
//
// http://unicode-search.net/unicode-namesearch.pl?term=SOLIDUS
var (
	charMap = map[rune]rune{
		'\\': '＼', // FULLWIDTH REVERSE SOLIDUS
		'<':  '＜', // FULLWIDTH LESS-THAN SIGN
		'>':  '＞', // FULLWIDTH GREATER-THAN SIGN
		'"':  '＂', // FULLWIDTH QUOTATION MARK - not on the list but seems to be reserved
		'\'': '＇', // FULLWIDTH APOSTROPHE
		'$':  '＄', // FULLWIDTH DOLLAR SIGN
		'`':  '｀', // FULLWIDTH GRAVE ACCENT
		' ':  '␠', // SYMBOL FOR SPACE
	}
	invCharMap           map[rune]rune
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
	// file names can't start with space either
	in = fixStartingWithSpace.ReplaceAllString(in, "$1"+string(charMap[' ']))
	// Replace reserved characters
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
