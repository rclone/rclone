// Implementation of sync/copy/move

package fs

import (
	"fmt"
	"sync"
	"time"

	"github.com/pkg/errors"
)

type syncCopyMove struct {
	// parameters
	fdst   Fs
	fsrc   Fs
	Delete bool
	DoMove bool
	dir    string
	// internal state
	noTraverse     bool                // if set don't trafevers the dst
	deleteBefore   bool                // set if we must delete objects before copying
	trackRenames   bool                // set if we should do server side renames
	dstFilesMu     sync.Mutex          // protect dstFiles
	dstFiles       map[string]Object   // dst files, always filled
	srcFiles       map[string]Object   // src files, only used if deleteBefore
	srcFilesChan   chan Object         // passes src objects
	srcFilesResult chan error          // error result of src listing
	dstFilesResult chan error          // error result of dst listing
	abort          chan struct{}       // signal to abort the copiers
	checkerWg      sync.WaitGroup      // wait for checkers
	toBeChecked    ObjectPairChan      // checkers channel
	transfersWg    sync.WaitGroup      // wait for transfers
	toBeUploaded   ObjectPairChan      // copiers channel
	errorMu        sync.Mutex          // Mutex covering the errors variables
	err            error               // normal error from copy process
	noRetryErr     error               // error with NoRetry set
	fatalErr       error               // fatal error
	commonHash     HashType            // common hash type between src and dst
	renameMapMu    sync.Mutex          // mutex to protect the below
	renameMap      map[string][]Object // dst files by hash - only used by trackRenames
	renamerWg      sync.WaitGroup      // wait for renamers
	toBeRenamed    ObjectPairChan      // renamers channel
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
		trackRenames:   Config.TrackRenames,
		commonHash:     fsrc.Hashes().Overlap(fdst.Hashes()).GetOne(),
		toBeRenamed:    make(ObjectPairChan, Config.Transfers),
	}
	if s.noTraverse && s.Delete {
		Debug(s.fdst, "Ignoring --no-traverse with sync")
		s.noTraverse = false
	}
	if s.trackRenames {
		// Don't track renames for remotes without server-side rename support.
		// Some remotes simulate rename by server-side copy and delete, so include
		// remotes that implements either Mover or Copier.
		switch fdst.(type) {
		case Mover, Copier:
		default:
			ErrorLog(fdst, "Ignoring --track-renames as the destination does not support server-side move or copy")
			s.trackRenames = false
		}
		if s.commonHash == HashNone {
			ErrorLog(fdst, "Ignoring --track-renames as the source and destination do not have a common hash")
			s.trackRenames = false
		}
	}
	if s.noTraverse && s.trackRenames {
		Debug(s.fdst, "Ignoring --no-traverse with --track-renames")
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

// This reads the map and pumps it into the channel passed in, closing
// the channel at the end
func (s *syncCopyMove) pumpMapToChan(files map[string]Object, out chan<- Object) {
outer:
	for _, o := range files {
		if s.aborting() {
			break outer
		}
		select {
		case out <- o:
		case <-s.abort:
			break outer
		}
	}
	close(out)
	s.srcFilesResult <- nil
}

// This reads the source files from s.srcFiles into srcFilesChan then
// closes it
//
// It returns the final result of the read into s.srcFilesResult
func (s *syncCopyMove) readSrcUsingMap() {
	s.pumpMapToChan(s.srcFiles, s.srcFilesChan)
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
	if err != nil {
		err = errors.Wrapf(err, "error listing source: %s", s.fsrc)
	}
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

// NeedTransfer checks to see if src needs to be copied to dst using
// the current config.
//
// Returns a flag which indicates whether the file needs to be
// transferred or not.
func NeedTransfer(dst, src Object) bool {
	if dst == nil {
		Debug(src, "Couldn't find file - need to transfer")
		return true
	}
	// If we should ignore existing files, don't transfer
	if Config.IgnoreExisting {
		Debug(src, "Destination exists, skipping")
		return false
	}
	// If we should upload unconditionally
	if Config.IgnoreTimes {
		Debug(src, "Transferring unconditionally as --ignore-times is in use")
		return true
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
			return false
		case dt <= -modifyWindow:
			Debug(src, "Destination is older than source, transferring")
		default:
			if src.Size() == dst.Size() {
				Debug(src, "Destination mod time is within %v of source and sizes identical, skipping", modifyWindow)
				return false
			}
			Debug(src, "Destination mod time is within %v of source but sizes differ, transferring", modifyWindow)
		}
	} else {
		// Check to see if changed or not
		if Equal(src, dst) {
			Debug(src, "Unchanged skipping")
			return false
		}
	}
	return true
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
		if !s.aborting() {
			close(s.abort)
		}
		s.fatalErr = err
	case IsNoRetryError(err):
		s.noRetryErr = err
	default:
		s.err = err
	}
}

// Returns the current error (if any) in the order of prececedence
//   fatalErr
//   normal error
//   noRetryErr
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
			// Check to see if can store this
			if src.Storable() {
				if NeedTransfer(pair.dst, pair.src) {
					out <- pair
				} else {
					// If moving need to delete the files we don't need to copy
					if s.DoMove {
						// Delete src if no error on copy
						s.processError(DeleteFile(src))
					}
				}
			}
			Stats.DoneChecking(src.Remote())
		case <-s.abort:
			return
		}
	}
}

// tryRename renames a src object when doing track renames if
// possible, it returns true if the object was renamed.
func (s *syncCopyMove) tryRename(src Object) bool {
	Stats.Checking(src.Remote())
	defer Stats.DoneChecking(src.Remote())

	hash, err := s.renameHash(src)
	if err != nil {
		Debug(src, "Failed to read hash: %v", err)
		return false
	}
	if hash == "" {
		return false
	}

	dst := s.popRenameMap(hash)
	if dst == nil {
		return false
	}

	err = MoveFile(s.fdst, s.fdst, src.Remote(), dst.Remote())
	if err != nil {
		Debug(src, "Failed to rename to %q: %v", dst.Remote(), err)
		return false
	}

	// remove file from dstFiles if present
	s.dstFilesMu.Lock()
	delete(s.dstFiles, dst.Remote())
	s.dstFilesMu.Unlock()

	Debug(src, "Renamed from %q", dst.Remote())
	return true
}

// pairRenamer reads Objects~s on in and attempts to rename them,
// otherwise it sends them out if they need transferring.
func (s *syncCopyMove) pairRenamer(in ObjectPairChan, out ObjectPairChan, wg *sync.WaitGroup) {
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
			if !s.tryRename(src) {
				// pass on if not renamed
				out <- pair
			}
		case <-s.abort:
			return
		}
	}
}

// pairCopyOrMove reads Objects on in and moves or copies them.
func (s *syncCopyMove) pairCopyOrMove(in ObjectPairChan, fdst Fs, wg *sync.WaitGroup) {
	defer wg.Done()
	var err error
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
			if s.DoMove {
				err = Move(fdst, pair.dst, src.Remote(), src)
			} else {
				err = Copy(fdst, pair.dst, src.Remote(), src)
			}
			s.processError(err)
			Stats.DoneTransferring(src.Remote(), err == nil)
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
	s.transfersWg.Add(Config.Transfers)
	for i := 0; i < Config.Transfers; i++ {
		go s.pairCopyOrMove(s.toBeUploaded, s.fdst, &s.transfersWg)
	}
}

// This stops the background transfers
func (s *syncCopyMove) stopTransfers() {
	close(s.toBeUploaded)
	Log(s.fdst, "Waiting for transfers to finish")
	s.transfersWg.Wait()
}

// This starts the background renamers.
func (s *syncCopyMove) startRenamers() {
	if !s.trackRenames {
		return
	}
	s.renamerWg.Add(Config.Checkers)
	for i := 0; i < Config.Checkers; i++ {
		go s.pairRenamer(s.toBeRenamed, s.toBeUploaded, &s.renamerWg)
	}
}

// This stops the background renamers
func (s *syncCopyMove) stopRenamers() {
	if !s.trackRenames {
		return
	}
	close(s.toBeRenamed)
	Log(s.fdst, "Waiting for renames to finish")
	s.renamerWg.Wait()
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

// renameHash makes a string with the size and the hash for rename detection
//
// it may return an empty string in which case no hash could be made
func (s *syncCopyMove) renameHash(obj Object) (hash string, err error) {
	hash, err = obj.Hash(s.commonHash)
	if err != nil {
		return hash, err
	}
	if hash == "" {
		return hash, nil
	}
	return fmt.Sprintf("%d,%s", obj.Size(), hash), nil
}

// makeRenameMap builds a map of the destination files by hash
func (s *syncCopyMove) makeRenameMap() error {
	Debug(s.fdst, "Making map for --track-renames")

	s.renameMap = make(map[string][]Object)
	in := make(chan Object, Config.Checkers)
	go s.pumpMapToChan(s.dstFiles, in)

	var wg sync.WaitGroup
	wg.Add(Config.Transfers)
	for i := 0; i < Config.Transfers; i++ {
		go func() {
			defer wg.Done()
			for {
				if s.aborting() {
					return
				}
				select {
				case obj, ok := <-in:
					if !ok {
						return
					}
					Stats.Checking(obj.Remote())
					hash, err := s.renameHash(obj)
					Stats.DoneChecking(obj.Remote())
					if err != nil {
						s.processError(err)
					} else if hash != "" {
						s.renameMapMu.Lock()
						s.renameMap[hash] = append(s.renameMap[hash], obj)
						s.renameMapMu.Unlock()
					}
				case <-s.abort:
					return
				}
			}
		}()
	}
	wg.Wait()
	Debug(s.fdst, "Finished making map for --track-renames")
	return s.currentError()
}

// popRenameMap finds the object with hash and pop the first match from
// renameMap or returns nil if not found.
func (s *syncCopyMove) popRenameMap(hash string) (dst Object) {
	s.renameMapMu.Lock()
	dsts, ok := s.renameMap[hash]
	if ok && len(dsts) > 0 {
		dst, dsts = dsts[0], dsts[1:]
		if len(dsts) > 0 {
			s.renameMap[hash] = dsts
		} else {
			delete(s.renameMap, hash)
		}
	}
	s.renameMapMu.Unlock()
	return dst
}

// delRenameMap removes obj from renameMap
func (s *syncCopyMove) delRenameMap(obj Object) {
	hash, err := s.renameHash(obj)
	if err != nil {
		return
	}
	if hash == "" {
		return
	}
	s.renameMapMu.Lock()
	dsts := s.renameMap[hash]
	for i, dst := range dsts {
		if obj.Remote() == dst.Remote() {
			// remove obj from list if found
			dsts = append(dsts[:i], dsts[i+1:]...)
			if len(dsts) > 0 {
				s.renameMap[hash] = dsts
			} else {
				delete(s.renameMap, hash)
			}
			break
		}
	}
	s.renameMapMu.Unlock()
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

	err := Mkdir(s.fdst, "")
	if err != nil {
		return err
	}

	// Start reading dstFiles if required
	if !s.noTraverse {
		go s.readDstFiles()
	}

	// If s.deleteBefore then we need to read the whole source map first
	readSourceMap := s.deleteBefore

	if readSourceMap {
		// Read source files into the map
		s.srcFiles, err = readFilesMap(s.fsrc, false, s.dir)
		if err != nil {
			return err
		}

	}

	// Wait for dstfiles to finish reading if we were reading them
	// and report any errors
	if !s.noTraverse {
		err = <-s.dstFilesResult
		if err != nil {
			return err
		}
	}

	// Build the map of destination files by hash if required
	// Have dstFiles complete at this point
	if s.trackRenames {
		if err = s.makeRenameMap(); err != nil {
			return err
		}
	}

	// Delete files first if required
	if s.deleteBefore {
		err = s.deleteFiles(true)
		if err != nil {
			return err
		}
	}

	// Now we can fill the src channel.
	if readSourceMap {
		// Pump the map into s.srcFilesChan
		go s.readSrcUsingMap()
	} else {
		go s.readSrcUsingChan()
	}

	// Start background checking and transferring pipeline
	s.startCheckers()
	s.startRenamers()
	s.startTransfers()

	// Do the transfers
	var renameCheck []Object
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
			s.dstFilesMu.Lock()
			var ok bool
			dst, ok = s.dstFiles[remote]
			if ok {
				// Remove file from s.dstFiles because it exists in srcFiles
				delete(s.dstFiles, remote)
			}
			s.dstFilesMu.Unlock()
			if ok && s.trackRenames {
				// remove file from rename tracking also
				s.delRenameMap(dst)
			}
		}
		if dst != nil {
			s.toBeChecked <- ObjectPair{src, dst}
		} else if s.trackRenames {
			// save object until all matches transferred
			renameCheck = append(renameCheck, src)
		} else {
			// No need to check since doesn't exist
			s.toBeUploaded <- ObjectPair{src, nil}
		}
	}

	if s.trackRenames {
		// Attempt renames for all the files which don't have a matching dst
		for _, src := range renameCheck {
			s.toBeRenamed <- ObjectPair{src, nil}
		}
	}

	// Stop background checking and transferring pipeline
	s.stopCheckers()
	s.stopRenamers()
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
	s.processError(err)
	return s.currentError()
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

	// The two remotes mustn't overlap if we didn't do server side move
	if Overlapping(fdst, fsrc) {
		err := ErrorCantMoveOverlapping
		ErrorLog(fdst, "%v", err)
		return err
	}

	// Otherwise move the files one by one
	return moveDir(fdst, fsrc)
}
