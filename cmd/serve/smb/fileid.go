package smb

import (
	"strconv"
	"strings"
	"sync"
)

// maxFileIDEntries bounds what fileIDs remembers: past it, a renamed file's id
// changes with its name as if nothing were remembered, which is safe.
const maxFileIDEntries = 1 << 20

// fileIDs gives files and directories the 64-bit FileId that clients identify
// them by. An id is the hash of a path (pathFileID): the file's own path, or if
// the file was renamed, the path its id was first given at, so the id stays with
// the file. Clients cache files by id, so no later file at a vacated name may
// get the same id, and ids are never reused that way.
//
// identity maps a path, or a directory above it, to the path the ids below it
// are hashed from: the path a renamed file or directory came from, or for a name
// a file was renamed away from, a marker no path can equal. Keys are the
// current names, so renaming a directory re-keys what it holds.
type fileIDs struct {
	mu       sync.Mutex
	identity map[string]string
	vacated  uint64 // markers handed out
}

// id returns the FileId of the file at path.
func (f *fileIDs) id(path string) uint64 {
	f.mu.Lock()
	defer f.mu.Unlock()
	return pathFileID(f.canonical(path))
}

// canonical returns the path that path's id is hashed from. The caller holds
// f.mu.
func (f *fileIDs) canonical(path string) string {
	for p := path; ; {
		if root, ok := f.identity[p]; ok {
			return root + path[len(p):]
		}
		i := strings.LastIndexByte(p, '/')
		if i < 0 {
			return path
		}
		p = p[:i]
	}
}

// marker reports whether what identity records for path is the marker a rename
// away from it left, rather than where a file renamed to it came from.
func marker(path, root string) bool {
	return strings.HasPrefix(root, path+"\x00")
}

// renamed records that the file or directory at oldPath is now at newPath: its
// id goes with it, and the next file at oldPath gets another.
func (f *fileIDs) renamed(oldPath, newPath string, isDir bool) {
	if oldPath == newPath || oldPath == "" || newPath == "" {
		return
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.identity == nil {
		f.identity = map[string]string{}
	}
	root := f.canonical(oldPath)
	if isDir {
		// What the directory holds is known by its new name now, and nothing
		// below the new name is left from a directory that had it before.
		oldPrefix, newPrefix := oldPath+"/", newPath+"/"
		for p := range f.identity {
			if strings.HasPrefix(p, newPrefix) {
				delete(f.identity, p)
			}
		}
		for p, r := range f.identity {
			if strings.HasPrefix(p, oldPrefix) {
				delete(f.identity, p)
				f.identity[newPrefix+p[len(oldPrefix):]] = r
			}
		}
	}
	// Whatever was at newPath is gone, replaced; the file that left oldPath takes
	// its id along.
	if r, ok := f.identity[newPath]; ok && !marker(newPath, r) {
		delete(f.identity, newPath)
	}
	if r, ok := f.identity[oldPath]; ok && !marker(oldPath, r) {
		delete(f.identity, oldPath)
	}
	if len(f.identity) >= maxFileIDEntries {
		return
	}
	f.identity[newPath] = root
	f.vacated++
	f.identity[oldPath] = oldPath + "\x00" + strconv.FormatUint(f.vacated, 10)
}

// removed records that the file or directory at path was deleted, freeing the
// id it brought along if it had been renamed. A name's marker stays: the file
// that left it may still exist under the id.
func (f *fileIDs) removed(path string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if r, ok := f.identity[path]; ok && !marker(path, r) {
		delete(f.identity, path)
	}
}
