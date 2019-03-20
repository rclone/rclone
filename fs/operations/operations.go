// Package operations does generic operations on filesystems and objects
package operations

import (
	"bytes"
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ncw/rclone/fs"
	"github.com/ncw/rclone/fs/accounting"
	"github.com/ncw/rclone/fs/fserrors"
	"github.com/ncw/rclone/fs/hash"
	"github.com/ncw/rclone/fs/march"
	"github.com/ncw/rclone/fs/object"
	"github.com/ncw/rclone/fs/walk"
	"github.com/ncw/rclone/lib/readers"
	"github.com/pkg/errors"
	"golang.org/x/sync/errgroup"
)

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
func CheckHashes(src fs.ObjectInfo, dst fs.Object) (equal bool, ht hash.Type, err error) {
	common := src.Fs().Hashes().Overlap(dst.Fs().Hashes())
	// fs.Debugf(nil, "Shared hashes: %v", common)
	if common.Count() == 0 {
		return true, hash.None, nil
	}
	ht = common.GetOne()
	srcHash, err := src.Hash(ht)
	if err != nil {
		fs.CountError(err)
		fs.Errorf(src, "Failed to calculate src hash: %v", err)
		return false, ht, err
	}
	if srcHash == "" {
		return true, hash.None, nil
	}
	dstHash, err := dst.Hash(ht)
	if err != nil {
		fs.CountError(err)
		fs.Errorf(dst, "Failed to calculate dst hash: %v", err)
		return false, ht, err
	}
	if dstHash == "" {
		return true, hash.None, nil
	}
	if srcHash != dstHash {
		fs.Debugf(src, "%v = %s (%v)", ht, srcHash, src.Fs())
		fs.Debugf(dst, "%v = %s (%v)", ht, dstHash, dst.Fs())
	}
	return srcHash == dstHash, ht, nil
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
func Equal(src fs.ObjectInfo, dst fs.Object) bool {
	return equal(src, dst, fs.Config.SizeOnly, fs.Config.CheckSum)
}

// sizeDiffers compare the size of src and dst taking into account the
// various ways of ignoring sizes
func sizeDiffers(src, dst fs.ObjectInfo) bool {
	if fs.Config.IgnoreSize || src.Size() < 0 || dst.Size() < 0 {
		return false
	}
	return src.Size() != dst.Size()
}

var checksumWarning sync.Once

func equal(src fs.ObjectInfo, dst fs.Object, sizeOnly, checkSum bool) bool {
	if sizeDiffers(src, dst) {
		fs.Debugf(src, "Sizes differ (src %d vs dst %d)", src.Size(), dst.Size())
		return false
	}
	if sizeOnly {
		fs.Debugf(src, "Sizes identical")
		return true
	}

	// Assert: Size is equal or being ignored

	// If checking checksum and not modtime
	if checkSum {
		// Check the hash
		same, ht, _ := CheckHashes(src, dst)
		if !same {
			fs.Debugf(src, "%v differ", ht)
			return false
		}
		if ht == hash.None {
			checksumWarning.Do(func() {
				fs.Logf(dst.Fs(), "--checksum is in use but the source and destination have no hashes in common; falling back to --size-only")
			})
			fs.Debugf(src, "Size of src and dst objects identical")
		} else {
			fs.Debugf(src, "Size and %v of src and dst objects identical", ht)
		}
		return true
	}

	// Sizes the same so check the mtime
	modifyWindow := fs.GetModifyWindow(src.Fs(), dst.Fs())
	if modifyWindow == fs.ModTimeNotSupported {
		fs.Debugf(src, "Sizes identical")
		return true
	}
	srcModTime := src.ModTime()
	dstModTime := dst.ModTime()
	dt := dstModTime.Sub(srcModTime)
	if dt < modifyWindow && dt > -modifyWindow {
		fs.Debugf(src, "Size and modification time the same (differ by %s, within tolerance %s)", dt, modifyWindow)
		return true
	}

	fs.Debugf(src, "Modification times differ by %s: %v, %v", dt, srcModTime, dstModTime)

	// Check if the hashes are the same
	same, ht, _ := CheckHashes(src, dst)
	if !same {
		fs.Debugf(src, "%v differ", ht)
		return false
	}
	if ht == hash.None {
		// if couldn't check hash, return that they differ
		return false
	}

	// mod time differs but hash is the same to reset mod time if required
	if !fs.Config.NoUpdateModTime {
		if fs.Config.DryRun {
			fs.Logf(src, "Not updating modification time as --dry-run")
		} else {
			// Size and hash the same but mtime different
			// Error if objects are treated as immutable
			if fs.Config.Immutable {
				fs.Errorf(dst, "Timestamp mismatch between immutable objects")
				return false
			}
			// Update the mtime of the dst object here
			err := dst.SetModTime(srcModTime)
			if err == fs.ErrorCantSetModTime {
				fs.Debugf(dst, "src and dst identical but can't set mod time without re-uploading")
				return false
			} else if err == fs.ErrorCantSetModTimeWithoutDelete {
				fs.Debugf(dst, "src and dst identical but can't set mod time without deleting and re-uploading")
				// Remove the file if BackupDir isn't set.  If BackupDir is set we would rather have the old file
				// put in the BackupDir than deleted which is what will happen if we don't delete it.
				if fs.Config.BackupDir == "" {
					err = dst.Remove()
					if err != nil {
						fs.Errorf(dst, "failed to delete before re-upload: %v", err)
					}
				}
				return false
			} else if err != nil {
				fs.CountError(err)
				fs.Errorf(dst, "Failed to set modification time: %v", err)
			} else {
				fs.Infof(src, "Updated modification time in destination")
			}
		}
	}
	return true
}

// Used to remove a failed copy
//
// Returns whether the file was succesfully removed or not
func removeFailedCopy(dst fs.Object) bool {
	if dst == nil {
		return false
	}
	fs.Infof(dst, "Removing failed copy")
	removeErr := dst.Remove()
	if removeErr != nil {
		fs.Infof(dst, "Failed to remove failed copy: %s", removeErr)
		return false
	}
	return true
}

// Wrapper to override the remote for an object
type overrideRemoteObject struct {
	fs.Object
	remote string
}

// Remote returns the overriden remote name
func (o *overrideRemoteObject) Remote() string {
	return o.remote
}

// MimeType returns the mime type of the underlying object or "" if it
// can't be worked out
func (o *overrideRemoteObject) MimeType() string {
	if do, ok := o.Object.(fs.MimeTyper); ok {
		return do.MimeType()
	}
	return ""
}

// Check interface is satisfied
var _ fs.MimeTyper = (*overrideRemoteObject)(nil)

// Copy src object to dst or f if nil.  If dst is nil then it uses
// remote as the name of the new object.
//
// It returns the destination object if possible.  Note that this may
// be nil.
func Copy(f fs.Fs, dst fs.Object, remote string, src fs.Object) (newDst fs.Object, err error) {
	newDst = dst
	if fs.Config.DryRun {
		fs.Logf(src, "Not copying as --dry-run")
		return newDst, nil
	}
	maxTries := fs.Config.LowLevelRetries
	tries := 0
	doUpdate := dst != nil
	// work out which hash to use - limit to 1 hash in common
	var common hash.Set
	hashType := hash.None
	if !fs.Config.SizeOnly {
		common = src.Fs().Hashes().Overlap(f.Hashes())
		if common.Count() > 0 {
			hashType = common.GetOne()
			common = hash.Set(hashType)
		}
	}
	hashOption := &fs.HashesOption{Hashes: common}
	var actionTaken string
	for {
		// Try server side copy first - if has optional interface and
		// is same underlying remote
		actionTaken = "Copied (server side copy)"
		if doCopy := f.Features().Copy; doCopy != nil && (SameConfig(src.Fs(), f) || (SameRemoteType(src.Fs(), f) && f.Features().ServerSideAcrossConfigs)) {
			// Check transfer limit for server side copies
			if fs.Config.MaxTransfer >= 0 && accounting.Stats.GetBytes() >= int64(fs.Config.MaxTransfer) {
				return nil, accounting.ErrorMaxTransferLimitReached
			}
			newDst, err = doCopy(src, remote)
			if err == nil {
				dst = newDst
				accounting.Stats.Bytes(dst.Size()) // account the bytes for the server side transfer
			}
		} else {
			err = fs.ErrorCantCopy
		}
		// If can't server side copy, do it manually
		if err == fs.ErrorCantCopy {
			var in0 io.ReadCloser
			in0, err = newReOpen(src, hashOption, fs.Config.LowLevelRetries)
			if err != nil {
				err = errors.Wrap(err, "failed to open source object")
			} else {
				if src.Size() == -1 {
					// -1 indicates unknown size. Use Rcat to handle both remotes supporting and not supporting PutStream.
					if doUpdate {
						actionTaken = "Copied (Rcat, replaced existing)"
					} else {
						actionTaken = "Copied (Rcat, new)"
					}
					dst, err = Rcat(f, remote, in0, src.ModTime())
					newDst = dst
				} else {
					in := accounting.NewAccount(in0, src).WithBuffer() // account and buffer the transfer
					var wrappedSrc fs.ObjectInfo = src
					// We try to pass the original object if possible
					if src.Remote() != remote {
						wrappedSrc = &overrideRemoteObject{Object: src, remote: remote}
					}
					if doUpdate {
						actionTaken = "Copied (replaced existing)"
						err = dst.Update(in, wrappedSrc, hashOption)
					} else {
						actionTaken = "Copied (new)"
						dst, err = f.Put(in, wrappedSrc, hashOption)
					}
					closeErr := in.Close()
					if err == nil {
						newDst = dst
						err = closeErr
					}
				}
			}
		}
		tries++
		if tries >= maxTries {
			break
		}
		// Retry if err returned a retry error
		if fserrors.IsRetryError(err) || fserrors.ShouldRetry(err) {
			fs.Debugf(src, "Received error: %v - low level retry %d/%d", err, tries, maxTries)
			continue
		}
		// otherwise finish
		break
	}
	if err != nil {
		fs.CountError(err)
		fs.Errorf(src, "Failed to copy: %v", err)
		return newDst, err
	}

	// Verify sizes are the same after transfer
	if sizeDiffers(src, dst) {
		err = errors.Errorf("corrupted on transfer: sizes differ %d vs %d", src.Size(), dst.Size())
		fs.Errorf(dst, "%v", err)
		fs.CountError(err)
		removeFailedCopy(dst)
		return newDst, err
	}

	// Verify hashes are the same after transfer - ignoring blank hashes
	// TODO(klauspost): This could be extended, so we always create a hash type matching
	// the destination, and calculate it while sending.
	if hashType != hash.None {
		var srcSum string
		srcSum, err = src.Hash(hashType)
		if err != nil {
			fs.CountError(err)
			fs.Errorf(src, "Failed to read src hash: %v", err)
		} else if srcSum != "" {
			var dstSum string
			dstSum, err = dst.Hash(hashType)
			if err != nil {
				fs.CountError(err)
				fs.Errorf(dst, "Failed to read hash: %v", err)
			} else if !fs.Config.IgnoreChecksum && !hash.Equals(srcSum, dstSum) {
				err = errors.Errorf("corrupted on transfer: %v hash differ %q vs %q", hashType, srcSum, dstSum)
				fs.Errorf(dst, "%v", err)
				fs.CountError(err)
				removeFailedCopy(dst)
				return newDst, err
			}
		}
	}

	fs.Infof(src, actionTaken)
	return newDst, err
}

// Move src object to dst or fdst if nil.  If dst is nil then it uses
// remote as the name of the new object.
//
// Note that you must check the destination does not exist before
// calling this and pass it as dst.  If you pass dst=nil and the
// destination does exist then this may create duplicates or return
// errors.
//
// It returns the destination object if possible.  Note that this may
// be nil.
func Move(fdst fs.Fs, dst fs.Object, remote string, src fs.Object) (newDst fs.Object, err error) {
	newDst = dst
	if fs.Config.DryRun {
		fs.Logf(src, "Not moving as --dry-run")
		return newDst, nil
	}
	// See if we have Move available
	if doMove := fdst.Features().Move; doMove != nil && (SameConfig(src.Fs(), fdst) || (SameRemoteType(src.Fs(), fdst) && fdst.Features().ServerSideAcrossConfigs)) {
		// Delete destination if it exists
		if dst != nil {
			err = DeleteFile(dst)
			if err != nil {
				return newDst, err
			}
		}
		// Move dst <- src
		newDst, err = doMove(src, remote)
		switch err {
		case nil:
			fs.Infof(src, "Moved (server side)")
			return newDst, nil
		case fs.ErrorCantMove:
			fs.Debugf(src, "Can't move, switching to copy")
		default:
			fs.CountError(err)
			fs.Errorf(src, "Couldn't move: %v", err)
			return newDst, err
		}
	}
	// Move not found or didn't work so copy dst <- src
	newDst, err = Copy(fdst, dst, remote, src)
	if err != nil {
		fs.Errorf(src, "Not deleting source as copy failed: %v", err)
		return newDst, err
	}
	// Delete src if no error on copy
	return newDst, DeleteFile(src)
}

// CanServerSideMove returns true if fdst support server side moves or
// server side copies
//
// Some remotes simulate rename by server-side copy and delete, so include
// remotes that implements either Mover or Copier.
func CanServerSideMove(fdst fs.Fs) bool {
	canMove := fdst.Features().Move != nil
	canCopy := fdst.Features().Copy != nil
	return canMove || canCopy
}

// SuffixName adds the current --suffix to the remote, obeying
// --suffix-keep-extension if set
func SuffixName(remote string) string {
	if fs.Config.Suffix == "" {
		return remote
	}
	if fs.Config.SuffixKeepExtension {
		ext := path.Ext(remote)
		base := remote[:len(remote)-len(ext)]
		return base + fs.Config.Suffix + ext
	}
	return remote + fs.Config.Suffix
}

// DeleteFileWithBackupDir deletes a single file respecting --dry-run
// and accumulating stats and errors.
//
// If backupDir is set then it moves the file to there instead of
// deleting
func DeleteFileWithBackupDir(dst fs.Object, backupDir fs.Fs) (err error) {
	accounting.Stats.Checking(dst.Remote())
	numDeletes := accounting.Stats.Deletes(1)
	if fs.Config.MaxDelete != -1 && numDeletes > fs.Config.MaxDelete {
		return fserrors.FatalError(errors.New("--max-delete threshold reached"))
	}
	action, actioned, actioning := "delete", "Deleted", "deleting"
	if backupDir != nil {
		action, actioned, actioning = "move into backup dir", "Moved into backup dir", "moving into backup dir"
	}
	if fs.Config.DryRun {
		fs.Logf(dst, "Not %s as --dry-run", actioning)
	} else if backupDir != nil {
		if !SameConfig(dst.Fs(), backupDir) {
			err = errors.New("parameter to --backup-dir has to be on the same remote as destination")
		} else {
			remoteWithSuffix := SuffixName(dst.Remote())
			overwritten, _ := backupDir.NewObject(remoteWithSuffix)
			_, err = Move(backupDir, overwritten, remoteWithSuffix, dst)
		}
	} else {
		err = dst.Remove()
	}
	if err != nil {
		fs.CountError(err)
		fs.Errorf(dst, "Couldn't %s: %v", action, err)
	} else if !fs.Config.DryRun {
		fs.Infof(dst, actioned)
	}
	accounting.Stats.DoneChecking(dst.Remote())
	return err
}

// DeleteFile deletes a single file respecting --dry-run and accumulating stats and errors.
//
// If useBackupDir is set and --backup-dir is in effect then it moves
// the file to there instead of deleting
func DeleteFile(dst fs.Object) (err error) {
	return DeleteFileWithBackupDir(dst, nil)
}

// DeleteFilesWithBackupDir removes all the files passed in the
// channel
//
// If backupDir is set the files will be placed into that directory
// instead of being deleted.
func DeleteFilesWithBackupDir(toBeDeleted fs.ObjectsChan, backupDir fs.Fs) error {
	var wg sync.WaitGroup
	wg.Add(fs.Config.Transfers)
	var errorCount int32
	var fatalErrorCount int32

	for i := 0; i < fs.Config.Transfers; i++ {
		go func() {
			defer wg.Done()
			for dst := range toBeDeleted {
				err := DeleteFileWithBackupDir(dst, backupDir)
				if err != nil {
					atomic.AddInt32(&errorCount, 1)
					if fserrors.IsFatalError(err) {
						fs.Errorf(nil, "Got fatal error on delete: %s", err)
						atomic.AddInt32(&fatalErrorCount, 1)
						return
					}
				}
			}
		}()
	}
	fs.Infof(nil, "Waiting for deletions to finish")
	wg.Wait()
	if errorCount > 0 {
		err := errors.Errorf("failed to delete %d files", errorCount)
		if fatalErrorCount > 0 {
			return fserrors.FatalError(err)
		}
		return err
	}
	return nil
}

// DeleteFiles removes all the files passed in the channel
func DeleteFiles(toBeDeleted fs.ObjectsChan) error {
	return DeleteFilesWithBackupDir(toBeDeleted, nil)
}

// SameRemoteType returns true if fdst and fsrc are the same type
func SameRemoteType(fdst, fsrc fs.Info) bool {
	return fmt.Sprintf("%T", fdst) == fmt.Sprintf("%T", fsrc)
}

// SameConfig returns true if fdst and fsrc are using the same config
// file entry
func SameConfig(fdst, fsrc fs.Info) bool {
	return fdst.Name() == fsrc.Name()
}

// Same returns true if fdst and fsrc point to the same underlying Fs
func Same(fdst, fsrc fs.Info) bool {
	return SameConfig(fdst, fsrc) && strings.Trim(fdst.Root(), "/") == strings.Trim(fsrc.Root(), "/")
}

// Overlapping returns true if fdst and fsrc point to the same
// underlying Fs and they overlap.
func Overlapping(fdst, fsrc fs.Info) bool {
	if !SameConfig(fdst, fsrc) {
		return false
	}
	// Return the Root with a trailing / if not empty
	fixedRoot := func(f fs.Info) string {
		s := strings.Trim(filepath.ToSlash(f.Root()), "/")
		if s != "" {
			s += "/"
		}
		return s
	}
	fdstRoot := fixedRoot(fdst)
	fsrcRoot := fixedRoot(fsrc)
	return strings.HasPrefix(fdstRoot, fsrcRoot) || strings.HasPrefix(fsrcRoot, fdstRoot)
}

// checkIdentical checks to see if dst and src are identical
//
// it returns true if differences were found
// it also returns whether it couldn't be hashed
func checkIdentical(dst, src fs.Object) (differ bool, noHash bool) {
	same, ht, err := CheckHashes(src, dst)
	if err != nil {
		// CheckHashes will log and count errors
		return true, false
	}
	if ht == hash.None {
		return false, true
	}
	if !same {
		err = errors.Errorf("%v differ", ht)
		fs.Errorf(src, "%v", err)
		fs.CountError(err)
		return true, false
	}
	return false, false
}

// checkFn is the the type of the checking function used in CheckFn()
type checkFn func(a, b fs.Object) (differ bool, noHash bool)

// checkMarch is used to march over two Fses in the same way as
// sync/copy
type checkMarch struct {
	fdst, fsrc      fs.Fs
	check           checkFn
	oneway          bool
	differences     int32
	noHashes        int32
	srcFilesMissing int32
	dstFilesMissing int32
	matches         int32
}

// DstOnly have an object which is in the destination only
func (c *checkMarch) DstOnly(dst fs.DirEntry) (recurse bool) {
	switch dst.(type) {
	case fs.Object:
		if c.oneway {
			return false
		}
		err := errors.Errorf("File not in %v", c.fsrc)
		fs.Errorf(dst, "%v", err)
		fs.CountError(err)
		atomic.AddInt32(&c.differences, 1)
		atomic.AddInt32(&c.srcFilesMissing, 1)
	case fs.Directory:
		// Do the same thing to the entire contents of the directory
		return true
	default:
		panic("Bad object in DirEntries")
	}
	return false
}

// SrcOnly have an object which is in the source only
func (c *checkMarch) SrcOnly(src fs.DirEntry) (recurse bool) {
	switch src.(type) {
	case fs.Object:
		err := errors.Errorf("File not in %v", c.fdst)
		fs.Errorf(src, "%v", err)
		fs.CountError(err)
		atomic.AddInt32(&c.differences, 1)
		atomic.AddInt32(&c.dstFilesMissing, 1)
	case fs.Directory:
		// Do the same thing to the entire contents of the directory
		return true
	default:
		panic("Bad object in DirEntries")
	}
	return false
}

// check to see if two objects are identical using the check function
func (c *checkMarch) checkIdentical(dst, src fs.Object) (differ bool, noHash bool) {
	accounting.Stats.Checking(src.Remote())
	defer accounting.Stats.DoneChecking(src.Remote())
	if sizeDiffers(src, dst) {
		err := errors.Errorf("Sizes differ")
		fs.Errorf(src, "%v", err)
		fs.CountError(err)
		return true, false
	}
	if fs.Config.SizeOnly {
		return false, false
	}
	return c.check(dst, src)
}

// Match is called when src and dst are present, so sync src to dst
func (c *checkMarch) Match(dst, src fs.DirEntry) (recurse bool) {
	switch srcX := src.(type) {
	case fs.Object:
		dstX, ok := dst.(fs.Object)
		if ok {
			differ, noHash := c.checkIdentical(dstX, srcX)
			if differ {
				atomic.AddInt32(&c.differences, 1)
			} else {
				atomic.AddInt32(&c.matches, 1)
				fs.Debugf(dstX, "OK")
			}
			if noHash {
				atomic.AddInt32(&c.noHashes, 1)
			}
		} else {
			err := errors.Errorf("is file on %v but directory on %v", c.fsrc, c.fdst)
			fs.Errorf(src, "%v", err)
			fs.CountError(err)
			atomic.AddInt32(&c.differences, 1)
			atomic.AddInt32(&c.dstFilesMissing, 1)
		}
	case fs.Directory:
		// Do the same thing to the entire contents of the directory
		_, ok := dst.(fs.Directory)
		if ok {
			return true
		}
		err := errors.Errorf("is file on %v but directory on %v", c.fdst, c.fsrc)
		fs.Errorf(dst, "%v", err)
		fs.CountError(err)
		atomic.AddInt32(&c.differences, 1)
		atomic.AddInt32(&c.srcFilesMissing, 1)

	default:
		panic("Bad object in DirEntries")
	}
	return false
}

// CheckFn checks the files in fsrc and fdst according to Size and
// hash using checkFunction on each file to check the hashes.
//
// checkFunction sees if dst and src are identical
//
// it returns true if differences were found
// it also returns whether it couldn't be hashed
func CheckFn(fdst, fsrc fs.Fs, check checkFn, oneway bool) error {
	c := &checkMarch{
		fdst:   fdst,
		fsrc:   fsrc,
		check:  check,
		oneway: oneway,
	}

	// set up a march over fdst and fsrc
	m := &march.March{
		Ctx:      context.Background(),
		Fdst:     fdst,
		Fsrc:     fsrc,
		Dir:      "",
		Callback: c,
	}
	fs.Infof(fdst, "Waiting for checks to finish")
	m.Run()

	if c.dstFilesMissing > 0 {
		fs.Logf(fdst, "%d files missing", c.dstFilesMissing)
	}
	if c.srcFilesMissing > 0 {
		fs.Logf(fsrc, "%d files missing", c.srcFilesMissing)
	}

	fs.Logf(fdst, "%d differences found", accounting.Stats.GetErrors())
	if c.noHashes > 0 {
		fs.Logf(fdst, "%d hashes could not be checked", c.noHashes)
	}
	if c.matches > 0 {
		fs.Logf(fdst, "%d matching files", c.matches)
	}
	if c.differences > 0 {
		return errors.Errorf("%d differences found", c.differences)
	}
	return nil
}

// Check the files in fsrc and fdst according to Size and hash
func Check(fdst, fsrc fs.Fs, oneway bool) error {
	return CheckFn(fdst, fsrc, checkIdentical, oneway)
}

// CheckEqualReaders checks to see if in1 and in2 have the same
// content when read.
//
// it returns true if differences were found
func CheckEqualReaders(in1, in2 io.Reader) (differ bool, err error) {
	const bufSize = 64 * 1024
	buf1 := make([]byte, bufSize)
	buf2 := make([]byte, bufSize)
	for {
		n1, err1 := readers.ReadFill(in1, buf1)
		n2, err2 := readers.ReadFill(in2, buf2)
		// check errors
		if err1 != nil && err1 != io.EOF {
			return true, err1
		} else if err2 != nil && err2 != io.EOF {
			return true, err2
		}
		// err1 && err2 are nil or io.EOF here
		// process the data
		if n1 != n2 || !bytes.Equal(buf1[:n1], buf2[:n2]) {
			return true, nil
		}
		// if both streams finished the we have finished
		if err1 == io.EOF && err2 == io.EOF {
			break
		}
	}
	return false, nil
}

// CheckIdentical checks to see if dst and src are identical by
// reading all their bytes if necessary.
//
// it returns true if differences were found
func CheckIdentical(dst, src fs.Object) (differ bool, err error) {
	in1, err := dst.Open()
	if err != nil {
		return true, errors.Wrapf(err, "failed to open %q", dst)
	}
	in1 = accounting.NewAccount(in1, dst).WithBuffer() // account and buffer the transfer
	defer fs.CheckClose(in1, &err)

	in2, err := src.Open()
	if err != nil {
		return true, errors.Wrapf(err, "failed to open %q", src)
	}
	in2 = accounting.NewAccount(in2, src).WithBuffer() // account and buffer the transfer
	defer fs.CheckClose(in2, &err)

	return CheckEqualReaders(in1, in2)
}

// CheckDownload checks the files in fsrc and fdst according to Size
// and the actual contents of the files.
func CheckDownload(fdst, fsrc fs.Fs, oneway bool) error {
	check := func(a, b fs.Object) (differ bool, noHash bool) {
		differ, err := CheckIdentical(a, b)
		if err != nil {
			fs.CountError(err)
			fs.Errorf(a, "Failed to download: %v", err)
			return true, true
		}
		return differ, false
	}
	return CheckFn(fdst, fsrc, check, oneway)
}

// ListFn lists the Fs to the supplied function
//
// Lists in parallel which may get them out of order
func ListFn(f fs.Fs, fn func(fs.Object)) error {
	return walk.ListR(f, "", false, fs.Config.MaxDepth, walk.ListObjects, func(entries fs.DirEntries) error {
		entries.ForObject(fn)
		return nil
	})
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
func List(f fs.Fs, w io.Writer) error {
	return ListFn(f, func(o fs.Object) {
		syncFprintf(w, "%9d %s\n", o.Size(), o.Remote())
	})
}

// ListLong lists the Fs to the supplied writer
//
// Shows size, mod time and path - obeys includes and excludes
//
// Lists in parallel which may get them out of order
func ListLong(f fs.Fs, w io.Writer) error {
	return ListFn(f, func(o fs.Object) {
		accounting.Stats.Checking(o.Remote())
		modTime := o.ModTime()
		accounting.Stats.DoneChecking(o.Remote())
		syncFprintf(w, "%9d %s %s\n", o.Size(), modTime.Local().Format("2006-01-02 15:04:05.000000000"), o.Remote())
	})
}

// Md5sum list the Fs to the supplied writer
//
// Produces the same output as the md5sum command - obeys includes and
// excludes
//
// Lists in parallel which may get them out of order
func Md5sum(f fs.Fs, w io.Writer) error {
	return HashLister(hash.MD5, f, w)
}

// Sha1sum list the Fs to the supplied writer
//
// Obeys includes and excludes
//
// Lists in parallel which may get them out of order
func Sha1sum(f fs.Fs, w io.Writer) error {
	return HashLister(hash.SHA1, f, w)
}

// DropboxHashSum list the Fs to the supplied writer
//
// Obeys includes and excludes
//
// Lists in parallel which may get them out of order
func DropboxHashSum(f fs.Fs, w io.Writer) error {
	return HashLister(hash.Dropbox, f, w)
}

// hashSum returns the human readable hash for ht passed in.  This may
// be UNSUPPORTED or ERROR.
func hashSum(ht hash.Type, o fs.Object) string {
	accounting.Stats.Checking(o.Remote())
	sum, err := o.Hash(ht)
	accounting.Stats.DoneChecking(o.Remote())
	if err == hash.ErrUnsupported {
		sum = "UNSUPPORTED"
	} else if err != nil {
		fs.Debugf(o, "Failed to read %v: %v", ht, err)
		sum = "ERROR"
	}
	return sum
}

// HashLister does a md5sum equivalent for the hash type passed in
func HashLister(ht hash.Type, f fs.Fs, w io.Writer) error {
	return ListFn(f, func(o fs.Object) {
		sum := hashSum(ht, o)
		syncFprintf(w, "%*s  %s\n", hash.Width[ht], sum, o.Remote())
	})
}

// Count counts the objects and their sizes in the Fs
//
// Obeys includes and excludes
func Count(f fs.Fs) (objects int64, size int64, err error) {
	err = ListFn(f, func(o fs.Object) {
		atomic.AddInt64(&objects, 1)
		atomic.AddInt64(&size, o.Size())
	})
	return
}

// ConfigMaxDepth returns the depth to use for a recursive or non recursive listing.
func ConfigMaxDepth(recursive bool) int {
	depth := fs.Config.MaxDepth
	if !recursive && depth < 0 {
		depth = 1
	}
	return depth
}

// ListDir lists the directories/buckets/containers in the Fs to the supplied writer
func ListDir(f fs.Fs, w io.Writer) error {
	return walk.ListR(f, "", false, ConfigMaxDepth(false), walk.ListDirs, func(entries fs.DirEntries) error {
		entries.ForDir(func(dir fs.Directory) {
			if dir != nil {
				syncFprintf(w, "%12d %13s %9d %s\n", dir.Size(), dir.ModTime().Local().Format("2006-01-02 15:04:05"), dir.Items(), dir.Remote())
			}
		})
		return nil
	})
}

// Mkdir makes a destination directory or container
func Mkdir(f fs.Fs, dir string) error {
	if fs.Config.DryRun {
		fs.Logf(fs.LogDirName(f, dir), "Not making directory as dry run is set")
		return nil
	}
	fs.Debugf(fs.LogDirName(f, dir), "Making directory")
	err := f.Mkdir(dir)
	if err != nil {
		fs.CountError(err)
		return err
	}
	return nil
}

// TryRmdir removes a container but not if not empty.  It doesn't
// count errors but may return one.
func TryRmdir(f fs.Fs, dir string) error {
	if fs.Config.DryRun {
		fs.Logf(fs.LogDirName(f, dir), "Not deleting as dry run is set")
		return nil
	}
	fs.Debugf(fs.LogDirName(f, dir), "Removing directory")
	return f.Rmdir(dir)
}

// Rmdir removes a container but not if not empty
func Rmdir(f fs.Fs, dir string) error {
	err := TryRmdir(f, dir)
	if err != nil {
		fs.CountError(err)
		return err
	}
	return err
}

// Purge removes a directory and all of its contents
func Purge(f fs.Fs, dir string) error {
	doFallbackPurge := true
	var err error
	if dir == "" {
		// FIXME change the Purge interface so it takes a dir - see #1891
		if doPurge := f.Features().Purge; doPurge != nil {
			doFallbackPurge = false
			if fs.Config.DryRun {
				fs.Logf(f, "Not purging as --dry-run set")
			} else {
				err = doPurge()
				if err == fs.ErrorCantPurge {
					doFallbackPurge = true
				}
			}
		}
	}
	if doFallbackPurge {
		// DeleteFiles and Rmdir observe --dry-run
		err = DeleteFiles(listToChan(f, dir))
		if err != nil {
			return err
		}
		err = Rmdirs(f, dir, false)
	}
	if err != nil {
		fs.CountError(err)
		return err
	}
	return nil
}

// Delete removes all the contents of a container.  Unlike Purge, it
// obeys includes and excludes.
func Delete(f fs.Fs) error {
	delChan := make(fs.ObjectsChan, fs.Config.Transfers)
	delErr := make(chan error, 1)
	go func() {
		delErr <- DeleteFiles(delChan)
	}()
	err := ListFn(f, func(o fs.Object) {
		delChan <- o
	})
	close(delChan)
	delError := <-delErr
	if err == nil {
		err = delError
	}
	return err
}

// listToChan will transfer all objects in the listing to the output
//
// If an error occurs, the error will be logged, and it will close the
// channel.
//
// If the error was ErrorDirNotFound then it will be ignored
func listToChan(f fs.Fs, dir string) fs.ObjectsChan {
	o := make(fs.ObjectsChan, fs.Config.Checkers)
	go func() {
		defer close(o)
		err := walk.ListR(f, dir, true, fs.Config.MaxDepth, walk.ListObjects, func(entries fs.DirEntries) error {
			entries.ForObject(func(obj fs.Object) {
				o <- obj
			})
			return nil
		})
		if err != nil && err != fs.ErrorDirNotFound {
			err = errors.Wrap(err, "failed to list")
			fs.CountError(err)
			fs.Errorf(nil, "%v", err)
		}
	}()
	return o
}

// CleanUp removes the trash for the Fs
func CleanUp(f fs.Fs) error {
	doCleanUp := f.Features().CleanUp
	if doCleanUp == nil {
		return errors.Errorf("%v doesn't support cleanup", f)
	}
	if fs.Config.DryRun {
		fs.Logf(f, "Not running cleanup as --dry-run set")
		return nil
	}
	return doCleanUp()
}

// wrap a Reader and a Closer together into a ReadCloser
type readCloser struct {
	io.Reader
	io.Closer
}

// Cat any files to the io.Writer
//
// if offset == 0 it will be ignored
// if offset > 0 then the file will be seeked to that offset
// if offset < 0 then the file will be seeked that far from the end
//
// if count < 0 then it will be ignored
// if count >= 0 then only that many characters will be output
func Cat(f fs.Fs, w io.Writer, offset, count int64) error {
	var mu sync.Mutex
	return ListFn(f, func(o fs.Object) {
		var err error
		accounting.Stats.Transferring(o.Remote())
		defer func() {
			accounting.Stats.DoneTransferring(o.Remote(), err == nil)
		}()
		opt := fs.RangeOption{Start: offset, End: -1}
		size := o.Size()
		if opt.Start < 0 {
			opt.Start += size
		}
		if count >= 0 {
			opt.End = opt.Start + count - 1
		}
		var options []fs.OpenOption
		if opt.Start > 0 || opt.End >= 0 {
			options = append(options, &opt)
		}
		in, err := o.Open(options...)
		if err != nil {
			fs.CountError(err)
			fs.Errorf(o, "Failed to open: %v", err)
			return
		}
		if count >= 0 {
			in = &readCloser{Reader: &io.LimitedReader{R: in, N: count}, Closer: in}
			// reduce remaining size to count
			if size > count {
				size = count
			}
		}
		in = accounting.NewAccountSizeName(in, size, o.Remote()).WithBuffer() // account and buffer the transfer
		defer func() {
			err = in.Close()
			if err != nil {
				fs.CountError(err)
				fs.Errorf(o, "Failed to close: %v", err)
			}
		}()
		// take the lock just before we output stuff, so at the last possible moment
		mu.Lock()
		defer mu.Unlock()
		_, err = io.Copy(w, in)
		if err != nil {
			fs.CountError(err)
			fs.Errorf(o, "Failed to send to output: %v", err)
		}
	})
}

// Rcat reads data from the Reader until EOF and uploads it to a file on remote
func Rcat(fdst fs.Fs, dstFileName string, in io.ReadCloser, modTime time.Time) (dst fs.Object, err error) {
	accounting.Stats.Transferring(dstFileName)
	in = accounting.NewAccountSizeName(in, -1, dstFileName).WithBuffer()
	defer func() {
		accounting.Stats.DoneTransferring(dstFileName, err == nil)
		if otherErr := in.Close(); otherErr != nil {
			fs.Debugf(fdst, "Rcat: failed to close source: %v", err)
		}
	}()

	hashOption := &fs.HashesOption{Hashes: fdst.Hashes()}
	hash, err := hash.NewMultiHasherTypes(fdst.Hashes())
	if err != nil {
		return nil, err
	}
	readCounter := readers.NewCountingReader(in)
	trackingIn := io.TeeReader(readCounter, hash)

	compare := func(dst fs.Object) error {
		src := object.NewStaticObjectInfo(dstFileName, modTime, int64(readCounter.BytesRead()), false, hash.Sums(), fdst)
		if !Equal(src, dst) {
			err = errors.Errorf("corrupted on transfer")
			fs.CountError(err)
			fs.Errorf(dst, "%v", err)
			return err
		}
		return nil
	}

	// check if file small enough for direct upload
	buf := make([]byte, fs.Config.StreamingUploadCutoff)
	if n, err := io.ReadFull(trackingIn, buf); err == io.EOF || err == io.ErrUnexpectedEOF {
		fs.Debugf(fdst, "File to upload is small (%d bytes), uploading instead of streaming", n)
		src := object.NewMemoryObject(dstFileName, modTime, buf[:n])
		return Copy(fdst, nil, dstFileName, src)
	}

	// Make a new ReadCloser with the bits we've already read
	in = &readCloser{
		Reader: io.MultiReader(bytes.NewReader(buf), trackingIn),
		Closer: in,
	}

	fStreamTo := fdst
	canStream := fdst.Features().PutStream != nil
	if !canStream {
		fs.Debugf(fdst, "Target remote doesn't support streaming uploads, creating temporary local FS to spool file")
		tmpLocalFs, err := fs.TemporaryLocalFs()
		if err != nil {
			return nil, errors.Wrap(err, "Failed to create temporary local FS to spool file")
		}
		defer func() {
			err := Purge(tmpLocalFs, "")
			if err != nil {
				fs.Infof(tmpLocalFs, "Failed to cleanup temporary FS: %v", err)
			}
		}()
		fStreamTo = tmpLocalFs
	}

	if fs.Config.DryRun {
		fs.Logf("stdin", "Not uploading as --dry-run")
		// prevents "broken pipe" errors
		_, err = io.Copy(ioutil.Discard, in)
		return nil, err
	}

	objInfo := object.NewStaticObjectInfo(dstFileName, modTime, -1, false, nil, nil)
	if dst, err = fStreamTo.Features().PutStream(in, objInfo, hashOption); err != nil {
		return dst, err
	}
	if err = compare(dst); err != nil {
		return dst, err
	}
	if !canStream {
		// copy dst (which is the local object we have just streamed to) to the remote
		return Copy(fdst, nil, dstFileName, dst)
	}
	return dst, nil
}

// PublicLink adds a "readable by anyone with link" permission on the given file or folder.
func PublicLink(f fs.Fs, remote string) (string, error) {
	doPublicLink := f.Features().PublicLink
	if doPublicLink == nil {
		return "", errors.Errorf("%v doesn't support public links", f)
	}
	return doPublicLink(remote)
}

// Rmdirs removes any empty directories (or directories only
// containing empty directories) under f, including f.
func Rmdirs(f fs.Fs, dir string, leaveRoot bool) error {
	dirEmpty := make(map[string]bool)
	dirEmpty[dir] = !leaveRoot
	err := walk.Walk(f, dir, true, fs.Config.MaxDepth, func(dirPath string, entries fs.DirEntries, err error) error {
		if err != nil {
			fs.CountError(err)
			fs.Errorf(f, "Failed to list %q: %v", dirPath, err)
			return nil
		}
		for _, entry := range entries {
			switch x := entry.(type) {
			case fs.Directory:
				// add a new directory as empty
				dir := x.Remote()
				_, found := dirEmpty[dir]
				if !found {
					dirEmpty[dir] = true
				}
			case fs.Object:
				// mark the parents of the file as being non-empty
				dir := x.Remote()
				for dir != "" {
					dir = path.Dir(dir)
					if dir == "." || dir == "/" {
						dir = ""
					}
					empty, found := dirEmpty[dir]
					// End if we reach a directory which is non-empty
					if found && !empty {
						break
					}
					dirEmpty[dir] = false
				}
			}
		}
		return nil
	})
	if err != nil {
		return errors.Wrap(err, "failed to rmdirs")
	}
	// Now delete the empty directories, starting from the longest path
	var toDelete []string
	for dir, empty := range dirEmpty {
		if empty {
			toDelete = append(toDelete, dir)
		}
	}
	sort.Strings(toDelete)
	for i := len(toDelete) - 1; i >= 0; i-- {
		dir := toDelete[i]
		err := TryRmdir(f, dir)
		if err != nil {
			fs.CountError(err)
			fs.Errorf(dir, "Failed to rmdir: %v", err)
			return err
		}
	}
	return nil
}

// NeedTransfer checks to see if src needs to be copied to dst using
// the current config.
//
// Returns a flag which indicates whether the file needs to be
// transferred or not.
func NeedTransfer(dst, src fs.Object) bool {
	if dst == nil {
		fs.Debugf(src, "Couldn't find file - need to transfer")
		return true
	}
	// If we should ignore existing files, don't transfer
	if fs.Config.IgnoreExisting {
		fs.Debugf(src, "Destination exists, skipping")
		return false
	}
	// If we should upload unconditionally
	if fs.Config.IgnoreTimes {
		fs.Debugf(src, "Transferring unconditionally as --ignore-times is in use")
		return true
	}
	// If UpdateOlder is in effect, skip if dst is newer than src
	if fs.Config.UpdateOlder {
		srcModTime := src.ModTime()
		dstModTime := dst.ModTime()
		dt := dstModTime.Sub(srcModTime)
		// If have a mutually agreed precision then use that
		modifyWindow := fs.GetModifyWindow(dst.Fs(), src.Fs())
		if modifyWindow == fs.ModTimeNotSupported {
			// Otherwise use 1 second as a safe default as
			// the resolution of the time a file was
			// uploaded.
			modifyWindow = time.Second
		}
		switch {
		case dt >= modifyWindow:
			fs.Debugf(src, "Destination is newer than source, skipping")
			return false
		case dt <= -modifyWindow:
			fs.Debugf(src, "Destination is older than source, transferring")
		default:
			if src.Size() == dst.Size() {
				fs.Debugf(src, "Destination mod time is within %v of source and sizes identical, skipping", modifyWindow)
				return false
			}
			fs.Debugf(src, "Destination mod time is within %v of source but sizes differ, transferring", modifyWindow)
		}
	} else {
		// Check to see if changed or not
		if Equal(src, dst) {
			fs.Debugf(src, "Unchanged skipping")
			return false
		}
	}
	return true
}

// RcatSize reads data from the Reader until EOF and uploads it to a file on remote.
// Pass in size >=0 if known, <0 if not known
func RcatSize(fdst fs.Fs, dstFileName string, in io.ReadCloser, size int64, modTime time.Time) (dst fs.Object, err error) {
	var obj fs.Object

	if size >= 0 {
		// Size known use Put
		accounting.Stats.Transferring(dstFileName)
		body := ioutil.NopCloser(in)                                 // we let the server close the body
		in := accounting.NewAccountSizeName(body, size, dstFileName) // account the transfer (no buffering)

		if fs.Config.DryRun {
			fs.Logf("stdin", "Not uploading as --dry-run")
			// prevents "broken pipe" errors
			_, err = io.Copy(ioutil.Discard, in)
			return nil, err
		}

		var err error
		defer func() {
			closeErr := in.Close()
			if closeErr != nil {
				accounting.Stats.Error(closeErr)
				fs.Errorf(dstFileName, "Post request: close failed: %v", closeErr)
			}
			accounting.Stats.DoneTransferring(dstFileName, err == nil)
		}()
		info := object.NewStaticObjectInfo(dstFileName, modTime, size, true, nil, fdst)
		obj, err = fdst.Put(in, info)
		if err != nil {
			fs.Errorf(dstFileName, "Post request put error: %v", err)

			return nil, err
		}
	} else {
		// Size unknown use Rcat
		obj, err = Rcat(fdst, dstFileName, in, modTime)
		if err != nil {
			fs.Errorf(dstFileName, "Post request rcat error: %v", err)

			return nil, err
		}
	}

	return obj, nil
}

// CopyURL copies the data from the url to (fdst, dstFileName)
func CopyURL(fdst fs.Fs, dstFileName string, url string) (dst fs.Object, err error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer fs.CheckClose(resp.Body, &err)
	return RcatSize(fdst, dstFileName, resp.Body, resp.ContentLength, time.Now())
}

// moveOrCopyFile moves or copies a single file possibly to a new name
func moveOrCopyFile(fdst fs.Fs, fsrc fs.Fs, dstFileName string, srcFileName string, cp bool) (err error) {
	dstFilePath := path.Join(fdst.Root(), dstFileName)
	srcFilePath := path.Join(fsrc.Root(), srcFileName)
	if fdst.Name() == fsrc.Name() && dstFilePath == srcFilePath {
		fs.Debugf(fdst, "don't need to copy/move %s, it is already at target location", dstFileName)
		return nil
	}

	// Choose operations
	Op := Move
	if cp {
		Op = Copy
	}

	// Find src object
	srcObj, err := fsrc.NewObject(srcFileName)
	if err != nil {
		return err
	}

	// Find dst object if it exists
	dstObj, err := fdst.NewObject(dstFileName)
	if err == fs.ErrorObjectNotFound {
		dstObj = nil
	} else if err != nil {
		return err
	}

	if NeedTransfer(dstObj, srcObj) {
		accounting.Stats.Transferring(srcFileName)
		_, err = Op(fdst, dstObj, dstFileName, srcObj)
		accounting.Stats.DoneTransferring(srcFileName, err == nil)
	} else {
		accounting.Stats.Checking(srcFileName)
		if !cp {
			err = DeleteFile(srcObj)
		}
		defer accounting.Stats.DoneChecking(srcFileName)
	}
	return err
}

// MoveFile moves a single file possibly to a new name
func MoveFile(fdst fs.Fs, fsrc fs.Fs, dstFileName string, srcFileName string) (err error) {
	return moveOrCopyFile(fdst, fsrc, dstFileName, srcFileName, false)
}

// CopyFile moves a single file possibly to a new name
func CopyFile(fdst fs.Fs, fsrc fs.Fs, dstFileName string, srcFileName string) (err error) {
	return moveOrCopyFile(fdst, fsrc, dstFileName, srcFileName, true)
}

// SetTier changes tier of object in remote
func SetTier(fsrc fs.Fs, tier string) error {
	return ListFn(fsrc, func(o fs.Object) {
		objImpl, ok := o.(fs.SetTierer)
		if !ok {
			fs.Errorf(fsrc, "Remote object does not implement SetTier")
			return
		}
		err := objImpl.SetTier(tier)
		if err != nil {
			fs.Errorf(fsrc, "Failed to do SetTier, %v", err)
		}
	})
}

// ListFormat defines files information print format
type ListFormat struct {
	separator string
	dirSlash  bool
	absolute  bool
	output    []func(entry *ListJSONItem) string
	csv       *csv.Writer
	buf       bytes.Buffer
}

// SetSeparator changes separator in struct
func (l *ListFormat) SetSeparator(separator string) {
	l.separator = separator
}

// SetDirSlash defines if slash should be printed
func (l *ListFormat) SetDirSlash(dirSlash bool) {
	l.dirSlash = dirSlash
}

// SetAbsolute prints a leading slash in front of path names
func (l *ListFormat) SetAbsolute(absolute bool) {
	l.absolute = absolute
}

// SetCSV defines if the output should be csv
//
// Note that you should call SetSeparator before this if you want a
// custom separator
func (l *ListFormat) SetCSV(useCSV bool) {
	if useCSV {
		l.csv = csv.NewWriter(&l.buf)
		if l.separator != "" {
			l.csv.Comma = []rune(l.separator)[0]
		}
	} else {
		l.csv = nil
	}
}

// SetOutput sets functions used to create files information
func (l *ListFormat) SetOutput(output []func(entry *ListJSONItem) string) {
	l.output = output
}

// AddModTime adds file's Mod Time to output
func (l *ListFormat) AddModTime() {
	l.AppendOutput(func(entry *ListJSONItem) string {
		return entry.ModTime.When.Local().Format("2006-01-02 15:04:05")
	})
}

// AddSize adds file's size to output
func (l *ListFormat) AddSize() {
	l.AppendOutput(func(entry *ListJSONItem) string {
		return strconv.FormatInt(entry.Size, 10)
	})
}

// normalisePath makes sure the path has the correct slashes for the current mode
func (l *ListFormat) normalisePath(entry *ListJSONItem, remote string) string {
	if l.absolute && !strings.HasPrefix(remote, "/") {
		remote = "/" + remote
	}
	if entry.IsDir && l.dirSlash {
		remote += "/"
	}
	return remote
}

// AddPath adds path to file to output
func (l *ListFormat) AddPath() {
	l.AppendOutput(func(entry *ListJSONItem) string {
		return l.normalisePath(entry, entry.Path)
	})
}

// AddEncrypted adds the encrypted path to file to output
func (l *ListFormat) AddEncrypted() {
	l.AppendOutput(func(entry *ListJSONItem) string {
		return l.normalisePath(entry, entry.Encrypted)
	})
}

// AddHash adds the hash of the type given to the output
func (l *ListFormat) AddHash(ht hash.Type) {
	hashName := ht.String()
	l.AppendOutput(func(entry *ListJSONItem) string {
		if entry.IsDir {
			return ""
		}
		return entry.Hashes[hashName]
	})
}

// AddID adds file's ID to the output if known
func (l *ListFormat) AddID() {
	l.AppendOutput(func(entry *ListJSONItem) string {
		return entry.ID
	})
}

// AddOrigID adds file's Original ID to the output if known
func (l *ListFormat) AddOrigID() {
	l.AppendOutput(func(entry *ListJSONItem) string {
		return entry.OrigID
	})
}

// AddMimeType adds file's MimeType to the output if known
func (l *ListFormat) AddMimeType() {
	l.AppendOutput(func(entry *ListJSONItem) string {
		return entry.MimeType
	})
}

// AppendOutput adds string generated by specific function to printed output
func (l *ListFormat) AppendOutput(functionToAppend func(item *ListJSONItem) string) {
	l.output = append(l.output, functionToAppend)
}

// Format prints information about the DirEntry in the format defined
func (l *ListFormat) Format(entry *ListJSONItem) (result string) {
	var out []string
	for _, fun := range l.output {
		out = append(out, fun(entry))
	}
	if l.csv != nil {
		l.buf.Reset()
		_ = l.csv.Write(out) // can't fail writing to bytes.Buffer
		l.csv.Flush()
		result = strings.TrimRight(l.buf.String(), "\n")
	} else {
		result = strings.Join(out, l.separator)
	}
	return result
}

// DirMove renames srcRemote to dstRemote
//
// It does this by loading the directory tree into memory (using ListR
// if available) and doing renames in parallel.
func DirMove(f fs.Fs, srcRemote, dstRemote string) (err error) {
	// Use DirMove if possible
	if doDirMove := f.Features().DirMove; doDirMove != nil {
		return doDirMove(f, srcRemote, dstRemote)
	}

	// Load the directory tree into memory
	tree, err := walk.NewDirTree(f, srcRemote, true, -1)
	if err != nil {
		return errors.Wrap(err, "RenameDir tree walk")
	}

	// Get the directories in sorted order
	dirs := tree.Dirs()

	// Make the destination directories - must be done in order not in parallel
	for _, dir := range dirs {
		dstPath := dstRemote + dir[len(srcRemote):]
		err := f.Mkdir(dstPath)
		if err != nil {
			return errors.Wrap(err, "RenameDir mkdir")
		}
	}

	// Rename the files in parallel
	type rename struct {
		o       fs.Object
		newPath string
	}
	renames := make(chan rename, fs.Config.Transfers)
	g, ctx := errgroup.WithContext(context.Background())
	for i := 0; i < fs.Config.Transfers; i++ {
		g.Go(func() error {
			for job := range renames {
				dstOverwritten, _ := f.NewObject(job.newPath)
				_, err := Move(f, dstOverwritten, job.newPath, job.o)
				if err != nil {
					return err
				}
				select {
				case <-ctx.Done():
					return ctx.Err()
				default:
				}

			}
			return nil
		})
	}
	for dir, entries := range tree {
		dstPath := dstRemote + dir[len(srcRemote):]
		for _, entry := range entries {
			if o, ok := entry.(fs.Object); ok {
				renames <- rename{o, path.Join(dstPath, path.Base(o.Remote()))}
			}
		}
	}
	close(renames)
	err = g.Wait()
	if err != nil {
		return errors.Wrap(err, "RenameDir renames")
	}

	// Remove the source directories in reverse order
	for i := len(dirs) - 1; i >= 0; i-- {
		err := f.Rmdir(dirs[i])
		if err != nil {
			return errors.Wrap(err, "RenameDir rmdir")
		}
	}

	return nil
}
