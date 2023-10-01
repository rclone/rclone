package bisync

import (
	"context"
	"fmt"
	"sort"

	"github.com/rclone/rclone/cmd/bisync/bilib"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/filter"
	"github.com/rclone/rclone/fs/operations"
	"github.com/rclone/rclone/fs/sync"
)

func (b *bisyncRun) fastCopy(ctx context.Context, fsrc, fdst fs.Fs, files bilib.Names, queueName string) error {
	if err := b.saveQueue(files, queueName); err != nil {
		return err
	}

	ctxCopy, filterCopy := filter.AddConfig(b.opt.setDryRun(ctx))
	for _, file := range files.ToList() {
		if err := filterCopy.AddFile(file); err != nil {
			return err
		}
	}

	var err error
	if b.opt.Resync {
		err = sync.CopyDir(ctxCopy, fdst, fsrc, b.opt.CreateEmptySrcDirs)
	} else {
		err = sync.Sync(ctxCopy, fdst, fsrc, b.opt.CreateEmptySrcDirs)
	}
	return err
}

// operation should be "make" or "remove"
func (b *bisyncRun) syncEmptyDirs(ctx context.Context, dst fs.Fs, candidates bilib.Names, dirsList *fileList, operation string) {
	if b.opt.CreateEmptySrcDirs && (!b.opt.Resync || operation == "make") {

		candidatesList := candidates.ToList()
		if operation == "remove" {
			// reverse the sort order to ensure we remove subdirs before parent dirs
			sort.Sort(sort.Reverse(sort.StringSlice(candidatesList)))
		}

		for _, s := range candidatesList {
			var direrr error
			if dirsList.has(s) { //make sure it's a dir, not a file
				if operation == "remove" {
					// directories made empty by the sync will have already been deleted during the sync
					// this just catches the already-empty ones (excluded from sync by --files-from filter)
					direrr = operations.TryRmdir(ctx, dst, s)
				} else if operation == "make" {
					direrr = operations.Mkdir(ctx, dst, s)
				} else {
					direrr = fmt.Errorf("invalid operation. Expected 'make' or 'remove', received '%q'", operation)
				}

				if direrr != nil {
					fs.Debugf(nil, "Error syncing directory: %v", direrr)
				}
			}
		}
	}
}

func (b *bisyncRun) saveQueue(files bilib.Names, jobName string) error {
	if !b.opt.SaveQueues {
		return nil
	}
	queueFile := fmt.Sprintf("%s.%s.que", b.basePath, jobName)
	return files.Save(queueFile)
}
