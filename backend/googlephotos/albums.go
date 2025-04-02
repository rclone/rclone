// This file contains the albums abstraction

package googlephotos

import (
	"path"
	"slices"
	"strings"
	"sync"

	"github.com/rclone/rclone/backend/googlephotos/api"
)

// All the albums
type albums struct {
	mu      sync.Mutex
	dupes   map[string][]*api.Album // duplicated names
	byID    map[string]*api.Album   //..indexed by ID
	byTitle map[string]*api.Album   //..indexed by Title
	path    map[string][]string     // partial album names to directory
}

// Create a new album
func newAlbums() *albums {
	return &albums{
		dupes:   map[string][]*api.Album{},
		byID:    map[string]*api.Album{},
		byTitle: map[string]*api.Album{},
		path:    map[string][]string{},
	}
}

// add an album
func (as *albums) add(album *api.Album) {
	// Munge the name of the album into a sensible path name
	album.Title = path.Clean(album.Title)
	if album.Title == "." || album.Title == "/" {
		album.Title = addID("", album.ID)
	}

	as.mu.Lock()
	as._add(album)
	as.mu.Unlock()
}

// _add an album - call with lock held
func (as *albums) _add(album *api.Album) {
	// update dupes by title
	dupes := as.dupes[album.Title]
	dupes = append(dupes, album)
	as.dupes[album.Title] = dupes

	// Dedupe the album name if necessary
	if len(dupes) >= 2 {
		// If this is the first dupe, then need to adjust the first one
		if len(dupes) == 2 {
			firstAlbum := dupes[0]
			as._del(firstAlbum)
			as._add(firstAlbum)
			// undo add of firstAlbum to dupes
			as.dupes[album.Title] = dupes
		}
		album.Title = addID(album.Title, album.ID)
	}

	// Store the new album
	as.byID[album.ID] = album
	as.byTitle[album.Title] = album

	// Store the partial paths
	dir, leaf := album.Title, ""
	for dir != "" {
		i := strings.LastIndex(dir, "/")
		if i >= 0 {
			dir, leaf = dir[:i], dir[i+1:]
		} else {
			dir, leaf = "", dir
		}
		dirs := as.path[dir]
		found := false
		for _, dir := range dirs {
			if dir == leaf {
				found = true
			}
		}
		if !found {
			as.path[dir] = append(as.path[dir], leaf)
		}
	}
}

// del an album
func (as *albums) del(album *api.Album) {
	as.mu.Lock()
	as._del(album)
	as.mu.Unlock()
}

// _del an album - call with lock held
func (as *albums) _del(album *api.Album) {
	// We leave in dupes so it doesn't cause albums to get renamed

	// Remove from byID and byTitle
	delete(as.byID, album.ID)
	delete(as.byTitle, album.Title)

	// Remove from paths
	dir, leaf := album.Title, ""
	for dir != "" {
		// Can't delete if this dir exists anywhere in the path structure
		if _, found := as.path[dir]; found {
			break
		}
		i := strings.LastIndex(dir, "/")
		if i >= 0 {
			dir, leaf = dir[:i], dir[i+1:]
		} else {
			dir, leaf = "", dir
		}
		dirs := as.path[dir]
		for i, dir := range dirs {
			if dir == leaf {
				dirs = slices.Delete(dirs, i, i+1)
				break
			}
		}
		if len(dirs) == 0 {
			delete(as.path, dir)
		} else {
			as.path[dir] = dirs
		}
	}
}

// get an album by title
func (as *albums) get(title string) (album *api.Album, ok bool) {
	as.mu.Lock()
	defer as.mu.Unlock()
	album, ok = as.byTitle[title]
	return album, ok
}

// getDirs gets directories below an album path
func (as *albums) getDirs(albumPath string) (dirs []string, ok bool) {
	as.mu.Lock()
	defer as.mu.Unlock()
	dirs, ok = as.path[albumPath]
	return dirs, ok
}
