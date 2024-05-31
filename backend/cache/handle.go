//go:build !plan9 && !js

package cache

import (
	"context"
	"errors"
	"fmt"
	"io"
	"path"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/operations"
)

var uploaderMap = make(map[string]*backgroundWriter)
var uploaderMapMx sync.Mutex

// initBackgroundUploader returns a single instance
func initBackgroundUploader(fs *Fs) (*backgroundWriter, error) {
	// write lock to create one
	uploaderMapMx.Lock()
	defer uploaderMapMx.Unlock()
	if b, ok := uploaderMap[fs.String()]; ok {
		// if it was already started we close it so that it can be started again
		if b.running {
			b.close()
		} else {
			return b, nil
		}
	}

	bb := newBackgroundWriter(fs)
	uploaderMap[fs.String()] = bb
	return uploaderMap[fs.String()], nil
}

// Handle is managing the read/write/seek operations on an open handle
type Handle struct {
	ctx            context.Context
	cachedObject   *Object
	cfs            *Fs
	memory         *Memory
	preloadQueue   chan int64
	preloadOffset  int64
	offset         int64
	seenOffsets    map[int64]bool
	mu             sync.Mutex
	workersWg      sync.WaitGroup
	confirmReading chan bool
	workers        int
	maxWorkerID    int
	UseMemory      bool
	closed         bool
	reading        bool
}

// NewObjectHandle returns a new Handle for an existing Object
func NewObjectHandle(ctx context.Context, o *Object, cfs *Fs) *Handle {
	r := &Handle{
		ctx:           ctx,
		cachedObject:  o,
		cfs:           cfs,
		offset:        0,
		preloadOffset: -1, // -1 to trigger the first preload

		UseMemory: !cfs.opt.ChunkNoMemory,
		reading:   false,
	}
	r.seenOffsets = make(map[int64]bool)
	r.memory = NewMemory(-1)

	// create a larger buffer to queue up requests
	r.preloadQueue = make(chan int64, r.cfs.opt.TotalWorkers*10)
	r.confirmReading = make(chan bool)
	r.startReadWorkers()
	return r
}

// cacheFs is a convenience method to get the parent cache FS of the object's manager
func (r *Handle) cacheFs() *Fs {
	return r.cfs
}

// storage is a convenience method to get the persistent storage of the object's manager
func (r *Handle) storage() *Persistent {
	return r.cacheFs().cache
}

// String representation of this reader
func (r *Handle) String() string {
	return r.cachedObject.abs()
}

// startReadWorkers will start the worker pool
func (r *Handle) startReadWorkers() {
	if r.workers > 0 {
		return
	}
	totalWorkers := r.cacheFs().opt.TotalWorkers

	if r.cacheFs().plexConnector.isConfigured() {
		if !r.cacheFs().plexConnector.isConnected() {
			err := r.cacheFs().plexConnector.authenticate()
			if err != nil {
				fs.Errorf(r, "failed to authenticate to Plex: %v", err)
			}
		}
		if r.cacheFs().plexConnector.isConnected() {
			totalWorkers = 1
		}
	}

	r.scaleWorkers(totalWorkers)
}

// scaleWorkers will increase the worker pool count by the provided amount
func (r *Handle) scaleWorkers(desired int) {
	current := r.workers
	if current == desired {
		return
	}
	if current > desired {
		// scale in gracefully
		for r.workers > desired {
			r.preloadQueue <- -1
			r.workers--
		}
	} else {
		// scale out
		for r.workers < desired {
			w := &worker{
				r:  r,
				id: r.maxWorkerID,
			}
			r.workersWg.Add(1)
			r.workers++
			r.maxWorkerID++
			go w.run()
		}
	}
	// ignore first scale out from 0
	if current != 0 {
		fs.Debugf(r, "scale workers to %v", desired)
	}
}

func (r *Handle) confirmExternalReading() {
	// if we have a max value of workers
	// then we skip this step
	if r.workers > 1 ||
		!r.cacheFs().plexConnector.isConfigured() {
		return
	}
	if !r.cacheFs().plexConnector.isPlaying(r.cachedObject) {
		return
	}
	fs.Infof(r, "confirmed reading by external reader")
	r.scaleWorkers(r.cacheFs().opt.TotalWorkers)
}

// queueOffset will send an offset to the workers if it's different from the last one
func (r *Handle) queueOffset(offset int64) {
	if offset != r.preloadOffset {
		// clean past in-memory chunks
		if r.UseMemory {
			go r.memory.CleanChunksByNeed(offset)
		}
		r.confirmExternalReading()
		r.preloadOffset = offset

		// clear the past seen chunks
		// they will remain in our persistent storage but will be removed from transient
		// so they need to be picked up by a worker
		for k := range r.seenOffsets {
			if k < offset {
				r.seenOffsets[k] = false
			}
		}

		for i := 0; i < r.workers; i++ {
			o := r.preloadOffset + int64(r.cacheFs().opt.ChunkSize)*int64(i)
			if o < 0 || o >= r.cachedObject.Size() {
				continue
			}
			if v, ok := r.seenOffsets[o]; ok && v {
				continue
			}

			r.seenOffsets[o] = true
			r.preloadQueue <- o
		}
	}
}

// getChunk is called by the FS to retrieve a specific chunk of known start and size from where it can find it
// it can be from transient or persistent cache
// it will also build the chunk from the cache's specific chunk boundaries and build the final desired chunk in a buffer
func (r *Handle) getChunk(chunkStart int64) ([]byte, error) {
	var data []byte
	var err error

	// we calculate the modulus of the requested offset with the size of a chunk
	offset := chunkStart % int64(r.cacheFs().opt.ChunkSize)

	// we align the start offset of the first chunk to a likely chunk in the storage
	chunkStart -= offset
	r.queueOffset(chunkStart)
	found := false

	if r.UseMemory {
		data, err = r.memory.GetChunk(r.cachedObject, chunkStart)
		if err == nil {
			found = true
		}
	}

	if !found {
		// we're gonna give the workers a chance to pickup the chunk
		// and retry a couple of times
		for i := 0; i < r.cacheFs().opt.ReadRetries*8; i++ {
			data, err = r.storage().GetChunk(r.cachedObject, chunkStart)
			if err == nil {
				found = true
				break
			}

			fs.Debugf(r, "%v: chunk retry storage: %v", chunkStart, i)
			time.Sleep(time.Millisecond * 500)
		}
	}

	// not found in ram or
	// the worker didn't managed to download the chunk in time so we abort and close the stream
	if err != nil || len(data) == 0 || !found {
		if r.workers == 0 {
			fs.Errorf(r, "out of workers")
			return nil, io.ErrUnexpectedEOF
		}

		return nil, fmt.Errorf("chunk not found %v", chunkStart)
	}

	// first chunk will be aligned with the start
	if offset > 0 {
		if offset > int64(len(data)) {
			fs.Errorf(r, "unexpected conditions during reading. current position: %v, current chunk position: %v, current chunk size: %v, offset: %v, chunk size: %v, file size: %v",
				r.offset, chunkStart, len(data), offset, r.cacheFs().opt.ChunkSize, r.cachedObject.Size())
			return nil, io.ErrUnexpectedEOF
		}
		data = data[int(offset):]
	}

	return data, nil
}

// Read a chunk from storage or len(p)
func (r *Handle) Read(p []byte) (n int, err error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var buf []byte

	// first reading
	if !r.reading {
		r.reading = true
	}
	// reached EOF
	if r.offset >= r.cachedObject.Size() {
		return 0, io.EOF
	}
	currentOffset := r.offset
	buf, err = r.getChunk(currentOffset)
	if err != nil && err != io.EOF && err != io.ErrUnexpectedEOF {
		fs.Errorf(r, "(%v/%v) error (%v) response", currentOffset, r.cachedObject.Size(), err)
	}
	if len(buf) == 0 && err != io.ErrUnexpectedEOF {
		return 0, io.EOF
	}
	readSize := copy(p, buf)
	newOffset := currentOffset + int64(readSize)
	r.offset = newOffset

	return readSize, err
}

// Close will tell the workers to stop
func (r *Handle) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closed {
		return errors.New("file already closed")
	}

	close(r.preloadQueue)
	r.closed = true
	// wait for workers to complete their jobs before returning
	r.workersWg.Wait()
	r.memory.db.Flush()

	fs.Debugf(r, "cache reader closed %v", r.offset)
	return nil
}

// Seek will move the current offset based on whence and instruct the workers to move there too
func (r *Handle) Seek(offset int64, whence int) (int64, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	var err error
	switch whence {
	case io.SeekStart:
		fs.Debugf(r, "moving offset set from %v to %v", r.offset, offset)
		r.offset = offset
	case io.SeekCurrent:
		fs.Debugf(r, "moving offset cur from %v to %v", r.offset, r.offset+offset)
		r.offset += offset
	case io.SeekEnd:
		fs.Debugf(r, "moving offset end (%v) from %v to %v", r.cachedObject.Size(), r.offset, r.cachedObject.Size()+offset)
		r.offset = r.cachedObject.Size() + offset
	default:
		err = fmt.Errorf("cache: unimplemented seek whence %v", whence)
	}

	chunkStart := r.offset - (r.offset % int64(r.cacheFs().opt.ChunkSize))
	if chunkStart >= int64(r.cacheFs().opt.ChunkSize) {
		chunkStart -= int64(r.cacheFs().opt.ChunkSize)
	}
	r.queueOffset(chunkStart)

	return r.offset, err
}

type worker struct {
	r  *Handle
	rc io.ReadCloser
	id int
}

// String is a representation of this worker
func (w *worker) String() string {
	return fmt.Sprintf("worker-%v <%v>", w.id, w.r.cachedObject.Name)
}

// reader will return a reader depending on the capabilities of the source reader:
//   - if it supports seeking it will seek to the desired offset and return the same reader
//   - if it doesn't support seeking it will close a possible existing one and open at the desired offset
//   - if there's no reader associated with this worker, it will create one
func (w *worker) reader(offset, end int64, closeOpen bool) (io.ReadCloser, error) {
	var err error
	r := w.rc
	if w.rc == nil {
		r, err = w.r.cacheFs().openRateLimited(func() (io.ReadCloser, error) {
			return w.r.cachedObject.Object.Open(w.r.ctx, &fs.RangeOption{Start: offset, End: end - 1})
		})
		if err != nil {
			return nil, err
		}
		return r, nil
	}

	if !closeOpen {
		if do, ok := r.(fs.RangeSeeker); ok {
			_, err = do.RangeSeek(w.r.ctx, offset, io.SeekStart, end-offset)
			return r, err
		} else if do, ok := r.(io.Seeker); ok {
			_, err = do.Seek(offset, io.SeekStart)
			return r, err
		}
	}

	_ = w.rc.Close()
	return w.r.cacheFs().openRateLimited(func() (io.ReadCloser, error) {
		r, err = w.r.cachedObject.Object.Open(w.r.ctx, &fs.RangeOption{Start: offset, End: end - 1})
		if err != nil {
			return nil, err
		}
		return r, nil
	})
}

// run is the main loop for the worker which receives offsets to preload
func (w *worker) run() {
	var err error
	var data []byte
	defer func() {
		if w.rc != nil {
			_ = w.rc.Close()
		}
		w.r.workersWg.Done()
	}()

	for {
		chunkStart, open := <-w.r.preloadQueue
		if chunkStart < 0 || !open {
			break
		}

		// skip if it exists
		if w.r.UseMemory {
			if w.r.memory.HasChunk(w.r.cachedObject, chunkStart) {
				continue
			}

			// add it in ram if it's in the persistent storage
			data, err = w.r.storage().GetChunk(w.r.cachedObject, chunkStart)
			if err == nil {
				err = w.r.memory.AddChunk(w.r.cachedObject.abs(), data, chunkStart)
				if err != nil {
					fs.Errorf(w, "failed caching chunk in ram %v: %v", chunkStart, err)
				} else {
					continue
				}
			}
		} else if w.r.storage().HasChunk(w.r.cachedObject, chunkStart) {
			continue
		}

		chunkEnd := chunkStart + int64(w.r.cacheFs().opt.ChunkSize)
		// TODO: Remove this comment if it proves to be reliable for #1896
		//if chunkEnd > w.r.cachedObject.Size() {
		//	chunkEnd = w.r.cachedObject.Size()
		//}

		w.download(chunkStart, chunkEnd, 0)
	}
}

func (w *worker) download(chunkStart, chunkEnd int64, retry int) {
	var err error
	var data []byte

	// stop retries
	if retry >= w.r.cacheFs().opt.ReadRetries {
		return
	}
	// back-off between retries
	if retry > 0 {
		time.Sleep(time.Second * time.Duration(retry))
	}

	closeOpen := false
	if retry > 0 {
		closeOpen = true
	}
	w.rc, err = w.reader(chunkStart, chunkEnd, closeOpen)
	// we seem to be getting only errors so we abort
	if err != nil {
		fs.Errorf(w, "object open failed %v: %v", chunkStart, err)
		err = w.r.cachedObject.refreshFromSource(w.r.ctx, true)
		if err != nil {
			fs.Errorf(w, "%v", err)
		}
		w.download(chunkStart, chunkEnd, retry+1)
		return
	}

	data = make([]byte, chunkEnd-chunkStart)
	var sourceRead int
	sourceRead, err = io.ReadFull(w.rc, data)
	if err != nil && err != io.EOF && err != io.ErrUnexpectedEOF {
		fs.Errorf(w, "failed to read chunk %v: %v", chunkStart, err)
		err = w.r.cachedObject.refreshFromSource(w.r.ctx, true)
		if err != nil {
			fs.Errorf(w, "%v", err)
		}
		w.download(chunkStart, chunkEnd, retry+1)
		return
	}
	data = data[:sourceRead] // reslice to remove extra garbage
	if err == io.ErrUnexpectedEOF {
		fs.Debugf(w, "partial downloaded chunk %v", fs.SizeSuffix(chunkStart))
	} else {
		fs.Debugf(w, "downloaded chunk %v", chunkStart)
	}

	if w.r.UseMemory {
		err = w.r.memory.AddChunk(w.r.cachedObject.abs(), data, chunkStart)
		if err != nil {
			fs.Errorf(w, "failed caching chunk in ram %v: %v", chunkStart, err)
		}
	}

	err = w.r.storage().AddChunk(w.r.cachedObject.abs(), data, chunkStart)
	if err != nil {
		fs.Errorf(w, "failed caching chunk in storage %v: %v", chunkStart, err)
	}
}

const (
	// BackgroundUploadStarted is a state for a temp file that has started upload
	BackgroundUploadStarted = iota
	// BackgroundUploadCompleted is a state for a temp file that has completed upload
	BackgroundUploadCompleted
	// BackgroundUploadError is a state for a temp file that has an error upload
	BackgroundUploadError
)

// BackgroundUploadState is an entity that maps to an existing file which is stored on the temp fs
type BackgroundUploadState struct {
	Remote string
	Status int
	Error  error
}

type backgroundWriter struct {
	fs       *Fs
	stateCh  chan int
	running  bool
	notifyCh chan BackgroundUploadState
	mu       sync.Mutex
}

func newBackgroundWriter(f *Fs) *backgroundWriter {
	b := &backgroundWriter{
		fs:       f,
		stateCh:  make(chan int),
		notifyCh: make(chan BackgroundUploadState),
	}

	return b
}

func (b *backgroundWriter) close() {
	b.stateCh <- 2
	b.mu.Lock()
	defer b.mu.Unlock()
	b.running = false

}

func (b *backgroundWriter) pause() {
	b.stateCh <- 1
}

func (b *backgroundWriter) play() {
	b.stateCh <- 0
}

func (b *backgroundWriter) isRunning() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.running
}

func (b *backgroundWriter) notify(remote string, status int, err error) {
	state := BackgroundUploadState{
		Remote: remote,
		Status: status,
		Error:  err,
	}
	select {
	case b.notifyCh <- state:
		fs.Debugf(remote, "notified background upload state: %v", state.Status)
	default:
	}
}

func (b *backgroundWriter) run() {
	state := 0
	for {
		b.mu.Lock()
		b.running = true
		b.mu.Unlock()
		select {
		case s := <-b.stateCh:
			state = s
		default:
			//
		}
		switch state {
		case 1:
			runtime.Gosched()
			time.Sleep(time.Millisecond * 500)
			continue
		case 2:
			return
		}

		absPath, err := b.fs.cache.getPendingUpload(b.fs.Root(), time.Duration(b.fs.opt.TempWaitTime))
		if err != nil || absPath == "" || !b.fs.isRootInPath(absPath) {
			time.Sleep(time.Second)
			continue
		}

		remote := b.fs.cleanRootFromPath(absPath)
		b.notify(remote, BackgroundUploadStarted, nil)
		fs.Infof(remote, "background upload: started upload")
		err = operations.MoveFile(context.TODO(), b.fs.UnWrap(), b.fs.tempFs, remote, remote)
		if err != nil {
			b.notify(remote, BackgroundUploadError, err)
			_ = b.fs.cache.rollbackPendingUpload(absPath)
			fs.Errorf(remote, "background upload: %v", err)
			continue
		}
		// clean empty dirs up to root
		thisDir := cleanPath(path.Dir(remote))
		for thisDir != "" {
			thisList, err := b.fs.tempFs.List(context.TODO(), thisDir)
			if err != nil {
				break
			}
			if len(thisList) > 0 {
				break
			}
			err = b.fs.tempFs.Rmdir(context.TODO(), thisDir)
			fs.Debugf(thisDir, "cleaned from temp path")
			if err != nil {
				break
			}
			thisDir = cleanPath(path.Dir(thisDir))
		}
		fs.Infof(remote, "background upload: uploaded entry")
		err = b.fs.cache.removePendingUpload(absPath)
		if err != nil && !strings.Contains(err.Error(), "pending upload not found") {
			fs.Errorf(remote, "background upload: %v", err)
		}
		parentCd := NewDirectory(b.fs, cleanPath(path.Dir(remote)))
		err = b.fs.cache.ExpireDir(parentCd)
		if err != nil {
			fs.Errorf(parentCd, "background upload: cache expire error: %v", err)
		}
		b.fs.notifyChangeUpstream(remote, fs.EntryObject)
		fs.Infof(remote, "finished background upload")
		b.notify(remote, BackgroundUploadCompleted, nil)
	}
}

// Check the interfaces are satisfied
var (
	_ io.ReadCloser = (*Handle)(nil)
	_ io.Seeker     = (*Handle)(nil)
)
