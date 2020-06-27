package ftp

import (
	"fmt"
	"os"
	"path"
	"strings"
)

//Walker traverses the directory tree of a remote FTP server
type Walker struct {
	serverConn *ServerConn
	root       string
	cur        *item
	stack      []*item
	descend    bool
}

type item struct {
	path  string
	entry *Entry
	err   error
}

// Next advances the Walker to the next file or directory,
// which will then be available through the Path, Stat, and Err methods.
// It returns false when the walk stops at the end of the tree.
func (w *Walker) Next() bool {
	var isRoot bool
	if w.cur == nil {
		w.cur = &item{
			path: strings.Trim(w.root, string(os.PathSeparator)),
		}

		isRoot = true
	}

	entries, err := w.serverConn.List(w.cur.path)
	w.cur.err = err
	if err == nil {
		if len(entries) == 0 {
			w.cur.err = fmt.Errorf("no such file or directory: %s", w.cur.path)

			return false
		}

		if isRoot && len(entries) == 1 && entries[0].Type == EntryTypeFile {
			w.cur.err = fmt.Errorf("root is not a directory: %s", w.cur.path)

			return false
		}

		for i, entry := range entries {
			if entry.Name == "." || (i == 0 && entry.Type == EntryTypeFile) {
				entry.Name = path.Base(w.cur.path)
				w.cur.entry = entry
				continue
			}

			if entry.Name == ".." || !w.descend {
				continue
			}

			item := &item{
				path:  path.Join(w.cur.path, entry.Name),
				entry: entry,
			}

			w.stack = append(w.stack, item)
		}
	}

	if len(w.stack) == 0 {
		return false
	}

	i := len(w.stack) - 1
	w.cur = w.stack[i]
	w.stack = w.stack[:i]
	w.descend = true

	return true
}

//SkipDir tells the Next function to skip the currently processed directory
func (w *Walker) SkipDir() {
	w.descend = false
}

//Err returns the error, if any, for the most recent attempt by Next to
//visit a file or a directory. If a directory has an error, the walker
//will not descend in that directory
func (w *Walker) Err() error {
	return w.cur.err
}

// Stat returns info for the most recent file or directory
// visited by a call to Step.
func (w *Walker) Stat() *Entry {
	return w.cur.entry
}

// Path returns the path to the most recent file or directory
// visited by a call to Next. It contains the argument to Walk
// as a prefix; that is, if Walk is called with "dir", which is
// a directory containing the file "a", Path will return "dir/a".
func (w *Walker) Path() string {
	return w.cur.path
}
