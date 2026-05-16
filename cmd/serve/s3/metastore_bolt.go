//go:build !plan9 && !js

package s3

import (
	"encoding/json"
	"strings"

	"github.com/rclone/rclone/fs"
	bolt "go.etcd.io/bbolt"
	berrors "go.etcd.io/bbolt/errors"
)

// boltMetaStore persists metadata to a bbolt database.
// Each S3 bucket maps to a bbolt bucket; object keys are bbolt keys;
// values are JSON-encoded map[string]string.
type boltMetaStore struct {
	db *bolt.DB
}

func newBoltMetaStore(path string) (*boltMetaStore, error) {
	db, err := bolt.Open(path, 0600, nil)
	if err != nil {
		return nil, err
	}
	return &boltMetaStore{db: db}, nil
}

// splitFp splits a "bucket/key" path into bucket and key.
func splitFp(fp string) (bucket, key string) {
	bucket, key, _ = strings.Cut(fp, "/")
	return bucket, key
}

func (s *boltMetaStore) Load(fp string) (map[string]string, bool) {
	bucket, key := splitFp(fp)
	var meta map[string]string
	err := s.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(bucket))
		if b == nil {
			return nil
		}
		data := b.Get([]byte(key))
		if data == nil {
			return nil
		}
		return json.Unmarshal(data, &meta)
	})
	if err != nil {
		fs.Errorf("serve s3", "failed to load metadata for %s: %v", fp, err)
		return nil, false
	}
	return meta, meta != nil
}

func (s *boltMetaStore) Store(fp string, meta map[string]string) {
	bucket, key := splitFp(fp)
	err := s.db.Update(func(tx *bolt.Tx) error {
		b, err := tx.CreateBucketIfNotExists([]byte(bucket))
		if err != nil {
			return err
		}
		data, err := json.Marshal(meta)
		if err != nil {
			return err
		}
		return b.Put([]byte(key), data)
	})
	if err != nil {
		fs.Errorf("serve s3", "failed to store metadata for %s: %v", fp, err)
	}
}

func (s *boltMetaStore) Delete(fp string) {
	bucket, key := splitFp(fp)
	err := s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(bucket))
		if b == nil {
			return nil
		}
		return b.Delete([]byte(key))
	})
	if err != nil {
		fs.Errorf("serve s3", "failed to delete metadata for %s: %v", fp, err)
	}
}

func (s *boltMetaStore) DeleteAll(bucket string) {
	err := s.db.Update(func(tx *bolt.Tx) error {
		return tx.DeleteBucket([]byte(bucket))
	})
	if err != nil && err != berrors.ErrBucketNotFound {
		fs.Errorf("serve s3", "failed to delete metadata for bucket %s: %v", bucket, err)
	}
}

func (s *boltMetaStore) Close() error {
	return s.db.Close()
}
