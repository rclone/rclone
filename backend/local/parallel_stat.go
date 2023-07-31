package local

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/accounting"
	"github.com/rclone/rclone/fs/filter"
	"github.com/rclone/rclone/fs/fserrors"
)

type (
	entryWithError struct {
		entry      os.FileInfo
		namepath   string
		entryError error
	}
	statJobStruct struct {
		dir       string
		name      string
		filter    *filter.Filter
		useFilter bool
		entryWG   *sync.WaitGroup
		entryCh   chan<- entryWithError
	}
)

const (
	dirListingStatSize  = 256 // Should never be greater than dirListingBatchSize
	dirListingBatchSize = 1024
)

func (f *Fs) doSingleStat(statJobInterface interface{}) {
	statJob, ok := statJobInterface.(*statJobStruct)
	if !ok {
		return
	}

	defer statJob.entryWG.Done()

	fsDirPath := f.localPath(statJob.dir)
	namepath := filepath.Join(fsDirPath, statJob.name)

	fi, fierr := os.Lstat(namepath)
	if os.IsNotExist(fierr) {
		// skip entry removed by a concurrent goroutine
		return
	}

	// Don't report errors on any file names that are excluded
	if fierr != nil && statJob.useFilter {
		newRemote := f.cleanRemote(statJob.dir, statJob.name)
		if !statJob.filter.IncludeRemote(newRemote) {
			return
		}
	}

	statJob.entryCh <- entryWithError{entry: fi, namepath: statJob.name, entryError: fierr}
}

func (f *Fs) doParallelStat(ctx context.Context, dir string, filter *filter.Filter, useFilter bool, names []string) []os.FileInfo {
	entriesCh := make(chan entryWithError, dirListingStatSize)

	entryWG := &sync.WaitGroup{}

	for _, name := range names {
		statJob := statJobStruct{
			dir:       dir,
			name:      name,
			filter:    filter,
			useFilter: useFilter,
			entryWG:   entryWG,
			entryCh:   entriesCh,
		}
		entryWG.Add(1)
		_ = f.lstatWorkerPool.Invoke(&statJob)
	}

	// close entriesCh channel when all workers are done processing the entries
	go func() {
		entryWG.Wait()
		close(entriesCh)
	}()

	fis := make([]os.FileInfo, 0, len(names))

	for e := range entriesCh {
		entryError := e.entryError
		if entryError != nil {
			entryError = fmt.Errorf("failed to get info about directory entry %q: %w", e.namepath, entryError)
			fs.Errorf(dir, "%v", entryError)
			_ = accounting.Stats(ctx).Error(fserrors.NoRetryError(entryError)) // fail the sync
			continue
		}
		fis = append(fis, e.entry)
	}

	return fis
}
