package genfilters

import (
	"errors"
	"fmt"
	"regexp/syntax"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/rclone/rclone/fs/filter"
)

// escape returns a glob matching name literally in a filter file
func escape(name string) string {
	s := filter.GlobEscape(name)
	// Lines in a filter file are trimmed, so trailing space must be
	// put in a character class to survive
	if last, size := utf8.DecodeLastRuneInString(s); unicode.IsSpace(last) {
		s = s[:len(s)-size] + "[" + string(last) + "]"
	}
	return s
}

// unescape returns the one name glob matches, relative to the root, or
// an error if it can match more than one.
//
// It asks rclone's glob compiler rather than parsing the glob itself, so
// the two can't read a glob differently.
func unescape(glob string) (string, error) {
	re, err := filter.GlobPathToRegexp("/"+glob, false)
	if err != nil {
		return "", err
	}
	p, err := syntax.Parse(re.String(), syntax.Perl)
	if err != nil {
		return "", err
	}
	p = p.Simplify()
	if p.Op != syntax.OpConcat || len(p.Sub) != 3 || p.Sub[0].Op != syntax.OpBeginText ||
		p.Sub[1].Op != syntax.OpLiteral || p.Sub[1].Flags&syntax.FoldCase != 0 || p.Sub[2].Op != syntax.OpEndText {
		return "", errors.New("it matches more than one name")
	}
	return string(p.Sub[1].Rune), nil
}

// lineError is an error in a line of filter rules
type lineError struct {
	line int // line number, counting from 1
	text string
	err  error
}

func (e *lineError) Error() string {
	return fmt.Sprintf("line %d: %q: %v", e.line, e.text, e.err)
}

func (e *lineError) Unwrap() error {
	return e.err
}

// errUnsupported marks rules which are valid rclone filters that
// genfilters can't edit
var errUnsupported = errors.New("genfilters can only edit rules it could have made")

// parse reads filter rules into a model.
//
// The rules may be any genfilters could have made: a rule for a file
// (`+ /path`) or a directory and everything in it (`+ /path/**`), each
// before the rules for the directories containing it, ending with the
// catch-all `+ **` or `- **`. Comments are allowed before the first
// rule and are kept. hasRules reports whether there were any rules, which
// sets the mode even without a catch-all: rclone then includes everything
// else.
func parse(lines []string) (mo *model, hasRules bool, err error) {
	mo = &model{selected: map[string]bool{}}
	var (
		rules     int
		catchAll  = -1 // line index of the catch-all rule
		rootRule  = -1 // line index of a `± /*` rule
		rootSign  bool
		seen      = map[string]int{} // key -> line index
		checkRule = func(text string) error {
			filt, err := filter.NewFilter(nil)
			if err != nil {
				return err
			}
			return filt.AddRule(text)
		}
	)
	for i, line := range lines {
		text := strings.TrimSpace(line)
		fail := func(err error) (*model, bool, error) {
			return nil, false, &lineError{line: i + 1, text: text, err: err}
		}
		switch {
		case text == "":
			if rules == 0 && len(mo.header) > 0 {
				mo.header = append(mo.header, "")
			}
			continue
		case text[0] == '#' || text[0] == ';':
			if rules > 0 {
				return fail(errors.New("comments must come before the first rule"))
			}
			mo.header = append(mo.header, line)
			continue
		case text == "!":
			mo.selected = map[string]bool{}
			seen = map[string]int{}
			catchAll, rootRule, rules = -1, -1, 0
			mo.mode = includeOthers
			continue
		}
		if err := checkRule(text); err != nil {
			return fail(err)
		}
		rules++
		if catchAll >= 0 {
			return fail(errors.New("rules after the catch-all rule are never used"))
		}
		pattern, include := strings.CutPrefix(text, "+ ")
		if !include {
			pattern, _ = strings.CutPrefix(text, "- ")
		}
		switch {
		case pattern == "**":
			catchAll = i
			mo.mode = excludeOthers
			if include {
				mo.mode = includeOthers
			}
			continue
		case pattern == "/*":
			rootRule, rootSign = i, include
			continue
		case !strings.HasPrefix(pattern, "/"):
			return fail(fmt.Errorf("%w: it must start with \"/\"", errUnsupported))
		}
		key := strings.TrimPrefix(pattern, "/")
		isDir := strings.HasSuffix(key, "/**")
		key = strings.TrimSuffix(key, "/**")
		errDir := fmt.Errorf("%w: use \"/dir/**\" for a directory and everything in it", errUnsupported)
		if key == "" || strings.HasSuffix(key, "/") || (!isDir && (key == "*" || strings.HasSuffix(key, "/*"))) {
			return fail(errDir)
		}
		key, err = unescape(key)
		if err != nil {
			return fail(fmt.Errorf("%w: %v", errUnsupported, err))
		}
		if isDir {
			key += "/"
		}
		if j, ok := seen[key]; ok {
			return fail(fmt.Errorf("duplicates the rule on line %d", j+1))
		}
		// A rule after the rule for a directory containing it would never be used
		if rootRule >= 0 && !isDir && !strings.Contains(key, "/") {
			return fail(fmt.Errorf("never used: the rule on line %d for \"/*\" matches first", rootRule+1))
		}
		for k, j := range seen {
			if strings.HasSuffix(k, "/") && strings.HasPrefix(key, k) {
				return fail(fmt.Errorf("never used: the rule on line %d for %q matches first", j+1, k))
			}
		}
		seen[key] = i
		mo.selected[key] = include
	}
	if rootRule >= 0 && (catchAll < 0 || rootSign != (mo.mode == includeOthers)) {
		return nil, false, &lineError{
			line: rootRule + 1, text: strings.TrimSpace(lines[rootRule]),
			err: fmt.Errorf("%w: \"/*\" must match the catch-all rule", errUnsupported),
		}
	}
	if err := mo.build(); err != nil {
		return nil, false, err
	}
	return mo, rules > 0, nil
}
