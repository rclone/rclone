// Generic operations on filesystems and objects

package fs

import (
	"fmt"
	"io"
	"mime"
	"path"
	"sync"
)

// Work out modify window for fses passed in - sets Config.ModifyWindow
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
		}
	}
	Debug(fs[0], "Modify window is %s\n", Config.ModifyWindow)
}

// Check the two files to see if the MD5sums are the same
//
// May return an error which will already have been logged
//
// If an error is returned it will return false
func CheckMd5sums(src, dst Object) (bool, error) {
	srcMd5, err := src.Md5sum()
	if err != nil {
		Stats.Error()
		Log(src, "Failed to calculate src md5: %s", err)
		return false, err
	}
	dstMd5, err := dst.Md5sum()
	if err != nil {
		Stats.Error()
		Log(dst, "Failed to calculate dst md5: %s", err)
		return false, err
	}
	// Debug("Src MD5 %s", srcMd5)
	// Debug("Dst MD5 %s", obj.Hash)
	return srcMd5 == dstMd5, nil
}

// Checks to see if the src and dst objects are equal by looking at
// size, mtime and MD5SUM
//
// If the src and dst size are different then it is considered to be
// not equal.
//
// If the size is the same and the mtime is the same then it is
// considered to be equal.  This is the heuristic rsync uses when
// not using --checksum.
//
// If the size is the same and and mtime is different or unreadable
// and the MD5SUM is the same then the file is considered to be equal.
// In this case the mtime on the dst is updated.
//
// Otherwise the file is considered to be not equal including if there
// were errors reading info.
func Equal(src, dst Object) bool {
	if src.Size() != dst.Size() {
		Debug(src, "Sizes differ")
		return false
	}

	// Size the same so check the mtime
	srcModTime := src.ModTime()
	dstModTime := dst.ModTime()
	dt := dstModTime.Sub(srcModTime)
	ModifyWindow := Config.ModifyWindow
	if dt >= ModifyWindow || dt <= -ModifyWindow {
		Debug(src, "Modification times differ by %s: %v, %v", dt, srcModTime, dstModTime)
	} else {
		Debug(src, "Size and modification time the same (differ by %s, within tolerance %s)", dt, ModifyWindow)
		return true
	}

	// mtime is unreadable or different but size is the same so
	// check the MD5SUM
	same, _ := CheckMd5sums(src, dst)
	if !same {
		Debug(src, "Md5sums differ")
		return false
	}

	// Size and MD5 the same but mtime different so update the
	// mtime of the dst object here
	dst.SetModTime(srcModTime)

	Debug(src, "Size and MD5SUM of src and dst objects identical")
	return true
}

// Returns a guess at the mime type from the extension
func MimeType(o Object) string {
	mimeType := mime.TypeByExtension(path.Ext(o.Remote()))
	if mimeType == "" {
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
	const maxTries = 10
	tries := 0
	doUpdate := dst != nil
tryAgain:
	in0, err := src.Open()
	if err != nil {
		Stats.Error()
		Log(src, "Failed to open: %s", err)
		return
	}
	in := NewAccount(in0) // account the transfer

	var actionTaken string
	if doUpdate {
		actionTaken = "Copied (updated existing)"
		err = dst.Update(in, src.ModTime(), src.Size())
	} else {
		actionTaken = "Copied (new)"
		dst, err = f.Put(in, src.Remote(), src.ModTime(), src.Size())
	}
	inErr := in.Close()
	// Retry if err returned a retry error
	if r, ok := err.(Retry); ok && r.Retry() && tries < maxTries {
		tries++
		Log(src, "Received error: %v - retrying %d/%d", err, tries, maxTries)
		if removeFailedCopy(dst) {
			// If we removed dst, then nil it out and note we are not updating
			dst = nil
			doUpdate = false
		}
		goto tryAgain
	}
	if err == nil {
		err = inErr
	}
	if err != nil {
		Stats.Error()
		Log(src, "Failed to copy: %s", err)
		removeFailedCopy(dst)
		return
	}

	// Verify sizes are the same after transfer
	if src.Size() != dst.Size() {
		Stats.Error()
		err = fmt.Errorf("Corrupted on transfer: sizes differ %d vs %d", src.Size(), dst.Size())
		Log(dst, "%s", err)
		removeFailedCopy(dst)
		return
	}

	// Verify md5sums are the same after transfer - ignoring blank md5sums
	srcMd5sum, md5sumErr := src.Md5sum()
	if md5sumErr != nil {
		Stats.Error()
		Log(src, "Failed to read md5sum: %s", md5sumErr)
	} else if srcMd5sum != "" {
		dstMd5sum, md5sumErr := dst.Md5sum()
		if md5sumErr != nil {
			Stats.Error()
			Log(dst, "Failed to read md5sum: %s", md5sumErr)
		} else if dstMd5sum != "" && srcMd5sum != dstMd5sum {
			Stats.Error()
			err = fmt.Errorf("Corrupted on transfer: md5sums differ %q vs %q", srcMd5sum, dstMd5sum)
			Log(dst, "%s", err)
			removeFailedCopy(dst)
			return
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
	// Check to see if changed or not
	if Equal(src, dst) {
		Debug(src, "Unchanged skipping")
		return
	}
	out <- pair
}

// Read Objects~s on in send to out if they need uploading
//
// FIXME potentially doing lots of MD5SUMS at once
func PairChecker(in ObjectPairChan, out ObjectPairChan, wg *sync.WaitGroup) {
	defer wg.Done()
	for pair := range in {
		src := pair.src
		Stats.Checking(src)
		checkOne(pair, out)
		Stats.DoneChecking(src)
	}
}

// Read Objects on in and copy them
func Copier(in ObjectPairChan, fdst Fs, wg *sync.WaitGroup) {
	defer wg.Done()
	for pair := range in {
		src := pair.src
		Stats.Transferring(src)
		if Config.DryRun {
			Debug(src, "Not copying as --dry-run")
		} else {

			Copy(fdst, pair.dst, src)
		}
		Stats.DoneTransferring(src)
	}
}

// Delete all the files passed in the channel
func DeleteFiles(to_be_deleted ObjectsChan) {
	var wg sync.WaitGroup
	wg.Add(Config.Transfers)
	for i := 0; i < Config.Transfers; i++ {
		go func() {
			defer wg.Done()
			for dst := range to_be_deleted {
				if Config.DryRun {

					Debug(dst, "Not deleting as --dry-run")
				} else {
					Stats.Checking(dst)
					err := dst.Remove()
					Stats.DoneChecking(dst)
					if err != nil {
						Stats.Error()
						Log(dst, "Couldn't delete: %s", err)
					} else {
						Debug(dst, "Deleted")
					}
				}
			}
		}()
	}
	Log(nil, "Waiting for deletions to finish")
	wg.Wait()
}

// Read a map of Object.Remote to Object for the given Fs
func readFilesMap(fs Fs) map[string]Object {
	files := make(map[string]Object)
	for o := range fs.List() {
		remote := o.Remote()
		if _, ok := files[remote]; !ok {
			files[remote] = o
		} else {
			Log(o, "Duplicate file detected")
		}
	}
	return files
}

// Syncs fsrc into fdst
//
// If Delete is true then it deletes any files in fdst that aren't in fsrc
func Sync(fdst, fsrc Fs, Delete bool) error {
	Log(fdst, "Starting Sync")

	err := fdst.Mkdir()
	if err != nil {
		Stats.Error()
		return err
	}

	Log(fdst, "Building file list")

	var delFiles map[string]Object

	if Config.CompareDest {
		// Read the destination files first
		// FIXME could do this in parallel and make it use less memory
		delFiles = readFilesMap(fdst)
	}

	// Read source files checking them off against dest files
	to_be_checked := make(ObjectPairChan, Config.Transfers)
	to_be_uploaded := make(ObjectPairChan, Config.Transfers)

	var checkerWg sync.WaitGroup
	checkerWg.Add(Config.Checkers)
	for i := 0; i < Config.Checkers; i++ {
		go PairChecker(to_be_checked, to_be_uploaded, &checkerWg)
	}

	var copierWg sync.WaitGroup
	copierWg.Add(Config.Transfers)
	for i := 0; i < Config.Transfers; i++ {
		go Copier(to_be_uploaded, fdst, &copierWg)
	}

	go func() {
		for src := range fsrc.List() {
			worked_on := false
			if Config.CompareDest {
				remote := src.Remote()
				dst, found := delFiles[remote]
				if found {
					delete(delFiles, remote)
					to_be_checked <- ObjectPair{src, dst}
					worked_on = true
				}
			}
			if !worked_on {
				// No need to check since doesn't exist or we want to override
				// blindly due to no CompareDest
				to_be_uploaded <- ObjectPair{src, nil}
			}
		}
		close(to_be_checked)
	}()

	Log(fdst, "Waiting for checks to finish")
	checkerWg.Wait()
	close(to_be_uploaded)
	Log(fdst, "Waiting for transfers to finish")
	copierWg.Wait()

	// Delete files if asked
	if Delete {
		if Stats.Errored() {
			Log(fdst, "Not deleting files as there were IO errors")
			return nil
		}

		// Delete the spare files
		toDelete := make(ObjectsChan, Config.Transfers)
		go func() {
			for _, fs := range delFiles {
				toDelete <- fs
			}
			close(toDelete)
		}()
		DeleteFiles(toDelete)
	}
	return nil
}

// Checks the files in fsrc and fdst according to Size and MD5SUM
func Check(fdst, fsrc Fs) error {
	Log(fdst, "Building file list")

	// Read the destination files first
	// FIXME could do this in parallel and make it use less memory
	dstFiles := readFilesMap(fdst)

	// Read the source files checking them against dstFiles
	// FIXME could do this in parallel and make it use less memory
	srcFiles := readFilesMap(fsrc)

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
		Log(dst, "File not in %v", fsrc)
	}

	Log(fsrc, "%d files not in %s", len(srcFiles), fdst)
	for _, src := range srcFiles {
		Stats.Error()
		Log(src, "File not in %v", fdst)
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
				dst, src := check[0], check[1]
				Stats.Checking(src)
				if src.Size() != dst.Size() {
					Stats.DoneChecking(src)
					Stats.Error()
					Log(src, "Sizes differ")
					continue
				}
				same, err := CheckMd5sums(src, dst)
				Stats.DoneChecking(src)
				if err != nil {
					continue
				}
				if !same {
					Stats.Error()
					Log(src, "Md5sums differ")
				}
				Debug(src, "OK")
			}
		}()
	}

	Log(fdst, "Waiting for checks to finish")
	checkerWg.Wait()
	Log(fdst, "%d differences found", Stats.GetErrors())
	if Stats.GetErrors() > 0 {
		return fmt.Errorf("%d differences found", Stats.GetErrors())
	}
	return nil
}

// List the Fs to the supplied function
//
// Lists in parallel which may get them out of order
func ListFn(f Fs, fn func(Object)) error {
	in := f.List()
	var wg sync.WaitGroup
	wg.Add(Config.Checkers)
	for i := 0; i < Config.Checkers; i++ {
		go func() {
			defer wg.Done()
			for o := range in {
				fn(o)
			}
		}()
	}
	wg.Wait()
	return nil
}

// mutex for synchronized output
var outMutex sync.Mutex

// Synchronized fmt.Fprintf
func syncFprintf(w io.Writer, format string, a ...interface{}) (n int, err error) {
	outMutex.Lock()
	defer outMutex.Unlock()
	return fmt.Fprintf(w, format, a...)
}

// List the Fs to stdout
//
// Shows size and path
//
// Lists in parallel which may get them out of order
func List(f Fs, w io.Writer) error {
	return ListFn(f, func(o Object) {
		syncFprintf(w, "%9d %s\n", o.Size(), o.Remote())
	})
}

// List the Fs to stdout
//
// Shows size, mod time and path
//
// Lists in parallel which may get them out of order
func ListLong(f Fs, w io.Writer) error {
	return ListFn(f, func(o Object) {
		Stats.Checking(o)
		modTime := o.ModTime()
		Stats.DoneChecking(o)
		syncFprintf(w, "%9d %s %s\n", o.Size(), modTime.Format("2006-01-02 15:04:05.000000000"), o.Remote())
	})
}

// List the Fs to stdout
//
// Produces the same output as the md5sum command
//
// Lists in parallel which may get them out of order
func Md5sum(f Fs, w io.Writer) error {
	return ListFn(f, func(o Object) {
		Stats.Checking(o)
		md5sum, err := o.Md5sum()
		Stats.DoneChecking(o)
		if err != nil {
			Debug(o, "Failed to read MD5: %v", err)
			md5sum = "UNKNOWN"
		}
		syncFprintf(w, "%32s  %s\n", md5sum, o.Remote())
	})
}

// List the directories/buckets/containers in the Fs to stdout
func ListDir(f Fs, w io.Writer) error {
	for dir := range f.ListDir() {
		syncFprintf(w, "%12d %13s %9d %s\n", dir.Bytes, dir.When.Format("2006-01-02 15:04:05"), dir.Count, dir.Name)
	}
	return nil
}

// Makes a destination directory or container
func Mkdir(f Fs) error {
	err := f.Mkdir()
	if err != nil {
		Stats.Error()
		return err
	}
	return nil
}

// Removes a container but not if not empty
func Rmdir(f Fs) error {
	if Config.DryRun {
		Log(f, "Not deleting as dry run is set")
	} else {
		err := f.Rmdir()
		if err != nil {
			Stats.Error()
			return err
		}
	}
	return nil
}

// Removes a container and all of its contents
//
// FIXME doesn't delete local directories
func Purge(f Fs) error {
	var err error
	if purger, ok := f.(Purger); ok {
		if Config.DryRun {
			Debug(f, "Not purging as --dry-run set")
		} else {
			err = purger.Purge()
		}
	} else {
		// DeleteFiles and Rmdir observe --dry-run
		DeleteFiles(f.List())
		err = Rmdir(f)
	}
	if err != nil {
		Stats.Error()
		return err
	}
	return nil
}
