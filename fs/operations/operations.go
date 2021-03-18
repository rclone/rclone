// Package operations does generic operations on filesystems and objects
package operations

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/csv"
	"encoding/hex"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
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
	"github.com/rclone/rclone/fs/config"
	"github.com/rclone/rclone/fs/filter"
	"github.com/rclone/rclone/fs/fserrors"
	"github.com/rclone/rclone/fs/fshttp"
	"github.com/rclone/rclone/fs/hash"
	"github.com/rclone/rclone/fs/object"
	"github.com/rclone/rclone/fs/walk"
	"github.com/rclone/rclone/lib/atexit"
	"github.com/rclone/rclone/lib/pacer"
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
			err = fs.CountError(err)
			fs.Errorf(src, "Failed to calculate src hash: %v", err)
		}
		return err
	})
	g.Go(func() (err error) {
		dstHash, err = dst.Hash(ctx, ht)
		if err != nil {
			err = fs.CountError(err)
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
	return equal(ctx, src, dst, defaultEqualOpt(ctx))
}

// sizeDiffers compare the size of src and dst taking into account the
// various ways of ignoring sizes
func sizeDiffers(ctx context.Context, src, dst fs.ObjectInfo) bool {
	ci := fs.GetConfig(ctx)
	if ci.IgnoreSize || src.Size() < 0 || dst.Size() < 0 {
		return false
	}
	return src.Size() != dst.Size()
}

var checksumWarning sync.Once

// options for equal function()
type equalOpt struct {
	sizeOnly          bool // if set only check size
	checkSum          bool // if set check checksum+size instead of modtime+size
	updateModTime     bool // if set update the modtime if hashes identical and checking with modtime+size
	forceModTimeMatch bool // if set assume modtimes match
}

// default set of options for equal()
func defaultEqualOpt(ctx context.Context) equalOpt {
	ci := fs.GetConfig(ctx)
	return equalOpt{
		sizeOnly:          ci.SizeOnly,
		checkSum:          ci.CheckSum,
		updateModTime:     !ci.NoUpdateModTime,
		forceModTimeMatch: false,
	}
}

var modTimeUploadOnce sync.Once

// emit a log if we are about to upload a file to set its modification time
func logModTimeUpload(dst fs.Object) {
	modTimeUploadOnce.Do(func() {
		fs.Logf(dst.Fs(), "Forced to upload files to set modification times on this backend.")
	})
}

func equal(ctx context.Context, src fs.ObjectInfo, dst fs.Object, opt equalOpt) bool {
	ci := fs.GetConfig(ctx)
	if sizeDiffers(ctx, src, dst) {
		fs.Debugf(src, "Sizes differ (src %d vs dst %d)", src.Size(), dst.Size())
		return false
	}
	if opt.sizeOnly {
		fs.Debugf(src, "Sizes identical")
		return true
	}

	// Assert: Size is equal or being ignored

	// If checking checksum and not modtime
	if opt.checkSum {
		// Check the hash
		same, ht, _ := CheckHashes(ctx, src, dst)
		if !same {
			fs.Debugf(src, "%v differ", ht)
			return false
		}
		if ht == hash.None {
			common := src.Fs().Hashes().Overlap(dst.Fs().Hashes())
			if common.Count() == 0 {
				checksumWarning.Do(func() {
					fs.Logf(dst.Fs(), "--checksum is in use but the source and destination have no hashes in common; falling back to --size-only")
				})
			}
			fs.Debugf(src, "Size of src and dst objects identical")
		} else {
			fs.Debugf(src, "Size and %v of src and dst objects identical", ht)
		}
		return true
	}

	srcModTime := src.ModTime(ctx)
	if !opt.forceModTimeMatch {
		// Sizes the same so check the mtime
		modifyWindow := fs.GetModifyWindow(ctx, src.Fs(), dst.Fs())
		if modifyWindow == fs.ModTimeNotSupported {
			fs.Debugf(src, "Sizes identical")
			return true
		}
		dstModTime := dst.ModTime(ctx)
		dt := dstModTime.Sub(srcModTime)
		if dt < modifyWindow && dt > -modifyWindow {
			fs.Debugf(src, "Size and modification time the same (differ by %s, within tolerance %s)", dt, modifyWindow)
			return true
		}

		fs.Debugf(src, "Modification times differ by %s: %v, %v", dt, srcModTime, dstModTime)
	}

	// Check if the hashes are the same
	same, ht, _ := CheckHashes(ctx, src, dst)
	if !same {
		fs.Debugf(src, "%v differ", ht)
		return false
	}
	if ht == hash.None && !ci.RefreshTimes {
		// if couldn't check hash, return that they differ
		return false
	}

	// mod time differs but hash is the same to reset mod time if required
	if opt.updateModTime {
		if !SkipDestructive(ctx, src, "update modification time") {
			// Size and hash the same but mtime different
			// Error if objects are treated as immutable
			if ci.Immutable {
				fs.Errorf(dst, "Timestamp mismatch between immutable objects")
				return false
			}
			// Update the mtime of the dst object here
			err := dst.SetModTime(ctx, srcModTime)
			if err == fs.ErrorCantSetModTime {
				logModTimeUpload(dst)
				fs.Infof(dst, "src and dst identical but can't set mod time without re-uploading")
				return false
			} else if err == fs.ErrorCantSetModTimeWithoutDelete {
				logModTimeUpload(dst)
				fs.Infof(dst, "src and dst identical but can't set mod time without deleting and re-uploading")
				// Remove the file if BackupDir isn't set.  If BackupDir is set we would rather have the old file
				// put in the BackupDir than deleted which is what will happen if we don't delete it.
				if ci.BackupDir == "" {
					err = dst.Remove(ctx)
					if err != nil {
						fs.Errorf(dst, "failed to delete before re-upload: %v", err)
					}
				}
				return false
			} else if err != nil {
				err = fs.CountError(err)
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

// OverrideRemote is a wrapper to override the Remote for an
// ObjectInfo
type OverrideRemote struct {
	fs.ObjectInfo
	remote string
}

// NewOverrideRemote returns an OverrideRemoteObject which will
// return the remote specified
func NewOverrideRemote(oi fs.ObjectInfo, remote string) *OverrideRemote {
	return &OverrideRemote{
		ObjectInfo: oi,
		remote:     remote,
	}
}

// Remote returns the overridden remote name
func (o *OverrideRemote) Remote() string {
	return o.remote
}

// MimeType returns the mime type of the underlying object or "" if it
// can't be worked out
func (o *OverrideRemote) MimeType(ctx context.Context) string {
	if do, ok := o.ObjectInfo.(fs.MimeTyper); ok {
		return do.MimeType(ctx)
	}
	return ""
}

// ID returns the ID of the Object if known, or "" if not
func (o *OverrideRemote) ID() string {
	if do, ok := o.ObjectInfo.(fs.IDer); ok {
		return do.ID()
	}
	return ""
}

// UnWrap returns the Object that this Object is wrapping or nil if it
// isn't wrapping anything
func (o *OverrideRemote) UnWrap() fs.Object {
	if o, ok := o.ObjectInfo.(fs.Object); ok {
		return o
	}
	return nil
}

// GetTier returns storage tier or class of the Object
func (o *OverrideRemote) GetTier() string {
	if do, ok := o.ObjectInfo.(fs.GetTierer); ok {
		return do.GetTier()
	}
	return ""
}

// Check all optional interfaces satisfied
var _ fs.FullObjectInfo = (*OverrideRemote)(nil)

// CommonHash returns a single hash.Type and a HashOption with that
// type which is in common between the two fs.Fs.
func CommonHash(ctx context.Context, fa, fb fs.Info) (hash.Type, *fs.HashesOption) {
	ci := fs.GetConfig(ctx)
	// work out which hash to use - limit to 1 hash in common
	var common hash.Set
	hashType := hash.None
	if !ci.IgnoreChecksum {
		common = fb.Hashes().Overlap(fa.Hashes())
		if common.Count() > 0 {
			hashType = common.GetOne()
			common = hash.Set(hashType)
		}
	}
	return hashType, &fs.HashesOption{Hashes: common}
}

// Copy src object to dst or f if nil.  If dst is nil then it uses
// remote as the name of the new object.
//
// It returns the destination object if possible.  Note that this may
// be nil.
func Copy(ctx context.Context, f fs.Fs, dst fs.Object, remote string, src fs.Object) (newDst fs.Object, err error) {
	ci := fs.GetConfig(ctx)
	tr := accounting.Stats(ctx).NewTransfer(src)
	defer func() {
		tr.Done(ctx, err)
	}()
	newDst = dst
	if SkipDestructive(ctx, src, "copy") {
		in := tr.Account(ctx, nil)
		in.DryRun(src.Size())
		return newDst, nil
	}
	maxTries := ci.LowLevelRetries
	tries := 0
	doUpdate := dst != nil
	hashType, hashOption := CommonHash(ctx, f, src.Fs())

	var actionTaken string
	for {
		// Try server-side copy first - if has optional interface and
		// is same underlying remote
		actionTaken = "Copied (server-side copy)"
		if ci.MaxTransfer >= 0 {
			var bytesSoFar int64
			if ci.CutoffMode == fs.CutoffModeCautious {
				bytesSoFar = accounting.Stats(ctx).GetBytesWithPending() + src.Size()
			} else {
				bytesSoFar = accounting.Stats(ctx).GetBytes()
			}
			if bytesSoFar >= int64(ci.MaxTransfer) {
				if ci.CutoffMode == fs.CutoffModeHard {
					return nil, accounting.ErrorMaxTransferLimitReachedFatal
				}
				return nil, accounting.ErrorMaxTransferLimitReachedGraceful
			}
		}
		if doCopy := f.Features().Copy; doCopy != nil && (SameConfig(src.Fs(), f) || (SameRemoteType(src.Fs(), f) && f.Features().ServerSideAcrossConfigs)) {
			in := tr.Account(ctx, nil) // account the transfer
			in.ServerSideCopyStart()
			newDst, err = doCopy(ctx, src, remote)
			if err == nil {
				dst = newDst
				in.ServerSideCopyEnd(dst.Size()) // account the bytes for the server-side transfer
				err = in.Close()
			} else {
				_ = in.Close()
			}
			if err == fs.ErrorCantCopy {
				tr.Reset(ctx) // skip incomplete accounting - will be overwritten by the manual copy below
			}
		} else {
			err = fs.ErrorCantCopy
		}
		// If can't server-side copy, do it manually
		if err == fs.ErrorCantCopy {
			if doMultiThreadCopy(ctx, f, src) {
				// Number of streams proportional to size
				streams := src.Size() / int64(ci.MultiThreadCutoff)
				// With maximum
				if streams > int64(ci.MultiThreadStreams) {
					streams = int64(ci.MultiThreadStreams)
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
				options := []fs.OpenOption{hashOption}
				for _, option := range ci.DownloadHeaders {
					options = append(options, option)
				}
				in0, err = NewReOpen(ctx, src, ci.LowLevelRetries, options...)
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
						in := tr.Account(ctx, in0).WithBuffer() // account and buffer the transfer
						var wrappedSrc fs.ObjectInfo = src
						// We try to pass the original object if possible
						if src.Remote() != remote {
							wrappedSrc = NewOverrideRemote(src, remote)
						}
						options := []fs.OpenOption{hashOption}
						for _, option := range ci.UploadHeaders {
							options = append(options, option)
						}
						if doUpdate {
							actionTaken = "Copied (replaced existing)"
							err = dst.Update(ctx, in, wrappedSrc, options...)
						} else {
							actionTaken = "Copied (new)"
							dst, err = f.Put(ctx, in, wrappedSrc, options...)
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
		if fserrors.ContextError(ctx, &err) {
			break
		}
		var retry bool
		if fserrors.IsRetryError(err) || fserrors.ShouldRetry(err) {
			retry = true
		} else if t, ok := pacer.IsRetryAfter(err); ok {
			fs.Debugf(src, "Sleeping for %v (as indicated by the server) to obey Retry-After error: %v", t, err)
			time.Sleep(t)
			retry = true
		}
		if retry {
			fs.Debugf(src, "Received error: %v - low level retry %d/%d", err, tries, maxTries)
			tr.Reset(ctx) // skip incomplete accounting - will be overwritten by retry
			continue
		}
		// otherwise finish
		break
	}
	if err != nil {
		err = fs.CountError(err)
		fs.Errorf(src, "Failed to copy: %v", err)
		return newDst, err
	}

	// Verify sizes are the same after transfer
	if sizeDiffers(ctx, src, dst) {
		err = errors.Errorf("corrupted on transfer: sizes differ %d vs %d", src.Size(), dst.Size())
		fs.Errorf(dst, "%v", err)
		err = fs.CountError(err)
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
			err = fs.CountError(err)
			removeFailedCopy(ctx, dst)
			return newDst, err
		}
	}
	if newDst != nil && src.String() != newDst.String() {
		fs.Infof(src, "%s to: %s", actionTaken, newDst.String())
	} else {
		fs.Infof(src, actionTaken)
	}
	return newDst, err
}

// SameObject returns true if src and dst could be pointing to the
// same object.
func SameObject(src, dst fs.Object) bool {
	srcFs, dstFs := src.Fs(), dst.Fs()
	if !SameConfig(srcFs, dstFs) {
		// If same remote type then check ID of objects if available
		doSrcID, srcIDOK := src.(fs.IDer)
		doDstID, dstIDOK := dst.(fs.IDer)
		if srcIDOK && dstIDOK && SameRemoteType(srcFs, dstFs) {
			srcID, dstID := doSrcID.ID(), doDstID.ID()
			if srcID != "" && srcID == dstID {
				return true
			}
		}
		return false
	}
	srcPath := path.Join(srcFs.Root(), src.Remote())
	dstPath := path.Join(dstFs.Root(), dst.Remote())
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
		if err == nil {
			accounting.Stats(ctx).Renames(1)
		}
		tr.Done(ctx, err)
	}()
	newDst = dst
	if SkipDestructive(ctx, src, "move") {
		in := tr.Account(ctx, nil)
		in.DryRun(src.Size())
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
			if newDst != nil && src.String() != newDst.String() {
				fs.Infof(src, "Moved (server-side) to: %s", newDst.String())
			} else {
				fs.Infof(src, "Moved (server-side)")
			}

			return newDst, nil
		case fs.ErrorCantMove:
			fs.Debugf(src, "Can't move, switching to copy")
		default:
			err = fs.CountError(err)
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

// CanServerSideMove returns true if fdst support server-side moves or
// server-side copies
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
func SuffixName(ctx context.Context, remote string) string {
	ci := fs.GetConfig(ctx)
	if ci.Suffix == "" {
		return remote
	}
	if ci.SuffixKeepExtension {
		ext := path.Ext(remote)
		base := remote[:len(remote)-len(ext)]
		return base + ci.Suffix + ext
	}
	return remote + ci.Suffix
}

// DeleteFileWithBackupDir deletes a single file respecting --dry-run
// and accumulating stats and errors.
//
// If backupDir is set then it moves the file to there instead of
// deleting
func DeleteFileWithBackupDir(ctx context.Context, dst fs.Object, backupDir fs.Fs) (err error) {
	ci := fs.GetConfig(ctx)
	tr := accounting.Stats(ctx).NewCheckingTransfer(dst)
	defer func() {
		tr.Done(ctx, err)
	}()
	numDeletes := accounting.Stats(ctx).Deletes(1)
	if ci.MaxDelete != -1 && numDeletes > ci.MaxDelete {
		return fserrors.FatalError(errors.New("--max-delete threshold reached"))
	}
	action, actioned := "delete", "Deleted"
	if backupDir != nil {
		action, actioned = "move into backup dir", "Moved into backup dir"
	}
	skip := SkipDestructive(ctx, dst, action)
	if skip {
		// do nothing
	} else if backupDir != nil {
		err = MoveBackupDir(ctx, backupDir, dst)
	} else {
		err = dst.Remove(ctx)
	}
	if err != nil {
		fs.Errorf(dst, "Couldn't %s: %v", action, err)
		err = fs.CountError(err)
	} else if !skip {
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
	ci := fs.GetConfig(ctx)
	wg.Add(ci.Transfers)
	var errorCount int32
	var fatalErrorCount int32

	for i := 0; i < ci.Transfers; i++ {
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
	fs.Debugf(nil, "Waiting for deletions to finish")
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

// SameConfigArr returns true if any of []fsrcs has same config file entry with fdst
func SameConfigArr(fdst fs.Info, fsrcs []fs.Fs) bool {
	for _, fsrc := range fsrcs {
		if fdst.Name() == fsrc.Name() {
			return true
		}
	}
	return false
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

// Retry runs fn up to maxTries times if it returns a retriable error
func Retry(ctx context.Context, o interface{}, maxTries int, fn func() error) (err error) {
	for tries := 1; tries <= maxTries; tries++ {
		// Call the function which might error
		err = fn()
		if err == nil {
			break
		}
		// Retry if err returned a retry error
		if fserrors.ContextError(ctx, &err) {
			break
		}
		if fserrors.IsRetryError(err) || fserrors.ShouldRetry(err) {
			fs.Debugf(o, "Received error: %v - low level retry %d/%d", err, tries, maxTries)
			continue
		}
		break
	}
	return err
}

// ListFn lists the Fs to the supplied function
//
// Lists in parallel which may get them out of order
func ListFn(ctx context.Context, f fs.Fs, fn func(fs.Object)) error {
	ci := fs.GetConfig(ctx)
	return walk.ListR(ctx, f, "", false, ci.MaxDepth, walk.ListObjects, func(entries fs.DirEntries) error {
		entries.ForObject(fn)
		return nil
	})
}

// mutex for synchronized output
var outMutex sync.Mutex

// SyncPrintf is a global var holding the Printf function used in syncFprintf so that it can be overridden
// Note, despite name, does not provide sync and should not be called directly
// Call syncFprintf, which provides sync
var SyncPrintf = func(format string, a ...interface{}) {
	fmt.Printf(format, a...)
}

// Synchronized fmt.Fprintf
//
// Ignores errors from Fprintf
//
// Updated to print to terminal if no writer is defined
// This special behavior is used to allow easier replacement of the print to terminal code by progress
func syncFprintf(w io.Writer, format string, a ...interface{}) {
	outMutex.Lock()
	defer outMutex.Unlock()
	if w == nil {
		SyncPrintf(format, a...)
	} else {
		_, _ = fmt.Fprintf(w, format, a...)
	}
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
			tr.Done(ctx, nil)
		}()
		modTime := o.ModTime(ctx)
		syncFprintf(w, "%9d %s %s\n", o.Size(), modTime.Local().Format("2006-01-02 15:04:05.000000000"), o.Remote())
	})
}

// hashSum returns the human readable hash for ht passed in.  This may
// be UNSUPPORTED or ERROR. If it isn't returning a valid hash it will
// return an error.
func hashSum(ctx context.Context, ht hash.Type, downloadFlag bool, o fs.Object) (string, error) {
	var sum string
	var err error

	// If downloadFlag is true, download and hash the file.
	// If downloadFlag is false, call o.Hash asking the remote for the hash
	if downloadFlag {
		// Setup: Define accounting, open the file with NewReOpen to provide restarts, account for the transfer, and setup a multi-hasher with the appropriate type
		// Execution: io.Copy file to hasher, get hash and encode in hex

		tr := accounting.Stats(ctx).NewTransfer(o)
		defer func() {
			tr.Done(ctx, err)
		}()

		// Open with NewReOpen to provide restarts
		var options []fs.OpenOption
		for _, option := range fs.GetConfig(ctx).DownloadHeaders {
			options = append(options, option)
		}
		in, err := NewReOpen(ctx, o, fs.GetConfig(ctx).LowLevelRetries, options...)
		if err != nil {
			return "ERROR", errors.Wrapf(err, "Failed to open file %v", o)
		}

		// Account and buffer the transfer
		in = tr.Account(ctx, in).WithBuffer()

		// Setup hasher
		hasher, err := hash.NewMultiHasherTypes(hash.NewHashSet(ht))
		if err != nil {
			return "UNSUPPORTED", errors.Wrap(err, "Hash unsupported")
		}

		// Copy to hasher, downloading the file and passing directly to hash
		_, err = io.Copy(hasher, in)
		if err != nil {
			return "ERROR", errors.Wrap(err, "Failed to copy file to hasher")
		}

		// Get hash and encode as hex
		byteSum, err := hasher.Sum(ht)
		if err != nil {
			return "ERROR", errors.Wrap(err, "Hasher returned an error")
		}
		sum = hex.EncodeToString(byteSum)
	} else {
		tr := accounting.Stats(ctx).NewCheckingTransfer(o)
		defer func() {
			tr.Done(ctx, err)
		}()

		sum, err = o.Hash(ctx, ht)
		if err == hash.ErrUnsupported {
			return "UNSUPPORTED", errors.Wrap(err, "Hash unsupported")
		} else if err != nil {
			return "ERROR", errors.Wrapf(err, "Failed to get hash %v from backed: %v", ht, err)
		}
	}

	return sum, nil
}

// HashLister does an md5sum equivalent for the hash type passed in
// Updated to handle both standard hex encoding and base64
// Updated to perform multiple hashes concurrently
func HashLister(ctx context.Context, ht hash.Type, outputBase64 bool, downloadFlag bool, f fs.Fs, w io.Writer) error {
	concurrencyControl := make(chan struct{}, fs.GetConfig(ctx).Transfers)
	var wg sync.WaitGroup
	err := ListFn(ctx, f, func(o fs.Object) {
		wg.Add(1)
		concurrencyControl <- struct{}{}
		go func() {
			defer func() {
				<-concurrencyControl
				wg.Done()
			}()
			sum, err := hashSum(ctx, ht, downloadFlag, o)
			if outputBase64 && err == nil {
				hexBytes, _ := hex.DecodeString(sum)
				sum = base64.URLEncoding.EncodeToString(hexBytes)
				width := base64.URLEncoding.EncodedLen(hash.Width(ht) / 2)
				syncFprintf(w, "%*s  %s\n", width, sum, o.Remote())
			} else {
				syncFprintf(w, "%*s  %s\n", hash.Width(ht), sum, o.Remote())
			}
			if err != nil {
				err = fs.CountError(err)
				fs.Errorf(o, "%v", err)
			}
		}()
	})
	wg.Wait()
	return err
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
func ConfigMaxDepth(ctx context.Context, recursive bool) int {
	ci := fs.GetConfig(ctx)
	depth := ci.MaxDepth
	if !recursive && depth < 0 {
		depth = 1
	}
	return depth
}

// ListDir lists the directories/buckets/containers in the Fs to the supplied writer
func ListDir(ctx context.Context, f fs.Fs, w io.Writer) error {
	return walk.ListR(ctx, f, "", false, ConfigMaxDepth(ctx, false), walk.ListDirs, func(entries fs.DirEntries) error {
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
	if SkipDestructive(ctx, fs.LogDirName(f, dir), "make directory") {
		return nil
	}
	fs.Debugf(fs.LogDirName(f, dir), "Making directory")
	err := f.Mkdir(ctx, dir)
	if err != nil {
		err = fs.CountError(err)
		return err
	}
	return nil
}

// TryRmdir removes a container but not if not empty.  It doesn't
// count errors but may return one.
func TryRmdir(ctx context.Context, f fs.Fs, dir string) error {
	accounting.Stats(ctx).DeletedDirs(1)
	if SkipDestructive(ctx, fs.LogDirName(f, dir), "remove directory") {
		return nil
	}
	fs.Debugf(fs.LogDirName(f, dir), "Removing directory")
	return f.Rmdir(ctx, dir)
}

// Rmdir removes a container but not if not empty
func Rmdir(ctx context.Context, f fs.Fs, dir string) error {
	err := TryRmdir(ctx, f, dir)
	if err != nil {
		err = fs.CountError(err)
		return err
	}
	return err
}

// Purge removes a directory and all of its contents
func Purge(ctx context.Context, f fs.Fs, dir string) (err error) {
	doFallbackPurge := true
	if doPurge := f.Features().Purge; doPurge != nil {
		doFallbackPurge = false
		accounting.Stats(ctx).DeletedDirs(1)
		if SkipDestructive(ctx, fs.LogDirName(f, dir), "purge directory") {
			return nil
		}
		err = doPurge(ctx, dir)
		if err == fs.ErrorCantPurge {
			doFallbackPurge = true
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
		err = fs.CountError(err)
		return err
	}
	return nil
}

// Delete removes all the contents of a container.  Unlike Purge, it
// obeys includes and excludes.
func Delete(ctx context.Context, f fs.Fs) error {
	ci := fs.GetConfig(ctx)
	delChan := make(fs.ObjectsChan, ci.Transfers)
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
	ci := fs.GetConfig(ctx)
	o := make(fs.ObjectsChan, ci.Checkers)
	go func() {
		defer close(o)
		err := walk.ListR(ctx, f, dir, true, ci.MaxDepth, walk.ListObjects, func(entries fs.DirEntries) error {
			entries.ForObject(func(obj fs.Object) {
				o <- obj
			})
			return nil
		})
		if err != nil && err != fs.ErrorDirNotFound {
			err = errors.Wrap(err, "failed to list")
			err = fs.CountError(err)
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
	if SkipDestructive(ctx, f, "clean up old files") {
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
	ci := fs.GetConfig(ctx)
	return ListFn(ctx, f, func(o fs.Object) {
		var err error
		tr := accounting.Stats(ctx).NewTransfer(o)
		defer func() {
			tr.Done(ctx, err)
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
		for _, option := range ci.DownloadHeaders {
			options = append(options, option)
		}
		in, err := o.Open(ctx, options...)
		if err != nil {
			err = fs.CountError(err)
			fs.Errorf(o, "Failed to open: %v", err)
			return
		}
		if count >= 0 {
			in = &readCloser{Reader: &io.LimitedReader{R: in, N: count}, Closer: in}
		}
		in = tr.Account(ctx, in).WithBuffer() // account and buffer the transfer
		// take the lock just before we output stuff, so at the last possible moment
		mu.Lock()
		defer mu.Unlock()
		_, err = io.Copy(w, in)
		if err != nil {
			err = fs.CountError(err)
			fs.Errorf(o, "Failed to send to output: %v", err)
		}
	})
}

// Rcat reads data from the Reader until EOF and uploads it to a file on remote
func Rcat(ctx context.Context, fdst fs.Fs, dstFileName string, in io.ReadCloser, modTime time.Time) (dst fs.Object, err error) {
	ci := fs.GetConfig(ctx)
	tr := accounting.Stats(ctx).NewTransferRemoteSize(dstFileName, -1)
	defer func() {
		tr.Done(ctx, err)
	}()
	in = tr.Account(ctx, in).WithBuffer()

	readCounter := readers.NewCountingReader(in)
	var trackingIn io.Reader
	var hasher *hash.MultiHasher
	var options []fs.OpenOption
	if !ci.IgnoreChecksum {
		hashes := hash.NewHashSet(fdst.Hashes().GetOne()) // just pick one hash
		hashOption := &fs.HashesOption{Hashes: hashes}
		options = append(options, hashOption)
		hasher, err = hash.NewMultiHasherTypes(hashes)
		if err != nil {
			return nil, err
		}
		trackingIn = io.TeeReader(readCounter, hasher)
	} else {
		trackingIn = readCounter
	}
	for _, option := range ci.UploadHeaders {
		options = append(options, option)
	}

	compare := func(dst fs.Object) error {
		var sums map[hash.Type]string
		if hasher != nil {
			sums = hasher.Sums()
		}
		src := object.NewStaticObjectInfo(dstFileName, modTime, int64(readCounter.BytesRead()), false, sums, fdst)
		if !Equal(ctx, src, dst) {
			err = errors.Errorf("corrupted on transfer")
			err = fs.CountError(err)
			fs.Errorf(dst, "%v", err)
			return err
		}
		return nil
	}

	// check if file small enough for direct upload
	buf := make([]byte, ci.StreamingUploadCutoff)
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
		tmpLocalFs, err := fs.TemporaryLocalFs(ctx)
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

	if SkipDestructive(ctx, dstFileName, "upload from pipe") {
		// prevents "broken pipe" errors
		_, err = io.Copy(ioutil.Discard, in)
		return nil, err
	}

	objInfo := object.NewStaticObjectInfo(dstFileName, modTime, -1, false, nil, nil)
	if dst, err = fStreamTo.Features().PutStream(ctx, in, objInfo, options...); err != nil {
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
func PublicLink(ctx context.Context, f fs.Fs, remote string, expire fs.Duration, unlink bool) (string, error) {
	doPublicLink := f.Features().PublicLink
	if doPublicLink == nil {
		return "", errors.Errorf("%v doesn't support public links", f)
	}
	return doPublicLink(ctx, remote, expire, unlink)
}

// Rmdirs removes any empty directories (or directories only
// containing empty directories) under f, including f.
//
// Rmdirs obeys the filters
func Rmdirs(ctx context.Context, f fs.Fs, dir string, leaveRoot bool) error {
	ci := fs.GetConfig(ctx)
	fi := filter.GetConfig(ctx)
	dirEmpty := make(map[string]bool)
	dirEmpty[dir] = !leaveRoot
	err := walk.Walk(ctx, f, dir, false, ci.MaxDepth, func(dirPath string, entries fs.DirEntries, err error) error {
		if err != nil {
			err = fs.CountError(err)
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
		// If a filter matches the directory then that
		// directory is a candidate for deletion
		if !fi.Include(dir+"/", 0, time.Now()) {
			continue
		}
		err = TryRmdir(ctx, f, dir)
		if err != nil {
			err = fs.CountError(err)
			fs.Errorf(dir, "Failed to rmdir: %v", err)
			return err
		}
	}
	return nil
}

// GetCompareDest sets up --compare-dest
func GetCompareDest(ctx context.Context) (CompareDest []fs.Fs, err error) {
	ci := fs.GetConfig(ctx)
	CompareDest, err = cache.GetArr(ctx, ci.CompareDest)
	if err != nil {
		return nil, fserrors.FatalError(errors.Errorf("Failed to make fs for --compare-dest %q: %v", ci.CompareDest, err))
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
func GetCopyDest(ctx context.Context, fdst fs.Fs) (CopyDest []fs.Fs, err error) {
	ci := fs.GetConfig(ctx)
	CopyDest, err = cache.GetArr(ctx, ci.CopyDest)
	if err != nil {
		return nil, fserrors.FatalError(errors.Errorf("Failed to make fs for --copy-dest %q: %v", ci.CopyDest, err))
	}
	if !SameConfigArr(fdst, CopyDest) {
		return nil, fserrors.FatalError(errors.New("parameter to --copy-dest has to be on the same remote as destination"))
	}
	for _, cf := range CopyDest {
		if cf.Features().Copy == nil {
			return nil, fserrors.FatalError(errors.New("can't use --copy-dest on a remote which doesn't support server side copy"))
		}
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
	opt := defaultEqualOpt(ctx)
	opt.updateModTime = false
	if equal(ctx, src, CopyDestFile, opt) {
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
			fs.Debugf(src, "Destination found in --copy-dest, using server-side copy")
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
func CompareOrCopyDest(ctx context.Context, fdst fs.Fs, dst, src fs.Object, CompareOrCopyDest []fs.Fs, backupDir fs.Fs) (NoNeedTransfer bool, err error) {
	ci := fs.GetConfig(ctx)
	if len(ci.CompareDest) > 0 {
		for _, compareF := range CompareOrCopyDest {
			NoNeedTransfer, err := compareDest(ctx, dst, src, compareF)
			if NoNeedTransfer || err != nil {
				return NoNeedTransfer, err
			}
		}
	} else if len(ci.CopyDest) > 0 {
		for _, copyF := range CompareOrCopyDest {
			NoNeedTransfer, err := copyDest(ctx, fdst, dst, src, copyF, backupDir)
			if NoNeedTransfer || err != nil {
				return NoNeedTransfer, err
			}
		}
	}
	return false, nil
}

// NeedTransfer checks to see if src needs to be copied to dst using
// the current config.
//
// Returns a flag which indicates whether the file needs to be
// transferred or not.
func NeedTransfer(ctx context.Context, dst, src fs.Object) bool {
	ci := fs.GetConfig(ctx)
	if dst == nil {
		fs.Debugf(src, "Need to transfer - File not found at Destination")
		return true
	}
	// If we should ignore existing files, don't transfer
	if ci.IgnoreExisting {
		fs.Debugf(src, "Destination exists, skipping")
		return false
	}
	// If we should upload unconditionally
	if ci.IgnoreTimes {
		fs.Debugf(src, "Transferring unconditionally as --ignore-times is in use")
		return true
	}
	// If UpdateOlder is in effect, skip if dst is newer than src
	if ci.UpdateOlder {
		srcModTime := src.ModTime(ctx)
		dstModTime := dst.ModTime(ctx)
		dt := dstModTime.Sub(srcModTime)
		// If have a mutually agreed precision then use that
		modifyWindow := fs.GetModifyWindow(ctx, dst.Fs(), src.Fs())
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
			// force --checksum on for the check and do update modtimes by default
			opt := defaultEqualOpt(ctx)
			opt.forceModTimeMatch = true
			if equal(ctx, src, dst, opt) {
				fs.Debugf(src, "Unchanged skipping")
				return false
			}
		default:
			// Do a size only compare unless --checksum is set
			opt := defaultEqualOpt(ctx)
			opt.sizeOnly = !ci.CheckSum
			if equal(ctx, src, dst, opt) {
				fs.Debugf(src, "Destination mod time is within %v of source and files identical, skipping", modifyWindow)
				return false
			}
			fs.Debugf(src, "Destination mod time is within %v of source but files differ, transferring", modifyWindow)
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
			tr.Done(ctx, err)
		}()
		body := ioutil.NopCloser(in) // we let the server close the body
		in := tr.Account(ctx, body)  // account the transfer (no buffering)

		if SkipDestructive(ctx, dstFileName, "upload from pipe") {
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

// copyURLFunc is called from CopyURLFn
type copyURLFunc func(ctx context.Context, dstFileName string, in io.ReadCloser, size int64, modTime time.Time) (err error)

// copyURLFn copies the data from the url to the function supplied
func copyURLFn(ctx context.Context, dstFileName string, url string, dstFileNameFromURL bool, fn copyURLFunc) (err error) {
	client := fshttp.NewClient(ctx)
	resp, err := client.Get(url)
	if err != nil {
		return err
	}
	defer fs.CheckClose(resp.Body, &err)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return errors.Errorf("CopyURL failed: %s", resp.Status)
	}
	modTime, err := http.ParseTime(resp.Header.Get("Last-Modified"))
	if err != nil {
		modTime = time.Now()
	}
	if dstFileNameFromURL {
		dstFileName = path.Base(resp.Request.URL.Path)
		if dstFileName == "." || dstFileName == "/" {
			return errors.Errorf("CopyURL failed: file name wasn't found in url")
		}
		fs.Debugf(dstFileName, "File name found in url")
	}
	return fn(ctx, dstFileName, resp.Body, resp.ContentLength, modTime)
}

// CopyURL copies the data from the url to (fdst, dstFileName)
func CopyURL(ctx context.Context, fdst fs.Fs, dstFileName string, url string, dstFileNameFromURL bool, noClobber bool) (dst fs.Object, err error) {

	err = copyURLFn(ctx, dstFileName, url, dstFileNameFromURL, func(ctx context.Context, dstFileName string, in io.ReadCloser, size int64, modTime time.Time) (err error) {
		if noClobber {
			_, err = fdst.NewObject(ctx, dstFileName)
			if err == nil {
				return errors.New("CopyURL failed: file already exist")
			}
		}
		dst, err = RcatSize(ctx, fdst, dstFileName, in, size, modTime)
		return err
	})
	return dst, err
}

// CopyURLToWriter copies the data from the url to the io.Writer supplied
func CopyURLToWriter(ctx context.Context, url string, out io.Writer) (err error) {
	return copyURLFn(ctx, "", url, false, func(ctx context.Context, dstFileName string, in io.ReadCloser, size int64, modTime time.Time) (err error) {
		_, err = io.Copy(out, in)
		return err
	})
}

// BackupDir returns the correctly configured --backup-dir
func BackupDir(ctx context.Context, fdst fs.Fs, fsrc fs.Fs, srcFileName string) (backupDir fs.Fs, err error) {
	ci := fs.GetConfig(ctx)
	if ci.BackupDir != "" {
		backupDir, err = cache.Get(ctx, ci.BackupDir)
		if err != nil {
			return nil, fserrors.FatalError(errors.Errorf("Failed to make fs for --backup-dir %q: %v", ci.BackupDir, err))
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
			if ci.Suffix == "" {
				if SameDir(fdst, backupDir) {
					return nil, fserrors.FatalError(errors.New("destination and parameter to --backup-dir mustn't be the same"))
				}
				if SameDir(fsrc, backupDir) {
					return nil, fserrors.FatalError(errors.New("source and parameter to --backup-dir mustn't be the same"))
				}
			}
		}
	} else if ci.Suffix != "" {
		// --backup-dir is not set but --suffix is - use the destination as the backupDir
		backupDir = fdst
	} else {
		return nil, fserrors.FatalError(errors.New("internal error: BackupDir called when --backup-dir and --suffix both empty"))
	}
	if !CanServerSideMove(backupDir) {
		return nil, fserrors.FatalError(errors.New("can't use --backup-dir on a remote which doesn't support server-side move or copy"))
	}
	return backupDir, nil
}

// MoveBackupDir moves a file to the backup dir
func MoveBackupDir(ctx context.Context, backupDir fs.Fs, dst fs.Object) (err error) {
	remoteWithSuffix := SuffixName(ctx, dst.Remote())
	overwritten, _ := backupDir.NewObject(ctx, remoteWithSuffix)
	_, err = Move(ctx, backupDir, overwritten, remoteWithSuffix, dst)
	return err
}

// moveOrCopyFile moves or copies a single file possibly to a new name
func moveOrCopyFile(ctx context.Context, fdst fs.Fs, fsrc fs.Fs, dstFileName string, srcFileName string, cp bool) (err error) {
	ci := fs.GetConfig(ctx)
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
	var dstObj fs.Object
	if !ci.NoCheckDest {
		dstObj, err = fdst.NewObject(ctx, dstFileName)
		if err == fs.ErrorObjectNotFound {
			dstObj = nil
		} else if err != nil {
			return err
		}
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
			tr.Done(ctx, err)
		}()
		tmpObj, err := Op(ctx, fdst, nil, tmpObjName, srcObj)
		if err != nil {
			return errors.Wrap(err, "error while moving file to temporary location")
		}
		_, err = Op(ctx, fdst, nil, dstFileName, tmpObj)
		return err
	}

	var backupDir fs.Fs
	var copyDestDir []fs.Fs
	if ci.BackupDir != "" || ci.Suffix != "" {
		backupDir, err = BackupDir(ctx, fdst, fsrc, srcFileName)
		if err != nil {
			return errors.Wrap(err, "creating Fs for --backup-dir failed")
		}
	}
	if len(ci.CompareDest) > 0 {
		copyDestDir, err = GetCompareDest(ctx)
		if err != nil {
			return err
		}
	} else if len(ci.CopyDest) > 0 {
		copyDestDir, err = GetCopyDest(ctx, fdst)
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
		tr.Done(ctx, err)
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
	ci := fs.GetConfig(ctx)

	if SkipDestructive(ctx, srcRemote, "dirMove") {
		accounting.Stats(ctx).Renames(1)
		return nil
	}

	// Use DirMove if possible
	if doDirMove := f.Features().DirMove; doDirMove != nil {
		err = doDirMove(ctx, f, srcRemote, dstRemote)
		if err == nil {
			accounting.Stats(ctx).Renames(1)
		}
		return err
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
	renames := make(chan rename, ci.Transfers)
	g, gCtx := errgroup.WithContext(context.Background())
	for i := 0; i < ci.Transfers; i++ {
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

var (
	interactiveMu sync.Mutex
	skipped       = map[string]bool{}
)

// skipDestructiveChoose asks the user which action to take
//
// Call with interactiveMu held
func skipDestructiveChoose(ctx context.Context, subject interface{}, action string) (skip bool) {
	fmt.Printf("rclone: %s \"%v\"?\n", action, subject)
	switch i := config.CommandDefault([]string{
		"yYes, this is OK",
		"nNo, skip this",
		fmt.Sprintf("sSkip all %s operations with no more questions", action),
		fmt.Sprintf("!Do all %s operations with no more questions", action),
		"qExit rclone now.",
	}, 0); i {
	case 'y':
		skip = false
	case 'n':
		skip = true
	case 's':
		skip = true
		skipped[action] = true
		fs.Logf(nil, "Skipping all %s operations from now on without asking", action)
	case '!':
		skip = false
		skipped[action] = false
		fs.Logf(nil, "Doing all %s operations from now on without asking", action)
	case 'q':
		fs.Logf(nil, "Quitting rclone now")
		atexit.Run()
		os.Exit(0)
	default:
		skip = true
		fs.Errorf(nil, "Bad choice %c", i)
	}
	return skip
}

// SkipDestructive should be called whenever rclone is about to do an destructive operation.
//
// It will check the --dry-run flag and it will ask the user if the --interactive flag is set.
//
// subject should be the object or directory in use
//
// action should be a descriptive word or short phrase
//
// Together they should make sense in this sentence: "Rclone is about
// to action subject".
func SkipDestructive(ctx context.Context, subject interface{}, action string) (skip bool) {
	var flag string
	ci := fs.GetConfig(ctx)
	switch {
	case ci.DryRun:
		flag = "--dry-run"
		skip = true
	case ci.Interactive:
		flag = "--interactive"
		interactiveMu.Lock()
		defer interactiveMu.Unlock()
		var found bool
		skip, found = skipped[action]
		if !found {
			skip = skipDestructiveChoose(ctx, subject, action)
		}
	default:
		return false
	}
	if skip {
		size := int64(-1)
		if do, ok := subject.(interface{ Size() int64 }); ok {
			size = do.Size()
		}
		if size >= 0 {
			fs.Logf(subject, "Skipped %s as %s is set (size %v)", fs.LogValue("skipped", action), flag, fs.LogValue("size", fs.SizeSuffix(size)))
		} else {
			fs.Logf(subject, "Skipped %s as %s is set", fs.LogValue("skipped", action), flag)
		}
	}
	return skip
}
