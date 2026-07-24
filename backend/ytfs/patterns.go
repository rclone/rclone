// Store the parsing of file patterns

package ytfs

import (
	"context"
	"path"
	"regexp"
	"strings"
	"time"

	"github.com/rclone/rclone/fs"
)

// lister describes the subset of the interfaces on Fs needed for the
// file pattern parsing
type lister interface {
	listChannels(ctx context.Context, prefix string) (fs.DirEntries, error)
	listChannelVideos(ctx context.Context, prefix, channelID string) (fs.DirEntries, error)
	listPlaylists(ctx context.Context, prefix string) (fs.DirEntries, error)
	listPlaylistVideos(ctx context.Context, prefix, playlistID string) (fs.DirEntries, error)
	dirTime() time.Time
}

// dirPattern describes a single directory pattern
type dirPattern struct {
	re        string
	match     *regexp.Regexp
	isFile    bool
	toEntries func(ctx context.Context, f lister, prefix string, match []string) (fs.DirEntries, error)
}

// dirPatterns is a slice of dirPattern
type dirPatterns []dirPattern

// mustCompile compiles every pattern's regexp in place.
func (ds dirPatterns) mustCompile() dirPatterns {
	for i := range ds {
		ds[i].match = regexp.MustCompile(ds[i].re)
	}
	return ds
}

// match finds the path passed in the matching structure and
// returns the parameters and a pointer to the match, or nil.
func (ds dirPatterns) match(root string, itemPath string, isFile bool) (match []string, prefix string, pattern *dirPattern) {
	itemPath = strings.Trim(itemPath, "/")
	absPath := path.Join(root, itemPath)
	prefix = strings.Trim(absPath[len(root):], "/")
	if prefix != "" {
		prefix += "/"
	}
	for i := range ds {
		pattern = &ds[i]
		if pattern.isFile != isFile {
			continue
		}
		match = pattern.match.FindStringSubmatch(absPath)
		if match != nil {
			return
		}
	}
	return nil, "", nil
}

// listRoot returns the top-level directories.
func listRoot(ctx context.Context, f lister, prefix string, match []string) (entries fs.DirEntries, err error) {
	entries = append(entries, fs.NewDir(prefix+"channels", f.dirTime()))
	entries = append(entries, fs.NewDir(prefix+"playlists", f.dirTime()))
	return entries, nil
}

// patterns describes the layout of the ytfs backend file system.
//
// NB no trailing / on paths. More-specific patterns come before
// generic patterns so they are not shadowed.
var patterns = dirPatterns{
	{ // root → channels + playlists
		re:        `^$`,
		toEntries: listRoot,
	},
	{ // channels dir → list all channels
		re: `^channels$`,
		toEntries: func(ctx context.Context, f lister, prefix string, match []string) (fs.DirEntries, error) {
			return f.listChannels(ctx, prefix)
		},
	},
	{ // channel video file (before channel dir to avoid shadowing)
		re:     `^channels/([^/]+)/([^/]+)$`,
		isFile: true,
	},
	{ // channel dir → list videos in channel
		re: `^channels/(.+)$`,
		toEntries: func(ctx context.Context, f lister, prefix string, match []string) (fs.DirEntries, error) {
			return f.listChannelVideos(ctx, prefix, match[1])
		},
	},
	{ // playlists dir → list all playlists
		re: `^playlists$`,
		toEntries: func(ctx context.Context, f lister, prefix string, match []string) (fs.DirEntries, error) {
			return f.listPlaylists(ctx, prefix)
		},
	},
	{ // playlist video file (before playlist dir to avoid shadowing)
		re:     `^playlists/([^/]+)/([^/]+)$`,
		isFile: true,
	},
	{ // playlist dir → list videos in playlist
		re: `^playlists/(.+)$`,
		toEntries: func(ctx context.Context, f lister, prefix string, match []string) (fs.DirEntries, error) {
			return f.listPlaylistVideos(ctx, prefix, match[1])
		},
	},
}.mustCompile()
