// dedupe - gets rid of identical files remotes which can have duplicate file names (drive, mega)

package operations

import (
	"fmt"
	"log"
	"path"
	"sort"
	"strings"

	"github.com/ncw/rclone/fs"
	"github.com/ncw/rclone/fs/config"
	"github.com/ncw/rclone/fs/hash"
	"github.com/ncw/rclone/fs/walk"
	"github.com/pkg/errors"
	"github.com/spf13/pflag"
)

// dedupeRename renames the objs slice to different names
func dedupeRename(f fs.Fs, remote string, objs []fs.Object) {
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
		_, err := f.NewObject(newName)
		for ; err != fs.ErrorObjectNotFound; suffix++ {
			if err != nil {
				fs.CountError(err)
				fs.Errorf(o, "Failed to check for existing object: %v", err)
				continue outer
			}
			if suffix > 100 {
				fs.Errorf(o, "Could not find an available new name")
				continue outer
			}
			newName = fmt.Sprintf("%s-%d%s", base, i+suffix, ext)
			_, err = f.NewObject(newName)
		}
		if !fs.Config.DryRun {
			newObj, err := doMove(o, newName)
			if err != nil {
				fs.CountError(err)
				fs.Errorf(o, "Failed to rename: %v", err)
				continue
			}
			fs.Infof(newObj, "renamed from: %v", o)
		} else {
			fs.Logf(remote, "Not renaming to %q as --dry-run", newName)
		}
	}
}

// dedupeDeleteAllButOne deletes all but the one in keep
func dedupeDeleteAllButOne(keep int, remote string, objs []fs.Object) {
	for i, o := range objs {
		if i == keep {
			continue
		}
		_ = DeleteFile(o)
	}
	fs.Logf(remote, "Deleted %d extra copies", len(objs)-1)
}

// dedupeDeleteIdentical deletes all but one of identical (by hash) copies
func dedupeDeleteIdentical(ht hash.Type, remote string, objs []fs.Object) (remainingObjs []fs.Object) {
	// See how many of these duplicates are identical
	byHash := make(map[string][]fs.Object, len(objs))
	for _, o := range objs {
		md5sum, err := o.Hash(ht)
		if err != nil || md5sum == "" {
			remainingObjs = append(remainingObjs, o)
		} else {
			byHash[md5sum] = append(byHash[md5sum], o)
		}
	}

	// Delete identical duplicates, filling remainingObjs with the ones remaining
	for md5sum, hashObjs := range byHash {
		if len(hashObjs) > 1 {
			fs.Logf(remote, "Deleting %d/%d identical duplicates (%v %q)", len(hashObjs)-1, len(hashObjs), ht, md5sum)
			for _, o := range hashObjs[1:] {
				_ = DeleteFile(o)
			}
		}
		remainingObjs = append(remainingObjs, hashObjs[0])
	}

	return remainingObjs
}

// dedupeInteractive interactively dedupes the slice of objects
func dedupeInteractive(f fs.Fs, ht hash.Type, remote string, objs []fs.Object) {
	fmt.Printf("%s: %d duplicates remain\n", remote, len(objs))
	for i, o := range objs {
		md5sum, err := o.Hash(ht)
		if err != nil {
			md5sum = err.Error()
		}
		fmt.Printf("  %d: %12d bytes, %s, %v %32s\n", i+1, o.Size(), o.ModTime().Local().Format("2006-01-02 15:04:05.000000000"), ht, md5sum)
	}
	switch config.Command([]string{"sSkip and do nothing", "kKeep just one (choose which in next step)", "rRename all to be different (by changing file.jpg to file-1.jpg)"}) {
	case 's':
	case 'k':
		keep := config.ChooseNumber("Enter the number of the file to keep", 1, len(objs))
		dedupeDeleteAllButOne(keep-1, remote, objs)
	case 'r':
		dedupeRename(f, remote, objs)
	}
}

type objectsSortedByModTime []fs.Object

func (objs objectsSortedByModTime) Len() int      { return len(objs) }
func (objs objectsSortedByModTime) Swap(i, j int) { objs[i], objs[j] = objs[j], objs[i] }
func (objs objectsSortedByModTime) Less(i, j int) bool {
	return objs[i].ModTime().Before(objs[j].ModTime())
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
	default:
		return errors.Errorf("Unknown mode for dedupe %q.", s)
	}
	return nil
}

// Type of the value
func (x *DeduplicateMode) Type() string {
	return "string"
}

// Check it satisfies the interface
var _ pflag.Value = (*DeduplicateMode)(nil)

// dedupeFindDuplicateDirs scans f for duplicate directories
func dedupeFindDuplicateDirs(f fs.Fs) ([][]fs.Directory, error) {
	dirs := map[string][]fs.Directory{}
	err := walk.ListR(f, "", true, fs.Config.MaxDepth, walk.ListDirs, func(entries fs.DirEntries) error {
		entries.ForDir(func(d fs.Directory) {
			dirs[d.Remote()] = append(dirs[d.Remote()], d)
		})
		return nil
	})
	if err != nil {
		return nil, errors.Wrap(err, "find duplicate dirs")
	}
	duplicateDirs := [][]fs.Directory{}
	for _, ds := range dirs {
		if len(ds) > 1 {
			duplicateDirs = append(duplicateDirs, ds)
		}
	}
	return duplicateDirs, nil
}

// dedupeMergeDuplicateDirs merges all the duplicate directories found
func dedupeMergeDuplicateDirs(f fs.Fs, duplicateDirs [][]fs.Directory) error {
	mergeDirs := f.Features().MergeDirs
	if mergeDirs == nil {
		return errors.Errorf("%v: can't merge directories", f)
	}
	dirCacheFlush := f.Features().DirCacheFlush
	if dirCacheFlush == nil {
		return errors.Errorf("%v: can't flush dir cache", f)
	}
	for _, dirs := range duplicateDirs {
		if !fs.Config.DryRun {
			fs.Infof(dirs[0], "Merging contents of duplicate directories")
			err := mergeDirs(dirs)
			if err != nil {
				return errors.Wrap(err, "merge duplicate dirs")
			}
		} else {
			fs.Infof(dirs[0], "NOT Merging contents of duplicate directories as --dry-run")
		}
	}
	dirCacheFlush()
	return nil
}

// Deduplicate interactively finds duplicate files and offers to
// delete all but one or rename them to be different. Only useful with
// Google Drive which can have duplicate file names.
func Deduplicate(f fs.Fs, mode DeduplicateMode) error {
	fs.Infof(f, "Looking for duplicates using %v mode.", mode)

	// Find duplicate directories first and fix them - repeat
	// until all fixed
	for {
		duplicateDirs, err := dedupeFindDuplicateDirs(f)
		if err != nil {
			return err
		}
		if len(duplicateDirs) == 0 {
			break
		}
		err = dedupeMergeDuplicateDirs(f, duplicateDirs)
		if err != nil {
			return err
		}
		if fs.Config.DryRun {
			break
		}
	}

	// find a hash to use
	ht := f.Hashes().GetOne()

	// Now find duplicate files
	files := map[string][]fs.Object{}
	err := walk.ListR(f, "", true, fs.Config.MaxDepth, walk.ListObjects, func(entries fs.DirEntries) error {
		entries.ForObject(func(o fs.Object) {
			remote := o.Remote()
			files[remote] = append(files[remote], o)
		})
		return nil
	})
	if err != nil {
		return err
	}

	for remote, objs := range files {
		if len(objs) > 1 {
			fs.Logf(remote, "Found %d duplicates - deleting identical copies", len(objs))
			objs = dedupeDeleteIdentical(ht, remote, objs)
			if len(objs) <= 1 {
				fs.Logf(remote, "All duplicates removed")
				continue
			}
			switch mode {
			case DeduplicateInteractive:
				dedupeInteractive(f, ht, remote, objs)
			case DeduplicateFirst:
				dedupeDeleteAllButOne(0, remote, objs)
			case DeduplicateNewest:
				sort.Sort(objectsSortedByModTime(objs)) // sort oldest first
				dedupeDeleteAllButOne(len(objs)-1, remote, objs)
			case DeduplicateOldest:
				sort.Sort(objectsSortedByModTime(objs)) // sort oldest first
				dedupeDeleteAllButOne(0, remote, objs)
			case DeduplicateRename:
				dedupeRename(f, remote, objs)
			case DeduplicateLargest:
				largest, largestIndex := int64(-1), -1
				for i, obj := range objs {
					size := obj.Size()
					if size > largest {
						largest, largestIndex = size, i
					}
				}
				if largestIndex > -1 {
					dedupeDeleteAllButOne(largestIndex, remote, objs)
				}
			case DeduplicateSkip:
				// skip
			default:
				//skip
			}
		}
	}
	return nil
}
