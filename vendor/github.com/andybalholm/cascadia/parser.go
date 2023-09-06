// Package cascadia is an implementation of CSS selectors.
package cascadia

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// a parser for CSS selectors
type parser struct {
	s string // the source text
	i int    // the current position

	// if `false`, parsing a pseudo-element
	// returns an error.
	acceptPseudoElements bool
}

// parseEscape parses a backslash escape.
func (p *parser) parseEscape() (result string, err error) {
	if len(p.s) < p.i+2 || p.s[p.i] != '\\' {
		return "", errors.New("invalid escape sequence")
	}

	start := p.i + 1
	c := p.s[start]
	switch {
	case c == '\r' || c == '\n' || c == '\f':
		return "", errors.New("escaped line ending outside string")
	case hexDigit(c):
		// unicode escape (hex)
		var i int
		for i = start; i < start+6 && i < len(p.s) && hexDigit(p.s[i]); i++ {
			// empty
		}
		v, _ := strconv.ParseUint(p.s[start:i], 16, 64)
		if len(p.s) > i {
			switch p.s[i] {
			case '\r':
				i++
				if len(p.s) > i && p.s[i] == '\n' {
					i++
				}
			case ' ', '\t', '\n', '\f':
				i++
			}
		}
		p.i = i
		return string(rune(v)), nil
	}

	// Return the literal character after the backslash.
	result = p.s[start : start+1]
	p.i += 2
	return result, nil
}

// toLowerASCII returns s with all ASCII capital letters lowercased.
func toLowerASCII(s string) string {
	var b []byte
	for i := 0; i < len(s); i++ {
		if c := s[i]; 'A' <= c && c <= 'Z' {
			if b == nil {
				b = make([]byte, len(s))
				copy(b, s)
			}
			b[i] = s[i] + ('a' - 'A')
		}
	}

	if b == nil {
		return s
	}

	return string(b)
}

func hexDigit(c byte) bool {
	return '0' <= c && c <= '9' || 'a' <= c && c <= 'f' || 'A' <= c && c <= 'F'
}

// nameStart returns whether c can be the first character of an identifier
// (not counting an initial hyphen, or an escape sequence).
func nameStart(c byte) bool {
	return 'a' <= c && c <= 'z' || 'A' <= c && c <= 'Z' || c == '_' || c > 127
}

// nameChar returns whether c can be a character within an identifier
// (not counting an escape sequence).
func nameChar(c byte) bool {
	return 'a' <= c && c <= 'z' || 'A' <= c && c <= 'Z' || c == '_' || c > 127 ||
		c == '-' || '0' <= c && c <= '9'
}

// parseIdentifier parses an identifier.
func (p *parser) parseIdentifier() (result string, err error) {
	const prefix = '-'
	var numPrefix int

	for len(p.s) > p.i && p.s[p.i] == prefix {
		p.i++
		numPrefix++
	}

	if len(p.s) <= p.i {
		return "", errors.New("expected identifier, found EOF instead")
	}

	if c := p.s[p.i]; !(nameStart(c) || c == '\\') {
		return "", fmt.Errorf("expected identifier, found %c instead", c)
	}

	result, err = p.parseName()
	if numPrefix > 0 && err == nil {
		result = strings.Repeat(string(prefix), numPrefix) + result
	}
	return
}

// parseName parses a name (which is like an identifier, but doesn't have
// extra restrictions on the first character).
func (p *parser) parseName() (result string, err error) {
	i := p.i
loop:
	for i < len(p.s) {
		c := p.s[i]
		switch {
		case nameChar(c):
			start := i
			for i < len(p.s) && nameChar(p.s[i]) {
				i++
			}
			result += p.s[start:i]
		case c == '\\':
			p.i = i
			val, err := p.parseEscape()
			if err != nil {
				return "", err
			}
			i = p.i
			result += val
		default:
			break loop
		}
	}

	if result == "" {
		return "", errors.New("expected name, found EOF instead")
	}

	p.i = i
	return result, nil
}

// parseString parses a single- or double-quoted string.
func (p *parser) parseString() (result string, err error) {
	i := p.i
	if len(p.s) < i+2 {
		return "", errors.New("expected string, found EOF instead")
	}

	quote := p.s[i]
	i++

loop:
	for i < len(p.s) {
		switch p.s[i] {
		case '\\':
			if len(p.s) > i+1 {
				switch c := p.s[i+1]; c {
				case '\r':
					if len(p.s) > i+2 && p.s[i+2] == '\n' {
						i += 3
						continue loop
					}
					fallthrough
				case '\n', '\f':
					i += 2
					continue loop
				}
			}
			p.i = i
			val, err := p.parseEscape()
			if err != nil {
				return "", err
			}
			i = p.i
			result += val
		case quote:
			break loop
		case '\r', '\n', '\f':
			return "", errors.New("unexpected end of line in string")
		default:
			start := i
			for i < len(p.s) {
				if c := p.s[i]; c == quote || c == '\\' || c == '\r' || c == '\n' || c == '\f' {
					break
				}
				i++
			}
			result += p.s[start:i]
		}
	}

	if i >= len(p.s) {
		return "", errors.New("EOF in string")
	}

	// Consume the final quote.
	i++

	p.i = i
	return result, nil
}

// parseRegex parses a regular expression; the end is defined by encountering an
// unmatched closing ')' or ']' which is not consumed
func (p *parser) parseRegex() (rx *regexp.Regexp, err error) {
	i := p.i
	if len(p.s) < i+2 {
		return nil, errors.New("expected regular expression, found EOF instead")
	}

	// number of open parens or brackets;
	// when it becomes negative, finished parsing regex
	open := 0

loop:
	for i < len(p.s) {
		switch p.s[i] {
		case '(', '[':
			open++
		case ')', ']':
			open--
			if open < 0 {
				break loop
			}
		}
		i++
	}

	if i >= len(p.s) {
		return nil, errors.New("EOF in regular expression")
	}
	rx, err = regexp.Compile(p.s[p.i:i])
	p.i = i
	return rx, err
}

// skipWhitespace consumes whitespace characters and comments.
// It returns true if there was actually anything to skip.
func (p *parser) skipWhitespace() bool {
	i := p.i
	for i < len(p.s) {
		switch p.s[i] {
		case ' ', '\t', '\r', '\n', '\f':
			i++
			continue
		case '/':
			if strings.HasPrefix(p.s[i:], "/*") {
				end := strings.Index(p.s[i+len("/*"):], "*/")
				if end != -1 {
					i += end + len("/**/")
					continue
				}
			}
		}
		break
	}

	if i > p.i {
		p.i = i
		return true
	}

	return false
}

// consumeParenthesis consumes an opening parenthesis and any following
// whitespace. It returns true if there was actually a parenthesis to skip.
func (p *parser) consumeParenthesis() bool {
	if p.i < len(p.s) && p.s[p.i] == '(' {
		p.i++
		p.skipWhitespace()
		return true
	}
	return false
}

// consumeClosingParenthesis consumes a closing parenthesis and any preceding
// whitespace. It returns true if there was actually a parenthesis to skip.
func (p *parser) consumeClosingParenthesis() bool {
	i := p.i
	p.skipWhitespace()
	if p.i < len(p.s) && p.s[p.i] == ')' {
		p.i++
		return true
	}
	p.i = i
	return false
}

// parseTypeSelector parses a type selector (one that matches by tag name).
func (p *parser) parseTypeSelector() (result tagSelector, err error) {
	tag, err := p.parseIdentifier()
	if err != nil {
		return
	}
	return tagSelector{tag: toLowerASCII(tag)}, nil
}

// parseIDSelector parses a selector that matches by id attribute.
func (p *parser) parseIDSelector() (idSelector, error) {
	if p.i >= len(p.s) {
		return idSelector{}, fmt.Errorf("expected id selector (#id), found EOF instead")
	}
	if p.s[p.i] != '#' {
		return idSelector{}, fmt.Errorf("expected id selector (#id), found '%c' instead", p.s[p.i])
	}

	p.i++
	id, err := p.parseName()
	if err != nil {
		return idSelector{}, err
	}

	return idSelector{id: id}, nil
}

// parseClassSelector parses a selector that matches by class attribute.
func (p *parser) parseClassSelector() (classSelector, error) {
	if p.i >= len(p.s) {
		return classSelector{}, fmt.Errorf("expected class selector (.class), found EOF instead")
	}
	if p.s[p.i] != '.' {
		return classSelector{}, fmt.Errorf("expected class selector (.class), found '%c' instead", p.s[p.i])
	}

	p.i++
	class, err := p.parseIdentifier()
	if err != nil {
		return classSelector{}, err
	}

	return classSelector{class: class}, nil
}

// parseAttributeSelector parses a selector that matches by attribute value.
func (p *parser) parseAttributeSelector() (attrSelector, error) {
	if p.i >= len(p.s) {
		return attrSelector{}, fmt.Errorf("expected attribute selector ([attribute]), found EOF instead")
	}
	if p.s[p.i] != '[' {
		return attrSelector{}, fmt.Errorf("expected attribute selector ([attribute]), found '%c' instead", p.s[p.i])
	}

	p.i++
	p.skipWhitespace()
	key, err := p.parseIdentifier()
	if err != nil {
		return attrSelector{}, err
	}
	key = toLowerASCII(key)

	p.skipWhitespace()
	if p.i >= len(p.s) {
		return attrSelector{}, errors.New("unexpected EOF in attribute selector")
	}

	if p.s[p.i] == ']' {
		p.i++
		return attrSelector{key: key, operation: ""}, nil
	}

	if p.i+2 >= len(p.s) {
		return attrSelector{}, errors.New("unexpected EOF in attribute selector")
	}

	op := p.s[p.i : p.i+2]
	if op[0] == '=' {
		op = "="
	} else if op[1] != '=' {
		return attrSelector{}, fmt.Errorf(`expected equality operator, found "%s" instead`, op)
	}
	p.i += len(op)

	p.skipWhitespace()
	if p.i >= len(p.s) {
		return attrSelector{}, errors.New("unexpected EOF in attribute selector")
	}
	var val string
	var rx *regexp.Regexp
	if op == "#=" {
		rx, err = p.parseRegex()
	} else {
		switch p.s[p.i] {
		case '\'', '"':
			val, err = p.parseString()
		default:
			val, err = p.parseIdentifier()
		}
	}
	if err != nil {
		return attrSelector{}, err
	}

	p.skipWhitespace()
	if p.i >= len(p.s) {
		return attrSelector{}, errors.New("unexpected EOF in attribute selector")
	}

	// check if the attribute contains an ignore case flag
	ignoreCase := false
	if p.s[p.i] == 'i' || p.s[p.i] == 'I' {
		ignoreCase = true
		p.i++
	}

	p.skipWhitespace()
	if p.i >= len(p.s) {
		return attrSelector{}, errors.New("unexpected EOF in attribute selector")
	}

	if p.s[p.i] != ']' {
		return attrSelector{}, fmt.Errorf("expected ']', found '%c' instead", p.s[p.i])
	}
	p.i++

	switch op {
	case "=", "!=", "~=", "|=", "^=", "$=", "*=", "#=":
		return attrSelector{key: key, val: val, operation: op, regexp: rx, insensitive: ignoreCase}, nil
	default:
		return attrSelector{}, fmt.Errorf("attribute operator %q is not supported", op)
	}
}

var (
	errExpectedParenthesis        = errors.New("expected '(' but didn't find it")
	errExpectedClosingParenthesis = errors.New("expected ')' but didn't find it")
	errUnmatchedParenthesis       = errors.New("unmatched '('")
)

// parsePseudoclassSelector parses a pseudoclass selector like :not(p) or a pseudo-element
// For backwards compatibility, both ':' and '::' prefix are allowed for pseudo-elements.
// https://drafts.csswg.org/selectors-3/#pseudo-elements
// Returning a nil `Sel` (and a nil `error`) means we found a pseudo-element.
func (p *parser) parsePseudoclassSelector() (out Sel, pseudoElement string, err error) {
	if p.i >= len(p.s) {
		return nil, "", fmt.Errorf("expected pseudoclass selector (:pseudoclass), found EOF instead")
	}
	if p.s[p.i] != ':' {
		return nil, "", fmt.Errorf("expected attribute selector (:pseudoclass), found '%c' instead", p.s[p.i])
	}

	p.i++
	var mustBePseudoElement bool
	if p.i >= len(p.s) {
		return nil, "", fmt.Errorf("got empty pseudoclass (or pseudoelement)")
	}
	if p.s[p.i] == ':' { // we found a pseudo-element
		mustBePseudoElement = true
		p.i++
	}

	name, err := p.parseIdentifier()
	if err != nil {
		return
	}
	name = toLowerASCII(name)
	if mustBePseudoElement && (name != "after" && name != "backdrop" && name != "before" &&
		name != "cue" && name != "first-letter" && name != "first-line" && name != "grammar-error" &&
		name != "marker" && name != "placeholder" && name != "selection" && name != "spelling-error") {
		return out, "", fmt.Errorf("unknown pseudoelement :%s", name)
	}

	switch name {
	case "not", "has", "haschild":
		if !p.consumeParenthesis() {
			return out, "", errExpectedParenthesis
		}
		sel, parseErr := p.parseSelectorGroup()
		if parseErr != nil {
			return out, "", parseErr
		}
		if !p.consumeClosingParenthesis() {
			return out, "", errExpectedClosingParenthesis
		}

		out = relativePseudoClassSelector{name: name, match: sel}

	case "contains", "containsown":
		if !p.consumeParenthesis() {
			return out, "", errExpectedParenthesis
		}
		if p.i == len(p.s) {
			return out, "", errUnmatchedParenthesis
		}
		var val string
		switch p.s[p.i] {
		case '\'', '"':
			val, err = p.parseString()
		default:
			val, err = p.parseIdentifier()
		}
		if err != nil {
			return out, "", err
		}
		val = strings.ToLower(val)
		p.skipWhitespace()
		if p.i >= len(p.s) {
			return out, "", errors.New("unexpected EOF in pseudo selector")
		}
		if !p.consumeClosingParenthesis() {
			return out, "", errExpectedClosingParenthesis
		}

		out = containsPseudoClassSelector{own: name == "containsown", value: val}

	case "matches", "matchesown":
		if !p.consumeParenthesis() {
			return out, "", errExpectedParenthesis
		}
		rx, err := p.parseRegex()
		if err != nil {
			return out, "", err
		}
		if p.i >= len(p.s) {
			return out, "", errors.New("unexpected EOF in pseudo selector")
		}
		if !p.consumeClosingParenthesis() {
			return out, "", errExpectedClosingParenthesis
		}

		out = regexpPseudoClassSelector{own: name == "matchesown", regexp: rx}

	case "nth-child", "nth-last-child", "nth-of-type", "nth-last-of-type":
		if !p.consumeParenthesis() {
			return out, "", errExpectedParenthesis
		}
		a, b, err := p.parseNth()
		if err != nil {
			return out, "", err
		}
		if !p.consumeClosingParenthesis() {
			return out, "", errExpectedClosingParenthesis
		}
		last := name == "nth-last-child" || name == "nth-last-of-type"
		ofType := name == "nth-of-type" || name == "nth-last-of-type"
		out = nthPseudoClassSelector{a: a, b: b, last: last, ofType: ofType}

	case "first-child":
		out = nthPseudoClassSelector{a: 0, b: 1, ofType: false, last: false}
	case "last-child":
		out = nthPseudoClassSelector{a: 0, b: 1, ofType: false, last: true}
	case "first-of-type":
		out = nthPseudoClassSelector{a: 0, b: 1, ofType: true, last: false}
	case "last-of-type":
		out = nthPseudoClassSelector{a: 0, b: 1, ofType: true, last: true}
	case "only-child":
		out = onlyChildPseudoClassSelector{ofType: false}
	case "only-of-type":
		out = onlyChildPseudoClassSelector{ofType: true}
	case "input":
		out = inputPseudoClassSelector{}
	case "empty":
		out = emptyElementPseudoClassSelector{}
	case "root":
		out = rootPseudoClassSelector{}
	case "link":
		out = linkPseudoClassSelector{}
	case "lang":
		if !p.consumeParenthesis() {
			return out, "", errExpectedParenthesis
		}
		if p.i == len(p.s) {
			return out, "", errUnmatchedParenthesis
		}
		val, err := p.parseIdentifier()
		if err != nil {
			return out, "", err
		}
		val = strings.ToLower(val)
		p.skipWhitespace()
		if p.i >= len(p.s) {
			return out, "", errors.New("unexpected EOF in pseudo selector")
		}
		if !p.consumeClosingParenthesis() {
			return out, "", errExpectedClosingParenthesis
		}
		out = langPseudoClassSelector{lang: val}
	case "enabled":
		out = enabledPseudoClassSelector{}
	case "disabled":
		out = disabledPseudoClassSelector{}
	case "checked":
		out = checkedPseudoClassSelector{}
	case "visited", "hover", "active", "focus", "target":
		// Not applicable in a static context: never match.
		out = neverMatchSelector{value: ":" + name}
	case "after", "backdrop", "before", "cue", "first-letter", "first-line", "grammar-error", "marker", "placeholder", "selection", "spelling-error":
		return nil, name, nil
	default:
		return out, "", fmt.Errorf("unknown pseudoclass or pseudoelement :%s", name)
	}
	return
}

// parseInteger parses a  decimal integer.
func (p *parser) parseInteger() (int, error) {
	i := p.i
	start := i
	for i < len(p.s) && '0' <= p.s[i] && p.s[i] <= '9' {
		i++
	}
	if i == start {
		return 0, errors.New("expected integer, but didn't find it")
	}
	p.i = i

	val, err := strconv.Atoi(p.s[start:i])
	if err != nil {
		return 0, err
	}

	return val, nil
}

// parseNth parses the argument for :nth-child (normally of the form an+b).
func (p *parser) parseNth() (a, b int, err error) {
	// initial state
	if p.i >= len(p.s) {
		goto eof
	}
	switch p.s[p.i] {
	case '-':
		p.i++
		goto negativeA
	case '+':
		p.i++
		goto positiveA
	case '0', '1', '2', '3', '4', '5', '6', '7', '8', '9':
		goto positiveA
	case 'n', 'N':
		a = 1
		p.i++
		goto readN
	case 'o', 'O', 'e', 'E':
		id, nameErr := p.parseName()
		if nameErr != nil {
			return 0, 0, nameErr
		}
		id = toLowerASCII(id)
		if id == "odd" {
			return 2, 1, nil
		}
		if id == "even" {
			return 2, 0, nil
		}
		return 0, 0, fmt.Errorf("expected 'odd' or 'even', but found '%s' instead", id)
	default:
		goto invalid
	}

positiveA:
	if p.i >= len(p.s) {
		goto eof
	}
	switch p.s[p.i] {
	case '0', '1', '2', '3', '4', '5', '6', '7', '8', '9':
		a, err = p.parseInteger()
		if err != nil {
			return 0, 0, err
		}
		goto readA
	case 'n', 'N':
		a = 1
		p.i++
		goto readN
	default:
		goto invalid
	}

negativeA:
	if p.i >= len(p.s) {
		goto eof
	}
	switch p.s[p.i] {
	case '0', '1', '2', '3', '4', '5', '6', '7', '8', '9':
		a, err = p.parseInteger()
		if err != nil {
			return 0, 0, err
		}
		a = -a
		goto readA
	case 'n', 'N':
		a = -1
		p.i++
		goto readN
	default:
		goto invalid
	}

readA:
	if p.i >= len(p.s) {
		goto eof
	}
	switch p.s[p.i] {
	case 'n', 'N':
		p.i++
		goto readN
	default:
		// The number we read as a is actually b.
		return 0, a, nil
	}

readN:
	p.skipWhitespace()
	if p.i >= len(p.s) {
		goto eof
	}
	switch p.s[p.i] {
	case '+':
		p.i++
		p.skipWhitespace()
		b, err = p.parseInteger()
		if err != nil {
			return 0, 0, err
		}
		return a, b, nil
	case '-':
		p.i++
		p.skipWhitespace()
		b, err = p.parseInteger()
		if err != nil {
			return 0, 0, err
		}
		return a, -b, nil
	default:
		return a, 0, nil
	}

eof:
	return 0, 0, errors.New("unexpected EOF while attempting to parse expression of form an+b")

invalid:
	return 0, 0, errors.New("unexpected character while attempting to parse expression of form an+b")
}

// parseSimpleSelectorSequence parses a selector sequence that applies to
// a single element.
func (p *parser) parseSimpleSelectorSequence() (Sel, error) {
	var selectors []Sel

	if p.i >= len(p.s) {
		return nil, errors.New("expected selector, found EOF instead")
	}

	switch p.s[p.i] {
	case '*':
		// It's the universal selector. Just skip over it, since it doesn't affect the meaning.
		p.i++
		if p.i+2 < len(p.s) && p.s[p.i:p.i+2] == "|*" { // other version of universal selector
			p.i += 2
		}
	case '#', '.', '[', ':':
		// There's no type selector. Wait to process the other till the main loop.
	default:
		r, err := p.parseTypeSelector()
		if err != nil {
			return nil, err
		}
		selectors = append(selectors, r)
	}

	var pseudoElement string
loop:
	for p.i < len(p.s) {
		var (
			ns               Sel
			newPseudoElement string
			err              error
		)
		switch p.s[p.i] {
		case '#':
			ns, err = p.parseIDSelector()
		case '.':
			ns, err = p.parseClassSelector()
		case '[':
			ns, err = p.parseAttributeSelector()
		case ':':
			ns, newPseudoElement, err = p.parsePseudoclassSelector()
		default:
			break loop
		}
		if err != nil {
			return nil, err
		}
		// From https://drafts.csswg.org/selectors-3/#pseudo-elements :
		// "Only one pseudo-element may appear per selector, and if present
		// it must appear after the sequence of simple selectors that
		// represents the subjects of the selector.""
		if ns == nil { // we found a pseudo-element
			if pseudoElement != "" {
				return nil, fmt.Errorf("only one pseudo-element is accepted per selector, got %s and %s", pseudoElement, newPseudoElement)
			}
			if !p.acceptPseudoElements {
				return nil, fmt.Errorf("pseudo-element %s found, but pseudo-elements support is disabled", newPseudoElement)
			}
			pseudoElement = newPseudoElement
		} else {
			if pseudoElement != "" {
				return nil, fmt.Errorf("pseudo-element %s must be at the end of selector", pseudoElement)
			}
			selectors = append(selectors, ns)
		}

	}
	if len(selectors) == 1 && pseudoElement == "" { // no need wrap the selectors in compoundSelector
		return selectors[0], nil
	}
	return compoundSelector{selectors: selectors, pseudoElement: pseudoElement}, nil
}

// parseSelector parses a selector that may include combinators.
func (p *parser) parseSelector() (Sel, error) {
	p.skipWhitespace()
	result, err := p.parseSimpleSelectorSequence()
	if err != nil {
		return nil, err
	}

	for {
		var (
			combinator byte
			c          Sel
		)
		if p.skipWhitespace() {
			combinator = ' '
		}
		if p.i >= len(p.s) {
			return result, nil
		}

		switch p.s[p.i] {
		case '+', '>', '~':
			combinator = p.s[p.i]
			p.i++
			p.skipWhitespace()
		case ',', ')':
			// These characters can't begin a selector, but they can legally occur after one.
			return result, nil
		}

		if combinator == 0 {
			return result, nil
		}

		c, err = p.parseSimpleSelectorSequence()
		if err != nil {
			return nil, err
		}
		result = combinedSelector{first: result, combinator: combinator, second: c}
	}
}

// parseSelectorGroup parses a group of selectors, separated by commas.
func (p *parser) parseSelectorGroup() (SelectorGroup, error) {
	current, err := p.parseSelector()
	if err != nil {
		return nil, err
	}
	result := SelectorGroup{current}

	for p.i < len(p.s) {
		if p.s[p.i] != ',' {
			break
		}
		p.i++
		c, err := p.parseSelector()
		if err != nil {
			return nil, err
		}
		result = append(result, c)
	}
	return result, nil
}
