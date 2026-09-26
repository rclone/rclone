//go:build !plan9 && !js

package s3

import (
	"encoding/json"
	"errors"
	"time"

	bolt "go.etcd.io/bbolt"
	berrors "go.etcd.io/bbolt/errors"
)

// boltMetaStore persists metadata to a bbolt database. Each namespace is a
// top level bbolt bucket holding a nested bbolt bucket per S3 bucket, which
// maps object keys to JSON encoded metaRecords.
type boltMetaStore struct {
	db *bolt.DB
}

func newBoltMetaStore(path string) (*boltMetaStore, error) {
	// Without a timeout, Open blocks forever if another process holds the lock.
	db, err := bolt.Open(path, 0600, &bolt.Options{Timeout: time.Second})
	if err != nil {
		return nil, err
	}
	return &boltMetaStore{db: db}, nil
}

// bucket returns the bbolt bucket for the S3 bucket in namespace ns, or nil
// if it doesn't exist.
func (s *boltMetaStore) bucket(tx *bolt.Tx, ns, bucket string) *bolt.Bucket {
	nsBucket := tx.Bucket([]byte(ns))
	if nsBucket == nil {
		return nil
	}
	return nsBucket.Bucket([]byte(bucket))
}

func (s *boltMetaStore) Load(ns, fp string) (rec metaRecord, found bool, err error) {
	bucket, key := splitFp(fp)
	err = s.db.View(func(tx *bolt.Tx) error {
		b := s.bucket(tx, ns, bucket)
		if b == nil {
			return nil
		}
		data := b.Get([]byte(key))
		if data == nil {
			return nil
		}
		found = true
		return json.Unmarshal(data, &rec)
	})
	if err != nil {
		return metaRecord{}, false, err
	}
	return rec, found, nil
}

func (s *boltMetaStore) Store(ns, fp string, rec metaRecord) error {
	bucket, key := splitFp(fp)
	data, err := json.Marshal(rec)
	if err != nil {
		return err
	}
	return s.db.Update(func(tx *bolt.Tx) error {
		nsBucket, err := tx.CreateBucketIfNotExists([]byte(ns))
		if err != nil {
			return err
		}
		b, err := nsBucket.CreateBucketIfNotExists([]byte(bucket))
		if err != nil {
			return err
		}
		return b.Put([]byte(key), data)
	})
}

func (s *boltMetaStore) Delete(ns, fp string) error {
	bucket, key := splitFp(fp)
	return s.db.Update(func(tx *bolt.Tx) error {
		b := s.bucket(tx, ns, bucket)
		if b == nil {
			return nil
		}
		return b.Delete([]byte(key))
	})
}

func (s *boltMetaStore) DeleteAll(ns, bucket string) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		nsBucket := tx.Bucket([]byte(ns))
		if nsBucket == nil {
			return nil
		}
		err := nsBucket.DeleteBucket([]byte(bucket))
		if errors.Is(err, berrors.ErrBucketNotFound) {
			return nil
		}
		return err
	})
}

func (s *boltMetaStore) Close() error {
	return s.db.Close()
}
