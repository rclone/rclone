// Package dirtree contains the DirTree type which is used for
// building filesystem hierarchies in memory.
package dirtree

import (
	"bytes"
	"fmt"
	"path"
	"sort"
	"time"

	"github.com/rclone/rclone/fs"
)

// DirTree is a map of directories to entries
type DirTree map[string]fs.DirEntries

// New returns a fresh DirTree
func New() DirTree {
	return make(DirTree)
}

// parentDir finds the parent directory of path
func parentDir(entryPath string) string {
	dirPath := path.Dir(entryPath)
	if dirPath == "." {
		dirPath = ""
	}
	return dirPath
}

// Add an entry to the tree
// it doesn't create parents
func (dt DirTree) Add(entry fs.DirEntry) {
	dirPath := parentDir(entry.Remote())
	dt[dirPath] = append(dt[dirPath], entry)
}

// AddDir adds a directory entry to the tree
// this creates the directory itself if required
// it doesn't create parents
func (dt DirTree) AddDir(entry fs.DirEntry) {
	dirPath := entry.Remote()
	if dirPath == "" {
		return
	}
	dt.Add(entry)
	// create the directory itself if it doesn't exist already
	if _, ok := dt[dirPath]; !ok {
		dt[dirPath] = nil
	}
}

// AddEntry adds the entry and creates the parents for it regardless
// of whether it is a file or a directory.
func (dt DirTree) AddEntry(entry fs.DirEntry) {
	switch entry.(type) {
	case fs.Directory:
		dt.AddDir(entry)
	case fs.Object:
		dt.Add(entry)
	default:
		panic("unknown entry type")
	}
	remoteParent := parentDir(entry.Remote())
	dt.checkParent("", remoteParent, nil)
}

// Find returns the DirEntry for filePath or nil if not found
//
// None that Find does a O(N) search so can be slow
func (dt DirTree) Find(filePath string) (parentPath string, entry fs.DirEntry) {
	parentPath = parentDir(filePath)
	for _, entry := range dt[parentPath] {
		if entry.Remote() == filePath {
			return parentPath, entry
		}
	}
	return parentPath, nil
}

// checkParent checks that dirPath has a *Dir in its parent
//
// If dirs is not nil it must contain entries for every *Dir found in
// the tree. It is used to speed up the checking when calling this
// repeatedly.
func (dt DirTree) checkParent(root, dirPath string, dirs map[string]struct{}) {
	var parentPath string
	for {
		if dirPath == root {
			return
		}
		// Can rely on dirs to have all directories in it so
		// we don't need to call Find.
		if dirs != nil {
			if _, found := dirs[dirPath]; found {
				return
			}
			parentPath = parentDir(dirPath)
		} else {
			var entry fs.DirEntry
			parentPath, entry = dt.Find(dirPath)
			if entry != nil {
				return
			}
		}
		dt[parentPath] = append(dt[parentPath], fs.NewDir(dirPath, time.Now()))
		if dirs != nil {
			dirs[dirPath] = struct{}{}
		}
		dirPath = parentPath
	}
}

// CheckParents checks every directory in the tree has *Dir in its parent
func (dt DirTree) CheckParents(root string) {
	dirs := make(map[string]struct{})
	// Find all the directories and stick them in dirs
	for _, entries := range dt {
		for _, entry := range entries {
			if _, ok := entry.(fs.Directory); ok {
				dirs[entry.Remote()] = struct{}{}
			}
		}
	}
	for dirPath := range dt {
		dt.checkParent(root, dirPath, dirs)
	}
}

// Sort sorts all the Entries
func (dt DirTree) Sort() {
	for _, entries := range dt {
		sort.Stable(entries)
	}
}

// Dirs returns the directories in sorted order
func (dt DirTree) Dirs() (dirNames []string) {
	for dirPath := range dt {
		dirNames = append(dirNames, dirPath)
	}
	sort.Strings(dirNames)
	return dirNames
}

// Prune remove directories from a directory tree. dirNames contains
// all directories to remove as keys, with true as values. dirNames
// will be modified in the function.
func (dt DirTree) Prune(dirNames map[string]bool) error {
	// We use map[string]bool to avoid recursion (and potential
	// stack exhaustion).

	// First we need delete directories from their parents.
	for dName, remove := range dirNames {
		if !remove {
			// Currently all values should be
			// true, therefore this should not
			// happen. But this makes function
			// more predictable.
			fs.Infof(dName, "Directory in the map for prune, but the value is false")
			continue
		}
		if dName == "" {
			// if dName is root, do nothing (no parent exist)
			continue
		}
		parent := parentDir(dName)
		// It may happen that dt does not have a dName key,
		// since directory was excluded based on a filter. In
		// such case the loop will be skipped.
		for i, entry := range dt[parent] {
			switch x := entry.(type) {
			case fs.Directory:
				if x.Remote() == dName {
					// the slice is not sorted yet
					// to delete item
					// a) replace it with the last one
					dt[parent][i] = dt[parent][len(dt[parent])-1]
					// b) remove last
					dt[parent] = dt[parent][:len(dt[parent])-1]
					// we modify a slice within a loop, but we stop
					// iterating immediately
					break
				}
			case fs.Object:
				// do nothing
			default:
				return fmt.Errorf("unknown object type %T", entry)

			}
		}
	}

	for len(dirNames) > 0 {
		// According to golang specs, if new keys were added
		// during range iteration, they may be skipped.
		for dName, remove := range dirNames {
			if !remove {
				fs.Infof(dName, "Directory in the map for prune, but the value is false")
				continue
			}
			// First, add all subdirectories to dirNames.

			// It may happen that dt[dName] does not exist.
			// If so, the loop will be skipped.
			for _, entry := range dt[dName] {
				switch x := entry.(type) {
				case fs.Directory:
					excludeDir := x.Remote()
					dirNames[excludeDir] = true
				case fs.Object:
					// do nothing
				default:
					return fmt.Errorf("unknown object type %T", entry)

				}
			}
			// Then remove current directory from DirTree
			delete(dt, dName)
			// and from dirNames
			delete(dirNames, dName)
		}
	}
	return nil
}

// String emits a simple representation of the DirTree
func (dt DirTree) String() string {
	out := new(bytes.Buffer)
	for _, dir := range dt.Dirs() {
		_, _ = fmt.Fprintf(out, "%s/\n", dir)
		for _, entry := range dt[dir] {
			flag := ""
			if _, ok := entry.(fs.Directory); ok {
				flag = "/"
			}
			_, _ = fmt.Fprintf(out, "  %s%s\n", path.Base(entry.Remote()), flag)
		}
	}
	return out.String()
}
