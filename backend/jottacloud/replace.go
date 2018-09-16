/*
Translate file names for JottaCloud adapted from OneDrive


The following characters are JottaClous reserved characters, and can't
be used in JottaCloud folder and file names.

  jottacloud  = "/" / "\" / "*" / "<" / ">" / "?" / "!" / "&" / ":" / ";" / "|" / "#" / "%" / """ / "'" / "." / "~"


*/

package jottacloud

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
		';':  '；', // FULLWIDTH SEMICOLON
		'|':  '｜', // FULLWIDTH VERTICAL LINE
		'"':  '＂', // FULLWIDTH QUOTATION MARK - not on the list but seems to be reserved
		' ':  '␠', // SYMBOL FOR SPACE
	}
	invCharMap           map[rune]rune
	fixStartingWithSpace = regexp.MustCompile(`(/|^) `)
	fixEndingWithSpace   = regexp.MustCompile(` (/|$)`)
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
