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
	//   {{ regexp }}
	tooHardRe = regexp.MustCompile(`\{\{|\}\}`)

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

	// Expand curly braces first
	expanded := expandBraces(glob)

	// Process each expanded pattern
	seen := make(map[string]bool)
	for _, pattern := range expanded {
		// Get rid of multiple /s
		pattern = squashSlash.ReplaceAllString(pattern, "/")

		// Split on / or **
		// (** can contain /)
		subGlob := pattern
		for {
			i := strings.LastIndex(subGlob, "/")
			j := strings.LastIndex(subGlob, "**")
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
			subGlob = subGlob[:i]
			newGlob := subGlob + what + "/"
			if !seen[newGlob] {
				seen[newGlob] = true
				out = append(out, newGlob)
			}
		}
	}

	return out
}

// expandBraces expands curly brace patterns like {a,b,c} into multiple strings
func expandBraces(pattern string) []string {
	// Simple case: no braces
	if !strings.Contains(pattern, "{") || !strings.Contains(pattern, "}") {
		return []string{pattern}
	}

	// Find the first complete brace pair, avoiding {{ and }}
	start := -1
	for i := 0; i < len(pattern); i++ {
		if pattern[i] == '{' {
			// Check if this is NOT a {{ pattern
			if i >= len(pattern)-1 || pattern[i+1] != '{' {
				start = i
				break
			} else {
				// Skip the {{ pattern
				i++
			}
		}
	}

	if start == -1 {
		return []string{pattern}
	}

	// Find the matching closing brace
	depth := 0
	end := -1
	for i := start; i < len(pattern); i++ {
		switch pattern[i] {
		case '{':
			// Skip {{ patterns
			if i < len(pattern)-1 && pattern[i+1] == '{' {
				i++ // skip both {
				continue
			}
			depth++
		case '}':
			// Skip }} patterns
			if i > 0 && pattern[i-1] == '}' {
				continue
			}
			depth--
			if depth == 0 {
				end = i
				goto found
			}
		}
	}
found:

	if end == -1 {
		return []string{pattern}
	}

	// Extract the options
	prefix := pattern[:start]
	braceContent := pattern[start+1:end]
	suffix := pattern[end+1:]

	// Split on commas, but be careful about nested braces
	var options []string
	var current strings.Builder
	depth = 0
	for _, char := range braceContent {
		switch char {
		case '{':
			depth++
			current.WriteRune(char)
		case '}':
			depth--
			current.WriteRune(char)
		case ',':
			if depth == 0 {
				options = append(options, current.String())
				current.Reset()
			} else {
				current.WriteRune(char)
			}
		default:
			current.WriteRune(char)
		}
	}
	if current.Len() > 0 {
		options = append(options, current.String())
	}

	var result []string
	for _, option := range options {
		expanded := prefix + option + suffix
		// Recursively expand any remaining braces
		subExpanded := expandBraces(expanded)
		result = append(result, subExpanded...)
	}

	return result
}
