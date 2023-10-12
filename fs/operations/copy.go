package operations

import (
	"context"
	"errors"
	"fmt"
	"io"
	"path"
	"strings"
	"time"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/accounting"
	"github.com/rclone/rclone/fs/fserrors"
	"github.com/rclone/rclone/fs/hash"
	"github.com/rclone/rclone/lib/atexit"
	"github.com/rclone/rclone/lib/pacer"
	"github.com/rclone/rclone/lib/random"
)

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

// Used to remove a failed partial copy
//
// Returns whether the file was successfully removed or not
func removeFailedPartialCopy(ctx context.Context, f fs.Fs, remotePartial string) bool {
	o, err := f.NewObject(ctx, remotePartial)
	if errors.Is(err, fs.ErrorObjectNotFound) {
		return true
	} else if err != nil {
		fs.Infof(remotePartial, "Failed to remove failed partial copy: %s", err)
		return false
	}
	return removeFailedCopy(ctx, o)
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

	if dst != nil {
		remote = dst.Remote()
	}

	var (
		inplace       = true
		remotePartial = remote
	)
	if !ci.Inplace && f.Features().Move != nil && f.Features().PartialUploads && !strings.HasSuffix(remote, ".rclonelink") {
		if len(ci.PartialSuffix) > 16 {
			return nil, fmt.Errorf("expecting length of --partial-suffix to be not greater than %d but got %d", 16, len(ci.PartialSuffix))
		}

		// Avoid making the leaf name longer if it's already lengthy to avoid
		// trouble with file name length limits.
		suffix := "." + random.String(8) + ci.PartialSuffix
		base := path.Base(remotePartial)
		if len(base) > 100 {
			remotePartial = remotePartial[:len(remotePartial)-len(suffix)] + suffix
		} else {
			remotePartial += suffix
		}
		inplace = false
	}

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
		if doCopy := f.Features().Copy; doCopy != nil && (SameConfig(src.Fs(), f) || (SameRemoteType(src.Fs(), f) && (f.Features().ServerSideAcrossConfigs || ci.ServerSideAcrossConfigs))) {
			in := tr.Account(ctx, nil) // account the transfer
			in.ServerSideTransferStart()
			newDst, err = doCopy(ctx, src, remote)
			if err == nil {
				dst = newDst
				in.ServerSideCopyEnd(dst.Size()) // account the bytes for the server-side transfer
				_ = in.Close()
				inplace = true
			} else {
				_ = in.Close()
			}
			if errors.Is(err, fs.ErrorCantCopy) {
				tr.Reset(ctx) // skip incomplete accounting - will be overwritten by the manual copy below
			}
		} else {
			err = fs.ErrorCantCopy
		}
		// If can't server-side copy, do it manually
		if errors.Is(err, fs.ErrorCantCopy) {
			// Remove partial files on premature exit
			var atexitRemovePartial atexit.FnHandle
			if !inplace {
				atexitRemovePartial = atexit.Register(func() {
					ctx := context.Background()
					removeFailedPartialCopy(ctx, f, remotePartial)
				})
			}

			uploadOptions := []fs.OpenOption{hashOption}
			for _, option := range ci.UploadHeaders {
				uploadOptions = append(uploadOptions, option)
			}
			if ci.MetadataSet != nil {
				uploadOptions = append(uploadOptions, fs.MetadataOption(ci.MetadataSet))
			}

			if doMultiThreadCopy(ctx, f, src) {
				dst, err = multiThreadCopy(ctx, f, remotePartial, src, ci.MultiThreadStreams, tr, uploadOptions...)
				if err == nil {
					newDst = dst
				}
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
				in0, err = Open(ctx, src, options...)
				if err != nil {
					err = fmt.Errorf("failed to open source object: %w", err)
				} else {
					if src.Size() == -1 {
						// -1 indicates unknown size. Use Rcat to handle both remotes supporting and not supporting PutStream.
						if doUpdate {
							actionTaken = "Copied (Rcat, replaced existing)"
						} else {
							actionTaken = "Copied (Rcat, new)"
						}
						// Make any metadata to pass to rcat
						var meta fs.Metadata
						if ci.Metadata {
							meta, err = fs.GetMetadata(ctx, src)
							if err != nil {
								fs.Errorf(src, "Failed to read metadata: %v", err)
							}
						}
						// NB Rcat closes in0
						dst, err = Rcat(ctx, f, remotePartial, in0, src.ModTime(ctx), meta)
						newDst = dst
					} else {
						in := tr.Account(ctx, in0).WithBuffer() // account and buffer the transfer
						var wrappedSrc fs.ObjectInfo = src
						// We try to pass the original object if possible
						if src.Remote() != remotePartial {
							wrappedSrc = fs.NewOverrideRemote(src, remotePartial)
						}
						if doUpdate && inplace {
							err = dst.Update(ctx, in, wrappedSrc, uploadOptions...)
						} else {
							dst, err = f.Put(ctx, in, wrappedSrc, uploadOptions...)
						}
						if doUpdate {
							actionTaken = "Copied (replaced existing)"
						} else {
							actionTaken = "Copied (new)"
						}
						closeErr := in.Close()
						if err == nil {
							newDst = dst
							err = closeErr
						}
					}
				}
			}
			if !inplace {
				atexit.Unregister(atexitRemovePartial)
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
		if !inplace {
			removeFailedPartialCopy(ctx, f, remotePartial)
		}
		return newDst, err
	}

	// Verify sizes are the same after transfer
	if sizeDiffers(ctx, src, dst) {
		err = fmt.Errorf("corrupted on transfer: sizes differ %d vs %d", src.Size(), dst.Size())
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
			err = fmt.Errorf("corrupted on transfer: %v hash differ %q vs %q", hashType, srcSum, dstSum)
			fs.Errorf(dst, "%v", err)
			err = fs.CountError(err)
			removeFailedCopy(ctx, dst)
			return newDst, err
		}
	}

	// Move the copied file to its real destination.
	if err == nil && !inplace && remotePartial != remote {
		dst, err = f.Features().Move(ctx, newDst, remote)
		if err == nil {
			fs.Debugf(newDst, "renamed to: %s", remote)
			newDst = dst
		} else {
			fs.Errorf(newDst, "partial file rename failed: %v", err)
			err = fs.CountError(err)
			removeFailedCopy(ctx, newDst)
			return newDst, err
		}
	}

	if newDst != nil && src.String() != newDst.String() {
		actionTaken = fmt.Sprintf("%s to: %s", actionTaken, newDst.String())
	}
	fs.Infof(src, "%s%s", actionTaken, fs.LogValueHide("size", fs.SizeSuffix(src.Size())))
	return newDst, err
}

// CopyFile moves a single file possibly to a new name
func CopyFile(ctx context.Context, fdst fs.Fs, fsrc fs.Fs, dstFileName string, srcFileName string) (err error) {
	return moveOrCopyFile(ctx, fdst, fsrc, dstFileName, srcFileName, true)
}
