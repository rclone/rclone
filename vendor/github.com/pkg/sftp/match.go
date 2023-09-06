package sftp

import (
	"path"
	"strings"
)

// ErrBadPattern indicates a globbing pattern was malformed.
var ErrBadPattern = path.ErrBadPattern

// Match reports whether name matches the shell pattern.
//
// This is an alias for path.Match from the standard library,
// offered so that callers need not import the path package.
// For details, see https://golang.org/pkg/path/#Match.
func Match(pattern, name string) (matched bool, err error) {
	return path.Match(pattern, name)
}

// detect if byte(char) is path separator
func isPathSeparator(c byte) bool {
	return c == '/'
}

// Split splits the path p immediately following the final slash,
// separating it into a directory and file name component.
//
// This is an alias for path.Split from the standard library,
// offered so that callers need not import the path package.
// For details, see https://golang.org/pkg/path/#Split.
func Split(p string) (dir, file string) {
	return path.Split(p)
}

// Glob returns the names of all files matching pattern or nil
// if there is no matching file. The syntax of patterns is the same
// as in Match. The pattern may describe hierarchical names such as
// /usr/*/bin/ed.
//
// Glob ignores file system errors such as I/O errors reading directories.
// The only possible returned error is ErrBadPattern, when pattern
// is malformed.
func (c *Client) Glob(pattern string) (matches []string, err error) {
	if !hasMeta(pattern) {
		file, err := c.Lstat(pattern)
		if err != nil {
			return nil, nil
		}
		dir, _ := Split(pattern)
		dir = cleanGlobPath(dir)
		return []string{Join(dir, file.Name())}, nil
	}

	dir, file := Split(pattern)
	dir = cleanGlobPath(dir)

	if !hasMeta(dir) {
		return c.glob(dir, file, nil)
	}

	// Prevent infinite recursion. See issue 15879.
	if dir == pattern {
		return nil, ErrBadPattern
	}

	var m []string
	m, err = c.Glob(dir)
	if err != nil {
		return
	}
	for _, d := range m {
		matches, err = c.glob(d, file, matches)
		if err != nil {
			return
		}
	}
	return
}

// cleanGlobPath prepares path for glob matching.
func cleanGlobPath(path string) string {
	switch path {
	case "":
		return "."
	case "/":
		return path
	default:
		return path[0 : len(path)-1] // chop off trailing separator
	}
}

// glob searches for files matching pattern in the directory dir
// and appends them to matches. If the directory cannot be
// opened, it returns the existing matches. New matches are
// added in lexicographical order.
func (c *Client) glob(dir, pattern string, matches []string) (m []string, e error) {
	m = matches
	fi, err := c.Stat(dir)
	if err != nil {
		return
	}
	if !fi.IsDir() {
		return
	}
	names, err := c.ReadDir(dir)
	if err != nil {
		return
	}
	//sort.Strings(names)

	for _, n := range names {
		matched, err := Match(pattern, n.Name())
		if err != nil {
			return m, err
		}
		if matched {
			m = append(m, Join(dir, n.Name()))
		}
	}
	return
}

// Join joins any number of path elements into a single path, separating
// them with slashes.
//
// This is an alias for path.Join from the standard library,
// offered so that callers need not import the path package.
// For details, see https://golang.org/pkg/path/#Join.
func Join(elem ...string) string {
	return path.Join(elem...)
}

// hasMeta reports whether path contains any of the magic characters
// recognized by Match.
func hasMeta(path string) bool {
	return strings.ContainsAny(path, "\\*?[")
}
