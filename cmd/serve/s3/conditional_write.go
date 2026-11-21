package s3

import (
	"context"
	"errors"
	"io"
	"os"
	"syscall"

	"github.com/rclone/gofakes3"
	"github.com/rclone/rclone/backend/local"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/hash"
	"github.com/rclone/rclone/vfs"
)

type xfsCommitKey struct{}

// SupportsWriteConditions reports that writes validate the destination under its lock.
func (b *conditionalBackend) SupportsWriteConditions() bool {
	// Distinct proxy VFS instances may cache different versions of one object.
	return b.s.provider.VFS() != nil
}

func (b *conditionalBackend) checkConditions(ctx context.Context, v *vfs.VFS, bucket, object string, conditions *gofakes3.WriteConditions) (exists bool, err error) {
	if _, err = v.Stat(bucket); err != nil {
		if errors.Is(err, vfs.ENOENT) {
			return false, gofakes3.BucketNotFound(bucket)
		}
		return false, err
	}
	fp, err := bucketObjectPath(bucket, object)
	if err != nil {
		return false, err
	}
	node, err := v.Stat(fp)
	if err != nil && !errors.Is(err, vfs.ENOENT) {
		return false, err
	}
	if errors.Is(err, vfs.ENOENT) && (v.Fs().Features().CaseInsensitive || v.Fs().Features().IsLocal) {
		// A VFS listing can miss a spelling the backing filesystem accepts.
		// Treating that name as absent would let a create-only request replace it.
		_, lookupErr := v.Fs().NewObject(ctx, fp)
		switch {
		case lookupErr == nil:
			return true, gofakes3.ErrInvalidArgument
		case errors.Is(lookupErr, fs.ErrorObjectNotFound), errors.Is(lookupErr, fs.ErrorIsDir):
		default:
			return false, lookupErr
		}
	}
	exists = err == nil && node.IsFile()
	var etag string
	if exists && conditions.NeedsETag() {
		if om, ok := b.tenant(ctx).getMeta(ctx, fp, node); ok && om.hash != nil {
			etag = quoteETag(om.hash)
		} else {
			hashType := b.s.etagHash(v)
			if hashType == hash.None || !v.Fs().Hashes().Contains(hashType) {
				return true, gofakes3.ErrNotImplemented
			}
			sum, err := readFileHash(ctx, node, hashType)
			if err != nil {
				if errors.Is(err, hash.ErrUnsupported) {
					return true, gofakes3.ErrNotImplemented
				}
				fs.Errorf(node, "Failed to read destination ETag: %v", err)
				return true, gofakes3.ErrInternal
			}
			if sum == "" {
				return true, gofakes3.ErrInternal
			}
			etag = `"` + sum + `"`
		}
	}
	if err := ctx.Err(); err != nil {
		return exists, err
	}
	return exists, conditions.Check(exists, etag)
}

func (b *conditionalBackend) beginConditionalWrite(ctx context.Context, v *vfs.VFS, bucket, object string) (guard *local.XFSCommit, err error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	conditions := gofakes3.WriteConditionsFromContext(ctx)
	if conditions == nil {
		return nil, nil
	}
	if _, marker, err := bucketDirMarkerPath(bucket, object); marker {
		if err != nil {
			return nil, err
		}
		obj, err := b.s3Backend.HeadObject(ctx, bucket, object)
		if gofakes3.HasErrorCode(err, gofakes3.ErrNoSuchKey) {
			return nil, conditions.Check(false, "")
		}
		if err != nil {
			return nil, err
		}
		defer obj.Contents.Close()
		return nil, conditions.Check(true, quoteETag(obj.Hash))
	}
	if b.xfs {
		fp, err := bucketObjectPath(bucket, object)
		if err != nil {
			return nil, err
		}
		// Sample before checking the ETag, including when re-reading VFS metadata.
		guard, err = startXFSCommitGuard(v.Fs(), fp)
		// A read-only inode may still be replaceable by rename.
		if err != nil && !os.IsNotExist(err) && !os.IsPermission(err) && !xfsCommitUnavailable(err) {
			return nil, err
		}
		if guard != nil {
			b.forgetPath(v, fp)
		}
	}
	exists, err := b.checkConditions(ctx, v, bucket, object, conditions)
	if err != nil || !exists {
		_ = guard.Close()
		guard = nil
	}
	return guard, err
}

// publishObject installs a completed staging object at its destination.
func (b *s3Backend) publishObject(ctx context.Context, v *vfs.VFS, staging, target string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if guard, _ := ctx.Value(xfsCommitKey{}).(*local.XFSCommit); guard != nil {
		err := guard.Publish(staging)
		switch {
		case err == nil:
			// Staging contains the old contents after the exchange.
			b.forgetPath(v, target)
			b.forgetPath(v, staging)
			if err := v.Remove(staging); err != nil {
				fs.Errorf(staging, "Failed to remove XFS staging object: %v", err)
			}
			return nil
		case errors.Is(err, syscall.EBUSY):
			return gofakes3.ErrConditionalRequestConflict
		case !xfsCommitUnavailable(err):
			return err
		}
		// The staged inode carries the metadata that rename must publish.
	}
	return v.Rename(staging, target)
}

func (b *conditionalBackend) PutObject(ctx context.Context, bucket, object string, meta map[string]string, input io.Reader, size int64) (result gofakes3.PutObjectResult, err error) {
	unlock, err := b.locks.lock(ctx, b.objectLockKey(ctx, bucket, object), writeWeight)
	if err != nil {
		return result, err
	}
	defer unlock()
	v, err := b.s.getVFS(ctx)
	if err != nil {
		return result, err
	}
	guard, err := b.beginConditionalWrite(ctx, v, bucket, object)
	if err != nil {
		return result, err
	}
	if guard != nil {
		defer func() { _ = guard.Close() }()
		ctx = context.WithValue(ctx, xfsCommitKey{}, guard)
	}
	return b.s3Backend.PutObject(ctx, bucket, object, meta, input, size)
}

func (b *conditionalBackend) CopyObject(ctx context.Context, srcBucket, srcKey, dstBucket, dstKey string, meta map[string]string) (result gofakes3.CopyObjectResult, err error) {
	unlock, err := b.lockCopy(ctx, srcBucket, srcKey, dstBucket, dstKey)
	if err != nil {
		return result, err
	}
	defer unlock()
	v, err := b.s.getVFS(ctx)
	if err != nil {
		return result, err
	}
	guard, err := b.beginConditionalWrite(ctx, v, dstBucket, dstKey)
	if err != nil {
		return result, err
	}
	if guard != nil {
		defer func() { _ = guard.Close() }()
		ctx = context.WithValue(ctx, xfsCommitKey{}, guard)
	}
	return b.s3Backend.CopyObject(ctx, srcBucket, srcKey, dstBucket, dstKey, meta)
}

func (b *conditionalBackend) CompleteMultipartUpload(ctx context.Context, bucket, object string, uploadID gofakes3.UploadID, input *gofakes3.CompleteMultipartUploadRequest) (gofakes3.VersionID, string, error) {
	v, err := b.s.getVFS(ctx)
	if err != nil {
		return "", "", err
	}
	up, err := b.loadUpload(ctx, uploadID)
	if err != nil {
		return "", "", err
	}
	if up.bucket != bucket || up.key != object || up.vfs != v {
		return "", "", gofakes3.ErrNoSuchUpload
	}
	if err := up.startActivity(); err != nil {
		return "", "", err
	}
	defer up.endActivity()
	unlock, err := b.locks.lock(ctx, b.objectLockKey(ctx, bucket, object), writeWeight)
	if err != nil {
		return "", "", err
	}
	defer unlock()
	// An earlier completion or abort may have consumed this upload while we waited.
	if _, err := b.loadUpload(ctx, uploadID); err != nil {
		return "", "", err
	}
	guard, err := b.beginConditionalWrite(ctx, v, bucket, object)
	if err != nil {
		return "", "", err
	}
	if guard != nil {
		defer func() { _ = guard.Close() }()
		ctx = context.WithValue(ctx, xfsCommitKey{}, guard)
	}
	return b.completeMultipartUpload(ctx, up, uploadID, input)
}
