// This file implements operations.Copy
//
// This is probably the most important operation in rclone.

package operations

import (
	"context"
	"errors"
	"fmt"
	"io"
	"path"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/accounting"
	"github.com/rclone/rclone/fs/fserrors"
	"github.com/rclone/rclone/fs/hash"
	"github.com/rclone/rclone/lib/atexit"
	"github.com/rclone/rclone/lib/pacer"
	"github.com/rclone/rclone/lib/random"
)

// State of the copy
type copy struct {
	f             fs.Fs                // destination fs.Fs
	dstFeatures   *fs.Features         // Features() for fs.Fs
	dst           fs.Object            // destination object to update, may be nil
	remote        string               // destination path, used if dst is nil
	src           fs.Object            // source object
	ci            *fs.ConfigInfo       // current config
	maxTries      int                  // max number of tries to do the copy
	doUpdate      bool                 // whether we are updating an existing file or not
	hashType      hash.Type            // common hash to use
	hashOption    *fs.HashesOption     // open option for the common hash
	tr            *accounting.Transfer // accounting for the transfer
	inplace       bool                 // set if we are updating inplace and not using a partial name
	remoteForCopy string               // the name used for the transfer, either remote or remote+".partial"
}

// Used to remove a failed copy
func (c *copy) removeFailedCopy(ctx context.Context, o fs.Object) {
	if o == nil {
		return
	}
	fs.Infof(o, "Removing failed copy")
	err := o.Remove(ctx)
	if err != nil {
		fs.Infof(o, "Failed to remove failed copy: %s", err)
	}
}

// Used to remove a failed partial copy
func (c *copy) removeFailedPartialCopy(ctx context.Context, f fs.Fs, remote string) {
	o, err := f.NewObject(ctx, remote)
	if errors.Is(err, fs.ErrorObjectNotFound) {
		// Assume object has been deleted
		return
	}
	if err != nil {
		fs.Infof(remote, "Failed to remove failed partial copy: %s", err)
		return
	}
	c.removeFailedCopy(ctx, o)
}

// TruncateString s to n bytes.
//
// If s is valid UTF-8 then this may truncate to fewer than n bytes to
// make the returned string also valid UTF-8.
func TruncateString(s string, n int) string {
	truncated := s[:n]
	if !utf8.ValidString(s) {
		// If input string wasn't valid UTF-8 then just return the truncation
		return truncated
	}
	for len(truncated) > 0 {
		if utf8.ValidString(truncated) {
			return truncated
		}
		// Remove 1 byte until valid
		truncated = truncated[:len(truncated)-1]
	}
	return truncated
}

// Check to see if we should be using a partial name and return the name for the copy and the inplace flag
func (c *copy) checkPartial() (remoteForCopy string, inplace bool, err error) {
	remoteForCopy = c.remote
	if c.ci.Inplace || c.dstFeatures.Move == nil || !c.dstFeatures.PartialUploads || strings.HasSuffix(c.remote, ".rclonelink") {
		return remoteForCopy, true, nil
	}
	if len(c.ci.PartialSuffix) > 16 {
		return remoteForCopy, true, fmt.Errorf("expecting length of --partial-suffix to be not greater than %d but got %d", 16, len(c.ci.PartialSuffix))
	}
	// Avoid making the leaf name longer if it's already lengthy to avoid
	// trouble with file name length limits.
	suffix := "." + random.String(8) + c.ci.PartialSuffix
	base := path.Base(c.remoteForCopy)
	if len(base) > 100 {
		remoteForCopy = TruncateString(c.remoteForCopy, len(c.remoteForCopy)-len(suffix)) + suffix
	} else {
		remoteForCopy += suffix
	}
	return remoteForCopy, false, nil
}

// Check to see if we have hit max transfer limits
func (c *copy) checkLimits(ctx context.Context) (err error) {
	if c.ci.MaxTransfer < 0 {
		return nil
	}
	var bytesSoFar int64
	if c.ci.CutoffMode == fs.CutoffModeCautious {
		bytesSoFar = accounting.Stats(ctx).GetBytesWithPending() + c.src.Size()
	} else {
		bytesSoFar = accounting.Stats(ctx).GetBytes()
	}
	if bytesSoFar >= int64(c.ci.MaxTransfer) {
		if c.ci.CutoffMode == fs.CutoffModeHard {
			return accounting.ErrorMaxTransferLimitReachedFatal
		}
		return accounting.ErrorMaxTransferLimitReachedGraceful
	}
	return nil
}

// Server side copy c.src to (c.f, c.remoteForCopy) if possible or return fs.ErrorCantCopy if not
func (c *copy) serverSideCopy(ctx context.Context) (actionTaken string, newDst fs.Object, err error) {
	doCopy := c.dstFeatures.Copy
	serverSideCopyOK := false
	if doCopy == nil {
		serverSideCopyOK = false
	} else if SameConfig(c.src.Fs(), c.f) {
		serverSideCopyOK = true
	} else if SameRemoteType(c.src.Fs(), c.f) {
		serverSideCopyOK = c.dstFeatures.ServerSideAcrossConfigs || c.ci.ServerSideAcrossConfigs
	}
	if !serverSideCopyOK {
		return actionTaken, nil, fs.ErrorCantCopy
	}
	in := c.tr.Account(ctx, nil) // account the transfer
	in.ServerSideTransferStart()
	newDst, err = doCopy(ctx, c.src, c.remoteForCopy)
	if err == nil {
		in.ServerSideCopyEnd(newDst.Size()) // account the bytes for the server-side transfer
	}
	_ = in.Close()
	if errors.Is(err, fs.ErrorCantCopy) {
		c.tr.Reset(ctx) // skip incomplete accounting - will be overwritten by the manual copy
	}
	actionTaken = "Copied (server-side copy)"
	return actionTaken, newDst, err
}

// Copy c.src to (c.f, c.remoteForCopy) using multiThreadCopy
func (c *copy) multiThreadCopy(ctx context.Context, uploadOptions []fs.OpenOption) (actionTaken string, newDst fs.Object, err error) {
	newDst, err = multiThreadCopy(ctx, c.f, c.remoteForCopy, c.src, c.ci.MultiThreadStreams, c.tr, uploadOptions...)
	if c.doUpdate {
		actionTaken = "Multi-thread Copied (replaced existing)"
	} else {
		actionTaken = "Multi-thread Copied (new)"
	}
	return actionTaken, newDst, err
}

// Copy the stream from in to (c.f, c.remoteForCopy) and close it
//
// Use Rcat to handle both remotes supporting and not supporting PutStream.
func (c *copy) rcat(ctx context.Context, in io.ReadCloser) (actionTaken string, newDst fs.Object, err error) {
	// Make any metadata to pass to rcat
	var meta fs.Metadata
	if c.ci.Metadata {
		meta, err = fs.GetMetadata(ctx, c.src)
		if err != nil {
			fs.Errorf(c.src, "Failed to read metadata: %v", err)
		}
	}

	// NB Rcat closes in0
	newDst, err = Rcat(ctx, c.f, c.remoteForCopy, in, c.src.ModTime(ctx), meta)
	if c.doUpdate {
		actionTaken = "Copied (Rcat, replaced existing)"
	} else {
		actionTaken = "Copied (Rcat, new)"
	}
	return actionTaken, newDst, err
}

// Copy the stream from in to (c.f, c.remoteForCopy) and close it
func (c *copy) updateOrPut(ctx context.Context, in io.ReadCloser, uploadOptions []fs.OpenOption) (actionTaken string, newDst fs.Object, err error) {
	// account and buffer the transfer
	inAcc := c.tr.Account(ctx, in).WithBuffer()
	var wrappedSrc fs.ObjectInfo = c.src

	// We try to pass the original object if possible
	if c.src.Remote() != c.remoteForCopy {
		wrappedSrc = fs.NewOverrideRemote(c.src, c.remoteForCopy)
	}
	if c.doUpdate && c.inplace {
		err = c.dst.Update(ctx, inAcc, wrappedSrc, uploadOptions...)
		// Make sure newDst is c.dst since we updated it
		if err == nil {
			newDst = c.dst
		}
	} else {
		newDst, err = c.f.Put(ctx, inAcc, wrappedSrc, uploadOptions...)
	}
	closeErr := inAcc.Close()
	if err == nil {
		err = closeErr
	}
	if c.doUpdate {
		actionTaken = "Copied (replaced existing)"
	} else {
		actionTaken = "Copied (new)"
	}
	return actionTaken, newDst, err
}

// Do a manual copy by reading the bytes and writing them
func (c *copy) manualCopy(ctx context.Context) (actionTaken string, newDst fs.Object, err error) {
	// Remove partial files on premature exit
	if !c.inplace {
		defer atexit.Unregister(atexit.Register(func() {
			ctx := context.Background()
			c.removeFailedPartialCopy(ctx, c.f, c.remoteForCopy)
		}))
	}

	// Options for the upload
	uploadOptions := []fs.OpenOption{c.hashOption}
	for _, option := range c.ci.UploadHeaders {
		uploadOptions = append(uploadOptions, option)
	}
	if c.ci.MetadataSet != nil {
		uploadOptions = append(uploadOptions, fs.MetadataOption(c.ci.MetadataSet))
	}

	// Options for the download
	downloadOptions := []fs.OpenOption{c.hashOption}
	for _, option := range c.ci.DownloadHeaders {
		downloadOptions = append(downloadOptions, option)
	}

	if doMultiThreadCopy(ctx, c.f, c.src) {
		return c.multiThreadCopy(ctx, uploadOptions)
	}

	var in io.ReadCloser
	in, err = Open(ctx, c.src, downloadOptions...)
	if err != nil {
		return actionTaken, nil, fmt.Errorf("failed to open source object: %w", err)
	}

	// Note that c.rcat and c.updateOrPut close in
	if c.src.Size() == -1 {
		return c.rcat(ctx, in)
	}
	return c.updateOrPut(ctx, in, uploadOptions)
}

// Verify the copy
func (c *copy) verify(ctx context.Context, newDst fs.Object) (err error) {
	// Verify sizes are the same after transfer
	if sizeDiffers(ctx, c.src, newDst) {
		return fmt.Errorf("corrupted on transfer: sizes differ %d vs %d", c.src.Size(), newDst.Size())
	}
	// Verify hashes are the same after transfer - ignoring blank hashes
	if c.hashType != hash.None {
		// checkHashes has logs and counts errors
		equal, _, srcSum, dstSum, _ := checkHashes(ctx, c.src, newDst, c.hashType)
		if !equal {
			return fmt.Errorf("corrupted on transfer: %v hash differ %q vs %q", c.hashType, srcSum, dstSum)
		}
	}
	return nil
}

// copy src object to dst or f if nil.  If dst is nil then it uses
// remote as the name of the new object.
//
// It returns the destination object if possible.  Note that this may
// be nil.
func (c *copy) copy(ctx context.Context) (newDst fs.Object, err error) {
	var actionTaken string
	retry := true
	for tries := 0; retry && tries < c.maxTries; tries++ {
		// Check we haven't hit any accounting limits
		err = c.checkLimits(ctx)
		if err != nil {
			return nil, err
		}

		// Try server side copy
		actionTaken, newDst, err = c.serverSideCopy(ctx)

		// If can't server-side copy, do it manually
		if errors.Is(err, fs.ErrorCantCopy) {
			actionTaken, newDst, err = c.manualCopy(ctx)
		}

		// End if ctx is in error
		if fserrors.ContextError(ctx, &err) {
			break
		}

		// Retry if err returned a retry error
		retry = false
		if fserrors.IsRetryError(err) || fserrors.ShouldRetry(err) {
			retry = true
		} else if t, ok := pacer.IsRetryAfter(err); ok {
			fs.Debugf(c.src, "Sleeping for %v (as indicated by the server) to obey Retry-After error: %v", t, err)
			time.Sleep(t)
			retry = true
		}
		if retry {
			fs.Debugf(c.src, "Received error: %v - low level retry %d/%d", err, tries, c.maxTries)
			c.tr.Reset(ctx) // skip incomplete accounting - will be overwritten by retry
			continue
		}
	}
	if err != nil {
		err = fs.CountError(err)
		fs.Errorf(c.src, "Failed to copy: %v", err)
		if !c.inplace {
			c.removeFailedPartialCopy(ctx, c.f, c.remoteForCopy)
		}
		return newDst, err
	}

	// Verify the copy
	err = c.verify(ctx, newDst)
	if err != nil {
		fs.Errorf(newDst, "%v", err)
		err = fs.CountError(err)
		c.removeFailedCopy(ctx, newDst)
		return nil, err
	}

	// Move the copied file to its real destination.
	if !c.inplace && c.remoteForCopy != c.remote {
		movedNewDst, err := c.dstFeatures.Move(ctx, newDst, c.remote)
		if err != nil {
			fs.Errorf(newDst, "partial file rename failed: %v", err)
			err = fs.CountError(err)
			c.removeFailedCopy(ctx, newDst)
			return nil, err
		}
		fs.Debugf(newDst, "renamed to: %s", c.remote)
		newDst = movedNewDst
	}

	// Log what we have done
	if newDst != nil && c.src.String() != newDst.String() {
		actionTaken = fmt.Sprintf("%s to: %s", actionTaken, newDst.String())
	}
	fs.Infof(c.src, "%s%s", actionTaken, fs.LogValueHide("size", fs.SizeSuffix(c.src.Size())))

	return newDst, nil
}

// Copy src object to dst or f if nil.  If dst is nil then it uses
// remote as the name of the new object.
//
// It returns the destination object if possible.  Note that this may
// be nil.
func Copy(ctx context.Context, f fs.Fs, dst fs.Object, remote string, src fs.Object) (newDst fs.Object, err error) {
	ci := fs.GetConfig(ctx)
	tr := accounting.Stats(ctx).NewTransfer(src, f)
	defer func() {
		tr.Done(ctx, err)
	}()
	if SkipDestructive(ctx, src, "copy") {
		in := tr.Account(ctx, nil)
		in.DryRun(src.Size())
		return newDst, nil
	}
	c := &copy{
		f:           f,
		dstFeatures: f.Features(),
		dst:         dst,
		remote:      remote,
		src:         src,
		ci:          ci,
		tr:          tr,
		maxTries:    ci.LowLevelRetries,
		doUpdate:    dst != nil,
	}
	c.hashType, c.hashOption = CommonHash(ctx, f, src.Fs())
	if c.dst != nil {
		c.remote = c.dst.Remote()
	}
	// Are we using partials?
	//
	// If so set the flag and update the name we use for the copy
	c.remoteForCopy, c.inplace, err = c.checkPartial()
	if err != nil {
		return nil, err
	}
	// Do the copy now everything is set up
	return c.copy(ctx)
}

// CopyFile moves a single file possibly to a new name
func CopyFile(ctx context.Context, fdst fs.Fs, fsrc fs.Fs, dstFileName string, srcFileName string) (err error) {
	return moveOrCopyFile(ctx, fdst, fsrc, dstFileName, srcFileName, true)
}
