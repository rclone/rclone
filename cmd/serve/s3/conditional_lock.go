package s3

import (
	"context"
	"io"
	"sync"

	"github.com/rclone/gofakes3"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/operations"
	"golang.org/x/sync/semaphore"
)

type objectLock struct {
	rw   *semaphore.Weighted
	refs int
}

type keyedRWMutex struct {
	mu    sync.Mutex
	locks map[string]*objectLock
}

func (m *keyedRWMutex) acquire(key string) *objectLock {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.locks == nil {
		m.locks = make(map[string]*objectLock)
	}
	lock := m.locks[key]
	if lock == nil {
		lock = &objectLock{rw: semaphore.NewWeighted(writeWeight)}
		m.locks[key] = lock
	}
	lock.refs++
	return lock
}

func (m *keyedRWMutex) release(key string, lock *objectLock) {
	m.mu.Lock()
	defer m.mu.Unlock()
	lock.refs--
	if lock.refs == 0 {
		delete(m.locks, key)
	}
}

// Readers take one slot. A writer waits for all slots.
const writeWeight = 1 << 62

func (m *keyedRWMutex) lock(ctx context.Context, key string, weight int64) (func(), error) {
	lock := m.acquire(key)
	if err := lock.rw.Acquire(ctx, weight); err != nil {
		m.release(key, lock)
		return nil, err
	}
	return func() {
		lock.rw.Release(weight)
		m.release(key, lock)
	}, nil
}

type conditionalBackend struct {
	*s3Backend
	locks keyedRWMutex
	xfs   bool
}

func newConditionalBackend(backend *s3Backend) *conditionalBackend {
	b := &conditionalBackend{
		s3Backend: backend,
	}
	if v := backend.s.provider.VFS(); v != nil {
		b.xfs, _ = xfsCommitRangeVFSCapability(v)
	}
	return b
}

var _ gofakes3.Backend = (*conditionalBackend)(nil)
var _ gofakes3.MultipartBackend = (*conditionalBackend)(nil)

func (b *conditionalBackend) objectLockKey(ctx context.Context, bucket, object string) string {
	v := b.s.provider.VFS()
	if v == nil {
		return bucket + "\x00" + object
	}
	if v.Fs().Features().CaseInsensitive {
		// Filesystem name equivalence can differ from Go's Unicode case folding.
		return ""
	}
	ci := fs.GetConfig(ctx)
	return operations.ToNormal(bucket+"\x00"+object, !ci.NoUnicodeNormalization, ci.IgnoreCaseSync || v.Opt.CaseInsensitive)
}

func (b *conditionalBackend) GetObject(ctx context.Context, bucketName, objectName string, rangeRequest *gofakes3.ObjectRangeRequest) (*gofakes3.Object, error) {
	if !b.xfs {
		return b.s3Backend.GetObject(ctx, bucketName, objectName, rangeRequest)
	}
	// A streamed GET must not straddle an in-place XFS exchange.
	unlock, err := b.locks.lock(ctx, b.objectLockKey(ctx, bucketName, objectName), 1)
	if err != nil {
		return nil, err
	}
	obj, err := b.s3Backend.GetObject(ctx, bucketName, objectName, rangeRequest)
	if err != nil {
		unlock()
		return nil, err
	}
	obj.Contents = &unlockReadCloser{ReadCloser: obj.Contents, unlock: unlock}
	return obj, nil
}

func (b *conditionalBackend) HeadObject(ctx context.Context, bucketName, objectName string) (*gofakes3.Object, error) {
	if !b.xfs {
		return b.s3Backend.HeadObject(ctx, bucketName, objectName)
	}
	unlock, err := b.locks.lock(ctx, b.objectLockKey(ctx, bucketName, objectName), 1)
	if err != nil {
		return nil, err
	}
	defer unlock()
	return b.s3Backend.HeadObject(ctx, bucketName, objectName)
}

func (b *conditionalBackend) DeleteObject(ctx context.Context, bucketName, objectName string) (gofakes3.ObjectDeleteResult, error) {
	unlock, err := b.locks.lock(ctx, b.objectLockKey(ctx, bucketName, objectName), writeWeight)
	if err != nil {
		return gofakes3.ObjectDeleteResult{}, err
	}
	defer unlock()
	return b.s3Backend.DeleteObject(ctx, bucketName, objectName)
}

func (b *conditionalBackend) DeleteMulti(ctx context.Context, bucket string, objects ...string) (result gofakes3.MultiDeleteResult, err error) {
	// Multi-delete is not a transaction; hold only the current object's lock.
	for _, object := range objects {
		unlock, err := b.locks.lock(ctx, b.objectLockKey(ctx, bucket, object), writeWeight)
		if err != nil {
			return result, err
		}
		part, err := b.s3Backend.DeleteMulti(ctx, bucket, object)
		unlock()
		if err != nil {
			return result, err
		}
		result.Deleted = append(result.Deleted, part.Deleted...)
		result.Error = append(result.Error, part.Error...)
	}
	return result, nil
}

func (b *conditionalBackend) lockCopy(ctx context.Context, srcBucket, srcObject, dstBucket, dstObject string) (func(), error) {
	src := b.objectLockKey(ctx, srcBucket, srcObject)
	dst := b.objectLockKey(ctx, dstBucket, dstObject)
	if src == dst || !b.xfs {
		return b.locks.lock(ctx, dst, writeWeight)
	}
	// Opposite-direction copies acquire both locks in the same order.
	firstKey, secondKey := src, dst
	firstWeight, secondWeight := int64(1), int64(writeWeight)
	if dst < src {
		firstKey, secondKey = dst, src
		firstWeight, secondWeight = secondWeight, firstWeight
	}
	first, err := b.locks.lock(ctx, firstKey, firstWeight)
	if err != nil {
		return nil, err
	}
	second, err := b.locks.lock(ctx, secondKey, secondWeight)
	if err != nil {
		first()
		return nil, err
	}
	return func() { second(); first() }, nil
}

type unlockReadCloser struct {
	io.ReadCloser
	unlock func()
	once   sync.Once
}

func (r *unlockReadCloser) Close() error {
	err := r.ReadCloser.Close()
	r.once.Do(r.unlock)
	return err
}
