// WriteBack manages asynchronous uploads to the remote
//
// This package keeps track of in progress uploads and provides
// methods to manage them. It also handles the queuing of uploads
// and the retry mechanism.

package writeback

import (
	"sync"
	"time"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/lib/pacer"
	"github.com/rclone/rclone/vfs/vfscommon"
)

// Handle is a handle for a writeback item
type Handle int64

// writeBackItem describes a single item in the cache which is
// having its attributes set or is being uploaded
type writeBackItem struct {
	mu         sync.Mutex
	wb         *WriteBack
	id         Handle
	name       string
	o          fs.Object
	src        fs.Object
	uploading  bool
	attempt    int
	delay      time.Duration
	expiry     time.Time
	retryDelay time.Duration
}

// IsUploading returns true if the item is currently being uploaded
func (wbItem *writeBackItem) IsUploading() bool {
	wbItem.mu.Lock()
	defer wbItem.mu.Unlock()
	return wbItem.uploading
}

// WriteBack manages a list of items which are in the process of
// having their attributes set or being uploaded
type WriteBack struct {
	mu       sync.Mutex
	cond     *sync.Cond
	lookup   map[Handle]*writeBackItem
	nextID   Handle
	fetching map[string]struct{}
	pacer    *pacer.Pacer
	opt      *vfscommon.Options
	shutdown bool
}

// New creates a new WriteBack
func New(opt *vfscommon.Options) *WriteBack {
	wb := &WriteBack{
		lookup:   make(map[Handle]*writeBackItem),
		fetching: make(map[string]struct{}),
		pacer:    pacer.New(),
		opt:      opt,
	}
	wb.cond = sync.NewCond(&wb.mu)
	return wb
}

// IsUploading returns true if the item is currently being uploaded
func (wb *WriteBack) IsUploading(id Handle) bool {
	wb.mu.Lock()
	defer wb.mu.Unlock()

	if wbItem, ok := wb.lookup[id]; ok {
		return wbItem.uploading
	}
	return false
}

// Get returns a writeback item by handle if it exists
func (wb *WriteBack) Get(id Handle) *writeBackItem {
	wb.mu.Lock()
	defer wb.mu.Unlock()

	return wb.lookup[id]
}

// Add adds a new item to the writeback queue
func (wb *WriteBack) Add(name string, o fs.Object, src fs.Object) Handle {
	wb.mu.Lock()
	defer wb.mu.Unlock()

	id := wb.nextID
	wb.nextID++

	wbItem := &writeBackItem{
		wb:    wb,
		id:    id,
		name:  name,
		o:     o,
		src:   src,
		delay: time.Duration(wb.opt.WriteBack),
	}
	wb.lookup[id] = wbItem

	return id
}

// Remove removes an item from the writeback queue
func (wb *WriteBack) Remove(id Handle) (found bool) {
	wb.mu.Lock()
	defer wb.mu.Unlock()

	return wb._remove(id)
}

// _remove removes an item from the writeback queue
// Call with lock held
func (wb *WriteBack) _remove(id Handle) (found bool) {
	wbItem, ok := wb.lookup[id]
	if !ok {
		return false
	}
	delete(wb.lookup, id)
	wb.cond.Broadcast()
	// Wake up the background uploader
	wbItem.wb.cond.Signal()
	return true
}