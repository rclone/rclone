// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package useragent

import (
	"bytes"
	"fmt"
	"strings"
)

// This parsing implements useragents based on:
//   https://tools.ietf.org/html/rfc7231#section-5.5.3
//
// User-Agent = product *( RWS ( product / comment ) )
//
// product         = token ["/" product-version]
// product-version = token
//
// token          = 1*tchar
//
// tchar          = "!" / "#" / "$" / "%" / "&" / "'" / "*"
//                / "+" / "-" / "." / "^" / "_" / "`" / "|" / "~"
//                / DIGIT / ALPHA
//                ; any VCHAR, except delimiters
//
// quoted-string  = DQUOTE *( qdtext / quoted-pair ) DQUOTE
// qdtext         = HTAB / SP /%x21 / %x23-5B / %x5D-7E / obs-text
// obs-text       = %x80-FF
//
// comment        = "(" *( ctext / quoted-pair / comment ) ")"
// ctext          = HTAB / SP / %x21-27 / %x2A-5B / %x5D-7E / obs-text
//
// quoted-pair    = "\" ( HTAB / SP / VCHAR / obs-text )
//
// delimiters     = DQUOTE and "(),/:;<=>?@[\]{}".
//
// VCHAR          = %x21-7E ; any 7bit ansi character, except space

// The code below uses variations of
//   `Mozilla/5.0 (Linux; U; Android 4.4.3;)`
// as examples.

// Entry represents a single item in useragent string.
type Entry struct {
	Product string
	Version string
	Comment string
}

// ParseEntries parses every entry in useragent string.
func ParseEntries(data []byte) ([]Entry, error) {
	data = bytes.TrimSpace(data)
	if len(data) == 0 {
		return []Entry{}, nil
	}

	// Parses the first entry, this must not be a comment.
	//  v---------v
	// `Mozilla/5.0    (Linux; U; Android 4.4.3;)`
	e, p, err := parseEntry(data, 0)
	if err != nil {
		return nil, err
	}
	if e.Comment != "" {
		return nil, fmt.Errorf("expected product not a comment @%d", 0)
	}

	entries := []Entry{e}

	for p < len(data) {
		var ok bool
		// Skip the required whitespace.
		//             v-----v
		// `Mozilla/5.0       (Linux; U; Android 4.4.3;)`
		if p, ok = requireWhitespace(data, p); !ok {
			return nil, fmt.Errorf("expected whitespace @%d", p)
		}

		// Parse any other entries, including comments.
		//              v------------------------v
		// `Mozilla/5.0 (Linux; U; Android 4.4.3;)`
		e, p, err = parseEntry(data, p)
		if err != nil {
			return entries, err
		}

		entries = append(entries, e)
	}

	return entries, nil
}

// parseEntry parses either a product or a comment.
//
//	Entry = ( product / comment )
func parseEntry(data []byte, from int) (e Entry, next int, err error) {
	// We are at the start of a character. Example valid input:
	// `Mozilla/5.0`
	// `(Linux; U; Android 4.4.3;)`

	var ok bool

	// This is a comment.
	//  v
	// `(Linux; U; Android 4.4.3;)`
	if _, ok = acceptOne(data, from, '('); ok {
		e.Comment, next, err = parseComment(data, from)
		return e, next, err
	}

	// Otherwise, it must be a product.
	e.Product, e.Version, next, err = parseProduct(data, from)
	return e, next, err
}

// parseProduct parses product with optional version.
//
//	product         = token ["/" product-version]
//	product-version = token
func parseProduct(data []byte, from int) (product, version string, next int, err error) {
	// Example valid input:
	// `Mozilla`
	// `Mozilla/5.0`
	// `Mozilla/5.0-rc.3`

	var ok bool
	p := from
	product, p, ok = parseToken(data, p)
	if !ok {
		return "", "", p, fmt.Errorf("expected product token @%d", p)
	}

	// should we have a version?
	p, ok = acceptOne(data, p, '/')
	if !ok {
		return product, "", p, nil
	}

	// we must have a version
	version, p, ok = parseToken(data, p)
	if !ok {
		return "", "", from, fmt.Errorf("expected version token @%d", p)
	}

	return product, version, p, nil
}

// parseToken parses a token.
//
//	token           = 1*tchar
func parseToken(data []byte, from int) (token string, next int, ok bool) {
	// token is a sequence of token characters.
	// See tchar for the allowed characters.
	next = from
	for ; next < len(data); next++ {
		if !istchar(data[next]) {
			break
		}
	}
	return string(data[from:next]), next, from < next
}

// parseComment parses a potentially comment. This does not allow nesting.
//
//	comment         = "(" *( ctext / quoted-pair / comment ) ")"
//	ctext           = HTAB / SP / %x21-27 / %x2A-5B / %x5D-7E / obs-text
//	quoted-pair     = "\" ( HTAB / SP / VCHAR / obs-text )
//	obs-text        = %x80-FF
func parseComment(data []byte, from int) (comment string, next int, err error) {
	// `(Linux; U; Android 4.4.3;)`
	// `(Linux; \(; Android)`
	// `(Linux; \); Android)`
	//
	// RFC also supports nested comments, however we disallow it:
	// `(Linux; (Android) Quoted)`

	p, ok := acceptOne(data, from, '(')
	if !ok {
		return "", from, fmt.Errorf("expected comment start '(' @%d", p)
	}

	var out strings.Builder
	for {
		if p >= len(data) {
			return "", from, fmt.Errorf("expected comment char @%d", p)
		}
		if data[p] == ')' {
			break
		}
		// divergence from RFC, disallow nested comments
		if data[p] == '(' {
			return "", 0, fmt.Errorf("nested comments not allowed @%d", p)
		}

		var b byte
		b, p, err = parseCommentChar(data, p)
		if err != nil {
			return "", from, err
		}

		_ = out.WriteByte(b)
	}

	p, ok = acceptOne(data, p, ')')
	if !ok {
		return "", from, fmt.Errorf("expected comment end ')' @%d", p)
	}

	return out.String(), p, nil
}

// parseCommentChar parses comment character.
//
//	ctext           = HTAB / SP / %x21-27 / %x2A-5B / %x5D-7E / obs-text
//	quoted-pair     = "\" ( HTAB / SP / VCHAR / obs-text )
//	obs-text        = %x80-FF
func parseCommentChar(data []byte, from int) (ch byte, next int, err error) {
	p := from
	b := data[p]
	p++

	// quoted-pair     = "\" ( HTAB / SP / VCHAR / obs-text )
	if b == '\\' {
		if p >= len(data) {
			return 0, from, fmt.Errorf("unexpected end at @%d", p)
		}
		b = data[p]
		p++
		if !(b == '\t' || b == ' ' || isvchar(b) || isobstext(b)) {
			return 0, from, fmt.Errorf("expected quoted-pair char at @%d", p-1)
		}
		return b, p, nil
	}

	// ctext           = HTAB / SP / %x21-27 / %x2A-5B / %x5D-7E / obs-text
	if b == '\t' || b == ' ' ||
		inrange(b, 0x21, 0x27) ||
		inrange(b, 0x2A, 0x5B) ||
		inrange(b, 0x5D, 0x7E) ||
		isobstext(b) {
		return b, p, nil
	}

	return 0, from, fmt.Errorf("expected ctext, quoted-pair or comment-char @%d, but got %q", p, string(b))
}

func inrange(b byte, from, to byte) bool {
	return from <= b && b <= to
}

func acceptOne(data []byte, from int, b byte) (next int, ok bool) {
	if from < len(data) {
		next = from
		if data[next] == b {
			next++
			return next, true
		}
	}
	return from, false
}

func requireWhitespace(data []byte, from int) (next int, ok bool) {
	next = from
	for ; next < len(data); next++ {
		p := data[next]
		if p != ' ' && p != '\t' {
			return next, from < next
		}
	}
	return next, true
}

// istchar checks for textual-character
//
//	tchar         = "!" / "#" / "$" / "%" / "&" / "'" / "*"
//	/ "+" / "-" / "." / "^" / "_" / "`" / "|" / "~"
//	/ DIGIT / ALPHA
//	; any VCHAR, except delimiters
func istchar(b byte) bool {
	// DIGIT / ALPHA
	if '0' <= b && b <= '9' || 'a' <= b && b <= 'z' || 'A' <= b && b <= 'Z' {
		return true
	}

	switch b {
	// the set of symbols allowed
	case '!', '#', '$', '%', '&', '\'', '*', '+', '-', '.', '^', '_', '`', '|', '~':
		return true
	}

	// otherwise, just verify that it is visible
	return isvchar(b) && !isdelim(b)
}

// isvchar verifies that VCHAR is a US-ASCII visible character.
func isvchar(b byte) bool {
	return 0x21 <= b && b <= 0x7e // TODO: should this be 7f?
}

func isdelim(b byte) bool {
	switch b {
	case '"', '(', ')', ',', '/', ':', ';', '<', '=', '>', '?', '@', '[', '\\', ']', '{', '}':
		return true
	}
	return false
}

func isobstext(b byte) bool {
	return 0x80 <= b && b <= 0xFF
}
