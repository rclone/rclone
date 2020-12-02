// dedupe - gets rid of identical files remotes which can have duplicate file names (drive, mega)

package operations

import (
	"context"
	"fmt"
	"log"
	"path"
	"sort"
	"strings"

	"github.com/pkg/errors"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config"
	"github.com/rclone/rclone/fs/hash"
	"github.com/rclone/rclone/fs/walk"
)

// dedupeRename renames the objs slice to different names
func dedupeRename(ctx context.Context, f fs.Fs, remote string, objs []fs.Object) {
	doMove := f.Features().Move
	if doMove == nil {
		log.Fatalf("Fs %v doesn't support Move", f)
	}
	ext := path.Ext(remote)
	base := remote[:len(remote)-len(ext)]

outer:
	for i, o := range objs {
		suffix := 1
		newName := fmt.Sprintf("%s-%d%s", base, i+suffix, ext)
		_, err := f.NewObject(ctx, newName)
		for ; err != fs.ErrorObjectNotFound; suffix++ {
			if err != nil {
				err = fs.CountError(err)
				fs.Errorf(o, "Failed to check for existing object: %v", err)
				continue outer
			}
			if suffix > 100 {
				fs.Errorf(o, "Could not find an available new name")
				continue outer
			}
			newName = fmt.Sprintf("%s-%d%s", base, i+suffix, ext)
			_, err = f.NewObject(ctx, newName)
		}
		if !SkipDestructive(ctx, o, "rename") {
			newObj, err := doMove(ctx, o, newName)
			if err != nil {
				err = fs.CountError(err)
				fs.Errorf(o, "Failed to rename: %v", err)
				continue
			}
			fs.Infof(newObj, "renamed from: %v", o)
		}
	}
}

// dedupeDeleteAllButOne deletes all but the one in keep
func dedupeDeleteAllButOne(ctx context.Context, keep int, remote string, objs []fs.Object) {
	count := 0
	for i, o := range objs {
		if i == keep {
			continue
		}
		err := DeleteFile(ctx, o)
		if err == nil {
			count++
		}
	}
	if count > 0 {
		fs.Logf(remote, "Deleted %d extra copies", count)
	}
}

// dedupeDeleteIdentical deletes all but one of identical (by hash) copies
func dedupeDeleteIdentical(ctx context.Context, ht hash.Type, remote string, objs []fs.Object) (remainingObjs []fs.Object) {
	ci := fs.GetConfig(ctx)

	// Make map of IDs
	IDs := make(map[string]int, len(objs))
	for _, o := range objs {
		if do, ok := o.(fs.IDer); ok {
			if ID := do.ID(); ID != "" {
				IDs[ID]++
			}
		}
	}

	// Remove duplicate IDs
	newObjs := objs[:0]
	for _, o := range objs {
		if do, ok := o.(fs.IDer); ok {
			if ID := do.ID(); ID != "" {
				if IDs[ID] <= 1 {
					newObjs = append(newObjs, o)
				} else {
					fs.Logf(o, "Ignoring as it appears %d times in the listing and deleting would lead to data loss", IDs[ID])
				}
			}
		}
	}
	objs = newObjs

	// See how many of these duplicates are identical
	dupesByID := make(map[string][]fs.Object, len(objs))
	for _, o := range objs {
		ID := ""
		if ci.SizeOnly && o.Size() >= 0 {
			ID = fmt.Sprintf("size %d", o.Size())
		} else if ht != hash.None {
			hashValue, err := o.Hash(ctx, ht)
			if err == nil && hashValue != "" {
				ID = fmt.Sprintf("%v %s", ht, hashValue)
			}
		}
		if ID == "" {
			remainingObjs = append(remainingObjs, o)
		} else {
			dupesByID[ID] = append(dupesByID[ID], o)
		}
	}

	// Delete identical duplicates, filling remainingObjs with the ones remaining
	for ID, dupes := range dupesByID {
		remainingObjs = append(remainingObjs, dupes[0])
		if len(dupes) > 1 {
			fs.Logf(remote, "Deleting %d/%d identical duplicates (%s)", len(dupes)-1, len(dupes), ID)
			for _, o := range dupes[1:] {
				err := DeleteFile(ctx, o)
				if err != nil {
					remainingObjs = append(remainingObjs, o)
				}
			}
		}
	}

	return remainingObjs
}

// dedupeList lists the duplicates and does nothing
func dedupeList(ctx context.Context, f fs.Fs, ht hash.Type, remote string, objs []fs.Object, byHash bool) {
	fmt.Printf("%s: %d duplicates\n", remote, len(objs))
	for i, o := range objs {
		hashValue := ""
		if ht != hash.None {
			var err error
			hashValue, err = o.Hash(ctx, ht)
			if err != nil {
				hashValue = err.Error()
			}
		}
		if byHash {
			fmt.Printf("  %d: %12d bytes, %s, %s\n", i+1, o.Size(), o.ModTime(ctx).Local().Format("2006-01-02 15:04:05.000000000"), o.Remote())
		} else {
			fmt.Printf("  %d: %12d bytes, %s, %v %32s\n", i+1, o.Size(), o.ModTime(ctx).Local().Format("2006-01-02 15:04:05.000000000"), ht, hashValue)
		}
	}
}

// dedupeInteractive interactively dedupes the slice of objects
func dedupeInteractive(ctx context.Context, f fs.Fs, ht hash.Type, remote string, objs []fs.Object, byHash bool) {
	dedupeList(ctx, f, ht, remote, objs, byHash)
	commands := []string{"sSkip and do nothing", "kKeep just one (choose which in next step)"}
	if !byHash {
		commands = append(commands, "rRename all to be different (by changing file.jpg to file-1.jpg)")
	}
	switch config.Command(commands) {
	case 's':
	case 'k':
		keep := config.ChooseNumber("Enter the number of the file to keep", 1, len(objs))
		dedupeDeleteAllButOne(ctx, keep-1, remote, objs)
	case 'r':
		dedupeRename(ctx, f, remote, objs)
	}
}

// DeduplicateMode is how the dedupe command chooses what to do
type DeduplicateMode int

// Deduplicate modes
const (
	DeduplicateInteractive DeduplicateMode = iota // interactively ask the user
	DeduplicateSkip                               // skip all conflicts
	DeduplicateFirst                              // choose the first object
	DeduplicateNewest                             // choose the newest object
	DeduplicateOldest                             // choose the oldest object
	DeduplicateRename                             // rename the objects
	DeduplicateLargest                            // choose the largest object
	DeduplicateSmallest                           // choose the smallest object
	DeduplicateList                               // list duplicates only
)

func (x DeduplicateMode) String() string {
	switch x {
	case DeduplicateInteractive:
		return "interactive"
	case DeduplicateSkip:
		return "skip"
	case DeduplicateFirst:
		return "first"
	case DeduplicateNewest:
		return "newest"
	case DeduplicateOldest:
		return "oldest"
	case DeduplicateRename:
		return "rename"
	case DeduplicateLargest:
		return "largest"
	case DeduplicateSmallest:
		return "smallest"
	case DeduplicateList:
		return "list"
	}
	return "unknown"
}

// Set a DeduplicateMode from a string
func (x *DeduplicateMode) Set(s string) error {
	switch strings.ToLower(s) {
	case "interactive":
		*x = DeduplicateInteractive
	case "skip":
		*x = DeduplicateSkip
	case "first":
		*x = DeduplicateFirst
	case "newest":
		*x = DeduplicateNewest
	case "oldest":
		*x = DeduplicateOldest
	case "rename":
		*x = DeduplicateRename
	case "largest":
		*x = DeduplicateLargest
	case "smallest":
		*x = DeduplicateSmallest
	case "list":
		*x = DeduplicateList
	default:
		return errors.Errorf("Unknown mode for dedupe %q.", s)
	}
	return nil
}

// Type of the value
func (x *DeduplicateMode) Type() string {
	return "string"
}

// dedupeFindDuplicateDirs scans f for duplicate directories
func dedupeFindDuplicateDirs(ctx context.Context, f fs.Fs) ([][]fs.Directory, error) {
	ci := fs.GetConfig(ctx)
	dirs := map[string][]fs.Directory{}
	err := walk.ListR(ctx, f, "", true, ci.MaxDepth, walk.ListDirs, func(entries fs.DirEntries) error {
		entries.ForDir(func(d fs.Directory) {
			dirs[d.Remote()] = append(dirs[d.Remote()], d)
		})
		return nil
	})
	if err != nil {
		return nil, errors.Wrap(err, "find duplicate dirs")
	}
	// make sure parents are before children
	duplicateNames := []string{}
	for name, ds := range dirs {
		if len(ds) > 1 {
			duplicateNames = append(duplicateNames, name)
		}
	}
	sort.Strings(duplicateNames)
	duplicateDirs := [][]fs.Directory{}
	for _, name := range duplicateNames {
		duplicateDirs = append(duplicateDirs, dirs[name])
	}
	return duplicateDirs, nil
}

// dedupeMergeDuplicateDirs merges all the duplicate directories found
func dedupeMergeDuplicateDirs(ctx context.Context, f fs.Fs, duplicateDirs [][]fs.Directory) error {
	mergeDirs := f.Features().MergeDirs
	if mergeDirs == nil {
		return errors.Errorf("%v: can't merge directories", f)
	}
	dirCacheFlush := f.Features().DirCacheFlush
	if dirCacheFlush == nil {
		return errors.Errorf("%v: can't flush dir cache", f)
	}
	for _, dirs := range duplicateDirs {
		if !SkipDestructive(ctx, dirs[0], "merge duplicate directories") {
			fs.Infof(dirs[0], "Merging contents of duplicate directories")
			err := mergeDirs(ctx, dirs)
			if err != nil {
				err = fs.CountError(err)
				fs.Errorf(nil, "merge duplicate dirs: %v", err)
			}
		}
	}
	dirCacheFlush()
	return nil
}

// sort oldest first
func sortOldestFirst(objs []fs.Object) {
	sort.Slice(objs, func(i, j int) bool {
		return objs[i].ModTime(context.TODO()).Before(objs[j].ModTime(context.TODO()))
	})
}

// sort smallest first
func sortSmallestFirst(objs []fs.Object) {
	sort.Slice(objs, func(i, j int) bool {
		return objs[i].Size() < objs[j].Size()
	})
}

// Deduplicate interactively finds duplicate files and offers to
// delete all but one or rename them to be different. Only useful with
// Google Drive which can have duplicate file names.
func Deduplicate(ctx context.Context, f fs.Fs, mode DeduplicateMode, byHash bool) error {
	ci := fs.GetConfig(ctx)
	// find a hash to use
	ht := f.Hashes().GetOne()
	what := "names"
	if byHash {
		if ht == hash.None {
			return errors.Errorf("%v has no hashes", f)
		}
		what = ht.String() + " hashes"
	}
	fs.Infof(f, "Looking for duplicate %s using %v mode.", what, mode)

	// Find duplicate directories first and fix them
	if !byHash {
		duplicateDirs, err := dedupeFindDuplicateDirs(ctx, f)
		if err != nil {
			return err
		}
		if len(duplicateDirs) != 0 {
			if mode != DeduplicateList {
				err = dedupeMergeDuplicateDirs(ctx, f, duplicateDirs)
				if err != nil {
					return err
				}
			} else {
				for _, dir := range duplicateDirs {
					fmt.Printf("%s: %d duplicates of this directory\n", dir[0].Remote(), len(dir))
				}
			}
		}
	}

	// Now find duplicate files
	files := map[string][]fs.Object{}
	err := walk.ListR(ctx, f, "", true, ci.MaxDepth, walk.ListObjects, func(entries fs.DirEntries) error {
		entries.ForObject(func(o fs.Object) {
			var remote string
			var err error
			if byHash {
				remote, err = o.Hash(ctx, ht)
				if err != nil {
					fs.Errorf(o, "Failed to hash: %v", err)
					remote = ""
				}
			} else {
				remote = o.Remote()
			}
			if remote != "" {
				files[remote] = append(files[remote], o)
			}
		})
		return nil
	})
	if err != nil {
		return err
	}

	for remote, objs := range files {
		if len(objs) > 1 {
			fs.Logf(remote, "Found %d files with duplicate %s", len(objs), what)
			if !byHash && mode != DeduplicateList {
				objs = dedupeDeleteIdentical(ctx, ht, remote, objs)
				if len(objs) <= 1 {
					fs.Logf(remote, "All duplicates removed")
					continue
				}
			}
			switch mode {
			case DeduplicateInteractive:
				dedupeInteractive(ctx, f, ht, remote, objs, byHash)
			case DeduplicateFirst:
				dedupeDeleteAllButOne(ctx, 0, remote, objs)
			case DeduplicateNewest:
				sortOldestFirst(objs)
				dedupeDeleteAllButOne(ctx, len(objs)-1, remote, objs)
			case DeduplicateOldest:
				sortOldestFirst(objs)
				dedupeDeleteAllButOne(ctx, 0, remote, objs)
			case DeduplicateRename:
				dedupeRename(ctx, f, remote, objs)
			case DeduplicateLargest:
				sortSmallestFirst(objs)
				dedupeDeleteAllButOne(ctx, len(objs)-1, remote, objs)
			case DeduplicateSmallest:
				sortSmallestFirst(objs)
				dedupeDeleteAllButOne(ctx, 0, remote, objs)
			case DeduplicateSkip:
				fs.Logf(remote, "Skipping %d files with duplicate %s", len(objs), what)
			case DeduplicateList:
				dedupeList(ctx, f, ht, remote, objs, byHash)
			default:
				//skip
			}
		}
	}
	return nil
}
