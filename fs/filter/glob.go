// rsync style glob parser

package filter

import (
	"bytes"
	"fmt"
	"regexp"
	"strings"

	"github.com/rclone/rclone/fs"
)

// GlobPathToRegexp converts an rsync style glob path to a regexp
func GlobPathToRegexp(glob string, ignoreCase bool) (*regexp.Regexp, error) {
	return globToRegexp(glob, true, true, ignoreCase)
}

// GlobStringToRegexp converts an rsync style glob string to a regexp
//
// Without adding of anchors but with ignoring of case, i.e. called
// `GlobStringToRegexp(glob, false, true)`, it takes a lenient approach
// where the glob "sum" would match "CheckSum", more similar to text
// search functions than strict glob filtering.
//
// With adding of anchors and not ignoring case, i.e. called
// `GlobStringToRegexp(glob, true, false)`, it uses a strict glob
// interpretation where the previous example would have to be changed to
// "*Sum" to match "CheckSum".
func GlobStringToRegexp(glob string, addAnchors bool, ignoreCase bool) (*regexp.Regexp, error) {
	return globToRegexp(glob, false, addAnchors, ignoreCase)
}

// globToRegexp converts an rsync style glob to a regexp
//
// Set pathMode true for matching of path/file names, e.g.
// special treatment of path separator `/` and double asterisk `**`,
// see filtering.md for details.
//
// Set addAnchors true to add start of string `^` and end of string `$` anchors.
func globToRegexp(glob string, pathMode bool, addAnchors bool, ignoreCase bool) (*regexp.Regexp, error) {
	var re bytes.Buffer
	if ignoreCase {
		_, _ = re.WriteString("(?i)")
	}
	if addAnchors {
		if pathMode {
			if strings.HasPrefix(glob, "/") {
				glob = glob[1:]
				_ = re.WriteByte('^')
			} else {
				_, _ = re.WriteString("(^|/)")
			}
		} else {
			_, _ = re.WriteString("^")
		}
	}
	consecutiveStars := 0
	insertStars := func() error {
		if consecutiveStars > 0 {
			if pathMode {
				switch consecutiveStars {
				case 1:
					_, _ = re.WriteString(`[^/]*`)
				case 2:
					_, _ = re.WriteString(`.*`)
				default:
					return fmt.Errorf("too many stars in %q", glob)
				}
			} else {
				switch consecutiveStars {
				case 1:
					_, _ = re.WriteString(`.*`)
				default:
					return fmt.Errorf("too many stars in %q", glob)
				}
			}
		}
		consecutiveStars = 0
		return nil
	}
	overwriteLastChar := func(c byte) {
		buf := re.Bytes()
		buf[len(buf)-1] = c
	}
	inBraces := false
	inBrackets := 0
	slashed := false
	inRegexp := false    // inside {{ ... }}
	inRegexpEnd := false // have received }} waiting for more
	var next, last rune
	for _, c := range glob {
		next, last = c, next
		if slashed {
			_, _ = re.WriteRune(c)
			slashed = false
			continue
		}
		if inRegexpEnd {
			if c == '}' {
				// Regexp is ending with }} choose longest segment
				// Replace final ) with }
				overwriteLastChar('}')
				_ = re.WriteByte(')')
				continue
			} else {
				inRegexpEnd = false
			}
		}
		if inRegexp {
			if c == '}' && last == '}' {
				inRegexp = false
				inRegexpEnd = true
				// Replace final } with )
				overwriteLastChar(')')
			} else {
				_, _ = re.WriteRune(c)
			}
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
			if pathMode {
				_, _ = re.WriteString(`[^/]`)
			} else {
				_, _ = re.WriteString(`.`)
			}
		case '[':
			_, _ = re.WriteRune(c)
			inBrackets++
		case ']':
			return nil, fmt.Errorf("mismatched ']' in glob %q", glob)
		case '{':
			if inBraces {
				if last == '{' {
					inRegexp = true
					inBraces = false
				} else {
					return nil, fmt.Errorf("can't nest '{' '}' in glob %q", glob)
				}
			} else {
				inBraces = true
				_ = re.WriteByte('(')
			}
		case '}':
			if !inBraces {
				return nil, fmt.Errorf("mismatched '{' and '}' in glob %q", glob)
			}
			_ = re.WriteByte(')')
			inBraces = false
		case ',':
			if inBraces {
				_ = re.WriteByte('|')
			} else {
				_, _ = re.WriteRune(c)
			}
		case '.', '+', '(', ')', '|', '^', '$': // regexp meta characters not dealt with above
			_ = re.WriteByte('\\')
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
		return nil, fmt.Errorf("mismatched '[' and ']' in glob %q", glob)
	}
	if inBraces {
		return nil, fmt.Errorf("mismatched '{' and '}' in glob %q", glob)
	}
	if inRegexp {
		return nil, fmt.Errorf("mismatched '{{' and '}}' in glob %q", glob)
	}
	if addAnchors {
		_ = re.WriteByte('$')
	}
	result, err := regexp.Compile(re.String())
	if err != nil {
		return nil, fmt.Errorf("bad glob pattern %q (regexp %q): %w", glob, re.String(), err)
	}
	return result, nil
}

var (
	// Can't deal with
	//   / or ** in {}
	//   {{ regexp }}
	tooHardRe = regexp.MustCompile(`({[^{}]*(\*\*|/)[^{}]*})|\{\{|\}\}`)

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
		fs.Infof(nil, "Can't figure out directory filters from %q: looking in all directories", glob)
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
