package s3

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"maps"
	"strings"
	"sync"
	"time"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/vfs"
)

// metaRecord is the metadata stored for an object, with the size and
// modtime of the file it was stored for so that stale records are spotted.
type metaRecord struct {
	Meta    map[string]string `json:"meta"`
	Size    int64             `json:"size"`
	ModTime time.Time         `json:"modtime"`
}

// metaModTimeSlack is the smallest modtime difference treated as a change
// to the file, as remotes may round modtimes more than they report.
const metaModTimeSlack = time.Second

// matches reports whether the record was stored for the file at node as it
// is now, rather than for a file since changed outside serve s3.
func (r *metaRecord) matches(node vfs.Node, precision time.Duration) bool {
	if r.Size != node.Size() {
		return false
	}
	if precision == fs.ModTimeNotSupported {
		return true
	}
	dt := r.ModTime.Sub(node.ModTime())
	if dt < 0 {
		dt = -dt
	}
	return dt <= max(precision, metaModTimeSlack)
}

// metadataStore stores object metadata. Records are grouped by a namespace
// (see metaNamespace) and keyed by "bucket/objectName" path within it.
type metadataStore interface {
	Load(ns, fp string) (rec metaRecord, found bool, err error)
	Store(ns, fp string, rec metaRecord) error
	Delete(ns, fp string) error
	DeleteAll(ns, bucket string) error
	Close() error
}

// metaNamespace returns the namespace for the metadata of objects in _vfs.
//
// Each auth proxy user gets their own, keyed on their access key ID as the
// names of the remotes the proxy makes change with the secret. Otherwise it
// is keyed on the remote so that a database reused for a different remote
// can't apply its records to it. Both are hashed so neither is persisted.
func metaNamespace(ctx context.Context, _vfs *vfs.VFS) string {
	if accessKey, ok := ctx.Value(ctxKeyAccessKey).(string); ok {
		return "key:" + sha256Hex(accessKey)
	}
	return "fs:" + sha256Hex(fs.ConfigString(_vfs.Fs()))
}

func sha256Hex(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

// loadMeta returns the stored metadata for the file at node, or nil if there
// is none or it was stored for a different version of the file.
func (b *s3Backend) loadMeta(ctx context.Context, _vfs *vfs.VFS, fp string, node vfs.Node) (map[string]string, error) {
	rec, found, err := b.meta.Load(metaNamespace(ctx, _vfs), fp)
	if err != nil {
		return nil, fmt.Errorf("failed to load metadata for %q: %w", fp, err)
	}
	// A stale record is left in place rather than deleted, as a concurrent
	// upload may have replaced it since node was read.
	if !found || !rec.matches(node, _vfs.Fs().Precision()) {
		return nil, nil
	}
	return rec.Meta, nil
}

// saveMeta stores meta for the file at fp as it is now.
func (b *s3Backend) saveMeta(ctx context.Context, _vfs *vfs.VFS, fp string, meta map[string]string) error {
	node, err := _vfs.Stat(fp)
	if err != nil {
		return err
	}
	rec := metaRecord{Meta: meta, Size: node.Size(), ModTime: node.ModTime()}
	if err := b.meta.Store(metaNamespace(ctx, _vfs), fp, rec); err != nil {
		return fmt.Errorf("failed to store metadata for %q: %w", fp, err)
	}
	return nil
}

// splitFp splits a "bucket/key" path into bucket and key.
func splitFp(fp string) (bucket, key string) {
	bucket, key, _ = strings.Cut(fp, "/")
	return bucket, key
}

type memoryMetaKey struct {
	ns, bucket, key string
}

func newMemoryMetaKey(ns, fp string) memoryMetaKey {
	bucket, key := splitFp(fp)
	return memoryMetaKey{ns: ns, bucket: bucket, key: key}
}

// memoryMetaStore keeps metadata in memory only.
type memoryMetaStore struct {
	m sync.Map
}

func newMemoryMetaStore() *memoryMetaStore {
	return &memoryMetaStore{}
}

func (s *memoryMetaStore) Load(ns, fp string) (metaRecord, bool, error) {
	val, ok := s.m.Load(newMemoryMetaKey(ns, fp))
	if !ok {
		return metaRecord{}, false, nil
	}
	return val.(metaRecord), true, nil
}

func (s *memoryMetaStore) Store(ns, fp string, rec metaRecord) error {
	// Callers modify the map after storing it, which would race with readers.
	rec.Meta = maps.Clone(rec.Meta)
	s.m.Store(newMemoryMetaKey(ns, fp), rec)
	return nil
}

func (s *memoryMetaStore) Delete(ns, fp string) error {
	s.m.Delete(newMemoryMetaKey(ns, fp))
	return nil
}

func (s *memoryMetaStore) DeleteAll(ns, bucket string) error {
	s.m.Range(func(k, _ any) bool {
		if key := k.(memoryMetaKey); key.ns == ns && key.bucket == bucket {
			s.m.Delete(k)
		}
		return true
	})
	return nil
}

func (s *memoryMetaStore) Close() error {
	return nil
}
