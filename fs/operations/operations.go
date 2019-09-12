// Package operations does generic operations on filesystems and objects
package operations

import (
	"bytes"
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"io/ioutil"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/pkg/errors"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/accounting"
	"github.com/rclone/rclone/fs/cache"
	"github.com/rclone/rclone/fs/fserrors"
	"github.com/rclone/rclone/fs/fshttp"
	"github.com/rclone/rclone/fs/hash"
	"github.com/rclone/rclone/fs/march"
	"github.com/rclone/rclone/fs/object"
	"github.com/rclone/rclone/fs/walk"
	"github.com/rclone/rclone/lib/random"
	"github.com/rclone/rclone/lib/readers"
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
func CheckHashes(ctx context.Context, src fs.ObjectInfo, dst fs.Object) (equal bool, ht hash.Type, err error) {
	common := src.Fs().Hashes().Overlap(dst.Fs().Hashes())
	// fs.Debugf(nil, "Shared hashes: %v", common)
	if common.Count() == 0 {
		return true, hash.None, nil
	}
	equal, ht, _, _, err = checkHashes(ctx, src, dst, common.GetOne())
	return equal, ht, err
}

// checkHashes does the work of CheckHashes but takes a hash.Type and
// returns the effective hash type used.
func checkHashes(ctx context.Context, src fs.ObjectInfo, dst fs.Object, ht hash.Type) (equal bool, htOut hash.Type, srcHash, dstHash string, err error) {
	// Calculate hashes in parallel
	g, ctx := errgroup.WithContext(ctx)
	g.Go(func() (err error) {
		srcHash, err = src.Hash(ctx, ht)
		if err != nil {
			fs.CountError(err)
			fs.Errorf(src, "Failed to calculate src hash: %v", err)
		}
		return err
	})
	g.Go(func() (err error) {
		dstHash, err = dst.Hash(ctx, ht)
		if err != nil {
			fs.CountError(err)
			fs.Errorf(dst, "Failed to calculate dst hash: %v", err)
		}
		return err
	})
	err = g.Wait()
	if err != nil {
		return false, ht, srcHash, dstHash, err
	}
	if srcHash == "" {
		return true, hash.None, srcHash, dstHash, nil
	}
	if dstHash == "" {
		return true, hash.None, srcHash, dstHash, nil
	}
	if srcHash != dstHash {
		fs.Debugf(src, "%v = %s (%v)", ht, srcHash, src.Fs())
		fs.Debugf(dst, "%v = %s (%v)", ht, dstHash, dst.Fs())
	} else {
		fs.Debugf(src, "%v = %s OK", ht, srcHash)
	}
	return srcHash == dstHash, ht, srcHash, dstHash, nil
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
func Equal(ctx context.Context, src fs.ObjectInfo, dst fs.Object) bool {
	return equal(ctx, src, dst, fs.Config.SizeOnly, fs.Config.CheckSum, !fs.Config.NoUpdateModTime)
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

func equal(ctx context.Context, src fs.ObjectInfo, dst fs.Object, sizeOnly, checkSum, UpdateModTime bool) bool {
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
		same, ht, _ := CheckHashes(ctx, src, dst)
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
	srcModTime := src.ModTime(ctx)
	dstModTime := dst.ModTime(ctx)
	dt := dstModTime.Sub(srcModTime)
	if dt < modifyWindow && dt > -modifyWindow {
		fs.Debugf(src, "Size and modification time the same (differ by %s, within tolerance %s)", dt, modifyWindow)
		return true
	}

	fs.Debugf(src, "Modification times differ by %s: %v, %v", dt, srcModTime, dstModTime)

	// Check if the hashes are the same
	same, ht, _ := CheckHashes(ctx, src, dst)
	if !same {
		fs.Debugf(src, "%v differ", ht)
		return false
	}
	if ht == hash.None {
		// if couldn't check hash, return that they differ
		return false
	}

	// mod time differs but hash is the same to reset mod time if required
	if UpdateModTime {
		if fs.Config.DryRun {
			fs.Logf(src, "Not updating modification time as --dry-run")
		} else {
			// Size and hash the same but mtime different
			// Error if objects are treated as immutable
			if fs.Config.Immutable {
				fs.Errorf(dst, "StartedAt mismatch between immutable objects")
				return false
			}
			// Update the mtime of the dst object here
			err := dst.SetModTime(ctx, srcModTime)
			if err == fs.ErrorCantSetModTime {
				fs.Debugf(dst, "src and dst identical but can't set mod time without re-uploading")
				return false
			} else if err == fs.ErrorCantSetModTimeWithoutDelete {
				fs.Debugf(dst, "src and dst identical but can't set mod time without deleting and re-uploading")
				// Remove the file if BackupDir isn't set.  If BackupDir is set we would rather have the old file
				// put in the BackupDir than deleted which is what will happen if we don't delete it.
				if fs.Config.BackupDir == "" {
					err = dst.Remove(ctx)
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
// Returns whether the file was successfully removed or not
func removeFailedCopy(ctx context.Context, dst fs.Object) bool {
	if dst == nil {
		return false
	}
	fs.Infof(dst, "Removing failed copy")
	removeErr := dst.Remove(ctx)
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

// Remote returns the overridden remote name
func (o *overrideRemoteObject) Remote() string {
	return o.remote
}

// MimeType returns the mime type of the underlying object or "" if it
// can't be worked out
func (o *overrideRemoteObject) MimeType(ctx context.Context) string {
	if do, ok := o.Object.(fs.MimeTyper); ok {
		return do.MimeType(ctx)
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
func Copy(ctx context.Context, f fs.Fs, dst fs.Object, remote string, src fs.Object) (newDst fs.Object, err error) {
	tr := accounting.Stats(ctx).NewTransfer(src)
	defer func() {
		tr.Done(err)
	}()
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
	if !fs.Config.IgnoreChecksum {
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
			if fs.Config.MaxTransfer >= 0 && accounting.Stats(ctx).GetBytes() >= int64(fs.Config.MaxTransfer) {
				return nil, accounting.ErrorMaxTransferLimitReached
			}
			newDst, err = doCopy(ctx, src, remote)
			if err == nil {
				dst = newDst
				accounting.Stats(ctx).Bytes(dst.Size()) // account the bytes for the server side transfer
			}
		} else {
			err = fs.ErrorCantCopy
		}
		// If can't server side copy, do it manually
		if err == fs.ErrorCantCopy {
			if doMultiThreadCopy(f, src) {
				// Number of streams proportional to size
				streams := src.Size() / int64(fs.Config.MultiThreadCutoff)
				// With maximum
				if streams > int64(fs.Config.MultiThreadStreams) {
					streams = int64(fs.Config.MultiThreadStreams)
				}
				if streams < 2 {
					streams = 2
				}
				dst, err = multiThreadCopy(ctx, f, remote, src, int(streams), tr)
				if doUpdate {
					actionTaken = "Multi-thread Copied (replaced existing)"
				} else {
					actionTaken = "Multi-thread Copied (new)"
				}
			} else {
				var in0 io.ReadCloser
				in0, err = newReOpen(ctx, src, hashOption, nil, fs.Config.LowLevelRetries)
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
						// NB Rcat closes in0
						dst, err = Rcat(ctx, f, remote, in0, src.ModTime(ctx))
						newDst = dst
					} else {
						in := tr.Account(in0).WithBuffer() // account and buffer the transfer
						var wrappedSrc fs.ObjectInfo = src
						// We try to pass the original object if possible
						if src.Remote() != remote {
							wrappedSrc = &overrideRemoteObject{Object: src, remote: remote}
						}
						if doUpdate {
							actionTaken = "Copied (replaced existing)"
							err = dst.Update(ctx, in, wrappedSrc, hashOption)
						} else {
							actionTaken = "Copied (new)"
							dst, err = f.Put(ctx, in, wrappedSrc, hashOption)
						}
						closeErr := in.Close()
						if err == nil {
							newDst = dst
							err = closeErr
						}
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
		removeFailedCopy(ctx, dst)
		return newDst, err
	}

	// Verify hashes are the same after transfer - ignoring blank hashes
	if hashType != hash.None {
		// checkHashes has logged and counted errors
		equal, _, srcSum, dstSum, _ := checkHashes(ctx, src, dst, hashType)
		if !equal {
			err = errors.Errorf("corrupted on transfer: %v hash differ %q vs %q", hashType, srcSum, dstSum)
			fs.Errorf(dst, "%v", err)
			fs.CountError(err)
			removeFailedCopy(ctx, dst)
			return newDst, err
		}
	}

	fs.Infof(src, actionTaken)
	return newDst, err
}

// SameObject returns true if src and dst could be pointing to the
// same object.
func SameObject(src, dst fs.Object) bool {
	if !SameConfig(src.Fs(), dst.Fs()) {
		return false
	}
	srcPath := path.Join(src.Fs().Root(), src.Remote())
	dstPath := path.Join(dst.Fs().Root(), dst.Remote())
	if dst.Fs().Features().CaseInsensitive {
		srcPath = strings.ToLower(srcPath)
		dstPath = strings.ToLower(dstPath)
	}
	return srcPath == dstPath
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
func Move(ctx context.Context, fdst fs.Fs, dst fs.Object, remote string, src fs.Object) (newDst fs.Object, err error) {
	tr := accounting.Stats(ctx).NewCheckingTransfer(src)
	defer func() {
		tr.Done(err)
	}()
	newDst = dst
	if fs.Config.DryRun {
		fs.Logf(src, "Not moving as --dry-run")
		return newDst, nil
	}
	// See if we have Move available
	if doMove := fdst.Features().Move; doMove != nil && (SameConfig(src.Fs(), fdst) || (SameRemoteType(src.Fs(), fdst) && fdst.Features().ServerSideAcrossConfigs)) {
		// Delete destination if it exists and is not the same file as src (could be same file while seemingly different if the remote is case insensitive)
		if dst != nil && !SameObject(src, dst) {
			err = DeleteFile(ctx, dst)
			if err != nil {
				return newDst, err
			}
		}
		// Move dst <- src
		newDst, err = doMove(ctx, src, remote)
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
	newDst, err = Copy(ctx, fdst, dst, remote, src)
	if err != nil {
		fs.Errorf(src, "Not deleting source as copy failed: %v", err)
		return newDst, err
	}
	// Delete src if no error on copy
	return newDst, DeleteFile(ctx, src)
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
func DeleteFileWithBackupDir(ctx context.Context, dst fs.Object, backupDir fs.Fs) (err error) {
	tr := accounting.Stats(ctx).NewCheckingTransfer(dst)
	defer func() {
		tr.Done(err)
	}()
	numDeletes := accounting.Stats(ctx).Deletes(1)
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
		err = MoveBackupDir(ctx, backupDir, dst)
	} else {
		err = dst.Remove(ctx)
	}
	if err != nil {
		fs.CountError(err)
		fs.Errorf(dst, "Couldn't %s: %v", action, err)
	} else if !fs.Config.DryRun {
		fs.Infof(dst, actioned)
	}
	return err
}

// DeleteFile deletes a single file respecting --dry-run and accumulating stats and errors.
//
// If useBackupDir is set and --backup-dir is in effect then it moves
// the file to there instead of deleting
func DeleteFile(ctx context.Context, dst fs.Object) (err error) {
	return DeleteFileWithBackupDir(ctx, dst, nil)
}

// DeleteFilesWithBackupDir removes all the files passed in the
// channel
//
// If backupDir is set the files will be placed into that directory
// instead of being deleted.
func DeleteFilesWithBackupDir(ctx context.Context, toBeDeleted fs.ObjectsChan, backupDir fs.Fs) error {
	var wg sync.WaitGroup
	wg.Add(fs.Config.Transfers)
	var errorCount int32
	var fatalErrorCount int32

	for i := 0; i < fs.Config.Transfers; i++ {
		go func() {
			defer wg.Done()
			for dst := range toBeDeleted {
				err := DeleteFileWithBackupDir(ctx, dst, backupDir)
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
func DeleteFiles(ctx context.Context, toBeDeleted fs.ObjectsChan) error {
	return DeleteFilesWithBackupDir(ctx, toBeDeleted, nil)
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

// fixRoot returns the Root with a trailing / if not empty. It is
// aware of case insensitive filesystems.
func fixRoot(f fs.Info) string {
	s := strings.Trim(filepath.ToSlash(f.Root()), "/")
	if s != "" {
		s += "/"
	}
	if f.Features().CaseInsensitive {
		s = strings.ToLower(s)
	}
	return s
}

// Overlapping returns true if fdst and fsrc point to the same
// underlying Fs and they overlap.
func Overlapping(fdst, fsrc fs.Info) bool {
	if !SameConfig(fdst, fsrc) {
		return false
	}
	fdstRoot := fixRoot(fdst)
	fsrcRoot := fixRoot(fsrc)
	return strings.HasPrefix(fdstRoot, fsrcRoot) || strings.HasPrefix(fsrcRoot, fdstRoot)
}

// SameDir returns true if fdst and fsrc point to the same
// underlying Fs and they are the same directory.
func SameDir(fdst, fsrc fs.Info) bool {
	if !SameConfig(fdst, fsrc) {
		return false
	}
	fdstRoot := fixRoot(fdst)
	fsrcRoot := fixRoot(fsrc)
	return fdstRoot == fsrcRoot
}

// checkIdentical checks to see if dst and src are identical
//
// it returns true if differences were found
// it also returns whether it couldn't be hashed
func checkIdentical(ctx context.Context, dst, src fs.Object) (differ bool, noHash bool) {
	same, ht, err := CheckHashes(ctx, src, dst)
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
type checkFn func(ctx context.Context, a, b fs.Object) (differ bool, noHash bool)

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
func (c *checkMarch) checkIdentical(ctx context.Context, dst, src fs.Object) (differ bool, noHash bool) {
	var err error
	tr := accounting.Stats(ctx).NewCheckingTransfer(src)
	defer func() {
		tr.Done(err)
	}()
	if sizeDiffers(src, dst) {
		err = errors.Errorf("Sizes differ")
		fs.Errorf(src, "%v", err)
		fs.CountError(err)
		return true, false
	}
	if fs.Config.SizeOnly {
		return false, false
	}
	return c.check(ctx, dst, src)
}

// Match is called when src and dst are present, so sync src to dst
func (c *checkMarch) Match(ctx context.Context, dst, src fs.DirEntry) (recurse bool) {
	switch srcX := src.(type) {
	case fs.Object:
		dstX, ok := dst.(fs.Object)
		if ok {
			differ, noHash := c.checkIdentical(ctx, dstX, srcX)
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
func CheckFn(ctx context.Context, fdst, fsrc fs.Fs, check checkFn, oneway bool) error {
	c := &checkMarch{
		fdst:   fdst,
		fsrc:   fsrc,
		check:  check,
		oneway: oneway,
	}

	// set up a march over fdst and fsrc
	m := &march.March{
		Ctx:      ctx,
		Fdst:     fdst,
		Fsrc:     fsrc,
		Dir:      "",
		Callback: c,
	}
	fs.Infof(fdst, "Waiting for checks to finish")
	err := m.Run()

	if c.dstFilesMissing > 0 {
		fs.Logf(fdst, "%d files missing", c.dstFilesMissing)
	}
	if c.srcFilesMissing > 0 {
		fs.Logf(fsrc, "%d files missing", c.srcFilesMissing)
	}

	fs.Logf(fdst, "%d differences found", accounting.Stats(ctx).GetErrors())
	if c.noHashes > 0 {
		fs.Logf(fdst, "%d hashes could not be checked", c.noHashes)
	}
	if c.matches > 0 {
		fs.Logf(fdst, "%d matching files", c.matches)
	}
	if c.differences > 0 {
		return errors.Errorf("%d differences found", c.differences)
	}
	return err
}

// Check the files in fsrc and fdst according to Size and hash
func Check(ctx context.Context, fdst, fsrc fs.Fs, oneway bool) error {
	return CheckFn(ctx, fdst, fsrc, checkIdentical, oneway)
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
func CheckIdentical(ctx context.Context, dst, src fs.Object) (differ bool, err error) {
	in1, err := dst.Open(ctx)
	if err != nil {
		return true, errors.Wrapf(err, "failed to open %q", dst)
	}
	tr1 := accounting.Stats(ctx).NewTransfer(dst)
	defer func() {
		tr1.Done(err)
	}()
	in1 = tr1.Account(in1).WithBuffer() // account and buffer the transfer

	in2, err := src.Open(ctx)
	if err != nil {
		return true, errors.Wrapf(err, "failed to open %q", src)
	}
	tr2 := accounting.Stats(ctx).NewTransfer(dst)
	defer func() {
		tr2.Done(err)
	}()
	in2 = tr2.Account(in2).WithBuffer() // account and buffer the transfer

	// To assign err variable before defer.
	differ, err = CheckEqualReaders(in1, in2)
	return
}

// CheckDownload checks the files in fsrc and fdst according to Size
// and the actual contents of the files.
func CheckDownload(ctx context.Context, fdst, fsrc fs.Fs, oneway bool) error {
	check := func(ctx context.Context, a, b fs.Object) (differ bool, noHash bool) {
		differ, err := CheckIdentical(ctx, a, b)
		if err != nil {
			fs.CountError(err)
			fs.Errorf(a, "Failed to download: %v", err)
			return true, true
		}
		return differ, false
	}
	return CheckFn(ctx, fdst, fsrc, check, oneway)
}

// ListFn lists the Fs to the supplied function
//
// Lists in parallel which may get them out of order
func ListFn(ctx context.Context, f fs.Fs, fn func(fs.Object)) error {
	return walk.ListR(ctx, f, "", false, fs.Config.MaxDepth, walk.ListObjects, func(entries fs.DirEntries) error {
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
func List(ctx context.Context, f fs.Fs, w io.Writer) error {
	return ListFn(ctx, f, func(o fs.Object) {
		syncFprintf(w, "%9d %s\n", o.Size(), o.Remote())
	})
}

// ListLong lists the Fs to the supplied writer
//
// Shows size, mod time and path - obeys includes and excludes
//
// Lists in parallel which may get them out of order
func ListLong(ctx context.Context, f fs.Fs, w io.Writer) error {
	return ListFn(ctx, f, func(o fs.Object) {
		tr := accounting.Stats(ctx).NewCheckingTransfer(o)
		defer func() {
			tr.Done(nil)
		}()
		modTime := o.ModTime(ctx)
		syncFprintf(w, "%9d %s %s\n", o.Size(), modTime.Local().Format("2006-01-02 15:04:05.000000000"), o.Remote())
	})
}

// Md5sum list the Fs to the supplied writer
//
// Produces the same output as the md5sum command - obeys includes and
// excludes
//
// Lists in parallel which may get them out of order
func Md5sum(ctx context.Context, f fs.Fs, w io.Writer) error {
	return HashLister(ctx, hash.MD5, f, w)
}

// Sha1sum list the Fs to the supplied writer
//
// Obeys includes and excludes
//
// Lists in parallel which may get them out of order
func Sha1sum(ctx context.Context, f fs.Fs, w io.Writer) error {
	return HashLister(ctx, hash.SHA1, f, w)
}

// DropboxHashSum list the Fs to the supplied writer
//
// Obeys includes and excludes
//
// Lists in parallel which may get them out of order
func DropboxHashSum(ctx context.Context, f fs.Fs, w io.Writer) error {
	return HashLister(ctx, hash.Dropbox, f, w)
}

// hashSum returns the human readable hash for ht passed in.  This may
// be UNSUPPORTED or ERROR.
func hashSum(ctx context.Context, ht hash.Type, o fs.Object) string {
	var err error
	tr := accounting.Stats(ctx).NewCheckingTransfer(o)
	defer func() {
		tr.Done(err)
	}()
	sum, err := o.Hash(ctx, ht)
	if err == hash.ErrUnsupported {
		sum = "UNSUPPORTED"
	} else if err != nil {
		fs.Debugf(o, "Failed to read %v: %v", ht, err)
		sum = "ERROR"
	}
	return sum
}

// HashLister does a md5sum equivalent for the hash type passed in
func HashLister(ctx context.Context, ht hash.Type, f fs.Fs, w io.Writer) error {
	return ListFn(ctx, f, func(o fs.Object) {
		sum := hashSum(ctx, ht, o)
		syncFprintf(w, "%*s  %s\n", hash.Width[ht], sum, o.Remote())
	})
}

// Count counts the objects and their sizes in the Fs
//
// Obeys includes and excludes
func Count(ctx context.Context, f fs.Fs) (objects int64, size int64, err error) {
	err = ListFn(ctx, f, func(o fs.Object) {
		atomic.AddInt64(&objects, 1)
		objectSize := o.Size()
		if objectSize > 0 {
			atomic.AddInt64(&size, objectSize)
		}
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
func ListDir(ctx context.Context, f fs.Fs, w io.Writer) error {
	return walk.ListR(ctx, f, "", false, ConfigMaxDepth(false), walk.ListDirs, func(entries fs.DirEntries) error {
		entries.ForDir(func(dir fs.Directory) {
			if dir != nil {
				syncFprintf(w, "%12d %13s %9d %s\n", dir.Size(), dir.ModTime(ctx).Local().Format("2006-01-02 15:04:05"), dir.Items(), dir.Remote())
			}
		})
		return nil
	})
}

// Mkdir makes a destination directory or container
func Mkdir(ctx context.Context, f fs.Fs, dir string) error {
	if fs.Config.DryRun {
		fs.Logf(fs.LogDirName(f, dir), "Not making directory as dry run is set")
		return nil
	}
	fs.Debugf(fs.LogDirName(f, dir), "Making directory")
	err := f.Mkdir(ctx, dir)
	if err != nil {
		fs.CountError(err)
		return err
	}
	return nil
}

// TryRmdir removes a container but not if not empty.  It doesn't
// count errors but may return one.
func TryRmdir(ctx context.Context, f fs.Fs, dir string) error {
	if fs.Config.DryRun {
		fs.Logf(fs.LogDirName(f, dir), "Not deleting as dry run is set")
		return nil
	}
	fs.Debugf(fs.LogDirName(f, dir), "Removing directory")
	return f.Rmdir(ctx, dir)
}

// Rmdir removes a container but not if not empty
func Rmdir(ctx context.Context, f fs.Fs, dir string) error {
	err := TryRmdir(ctx, f, dir)
	if err != nil {
		fs.CountError(err)
		return err
	}
	return err
}

// Purge removes a directory and all of its contents
func Purge(ctx context.Context, f fs.Fs, dir string) error {
	doFallbackPurge := true
	var err error
	if dir == "" {
		// FIXME change the Purge interface so it takes a dir - see #1891
		if doPurge := f.Features().Purge; doPurge != nil {
			doFallbackPurge = false
			if fs.Config.DryRun {
				fs.Logf(f, "Not purging as --dry-run set")
			} else {
				err = doPurge(ctx)
				if err == fs.ErrorCantPurge {
					doFallbackPurge = true
				}
			}
		}
	}
	if doFallbackPurge {
		// DeleteFiles and Rmdir observe --dry-run
		err = DeleteFiles(ctx, listToChan(ctx, f, dir))
		if err != nil {
			return err
		}
		err = Rmdirs(ctx, f, dir, false)
	}
	if err != nil {
		fs.CountError(err)
		return err
	}
	return nil
}

// Delete removes all the contents of a container.  Unlike Purge, it
// obeys includes and excludes.
func Delete(ctx context.Context, f fs.Fs) error {
	delChan := make(fs.ObjectsChan, fs.Config.Transfers)
	delErr := make(chan error, 1)
	go func() {
		delErr <- DeleteFiles(ctx, delChan)
	}()
	err := ListFn(ctx, f, func(o fs.Object) {
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
func listToChan(ctx context.Context, f fs.Fs, dir string) fs.ObjectsChan {
	o := make(fs.ObjectsChan, fs.Config.Checkers)
	go func() {
		defer close(o)
		err := walk.ListR(ctx, f, dir, true, fs.Config.MaxDepth, walk.ListObjects, func(entries fs.DirEntries) error {
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
func CleanUp(ctx context.Context, f fs.Fs) error {
	doCleanUp := f.Features().CleanUp
	if doCleanUp == nil {
		return errors.Errorf("%v doesn't support cleanup", f)
	}
	if fs.Config.DryRun {
		fs.Logf(f, "Not running cleanup as --dry-run set")
		return nil
	}
	return doCleanUp(ctx)
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
func Cat(ctx context.Context, f fs.Fs, w io.Writer, offset, count int64) error {
	var mu sync.Mutex
	return ListFn(ctx, f, func(o fs.Object) {
		var err error
		tr := accounting.Stats(ctx).NewTransfer(o)
		defer func() {
			tr.Done(err)
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
		in, err := o.Open(ctx, options...)
		if err != nil {
			fs.CountError(err)
			fs.Errorf(o, "Failed to open: %v", err)
			return
		}
		if count >= 0 {
			in = &readCloser{Reader: &io.LimitedReader{R: in, N: count}, Closer: in}
		}
		in = tr.Account(in).WithBuffer() // account and buffer the transfer
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
func Rcat(ctx context.Context, fdst fs.Fs, dstFileName string, in io.ReadCloser, modTime time.Time) (dst fs.Object, err error) {
	tr := accounting.Stats(ctx).NewTransferRemoteSize(dstFileName, -1)
	defer func() {
		tr.Done(err)
	}()
	in = tr.Account(in).WithBuffer()

	hashes := hash.NewHashSet(fdst.Hashes().GetOne()) // just pick one hash
	hashOption := &fs.HashesOption{Hashes: hashes}
	hash, err := hash.NewMultiHasherTypes(hashes)
	if err != nil {
		return nil, err
	}
	readCounter := readers.NewCountingReader(in)
	trackingIn := io.TeeReader(readCounter, hash)

	compare := func(dst fs.Object) error {
		src := object.NewStaticObjectInfo(dstFileName, modTime, int64(readCounter.BytesRead()), false, hash.Sums(), fdst)
		if !Equal(ctx, src, dst) {
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
		return Copy(ctx, fdst, nil, dstFileName, src)
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
			err := Purge(ctx, tmpLocalFs, "")
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
	if dst, err = fStreamTo.Features().PutStream(ctx, in, objInfo, hashOption); err != nil {
		return dst, err
	}
	if err = compare(dst); err != nil {
		return dst, err
	}
	if !canStream {
		// copy dst (which is the local object we have just streamed to) to the remote
		return Copy(ctx, fdst, nil, dstFileName, dst)
	}
	return dst, nil
}

// PublicLink adds a "readable by anyone with link" permission on the given file or folder.
func PublicLink(ctx context.Context, f fs.Fs, remote string) (string, error) {
	doPublicLink := f.Features().PublicLink
	if doPublicLink == nil {
		return "", errors.Errorf("%v doesn't support public links", f)
	}
	return doPublicLink(ctx, remote)
}

// Rmdirs removes any empty directories (or directories only
// containing empty directories) under f, including f.
func Rmdirs(ctx context.Context, f fs.Fs, dir string, leaveRoot bool) error {
	dirEmpty := make(map[string]bool)
	dirEmpty[dir] = !leaveRoot
	err := walk.Walk(ctx, f, dir, true, fs.Config.MaxDepth, func(dirPath string, entries fs.DirEntries, err error) error {
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
		err := TryRmdir(ctx, f, dir)
		if err != nil {
			fs.CountError(err)
			fs.Errorf(dir, "Failed to rmdir: %v", err)
			return err
		}
	}
	return nil
}

// GetCompareDest sets up --compare-dest
func GetCompareDest() (CompareDest fs.Fs, err error) {
	CompareDest, err = cache.Get(fs.Config.CompareDest)
	if err != nil {
		return nil, fserrors.FatalError(errors.Errorf("Failed to make fs for --compare-dest %q: %v", fs.Config.CompareDest, err))
	}
	return CompareDest, nil
}

// compareDest checks --compare-dest to see if src needs to
// be copied
//
// Returns True if src is in --compare-dest
func compareDest(ctx context.Context, dst, src fs.Object, CompareDest fs.Fs) (NoNeedTransfer bool, err error) {
	var remote string
	if dst == nil {
		remote = src.Remote()
	} else {
		remote = dst.Remote()
	}
	CompareDestFile, err := CompareDest.NewObject(ctx, remote)
	switch err {
	case fs.ErrorObjectNotFound:
		return false, nil
	case nil:
		break
	default:
		return false, err
	}
	if Equal(ctx, src, CompareDestFile) {
		fs.Debugf(src, "Destination found in --compare-dest, skipping")
		return true, nil
	}
	return false, nil
}

// GetCopyDest sets up --copy-dest
func GetCopyDest(fdst fs.Fs) (CopyDest fs.Fs, err error) {
	CopyDest, err = cache.Get(fs.Config.CopyDest)
	if err != nil {
		return nil, fserrors.FatalError(errors.Errorf("Failed to make fs for --copy-dest %q: %v", fs.Config.CopyDest, err))
	}
	if !SameConfig(fdst, CopyDest) {
		return nil, fserrors.FatalError(errors.New("parameter to --copy-dest has to be on the same remote as destination"))
	}
	if CopyDest.Features().Copy == nil {
		return nil, fserrors.FatalError(errors.New("can't use --copy-dest on a remote which doesn't support server side copy"))
	}
	return CopyDest, nil
}

// copyDest checks --copy-dest to see if src needs to
// be copied
//
// Returns True if src was copied from --copy-dest
func copyDest(ctx context.Context, fdst fs.Fs, dst, src fs.Object, CopyDest, backupDir fs.Fs) (NoNeedTransfer bool, err error) {
	var remote string
	if dst == nil {
		remote = src.Remote()
	} else {
		remote = dst.Remote()
	}
	CopyDestFile, err := CopyDest.NewObject(ctx, remote)
	switch err {
	case fs.ErrorObjectNotFound:
		return false, nil
	case nil:
		break
	default:
		return false, err
	}
	if equal(ctx, src, CopyDestFile, fs.Config.SizeOnly, fs.Config.CheckSum, false) {
		if dst == nil || !Equal(ctx, src, dst) {
			if dst != nil && backupDir != nil {
				err = MoveBackupDir(ctx, backupDir, dst)
				if err != nil {
					return false, errors.Wrap(err, "moving to --backup-dir failed")
				}
				// If successful zero out the dstObj as it is no longer there
				dst = nil
			}
			_, err := Copy(ctx, fdst, dst, remote, CopyDestFile)
			if err != nil {
				fs.Errorf(src, "Destination found in --copy-dest, error copying")
				return false, nil
			}
			fs.Debugf(src, "Destination found in --copy-dest, using server side copy")
			return true, nil
		}
		fs.Debugf(src, "Unchanged skipping")
		return true, nil
	}
	fs.Debugf(src, "Destination not found in --copy-dest")
	return false, nil
}

// CompareOrCopyDest checks --compare-dest and --copy-dest to see if src
// does not need to be copied
//
// Returns True if src does not need to be copied
func CompareOrCopyDest(ctx context.Context, fdst fs.Fs, dst, src fs.Object, CompareOrCopyDest, backupDir fs.Fs) (NoNeedTransfer bool, err error) {
	if fs.Config.CompareDest != "" {
		return compareDest(ctx, dst, src, CompareOrCopyDest)
	} else if fs.Config.CopyDest != "" {
		return copyDest(ctx, fdst, dst, src, CompareOrCopyDest, backupDir)
	}
	return false, nil
}

// NeedTransfer checks to see if src needs to be copied to dst using
// the current config.
//
// Returns a flag which indicates whether the file needs to be
// transferred or not.
func NeedTransfer(ctx context.Context, dst, src fs.Object) bool {
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
		srcModTime := src.ModTime(ctx)
		dstModTime := dst.ModTime(ctx)
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
			if !sizeDiffers(src, dst) {
				fs.Debugf(src, "Destination mod time is within %v of source and sizes identical, skipping", modifyWindow)
				return false
			}
			fs.Debugf(src, "Destination mod time is within %v of source but sizes differ, transferring", modifyWindow)
		}
	} else {
		// Check to see if changed or not
		if Equal(ctx, src, dst) {
			fs.Debugf(src, "Unchanged skipping")
			return false
		}
	}
	return true
}

// RcatSize reads data from the Reader until EOF and uploads it to a file on remote.
// Pass in size >=0 if known, <0 if not known
func RcatSize(ctx context.Context, fdst fs.Fs, dstFileName string, in io.ReadCloser, size int64, modTime time.Time) (dst fs.Object, err error) {
	var obj fs.Object

	if size >= 0 {
		var err error
		// Size known use Put
		tr := accounting.Stats(ctx).NewTransferRemoteSize(dstFileName, size)
		defer func() {
			tr.Done(err)
		}()
		body := ioutil.NopCloser(in) // we let the server close the body
		in := tr.Account(body)       // account the transfer (no buffering)

		if fs.Config.DryRun {
			fs.Logf("stdin", "Not uploading as --dry-run")
			// prevents "broken pipe" errors
			_, err = io.Copy(ioutil.Discard, in)
			return nil, err
		}

		info := object.NewStaticObjectInfo(dstFileName, modTime, size, true, nil, fdst)
		obj, err = fdst.Put(ctx, in, info)
		if err != nil {
			fs.Errorf(dstFileName, "Post request put error: %v", err)

			return nil, err
		}
	} else {
		// Size unknown use Rcat
		obj, err = Rcat(ctx, fdst, dstFileName, in, modTime)
		if err != nil {
			fs.Errorf(dstFileName, "Post request rcat error: %v", err)

			return nil, err
		}
	}

	return obj, nil
}

// CopyURL copies the data from the url to (fdst, dstFileName)
func CopyURL(ctx context.Context, fdst fs.Fs, dstFileName string, url string) (dst fs.Object, err error) {
	client := fshttp.NewClient(fs.Config)
	resp, err := client.Get(url)
	if err != nil {
		return nil, err
	}
	defer fs.CheckClose(resp.Body, &err)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, errors.Errorf("CopyURL failed: %s", resp.Status)
	}
	return RcatSize(ctx, fdst, dstFileName, resp.Body, resp.ContentLength, time.Now())
}

// BackupDir returns the correctly configured --backup-dir
func BackupDir(fdst fs.Fs, fsrc fs.Fs, srcFileName string) (backupDir fs.Fs, err error) {
	if fs.Config.BackupDir != "" {
		backupDir, err = cache.Get(fs.Config.BackupDir)
		if err != nil {
			return nil, fserrors.FatalError(errors.Errorf("Failed to make fs for --backup-dir %q: %v", fs.Config.BackupDir, err))
		}
		if !SameConfig(fdst, backupDir) {
			return nil, fserrors.FatalError(errors.New("parameter to --backup-dir has to be on the same remote as destination"))
		}
		if srcFileName == "" {
			if Overlapping(fdst, backupDir) {
				return nil, fserrors.FatalError(errors.New("destination and parameter to --backup-dir mustn't overlap"))
			}
			if Overlapping(fsrc, backupDir) {
				return nil, fserrors.FatalError(errors.New("source and parameter to --backup-dir mustn't overlap"))
			}
		} else {
			if fs.Config.Suffix == "" {
				if SameDir(fdst, backupDir) {
					return nil, fserrors.FatalError(errors.New("destination and parameter to --backup-dir mustn't be the same"))
				}
				if SameDir(fsrc, backupDir) {
					return nil, fserrors.FatalError(errors.New("source and parameter to --backup-dir mustn't be the same"))
				}
			}
		}
	} else {
		if srcFileName == "" {
			return nil, fserrors.FatalError(errors.New("--suffix must be used with a file or with --backup-dir"))
		}
		// --backup-dir is not set but --suffix is - use the destination as the backupDir
		backupDir = fdst
	}
	if !CanServerSideMove(backupDir) {
		return nil, fserrors.FatalError(errors.New("can't use --backup-dir on a remote which doesn't support server side move or copy"))
	}
	return backupDir, nil
}

// MoveBackupDir moves a file to the backup dir
func MoveBackupDir(ctx context.Context, backupDir fs.Fs, dst fs.Object) (err error) {
	remoteWithSuffix := SuffixName(dst.Remote())
	overwritten, _ := backupDir.NewObject(ctx, remoteWithSuffix)
	_, err = Move(ctx, backupDir, overwritten, remoteWithSuffix, dst)
	return err
}

// moveOrCopyFile moves or copies a single file possibly to a new name
func moveOrCopyFile(ctx context.Context, fdst fs.Fs, fsrc fs.Fs, dstFileName string, srcFileName string, cp bool) (err error) {
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
	srcObj, err := fsrc.NewObject(ctx, srcFileName)
	if err != nil {
		return err
	}

	// Find dst object if it exists
	dstObj, err := fdst.NewObject(ctx, dstFileName)
	if err == fs.ErrorObjectNotFound {
		dstObj = nil
	} else if err != nil {
		return err
	}

	// Special case for changing case of a file on a case insensitive remote
	// This will move the file to a temporary name then
	// move it back to the intended destination. This is required
	// to avoid issues with certain remotes and avoid file deletion.
	if !cp && fdst.Name() == fsrc.Name() && fdst.Features().CaseInsensitive && dstFileName != srcFileName && strings.ToLower(dstFilePath) == strings.ToLower(srcFilePath) {
		// Create random name to temporarily move file to
		tmpObjName := dstFileName + "-rclone-move-" + random.String(8)
		_, err := fdst.NewObject(ctx, tmpObjName)
		if err != fs.ErrorObjectNotFound {
			if err == nil {
				return errors.New("found an already existing file with a randomly generated name. Try the operation again")
			}
			return errors.Wrap(err, "error while attempting to move file to a temporary location")
		}
		tr := accounting.Stats(ctx).NewTransfer(srcObj)
		defer func() {
			tr.Done(err)
		}()
		tmpObj, err := Op(ctx, fdst, nil, tmpObjName, srcObj)
		if err != nil {
			return errors.Wrap(err, "error while moving file to temporary location")
		}
		_, err = Op(ctx, fdst, nil, dstFileName, tmpObj)
		return err
	}

	var backupDir, copyDestDir fs.Fs
	if fs.Config.BackupDir != "" || fs.Config.Suffix != "" {
		backupDir, err = BackupDir(fdst, fsrc, srcFileName)
		if err != nil {
			return errors.Wrap(err, "creating Fs for --backup-dir failed")
		}
	}
	if fs.Config.CompareDest != "" {
		copyDestDir, err = GetCompareDest()
		if err != nil {
			return err
		}
	} else if fs.Config.CopyDest != "" {
		copyDestDir, err = GetCopyDest(fdst)
		if err != nil {
			return err
		}
	}
	NoNeedTransfer, err := CompareOrCopyDest(ctx, fdst, dstObj, srcObj, copyDestDir, backupDir)
	if err != nil {
		return err
	}
	if !NoNeedTransfer && NeedTransfer(ctx, dstObj, srcObj) {
		// If destination already exists, then we must move it into --backup-dir if required
		if dstObj != nil && backupDir != nil {
			err = MoveBackupDir(ctx, backupDir, dstObj)
			if err != nil {
				return errors.Wrap(err, "moving to --backup-dir failed")
			}
			// If successful zero out the dstObj as it is no longer there
			dstObj = nil
		}

		_, err = Op(ctx, fdst, dstObj, dstFileName, srcObj)
	} else {
		tr := accounting.Stats(ctx).NewCheckingTransfer(srcObj)
		if !cp {
			err = DeleteFile(ctx, srcObj)
		}
		tr.Done(err)
	}
	return err
}

// MoveFile moves a single file possibly to a new name
func MoveFile(ctx context.Context, fdst fs.Fs, fsrc fs.Fs, dstFileName string, srcFileName string) (err error) {
	return moveOrCopyFile(ctx, fdst, fsrc, dstFileName, srcFileName, false)
}

// CopyFile moves a single file possibly to a new name
func CopyFile(ctx context.Context, fdst fs.Fs, fsrc fs.Fs, dstFileName string, srcFileName string) (err error) {
	return moveOrCopyFile(ctx, fdst, fsrc, dstFileName, srcFileName, true)
}

// SetTier changes tier of object in remote
func SetTier(ctx context.Context, fsrc fs.Fs, tier string) error {
	return ListFn(ctx, fsrc, func(o fs.Object) {
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

// AddTier adds file's Tier to the output if known
func (l *ListFormat) AddTier() {
	l.AppendOutput(func(entry *ListJSONItem) string {
		return entry.Tier
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
func DirMove(ctx context.Context, f fs.Fs, srcRemote, dstRemote string) (err error) {
	// Use DirMove if possible
	if doDirMove := f.Features().DirMove; doDirMove != nil {
		return doDirMove(ctx, f, srcRemote, dstRemote)
	}

	// Load the directory tree into memory
	tree, err := walk.NewDirTree(ctx, f, srcRemote, true, -1)
	if err != nil {
		return errors.Wrap(err, "RenameDir tree walk")
	}

	// Get the directories in sorted order
	dirs := tree.Dirs()

	// Make the destination directories - must be done in order not in parallel
	for _, dir := range dirs {
		dstPath := dstRemote + dir[len(srcRemote):]
		err := f.Mkdir(ctx, dstPath)
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
	g, gCtx := errgroup.WithContext(context.Background())
	for i := 0; i < fs.Config.Transfers; i++ {
		g.Go(func() error {
			for job := range renames {
				dstOverwritten, _ := f.NewObject(gCtx, job.newPath)
				_, err := Move(gCtx, f, dstOverwritten, job.newPath, job.o)
				if err != nil {
					return err
				}
				select {
				case <-gCtx.Done():
					return gCtx.Err()
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
		err := f.Rmdir(ctx, dirs[i])
		if err != nil {
			return errors.Wrap(err, "RenameDir rmdir")
		}
	}

	return nil
}

// FsInfo provides information about a remote
type FsInfo struct {
	// Name of the remote (as passed into NewFs)
	Name string

	// Root of the remote (as passed into NewFs)
	Root string

	// String returns a description of the FS
	String string

	// Precision of the ModTimes in this Fs in Nanoseconds
	Precision time.Duration

	// Returns the supported hash types of the filesystem
	Hashes []string

	// Features returns the optional features of this Fs
	Features map[string]bool
}

// GetFsInfo gets the information (FsInfo) about a given Fs
func GetFsInfo(f fs.Fs) *FsInfo {
	info := &FsInfo{
		Name:      f.Name(),
		Root:      f.Root(),
		String:    f.String(),
		Precision: f.Precision(),
		Hashes:    make([]string, 0, 4),
		Features:  f.Features().Enabled(),
	}
	for _, hashType := range f.Hashes().Array() {
		info.Hashes = append(info.Hashes, hashType.String())
	}
	return info
}
