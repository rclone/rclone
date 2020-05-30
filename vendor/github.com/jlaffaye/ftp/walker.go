package ftp

import (
	pa "path"
	"strings"
)

//Walker traverses the directory tree of a remote FTP server
type Walker struct {
	serverConn *ServerConn
	root       string
	cur        item
	stack      []item
	descend    bool
}

type item struct {
	path  string
	entry Entry
	err   error
}

// Next advances the Walker to the next file or directory,
// which will then be available through the Path, Stat, and Err methods.
// It returns false when the walk stops at the end of the tree.
func (w *Walker) Next() bool {
	if w.descend && w.cur.err == nil && w.cur.entry.Type == EntryTypeFolder {
		list, err := w.serverConn.List(w.cur.path)
		if err != nil {
			w.cur.err = err
			w.stack = append(w.stack, w.cur)
		} else {
			for i := len(list) - 1; i >= 0; i-- {
				if !strings.HasSuffix(w.cur.path, "/") {
					w.cur.path += "/"
				}

				var path string
				if list[i].Type == EntryTypeFolder {
					path = pa.Join(w.cur.path, list[i].Name)
				} else {
					path = w.cur.path
				}

				w.stack = append(w.stack, item{path, *list[i], nil})
			}
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
func (w *Walker) Stat() Entry {
	return w.cur.entry
}

// Path returns the path to the most recent file or directory
// visited by a call to Next. It contains the argument to Walk
// as a prefix; that is, if Walk is called with "dir", which is
// a directory containing the file "a", Path will return "dir/a".
func (w *Walker) Path() string {
	return w.cur.path
}
