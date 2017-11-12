// +build !plan9

package cache

import (
	"fmt"
	"io"
	"os"
	"sync"
	"time"

	"github.com/ncw/rclone/fs"
	"github.com/pkg/errors"
)

// Handle is managing the read/write/seek operations on an open handle
type Handle struct {
	cachedObject  *Object
	memory        ChunkStorage
	preloadQueue  chan int64
	preloadOffset int64
	offset        int64
	seenOffsets   map[int64]bool
	mu            sync.Mutex

	ReadRetries  int
	TotalWorkers int
	UseMemory    bool
	workers      []*worker
	chunkAge     time.Duration
	warmup       bool
	closed       bool
}

// NewObjectHandle returns a new Handle for an existing Object
func NewObjectHandle(o *Object) *Handle {
	r := &Handle{
		cachedObject:  o,
		offset:        0,
		preloadOffset: -1, // -1 to trigger the first preload

		ReadRetries:  o.CacheFs.readRetries,
		TotalWorkers: o.CacheFs.totalWorkers,
		UseMemory:    o.CacheFs.chunkMemory,
		chunkAge:     o.CacheFs.chunkAge,
		warmup:       o.CacheFs.InWarmUp(),
	}
	r.seenOffsets = make(map[int64]bool)
	r.memory = NewMemory(o.CacheFs.chunkAge)
	if o.CacheFs.InWarmUp() {
		r.chunkAge = o.CacheFs.metaAge
	}

	// create a larger buffer to queue up requests
	r.preloadQueue = make(chan int64, r.TotalWorkers*10)
	r.startReadWorkers()
	return r
}

// cacheFs is a convenience method to get the parent cache FS of the object's manager
func (r *Handle) cacheFs() *Fs {
	return r.cachedObject.CacheFs
}

// storage is a convenience method to get the persistent storage of the object's manager
func (r *Handle) storage() Storage {
	return r.cacheFs().cache
}

// String representation of this reader
func (r *Handle) String() string {
	return r.cachedObject.abs()
}

// InWarmUp says if this handle is in warmup mode
func (r *Handle) InWarmUp() bool {
	return r.warmup
}

// startReadWorkers will start the worker pool
func (r *Handle) startReadWorkers() {
	if r.hasAtLeastOneWorker() {
		return
	}
	for i := 0; i < r.TotalWorkers; i++ {
		w := &worker{
			r:  r,
			ch: r.preloadQueue,
			id: i,
		}
		go w.run()

		r.workers = append(r.workers, w)
	}
}

// queueOffset will send an offset to the workers if it's different from the last one
func (r *Handle) queueOffset(offset int64) {
	if offset != r.preloadOffset {
		r.preloadOffset = offset
		previousChunksCounter := 0
		maxOffset := r.cacheFs().chunkSize * int64(r.cacheFs().originalSettingWorkers())

		// clear the past seen chunks
		// they will remain in our persistent storage but will be removed from transient
		// so they need to be picked up by a worker
		for k := range r.seenOffsets {
			if k < offset {
				r.seenOffsets[k] = false

				// we count how many continuous chunks were seen before
				if offset >= maxOffset && k >= offset-maxOffset {
					previousChunksCounter++
				}
			}
		}

		// if we read all the previous chunks that could have been preloaded
		// we should then disable warm up setting for this handle
		if r.warmup && previousChunksCounter >= r.cacheFs().originalSettingWorkers() {
			r.TotalWorkers = r.cacheFs().originalSettingWorkers()
			r.UseMemory = !r.cacheFs().originalSettingChunkNoMemory()
			r.chunkAge = r.cacheFs().chunkAge
			r.warmup = false
			fs.Infof(r, "disabling warm up")
		}

		for i := 0; i < r.TotalWorkers; i++ {
			o := r.preloadOffset + r.cacheFs().chunkSize*int64(i)
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

func (r *Handle) hasAtLeastOneWorker() bool {
	oneWorker := false
	for i := 0; i < len(r.workers); i++ {
		if r.workers[i].isRunning() {
			oneWorker = true
		}
	}
	return oneWorker
}

// getChunk is called by the FS to retrieve a specific chunk of known start and size from where it can find it
// it can be from transient or persistent cache
// it will also build the chunk from the cache's specific chunk boundaries and build the final desired chunk in a buffer
func (r *Handle) getChunk(chunkStart int64) ([]byte, error) {
	var data []byte
	var err error

	// we reached the end of the file
	if chunkStart >= r.cachedObject.Size() {
		fs.Debugf(r, "reached EOF %v", chunkStart)
		return nil, io.EOF
	}

	// we calculate the modulus of the requested offset with the size of a chunk
	offset := chunkStart % r.cacheFs().chunkSize

	// we align the start offset of the first chunk to a likely chunk in the storage
	chunkStart = chunkStart - offset
	r.queueOffset(chunkStart)
	found := false

	// delete old chunks from memory
	if r.UseMemory {
		go r.memory.CleanChunksByNeed(chunkStart)

		data, err = r.memory.GetChunk(r.cachedObject, chunkStart)
		if err == nil {
			found = true
		}
	}

	if !found {
		// we're gonna give the workers a chance to pickup the chunk
		// and retry a couple of times
		for i := 0; i < r.ReadRetries; i++ {
			data, err = r.storage().GetChunk(r.cachedObject, chunkStart)

			if err == nil {
				found = true
				break
			}

			fs.Debugf(r, "%v: chunk retry storage: %v", chunkStart, i)
			time.Sleep(time.Second)
		}
	}

	// not found in ram or
	// the worker didn't managed to download the chunk in time so we abort and close the stream
	if err != nil || len(data) == 0 || !found {
		if !r.hasAtLeastOneWorker() {
			fs.Errorf(r, "out of workers")
			return nil, io.ErrUnexpectedEOF
		}

		return nil, errors.Errorf("chunk not found %v", chunkStart)
	}

	// first chunk will be aligned with the start
	if offset > 0 {
		data = data[int(offset):]
	}

	return data, nil
}

// Read a chunk from storage or len(p)
func (r *Handle) Read(p []byte) (n int, err error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var buf []byte

	currentOffset := r.offset
	buf, err = r.getChunk(currentOffset)
	if err != nil && len(buf) == 0 {
		return 0, io.EOF
	}
	readSize := copy(p, buf)
	newOffset := currentOffset + int64(readSize)
	r.offset = newOffset
	if r.offset >= r.cachedObject.Size() {
		err = io.EOF
	}

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
	waitCount := 3
	for i := 0; i < len(r.workers); i++ {
		waitIdx := 0
		for r.workers[i].isRunning() && waitIdx < waitCount {
			time.Sleep(time.Second)
			waitIdx++
		}
	}

	fs.Debugf(r, "cache reader closed %v", r.offset)
	return nil
}

// Seek will move the current offset based on whence and instruct the workers to move there too
func (r *Handle) Seek(offset int64, whence int) (int64, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	var err error
	switch whence {
	case os.SEEK_SET:
		fs.Debugf(r, "moving offset set from %v to %v", r.offset, offset)
		r.offset = offset
	case os.SEEK_CUR:
		fs.Debugf(r, "moving offset cur from %v to %v", r.offset, r.offset+offset)
		r.offset += offset
	case os.SEEK_END:
		fs.Debugf(r, "moving offset end (%v) from %v to %v", r.cachedObject.Size(), r.offset, r.cachedObject.Size()+offset)
		r.offset = r.cachedObject.Size() + offset
	default:
		err = errors.Errorf("cache: unimplemented seek whence %v", whence)
	}

	chunkStart := r.offset - (r.offset % r.cacheFs().chunkSize)
	if chunkStart >= r.cacheFs().chunkSize {
		chunkStart = chunkStart - r.cacheFs().chunkSize
	}
	r.queueOffset(chunkStart)

	return r.offset, err
}

type worker struct {
	r       *Handle
	ch      <-chan int64
	rc      io.ReadCloser
	id      int
	running bool
	mu      sync.Mutex
}

// String is a representation of this worker
func (w *worker) String() string {
	return fmt.Sprintf("worker-%v <%v>", w.id, w.r.cachedObject.Name)
}

// reader will return a reader depending on the capabilities of the source reader:
//   - if it supports seeking it will seek to the desired offset and return the same reader
//   - if it doesn't support seeking it will close a possible existing one and open at the desired offset
//   - if there's no reader associated with this worker, it will create one
func (w *worker) reader(offset, end int64) (io.ReadCloser, error) {
	var err error
	r := w.rc
	if w.rc == nil {
		r, err = w.r.cacheFs().OpenRateLimited(func() (io.ReadCloser, error) {
			return w.r.cachedObject.Object.Open(&fs.RangeOption{Start: offset, End: end}, &fs.SeekOption{Offset: offset})
		})
		if err != nil {
			return nil, err
		}
		return r, nil
	}

	seekerObj, ok := r.(io.Seeker)
	if ok {
		_, err = seekerObj.Seek(offset, os.SEEK_SET)
		return r, err
	}

	_ = w.rc.Close()
	return w.r.cacheFs().OpenRateLimited(func() (io.ReadCloser, error) {
		r, err = w.r.cachedObject.Object.Open(&fs.RangeOption{Start: offset, End: end}, &fs.SeekOption{Offset: offset})
		if err != nil {
			return nil, err
		}
		return r, nil
	})
}

func (w *worker) isRunning() bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.running
}

func (w *worker) setRunning(f bool) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.running = f
}

// run is the main loop for the worker which receives offsets to preload
func (w *worker) run() {
	var err error
	var data []byte
	defer w.setRunning(false)
	defer func() {
		if w.rc != nil {
			_ = w.rc.Close()
			w.setRunning(false)
		}
	}()

	for {
		chunkStart, open := <-w.ch
		w.setRunning(true)
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
				err = w.r.memory.AddChunk(w.r.cachedObject, data, chunkStart)
				if err != nil {
					fs.Errorf(w, "failed caching chunk in ram %v: %v", chunkStart, err)
				} else {
					continue
				}
			}
			err = nil
		} else {
			if w.r.storage().HasChunk(w.r.cachedObject, chunkStart) {
				continue
			}
		}

		chunkEnd := chunkStart + w.r.cacheFs().chunkSize
		if chunkEnd > w.r.cachedObject.Size() {
			chunkEnd = w.r.cachedObject.Size()
		}
		w.rc, err = w.reader(chunkStart, chunkEnd)
		// we seem to be getting only errors so we abort
		if err != nil {
			fs.Errorf(w, "object open failed %v: %v", chunkStart, err)
			return
		}

		data = make([]byte, chunkEnd-chunkStart)
		sourceRead := 0
		sourceRead, err = io.ReadFull(w.rc, data)
		if err != nil && err != io.EOF && err != io.ErrUnexpectedEOF {
			fs.Errorf(w, "failed to read chunk %v: %v", chunkStart, err)
			return
		}
		if err == io.ErrUnexpectedEOF {
			fs.Debugf(w, "partial read chunk %v: %v", chunkStart, err)
		}
		data = data[:sourceRead] // reslice to remove extra garbage
		fs.Debugf(w, "downloaded chunk %v", fs.SizeSuffix(chunkStart))

		if w.r.UseMemory {
			err = w.r.memory.AddChunk(w.r.cachedObject, data, chunkStart)
			if err != nil {
				fs.Errorf(w, "failed caching chunk in ram %v: %v", chunkStart, err)
			}
		}

		err = w.r.storage().AddChunkAhead(w.r.cachedObject.abs(), data, chunkStart, w.r.chunkAge)
		if err != nil {
			fs.Errorf(w, "failed caching chunk in storage %v: %v", chunkStart, err)
		}
	}
}

// Check the interfaces are satisfied
var (
	_ io.ReadCloser = (*Handle)(nil)
	_ io.Seeker     = (*Handle)(nil)
)
