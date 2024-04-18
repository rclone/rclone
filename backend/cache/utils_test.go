//go:build !plan9 && !js
// +build !plan9,!js

package cache

import bolt "go.etcd.io/bbolt"

// PurgeTempUploads will remove all the pending uploads from the queue
func (b *Persistent) PurgeTempUploads() {
	b.tempQueueMux.Lock()
	defer b.tempQueueMux.Unlock()

	_ = b.db.Update(func(tx *bolt.Tx) error {
		_ = tx.DeleteBucket([]byte(tempBucket))
		_, _ = tx.CreateBucketIfNotExists([]byte(tempBucket))
		return nil
	})
}

// SetPendingUploadToStarted is a way to mark an entry as started (even if it's not already)
func (b *Persistent) SetPendingUploadToStarted(remote string) error {
	return b.updatePendingUpload(remote, func(item *tempUploadInfo) error {
		item.Started = true
		return nil
	})
}
