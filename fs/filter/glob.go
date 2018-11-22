// rsync style glob parser

package filter

import (
	"bytes"
	"regexp"
	"strings"

	"github.com/pkg/errors"
)

// globToRegexp converts an rsync style glob to a regexp
//
// documented in filtering.md
func globToRegexp(glob string, ignoreCase bool) (*regexp.Regexp, error) {
	var re bytes.Buffer
	if ignoreCase {
		_, _ = re.WriteString("(?i)")
	}
	if strings.HasPrefix(glob, "/") {
		glob = glob[1:]
		_, _ = re.WriteRune('^')
	} else {
		_, _ = re.WriteString("(^|/)")
	}
	consecutiveStars := 0
	insertStars := func() error {
		if consecutiveStars > 0 {
			switch consecutiveStars {
			case 1:
				_, _ = re.WriteString(`[^/]*`)
			case 2:
				_, _ = re.WriteString(`.*`)
			default:
				return errors.Errorf("too many stars in %q", glob)
			}
		}
		consecutiveStars = 0
		return nil
	}
	inBraces := false
	inBrackets := 0
	slashed := false
	for _, c := range glob {
		if slashed {
			_, _ = re.WriteRune(c)
			slashed = false
			continue
		}
		if c != '*' {
			err := insertStars()
			if err != nil {
				return nil, err
			}
		}
		if inBrackets > 0 {
			_, _ = re.WriteRune(c)
			if c == '[' {
				inBrackets++
			}
			if c == ']' {
				inBrackets--
			}
			continue
		}
		switch c {
		case '\\':
			_, _ = re.WriteRune(c)
			slashed = true
		case '*':
			consecutiveStars++
		case '?':
			_, _ = re.WriteString(`[^/]`)
		case '[':
			_, _ = re.WriteRune(c)
			inBrackets++
		case ']':
			return nil, errors.Errorf("mismatched ']' in glob %q", glob)
		case '{':
			if inBraces {
				return nil, errors.Errorf("can't nest '{' '}' in glob %q", glob)
			}
			inBraces = true
			_, _ = re.WriteRune('(')
		case '}':
			if !inBraces {
				return nil, errors.Errorf("mismatched '{' and '}' in glob %q", glob)
			}
			_, _ = re.WriteRune(')')
			inBraces = false
		case ',':
			if inBraces {
				_, _ = re.WriteRune('|')
			} else {
				_, _ = re.WriteRune(c)
			}
		case '.', '+', '(', ')', '|', '^', '$': // regexp meta characters not dealt with above
			_, _ = re.WriteRune('\\')
			_, _ = re.WriteRune(c)
		default:
			_, _ = re.WriteRune(c)
		}
	}
	err := insertStars()
	if err != nil {
		return nil, err
	}
	if inBrackets > 0 {
		return nil, errors.Errorf("mismatched '[' and ']' in glob %q", glob)
	}
	if inBraces {
		return nil, errors.Errorf("mismatched '{' and '}' in glob %q", glob)
	}
	_, _ = re.WriteRune('$')
	result, err := regexp.Compile(re.String())
	if err != nil {
		return nil, errors.Wrapf(err, "bad glob pattern %q (regexp %q)", glob, re.String())
	}
	return result, nil
}

var (
	// Can't deal with / or ** in {}
	tooHardRe = regexp.MustCompile(`{[^{}]*(\*\*|/)[^{}]*}`)

	// Squash all /
	squashSlash = regexp.MustCompile(`/{2,}`)
)

// globToDirGlobs takes a file glob and turns it into a series of
// directory globs.  When matched with a directory (with a trailing /)
// this should answer the question as to whether this glob could be in
// this directory.
func globToDirGlobs(glob string) (out []string) {
	if tooHardRe.MatchString(glob) {
		// Can't figure this one out so return any directory might match
		out = append(out, "/**")
		return out
	}

	// Get rid of multiple /s
	glob = squashSlash.ReplaceAllString(glob, "/")

	// Split on / or **
	// (** can contain /)
	for {
		i := strings.LastIndex(glob, "/")
		j := strings.LastIndex(glob, "**")
		what := ""
		if j > i {
			i = j
			what = "**"
		}
		if i < 0 {
			if len(out) == 0 {
				out = append(out, "/**")
			}
			break
		}
		glob = glob[:i]
		newGlob := glob + what + "/"
		if len(out) == 0 || out[len(out)-1] != newGlob {
			out = append(out, newGlob)
		}
	}

	return out
}
