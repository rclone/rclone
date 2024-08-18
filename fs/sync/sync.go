// Package sync is the implementation of sync/copy/move
package sync

import (
	"context"
	"errors"
	"fmt"
	"path"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/accounting"
	"github.com/rclone/rclone/fs/filter"
	"github.com/rclone/rclone/fs/fserrors"
	"github.com/rclone/rclone/fs/hash"
	"github.com/rclone/rclone/fs/march"
	"github.com/rclone/rclone/fs/operations"
	"github.com/rclone/rclone/lib/errcount"
	"golang.org/x/sync/errgroup"
)

// ErrorMaxDurationReached defines error when transfer duration is reached
// Used for checking on exit and matching to correct exit code.
var ErrorMaxDurationReached = errors.New("max transfer duration reached as set by --max-duration")

// ErrorMaxDurationReachedFatal is returned from when the max
// duration limit is reached.
var ErrorMaxDurationReachedFatal = fserrors.FatalError(ErrorMaxDurationReached)

type syncCopyMove struct {
	// parameters
	fdst               fs.Fs
	fsrc               fs.Fs
	deleteMode         fs.DeleteMode // how we are doing deletions
	DoMove             bool
	copyEmptySrcDirs   bool
	deleteEmptySrcDirs bool
	dir                string
	// internal state
	ci                     *fs.ConfigInfo         // global config
	fi                     *filter.Filter         // filter config
	ctx                    context.Context        // internal context for controlling go-routines
	cancel                 func()                 // cancel the context
	inCtx                  context.Context        // internal context for controlling march
	inCancel               func()                 // cancel the march context
	noTraverse             bool                   // if set don't traverse the dst
	noCheckDest            bool                   // if set transfer all objects regardless without checking dst
	noUnicodeNormalization bool                   // don't normalize unicode characters in filenames
	deletersWg             sync.WaitGroup         // for delete before go routine
	deleteFilesCh          chan fs.Object         // channel to receive deletes if delete before
	trackRenames           bool                   // set if we should do server-side renames
	trackRenamesStrategy   trackRenamesStrategy   // strategies used for tracking renames
	dstFilesMu             sync.Mutex             // protect dstFiles
	dstFiles               map[string]fs.Object   // dst files, always filled
	srcFiles               map[string]fs.Object   // src files, only used if deleteBefore
	srcFilesChan           chan fs.Object         // passes src objects
	srcFilesResult         chan error             // error result of src listing
	dstFilesResult         chan error             // error result of dst listing
	dstEmptyDirsMu         sync.Mutex             // protect dstEmptyDirs
	dstEmptyDirs           map[string]fs.DirEntry // potentially empty directories
	srcEmptyDirsMu         sync.Mutex             // protect srcEmptyDirs
	srcEmptyDirs           map[string]fs.DirEntry // potentially empty directories
	srcMoveEmptyDirs       map[string]fs.DirEntry // potentially empty directories when moving files out of them
	checkerWg              sync.WaitGroup         // wait for checkers
	toBeChecked            *pipe                  // checkers channel
	transfersWg            sync.WaitGroup         // wait for transfers
	toBeUploaded           *pipe                  // copiers channel
	errorMu                sync.Mutex             // Mutex covering the errors variables
	err                    error                  // normal error from copy process
	noRetryErr             error                  // error with NoRetry set
	fatalErr               error                  // fatal error
	commonHash             hash.Type              // common hash type between src and dst
	modifyWindow           time.Duration          // modify window between fsrc, fdst
	renameMapMu            sync.Mutex             // mutex to protect the below
	renameMap              map[string][]fs.Object // dst files by hash - only used by trackRenames
	renamerWg              sync.WaitGroup         // wait for renamers
	toBeRenamed            *pipe                  // renamers channel
	trackRenamesWg         sync.WaitGroup         // wg for background track renames
	trackRenamesCh         chan fs.Object         // objects are pumped in here
	renameCheck            []fs.Object            // accumulate files to check for rename here
	compareCopyDest        []fs.Fs                // place to check for files to server side copy
	backupDir              fs.Fs                  // place to store overwrites/deletes
	checkFirst             bool                   // if set run all the checkers before starting transfers
	maxDurationEndTime     time.Time              // end time if --max-duration is set
	logger                 operations.LoggerFn    // LoggerFn used to report the results of a sync (or bisync) to an io.Writer
	usingLogger            bool                   // whether we are using logger
	setDirMetadata         bool                   // if set we set the directory metadata
	setDirModTime          bool                   // if set we set the directory modtimes
	setDirModTimeAfter     bool                   // if set we set the directory modtimes at the end of the sync
	setDirModTimeMu        sync.Mutex             // protect setDirModTimes and modifiedDirs
	setDirModTimes         []setDirModTime        // directories that need their modtime set
	setDirModTimesMaxLevel int                    // max level of the directories to set
	modifiedDirs           map[string]struct{}    // dirs with changed contents (if s.setDirModTimeAfter)
}

// For keeping track of delayed modtime sets
type setDirModTime struct {
	src     fs.Directory
	dst     fs.Directory
	dir     string
	modTime time.Time
	level   int // the level of the directory, 0 is root
}

type trackRenamesStrategy byte

const (
	trackRenamesStrategyHash trackRenamesStrategy = 1 << iota
	trackRenamesStrategyModtime
	trackRenamesStrategyLeaf
)

func (strategy trackRenamesStrategy) hash() bool {
	return (strategy & trackRenamesStrategyHash) != 0
}

func (strategy trackRenamesStrategy) modTime() bool {
	return (strategy & trackRenamesStrategyModtime) != 0
}

func (strategy trackRenamesStrategy) leaf() bool {
	return (strategy & trackRenamesStrategyLeaf) != 0
}

func newSyncCopyMove(ctx context.Context, fdst, fsrc fs.Fs, deleteMode fs.DeleteMode, DoMove bool, deleteEmptySrcDirs bool, copyEmptySrcDirs bool) (*syncCopyMove, error) {
	if (deleteMode != fs.DeleteModeOff || DoMove) && operations.OverlappingFilterCheck(ctx, fdst, fsrc) {
		return nil, fserrors.FatalError(fs.ErrorOverlapping)
	}
	ci := fs.GetConfig(ctx)
	fi := filter.GetConfig(ctx)
	s := &syncCopyMove{
		ci:                     ci,
		fi:                     fi,
		fdst:                   fdst,
		fsrc:                   fsrc,
		deleteMode:             deleteMode,
		DoMove:                 DoMove,
		copyEmptySrcDirs:       copyEmptySrcDirs,
		deleteEmptySrcDirs:     deleteEmptySrcDirs,
		dir:                    "",
		srcFilesChan:           make(chan fs.Object, ci.Checkers+ci.Transfers),
		srcFilesResult:         make(chan error, 1),
		dstFilesResult:         make(chan error, 1),
		dstEmptyDirs:           make(map[string]fs.DirEntry),
		srcEmptyDirs:           make(map[string]fs.DirEntry),
		srcMoveEmptyDirs:       make(map[string]fs.DirEntry),
		noTraverse:             ci.NoTraverse,
		noCheckDest:            ci.NoCheckDest,
		noUnicodeNormalization: ci.NoUnicodeNormalization,
		deleteFilesCh:          make(chan fs.Object, ci.Checkers),
		trackRenames:           ci.TrackRenames,
		commonHash:             fsrc.Hashes().Overlap(fdst.Hashes()).GetOne(),
		modifyWindow:           fs.GetModifyWindow(ctx, fsrc, fdst),
		trackRenamesCh:         make(chan fs.Object, ci.Checkers),
		checkFirst:             ci.CheckFirst,
		setDirMetadata:         ci.Metadata && fsrc.Features().ReadDirMetadata && fdst.Features().WriteDirMetadata,
		setDirModTime:          (!ci.NoUpdateDirModTime && fsrc.Features().CanHaveEmptyDirectories) && (fdst.Features().WriteDirSetModTime || fdst.Features().MkdirMetadata != nil || fdst.Features().DirSetModTime != nil),
		setDirModTimeAfter:     !ci.NoUpdateDirModTime && (!copyEmptySrcDirs || fsrc.Features().CanHaveEmptyDirectories && fdst.Features().DirModTimeUpdatesOnWrite),
		modifiedDirs:           make(map[string]struct{}),
	}

	s.logger, s.usingLogger = operations.GetLogger(ctx)

	if deleteMode == fs.DeleteModeOff {
		loggerOpt := operations.GetLoggerOpt(ctx)
		loggerOpt.DeleteModeOff = true
		loggerOpt.LoggerFn = s.logger
		ctx = operations.WithLoggerOpt(ctx, loggerOpt)
	}

	backlog := ci.MaxBacklog
	if s.checkFirst {
		fs.Infof(s.fdst, "Running all checks before starting transfers")
		backlog = -1
	}
	var err error
	s.toBeChecked, err = newPipe(ci.OrderBy, accounting.Stats(ctx).SetCheckQueue, backlog)
	if err != nil {
		return nil, err
	}
	s.toBeUploaded, err = newPipe(ci.OrderBy, accounting.Stats(ctx).SetTransferQueue, backlog)
	if err != nil {
		return nil, err
	}
	s.toBeRenamed, err = newPipe(ci.OrderBy, accounting.Stats(ctx).SetRenameQueue, backlog)
	if err != nil {
		return nil, err
	}
	if ci.MaxDuration > 0 {
		s.maxDurationEndTime = time.Now().Add(ci.MaxDuration)
		fs.Infof(s.fdst, "Transfer session %v deadline: %s", ci.CutoffMode, s.maxDurationEndTime.Format("2006/01/02 15:04:05"))
	}
	// If a max session duration has been defined add a deadline
	// to the main context if cutoff mode is hard. This will cut
	// the transfers off.
	if !s.maxDurationEndTime.IsZero() && ci.CutoffMode == fs.CutoffModeHard {
		s.ctx, s.cancel = context.WithDeadline(ctx, s.maxDurationEndTime)
	} else {
		s.ctx, s.cancel = context.WithCancel(ctx)
	}
	// Input context - cancel this for graceful stop.
	//
	// If a max session duration has been defined add a deadline
	// to the input context if cutoff mode is graceful or soft.
	// This won't stop the transfers but will cut the
	// list/check/transfer pipelines.
	if !s.maxDurationEndTime.IsZero() && ci.CutoffMode != fs.CutoffModeHard {
		s.inCtx, s.inCancel = context.WithDeadline(s.ctx, s.maxDurationEndTime)
	} else {
		s.inCtx, s.inCancel = context.WithCancel(s.ctx)
	}
	if s.noTraverse && s.deleteMode != fs.DeleteModeOff {
		if !fi.HaveFilesFrom() {
			fs.Errorf(nil, "Ignoring --no-traverse with sync")
		}
		s.noTraverse = false
	}
	s.trackRenamesStrategy, err = parseTrackRenamesStrategy(ci.TrackRenamesStrategy)
	if err != nil {
		return nil, err
	}
	if s.noCheckDest {
		if s.deleteMode != fs.DeleteModeOff {
			return nil, errors.New("can't use --no-check-dest with sync: use copy instead")
		}
		if ci.Immutable {
			return nil, errors.New("can't use --no-check-dest with --immutable")
		}
		if s.backupDir != nil {
			return nil, errors.New("can't use --no-check-dest with --backup-dir")
		}
	}
	if s.trackRenames {
		// Don't track renames for remotes without server-side move support.
		if !operations.CanServerSideMove(fdst) {
			fs.Errorf(fdst, "Ignoring --track-renames as the destination does not support server-side move or copy")
			s.trackRenames = false
		}
		if s.trackRenamesStrategy.hash() && s.commonHash == hash.None {
			fs.Errorf(fdst, "Ignoring --track-renames as the source and destination do not have a common hash")
			s.trackRenames = false
		}

		if s.trackRenamesStrategy.modTime() && s.modifyWindow == fs.ModTimeNotSupported {
			fs.Errorf(fdst, "Ignoring --track-renames as either the source or destination do not support modtime")
			s.trackRenames = false
		}

		if s.deleteMode == fs.DeleteModeOff {
			fs.Errorf(fdst, "Ignoring --track-renames as it doesn't work with copy or move, only sync")
			s.trackRenames = false
		}
	}
	if s.trackRenames {
		// track renames needs delete after
		if s.deleteMode != fs.DeleteModeOff {
			s.deleteMode = fs.DeleteModeAfter
		}
		if s.noTraverse {
			fs.Errorf(nil, "Ignoring --no-traverse with --track-renames")
			s.noTraverse = false
		}
	}
	// Make Fs for --backup-dir if required
	if ci.BackupDir != "" || ci.Suffix != "" {
		var err error
		s.backupDir, err = operations.BackupDir(ctx, fdst, fsrc, "")
		if err != nil {
			return nil, err
		}
	}
	if len(ci.CompareDest) > 0 {
		var err error
		s.compareCopyDest, err = operations.GetCompareDest(ctx)
		if err != nil {
			return nil, err
		}
	} else if len(ci.CopyDest) > 0 {
		var err error
		s.compareCopyDest, err = operations.GetCopyDest(ctx, fdst)
		if err != nil {
			return nil, err
		}
	}
	return s, nil
}

// Check to see if the context has been cancelled
func (s *syncCopyMove) aborting() bool {
	return s.ctx.Err() != nil
}

// This reads the map and pumps it into the channel passed in, closing
// the channel at the end
func (s *syncCopyMove) pumpMapToChan(files map[string]fs.Object, out chan<- fs.Object) {
outer:
	for _, o := range files {
		if s.aborting() {
			break outer
		}
		select {
		case out <- o:
		case <-s.ctx.Done():
			break outer
		}
	}
	close(out)
	s.srcFilesResult <- nil
}

// This checks the types of errors returned while copying files
func (s *syncCopyMove) processError(err error) {
	if err == nil {
		return
	}
	if err == context.DeadlineExceeded {
		err = fserrors.NoRetryError(err)
	} else if err == accounting.ErrorMaxTransferLimitReachedGraceful {
		if s.inCtx.Err() == nil {
			fs.Logf(nil, "%v - stopping transfers", err)
			// Cancel the march and stop the pipes
			s.inCancel()
		}
	} else if err == context.Canceled && s.inCtx.Err() != nil {
		// Ignore context Canceled if we have called s.inCancel()
		return
	}
	s.errorMu.Lock()
	defer s.errorMu.Unlock()
	switch {
	case fserrors.IsFatalError(err):
		if !s.aborting() {
			fs.Errorf(nil, "Cancelling sync due to fatal error: %v", err)
			s.cancel()
		}
		s.fatalErr = err
	case fserrors.IsNoRetryError(err):
		s.noRetryErr = err
	default:
		s.err = err
	}
}

// Returns the current error (if any) in the order of precedence
//
//	fatalErr
//	normal error
//	noRetryErr
func (s *syncCopyMove) currentError() error {
	s.errorMu.Lock()
	defer s.errorMu.Unlock()
	if s.fatalErr != nil {
		return s.fatalErr
	}
	if s.err != nil {
		return s.err
	}
	return s.noRetryErr
}

// pairChecker reads Objects~s on in send to out if they need transferring.
//
// FIXME potentially doing lots of hashes at once
func (s *syncCopyMove) pairChecker(in *pipe, out *pipe, fraction int, wg *sync.WaitGroup) {
	defer wg.Done()
	for {
		pair, ok := in.GetMax(s.inCtx, fraction)
		if !ok {
			return
		}
		src := pair.Src
		var err error
		tr := accounting.Stats(s.ctx).NewCheckingTransfer(src, "checking")
		// Check to see if can store this
		if src.Storable() {
			needTransfer := operations.NeedTransfer(s.ctx, pair.Dst, pair.Src)
			if needTransfer {
				NoNeedTransfer, err := operations.CompareOrCopyDest(s.ctx, s.fdst, pair.Dst, pair.Src, s.compareCopyDest, s.backupDir)
				if err != nil {
					s.processError(err)
					s.logger(s.ctx, operations.TransferError, pair.Src, pair.Dst, err)
				}
				if NoNeedTransfer {
					needTransfer = false
				}
			}
			// Fix case for case insensitive filesystems
			if s.ci.FixCase && !s.ci.Immutable && src.Remote() != pair.Dst.Remote() {
				if newDst, err := operations.Move(s.ctx, s.fdst, nil, src.Remote(), pair.Dst); err != nil {
					fs.Errorf(pair.Dst, "Error while attempting to rename to %s: %v", src.Remote(), err)
					s.processError(err)
				} else {
					fs.Infof(pair.Dst, "Fixed case by renaming to: %s", src.Remote())
					pair.Dst = newDst
				}
			}
			if needTransfer {
				// If files are treated as immutable, fail if destination exists and does not match
				if s.ci.Immutable && pair.Dst != nil {
					err := fs.CountError(fserrors.NoRetryError(fs.ErrorImmutableModified))
					fs.Errorf(pair.Dst, "Source and destination exist but do not match: %v", err)
					s.processError(err)
				} else {
					if pair.Dst != nil {
						s.markDirModifiedObject(pair.Dst)
					} else {
						s.markDirModifiedObject(src)
					}
					// If destination already exists, then we must move it into --backup-dir if required
					if pair.Dst != nil && s.backupDir != nil {
						err := operations.MoveBackupDir(s.ctx, s.backupDir, pair.Dst)
						if err != nil {
							s.processError(err)
							s.logger(s.ctx, operations.TransferError, pair.Src, pair.Dst, err)
						} else {
							// If successful zero out the dst as it is no longer there and copy the file
							pair.Dst = nil
							ok = out.Put(s.inCtx, pair)
							if !ok {
								return
							}
						}
					} else {
						ok = out.Put(s.inCtx, pair)
						if !ok {
							return
						}
					}
				}
			} else {
				// If moving need to delete the files we don't need to copy
				if s.DoMove {
					// Delete src if no error on copy
					if operations.SameObject(src, pair.Dst) {
						fs.Logf(src, "Not removing source file as it is the same file as the destination")
					} else if s.ci.IgnoreExisting {
						fs.Debugf(src, "Not removing source file as destination file exists and --ignore-existing is set")
					} else if s.checkFirst && s.ci.OrderBy != "" {
						// If we want perfect ordering then use the transfers to delete the file
						//
						// We send src == dst, to say we want the src deleted
						ok = out.Put(s.inCtx, fs.ObjectPair{Src: src, Dst: src})
						if !ok {
							return
						}
					} else {
						deleteFileErr := operations.DeleteFile(s.ctx, src)
						s.processError(deleteFileErr)
						s.logger(s.ctx, operations.TransferError, pair.Src, pair.Dst, deleteFileErr)
					}
				}
			}
		}
		tr.Done(s.ctx, err)
	}
}

// pairRenamer reads Objects~s on in and attempts to rename them,
// otherwise it sends them out if they need transferring.
func (s *syncCopyMove) pairRenamer(in *pipe, out *pipe, fraction int, wg *sync.WaitGroup) {
	defer wg.Done()
	for {
		pair, ok := in.GetMax(s.inCtx, fraction)
		if !ok {
			return
		}
		src := pair.Src
		if !s.tryRename(src) {
			// pass on if not renamed
			fs.Debugf(src, "Need to transfer - No matching file found at Destination")
			ok = out.Put(s.inCtx, pair)
			if !ok {
				return
			}
		}
	}
}

// pairCopyOrMove reads Objects on in and moves or copies them.
func (s *syncCopyMove) pairCopyOrMove(ctx context.Context, in *pipe, fdst fs.Fs, fraction int, wg *sync.WaitGroup) {
	defer wg.Done()
	var err error
	for {
		pair, ok := in.GetMax(s.inCtx, fraction)
		if !ok {
			return
		}
		src := pair.Src
		dst := pair.Dst
		if s.DoMove {
			if src != dst {
				_, err = operations.MoveTransfer(ctx, fdst, dst, src.Remote(), src)
			} else {
				// src == dst signals delete the src
				err = operations.DeleteFile(ctx, src)
			}
		} else {
			_, err = operations.Copy(ctx, fdst, dst, src.Remote(), src)
		}
		s.processError(err)
		if err != nil {
			s.logger(ctx, operations.TransferError, src, dst, err)
		}
	}
}

// This starts the background checkers.
func (s *syncCopyMove) startCheckers() {
	s.checkerWg.Add(s.ci.Checkers)
	for i := 0; i < s.ci.Checkers; i++ {
		fraction := (100 * i) / s.ci.Checkers
		go s.pairChecker(s.toBeChecked, s.toBeUploaded, fraction, &s.checkerWg)
	}
}

// This stops the background checkers
func (s *syncCopyMove) stopCheckers() {
	s.toBeChecked.Close()
	fs.Debugf(s.fdst, "Waiting for checks to finish")
	s.checkerWg.Wait()
}

// This starts the background transfers
func (s *syncCopyMove) startTransfers() {
	s.transfersWg.Add(s.ci.Transfers)
	for i := 0; i < s.ci.Transfers; i++ {
		fraction := (100 * i) / s.ci.Transfers
		go s.pairCopyOrMove(s.ctx, s.toBeUploaded, s.fdst, fraction, &s.transfersWg)
	}
}

// This stops the background transfers
func (s *syncCopyMove) stopTransfers() {
	s.toBeUploaded.Close()
	fs.Debugf(s.fdst, "Waiting for transfers to finish")
	s.transfersWg.Wait()
}

// This starts the background renamers.
func (s *syncCopyMove) startRenamers() {
	if !s.trackRenames {
		return
	}
	s.renamerWg.Add(s.ci.Checkers)
	for i := 0; i < s.ci.Checkers; i++ {
		fraction := (100 * i) / s.ci.Checkers
		go s.pairRenamer(s.toBeRenamed, s.toBeUploaded, fraction, &s.renamerWg)
	}
}

// This stops the background renamers
func (s *syncCopyMove) stopRenamers() {
	if !s.trackRenames {
		return
	}
	s.toBeRenamed.Close()
	fs.Debugf(s.fdst, "Waiting for renames to finish")
	s.renamerWg.Wait()
}

// This starts the collection of possible renames
func (s *syncCopyMove) startTrackRenames() {
	if !s.trackRenames {
		return
	}
	s.trackRenamesWg.Add(1)
	go func() {
		defer s.trackRenamesWg.Done()
		for o := range s.trackRenamesCh {
			s.renameCheck = append(s.renameCheck, o)
		}
	}()
}

// This stops the background rename collection
func (s *syncCopyMove) stopTrackRenames() {
	if !s.trackRenames {
		return
	}
	close(s.trackRenamesCh)
	s.trackRenamesWg.Wait()
}

// This starts the background deletion of files for --delete-during
func (s *syncCopyMove) startDeleters() {
	if s.deleteMode != fs.DeleteModeDuring && s.deleteMode != fs.DeleteModeOnly {
		return
	}
	s.deletersWg.Add(1)
	go func() {
		defer s.deletersWg.Done()
		err := operations.DeleteFilesWithBackupDir(s.ctx, s.deleteFilesCh, s.backupDir)
		s.processError(err)
	}()
}

// This stops the background deleters
func (s *syncCopyMove) stopDeleters() {
	if s.deleteMode != fs.DeleteModeDuring && s.deleteMode != fs.DeleteModeOnly {
		return
	}
	close(s.deleteFilesCh)
	s.deletersWg.Wait()
}

// This deletes the files in the dstFiles map.  If checkSrcMap is set
// then it checks to see if they exist first in srcFiles the source
// file map, otherwise it unconditionally deletes them.  If
// checkSrcMap is clear then it assumes that the any source files that
// have been found have been removed from dstFiles already.
func (s *syncCopyMove) deleteFiles(checkSrcMap bool) error {
	if accounting.Stats(s.ctx).Errored() && !s.ci.IgnoreErrors {
		fs.Errorf(s.fdst, "%v", fs.ErrorNotDeleting)
		// log all deletes as errors
		for remote, o := range s.dstFiles {
			if checkSrcMap {
				_, exists := s.srcFiles[remote]
				if exists {
					continue
				}
			}
			s.logger(s.ctx, operations.TransferError, nil, o, fs.ErrorNotDeleting)
		}
		return fs.ErrorNotDeleting
	}

	// Delete the spare files
	toDelete := make(fs.ObjectsChan, s.ci.Checkers)
	go func() {
	outer:
		for remote, o := range s.dstFiles {
			if checkSrcMap {
				_, exists := s.srcFiles[remote]
				if exists {
					continue
				}
			}
			if s.aborting() {
				break
			}
			select {
			case <-s.ctx.Done():
				break outer
			case toDelete <- o:
			}
		}
		close(toDelete)
	}()
	return operations.DeleteFilesWithBackupDir(s.ctx, toDelete, s.backupDir)
}

// This deletes the empty directories in the slice passed in.  It
// ignores any errors deleting directories
func (s *syncCopyMove) deleteEmptyDirectories(ctx context.Context, f fs.Fs, entriesMap map[string]fs.DirEntry) error {
	if len(entriesMap) == 0 {
		return nil
	}
	if accounting.Stats(ctx).Errored() && !s.ci.IgnoreErrors {
		fs.Errorf(f, "%v", fs.ErrorNotDeletingDirs)
		return fs.ErrorNotDeletingDirs
	}

	var entries fs.DirEntries
	for _, entry := range entriesMap {
		entries = append(entries, entry)
	}
	// Now delete the empty directories starting from the longest path
	sort.Sort(entries)
	var errorCount int
	var okCount int
	for i := len(entries) - 1; i >= 0; i-- {
		entry := entries[i]
		dir, ok := entry.(fs.Directory)
		if ok {
			// TryRmdir only deletes empty directories
			err := operations.TryRmdir(ctx, f, dir.Remote())
			if err != nil {
				fs.Debugf(fs.LogDirName(f, dir.Remote()), "Failed to Rmdir: %v", err)
				errorCount++
			} else {
				okCount++
			}
		} else {
			fs.Errorf(f, "Not a directory: %v", entry)
		}
	}
	if errorCount > 0 {
		fs.Debugf(f, "failed to delete %d directories", errorCount)
	}
	if okCount > 0 {
		fs.Debugf(f, "deleted %d directories", okCount)
	}
	return nil
}

// mark the parent of entry as not empty and if entry is a directory mark it as potentially empty.
func (s *syncCopyMove) markParentNotEmpty(entry fs.DirEntry) {
	s.srcEmptyDirsMu.Lock()
	defer s.srcEmptyDirsMu.Unlock()
	// Mark entry as potentially empty if it is a directory
	_, isDir := entry.(fs.Directory)
	if isDir {
		s.srcEmptyDirs[entry.Remote()] = entry
		// if DoMove and --delete-empty-src-dirs flag is set then record the parent but
		// don't remove any as we are about to move files out of them them making the
		// directory empty.
		if s.DoMove && s.deleteEmptySrcDirs {
			s.srcMoveEmptyDirs[entry.Remote()] = entry
		}
	}
	parentDir := path.Dir(entry.Remote())
	if isDir && s.copyEmptySrcDirs {
		// Mark its parent as not empty
		if parentDir == "." {
			parentDir = ""
		}
		delete(s.srcEmptyDirs, parentDir)
	}
	if !isDir {
		// Mark ALL its parents as not empty
		for {
			if parentDir == "." {
				parentDir = ""
			}
			delete(s.srcEmptyDirs, parentDir)
			if parentDir == "" {
				break
			}
			parentDir = path.Dir(parentDir)
		}
	}
}

// parseTrackRenamesStrategy turns a config string into a trackRenamesStrategy
func parseTrackRenamesStrategy(strategies string) (strategy trackRenamesStrategy, err error) {
	if len(strategies) == 0 {
		return strategy, nil
	}
	for _, s := range strings.Split(strategies, ",") {
		switch s {
		case "hash":
			strategy |= trackRenamesStrategyHash
		case "modtime":
			strategy |= trackRenamesStrategyModtime
		case "leaf":
			strategy |= trackRenamesStrategyLeaf
		case "size":
			// ignore
		default:
			return strategy, fmt.Errorf("unknown track renames strategy %q", s)
		}
	}
	return strategy, nil
}

// renameID makes a string with the size and the other identifiers of the requested rename strategies
//
// it may return an empty string in which case no hash could be made
func (s *syncCopyMove) renameID(obj fs.Object, renamesStrategy trackRenamesStrategy, precision time.Duration) string {
	var builder strings.Builder

	fmt.Fprintf(&builder, "%d", obj.Size())

	if renamesStrategy.hash() {
		var err error
		hash, err := obj.Hash(s.ctx, s.commonHash)
		if err != nil {
			fs.Debugf(obj, "Hash failed: %v", err)
			return ""
		}
		if hash == "" {
			return ""
		}

		builder.WriteRune(',')
		builder.WriteString(hash)
	}

	// for renamesStrategy.modTime() we don't add to the hash but we check the times in
	// popRenameMap

	if renamesStrategy.leaf() {
		builder.WriteRune(',')
		builder.WriteString(path.Base(obj.Remote()))
	}

	return builder.String()
}

// pushRenameMap adds the object with hash to the rename map
func (s *syncCopyMove) pushRenameMap(hash string, obj fs.Object) {
	s.renameMapMu.Lock()
	s.renameMap[hash] = append(s.renameMap[hash], obj)
	s.renameMapMu.Unlock()
}

// popRenameMap finds the object with hash and pop the first match from
// renameMap or returns nil if not found.
func (s *syncCopyMove) popRenameMap(hash string, src fs.Object) (dst fs.Object) {
	s.renameMapMu.Lock()
	defer s.renameMapMu.Unlock()
	dsts, ok := s.renameMap[hash]
	if ok && len(dsts) > 0 {
		// Element to remove
		i := 0

		// If using track renames strategy modtime then we need to check the modtimes here
		if s.trackRenamesStrategy.modTime() {
			i = -1
			srcModTime := src.ModTime(s.ctx)
			for j, dst := range dsts {
				dstModTime := dst.ModTime(s.ctx)
				dt := dstModTime.Sub(srcModTime)
				if dt < s.modifyWindow && dt > -s.modifyWindow {
					i = j
					break
				}
			}
			// If nothing matched then return nil
			if i < 0 {
				return nil
			}
		}

		// Remove the entry and return it
		dst = dsts[i]
		dsts = append(dsts[:i], dsts[i+1:]...)
		if len(dsts) > 0 {
			s.renameMap[hash] = dsts
		} else {
			delete(s.renameMap, hash)
		}
	}
	return dst
}

// makeRenameMap builds a map of the destination files by hash that
// match sizes in the slice of objects in s.renameCheck
func (s *syncCopyMove) makeRenameMap() {
	fs.Infof(s.fdst, "Making map for --track-renames")

	// first make a map of possible sizes we need to check
	possibleSizes := map[int64]struct{}{}
	for _, obj := range s.renameCheck {
		possibleSizes[obj.Size()] = struct{}{}
	}

	// pump all the dstFiles into in
	in := make(chan fs.Object, s.ci.Checkers)
	go s.pumpMapToChan(s.dstFiles, in)

	// now make a map of size,hash for all dstFiles
	s.renameMap = make(map[string][]fs.Object)
	var wg sync.WaitGroup
	wg.Add(s.ci.Checkers)
	for i := 0; i < s.ci.Checkers; i++ {
		go func() {
			defer wg.Done()
			for obj := range in {
				// only create hash for dst fs.Object if its size could match
				if _, found := possibleSizes[obj.Size()]; found {
					tr := accounting.Stats(s.ctx).NewCheckingTransfer(obj, "renaming")
					hash := s.renameID(obj, s.trackRenamesStrategy, s.modifyWindow)

					if hash != "" {
						s.pushRenameMap(hash, obj)
					}

					tr.Done(s.ctx, nil)
				}
			}
		}()
	}
	wg.Wait()
	fs.Infof(s.fdst, "Finished making map for --track-renames")
}

// tryRename renames an src object when doing track renames if
// possible, it returns true if the object was renamed.
func (s *syncCopyMove) tryRename(src fs.Object) bool {
	// Calculate the hash of the src object
	hash := s.renameID(src, s.trackRenamesStrategy, fs.GetModifyWindow(s.ctx, s.fsrc, s.fdst))

	if hash == "" {
		return false
	}

	// Get a match on fdst
	dst := s.popRenameMap(hash, src)
	if dst == nil {
		return false
	}

	// Find dst object we are about to overwrite if it exists
	dstOverwritten, _ := s.fdst.NewObject(s.ctx, src.Remote())

	// Rename dst to have name src.Remote()
	_, err := operations.Move(s.ctx, s.fdst, dstOverwritten, src.Remote(), dst)
	if err != nil {
		fs.Debugf(src, "Failed to rename to %q: %v", dst.Remote(), err)
		return false
	}

	// remove file from dstFiles if present
	s.dstFilesMu.Lock()
	delete(s.dstFiles, dst.Remote())
	s.dstFilesMu.Unlock()

	fs.Infof(src, "Renamed from %q", dst.Remote())
	return true
}

// Syncs fsrc into fdst
//
// If Delete is true then it deletes any files in fdst that aren't in fsrc
//
// If DoMove is true then files will be moved instead of copied.
//
// dir is the start directory, "" for root
func (s *syncCopyMove) run() error {
	if operations.Same(s.fdst, s.fsrc) {
		fs.Errorf(s.fdst, "Nothing to do as source and destination are the same")
		return nil
	}

	// Start background checking and transferring pipeline
	s.startCheckers()
	s.startRenamers()
	if !s.checkFirst {
		s.startTransfers()
	}
	s.startDeleters()
	s.dstFiles = make(map[string]fs.Object)

	s.startTrackRenames()

	// set up a march over fdst and fsrc
	m := &march.March{
		Ctx:                    s.inCtx,
		Fdst:                   s.fdst,
		Fsrc:                   s.fsrc,
		Dir:                    s.dir,
		NoTraverse:             s.noTraverse,
		Callback:               s,
		DstIncludeAll:          s.fi.Opt.DeleteExcluded,
		NoCheckDest:            s.noCheckDest,
		NoUnicodeNormalization: s.noUnicodeNormalization,
	}
	s.processError(m.Run(s.ctx))

	s.stopTrackRenames()
	if s.trackRenames {
		// Build the map of the remaining dstFiles by hash
		s.makeRenameMap()
		// Attempt renames for all the files which don't have a matching dst
		for _, src := range s.renameCheck {
			ok := s.toBeRenamed.Put(s.inCtx, fs.ObjectPair{Src: src, Dst: nil})
			if !ok {
				break
			}
		}
	}

	// Stop background checking and transferring pipeline
	s.stopCheckers()
	if s.checkFirst {
		fs.Infof(s.fdst, "Checks finished, now starting transfers")
		s.startTransfers()
	}
	s.stopRenamers()
	s.stopTransfers()
	s.stopDeleters()

	// Delete files after
	if s.deleteMode == fs.DeleteModeAfter {
		if s.currentError() != nil && !s.ci.IgnoreErrors {
			fs.Errorf(s.fdst, "%v", fs.ErrorNotDeleting)
		} else {
			s.processError(s.deleteFiles(false))
		}
	}

	// Update modtimes for directories if necessary
	if s.setDirModTime && s.setDirModTimeAfter {
		s.processError(s.setDelayedDirModTimes(s.ctx))
	}

	// Prune empty directories
	if s.deleteMode != fs.DeleteModeOff {
		if s.currentError() != nil && !s.ci.IgnoreErrors {
			fs.Errorf(s.fdst, "%v", fs.ErrorNotDeletingDirs)
		} else {
			s.processError(s.deleteEmptyDirectories(s.ctx, s.fdst, s.dstEmptyDirs))
		}
	}

	// Delete empty fsrc subdirectories
	// if DoMove and --delete-empty-src-dirs flag is set
	if s.DoMove && s.deleteEmptySrcDirs {
		// delete potentially empty subdirectories that were part of the move
		s.processError(s.deleteEmptyDirectories(s.ctx, s.fsrc, s.srcMoveEmptyDirs))
	}

	// Read the error out of the contexts if there is one
	s.processError(s.ctx.Err())
	s.processError(s.inCtx.Err())

	// If the duration was exceeded then add a Fatal Error so we don't retry
	if !s.maxDurationEndTime.IsZero() && time.Since(s.maxDurationEndTime) > 0 {
		fs.Errorf(s.fdst, "%v", ErrorMaxDurationReachedFatal)
		s.processError(ErrorMaxDurationReachedFatal)
	}

	// Print nothing to transfer message if there were no transfers and no errors
	if s.deleteMode != fs.DeleteModeOnly && accounting.Stats(s.ctx).GetTransfers() == 0 && s.currentError() == nil {
		fs.Infof(nil, "There was nothing to transfer")
	}

	// cancel the contexts to free resources
	s.inCancel()
	s.cancel()
	return s.currentError()
}

// DstOnly have an object which is in the destination only
func (s *syncCopyMove) DstOnly(dst fs.DirEntry) (recurse bool) {
	if s.deleteMode == fs.DeleteModeOff {
		if s.usingLogger {
			switch x := dst.(type) {
			case fs.Object:
				s.logger(s.ctx, operations.MissingOnSrc, nil, x, nil)
			case fs.Directory:
				// it's a directory that we'd normally skip, because we're not deleting anything on the dest
				// however, to make sure every file is logged, we need to list it, so we need to return true here.
				// we skip this when not using logger.
				s.logger(s.ctx, operations.MissingOnSrc, nil, dst, fs.ErrorIsDir)
				return true
			}
		}
		return false
	}
	switch x := dst.(type) {
	case fs.Object:
		s.logger(s.ctx, operations.MissingOnSrc, nil, x, nil)
		switch s.deleteMode {
		case fs.DeleteModeAfter:
			// record object as needs deleting
			s.dstFilesMu.Lock()
			s.dstFiles[x.Remote()] = x
			s.dstFilesMu.Unlock()
		case fs.DeleteModeDuring, fs.DeleteModeOnly:
			select {
			case <-s.ctx.Done():
				return
			case s.deleteFilesCh <- x:
			}
		default:
			panic(fmt.Sprintf("unexpected delete mode %d", s.deleteMode))
		}
	case fs.Directory:
		// Do the same thing to the entire contents of the directory
		// Record directory as it is potentially empty and needs deleting
		if s.fdst.Features().CanHaveEmptyDirectories {
			s.dstEmptyDirsMu.Lock()
			s.dstEmptyDirs[dst.Remote()] = dst
			s.dstEmptyDirsMu.Unlock()
			s.logger(s.ctx, operations.MissingOnSrc, nil, dst, fs.ErrorIsDir)
		}
		return true
	default:
		panic("Bad object in DirEntries")

	}
	return false
}

// keeps track of dirs with changed contents, to avoid setting modtimes on dirs that haven't changed
func (s *syncCopyMove) markDirModified(dir string) {
	if !s.setDirModTimeAfter {
		return
	}
	s.setDirModTimeMu.Lock()
	defer s.setDirModTimeMu.Unlock()
	s.modifiedDirs[dir] = struct{}{}
}

// like markDirModified, but accepts an Object instead of a string.
// the marked dir will be this object's parent.
func (s *syncCopyMove) markDirModifiedObject(o fs.Object) {
	dir := path.Dir(o.Remote())
	if dir == "." {
		dir = ""
	}
	s.markDirModified(dir)
}

// copyDirMetadata copies the src directory modTime or Metadata to dst
// or f if nil. If dst is nil then it uses dir as the name of the new
// directory.
//
// It returns the destination directory if possible.  Note that this may
// be nil.
func (s *syncCopyMove) copyDirMetadata(ctx context.Context, f fs.Fs, dst fs.Directory, dir string, src fs.Directory) (newDst fs.Directory) {
	var err error
	equal := operations.DirsEqual(ctx, src, dst, operations.DirsEqualOpt{ModifyWindow: s.modifyWindow, SetDirModtime: s.setDirModTime, SetDirMetadata: s.setDirMetadata})
	if !s.setDirModTimeAfter && equal {
		return nil
	}
	if s.setDirModTimeAfter && equal {
		newDst = dst
	} else if s.copyEmptySrcDirs {
		if s.setDirMetadata {
			newDst, err = operations.CopyDirMetadata(ctx, f, dst, dir, src)
		} else if s.setDirModTime {
			if dst == nil {
				newDst, err = operations.MkdirModTime(ctx, f, dir, src.ModTime(ctx))
			} else {
				newDst, err = operations.SetDirModTime(ctx, f, dst, dir, src.ModTime(ctx))
			}
		} else if dst == nil {
			// Create the directory if it doesn't exist
			err = operations.Mkdir(ctx, f, dir)
		}
	} else {
		newDst = dst
	}
	// If we need to set modtime after and we created a dir, then save it for later
	if s.setDirModTime && s.setDirModTimeAfter && err == nil {
		if newDst != nil {
			dir = newDst.Remote()
		}
		level := strings.Count(dir, "/") + 1
		// The root directory "" is at the top level
		if dir == "" {
			level = 0
		}
		s.setDirModTimeMu.Lock()
		// Keep track of the maximum level inserted
		if level > s.setDirModTimesMaxLevel {
			s.setDirModTimesMaxLevel = level
		}
		set := setDirModTime{
			src:     src,
			dst:     newDst,
			dir:     dir,
			modTime: src.ModTime(ctx),
			level:   level,
		}
		s.setDirModTimes = append(s.setDirModTimes, set)
		s.setDirModTimeMu.Unlock()
		fs.Debugf(nil, "Added delayed dir = %q, newDst=%v", dir, newDst)
	}
	s.processError(err)
	if err != nil {
		return nil
	}
	return newDst
}

// Set the modtimes for directories
func (s *syncCopyMove) setDelayedDirModTimes(ctx context.Context) error {
	s.setDirModTimeMu.Lock()
	defer s.setDirModTimeMu.Unlock()

	// Timestamp all directories at the same level in parallel, deepest first
	// We do this by iterating the slice multiple times to save memory
	// There could be a lot of directories in this slice.
	errCount := errcount.New()
	for level := s.setDirModTimesMaxLevel; level >= 0; level-- {
		g, gCtx := errgroup.WithContext(ctx)
		g.SetLimit(s.ci.Checkers)
		for _, item := range s.setDirModTimes {
			if item.level != level {
				continue
			}
			// End early if error
			if gCtx.Err() != nil {
				break
			}
			if _, ok := s.modifiedDirs[item.dir]; !ok {
				continue
			}
			if !s.copyEmptySrcDirs {
				if _, isEmpty := s.srcEmptyDirs[item.dir]; isEmpty {
					continue
				}
			}
			item := item
			if s.setDirModTimeAfter { // mark dir's parent as modified
				dir := path.Dir(item.dir)
				if dir == "." {
					dir = ""
				}
				s.modifiedDirs[dir] = struct{}{} // lock is already held
			}
			g.Go(func() error {
				var err error
				if s.setDirMetadata {
					_, err = operations.CopyDirMetadata(gCtx, s.fdst, item.dst, item.dir, item.src)
				} else {
					_, err = operations.SetDirModTime(gCtx, s.fdst, item.dst, item.dir, item.modTime)
				}
				if err != nil {
					err = fs.CountError(err)
					fs.Errorf(item.dir, "Failed to update directory timestamp or metadata: %v", err)
					errCount.Add(err)
				}
				return nil // don't return errors, just count them
			})
		}
		err := g.Wait()
		if err != nil {
			return err
		}
	}
	return errCount.Err("failed to set directory modtime")
}

// SrcOnly have an object which is in the source only
func (s *syncCopyMove) SrcOnly(src fs.DirEntry) (recurse bool) {
	if s.deleteMode == fs.DeleteModeOnly {
		return false
	}
	switch x := src.(type) {
	case fs.Object:
		s.logger(s.ctx, operations.MissingOnDst, x, nil, nil)
		s.markParentNotEmpty(src)

		if s.trackRenames {
			// Save object to check for a rename later
			select {
			case <-s.ctx.Done():
				return
			case s.trackRenamesCh <- x:
			}
		} else {
			// Check CompareDest && CopyDest
			NoNeedTransfer, err := operations.CompareOrCopyDest(s.ctx, s.fdst, nil, x, s.compareCopyDest, s.backupDir)
			if err != nil {
				s.processError(err)
				s.logger(s.ctx, operations.TransferError, x, nil, err)
			}
			if !NoNeedTransfer {
				// No need to check since doesn't exist
				fs.Debugf(src, "Need to transfer - File not found at Destination")
				s.markDirModifiedObject(x)
				ok := s.toBeUploaded.Put(s.inCtx, fs.ObjectPair{Src: x, Dst: nil})
				if !ok {
					return
				}
			}
		}
	case fs.Directory:
		// Do the same thing to the entire contents of the directory
		s.markParentNotEmpty(src)
		s.logger(s.ctx, operations.MissingOnDst, src, nil, fs.ErrorIsDir)

		// Create the directory and make sure the Metadata/ModTime is correct
		s.copyDirMetadata(s.ctx, s.fdst, nil, x.Remote(), x)
		s.markDirModified(x.Remote())
		return true
	default:
		panic("Bad object in DirEntries")
	}
	return false
}

// Match is called when src and dst are present, so sync src to dst
func (s *syncCopyMove) Match(ctx context.Context, dst, src fs.DirEntry) (recurse bool) {
	switch srcX := src.(type) {
	case fs.Object:
		s.markParentNotEmpty(src)

		if s.deleteMode == fs.DeleteModeOnly {
			return false
		}
		dstX, ok := dst.(fs.Object)
		if ok {
			// No logger here because we'll handle it in equal()
			ok = s.toBeChecked.Put(s.inCtx, fs.ObjectPair{Src: srcX, Dst: dstX})
			if !ok {
				return false
			}
		} else {
			// FIXME src is file, dst is directory
			err := errors.New("can't overwrite directory with file")
			fs.Errorf(dst, "%v", err)
			s.processError(err)
			s.logger(ctx, operations.TransferError, srcX, dstX, err)
		}
	case fs.Directory:
		// Do the same thing to the entire contents of the directory
		s.markParentNotEmpty(src)
		dstX, ok := dst.(fs.Directory)
		if ok {
			s.logger(s.ctx, operations.Match, src, dst, fs.ErrorIsDir)
			// Create the directory and make sure the Metadata/ModTime is correct
			s.copyDirMetadata(s.ctx, s.fdst, dstX, "", srcX)

			if s.ci.FixCase && !s.ci.Immutable && src.Remote() != dst.Remote() {
				// Fix case for case insensitive filesystems
				// Fix each dir before recursing into subdirs and files
				err := operations.DirMoveCaseInsensitive(s.ctx, s.fdst, dst.Remote(), src.Remote())
				if err != nil {
					fs.Errorf(dst, "Error while attempting to rename to %s: %v", src.Remote(), err)
					s.processError(err)
				} else {
					fs.Infof(dst, "Fixed case by renaming to: %s", src.Remote())
				}
			}

			return true
		}
		// FIXME src is dir, dst is file
		err := errors.New("can't overwrite file with directory")
		fs.Errorf(dst, "%v", err)
		s.processError(err)
		s.logger(ctx, operations.TransferError, src.(fs.ObjectInfo), dst.(fs.ObjectInfo), err)
	default:
		panic("Bad object in DirEntries")
	}
	return false
}

// Syncs fsrc into fdst
//
// If Delete is true then it deletes any files in fdst that aren't in fsrc
//
// If DoMove is true then files will be moved instead of copied.
//
// dir is the start directory, "" for root
func runSyncCopyMove(ctx context.Context, fdst, fsrc fs.Fs, deleteMode fs.DeleteMode, DoMove bool, deleteEmptySrcDirs bool, copyEmptySrcDirs bool) error {
	ci := fs.GetConfig(ctx)
	if deleteMode != fs.DeleteModeOff && DoMove {
		return fserrors.FatalError(errors.New("can't delete and move at the same time"))
	}
	// Run an extra pass to delete only
	if deleteMode == fs.DeleteModeBefore {
		if ci.TrackRenames {
			return fserrors.FatalError(errors.New("can't use --delete-before with --track-renames"))
		}
		// only delete stuff during in this pass
		do, err := newSyncCopyMove(ctx, fdst, fsrc, fs.DeleteModeOnly, false, deleteEmptySrcDirs, copyEmptySrcDirs)
		if err != nil {
			return err
		}
		err = do.run()
		if err != nil {
			return err
		}
		// Next pass does a copy only
		deleteMode = fs.DeleteModeOff
	}
	do, err := newSyncCopyMove(ctx, fdst, fsrc, deleteMode, DoMove, deleteEmptySrcDirs, copyEmptySrcDirs)
	if err != nil {
		return err
	}
	return do.run()
}

// Sync fsrc into fdst
func Sync(ctx context.Context, fdst, fsrc fs.Fs, copyEmptySrcDirs bool) error {
	ci := fs.GetConfig(ctx)
	return runSyncCopyMove(ctx, fdst, fsrc, ci.DeleteMode, false, false, copyEmptySrcDirs)
}

// CopyDir copies fsrc into fdst
func CopyDir(ctx context.Context, fdst, fsrc fs.Fs, copyEmptySrcDirs bool) error {
	return runSyncCopyMove(ctx, fdst, fsrc, fs.DeleteModeOff, false, false, copyEmptySrcDirs)
}

// moveDir moves fsrc into fdst
func moveDir(ctx context.Context, fdst, fsrc fs.Fs, deleteEmptySrcDirs bool, copyEmptySrcDirs bool) error {
	return runSyncCopyMove(ctx, fdst, fsrc, fs.DeleteModeOff, true, deleteEmptySrcDirs, copyEmptySrcDirs)
}

// MoveDir moves fsrc into fdst
func MoveDir(ctx context.Context, fdst, fsrc fs.Fs, deleteEmptySrcDirs bool, copyEmptySrcDirs bool) error {
	fi := filter.GetConfig(ctx)
	if operations.Same(fdst, fsrc) {
		fs.Errorf(fdst, "Nothing to do as source and destination are the same")
		return nil
	}

	// First attempt to use DirMover if exists, same Fs and no filters are active
	if fdstDirMove := fdst.Features().DirMove; fdstDirMove != nil && operations.SameConfig(fsrc, fdst) && fi.InActive() {
		if operations.SkipDestructive(ctx, fdst, "server-side directory move") {
			return nil
		}
		fs.Debugf(fdst, "Using server-side directory move")
		err := fdstDirMove(ctx, fsrc, "", "")
		switch err {
		case fs.ErrorCantDirMove, fs.ErrorDirExists:
			fs.Infof(fdst, "Server side directory move failed - fallback to file moves: %v", err)
		case nil:
			fs.Infof(fdst, "Server side directory move succeeded")
			return nil
		default:
			err = fs.CountError(err)
			fs.Errorf(fdst, "Server side directory move failed: %v", err)
			return err
		}
	}

	// Otherwise move the files one by one
	return moveDir(ctx, fdst, fsrc, deleteEmptySrcDirs, copyEmptySrcDirs)
}
