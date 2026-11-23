package s3

import (
	"context"
	"net/http"
	"net/url"
	"strings"
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
}

func newConditionalBackend(b *s3Backend) *conditionalBackend {
	return &conditionalBackend{s3Backend: b}
}

var _ gofakes3.Backend = (*conditionalBackend)(nil)
var _ gofakes3.MultipartBackend = (*conditionalBackend)(nil)

func (b *conditionalBackend) objectLockKey(ctx context.Context, bucket, object string) string {
	v, err := b.s.getVFS(ctx)
	if err != nil {
		return bucket + "\x00" + object
	}
	if v.Fs().Features().CaseInsensitive {
		// Valid S3 bucket names are ASCII. Filesystem case folding of keys can differ from Unicode folding.
		return fs.ConfigString(v.Fs()) + "\x00" + strings.ToLower(bucket)
	}
	ci := fs.GetConfig(ctx)
	return fs.ConfigString(v.Fs()) + "\x00" + operations.ToNormal(bucket+"\x00"+object, !ci.NoUnicodeNormalization, ci.IgnoreCaseSync || v.Opt.CaseInsensitive)
}

// A COPY's source HEAD and backend copy must observe the same object.
type copyLeaseKey struct{}
type copyLease struct {
	path   *url.URL
	unlock func()
}

func (b *conditionalBackend) HeadObject(ctx context.Context, bucket, key string) (*gofakes3.Object, error) {
	if lease, _ := ctx.Value(copyLeaseKey{}).(*copyLease); lease != nil && lease.unlock == nil {
		// gofakes3 has already rewritten virtual-host requests to path style.
		dstBucket, dstKey, _ := strings.Cut(strings.TrimLeft(lease.path.Path, "/"), "/")
		unlock, err := b.lockCopy(ctx, bucket, key, dstBucket, dstKey)
		if err != nil {
			return nil, err
		}
		lease.unlock = unlock
	}
	c, _ := ctx.Value(writeConditionsKey{}).(*writeConditions)
	lease, _ := ctx.Value(copyLeaseKey{}).(*copyLease)
	if c == nil || lease != nil {
		return b.s3Backend.HeadObject(ctx, bucket, key)
	}
	// gofakes3 preflights PUT and multipart completion with HEAD.
	if c.err != nil {
		return nil, c.err
	}
	obj, err := b.s3Backend.HeadObject(ctx, bucket, key)
	if err == nil && c.needsETag() && len(obj.Hash) == 0 {
		v, err := b.s.getVFS(ctx)
		if err == nil {
			err = b.checkWrite(ctx, v, bucket, key)
		}
		if err != nil {
			_ = obj.Contents.Close()
			return nil, err
		}
	}
	return obj, err
}

func withConditions(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut && r.Method != http.MethodPost {
			next.ServeHTTP(w, r)
			return
		}
		ctx := r.Context()
		multipart := r.URL.Query().Get("uploadId") != ""
		if (r.Method == http.MethodPut && !multipart) || (r.Method == http.MethodPost && multipart) {
			// Field lines combine with commas in both HTTP and the SigV4 canonical request.
			// gofakes3's preflight reads the first field line with Header.Get.
			for _, name := range []string{"If-Match", "If-None-Match"} {
				if values := r.Header.Values(name); len(values) > 1 {
					fields := make([]string, len(values))
					for i, value := range values {
						fields[i] = strings.TrimSpace(value)
					}
					r.Header.Set(name, strings.Join(fields, ","))
				}
			}
			if c := parseWriteConditions(r.Header); c != nil {
				ctx = context.WithValue(ctx, writeConditionsKey{}, c)
			}
		}
		if r.Method == http.MethodPut && r.Header.Get("X-Amz-Copy-Source") != "" && !multipart {
			lease := &copyLease{path: r.URL}
			defer func() {
				if lease.unlock != nil {
					lease.unlock()
				}
			}()
			ctx = context.WithValue(ctx, copyLeaseKey{}, lease)
		}
		next.ServeHTTP(w, r.WithContext(ctx))
	})
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
	if src == dst {
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
