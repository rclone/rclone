// Generic operations on filesystems and objects

package fs

import (
	"fmt"
	"io"
	"log"
	"mime"
	"path"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/pkg/errors"

	"golang.org/x/text/unicode/norm"
)

// CalculateModifyWindow works out modify window for Fses passed in -
// sets Config.ModifyWindow
//
// This is the largest modify window of all the fses in use, and the
// user configured value
func CalculateModifyWindow(fs ...Fs) {
	for _, f := range fs {
		if f != nil {
			precision := f.Precision()
			if precision > Config.ModifyWindow {
				Config.ModifyWindow = precision
			}
			if precision == ModTimeNotSupported {
				Debug(f, "Modify window not supported")
				return
			}
		}
	}
	Debug(fs[0], "Modify window is %s", Config.ModifyWindow)
}

// HashEquals checks to see if src == dst, but ignores empty strings
// and returns true if either is empty.
func HashEquals(src, dst string) bool {
	if src == "" || dst == "" {
		return true
	}
	return src == dst
}

// CheckHashes checks the two files to see if they have common
// known hash types and compares them
//
// Returns
//
// equal - which is equality of the hashes
//
// hash - the HashType. This is HashNone if either of the hashes were
// unset or a compatible hash couldn't be found.
//
// err - may return an error which will already have been logged
//
// If an error is returned it will return equal as false
func CheckHashes(src, dst Object) (equal bool, hash HashType, err error) {
	common := src.Fs().Hashes().Overlap(dst.Fs().Hashes())
	// Debug(nil, "Shared hashes: %v", common)
	if common.Count() == 0 {
		return true, HashNone, nil
	}
	hash = common.GetOne()
	srcHash, err := src.Hash(hash)
	if err != nil {
		Stats.Error()
		ErrorLog(src, "Failed to calculate src hash: %v", err)
		return false, hash, err
	}
	if srcHash == "" {
		return true, HashNone, nil
	}
	dstHash, err := dst.Hash(hash)
	if err != nil {
		Stats.Error()
		ErrorLog(dst, "Failed to calculate dst hash: %v", err)
		return false, hash, err
	}
	if dstHash == "" {
		return true, HashNone, nil
	}
	return srcHash == dstHash, hash, nil
}

// Equal checks to see if the src and dst objects are equal by looking at
// size, mtime and hash
//
// If the src and dst size are different then it is considered to be
// not equal.  If --size-only is in effect then this is the only check
// that is done.  If --ignore-size is in effect then this check is
// skipped and the files are considered the same size.
//
// If the size is the same and the mtime is the same then it is
// considered to be equal.  This check is skipped if using --checksum.
//
// If the size is the same and mtime is different, unreadable or
// --checksum is set and the hash is the same then the file is
// considered to be equal.  In this case the mtime on the dst is
// updated if --checksum is not set.
//
// Otherwise the file is considered to be not equal including if there
// were errors reading info.
func Equal(src, dst Object) bool {
	if !Config.IgnoreSize {
		if src.Size() != dst.Size() {
			Debug(src, "Sizes differ")
			return false
		}
	}
	if Config.SizeOnly {
		Debug(src, "Sizes identical")
		return true
	}

	var srcModTime time.Time
	if !Config.CheckSum {
		if Config.ModifyWindow == ModTimeNotSupported {
			Debug(src, "Sizes identical")
			return true
		}
		// Size the same so check the mtime
		srcModTime = src.ModTime()
		dstModTime := dst.ModTime()
		dt := dstModTime.Sub(srcModTime)
		ModifyWindow := Config.ModifyWindow
		if dt >= ModifyWindow || dt <= -ModifyWindow {
			Debug(src, "Modification times differ by %s: %v, %v", dt, srcModTime, dstModTime)
		} else {
			Debug(src, "Size and modification time the same (differ by %s, within tolerance %s)", dt, ModifyWindow)
			return true
		}
	}

	// mtime is unreadable or different but size is the same so
	// check the hash
	same, hash, _ := CheckHashes(src, dst)
	if !same {
		Debug(src, "Hash differ")
		return false
	}

	if !Config.CheckSum {
		// Size and hash the same but mtime different so update the
		// mtime of the dst object here
		err := dst.SetModTime(srcModTime)
		if err == ErrorCantSetModTime {
			Debug(src, "src and dst identical but can't set mod time without re-uploading")
			return false
		} else if err != nil {
			Stats.Error()
			ErrorLog(dst, "Failed to read set modification time: %v", err)
		}
	}

	if hash == HashNone {
		Debug(src, "Size of src and dst objects identical")
	} else {
		Debug(src, "Size and %v of src and dst objects identical", hash)
	}
	return true
}

// MimeType returns a guess at the mime type from the extension
func MimeType(o ObjectInfo) string {
	mimeType := mime.TypeByExtension(path.Ext(o.Remote()))
	if !strings.ContainsRune(mimeType, '/') {
		mimeType = "application/octet-stream"
	}
	return mimeType
}

// Used to remove a failed copy
//
// Returns whether the file was succesfully removed or not
func removeFailedCopy(dst Object) bool {
	if dst == nil {
		return false
	}
	Debug(dst, "Removing failed copy")
	removeErr := dst.Remove()
	if removeErr != nil {
		Debug(dst, "Failed to remove failed copy: %s", removeErr)
		return false
	}
	return true
}

// Copy src object to dst or f if nil
//
// If dst is nil then the object must not exist already.  If you do
// call Copy() with dst nil on a pre-existing file then some filing
// systems (eg Drive) may duplicate the file.
func Copy(f Fs, dst, src Object) {
	maxTries := Config.LowLevelRetries
	tries := 0
	doUpdate := dst != nil
	var err error
	var actionTaken string
	for {
		// Try server side copy first - if has optional interface and
		// is same underlying remote
		actionTaken = "Copied (server side copy)"
		if fCopy, ok := f.(Copier); ok && src.Fs().Name() == f.Name() {
			var newDst Object
			newDst, err = fCopy.Copy(src, src.Remote())
			if err == nil {
				dst = newDst
			}
		} else {
			err = ErrorCantCopy
		}
		// If can't server side copy, do it manually
		if err == ErrorCantCopy {
			var in0 io.ReadCloser
			in0, err = src.Open()
			if err != nil {
				err = errors.Wrap(err, "failed to open source object")
			} else {
				// On big files add a buffer
				if src.Size() > 10<<20 {
					in0, _ = newAsyncReader(in0, 4, 4<<20)
				}

				in := NewAccount(in0, src) // account the transfer

				if doUpdate {
					actionTaken = "Copied (updated existing)"
					err = dst.Update(in, src)
				} else {
					actionTaken = "Copied (new)"
					dst, err = f.Put(in, src)
				}
				closeErr := in.Close()
				if err == nil {
					err = closeErr
				}
			}
		}
		tries++
		if tries >= maxTries {
			break
		}
		// Retry if err returned a retry error
		if IsRetryError(err) || ShouldRetry(err) {
			Debug(src, "Received error: %v - low level retry %d/%d", err, tries, maxTries)
			if removeFailedCopy(dst) {
				// If we removed dst, then nil it out and note we are not updating
				dst = nil
				doUpdate = false
			}
			continue
		}
		// otherwise finish
		break
	}
	if err != nil {
		Stats.Error()
		ErrorLog(src, "Failed to copy: %v", err)
		removeFailedCopy(dst)
		return
	}

	// Verify sizes are the same after transfer
	if !Config.IgnoreSize && src.Size() != dst.Size() {
		Stats.Error()
		err = errors.Errorf("corrupted on transfer: sizes differ %d vs %d", src.Size(), dst.Size())
		ErrorLog(dst, "%v", err)
		removeFailedCopy(dst)
		return
	}

	// Verify hashes are the same after transfer - ignoring blank hashes
	// TODO(klauspost): This could be extended, so we always create a hash type matching
	// the destination, and calculate it while sending.
	common := src.Fs().Hashes().Overlap(dst.Fs().Hashes())
	// Debug(src, "common hashes: %v", common)
	if !Config.SizeOnly && common.Count() > 0 {
		// Get common hash type
		hashType := common.GetOne()

		srcSum, err := src.Hash(hashType)
		if err != nil {
			Stats.Error()
			ErrorLog(src, "Failed to read src hash: %v", err)
		} else if srcSum != "" {
			dstSum, err := dst.Hash(hashType)
			if err != nil {
				Stats.Error()
				ErrorLog(dst, "Failed to read hash: %v", err)
			} else if !HashEquals(srcSum, dstSum) {
				Stats.Error()
				err = errors.Errorf("corrupted on transfer: %v hash differ %q vs %q", hashType, srcSum, dstSum)
				ErrorLog(dst, "%v", err)
				removeFailedCopy(dst)
				return
			}
		}
	}

	Debug(src, actionTaken)
}

// Check to see if src needs to be copied to dst and if so puts it in out
func checkOne(pair ObjectPair, out ObjectPairChan) {
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

// PairChecker reads Objects~s on in send to out if they need transferring.
//
// FIXME potentially doing lots of hashes at once
func PairChecker(in ObjectPairChan, out ObjectPairChan, wg *sync.WaitGroup) {
	defer wg.Done()
	for pair := range in {
		src := pair.src
		Stats.Checking(src.Remote())
		checkOne(pair, out)
		Stats.DoneChecking(src.Remote())
	}
}

// PairCopier reads Objects on in and copies them.
func PairCopier(in ObjectPairChan, fdst Fs, wg *sync.WaitGroup) {
	defer wg.Done()
	for pair := range in {
		src := pair.src
		Stats.Transferring(src.Remote())
		if Config.DryRun {
			Log(src, "Not copying as --dry-run")
		} else {
			Copy(fdst, pair.dst, src)
		}
		Stats.DoneTransferring(src.Remote())
	}
}

// PairMover reads Objects on in and moves them if possible, or copies
// them if not
func PairMover(in ObjectPairChan, fdst Fs, wg *sync.WaitGroup) {
	defer wg.Done()
	// See if we have Move available
	fdstMover, haveMover := fdst.(Mover)
	for pair := range in {
		src := pair.src
		dst := pair.dst
		Stats.Transferring(src.Remote())
		if Config.DryRun {
			Log(src, "Not moving as --dry-run")
		} else if haveMover && src.Fs().Name() == fdst.Name() {
			// Delete destination if it exists
			if pair.dst != nil {
				err := dst.Remove()
				if err != nil {
					Stats.Error()
					ErrorLog(dst, "Couldn't delete: %v", err)
				}
			}
			_, err := fdstMover.Move(src, src.Remote())
			if err != nil {
				Stats.Error()
				ErrorLog(dst, "Couldn't move: %v", err)
			} else {
				Debug(src, "Moved")
			}
		} else {
			Copy(fdst, pair.dst, src)
		}
		Stats.DoneTransferring(src.Remote())
	}
}

// DeleteFile deletes a single file respecting --dry-run and accumulating stats and errors.
func DeleteFile(dst Object) (err error) {
	if Config.DryRun {
		Log(dst, "Not deleting as --dry-run")
	} else {
		Stats.Checking(dst.Remote())
		err = dst.Remove()
		Stats.DoneChecking(dst.Remote())
		if err != nil {
			Stats.Error()
			ErrorLog(dst, "Couldn't delete: %v", err)
		} else {
			Debug(dst, "Deleted")
		}
	}
	return err
}

// DeleteFiles removes all the files passed in the channel
func DeleteFiles(toBeDeleted ObjectsChan) error {
	var wg sync.WaitGroup
	wg.Add(Config.Transfers)
	var errorCount int32
	for i := 0; i < Config.Transfers; i++ {
		go func() {
			defer wg.Done()
			for dst := range toBeDeleted {
				err := DeleteFile(dst)
				if err != nil {
					atomic.AddInt32(&errorCount, 1)
				}
			}
		}()
	}
	Log(nil, "Waiting for deletions to finish")
	wg.Wait()
	if errorCount > 0 {
		return errors.Errorf("failed to delete %d files", errorCount)
	}
	return nil
}

// Read a Objects into add() for the given Fs.
// dir is the start directory, "" for root
// If includeAll is specified all files will be added,
// otherwise only files passing the filter will be added.
func readFilesFn(fs Fs, includeAll bool, dir string, add func(Object)) (err error) {
	list := NewLister()
	if !includeAll {
		list.SetFilter(Config.Filter)
		list.SetLevel(Config.MaxDepth)
	}
	list.Start(fs, dir)
	for {
		o, err := list.GetObject()
		if err != nil {
			return err
		}
		// Check if we are finished
		if o == nil {
			break
		}
		// Make sure we don't delete excluded files if not required
		if includeAll || Config.Filter.IncludeObject(o) {
			add(o)
		} else {
			Debug(o, "Excluded from sync (and deletion)")
		}
	}
	return nil
}

// Read a map of Object.Remote to Object for the given Fs.
// dir is the start directory, "" for root
// If includeAll is specified all files will be added,
// otherwise only files passing the filter will be added.
//
// This also detects duplicates and normalised duplicates
func readFilesMap(fs Fs, includeAll bool, dir string) (files map[string]Object, err error) {
	files = make(map[string]Object)
	normalised := make(map[string]struct{})
	err = readFilesFn(fs, includeAll, dir, func(o Object) {
		remote := o.Remote()
		normalisedRemote := strings.ToLower(norm.NFC.String(remote))
		if _, ok := files[remote]; !ok {
			files[remote] = o
			if _, ok := normalised[normalisedRemote]; ok {
				Log(o, "Warning: File found with same name but different case on %v", o.Fs())
			}
		} else {
			Log(o, "Duplicate file detected")
		}
		normalised[normalisedRemote] = struct{}{}
	})
	return files, err
}

// readFilesMaps runs readFilesMap on fdst and fsrc at the same time
// dir is the start directory, "" for root
func readFilesMaps(fdst Fs, fdstIncludeAll bool, fsrc Fs, fsrcIncludeAll bool, dir string) (dstFiles, srcFiles map[string]Object, err error) {
	var wg sync.WaitGroup
	var srcErr, dstErr error

	list := func(fs Fs, includeAll bool, pMap *map[string]Object, pErr *error) {
		defer wg.Done()
		Log(fs, "Building file list")
		files, listErr := readFilesMap(fs, includeAll, dir)
		if listErr != nil {
			ErrorLog(fs, "Error building file list: %v", listErr)
			*pErr = listErr
		} else {
			Debug(fs, "Done building file list")
			*pMap = files
		}
	}

	wg.Add(2)
	go list(fdst, fdstIncludeAll, &dstFiles, &srcErr)
	go list(fsrc, fsrcIncludeAll, &srcFiles, &dstErr)
	wg.Wait()

	if srcErr != nil {
		err = srcErr
	}
	if dstErr != nil {
		err = dstErr
	}
	return dstFiles, srcFiles, err
}

// Same returns true if fdst and fsrc point to the same underlying Fs
func Same(fdst, fsrc Fs) bool {
	return fdst.Name() == fsrc.Name() && fdst.Root() == fsrc.Root()
}

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
	checkerWg      sync.WaitGroup    // wait for checkers
	toBeChecked    ObjectPairChan    // checkers channel
	copierWg       sync.WaitGroup    // wait for copiers
	toBeUploaded   ObjectPairChan    // copiers channel
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

// This reads the source files from s.srcFiles into srcFilesChan then
// closes it
//
// It returns the final result of the read into s.srcFilesResult
func (s *syncCopyMove) readSrcUsingMap() {
	for _, o := range s.srcFiles {
		s.srcFilesChan <- o
	}
	close(s.srcFilesChan)
	s.srcFilesResult <- nil
}

// This reads the source files into srcFilesChan then closes it
//
// It returns the final result of the read into s.srcFilesResult
func (s *syncCopyMove) readSrcUsingChan() {
	err := readFilesFn(s.fsrc, false, s.dir, func(o Object) {
		s.srcFilesChan <- o
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
				if !exists {
					toDelete <- o
				}
			} else {
				toDelete <- o
			}
		}
		close(toDelete)
	}()
	return DeleteFiles(toDelete)
}

// This starts the background checkers.
func (s *syncCopyMove) startCheckers() {
	s.checkerWg.Add(Config.Checkers)
	for i := 0; i < Config.Checkers; i++ {
		go PairChecker(s.toBeChecked, s.toBeUploaded, &s.checkerWg)
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
			go PairMover(s.toBeUploaded, s.fdst, &s.copierWg)
		} else {
			go PairCopier(s.toBeUploaded, s.fdst, &s.copierWg)
		}
	}
}

// This stops the background transfers
func (s *syncCopyMove) stopTransfers() {
	close(s.toBeUploaded)
	Log(s.fdst, "Waiting for transfers to finish")
	s.copierWg.Wait()
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
	if err != nil {
		return err
	}

	// Delete files during or after
	if s.Delete && (Config.DeleteDuring || Config.DeleteAfter) {
		err = s.deleteFiles(false)
		if err != nil {
			return err
		}
	}

	return nil
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
		err := fdstDirMover.DirMove(fsrc)
		Debug(fdst, "Using server side directory move")
		switch err {
		case ErrorCantDirMove, ErrorDirExists:
			Debug(fdst, "Server side directory move failed - fallback to copy/delete: %v", err)
		case nil:
			Debug(fdst, "Server side directory move succeeded")
			return nil
		default:
			Stats.Error()
			ErrorLog(fdst, "Server side directory move failed: %v", err)
			return err
		}
	}

	// Now move the files
	err := moveDir(fdst, fsrc)
	if err != nil || Stats.Errored() {
		ErrorLog(fdst, "Not deleting files as there were IO errors")
		return err
	}
	// If no filters then purge
	if Config.Filter.InActive() {
		return Purge(fsrc)
	}
	// Otherwise remove any remaining files obeying filters
	err = Delete(fsrc)
	if err != nil {
		return err
	}
	// and try to remove the directory if empty - ignoring error
	_ = TryRmdir(fsrc)
	return nil
}

// checkIdentical checks to see if dst and src are identical
//
// it returns true if differences were found
func checkIdentical(dst, src Object) bool {
	Stats.Checking(src.Remote())
	defer Stats.DoneChecking(src.Remote())
	if src.Size() != dst.Size() {
		Stats.Error()
		ErrorLog(src, "Sizes differ")
		return true
	}
	if !Config.SizeOnly {
		same, _, err := CheckHashes(src, dst)
		if err != nil {
			// CheckHashes will log and count errors
			return true
		}
		if !same {
			Stats.Error()
			ErrorLog(src, "Md5sums differ")
			return true
		}
	}
	Debug(src, "OK")
	return false
}

// Check the files in fsrc and fdst according to Size and hash
func Check(fdst, fsrc Fs) error {
	dstFiles, srcFiles, err := readFilesMaps(fdst, false, fsrc, false, "")
	if err != nil {
		return err
	}
	differences := int32(0)

	// FIXME could do this as it goes along and make it use less
	// memory.

	// Move all the common files into commonFiles and delete then
	// from srcFiles and dstFiles
	commonFiles := make(map[string][]Object)
	for remote, src := range srcFiles {
		if dst, ok := dstFiles[remote]; ok {
			commonFiles[remote] = []Object{dst, src}
			delete(srcFiles, remote)
			delete(dstFiles, remote)
		}
	}

	Log(fdst, "%d files not in %v", len(dstFiles), fsrc)
	for _, dst := range dstFiles {
		Stats.Error()
		ErrorLog(dst, "File not in %v", fsrc)
		atomic.AddInt32(&differences, 1)
	}

	Log(fsrc, "%d files not in %s", len(srcFiles), fdst)
	for _, src := range srcFiles {
		Stats.Error()
		ErrorLog(src, "File not in %v", fdst)
		atomic.AddInt32(&differences, 1)
	}

	checks := make(chan []Object, Config.Transfers)
	go func() {
		for _, check := range commonFiles {
			checks <- check
		}
		close(checks)
	}()

	var checkerWg sync.WaitGroup
	checkerWg.Add(Config.Checkers)
	for i := 0; i < Config.Checkers; i++ {
		go func() {
			defer checkerWg.Done()
			for check := range checks {
				if checkIdentical(check[0], check[1]) {
					atomic.AddInt32(&differences, 1)
				}
			}
		}()
	}

	Log(fdst, "Waiting for checks to finish")
	checkerWg.Wait()
	Log(fdst, "%d differences found", Stats.GetErrors())
	if differences > 0 {
		return errors.Errorf("%d differences found", differences)
	}
	return nil
}

// ListFn lists the Fs to the supplied function
//
// Lists in parallel which may get them out of order
func ListFn(f Fs, fn func(Object)) error {
	list := NewLister().SetFilter(Config.Filter).SetLevel(Config.MaxDepth).Start(f, "")
	var wg sync.WaitGroup
	wg.Add(Config.Checkers)
	for i := 0; i < Config.Checkers; i++ {
		go func() {
			defer wg.Done()
			for {
				o, err := list.GetObject()
				if err != nil {
					log.Fatal(err)
				}
				// check if we are finished
				if o == nil {
					return
				}
				if Config.Filter.IncludeObject(o) {
					fn(o)
				}
			}
		}()
	}
	wg.Wait()
	return nil
}

// mutex for synchronized output
var outMutex sync.Mutex

// Synchronized fmt.Fprintf
//
// Ignores errors from Fprintf
func syncFprintf(w io.Writer, format string, a ...interface{}) {
	outMutex.Lock()
	defer outMutex.Unlock()
	_, _ = fmt.Fprintf(w, format, a...)
}

// List the Fs to the supplied writer
//
// Shows size and path - obeys includes and excludes
//
// Lists in parallel which may get them out of order
func List(f Fs, w io.Writer) error {
	return ListFn(f, func(o Object) {
		syncFprintf(w, "%9d %s\n", o.Size(), o.Remote())
	})
}

// ListLong lists the Fs to the supplied writer
//
// Shows size, mod time and path - obeys includes and excludes
//
// Lists in parallel which may get them out of order
func ListLong(f Fs, w io.Writer) error {
	return ListFn(f, func(o Object) {
		Stats.Checking(o.Remote())
		modTime := o.ModTime()
		Stats.DoneChecking(o.Remote())
		syncFprintf(w, "%9d %s %s\n", o.Size(), modTime.Local().Format("2006-01-02 15:04:05.000000000"), o.Remote())
	})
}

// Md5sum list the Fs to the supplied writer
//
// Produces the same output as the md5sum command - obeys includes and
// excludes
//
// Lists in parallel which may get them out of order
func Md5sum(f Fs, w io.Writer) error {
	return hashLister(HashMD5, f, w)
}

// Sha1sum list the Fs to the supplied writer
//
// Obeys includes and excludes
//
// Lists in parallel which may get them out of order
func Sha1sum(f Fs, w io.Writer) error {
	return hashLister(HashSHA1, f, w)
}

func hashLister(ht HashType, f Fs, w io.Writer) error {
	return ListFn(f, func(o Object) {
		Stats.Checking(o.Remote())
		sum, err := o.Hash(ht)
		Stats.DoneChecking(o.Remote())
		if err == ErrHashUnsupported {
			sum = "UNSUPPORTED"
		} else if err != nil {
			Debug(o, "Failed to read %v: %v", ht, err)
			sum = "ERROR"
		}
		syncFprintf(w, "%*s  %s\n", HashWidth[ht], sum, o.Remote())
	})
}

// Count counts the objects and their sizes in the Fs
//
// Obeys includes and excludes
func Count(f Fs) (objects int64, size int64, err error) {
	err = ListFn(f, func(o Object) {
		atomic.AddInt64(&objects, 1)
		atomic.AddInt64(&size, o.Size())
	})
	return
}

// ListDir lists the directories/buckets/containers in the Fs to the supplied writer
func ListDir(f Fs, w io.Writer) error {
	level := 1
	if Config.MaxDepth > 0 {
		level = Config.MaxDepth
	}
	list := NewLister().SetLevel(level).Start(f, "")
	for {
		dir, err := list.GetDir()
		if err != nil {
			log.Fatal(err)
		}
		if dir == nil {
			break
		}
		syncFprintf(w, "%12d %13s %9d %s\n", dir.Bytes, dir.When.Format("2006-01-02 15:04:05"), dir.Count, dir.Name)
	}
	return nil
}

// Mkdir makes a destination directory or container
func Mkdir(f Fs) error {
	if Config.DryRun {
		Log(f, "Not making directory as dry run is set")
		return nil
	}
	err := f.Mkdir()
	if err != nil {
		Stats.Error()
		return err
	}
	return nil
}

// TryRmdir removes a container but not if not empty.  It doesn't
// count errors but may return one.
func TryRmdir(f Fs) error {
	if Config.DryRun {
		Log(f, "Not deleting as dry run is set")
		return nil
	}
	return f.Rmdir()
}

// Rmdir removes a container but not if not empty
func Rmdir(f Fs) error {
	err := TryRmdir(f)
	if err != nil {
		Stats.Error()
		return err
	}
	return err
}

// Purge removes a container and all of its contents
//
// FIXME doesn't delete local directories
func Purge(f Fs) error {
	doFallbackPurge := true
	var err error
	if purger, ok := f.(Purger); ok {
		doFallbackPurge = false
		if Config.DryRun {
			Log(f, "Not purging as --dry-run set")
		} else {
			err = purger.Purge()
			if err == ErrorCantPurge {
				doFallbackPurge = true
			}
		}
	}
	if doFallbackPurge {
		// DeleteFiles and Rmdir observe --dry-run
		list := NewLister().Start(f, "")
		err = DeleteFiles(listToChan(list))
		if err != nil {
			return err
		}
		err = Rmdir(f)
	}
	if err != nil {
		Stats.Error()
		return err
	}
	return nil
}

// Delete removes all the contents of a container.  Unlike Purge, it
// obeys includes and excludes.
func Delete(f Fs) error {
	delete := make(ObjectsChan, Config.Transfers)
	delErr := make(chan error, 1)
	go func() {
		delErr <- DeleteFiles(delete)
	}()
	err := ListFn(f, func(o Object) {
		delete <- o
	})
	close(delete)
	delError := <-delErr
	if err == nil {
		err = delError
	}
	return err
}

// dedupeRename renames the objs slice to different names
func dedupeRename(remote string, objs []Object) {
	f := objs[0].Fs()
	mover, ok := f.(Mover)
	if !ok {
		log.Fatalf("Fs %v doesn't support Move", f)
	}
	ext := path.Ext(remote)
	base := remote[:len(remote)-len(ext)]
	for i, o := range objs {
		newName := fmt.Sprintf("%s-%d%s", base, i+1, ext)
		if !Config.DryRun {
			newObj, err := mover.Move(o, newName)
			if err != nil {
				Stats.Error()
				ErrorLog(o, "Failed to rename: %v", err)
				continue
			}
			Log(newObj, "renamed from: %v", o)
		} else {
			Log(remote, "Not renaming to %q as --dry-run", newName)
		}
	}
}

// dedupeDeleteAllButOne deletes all but the one in keep
func dedupeDeleteAllButOne(keep int, remote string, objs []Object) {
	for i, o := range objs {
		if i == keep {
			continue
		}
		_ = DeleteFile(o)
	}
	Log(remote, "Deleted %d extra copies", len(objs)-1)
}

// dedupeDeleteIdentical deletes all but one of identical (by hash) copies
func dedupeDeleteIdentical(remote string, objs []Object) []Object {
	// See how many of these duplicates are identical
	byHash := make(map[string][]Object, len(objs))
	for _, o := range objs {
		md5sum, err := o.Hash(HashMD5)
		if err == nil {
			byHash[md5sum] = append(byHash[md5sum], o)
		}
	}

	// Delete identical duplicates, refilling obj with the ones remaining
	objs = nil
	for md5sum, hashObjs := range byHash {
		if len(hashObjs) > 1 {
			Log(remote, "Deleting %d/%d identical duplicates (md5sum %q)", len(hashObjs)-1, len(hashObjs), md5sum)
			for _, o := range hashObjs[1:] {
				_ = DeleteFile(o)
			}
		}
		objs = append(objs, hashObjs[0])
	}

	return objs
}

// dedupeInteractive interactively dedupes the slice of objects
func dedupeInteractive(remote string, objs []Object) {
	fmt.Printf("%s: %d duplicates remain\n", remote, len(objs))
	for i, o := range objs {
		md5sum, err := o.Hash(HashMD5)
		if err != nil {
			md5sum = err.Error()
		}
		fmt.Printf("  %d: %12d bytes, %s, md5sum %32s\n", i+1, o.Size(), o.ModTime().Format("2006-01-02 15:04:05.000000000"), md5sum)
	}
	switch Command([]string{"sSkip and do nothing", "kKeep just one (choose which in next step)", "rRename all to be different (by changing file.jpg to file-1.jpg)"}) {
	case 's':
	case 'k':
		keep := ChooseNumber("Enter the number of the file to keep", 1, len(objs))
		dedupeDeleteAllButOne(keep-1, remote, objs)
	case 'r':
		dedupeRename(remote, objs)
	}
}

type objectsSortedByModTime []Object

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
)

func (mode DeduplicateMode) String() string {
	switch mode {
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
	}
	return "unknown"
}

// Deduplicate interactively finds duplicate files and offers to
// delete all but one or rename them to be different. Only useful with
// Google Drive which can have duplicate file names.
func Deduplicate(f Fs, mode DeduplicateMode) error {
	Log(f, "Looking for duplicates using %v mode.", mode)
	files := map[string][]Object{}
	list := NewLister().Start(f, "")
	for {
		o, err := list.GetObject()
		if err != nil {
			return err
		}
		// Check if we are finished
		if o == nil {
			break
		}
		remote := o.Remote()
		files[remote] = append(files[remote], o)
	}
	for remote, objs := range files {
		if len(objs) > 1 {
			Log(remote, "Found %d duplicates - deleting identical copies", len(objs))
			objs = dedupeDeleteIdentical(remote, objs)
			if len(objs) <= 1 {
				Log(remote, "All duplicates removed")
				continue
			}
			switch mode {
			case DeduplicateInteractive:
				dedupeInteractive(remote, objs)
			case DeduplicateFirst:
				dedupeDeleteAllButOne(0, remote, objs)
			case DeduplicateNewest:
				sort.Sort(objectsSortedByModTime(objs)) // sort oldest first
				dedupeDeleteAllButOne(len(objs)-1, remote, objs)
			case DeduplicateOldest:
				sort.Sort(objectsSortedByModTime(objs)) // sort oldest first
				dedupeDeleteAllButOne(0, remote, objs)
			case DeduplicateRename:
				dedupeRename(remote, objs)
			case DeduplicateSkip:
				// skip
			default:
				//skip
			}
		}
	}
	return nil
}

// listToChan will transfer all incoming objects to a new channel.
//
// If an error occurs, the error will be logged, and it will close the
// channel.
//
// If the error was ErrorDirNotFound then it will be ignored
func listToChan(list *Lister) ObjectsChan {
	o := make(ObjectsChan, Config.Checkers)
	go func() {
		defer close(o)
		for {
			obj, dir, err := list.Get()
			if err != nil {
				if err != ErrorDirNotFound {
					Stats.Error()
					ErrorLog(nil, "Failed to list: %v", err)
				}
				return
			}
			if dir == nil && obj == nil {
				return
			}
			if o == nil {
				continue
			}
			o <- obj
		}
	}()
	return o
}

// CleanUp removes the trash for the Fs
func CleanUp(f Fs) error {
	fc, ok := f.(CleanUpper)
	if !ok {
		return errors.Errorf("%v doesn't support cleanup", f)
	}
	if Config.DryRun {
		Log(f, "Not running cleanup as --dry-run set")
		return nil
	}
	return fc.CleanUp()
}
