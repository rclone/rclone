// Implementation of sync/copy/move

package fs

import (
	"sync"
	"time"
)

type syncCopyMove struct {
	// parameters
	fdst   Fs
	fsrc   Fs
	Delete bool
	DoMove bool
	dir    string
	// internal state
	noTraverse     bool              // if set don't trafevers the dst
	deleteBefore   bool              // set if we must delete objects before copying
	dstFiles       map[string]Object // dst files, only used if Delete
	srcFiles       map[string]Object // src files, only used if deleteBefore
	srcFilesChan   chan Object       // passes src objects
	srcFilesResult chan error        // error result of src listing
	dstFilesResult chan error        // error result of dst listing
	abort          chan struct{}     // signal to abort the copiers
	checkerWg      sync.WaitGroup    // wait for checkers
	toBeChecked    ObjectPairChan    // checkers channel
	copierWg       sync.WaitGroup    // wait for copiers
	toBeUploaded   ObjectPairChan    // copiers channel
	errorMu        sync.Mutex        // Mutex covering the errors variables
	err            error             // normal error from copy process
	noRetryErr     error             // error with NoRetry set
	fatalErr       error             // fatal error
}

func newSyncCopyMove(fdst, fsrc Fs, Delete bool, DoMove bool) *syncCopyMove {
	s := &syncCopyMove{
		fdst:           fdst,
		fsrc:           fsrc,
		Delete:         Delete,
		DoMove:         DoMove,
		dir:            "",
		srcFilesChan:   make(chan Object, Config.Checkers+Config.Transfers),
		srcFilesResult: make(chan error, 1),
		dstFilesResult: make(chan error, 1),
		noTraverse:     Config.NoTraverse,
		abort:          make(chan struct{}),
		toBeChecked:    make(ObjectPairChan, Config.Transfers),
		toBeUploaded:   make(ObjectPairChan, Config.Transfers),
		deleteBefore:   Delete && Config.DeleteBefore,
	}
	if s.noTraverse && s.Delete {
		Debug(s.fdst, "Ignoring --no-traverse with sync")
		s.noTraverse = false
	}
	return s

}

// Check to see if have set the abort flag
func (s *syncCopyMove) aborting() bool {
	select {
	case <-s.abort:
		return true
	default:
	}
	return false
}

// This reads the source files from s.srcFiles into srcFilesChan then
// closes it
//
// It returns the final result of the read into s.srcFilesResult
func (s *syncCopyMove) readSrcUsingMap() {
outer:
	for _, o := range s.srcFiles {
		if s.aborting() {
			break outer
		}
		select {
		case s.srcFilesChan <- o:
		case <-s.abort:
			break outer
		}
	}
	close(s.srcFilesChan)
	s.srcFilesResult <- nil
}

// This reads the source files into srcFilesChan then closes it
//
// It returns the final result of the read into s.srcFilesResult
func (s *syncCopyMove) readSrcUsingChan() {
	err := readFilesFn(s.fsrc, false, s.dir, func(o Object) error {
		if s.aborting() {
			return ErrorListAborted
		}
		select {
		case s.srcFilesChan <- o:
		case <-s.abort:
			return ErrorListAborted
		}
		return nil
	})
	close(s.srcFilesChan)
	s.srcFilesResult <- err
}

// This reads the destination files in into dstFiles
//
// It returns the final result of the read into s.dstFilesResult
func (s *syncCopyMove) readDstFiles() {
	var err error
	s.dstFiles, err = readFilesMap(s.fdst, Config.Filter.DeleteExcluded, s.dir)
	s.dstFilesResult <- err
}

// Check to see if src needs to be copied to dst and if so puts it in out
func (s *syncCopyMove) checkOne(pair ObjectPair, out ObjectPairChan) {
	src, dst := pair.src, pair.dst
	if dst == nil {
		Debug(src, "Couldn't find file - need to transfer")
		out <- pair
		return
	}
	// Check to see if can store this
	if !src.Storable() {
		return
	}
	// If we should ignore existing files, don't transfer
	if Config.IgnoreExisting {
		Debug(src, "Destination exists, skipping")
		return
	}
	// If we should upload unconditionally
	if Config.IgnoreTimes {
		Debug(src, "Uploading unconditionally as --ignore-times is in use")
		out <- pair
		return
	}
	// If UpdateOlder is in effect, skip if dst is newer than src
	if Config.UpdateOlder {
		srcModTime := src.ModTime()
		dstModTime := dst.ModTime()
		dt := dstModTime.Sub(srcModTime)
		// If have a mutually agreed precision then use that
		modifyWindow := Config.ModifyWindow
		if modifyWindow == ModTimeNotSupported {
			// Otherwise use 1 second as a safe default as
			// the resolution of the time a file was
			// uploaded.
			modifyWindow = time.Second
		}
		switch {
		case dt >= modifyWindow:
			Debug(src, "Destination is newer than source, skipping")
			return
		case dt <= -modifyWindow:
			Debug(src, "Destination is older than source, transferring")
		default:
			if src.Size() == dst.Size() {
				Debug(src, "Destination mod time is within %v of source and sizes identical, skipping", modifyWindow)
				return
			}
			Debug(src, "Destination mod time is within %v of source but sizes differ, transferring", modifyWindow)
		}
	} else {
		// Check to see if changed or not
		if Equal(src, dst) {
			Debug(src, "Unchanged skipping")
			return
		}
	}
	out <- pair
}

// This checks the types of errors returned while copying files
func (s *syncCopyMove) processError(err error) {
	if err == nil {
		return
	}
	s.errorMu.Lock()
	defer s.errorMu.Unlock()
	switch {
	case IsFatalError(err):
		close(s.abort)
		s.fatalErr = err
	case IsNoRetryError(err):
		s.noRetryErr = err
	default:
		s.err = err
	}
}

// pairChecker reads Objects~s on in send to out if they need transferring.
//
// FIXME potentially doing lots of hashes at once
func (s *syncCopyMove) pairChecker(in ObjectPairChan, out ObjectPairChan, wg *sync.WaitGroup) {
	defer wg.Done()
	for {
		if s.aborting() {
			return
		}
		select {
		case pair, ok := <-in:
			if !ok {
				return
			}
			src := pair.src
			Stats.Checking(src.Remote())
			s.checkOne(pair, out)
			Stats.DoneChecking(src.Remote())
		case <-s.abort:
			return
		}
	}
}

// pairCopier reads Objects on in and copies them.
func (s *syncCopyMove) pairCopier(in ObjectPairChan, fdst Fs, wg *sync.WaitGroup) {
	defer wg.Done()
	for {
		if s.aborting() {
			return
		}
		select {
		case pair, ok := <-in:
			if !ok {
				return
			}
			src := pair.src
			Stats.Transferring(src.Remote())
			if Config.DryRun {
				Log(src, "Not copying as --dry-run")
			} else {
				s.processError(Copy(fdst, pair.dst, src))
			}
			Stats.DoneTransferring(src.Remote())
		case <-s.abort:
			return
		}
	}
}

// pairMover reads Objects on in and moves them if possible, or copies
// them if not
func (s *syncCopyMove) pairMover(in ObjectPairChan, fdst Fs, wg *sync.WaitGroup) {
	defer wg.Done()
	// See if we have Move available
	fdstMover, haveMover := fdst.(Mover)
	for {
		if s.aborting() {
			return
		}
		select {
		case pair, ok := <-in:
			if !ok {
				return
			}
			src := pair.src
			dst := pair.dst
			Stats.Transferring(src.Remote())
			if Config.DryRun {
				Log(src, "Not moving as --dry-run")
			} else if haveMover && src.Fs().Name() == fdst.Name() {
				// Delete destination if it exists
				if dst != nil {
					s.processError(DeleteFile(src))
				}
				// Move dst <- src
				_, err := fdstMover.Move(src, src.Remote())
				if err != nil {
					Stats.Error()
					ErrorLog(dst, "Couldn't move: %v", err)
					s.processError(err)
				} else {
					Debug(src, "Moved")
				}
			} else {
				// Copy dst <- src
				err := Copy(fdst, dst, src)
				s.processError(err)
				if err != nil {
					ErrorLog(src, "Not deleting as copy failed: %v", err)
				} else {
					// Delete src if no error on copy
					s.processError(DeleteFile(src))
				}
			}
			Stats.DoneTransferring(src.Remote())
		case <-s.abort:
			return
		}
	}
}

// This starts the background checkers.
func (s *syncCopyMove) startCheckers() {
	s.checkerWg.Add(Config.Checkers)
	for i := 0; i < Config.Checkers; i++ {
		go s.pairChecker(s.toBeChecked, s.toBeUploaded, &s.checkerWg)
	}
}

// This stops the background checkers
func (s *syncCopyMove) stopCheckers() {
	close(s.toBeChecked)
	Log(s.fdst, "Waiting for checks to finish")
	s.checkerWg.Wait()
}

// This starts the background transfers
func (s *syncCopyMove) startTransfers() {
	s.copierWg.Add(Config.Transfers)
	for i := 0; i < Config.Transfers; i++ {
		if s.DoMove {
			go s.pairMover(s.toBeUploaded, s.fdst, &s.copierWg)
		} else {
			go s.pairCopier(s.toBeUploaded, s.fdst, &s.copierWg)
		}
	}
}

// This stops the background transfers
func (s *syncCopyMove) stopTransfers() {
	close(s.toBeUploaded)
	Log(s.fdst, "Waiting for transfers to finish")
	s.copierWg.Wait()
}

// This deletes the files in the dstFiles map.  If checkSrcMap is set
// then it checks to see if they exist first in srcFiles the source
// file map, otherwise it unconditionally deletes them.  If
// checkSrcMap is clear then it assumes that the any source files that
// have been found have been removed from dstFiles already.
func (s *syncCopyMove) deleteFiles(checkSrcMap bool) error {
	if Stats.Errored() {
		ErrorLog(s.fdst, "%v", ErrorNotDeleting)
		return ErrorNotDeleting
	}

	// Delete the spare files
	toDelete := make(ObjectsChan, Config.Transfers)
	go func() {
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
			toDelete <- o
		}
		close(toDelete)
	}()
	return DeleteFiles(toDelete)
}

// Syncs fsrc into fdst
//
// If Delete is true then it deletes any files in fdst that aren't in fsrc
//
// If DoMove is true then files will be moved instead of copied
//
// dir is the start directory, "" for root
func (s *syncCopyMove) run() error {
	if Same(s.fdst, s.fsrc) {
		ErrorLog(s.fdst, "Nothing to do as source and destination are the same")
		return nil
	}

	err := Mkdir(s.fdst)
	if err != nil {
		return err
	}

	// Start reading dstFiles if required
	if !s.noTraverse {
		go s.readDstFiles()
	}

	// If s.deleteBefore then we need to read the whole source map first
	if s.deleteBefore {
		// Read source files into the map
		s.srcFiles, err = readFilesMap(s.fsrc, false, s.dir)
		if err != nil {
			return err
		}
		// Pump the map into s.srcFilesChan
		go s.readSrcUsingMap()
	} else {
		go s.readSrcUsingChan()
	}

	// Wait for dstfiles to finish reading if we were reading them
	// and report any errors
	if !s.noTraverse {
		err = <-s.dstFilesResult
		if err != nil {
			return err
		}
	}

	// Delete files first if required
	// Have dstFiles and srcFiles complete at this point
	if s.deleteBefore {
		err = s.deleteFiles(true)
		if err != nil {
			return err
		}
	}

	// Start background checking and transferring pipeline
	s.startCheckers()
	s.startTransfers()

	// Do the transfers
	for src := range s.srcFilesChan {
		remote := src.Remote()
		var dst Object
		if s.noTraverse {
			var err error
			dst, err = s.fdst.NewObject(remote)
			if err != nil {
				dst = nil
				if err != ErrorObjectNotFound {
					Debug(src, "Error making NewObject: %v", err)
				}
			}
		} else {
			dst = s.dstFiles[remote]
			// Remove file from s.dstFiles because it exists in srcFiles
			delete(s.dstFiles, remote)
		}
		if dst != nil {
			s.toBeChecked <- ObjectPair{src, dst}
		} else {
			// No need to check since doesn't exist
			s.toBeUploaded <- ObjectPair{src, nil}
		}
	}

	// Stop background checking and transferring pipeline
	s.stopCheckers()
	s.stopTransfers()

	// Retrieve the delayed error from the source listing goroutine
	err = <-s.srcFilesResult

	// Delete files during or after
	if s.Delete && (Config.DeleteDuring || Config.DeleteAfter) {
		if err != nil {
			ErrorLog(s.fdst, "%v", ErrorNotDeleting)
		} else {
			err = s.deleteFiles(false)
		}
	}

	// Return errors in the precedence
	//   fatalErr
	//   error from above
	//   error from a copy
	//   noRetryErr
	s.processError(err)
	if s.fatalErr != nil {
		return s.fatalErr
	}
	if s.err != nil {
		return s.err
	}
	return s.noRetryErr
}

// Sync fsrc into fdst
func Sync(fdst, fsrc Fs) error {
	return newSyncCopyMove(fdst, fsrc, true, false).run()
}

// CopyDir copies fsrc into fdst
func CopyDir(fdst, fsrc Fs) error {
	return newSyncCopyMove(fdst, fsrc, false, false).run()
}

// moveDir moves fsrc into fdst
func moveDir(fdst, fsrc Fs) error {
	return newSyncCopyMove(fdst, fsrc, false, true).run()
}

// MoveDir moves fsrc into fdst
func MoveDir(fdst, fsrc Fs) error {
	if Same(fdst, fsrc) {
		ErrorLog(fdst, "Nothing to do as source and destination are the same")
		return nil
	}
	// The two remotes mustn't overlap
	if Overlapping(fdst, fsrc) {
		err := ErrorCantMoveOverlapping
		ErrorLog(fdst, "%v", err)
		return err
	}

	// First attempt to use DirMover if exists, same Fs and no filters are active
	if fdstDirMover, ok := fdst.(DirMover); ok && fsrc.Name() == fdst.Name() && Config.Filter.InActive() {
		if Config.DryRun {
			Log(fdst, "Not doing server side directory move as --dry-run")
			return nil
		}
		Debug(fdst, "Using server side directory move")
		err := fdstDirMover.DirMove(fsrc)
		switch err {
		case ErrorCantDirMove, ErrorDirExists:
			Debug(fdst, "Server side directory move failed - fallback to file moves: %v", err)
		case nil:
			Debug(fdst, "Server side directory move succeeded")
			return nil
		default:
			Stats.Error()
			ErrorLog(fdst, "Server side directory move failed: %v", err)
			return err
		}
	}

	// Otherwise move the files one by one
	return moveDir(fdst, fsrc)
}
