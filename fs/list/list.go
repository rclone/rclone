// Package list contains list functions
package list

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/accounting"
	"github.com/rclone/rclone/fs/filter"
	"github.com/rclone/rclone/lib/bucket"
)

// DirSorted reads Object and *Dir into entries for the given Fs.
//
// dir is the start directory, "" for root
//
// If includeAll is specified all files will be added, otherwise only
// files and directories passing the filter will be added.
//
// Files will be returned in sorted order
func DirSorted(ctx context.Context, f fs.Fs, includeAll bool, dir string) (entries fs.DirEntries, err error) {
	// Get unfiltered entries from the fs
	entries, err = f.List(ctx, dir)
	accounting.Stats(ctx).Listed(int64(len(entries)))
	if err != nil {
		return nil, err
	}
	// This should happen only if exclude files lives in the
	// starting directory, otherwise ListDirSorted should not be
	// called.
	fi := filter.GetConfig(ctx)
	if !includeAll && fi.ListContainsExcludeFile(entries) {
		fs.Debugf(dir, "Excluded")
		return nil, nil
	}
	return filterAndSortDir(ctx, entries, includeAll, dir, fi.IncludeObject, fi.IncludeDirectory(ctx, f))
}

// listP for every backend
func listP(ctx context.Context, f fs.Fs, dir string, callback fs.ListRCallback) error {
	if doListP := f.Features().ListP; doListP != nil {
		return doListP(ctx, dir, callback)
	}
	// Fallback to List
	entries, err := f.List(ctx, dir)
	if err != nil {
		return err
	}
	return callback(entries)
}

// DirSortedFn reads Object and *Dir into entries for the given Fs.
//
// dir is the start directory, "" for root
//
// If includeAll is specified all files will be added, otherwise only
// files and directories passing the filter will be added.
//
// Files will be returned through callback in sorted order
func DirSortedFn(ctx context.Context, f fs.Fs, includeAll bool, dir string, callback fs.ListRCallback, keyFn KeyFn) (err error) {
	stats := accounting.Stats(ctx)
	fi := filter.GetConfig(ctx)

	// Sort the entries, in or out of memory
	sorter, err := NewSorter(ctx, f, callback, keyFn)
	if err != nil {
		return fmt.Errorf("failed to create directory sorter: %w", err)
	}
	defer sorter.CleanUp()

	// Get unfiltered entries from the fs
	err = listP(ctx, f, dir, func(entries fs.DirEntries) error {
		stats.Listed(int64(len(entries)))

		// This should happen only if exclude files lives in the
		// starting directory, otherwise ListDirSorted should not be
		// called.
		if !includeAll && fi.ListContainsExcludeFile(entries) {
			fs.Debugf(dir, "Excluded")
			return nil
		}

		entries, err := filterDir(ctx, entries, includeAll, dir, fi.IncludeObject, fi.IncludeDirectory(ctx, f))
		if err != nil {
			return err
		}
		return sorter.Add(entries)
	})
	if err != nil {
		return err
	}
	return sorter.Send()
}

// Filter the entries passed in
func filterDir(ctx context.Context, entries fs.DirEntries, includeAll bool, dir string,
	IncludeObject func(ctx context.Context, o fs.Object) bool,
	IncludeDirectory func(remote string) (bool, error)) (newEntries fs.DirEntries, err error) {
	newEntries = entries[:0] // in place filter
	prefix := ""
	if dir != "" {
		prefix = dir
		if !bucket.IsAllSlashes(dir) {
			prefix += "/"
		}
	}
	for _, entry := range entries {
		ok := true
		// check includes and types
		switch x := entry.(type) {
		case fs.Object:
			// Make sure we don't delete excluded files if not required
			if !includeAll && !IncludeObject(ctx, x) {
				ok = false
				fs.Debugf(x, "Excluded")
			}
		case fs.Directory:
			if !includeAll {
				include, err := IncludeDirectory(x.Remote())
				if err != nil {
					return nil, err
				}
				if !include {
					ok = false
					fs.Debugf(x, "Excluded")
				}
			}
		default:
			return nil, fmt.Errorf("unknown object type %T", entry)
		}
		// check remote name belongs in this directory
		remote := entry.Remote()
		switch {
		case !ok:
			// ignore
		case !strings.HasPrefix(remote, prefix):
			ok = false
			fs.Errorf(entry, "Entry doesn't belong in directory %q (too short) - ignoring", dir)
		case remote == dir:
			ok = false
			fs.Errorf(entry, "Entry doesn't belong in directory %q (same as directory) - ignoring", dir)
		case strings.ContainsRune(remote[len(prefix):], '/') && !bucket.IsAllSlashes(remote[len(prefix):]):
			ok = false
			fs.Errorf(entry, "Entry doesn't belong in directory %q (contains subdir) - ignoring", dir)
		default:
			// ok
		}
		if ok {
			newEntries = append(newEntries, entry)
		}
	}
	return newEntries, nil
}

// filter and sort the entries
func filterAndSortDir(ctx context.Context, entries fs.DirEntries, includeAll bool, dir string,
	IncludeObject func(ctx context.Context, o fs.Object) bool,
	IncludeDirectory func(remote string) (bool, error)) (newEntries fs.DirEntries, err error) {
	// Filter the directory entries (in place)
	entries, err = filterDir(ctx, entries, includeAll, dir, IncludeObject, IncludeDirectory)
	if err != nil {
		return nil, err
	}

	// Sort the directory entries by Remote
	//
	// We use a stable sort here just in case there are
	// duplicates. Assuming the remote delivers the entries in a
	// consistent order, this will give the best user experience
	// in syncing as it will use the first entry for the sync
	// comparison.
	sort.Stable(entries)
	return entries, nil
}
