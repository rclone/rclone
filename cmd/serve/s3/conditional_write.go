package s3

import (
	"context"
	"errors"
	"io"

	"github.com/rclone/gofakes3"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/hash"
	"github.com/rclone/rclone/vfs"
)

func (b *conditionalBackend) checkConditions(ctx context.Context, v *vfs.VFS, bucket, object string, conditions *writeConditions) error {
	if _, err := v.Stat(bucket); err != nil {
		if errors.Is(err, vfs.ENOENT) {
			return gofakes3.BucketNotFound(bucket)
		}
		return err
	}
	fp, err := bucketObjectPath(bucket, object)
	if err != nil {
		return err
	}
	node, err := v.Stat(fp)
	if err != nil && !errors.Is(err, vfs.ENOENT) {
		return err
	}
	if errors.Is(err, vfs.ENOENT) && (v.Fs().Features().CaseInsensitive || v.Fs().Features().IsLocal) {
		// A VFS listing can miss a spelling the backing filesystem accepts.
		// Treating that name as absent would let a create-only request replace it.
		_, lookupErr := v.Fs().NewObject(ctx, fp)
		switch {
		case lookupErr == nil:
			return gofakes3.ErrInvalidArgument
		case errors.Is(lookupErr, fs.ErrorObjectNotFound), errors.Is(lookupErr, fs.ErrorIsDir):
		default:
			return lookupErr
		}
	}
	exists := err == nil && node.IsFile()
	var etag string
	if exists && conditions.needsETag() {
		if om, ok := b.tenant(ctx).getMeta(ctx, fp, node); ok && om.hash != nil {
			etag = quoteETag(om.hash)
		} else {
			hashType := b.s.etagHash(v)
			if hashType == hash.None || !v.Fs().Hashes().Contains(hashType) {
				return gofakes3.ErrNotImplemented
			}
			sum, err := readFileHash(ctx, node, hashType)
			if err != nil {
				if errors.Is(err, hash.ErrUnsupported) {
					return gofakes3.ErrNotImplemented
				}
				fs.Errorf(node, "Failed to read destination ETag: %v", err)
				return gofakes3.ErrInternal
			}
			if sum == "" {
				return gofakes3.ErrInternal
			}
			etag = `"` + sum + `"`
		}
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	return conditions.check(exists, etag)
}

func (b *conditionalBackend) checkWrite(ctx context.Context, v *vfs.VFS, bucket, object string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	conditions, _ := ctx.Value(writeConditionsKey{}).(*writeConditions)
	if conditions == nil {
		return nil
	}
	if conditions.err != nil {
		return conditions.err
	}
	if _, marker, err := bucketDirMarkerPath(bucket, object); marker {
		if err != nil {
			return err
		}
		obj, err := b.s3Backend.HeadObject(ctx, bucket, object)
		if gofakes3.HasErrorCode(err, gofakes3.ErrNoSuchKey) {
			return conditions.check(false, "")
		}
		if err != nil {
			return err
		}
		defer obj.Contents.Close()
		return conditions.check(true, quoteETag(obj.Hash))
	}
	return b.checkConditions(ctx, v, bucket, object, conditions)
}

func (b *conditionalBackend) PutObject(ctx context.Context, bucket, object string, meta map[string]string, input io.Reader, size int64) (result gofakes3.PutObjectResult, err error) {
	return b.s3Backend.putObject(ctx, bucket, object, meta, input, size, func(v *vfs.VFS) (func(), error) {
		unlock, err := b.locks.lock(ctx, b.objectLockKey(ctx, bucket, object), writeWeight)
		if err != nil {
			return nil, err
		}
		if err := b.checkWrite(ctx, v, bucket, object); err != nil {
			unlock()
			return nil, err
		}
		return unlock, nil
	})
}

func (b *conditionalBackend) CopyObject(ctx context.Context, srcBucket, srcKey, dstBucket, dstKey string, meta map[string]string) (result gofakes3.CopyObjectResult, err error) {
	if lease, _ := ctx.Value(copyLeaseKey{}).(*copyLease); lease == nil || lease.unlock == nil {
		unlock, err := b.lockCopy(ctx, srcBucket, srcKey, dstBucket, dstKey)
		if err != nil {
			return result, err
		}
		defer unlock()
	}
	v, err := b.s.getVFS(ctx)
	if err != nil {
		return result, err
	}
	if err := b.checkWrite(ctx, v, dstBucket, dstKey); err != nil {
		return result, err
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
	if err := b.checkWrite(ctx, v, bucket, object); err != nil {
		return "", "", err
	}
	return b.completeMultipartUpload(ctx, up, uploadID, input)
}
