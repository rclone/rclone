// Package raid3 implements a backend that splits data across three remotes using byte-level striping
package raid3

// This file contains the heal infrastructure for automatic particle reconstruction.
//
// It includes:
//   - uploadJob and uploadQueue types for managing background uploads
//   - backgroundUploader: Goroutine workers for processing heal uploads
//   - uploadParticle: Upload a reconstructed particle to its backend
//   - queueParticleUpload: Queue a particle for background upload (deduplicated)
//   - Heal is triggered automatically when reading degraded objects (2/3 particles)

import (
	"bytes"
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/object"
)

// uploadJob represents a particle that needs to be uploaded for heal
type uploadJob struct {
	remote       string
	particleType string // "even", "odd", or "parity"
	data         []byte
	isOddLength  bool
}

// uploadQueue manages pending heal uploads
type uploadQueue struct {
	mu      sync.Mutex
	pending map[string]bool // key: remote+particleType, value: true if queued
	jobs    chan *uploadJob
}

// newUploadQueue creates a new upload queue
func newUploadQueue() *uploadQueue {
	return &uploadQueue{
		pending: make(map[string]bool),
		jobs:    make(chan *uploadJob, defaultUploadQueueSize),
	}
}

// add adds a job to the queue (deduplicates)
func (q *uploadQueue) add(job *uploadJob) bool {
	q.mu.Lock()
	defer q.mu.Unlock()

	key := job.remote + ":" + job.particleType
	if q.pending[key] {
		return false // Already queued
	}

	q.pending[key] = true
	q.jobs <- job
	return true
}

// remove removes a job from the pending map
func (q *uploadQueue) remove(job *uploadJob) {
	q.mu.Lock()
	defer q.mu.Unlock()

	key := job.remote + ":" + job.particleType
	delete(q.pending, key)
}

// len returns the number of pending uploads
func (q *uploadQueue) len() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return len(q.pending)
}

// backgroundUploader runs as a goroutine to process heal uploads
func (f *Fs) backgroundUploader(ctx context.Context, workerID int) {
	fs.Debugf(f, "Heal worker %d started", workerID)
	defer fs.Debugf(f, "Heal worker %d stopped", workerID)

	for {
		select {
		case job, ok := <-f.uploadQueue.jobs:
			if !ok {
				// Channel closed, exit
				return
			}

			fs.Infof(f, "Heal: uploading %s particle for %s", job.particleType, job.remote)

			err := f.uploadParticle(ctx, job)
			if err != nil {
				fs.Errorf(f, "Heal upload failed for %s (%s): %v", job.remote, job.particleType, err)
				// TODO: Could implement retry logic here
			} else {
				fs.Infof(f, "Heal upload completed for %s (%s)", job.remote, job.particleType)
			}

			// Remove from pending map and mark as done
			f.uploadQueue.remove(job)
			f.uploadWg.Done()

		case <-ctx.Done():
			// Context cancelled, exit
			return
		}
	}
}

// uploadParticle uploads a single particle to its backend.
// Reads the full footer from one of the other two particles and appends a 90-byte footer to the payload.
func (f *Fs) uploadParticle(ctx context.Context, job *uploadJob) error {
	var targetFs fs.Fs
	var filename string

	switch job.particleType {
	case "even":
		targetFs = f.even
		filename = job.remote
	case "odd":
		targetFs = f.odd
		filename = job.remote
	case "parity":
		targetFs = f.parity
		filename = job.remote
	default:
		return fmt.Errorf("unknown particle type: %s", job.particleType)
	}

	var currentShard int
	switch job.particleType {
	case "even":
		currentShard = ShardEven
	case "odd":
		currentShard = ShardOdd
	case "parity":
		currentShard = ShardParity
	default:
		currentShard = 0
	}

	// Read full footer from one of the other two particles (same remote; footers are identical except CurrentShard)
	var sourceFt *Footer
	tryOrder := []struct {
		fs   fs.Fs
		name string
	}{}
	switch job.particleType {
	case "even":
		tryOrder = []struct{ fs fs.Fs; name string }{{f.odd, "odd"}, {f.parity, "parity"}}
	case "odd":
		tryOrder = []struct{ fs fs.Fs; name string }{{f.even, "even"}, {f.parity, "parity"}}
	case "parity":
		tryOrder = []struct{ fs fs.Fs; name string }{{f.even, "even"}, {f.odd, "odd"}}
	}
	for _, b := range tryOrder {
		obj, err := b.fs.NewObject(ctx, job.remote)
		if err != nil {
			continue
		}
		sourceFt, err = readFooterFromBackendObject(ctx, obj)
		if err != nil {
			continue
		}
		break
	}
	if sourceFt == nil {
		return fmt.Errorf("could not read footer from any other particle for heal of %s", job.remote)
	}

	ft := FooterFromReconstructed(sourceFt.ContentLength, sourceFt.MD5[:], sourceFt.SHA256[:], time.Unix(sourceFt.Mtime, 0), sourceFt.Compression, currentShard)
	fb, errMarshal := ft.MarshalBinary()
	if errMarshal != nil {
		return errMarshal
	}
	payload := make([]byte, len(job.data)+FooterSize)
	copy(payload, job.data)
	copy(payload[len(job.data):], fb)

	// Create a basic ObjectInfo for the particle
	baseInfo := object.NewStaticObjectInfo(filename, time.Now(), int64(len(payload)), true, nil, nil)

	src := &particleObjectInfo{
		ObjectInfo: baseInfo,
		remote:     filename,
		size:       int64(len(payload)),
	}

	// Upload the particle
	reader := bytes.NewReader(payload)
	_, err := targetFs.Put(ctx, reader, src)
	return err
}

// queueParticleUpload queues a particle for background upload
func (f *Fs) queueParticleUpload(remote, particleType string, data []byte, isOddLength bool) {
	job := &uploadJob{
		remote:       remote,
		particleType: particleType,
		data:         data,
		isOddLength:  isOddLength,
	}

	if f.uploadQueue.add(job) {
		f.uploadWg.Add(1)
		fs.Infof(f, "Queued %s particle for heal upload: %s", particleType, remote)
	} else {
		fs.Debugf(f, "Upload already queued for %s particle: %s", particleType, remote)
	}
}
