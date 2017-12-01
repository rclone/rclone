// Generic operations on filesystems and objects

package fs

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"mime"
	"path"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/pkg/errors"
	"github.com/spf13/pflag"
	"golang.org/x/net/context"
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
				Infof(f, "Modify window not supported")
				return
			}
		}
	}
	Infof(fs[0], "Modify window is %s", Config.ModifyWindow)
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
func CheckHashes(src ObjectInfo, dst Object) (equal bool, hash HashType, err error) {
	common := src.Fs().Hashes().Overlap(dst.Fs().Hashes())
	// Debugf(nil, "Shared hashes: %v", common)
	if common.Count() == 0 {
		return true, HashNone, nil
	}
	hash = common.GetOne()
	srcHash, err := src.Hash(hash)
	if err != nil {
		Stats.Error(err)
		Errorf(src, "Failed to calculate src hash: %v", err)
		return false, hash, err
	}
	if srcHash == "" {
		return true, HashNone, nil
	}
	dstHash, err := dst.Hash(hash)
	if err != nil {
		Stats.Error(err)
		Errorf(dst, "Failed to calculate dst hash: %v", err)
		return false, hash, err
	}
	if dstHash == "" {
		return true, HashNone, nil
	}
	if srcHash != dstHash {
		Debugf(src, "%v = %s (%v)", hash, srcHash, src.Fs())
		Debugf(dst, "%v = %s (%v)", hash, dstHash, dst.Fs())
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
func Equal(src ObjectInfo, dst Object) bool {
	return equal(src, dst, Config.SizeOnly, Config.CheckSum)
}

func equal(src ObjectInfo, dst Object, sizeOnly, checkSum bool) bool {
	if !Config.IgnoreSize {
		if src.Size() != dst.Size() {
			Debugf(src, "Sizes differ (src %d vs dst %d)", src.Size(), dst.Size())
			return false
		}
	}
	if sizeOnly {
		Debugf(src, "Sizes identical")
		return true
	}

	// Assert: Size is equal or being ignored

	// If checking checksum and not modtime
	if checkSum {
		// Check the hash
		same, hash, _ := CheckHashes(src, dst)
		if !same {
			Debugf(src, "%v differ", hash)
			return false
		}
		if hash == HashNone {
			Debugf(src, "Size of src and dst objects identical")
		} else {
			Debugf(src, "Size and %v of src and dst objects identical", hash)
		}
		return true
	}

	// Sizes the same so check the mtime
	if Config.ModifyWindow == ModTimeNotSupported {
		Debugf(src, "Sizes identical")
		return true
	}
	srcModTime := src.ModTime()
	dstModTime := dst.ModTime()
	dt := dstModTime.Sub(srcModTime)
	ModifyWindow := Config.ModifyWindow
	if dt < ModifyWindow && dt > -ModifyWindow {
		Debugf(src, "Size and modification time the same (differ by %s, within tolerance %s)", dt, ModifyWindow)
		return true
	}

	Debugf(src, "Modification times differ by %s: %v, %v", dt, srcModTime, dstModTime)

	// Check if the hashes are the same
	same, hash, _ := CheckHashes(src, dst)
	if !same {
		Debugf(src, "%v differ", hash)
		return false
	}
	if hash == HashNone {
		// if couldn't check hash, return that they differ
		return false
	}

	// mod time differs but hash is the same to reset mod time if required
	if !Config.NoUpdateModTime {
		if Config.DryRun {
			Logf(src, "Not updating modification time as --dry-run")
		} else {
			// Size and hash the same but mtime different
			// Error if objects are treated as immutable
			if Config.Immutable {
				Errorf(dst, "Timestamp mismatch between immutable objects")
				return false
			}
			// Update the mtime of the dst object here
			err := dst.SetModTime(srcModTime)
			if err == ErrorCantSetModTime {
				Debugf(dst, "src and dst identical but can't set mod time without re-uploading")
				return false
			} else if err == ErrorCantSetModTimeWithoutDelete {
				Debugf(dst, "src and dst identical but can't set mod time without deleting and re-uploading")
				err = dst.Remove()
				if err != nil {
					Errorf(dst, "failed to delete before re-upload: %v", err)
				}
				return false
			} else if err != nil {
				Stats.Error(err)
				Errorf(dst, "Failed to set modification time: %v", err)
			} else {
				Infof(src, "Updated modification time in destination")
			}
		}
	}
	return true
}

// MimeTypeFromName returns a guess at the mime type from the name
func MimeTypeFromName(remote string) (mimeType string) {
	mimeType = mime.TypeByExtension(path.Ext(remote))
	if !strings.ContainsRune(mimeType, '/') {
		mimeType = "application/octet-stream"
	}
	return mimeType
}

// MimeType returns the MimeType from the object, either by calling
// the MimeTyper interface or using MimeTypeFromName
func MimeType(o ObjectInfo) (mimeType string) {
	// Read the MimeType from the optional interface if available
	if do, ok := o.(MimeTyper); ok {
		mimeType = do.MimeType()
		// Debugf(o, "Read MimeType as %q", mimeType)
		if mimeType != "" {
			return mimeType
		}
	}
	return MimeTypeFromName(o.Remote())
}

// Used to remove a failed copy
//
// Returns whether the file was succesfully removed or not
func removeFailedCopy(dst Object) bool {
	if dst == nil {
		return false
	}
	Infof(dst, "Removing failed copy")
	removeErr := dst.Remove()
	if removeErr != nil {
		Infof(dst, "Failed to remove failed copy: %s", removeErr)
		return false
	}
	return true
}

// Wrapper to override the remote for an object
type overrideRemoteObject struct {
	Object
	remote string
}

// Remote returns the overriden remote name
func (o *overrideRemoteObject) Remote() string {
	return o.remote
}

// MimeType returns the mime type of the underlying object or "" if it
// can't be worked out
func (o *overrideRemoteObject) MimeType() string {
	if do, ok := o.Object.(MimeTyper); ok {
		return do.MimeType()
	}
	return ""
}

// Check interface is satisfied
var _ MimeTyper = (*overrideRemoteObject)(nil)

// Copy src object to dst or f if nil.  If dst is nil then it uses
// remote as the name of the new object.
//
// It returns the destination object if possible.  Note that this may
// be nil.
func Copy(f Fs, dst Object, remote string, src Object) (newDst Object, err error) {
	newDst = dst
	if Config.DryRun {
		Logf(src, "Not copying as --dry-run")
		return newDst, nil
	}
	maxTries := Config.LowLevelRetries
	tries := 0
	doUpdate := dst != nil
	// work out which hash to use - limit to 1 hash in common
	var common HashSet
	hashType := HashNone
	if !Config.SizeOnly {
		common = src.Fs().Hashes().Overlap(f.Hashes())
		if common.Count() > 0 {
			hashType = common.GetOne()
			common = HashSet(hashType)
		}
	}
	hashOption := &HashesOption{Hashes: common}
	var actionTaken string
	for {
		// Try server side copy first - if has optional interface and
		// is same underlying remote
		actionTaken = "Copied (server side copy)"
		if doCopy := f.Features().Copy; doCopy != nil && SameConfig(src.Fs(), f) {
			newDst, err = doCopy(src, remote)
			if err == nil {
				dst = newDst
			}
		} else {
			err = ErrorCantCopy
		}
		// If can't server side copy, do it manually
		if err == ErrorCantCopy {
			var in0 io.ReadCloser
			in0, err = src.Open(hashOption)
			if err != nil {
				err = errors.Wrap(err, "failed to open source object")
			} else {
				in := NewAccount(in0, src).WithBuffer() // account and buffer the transfer
				var wrappedSrc ObjectInfo = src
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
		tries++
		if tries >= maxTries {
			break
		}
		// Retry if err returned a retry error
		if IsRetryError(err) || ShouldRetry(err) {
			Debugf(src, "Received error: %v - low level retry %d/%d", err, tries, maxTries)
			continue
		}
		// otherwise finish
		break
	}
	if err != nil {
		Stats.Error(err)
		Errorf(src, "Failed to copy: %v", err)
		return newDst, err
	}

	// Verify sizes are the same after transfer
	if !Config.IgnoreSize && src.Size() != dst.Size() {
		err = errors.Errorf("corrupted on transfer: sizes differ %d vs %d", src.Size(), dst.Size())
		Errorf(dst, "%v", err)
		Stats.Error(err)
		removeFailedCopy(dst)
		return newDst, err
	}

	// Verify hashes are the same after transfer - ignoring blank hashes
	// TODO(klauspost): This could be extended, so we always create a hash type matching
	// the destination, and calculate it while sending.
	if hashType != HashNone {
		var srcSum string
		srcSum, err = src.Hash(hashType)
		if err != nil {
			Stats.Error(err)
			Errorf(src, "Failed to read src hash: %v", err)
		} else if srcSum != "" {
			var dstSum string
			dstSum, err = dst.Hash(hashType)
			if err != nil {
				Stats.Error(err)
				Errorf(dst, "Failed to read hash: %v", err)
			} else if !Config.IgnoreChecksum && !HashEquals(srcSum, dstSum) {
				err = errors.Errorf("corrupted on transfer: %v hash differ %q vs %q", hashType, srcSum, dstSum)
				Errorf(dst, "%v", err)
				Stats.Error(err)
				removeFailedCopy(dst)
				return newDst, err
			}
		}
	}

	Infof(src, actionTaken)
	return newDst, err
}

// Move src object to dst or fdst if nil.  If dst is nil then it uses
// remote as the name of the new object.
//
// It returns the destination object if possible.  Note that this may
// be nil.
func Move(fdst Fs, dst Object, remote string, src Object) (newDst Object, err error) {
	newDst = dst
	if Config.DryRun {
		Logf(src, "Not moving as --dry-run")
		return newDst, nil
	}
	// See if we have Move available
	if doMove := fdst.Features().Move; doMove != nil && SameConfig(src.Fs(), fdst) {
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
			Infof(src, "Moved (server side)")
			return newDst, nil
		case ErrorCantMove:
			Debugf(src, "Can't move, switching to copy")
		default:
			Stats.Error(err)
			Errorf(src, "Couldn't move: %v", err)
			return newDst, err
		}
	}
	// Move not found or didn't work so copy dst <- src
	newDst, err = Copy(fdst, dst, remote, src)
	if err != nil {
		Errorf(src, "Not deleting source as copy failed: %v", err)
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
func CanServerSideMove(fdst Fs) bool {
	canMove := fdst.Features().Move != nil
	canCopy := fdst.Features().Copy != nil
	return canMove || canCopy
}

// deleteFileWithBackupDir deletes a single file respecting --dry-run
// and accumulating stats and errors.
//
// If backupDir is set then it moves the file to there instead of
// deleting
func deleteFileWithBackupDir(dst Object, backupDir Fs) (err error) {
	Stats.Checking(dst.Remote())
	action, actioned, actioning := "delete", "Deleted", "deleting"
	if backupDir != nil {
		action, actioned, actioning = "move into backup dir", "Moved into backup dir", "moving into backup dir"
	}
	if Config.DryRun {
		Logf(dst, "Not %s as --dry-run", actioning)
	} else if backupDir != nil {
		if !SameConfig(dst.Fs(), backupDir) {
			err = errors.New("parameter to --backup-dir has to be on the same remote as destination")
		} else {
			remoteWithSuffix := dst.Remote() + Config.Suffix
			overwritten, _ := backupDir.NewObject(remoteWithSuffix)
			_, err = Move(backupDir, overwritten, remoteWithSuffix, dst)
		}
	} else {
		err = dst.Remove()
	}
	if err != nil {
		Stats.Error(err)
		Errorf(dst, "Couldn't %s: %v", action, err)
	} else if !Config.DryRun {
		Infof(dst, actioned)
	}
	Stats.DoneChecking(dst.Remote())
	return err
}

// DeleteFile deletes a single file respecting --dry-run and accumulating stats and errors.
//
// If useBackupDir is set and --backup-dir is in effect then it moves
// the file to there instead of deleting
func DeleteFile(dst Object) (err error) {
	return deleteFileWithBackupDir(dst, nil)
}

// deleteFilesWithBackupDir removes all the files passed in the
// channel
//
// If backupDir is set the files will be placed into that directory
// instead of being deleted.
func deleteFilesWithBackupDir(toBeDeleted ObjectsChan, backupDir Fs) error {
	var wg sync.WaitGroup
	wg.Add(Config.Transfers)
	var errorCount int32
	for i := 0; i < Config.Transfers; i++ {
		go func() {
			defer wg.Done()
			for dst := range toBeDeleted {
				err := deleteFileWithBackupDir(dst, backupDir)
				if err != nil {
					atomic.AddInt32(&errorCount, 1)
				}
			}
		}()
	}
	Infof(nil, "Waiting for deletions to finish")
	wg.Wait()
	if errorCount > 0 {
		return errors.Errorf("failed to delete %d files", errorCount)
	}
	return nil
}

// DeleteFiles removes all the files passed in the channel
func DeleteFiles(toBeDeleted ObjectsChan) error {
	return deleteFilesWithBackupDir(toBeDeleted, nil)
}

// Read a Objects into add() for the given Fs.
// dir is the start directory, "" for root
// If includeAll is specified all files will be added,
// otherwise only files passing the filter will be added.
//
// Each object is passed ito the function provided.  If that returns
// an error then the listing will be aborted and that error returned.
func readFilesFn(fs Fs, includeAll bool, dir string, add func(Object) error) (err error) {
	return Walk(fs, "", includeAll, Config.MaxDepth, func(dirPath string, entries DirEntries, err error) error {
		if err != nil {
			return err
		}
		return entries.ForObjectError(add)
	})
}

// DirEntries is a slice of Object or *Dir
type DirEntries []DirEntry

// Len is part of sort.Interface.
func (ds DirEntries) Len() int {
	return len(ds)
}

// Swap is part of sort.Interface.
func (ds DirEntries) Swap(i, j int) {
	ds[i], ds[j] = ds[j], ds[i]
}

// Less is part of sort.Interface.
func (ds DirEntries) Less(i, j int) bool {
	return ds[i].Remote() < ds[j].Remote()
}

// ForObject runs the function supplied on every object in the entries
func (ds DirEntries) ForObject(fn func(o Object)) {
	for _, entry := range ds {
		o, ok := entry.(Object)
		if ok {
			fn(o)
		}
	}
}

// ForObjectError runs the function supplied on every object in the entries
func (ds DirEntries) ForObjectError(fn func(o Object) error) error {
	for _, entry := range ds {
		o, ok := entry.(Object)
		if ok {
			err := fn(o)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

// ForDir runs the function supplied on every Directory in the entries
func (ds DirEntries) ForDir(fn func(dir Directory)) {
	for _, entry := range ds {
		dir, ok := entry.(Directory)
		if ok {
			fn(dir)
		}
	}
}

// ForDirError runs the function supplied on every Directory in the entries
func (ds DirEntries) ForDirError(fn func(dir Directory) error) error {
	for _, entry := range ds {
		dir, ok := entry.(Directory)
		if ok {
			err := fn(dir)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

// DirEntryType returns a string description of the DirEntry, either
// "object", "directory" or "unknown type XXX"
func DirEntryType(d DirEntry) string {
	switch d.(type) {
	case Object:
		return "object"
	case Directory:
		return "directory"
	}
	return fmt.Sprintf("unknown type %T", d)
}

// ListDirSorted reads Object and *Dir into entries for the given Fs.
//
// dir is the start directory, "" for root
//
// If includeAll is specified all files will be added, otherwise only
// files and directories passing the filter will be added.
//
// Files will be returned in sorted order
func ListDirSorted(fs Fs, includeAll bool, dir string) (entries DirEntries, err error) {
	// Get unfiltered entries from the fs
	entries, err = fs.List(dir)
	if err != nil {
		return nil, err
	}
	// This should happen only if exclude files lives in the
	// starting directory, otherwise ListDirSorted should not be
	// called.
	if !includeAll && Config.Filter.ListContainsExcludeFile(entries) {
		Debugf(dir, "Excluded from sync (and deletion)")
		return nil, nil
	}
	return filterAndSortDir(entries, includeAll, dir, Config.Filter.IncludeObject, Config.Filter.IncludeDirectory(fs))
}

// filter (if required) and check the entries, then sort them
func filterAndSortDir(entries DirEntries, includeAll bool, dir string,
	IncludeObject func(o Object) bool,
	IncludeDirectory func(remote string) (bool, error)) (newEntries DirEntries, err error) {
	newEntries = entries[:0] // in place filter
	prefix := ""
	if dir != "" {
		prefix = dir + "/"
	}
	for _, entry := range entries {
		ok := true
		// check includes and types
		switch x := entry.(type) {
		case Object:
			// Make sure we don't delete excluded files if not required
			if !includeAll && !IncludeObject(x) {
				ok = false
				Debugf(x, "Excluded from sync (and deletion)")
			}
		case Directory:
			if !includeAll {
				include, err := IncludeDirectory(x.Remote())
				if err != nil {
					return nil, err
				}
				if !include {
					ok = false
					Debugf(x, "Excluded from sync (and deletion)")
				}
			}
		default:
			return nil, errors.Errorf("unknown object type %T", entry)
		}
		// check remote name belongs in this directry
		remote := entry.Remote()
		switch {
		case !ok:
			// ignore
		case !strings.HasPrefix(remote, prefix):
			ok = false
			Errorf(entry, "Entry doesn't belong in directory %q (too short) - ignoring", dir)
		case remote == prefix:
			ok = false
			Errorf(entry, "Entry doesn't belong in directory %q (same as directory) - ignoring", dir)
		case strings.ContainsRune(remote[len(prefix):], '/'):
			ok = false
			Errorf(entry, "Entry doesn't belong in directory %q (contains subdir) - ignoring", dir)
		default:
			// ok
		}
		if ok {
			newEntries = append(newEntries, entry)
		}
	}
	entries = newEntries

	// Sort the directory entries by Remote
	//
	// We use a stable sort here just in case there are
	// duplicates. Assuming the remote delivers the entries in a
	// consistent order, this will give the best user experience
	// in syncing as it will use the first entry for the sync
	// comparison.
	sort.Stable(entries)
	return entries, nil
}

// SameConfig returns true if fdst and fsrc are using the same config
// file entry
func SameConfig(fdst, fsrc Info) bool {
	return fdst.Name() == fsrc.Name()
}

// Same returns true if fdst and fsrc point to the same underlying Fs
func Same(fdst, fsrc Info) bool {
	return SameConfig(fdst, fsrc) && fdst.Root() == fsrc.Root()
}

// Overlapping returns true if fdst and fsrc point to the same
// underlying Fs and they overlap.
func Overlapping(fdst, fsrc Info) bool {
	if !SameConfig(fdst, fsrc) {
		return false
	}
	// Return the Root with a trailing / if not empty
	fixedRoot := func(f Info) string {
		s := strings.Trim(f.Root(), "/")
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
func checkIdentical(dst, src Object) (differ bool, noHash bool) {
	same, hash, err := CheckHashes(src, dst)
	if err != nil {
		// CheckHashes will log and count errors
		return true, false
	}
	if hash == HashNone {
		return false, true
	}
	if !same {
		err = errors.Errorf("%v differ", hash)
		Errorf(src, "%v", err)
		Stats.Error(err)
		return true, false
	}
	return false, false
}

// checkFn is the the type of the checking function used in CheckFn()
type checkFn func(a, b Object) (differ bool, noHash bool)

// checkMarch is used to march over two Fses in the same way as
// sync/copy
type checkMarch struct {
	fdst, fsrc      Fs
	check           checkFn
	differences     int32
	noHashes        int32
	srcFilesMissing int32
	dstFilesMissing int32
}

// DstOnly have an object which is in the destination only
func (c *checkMarch) DstOnly(dst DirEntry) (recurse bool) {
	switch dst.(type) {
	case Object:
		err := errors.Errorf("File not in %v", c.fsrc)
		Errorf(dst, "%v", err)
		Stats.Error(err)
		atomic.AddInt32(&c.differences, 1)
		atomic.AddInt32(&c.srcFilesMissing, 1)
	case Directory:
		// Do the same thing to the entire contents of the directory
		return true
	default:
		panic("Bad object in DirEntries")
	}
	return false
}

// SrcOnly have an object which is in the source only
func (c *checkMarch) SrcOnly(src DirEntry) (recurse bool) {
	switch src.(type) {
	case Object:
		err := errors.Errorf("File not in %v", c.fdst)
		Errorf(src, "%v", err)
		Stats.Error(err)
		atomic.AddInt32(&c.differences, 1)
		atomic.AddInt32(&c.dstFilesMissing, 1)
	case Directory:
		// Do the same thing to the entire contents of the directory
		return true
	default:
		panic("Bad object in DirEntries")
	}
	return false
}

// check to see if two objects are identical using the check function
func (c *checkMarch) checkIdentical(dst, src Object) (differ bool, noHash bool) {
	Stats.Checking(src.Remote())
	defer Stats.DoneChecking(src.Remote())
	if !Config.IgnoreSize && src.Size() != dst.Size() {
		err := errors.Errorf("Sizes differ")
		Errorf(src, "%v", err)
		Stats.Error(err)
		return true, false
	}
	if Config.SizeOnly {
		return false, false
	}
	return c.check(dst, src)
}

// Match is called when src and dst are present, so sync src to dst
func (c *checkMarch) Match(dst, src DirEntry) (recurse bool) {
	switch srcX := src.(type) {
	case Object:
		dstX, ok := dst.(Object)
		if ok {
			differ, noHash := c.checkIdentical(dstX, srcX)
			if differ {
				atomic.AddInt32(&c.differences, 1)
			} else {
				Debugf(dstX, "OK")
			}
			if noHash {
				atomic.AddInt32(&c.noHashes, 1)
			}
		} else {
			err := errors.Errorf("is file on %v but directory on %v", c.fsrc, c.fdst)
			Errorf(src, "%v", err)
			Stats.Error(err)
			atomic.AddInt32(&c.differences, 1)
			atomic.AddInt32(&c.dstFilesMissing, 1)
		}
	case Directory:
		// Do the same thing to the entire contents of the directory
		_, ok := dst.(Directory)
		if ok {
			return true
		}
		err := errors.Errorf("is file on %v but directory on %v", c.fdst, c.fsrc)
		Errorf(dst, "%v", err)
		Stats.Error(err)
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
func CheckFn(fdst, fsrc Fs, check checkFn) error {
	c := &checkMarch{
		fdst:  fdst,
		fsrc:  fsrc,
		check: check,
	}

	// set up a march over fdst and fsrc
	m := newMarch(context.Background(), fdst, fsrc, "", c)
	Infof(fdst, "Waiting for checks to finish")
	m.run()

	if c.dstFilesMissing > 0 {
		Logf(fdst, "%d files missing", c.dstFilesMissing)
	}
	if c.srcFilesMissing > 0 {
		Logf(fsrc, "%d files missing", c.srcFilesMissing)
	}

	Logf(fdst, "%d differences found", Stats.GetErrors())
	if c.noHashes > 0 {
		Logf(fdst, "%d hashes could not be checked", c.noHashes)
	}
	if c.differences > 0 {
		return errors.Errorf("%d differences found", c.differences)
	}
	return nil
}

// Check the files in fsrc and fdst according to Size and hash
func Check(fdst, fsrc Fs) error {
	return CheckFn(fdst, fsrc, checkIdentical)
}

// ReadFill reads as much data from r into buf as it can
//
// It reads until the buffer is full or r.Read returned an error.
//
// This is io.ReadFull but when you just want as much data as
// possible, not an exact size of block.
func ReadFill(r io.Reader, buf []byte) (n int, err error) {
	var nn int
	for n < len(buf) && err == nil {
		nn, err = r.Read(buf[n:])
		n += nn
	}
	return n, err
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
		n1, err1 := ReadFill(in1, buf1)
		n2, err2 := ReadFill(in2, buf2)
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
func CheckIdentical(dst, src Object) (differ bool, err error) {
	in1, err := dst.Open()
	if err != nil {
		return true, errors.Wrapf(err, "failed to open %q", dst)
	}
	in1 = NewAccount(in1, dst).WithBuffer() // account and buffer the transfer
	defer CheckClose(in1, &err)

	in2, err := src.Open()
	if err != nil {
		return true, errors.Wrapf(err, "failed to open %q", src)
	}
	in2 = NewAccount(in2, src).WithBuffer() // account and buffer the transfer
	defer CheckClose(in2, &err)

	return CheckEqualReaders(in1, in2)
}

// CheckDownload checks the files in fsrc and fdst according to Size
// and the actual contents of the files.
func CheckDownload(fdst, fsrc Fs) error {
	check := func(a, b Object) (differ bool, noHash bool) {
		differ, err := CheckIdentical(a, b)
		if err != nil {
			Stats.Error(err)
			Errorf(a, "Failed to download: %v", err)
			return true, true
		}
		return differ, false
	}
	return CheckFn(fdst, fsrc, check)
}

// ListFn lists the Fs to the supplied function
//
// Lists in parallel which may get them out of order
func ListFn(f Fs, fn func(Object)) error {
	return Walk(f, "", false, Config.MaxDepth, func(dirPath string, entries DirEntries, err error) error {
		if err != nil {
			// FIXME count errors and carry on for listing
			return err
		}
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

// DropboxHashSum list the Fs to the supplied writer
//
// Obeys includes and excludes
//
// Lists in parallel which may get them out of order
func DropboxHashSum(f Fs, w io.Writer) error {
	return hashLister(HashDropbox, f, w)
}

func hashLister(ht HashType, f Fs, w io.Writer) error {
	return ListFn(f, func(o Object) {
		Stats.Checking(o.Remote())
		sum, err := o.Hash(ht)
		Stats.DoneChecking(o.Remote())
		if err == ErrHashUnsupported {
			sum = "UNSUPPORTED"
		} else if err != nil {
			Debugf(o, "Failed to read %v: %v", ht, err)
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

// ConfigMaxDepth returns the depth to use for a recursive or non recursive listing.
func ConfigMaxDepth(recursive bool) int {
	depth := Config.MaxDepth
	if !recursive && depth < 0 {
		depth = 1
	}
	return depth
}

// ListDir lists the directories/buckets/containers in the Fs to the supplied writer
func ListDir(f Fs, w io.Writer) error {
	return Walk(f, "", false, ConfigMaxDepth(false), func(dirPath string, entries DirEntries, err error) error {
		if err != nil {
			// FIXME count errors and carry on for listing
			return err
		}
		entries.ForDir(func(dir Directory) {
			if dir != nil {
				syncFprintf(w, "%12d %13s %9d %s\n", dir.Size(), dir.ModTime().Format("2006-01-02 15:04:05"), dir.Items(), dir.Remote())
			}
		})
		return nil
	})
}

// logDirName returns an object for the logger
func logDirName(f Fs, dir string) interface{} {
	if dir != "" {
		return dir
	}
	return f
}

// Mkdir makes a destination directory or container
func Mkdir(f Fs, dir string) error {
	if Config.DryRun {
		Logf(logDirName(f, dir), "Not making directory as dry run is set")
		return nil
	}
	Debugf(logDirName(f, dir), "Making directory")
	err := f.Mkdir(dir)
	if err != nil {
		Stats.Error(err)
		return err
	}
	return nil
}

// TryRmdir removes a container but not if not empty.  It doesn't
// count errors but may return one.
func TryRmdir(f Fs, dir string) error {
	if Config.DryRun {
		Logf(logDirName(f, dir), "Not deleting as dry run is set")
		return nil
	}
	Debugf(logDirName(f, dir), "Removing directory")
	return f.Rmdir(dir)
}

// Rmdir removes a container but not if not empty
func Rmdir(f Fs, dir string) error {
	err := TryRmdir(f, dir)
	if err != nil {
		Stats.Error(err)
		return err
	}
	return err
}

// Purge removes a container and all of its contents
func Purge(f Fs) error {
	doFallbackPurge := true
	var err error
	if doPurge := f.Features().Purge; doPurge != nil {
		doFallbackPurge = false
		if Config.DryRun {
			Logf(f, "Not purging as --dry-run set")
		} else {
			err = doPurge()
			if err == ErrorCantPurge {
				doFallbackPurge = true
			}
		}
	}
	if doFallbackPurge {
		// DeleteFiles and Rmdir observe --dry-run
		err = DeleteFiles(listToChan(f))
		if err != nil {
			return err
		}
		err = Rmdirs(f, "")
	}
	if err != nil {
		Stats.Error(err)
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
	doMove := f.Features().Move
	if doMove == nil {
		log.Fatalf("Fs %v doesn't support Move", f)
	}
	ext := path.Ext(remote)
	base := remote[:len(remote)-len(ext)]
	for i, o := range objs {
		newName := fmt.Sprintf("%s-%d%s", base, i+1, ext)
		if !Config.DryRun {
			newObj, err := doMove(o, newName)
			if err != nil {
				Stats.Error(err)
				Errorf(o, "Failed to rename: %v", err)
				continue
			}
			Infof(newObj, "renamed from: %v", o)
		} else {
			Logf(remote, "Not renaming to %q as --dry-run", newName)
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
	Logf(remote, "Deleted %d extra copies", len(objs)-1)
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
			Logf(remote, "Deleting %d/%d identical duplicates (md5sum %q)", len(hashObjs)-1, len(hashObjs), md5sum)
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
func dedupeFindDuplicateDirs(f Fs) ([][]Directory, error) {
	duplicateDirs := [][]Directory{}
	err := Walk(f, "", true, Config.MaxDepth, func(dirPath string, entries DirEntries, err error) error {
		if err != nil {
			return err
		}
		dirs := map[string][]Directory{}
		entries.ForDir(func(d Directory) {
			dirs[d.Remote()] = append(dirs[d.Remote()], d)
		})
		for _, ds := range dirs {
			if len(ds) > 1 {
				duplicateDirs = append(duplicateDirs, ds)
			}
		}
		return nil
	})
	if err != nil {
		return nil, errors.Wrap(err, "find duplicate dirs")
	}
	return duplicateDirs, nil
}

// dedupeMergeDuplicateDirs merges all the duplicate directories found
func dedupeMergeDuplicateDirs(f Fs, duplicateDirs [][]Directory) error {
	mergeDirs := f.Features().MergeDirs
	if mergeDirs == nil {
		return errors.Errorf("%v: can't merge directories", f)
	}
	dirCacheFlush := f.Features().DirCacheFlush
	if dirCacheFlush == nil {
		return errors.Errorf("%v: can't flush dir cache", f)
	}
	for _, dirs := range duplicateDirs {
		if !Config.DryRun {
			Infof(dirs[0], "Merging contents of duplicate directories")
			err := mergeDirs(dirs)
			if err != nil {
				return errors.Wrap(err, "merge duplicate dirs")
			}
		} else {
			Infof(dirs[0], "NOT Merging contents of duplicate directories as --dry-run")
		}
	}
	dirCacheFlush()
	return nil
}

// Deduplicate interactively finds duplicate files and offers to
// delete all but one or rename them to be different. Only useful with
// Google Drive which can have duplicate file names.
func Deduplicate(f Fs, mode DeduplicateMode) error {
	Infof(f, "Looking for duplicates using %v mode.", mode)

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
		if Config.DryRun {
			break
		}
	}

	// Now find duplicate files
	files := map[string][]Object{}
	err := Walk(f, "", true, Config.MaxDepth, func(dirPath string, entries DirEntries, err error) error {
		if err != nil {
			return err
		}
		entries.ForObject(func(o Object) {
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
			Logf(remote, "Found %d duplicates - deleting identical copies", len(objs))
			objs = dedupeDeleteIdentical(remote, objs)
			if len(objs) <= 1 {
				Logf(remote, "All duplicates removed")
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

// listToChan will transfer all objects in the listing to the output
//
// If an error occurs, the error will be logged, and it will close the
// channel.
//
// If the error was ErrorDirNotFound then it will be ignored
func listToChan(f Fs) ObjectsChan {
	o := make(ObjectsChan, Config.Checkers)
	go func() {
		defer close(o)
		_ = Walk(f, "", true, Config.MaxDepth, func(dirPath string, entries DirEntries, err error) error {
			if err != nil {
				if err == ErrorDirNotFound {
					return nil
				}
				err = errors.Errorf("Failed to list: %v", err)
				Stats.Error(err)
				Errorf(nil, "%v", err)
				return nil
			}
			entries.ForObject(func(obj Object) {
				o <- obj
			})
			return nil
		})
	}()
	return o
}

// CleanUp removes the trash for the Fs
func CleanUp(f Fs) error {
	doCleanUp := f.Features().CleanUp
	if doCleanUp == nil {
		return errors.Errorf("%v doesn't support cleanup", f)
	}
	if Config.DryRun {
		Logf(f, "Not running cleanup as --dry-run set")
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
func Cat(f Fs, w io.Writer, offset, count int64) error {
	var mu sync.Mutex
	return ListFn(f, func(o Object) {
		var err error
		Stats.Transferring(o.Remote())
		defer func() {
			Stats.DoneTransferring(o.Remote(), err == nil)
		}()
		size := o.Size()
		thisOffset := offset
		if thisOffset < 0 {
			thisOffset += size
		}
		// size remaining is now reduced by thisOffset
		size -= thisOffset
		var options []OpenOption
		if thisOffset > 0 {
			options = append(options, &SeekOption{Offset: thisOffset})
		}
		in, err := o.Open(options...)
		if err != nil {
			Stats.Error(err)
			Errorf(o, "Failed to open: %v", err)
			return
		}
		if count >= 0 {
			in = &readCloser{Reader: &io.LimitedReader{R: in, N: count}, Closer: in}
			// reduce remaining size to count
			if size > count {
				size = count
			}
		}
		in = NewAccountSizeName(in, size, o.Remote()).WithBuffer() // account and buffer the transfer
		defer func() {
			err = in.Close()
			if err != nil {
				Stats.Error(err)
				Errorf(o, "Failed to close: %v", err)
			}
		}()
		// take the lock just before we output stuff, so at the last possible moment
		mu.Lock()
		defer mu.Unlock()
		_, err = io.Copy(w, in)
		if err != nil {
			Stats.Error(err)
			Errorf(o, "Failed to send to output: %v", err)
		}
	})
}

// Rcat reads data from the Reader until EOF and uploads it to a file on remote
func Rcat(fdst Fs, dstFileName string, in io.ReadCloser, modTime time.Time) (dst Object, err error) {
	Stats.Transferring(dstFileName)
	in = NewAccountSizeName(in, -1, dstFileName).WithBuffer()
	defer func() {
		Stats.DoneTransferring(dstFileName, err == nil)
		if otherErr := in.Close(); otherErr != nil {
			Debugf(fdst, "Rcat: failed to close source: %v", err)
		}
	}()

	hashOption := &HashesOption{Hashes: fdst.Hashes()}
	hash, err := NewMultiHasherTypes(fdst.Hashes())
	if err != nil {
		return nil, err
	}
	readCounter := NewCountingReader(in)
	trackingIn := io.TeeReader(readCounter, hash)

	compare := func(dst Object) error {
		src := NewStaticObjectInfo(dstFileName, modTime, int64(readCounter.BytesRead()), false, hash.Sums(), fdst)
		if !Equal(src, dst) {
			err = errors.Errorf("corrupted on transfer")
			Stats.Error(err)
			Errorf(dst, "%v", err)
			return err
		}
		return nil
	}

	// check if file small enough for direct upload
	buf := make([]byte, Config.StreamingUploadCutoff)
	if n, err := io.ReadFull(trackingIn, buf); err == io.EOF || err == io.ErrUnexpectedEOF {
		Debugf(fdst, "File to upload is small (%d bytes), uploading instead of streaming", n)
		src := NewMemoryObject(dstFileName, modTime, buf[:n])
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
		Debugf(fdst, "Target remote doesn't support streaming uploads, creating temporary local FS to spool file")
		tmpLocalFs, err := temporaryLocalFs()
		if err != nil {
			return nil, errors.Wrap(err, "Failed to create temporary local FS to spool file")
		}
		defer func() {
			err := Purge(tmpLocalFs)
			if err != nil {
				Infof(tmpLocalFs, "Failed to cleanup temporary FS: %v", err)
			}
		}()
		fStreamTo = tmpLocalFs
	}

	if Config.DryRun {
		Logf("stdin", "Not uploading as --dry-run")
		// prevents "broken pipe" errors
		_, err = io.Copy(ioutil.Discard, in)
		return nil, err
	}

	objInfo := NewStaticObjectInfo(dstFileName, modTime, -1, false, nil, nil)
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

// Rmdirs removes any empty directories (or directories only
// containing empty directories) under f, including f.
func Rmdirs(f Fs, dir string) error {
	dirEmpty := make(map[string]bool)
	dirEmpty[""] = true
	err := Walk(f, dir, true, Config.MaxDepth, func(dirPath string, entries DirEntries, err error) error {
		if err != nil {
			Stats.Error(err)
			Errorf(f, "Failed to list %q: %v", dirPath, err)
			return nil
		}
		for _, entry := range entries {
			switch x := entry.(type) {
			case Directory:
				// add a new directory as empty
				dir := x.Remote()
				_, found := dirEmpty[dir]
				if !found {
					dirEmpty[dir] = true
				}
			case Object:
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
			Stats.Error(err)
			Errorf(dir, "Failed to rmdir: %v", err)
			return err
		}
	}
	return nil
}

// moveOrCopyFile moves or copies a single file possibly to a new name
func moveOrCopyFile(fdst Fs, fsrc Fs, dstFileName string, srcFileName string, cp bool) (err error) {
	dstFilePath := path.Join(fdst.Root(), dstFileName)
	srcFilePath := path.Join(fsrc.Root(), srcFileName)
	if fdst.Name() == fsrc.Name() && dstFilePath == srcFilePath {
		Debugf(fdst, "don't need to copy/move %s, it is already at target location", dstFileName)
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
	if err == ErrorObjectNotFound {
		dstObj = nil
	} else if err != nil {
		return err
	}

	if NeedTransfer(dstObj, srcObj) {
		Stats.Transferring(srcFileName)
		_, err = Op(fdst, dstObj, dstFileName, srcObj)
		Stats.DoneTransferring(srcFileName, err == nil)
	} else {
		Stats.Checking(srcFileName)
		if !cp {
			err = DeleteFile(srcObj)
		}
		defer Stats.DoneChecking(srcFileName)
	}
	return err
}

// MoveFile moves a single file possibly to a new name
func MoveFile(fdst Fs, fsrc Fs, dstFileName string, srcFileName string) (err error) {
	return moveOrCopyFile(fdst, fsrc, dstFileName, srcFileName, false)
}

// CopyFile moves a single file possibly to a new name
func CopyFile(fdst Fs, fsrc Fs, dstFileName string, srcFileName string) (err error) {
	return moveOrCopyFile(fdst, fsrc, dstFileName, srcFileName, true)
}
